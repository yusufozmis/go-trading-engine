package engine

import (
	"math"
	"strconv"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// MoveTPSLFromPlan re-anchors TP/SL to the candle close while preserving
// the plan's original reward distance and risk distance.
func MoveTPSLFromPlan(candle types.Candle, plan types.EntryPlan) (newTP float64, newSL float64, err error) {
	if err := validateCandleData(candle); err != nil {
		return 0, 0, err
	}
	if err := validateEntryPlanData(plan); err != nil {
		return 0, 0, err
	}

	entry := plan.EntryPrice
	tp := plan.TakeProfit
	sl := plan.StopLoss

	rewardDist := math.Abs(tp - entry)
	riskDist := math.Abs(entry - sl)

	if rewardDist == 0 || riskDist == 0 {
		return 0, 0, apperrors.ErrInvalidEntryPlanBracket
	}

	newEntry := candle.PriceData.ClosePrice

	switch plan.Side {
	case types.PositionLong:
		return newEntry + rewardDist, newEntry - riskDist, nil
	case types.PositionShort:
		return newEntry - rewardDist, newEntry + riskDist, nil
	default:
		return 0, 0, apperrors.ErrInvalidSide
	}
}

func (eng *Engine) validateCandle(candle types.Candle) error {
	if err := eng.validate(); err != nil {
		return err
	}
	if candle.Symbol != eng.symbol || candle.Timeframe != eng.timeframe {
		return apperrors.ErrCandleMarketMismatch
	}

	return validateCandleData(candle)
}

func validateCandleData(candle types.Candle) error {
	prices := []float64{
		candle.PriceData.OpenPrice,
		candle.PriceData.HighPrice,
		candle.PriceData.LowPrice,
		candle.PriceData.ClosePrice,
	}
	for _, price := range prices {
		if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
			return apperrors.ErrInvalidPrice
		}
	}

	if math.IsNaN(candle.Volume) || math.IsInf(candle.Volume, 0) || candle.Volume < 0 {
		return apperrors.ErrInvalidVolume
	}

	low := candle.PriceData.LowPrice
	high := candle.PriceData.HighPrice
	open := candle.PriceData.OpenPrice
	closePrice := candle.PriceData.ClosePrice
	if low > high || open < low || open > high || closePrice < low || closePrice > high {
		return apperrors.ErrInvalidSetOfCandles
	}

	return nil
}

// validateEntryPlan ensures that a plan targets this engine and that its
// prices form a valid bracket for the requested direction.
func (eng *Engine) validateEntryPlan(plan types.EntryPlan) error {
	if err := eng.validate(); err != nil {
		return err
	}

	if plan.Symbol != eng.symbol || plan.Timeframe != eng.timeframe {
		return apperrors.ErrEntryPlanMarketMismatch
	}

	return validateEntryPlanData(plan)
}

func validateEntryPlanData(plan types.EntryPlan) error {
	if !plan.Side.Valid() {
		return apperrors.ErrInvalidSide
	}
	if !plan.Confirmation.Valid() {
		return apperrors.ErrInvalidConfirmationState
	}

	prices := [...]float64{
		plan.EntryPrice,
		plan.StopLoss,
		plan.TakeProfit,
	}
	for _, price := range prices {
		if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
			return apperrors.ErrInvalidPrice
		}
	}

	if math.IsNaN(plan.LockPrice) || math.IsInf(plan.LockPrice, 0) {
		return apperrors.ErrInvalidPrice
	}

	switch plan.Side {
	case types.PositionLong:
		if plan.TakeProfit <= plan.EntryPrice || plan.StopLoss >= plan.EntryPrice {
			return apperrors.ErrInvalidEntryPlanBracket
		}
	case types.PositionShort:
		if plan.TakeProfit >= plan.EntryPrice || plan.StopLoss <= plan.EntryPrice {
			return apperrors.ErrInvalidEntryPlanBracket
		}
	default:
		return apperrors.ErrInvalidSide
	}

	return nil
}

func (eng *Engine) validPosition(position types.Position) bool {
	if eng.validate() != nil ||
		position.Symbol != eng.symbol ||
		position.Timeframe != eng.timeframe ||
		position.OpenTimestamp <= 0 ||
		position.CloseTimestamp != 0 ||
		position.ExitPrice != 0 ||
		position.State != types.PositionOpen {
		return false
	}

	values := [...]float64{
		position.EntryPrice,
		position.StopLoss,
		position.TP,
		position.Amount,
	}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return false
		}
	}

	switch position.Side {
	case types.PositionLong:
		return position.TP > position.EntryPrice &&
			position.StopLoss < position.EntryPrice
	case types.PositionShort:
		return position.TP < position.EntryPrice &&
			position.StopLoss > position.EntryPrice
	default:
		return false
	}
}

func (eng *Engine) activePlanIndex(target types.EntryPlan) int {
	for i, plan := range eng.activePlans {
		if plan == target {
			return i
		}
	}

	return -1
}

func entryPlanLockKey(plan types.EntryPlan) string {

	key := strconv.FormatFloat(plan.LockPrice, 'g', -1, 64)

	return plan.Symbol + "|" + plan.Timeframe + "|" + key
}
