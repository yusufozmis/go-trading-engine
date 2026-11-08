package exchange

import (
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
type OrderSubscriptionHealthSnapshot struct {
	Symbol  string
	Regular OrderWatcherHealthSnapshot
	// Trigger is nil when the provider/market has no separate conditional watcher.
	Trigger *OrderWatcherHealthSnapshot
}

// OrderStreamHealthSnapshot separates regular/conditional watcher activity from
// socket heartbeat timing. The slices are independent snapshots in unspecified order.
type OrderStreamHealthSnapshot struct {
	Subscriptions []OrderSubscriptionHealth
	// Empty Connections means no socket health is available, not healthy sockets.
	Connections []StreamConnectionHealth
}

// OrderStreamHealth reads local watcher state and existing CCXT heartbeats
// without network requests. It is safe during stream activity and shutdown.
// SubscribeOrder and UnsubscribeOrder change only local symbol filters, so they
// do not add or remove watcher entries here. A closed stream returns
// ErrOrderStreamNotRunning; no final snapshot is retained. Heartbeats do not prove
// authentication, subscription readiness, or order delivery.
func (client *Client) OrderStreamHealth() (OrderStreamHealthSnapshot, error) {
	if err := client.Validate(); err != nil {
		return OrderStreamHealthSnapshot{}, err
	}
	client.orderMu.Lock()
	stream := client.orderStream
	client.orderMu.Unlock()
	if stream == nil {
		return OrderStreamHealthSnapshot{}, apperrors.ErrOrderStreamNotRunning
	}

	stream.mu.Lock()
	health := make([]OrderSubscriptionHealth, 0, len(stream.health))
	for _, subscription := range stream.health {
		health = append(health, subscription)
	}
	closed := stream.closed
	stream.mu.Unlock()
	if closed {
		return OrderStreamHealthSnapshot{}, apperrors.ErrOrderStreamNotRunning
	}
	return OrderStreamHealthSnapshot{
		Subscriptions: health,
		Connections:   streamConnectionsHealth(client.orderBaseExchange, ""),
	}, nil
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
	regular, hasRegular := stream.health[false]
	trigger, hasTrigger := stream.health[true]
	stream.mu.Unlock()
	if closed {
		return OrderSubscriptionHealthSnapshot{}, apperrors.ErrOrderStreamNotRunning
	}
	market, exists := client.markets[symbol]
	if !exists || !isOrderStreamMarket(market, marketType) {
		return OrderSubscriptionHealthSnapshot{}, apperrors.ErrInvalidSymbol
	}
	if !allowed || !hasRegular {
		return OrderSubscriptionHealthSnapshot{}, apperrors.ErrOrderSubscriptionNotFound
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
		health := regular
		if config.Trigger {
			if !hasTrigger {
				return OrderSubscriptionHealthSnapshot{}, apperrors.ErrOrderSubscriptionNotFound
			}
			health = trigger
		}
		key := client.orderStreamAdapter.SubscriptionKey(config)
		connections := streamConnectionsHealth(client.orderBaseExchange, key)
		if len(connections) > 1 {
			return OrderSubscriptionHealthSnapshot{}, apperrors.ErrAmbiguousOrderConnection
		}
		watcher := OrderWatcherHealthSnapshot{Subscription: health}
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
