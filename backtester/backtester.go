package backtester

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/engine"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

type Backtester struct {
	engine   *engine.Engine
	candles  []types.Candle
	strategy types.Strategy
}

func Validate(symbol, timeframe string, candles []types.Candle) error {
	if len(candles) == 0 {
		return errors.ErrEmptyCandleSet
	}
	if symbol == "" {
		return errors.ErrNilSymbol
	}
	if timeframe == "" {
		return errors.ErrNilTimeframe
	}

	for i, candle := range candles {
		if candle.Symbol != symbol || candle.Timeframe != timeframe {
			return errors.ErrInvalidSetOfCandles
		}

		if i > 0 && candle.Timestamp <= candles[i-1].Timestamp {
			return errors.ErrInvalidSetOfCandles
		}

		low := candle.PriceData.LowPrice
		high := candle.PriceData.HighPrice
		open := candle.PriceData.OpenPrice
		close := candle.PriceData.ClosePrice
		volume := candle.Volume

		if math.IsNaN(low) || math.IsInf(low, 0) || low <= 0 {
			return errors.ErrInvalidPrice
		}
		if math.IsNaN(high) || math.IsInf(high, 0) || high <= 0 {
			return errors.ErrInvalidPrice
		}
		if math.IsNaN(open) || math.IsInf(open, 0) || open <= 0 {
			return errors.ErrInvalidPrice
		}
		if math.IsNaN(close) || math.IsInf(close, 0) || close <= 0 {
			return errors.ErrInvalidPrice
		}
		if math.IsNaN(volume) || math.IsInf(volume, 0) || volume <= 0 {
			return errors.ErrInvalidPrice
		}

		if low > high ||
			open < low || open > high ||
			close < low || close > high {
			return errors.ErrInvalidSetOfCandles
		}

	}
	return nil
}

func NewBacktester(symbol, timeframe string, candles []types.Candle, isCloseAutomated bool, strategy types.Strategy) (*Backtester, error) {
	if err := Validate(symbol, timeframe, candles); err != nil {
		return nil, err
	}

	return &Backtester{
		engine:   engine.NewEngine(symbol, timeframe, isCloseAutomated),
		candles:  candles,
		strategy: strategy,
	}, nil
}
