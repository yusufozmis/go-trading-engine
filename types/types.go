package types

type MarginMode string
type PositionSide string
type SpotSide string

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
