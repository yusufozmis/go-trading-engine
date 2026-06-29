package types

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
