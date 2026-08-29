package types

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/errors"
)

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

// Validate reports whether the position contains valid identity, pricing,
// amount, and state values.
func (pos *Position) Validate() error {
	if pos == nil {
		return errors.ErrNilPosition
	}

	if pos.Symbol == "" {
		return errors.ErrNilSymbol
	}

	if pos.Timeframe == "" {
		return errors.ErrNilTimeframe
	}

	if pos.Timestamp <= 0 {
		return errors.ErrInvalidTimestamp
	}

	if !validPositiveFloat(pos.EntryPrice) {
		return errors.ErrInvalidPrice
	}
	if pos.StopLoss != 0 && !validPositiveFloat(pos.StopLoss) {
		return errors.ErrInvalidPrice
	}
	if pos.TP != 0 && !validPositiveFloat(pos.TP) {
		return errors.ErrInvalidPrice
	}

	if !validPositiveFloat(pos.Amount) {
		return errors.ErrInvalidAmount
	}

	switch pos.State {
	case LongOpen:
		if pos.StopLoss != 0 && pos.StopLoss >= pos.EntryPrice {
			return errors.ErrInvalidTPSLBracket
		}
		if pos.TP != 0 && pos.TP <= pos.EntryPrice {
			return errors.ErrInvalidTPSLBracket
		}

	case ShortOpen:
		if pos.StopLoss != 0 && pos.StopLoss <= pos.EntryPrice {
			return errors.ErrInvalidTPSLBracket
		}
		if pos.TP != 0 && pos.TP >= pos.EntryPrice {
			return errors.ErrInvalidTPSLBracket
		}

	case ClosedByStop, ClosedByProfit:
		// Closed states no longer contain the original long/short direction,
		// so only the individual price fields can be validated here.

	default:
		return errors.ErrInvalidPositionState
	}

	return nil
}

func validPositiveFloat(value float64) bool {
	return !math.IsNaN(value) &&
		!math.IsInf(value, 0) &&
		value > 0
}

// PerformanceResult summarizes realized backtest results for one market.
type PerformanceResult struct {
	Symbol    string
	Timeframe string

	TPCount int
	SLCount int

	Profit float64
	Loss   float64
}
