package exchange

import (
	"fmt"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
)

// CancelOrder requests cancellation of one regular spot or futures order by
// exchange ID. OCO/algo orders use CancelFuturesPositionOCO instead. The result
// is the provider's acknowledgment, not a synthesized final status. Cancellation
// can race with execution; a timeout leaves the outcome unknown. No retry occurs.
func (client *Client) CancelOrder(symbol, orderID string) (OrderUpdate, error) {
	if err := client.validateOrderSymbol(symbol); err != nil {
		return OrderUpdate{}, err
	}
	if orderID == "" {
		return OrderUpdate{}, apperrors.ErrNilOrderID
	}

	order, err := client.iExchange.CancelOrder(orderID, ccxt.WithCancelOrderSymbol(symbol))
	if err != nil {
		return OrderUpdate{}, fmt.Errorf("cancel order %q %q: %w", symbol, orderID, normalizeError(err))
	}
	update := wrapOrderUpdate(order, false)
	if update.Status == "rejected" {
		return update, apperrors.ErrOperationRejected
	}
	return update, nil
}

// CancelAllOrders requests cancellation of all open regular orders for symbol,
// including manual orders. Separately stored OCO/algo protection is not targeted.
// A native bulk endpoint is used when supported; otherwise the current open
// orders are fetched and canceled individually (OKX). The fallback only covers
// FetchOpenOrders' default page (up to 100 on OKX). This is not atomic and orders
// submitted after the snapshot are not guaranteed to be canceled.
// Results are acknowledgments and may omit IDs or final statuses. On a fallback
// failure, completed cancellations are returned with the error; no retry occurs.
func (client *Client) CancelAllOrders(symbol string) ([]OrderUpdate, error) {
	if err := client.validateOrderSymbol(symbol); err != nil {
		return nil, err
	}
	if client.iExchange.GetHas()["cancelAllOrders"] == true {
		orders, err := client.iExchange.CancelAllOrders(ccxt.WithCancelAllOrdersSymbol(symbol))
		if err != nil {
			return nil, fmt.Errorf("cancel all orders %q: %w", symbol, normalizeError(err))
		}
		updates := make([]OrderUpdate, 0, len(orders))
		for _, order := range orders {
			update := wrapOrderUpdate(order, false)
			updates = append(updates, update)
			if update.Status == "rejected" {
				return updates, apperrors.ErrOperationRejected
			}
		}
		return updates, nil
	}

	orders, err := client.FetchOpenOrders(symbol)
	if err != nil {
		return nil, err
	}
	updates := make([]OrderUpdate, 0, len(orders))
	for _, order := range orders {
		update, err := client.CancelOrder(symbol, order.ID)
		if err != nil {
			return updates, err
		}
		updates = append(updates, update)
	}
	return updates, nil
}
