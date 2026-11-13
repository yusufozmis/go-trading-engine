package engine

import (
	"fmt"
	"math"
	"strconv"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

func positionSides() [2]types.PositionSide {
	return [2]types.PositionSide{types.PositionLong, types.PositionShort}
}

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

// ValidateCandle validates and records a candle for processing, rejecting an
// already accepted timestamp with ErrDuplicateCandle. Call it once before
// passing each new candle to the strategy; engine action methods only validate
// candle data, so the same candle can be used throughout one processing cycle.
// Invalid candles are not recorded. Only finalized candles should be submitted.
// Call Strategy.AddBar only after validation succeeds to prevent
// duplicate candles from distorting indicators and backtest results;
// Backtester.Run enforces this order.
// The first candle establishes the sequence. Later candles must advance by
// exactly one timeframe; missing intervals return ErrCandleGap and must be
// supplied in order before processing can resume. Rejections do not advance
// the sequence. Monthly timeframes use UTC calendar months, not fixed days.
func (eng *Engine) ValidateCandle(candle types.Candle) error {
	if err := eng.validateCandle(candle); err != nil {
		return err
	}
	if eng.acceptedCandles[candle.Timestamp] {
		return apperrors.ErrDuplicateCandle
	}
	if candle.Timestamp <= eng.lastCandleTimestamp {
		return apperrors.ErrInvalidTimestamp
	}
	expectedTimestamp, err := nextCandleTimestamp(eng.lastCandleTimestamp, eng.timeframe)
	if err != nil {
		return err
	}
	if eng.lastCandleTimestamp != 0 && candle.Timestamp != expectedTimestamp {
		sequenceErr := apperrors.ErrCandleGap
		if candle.Timestamp < expectedTimestamp {
			sequenceErr = apperrors.ErrInvalidTimestamp
		}
		return fmt.Errorf("%w: expected timestamp %d, got %d", sequenceErr, expectedTimestamp, candle.Timestamp)
	}
	if eng.acceptedCandles == nil {
		eng.acceptedCandles = make(map[int64]bool)
	}
	eng.acceptedCandles[candle.Timestamp] = true
	eng.lastCandleTimestamp = candle.Timestamp
	return nil
}

// validateCandle checks engine compatibility and candle data without recording
// acceptance, allowing decision methods to share a candle in one processing cycle.
func (eng *Engine) validateCandle(candle types.Candle) error {
	if err := eng.Validate(); err != nil {
		return err
	}
	if candle.Symbol != eng.symbol || candle.Timeframe != eng.timeframe {
		return apperrors.ErrCandleMarketMismatch
	}
	return validateCandleData(candle)
}

func validateCandleData(candle types.Candle) error {
	if candle.Timestamp <= 0 {
		return apperrors.ErrInvalidTimestamp
	}

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

// ValidateEntryPlan ensures that a plan targets this engine and that its
// prices form a valid bracket for the requested direction.
func (eng *Engine) ValidateEntryPlan(plan types.EntryPlan) error {
	if err := eng.Validate(); err != nil {
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
	if eng.Validate() != nil ||
		position.Symbol != eng.symbol ||
		position.Timeframe != eng.timeframe ||
		position.OpenTimestamp <= 0 ||
		position.CloseTimestamp != 0 ||
		position.ExitPrice != 0 ||
		math.IsNaN(position.MarkPrice) ||
		math.IsInf(position.MarkPrice, 0) ||
		position.MarkPrice < 0 ||
		math.IsNaN(position.Fee) ||
		math.IsInf(position.Fee, 0) ||
		position.Fee < 0 ||
		math.IsNaN(position.FundingFee) ||
		math.IsInf(position.FundingFee, 0) ||
		math.IsNaN(position.SlippageCost) ||
		math.IsInf(position.SlippageCost, 0) ||
		position.SlippageCost < 0 ||
		math.IsNaN(position.NetProfit) ||
		math.IsInf(position.NetProfit, 0) ||
		position.State != types.PositionOpen {
		return false
	}
	if position.PartialTakeProfitCount < 0 || position.PartialLoseCount < 0 {
		return false
	}
	if (position.PartialTakeProfitExecuted &&
		position.PartialTPTimestamp <= position.OpenTimestamp) ||
		(!position.PartialTakeProfitExecuted && position.PartialTPTimestamp != 0) {
		return false
	}
	if eng.liquidationModel == nil {
		if position.Leverage != 0 || position.LiquidationPrice != 0 {
			return false
		}
	} else if math.IsNaN(position.Leverage) || math.IsInf(position.Leverage, 0) ||
		position.Leverage <= 1 || math.IsNaN(position.LiquidationPrice) ||
		math.IsInf(position.LiquidationPrice, 0) || position.LiquidationPrice <= 0 {
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

func (eng *Engine) applyLiquidationModel(position *types.Position) error {
	if eng.liquidationModel == nil {
		position.Leverage = 0
		position.LiquidationPrice = 0
		return nil
	}

	liquidationPrice, err := eng.liquidationModel.CalculateLiquidationPrice(*position)
	if err != nil {
		return err
	}

	position.Leverage = eng.liquidationModel.Leverage()
	position.LiquidationPrice = liquidationPrice
	return ValidateLiquidationPrice(*position)
}

func ValidateLiquidationPrice(position types.Position) error {
	if math.IsNaN(position.Leverage) || math.IsInf(position.Leverage, 0) ||
		position.Leverage <= 1 {
		return apperrors.ErrInvalidLeverage
	}
	if math.IsNaN(position.LiquidationPrice) ||
		math.IsInf(position.LiquidationPrice, 0) || position.LiquidationPrice <= 0 {
		return apperrors.ErrInvalidLiquidationPrice
	}

	switch position.Side {
	case types.PositionLong:
		if position.LiquidationPrice >= position.EntryPrice {
			return apperrors.ErrInvalidLiquidationPrice
		}
	case types.PositionShort:
		if position.LiquidationPrice <= position.EntryPrice {
			return apperrors.ErrInvalidLiquidationPrice
		}
	default:
		return apperrors.ErrInvalidSide
	}

	return nil
}

func (eng *Engine) entryFillPrice(price float64, side types.PositionSide) float64 {
	if side == types.PositionLong {
		return price * (1 + eng.slippageRate)
	}
	return price * (1 - eng.slippageRate)
}

func (eng *Engine) exitFillPrice(price float64, side types.PositionSide) float64 {
	if side == types.PositionLong {
		return price * (1 - eng.slippageRate)
	}
	return price * (1 + eng.slippageRate)
}

func (eng *Engine) entrySlippageCost(
	fillPrice float64,
	amount float64,
	side types.PositionSide,
) float64 {
	if eng.slippageRate == 0 {
		return 0
	}

	idealPrice := fillPrice / (1 + eng.slippageRate)
	if side == types.PositionShort {
		idealPrice = fillPrice / (1 - eng.slippageRate)
	}

	return math.Abs(fillPrice-idealPrice) * amount
}

func (eng *Engine) exitSlippageCost(
	fillPrice float64,
	amount float64,
	side types.PositionSide,
) float64 {
	if eng.slippageRate == 0 {
		return 0
	}

	idealPrice := fillPrice / (1 - eng.slippageRate)
	if side == types.PositionShort {
		idealPrice = fillPrice / (1 + eng.slippageRate)
	}

	return math.Abs(fillPrice-idealPrice) * amount
}

func (eng *Engine) activePlanIndex(target types.EntryPlan) int {
	for i, plan := range eng.activePlans {
		if plan == target {
			return i
		}
	}

	return -1
}

func (eng *Engine) entryPlanLockKey(plan types.EntryPlan) string {

	key := strconv.FormatFloat(plan.LockPrice, 'g', -1, 64)
	if eng.hedgeMode {
		return plan.Symbol + "|" + plan.Timeframe + "|" + plan.Side.String() + "|" + key
	}

	return plan.Symbol + "|" + plan.Timeframe + "|" + key
}

func (eng *Engine) StrategyContext() (types.StrategyContext, error) {
	if err := eng.Validate(); err != nil {
		return types.StrategyContext{}, err
	}
	return types.StrategyContext{
		HedgeMode:            eng.HedgeModeEnabled(),
		HasPendingPosition:   eng.PendingExists(),
		HasOpenPosition:      eng.PositionExists(),
		PendingPositionSides: eng.PendingSides(),
		OpenPositionSides:    eng.PositionSides(),
	}, nil
}
