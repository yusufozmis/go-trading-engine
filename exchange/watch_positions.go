package exchange

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// PositionUpdate contains either the latest exchange-reported futures position
// state or a terminal stream error. Optional fields remain nil when omitted by
// the provider.
type PositionUpdate struct {
	PositionID string
	Symbol     string
	Side       types.PositionSide
	Contracts  float64
	Notional   *float64
	Timestamp  *int64
	EntryPrice *float64
	MarkPrice  *float64

	LiquidationPrice *float64
	Leverage         *float64
	UnrealizedPnL    *float64
	RealizedPnL      *float64
	MarginMode       types.MarginMode
	Hedged           *bool

	// Err is non-nil only when the watcher has stopped after a terminal failure.
	Err error
}

// Closed reports whether the exchange update states that no contracts remain.
func (update PositionUpdate) Closed() bool {
	return update.Err == nil && update.Contracts == 0
}

type positionStream struct {
	updates chan PositionUpdate
	done    chan struct{}

	mu       sync.Mutex
	closed   bool
	closeErr error

	wg        sync.WaitGroup
	closeOnce sync.Once
}

func (stream *positionStream) isClosed() bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	return stream.closed
}

func (stream *positionStream) emit(update PositionUpdate) bool {
	select {
	case <-stream.done:
		return false
	case stream.updates <- update:
		return true
	}
}

func (stream *positionStream) waitForRetry(delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-stream.done:
		return false
	case <-timer.C:
		return true
	}
}

// RunPositionStream starts one private WebSocket watcher for the requested
// futures symbols. The returned channel receives the initial provider snapshot
// when available and every later position update until ClosePositionStream is
// called or a terminal watcher error occurs. Passing no symbols watches all
// futures positions supported by the configured account.
func (client *Client) RunPositionStream(symbols ...string) (<-chan PositionUpdate, error) {
	if err := client.validateConfigured(); err != nil {
		return nil, err
	}
	if client.positionExchange == nil {
		return nil, apperrors.ErrUninitializedClient
	}
	if client.positionStream != nil {
		return nil, apperrors.ErrPositionStreamAlreadyExists
	}

	for _, symbol := range symbols {
		if symbol == "" {
			return nil, apperrors.ErrNilSymbol
		}

		market, exists := client.markets[symbol]
		if !exists || market.Swap == nil || !*market.Swap {
			return nil, apperrors.ErrInvalidSymbol
		}
	}

	bufferSize := max(len(symbols)*2, 2)
	stream := &positionStream{
		updates: make(chan PositionUpdate, bufferSize),
		done:    make(chan struct{}),
	}
	client.positionStream = stream

	stream.wg.Add(1)
	go func() {
		defer stream.wg.Done()
		client.watchPositions(symbols)
	}()

	return stream.updates, nil
}

// ClosePositionStream stops the private position watcher without closing the
// main exchange client or the candle stream.
func (client *Client) ClosePositionStream() error {
	if err := client.Validate(); err != nil {
		return err
	}
	if client.positionStream == nil {
		return apperrors.ErrPositionStreamNotRunning
	}

	stream := client.positionStream
	stream.closeOnce.Do(func() {
		stream.mu.Lock()
		stream.closed = true
		stream.mu.Unlock()
		close(stream.done)

		var closeErrors []error
		// Closing the dedicated instance interrupts a blocked WatchPositions call
		// without affecting order submission or public candle subscriptions.
		for _, err := range client.positionExchange.Close() {
			closeErrors = append(closeErrors, normalizeError(err))
		}

		stream.wg.Wait()
		close(stream.updates)
		client.positionStream = nil

		if len(closeErrors) > 0 {
			stream.closeErr = errors.Join(closeErrors...)
		}
	})

	return stream.closeErr
}

func (client *Client) watchPositions(symbols []string) {
	stream := client.positionStream

	var options []ccxt.WatchPositionsOptions
	if len(symbols) > 0 {
		options = append(options, ccxt.WithWatchPositionsSymbols(symbols))
	}

	retryCount := 0
	for {
		positions, err := client.positionExchange.WatchPositions(options...)
		if err != nil {
			if stream.isClosed() {
				return
			}

			if isRetryableWatchError(err) && retryCount < maxWatchRetries {
				retryCount++
				delay := watchRetryBaseWait * time.Duration(1<<(retryCount-1))
				if !stream.waitForRetry(delay) {
					return
				}
				continue
			}

			stream.emit(PositionUpdate{
				Err: fmt.Errorf("watch positions: %w", normalizeError(err)),
			})
			return
		}
		retryCount = 0

		for _, position := range positions {
			update, err := wrapPositionUpdate(position)
			if err != nil {
				stream.emit(PositionUpdate{Err: err})
				return
			}
			if !stream.emit(update) {
				return
			}
		}
	}
}

func wrapPositionUpdate(position ccxt.Position) (PositionUpdate, error) {
	if position.Symbol == nil || *position.Symbol == "" {
		return PositionUpdate{}, fmt.Errorf("watch positions: provider omitted symbol")
	}
	if position.Contracts == nil || math.IsNaN(*position.Contracts) ||
		math.IsInf(*position.Contracts, 0) || *position.Contracts < 0 {
		return PositionUpdate{}, fmt.Errorf(
			"watch positions %q: %w",
			*position.Symbol,
			apperrors.ErrInvalidAmount,
		)
	}
	update := PositionUpdate{
		Notional:         position.Notional,
		Symbol:           *position.Symbol,
		Contracts:        *position.Contracts,
		EntryPrice:       position.EntryPrice,
		MarkPrice:        position.MarkPrice,
		LiquidationPrice: position.LiquidationPrice,
		Leverage:         position.Leverage,
		UnrealizedPnL:    position.UnrealizedPnl,
		RealizedPnL:      position.RealizedPnl,
		Hedged:           position.Hedged,
	}
	if position.Id != nil {
		update.PositionID = *position.Id
	}

	if position.Side != nil {
		side := types.PositionSide(*position.Side)
		if side.Valid() {
			update.Side = side
		}
	}
	if position.MarginMode != nil {
		marginMode := types.MarginMode(*position.MarginMode)
		if marginMode.Valid() {
			update.MarginMode = marginMode
		}
	}

	timestamp := position.LastUpdateTimestamp
	if timestamp == nil {
		timestamp = position.Timestamp
	}
	if timestamp != nil && !math.IsNaN(*timestamp) && !math.IsInf(*timestamp, 0) {
		value := int64(*timestamp)
		update.Timestamp = &value
	}

	return update, nil
}
