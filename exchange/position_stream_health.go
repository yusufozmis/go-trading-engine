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

// PositionStreamHealthSnapshot separates watcher activity from socket heartbeat
// timing. The slices are independent snapshots in unspecified order.
type PositionStreamHealthSnapshot struct {
	Subscriptions []PositionSubscriptionHealth
	// Empty Connections means no socket health is available, not healthy sockets.
	Connections []StreamConnectionHealth
}

// PositionStreamHealth reads local watcher state and existing CCXT heartbeats
// without network requests. It is safe during stream activity and shutdown.
// Failed watchers remain visible until resubscribed or unsubscribed. A closed
// stream returns ErrPositionStreamNotRunning; no final snapshot is retained.
// Heartbeats do not prove authentication, subscription readiness, or delivery.
func (client *Client) PositionStreamHealth() (PositionStreamHealthSnapshot, error) {
	if err := client.Validate(); err != nil {
		return PositionStreamHealthSnapshot{}, err
	}
	client.positionMu.Lock()
	stream := client.positionStream
	client.positionMu.Unlock()
	if stream == nil {
		return PositionStreamHealthSnapshot{}, apperrors.ErrPositionStreamNotRunning
	}

	stream.mu.Lock()
	health := make([]PositionSubscriptionHealth, 0, len(stream.health))
	for _, subscription := range stream.health {
		health = append(health, subscription)
	}
	closed := stream.closed
	stream.mu.Unlock()
	if closed {
		return PositionStreamHealthSnapshot{}, apperrors.ErrPositionStreamNotRunning
	}
	return PositionStreamHealthSnapshot{
		Subscriptions: health,
		Connections:   streamConnectionsHealth(client.positionBaseExchange),
	}, nil
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
