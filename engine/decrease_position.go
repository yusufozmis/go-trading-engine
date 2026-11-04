package engine

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// DecreasePositionAction describes an executed reduction of an open position.
// It can be confirmed repeatedly using a fresh position snapshot each time.
type DecreasePositionAction struct {
	// Position is the original open snapshot used to reject stale confirmations.
	Position types.Position
	// Amount is the new remaining base-asset amount, not the reduced quantity.
	// It must remain positive; a complete close uses ConfirmClosePosition.
	Amount float64
	// ExitPrice is the reduced quantity's fill price (or its estimated average),
	// not the remaining position's entry price. Slippage is not applied again.
	ExitPrice float64
	// Timestamp is the reduction's Unix timestamp in milliseconds.
	Timestamp int64
	// Set only by DecideDecreasePositions for the one-shot partial-TP option.
	partialTakeProfit bool
}

// DecideDecreasePositions evaluates automatic reduction rules without mutating
// position state. WithPartialTakeProfit produces at most one reduction per
// position. Confirm the returned action with ConfirmDecreasePosition after
// execution so the one-shot option is consumed on success. External reductions may
// instead construct DecreasePositionAction directly without calling this method.
// Full closes from DecideClosePositions must be confirmed first so they take
// priority over reductions on the same candle.
func (eng *Engine) DecideDecreasePositions(
	candle types.Candle,
) ([]DecreasePositionAction, error) {
	if err := eng.validateCandle(candle); err != nil {
		return nil, err
	}
	if eng.partialTakeProfitPercentage == 0 {
		return nil, nil
	}

	actions := make([]DecreasePositionAction, 0, 2)
	for _, side := range positionSides() {
		position := eng.positions[side]
		if position == nil || position.State != types.PositionOpen ||
			position.PartialTakeProfitExecuted ||
			candle.Timestamp <= position.OpenTimestamp {
			continue
		}

		triggerPrice, err := eng.decreaseTriggerPrice(*position)
		if err != nil {
			return nil, err
		}
		if !eng.decreaseTriggerReached(*position, candle, triggerPrice) {
			continue
		}

		reductionAmount := position.Amount * eng.partialReductionPercentage
		remainingAmount := position.Amount - reductionAmount
		if !validPartialValue(reductionAmount) || !validPartialValue(remainingAmount) {
			return nil, apperrors.ErrInvalidAmount
		}

		exitPrice := triggerPrice
		if !eng.isCloseAutomated {
			exitPrice = candle.PriceData.ClosePrice
		}
		exitPrice = eng.exitFillPrice(exitPrice, position.Side)

		actions = append(actions, DecreasePositionAction{
			Position:          *position,
			Amount:            remainingAmount,
			ExitPrice:         exitPrice,
			Timestamp:         candle.Timestamp,
			partialTakeProfit: true,
		})
	}

	return actions, nil
}

// ConfirmDecreasePosition records an executed reduction without sending
// an exchange order. It realizes the difference between the tracked amount and
// action.Amount, accumulating PnL, configured entry/exit fees, and exit slippage
// cost. Each reduction increments PartialTakeProfitCount or PartialLoseCount
// according to its net PnL after fees; zero net PnL increments neither.
// Entry price, TP/SL, and opening time are preserved. An action returned by
// DecideDecreasePositions consumes the one-shot partial-TP option on success;
// externally constructed actions neither consume nor rearm that option.
func (eng *Engine) ConfirmDecreasePosition(action DecreasePositionAction) error {
	if err := eng.Validate(); err != nil {
		return err
	}
	remaining, err := eng.decreasedPosition(action)
	if err != nil {
		return err
	}
	eng.positions[remaining.Side] = &remaining
	eng.realizedMu.Lock()
	eng.partiallyRealizedPositions[remaining.Side] = remaining
	eng.realizedMu.Unlock()
	return nil
}

