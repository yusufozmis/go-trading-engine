package engine

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// DecideOpenPosition evaluates the active plans without recording a successful
// fill. A non-nil action remains unconfirmed until ConfirmOpenPosition is called.
func (eng *Engine) DecideOpenPosition(candle types.Candle) (*OpenPositionAction, error) {

	if err := eng.validateCandle(candle); err != nil {
		return nil, err
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
		switch plan.Side {
		case types.PositionLong:
			canOpen = closePrice >= plan.EntryPrice
		case types.PositionShort:
			canOpen = closePrice <= plan.EntryPrice
		default:
			return nil, apperrors.ErrInvalidSide
		}

		if !canOpen {
			i++
			continue
		}

		if eng.maxEntryDeviation != nil &&
			eng.hasPriceMovedTooFar(plan.EntryPrice, candle.PriceData.ClosePrice) {
			eng.removeActivePlanAt(i)
			continue
		}

		newTP, newSL, err := MoveTPSLFromPlan(candle, plan)
		if err != nil {
			return nil, err
		}
		entryPrice := eng.entryFillPrice(closePrice, plan.Side)
		if (plan.Side == types.PositionLong && (newSL >= entryPrice || newTP <= entryPrice)) ||
			(plan.Side == types.PositionShort && (newSL <= entryPrice || newTP >= entryPrice)) {
			return nil, apperrors.ErrInvalidEntryPlanBracket
		}

		position := types.Position{
			Symbol:        eng.symbol,
			Timeframe:     eng.timeframe,
			OpenTimestamp: candle.Timestamp,
			Side:          plan.Side,
			EntryPrice:    entryPrice,
			TP:            newTP,
			StopLoss:      newSL,
			State:         types.PositionOpen,
		}

		amount, err := eng.positionSizer.CalculatePositionSize(position)
		if err != nil {
			return nil, err
		}
		if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
			return nil, apperrors.ErrInvalidAmount
		}

		position.Amount = amount
		position.SlippageCost = eng.entrySlippageCost(
			position.EntryPrice,
			position.Amount,
			position.Side,
		)

		// The plan is intentionally left active until the caller confirms that
		// execution succeeded. A failed live order can therefore be retried.
		return &OpenPositionAction{
			Position: position,
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
		return apperrors.ErrPositionOrPendingExists
	}

	if action.Position.Side != action.plan.Side {
		return apperrors.ErrInvalidPositionAction
	}
	if action.Position.State != types.PositionOpen || !eng.validPosition(action.Position) {
		return apperrors.ErrInvalidPositionAction
	}

	planIndex := eng.activePlanIndex(action.plan)
	if planIndex < 0 {
		return apperrors.ErrStalePositionAction
	}

	lockKey := entryPlanLockKey(action.plan)
	if eng.lockKeyMap[lockKey] {
		return apperrors.ErrStalePositionAction
	}

	accepted, err := eng.SetPosition(action.Position)
	if err != nil {
		return err
	}
	if !accepted {
		return apperrors.ErrInvalidPositionAction
	}

	eng.lockKeyMap[lockKey] = true
	eng.removeActivePlanAt(planIndex)
	return nil
}

// SetPosition records position when the engine does not already have an open
// position. It reports whether the position was accepted and why it was rejected.
func (eng *Engine) SetPosition(position types.Position) (bool, error) {

	if err := eng.validate(); err != nil {
		return false, err
	}

	if !eng.validPosition(position) {
		return false, apperrors.ErrInvalidPositionAction
	}
	position.SlippageCost = eng.entrySlippageCost(
		position.EntryPrice,
		position.Amount,
		position.Side,
	)

	if eng.lastPosition == nil {
		eng.lastPosition = &position
		return true, nil
	}

	if eng.lastPosition.State == types.PositionOpen {
		return false, apperrors.ErrPositionOrPendingExists
	}

	eng.lastPosition = &position

	return true, nil
}

// PositionExists reports whether the engine currently tracks an open position.
func (eng *Engine) PositionExists() bool {

	if eng == nil {
		return false
	}

	if eng.lastPosition == nil {
		return false
	}

	if eng.lastPosition.State == types.PositionOpen {
		return true
	}

	return false
}

func (eng *Engine) hasPriceMovedTooFar(entry, currentPrice float64) bool {
	deviation := math.Abs(currentPrice-entry) / entry
	return deviation >= *eng.maxEntryDeviation
}
