package engine

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/types"
)

// Performance calculates gross profit and loss from confirmed closed positions.
// Trading fees and net profit are summed from each position's realized values.
func (eng *Engine) Performance() (types.PerformanceResult, []types.Position) {
	if eng == nil {
		return types.PerformanceResult{}, nil
	}

	tpCount := 0
	slCount := 0
	liquidationCount := 0

	var profit, loss, tradingFees, netProfit float64
	var positions []types.Position

	for _, position := range eng.closedPositions {
		switch position.State {
		case types.ClosedByProfit:
			tpCount++
			positions = append(positions, position)

		case types.ClosedByStop:
			slCount++
			positions = append(positions, position)

		case types.ClosedByLiquidation:
			liquidationCount++
			positions = append(positions, position)

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
	}, positions
}
