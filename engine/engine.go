// Package engine evaluates strategy plans and tracks confirmed position state.
package engine

import (
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// Engine evaluates strategy plans and tracks confirmed position state.
// Its methods must not be called concurrently.
type Engine struct {
	symbol    string
	timeframe string

	isCloseAutomated bool
	lastPosition     *types.Position

	closedPositions []types.Position

	lockKeyMap  map[string]bool
	activePlans []types.EntryPlan

	pendingConfirmation *types.EntryPlan
}

// NewEngine creates an engine for one symbol and timeframe.
func NewEngine(symbol, timeframe string, isCloseAutomated bool) (*Engine, error) {

	if symbol == "" {
		return nil, errors.ErrNilSymbol
	}
	if timeframe == "" {
		return nil, errors.ErrNilTimeframe
	}

	return &Engine{
		symbol:           symbol,
		timeframe:        timeframe,
		isCloseAutomated: isCloseAutomated,
		lockKeyMap:       make(map[string]bool),
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
