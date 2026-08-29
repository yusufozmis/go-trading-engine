// Package backtester runs strategies against immutable historical candle sets.
package backtester

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/engine/options"
	"github.com/yusufozmis/go-trading-engine/types"
)

// Backtester owns an immutable snapshot of historical candles for one market.
type Backtester struct {
	symbol        string
	timeframe     string
	positionSizer types.PositionSizer
	options       []options.Option
	candles       []types.Candle
}

func validateCandles(symbol, timeframe string, candles []types.Candle) error {
	if len(candles) == 0 {
		return apperrors.ErrEmptyCandleSet
	}
	if symbol == "" {
		return apperrors.ErrNilSymbol
	}
	if timeframe == "" {
		return apperrors.ErrNilTimeframe
	}

	for i, candle := range candles {
		if candle.Symbol != symbol || candle.Timeframe != timeframe {
			return apperrors.ErrInvalidSetOfCandles
		}

		if i > 0 && candle.Timestamp <= candles[i-1].Timestamp {
			return apperrors.ErrInvalidSetOfCandles
		}

		low := candle.PriceData.LowPrice
		high := candle.PriceData.HighPrice
		open := candle.PriceData.OpenPrice
		close := candle.PriceData.ClosePrice
		volume := candle.Volume

		if math.IsNaN(low) || math.IsInf(low, 0) || low <= 0 {
			return apperrors.ErrInvalidPrice
		}
		if math.IsNaN(high) || math.IsInf(high, 0) || high <= 0 {
			return apperrors.ErrInvalidPrice
		}
		if math.IsNaN(open) || math.IsInf(open, 0) || open <= 0 {
			return apperrors.ErrInvalidPrice
		}
		if math.IsNaN(close) || math.IsInf(close, 0) || close <= 0 {
			return apperrors.ErrInvalidPrice
		}
		if math.IsNaN(volume) || math.IsInf(volume, 0) || volume < 0 {
			return apperrors.ErrInvalidVolume
		}

		if low > high ||
			open < low || open > high ||
			close < low || close > high {
			return apperrors.ErrInvalidSetOfCandles
		}

	}
	return nil
}

// NewBacktester validates and snapshots candles for repeated backtest runs.
func NewBacktester(symbol, timeframe string,
	candles []types.Candle,
	positionSizer types.PositionSizer,
	opts ...options.Option,
) (*Backtester, error) {
	candlesCopy := append([]types.Candle(nil), candles...)

	if err := validateCandles(symbol, timeframe, candlesCopy); err != nil {
		return nil, err
	}

	if positionSizer == nil {
		return nil, apperrors.ErrNilPositionSizer
	}

	return &Backtester{
		symbol:        symbol,
		timeframe:     timeframe,
		candles:       candlesCopy,
		positionSizer: positionSizer,
		options:       append([]options.Option(nil), opts...),
	}, nil
}
