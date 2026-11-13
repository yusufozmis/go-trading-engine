package exchange

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/exchange/internal/adapters"
	"github.com/yusufozmis/go-trading-engine/types"
)

// OrderUpdate contains an exchange-reported order state from submission,
// fetching, or streaming, or a terminal watcher error. A submission response is
// not proof of a fill; optional fields remain nil when omitted by the provider.
type OrderUpdate struct {
	ID            string
	ClientOrderID string
	Symbol        string
	Type          string
	Side          string
	Status        string
	Timestamp     *int64
	// PositionSide is the provider's hedge side; net/both identify one-way mode.
	PositionSide types.PositionSide
	// MarginMode is empty when the provider omits the order's margin setting.
	MarginMode types.MarginMode
	// ClosePosition reports a close-all order, including OKX closeFraction=1.
	ClosePosition bool

	Price   *float64
	Average *float64
	// Amount, Filled, and Remaining use the exchange's order units: base assets
	// for spot and contracts for futures. Unlike futures method inputs, these
	// returned quantities are not converted to base-asset amounts.
	Amount    *float64
	Filled    *float64
	Remaining *float64
	Cost      *float64

	TriggerPrice    *float64
	StopLossPrice   *float64
	TakeProfitPrice *float64
	ReduceOnly      *bool
	PostOnly        *bool
	FeeRate         *float64
	FeeCost         *float64

	// ActualSide classifies a triggered protection as ClosedByProfit or
	// ClosedByStop; it is UNSPECIFIED when the event cannot identify one.
	// OKX reports the triggered leg directly; Binance uses the original order
	// type together with closing direction and execution/trigger evidence.
	// This is a reason classification, not proof that the entire position is
	// closed: a triggered order can remain unfilled or close only part of it.
	ActualSide types.PositionState
	// TriggeredOrderIDs contains the execution order IDs created by the algo
	// order (OKX ordIdList). Match these against a regular order update's ID to
	// obtain its actual fill. The list is nil when absent or empty.
	TriggeredOrderIDs []string

	// Trigger identifies updates received from a provider's separate
	// conditional-order channel, such as OKX orders-algo.
	Trigger bool
	// Err is non-nil only when the identified watcher has stopped permanently.
	Err error
}

type orderStream struct {
	// Only the supervisor publishes here. Cleanup closes the public channel
	// after it exits; regular and trigger readers forward results internally.
	updates chan OrderUpdate
	// done stops the public stream and prevents further reconnect attempts.
	done chan struct{}
	// closeDone publishes closeErr once CCXT cleanup is complete and the publisher has stopped.
	closeDone chan struct{}
	closeErr  error

	mu                  sync.Mutex
	closed              bool
	marketType          types.MarketType
	watchAllSymbols     bool
	symbolSubscriptions map[string]bool
	health              map[bool]OrderSubscriptionHealth

	// Only the supervisor is counted; a blocked CCXT watcher must not make the
	// public CloseOrderStream call wait indefinitely.
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// orderWatchEvent carries a result to the supervisor. Trigger identifies which
// provider subscription failed when a terminal error is sent to the caller.
type orderWatchEvent struct {
	update  *OrderUpdate
	err     error
	trigger bool
}

func (stream *orderStream) isClosed() bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	return stream.closed
}

func (stream *orderStream) watches(symbol string) bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()

	subscribed, exists := stream.symbolSubscriptions[symbol]
	if stream.watchAllSymbols && !exists {
		return true
	}

	return subscribed
}

func (stream *orderStream) emit(update OrderUpdate) bool {
	// Shutdown releases publication even when the caller has stopped reading.
	select {
	case <-stream.done:
		return false
	case stream.updates <- update:
		return true
	}
}

func (stream *orderStream) waitForRetry(delay time.Duration) bool {
	// Closing the public stream interrupts backoff immediately.
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-stream.done:
		return false
	case <-timer.C:
		return true
	}
}

