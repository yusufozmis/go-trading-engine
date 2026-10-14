package exchange

import (
	"fmt"
	"log"
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
	// updates survives reconnects. Only the supervisor sends to and closes it;
	// individual CCXT watchers send their results through an internal channel.
	updates chan PositionUpdate
	// done stops the entire public stream, including future reconnect attempts.
	done chan struct{}

	mu     sync.Mutex
	closed bool
	// wg tracks the supervisor only. Waiting for a watcher blocked inside CCXT
	// would make ClosePositionStream depend on CCXT successfully unblocking it.
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// positionWatchEvent is an internal watcher result. The supervisor decides
// whether to publish the update, retry the connection, or terminate the stream.
type positionWatchEvent struct {
	update *PositionUpdate
	err    error
}

func (stream *positionStream) isClosed() bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	return stream.closed
}

func (stream *positionStream) emit(update PositionUpdate) bool {
	// A slow consumer can block publication, but closing the stream releases
	// this send so shutdown does not require the consumer to keep reading.
	select {
	case <-stream.done:
		return false
	case stream.updates <- update:
		return true
	}
}

func (stream *positionStream) waitForRetry(delay time.Duration) bool {
	// Backoff is interruptible: explicit shutdown should not wait for the timer.
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-stream.done:
		return false
	case <-timer.C:
		return true
	}
}

func (stream *positionStream) stop() {
	stream.closeOnce.Do(func() {
		stream.mu.Lock()
		stream.closed = true
		stream.mu.Unlock()
		close(stream.done)
	})
}

