package types

// Candle contains one OHLCV market interval.
type Candle struct {
	Symbol    string
	Timeframe string
	// Timestamp is the candle's Unix timestamp in milliseconds.
	Timestamp int64
	PriceData Prices
	Volume    float64
}

// Prices contains the OHLC prices of a candle.
type Prices struct {
	OpenPrice  float64
	HighPrice  float64
	LowPrice   float64
	ClosePrice float64
}
