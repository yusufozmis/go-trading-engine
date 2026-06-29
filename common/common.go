package common

import (
	"math"
)

const PosMarginForBacktest float64 = 15
const LeverageForBacktest float64 = 10
const MaxPercentageToOpenPosition float64 = 0.02

func CalculatePositionSizeForBacktest(entry, stop float64) float64 {

	percentage := math.Abs(entry-stop) / entry

	percentage *= 100

	amount := (PosMarginForBacktest * LeverageForBacktest) / percentage

	amount /= entry

	return amount
}

func HasPriceMovedTooFar(entry, currentPrice float64) bool {
	percentage := (math.Abs(currentPrice-entry) / entry)
	result := (percentage >= MaxPercentageToOpenPosition)

	return result
}
