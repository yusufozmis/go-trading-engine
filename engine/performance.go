package engine

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/types"
)

func (eng *Engine) Performance() types.PerformanceResult {

	if eng == nil {
		return types.PerformanceResult{}
	}

	tpCount, slCount := 0, 0
	var profit, loss float64

	for _, pos := range eng.ClosedPositions {

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

	/*netPL := profit - loss

	fmt.Printf("TP: %d | SL: %d \n", tpCount, slCount)
	fmt.Printf("Total profit: $%.2f | Total loss: $%.2f\n", profit, loss)
	fmt.Printf("Net P/L: $%.2f\n", netPL)*/

	return types.PerformanceResult{
		Symbol:    eng.Symbol,
		Timeframe: eng.Timeframe,
		TPcount:   tpCount,
		SLcount:   slCount,
		Profit:    profit,
		Loss:      loss,
	}
}
