package errors

import "errors"

// Engine Errors
var (
	ErrInvalidEntryPlanBracket = errors.New("invalid entry plan tp/sl bracket")
	ErrNilEngine               = errors.New("nil engine")
	ErrInvalidEntryPlanType    = errors.New("invalid entry plan type")
	ErrInvalidPlanMode         = errors.New("invalid plan mode")
	ErrExpectedSinglePlan      = errors.New("expected single plan")
	ErrInvalidPositionState    = errors.New("invalid position state")
	ErrPositionOrPendingExists = errors.New("position or a pending position exists")
	ErrEntryPlanMarketMismatch = errors.New("entry plan market does not match engine")
)

// Backtest Errors
var (
	ErrNilBacktester       = errors.New("nil backtester")
	ErrNilStrategy         = errors.New("nil strategy")
	ErrInvalidSetOfCandles = errors.New("invalid set of candles")
	ErrEmptyCandleSet      = errors.New("empty candle set")
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
	ErrInvalidVolume     = errors.New("invalid volume")
	ErrInvalidLeverage   = errors.New("invalid leverage")
	ErrInvalidMarginMode = errors.New("invalid margin mode")
	// ErrInsufficientFunds indicates that an exchange rejected an order because
	// the account did not have enough available balance or margin.
	ErrInsufficientFunds = errors.New("insufficient funds")
	// ErrAuthenticationFailed indicates that the exchange rejected the configured credentials.
	ErrAuthenticationFailed = errors.New("authentication failed")
	// ErrPermissionDenied indicates that the credentials cannot perform the requested operation.
	ErrPermissionDenied = errors.New("permission denied")
	// ErrRateLimitExceeded indicates that the exchange rejected a request due to its rate limit.
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	// ErrInvalidOrder indicates that the exchange rejected the submitted order parameters.
	ErrInvalidOrder = errors.New("invalid order")
	// ErrPositionModeChangeRejected indicates that the exchange refused to
	// change position mode, commonly because positions, orders, or bots are active.
	ErrPositionModeChangeRejected = errors.New("position mode change rejected")
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

	// ErrOperationRejected indicates that the exchange rejected an operation
	// without exposing a more specific normalized error category.
	ErrOperationRejected = errors.New("operation rejected")
)
