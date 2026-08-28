package types

// Position contains the engine state and pricing data for one position.
type Position struct {
	Symbol    string
	Timeframe string
	// Timestamp is the position's opening Unix timestamp in milliseconds.
	Timestamp  int64
	EntryPrice float64
	StopLoss   float64
	TP         float64
	Amount     float64
	State      PositionState
}

// PerformanceResult summarizes realized backtest results for one market.
type PerformanceResult struct {
	Symbol    string
	Timeframe string

	TPcount int
	SLcount int

	Profit float64
	Loss   float64
}
