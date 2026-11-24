package engine

import (
	"math"
	"slices"

	"github.com/yusufozmis/go-trading-engine/types"
)

// Performance summarizes only fully closed positions, including their cumulative
// partial-exit accounting and signed funding. Open positions contribute nothing.
func (eng *Engine) Performance() (types.PerformanceResult, []types.Position) {
	if eng == nil {
		return types.PerformanceResult{}, nil
	}

	eng.realizedMu.RLock()
	closedPositions := slices.Clone(eng.closedPositions)
	eng.realizedMu.RUnlock()

	result := types.PerformanceResult{
		Symbol:    eng.symbol,
		Timeframe: eng.timeframe,
	}
	positions := make([]types.Position, 0, len(closedPositions))

	for _, position := range closedPositions {
		switch position.State {
		case types.ClosedByProfit:
			result.TPCount++

		case types.ClosedByStop:
			result.SLCount++

		case types.ClosedByLiquidation:
			result.LiquidationCount++

		case types.ClosedByDuration:
			result.DurationCloseCount++

		case types.ClosedByNoProgress:
			result.NoProgressCloseCount++

		case types.ClosedManually:
			result.ManualCloseCount++

		default:
			continue
		}

		positions = append(positions, position)
		result.PartialTakeProfitCount += position.PartialTakeProfitCount
		result.PartialLoseCount += position.PartialLoseCount

		// NetProfit already has fees deducted, so add them back when grouping
		// fee-exclusive gross profit and loss.
		grossPnL := position.NetProfit + position.Fee
		if grossPnL >= 0 {
			result.Profit += grossPnL
		} else {
			result.Loss += math.Abs(grossPnL)
		}

		result.TradingFees += position.Fee
		result.FundingFees += position.FundingFee
		result.NetProfit += position.NetProfit + position.FundingFee
	}

	return result, positions
}
