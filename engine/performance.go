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

	var profit float64
	var loss float64
	var tradingFees float64
	var netProfit float64

	for _, position := range eng.closedPositions {
		switch position.State {
		case types.ClosedByProfit:
			tpCount++

			difference := math.Abs(position.ExitPrice - position.EntryPrice)
			profit += difference * position.Amount

		case types.ClosedByStop:
			slCount++

			difference := math.Abs(position.EntryPrice - position.ExitPrice)
			loss += difference * position.Amount

		default:
			continue
		}

		tradingFees += position.Fee
		netProfit += position.NetProfit
	}

	return types.PerformanceResult{
		Symbol:      eng.symbol,
		Timeframe:   eng.timeframe,
		TPCount:     tpCount,
		SLCount:     slCount,
		Profit:      profit,
		Loss:        loss,
		TradingFees: tradingFees,
		NetProfit:   netProfit,
	}
}
