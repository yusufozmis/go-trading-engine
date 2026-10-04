package engine

import (
	"maps"
	"math"
	"slices"

	"github.com/yusufozmis/go-trading-engine/types"
)

// Performance calculates gross profit and loss from confirmed closes and
// realized partial profits. The returned positions contain only complete closes.
func (eng *Engine) Performance() (types.PerformanceResult, []types.Position) {
	if eng == nil {
		return types.PerformanceResult{}, nil
	}

	eng.realizedMu.RLock()
	closedPositions := slices.Clone(eng.closedPositions)
	partiallyRealizedPositions := maps.Clone(eng.partiallyRealizedPositions)
	eng.realizedMu.RUnlock()

	result := types.PerformanceResult{
		Symbol:                 eng.symbol,
		Timeframe:              eng.timeframe,
		PartialTakeProfitCount: len(partiallyRealizedPositions),
	}
	positions := make([]types.Position, 0, len(closedPositions))

	// Only complete closes are returned to the caller and included in close-reason
	// counts. Open positions with partial profit remain absent from this result.
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

		default:
			continue
		}

		positions = append(positions, position)
		if position.PartialTakeProfitExecuted {
			result.PartialTakeProfitCount++
		}

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
