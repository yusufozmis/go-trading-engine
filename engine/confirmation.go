package engine

import (
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// CheckConfirmations applies candle to each pending side. Confirmed plans become
// active independently, allowing both sides to progress in hedge mode.
func (eng *Engine) CheckConfirmations(candle types.Candle) error {
	if err := eng.validateCandle(candle); err != nil {
		return err
	}

	for _, side := range positionSides() {
		pending := eng.pendingConfirmations[side]
		if pending == nil {
			continue
		}

		plan := *pending
		closePrice := candle.PriceData.ClosePrice
		switch plan.Side {
		case types.PositionLong:
			if closePrice <= plan.StopLoss || closePrice >= plan.TakeProfit {
				delete(eng.pendingConfirmations, side)
				continue
			}
		case types.PositionShort:
			if closePrice >= plan.StopLoss || closePrice <= plan.TakeProfit {
				delete(eng.pendingConfirmations, side)
				continue
			}
		default:
			return apperrors.ErrInvalidSide
		}

		confirmed := false
		switch plan.Confirmation {
		case types.ConfirmationWaitingAboveEntry:
			confirmed = closePrice >= plan.EntryPrice
		case types.ConfirmationWaitingBelowEntry:
			confirmed = closePrice <= plan.EntryPrice
		default:
			return apperrors.ErrInvalidConfirmationState
		}

		if confirmed {
			plan.Confirmation = types.ConfirmationNone
			eng.activePlans = append(eng.activePlans, plan)
			delete(eng.pendingConfirmations, side)
		}
	}

	return nil
}

// PendingExists reports whether either side is waiting for confirmation.
func (eng *Engine) PendingExists() bool {
	return eng != nil &&
		(eng.pendingExists(types.PositionLong) ||
			eng.pendingExists(types.PositionShort))
}

// PendingSides returns the sides currently waiting for entry confirmation.
func (eng *Engine) PendingSides() []types.PositionSide {
	if eng == nil {
		return nil
	}

	sides := make([]types.PositionSide, 0, 2)
	for _, side := range positionSides() {
		if eng.pendingExists(side) {
			sides = append(sides, side)
		}
	}
	return sides
}

func (eng *Engine) pendingExists(side types.PositionSide) bool {
	return eng != nil && side.Valid() && eng.pendingConfirmations[side] != nil
}

// SetPending validates and records one confirmation plan.
func (eng *Engine) SetPending(pending types.EntryPlan) error {

	if err := eng.validate(); err != nil {
		return err
	}

	if pending.Confirmation != types.ConfirmationWaitingAboveEntry &&
		pending.Confirmation != types.ConfirmationWaitingBelowEntry {
		return apperrors.ErrInvalidConfirmationState
	}
	if err := eng.validateEntryPlan(pending); err != nil {
		return err
	}
	// A filtered confirmation is intentionally ignored instead of occupying
	// pending state and blocking a permitted side.
	if !eng.isPositionSideAllowed(pending.Side) {
		return nil
	}

	if eng.positionExists(pending.Side) || eng.pendingExists(pending.Side) ||
		(!eng.hedgeMode && (eng.PositionExists() || eng.PendingExists())) {
		return apperrors.ErrPositionOrPendingExists
	}

	eng.activePlans = nil
	pendingCopy := pending
	eng.pendingConfirmations[pending.Side] = &pendingCopy

	return nil
}
