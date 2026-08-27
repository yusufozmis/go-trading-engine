package exchange

import (
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// FetchCandles returns up to limit fully formed candles for the given symbol and timeframe,
// conservatively dropping the newest fetched candle because it may still be in progress.
func (client *Client) FetchCandles(symbol, timeframe string, limit int64) ([]types.Candle, error) {

	if err := client.Validate(); err != nil {
		return nil, err
	}

	if symbol == "" {
		return nil, errors.ErrNilSymbol
	}

	if _, exists := client.markets[symbol]; !exists {
		return nil, errors.ErrInvalidSymbol
	}

	if timeframe == "" {
		return nil, errors.ErrNilTimeframe
	}

	if limit <= 0 {
		return nil, errors.ErrInvalidLimit
	}

	if limit == math.MaxInt64 {
		return nil, errors.ErrLimitTooLarge
	}

	candles, err := client.iExchange.FetchOHLCV(
		symbol,
		ccxt.WithFetchOHLCVTimeframe(timeframe),
		ccxt.WithFetchOHLCVLimit(limit+1),
	)
	if err != nil {
		return nil, err
	}

	if len(candles) == 0 {
		return nil, errors.ErrNoCandles
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
		wrapped = append(wrapped, client.wrapOHLCV(symbol, timeframe, ohlcv))
	}

	return wrapped, nil
}

func (client *Client) wrapOHLCV(symbol, timeframe string, ohlcv ccxt.OHLCV) types.Candle {
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

// FetchFundingRate returns the current funding rate as a percentage.
// The symbol must use a futures format such as BTC/USDT:USDT.
func (client *Client) FetchFundingRate(symbol string) (float64, error) {

	if err := client.Validate(); err != nil {
		return 0, err
	}

	if symbol == "" {
		return 0, errors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return 0, errors.ErrInvalidSymbol
	}

	if marketInfo.Swap == nil || !*marketInfo.Swap {
		return 0, errors.ErrInvalidSymbol
	}

	fundingRate, err := client.iExchange.FetchFundingRate(symbol)
	if err != nil {
		return 0, err
	}

	rate := fundingRate.FundingRate
	if rate == nil {
		return 0, errors.ErrFundingRateUnavailable
	}

	return (*rate) * 100, nil
}
