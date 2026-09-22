package types

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
)

// Position contains the engine state and pricing data for one position.
type Position struct {
	Symbol    string
	Timeframe string
	// OpenTimestamp is the position's opening Unix timestamp in milliseconds.
	OpenTimestamp int64
	// CloseTimestamp is the position's close Unix timestamp in milliseconds.
	CloseTimestamp int64
	EntryPrice     float64
	StopLoss       float64
	TP             float64
	Amount         float64
	State          PositionState
}

// Validate reports whether the position contains valid identity, pricing,
// amount, and state values.
func (pos *Position) Validate() error {
	if pos == nil {
		return apperrors.ErrNilPosition
	}

	if pos.Symbol == "" {
		return apperrors.ErrNilSymbol
	}

	if pos.Timeframe == "" {
		return apperrors.ErrNilTimeframe
	}

	if pos.OpenTimestamp <= 0 {
		return apperrors.ErrInvalidTimestamp
	}

	switch pos.State {
	case LongOpen, ShortOpen:
		if pos.CloseTimestamp != 0 {
			return apperrors.ErrInvalidTimestamp
		}

	case ClosedByStop, ClosedByProfit:
		if pos.CloseTimestamp == 0 ||
			pos.CloseTimestamp < pos.OpenTimestamp {
			return apperrors.ErrInvalidTimestamp
		}
	}

	if !validPositiveFloat(pos.EntryPrice) {
		return apperrors.ErrInvalidPrice
	}
	if pos.StopLoss != 0 && !validPositiveFloat(pos.StopLoss) {
		return apperrors.ErrInvalidPrice
	}
	if pos.TP != 0 && !validPositiveFloat(pos.TP) {
		return apperrors.ErrInvalidPrice
	}

	if !validPositiveFloat(pos.Amount) {
		return apperrors.ErrInvalidAmount
	}

	switch pos.State {
	case LongOpen:
		if pos.StopLoss != 0 && pos.StopLoss >= pos.EntryPrice {
			return apperrors.ErrInvalidTPSLBracket
		}
		if pos.TP != 0 && pos.TP <= pos.EntryPrice {
			return apperrors.ErrInvalidTPSLBracket
		}

	case ShortOpen:
		if pos.StopLoss != 0 && pos.StopLoss <= pos.EntryPrice {
			return apperrors.ErrInvalidTPSLBracket
		}
		if pos.TP != 0 && pos.TP >= pos.EntryPrice {
			return apperrors.ErrInvalidTPSLBracket
		}

	case ClosedByStop, ClosedByProfit:
		// Closed states no longer contain the original long/short direction,
		// so only the individual price fields can be validated here.

	default:
		return apperrors.ErrInvalidPositionState
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

	TradingFees float64
	NetProfit   float64
}
