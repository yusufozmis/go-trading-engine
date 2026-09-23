package positionsizers

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// LeveragedMarginSizer sizes a position from a fixed margin, leverage, and the
// percentage distance between its simulated entry fill and stop-loss price.
// Entry slippage is therefore already reflected; fees and eventual exit
// slippage are not included.
type LeveragedMarginSizer struct {
	margin   float64
	leverage float64
}

// NewLeveragedMarginSizer creates a sizer using the supplied margin and leverage.
func NewLeveragedMarginSizer(margin, leverage float64) (LeveragedMarginSizer, error) {
	if math.IsNaN(margin) || math.IsInf(margin, 0) || margin <= 0 {
		return LeveragedMarginSizer{}, apperrors.ErrInvalidAmount
	}
	if math.IsNaN(leverage) || math.IsInf(leverage, 0) || leverage <= 0 {
		return LeveragedMarginSizer{}, apperrors.ErrInvalidLeverage
	}

	return LeveragedMarginSizer{
		margin:   margin,
		leverage: leverage,
	}, nil
}

// CalculatePositionSize applies the original backtest sizing formula using the
// position's simulated entry fill and stop-loss price.
func (s LeveragedMarginSizer) CalculatePositionSize(position types.Position) (float64, error) {
	if math.IsNaN(s.margin) || math.IsInf(s.margin, 0) || s.margin <= 0 {
		return 0, apperrors.ErrInvalidAmount
	}
	if math.IsNaN(s.leverage) || math.IsInf(s.leverage, 0) || s.leverage <= 0 {
		return 0, apperrors.ErrInvalidLeverage
	}
	if math.IsNaN(position.EntryPrice) || math.IsInf(position.EntryPrice, 0) ||
		position.EntryPrice <= 0 || math.IsNaN(position.StopLoss) ||
		math.IsInf(position.StopLoss, 0) || position.StopLoss <= 0 {
		return 0, apperrors.ErrInvalidPrice
	}

	percentage := math.Abs(position.EntryPrice-position.StopLoss) / position.EntryPrice
	percentage *= 100
	if percentage == 0 {
		return 0, apperrors.ErrInvalidEntryPlanBracket
	}

	amount := (s.margin * s.leverage) / percentage
	amount /= position.EntryPrice

	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return 0, apperrors.ErrInvalidAmount
	}

	return amount, nil
}
