package engine

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/types"
)

// Performance calculates realized profit and loss from confirmed closed positions.
func (eng *Engine) Performance() types.PerformanceResult {

	if eng == nil {
		return types.PerformanceResult{}
	}

	tpCount, slCount := 0, 0
	var profit, loss float64

	for _, pos := range eng.closedPositions {

		if pos.State == types.ClosedByProfit {
			tpCount++

			diff := math.Abs(pos.TP - pos.EntryPrice)
			profit += (diff * pos.Amount)

		}
		if pos.State == types.ClosedByStop {
			slCount++

			diff := math.Abs(pos.EntryPrice - pos.StopLoss)
			loss += (diff * pos.Amount)

		}
	}

	return types.PerformanceResult{
		Symbol:    eng.symbol,
		Timeframe: eng.timeframe,
		TPCount:   tpCount,
		SLCount:   slCount,
		Profit:    profit,
		Loss:      loss,
	}
}
