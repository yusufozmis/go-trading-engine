package engine

import (
	"fmt"
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// IncreasePositionAction describes the resulting state after adding to an open
// side. Live callers should execute Amount - Position.Amount on the exchange,
// then confirm using the actual total amount and average entry price.
type IncreasePositionAction struct {
	// Position is the original open snapshot used to reject stale confirmations.
	Position types.Position
	// Amount is the new total base-asset amount after the increase.
	Amount float64
	// EntryPrice is the new average entry price of the entire remaining position,
	// not the fill price of the additional entry.
	EntryPrice float64
	// Timestamp is the additional entry's Unix timestamp in milliseconds.
	Timestamp int64
}

// DecideIncreasePosition prepares an increase to newAmount without changing
// engine state. It estimates the resulting average entry using the candle close
// and configured slippage for the added quantity. The caller supplies the new
// total base-asset amount; the position sizer is not used. Liquidation is
// calculated only on confirmation, using the final amount and average entry.
func (eng *Engine) DecideIncreasePosition(
	candle types.Candle,
	side types.PositionSide,
	newAmount float64,
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
		Amount:     newAmount,
		EntryPrice: eng.entryFillPrice(candle.PriceData.ClosePrice, side),
		Timestamp:  candle.Timestamp,
	}
	if err := eng.validateIncreasePosition(action); err != nil {
		return nil, err
	}
	addedAmount := newAmount - position.Amount
	action.EntryPrice = position.EntryPrice +
		(action.EntryPrice-position.EntryPrice)*(addedAmount/newAmount)
	return &action, nil
}

// ConfirmIncreasePosition records the new total amount and average entry price
// reported by the action. TP/SL, the original opening time, and realized
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
	addedAmount := action.Amount - position.Amount
	// Infer the added fill from the old and new average entries for slippage
	// reporting. Do not average the reported average entry a second time.
	addedEntryPrice := action.EntryPrice +
		((action.EntryPrice-position.EntryPrice)/addedAmount)*position.Amount
	if math.IsNaN(addedEntryPrice) || math.IsInf(addedEntryPrice, 0) || addedEntryPrice <= 0 {
		return apperrors.ErrInvalidPrice
	}
	position.Amount = action.Amount
	position.EntryPrice = action.EntryPrice
	position.SlippageCost += eng.entrySlippageCost(addedEntryPrice, addedAmount, position.Side)
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
	if math.IsNaN(action.Amount) || math.IsInf(action.Amount, 0) || action.Amount <= current.Amount {
		return apperrors.ErrInvalidAmount
	}
	if math.IsNaN(action.EntryPrice) || math.IsInf(action.EntryPrice, 0) || action.EntryPrice <= 0 {
		return apperrors.ErrInvalidPrice
	}
	if action.Timestamp <= current.OpenTimestamp || action.Timestamp < current.PartialTPTimestamp {
		return apperrors.ErrInvalidTimestamp
	}

	if eng.liquidationModel == nil && (current.Leverage != 0 || current.LiquidationPrice != 0) {
		return fmt.Errorf("increase leveraged position: %w", apperrors.ErrNilLiquidationModel)
	}
	return nil
}

// DirectAmountIncrease immediately records the new total base-asset amount and
// average entry price on the specified open side without a separate
// decide/confirm step. The average entry is stored as supplied, without applying
// slippage again. Timestamp is the increase's Unix timestamp in milliseconds.
// It sends no exchange order. Not recommended for production unless the caller
// has already verified execution, since it updates engine state immediately.
func (eng *Engine) DirectAmountIncrease(
	side types.PositionSide,
	averageEntryPrice, newAmount float64,
	timestamp int64,
) error {
	if err := eng.Validate(); err != nil {
		return err
	}
	if !side.Valid() {
		return apperrors.ErrInvalidSide
	}
	position := eng.positions[side]
	if position == nil || position.State != types.PositionOpen {
		return apperrors.ErrPositionNotFound
	}

	return eng.ConfirmIncreasePosition(IncreasePositionAction{
		Position:   *position,
		Amount:     newAmount,
		EntryPrice: averageEntryPrice,
		Timestamp:  timestamp,
	})
}
