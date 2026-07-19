package websocket

import (
	"fmt"
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	ccxtpro "github.com/ccxt/ccxt/go/v4/pro"
)

type Exchange struct {
	exchange ccxtpro.IExchange
}

func NewBinance() *Exchange {
	return &Exchange{
		exchange: ccxtpro.NewBinance(nil),
	}
}

func NewOkx() *Exchange {
	return &Exchange{
		exchange: ccxtpro.NewOkx(nil),
	}
}

func (exchange *Exchange) validate() error {

	if exchange == nil {
		return fmt.Errorf("")
	}

	if exchange.exchange == nil {
		return fmt.Errorf("")
	}
	return nil
}

// FetchCandles returns up to limit fully formed candles for the given symbol and timeframe,
// conservatively dropping the newest fetched candle because it may still be in progress.
func (exchange *Exchange) FetchCandles(symbol, timeframe string, limit int64) ([]ccxt.OHLCV, error) {

	if err := exchange.validate(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		return nil, fmt.Errorf("")
	}

	if limit == math.MaxInt64 {
		return nil, fmt.Errorf("")
	}

	candles, err := exchange.exchange.FetchOHLCV(
		symbol,
		ccxt.WithFetchOHLCVTimeframe(timeframe),
		ccxt.WithFetchOHLCVLimit(limit+1),
	)
	if err != nil {
		return nil, err
	}

	if len(candles) == 0 {
		return nil, fmt.Errorf("")
	}

	// Never trust the newest candle.
	// It may still be forming, and without an exchange-specific confirm flag
	// we cannot know for sure.
	candles = candles[:len(candles)-1]

	if limit > 0 && int64(len(candles)) > limit {
		candles = candles[len(candles)-int(limit):]
	}

	return candles, nil

}
