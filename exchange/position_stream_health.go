package exchange

import (
	"time"

	"github.com/yusufozmis/go-trading-engine/apperrors"
)

// PositionSubscriptionHealth describes one local position watcher. An empty
// Symbol identifies the all-positions subscription, including its local exclusions.
type PositionSubscriptionHealth struct {
	Symbol string
	Status StreamStatus
	// LastUpdate is the local receipt time of the latest non-empty CCXT result,
	// even if its positions were excluded from the output. Zero means no data yet.
	// Quiet private streams are not automatically considered stale.
	LastUpdate time.Time
	// RetryCount counts retries since the latest successful watch call.
	RetryCount int
	// LastError retains the latest normalized error, including after recovery.
	LastError error
}

// PositionSubscriptionHealthSnapshot contains the requested symbol's local watcher
// record and its matching CCXT heartbeat. Subscription.Symbol is empty when the
// symbol is covered by the all-positions watcher; its metrics are then account-wide.
// Symbol itself is empty for an all-positions entry returned by PositionStreamHealth.
type PositionSubscriptionHealthSnapshot struct {
	Symbol       string
	Subscription PositionSubscriptionHealth
	// Connection is nil when no matching socket is registered. Nil is not healthy.
	Connection *StreamConnectionHealth
}

// PositionStreamHealth returns one snapshot per retained position watcher in
// unspecified order. Each contains local watcher state and its matching CCXT
// heartbeat. An all-positions watcher is represented once with Symbol empty;
// its current symbol exclusions still apply to delivery and per-symbol queries.
// No requests or connections are made. Failed watchers remain until resubscribed
// or unsubscribed. A closed stream returns ErrPositionStreamNotRunning.
// Heartbeats do not prove authentication, subscription readiness, or delivery.
func (client *Client) PositionStreamHealth() ([]PositionSubscriptionHealthSnapshot, error) {
	if err := client.Validate(); err != nil {
		return nil, err
	}
	client.positionMu.Lock()
	stream := client.positionStream
	client.positionMu.Unlock()
	if stream == nil {
		return nil, apperrors.ErrPositionStreamNotRunning
	}

	stream.mu.Lock()
	health := make([]PositionSubscriptionHealth, 0, len(stream.health))
	for _, subscription := range stream.health {
		health = append(health, subscription)
	}
	closed := stream.closed
	stream.mu.Unlock()
	if closed {
		return nil, apperrors.ErrPositionStreamNotRunning
	}
	snapshots := make([]PositionSubscriptionHealthSnapshot, 0, len(health))
	for _, subscription := range health {
		snapshot, err := client.positionSubscriptionHealthSnapshot(subscription.Symbol, subscription)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

// PositionSubscriptionHealth returns one symbol's watcher and matching CCXT socket
// without making requests or opening connections. Explicit subscriptions and the
// all-positions subscription are supported; excluded symbols return
// ErrPositionSubscriptionNotFound. Failed watcher records remain queryable until
// resubscribed or unsubscribed. LastUpdate belongs to the selected watcher, not
// necessarily the requested symbol, and silence alone is not considered failure.
// Multiple matching sockets return ErrAmbiguousPositionConnection. A heartbeat
// does not prove authentication, subscription readiness, or position delivery.
// Watcher and socket observations are not atomic.
func (client *Client) PositionSubscriptionHealth(symbol string) (PositionSubscriptionHealthSnapshot, error) {
	if err := client.Validate(); err != nil {
		return PositionSubscriptionHealthSnapshot{}, err
	}
	if symbol == "" {
		return PositionSubscriptionHealthSnapshot{}, apperrors.ErrNilSymbol
	}
	client.positionMu.Lock()
	stream := client.positionStream
	client.positionMu.Unlock()
	if stream == nil {
		return PositionSubscriptionHealthSnapshot{}, apperrors.ErrPositionStreamNotRunning
	}

	stream.mu.Lock()
	health, exists := stream.health[symbol]
	if !exists {
		health, exists = stream.health[""]
	}
	excluded := stream.excludedSymbols[symbol]
	closed := stream.closed
	stream.mu.Unlock()
	if closed {
		return PositionSubscriptionHealthSnapshot{}, apperrors.ErrPositionStreamNotRunning
	}
	if !exists || excluded {
		return PositionSubscriptionHealthSnapshot{}, apperrors.ErrPositionSubscriptionNotFound
	}
	return client.positionSubscriptionHealthSnapshot(symbol, health)
}

// positionSubscriptionHealthSnapshot shares socket matching between individual
// and full-stream queries. Empty symbol represents the all-positions scope.
func (client *Client) positionSubscriptionHealthSnapshot(symbol string, health PositionSubscriptionHealth) (PositionSubscriptionHealthSnapshot, error) {
	if client.positionStreamAdapter == nil {
		return PositionSubscriptionHealthSnapshot{}, apperrors.ErrUnsupportedProvider
	}
	market, exists := client.markets[symbol]
	if symbol != "" && (!exists || market.Swap == nil || !*market.Swap) {
		return PositionSubscriptionHealthSnapshot{}, apperrors.ErrInvalidSymbol
	}
	key := client.positionStreamAdapter.SubscriptionKey(health.Symbol, market)
	if key == "" {
		return PositionSubscriptionHealthSnapshot{}, apperrors.ErrPositionSubscriptionNotFound
	}
	connections := streamConnectionsHealth(client.positionBaseExchange, key)
	if len(connections) > 1 {
		return PositionSubscriptionHealthSnapshot{}, apperrors.ErrAmbiguousPositionConnection
	}
	snapshot := PositionSubscriptionHealthSnapshot{Symbol: symbol, Subscription: health}
	if len(connections) == 1 {
		snapshot.Connection = &connections[0]
	}
	return snapshot, nil
}

// recordHealth ignores late results from an unsubscribed or replaced watcher.
func (stream *positionStream) recordHealth(symbol string, owner <-chan struct{}, health PositionSubscriptionHealth) bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closed || stream.subscriptions[symbol] != owner {
		return false
	}
	stream.health[symbol] = health
	return true
}
