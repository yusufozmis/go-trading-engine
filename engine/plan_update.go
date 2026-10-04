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
			clear(eng.confirmationProgress)
			clear(eng.pendingConfirmations)
			return nil
		}

		plans := make([]types.EntryPlan, 0, len(update.Plans))
		for _, plan := range update.Plans {
			if plan.Confirmation != types.ConfirmationNone {
				return apperrors.ErrInvalidConfirmationState
			}
			if err := eng.validateEntryPlan(plan); err != nil {
				return err
			}
			// Validate every supplied plan, then keep only sides permitted by the
			// engine policy so a filtered plan cannot occupy active state.
			if eng.isPositionSideAllowed(plan.Side) {
				plans = append(plans, plan)
			}
		}

		// Preserve progress only for plans that remain exactly unchanged.
		nextProgress := make(map[types.EntryPlan]candleConfirmationProgress, len(plans))
		for _, plan := range plans {
			if progress, ok := eng.confirmationProgress[plan]; ok {
				nextProgress[plan] = progress
			}
		}
		eng.activePlans = plans
		eng.confirmationProgress = nextProgress
		clear(eng.pendingConfirmations)
		return nil

	case types.ClearPlans:
		eng.activePlans = nil
		clear(eng.confirmationProgress)
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
			// Filter before recording pending state; otherwise a forbidden side
			// could block the allowed side in non-hedge mode.
			if !eng.isPositionSideAllowed(pending.Side) {
				continue
			}
			if pendingBySide[pending.Side] != nil {
				return apperrors.ErrPositionOrPendingExists
			}

			pendingCopy := pending
			pendingBySide[pending.Side] = &pendingCopy
		}

		eng.activePlans = nil
		clear(eng.confirmationProgress)
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

	removedPlan := eng.activePlans[idx]
	eng.activePlans = append(eng.activePlans[:idx], eng.activePlans[idx+1:]...)
	delete(eng.confirmationProgress, removedPlan)
}
