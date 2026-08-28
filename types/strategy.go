package types

// Strategy consumes candles and produces plan updates for an engine.
type Strategy interface {
	AddBar(candle Candle)
	Calculate(ctx StrategyContext) (PlanUpdate, error)
}

// StrategyContext describes the engine state visible during calculation.
type StrategyContext struct {
	HasPendingPosition bool
	HasOpenPosition    bool
}

// EntryPlan defines an entry trigger and its stop-loss and take-profit bracket.
type EntryPlan struct {
	Symbol    string
	Timeframe string

	Type PositionState

	EntryPrice float64
	StopLoss   float64
	TakeProfit float64

	LockPrice float64
}

// PlanUpdate describes how an engine should update its active strategy plans.
type PlanUpdate struct {
	Mode  PlanUpdateMode
	Plans []EntryPlan
}
