package exchange

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// StreamMode controls whether a candle stream emits forming candles or only
// candles confirmed closed by the arrival of a newer timestamp.
type StreamMode int64

const (
	// AllUpdates emits the latest candle on every watcher update.
	AllUpdates StreamMode = iota
	// ClosedOnly emits a candle only after a newer candle timestamp arrives.
	ClosedOnly
)

// closeExchangeAsync prevents a slow CCXT WebSocket shutdown from blocking
// the public stream lifecycle. Close leaves credentials and markets intact, so
// the dedicated instance can establish a fresh WebSocket generation afterward.
func closeExchangeAsync(exchange ccxt.IExchange) {
	// CCXT Close clears its WebSocket client registry and closes those sockets.
	// It does not permanently disable REST or discard configuration by default.
	// This helper deliberately does not wait or report cleanup errors. It also
	// does not guarantee that cleanup completes before the next watch starts.
	go func() {
		_ = exchange.Close()
	}()
}

const (
	// Retry transient watcher failures with 1s, 2s, and 4s waits.
	maxWatchRetries    = 3
	watchRetryBaseWait = time.Second
	// Bound the caller's wait without interrupting background cleanup.
	streamCloseTimeout = 10 * time.Second
)

// CandleUpdate carries either a candle or an asynchronous watcher error.
// When Err is non-nil, Candle is the zero value and that watcher has stopped.
type CandleUpdate struct {
	// Candle contains data when Err is nil.
	Candle types.Candle
	// Err contains a terminal watcher failure after any retries are exhausted.
	Err error

	// Symbol and Timeframe identify the stopped subscription when Err is non-nil.
	Symbol    string
	Timeframe string
}

// Valid reports whether the stream mode is supported.
func (s StreamMode) Valid() bool {
	switch s {
	case AllUpdates, ClosedOnly:
		return true
	default:
		return false
	}
}

type subKey struct {
	symbol    string
	timeframe string
}

type candleStream struct {
	symbols    []string
	timeframes []string

	stream chan CandleUpdate

	streamMode StreamMode

	// This is tokenized on purpose.
	// A plain bool could only answer "is BTC/USDT 5m active?" and that breaks when:
	// 1. old watcher A is unsubscribed
	// 2. the same pair is subscribed again as watcher B
	// 3. watcher A resumes later and sees the pair as active again
	// With tokens, watcher A can prove "that active entry is not mine anymore" and exit.
	activeSymbols map[subKey]uint64
	nextToken     uint64
	health        map[subKey]CandleSubscriptionHealth

	closed   bool
	closeErr error
	mu       sync.Mutex

	wg        sync.WaitGroup
	closeOnce sync.Once
	// done exists only to break goroutines out of a blocked send on s.stream during CloseCandleStream().
	// UnWatchOHLCV may stop WatchOHLCV, but it does nothing for a goroutine already stuck on:
	//     s.stream <- candle
	done chan struct{}
	// closeDone publishes closeErr after exchange cleanup and all watchers finish.
	closeDone chan struct{}
}

func newSubKey(symbol, timeframe string) subKey {
	return subKey{
		symbol:    symbol,
		timeframe: timeframe,
	}
}

func (s *candleStream) isCurrent(key subKey, token uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.activeSymbols[key]
	return ok && current == token
}

// clearIfCurrent removes only the watcher that still owns this subscription.
// False means it was already unsubscribed or replaced, so no error should be emitted.
func (s *candleStream) clearIfCurrent(key subKey, token uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.activeSymbols[key]
	if ok && current == token {
		delete(s.activeSymbols, key)
		return true
	}

	return false
}

// emit sends both candle data and terminal watcher errors through the same channel.
// The done case prevents a blocked send from delaying stream shutdown.
func (s *candleStream) emit(update CandleUpdate) bool {
	select {
	case <-s.done:
		return false
	default:
	}

	select {
	case <-s.done:
		return false
	case s.stream <- update:
		return true
	}
}

