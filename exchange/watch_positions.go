package exchange

import (
	"errors"
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

	// Err identifies a stopped subscription after retries are exhausted or a
	// permanent failure occurs. Symbol is empty for the all-positions subscription.
	// SubscribePositionStream can restart it on the same output channel.
	Err error
}

// Closed reports whether the exchange update states that no contracts remain.
func (update PositionUpdate) Closed() bool {
	return update.Err == nil && update.Contracts == 0
}

type positionStream struct {
	// Subscription supervisors publish here; ClosePositionStream closes it only
	// after they have stopped. Failed subscriptions do not close this channel.
	updates chan PositionUpdate
	// done stops the entire public stream, including every subscription.
	done chan struct{}
	// closeDone publishes closeErr after CCXT cleanup and all publishers finish.
	closeDone chan struct{}
	closeErr  error

	mu     sync.Mutex
	closed bool
	// Each cancellation channel also identifies its subscription's owner, so an
	// old watcher cannot publish into a replacement for the same symbol.
	subscriptions   map[string]chan struct{}
	excludedSymbols map[string]bool
	health          map[string]PositionSubscriptionHealth
	// wg tracks subscription supervisors only. Waiting for a watcher blocked inside CCXT
	// would make ClosePositionStream depend on CCXT successfully unblocking it.
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// positionWatchEvent is an internal watcher result. Its supervisor forwards
// updates or retires the failed subscription without closing the public stream.
type positionWatchEvent struct {
	update *PositionUpdate
	err    error
}

func (stream *positionStream) isCurrent(symbol string, subscriptionDone <-chan struct{}) bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	return !stream.closed && stream.subscriptions[symbol] == subscriptionDone
}

func (stream *positionStream) watches(symbol string, subscriptionDone <-chan struct{}, updateSymbol string) bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	return !stream.closed && stream.subscriptions[symbol] == subscriptionDone &&
		(symbol == updateSymbol || (symbol == "" && !stream.excludedSymbols[updateSymbol]))
}

// clearIfCurrent retains terminal health only for the subscription being removed.
func (stream *positionStream) clearIfCurrent(symbol string, subscriptionDone <-chan struct{}, err error) bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	if stream.closed || stream.subscriptions[symbol] != subscriptionDone {
		return false
	}
	health := stream.health[symbol]
	health.Status = StreamStopped
	health.LastError = normalizeError(err)
	stream.health[symbol] = health
	close(stream.subscriptions[symbol])
	delete(stream.subscriptions, symbol)
	return true
}

func (stream *positionStream) emit(update PositionUpdate, subscriptionDone <-chan struct{}) bool {
	// A slow consumer can block publication, but closing the stream releases
	// this send so shutdown does not require the consumer to keep reading.
	// Terminal errors use nil because their subscription has already been removed.
	select {
	case <-stream.done:
		return false
	case <-subscriptionDone:
		return false
	default:
	}
	select {
	case <-stream.done:
		return false
	case <-subscriptionDone:
		return false
	case stream.updates <- update:
		return true
	}
}

func (stream *positionStream) waitForRetry(delay time.Duration, subscriptionDone <-chan struct{}) bool {
	// Backoff is interruptible: explicit shutdown should not wait for the timer.
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-stream.done:
		return false
	case <-subscriptionDone:
		return false
	case <-timer.C:
		return true
	}
}

// RunPositionStream starts private WebSocket watchers for the requested
// futures symbols. The returned channel receives the initial provider snapshot
// when available and later updates from active subscriptions until
// ClosePositionStream is called. Passing no symbols watches all futures
// positions supported by the configured account.
// Transient failures are retried with 1s, 2s, and 4s delays. An exhausted or
// permanent failure removes that subscription and emits Err without closing the
// output channel. SubscribePositionStream can then restart the subscription.
func (client *Client) RunPositionStream(symbols ...string) (<-chan PositionUpdate, error) {
	if err := client.validatePositionStreamSymbols(symbols); err != nil {
		return nil, err
	}

	// Serialize lifecycle changes so creation, subscription, and close cannot
	// overlap a WaitGroup Add with a shutdown wait.
	client.positionMu.Lock()
	defer client.positionMu.Unlock()

	if client.positionExchange == nil {
		return nil, apperrors.ErrUninitializedClient
	}
	if client.positionStream != nil {
		return nil, apperrors.ErrPositionStreamAlreadyExists
	}

	stream := &positionStream{
		updates:         make(chan PositionUpdate, 100),
		done:            make(chan struct{}),
		closeDone:       make(chan struct{}),
		subscriptions:   make(map[string]chan struct{}),
		excludedSymbols: make(map[string]bool),
		health:          make(map[string]PositionSubscriptionHealth),
	}
	client.positionStream = stream
	client.subscribePositionStream(stream, symbols)

	return stream.updates, nil
}

