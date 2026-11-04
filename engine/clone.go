package engine

import (
	"maps"
	"slices"

	"github.com/yusufozmis/go-trading-engine/types"
)

// Clone copies the engine's configuration and trading state with fresh mutexes.
// Positions, plans, confirmation progress, and realized history are independent
// of the original. Do not call it concurrently with state-changing methods.
// Injected PositionSizer and LiquidationModel implementations are reused, not
// deep-copied; custom mutable implementations must account for that sharing.
// This is an in-memory copy, not persistence or exchange reconciliation.
func (eng *Engine) Clone() (*Engine, error) {
	if err := eng.Validate(); err != nil {
		return nil, err
	}
	eng.positionSizerMu.Lock()
	sizer := eng.positionSizer
	eng.positionSizerMu.Unlock()
	eng.realizedMu.RLock()
	defer eng.realizedMu.RUnlock()

	// Build a new engine explicitly: copying the struct would copy used mutexes.
	clone := &Engine{
		symbol:                      eng.symbol,
		timeframe:                   eng.timeframe,
		acceptedCandles:             maps.Clone(eng.acceptedCandles),
		lastCandleTimestamp:         eng.lastCandleTimestamp,
		positions:                   make(map[types.PositionSide]*types.Position, len(eng.positions)),
		closedPositions:             slices.Clone(eng.closedPositions),
		partiallyRealizedPositions:  maps.Clone(eng.partiallyRealizedPositions),
		lockKeyMap:                  maps.Clone(eng.lockKeyMap),
		activePlans:                 slices.Clone(eng.activePlans),
		confirmationProgress:        maps.Clone(eng.confirmationProgress),
		pendingConfirmations:        make(map[types.PositionSide]*types.EntryPlan, len(eng.pendingConfirmations)),
		pendingStartedAt:            maps.Clone(eng.pendingStartedAt),
		positionSizer:               sizer,
		hedgeMode:                   eng.hedgeMode,
		isCloseAutomated:            eng.isCloseAutomated,
		tradingFeeRate:              eng.tradingFeeRate,
		slippageRate:                eng.slippageRate,
		liquidationModel:            eng.liquidationModel,
		maximumPositionDuration:     eng.maximumPositionDuration,
		breakEvenStopRate:           eng.breakEvenStopRate,
		allowedSide:                 eng.allowedSide,
		confirmationPercentage:      eng.confirmationPercentage,
		confirmationCandleCount:     eng.confirmationCandleCount,
		confirmationTimeout:         eng.confirmationTimeout,
		partialTakeProfitPercentage: eng.partialTakeProfitPercentage,
		partialReductionPercentage:  eng.partialReductionPercentage,
		profitLockTriggerRate:       eng.profitLockTriggerRate,
		profitLockRate:              eng.profitLockRate,
	}
	if eng.maxEntryDeviation != nil {
		deviation := *eng.maxEntryDeviation
		clone.maxEntryDeviation = &deviation
	}
	for side, position := range eng.positions {
		if position == nil {
			clone.positions[side] = nil
			continue
		}
		positionCopy := *position
		clone.positions[side] = &positionCopy
	}
	for side, plan := range eng.pendingConfirmations {
		if plan == nil {
			clone.pendingConfirmations[side] = nil
			continue
		}
		planCopy := *plan
		clone.pendingConfirmations[side] = &planCopy
	}
	return clone, nil
}
