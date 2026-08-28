package engine

import (
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

func (eng *Engine) CheckConfirmation(candle types.Candle) bool {
	if !eng.acceptsCandle(candle) || eng.pendingConfirmation == nil {
		return false
	}

	plan := *eng.pendingConfirmation
	closePrice := candle.PriceData.ClosePrice

	switch plan.Type {
	case types.ConfirmationWaitingBiggerThanEntry:
		if closePrice <= plan.StopLoss || closePrice >= plan.TakeProfit {
			eng.pendingConfirmation = nil
			return false
		}
		if closePrice >= plan.EntryPrice {
			plan.Type = types.LONG_OPEN
			eng.activePlans = []types.EntryPlan{plan}
			eng.pendingConfirmation = nil
			return true
		}

	case types.ConfirmationWaitingSmallerThanEntry:
		if closePrice >= plan.StopLoss || closePrice <= plan.TakeProfit {
			eng.pendingConfirmation = nil
			return false
		}
		if closePrice <= plan.EntryPrice {
			plan.Type = types.SHORT_OPEN
			eng.activePlans = []types.EntryPlan{plan}
			eng.pendingConfirmation = nil
			return true
		}
	}

	return false
}

func (eng *Engine) PendingExists() bool {
	return eng != nil && eng.pendingConfirmation != nil
}

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

	eng.pendingConfirmation = &pending

	return nil
}
