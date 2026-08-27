package exchange

import (
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/exchange/internal/adapters"
	"github.com/yusufozmis/go-trading-engine/types"
)

func (client *Client) CreateFuturesMarketOrder(symbol string, side types.PositionSide, amount float64) error {

	if err := client.validateConfigured(); err != nil {
		return err
	}

	client.futuresMu.Lock()
	defer client.futuresMu.Unlock()

	req := adapters.FuturesOrderRequest{
		Symbol:     symbol,
		Side:       side,
		Type:       "market",
		Amount:     amount,
		Leverage:   client.futuresConfigs.Leverage,
		MarginMode: client.futuresConfigs.MarginMode,
		Hedged:     client.futuresConfigs.Hedged,
	}

	return client.createFuturesOrder(req)
}

func (client *Client) CreateFuturesLimitOrder(symbol string, side types.PositionSide, amount, price float64) error {

	if err := client.validateConfigured(); err != nil {
		return err
	}

	client.futuresMu.Lock()
	defer client.futuresMu.Unlock()

	req := adapters.FuturesOrderRequest{
		Symbol:     symbol,
		Side:       side,
		Type:       "limit",
		Amount:     amount,
		Price:      price,
		Leverage:   client.futuresConfigs.Leverage,
		MarginMode: client.futuresConfigs.MarginMode,
		Hedged:     client.futuresConfigs.Hedged,
	}

	return client.createFuturesOrder(req)
}

// CloseFuturesPosition closes the entire open position for the requested side.
// Binance support is currently limited to USD-M (linear) futures positions.
func (client *Client) CloseFuturesPosition(
	symbol string,
	positionSide types.PositionSide,
) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	marketInfo, exists := client.markets[symbol]
	if !exists || marketInfo.Swap == nil || !*marketInfo.Swap {
		return errors.ErrInvalidSymbol
	}

	if !positionSide.Valid() {
		return errors.ErrInvalidSide
	}

	client.futuresMu.Lock()
	defer client.futuresMu.Unlock()

	if client.futuresAdapter == nil {
		return errors.ErrUnsupportedProvider
	}

	positions, err := client.iExchange.FetchPositions(
		ccxt.WithFetchPositionsSymbols([]string{symbol}),
	)
	if err != nil {
		return err
	}

	for _, position := range positions {
		if position.Symbol == nil || *position.Symbol != symbol {
			continue
		}

		if position.Side == nil || *position.Side != positionSide.String() {
			continue
		}

		if position.Contracts == nil || math.IsNaN(*position.Contracts) ||
			math.IsInf(*position.Contracts, 0) || *position.Contracts <= 0 {
			continue
		}

		// Prefer the position's actual settings because the client config may
		// have changed after this position was opened.
		marginMode := client.futuresConfigs.MarginMode
		if position.MarginMode != nil {
			actualMarginMode := types.MarginMode(*position.MarginMode)
			if actualMarginMode.Valid() {
				marginMode = actualMarginMode
			}
		}

		hedged := client.futuresConfigs.Hedged
		if position.Hedged != nil {
			hedged = *position.Hedged
		}

		req := adapters.FuturesOrderRequest{
			Symbol:     symbol,
			Side:       positionSide,
			Type:       "market",
			Amount:     *position.Contracts,
			Leverage:   client.futuresConfigs.Leverage,
			MarginMode: marginMode,
			Hedged:     hedged,
			IsClosing:  true,
		}

		// A reduce-only order closes the position without risking an
		// accidental position in the opposite direction.
		_, err = client.iExchange.CreateReduceOnlyOrder(
			req.Symbol,
			req.Type,
			closeOrderSide(req.Side).String(),
			req.Amount,
			ccxt.WithCreateReduceOnlyOrderParams(
				client.futuresAdapter.OrderParams(req),
			),
		)
		return err
	}

	return errors.ErrPositionNotFound
}

func (client *Client) createFuturesOrder(req adapters.FuturesOrderRequest) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if err := client.validateFuturesOrder(req); err != nil {
		return err
	}

	options := []ccxt.CreateOrderOptions{
		ccxt.WithCreateOrderParams(client.futuresAdapter.OrderParams(req)),
	}

	if req.Type == "limit" {
		options = append(options, ccxt.WithCreateOrderPrice(req.Price))
	}

	if !client.preparedFuturesSymbols[req.Symbol] {
		if err := client.futuresAdapter.Prepare(req.Symbol, adapters.FuturesConfig{
			Leverage:   req.Leverage,
			MarginMode: req.MarginMode,
			Hedged:     req.Hedged,
		}); err != nil {
			return err
		}
		client.preparedFuturesSymbols[req.Symbol] = true
	}

	_, err := client.iExchange.CreateOrder(
		req.Symbol,
		req.Type,
		orderSide(req.Side).String(),
		req.Amount,
		options...,
	)
	return err
}

func (client *Client) validateFuturesOrder(req adapters.FuturesOrderRequest) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if client.futuresAdapter == nil {
		return errors.ErrUnsupportedProvider
	}

	if err := client.futuresConfigs.Validate(); err != nil {
		return err
	}

	if req.Symbol == "" {
		return errors.ErrNilSymbol
	}

	marketInfo, exists := client.markets[req.Symbol]
	if !exists || marketInfo.Swap == nil || !*marketInfo.Swap {
		return errors.ErrInvalidSymbol
	}

	if !req.Side.Valid() {
		return errors.ErrInvalidSide
	}

	if math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) || req.Amount <= 0 {
		return errors.ErrInvalidAmount
	}

	if req.Type == "limit" {
		if math.IsNaN(req.Price) || math.IsInf(req.Price, 0) || req.Price <= 0 {
			return errors.ErrInvalidPrice
		}
	}

	return nil
}

func orderSide(side types.PositionSide) types.SpotSide {
	if side == types.PositionLong {
		return types.SpotBuy
	}
	return types.SpotSell
}

// closeOrderSide returns the order side that closes the given position side.
func closeOrderSide(side types.PositionSide) types.SpotSide {
	if side == types.PositionLong {
		return types.SpotSell
	}
	return types.SpotBuy
}