// RunOrderStream starts private WebSocket watchers for the selected market.
// Futures updates include conditional TP/SL orders when the provider publishes
// them separately. Passing no symbols watches every order in the selected market.
// Transient failures are retried and reconnect automatically on the same output
// channel. Authentication and other permanent failures terminate the stream.
func (client *Client) RunOrderStream(
	marketType types.MarketType,
	symbols ...string,
) (<-chan OrderUpdate, error) {
	if err := client.validateConfigured(); err != nil {
		return nil, err
	}
	if !marketType.Valid() {
		return nil, apperrors.ErrInvalidMarketType
	}
	symbolSubscriptions := make(map[string]bool, len(symbols))
	for _, symbol := range symbols {
		if symbol == "" {
			return nil, apperrors.ErrNilSymbol
		}

		market, exists := client.markets[symbol]
		if !exists || !isOrderStreamMarket(market, marketType) {
			return nil, apperrors.ErrInvalidSymbol
		}
		symbolSubscriptions[symbol] = true
	}

	if client.orderStreamAdapter == nil {
		return nil, apperrors.ErrUnsupportedProvider
	}
	watcherConfigs, err := client.orderStreamAdapter.WatcherConfigs(marketType)
	if err != nil {
		return nil, err
	}

	// Protect the Client's active-stream field during creation and removal.
	client.orderMu.Lock()
	defer client.orderMu.Unlock()

	if client.orderExchange == nil {
		return nil, apperrors.ErrUninitializedClient
	}
	if client.orderStream != nil {
		return nil, apperrors.ErrOrderStreamAlreadyExists
	}

	stream := &orderStream{
		updates:             make(chan OrderUpdate, 100),
		done:                make(chan struct{}),
		closeDone:           make(chan struct{}),
		marketType:          marketType,
		watchAllSymbols:     len(symbols) == 0,
		symbolSubscriptions: symbolSubscriptions,
		health:              make(map[bool]OrderSubscriptionHealth, len(watcherConfigs)),
	}
	for _, config := range watcherConfigs {
		stream.health[config.Trigger] = OrderSubscriptionHealth{
			MarketType: marketType, Trigger: config.Trigger, Status: StreamStarting,
		}
	}
	client.orderStream = stream
	// Reconnect reuses this prepared instance, including credentials and demo
	// settings. It does not create or configure another exchange object.
	orderExchange := client.orderExchange

	stream.wg.Add(1)
	go func() {
		defer stream.wg.Done()
		client.superviseOrderStream(stream, orderExchange, watcherConfigs)
	}()

	return stream.updates, nil
}

