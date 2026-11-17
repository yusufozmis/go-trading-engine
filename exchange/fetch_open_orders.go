package exchange

import (
	"fmt"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
)

// FetchOpenProtectionOrders returns the active futures TP/SL orders for one symbol using
// provider-specific adapter parameters. Regular entry orders are not included.
// Only the provider's default page is fetched. Results use the same OrderUpdate
// representation as the order stream, with Err nil and Trigger true.
func (client *Client) FetchOpenProtectionOrders(symbol string) ([]OrderUpdate, error) {
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

// FetchOpenOrders returns the provider's default page of open regular orders
// for one spot or futures symbol (up to 100 on OKX). Only one request is made;
// separately stored TP/SL and OCO/algo orders are not requested.
// Results have Trigger false; futures amounts remain exchange contracts.
// Absence from this list does not prove an order failed: use FetchOrder to
// distinguish a filled or canceled order from one whose outcome is unknown.
func (client *Client) FetchOpenOrders(symbol string) ([]OrderUpdate, error) {
	if err := client.validateOrderSymbol(symbol); err != nil {
		return nil, err
	}

	orders, err := client.iExchange.FetchOpenOrders(ccxt.WithFetchOpenOrdersSymbol(symbol))
	if err != nil {
		return nil, fmt.Errorf("fetch open orders %q: %w", symbol, normalizeError(err))
	}
	wrapped := make([]OrderUpdate, 0, len(orders))
	for _, order := range orders {
		wrapped = append(wrapped, wrapOrderUpdate(order, false))
	}
	return wrapped, nil
}
