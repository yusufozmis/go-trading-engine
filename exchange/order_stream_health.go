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
		Connections:   streamConnectionsHealth(client.orderBaseExchange),
	}, nil
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
