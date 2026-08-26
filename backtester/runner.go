package backtester

import (
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

func (b *Backtester) Run() (types.PerformanceResult, error) {

	if b == nil {
		return types.PerformanceResult{}, errors.ErrNilBacktester
	}

	if b.strategy == nil {
		return types.PerformanceResult{}, errors.ErrNilStrategy
	}

	candles := b.candles

	for _, candle := range candles {

		b.strategy.AddBar(candle)

		plans, err := b.strategy.Calculate(types.StrategyContext{
			HasPendingPosition: b.engine.PendingExists(),
			HasOpenPosition:    b.engine.PositionExists(),
		})
		if err != nil {
			return types.PerformanceResult{}, err
		}

		err = b.engine.ApplyPlanUpdate(plans)
		if err != nil {
			return types.PerformanceResult{}, err
		}

		if b.engine.PendingExists() {
			b.engine.CheckConfirmation(candle)
		}

		err = b.engine.OpenPosition(candle)
		if err != nil {
			return types.PerformanceResult{}, err
		}

		if b.engine.PositionExists() {
			b.engine.ClosePosition(candle)
		}
	}

	return b.engine.Performance(), nil
}
