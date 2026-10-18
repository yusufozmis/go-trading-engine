package exchange

import (
	"fmt"
	"log"
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

	// Trigger identifies updates received from a provider's separate
	// conditional-order channel, such as OKX orders-algo.
	Trigger bool
	// Err is non-nil only when the identified watcher has stopped permanently.
	Err error
}

type orderStream struct {
	// The supervisor owns this public channel. Regular and trigger watchers
	// forward results internally so neither can close another watcher's output.
	updates chan OrderUpdate
	// done stops the public stream and prevents further reconnect attempts.
	done chan struct{}

	mu                  sync.Mutex
	closed              bool
	marketType          types.MarketType
	watchAllSymbols     bool
	symbolSubscriptions map[string]bool

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

func (stream *orderStream) stop() {
	stream.closeOnce.Do(func() {
		stream.mu.Lock()
		stream.closed = true
		stream.mu.Unlock()
		close(stream.done)
	})
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

	bufferSize := max(len(symbols)*len(watcherConfigs)*2, 2)
	stream := &orderStream{
		updates:             make(chan OrderUpdate, bufferSize),
		done:                make(chan struct{}),
		marketType:          marketType,
		watchAllSymbols:     len(symbols) == 0,
		symbolSubscriptions: symbolSubscriptions,
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

// CloseOrderStream stops private order watchers without closing position,
// candle, or order-submission connections.
// WebSocket cleanup runs in the background; this method waits only for the
// supervisor to stop publishing and close the output channel.
func (client *Client) CloseOrderStream() error {
	if err := client.Validate(); err != nil {
		return err
	}
	client.orderMu.Lock()
	stream := client.orderStream
	client.orderMu.Unlock()
	if stream == nil {
		return apperrors.ErrOrderStreamNotRunning
	}

	// The supervisor closes the public channel; CCXT socket cleanup continues
	// separately, so a stuck Close call does not block this wait.
	stream.stop()
	stream.wg.Wait()
	return nil
}

// superviseOrderStream owns the public update channel and restarts all provider
// order watchers together when a retryable WebSocket failure survives backoff.
func (client *Client) superviseOrderStream(
	stream *orderStream,
	orderExchange ccxt.IExchange,
	watcherConfigs []adapters.OrderWatcherConfig,
) {
	// Watchers never publish directly to updates. This remains safe even if an
	// old CCXT call returns after the supervisor has closed the public channel.
	defer close(stream.updates)
	defer func() {
		client.orderMu.Lock()
		if client.orderStream == stream {
			client.orderStream = nil
		}
		client.orderMu.Unlock()
	}()

	waitingForRestartUpdate := false
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
				closeExchangeAsync(orderExchange)
				return

			case event := <-events:
				if event.err == nil {
					if event.update != nil && waitingForRestartUpdate {
						log.Printf("order stream: received first update after watcher restart (trigger=%t)", event.update.Trigger)
						waitingForRestartUpdate = false
					}
					if event.update != nil && !stream.emit(*event.update) {
						close(generationDone)
						closeExchangeAsync(orderExchange)
						return
					}
					continue
				}

				// Stop sibling watchers before restarting their shared connection.
				// Their old events channel is never reused by the next generation.
				close(generationDone)
				if !isRetryableWatchError(event.err) {
					log.Printf("order stream: terminal error (trigger=%t): %v", event.trigger, normalizeError(event.err))
					closeExchangeAsync(orderExchange)
					stream.emit(OrderUpdate{
						Trigger: event.trigger,
						Err:     normalizeError(event.err),
					})
					return
				}

				// CCXT Close resets only this instance's WebSocket clients. Its
				// credentials and loaded markets remain available for reconnection.
				log.Printf("order stream: retries exhausted, restarting watchers after %s (trigger=%t): %v", watchRetryBaseWait, event.trigger, event.err)
				closeExchangeAsync(orderExchange)
				// The delay limits reconnect frequency. It does not synchronize
				// with asynchronous Close; cleanup may overlap the next watcher group.
				if !stream.waitForRetry(watchRetryBaseWait) {
					return
				}
				// End this group. The outer loop starts its subscriptions again.
				waitingForRestartUpdate = true
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

			if isRetryableWatchError(err) && retryCount < maxWatchRetries {
				// Retry this subscription first with 1s, 2s, and 4s waits. Only
				// exhausted retries ask the supervisor to restart the watcher group.
				retryCount++
				delay := watchRetryBaseWait * time.Duration(1<<(retryCount-1))
				log.Printf("order stream: retry %d/%d, waiting %s (market=%s trigger=%t): %v", retryCount, maxWatchRetries, delay, watcherConfig.MarketType, watcherConfig.Trigger, err)
				if !stream.waitForRetry(delay) {
					return
				}
				continue
			}

			// Preserve the original CCXT error in the wrapped chain so the
			// supervisor can classify it before normalizing a public error.
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
		if retryCount > 0 {
			log.Printf("order stream: watch succeeded after %d retries (market=%s trigger=%t)", retryCount, watcherConfig.MarketType, watcherConfig.Trigger)
		}
		retryCount = 0

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
	update.PositionSide = types.PositionSide(strings.ToLower(positionSide))
	marginMode, _ := historyString(order.Info, "tdMode")
	update.MarginMode = types.MarginMode(marginMode)
	closePosition, _ := historyString(order.Info, "closePosition")
	closeFraction, _ := historyString(order.Info, "closeFraction")
	update.ClosePosition = order.Info["closePosition"] == true ||
		closePosition == "true" || closeFraction == "1"

	// Binance's unified order contains TriggerPrice but not the TP/SL role.
	// Resolve it here so fetched and streamed orders share the same wrapper.
	orderType, _ := historyString(order.Info, "orderType")
	if orderType == "" {
		orderType, _ = historyString(order.Info, "type")
	}
	switch orderType {
	case "STOP", "STOP_MARKET":
		update.StopLossPrice = order.TriggerPrice
	case "TAKE_PROFIT", "TAKE_PROFIT_MARKET":
		update.TakeProfitPrice = order.TriggerPrice
	}

	return update
}