// RunPositionStream starts one private WebSocket watcher for the requested
// futures symbols. The returned channel receives the initial provider snapshot
// when available and every later position update until ClosePositionStream is
// called or a terminal watcher error occurs. Passing no symbols watches all
// futures positions supported by the configured account.
// Transient failures are retried and reconnect automatically on the same output
// channel. Authentication and other permanent failures terminate the stream.
func (client *Client) RunPositionStream(symbols ...string) (<-chan PositionUpdate, error) {
	if err := client.validateConfigured(); err != nil {
		return nil, err
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

	// Serialize lifecycle changes so two callers cannot create two public
	// streams, or inspect a stream while the supervisor removes it.
	client.positionMu.Lock()
	defer client.positionMu.Unlock()

	if client.positionExchange == nil {
		return nil, apperrors.ErrUninitializedClient
	}
	if client.positionStream != nil {
		return nil, apperrors.ErrPositionStreamAlreadyExists
	}

	bufferSize := max(len(symbols)*2, 2)
	stream := &positionStream{
		updates: make(chan PositionUpdate, bufferSize),
		done:    make(chan struct{}),
	}
	client.positionStream = stream
	// Capture this instance for the supervisor and its watchers. The dedicated
	// exchange already contains credentials, demo settings, and loaded markets.
	positionExchange := client.positionExchange

	stream.wg.Add(1)
	go func() {
		defer stream.wg.Done()
		client.supervisePositionStream(stream, positionExchange, symbols)
	}()

	return stream.updates, nil
}

// ClosePositionStream stops the private position watcher without closing the
// main exchange client or the candle stream.
// WebSocket cleanup runs in the background; this method waits only for the
// supervisor to stop publishing and close the output channel.
func (client *Client) ClosePositionStream() error {
	if err := client.Validate(); err != nil {
		return err
	}
	client.positionMu.Lock()
	stream := client.positionStream
	client.positionMu.Unlock()
	if stream == nil {
		return apperrors.ErrPositionStreamNotRunning
	}

	// Signal shutdown first, then wait only for output publication to finish.
	// The supervisor starts CCXT cleanup separately and closes updates on exit.
	stream.stop()
	stream.wg.Wait()
	return nil
}

// supervisePositionStream owns the public update channel and replaces a failed
// watcher generation while retaining the configured CCXT exchange instance.
func (client *Client) supervisePositionStream(
	stream *positionStream,
	positionExchange ccxt.IExchange,
	symbols []string,
) {
	// Keeping channel closure here avoids send-on-closed-channel panics from
	// CCXT watchers that return after public shutdown has already completed.
	defer close(stream.updates)
	defer func() {
		client.positionMu.Lock()
		if client.positionStream == stream {
			client.positionStream = nil
		}
		client.positionMu.Unlock()
	}()

	waitingForRestartUpdate := false
	for {
		// A generation is one watcher attempt. Reconnect creates new internal
		// channels while the caller continues reading the same updates channel.
		events := make(chan positionWatchEvent, max(len(symbols)*2, 2))
		// This cancels local work and event forwarding. It cannot interrupt a
		// WatchPositions call already blocked inside CCXT; Close handles that.
		generationDone := make(chan struct{})
		go client.watchPositions(
			stream,
			positionExchange,
			symbols,
			generationDone,
			events,
		)

		// Process this watcher until shutdown or a reconnect is needed.
		watching := true
		for watching {
			select {
			case <-stream.done:
				// Public shutdown retires the watcher and starts socket cleanup.
				close(generationDone)
				closeExchangeAsync(positionExchange)
				return

			case event := <-events:
				if event.err == nil {
					if event.update != nil && waitingForRestartUpdate {
						log.Print("position stream: received first update after watcher restart")
						waitingForRestartUpdate = false
					}
					if event.update != nil && !stream.emit(*event.update) {
						close(generationDone)
						closeExchangeAsync(positionExchange)
						return
					}
					continue
				}

				// The watcher has reported failure and will return. Retire this
				// generation before deciding whether another attempt is allowed.
				close(generationDone)
				if !isRetryableWatchError(event.err) {
					log.Printf("position stream: terminal error: %v", normalizeError(event.err))
					closeExchangeAsync(positionExchange)
					stream.emit(PositionUpdate{
						Err: fmt.Errorf("watch positions: %w", normalizeError(event.err)),
					})
					return
				}

				// Close only the stale WebSocket generation. CCXT keeps credentials,
				// market metadata, and REST state on this reusable exchange instance.
				log.Printf("position stream: retries exhausted, restarting watcher after %s: %v", watchRetryBaseWait, event.err)
				closeExchangeAsync(positionExchange)
				// This is reconnect backoff, not confirmation that Close finished.
				// The current implementation can overlap cleanup and the next watch.
				if !stream.waitForRetry(watchRetryBaseWait) {
					return
				}
				// End this watcher. The outer loop starts its subscription again.
				waitingForRestartUpdate = true
				watching = false
			}
		}
	}
}

// watchPositions reads one position subscription generation. The captured
// exchange pointer remains stable even while Client lifecycle state changes.
func (client *Client) watchPositions(
	stream *positionStream,
	positionExchange ccxt.IExchange,
	symbols []string,
	generationDone <-chan struct{},
	events chan<- positionWatchEvent,
) {

	var options []ccxt.WatchPositionsOptions
	if len(symbols) > 0 {
		options = append(options, ccxt.WithWatchPositionsSymbols(symbols))
	}

	retryCount := 0
	for {
		select {
		case <-stream.done:
			return
		case <-generationDone:
			return
		default:
		}

		positions, err := positionExchange.WatchPositions(options...)
		if err != nil {
			if stream.isClosed() {
				return
			}

			if isRetryableWatchError(err) && retryCount < maxWatchRetries {
				// First retry the existing subscription with 1s, 2s, and 4s waits.
				// After these attempts fail, the supervisor handles reconnection.
				retryCount++
				delay := watchRetryBaseWait * time.Duration(1<<(retryCount-1))
				log.Printf("position stream: retry %d/%d, waiting %s: %v", retryCount, maxWatchRetries, delay, err)
				if !stream.waitForRetry(delay) {
					return
				}
				continue
			}

			client.sendPositionWatchEvent(generationDone, events, positionWatchEvent{err: err})
			return
		}
		// A successful response resets consecutive failures for this watcher.
		if retryCount > 0 {
			log.Printf("position stream: watch succeeded after %d retries", retryCount)
		}
		retryCount = 0

		for _, position := range positions {
			update, err := wrapPositionUpdate(position)
			if err != nil {
				client.sendPositionWatchEvent(generationDone, events, positionWatchEvent{err: err})
				return
			}
			if !client.sendPositionWatchEvent(
				generationDone,
				events,
				positionWatchEvent{update: &update},
			) {
				return
			}
		}
	}
}

// sendPositionWatchEvent lets cancellation release a blocked internal send.
// Each generation has its own events channel, so even a late send cannot reach
// the next generation's supervisor input.
func (client *Client) sendPositionWatchEvent(
	generationDone <-chan struct{},
	events chan<- positionWatchEvent,
	event positionWatchEvent,
) bool {
	select {
	case <-generationDone:
		return false
	case events <- event:
		return true
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
