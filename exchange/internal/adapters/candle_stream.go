package adapters

import ccxt "github.com/ccxt/ccxt/go/v4"

// CandleStreamAdapter identifies default WatchOHLCV subscriptions and handles
// provider-specific CCXT cleanup. Interval is the provider's timeframe value.
type CandleStreamAdapter interface {
	SubscriptionKey(market ccxt.MarketInterface, timeframe, interval string) string
	// CleanupUnsubscription runs only after the provider confirms unsubscribe.
	CleanupUnsubscription(base *ccxt.BaseExchange, subscriptionKey string)
}

// BinanceCandleStreamAdapter matches Binance's unified OHLCV subscription keys.
type BinanceCandleStreamAdapter struct{}

// SubscriptionKey follows CCXT's WatchOHLCVForSymbols subscription registration.
func (BinanceCandleStreamAdapter) SubscriptionKey(market ccxt.MarketInterface, timeframe, _ string) string {
	if market.Symbol == nil || *market.Symbol == "" {
		return ""
	}
	return "ohlcv::" + *market.Symbol + "::" + timeframe
}

// CleanupUnsubscription is unnecessary for Binance: CCXT removes the same key
// that WatchOHLCV registers and rejects its pending watch future itself.
func (BinanceCandleStreamAdapter) CleanupUnsubscription(_ *ccxt.BaseExchange, _ string) {}

// OKXCandleStreamAdapter matches OKX candle channels using the exchange market ID.
type OKXCandleStreamAdapter struct{}

// SubscriptionKey follows CCXT's WatchOHLCV and Subscribe registration, including
// translated intervals such as 1h -> 1H. A registry entry is not a provider ACK.
func (OKXCandleStreamAdapter) SubscriptionKey(market ccxt.MarketInterface, _, interval string) string {
	if market.Id == nil || *market.Id == "" {
		return ""
	}
	return "candle" + interval + ":" + *market.Id
}

// CleanupUnsubscription works around CCXT Go v4.5.70 cleaning only the multi-pair
// key after OKX acknowledges an OHLCV unsubscribe. The single-pair WatchOHLCV key
// otherwise survives and prevents the next watch from sending a new subscribe.
// Reject its old future as well so the retired watcher cannot share a pending
// result with its replacement. Other keys and the shared socket are preserved.
func (OKXCandleStreamAdapter) CleanupUnsubscription(base *ccxt.BaseExchange, subscriptionKey string) {
	if base == nil || subscriptionKey == "" {
		return
	}
	base.WsClientsMu.Lock()
	defer base.WsClientsMu.Unlock()
	for _, value := range base.Clients {
		ws, ok := value.(*ccxt.WSClient)
		if !ok || ws == nil || ws.Client == nil || ws.Subscriptions == nil {
			continue
		}
		ws.Subscriptions.Delete(subscriptionKey)
		ws.Reject(ccxt.UnsubscribeError("okx "+subscriptionKey), subscriptionKey)
	}
}
