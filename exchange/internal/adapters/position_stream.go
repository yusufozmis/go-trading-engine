package adapters

import ccxt "github.com/ccxt/ccxt/go/v4"

// PositionStreamAdapter identifies the CCXT socket subscription used by a
// logical position watcher. An empty symbol identifies the all-positions watcher.
// An empty key means that watcher does not cover the requested market.
type PositionStreamAdapter interface {
	SubscriptionKey(symbol string, market ccxt.MarketInterface) string
}

// OKXPositionStreamAdapter identifies OKX's account-wide or per-symbol watches.
type OKXPositionStreamAdapter struct{}

// SubscriptionKey follows WatchPositions and SubscribeMultiple. The per-symbol
// key uses the unified symbol, not the exchange's instrument ID.
func (OKXPositionStreamAdapter) SubscriptionKey(symbol string, _ ccxt.MarketInterface) string {
	if symbol == "" {
		return "positions"
	}
	return "positions::" + symbol
}

// BinancePositionStreamAdapter identifies Binance's shared user-data stream.
type BinancePositionStreamAdapter struct{}

// SubscriptionKey follows WatchPositions, which registers the market type rather
// than its symbol-specific result hash. This library's default all-positions watch
// uses USD-M (future); an individual inverse watch uses COIN-M (delivery).
func (BinancePositionStreamAdapter) SubscriptionKey(symbol string, market ccxt.MarketInterface) string {
	if market.Inverse != nil && *market.Inverse {
		if symbol == "" {
			return ""
		}
		return "delivery"
	}
	return "future"
}
