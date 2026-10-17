package engine

import (
	"fmt"
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// IncreasePositionAction describes an additional entry on an already open side.
// Live callers should execute Amount on the exchange, replace EntryPrice and
// Timestamp with the actual fill details, and confirm only after execution.
type IncreasePositionAction struct {
	// Position is the original open snapshot used to reject stale confirmations.
	Position types.Position
	// Amount is the additional base-asset amount, not the resulting total.
	Amount float64
	// EntryPrice is the additional entry fill, including simulated slippage.
	EntryPrice float64
	// Timestamp is the additional entry's Unix timestamp in milliseconds.
	Timestamp int64
}

// DecideIncreasePosition prepares an explicit increase using the candle close
// and configured slippage without changing engine state. The caller chooses
// when to increase and supplies the additional amount; the sizer is not used.
// The resulting average entry and liquidation price are calculated and
// validated only on confirmation, using the final execution details.
func (eng *Engine) DecideIncreasePosition(
	candle types.Candle,
	side types.PositionSide,
	amount float64,
) (*IncreasePositionAction, error) {
	if err := eng.ValidateCandle(candle); err != nil {
		return nil, err
	}
	if !side.Valid() {
		return nil, apperrors.ErrInvalidSide
	}
	position := eng.positions[side]
	if position == nil || position.State != types.PositionOpen {
		return nil, apperrors.ErrPositionNotFound
	}

	action := IncreasePositionAction{
		Position:   *position,
		Amount:     amount,
		EntryPrice: eng.entryFillPrice(candle.PriceData.ClosePrice, side),
		Timestamp:  candle.Timestamp,
	}
	if err := eng.validateIncreasePosition(action); err != nil {
		return nil, err
	}
	return &action, nil
}

// ConfirmIncreasePosition adds an executed entry to the tracked side using a
// weighted average entry price. TP/SL, the original opening time, and realized
// accounting are preserved; an executed partial TP is not rearmed. Liquidation
// is recalculated when configured. Preloaded liquidation data requires a model
// so an increase cannot silently retain an outdated liquidation price.
func (eng *Engine) ConfirmIncreasePosition(action IncreasePositionAction) error {
	if err := eng.Validate(); err != nil {
		return err
	}
	if err := eng.validateIncreasePosition(action); err != nil {
		return err
	}

	position := action.Position
	position.Amount += action.Amount
	// This is the amount-weighted average without multiplying large notionals.
	position.EntryPrice += (action.EntryPrice - position.EntryPrice) * (action.Amount / position.Amount)
	position.SlippageCost += eng.entrySlippageCost(action.EntryPrice, action.Amount, position.Side)
	// Entry fees remain deferred until reduction or close, where the weighted
	// entry accounts for both the existing amount and this additional entry.
	if eng.liquidationModel != nil {
		if err := eng.applyLiquidationModel(&position); err != nil {
			return err
		}
	}
	if err := position.Validate(); err != nil {
		return err
	}

	eng.positions[position.Side] = &position
	return nil
}

func (eng *Engine) validateIncreasePosition(action IncreasePositionAction) error {
	if !action.Position.Side.Valid() {
		return apperrors.ErrInvalidSide
	}
	if !eng.isPositionSideAllowed(action.Position.Side) {
		return apperrors.ErrInvalidPositionAction
	}
	current := eng.positions[action.Position.Side]
	if current == nil || current.State != types.PositionOpen || *current != action.Position {
		return apperrors.ErrStalePositionAction
	}
	if math.IsNaN(action.Amount) || math.IsInf(action.Amount, 0) || action.Amount <= 0 {
		return apperrors.ErrInvalidAmount
	}
	if math.IsNaN(action.EntryPrice) || math.IsInf(action.EntryPrice, 0) || action.EntryPrice <= 0 {
		return apperrors.ErrInvalidPrice
	}
	if action.Timestamp <= current.OpenTimestamp || action.Timestamp < current.PartialTPTimestamp {
		return apperrors.ErrInvalidTimestamp
	}

	totalAmount := current.Amount + action.Amount
	if math.IsInf(totalAmount, 0) || totalAmount <= current.Amount {
		return apperrors.ErrInvalidAmount
	}
	if eng.liquidationModel == nil && (current.Leverage != 0 || current.LiquidationPrice != 0) {
		return fmt.Errorf("increase leveraged position: %w", apperrors.ErrNilLiquidationModel)
	}
	return nil
}
