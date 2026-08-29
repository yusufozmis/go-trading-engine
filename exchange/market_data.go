package exchange

import (
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// FetchCandles returns up to limit fully formed candles for the given symbol and timeframe,
// conservatively dropping the newest fetched candle because it may still be in progress.
func (client *Client) FetchCandles(symbol, timeframe string, limit int64) ([]types.Candle, error) {

	if err := client.Validate(); err != nil {
		return nil, err
	}

	if symbol == "" {
		return nil, apperrors.ErrNilSymbol
	}

	if _, exists := client.markets[symbol]; !exists {
		return nil, apperrors.ErrInvalidSymbol
	}

	if timeframe == "" {
		return nil, apperrors.ErrNilTimeframe
	}

	if limit <= 0 {
		return nil, apperrors.ErrInvalidLimit
	}

	if limit == math.MaxInt64 {
		return nil, apperrors.ErrLimitTooLarge
	}

	candles, err := client.iExchange.FetchOHLCV(
		symbol,
		ccxt.WithFetchOHLCVTimeframe(timeframe),
		ccxt.WithFetchOHLCVLimit(limit+1),
	)
	if err != nil {
		return nil, normalizeError(err)
	}

	if len(candles) == 0 {
		return nil, apperrors.ErrNoCandles
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
		return 0, apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return 0, apperrors.ErrInvalidSymbol
	}

	if marketInfo.Swap == nil || !*marketInfo.Swap {
		return 0, apperrors.ErrInvalidSymbol
	}

	fundingRate, err := client.iExchange.FetchFundingRate(symbol)
	if err != nil {
		return 0, normalizeError(err)
	}

	rate := fundingRate.FundingRate
	if rate == nil {
		return 0, apperrors.ErrFundingRateUnavailable
	}

	return (*rate) * 100, nil
}

// SupportedTimeframes returns the unified CCXT timeframe values supported by
// the active provider. The returned order is unspecified.
func (client *Client) SupportedTimeframes() []string {
	if client == nil || client.iExchange == nil {
		return nil
	}

	timeframes := client.iExchange.GetTimeframes()
	supported := make([]string, 0, len(timeframes))
	for timeframe := range timeframes {
		supported = append(supported, timeframe)
	}

	return supported
}
