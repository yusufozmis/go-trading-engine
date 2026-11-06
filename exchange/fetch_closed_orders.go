package exchange

import (
	"fmt"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
)

// FetchClosedOrders returns the provider's default page of completed regular
// orders for one spot or futures symbol. Canceled orders and separately stored
// conditional/algo order history are not requested. A completed order does not
// necessarily close a position: it may open, increase, or reduce one.
// Results use the same OrderUpdate representation as the order stream, with
// Err nil and Trigger false. Futures quantities remain exchange contracts.
func (client *Client) FetchClosedOrders(symbol string) ([]OrderUpdate, error) {
	if err := client.validateConfigured(); err != nil {
		return nil, err
	}
	if symbol == "" {
		return nil, apperrors.ErrNilSymbol
	}
	if _, exists := client.markets[symbol]; !exists {
		return nil, apperrors.ErrInvalidSymbol
	}

	orders, err := client.iExchange.FetchClosedOrders(
		ccxt.WithFetchClosedOrdersSymbol(symbol),
	)
	if err != nil {
		return nil, fmt.Errorf("fetch closed orders %q: %w", symbol, normalizeError(err))
	}

	wrapped := make([]OrderUpdate, 0, len(orders))
	for _, order := range orders {
		wrapped = append(wrapped, wrapOrderUpdate(order, false))
	}
	return wrapped, nil
}
