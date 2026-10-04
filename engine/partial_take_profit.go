package engine

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// DecidePartialTakeProfits returns one reduction action for each eligible open
// side without mutating position state. Live callers should first execute and
// confirm actions returned by DecideClosePositions so a full stop, TP,
// liquidation, or duration close takes priority on the candle.
func (eng *Engine) DecidePartialTakeProfits(
	candle types.Candle,
) ([]PartialTakeProfitAction, error) {
	if err := eng.validateCandle(candle); err != nil {
		return nil, err
	}
	if eng.partialTakeProfitPercentage == 0 {
		return nil, nil
	}

	actions := make([]PartialTakeProfitAction, 0, 2)
	for _, side := range positionSides() {
		position := eng.positions[side]
		if position == nil || position.State != types.PositionOpen ||
			position.PartialTakeProfitExecuted ||
			candle.Timestamp <= position.OpenTimestamp {
			continue
		}

		triggerPrice, err := eng.partialTakeProfitTrigger(*position)
		if err != nil {
			return nil, err
		}
		if !eng.partialTakeProfitReached(*position, candle, triggerPrice) {
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

		actions = append(actions, PartialTakeProfitAction{
			Position:  *position,
			Amount:    reductionAmount,
			ExitPrice: exitPrice,
			Timestamp: candle.Timestamp,
		})
	}

	return actions, nil
}

// ConfirmPartialTakeProfit records a successfully executed reduction, realizes
// its fee and PnL, and leaves the remaining amount open.
func (eng *Engine) ConfirmPartialTakeProfit(action PartialTakeProfitAction) error {
	if err := eng.validate(); err != nil {
		return err
	}

	current := eng.positions[action.Position.Side]
	if current == nil || current.State != types.PositionOpen ||
		current.PartialTakeProfitExecuted {
		return apperrors.ErrStalePositionAction
	}
	if *current != action.Position {
		return apperrors.ErrStalePositionAction
	}

	expectedAmount := current.Amount * eng.partialReductionPercentage
	if action.Amount != expectedAmount || !validPartialValue(action.Amount) ||
		!validPartialValue(action.ExitPrice) ||
		action.Timestamp <= current.OpenTimestamp {
		return apperrors.ErrInvalidPositionAction
	}

	remainingAmount := current.Amount - action.Amount
	if !validPartialValue(remainingAmount) {
		return apperrors.ErrInvalidAmount
	}

	// A partial take-profit realizes only the reduced amount while the position
	// itself remains open and accumulates the resulting accounting values.
	entryFee := current.EntryPrice * action.Amount * eng.tradingFeeRate
	exitFee := action.ExitPrice * action.Amount * eng.tradingFeeRate
	partialFee := entryFee + exitFee

	exitSlippage := eng.exitSlippageCost(
		action.ExitPrice,
		action.Amount,
		current.Side,
	)

	var partialNetProfit float64
	switch current.Side {
	case types.PositionLong:
		partialNetProfit = (action.ExitPrice - current.EntryPrice) * action.Amount
	case types.PositionShort:
		partialNetProfit = (current.EntryPrice - action.ExitPrice) * action.Amount
	default:
		return apperrors.ErrInvalidSide
	}
	partialNetProfit -= partialFee

	if math.IsNaN(partialFee) || math.IsInf(partialFee, 0) {
		return apperrors.ErrInvalidFee
	}
	if math.IsNaN(exitSlippage) || math.IsInf(exitSlippage, 0) {
		return apperrors.ErrInvalidSlippageCost
	}
	if math.IsNaN(partialNetProfit) || math.IsInf(partialNetProfit, 0) {
		return apperrors.ErrInvalidNetProfit
	}

	remaining := *current
	remaining.Amount = remainingAmount
	remaining.PartialTakeProfitExecuted = true
	remaining.PartialTPTimestamp = action.Timestamp
	// Keep cumulative realized accounting on the open position. The original
	// entry slippage already represents the whole entry, so only the reduction's
	// exit slippage is added here.
	remaining.Fee += partialFee
	remaining.NetProfit += partialNetProfit
	remaining.SlippageCost += exitSlippage
	if err := remaining.Validate(); err != nil {
		return apperrors.ErrInvalidPositionAction
	}

	eng.positions[remaining.Side] = &remaining

	eng.realizedMu.Lock()
	// Performance reads this snapshot instead of the mutable positions map. It
	// is removed after the final close because closedPositions then contains the
	// same cumulative accounting values.
	eng.partiallyRealizedPositions[remaining.Side] = remaining
	eng.realizedMu.Unlock()
	return nil
}

func (eng *Engine) partialTakeProfitTrigger(position types.Position) (float64, error) {
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

func (eng *Engine) partialTakeProfitReached(
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
