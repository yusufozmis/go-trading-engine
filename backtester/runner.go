package backtester

import (
	"github.com/yusufozmis/trading-library/errors"
	"github.com/yusufozmis/trading-library/types"
)

func (b *Backtester) Run() (types.PerformanceResults, error) {

	if b == nil {
		return types.PerformanceResults{}, errors.ErrNilBacktester
	}

	if b.strategy == nil {
		return types.PerformanceResults{}, errors.ErrNilStrategy
	}

	candles := b.candles

	for _, candle := range candles {

		b.strategy.AddBar(candle)

		plans, err := b.strategy.Calculate(types.Context{
			HasPendingPosition: b.engine.PendingExists(),
			HasOpenPosition:    b.engine.PositionExists(),
		})
		if err != nil {
			return types.PerformanceResults{}, err
		}

		err = b.engine.ApplyPlanUpdate(plans)
		if err != nil {
			return types.PerformanceResults{}, err
		}

		if b.engine.PendingExists() {
			b.engine.CheckConfirmation(candle)
		}

		err = b.engine.OpenPosition(candle)
		if err != nil {
			return types.PerformanceResults{}, err
		}

		if b.engine.PositionExists() {
			b.engine.ClosePosition(candle)
		}
	}

	return b.engine.Performance(), nil
}
