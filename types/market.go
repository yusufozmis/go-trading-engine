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
