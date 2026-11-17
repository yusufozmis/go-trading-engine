package exchange

import (
	"fmt"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
)

// FetchOrder queries one regular spot or futures order by its exchange ID,
// whether open, filled, or canceled. Status, Filled, and Remaining are copied
// from the provider without inventing missing values. Trigger is false and
// futures quantities remain exchange contracts. OCO/algo IDs are not supported.
// A query error leaves the execution outcome unknown; it is not a reason to
// resend the entry order. The provider's order-history retention limits apply.
func (client *Client) FetchOrder(symbol, orderID string) (OrderUpdate, error) {
	if err := client.validateOrderSymbol(symbol); err != nil {
		return OrderUpdate{}, err
	}
	if orderID == "" {
		return OrderUpdate{}, apperrors.ErrNilOrderID
	}

	order, err := client.iExchange.FetchOrder(orderID, ccxt.WithFetchOrderSymbol(symbol))
	if err != nil {
		return OrderUpdate{}, fmt.Errorf("fetch order %q %q: %w", symbol, orderID, normalizeError(err))
	}
	return wrapOrderUpdate(order, false), nil
}

// FetchOpenOrder queries one regular order and requires it to still be open,
// including partially filled orders. A different or missing status returns the
// fetched order together with ErrOrderNotOpen. Use FetchOrder for recovery when
// filled and canceled orders are also expected; no execution outcome is inferred.
func (client *Client) FetchOpenOrder(symbol, orderID string) (OrderUpdate, error) {
	order, err := client.FetchOrder(symbol, orderID)
	if err != nil {
		return OrderUpdate{}, err
	}
	if order.Status != "open" {
		return order, apperrors.ErrOrderNotOpen
	}
	return order, nil
}

func (client *Client) validateOrderSymbol(symbol string) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}
	if symbol == "" {
		return apperrors.ErrNilSymbol
	}
	if _, exists := client.markets[symbol]; !exists {
		return apperrors.ErrInvalidSymbol
	}
	return nil
}
