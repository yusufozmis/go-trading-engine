package engine

import (
	"github.com/yusufozmis/trading-library/errors"
	"github.com/yusufozmis/trading-library/types"
)

func (eng *Engine) ApplyPlanUpdate(update types.PlanUpdate) error {

	if eng == nil {
		return errors.ErrNilEngine
	}

	switch update.Mode {
	case types.KeepPlans:
		return nil
	case types.ReplacePlans:
		if len(update.Plans) == 0 {
			eng.activePlans = nil
			return nil
		}

		plansCopy := make([]types.EntryPlan, len(update.Plans))
		copy(plansCopy, update.Plans)
		eng.activePlans = plansCopy
		return nil

	case types.ClearPlans:
		eng.activePlans = nil
		eng.pendingConfirmation = nil
		return nil

	case types.ConfirmationWaiting:

		if len(update.Plans) != 1 {
			return errors.ErrExpectedSinglePlan
		}
		pending := update.Plans[0]
		eng.activePlans = nil
		eng.pendingConfirmation = &pending
		return nil

	default:
		return errors.ErrInvalidPlanMode
	}
}

func (eng *Engine) removeActivePlanAt(idx int) {
	if eng == nil || idx < 0 || idx >= len(eng.activePlans) {
		return
	}

	eng.activePlans = append(eng.activePlans[:idx], eng.activePlans[idx+1:]...)
}
