// Package liquidationmodels provides ready-to-use liquidation policies for an engine.
package liquidationmodels

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// IsolatedLinear estimates liquidation for an isolated linear position without
// funding, added margin, cross-account collateral, or provider margin tiers.
// liquidationFeeRate affects the estimated trigger; realized position fees are
// still controlled independently by options.WithTradingFeeRate.
type IsolatedLinear struct {
	leverage              float64
	maintenanceMarginRate float64
	liquidationFeeRate    float64
}

// NewIsolatedLinear creates the basic isolated USDT-margined liquidation model.
func NewIsolatedLinear(
	leverage,
	maintenanceMarginRate,
	liquidationFeeRate float64,
) (IsolatedLinear, error) {
	if math.IsNaN(leverage) || math.IsInf(leverage, 0) || leverage <= 1 {
		return IsolatedLinear{}, apperrors.ErrInvalidLeverage
	}
	if math.IsNaN(maintenanceMarginRate) || math.IsInf(maintenanceMarginRate, 0) ||
		maintenanceMarginRate < 0 || maintenanceMarginRate >= 1 {
		return IsolatedLinear{}, apperrors.ErrInvalidMaintenanceMarginRate
	}
	if math.IsNaN(liquidationFeeRate) || math.IsInf(liquidationFeeRate, 0) ||
		liquidationFeeRate < 0 || liquidationFeeRate >= 1 {
		return IsolatedLinear{}, apperrors.ErrInvalidLiquidationFeeRate
	}
	if maintenanceMarginRate+liquidationFeeRate >= 1 {
		return IsolatedLinear{}, apperrors.ErrInvalidMaintenanceMarginRate
	}
	// The maintenance boundary must remain below the initial margin rate;
	// otherwise the estimated liquidation level would cross the entry price.
	if maintenanceMarginRate+liquidationFeeRate >= 1/leverage {
		return IsolatedLinear{}, apperrors.ErrInvalidMaintenanceMarginRate
	}

	return IsolatedLinear{
		leverage:              leverage,
		maintenanceMarginRate: maintenanceMarginRate,
		liquidationFeeRate:    liquidationFeeRate,
	}, nil
}

// Leverage returns the leverage represented by this model.
func (model IsolatedLinear) Leverage() float64 {
	return model.leverage
}

// CalculateLiquidationPrice estimates the liquidation level from the initial
// isolated margin and the configured maintenance and liquidation fee rates.
func (model IsolatedLinear) CalculateLiquidationPrice(position types.Position) (float64, error) {
	if math.IsNaN(model.leverage) || math.IsInf(model.leverage, 0) ||
		model.leverage <= 1 {
		return 0, apperrors.ErrInvalidLeverage
	}
	if math.IsNaN(position.EntryPrice) || math.IsInf(position.EntryPrice, 0) ||
		position.EntryPrice <= 0 {
		return 0, apperrors.ErrInvalidPrice
	}
	if math.IsNaN(position.Amount) || math.IsInf(position.Amount, 0) ||
		position.Amount <= 0 {
		return 0, apperrors.ErrInvalidAmount
	}
	if !position.Side.Valid() {
		return 0, apperrors.ErrInvalidSide
	}

	positionNotional := position.EntryPrice * position.Amount
	initialMargin := positionNotional / model.leverage
	riskRate := model.maintenanceMarginRate + model.liquidationFeeRate

	var liquidationPrice float64
	switch position.Side {
	case types.PositionLong:
		liquidationPrice = (positionNotional - initialMargin) /
			(position.Amount * (1 - riskRate))
	case types.PositionShort:
		liquidationPrice = (positionNotional + initialMargin) /
			(position.Amount * (1 + riskRate))
	}

	if math.IsNaN(liquidationPrice) || math.IsInf(liquidationPrice, 0) ||
		liquidationPrice <= 0 {
		return 0, apperrors.ErrInvalidLiquidationPrice
	}

	return liquidationPrice, nil
}

var _ types.LiquidationModel = IsolatedLinear{}