// decreasedPosition validates and calculates a reduction without committing it.
func (eng *Engine) decreasedPosition(action DecreasePositionAction) (types.Position, error) {
	if !action.Position.Side.Valid() {
		return types.Position{}, apperrors.ErrInvalidSide
	}
	current := eng.positions[action.Position.Side]
	if current == nil || current.State != types.PositionOpen || *current != action.Position ||
		(action.partialTakeProfit && current.PartialTakeProfitExecuted) {
		return types.Position{}, apperrors.ErrStalePositionAction
	}
	if !validPartialValue(action.Amount) || action.Amount >= current.Amount {
		return types.Position{}, apperrors.ErrInvalidAmount
	}
	if !validPartialValue(action.ExitPrice) {
		return types.Position{}, apperrors.ErrInvalidPrice
	}
	if action.Timestamp <= current.OpenTimestamp || action.Timestamp < current.PartialTPTimestamp {
		return types.Position{}, apperrors.ErrInvalidTimestamp
	}

	reductionAmount := current.Amount - action.Amount
	if !validPartialValue(reductionAmount) {
		return types.Position{}, apperrors.ErrInvalidAmount
	}
	entryFee := current.EntryPrice * reductionAmount * eng.tradingFeeRate
	exitFee := action.ExitPrice * reductionAmount * eng.tradingFeeRate
	reductionFee := entryFee + exitFee
	exitSlippage := eng.exitSlippageCost(action.ExitPrice, reductionAmount, current.Side)

	var reductionNetProfit float64
	switch current.Side {
	case types.PositionLong:
		reductionNetProfit = (action.ExitPrice - current.EntryPrice) * reductionAmount
	case types.PositionShort:
		reductionNetProfit = (current.EntryPrice - action.ExitPrice) * reductionAmount
	}
	reductionNetProfit -= reductionFee
	if math.IsNaN(reductionFee) || math.IsInf(reductionFee, 0) {
		return types.Position{}, apperrors.ErrInvalidFee
	}
	if math.IsNaN(exitSlippage) || math.IsInf(exitSlippage, 0) {
		return types.Position{}, apperrors.ErrInvalidSlippageCost
	}
	if math.IsNaN(reductionNetProfit) || math.IsInf(reductionNetProfit, 0) {
		return types.Position{}, apperrors.ErrInvalidNetProfit
	}

	remaining := *current
	remaining.Amount = action.Amount
	remaining.Fee += reductionFee
	remaining.NetProfit += reductionNetProfit
	if reductionNetProfit > 0 {
		remaining.PartialTakeProfitCount++
	} else if reductionNetProfit < 0 {
		remaining.PartialLoseCount++
	}
	if action.partialTakeProfit {
		remaining.PartialTakeProfitExecuted = true
		remaining.PartialTPTimestamp = action.Timestamp
	}
	// Entry slippage already represents the full entry; add only this exit.
	remaining.SlippageCost += exitSlippage
	if err := remaining.Validate(); err != nil {
		return types.Position{}, apperrors.ErrInvalidPositionAction
	}
	return remaining, nil
}

// decreaseTriggerPrice locates the configured trigger along the entry-to-TP distance.
func (eng *Engine) decreaseTriggerPrice(position types.Position) (float64, error) {
	switch position.Side {
	case types.PositionLong:
		if position.TP <= position.EntryPrice {
			return 0, apperrors.ErrInvalidTPSLBracket
		}
		return position.EntryPrice +
			((position.TP - position.EntryPrice) * eng.partialTakeProfitPercentage), nil
	case types.PositionShort:
		if position.TP <= 0 || position.TP >= position.EntryPrice {
			return 0, apperrors.ErrInvalidTPSLBracket
		}
		return position.EntryPrice -
			((position.EntryPrice - position.TP) * eng.partialTakeProfitPercentage), nil
	default:
		return 0, apperrors.ErrInvalidSide
	}
}

// decreaseTriggerReached uses intrabar extremes for automated closes and the
// candle close otherwise, matching the engine's configured execution policy.
func (eng *Engine) decreaseTriggerReached(
	position types.Position,
	candle types.Candle,
	triggerPrice float64,
) bool {
	price := candle.PriceData.ClosePrice
	if eng.isCloseAutomated {
		if position.Side == types.PositionLong {
			price = candle.PriceData.HighPrice
		} else {
			price = candle.PriceData.LowPrice
		}
	}

	switch position.Side {
	case types.PositionLong:
		return price >= triggerPrice
	case types.PositionShort:
		return price <= triggerPrice
	default:
		return false
	}
}

func validPartialValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}
