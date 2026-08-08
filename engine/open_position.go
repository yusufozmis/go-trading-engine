package engine

import (
	"github.com/yusufozmis/trading-library/common"
	"github.com/yusufozmis/trading-library/errors"
	"github.com/yusufozmis/trading-library/types"
)

func (eng *Engine) OpenPosition(candle types.Candle) error {

	if eng == nil {
		return errors.ErrNilEngine
	}

	if eng.PositionExists() {
		return nil
	}

	closePrice := candle.PriceData.ClosePrice

	entryPlans := eng.activePlans
	if len(entryPlans) == 0 {
		return nil
	}

	for i, plan := range entryPlans {

		lockKey := entryPlanLockKey(plan)
		if eng.lockKeyMap[lockKey] {
			continue
		}

		if candle.Symbol != plan.Symbol || candle.Timeframe != plan.Timeframe {
			continue
		}

		canOpen := false

		switch plan.Type {
		case types.LONG_OPEN:
			canOpen = closePrice >= plan.EntryPrice
		case types.SHORT_OPEN:
			canOpen = closePrice <= plan.EntryPrice
		default:
			return errors.ErrInvalidEntryPlanType
		}

		if !canOpen {
			continue
		}

		if common.HasPriceMovedTooFar(plan.EntryPrice, candle.PriceData.ClosePrice) {
			eng.removeActivePlanAt(i)
			continue
		}

		newTP, newSL, err := MoveTPSLFromPlan(candle, plan)
		if err != nil {
			return err
		}

		pos := types.Position{
			Symbol:     eng.Symbol,
			Timeframe:  eng.Timeframe,
			Timestamp:  candle.Timestamp,
			EntryPrice: closePrice,
			TP:         newTP,
			StopLoss:   newSL,
			Amount:     common.CalculatePositionSizeForBacktest(closePrice, newSL),
			State:      plan.Type,
		}

		if eng.SetPosition(pos) {
			eng.lockKeyMap[lockKey] = true
			eng.removeActivePlanAt(i)
		}

		// First valid fill wins.
		return nil
	}

	return nil
}

// mapteki son pozisyonun state'i long open ya da short open ise yeni pozisyon insertleme
// returns true when a position opened
func (eng *Engine) SetPosition(position types.Position) bool {

	if eng == nil {
		return false
	}

	if eng.LastPosition == nil {
		eng.LastPosition = &position
		return true
	}

	if eng.LastPosition.State == types.LONG_OPEN || eng.LastPosition.State == types.SHORT_OPEN {
		return false
	}

	eng.LastPosition = &position

	return true
}

func (eng *Engine) PositionExists() bool {

	if eng == nil {
		return false
	}

	if eng.LastPosition == nil {
		return false
	}

	if eng.LastPosition.State == types.LONG_OPEN || eng.LastPosition.State == types.SHORT_OPEN {
		return true
	}

	return false
}
