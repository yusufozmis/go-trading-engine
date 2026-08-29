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
