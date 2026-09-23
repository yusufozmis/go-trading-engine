package engine

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/types"
)

// Performance calculates gross profit and loss from confirmed closed positions.
// Trading fees and net profit are summed from each position's realized values.
func (eng *Engine) Performance() types.PerformanceResult {
	if eng == nil {
		return types.PerformanceResult{}
	}

	tpCount := 0
	slCount := 0
	liquidationCount := 0

	var profit float64
	var loss float64
	var tradingFees float64
	var netProfit float64

	for _, position := range eng.closedPositions {
		switch position.State {
		case types.ClosedByProfit:
			tpCount++

		case types.ClosedByStop:
			slCount++

		case types.ClosedByLiquidation:
			liquidationCount++

		default:
			continue
		}

		// NetProfit already has fees deducted, so add them back when grouping
		// fee-exclusive gross profit and loss.
		grossPnL := position.NetProfit + position.Fee
		if grossPnL >= 0 {
			profit += grossPnL
		} else {
			loss += math.Abs(grossPnL)
		}

		tradingFees += position.Fee
		netProfit += position.NetProfit
	}

	return types.PerformanceResult{
		Symbol:           eng.symbol,
		Timeframe:        eng.timeframe,
		TPCount:          tpCount,
		SLCount:          slCount,
		LiquidationCount: liquidationCount,
		Profit:           profit,
		Loss:             loss,
		TradingFees:      tradingFees,
		NetProfit:        netProfit,
	}
}

// FetchClosedPositions returns a snapshot of all confirmed closed positions.
// Mutating the returned slice does not change the engine's internal state.
func (eng *Engine) FetchClosedPositions() ([]types.Position, error) {
	if err := eng.validate(); err != nil {
		return nil, err
	}

	positions := append([]types.Position(nil), eng.closedPositions...)
	return positions, nil
}
