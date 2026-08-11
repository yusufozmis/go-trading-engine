package exchange

import (
	"errors"
	"fmt"
	"sync"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	apperrors "github.com/yusufozmis/trading-library/errors"
	"github.com/yusufozmis/trading-library/types"
)

type StreamMode int64

const (
	AllUpdates StreamMode = iota
	ClosedOnly
)

const (
	// Retry transient watcher failures with 1s, 2s, and 4s waits.
	maxWatchRetries    = 3
	watchRetryBaseWait = time.Second
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

	closed   bool
	closeErr error
	mu       sync.Mutex

	wg        sync.WaitGroup
	closeOnce sync.Once
	// done exists only to break goroutines out of a blocked send on s.stream during CloseCandleStream().
	// UnWatchOHLCV may stop WatchOHLCV, but it does nothing for a goroutine already stuck on:
	//     s.stream <- candle
	done chan struct{}
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
func (s *Client) RunCandleStream(symbols, timeframes []string, mode StreamMode) (<-chan CandleUpdate, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	if s.stream != nil {
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
	}
	for _, timeframe := range timeframes {
		if timeframe == "" {
			return nil, apperrors.ErrNilTimeframe
		}
	}

	s.stream = &candleStream{
		symbols:       symbols,
		timeframes:    timeframes,
		stream:        make(chan CandleUpdate, len(symbols)*2),
		streamMode:    mode,
		activeSymbols: make(map[subKey]uint64),
		done:          make(chan struct{}),
	}

	for _, symbol := range s.stream.symbols {
		for _, timeframe := range s.stream.timeframes {
			if err := s.Subscribe(symbol, timeframe); err != nil {
				closeErr := s.CloseCandleStream()
				return nil, errors.Join(err, closeErr)
			}
		}
	}
	return s.stream.stream, nil
}

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

	if s.stream == nil {
		return apperrors.ErrStreamNotRunning
	}

	key := newSubKey(symbol, timeframe)

	s.stream.mu.Lock()
	if s.stream.closed {
		s.stream.mu.Unlock()
		return nil
	}
	if _, ok := s.stream.activeSymbols[key]; ok {
		s.stream.mu.Unlock()
		return nil
	}

	s.stream.nextToken++
	token := s.stream.nextToken
	s.stream.activeSymbols[key] = token
	s.stream.wg.Add(1)
	s.stream.mu.Unlock()

	go func() {
		defer s.stream.wg.Done()
		s.watch(key, token)
	}()

	return nil
}

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

	if s.stream == nil {
		return apperrors.ErrStreamNotRunning
	}

	key := newSubKey(symbol, timeframe)

	s.stream.mu.Lock()
	_, ok := s.stream.activeSymbols[key]
	if ok {
		delete(s.stream.activeSymbols, key)
	}
	s.stream.mu.Unlock()

	if !ok {
		return nil
	}

	_, err := s.iExchange.UnWatchOHLCV(
		symbol,
		ccxt.WithUnWatchOHLCVTimeframe(timeframe),
	)
	return err
}

// CloseCandleStream shuts down the running candle stream and detaches it from the client.
// Order matters:
// 1. mark the stream closed and clear active subscriptions
// 2. close done so blocked senders can stop
// 3. unwatch active subscriptions so WatchOHLCV should return
// 4. wait for watcher goroutines
// 5. only then close s.stream
//
// The channel must be closed last; closing it earlier risks "send on closed channel".
// Remaining limitation: if UnWatchOHLCV does not make pending WatchOHLCV calls return,
// Close can still hang in wg.Wait().
func (s *Client) CloseCandleStream() error {
	if err := s.Validate(); err != nil {
		return err
	}

	if s.stream == nil {
		return apperrors.ErrStreamNotRunning
	}

	stream := s.stream

	stream.closeOnce.Do(func() {
		stream.mu.Lock()
		stream.closed = true

		keys := make([]subKey, 0, len(stream.activeSymbols))
		for key := range stream.activeSymbols {
			keys = append(keys, key)
		}
		stream.mu.Unlock()

		close(stream.done)

		var errs []error
		for _, key := range keys {
			if err := s.Unsubscribe(key.symbol, key.timeframe); err != nil {
				errs = append(errs, err)
			}
		}

		stream.wg.Wait()
		close(stream.stream)
		s.stream = nil

		if len(errs) > 0 {
			stream.closeErr = errors.Join(errs...)
		}
	})

	return stream.closeErr
}

// watch belongs to one symbol/timeframe pair and one token only.
// The repeated isCurrent checks are deliberate: they stop an old watcher from
// publishing after unsubscribe+resubscribe created a newer owner for the same pair.
func (s *Client) watch(key subKey, token uint64) {
	var previous ccxt.OHLCV
	hasPrevious := false
	retryCount := 0

	for {
		if !s.stream.isCurrent(key, token) {
			return
		}

		candles, err := s.iExchange.WatchOHLCV(
			key.symbol,
			ccxt.WithWatchOHLCVTimeframe(key.timeframe),
		)
		if err != nil {
			// Unsubscribe can make WatchOHLCV return an error during normal shutdown.
			if !s.stream.isCurrent(key, token) {
				return
			}

			// Retry known temporary failures with a bounded exponential backoff.
			if isRetryableWatchError(err) && retryCount < maxWatchRetries {
				retryCount++
				delay := watchRetryBaseWait * time.Duration(1<<(retryCount-1))
				if !s.stream.waitForRetry(delay) {
					return
				}
				// retries
				continue
			}

			// Publish only terminal errors from the watcher that still owns the key.
			if s.stream.clearIfCurrent(key, token) {
				s.stream.emit(CandleUpdate{
					Err:       fmt.Errorf("watch OHLCV %q %q: %w", key.symbol, key.timeframe, err),
					Symbol:    key.symbol,
					Timeframe: key.timeframe,
				})
			}
			return
		}
		// Any successful response starts a fresh retry budget.
		retryCount = 0

		if len(candles) == 0 {
			continue
		}

		if !s.stream.isCurrent(key, token) {
			return
		}

		current := candles[len(candles)-1]

		if s.stream.streamMode == ClosedOnly {
			if hasPrevious && current.Timestamp > previous.Timestamp {
				if !s.stream.isCurrent(key, token) {
					return
				}

				// The previous candle is fully formed once a newer timestamp arrives.
				if !s.stream.emit(CandleUpdate{
					Candle: s.wrapOHLCV(key.symbol, key.timeframe, previous),
				}) {
					return
				}
			}

			previous = current
			hasPrevious = true
			continue
		}

		if !s.stream.isCurrent(key, token) {
			return
		}

		// AllUpdates publishes the latest candle even while it is still forming.
		if !s.stream.emit(CandleUpdate{
			Candle: s.wrapOHLCV(key.symbol, key.timeframe, current),
		}) {
			return
		}

		previous = current
		hasPrevious = true
	}
}
