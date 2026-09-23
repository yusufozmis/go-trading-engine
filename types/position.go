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

	Side PositionSide

	EntryPrice float64
	// ExitPrice is zero while the position is open. For automated closes it is
	// the triggered TP or stop price; otherwise it is the candle close price
	// used as the simulated exit fill.
	ExitPrice float64

	// StopLoss is the configured stop trigger and is not overwritten by the
	// eventual exit fill.
	StopLoss float64
	// TP is the configured take-profit trigger and is not overwritten by the
	// eventual exit fill.
	TP     float64
	Amount float64

	// Fee is the total entry and exit fee paid for a confirmed closed position.
	// It remains zero when no trading fee rate is configured.
	Fee float64

	// NetProfit is the signed realized PnL after fees. It remains zero while the
	// position is open and may be positive, negative, or zero after closing.
	NetProfit float64

	State PositionState
}

// Validate reports whether the position contains valid identity, pricing,
// amount, fee, and state values.
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
	case PositionOpen:
		if !pos.Side.Valid() {
			return apperrors.ErrInvalidSide
		}
		if pos.CloseTimestamp != 0 {
			return apperrors.ErrInvalidTimestamp
		}
		if pos.ExitPrice != 0 {
			return apperrors.ErrInvalidPrice
		}
		if pos.Fee != 0 {
			return apperrors.ErrInvalidFee
		}
		if pos.NetProfit != 0 {
			return apperrors.ErrInvalidNetProfit
		}

	case ClosedByStop, ClosedByProfit:
		if !pos.Side.Valid() {
			return apperrors.ErrInvalidSide
		}
		if pos.CloseTimestamp == 0 ||
			pos.CloseTimestamp < pos.OpenTimestamp {
			return apperrors.ErrInvalidTimestamp
		}
		if !validPositiveFloat(pos.ExitPrice) {
			return apperrors.ErrInvalidPrice
		}

	default:
		return apperrors.ErrInvalidPositionState
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
	if math.IsNaN(pos.Fee) || math.IsInf(pos.Fee, 0) || pos.Fee < 0 {
		return apperrors.ErrInvalidFee
	}
	if math.IsNaN(pos.NetProfit) || math.IsInf(pos.NetProfit, 0) {
		return apperrors.ErrInvalidNetProfit
	}

	switch pos.Side {
	case PositionLong:
		if pos.StopLoss != 0 && pos.StopLoss >= pos.EntryPrice {
			return apperrors.ErrInvalidTPSLBracket
		}
		if pos.TP != 0 && pos.TP <= pos.EntryPrice {
			return apperrors.ErrInvalidTPSLBracket
		}

	case PositionShort:
		if pos.StopLoss != 0 && pos.StopLoss <= pos.EntryPrice {
			return apperrors.ErrInvalidTPSLBracket
		}
		if pos.TP != 0 && pos.TP >= pos.EntryPrice {
			return apperrors.ErrInvalidTPSLBracket
		}

	default:
		return apperrors.ErrInvalidSide
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
