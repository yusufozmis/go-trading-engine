package engine

import (
	"github.com/yusufozmis/go-trading-engine/common"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// DecideOpenPosition evaluates the active plans without recording a successful
// fill. A non-nil action remains unconfirmed until ConfirmOpenPosition is called.
func (eng *Engine) DecideOpenPosition(candle types.Candle) (*OpenPositionAction, error) {

	if err := eng.validate(); err != nil {
		return nil, err
	}
	if !eng.acceptsCandle(candle) {
		return nil, errors.ErrCandleMarketMismatch
	}

	if eng.PositionExists() {
		return nil, nil
	}

	closePrice := candle.PriceData.ClosePrice

	if len(eng.activePlans) == 0 {
		return nil, nil
	}

	for i := 0; i < len(eng.activePlans); {
		plan := eng.activePlans[i]

		lockKey := entryPlanLockKey(plan)
		if eng.lockKeyMap[lockKey] {
			i++
			continue
		}

		if candle.Symbol != plan.Symbol || candle.Timeframe != plan.Timeframe {
			i++
			continue
		}

		canOpen := false

		switch plan.Type {
		case types.LONG_OPEN:
			canOpen = closePrice >= plan.EntryPrice
		case types.SHORT_OPEN:
			canOpen = closePrice <= plan.EntryPrice
		default:
			return nil, errors.ErrInvalidEntryPlanType
		}

		if !canOpen {
			i++
			continue
		}

		if common.HasPriceMovedTooFar(plan.EntryPrice, candle.PriceData.ClosePrice) {
			// The next plan shifts into this index, so process the same index again.
			eng.removeActivePlanAt(i)
			continue
		}

		newTP, newSL, err := MoveTPSLFromPlan(candle, plan)
		if err != nil {
			return nil, err
		}

		pos := types.Position{
			Symbol:     eng.symbol,
			Timeframe:  eng.timeframe,
			Timestamp:  candle.Timestamp,
			EntryPrice: closePrice,
			TP:         newTP,
			StopLoss:   newSL,
			Amount:     common.CalculatePositionSizeForBacktest(closePrice, newSL),
			State:      plan.Type,
		}

		// The plan is intentionally left active until the caller confirms that
		// execution succeeded. A failed live order can therefore be retried.
		return &OpenPositionAction{
			Position: pos,
			plan:     plan,
		}, nil
	}

	return nil, nil
}

// ConfirmOpenPosition records a previously decided action as successfully
// opened. It rejects actions whose originating plan is no longer active.
func (eng *Engine) ConfirmOpenPosition(action OpenPositionAction) error {
	if err := eng.validate(); err != nil {
		return err
	}
	if eng.PositionExists() {
		return errors.ErrPositionOrPendingExists
	}
	if action.Position.State != action.plan.Type || !eng.validPosition(action.Position) {
		return errors.ErrInvalidPositionAction
	}

	planIndex := eng.activePlanIndex(action.plan)
	if planIndex < 0 {
		return errors.ErrStalePositionAction
	}

	lockKey := entryPlanLockKey(action.plan)
	if eng.lockKeyMap[lockKey] {
		return errors.ErrStalePositionAction
	}

	if !eng.SetPosition(action.Position) {
		return errors.ErrInvalidPositionAction
	}

	eng.lockKeyMap[lockKey] = true
	eng.removeActivePlanAt(planIndex)
	return nil
}

// SetPosition records position when the engine does not already have an open
// position. It returns whether the position was accepted.
func (eng *Engine) SetPosition(position types.Position) bool {

	if eng == nil {
		return false
	}

	if !eng.validPosition(position) {
		return false
	}

	if eng.lastPosition == nil {
		eng.lastPosition = &position
		return true
	}

	if eng.lastPosition.State == types.LONG_OPEN || eng.lastPosition.State == types.SHORT_OPEN {
		return false
	}

	eng.lastPosition = &position

	return true
}

// PositionExists reports whether the engine currently tracks an open position.
func (eng *Engine) PositionExists() bool {

	if eng == nil {
		return false
	}

	if eng.lastPosition == nil {
		return false
	}

	if eng.lastPosition.State == types.LONG_OPEN || eng.lastPosition.State == types.SHORT_OPEN {
		return true
	}

	return false
}
