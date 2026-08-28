package engine

import (
	"github.com/yusufozmis/go-trading-engine/errors"
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

	switch plan.Type {
	case types.ConfirmationWaitingBiggerThanEntry:
		if closePrice <= plan.StopLoss || closePrice >= plan.TakeProfit {
			eng.pendingConfirmation = nil
			return false, nil
		}
		if closePrice >= plan.EntryPrice {
			plan.Type = types.LongOpen
			eng.activePlans = []types.EntryPlan{plan}
			eng.pendingConfirmation = nil
			return true, nil
		}

	case types.ConfirmationWaitingSmallerThanEntry:
		if closePrice >= plan.StopLoss || closePrice <= plan.TakeProfit {
			eng.pendingConfirmation = nil
			return false, nil
		}
		if closePrice <= plan.EntryPrice {
			plan.Type = types.ShortOpen
			eng.activePlans = []types.EntryPlan{plan}
			eng.pendingConfirmation = nil
			return true, nil
		}
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

	if pending.Type != types.ConfirmationWaitingBiggerThanEntry &&
		pending.Type != types.ConfirmationWaitingSmallerThanEntry {
		return errors.ErrInvalidEntryPlanType
	}
	if err := eng.validateEntryPlan(pending); err != nil {
		return err
	}

	if eng.PositionExists() || eng.PendingExists() {
		return errors.ErrPositionOrPendingExists
	}

	eng.activePlans = nil
	eng.pendingConfirmation = &pending

	return nil
}
