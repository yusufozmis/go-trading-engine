// Package engine evaluates strategy plans and tracks confirmed position state.
package engine

import (
	"github.com/yusufozmis/go-trading-engine/engine/options"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// Engine evaluates strategy plans and tracks confirmed position state.
// Its methods must not be called concurrently.
type Engine struct {
	symbol    string
	timeframe string

	lastPosition *types.Position

	closedPositions []types.Position

	lockKeyMap  map[string]bool
	activePlans []types.EntryPlan

	pendingConfirmation *types.EntryPlan

	isCloseAutomated  bool
	maxEntryDeviation *float64
}

// NewEngine creates an engine for one symbol and timeframe.
func NewEngine(symbol, timeframe string,
	opts ...options.Option) (*Engine, error) {

	if symbol == "" {
		return nil, errors.ErrNilSymbol
	}
	if timeframe == "" {
		return nil, errors.ErrNilTimeframe
	}

	cfg := options.Config{}

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	return &Engine{
		symbol:            symbol,
		timeframe:         timeframe,
		lockKeyMap:        make(map[string]bool),
		maxEntryDeviation: cfg.MaxEntryDeviation,
		isCloseAutomated:  cfg.IsCloseAutomated,
	}, nil
}

func (eng *Engine) validate() error {
	if eng == nil {
		return errors.ErrNilEngine
	}
	if eng.symbol == "" {
		return errors.ErrNilSymbol
	}
	if eng.timeframe == "" {
		return errors.ErrNilTimeframe
	}
	return nil
}