// waitForRetry applies backoff while allowing CloseCandleStream to interrupt the wait.
func (s *candleStream) waitForRetry(delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-s.done:
		return false
	case <-timer.C:
		return true
	}
}

// isRetryableWatchError limits retries to CCXT failures that can recover without
// changing the subscription or credentials.
func isRetryableWatchError(err error) bool {
	var exchangeErr *ccxt.Error
	if !errors.As(err, &exchangeErr) {
		return false
	}

	// CCXT sometimes wraps a failed DNS dial in a generic Exception instead of
	// NetworkError. A lookup timeout is still a transient connection failure.
	if exchangeErr.Type == ccxt.ErrorType("Exception") &&
		strings.Contains(exchangeErr.Message, "dial tcp: lookup ") &&
		strings.Contains(exchangeErr.Message, "i/o timeout") {
		return true
	}

	switch exchangeErr.Type {
	case ccxt.NetworkErrorErrType,
		ccxt.DDoSProtectionErrType,
		ccxt.RateLimitExceededErrType,
		ccxt.ExchangeNotAvailableErrType,
		ccxt.OnMaintenanceErrType,
		ccxt.ChecksumErrorErrType,
		ccxt.RequestTimeoutErrType,
		ccxt.BadResponseErrType,
		ccxt.NullResponseErrType:
		return true
	default:
		return false
	}
}

// RunCandleStream starts watchers for every symbol and timeframe combination.
// The returned channel carries candle updates and terminal watcher errors until CloseCandleStream closes it.
// RunCandleStream, Subscribe, Unsubscribe, and CloseCandleStream must not be called concurrently.
// Other Client methods may be called while the candle stream is running.
func (s *Client) RunCandleStream(symbols, timeframes []string, mode StreamMode) (<-chan CandleUpdate, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	s.candleMu.Lock()
	hasStream := s.stream != nil
	s.candleMu.Unlock()
	if hasStream {
		return nil, apperrors.ErrStreamAlreadyExists
	}

	if !mode.Valid() {
		return nil, apperrors.ErrInvalidStreamMode
	}

	if len(symbols) == 0 {
		return nil, apperrors.ErrEmptySymbols
	}
	if len(timeframes) == 0 {
		return nil, apperrors.ErrEmptyTimeframes
	}

	for _, symbol := range symbols {
		if symbol == "" {
			return nil, apperrors.ErrNilSymbol
		}

		if _, exists := s.markets[symbol]; !exists {
			return nil, apperrors.ErrInvalidSymbol
		}
	}

	for _, timeframe := range timeframes {
		if timeframe == "" {
			return nil, apperrors.ErrNilTimeframe
		}
	}

	s.candleMu.Lock()
	s.stream = &candleStream{
		symbols:       symbols,
		timeframes:    timeframes,
		stream:        make(chan CandleUpdate, 100),
		streamMode:    mode,
		activeSymbols: make(map[subKey]uint64),
		health:        make(map[subKey]CandleSubscriptionHealth),
		done:          make(chan struct{}),
		closeDone:     make(chan struct{}),
	}
	s.candleMu.Unlock()

	for _, symbol := range s.stream.symbols {
		for _, timeframe := range s.stream.timeframes {
			if err := s.Subscribe(symbol, timeframe); err != nil {
				// Roll back previously started watchers so startup is all-or-nothing.
				closeErr := s.CloseCandleStream()
				return nil, errors.Join(err, closeErr)
			}
		}
	}
	return s.stream.stream, nil
}

