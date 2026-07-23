package exchange

import (
	"errors"
	"sync"

	ccxt "github.com/ccxt/ccxt/go/v4"
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
	// done exists only to break goroutines out of a blocked send on s.stream during Close().
	// exchange.Close() may stop WatchOHLCV, but it does nothing for a goroutine already stuck on:
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

func (s *Client) RunCandleStream(symbols, timeframes []string, mode StreamMode) {
	if s == nil || s.stream != nil {
		return
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
			s.Subscribe(symbol, timeframe)
		}
	}
}

// This stays receive-only so callers can consume updates without being able to
// send into the channel or close it from outside the package.
func (s *Client) Updates() <-chan types.Candle {
	return s.stream.stream
}

func (s *Client) Subscribe(symbol, timeframe string) {
	key := newSubKey(symbol, timeframe)

	s.stream.mu.Lock()
	if s.stream.closed {
		s.stream.mu.Unlock()
		return
	}
	if _, ok := s.stream.activeSymbols[key]; ok {
		s.stream.mu.Unlock()
		return
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
}

func (s *Client) Unsubscribe(symbol, timeframe string) error {
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

// Close order matters:
// 1. mark the stream closed and drop local ownership
// 2. close done so blocked senders can stop
// 3. close the exchange so WatchOHLCV should return
// 4. wait for watcher goroutines
// 5. only then close s.stream
//
// The channel must be closed last; closing it earlier risks "send on closed channel".
// Remaining limitation: if exchange.Close() does not make pending WatchOHLCV calls return,
// Close can still hang in wg.Wait().
func (s *Client) Close() error {
	s.stream.closeOnce.Do(func() {
		s.stream.mu.Lock()
		s.stream.closed = true
		s.stream.activeSymbols = make(map[subKey]uint64)
		s.stream.mu.Unlock()

		close(s.stream.done)

		if errs := s.iExchange.Close(); len(errs) > 0 {
			s.stream.closeErr = errors.Join(errs...)
		}

		s.stream.wg.Wait()
		close(s.stream.stream)
	})

	return s.stream.closeErr
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