// SubscribePositionStream adds or restarts position subscriptions on the existing
// output channel. Active subscriptions are left unchanged. Passing no symbols
// replaces individual subscriptions with one all-positions subscription and
// clears exclusions; while it is active, subscribing a symbol only unmutes it.
// After an Err update, pass its Symbol, or no arguments when Symbol is empty.
func (client *Client) SubscribePositionStream(symbols ...string) error {
	if err := client.validatePositionStreamSymbols(symbols); err != nil {
		return err
	}
	client.positionMu.Lock()
	defer client.positionMu.Unlock()
	if client.positionStream == nil {
		return apperrors.ErrPositionStreamNotRunning
	}
	stream := client.positionStream
	stream.mu.Lock()
	closed := stream.closed
	stream.mu.Unlock()
	if closed {
		return apperrors.ErrPositionStreamNotRunning
	}
	client.subscribePositionStream(stream, symbols)
	return nil
}

// subscribePositionStream runs with positionMu held, serializing subscription
// creation with ClosePositionStream and its WaitGroup wait.
func (client *Client) subscribePositionStream(stream *positionStream, symbols []string) {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(symbols) == 0 {
		clear(stream.excludedSymbols)
		if _, exists := stream.subscriptions[""]; exists {
			return
		}
		for symbol, done := range stream.subscriptions {
			close(done)
			delete(stream.subscriptions, symbol)
		}
		clear(stream.health)
		symbols = []string{""}
	}
	for _, symbol := range symbols {
		if _, all := stream.subscriptions[""]; all {
			delete(stream.excludedSymbols, symbol)
			continue
		}
		if _, exists := stream.subscriptions[symbol]; exists {
			continue
		}
		subscriptionDone := make(chan struct{})
		stream.subscriptions[symbol] = subscriptionDone
		stream.health[symbol] = PositionSubscriptionHealth{
			Symbol: symbol, Status: StreamStarting,
		}
		positionExchange := client.positionExchange
		stream.wg.Add(1)
		go func() {
			defer stream.wg.Done()
			client.supervisePositionStream(stream, positionExchange, symbol, subscriptionDone)
		}()
	}
}

// UnsubscribePositionStream stops forwarding the requested symbols without
// closing the output channel or other subscriptions. Passing no symbols pauses
// all subscriptions. Already-buffered or in-flight updates may still be received.
// CCXT does not implement position UnWatch for the supported providers: a call
// already waiting inside CCXT can finish later, but its result is discarded.
func (client *Client) UnsubscribePositionStream(symbols ...string) error {
	if err := client.validatePositionStreamSymbols(symbols); err != nil {
		return err
	}
	client.positionMu.Lock()
	defer client.positionMu.Unlock()
	stream := client.positionStream
	if stream == nil {
		return apperrors.ErrPositionStreamNotRunning
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(symbols) == 0 {
		for symbol, done := range stream.subscriptions {
			close(done)
			delete(stream.subscriptions, symbol)
		}
		clear(stream.excludedSymbols)
		clear(stream.health)
		return nil
	}
	for _, symbol := range symbols {
		if _, all := stream.subscriptions[""]; all {
			stream.excludedSymbols[symbol] = true
		}
		if done, exists := stream.subscriptions[symbol]; exists {
			close(done)
			delete(stream.subscriptions, symbol)
		}
		delete(stream.health, symbol)
	}
	return nil
}

func (client *Client) validatePositionStreamSymbols(symbols []string) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}
	for _, symbol := range symbols {
		if symbol == "" {
			return apperrors.ErrNilSymbol
		}
		market, exists := client.markets[symbol]
		if !exists || market.Swap == nil || !*market.Swap {
			return apperrors.ErrInvalidSymbol
		}
	}
	return nil
}

