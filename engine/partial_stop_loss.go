package engine

import "github.com/yusufozmis/go-trading-engine/types"

// partialStopLossTrigger checks the current loss-side stop. Stops moved to
// break-even or into profit must not turn this loss-reduction rule into a TP.
func (eng *Engine) partialStopLossTrigger(position types.Position, candle types.Candle) (float64, bool) {
	if eng.partialStopLossPercentage == 0 || position.PartialStopLossExecuted || position.StopLoss <= 0 {
		return 0, false
	}
	trigger := position.EntryPrice + (position.StopLoss-position.EntryPrice)*eng.partialStopLossPercentage
	price := candle.PriceData.ClosePrice
	switch position.Side {
	case types.PositionLong:
		if position.StopLoss >= position.EntryPrice {
			return 0, false
		}
		if eng.isCloseAutomated {
			// A TP reached at the open closes before any later intrabar loss.
			if position.TP > 0 && candle.PriceData.OpenPrice >= position.TP {
				return 0, false
			}
			price = candle.PriceData.LowPrice
		}
		return trigger, price <= trigger
	case types.PositionShort:
		if position.StopLoss <= position.EntryPrice {
			return 0, false
		}
		if eng.isCloseAutomated {
			if position.TP > 0 && candle.PriceData.OpenPrice <= position.TP {
				return 0, false
			}
			price = candle.PriceData.HighPrice
		}
		return trigger, price >= trigger
	default:
		return 0, false
	}
}
