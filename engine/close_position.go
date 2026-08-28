package engine

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// DecideClosePosition evaluates whether the current candle closes the tracked
// position without mutating engine state.
func (eng *Engine) DecideClosePosition(candle types.Candle) (*ClosePositionAction, error) {
	if err := eng.validateCandle(candle); err != nil {
		return nil, err
	}
	if !eng.PositionExists() || candle.Timestamp <= eng.lastPosition.Timestamp {
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
		return errors.ErrStalePositionAction
	}

	currentPosition := eng.lastPosition
	closedPosition := action.Position
	expectedSide := types.PositionLong
	if currentPosition.State == types.SHORT_OPEN {
		expectedSide = types.PositionShort
	}
	if action.Side != expectedSide {
		return errors.ErrInvalidPositionAction
	}

	if currentPosition.Symbol != closedPosition.Symbol ||
		currentPosition.Timeframe != closedPosition.Timeframe ||
		currentPosition.Timestamp != closedPosition.Timestamp ||
		currentPosition.EntryPrice != closedPosition.EntryPrice ||
		currentPosition.Amount != closedPosition.Amount {
		return errors.ErrStalePositionAction
	}

	var closePrice float64
	switch closedPosition.State {
	case types.ClosedByProfit:
		closePrice = closedPosition.TP
		if closedPosition.StopLoss != currentPosition.StopLoss {
			return errors.ErrInvalidPositionAction
		}
	case types.ClosedByStop:
		closePrice = closedPosition.StopLoss
		if closedPosition.TP != currentPosition.TP {
			return errors.ErrInvalidPositionAction
		}
	default:
		return errors.ErrInvalidPositionAction
	}
	if math.IsNaN(closePrice) || math.IsInf(closePrice, 0) || closePrice <= 0 {
		return errors.ErrInvalidPositionAction
	}

	eng.lastPosition = &closedPosition
	eng.closedPositions = append(eng.closedPositions, closedPosition)
	return nil
}

func (eng *Engine) automatedCloseAction(candle types.Candle) *ClosePositionAction {
	position := *eng.lastPosition
	side := types.PositionLong
	if position.State == types.SHORT_OPEN {
		side = types.PositionShort
	}

	isTP := position.TP <= candle.PriceData.HighPrice && position.TP >= candle.PriceData.LowPrice
	isSL := position.StopLoss <= candle.PriceData.HighPrice && position.StopLoss >= candle.PriceData.LowPrice

	// When both thresholds occur within one candle, preserve the existing
	// conservative backtest rule and assume that the stop was reached first.
	if isSL {
		position.State = types.ClosedByStop
		return &ClosePositionAction{
			Position: position,
			Side:     side,
		}
	}
	if isTP {
		position.State = types.ClosedByProfit
		return &ClosePositionAction{
			Position: position,
			Side:     side,
		}
	}

	return nil
}

func (eng *Engine) candleCloseAction(candle types.Candle) *ClosePositionAction {
	position := *eng.lastPosition
	closePrice := candle.PriceData.ClosePrice

	switch position.State {
	case types.LONG_OPEN:
		if closePrice <= position.StopLoss {
			position.State = types.ClosedByStop
			position.StopLoss = closePrice
			return &ClosePositionAction{
				Position: position,
				Side:     types.PositionLong,
			}
		}
		if closePrice >= position.TP {
			position.State = types.ClosedByProfit
			position.TP = closePrice
			return &ClosePositionAction{
				Position: position,
				Side:     types.PositionLong,
			}
		}
	case types.SHORT_OPEN:
		if closePrice >= position.StopLoss {
			position.State = types.ClosedByStop
			position.StopLoss = closePrice
			return &ClosePositionAction{
				Position: position,
				Side:     types.PositionShort,
			}
		}
		if closePrice <= position.TP {
			position.State = types.ClosedByProfit
			position.TP = closePrice
			return &ClosePositionAction{
				Position: position,
				Side:     types.PositionShort,
			}
		}
	}

	return nil
}
