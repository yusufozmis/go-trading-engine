package positionsizers

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// LeveragedMarginSizer sizes a position from a fixed margin, leverage, and the
// percentage distance between the actual entry price and stop-loss price.
type LeveragedMarginSizer struct {
	margin   float64
	leverage float64
}

// NewLeveragedMarginSizer creates a sizer using the supplied margin and leverage.
func NewLeveragedMarginSizer(margin, leverage float64) (LeveragedMarginSizer, error) {
	if math.IsNaN(margin) || math.IsInf(margin, 0) || margin <= 0 {
		return LeveragedMarginSizer{}, errors.ErrInvalidAmount
	}
	if math.IsNaN(leverage) || math.IsInf(leverage, 0) || leverage <= 0 {
		return LeveragedMarginSizer{}, errors.ErrInvalidLeverage
	}

	return LeveragedMarginSizer{
		margin:   margin,
		leverage: leverage,
	}, nil
}

// CalculatePositionSize applies the original backtest sizing formula using the
// position's actual entry and stop-loss prices.
func (s LeveragedMarginSizer) CalculatePositionSize(position types.Position) (float64, error) {
	if math.IsNaN(s.margin) || math.IsInf(s.margin, 0) || s.margin <= 0 {
		return 0, errors.ErrInvalidAmount
	}
	if math.IsNaN(s.leverage) || math.IsInf(s.leverage, 0) || s.leverage <= 0 {
		return 0, errors.ErrInvalidLeverage
	}
	if math.IsNaN(position.EntryPrice) || math.IsInf(position.EntryPrice, 0) ||
		position.EntryPrice <= 0 || math.IsNaN(position.StopLoss) ||
		math.IsInf(position.StopLoss, 0) || position.StopLoss <= 0 {
		return 0, errors.ErrInvalidPrice
	}

	percentage := math.Abs(position.EntryPrice-position.StopLoss) / position.EntryPrice
	percentage *= 100
	if percentage == 0 {
		return 0, errors.ErrInvalidEntryPlanBracket
	}

	amount := (s.margin * s.leverage) / percentage
	amount /= position.EntryPrice

	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return 0, errors.ErrInvalidAmount
	}

	return amount, nil
}
