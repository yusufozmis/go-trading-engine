package engine

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/types"
)

// Performance calculates gross profit, gross loss, trading fees, and net profit
// from confirmed closed positions. The configured trading fee rate is applied
// independently to each position's entry and exit notional.
func (eng *Engine) Performance() types.PerformanceResult {
	if eng == nil {
		return types.PerformanceResult{}
	}

	tpCount := 0
	slCount := 0

	var profit float64
	var loss float64
	var tradingFees float64

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

		entryNotional := position.EntryPrice * position.Amount
		exitNotional := position.ExitPrice * position.Amount

		entryFee := entryNotional * eng.tradingFeeRate
		exitFee := exitNotional * eng.tradingFeeRate

		tradingFees += entryFee + exitFee
	}

	return types.PerformanceResult{
		Symbol:      eng.symbol,
		Timeframe:   eng.timeframe,
		TPCount:     tpCount,
		SLCount:     slCount,
		Profit:      profit,
		Loss:        loss,
		TradingFees: tradingFees,
		NetProfit:   profit - loss - tradingFees,
	}
}
