package backtester

import (
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/engine"
	"github.com/yusufozmis/go-trading-engine/types"
)

// Run executes the backtest with strategy.
// Each call must receive a fresh strategy instance because Run does not reset
// state accumulated by the strategy during previous executions.
func (b *Backtester) Run(strategy types.Strategy) (types.PerformanceResult, []types.Position, error) {

	if b == nil {
		return types.PerformanceResult{}, nil, apperrors.ErrNilBacktester
	}

	if strategy == nil {
		return types.PerformanceResult{}, nil, apperrors.ErrNilStrategy
	}

	eng, err := engine.NewEngine(b.symbol, b.timeframe,
		b.positionSizer, b.options...)
	if err != nil {
		return types.PerformanceResult{}, nil, err
	}

	for _, candle := range b.candles {

		strategy.AddBar(candle)

		plans, err := strategy.Calculate(types.StrategyContext{
			HasPendingPosition: eng.PendingExists(),
			HasOpenPosition:    eng.PositionExists(),
		})
		if err != nil {
			return types.PerformanceResult{}, nil, err
		}

		err = eng.ApplyPlanUpdate(plans)
		if err != nil {
			return types.PerformanceResult{}, nil, err
		}

		if eng.PendingExists() {
			if _, err := eng.CheckConfirmation(candle); err != nil {
				return types.PerformanceResult{}, nil, err
			}
		}

		openAction, err := eng.DecideOpenPosition(candle)
		if err != nil {
			return types.PerformanceResult{}, nil, err
		}
		if openAction != nil {
			// Backtests assume immediate execution, so a decided action can be
			// confirmed without waiting for an external exchange operation.
			if err := eng.ConfirmOpenPosition(*openAction); err != nil {
				return types.PerformanceResult{}, nil, err
			}
		}

		if eng.PositionExists() {
			closeAction, err := eng.DecideClosePosition(candle)
			if err != nil {
				return types.PerformanceResult{}, nil, err
			}
			if closeAction != nil {
				if err := eng.ConfirmClosePosition(*closeAction); err != nil {
					return types.PerformanceResult{}, nil, err
				}
			}
		}
	}

	positions, err := eng.FetchClosedPositions()
	if err != nil {
		return types.PerformanceResult{}, nil, err
	}

	return eng.Performance(), positions, nil
}
