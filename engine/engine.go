package engine

import (
	"github.com/yusufozmis/trading-library/types"
)

type Engine struct {
	Symbol    string
	Timeframe string

	isCloseAutomated bool
	LastPosition     *types.Position

	ClosedPositions []types.Position

	lockKeyMap  map[string]bool
	activePlans []types.EntryPlan

	pendingConfirmation *types.EntryPlan
}

func NewEngine(symbol, timeframe string, isCloseAutomated bool) *Engine {
	return &Engine{
		Symbol:           symbol,
		Timeframe:        timeframe,
		isCloseAutomated: isCloseAutomated,
		lockKeyMap:       make(map[string]bool),
	}
}
