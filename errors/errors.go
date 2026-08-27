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
	ErrStreamAlreadyExists = errors.New("there is already a stream running")
	ErrStreamNotRunning    = errors.New("exchange: candle stream is not running")
)

// Exchange Errors
var (
	// ErrNilAPIKey indicates that authenticated exchange configuration omitted the API key.
	ErrNilAPIKey    = errors.New("nil api key")
	ErrNilSecretKey = errors.New("nil secret key")
	ErrNilPassword  = errors.New("nil password")

	ErrNilSymbol         = errors.New("nil symbol")
	ErrNilTimeframe      = errors.New("nil timeframe")
	ErrInvalidSide       = errors.New("invalid side")
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrInvalidPrice      = errors.New("invalid price")
	ErrInvalidLeverage   = errors.New("invalid leverage")
	ErrInvalidMarginMode = errors.New("invalid margin mode")
	// ErrContractSizeUnavailable indicates that contract metadata cannot be used
	// to convert a base-asset amount into an exchange contract amount.
	ErrContractSizeUnavailable = errors.New("contract size unavailable")
	// ErrUnsupportedFuturesMarket indicates that base-asset amount conversion is
	// not supported for the requested futures market.
	ErrUnsupportedFuturesMarket = errors.New("unsupported futures market")
	// ErrInvalidSymbol indicates that a symbol does not use the required format.
	ErrInvalidSymbol = errors.New("invalid symbol")
	// ErrPositionNotFound indicates that there is no open position to close.
	ErrPositionNotFound = errors.New("position not found")

	ErrEmptySymbols      = errors.New("empty symbols")
	ErrEmptyTimeframes   = errors.New("empty timeframes")
	ErrInvalidStreamMode = errors.New("invalid stream mode")

	ErrNotEnoughAmount = errors.New("not enough amount")
	// ErrMinimumAmountUnavailable indicates that the exchange did not publish an order minimum.
	ErrMinimumAmountUnavailable = errors.New("exchange: minimum order amount is unavailable")
	// ErrBalanceNotFound indicates that the requested asset is absent from the balance response.
	ErrBalanceNotFound        = errors.New("balance not found")
	ErrFundingRateUnavailable = errors.New("funding rate unavailable")
	ErrConfigAlreadySet       = errors.New("config already set")
	ErrClientNotConfigured    = errors.New("client not configured")
)
