// Package positionsizers provides ready-to-use position sizing policies for an engine.
package positionsizers

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// FixedNotionalSizer converts a fixed quote-currency value into a base-asset
// amount using the position's actual entry price.
type FixedNotionalSizer struct {
	notional float64
}

// NewFixedNotionalSizer creates a sizer that allocates the given quote-currency
// value to every position. For example, a notional of 10 at an entry price of
// 10 produces an amount of 1.
func NewFixedNotionalSizer(notional float64) (FixedNotionalSizer, error) {
	if math.IsNaN(notional) || math.IsInf(notional, 0) || notional <= 0 {
		return FixedNotionalSizer{}, errors.ErrInvalidAmount
	}

	return FixedNotionalSizer{notional: notional}, nil
}

// CalculatePositionSize returns the fixed notional divided by the actual entry price.
func (s FixedNotionalSizer) CalculatePositionSize(position types.Position) (float64, error) {
	if math.IsNaN(s.notional) || math.IsInf(s.notional, 0) || s.notional <= 0 {
		return 0, errors.ErrInvalidAmount
	}
	if math.IsNaN(position.EntryPrice) || math.IsInf(position.EntryPrice, 0) ||
		position.EntryPrice <= 0 {
		return 0, errors.ErrInvalidPrice
	}

	return s.notional / position.EntryPrice, nil
}
