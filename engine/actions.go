package engine

import "github.com/yusufozmis/go-trading-engine/types"

// OpenPositionAction describes a position that the engine has decided should
// be opened. Live callers should execute the corresponding exchange order and
// call ConfirmOpenPosition only after that operation succeeds.
type OpenPositionAction struct {
	// Position is the engine's estimated position state for this decision. Its
	// State identifies the long or short side and Amount contains the current
	// engine sizing result.
	Position types.Position

	// plan links the action to the active plan that produced it. Keeping this
	// private prevents callers from manufacturing an action for an arbitrary plan.
	plan types.EntryPlan
}

// ClosePositionAction contains the closed position state decided by the engine.
// Live callers should execute the corresponding exchange operation and call
// ConfirmClosePosition only after it succeeds.
type ClosePositionAction struct {
	// Position contains the close reason and the realized TP or stop price.
	Position types.Position
	// Side identifies the long or short position that live execution must close.
	Side types.PositionSide
}
