// Package errors defines sentinel errors returned by the trading engine.
package errors

import "errors"

// Engine Errors
var (
	// ErrInvalidEntryPlanBracket indicates an invalid entry, TP, and stop relationship.
	ErrInvalidEntryPlanBracket = errors.New("invalid entry plan tp/sl bracket")
	// ErrNilEngine indicates an operation on a nil engine.
	ErrNilEngine = errors.New("nil engine")
	// ErrInvalidEntryPlanType indicates an unsupported entry-plan state.
	ErrInvalidEntryPlanType = errors.New("invalid entry plan type")
	// ErrInvalidPlanMode indicates an unsupported plan-update mode.
	ErrInvalidPlanMode = errors.New("invalid plan mode")
	// ErrExpectedSinglePlan indicates that confirmation received the wrong plan count.
	ErrExpectedSinglePlan = errors.New("expected single plan")
	// ErrPositionOrPendingExists indicates that the engine is already occupied.
	ErrPositionOrPendingExists = errors.New("position or a pending position exists")
	// ErrEntryPlanMarketMismatch indicates that a plan targets another market.
	ErrEntryPlanMarketMismatch = errors.New("entry plan market does not match engine")
	// ErrCandleMarketMismatch indicates that a candle targets another market.
	ErrCandleMarketMismatch = errors.New("candle market does not match engine")
	// ErrInvalidPositionAction indicates that a position action would violate
	// the engine's position invariants.
	ErrInvalidPositionAction = errors.New("invalid position action")
	// ErrStalePositionAction indicates that engine state changed after an action
	// was decided and before it was confirmed.
	ErrStalePositionAction = errors.New("stale position action")
	// ErrNilPositionSizer indicates that the user provided a nil position sizer.
	ErrNilPositionSizer = errors.New("nil position sizer")
)

// Backtest Errors
var (
	// ErrNilBacktester indicates an operation on a nil backtester.
	ErrNilBacktester = errors.New("nil backtester")
	// ErrNilStrategy indicates that a backtest received no strategy.
	ErrNilStrategy = errors.New("nil strategy")
	// ErrInvalidSetOfCandles indicates malformed or inconsistent candle data.
	ErrInvalidSetOfCandles = errors.New("invalid set of candles")
	// ErrEmptyCandleSet indicates that a backtest received no candles.
	ErrEmptyCandleSet = errors.New("empty candle set")
)

// Stream Errors
var (
	// ErrUnsupportedProvider indicates that the selected exchange has no supported adapter.
	ErrUnsupportedProvider = errors.New("exchange: unsupported provider")
	// ErrNilClient indicates an operation on a nil exchange client.
	ErrNilClient = errors.New("exchange: nil client")
	// ErrUninitializedClient indicates that an exchange client has no provider instance.
	ErrUninitializedClient = errors.New("exchange: uninitialized client")
	// ErrInvalidLimit indicates that a candle request limit is not positive.
	ErrInvalidLimit = errors.New("exchange: limit must be greater than 0")
	// ErrLimitTooLarge indicates that a candle request exceeds the supported limit.
	ErrLimitTooLarge = errors.New("exchange: limit too large")
	// ErrNoCandles indicates that an exchange returned no candle data.
	ErrNoCandles = errors.New("exchange: no candles returned")
	// ErrStreamAlreadyExists indicates that the client already owns a running candle stream.
	ErrStreamAlreadyExists = errors.New("there is already a stream running")
	// ErrStreamNotRunning indicates that the client has no running candle stream.
	ErrStreamNotRunning = errors.New("exchange: candle stream is not running")
)

// Options Errors
var (
	// ErrInvalidMaxEntryDeviation indicates a non-finite or non-positive deviation limit.
	ErrInvalidMaxEntryDeviation = errors.New("invalid max entry deviation")
)

// Exchange Errors
var (
	// ErrNilAPIKey indicates that authenticated exchange configuration omitted the API key.
	ErrNilAPIKey = errors.New("nil api key")
	// ErrNilSecretKey indicates that authenticated exchange configuration omitted the secret key.
	ErrNilSecretKey = errors.New("nil secret key")
	// ErrNilPassword indicates that authenticated exchange configuration omitted a required password.
	ErrNilPassword = errors.New("nil password")

	// ErrNilSymbol indicates that a required symbol is empty.
	ErrNilSymbol = errors.New("nil symbol")
	// ErrNilTimeframe indicates that a required timeframe is empty.
	ErrNilTimeframe = errors.New("nil timeframe")
	// ErrInvalidSide indicates an unsupported order or position side.
	ErrInvalidSide = errors.New("invalid side")
	// ErrInvalidAmount indicates a non-finite or non-positive order amount.
	ErrInvalidAmount = errors.New("invalid amount")
	// ErrInvalidPrice indicates a non-finite or non-positive price.
	ErrInvalidPrice = errors.New("invalid price")
	// ErrInvalidVolume indicates a non-finite or negative volume.
	ErrInvalidVolume = errors.New("invalid volume")
	// ErrInvalidLeverage indicates an unsupported leverage value.
	ErrInvalidLeverage = errors.New("invalid leverage")
	// ErrInvalidMarginMode indicates an unsupported futures margin mode.
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

	// ErrEmptySymbols indicates that a stream received no symbols.
	ErrEmptySymbols = errors.New("empty symbols")
	// ErrEmptyTimeframes indicates that a stream received no timeframes.
	ErrEmptyTimeframes = errors.New("empty timeframes")
	// ErrInvalidStreamMode indicates an unsupported candle stream mode.
	ErrInvalidStreamMode = errors.New("invalid stream mode")

	// ErrNotEnoughAmount indicates that the available position or asset amount is too small.
	ErrNotEnoughAmount = errors.New("not enough amount")
	// ErrMinimumAmountUnavailable indicates that the exchange did not publish an order minimum.
	ErrMinimumAmountUnavailable = errors.New("exchange: minimum order amount is unavailable")
	// ErrBalanceNotFound indicates that the requested asset is absent from the balance response.
	ErrBalanceNotFound = errors.New("balance not found")
	// ErrFundingRateUnavailable indicates that an exchange omitted the requested funding rate.
	ErrFundingRateUnavailable = errors.New("funding rate unavailable")
	// ErrConfigAlreadySet indicates that immutable client credentials were already configured.
	ErrConfigAlreadySet = errors.New("config already set")
	// ErrClientNotConfigured indicates that an authenticated operation was attempted before configuration.
	ErrClientNotConfigured = errors.New("client not configured")

	// ErrOperationRejected indicates that the exchange rejected an operation
	// without exposing a more specific normalized error category.
	ErrOperationRejected = errors.New("operation rejected")
)
