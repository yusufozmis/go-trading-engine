package exchange

import (
	"errors"
	"sync"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/trading-library/types"
)

type StreamMode int64
type Provider int64

const (
	AllUpdates StreamMode = iota
	ClosedOnly
)

const (
	Binance Provider = iota
	Okx
)

type subKey struct {
	symbol    string
	timeframe string
}

type CandleStream struct {
	symbols    []string
	timeframes []string

	exchange *Client
	stream   chan types.Candle

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

func NewCandleStream(provider Provider, symbols []string,
	timeframes []string, flag StreamMode) (*CandleStream, error) {

	exchange, err := NewClient(provider)
	if err != nil {
		return nil, err
	}

	if err := exchange.validate(); err != nil {
		return nil, err
	}

	return &CandleStream{
		symbols:       symbols,
		timeframes:    timeframes,
		exchange:      exchange,
		stream:        make(chan types.Candle, len(symbols)*2),
		flag:          flag,
		activeSymbols: make(map[subKey]uint64),
		done:          make(chan struct{}),
	}, nil
}

func newSubKey(symbol, timeframe string) subKey {
	return subKey{
		symbol:    symbol,
		timeframe: timeframe,
	}
}

func (s *CandleStream) isCurrent(key subKey, token uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.activeSymbols[key]
	return ok && current == token
}

func (s *CandleStream) clearIfCurrent(key subKey, token uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.activeSymbols[key]
	if ok && current == token {
		delete(s.activeSymbols, key)
	}
}

func (s *CandleStream) emit(candle types.Candle) bool {
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

func (s *CandleStream) Run() {
	for _, symbol := range s.symbols {
		for _, timeframe := range s.timeframes {
			s.Subscribe(symbol, timeframe)
		}
	}
}

// This stays receive-only so callers can consume updates without being able to
// send into the channel or close it from outside the package.
func (s *CandleStream) Updates() <-chan types.Candle {
	return s.stream
}

func (s *CandleStream) Subscribe(symbol, timeframe string) {
	key := newSubKey(symbol, timeframe)

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	if _, ok := s.activeSymbols[key]; ok {
		s.mu.Unlock()
		return
	}

	s.nextToken++
	token := s.nextToken
	s.activeSymbols[key] = token
	s.wg.Add(1)
	s.mu.Unlock()

	go func() {
		defer s.wg.Done()
		s.watch(key, token)
	}()
}

func (s *CandleStream) Unsubscribe(symbol, timeframe string) error {
	key := newSubKey(symbol, timeframe)

	s.mu.Lock()
	_, ok := s.activeSymbols[key]
	if ok {
		delete(s.activeSymbols, key)
	}
	s.mu.Unlock()

	if !ok {
		return nil
	}

	_, err := s.exchange.iExchange.UnWatchOHLCV(
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
func (s *CandleStream) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.activeSymbols = make(map[subKey]uint64)
		s.mu.Unlock()

		close(s.done)

		if errs := s.exchange.iExchange.Close(); len(errs) > 0 {
			s.closeErr = errors.Join(errs...)
		}

		s.wg.Wait()
		close(s.stream)
	})

	return s.closeErr
}

// watch belongs to one symbol/timeframe pair and one token only.
// The repeated isCurrent checks are deliberate: they stop an old watcher from
// publishing after unsubscribe+resubscribe created a newer owner for the same pair.
func (s *CandleStream) watch(key subKey, token uint64) {
	var previous ccxt.OHLCV
	hasPrevious := false

	for {
		if !s.isCurrent(key, token) {
			return
		}

		candles, err := s.exchange.iExchange.WatchOHLCV(
			key.symbol,
			ccxt.WithWatchOHLCVTimeframe(key.timeframe),
		)
		if err != nil {
			s.clearIfCurrent(key, token)
			return
		}

		if len(candles) == 0 {
			continue
		}

		if !s.isCurrent(key, token) {
			return
		}

		current := candles[len(candles)-1]

		if s.flag == ClosedOnly {
			if hasPrevious && current.Timestamp > previous.Timestamp {
				if !s.isCurrent(key, token) {
					return
				}

				if !s.emit(s.exchange.wrapOHLCV(key.symbol, key.timeframe, previous)) {
					return
				}
			}

			previous = current
			hasPrevious = true
			continue
		}

		if !s.isCurrent(key, token) {
			return
		}

		if !s.emit(s.exchange.wrapOHLCV(key.symbol, key.timeframe, current)) {
			return
		}

		previous = current
		hasPrevious = true
	}
}
