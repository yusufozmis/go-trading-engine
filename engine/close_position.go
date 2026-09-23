package engine

import (
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// DecideClosePosition evaluates whether the current candle closes the tracked
// position without mutating engine state.
func (eng *Engine) DecideClosePosition(candle types.Candle) (*ClosePositionAction, error) {
	if err := eng.validateCandle(candle); err != nil {
		return nil, err
	}
	if !eng.PositionExists() || candle.Timestamp <= eng.lastPosition.OpenTimestamp {
		return nil, nil
	}

	if eng.isCloseAutomated {
		return eng.automatedCloseAction(candle), nil
	}

	return eng.candleCloseAction(candle), nil
}

// ConfirmClosePosition records a previously decided action as successfully
// closed. It rejects actions that no longer match the tracked open position.
func (eng *Engine) ConfirmClosePosition(action ClosePositionAction) error {
	if err := eng.validate(); err != nil {
		return err
	}
	if !eng.PositionExists() {
		return apperrors.ErrStalePositionAction
	}

	currentPosition := eng.lastPosition
	closedPosition := action.Position
	if err := closedPosition.Validate(); err != nil {
		return apperrors.ErrInvalidPositionAction
	}

	if currentPosition.Symbol != closedPosition.Symbol ||
		currentPosition.Timeframe != closedPosition.Timeframe ||
		currentPosition.OpenTimestamp != closedPosition.OpenTimestamp ||
		currentPosition.Side != closedPosition.Side ||
		currentPosition.EntryPrice != closedPosition.EntryPrice ||
		currentPosition.StopLoss != closedPosition.StopLoss ||
		currentPosition.TP != closedPosition.TP ||
		currentPosition.Amount != closedPosition.Amount {
		return apperrors.ErrStalePositionAction
	}

	eng.lastPosition = &closedPosition
	eng.closedPositions = append(eng.closedPositions, closedPosition)
	return nil
}

func (eng *Engine) automatedCloseAction(candle types.Candle) *ClosePositionAction {
	position := *eng.lastPosition

	isTP := position.TP <= candle.PriceData.HighPrice && position.TP >= candle.PriceData.LowPrice
	isSL := position.StopLoss <= candle.PriceData.HighPrice && position.StopLoss >= candle.PriceData.LowPrice

	// When both thresholds occur within one candle, preserve the existing
	// conservative backtest rule and assume that the stop was reached first.
	if isSL {
		position.State = types.ClosedByStop
		position.CloseTimestamp = candle.Timestamp
		position.ExitPrice = position.StopLoss
		return &ClosePositionAction{
			Position: position,
		}
	}
	if isTP {
		position.State = types.ClosedByProfit
		position.CloseTimestamp = candle.Timestamp
		position.ExitPrice = position.TP
		return &ClosePositionAction{
			Position: position,
		}
	}

	return nil
}

func (eng *Engine) candleCloseAction(candle types.Candle) *ClosePositionAction {
	position := *eng.lastPosition
	closePrice := candle.PriceData.ClosePrice

	// Preserve the configured TP/SL levels; ExitPrice records the simulated fill.
	switch position.Side {
	case types.PositionLong:
		if closePrice <= position.StopLoss {
			position.State = types.ClosedByStop
			position.CloseTimestamp = candle.Timestamp
			position.ExitPrice = closePrice
			return &ClosePositionAction{
				Position: position,
			}
		}
		if closePrice >= position.TP {
			position.State = types.ClosedByProfit
			position.CloseTimestamp = candle.Timestamp
			position.ExitPrice = closePrice
			return &ClosePositionAction{
				Position: position,
			}
		}
	case types.PositionShort:
		if closePrice >= position.StopLoss {
			position.State = types.ClosedByStop
			position.CloseTimestamp = candle.Timestamp
			position.ExitPrice = closePrice
			return &ClosePositionAction{
				Position: position,
			}
		}
		if closePrice <= position.TP {
			position.State = types.ClosedByProfit
			position.CloseTimestamp = candle.Timestamp
			position.ExitPrice = closePrice
			return &ClosePositionAction{
				Position: position,
			}
		}
	}

	return nil
}
