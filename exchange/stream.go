package exchange

import (
	"errors"
	"sync"

	ccxt "github.com/ccxt/ccxt/go/v4"
	apperrors "github.com/yusufozmis/trading-library/errors"
	"github.com/yusufozmis/trading-library/types"
)

type StreamMode int64

const (
	AllUpdates StreamMode = iota
	ClosedOnly
)

type subKey struct {
	symbol    string
	timeframe string
}

type candleStream struct {
	symbols    []string
	timeframes []string

	stream chan types.Candle

	flag StreamMode

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

func (s *candleStream) clearIfCurrent(key subKey, token uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.activeSymbols[key]
	if ok && current == token {
		delete(s.activeSymbols, key)
	}
}

func (s *candleStream) emit(candle types.Candle) bool {
	select {
	case <-s.done:
		return false
	default:
	}

	select {
	case <-s.done:
		return false
	case s.stream <- candle:
		return true
	}
}

func (s *Client) RunCandleStream(symbols, timeframes []string, mode StreamMode) error {
	if err := s.validate(); err != nil {
		return err
	}

	if s.stream != nil {
		return apperrors.ErrStreamAlreadyExists
	}

	s.stream = &candleStream{
		symbols:       symbols,
		timeframes:    timeframes,
		stream:        make(chan types.Candle, len(symbols)*2),
		flag:          mode,
		activeSymbols: make(map[subKey]uint64),
		done:          make(chan struct{}),
	}

	for _, symbol := range s.stream.symbols {
		for _, timeframe := range s.stream.timeframes {
			if err := s.Subscribe(symbol, timeframe); err != nil {
				return err
			}
		}
	}
	return nil
}

// This stays receive-only so callers can consume updates without being able to
// send into the channel or close it from outside the package.
func (s *Client) Updates() (<-chan types.Candle, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}

	if s.stream == nil {
		return nil, apperrors.ErrStreamNotRunning
	}

	return s.stream.stream, nil
}

func (s *Client) Subscribe(symbol, timeframe string) error {
	if err := s.validate(); err != nil {
		return err
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
	if err := s.validate(); err != nil {
		return err
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
	if err := s.validate(); err != nil {
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

	for {
		if !s.stream.isCurrent(key, token) {
			return
		}

		candles, err := s.iExchange.WatchOHLCV(
			key.symbol,
			ccxt.WithWatchOHLCVTimeframe(key.timeframe),
		)
		if err != nil {
			s.stream.clearIfCurrent(key, token)
			return
		}

		if len(candles) == 0 {
			continue
		}

		if !s.stream.isCurrent(key, token) {
			return
		}

		current := candles[len(candles)-1]

		if s.stream.flag == ClosedOnly {
			if hasPrevious && current.Timestamp > previous.Timestamp {
				if !s.stream.isCurrent(key, token) {
					return
				}

				if !s.stream.emit(s.wrapOHLCV(key.symbol, key.timeframe, previous)) {
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

		if !s.stream.emit(s.wrapOHLCV(key.symbol, key.timeframe, current)) {
			return
		}

		previous = current
		hasPrevious = true
	}
}
