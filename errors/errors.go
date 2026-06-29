package errors

import "errors"

// Engine Errors
var (
	ErrInvalidEntryPlanBracket = errors.New("invalid entry plan tp/sl bracket")
	ErrNilEngine               = errors.New("nil engine")
	ErrInvalidEntryPlanType    = errors.New("invalid entry plan type")
	ErrInvalidPlanMode         = errors.New("invalid plan mode")
	ErrExpectedSinglePlan      = errors.New("expected single plan")
)

// Backtest Errors
var (
	ErrNilBacktester       = errors.New("nil backtester")
	ErrNilStrategy         = errors.New("nil strategy")
	ErrInvalidSetOfCandles = errors.New("invalid set of candles")
)
