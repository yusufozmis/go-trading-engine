package exchange

import (
	"math"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
)

// StreamStatus describes local watcher activity, not WebSocket connectivity or
// successful provider subscription acknowledgement.
type StreamStatus string

const (
	// StreamStarting means the watcher has not returned successfully yet.
	StreamStarting StreamStatus = "starting"
	// StreamRunning means the latest watch call succeeded. It does not guarantee
	// that the connection is still alive while the next call is waiting.
	StreamRunning StreamStatus = "running"
	// StreamRetrying means a watch call failed and recovery is in progress.
	StreamRetrying StreamStatus = "retrying"
	// StreamStopped means the watcher stopped after a terminal failure.
	StreamStopped StreamStatus = "stopped"
)

// CandleSubscriptionHealth is a snapshot of one candle watcher's current generation.
// Subscribe resets the snapshot when starting a replacement watcher.
type CandleSubscriptionHealth struct {
	Symbol    string
	Timeframe string
	Status    StreamStatus
	// LastUpdate is the local receipt time of the latest non-empty CCXT result,
	// including forming candles suppressed by ClosedOnly. Zero means no data yet.
	// No automatic stale timeout is applied: silence alone does not prove failure.
	LastUpdate time.Time
	// RetryCount counts retries started since the last successful watch call.
	RetryCount int
	// LastError retains the latest normalized watch error, even after recovery.
	// Inspect Status to distinguish an ongoing failure from a recovered one.
	LastError error
}

// HeartbeatStatus describes the freshness of CCXT's recorded WebSocket heartbeat.
// It does not confirm that a particular candle subscription is receiving data.
type HeartbeatStatus string

const (
	// HeartbeatUnknown means heartbeat timing is not available or enabled.
	HeartbeatUnknown HeartbeatStatus = "unknown"
	// HeartbeatHealthy means the recorded heartbeat is within CCXT's timeout window.
	HeartbeatHealthy HeartbeatStatus = "healthy"
	// HeartbeatStale means the recorded heartbeat exceeds CCXT's timeout window.
	HeartbeatStale HeartbeatStatus = "stale"
)

// StreamConnectionHealth contains heartbeat data read from an existing CCXT
// WebSocket client. A connection can serve multiple candle subscriptions.
type StreamConnectionHealth struct {
	URL    string
	Status HeartbeatStatus
	// LastPong is CCXT's heartbeat timestamp, not a candle timestamp. CCXT may
	// initialize this value before the first pong, so it is not proof of a reply.
	LastPong time.Time
	// KeepAlive is CCXT's configured ping interval.
	KeepAlive time.Duration
	// HeartbeatTimeout follows CCXT's keepAlive * integer(maxPingPongMisses) rule.
	// Zero means no usable timeout could be determined.
	HeartbeatTimeout time.Duration
}

// CandleStreamHealthSnapshot separates subscription activity from the shared
// WebSocket connections' heartbeat state. Both slices are independent snapshots
// in unspecified order; their observations are not an atomic provider snapshot.
type CandleStreamHealthSnapshot struct {
	Subscriptions []CandleSubscriptionHealth
	// Connections contains only clients currently registered in CCXT. An empty
	// slice means no connection health is available, never "all connections healthy".
	Connections []StreamConnectionHealth
}

// CandleStreamHealth reads watcher state and the dedicated candle exchange's
// existing CCXT WebSocket clients without creating connections or sending requests.
// It can run concurrently with stream activity and shutdown. Failed watchers stay
// visible until resubscribed or unsubscribed; successful unsubscribe removes the entry.
// ErrStreamNotRunning means the stream has not started or is closing/closed.
// Recent heartbeats do not prove subscription readiness or candle delivery;
// inspect LastUpdate separately.
func (client *Client) CandleStreamHealth() (CandleStreamHealthSnapshot, error) {
	if err := client.Validate(); err != nil {
		return CandleStreamHealthSnapshot{}, err
	}
	client.candleMu.Lock()
	stream := client.stream
	client.candleMu.Unlock()
	if stream == nil {
		return CandleStreamHealthSnapshot{}, apperrors.ErrStreamNotRunning
	}

	stream.mu.Lock()
	health := make([]CandleSubscriptionHealth, 0, len(stream.health))
	for _, subscription := range stream.health {
		health = append(health, subscription)
	}
	closed := stream.closed
	stream.mu.Unlock()
	if closed {
		return CandleStreamHealthSnapshot{}, apperrors.ErrStreamNotRunning
	}

	base := client.streamBaseExchange
	snapshot := CandleStreamHealthSnapshot{Subscriptions: health}
	if base == nil {
		return snapshot, nil
	}
	// Client(url) would create a client when absent; inspecting the registry must
	// remain read-only. CCXT protects this registry with WsClientsMu.
	base.WsClientsMu.Lock()
	defer base.WsClientsMu.Unlock()
	now := time.Now()
	for _, value := range base.Clients {
		if ws, ok := value.(*ccxt.WSClient); ok && ws != nil && ws.Client != nil {
			snapshot.Connections = append(snapshot.Connections, connectionHeartbeat(ws, now))
		}
	}
	return snapshot, nil
}

// connectionHeartbeat mirrors CCXT's timeout calculation without reading its
// unsynchronized IsConnected/Error fields or altering its heartbeat machinery.
func connectionHeartbeat(ws *ccxt.WSClient, now time.Time) StreamConnectionHealth {
	health := StreamConnectionHealth{URL: ws.GetUrl(), Status: HeartbeatUnknown}
	timestamp, hasPong := ws.GetLastPong().(int64)
	if hasPong && timestamp > 0 {
		health.LastPong = time.UnixMilli(timestamp)
	}
	interval, ok := ws.GetKeepAlive().(int64)
	if !ok || interval <= 0 || interval > math.MaxInt64/int64(time.Millisecond) {
		return health
	}
	health.KeepAlive = time.Duration(interval) * time.Millisecond
	misses := 2.0 // CCXT's default when no float64 override is configured.
	if configured, ok := ws.MaxPingPongMisses.(float64); ok {
		misses = math.Trunc(configured)
	}
	if math.IsNaN(misses) || misses < 1 || misses > float64(math.MaxInt64/int64(health.KeepAlive)) {
		return health
	}
	health.HeartbeatTimeout = health.KeepAlive * time.Duration(misses)
	if health.LastPong.IsZero() || now.Before(health.LastPong) {
		return health
	}
	health.Status = HeartbeatHealthy
	if now.Sub(health.LastPong) > health.HeartbeatTimeout {
		health.Status = HeartbeatStale
	}
	return health
}

// recordHealth rejects late results from an unsubscribed or replaced watcher.
func (stream *candleStream) recordHealth(key subKey, token uint64, health CandleSubscriptionHealth) bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closed || stream.activeSymbols[key] != token {
		return false
	}
	stream.health[key] = health
	return true
}