// SubscribeOrder allows updates for symbol through the running order stream.
// The provider connection is account-wide, so changing this local filter does
// not require another WebSocket subscription.
func (client *Client) SubscribeOrder(symbol string) error {
	stream, err := client.validateOrderStreamSymbol(symbol)
	if err != nil {
		return err
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closed {
		return apperrors.ErrOrderStreamNotRunning
	}

	stream.symbolSubscriptions[symbol] = true
	return nil
}

// UnsubscribeOrder suppresses updates for symbol without closing the
// account-wide provider subscription.
func (client *Client) UnsubscribeOrder(symbol string) error {
	stream, err := client.validateOrderStreamSymbol(symbol)
	if err != nil {
		return err
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closed {
		return apperrors.ErrOrderStreamNotRunning
	}

	stream.symbolSubscriptions[symbol] = false
	return nil
}

func (client *Client) validateOrderStreamSymbol(symbol string) (*orderStream, error) {
	if err := client.validateConfigured(); err != nil {
		return nil, err
	}
	if symbol == "" {
		return nil, apperrors.ErrNilSymbol
	}

	client.orderMu.Lock()
	stream := client.orderStream
	client.orderMu.Unlock()
	if stream == nil {
		return nil, apperrors.ErrOrderStreamNotRunning
	}

	market, exists := client.markets[symbol]
	if !exists || !isOrderStreamMarket(market, stream.marketType) {
		return nil, apperrors.ErrInvalidSymbol
	}

	return stream, nil
}

// CloseOrderStream starts private order stream shutdown and waits up to ten
// seconds for CCXT cleanup and the publishing supervisor to stop. Other streams
// and order submission are unaffected. ErrStreamCloseTimeout does not cancel
// stuck CCXT calls: cleanup continues in the background, and the public channel
// closes only after its publisher stops. Repeated closes share the same cleanup.
// Until cleanup finishes, use an independent client or Clone for recovery;
// normal completion allows a new order stream on this client.
func (client *Client) CloseOrderStream() error {
	if err := client.Validate(); err != nil {
		return err
	}
	client.orderMu.Lock()
	stream := client.orderStream
	orderExchange := client.orderExchange
	client.orderMu.Unlock()
	if stream == nil {
		return apperrors.ErrOrderStreamNotRunning
	}

	client.closeOrderStream(stream, orderExchange)
	timer := time.NewTimer(streamCloseTimeout)
	defer timer.Stop()
	select {
	case <-stream.closeDone:
		return stream.closeErr
	case <-timer.C:
		return apperrors.ErrStreamCloseTimeout
	}
}

// closeOrderStream starts one cleanup shared by explicit shutdown and terminal
// supervisor exits. It never waits on the supervisor that may be calling it.
func (client *Client) closeOrderStream(stream *orderStream, orderExchange ccxt.IExchange) {
	stream.closeOnce.Do(func() {
		stream.mu.Lock()
		stream.closed = true
		stream.watchAllSymbols = false
		clear(stream.symbolSubscriptions)
		clear(stream.health)
		stream.mu.Unlock()
		close(stream.done)

		go func() {
			var errs []error
			for _, err := range orderExchange.Close() {
				errs = append(errs, normalizeError(err))
			}
			// CCXT readers send internally; only the supervisor sends to updates.
			stream.wg.Wait()
			close(stream.updates)
			client.orderMu.Lock()
			if client.orderStream == stream {
				client.orderStream = nil
			}
			client.orderMu.Unlock()
			stream.closeErr = errors.Join(errs...)
			close(stream.closeDone)
		}()
	})
}

// superviseOrderStream publishes public updates and restarts all provider
// order watchers together when a retryable WebSocket failure survives backoff.
func (client *Client) superviseOrderStream(
	stream *orderStream,
	orderExchange ccxt.IExchange,
	watcherConfigs []adapters.OrderWatcherConfig,
) {
	// Schedule cleanup without waiting here: wg includes this supervisor.
	defer client.closeOrderStream(stream, orderExchange)

	for {
		// A generation contains all regular/conditional subscriptions. They share
		// one dedicated exchange, so a reconnect retires the entire group.
		events := make(chan orderWatchEvent, max(len(watcherConfigs)*2, 2))
		// Cancellation stops local forwarding and future watch calls. A watch
		// already blocked inside CCXT still depends on socket cleanup to return.
		generationDone := make(chan struct{})
		for _, watcherConfig := range watcherConfigs {
			go client.watchOrders(
				stream,
				orderExchange,
				watcherConfig,
				generationDone,
				events,
			)
		}

		// Process this watcher group until shutdown or a reconnect is needed.
		watching := true
		for watching {
			select {
			case <-stream.done:
				// Public shutdown cancels every subscription in the watcher group.
				close(generationDone)
				return

			case event := <-events:
				if event.err == nil {
					if event.update != nil && !stream.emit(*event.update) {
						close(generationDone)
						return
					}
					continue
				}

				// Stop sibling watchers before restarting their shared connection.
				// Their old events channel is never reused by the next generation.
				close(generationDone)
				if !isRetryableWatchError(event.err) {
					stream.emit(OrderUpdate{
						Trigger: event.trigger,
						Err:     normalizeError(event.err),
					})
					return
				}

				// CCXT Close resets only this instance's WebSocket clients. Its
				// credentials and loaded markets remain available for reconnection.
				// Both watchers are paused while their shared connection restarts.
				stream.mu.Lock()
				for trigger, health := range stream.health {
					health.Status = StreamRetrying
					health.LastError = normalizeError(event.err)
					stream.health[trigger] = health
				}
				stream.mu.Unlock()
				closeExchangeAsync(orderExchange)
				// The delay limits reconnect frequency. It does not synchronize
				// with asynchronous Close; cleanup may overlap the next watcher group.
				if !stream.waitForRetry(watchRetryBaseWait) {
					return
				}
				// End this group. The outer loop starts its subscriptions again.
				watching = false
			}
		}
	}
}

// watchOrders runs one provider subscription for the current stream generation.
// OKX, for example, uses separate watchers for regular and trigger orders.
func (client *Client) watchOrders(
	stream *orderStream,
	orderExchange ccxt.IExchange,
	watcherConfig adapters.OrderWatcherConfig,
	generationDone <-chan struct{},
	events chan<- orderWatchEvent,
) {
	retryCount := 0
	stream.mu.Lock()
	health := stream.health[watcherConfig.Trigger]
	stream.mu.Unlock()

	for {
		select {
		case <-stream.done:
			return
		case <-generationDone:
			return
		default:
		}

		params := map[string]any{
			"type": watcherConfig.MarketType,
		}
		if watcherConfig.Trigger {
			params["trigger"] = true
		}

		orders, err := orderExchange.WatchOrders(
			ccxt.WithWatchOrdersParams(params),
		)
		if err != nil {
			if stream.isClosed() {
				return
			}
			health.LastError = normalizeError(err)

			if isRetryableWatchError(err) && retryCount < maxWatchRetries {
				// Retry this subscription first with 1s, 2s, and 4s waits. Only
				// exhausted retries ask the supervisor to restart the watcher group.
				retryCount++
				health.Status = StreamRetrying
				health.RetryCount = retryCount
				if !stream.recordHealth(generationDone, health) {
					return
				}
				delay := watchRetryBaseWait * time.Duration(1<<(retryCount-1))
				if !stream.waitForRetry(delay) {
					return
				}
				continue
			}

			// Preserve the original CCXT error in the wrapped chain so the
			// supervisor can classify it before normalizing a public error.
			health.Status = StreamStopped
			if !stream.recordHealth(generationDone, health) {
				return
			}
			client.sendOrderWatchEvent(generationDone, events, orderWatchEvent{
				trigger: watcherConfig.Trigger,
				err: fmt.Errorf(
					"watch %s orders (trigger=%t): %w",
					watcherConfig.MarketType,
					watcherConfig.Trigger,
					err,
				),
			})
			return
		}
		// Backoff counts consecutive failures, not the lifetime total.
		retryCount = 0
		health.Status = StreamRunning
		health.RetryCount = 0
		if len(orders) > 0 {
			health.LastUpdate = time.Now()
		}
		if !stream.recordHealth(generationDone, health) {
			return
		}

		for _, order := range orders {
			update := wrapOrderUpdate(order, watcherConfig.Trigger)
			if !stream.watches(update.Symbol) {
				continue
			}
			if !client.sendOrderWatchEvent(
				generationDone,
				events,
				orderWatchEvent{update: &update},
			) {
				return
			}
		}
	}
}

// sendOrderWatchEvent lets generation cancellation release an internal send.
// A retired watcher retains its own events channel; any late result cannot be
// delivered through the next generation's channel.
func (client *Client) sendOrderWatchEvent(
	generationDone <-chan struct{},
	events chan<- orderWatchEvent,
	event orderWatchEvent,
) bool {
	select {
	case <-generationDone:
		return false
	case events <- event:
		return true
	}
}

func isOrderStreamMarket(market ccxt.MarketInterface, marketType types.MarketType) bool {
	switch marketType {
	case types.MarketSpot:
		return market.Spot != nil && *market.Spot
	case types.MarketFutures:
		return market.Swap != nil && *market.Swap
	default:
		return false
	}
}

func wrapOrderUpdate(order ccxt.Order, trigger bool) OrderUpdate {
	update := OrderUpdate{
		Timestamp:       order.Timestamp,
		Price:           order.Price,
		Average:         order.Average,
		Amount:          order.Amount,
		Filled:          order.Filled,
		Remaining:       order.Remaining,
		Cost:            order.Cost,
		TriggerPrice:    order.TriggerPrice,
		StopLossPrice:   order.StopLossPrice,
		TakeProfitPrice: order.TakeProfitPrice,
		ReduceOnly:      order.ReduceOnly,
		PostOnly:        order.PostOnly,
		FeeRate:         order.Fee.Rate,
		FeeCost:         order.Fee.Cost,
		Trigger:         trigger,
	}

	if order.Id != nil {
		update.ID = *order.Id
	}
	if order.ClientOrderId != nil {
		update.ClientOrderID = *order.ClientOrderId
	}
	if order.Symbol != nil {
		update.Symbol = *order.Symbol
	}
	if order.Type != nil {
		update.Type = *order.Type
	}
	if order.Side != nil {
		update.Side = *order.Side
	}
	if order.Status != nil {
		update.Status = *order.Status
	}
	positionSide, _ := historyString(order.Info, "posSide")
	if positionSide == "" {
		positionSide, _ = historyString(order.Info, "positionSide")
	}
	if positionSide == "" {
		positionSide, _ = historyString(order.Info, "ps")
	}
	update.PositionSide = types.PositionSide(strings.ToLower(positionSide))
	marginMode, _ := historyString(order.Info, "tdMode")
	update.MarginMode = types.MarginMode(marginMode)
	closePosition, _ := historyString(order.Info, "closePosition")
	closeFraction, _ := historyString(order.Info, "closeFraction")
	update.ClosePosition = order.Info["closePosition"] == true ||
		closePosition == "true" || closeFraction == "1" || order.Info["cp"] == true

	actualSide, _ := historyString(order.Info, "actualSide")
	switch actualSide {
	case "tp":
		update.ActualSide = types.ClosedByProfit
	case "sl":
		update.ActualSide = types.ClosedByStop
	}
	// JSON responses decode arrays as []any; typed callers may supply []string.
	// Copy the IDs so returned updates do not share the provider's mutable list.
	switch ids := order.Info["ordIdList"].(type) {
	case []string:
		update.TriggeredOrderIDs = append([]string(nil), ids...)
	case []any:
		for _, value := range ids {
			if id, ok := value.(string); ok {
				update.TriggeredOrderIDs = append(update.TriggeredOrderIDs, id)
			}
		}
	}

	// Binance's unified order contains TriggerPrice but not the TP/SL role.
	// Resolve it here so fetched and streamed orders share the same wrapper.
	var orderType string
	for _, key := range []string{"ot", "origType", "orderType", "type", "o"} {
		orderType, _ = historyString(order.Info, key)
		if orderType != "" {
			break
		}
	}
	var protection types.PositionState
	switch orderType {
	case "STOP", "STOP_MARKET", "STOP_LOSS", "STOP_LOSS_LIMIT":
		update.StopLossPrice = order.TriggerPrice
		protection = types.ClosedByStop
	case "TAKE_PROFIT", "TAKE_PROFIT_MARKET", "TAKE_PROFIT_LIMIT":
		update.TakeProfitPrice = order.TriggerPrice
		protection = types.ClosedByProfit
	}
	if update.ActualSide == types.UNSPECIFIED && protection != types.UNSPECIFIED {
		// A stop entry is not a stop-loss close. Hedge closing direction can
		// identify a reduction even when Binance does not set reduceOnly.
		closing := update.ClosePosition || (order.ReduceOnly != nil && *order.ReduceOnly) ||
			order.Info["R"] == true ||
			(update.PositionSide == types.PositionLong && update.Side == "sell") ||
			(update.PositionSide == types.PositionShort && update.Side == "buy")
		var rawStatus string
		for _, key := range []string{"X", "algoStatus", "status"} {
			rawStatus, _ = historyString(order.Info, key)
			if rawStatus != "" {
				break
			}
		}
		triggered := update.Status == "closed" || (order.Filled != nil && *order.Filled > 0) ||
			rawStatus == "TRIGGERED" || rawStatus == "FINISHED" ||
			rawStatus == "FILLED" || rawStatus == "PARTIALLY_FILLED"
		if closing && triggered {
			update.ActualSide = protection
		}
	}

	return update
}
