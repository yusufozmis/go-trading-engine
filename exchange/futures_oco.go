package exchange

import (
	"fmt"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/exchange/internal/adapters"
	"github.com/yusufozmis/go-trading-engine/types"
)

// CreateFuturesPositionOCO adds linked take-profit and stop-loss protection to
// an existing open side. Both trigger prices are required and both legs execute
// at market; triggering either leg cancels the other on the exchange.
// Protection covers the entire position using closeFraction=1, not a fixed
// quantity. The position is fetched once to check that the side is open and
// use its actual margin and hedge settings. Portfolio Margin is not supported.
// Existing protective orders are not replaced or canceled by this method.
// Only OKX is supported. Binance returns ErrFuturesOCOUnsupported without
// fetching positions or submitting an order. The returned order is an
// acknowledgment, not proof that either protective leg has executed.
func (client *Client) CreateFuturesPositionOCO(
	symbol string,
	side types.PositionSide,
	takeProfit, stopLoss float64,
) (OrderUpdate, error) {
	if err := client.validateConfigured(); err != nil {
		return OrderUpdate{}, err
	}
	if symbol == "" {
		return OrderUpdate{}, apperrors.ErrNilSymbol
	}
	market, exists := client.markets[symbol]
	if !exists || market.Swap == nil || !*market.Swap {
		return OrderUpdate{}, apperrors.ErrInvalidSymbol
	}
	if !side.Valid() {
		return OrderUpdate{}, apperrors.ErrInvalidSide
	}
	if !validPositiveNumber(takeProfit) || !validPositiveNumber(stopLoss) {
		return OrderUpdate{}, apperrors.ErrInvalidPrice
	}
	// A stop may already have moved into profit, so compare the two triggers
	// rather than requiring the stop to be on the loss side of the entry.
	if (side == types.PositionLong && stopLoss >= takeProfit) ||
		(side == types.PositionShort && stopLoss <= takeProfit) {
		return OrderUpdate{}, apperrors.ErrInvalidTPSLBracket
	}

	client.futuresMu.Lock()
	defer client.futuresMu.Unlock()
	if client.futuresAdapter == nil {
		return OrderUpdate{}, apperrors.ErrUnsupportedProvider
	}
	req := adapters.FuturesOrderRequest{
		Symbol: symbol, Side: side, Type: "oco",
		StopLoss: &stopLoss, TakeProfit: &takeProfit, IsClosing: true,
		MarginMode: client.futuresConfigs.MarginMode,
		Hedged:     client.futuresConfigs.Hedged,
	}
	params, err := client.futuresAdapter.OCOParams(req)
	if err != nil {
		return OrderUpdate{}, err
	}
	positions, err := client.iExchange.FetchPositions(
		ccxt.WithFetchPositionsSymbols([]string{symbol}),
	)
	if err != nil {
		return OrderUpdate{}, fmt.Errorf("fetch position for oco %q: %w", symbol, normalizeError(err))
	}
	for _, position := range positions {
		if position.Symbol == nil || *position.Symbol != symbol ||
			position.Side == nil || *position.Side != side.String() ||
			position.Contracts == nil || !validPositiveNumber(*position.Contracts) {
			continue
		}
		if position.MarginMode != nil {
			req.MarginMode = types.MarginMode(*position.MarginMode)
		}
		if !req.MarginMode.Valid() {
			return OrderUpdate{}, apperrors.ErrInvalidMarginMode
		}
		if position.Hedged != nil {
			req.Hedged = *position.Hedged
		}
		for key, value := range client.futuresAdapter.OrderParams(req) {
			params[key] = value
		}
		// CCXT requires an amount argument but omits sz when closeFraction is set.
		order, err := client.iExchange.CreateOrder(
			symbol, req.Type, closeOrderSide(side).String(), 0,
			ccxt.WithCreateOrderParams(params),
		)
		if err != nil {
			return OrderUpdate{}, fmt.Errorf("create futures oco %q: %w", symbol, normalizeError(err))
		}
		return wrapOrderUpdate(order, true), nil
	}
	return OrderUpdate{}, apperrors.ErrPositionNotFound
}
