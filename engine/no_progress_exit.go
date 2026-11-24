package engine

import "github.com/yusufozmis/go-trading-engine/types"

// updateNoProgressTarget latches price progress, not current profit. Fees and
// later retracements cannot erase a milestone already observed by the engine.
func (eng *Engine) updateNoProgressTarget(position *types.Position, candle types.Candle) {
	if eng.noProgressDuration == 0 || position.NoProgressTargetReached {
		return
	}
	target := position.EntryPrice + (position.TP-position.EntryPrice)*eng.noProgressPercentage
	switch position.Side {
	case types.PositionLong:
		if position.TP > position.EntryPrice && candle.PriceData.HighPrice >= target {
			position.NoProgressTargetReached = true
		}
	case types.PositionShort:
		if position.TP > 0 && position.TP < position.EntryPrice && candle.PriceData.LowPrice <= target {
			position.NoProgressTargetReached = true
		}
	}
}

func (eng *Engine) noProgressCloseAction(current *types.Position, candle types.Candle) *ClosePositionAction {
	if eng.noProgressDuration == 0 || current.NoProgressTargetReached ||
		candle.Timestamp-current.OpenTimestamp < eng.noProgressDuration.Milliseconds() {
		return nil
	}
	// A missing or non-profitable TP cannot define the required progress distance.
	if (current.Side == types.PositionLong && current.TP <= current.EntryPrice) ||
		(current.Side == types.PositionShort && (current.TP <= 0 || current.TP >= current.EntryPrice)) {
		return nil
	}
	position := *current
	position.State = types.ClosedByNoProgress
	position.CloseTimestamp = candle.Timestamp
	position.ExitPrice = eng.exitFillPrice(candle.PriceData.ClosePrice, position.Side)
	return &ClosePositionAction{Position: position}
}
