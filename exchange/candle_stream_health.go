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

// CandleSubscriptionHealthSnapshot contains one watcher's state and the heartbeat
// of its matching CCXT socket. The two observations are not atomic.
type CandleSubscriptionHealthSnapshot struct {
	Subscription CandleSubscriptionHealth
	// Connection is nil when no matching socket is registered yet. Nil does not
	// mean healthy; a non-nil connection can still have unknown heartbeat timing.
	Connection *StreamConnectionHealth
}

// CandleStreamHealth returns one snapshot per retained symbol/timeframe in
// unspecified order. Each entry pairs local watcher state with its matching CCXT
// socket heartbeat; pairs sharing a socket each receive an independent copy.
// No connections or requests are made. It can run concurrently with stream
// activity and shutdown. Failed watchers remain until resubscribed or unsubscribed.
// A nil Connection means no matching socket is registered, not a healthy socket.
// ErrStreamNotRunning means the stream is absent or closing/closed. Multiple
// sockets matching one pair return ErrAmbiguousCandleConnection.
// Heartbeats do not prove subscription readiness or candle delivery.
func (client *Client) CandleStreamHealth() ([]CandleSubscriptionHealthSnapshot, error) {
	if err := client.Validate(); err != nil {
		return nil, err
	}
	client.candleMu.Lock()
	stream := client.stream
	client.candleMu.Unlock()
	if stream == nil {
		return nil, apperrors.ErrStreamNotRunning
	}

	stream.mu.Lock()
	health := make([]CandleSubscriptionHealth, 0, len(stream.health))
	for _, subscription := range stream.health {
		health = append(health, subscription)
	}
	closed := stream.closed
	stream.mu.Unlock()
	if closed {
		return nil, apperrors.ErrStreamNotRunning
	}

	snapshots := make([]CandleSubscriptionHealthSnapshot, 0, len(health))
	for _, subscription := range health {
		snapshot, err := client.candleSubscriptionHealthSnapshot(subscription)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

// CandleSubscriptionHealth returns one symbol/timeframe's watcher state and its
// matching CCXT socket heartbeat without making requests or opening connections.
// Connection is nil when no matching socket is registered. Multiple matches return
// ErrAmbiguousCandleConnection rather than selecting an arbitrary socket.
// A registered key is not proof of provider acknowledgement, and a fresh heartbeat
// does not prove candle delivery.
// Failed watchers remain visible until resubscribed or unsubscribed. A missing
// pair returns ErrCandleSubscriptionNotFound; a closed stream returns
// ErrStreamNotRunning. Watcher and socket observations are not atomic.
func (client *Client) CandleSubscriptionHealth(symbol, timeframe string) (CandleSubscriptionHealthSnapshot, error) {
	if err := client.Validate(); err != nil {
		return CandleSubscriptionHealthSnapshot{}, err
	}
	if symbol == "" {
		return CandleSubscriptionHealthSnapshot{}, apperrors.ErrNilSymbol
	}
	if timeframe == "" {
		return CandleSubscriptionHealthSnapshot{}, apperrors.ErrNilTimeframe
	}

	client.candleMu.Lock()
	stream := client.stream
	client.candleMu.Unlock()
	if stream == nil {
		return CandleSubscriptionHealthSnapshot{}, apperrors.ErrStreamNotRunning
	}

	stream.mu.Lock()
	health, exists := stream.health[newSubKey(symbol, timeframe)]
	closed := stream.closed
	stream.mu.Unlock()
	if closed {
		return CandleSubscriptionHealthSnapshot{}, apperrors.ErrStreamNotRunning
	}
	if !exists {
		return CandleSubscriptionHealthSnapshot{}, apperrors.ErrCandleSubscriptionNotFound
	}
	return client.candleSubscriptionHealthSnapshot(health)
}

// candleSubscriptionHealthSnapshot pairs the copied watcher record with its
// current socket so single-pair and all-pair queries use identical matching rules.
func (client *Client) candleSubscriptionHealthSnapshot(health CandleSubscriptionHealth) (CandleSubscriptionHealthSnapshot, error) {
	if client.candleStreamAdapter == nil {
		return CandleSubscriptionHealthSnapshot{}, apperrors.ErrUnsupportedProvider
	}
	market, exists := client.markets[health.Symbol]
	if !exists {
		return CandleSubscriptionHealthSnapshot{}, apperrors.ErrInvalidSymbol
	}
	interval, ok := client.streamExchange.GetTimeframes()[health.Timeframe].(string)
	if !ok {
		interval = health.Timeframe
	}
	key := client.candleStreamAdapter.SubscriptionKey(market, health.Timeframe, interval)
	if key == "" {
		return CandleSubscriptionHealthSnapshot{}, apperrors.ErrInvalidSymbol
	}
	connections := streamConnectionsHealth(client.streamBaseExchange, key)
	if len(connections) > 1 {
		return CandleSubscriptionHealthSnapshot{}, apperrors.ErrAmbiguousCandleConnection
	}
	snapshot := CandleSubscriptionHealthSnapshot{Subscription: health}
	if len(connections) == 1 {
		snapshot.Connection = &connections[0]
	}
	return snapshot, nil
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
