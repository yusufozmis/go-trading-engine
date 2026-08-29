package types

// Unified CCXT timeframe values. Individual providers may support only a
// subset; use Client.SupportedTimeframes to discover the active provider's set.
const (
	// Timeframe1Second represents one-second candles.
	Timeframe1Second = "1s"
	// Timeframe1Minute represents one-minute candles.
	Timeframe1Minute = "1m"
	// Timeframe3Minutes represents three-minute candles.
	Timeframe3Minutes = "3m"
	// Timeframe5Minutes represents five-minute candles.
	Timeframe5Minutes = "5m"
	// Timeframe15Minutes represents fifteen-minute candles.
	Timeframe15Minutes = "15m"
	// Timeframe30Minutes represents thirty-minute candles.
	Timeframe30Minutes = "30m"
	// Timeframe1Hour represents one-hour candles.
	Timeframe1Hour = "1h"
	// Timeframe2Hours represents two-hour candles.
	Timeframe2Hours = "2h"
	// Timeframe4Hours represents four-hour candles.
	Timeframe4Hours = "4h"
	// Timeframe6Hours represents six-hour candles.
	Timeframe6Hours = "6h"
	// Timeframe8Hours represents eight-hour candles.
	Timeframe8Hours = "8h"
	// Timeframe12Hours represents twelve-hour candles.
	Timeframe12Hours = "12h"
	// Timeframe1Day represents one-day candles.
	Timeframe1Day = "1d"
	// Timeframe3Days represents three-day candles.
	Timeframe3Days = "3d"
	// Timeframe1Week represents one-week candles.
	Timeframe1Week = "1w"
	// Timeframe1Month represents one-month candles.
	Timeframe1Month = "1M"
	// Timeframe3Months represents three-month candles.
	Timeframe3Months = "3M"
)
