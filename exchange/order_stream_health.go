package exchange

import (
	"maps"
	"time"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// OrderSubscriptionHealth describes an account-wide provider order watcher.
// Symbol filtering does not create or remove these provider subscriptions.
type OrderSubscriptionHealth struct {
	MarketType types.MarketType
	// Trigger distinguishes the provider's separate conditional-order watcher.
	Trigger bool
	Status  StreamStatus
	// LastUpdate is the local receipt time of the latest non-empty CCXT result,
	// including orders removed by local symbol filtering. Zero means no data yet.
	// A quiet order stream is not automatically considered stale.
	LastUpdate time.Time
	// RetryCount counts retries since the latest successful watch call.
	RetryCount int
	// LastError retains the latest normalized error, including after recovery.
	LastError error
}

// OrderWatcherHealthSnapshot pairs a regular or conditional watcher's state with
// its matching CCXT socket heartbeat. Watcher metrics are account-wide, not per symbol.
type OrderWatcherHealthSnapshot struct {
	Subscription OrderSubscriptionHealth
	// Connection is nil when no matching socket is registered. Nil is not healthy.
	Connection *StreamConnectionHealth
}

// OrderSubscriptionHealthSnapshot describes the account-wide order watchers
// delivering updates for one locally allowed symbol. The observations are not atomic.
// Symbol is empty for the all-symbols entry returned by OrderStreamHealth.
type OrderSubscriptionHealthSnapshot struct {
	Symbol  string
	Regular OrderWatcherHealthSnapshot
	// Trigger is nil when the provider/market has no separate conditional watcher.
	Trigger *OrderWatcherHealthSnapshot
}

// OrderStreamHealth returns one snapshot per locally allowed symbol in unspecified
// order, pairing regular and optional TP/SL watcher state with matching heartbeats.
// All-symbols mode returns one entry with Symbol empty, subject to the current
// local exclusions; it does not enumerate exchange markets. Muted symbols are
// omitted in explicit-symbol mode. Metrics remain account-wide and snapshots are
// independent copies. No requests or connections are made. A closed stream returns
// ErrOrderStreamNotRunning. Heartbeats do not prove readiness or order delivery.
func (client *Client) OrderStreamHealth() ([]OrderSubscriptionHealthSnapshot, error) {
	if err := client.Validate(); err != nil {
		return nil, err
	}
	client.orderMu.Lock()
	stream := client.orderStream
	client.orderMu.Unlock()
	if stream == nil {
		return nil, apperrors.ErrOrderStreamNotRunning
	}

	stream.mu.Lock()
	symbols := make([]string, 0, len(stream.symbolSubscriptions))
	if stream.watchAllSymbols {
		symbols = append(symbols, "")
	} else {
		for symbol, allowed := range stream.symbolSubscriptions {
			if allowed {
				symbols = append(symbols, symbol)
			}
		}
	}
	health := maps.Clone(stream.health)
	marketType := stream.marketType
	closed := stream.closed
	stream.mu.Unlock()
	if closed {
		return nil, apperrors.ErrOrderStreamNotRunning
	}
	snapshots := make([]OrderSubscriptionHealthSnapshot, 0, len(symbols))
	for _, symbol := range symbols {
		snapshot, err := client.orderSubscriptionHealthSnapshot(symbol, marketType, health)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

// OrderSubscriptionHealth returns one symbol's regular and, when configured,
// separate TP/SL watcher health without requests or new connections. Symbols
// muted by UnsubscribeOrder or outside the local filter return
// ErrOrderSubscriptionNotFound. LastUpdate and retry state belong to each shared
// watcher, not only the requested symbol. Multiple sockets matching one watcher
// return ErrAmbiguousOrderConnection. A heartbeat does not prove authentication,
// subscription readiness, or order delivery.
func (client *Client) OrderSubscriptionHealth(symbol string) (OrderSubscriptionHealthSnapshot, error) {
	if err := client.Validate(); err != nil {
		return OrderSubscriptionHealthSnapshot{}, err
	}
	if symbol == "" {
		return OrderSubscriptionHealthSnapshot{}, apperrors.ErrNilSymbol
	}
	client.orderMu.Lock()
	stream := client.orderStream
	client.orderMu.Unlock()
	if stream == nil {
		return OrderSubscriptionHealthSnapshot{}, apperrors.ErrOrderStreamNotRunning
	}

	stream.mu.Lock()
	closed := stream.closed
	marketType := stream.marketType
	allowed, explicit := stream.symbolSubscriptions[symbol]
	if stream.watchAllSymbols && !explicit {
		allowed = true
	}
	health := maps.Clone(stream.health)
	stream.mu.Unlock()
	if closed {
		return OrderSubscriptionHealthSnapshot{}, apperrors.ErrOrderStreamNotRunning
	}
	if !allowed {
		return OrderSubscriptionHealthSnapshot{}, apperrors.ErrOrderSubscriptionNotFound
	}
	return client.orderSubscriptionHealthSnapshot(symbol, marketType, health)
}

// orderSubscriptionHealthSnapshot shares watcher/socket matching between both
// query methods. Empty symbol describes the all-symbols local filter.
func (client *Client) orderSubscriptionHealthSnapshot(symbol string, marketType types.MarketType, health map[bool]OrderSubscriptionHealth) (OrderSubscriptionHealthSnapshot, error) {
	market, exists := client.markets[symbol]
	if symbol != "" && (!exists || !isOrderStreamMarket(market, marketType)) {
		return OrderSubscriptionHealthSnapshot{}, apperrors.ErrInvalidSymbol
	}
	if client.orderStreamAdapter == nil {
		return OrderSubscriptionHealthSnapshot{}, apperrors.ErrUnsupportedProvider
	}
	configs, err := client.orderStreamAdapter.WatcherConfigs(marketType)
	if err != nil {
		return OrderSubscriptionHealthSnapshot{}, err
	}
	snapshot := OrderSubscriptionHealthSnapshot{Symbol: symbol}
	for _, config := range configs {
		subscription, exists := health[config.Trigger]
		if !exists {
			return OrderSubscriptionHealthSnapshot{}, apperrors.ErrOrderSubscriptionNotFound
		}
		key := client.orderStreamAdapter.SubscriptionKey(config)
		connections := streamConnectionsHealth(client.orderBaseExchange, key)
		if len(connections) > 1 {
			return OrderSubscriptionHealthSnapshot{}, apperrors.ErrAmbiguousOrderConnection
		}
		watcher := OrderWatcherHealthSnapshot{Subscription: subscription}
		if len(connections) == 1 {
			watcher.Connection = &connections[0]
		}
		if config.Trigger {
			snapshot.Trigger = &watcher
		} else {
			snapshot.Regular = watcher
		}
	}
	return snapshot, nil
}

// recordHealth rejects late results from retired watcher groups.
func (stream *orderStream) recordHealth(owner <-chan struct{}, health OrderSubscriptionHealth) bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closed {
		return false
	}
	select {
	case <-owner:
		return false
	default:
	}
	stream.health[health.Trigger] = health
	return true
}
