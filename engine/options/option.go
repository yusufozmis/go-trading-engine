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
	IsCloseAutomated        bool
	MaxEntryDeviation       *float64
	TradingFeeRate          float64
	SlippageRate            float64
	LiquidationModel        types.LiquidationModel
	MaximumPositionDuration time.Duration
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
// wall-clock timer, so evaluation occurs only when DecideClosePosition is called.
func WithMaximumPositionDuration(duration time.Duration) Option {
	return func(cfg *Config) error {
		if duration < time.Millisecond {
			return apperrors.ErrInvalidMaximumPositionDuration
		}

		cfg.MaximumPositionDuration = duration
		return nil
	}
}
