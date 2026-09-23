package positionsizers

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// FixedRiskSizer sizes each position so the distance from its simulated entry
// fill to its stop-loss represents the configured quote-currency risk. Entry
// slippage is therefore already reflected; fees and eventual exit slippage are
// not included.
type FixedRiskSizer struct {
	risk float64
}

// NewFixedRiskSizer creates a sizer using the supplied quote-currency risk.
func NewFixedRiskSizer(risk float64) (FixedRiskSizer, error) {
	if math.IsNaN(risk) || math.IsInf(risk, 0) || risk <= 0 {
		return FixedRiskSizer{}, apperrors.ErrInvalidAmount
	}

	return FixedRiskSizer{risk: risk}, nil
}

// CalculatePositionSize divides the configured risk by the absolute distance
// between the position's simulated entry fill and stop-loss price.
func (s FixedRiskSizer) CalculatePositionSize(position types.Position) (float64, error) {
	if math.IsNaN(s.risk) || math.IsInf(s.risk, 0) || s.risk <= 0 {
		return 0, apperrors.ErrInvalidAmount
	}
	if math.IsNaN(position.EntryPrice) || math.IsInf(position.EntryPrice, 0) ||
		position.EntryPrice <= 0 || math.IsNaN(position.StopLoss) ||
		math.IsInf(position.StopLoss, 0) || position.StopLoss <= 0 {
		return 0, apperrors.ErrInvalidPrice
	}

	stopDistance := math.Abs(position.EntryPrice - position.StopLoss)
	if stopDistance == 0 {
		return 0, apperrors.ErrInvalidEntryPlanBracket
	}

	amount := s.risk / stopDistance
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return 0, apperrors.ErrInvalidAmount
	}

	return amount, nil
}
