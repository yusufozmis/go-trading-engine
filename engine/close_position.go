package engine

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// DecideClosePosition evaluates whether the current candle closes the tracked
// position. When break-even is configured, a candle that reaches its trigger
// can update the tracked stop for subsequent candles without producing a close.
func (eng *Engine) DecideClosePosition(candle types.Candle) (*ClosePositionAction, error) {
	if err := eng.validateCandle(candle); err != nil {
		return nil, err
	}
	if !eng.PositionExists() || candle.Timestamp <= eng.lastPosition.OpenTimestamp {
		return nil, nil
	}
	liquidationAction := eng.liquidationCloseAction(candle)
	if liquidationAction != nil && eng.liquidationHasPriority(candle) {
		return liquidationAction, nil
	}

	if eng.isCloseAutomated {
		if action := eng.automatedCloseAction(candle); action != nil {
			return action, nil
		}
	} else if action := eng.candleCloseAction(candle); action != nil {
		return action, nil
	}
	if liquidationAction != nil {
		return liquidationAction, nil
	}

	eng.applyBreakEvenStop(candle)

	return eng.maximumDurationCloseAction(candle), nil
}

func (eng *Engine) liquidationHasPriority(candle types.Candle) bool {
	position := eng.lastPosition

	// Opening beyond the liquidation level represents a gap that bypassed any
	// protective stop available inside the candle's range.
	switch position.Side {
	case types.PositionLong:
		if candle.PriceData.OpenPrice <= position.LiquidationPrice {
			return true
		}
	case types.PositionShort:
		if candle.PriceData.OpenPrice >= position.LiquidationPrice {
			return true
		}
	}
	// Without an attached stop, the engine acts only on the candle close. If the
	// candle touched liquidation first, a later close beyond SL cannot save it.
	if !eng.isCloseAutomated {
		return true
	}
	// An attached stop has already triggered when the candle opens beyond it.
	// This known event precedes any later TP or liquidation touch in the candle.
	if eng.stopReachedAtOpen(candle) {
		return false
	}
	// An attached TP has already triggered when the candle opens beyond it, so
	// later movement to the liquidation level cannot replace that known outcome.
	if eng.takeProfitReachedAtOpen(candle) {
		return false
	}

	// When TP and liquidation are both touched, OHLC data cannot establish their
	// intrabar order. Liquidation is the conservative outcome.
	if eng.takeProfitTouched(candle) {
		return true
	}

	stopProtects := false
	switch position.Side {
	case types.PositionLong:
		stopProtects = position.StopLoss > position.LiquidationPrice
	case types.PositionShort:
		stopProtects = position.StopLoss < position.LiquidationPrice
	}

	return !stopProtects || !eng.stopTouched(candle)
}

func (eng *Engine) stopTouched(candle types.Candle) bool {
	position := eng.lastPosition
	if eng.isCloseAutomated {
		switch position.Side {
		case types.PositionLong:
			return candle.PriceData.LowPrice <= position.StopLoss
		case types.PositionShort:
			return candle.PriceData.HighPrice >= position.StopLoss
		default:
			return false
		}
	}

	switch position.Side {
	case types.PositionLong:
		return candle.PriceData.ClosePrice <= position.StopLoss
	case types.PositionShort:
		return candle.PriceData.ClosePrice >= position.StopLoss
	default:
		return false
	}
}

func (eng *Engine) stopReachedAtOpen(candle types.Candle) bool {
	switch eng.lastPosition.Side {
	case types.PositionLong:
		return candle.PriceData.OpenPrice <= eng.lastPosition.StopLoss
	case types.PositionShort:
		return candle.PriceData.OpenPrice >= eng.lastPosition.StopLoss
	default:
		return false
	}
}

func (eng *Engine) takeProfitTouched(candle types.Candle) bool {
	switch eng.lastPosition.Side {
	case types.PositionLong:
		return candle.PriceData.HighPrice >= eng.lastPosition.TP
	case types.PositionShort:
		return candle.PriceData.LowPrice <= eng.lastPosition.TP
	default:
		return false
	}
}

func (eng *Engine) takeProfitReachedAtOpen(candle types.Candle) bool {
	switch eng.lastPosition.Side {
	case types.PositionLong:
		return candle.PriceData.OpenPrice >= eng.lastPosition.TP
	case types.PositionShort:
		return candle.PriceData.OpenPrice <= eng.lastPosition.TP
	default:
		return false
	}
}