// Subscribe adds a symbol and timeframe watcher to the running candle stream.
// It is a no-op when the same subscription is already active.
func (s *Client) Subscribe(symbol, timeframe string) error {
	if err := s.Validate(); err != nil {
		return err
	}

	if symbol == "" {
		return apperrors.ErrNilSymbol
	}

	if timeframe == "" {
		return apperrors.ErrNilTimeframe
	}

	if _, exists := s.markets[symbol]; !exists {
		return apperrors.ErrInvalidSymbol
	}

	s.candleMu.Lock()
	stream := s.stream
	s.candleMu.Unlock()
	if stream == nil {
		return apperrors.ErrStreamNotRunning
	}

	key := newSubKey(symbol, timeframe)

	stream.mu.Lock()
	if stream.closed {
		stream.mu.Unlock()
		return apperrors.ErrStreamNotRunning
	}
	if _, ok := stream.activeSymbols[key]; ok {
		stream.mu.Unlock()
		return nil
	}

	stream.nextToken++
	token := stream.nextToken
	stream.activeSymbols[key] = token
	stream.health[key] = CandleSubscriptionHealth{
		Symbol: symbol, Timeframe: timeframe, Status: StreamStarting,
	}
	stream.wg.Add(1)
	stream.mu.Unlock()

	go func() {
		defer stream.wg.Done()
		s.watch(key, token)
	}()

	return nil
}

// Unsubscribe stops a symbol and timeframe watcher and removes its health entry.
// An already-stopped watcher needs no provider request; its health entry is removed too.
// Provider-specific CCXT cleanup runs only after a successful unsubscribe, so a
// later Subscribe can register a fresh watch without restarting other pairs.
func (s *Client) Unsubscribe(symbol, timeframe string) error {
	if err := s.Validate(); err != nil {
		return err
	}

	if symbol == "" {
		return apperrors.ErrNilSymbol
	}

	if timeframe == "" {
		return apperrors.ErrNilTimeframe
	}

	s.candleMu.Lock()
	stream := s.stream
	s.candleMu.Unlock()
	if stream == nil {
		return apperrors.ErrStreamNotRunning
	}

	key := newSubKey(symbol, timeframe)

	stream.mu.Lock()
	defer stream.mu.Unlock()

	if _, ok := stream.activeSymbols[key]; !ok {
		delete(stream.health, key)
		return nil
	}

	// Keep the subscription active until CCXT confirms that its watcher was
	// removed. If UnWatchOHLCV fails, the caller can retry without losing the
	// library's record of the still-running subscription.
	_, err := s.streamExchange.UnWatchOHLCV(
		symbol,
		ccxt.WithUnWatchOHLCVTimeframe(timeframe),
	)
	if err != nil {
		return normalizeError(err)
	}

	delete(stream.activeSymbols, key)
	delete(stream.health, key)
	if s.candleStreamAdapter != nil && s.streamBaseExchange != nil {
		interval, ok := s.streamExchange.GetTimeframes()[timeframe].(string)
		if !ok {
			interval = timeframe
		}
		subscriptionKey := s.candleStreamAdapter.SubscriptionKey(s.markets[symbol], timeframe, interval)
		// The local owner is already removed while stream.mu is held. Rejecting
		// its pending CCXT future cannot report a failure against a replacement.
		s.candleStreamAdapter.CleanupUnsubscription(s.streamBaseExchange, subscriptionKey)
	}
	return nil
}

// CloseCandleStream starts candle stream shutdown and waits up to ten seconds
// for cleanup. ErrStreamCloseTimeout bounds the caller's wait; it does not
// cancel a stuck CCXT call. Cleanup continues and closes the public channel only
// after all watchers exit. REST, order, and position operations are unaffected.
// The same client cannot start a new candle stream until cleanup finishes;
// after a timeout, use an independent client or Clone for immediate recovery.
// Repeated closes while cleanup is pending wait for the same cleanup operation.
// Normal completion detaches the stream and permits reopening on this client.
func (s *Client) CloseCandleStream() error {
	if err := s.Validate(); err != nil {
		return err
	}

	s.candleMu.Lock()
	stream := s.stream
	s.candleMu.Unlock()
	if stream == nil {
		return apperrors.ErrStreamNotRunning
	}

	stream.closeOnce.Do(func() {
		stream.mu.Lock()
		stream.closed = true
		clear(stream.activeSymbols)
		clear(stream.health)
		stream.mu.Unlock()

		close(stream.done)

		go func() {
			var errs []error
			for _, err := range s.streamExchange.Close() {
				errs = append(errs, normalizeError(err))
			}

			// A timed-out watcher may still send; never close the channel early.
			stream.wg.Wait()
			close(stream.stream)
			s.candleMu.Lock()
			if s.stream == stream {
				s.stream = nil
			}
			s.candleMu.Unlock()

			stream.closeErr = errors.Join(errs...)
			close(stream.closeDone)
		}()
	})

	timer := time.NewTimer(streamCloseTimeout)
	defer timer.Stop()
	select {
	case <-stream.closeDone:
		return stream.closeErr
	case <-timer.C:
		return apperrors.ErrStreamCloseTimeout
	}
}

