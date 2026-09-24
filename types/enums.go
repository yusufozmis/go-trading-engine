// Package types defines the shared market, strategy, and position contracts.
package types

// PlanUpdateMode controls how a plan update changes existing engine plans.
type PlanUpdateMode uint

// PositionState identifies whether a position is open or why it was closed.
type PositionState uint

// ConfirmationState identifies how an entry plan waits for price confirmation.
type ConfirmationState uint

// MarginMode identifies an exchange futures margin mode.
type MarginMode string

// PositionSide identifies a long or short futures position.
type PositionSide string

// SpotSide identifies a spot buy or sell order.
type SpotSide string

const (
	// UNSPECIFIED is the zero value for an unspecified position state.
	UNSPECIFIED PositionState = iota

	// PositionOpen identifies an open position. Side contains its direction.
	PositionOpen

	// ClosedByStop identifies a position closed by its stop-loss.
	ClosedByStop
	// ClosedByProfit identifies a position closed by its take-profit.
	ClosedByProfit
	// ClosedByLiquidation identifies a position closed at its liquidation level.
	ClosedByLiquidation
	// ClosedByDuration identifies a position closed after reaching its maximum lifetime.
	ClosedByDuration
)

const (
	// ConfirmationNone makes an entry plan immediately eligible for execution.
	ConfirmationNone ConfirmationState = iota
	// ConfirmationWaitingAboveEntry waits for price to reach or cross above entry.
	ConfirmationWaitingAboveEntry
	// ConfirmationWaitingBelowEntry waits for price to reach or cross below entry.
	ConfirmationWaitingBelowEntry
)

const (
	// KeepPlans leaves existing plans unchanged.
	KeepPlans PlanUpdateMode = iota
	// ReplacePlans replaces all existing plans.
	ReplacePlans
	// ClearPlans removes all active and pending plans.
	ClearPlans

	// ConfirmationWaiting replaces active plans with one pending confirmation.
	ConfirmationWaiting
)

const (
	// MarginModeCross uses cross margin.
	MarginModeCross MarginMode = "cross"
	// MarginModeIsolated uses isolated margin.
	MarginModeIsolated MarginMode = "isolated"
)

const (
	// PositionLong identifies a long futures position.
	PositionLong PositionSide = "long"
	// PositionShort identifies a short futures position.
	PositionShort PositionSide = "short"
)

const (
	// SpotBuy identifies a spot buy order.
	SpotBuy SpotSide = "buy"
	// SpotSell identifies a spot sell order.
	SpotSell SpotSide = "sell"
)

// String returns the exchange margin-mode value.
func (m MarginMode) String() string {
	return string(m)
}

// Valid reports whether m is a supported margin mode.
func (m MarginMode) Valid() bool {
	switch m {
	case MarginModeCross, MarginModeIsolated:
		return true
	default:
		return false
	}
}

// String returns the exchange position-side value.
func (s PositionSide) String() string {
	return string(s)
}

// Valid reports whether s is a supported position side.
func (s PositionSide) Valid() bool {
	switch s {
	case PositionLong, PositionShort:
		return true
	default:
		return false
	}
}

// Valid reports whether s is a supported confirmation state.
func (s ConfirmationState) Valid() bool {
	switch s {
	case ConfirmationNone, ConfirmationWaitingAboveEntry, ConfirmationWaitingBelowEntry:
		return true
	default:
		return false
	}
}

// String returns the exchange spot-side value.
func (s SpotSide) String() string {
	return string(s)
}

// Valid reports whether s is a supported spot side.
func (s SpotSide) Valid() bool {
	switch s {
	case SpotBuy, SpotSell:
		return true
	default:
		return false
	}
}
