package backtester

import (
	"github.com/yusufozmis/go-trading-engine/engine"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

type Backtester struct {
	engine   *engine.Engine
	candles  []types.Candle
	strategy types.Strategy
}

func Validate(candles []types.Candle) {

	m := make(map[string]bool)

	m[candles[0].Symbol] = true

	for _, candle := range candles {

		if !m[candle.Symbol] {
			panic(errors.ErrInvalidSetOfCandles)
		}
	}
}

func NewBacktester(symbol, timeframe string, candles []types.Candle, isCloseAutomated bool, strategy types.Strategy) *Backtester {

	Validate(candles)

	return &Backtester{
		engine:   engine.NewEngine(symbol, timeframe, isCloseAutomated),
		candles:  candles,
		strategy: strategy,
	}
}
