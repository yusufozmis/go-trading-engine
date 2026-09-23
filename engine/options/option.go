// Package options defines functional options for configuring an engine.
package options

import (
	"math"

	"github.com/yusufozmis/go-trading-engine/apperrors"
)

// Option applies one validated engine configuration change.
type Option func(*Config) error

// Config stores the values applied by engine options.
type Config struct {
	IsCloseAutomated  bool
	MaxEntryDeviation *float64
	TradingFeeRate    float64
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
