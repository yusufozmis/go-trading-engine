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
