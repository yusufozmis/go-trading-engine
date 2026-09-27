package exchange

import (
	"errors"
	"fmt"
	"sync"
	"time"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/exchange/internal/adapters"
	"github.com/yusufozmis/go-trading-engine/types"
)

// OrderUpdate contains either the latest exchange-reported order state or a
// terminal watcher error. Optional fields remain nil when omitted by the provider.
type OrderUpdate struct {
	ID            string
	ClientOrderID string
	Symbol        string
	Type          string
	Side          string
	Status        string
	Timestamp     *int64

	Price     *float64
	Average   *float64
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
	updates chan OrderUpdate
	done    chan struct{}

	mu                  sync.Mutex
	closed              bool
	marketType          types.MarketType
	watchAllSymbols     bool
	symbolSubscriptions map[string]bool
	closeErr            error

	wg        sync.WaitGroup
	closeOnce sync.Once
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
	select {
	case <-stream.done:
		return false
	case stream.updates <- update:
		return true
	}
}

func (stream *orderStream) waitForRetry(delay time.Duration) bool {
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
	if client.orderExchange == nil {
		return nil, apperrors.ErrUninitializedClient
	}
	if client.orderStream != nil {
		return nil, apperrors.ErrOrderStreamAlreadyExists
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

	bufferSize := max(len(symbols)*len(watcherConfigs)*2, 2)
	client.orderStream = &orderStream{
		updates:             make(chan OrderUpdate, bufferSize),
		done:                make(chan struct{}),
		marketType:          marketType,
		watchAllSymbols:     len(symbols) == 0,
		symbolSubscriptions: symbolSubscriptions,
	}

	client.orderStream.wg.Add(len(watcherConfigs))
	for _, watcherConfig := range watcherConfigs {
		go func() {
			defer client.orderStream.wg.Done()
			client.watchOrders(watcherConfig)
		}()
	}

	return client.orderStream.updates, nil
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

	stream := client.orderStream
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
func (client *Client) CloseOrderStream() error {
	if err := client.Validate(); err != nil {
		return err
	}
	if client.orderStream == nil {
		return apperrors.ErrOrderStreamNotRunning
	}

	stream := client.orderStream
	stream.closeOnce.Do(func() {
		stream.mu.Lock()
		stream.closed = true
		stream.mu.Unlock()
		close(stream.done)

		var closeErrors []error
		// The dedicated instance may own both normal and conditional-order
		// subscriptions, so one Close interrupts every blocked watcher together.
		for _, err := range client.orderExchange.Close() {
			closeErrors = append(closeErrors, normalizeError(err))
		}

		stream.wg.Wait()
		close(stream.updates)
		client.orderStream = nil

		if len(closeErrors) > 0 {
			stream.closeErr = errors.Join(closeErrors...)
		}
	})

	return stream.closeErr
}

func (client *Client) watchOrders(watcherConfig adapters.OrderWatcherConfig) {
	stream := client.orderStream
	retryCount := 0

	for {
		params := map[string]any{
			"type": watcherConfig.MarketType,
		}
		if watcherConfig.Trigger {
			params["trigger"] = true
		}

		orders, err := client.orderExchange.WatchOrders(
			ccxt.WithWatchOrdersParams(params),
		)
		if err != nil {
			if stream.isClosed() {
				return
			}

			if isRetryableWatchError(err) && retryCount < maxWatchRetries {
				retryCount++
				delay := watchRetryBaseWait * time.Duration(1<<(retryCount-1))
				if !stream.waitForRetry(delay) {
					return
				}
				continue
			}

			stream.emit(OrderUpdate{
				Trigger: watcherConfig.Trigger,
				Err: fmt.Errorf(
					"watch %s orders (trigger=%t): %w",
					watcherConfig.MarketType,
					watcherConfig.Trigger,
					normalizeError(err),
				),
			})
			return
		}
		retryCount = 0

		for _, order := range orders {
			update := wrapOrderUpdate(order, watcherConfig.Trigger)
			if !stream.watches(update.Symbol) {
				continue
			}
			if !stream.emit(update) {
				return
			}
		}
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

	return update
}
