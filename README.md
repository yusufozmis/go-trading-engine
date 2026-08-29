# go-trading-engine

`go-trading-engine` is a work-in-progress Go library for running candle-based
strategies in backtests and connecting the same strategy domain to live exchange
data and order execution.

The library currently includes:

- A stateful strategy engine with explicit decide/confirm execution actions.
- Repeatable backtests over validated, immutable candle data.
- Pluggable position sizing with several built-in sizing policies.
- Spot and perpetual futures operations for Binance and OKX through CCXT.
- Historical candle fetching and multi-symbol candle streams.
- Market and limit entries, position reduction, and position closing.
- Attached take-profit and stop-loss entries on OKX.

> [!WARNING]
> This project is under active development. Its public API may change before the
> first stable release. Test with sandbox accounts and small amounts before using
> it with real funds.

## Requirements

- Go 1.26.4 or newer.
- Exchange API credentials for authenticated account and order operations.
- A Binance or OKX account for the current exchange integrations.

## Installation

```bash
go get github.com/yusufozmis/go-trading-engine
```

## Packages

| Package | Purpose |
| --- | --- |
| `backtester` | Runs a strategy against an immutable historical candle set. |
| `engine` | Evaluates strategy plans and tracks confirmed position state. |
| `engine/options` | Configures optional engine behavior. |
| `engine/positionsizers` | Provides ready-to-use position sizing policies. |
| `exchange` | Provides market data, streams, balances, and order execution. |
| `types` | Defines the shared candle, strategy, plan, and position contracts. |
| `apperrors` | Exposes stable errors for caller-side classification. |

## Backtesting

A strategy implements `types.Strategy`:

```go
type Strategy interface {
	AddBar(candle types.Candle)
	Calculate(ctx types.StrategyContext) (types.PlanUpdate, error)
}
```

Create a position sizer, construct the backtester, and run a fresh strategy
instance:

```go
sizer, err := positionsizers.NewFixedRiskSizer(10)
if err != nil {
	return err
}

tester, err := backtester.NewBacktester(
	"BTC/USDT",
	"5m",
	candles,
	sizer,
	options.WithAutomatedClose(),
	options.WithMaxEntryDeviation(0.02),
)
if err != nil {
	return err
}

result, err := tester.Run(strategy)
if err != nil {
	return err
}

fmt.Printf("TP: %d, SL: %d, profit: %.2f, loss: %.2f\n",
	result.TPCount,
	result.SLCount,
	result.Profit,
	result.Loss,
)
```

Each `Run` starts with a fresh engine, but strategies may retain their own candle
history and state. Pass a fresh strategy instance to every run.

### Position Sizers

The built-in policies are:

- `FixedAmountSizer`: returns the same base-asset amount for every position.
- `FixedNotionalSizer`: converts a fixed quote-currency value into base amount.
- `FixedRiskSizer`: targets a fixed quote-currency loss at the stop-loss price.
- `LeveragedMarginSizer`: sizes from margin, leverage, and stop distance.

Custom policies can implement `types.PositionSizer`:

```go
type PositionSizer interface {
	CalculatePositionSize(position types.Position) (float64, error)
}
```

## Exchange Setup

Constructors load current market metadata and may therefore return an exchange or
network error:

```go
client, err := exchange.NewOKX()
if err != nil {
	return err
}

err = client.SetConfig(exchange.ExchangeConfig{
	APIKey:    os.Getenv("OKX_API_KEY"),
	SecretKey: os.Getenv("OKX_SECRET_KEY"),
	Password:  os.Getenv("OKX_PASSPHRASE"),
})
if err != nil {
	return err
}
```

`SetConfig` may be called only once. Call it before authenticated balance,
position, or order operations, and do not call it concurrently with another
client method.

### Futures

Configure futures account and symbol settings before placing orders:

```go
const symbol = "BTC/USDT:USDT"

err = client.SetFuturesConfig(
	5,
	types.MarginModeIsolated,
	true,
	symbol,
)
if err != nil {
	return err
}

err = client.CreateFuturesMarketOrder(
	symbol,
	types.PositionLong,
	0.001,
)
if err != nil {
	return err
}
```

Public futures amounts are base-asset quantities. The client estimates the
exchange contract count using loaded market metadata; exchange precision can
change the final traded quantity. Inverse futures markets are not currently
supported by this conversion.

`SetFuturesConfig` applies remote position mode, margin mode, and leverage
settings. Changing position mode may be rejected when the account has open
positions or pending orders.

### Attached TP/SL

OKX supports market and limit entry orders with an attached stop-loss,
take-profit, or both:

```go
position := types.Position{
	Symbol:     "BTC/USDT:USDT",
	Timeframe:  "5m",
	Timestamp:  time.Now().UnixMilli(),
	EntryPrice: 60_000,
	StopLoss:   59_000,
	TP:         62_000,
	Amount:     0.001,
	State:      types.LongOpen,
}

err = client.CreateFuturesLimitOrderWithTPSL(position)
```

The market variant is `CreateFuturesMarketOrderWithTPSL`. The entry order may be
market or limit, but the attached protections currently execute as market orders
after their trigger prices are reached. Binance attached TP/SL is currently
unsupported because it requires separate conditional orders.

## Candle Streams

Streams can emit every forming-candle update or only candles confirmed closed by
the arrival of a newer timestamp:

```go
updates, err := client.RunCandleStream(
	[]string{"BTC/USDT"},
	[]string{types.Timeframe5Minutes},
	exchange.ClosedOnly,
)
if err != nil {
	return err
}

for update := range updates {
	if update.Err != nil {
		log.Printf("%s %s stream stopped: %v",
			update.Symbol,
			update.Timeframe,
			update.Err,
		)
		continue
	}

	process(update.Candle)
}
```

Watcher failures are retried with bounded backoff. A terminal failure is sent as
a `CandleUpdate` with `Err`, `Symbol`, and `Timeframe` populated so the caller can
decide whether to subscribe again.

Call `CloseCandleStream` to stop all watchers and close the updates channel.
`RunCandleStream`, `Subscribe`, `Unsubscribe`, and `CloseCandleStream` must not be
called concurrently. Account queries and order methods may be used while candle
updates are being consumed.

Timeframe constants such as `types.Timeframe5Minutes` use CCXT's unified
values. Provider-specific formats are translated by CCXT. Use
`client.SupportedTimeframes()` to discover which unified values the active
provider supports.

## Error Handling

The exchange package translates supported CCXT errors into stable library errors.
Use `errors.Is` to classify them:

```go
err := client.CreateFuturesMarketOrder(symbol, types.PositionLong, 0.001)
if err != nil {
	switch {
	case stderrors.Is(err, apperrors.ErrInsufficientFunds):
		// Reduce the requested amount or add collateral.
	case stderrors.Is(err, apperrors.ErrRateLimitExceeded):
		// Retry according to the application's policy.
	default:
		return err
	}
}
```

```go
import (
	stderrors "errors"

	"github.com/yusufozmis/go-trading-engine/apperrors"
)
```

## Current Scope

- Supported exchange providers: Binance and OKX.
- Binance futures support is currently limited to USD-M linear perpetuals.
- Attached TP/SL entry orders are currently OKX-only.
- Exchange metadata is loaded when a client is constructed.
- Funding rates are returned as percentages.
- `FetchCandles` conservatively excludes the newest fetched candle because it
  may still be forming.

The library is not financial advice. Exchange behavior, fees, precision rules,
minimum order sizes, and API availability can change; callers remain responsible
for validating orders against their account and market requirements.
