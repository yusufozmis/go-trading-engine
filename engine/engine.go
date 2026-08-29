// Package engine evaluates strategy plans and tracks confirmed position state.
package engine

import (
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/engine/options"
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

	positionSizer     types.PositionSizer
	isCloseAutomated  bool
	maxEntryDeviation *float64
}

// NewEngine creates an engine for one symbol and timeframe. Callers may provide
// a custom position sizer or use one from the engine/positionsizers package.
func NewEngine(symbol, timeframe string,
	positionSizer types.PositionSizer,
	opts ...options.Option) (*Engine, error) {

	if symbol == "" {
		return nil, apperrors.ErrNilSymbol
	}
	if timeframe == "" {
		return nil, apperrors.ErrNilTimeframe
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

	if positionSizer == nil {
		return nil, apperrors.ErrNilPositionSizer
	}

	return &Engine{
		symbol:            symbol,
		timeframe:         timeframe,
		lockKeyMap:        make(map[string]bool),
		positionSizer:     positionSizer,
		maxEntryDeviation: cfg.MaxEntryDeviation,
		isCloseAutomated:  cfg.IsCloseAutomated,
	}, nil
}

func (eng *Engine) validate() error {
	if eng == nil {
		return apperrors.ErrNilEngine
	}
	if eng.symbol == "" {
		return apperrors.ErrNilSymbol
	}
	if eng.timeframe == "" {
		return apperrors.ErrNilTimeframe
	}
	return nil
}
