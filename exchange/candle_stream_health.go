package exchange

import (
	"math"
	"sync"
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
// It does not confirm that a particular subscription is receiving data.
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
// WebSocket client. A connection can serve multiple subscriptions.
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

	return CandleStreamHealthSnapshot{
		Subscriptions: health,
		Connections:   streamConnectionsHealth(client.streamBaseExchange, ""),
	}, nil
}

// CandleSubscriptionHealth returns the same snapshot shape as CandleStreamHealth,
// filtered to one symbol/timeframe and the CCXT sockets whose subscription registry
// contains that pair's provider-specific key. No requests or connections are made.
// Subscriptions contains exactly one local watcher record; Connections contains
// only matching sockets and their CCXT heartbeat values. No match means an empty
// Connections slice, not a healthy connection. A registered key is not proof of
// provider acknowledgement, and a fresh heartbeat does not prove candle delivery.
// Failed watchers remain visible until resubscribed or unsubscribed. A missing
// pair returns ErrCandleSubscriptionNotFound; a closed stream returns
// ErrStreamNotRunning. Watcher and socket observations are not atomic.
func (client *Client) CandleSubscriptionHealth(symbol, timeframe string) (CandleStreamHealthSnapshot, error) {
	if err := client.Validate(); err != nil {
		return CandleStreamHealthSnapshot{}, err
	}
	if symbol == "" {
		return CandleStreamHealthSnapshot{}, apperrors.ErrNilSymbol
	}
	if timeframe == "" {
		return CandleStreamHealthSnapshot{}, apperrors.ErrNilTimeframe
	}

	client.candleMu.Lock()
	stream := client.stream
	client.candleMu.Unlock()
	if stream == nil {
		return CandleStreamHealthSnapshot{}, apperrors.ErrStreamNotRunning
	}

	stream.mu.Lock()
	health, exists := stream.health[newSubKey(symbol, timeframe)]
	closed := stream.closed
	stream.mu.Unlock()
	if closed {
		return CandleStreamHealthSnapshot{}, apperrors.ErrStreamNotRunning
	}
	if !exists {
		return CandleStreamHealthSnapshot{}, apperrors.ErrCandleSubscriptionNotFound
	}
	if client.candleStreamAdapter == nil {
		return CandleStreamHealthSnapshot{}, apperrors.ErrUnsupportedProvider
	}
	market, exists := client.markets[symbol]
	if !exists {
		return CandleStreamHealthSnapshot{}, apperrors.ErrInvalidSymbol
	}
	interval, ok := client.streamExchange.GetTimeframes()[timeframe].(string)
	if !ok {
		interval = timeframe
	}
	key := client.candleStreamAdapter.SubscriptionKey(market, timeframe, interval)
	if key == "" {
		return CandleStreamHealthSnapshot{}, apperrors.ErrInvalidSymbol
	}
	return CandleStreamHealthSnapshot{
		Subscriptions: []CandleSubscriptionHealth{health},
		Connections:   streamConnectionsHealth(client.streamBaseExchange, key),
	}, nil
}

// streamConnectionsHealth inspects existing sockets without opening connections
// or issuing requests. A non-empty key restricts results to sockets with that
// exact CCXT subscription. An empty key returns every socket in the registry.
func streamConnectionsHealth(base *ccxt.BaseExchange, subscriptionKey string) []StreamConnectionHealth {
	if base == nil {
		return nil
	}
	// Client(url) would create a client when absent; inspecting the registry must
	// remain read-only. CCXT protects this registry with WsClientsMu.
	base.WsClientsMu.Lock()
	defer base.WsClientsMu.Unlock()
	now := time.Now()
	var connections []StreamConnectionHealth
	for _, value := range base.Clients {
		if ws, ok := value.(*ccxt.WSClient); ok && ws != nil && ws.Client != nil {
			if subscriptionKey != "" {
				subscriptions, ok := ws.GetSubscriptions().(*sync.Map)
				if !ok || subscriptions == nil {
					continue
				}
				if _, exists := subscriptions.Load(subscriptionKey); !exists {
					continue
				}
			}
			connections = append(connections, connectionHeartbeat(ws, now))
		}
	}
	return connections
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
