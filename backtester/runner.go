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

		pendingSides := eng.PendingSides()
		positionSides := eng.PositionSides()
		plans, err := strategy.Calculate(types.StrategyContext{
			HedgeMode:            eng.HedgeModeEnabled(),
			HasPendingPosition:   eng.PendingExists(),
			HasOpenPosition:      eng.PositionExists(),
			PendingPositionSides: pendingSides,
			OpenPositionSides:    positionSides,
		})
		if err != nil {
			return types.PerformanceResult{}, nil, err
		}

		err = eng.ApplyPlanUpdate(plans)
		if err != nil {
			return types.PerformanceResult{}, nil, err
		}

		if eng.PendingExists() {
			if err := eng.CheckConfirmations(candle); err != nil {
				return types.PerformanceResult{}, nil, err
			}
		}

		openActions, err := eng.DecideOpenPositions(candle)
		if err != nil {
			return types.PerformanceResult{}, nil, err
		}
		for _, action := range openActions {
			// Backtests assume immediate execution, so a decided action can be
			// confirmed without waiting for an external exchange operation.
			if err := eng.ConfirmOpenPosition(action); err != nil {
				return types.PerformanceResult{}, nil, err
			}
		}

		if eng.PositionExists() {
			closeActions, err := eng.DecideClosePositions(candle)
			if err != nil {
				return types.PerformanceResult{}, nil, err
			}
			for _, action := range closeActions {
				if err := eng.ConfirmClosePosition(action); err != nil {
					return types.PerformanceResult{}, nil, err
				}
			}
		}

		if eng.PositionExists() {
			partialActions, err := eng.DecidePartialTakeProfits(candle)
			if err != nil {
				return types.PerformanceResult{}, nil, err
			}
			for _, action := range partialActions {
				// Backtests assume the reduce order fills immediately. Live callers
				// should confirm only after the exchange reduction succeeds.
				if err := eng.ConfirmPartialTakeProfit(action); err != nil {
					return types.PerformanceResult{}, nil, err
				}
			}
		}
	}

	performance, positions := eng.Performance()
	return performance, positions, nil
}
