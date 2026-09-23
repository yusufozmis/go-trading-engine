package engine

import (
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// CheckConfirmation applies candle to the pending confirmation plan and reports
// whether that plan became an active entry plan.
func (eng *Engine) CheckConfirmation(candle types.Candle) (bool, error) {
	if err := eng.validateCandle(candle); err != nil {
		return false, err
	}
	if eng.pendingConfirmation == nil {
		return false, nil
	}

	plan := *eng.pendingConfirmation
	closePrice := candle.PriceData.ClosePrice

	switch plan.Side {
	case types.PositionLong:
		if closePrice <= plan.StopLoss || closePrice >= plan.TakeProfit {
			eng.pendingConfirmation = nil
			return false, nil
		}
	case types.PositionShort:
		if closePrice >= plan.StopLoss || closePrice <= plan.TakeProfit {
			eng.pendingConfirmation = nil
			return false, nil
		}
	default:
		return false, apperrors.ErrInvalidSide
	}

	switch plan.Confirmation {
	case types.ConfirmationWaitingAboveEntry:
		if closePrice >= plan.EntryPrice {
			plan.Confirmation = types.ConfirmationNone
			eng.activePlans = []types.EntryPlan{plan}
			eng.pendingConfirmation = nil
			return true, nil
		}

	case types.ConfirmationWaitingBelowEntry:
		if closePrice <= plan.EntryPrice {
			plan.Confirmation = types.ConfirmationNone
			eng.activePlans = []types.EntryPlan{plan}
			eng.pendingConfirmation = nil
			return true, nil
		}

	default:
		return false, apperrors.ErrInvalidConfirmationState
	}

	return false, nil
}

// PendingExists reports whether the engine is waiting for entry confirmation.
func (eng *Engine) PendingExists() bool {
	return eng != nil && eng.pendingConfirmation != nil
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

	if eng.PositionExists() || eng.PendingExists() {
		return apperrors.ErrPositionOrPendingExists
	}

	eng.activePlans = nil
	eng.pendingConfirmation = &pending

	return nil
}
