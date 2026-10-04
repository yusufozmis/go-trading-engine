package engine

import "github.com/yusufozmis/go-trading-engine/types"

// OpenPositionAction describes a position that the engine has decided should
// be opened. Live callers should execute the corresponding exchange order and
// call ConfirmOpenPosition only after that operation succeeds.
type OpenPositionAction struct {
	// Position is the engine's estimated position state for this decision. Its
	// Side identifies the trade direction and Amount contains the current
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
	// Position retains the trade side and contains its close reason and exit price.
	Position types.Position
}

// PartialTakeProfitAction describes a requested reduction of an open position.
// Live callers should reduce Amount on the exchange and call
// ConfirmPartialTakeProfit only after that operation succeeds.
type PartialTakeProfitAction struct {
	// Position is the open position snapshot used to reject stale confirmations.
	Position types.Position
	// Amount is the base-asset amount to reduce.
	Amount float64
	// ExitPrice is the simulated reduction fill including configured slippage.
	ExitPrice float64
	// Timestamp is the triggering candle's Unix timestamp in milliseconds.
	Timestamp int64
}
