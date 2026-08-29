package exchange

import (
	"math"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

const (
	binanceOHLCVPageSize int64 = 1000
	okxOHLCVPageSize     int64 = 200
)

// FetchCandles returns up to limit fully formed candles for the given symbol and
// timeframe. Requests larger than one provider page are paginated automatically.
// The newest fetched candle is conservatively dropped because it may still be
// in progress.
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
	if _, supported := client.iExchange.GetTimeframes()[timeframe]; !supported {
		return nil, apperrors.ErrInvalidTimeframe
	}

	if limit <= 0 {
		return nil, apperrors.ErrInvalidLimit
	}

	if limit > math.MaxInt64-2 {
		return nil, apperrors.ErrLimitTooLarge
	}

	requestedLimit := limit + 1
	candles, err := client.fetchOHLCVPages(symbol, timeframe, requestedLimit)
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
	if len(candles) == 0 {
		return nil, apperrors.ErrNoCandles
	}

	if limit > 0 && int64(len(candles)) > limit {
		candles = candles[len(candles)-int(limit):]
	}

	return candles, nil
}

// fetchOHLCVPages walks forward through provider pages because CCXT Go's
// built-in deterministic paginator cannot dispatch FetchOHLCV reliably.
func (client *Client) fetchOHLCVPages(
	symbol string,
	timeframe string,
	requestedLimit int64,
) ([]types.Candle, error) {
	timeframeMilliseconds := ccxt.ParseTimeframe(timeframe) * 1000
	if timeframeMilliseconds <= 0 {
		return nil, apperrors.ErrInvalidTimeframe
	}

	// Start one interval earlier than required, then retain the newest candles.
	// This avoids losing the current candle when the request lands exactly on a
	// timeframe boundary.
	fetchLimit := requestedLimit + 1
	if fetchLimit > math.MaxInt64/timeframeMilliseconds {
		return nil, apperrors.ErrLimitTooLarge
	}
	lookback := fetchLimit * timeframeMilliseconds
	now := time.Now().UnixMilli()
	since := max(now-lookback, 0)

	maxCalls := fetchLimit / client.ohlcvPageSize
	if fetchLimit%client.ohlcvPageSize != 0 {
		maxCalls++
	}
	maxCalls++ // Allow one final partial page at the current timestamp.

	var candles []types.Candle
	for range maxCalls {
		page, err := client.iExchange.FetchOHLCV(
			symbol,
			ccxt.WithFetchOHLCVTimeframe(timeframe),
			ccxt.WithFetchOHLCVSince(since),
			ccxt.WithFetchOHLCVLimit(client.ohlcvPageSize),
		)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		for _, candle := range page {
			if candle.Timestamp <= 0 {
				return nil, apperrors.ErrInvalidTimestamp
			}

			if len(candles) == 0 {
				candles = append(candles, client.wrapOHLCV(symbol, timeframe, candle))
				continue
			}

			lastTimestamp := candles[len(candles)-1].Timestamp
			switch {
			case candle.Timestamp == lastTimestamp:
				continue
			case candle.Timestamp < lastTimestamp:
				return nil, apperrors.ErrInvalidSetOfCandles
			default:
				candles = append(candles, client.wrapOHLCV(symbol, timeframe, candle))
			}
		}

		lastTimestamp := page[len(page)-1].Timestamp
		nextSince := lastTimestamp + timeframeMilliseconds
		if nextSince <= since || nextSince > now {
			break
		}
		since = nextSince
	}

	if int64(len(candles)) > requestedLimit {
		candles = candles[len(candles)-int(requestedLimit):]
	}

	return candles, nil
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
