package engine

import (
	"maps"
	"math"
	"slices"

	"github.com/yusufozmis/go-trading-engine/types"
)

// Performance calculates gross profit and loss from confirmed closes and
// realized reductions. The returned positions contain only complete closes.
func (eng *Engine) Performance() (types.PerformanceResult, []types.Position) {
	if eng == nil {
		return types.PerformanceResult{}, nil
	}

	eng.realizedMu.RLock()
	closedPositions := slices.Clone(eng.closedPositions)
	partiallyRealizedPositions := maps.Clone(eng.partiallyRealizedPositions)
	eng.realizedMu.RUnlock()

	result := types.PerformanceResult{
		Symbol:    eng.symbol,
		Timeframe: eng.timeframe,
	}
	positions := make([]types.Position, 0, len(closedPositions))

	// Only complete closes are returned to the caller and included in close-reason
	// counts. Open positions with realized reductions remain absent from this result.
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
		result.NetProfit += position.NetProfit
	}

	// These snapshots contribute only the realized partial accounting of
	// positions that remain open, so they do not affect close-reason counts.
	for _, position := range partiallyRealizedPositions {
		result.PartialTakeProfitCount += position.PartialTakeProfitCount
		result.PartialLoseCount += position.PartialLoseCount
		grossPnL := position.NetProfit + position.Fee
		if grossPnL >= 0 {
			result.Profit += grossPnL
		} else {
			result.Loss += math.Abs(grossPnL)
		}

		result.TradingFees += position.Fee
		result.NetProfit += position.NetProfit
	}

	return result, positions
}
