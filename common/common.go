// Package common contains shared calculations used by the trading engine.
package common

import (
	"math"
)

// PosMarginForBacktest is the fixed margin used by the current sizing model.
const PosMarginForBacktest float64 = 15

// LeverageForBacktest is the fixed leverage used by the current sizing model.
const LeverageForBacktest float64 = 10

// MaxPercentageToOpenPosition is the maximum accepted move from a planned entry.
const MaxPercentageToOpenPosition float64 = 0.02

// CalculatePositionSizeForBacktest calculates the base-asset amount for the
// current fixed-margin backtest model.
func CalculatePositionSizeForBacktest(entry, stop float64) float64 {

	percentage := math.Abs(entry-stop) / entry

	percentage *= 100

	amount := (PosMarginForBacktest * LeverageForBacktest) / percentage

	amount /= entry

	return amount
}

// HasPriceMovedTooFar reports whether currentPrice moved too far from entry.
func HasPriceMovedTooFar(entry, currentPrice float64) bool {
	percentage := (math.Abs(currentPrice-entry) / entry)
	result := (percentage >= MaxPercentageToOpenPosition)

	return result
}
