package engine

import "github.com/yusufozmis/go-trading-engine/types"

func (eng *Engine) ClosePosition(candle types.Candle) {

	if eng == nil {
		return
	}

	if eng.lastPosition == nil {
		return
	}

	if !eng.PositionExists() {
		return
	}

	if eng.lastPosition.Timestamp == candle.Timestamp {
		return
	}

	if eng.isCloseAutomated {
		eng.closeAutomated(candle)
	} else {
		eng.closeWithCandle(candle)
	}

}

func (eng *Engine) closeAutomated(candle types.Candle) {

	if eng == nil {
		return
	}

	if eng.lastPosition == nil {
		return
	}

	pos := eng.lastPosition
	if pos.State != types.LONG_OPEN && pos.State != types.SHORT_OPEN {
		return
	}

	// low price < TP < high price. Bunu kontrol etme sebebi TPnin gelip gelmediğini görmek.
	isTp := pos.TP <= candle.PriceData.HighPrice && pos.TP >= candle.PriceData.LowPrice

	// low price < StopLoss < high price. Bunu kontrol etme sebebi stop'un gelip gelmediğini görmek.
	isSL := pos.StopLoss <= candle.PriceData.HighPrice && pos.StopLoss >= candle.PriceData.LowPrice

	// Eğer TP ve SL fiyatlarının ikisi de mumun içerisindeyse (high'dan küçük lowdan büyük)
	// en kötüyü varsay ve database'e SL olarak geçir
	if isTp && isSL {
		eng.lastPosition.State = types.ClosedByStop

		eng.closedPositions = append(eng.closedPositions, *pos)

		return
	}
	if isTp {
		eng.lastPosition.State = types.ClosedByProfit

		eng.closedPositions = append(eng.closedPositions, *pos)

	}
	if isSL {

		eng.lastPosition.State = types.ClosedByStop

		eng.closedPositions = append(eng.closedPositions, *pos)

	}
}

func (eng *Engine) closeWithCandle(candle types.Candle) {

	if eng == nil {
		return
	}

	if eng.lastPosition == nil {
		return
	}

	pos := eng.lastPosition

	if pos.State == types.LONG_OPEN {

		isTP := candle.PriceData.ClosePrice >= pos.TP

		if candle.PriceData.ClosePrice <= pos.StopLoss {

			eng.lastPosition.State = types.ClosedByStop

			eng.lastPosition.StopLoss = candle.PriceData.ClosePrice

			eng.closedPositions = append(eng.closedPositions, *pos)

		} else if isTP {

			eng.lastPosition.State = types.ClosedByProfit

			eng.lastPosition.TP = candle.PriceData.ClosePrice

			eng.closedPositions = append(eng.closedPositions, *pos)

		}
	}
	if pos.State == types.SHORT_OPEN {

		isTP := candle.PriceData.ClosePrice <= pos.TP

		if candle.PriceData.ClosePrice >= pos.StopLoss {

			eng.lastPosition.State = types.ClosedByStop

			eng.lastPosition.StopLoss = candle.PriceData.ClosePrice

			eng.closedPositions = append(eng.closedPositions, *pos)

		} else if isTP {

			eng.lastPosition.State = types.ClosedByProfit

			eng.lastPosition.TP = candle.PriceData.ClosePrice

			eng.closedPositions = append(eng.closedPositions, *pos)

		}
	}
}