// ClosePositionStream starts private position stream shutdown and waits up to
// ten seconds for CCXT cleanup and the publishing supervisors to stop. The main
// client and other streams are unaffected. ErrStreamCloseTimeout does not cancel
// stuck CCXT calls: cleanup continues in the background, and the public channel
// closes only after its publishers stop. Repeated closes share the same cleanup.
// Until cleanup finishes, use an independent client or Clone for recovery;
// normal completion allows a new position stream on this client.
func (client *Client) ClosePositionStream() error {
	if err := client.Validate(); err != nil {
		return err
	}
	client.positionMu.Lock()
	stream := client.positionStream
	if stream == nil {
		client.positionMu.Unlock()
		return apperrors.ErrPositionStreamNotRunning
	}

	positionExchange := client.positionExchange
	stream.closeOnce.Do(func() {
		stream.mu.Lock()
		stream.closed = true
		for _, done := range stream.subscriptions {
			close(done)
		}
		clear(stream.subscriptions)
		clear(stream.excludedSymbols)
		clear(stream.health)
		stream.mu.Unlock()
		close(stream.done)

		go func() {
			var errs []error
			for _, err := range positionExchange.Close() {
				errs = append(errs, normalizeError(err))
			}
			// Only supervisors publish to updates; late CCXT results are discarded.
			stream.wg.Wait()
			close(stream.updates)
			client.positionMu.Lock()
			if client.positionStream == stream {
				client.positionStream = nil
			}
			client.positionMu.Unlock()
			stream.closeErr = errors.Join(errs...)
			close(stream.closeDone)
		}()
	})
	client.positionMu.Unlock()

	timer := time.NewTimer(streamCloseTimeout)
	defer timer.Stop()
	select {
	case <-stream.closeDone:
		return stream.closeErr
	case <-timer.C:
		return apperrors.ErrStreamCloseTimeout
	}
}

// supervisePositionStream forwards one subscription until it is removed or
// fails. Only the current owner can remove it and publish a terminal error.
func (client *Client) supervisePositionStream(
	stream *positionStream,
	positionExchange ccxt.IExchange,
	symbol string,
	subscriptionDone <-chan struct{},
) {
	if !stream.isCurrent(symbol, subscriptionDone) {
		return
	}
	var symbols []string
	if symbol != "" {
		symbols = []string{symbol}
	}
	events := make(chan positionWatchEvent, 2)
	go client.watchPositions(stream, positionExchange, symbols, subscriptionDone, events)
	for {
		select {
		case <-stream.done:
			return
		case <-subscriptionDone:
			return
		case event := <-events:
			if event.err != nil {
				if stream.clearIfCurrent(symbol, subscriptionDone, event.err) {
					log.Printf("position stream: subscription stopped (symbol=%q): %v", symbol, normalizeError(event.err))
					stream.emit(PositionUpdate{
						Symbol: symbol,
						Err:    fmt.Errorf("watch positions %q: %w", symbol, normalizeError(event.err)),
					}, nil)
				}
				return
			}
			if event.update != nil && stream.watches(symbol, subscriptionDone, event.update.Symbol) {
				if !stream.emit(*event.update, subscriptionDone) {
					return
				}
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
	symbol := ""
	var options []ccxt.WatchPositionsOptions
	if len(symbols) > 0 {
		symbol = symbols[0]
		options = append(options, ccxt.WithWatchPositionsSymbols(symbols))
	}

	retryCount := 0
	health := PositionSubscriptionHealth{Symbol: symbol, Status: StreamStarting}
	for {
		if !stream.isCurrent(symbol, generationDone) {
			return
		}

		positions, err := positionExchange.WatchPositions(options...)
		if !stream.isCurrent(symbol, generationDone) {
			return
		}
		if err != nil {
			health.LastError = normalizeError(err)
			if isRetryableWatchError(err) && retryCount < maxWatchRetries {
				// First retry the existing subscription with 1s, 2s, and 4s waits.
				// Exhaustion removes this subscription so the caller can restart it.
				retryCount++
				health.Status = StreamRetrying
				health.RetryCount = retryCount
				if !stream.recordHealth(symbol, generationDone, health) {
					return
				}
				delay := watchRetryBaseWait * time.Duration(1<<(retryCount-1))
				log.Printf("position stream: retry %d/%d, waiting %s (symbol=%q): %v", retryCount, maxWatchRetries, delay, symbol, err)
				if !stream.waitForRetry(delay, generationDone) {
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
		health.Status = StreamRunning
		health.RetryCount = 0
		if len(positions) > 0 {
			health.LastUpdate = time.Now()
		}
		if !stream.recordHealth(symbol, generationDone, health) {
			return
		}

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
