package exchange

import (
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/exchange/internal/adapters"
	"github.com/yusufozmis/go-trading-engine/types"
)

// CreateFuturesMarketOrder creates a market order using a requested base-asset
// amount. The amount is estimated as contracts from the market's contract size,
// so exchange precision may change the final traded base-asset quantity.
func (client *Client) CreateFuturesMarketOrder(symbol string, side types.PositionSide, amount float64) error {

	if err := client.validateConfigured(); err != nil {
		return err
	}

	contractAmount, err := client.baseAmountToContracts(symbol, amount)
	if err != nil {
		return err
	}

	client.futuresMu.Lock()
	defer client.futuresMu.Unlock()

	req := adapters.FuturesOrderRequest{
		Symbol:     symbol,
		Side:       side,
		Type:       "market",
		Amount:     contractAmount,
		Leverage:   client.futuresConfigs.Leverage,
		MarginMode: client.futuresConfigs.MarginMode,
		Hedged:     client.futuresConfigs.Hedged,
	}

	return client.createFuturesOrder(req)
}

// CreateFuturesLimitOrder creates a limit order using a requested base-asset
// amount. The amount is estimated as contracts from the market's contract size,
// so exchange precision may change the final traded base-asset quantity.
func (client *Client) CreateFuturesLimitOrder(symbol string, side types.PositionSide, amount, price float64) error {

	if err := client.validateConfigured(); err != nil {
		return err
	}

	contractAmount, err := client.baseAmountToContracts(symbol, amount)
	if err != nil {
		return err
	}

	client.futuresMu.Lock()
	defer client.futuresMu.Unlock()

	req := adapters.FuturesOrderRequest{
		Symbol:     symbol,
		Side:       side,
		Type:       "limit",
		Amount:     contractAmount,
		Price:      price,
		Leverage:   client.futuresConfigs.Leverage,
		MarginMode: client.futuresConfigs.MarginMode,
		Hedged:     client.futuresConfigs.Hedged,
	}

	return client.createFuturesOrder(req)
}

// CreateFuturesWithTPSL opens a market futures position and asks the provider
// to attach market take-profit and stop-loss orders to the same entry request.
// The position amount is interpreted as a base-asset quantity and estimated as
// contracts from market metadata. Currently, only OKX supports this operation.
func (client *Client) CreateFuturesWithTPSL(position types.Position) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	var side types.PositionSide
	switch position.State {
	case types.LongOpen:
		side = types.PositionLong
	case types.ShortOpen:
		side = types.PositionShort
	default:
		return errors.ErrInvalidSide
	}

	if err := position.Validate(); err != nil {
		return err
	}

	contractAmount, err := client.baseAmountToContracts(position.Symbol, position.Amount)
	if err != nil {
		return err
	}

	client.futuresMu.Lock()
	defer client.futuresMu.Unlock()

	req := adapters.FuturesOrderRequest{
		Symbol:     position.Symbol,
		Side:       side,
		Type:       "market",
		Amount:     contractAmount,
		StopLoss:   position.StopLoss,
		TakeProfit: position.TP,
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
		return normalizeError(err)
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
		return normalizeError(err)
	}

	return errors.ErrPositionNotFound
}

// ReduceFuturesPositionMarket reduces the requested side of an open perpetual
// futures position by a requested base-asset amount at market price. The amount
// is estimated as contracts, so exchange precision may change the final quantity.
// Binance support is currently limited to USD-M (linear) futures positions.
func (client *Client) ReduceFuturesPositionMarket(
	symbol string,
	positionSide types.PositionSide,
	amount float64,
) error {
	return client.reduceFuturesPosition(symbol, positionSide, amount, "market", 0)
}

// ReduceFuturesPositionLimit places a limit order that reduces the requested
// side of an open perpetual futures position by a requested base-asset amount.
// The amount is estimated as contracts, so exchange precision may change the final quantity.
// Binance support is currently limited to USD-M (linear) futures positions.
func (client *Client) ReduceFuturesPositionLimit(
	symbol string,
	positionSide types.PositionSide,
	amount, price float64,
) error {
	return client.reduceFuturesPosition(symbol, positionSide, amount, "limit", price)
}

// reduceFuturesPosition contains the position lookup and reduce-only order flow
// shared by the market and limit public methods.
func (client *Client) reduceFuturesPosition(
	symbol string,
	positionSide types.PositionSide,
	amount float64,
	orderType string,
	price float64,
) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	contractAmount, err := client.baseAmountToContracts(symbol, amount)
	if err != nil {
		return err
	}

	if orderType == "limit" &&
		(math.IsNaN(price) || math.IsInf(price, 0) || price <= 0) {
		return errors.ErrInvalidPrice
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
		return normalizeError(err)
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

		if *position.Contracts < contractAmount {
			return errors.ErrNotEnoughAmount
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
			Type:       orderType,
			Amount:     contractAmount,
			Price:      price,
			Leverage:   client.futuresConfigs.Leverage,
			MarginMode: marginMode,
			Hedged:     hedged,
			IsClosing:  true,
		}

		options := []ccxt.CreateReduceOnlyOrderOptions{
			ccxt.WithCreateReduceOnlyOrderParams(
				client.futuresAdapter.OrderParams(req),
			),
		}

		if req.Type == "limit" {
			options = append(options, ccxt.WithCreateReduceOnlyOrderPrice(req.Price))
		}

		// A reduce-only order closes the position without risking an
		// accidental position in the opposite direction.
		_, err = client.iExchange.CreateReduceOnlyOrder(
			req.Symbol,
			req.Type,
			closeOrderSide(req.Side).String(),
			req.Amount,
			options...,
		)
		return normalizeError(err)
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

	params := client.futuresAdapter.OrderParams(req)

	// Protective prices are zero for every existing order method. Only the new
	// attached TP/SL flow reaches the adapter capability below.
	if req.StopLoss != 0 || req.TakeProfit != 0 {
		attachedParams, err := client.futuresAdapter.AttachedTPSLParams(req)
		if err != nil {
			return err
		}
		for key, value := range attachedParams {
			params[key] = value
		}
	}

	options := []ccxt.CreateOrderOptions{
		ccxt.WithCreateOrderParams(params),
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
			return normalizeError(err)
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
	return normalizeError(err)
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

// baseAmountToContracts estimates a linear swap contract amount from a
// requested base-asset quantity using the exchange's loaded market metadata.
func (client *Client) baseAmountToContracts(symbol string, amount float64) (float64, error) {
	if symbol == "" {
		return 0, errors.ErrNilSymbol
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return 0, errors.ErrInvalidAmount
	}

	marketInfo, exists := client.markets[symbol]
	if !exists || marketInfo.Swap == nil || !*marketInfo.Swap {
		return 0, errors.ErrInvalidSymbol
	}

	// Inverse contracts are quote-denominated and require a price to convert a
	// base-asset amount safely; this metadata-only conversion is linear-only.
	if marketInfo.Linear == nil || !*marketInfo.Linear {
		return 0, errors.ErrUnsupportedFuturesMarket
	}

	if marketInfo.ContractSize == nil || math.IsNaN(*marketInfo.ContractSize) ||
		math.IsInf(*marketInfo.ContractSize, 0) || *marketInfo.ContractSize <= 0 {
		return 0, errors.ErrContractSizeUnavailable
	}

	contractAmount := amount / *marketInfo.ContractSize
	if math.IsNaN(contractAmount) || math.IsInf(contractAmount, 0) || contractAmount <= 0 {
		return 0, errors.ErrInvalidAmount
	}

	return contractAmount, nil
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
