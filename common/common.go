// Package common contains shared calculations used by the trading engine.
package common

import "math"

// PosMarginForBacktest is the fixed margin used by the current sizing model.
const PosMarginForBacktest float64 = 15

// LeverageForBacktest is the fixed leverage used by the current sizing model.
const LeverageForBacktest float64 = 10

// CalculatePositionSizeForBacktest calculates the base-asset amount for the
// current fixed-margin backtest model.
func CalculatePositionSizeForBacktest(entry, stop float64) float64 {

	percentage := math.Abs(entry-stop) / entry

	percentage *= 100

	amount := (PosMarginForBacktest * LeverageForBacktest) / percentage

	amount /= entry

	return amount
}
