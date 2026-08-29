package positionsizers

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// FixedAmountSizer returns the same base-asset amount for every position.
type FixedAmountSizer struct {
	amount float64
}

// NewFixedAmountSizer creates a sizer using the supplied base-asset amount.
func NewFixedAmountSizer(amount float64) (FixedAmountSizer, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return FixedAmountSizer{}, apperrors.ErrInvalidAmount
	}

	return FixedAmountSizer{amount: amount}, nil
}

// CalculatePositionSize returns the configured base-asset amount.
func (s FixedAmountSizer) CalculatePositionSize(_ types.Position) (float64, error) {
	if math.IsNaN(s.amount) || math.IsInf(s.amount, 0) || s.amount <= 0 {
		return 0, apperrors.ErrInvalidAmount
	}

	return s.amount, nil
}
