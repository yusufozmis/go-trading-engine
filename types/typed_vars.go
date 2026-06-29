package types

type PlanUpdateMode uint
type State uint

const (
	LONG_OPEN State = iota
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
