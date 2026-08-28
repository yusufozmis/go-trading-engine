package engine

import (
	"math"
	"strconv"

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

// validateEntryPlan ensures that a plan targets this engine and that its
// prices form a valid bracket for the requested direction.
func (eng *Engine) validateEntryPlan(plan types.EntryPlan) error {
	if err := eng.validate(); err != nil {
		return err
	}

	if plan.Symbol != eng.symbol || plan.Timeframe != eng.timeframe {
		return errors.ErrEntryPlanMarketMismatch
	}

	prices := [...]float64{
		plan.EntryPrice,
		plan.StopLoss,
		plan.TakeProfit,
	}
	for _, price := range prices {
		if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
			return errors.ErrInvalidPrice
		}
	}

	if math.IsNaN(plan.LockPrice) || math.IsInf(plan.LockPrice, 0) {
		return errors.ErrInvalidPrice
	}

	switch plan.Type {
	case types.LONG_OPEN, types.ConfirmationWaitingBiggerThanEntry:
		if plan.TakeProfit <= plan.EntryPrice || plan.StopLoss >= plan.EntryPrice {
			return errors.ErrInvalidEntryPlanBracket
		}
	case types.SHORT_OPEN, types.ConfirmationWaitingSmallerThanEntry:
		if plan.TakeProfit >= plan.EntryPrice || plan.StopLoss <= plan.EntryPrice {
			return errors.ErrInvalidEntryPlanBracket
		}
	default:
		return errors.ErrInvalidEntryPlanType
	}

	return nil
}

func (eng *Engine) validPosition(position types.Position) bool {
	if eng.validate() != nil ||
		position.Symbol != eng.symbol ||
		position.Timeframe != eng.timeframe {
		return false
	}

	values := [...]float64{
		position.EntryPrice,
		position.StopLoss,
		position.TP,
		position.Amount,
	}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return false
		}
	}

	switch position.State {
	case types.LONG_OPEN:
		return position.TP > position.EntryPrice && position.StopLoss < position.EntryPrice
	case types.SHORT_OPEN:
		return position.TP < position.EntryPrice && position.StopLoss > position.EntryPrice
	default:
		return false
	}
}

func (eng *Engine) activePlanIndex(target types.EntryPlan) int {
	for i, plan := range eng.activePlans {
		if plan == target {
			return i
		}
	}

	return -1
}

func entryPlanLockKey(plan types.EntryPlan) string {

	key := strconv.FormatFloat(plan.LockPrice, 'g', -1, 64)

	return plan.Symbol + "|" + plan.Timeframe + "|" + key
}
