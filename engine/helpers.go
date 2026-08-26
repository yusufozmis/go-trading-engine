package engine

import (
	"fmt"
	"math"

	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// MoveTPSLFromPlan re-anchors TP/SL to the candle close while preserving
// the plan's original reward distance and risk distance.
func MoveTPSLFromPlan(candle types.Candle, plan types.EntryPlan) (newTP float64, newSL float64, err error) {

	entry := plan.EntryPrice
	tp := plan.TakeProfit
	sl := plan.StopLoss

	rewardDist := math.Abs(tp - entry)
	riskDist := math.Abs(entry - sl)

	if rewardDist == 0 || riskDist == 0 {
		return 0, 0, errors.ErrInvalidEntryPlanBracket
	}

	isLong := tp > entry && sl < entry
	isShort := tp < entry && sl > entry

	if !isLong && !isShort {
		return 0, 0, errors.ErrInvalidEntryPlanBracket
	}

	newEntry := candle.PriceData.ClosePrice

	if isLong {
		return newEntry + rewardDist, newEntry - riskDist, nil
	}

	return newEntry - rewardDist, newEntry + riskDist, nil
}

func entryPlanLockKey(plan types.EntryPlan) string {

	key := fmt.Sprintf("%.6f", plan.LockPrice)

	return plan.Symbol + "|" + plan.Timeframe + "|" + key
}
