package engine

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// DecideOpenPositions evaluates active plans without recording successful
// fills. Hedge mode may produce one action for each currently free side;
// otherwise at most one action is returned.
func (eng *Engine) DecideOpenPositions(candle types.Candle) ([]OpenPositionAction, error) {

	if err := eng.validateCandle(candle); err != nil {
		return nil, err
	}

	if !eng.hedgeMode && eng.PositionExists() {
		return nil, nil
	}

	closePrice := candle.PriceData.ClosePrice

	if len(eng.activePlans) == 0 {
		return nil, nil
	}

	actions := make([]OpenPositionAction, 0, 2)
	selectedSides := make(map[types.PositionSide]bool, 2)
	for i := 0; i < len(eng.activePlans); {
		plan := eng.activePlans[i]
		// A filtered side remains a valid strategy plan, but it cannot produce an
		// open action. Continue so another allowed plan can still open this candle.
		if !eng.isPositionSideAllowed(plan.Side) {
			i++
			continue
		}
		if eng.positionExists(plan.Side) || selectedSides[plan.Side] {
			i++
			continue
		}

		lockKey := eng.entryPlanLockKey(plan)
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
		if !eng.hasRequiredBodyConfirmation(candle, plan) {
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
		if err := eng.applyLiquidationModel(&position); err != nil {
			return nil, err
		}

		// The plan is intentionally left active until the caller confirms that
		// execution succeeded. A failed live order can therefore be retried.
		actions = append(actions, OpenPositionAction{
			Position: position,
			plan:     plan,
		})
		selectedSides[plan.Side] = true
		i++

		if !eng.hedgeMode {
			break
		}
	}

	return actions, nil
}

// ConfirmOpenPosition records a previously decided action as successfully
// opened. It rejects actions whose originating plan is no longer active.
func (eng *Engine) ConfirmOpenPosition(action OpenPositionAction) error {
	if err := eng.validate(); err != nil {
		return err
	}
	// Enforce the filter again at the public confirmation boundary so a caller
	// cannot bypass it with an action created outside DecideOpenPositions.
	if !eng.isPositionSideAllowed(action.Position.Side) {
		return apperrors.ErrInvalidPositionAction
	}
	if eng.positionExists(action.Position.Side) ||
		(!eng.hedgeMode && eng.PositionExists()) {
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

	lockKey := eng.entryPlanLockKey(action.plan)
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

func (eng *Engine) isPositionSideAllowed(side types.PositionSide) bool {
	return side.Valid() &&
		(eng.allowedSide == "" || eng.allowedSide == side)
}

// SetPosition records position when the engine does not already have an open
// position. It reports whether the position was accepted and why it was rejected.
func (eng *Engine) SetPosition(position types.Position) (bool, error) {

	if err := eng.validate(); err != nil {
		return false, err
	}
	// SetPosition is also a position-opening boundary and must obey the same
	// side policy as actions produced by DecideOpenPositions.
	if !eng.isPositionSideAllowed(position.Side) {
		return false, apperrors.ErrInvalidPositionAction
	}

	position.SlippageCost = eng.entrySlippageCost(
		position.EntryPrice,
		position.Amount,
		position.Side,
	)
	if err := eng.applyLiquidationModel(&position); err != nil {
		return false, err
	}
	if !eng.validPosition(position) {
		return false, apperrors.ErrInvalidPositionAction
	}

	if eng.positionExists(position.Side) ||
		(!eng.hedgeMode && eng.PositionExists()) {
		return false, apperrors.ErrPositionOrPendingExists
	}

	eng.positions[position.Side] = &position

	return true, nil
}

// PositionExists reports whether either side currently has an open position.
func (eng *Engine) PositionExists() bool {
	return eng != nil &&
		(eng.positionExists(types.PositionLong) ||
			eng.positionExists(types.PositionShort))
}

// PositionSides returns the sides that currently have an open position.
func (eng *Engine) PositionSides() []types.PositionSide {
	if eng == nil {
		return nil
	}

	sides := make([]types.PositionSide, 0, 2)
	for _, side := range positionSides() {
		if eng.positionExists(side) {
			sides = append(sides, side)
		}
	}
	return sides
}

func (eng *Engine) positionExists(side types.PositionSide) bool {
	if eng == nil || !side.Valid() {
		return false
	}

	position := eng.positions[side]
	return position != nil && position.State == types.PositionOpen
}

func (eng *Engine) hasPriceMovedTooFar(entry, currentPrice float64) bool {
	deviation := math.Abs(currentPrice-entry) / entry
	return deviation >= *eng.maxEntryDeviation
}

// hasRequiredBodyConfirmation reports whether enough of the candle body has
// crossed the entry in the position direction. Wicks are intentionally ignored.
func (eng *Engine) hasRequiredBodyConfirmation(
	candle types.Candle,
	plan types.EntryPlan,
) bool {
	if eng.confirmationPercentage == 0 {
		return true
	}

	bodyLow := math.Min(candle.PriceData.OpenPrice, candle.PriceData.ClosePrice)
	bodyHigh := math.Max(candle.PriceData.OpenPrice, candle.PriceData.ClosePrice)
	bodySize := bodyHigh - bodyLow
	if bodySize == 0 {
		return false
	}

	var crossedBody float64
	switch plan.Side {
	case types.PositionLong:
		crossedBody = bodyHigh - math.Max(bodyLow, plan.EntryPrice)
	case types.PositionShort:
		crossedBody = math.Min(bodyHigh, plan.EntryPrice) - bodyLow
	default:
		return false
	}

	return crossedBody > 0 && crossedBody/bodySize >= eng.confirmationPercentage
}
