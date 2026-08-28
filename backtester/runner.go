package backtester

import (
	"github.com/yusufozmis/go-trading-engine/engine"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// Run executes the backtest with strategy.
// Each call must receive a fresh strategy instance because Run does not reset
// state accumulated by the strategy during previous executions.
func (b *Backtester) Run(strategy types.Strategy) (types.PerformanceResult, error) {

	if b == nil {
		return types.PerformanceResult{}, errors.ErrNilBacktester
	}

	if strategy == nil {
		return types.PerformanceResult{}, errors.ErrNilStrategy
	}

	eng, err := engine.NewEngine(b.symbol, b.timeframe, b.isCloseAutomated)
	if err != nil {
		return types.PerformanceResult{}, nil
	}

	for _, candle := range b.candles {

		strategy.AddBar(candle)

		plans, err := strategy.Calculate(types.StrategyContext{
			HasPendingPosition: eng.PendingExists(),
			HasOpenPosition:    eng.PositionExists(),
		})
		if err != nil {
			return types.PerformanceResult{}, err
		}

		err = eng.ApplyPlanUpdate(plans)
		if err != nil {
			return types.PerformanceResult{}, err
		}

		if eng.PendingExists() {
			eng.CheckConfirmation(candle)
		}

		err = eng.OpenPosition(candle)
		if err != nil {
			return types.PerformanceResult{}, err
		}

		if eng.PositionExists() {
			eng.ClosePosition(candle)
		}
	}

	return eng.Performance(), nil
}