// watch belongs to one symbol/timeframe pair and one token only.
// The repeated isCurrent checks are deliberate: they stop an old watcher from
// publishing after unsubscribe+resubscribe created a newer owner for the same pair.
func (s *Client) watch(key subKey, token uint64) {
	stream := s.stream
	health := CandleSubscriptionHealth{
		Symbol: key.symbol, Timeframe: key.timeframe, Status: StreamStarting,
	}
	var previous ccxt.OHLCV
	hasPrevious := false
	retryCount := 0

	for {
		if !stream.isCurrent(key, token) {
			return
		}

		candles, err := s.streamExchange.WatchOHLCV(
			key.symbol,
			ccxt.WithWatchOHLCVTimeframe(key.timeframe),
		)
		if err != nil {
			// Unsubscribe can make WatchOHLCV return an error during normal shutdown.
			if !stream.isCurrent(key, token) {
				return
			}
			health.LastError = normalizeError(err)

			// Retry known temporary failures with a bounded exponential backoff.
			if isRetryableWatchError(err) && retryCount < maxWatchRetries {
				retryCount++
				health.Status = StreamRetrying
				health.RetryCount = retryCount
				if !stream.recordHealth(key, token, health) {
					return
				}
				delay := watchRetryBaseWait * time.Duration(1<<(retryCount-1))
				if !stream.waitForRetry(delay) {
					return
				}
				// retries
				continue
			}

			// Publish only terminal errors from the watcher that still owns the key.
			health.Status = StreamStopped
			if stream.recordHealth(key, token, health) && stream.clearIfCurrent(key, token) {
				stream.emit(CandleUpdate{
					Err: fmt.Errorf(
						"watch OHLCV %q %q: %w",
						key.symbol,
						key.timeframe,
						normalizeError(err),
					),
					Symbol:    key.symbol,
					Timeframe: key.timeframe,
				})
			}
			return
		}
		// Any successful response starts a fresh retry budget.
		retryCount = 0
		health.Status = StreamRunning
		health.RetryCount = 0
		if len(candles) > 0 {
			health.LastUpdate = time.Now()
		}
		if !stream.recordHealth(key, token, health) {
			return
		}

		if len(candles) == 0 {
			continue
		}

		if !stream.isCurrent(key, token) {
			return
		}

		current := candles[len(candles)-1]

		if stream.streamMode == ClosedOnly {
			if hasPrevious && current.Timestamp > previous.Timestamp {
				if !stream.isCurrent(key, token) {
					return
				}

				// The previous candle is fully formed once a newer timestamp arrives.
				if !stream.emit(CandleUpdate{
					Candle: s.wrapOHLCV(key.symbol, key.timeframe, previous),
				}) {
					return
				}
			}

			previous = current
			hasPrevious = true
			continue
		}

		if !stream.isCurrent(key, token) {
			return
		}

		// AllUpdates publishes the latest candle even while it is still forming.
		if !stream.emit(CandleUpdate{
			Candle: s.wrapOHLCV(key.symbol, key.timeframe, current),
		}) {
			return
		}

		previous = current
		hasPrevious = true
	}
}
