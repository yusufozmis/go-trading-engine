// Package engine evaluates strategy plans and tracks confirmed position state.
package engine

import (
	"sync"
	"time"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/engine/options"
	"github.com/yusufozmis/go-trading-engine/types"
)

// Engine evaluates strategy plans and tracks confirmed position state.
// Its methods must not be called concurrently, except Performance may be
// called concurrently with other methods.
type Engine struct {
	symbol    string
	timeframe string

	positions map[types.PositionSide]*types.Position

	// realizedMu protects complete closes and snapshots of still-open positions
	// with realized partial profit while Performance runs concurrently.
	realizedMu                 sync.RWMutex
	closedPositions            []types.Position
	partiallyRealizedPositions map[types.PositionSide]types.Position

	lockKeyMap           map[string]bool
	activePlans          []types.EntryPlan
	confirmationProgress map[types.EntryPlan]candleConfirmationProgress

	pendingConfirmations map[types.PositionSide]*types.EntryPlan
	pendingStartedAt     map[types.PositionSide]int64

	positionSizer types.PositionSizer

	hedgeMode                   bool
	isCloseAutomated            bool
	maxEntryDeviation           *float64
	tradingFeeRate              float64
	slippageRate                float64
	liquidationModel            types.LiquidationModel
	maximumPositionDuration     time.Duration
	breakEvenStopRate           float64
	allowedSide                 types.PositionSide
	confirmationPercentage      float64
	confirmationCandleCount     int
	confirmationTimeout         time.Duration
	partialTakeProfitPercentage float64
	partialReductionPercentage  float64
	profitLockTriggerRate       float64
	profitLockRate              float64
}

type candleConfirmationProgress struct {
	count         int
	lastTimestamp int64
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
		symbol:                      symbol,
		timeframe:                   timeframe,
		positions:                   make(map[types.PositionSide]*types.Position, 2),
		partiallyRealizedPositions:  make(map[types.PositionSide]types.Position, 2),
		lockKeyMap:                  make(map[string]bool),
		confirmationProgress:        make(map[types.EntryPlan]candleConfirmationProgress),
		pendingConfirmations:        make(map[types.PositionSide]*types.EntryPlan, 2),
		pendingStartedAt:            make(map[types.PositionSide]int64, 2),
		positionSizer:               positionSizer,
		hedgeMode:                   cfg.HedgeMode,
		maxEntryDeviation:           cfg.MaxEntryDeviation,
		isCloseAutomated:            cfg.IsCloseAutomated,
		tradingFeeRate:              cfg.TradingFeeRate,
		slippageRate:                cfg.SlippageRate,
		liquidationModel:            cfg.LiquidationModel,
		maximumPositionDuration:     cfg.MaximumPositionDuration,
		breakEvenStopRate:           cfg.BreakEvenStopRate,
		allowedSide:                 cfg.AllowedPositionSide,
		confirmationPercentage:      cfg.ConfirmationPercentage,
		confirmationCandleCount:     cfg.ConfirmationCandleCount,
		confirmationTimeout:         cfg.ConfirmationTimeout,
		partialTakeProfitPercentage: cfg.PartialTakeProfitPercentage,
		partialReductionPercentage:  cfg.PartialReductionPercentage,
		profitLockTriggerRate:       cfg.ProfitLockTriggerRate,
		profitLockRate:              cfg.ProfitLockRate,
	}, nil
}

// NewEngineWithPreloadedPositions creates an engine that takes ownership of
// existing open positions. Without WithHedgeMode, at most one position may be
// supplied; hedge mode accepts one long and one short position.
func NewEngineWithPreloadedPositions(
	positions []types.Position,
	positionSizer types.PositionSizer,
	opts ...options.Option,
) (*Engine, error) {
	if len(positions) == 0 {
		return nil, apperrors.ErrPositionNotFound
	}
	first := positions[0]

	eng, err := NewEngine(
		first.Symbol,
		first.Timeframe,
		positionSizer,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	for _, position := range positions {
		if position.State != types.PositionOpen {
			return nil, apperrors.ErrInvalidPositionState
		}
		if err := position.Validate(); err != nil {
			return nil, err
		}
		if position.Symbol != eng.symbol || position.Timeframe != eng.timeframe {
			return nil, apperrors.ErrInvalidPositionAction
		}
		if eng.positionExists(position.Side) ||
			(!eng.hedgeMode && eng.PositionExists()) {
			return nil, apperrors.ErrPositionOrPendingExists
		}

		positionCopy := position
		eng.positions[position.Side] = &positionCopy
		if position.PartialTakeProfitExecuted {
			eng.partiallyRealizedPositions[position.Side] = positionCopy
		}
	}

	return eng, nil
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

// HedgeModeEnabled reports whether the engine permits simultaneous long and
// short positions.
func (eng *Engine) HedgeModeEnabled() bool {
	return eng != nil && eng.hedgeMode
}
