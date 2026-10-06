// Package options defines functional options for configuring an engine.
package options

import (
	"math"
	"time"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// Option applies one validated engine configuration change.
type Option func(*Config) error

// Config stores the values applied by engine options.
type Config struct {
	HedgeMode                   bool
	IsCloseAutomated            bool
	MaxEntryDeviation           *float64
	TradingFeeRate              float64
	SlippageRate                float64
	LiquidationModel            types.LiquidationModel
	MaximumPositionDuration     time.Duration
	BreakEvenStopRate           float64
	AllowedPositionSide         types.PositionSide
	ConfirmationPercentage      float64
	ConfirmationCandleCount     int
	ConfirmationTimeout         time.Duration
	PartialTakeProfitPercentage float64
	PartialReductionPercentage  float64
	ProfitLockTriggerRate       float64
	ProfitLockRate              float64
}

// WithHedgeMode allows one long and one short position to remain open at the
// same time. Without this option, the engine preserves its single-position
// behavior and rejects a second position regardless of side.
func WithHedgeMode() Option {
	return func(cfg *Config) error {
		cfg.HedgeMode = true
		return nil
	}
}

// WithLiquidationModel enables liquidation simulation using model. For
// exchange-accurate decisions, candles passed to the engine should represent
// mark prices rather than last-traded prices.
func WithLiquidationModel(model types.LiquidationModel) Option {
	return func(cfg *Config) error {
		if model == nil {
			return apperrors.ErrNilLiquidationModel
		}

		cfg.LiquidationModel = model
		return nil
	}
}

// WithMaxEntryDeviation limits how far execution may move from the planned
// entry price. A value of 0.02 represents a maximum deviation of 2 percent.
func WithMaxEntryDeviation(deviation float64) Option {
	return func(cfg *Config) error {
		if math.IsNaN(deviation) || math.IsInf(deviation, 0) || deviation <= 0 {
			return apperrors.ErrInvalidMaxEntryDeviation
		}

		cfg.MaxEntryDeviation = &deviation
		return nil
	}
}

// WithAutomatedClose controls whether candle high and low prices can trigger
// take-profit and stop-loss closes.
func WithAutomatedClose() Option {
	return func(cfg *Config) error {
		cfg.IsCloseAutomated = true
		return nil
	}
}

// WithTradingFeeRate applies the same feeRate independently to the entry and
// exit fill of every confirmed closed position. The resulting total is stored
// in Position.Fee and summed by Performance. Fees use quote notional (price
// times amount), as required by linear futures such as USDT-margined
// perpetuals. The rate is a decimal fraction, so 0.0005 means 0.05% per fill.
// When omitted, position fees default to zero. If supplied more than once, the
// last rate replaces earlier rates.
func WithTradingFeeRate(feeRate float64) Option {
	return func(cfg *Config) error {
		if math.IsNaN(feeRate) || math.IsInf(feeRate, 0) || feeRate <= 0 {
			return apperrors.ErrInvalidTradingFeeRate
		}
		cfg.TradingFeeRate = feeRate
		return nil
	}
}

// WithSlippageRate applies an adverse percentage adjustment to every entry and
// exit fill. A value of 0.0005 represents 0.05 percent slippage. When omitted,
// fills are not adjusted. The total entry and exit impact is also recorded in
// Position.SlippageCost for reporting; it is not deducted from NetProfit again
// because the adjusted fill prices already include it. If supplied more than
// once, the last rate replaces earlier rates.
func WithSlippageRate(slippageRate float64) Option {
	return func(cfg *Config) error {
		if math.IsNaN(slippageRate) || math.IsInf(slippageRate, 0) ||
			slippageRate <= 0 || slippageRate >= 1 {
			return apperrors.ErrInvalidSlippageRate
		}

		cfg.SlippageRate = slippageRate
		return nil
	}
}

// WithMaximumPositionDuration closes a position on the first subsequent candle
// whose timestamp reaches the configured maximum lifetime. It does not start a
// wall-clock timer, so evaluation occurs only when DecideClosePositions is called.
func WithMaximumPositionDuration(duration time.Duration) Option {
	return func(cfg *Config) error {
		if duration < time.Millisecond {
			return apperrors.ErrInvalidMaximumPositionDuration
		}

		cfg.MaximumPositionDuration = duration
		return nil
	}
}

// WithBreakEvenStop moves an open position's stop-loss to its entry price after
// price covers the configured fraction of the entry-to-TP distance. A value of
// 0.50 arms break-even halfway to TP. The updated stop becomes effective from
// the next candle because OHLC data cannot reveal intrabar event ordering.
func WithBreakEvenStop(breakEvenStopRate float64) Option {
	return func(cfg *Config) error {
		if math.IsNaN(breakEvenStopRate) ||
			math.IsInf(breakEvenStopRate, 0) ||
			breakEvenStopRate <= 0 || breakEvenStopRate >= 1 {
			return apperrors.ErrInvalidBreakEvenStopRate
		}

		cfg.BreakEvenStopRate = breakEvenStopRate
		return nil
	}
}

// WithPositionSideFilter permits new positions only for allowedSide. Without
// this option, both long and short positions are allowed. The filter applies to
// positions opened after engine construction; preloaded positions remain
// manageable even when their side differs. If supplied more than once, the last
// side replaces earlier values.
func WithPositionSideFilter(allowedSide types.PositionSide) Option {
	return func(cfg *Config) error {
		if !allowedSide.Valid() {
			return apperrors.ErrInvalidPositionSide
		}

		cfg.AllowedPositionSide = allowedSide
		return nil
	}
}

// WithConfirmationPercentageFilter requires the configured fraction of a
// candle body to cross the entry price before a position can open. Long plans
// measure the body above entry and short plans measure the body below entry;
// candle wicks do not count. A value of 0.50 requires half of the body to cross.
// Without this option, the existing close-price trigger is used by itself. If
// supplied more than once, the last percentage replaces earlier values.
func WithConfirmationPercentageFilter(confirmationPercentage float64) Option {
	return func(cfg *Config) error {
		if math.IsNaN(confirmationPercentage) ||
			math.IsInf(confirmationPercentage, 0) ||
			confirmationPercentage <= 0 || confirmationPercentage >= 1 {
			return apperrors.ErrInvalidConfirmationPercentage
		}

		cfg.ConfirmationPercentage = confirmationPercentage
		return nil
	}
}

// WithConfirmationCandleCountFilter requires confirmationCandleCount
// consecutive candle closes beyond the entry before a position can open. Long
// plans count closes at or above entry and short plans count closes at or below
// entry. A close back across entry resets that plan's count. Re-evaluating the
// same candle does not increment the count twice. Without this option, one
// qualifying close is sufficient. If supplied more than once, the last count
// replaces earlier values.
func WithConfirmationCandleCountFilter(confirmationCandleCount int) Option {
	return func(cfg *Config) error {
		if confirmationCandleCount <= 0 {
			return apperrors.ErrInvalidConfirmationCandleCount
		}

		cfg.ConfirmationCandleCount = confirmationCandleCount
		return nil
	}
}

// WithConfirmationTimeout cancels a pending confirmation plan when it has not
// validated within timeout. The duration starts at the first candle evaluated
// for that pending plan and advances using candle timestamps, keeping backtests
// deterministic. Expiration is checked only when CheckConfirmations receives a
// candle. If supplied more than once, the last timeout replaces earlier values.
func WithConfirmationTimeout(timeout time.Duration) Option {
	return func(cfg *Config) error {
		if timeout < time.Millisecond {
			return apperrors.ErrInvalidConfirmationTimeout
		}

		cfg.ConfirmationTimeout = timeout
		return nil
	}
}

// WithPartialTakeProfit configures one partial reduction per position. The
// triggerPercentage is measured across the entry-to-TP distance, while
// reductionPercentage is the fraction of the then-open amount to realize. For
// example, (0.50, 0.50) realizes half the position halfway to TP. Without this
// option no partial reduction occurs. If supplied more than once, the last
// values replace earlier values.
func WithPartialTakeProfit(triggerPercentage, reductionPercentage float64) Option {
	return func(cfg *Config) error {
		if math.IsNaN(triggerPercentage) ||
			math.IsInf(triggerPercentage, 0) ||
			triggerPercentage <= 0 || triggerPercentage >= 1 {
			return apperrors.ErrInvalidPartialTakeProfitPercentage
		}
		if math.IsNaN(reductionPercentage) ||
			math.IsInf(reductionPercentage, 0) ||
			reductionPercentage <= 0 || reductionPercentage >= 1 {
			return apperrors.ErrInvalidPartialReductionPercentage
		}

		cfg.PartialTakeProfitPercentage = triggerPercentage
		cfg.PartialReductionPercentage = reductionPercentage
		return nil
	}
}

// WithProfitLockStop moves the stop into profit after price reaches a configured
// fraction of the entry-to-TP distance. For example, (0.50, 0.25) moves the stop
// to 25 percent of that distance after price reaches 50 percent. The new stop is
// effective from the next candle because OHLC data cannot establish whether the
// trigger or the updated stop was reached first within the triggering candle.
func WithProfitLockStop(triggerRate, lockRate float64) Option {
	return func(cfg *Config) error {
		if math.IsNaN(triggerRate) || math.IsInf(triggerRate, 0) ||
			triggerRate <= 0 || triggerRate >= 1 {
			return apperrors.ErrInvalidProfitLockTriggerRate
		}
		if math.IsNaN(lockRate) || math.IsInf(lockRate, 0) ||
			lockRate <= 0 || lockRate >= triggerRate {
			return apperrors.ErrInvalidProfitLockRate
		}

		cfg.ProfitLockTriggerRate = triggerRate
		cfg.ProfitLockRate = lockRate
		return nil
	}
}
