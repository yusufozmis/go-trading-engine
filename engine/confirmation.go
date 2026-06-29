package engine

import "github.com/yusufozmis/trading-library/types"

func (eng *Engine) CheckConfirmation(candle types.Candle) bool {
	if eng == nil || eng.pendingConfirmation == nil {
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

	if eng == nil {
		return false
	}

	if eng.pendingConfirmation == nil {
		eng.pendingConfirmation = &types.EntryPlan{}
		return false
	}

	if eng.pendingConfirmation.Type == types.ConfirmationWaitingBiggerThanEntry ||
		eng.pendingConfirmation.Type == types.ConfirmationWaitingSmallerThanEntry {
		return true
	}

	return false
}

func (eng *Engine) SetPending(pending types.EntryPlan) {

	if eng == nil {
		return
	}

	if eng.PositionExists() || eng.PendingExists() {
		return
	}

	eng.pendingConfirmation = &pending

}
