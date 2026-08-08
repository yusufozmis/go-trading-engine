package types

type Strategy interface {
	AddBar(candle Candle)
	Calculate(ctx StrategyContext) (PlanUpdate, error)
}

type StrategyContext struct {
	HasPendingPosition bool
	HasOpenPosition    bool
}

type EntryPlan struct {
	Symbol    string
	Timeframe string

	Type PositionState

	EntryPrice float64
	StopLoss   float64
	TakeProfit float64

	LockPrice float64
}

type PlanUpdate struct {
	Mode  PlanUpdateMode
	Plans []EntryPlan
}
