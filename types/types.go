package types

const (
	BUY  = "buy"
	SELL = "sell"

	LONG  = "long"
	SHORT = "short"
)

type Candle struct {
	Symbol    string
	Timeframe string
	Timestamp int64
	PriceData Prices
	Volume    float64
}

type Prices struct {
	OpenPrice  float64
	HighPrice  float64
	LowPrice   float64
	ClosePrice float64
}

type Context struct {
	HasPendingPosition bool
	HasOpenPosition    bool
}

type EntryPlan struct {
	Symbol    string
	Timeframe string

	Type State

	EntryPrice float64
	StopLoss   float64
	TakeProfit float64

	LockPrice float64
}

type PlanUpdate struct {
	Mode  PlanUpdateMode
	Plans []EntryPlan
}

type Positions struct {
	Symbol     string
	TimeFrame  string
	Timestamp  int64
	EntryPrice float64
	StopLoss   float64
	TP         float64
	Amount     float64
	State      State
}

type PerformanceResults struct {
	Symbol    string
	TimeFrame string

	TPcount int
	SLcount int

	Profit float64
	Loss   float64
}
