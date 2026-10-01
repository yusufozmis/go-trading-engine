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
			clear(eng.pendingConfirmations)
			return nil
		}

		for _, plan := range update.Plans {
			if plan.Confirmation != types.ConfirmationNone {
				return apperrors.ErrInvalidConfirmationState
			}
			if err := eng.validateEntryPlan(plan); err != nil {
				return err
			}
		}

		plansCopy := make([]types.EntryPlan, len(update.Plans))
		copy(plansCopy, update.Plans)
		eng.activePlans = plansCopy
		clear(eng.pendingConfirmations)
		return nil

	case types.ClearPlans:
		eng.activePlans = nil
		clear(eng.pendingConfirmations)
		return nil

	case types.ConfirmationWaiting:
		if len(update.Plans) == 0 || len(update.Plans) > 2 ||
			(!eng.hedgeMode && len(update.Plans) != 1) {
			return apperrors.ErrInvalidConfirmationPlanCount
		}

		pendingBySide := make(map[types.PositionSide]*types.EntryPlan, len(update.Plans))
		for _, pending := range update.Plans {
			if pending.Confirmation != types.ConfirmationWaitingAboveEntry &&
				pending.Confirmation != types.ConfirmationWaitingBelowEntry {
				return apperrors.ErrInvalidConfirmationState
			}
			if err := eng.validateEntryPlan(pending); err != nil {
				return err
			}
			if pendingBySide[pending.Side] != nil {
				return apperrors.ErrPositionOrPendingExists
			}

			pendingCopy := pending
			pendingBySide[pending.Side] = &pendingCopy
		}

		eng.activePlans = nil
		eng.pendingConfirmations = pendingBySide
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
