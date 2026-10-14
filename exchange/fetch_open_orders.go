package exchange

import (
	"fmt"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
)

// FetchOpenOrders returns the active futures TP/SL orders for one symbol using
// provider-specific adapter parameters. Regular entry orders are not included.
// Only the provider's default page is fetched. Results use the same OrderUpdate
// representation as the order stream, with Err nil and Trigger true.
func (client *Client) FetchOpenOrders(symbol string) ([]OrderUpdate, error) {
	if err := client.validateConfigured(); err != nil {
		return nil, err
	}
	if symbol == "" {
		return nil, apperrors.ErrNilSymbol
	}
	if err := client.validateFetchPositionMarket(symbol); err != nil {
		return nil, err
	}
	if client.futuresAdapter == nil {
		return nil, apperrors.ErrUnsupportedProvider
	}

	var orders []OrderUpdate
	for _, params := range client.futuresAdapter.ProtectionOrderParams() {
		page, err := client.iExchange.FetchOpenOrders(
			ccxt.WithFetchOpenOrdersSymbol(symbol),
			ccxt.WithFetchOpenOrdersParams(params),
		)
		if err != nil {
			return nil, fmt.Errorf("fetch open protection orders %q: %w", symbol, normalizeError(err))
		}
		for _, order := range page {
			orders = append(orders, wrapOrderUpdate(order, true))
		}
	}
	return orders, nil
}
