package exchange

import (
	"fmt"
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	ccxtpro "github.com/ccxt/ccxt/go/v4/pro"
	"github.com/yusufozmis/trading-library/types"
)

type Client struct {
	iExchange ccxtpro.IExchange
}

func NewClient(provider Provider) (*Client, error) {
	switch provider {
	case Binance:
		return &Client{iExchange: ccxtpro.NewBinance(nil)}, nil
	case Okx:
		return &Client{iExchange: ccxtpro.NewOkx(nil)}, nil
	default:
		return nil, fmt.Errorf("")
	}
}

func (exchange *Client) validate() error {

	if exchange == nil {
		return fmt.Errorf("")
	}

	if exchange.iExchange == nil {
		return fmt.Errorf("")
	}
	return nil
}

func (exchange *Client) wrapOHLCV(symbol, timeframe string, ohlcv ccxt.OHLCV) types.Candle {

	return types.Candle{
		Symbol:    symbol,
		Timeframe: timeframe,
		Timestamp: ohlcv.Timestamp,
		PriceData: types.Prices{
			OpenPrice:  ohlcv.Open,
			HighPrice:  ohlcv.High,
			LowPrice:   ohlcv.Low,
			ClosePrice: ohlcv.Close,
		},
		Volume: ohlcv.Volume,
	}
}

// FetchCandles returns up to limit fully formed candles for the given symbol and timeframe,
// conservatively dropping the newest fetched candle because it may still be in progress.
func (exchange *Client) FetchCandles(symbol, timeframe string, limit int64) ([]types.Candle, error) {

	if err := exchange.validate(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		return nil, fmt.Errorf("")
	}

	if limit == math.MaxInt64 {
		return nil, fmt.Errorf("")
	}

	candles, err := exchange.iExchange.FetchOHLCV(
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

	var wrapped []types.Candle

	for _, ohlcv := range candles {
		wrapped = append(wrapped, exchange.wrapOHLCV(symbol, timeframe, ohlcv))
	}

	return wrapped, nil
}
