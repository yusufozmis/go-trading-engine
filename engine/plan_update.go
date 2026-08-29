package engine

import (
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// ApplyPlanUpdate validates and applies a strategy's requested plan transition.
func (eng *Engine) ApplyPlanUpdate(update types.PlanUpdate) error {

	if err := eng.validate(); err != nil {
		return err
	}

	switch update.Mode {
	case types.KeepPlans:
		return nil
	case types.ReplacePlans:
		if len(update.Plans) == 0 {
			eng.activePlans = nil
			eng.pendingConfirmation = nil
			return nil
		}

		for _, plan := range update.Plans {
			if plan.Type != types.LongOpen && plan.Type != types.ShortOpen {
				return apperrors.ErrInvalidEntryPlanType
			}
			if err := eng.validateEntryPlan(plan); err != nil {
				return err
			}
		}

		plansCopy := make([]types.EntryPlan, len(update.Plans))
		copy(plansCopy, update.Plans)
		eng.activePlans = plansCopy
		eng.pendingConfirmation = nil
		return nil

	case types.ClearPlans:
		eng.activePlans = nil
		eng.pendingConfirmation = nil
		return nil

	case types.ConfirmationWaiting:

		if len(update.Plans) != 1 {
			return apperrors.ErrExpectedSinglePlan
		}
		pending := update.Plans[0]
		if pending.Type != types.ConfirmationWaitingBiggerThanEntry &&
			pending.Type != types.ConfirmationWaitingSmallerThanEntry {
			return apperrors.ErrInvalidEntryPlanType
		}
		if err := eng.validateEntryPlan(pending); err != nil {
			return err
		}

		eng.activePlans = nil
		eng.pendingConfirmation = &pending
		return nil

	default:
		return apperrors.ErrInvalidPlanMode
	}
}

func (eng *Engine) removeActivePlanAt(idx int) {
	if eng == nil || idx < 0 || idx >= len(eng.activePlans) {
		return
	}

	eng.activePlans = append(eng.activePlans[:idx], eng.activePlans[idx+1:]...)
}
