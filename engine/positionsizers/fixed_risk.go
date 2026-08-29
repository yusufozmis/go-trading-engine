package positionsizers

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// FixedRiskSizer sizes each position so reaching its stop-loss produces the
// configured quote-currency loss, excluding fees and slippage.
type FixedRiskSizer struct {
	risk float64
}

// NewFixedRiskSizer creates a sizer using the supplied quote-currency risk.
func NewFixedRiskSizer(risk float64) (FixedRiskSizer, error) {
	if math.IsNaN(risk) || math.IsInf(risk, 0) || risk <= 0 {
		return FixedRiskSizer{}, errors.ErrInvalidAmount
	}

	return FixedRiskSizer{risk: risk}, nil
}

// CalculatePositionSize divides the configured risk by the absolute distance
// between the position's actual entry and stop-loss prices.
func (s FixedRiskSizer) CalculatePositionSize(position types.Position) (float64, error) {
	if math.IsNaN(s.risk) || math.IsInf(s.risk, 0) || s.risk <= 0 {
		return 0, errors.ErrInvalidAmount
	}
	if math.IsNaN(position.EntryPrice) || math.IsInf(position.EntryPrice, 0) ||
		position.EntryPrice <= 0 || math.IsNaN(position.StopLoss) ||
		math.IsInf(position.StopLoss, 0) || position.StopLoss <= 0 {
		return 0, errors.ErrInvalidPrice
	}

	stopDistance := math.Abs(position.EntryPrice - position.StopLoss)
	if stopDistance == 0 {
		return 0, errors.ErrInvalidEntryPlanBracket
	}

	amount := s.risk / stopDistance
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return 0, errors.ErrInvalidAmount
	}

	return amount, nil
}