func (eng *Engine) applyBreakEvenStop(candle types.Candle) {
	if eng.breakEvenStopRate == 0 ||
		eng.lastPosition.StopLoss == eng.lastPosition.EntryPrice {
		return
	}

	position := eng.lastPosition
	entry := position.EntryPrice

	triggered := false
	switch position.Side {
	case types.PositionLong:
		triggerPrice := entry + ((position.TP - entry) * eng.breakEvenStopRate)
		triggered = candle.PriceData.HighPrice >= triggerPrice
	case types.PositionShort:
		triggerPrice := entry - ((entry - position.TP) * eng.breakEvenStopRate)
		triggered = candle.PriceData.LowPrice <= triggerPrice
	}

	// Close conditions were evaluated before this mutation. Therefore, even if
	// this candle also crossed entry, the break-even stop starts on the next candle.
	if triggered {
		eng.lastPosition.StopLoss = eng.lastPosition.EntryPrice
	}
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
		currentPosition.Amount != closedPosition.Amount ||
		currentPosition.Leverage != closedPosition.Leverage ||
		currentPosition.LiquidationPrice != closedPosition.LiquidationPrice {
		return apperrors.ErrStalePositionAction
	}

	entryFee := closedPosition.EntryPrice * closedPosition.Amount * eng.tradingFeeRate
	exitFee := closedPosition.ExitPrice * closedPosition.Amount * eng.tradingFeeRate
	closedPosition.Fee = entryFee + exitFee
	if math.IsNaN(closedPosition.Fee) || math.IsInf(closedPosition.Fee, 0) {
		return apperrors.ErrInvalidFee
	}
	closedPosition.SlippageCost = currentPosition.SlippageCost + eng.exitSlippageCost(
		closedPosition.ExitPrice,
		closedPosition.Amount,
		closedPosition.Side,
	)
	if math.IsNaN(closedPosition.SlippageCost) ||
		math.IsInf(closedPosition.SlippageCost, 0) {
		return apperrors.ErrInvalidSlippageCost
	}

	switch closedPosition.Side {
	case types.PositionLong:
		closedPosition.NetProfit = (closedPosition.ExitPrice - closedPosition.EntryPrice) *
			closedPosition.Amount
	case types.PositionShort:
		closedPosition.NetProfit = (closedPosition.EntryPrice - closedPosition.ExitPrice) *
			closedPosition.Amount
	default:
		return apperrors.ErrInvalidSide
	}
	closedPosition.NetProfit -= closedPosition.Fee
	if math.IsNaN(closedPosition.NetProfit) || math.IsInf(closedPosition.NetProfit, 0) {
		return apperrors.ErrInvalidNetProfit
	}

	eng.lastPosition = &closedPosition
	eng.closedPositions = append(eng.closedPositions, closedPosition)
	return nil
}

func (eng *Engine) liquidationCloseAction(candle types.Candle) *ClosePositionAction {
	position := *eng.lastPosition
	if position.LiquidationPrice == 0 {
		return nil
	}

	liquidated := false
	switch position.Side {
	case types.PositionLong:
		liquidated = candle.PriceData.LowPrice <= position.LiquidationPrice
	case types.PositionShort:
		liquidated = candle.PriceData.HighPrice >= position.LiquidationPrice
	}
	if !liquidated {
		return nil
	}

	position.State = types.ClosedByLiquidation
	position.CloseTimestamp = candle.Timestamp
	position.ExitPrice = eng.exitFillPrice(position.LiquidationPrice, position.Side)
	return &ClosePositionAction{Position: position}
}

func (eng *Engine) maximumDurationCloseAction(candle types.Candle) *ClosePositionAction {
	if eng.maximumPositionDuration == 0 {
		return nil
	}

	position := *eng.lastPosition
	maximumMilliseconds := eng.maximumPositionDuration.Milliseconds()
	if candle.Timestamp-position.OpenTimestamp < maximumMilliseconds {
		return nil
	}

	// Duration exits use the first available candle close after the configured
	// lifetime is reached because OHLC input has no finer execution timestamp.
	position.State = types.ClosedByDuration
	position.CloseTimestamp = candle.Timestamp
	position.ExitPrice = eng.exitFillPrice(candle.PriceData.ClosePrice, position.Side)
	return &ClosePositionAction{Position: position}
}

func (eng *Engine) automatedCloseAction(candle types.Candle) *ClosePositionAction {
	position := *eng.lastPosition

	isTP := eng.takeProfitTouched(candle)
	isSL := eng.stopTouched(candle)
	if eng.stopReachedAtOpen(candle) {
		position.State = types.ClosedByStop
		position.CloseTimestamp = candle.Timestamp
		position.ExitPrice = eng.exitFillPrice(candle.PriceData.OpenPrice, position.Side)
		return &ClosePositionAction{Position: position}
	}
	if eng.takeProfitReachedAtOpen(candle) {
		position.State = types.ClosedByProfit
		position.CloseTimestamp = candle.Timestamp
		position.ExitPrice = eng.exitFillPrice(position.TP, position.Side)
		return &ClosePositionAction{Position: position}
	}

	// When both thresholds occur within one candle, preserve the existing
	// conservative backtest rule and assume that the stop was reached first.
	if isSL {
		position.State = types.ClosedByStop
		position.CloseTimestamp = candle.Timestamp
		position.ExitPrice = eng.exitFillPrice(position.StopLoss, position.Side)
		return &ClosePositionAction{
			Position: position,
		}
	}
	if isTP {
		position.State = types.ClosedByProfit
		position.CloseTimestamp = candle.Timestamp
		position.ExitPrice = eng.exitFillPrice(position.TP, position.Side)
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
			position.ExitPrice = eng.exitFillPrice(closePrice, position.Side)
			return &ClosePositionAction{
				Position: position,
			}
		}
		if closePrice >= position.TP {
			position.State = types.ClosedByProfit
			position.CloseTimestamp = candle.Timestamp
			position.ExitPrice = eng.exitFillPrice(closePrice, position.Side)
			return &ClosePositionAction{
				Position: position,
			}
		}
	case types.PositionShort:
		if closePrice >= position.StopLoss {
			position.State = types.ClosedByStop
			position.CloseTimestamp = candle.Timestamp
			position.ExitPrice = eng.exitFillPrice(closePrice, position.Side)
			return &ClosePositionAction{
				Position: position,
			}
		}
		if closePrice <= position.TP {
			position.State = types.ClosedByProfit
			position.CloseTimestamp = candle.Timestamp
			position.ExitPrice = eng.exitFillPrice(closePrice, position.Side)
			return &ClosePositionAction{
				Position: position,
			}
		}
	}

	return nil
}
