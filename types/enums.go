package types

type PlanUpdateMode uint
type PositionState uint

type MarginMode string
type PositionSide string
type SpotSide string

const (
	UNSPECIFIED PositionState = iota

	LONG_OPEN
	SHORT_OPEN

	ConfirmationWaitingBiggerThanEntry
	ConfirmationWaitingSmallerThanEntry

	CancelSignal

	ClosedByStop
	ClosedByProfit
)

const (
	KeepPlans PlanUpdateMode = iota
	ReplacePlans
	ClearPlans

	ConfirmationWaiting
)

const (
	MarginModeCross    MarginMode = "cross"
	MarginModeIsolated MarginMode = "isolated"
)

const (
	PositionLong  PositionSide = "long"
	PositionShort PositionSide = "short"
)

const (
	SpotBuy  SpotSide = "buy"
	SpotSell SpotSide = "sell"
)

func (m MarginMode) String() string {
	return string(m)
}

func (m MarginMode) Valid() bool {
	switch m {
	case MarginModeCross, MarginModeIsolated:
		return true
	default:
		return false
	}
}

func (s PositionSide) String() string {
	return string(s)
}

func (s PositionSide) Valid() bool {
	switch s {
	case PositionLong, PositionShort:
		return true
	default:
		return false
	}
}

func (s SpotSide) String() string {
	return string(s)
}

func (s SpotSide) Valid() bool {
	switch s {
	case SpotBuy, SpotSell:
		return true
	default:
		return false
	}
}
