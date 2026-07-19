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

// Stream Errors

var (
	ErrUnsupportedProvider = errors.New("exchange: unsupported provider")
	ErrNilClient           = errors.New("exchange: nil client")
	ErrUninitializedClient = errors.New("exchange: uninitialized client")
	ErrInvalidLimit        = errors.New("exchange: limit must be greater than 0")
	ErrLimitTooLarge       = errors.New("exchange: limit too large")
	ErrNoCandles           = errors.New("exchange: no candles returned")
)
