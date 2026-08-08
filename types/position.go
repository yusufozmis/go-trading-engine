package types

type Position struct {
	Symbol     string
	Timeframe  string
	Timestamp  int64
	EntryPrice float64
	StopLoss   float64
	TP         float64
	Amount     float64
	State      PositionState
}

type PerformanceResult struct {
	Symbol    string
	Timeframe string

	TPcount int
	SLcount int

	Profit float64
	Loss   float64
}
