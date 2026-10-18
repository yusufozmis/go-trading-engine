package exchange

import (
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// CreateSpotMarketOrder creates a spot market order for a base-asset amount.
func (client *Client) CreateSpotMarketOrder(symbol string, side types.SpotSide, amount float64) (OrderUpdate, error) {
	if err := client.validateConfigured(); err != nil {
		return OrderUpdate{}, err
	}

	if symbol == "" {
		return OrderUpdate{}, apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok || marketInfo.Spot == nil || !*marketInfo.Spot {
		return OrderUpdate{}, apperrors.ErrInvalidSymbol
	}

	if !side.Valid() {
		return OrderUpdate{}, apperrors.ErrInvalidSide
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return OrderUpdate{}, apperrors.ErrInvalidAmount
	}

	if amount <= 0 {
		return OrderUpdate{}, apperrors.ErrInvalidAmount
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return OrderUpdate{}, apperrors.ErrMinimumAmountUnavailable
	}

	if amount < *minAmount {
		return OrderUpdate{}, apperrors.ErrNotEnoughAmount
	}

	order, err := client.iExchange.CreateOrder(symbol, "market", side.String(), amount)
	if err != nil {
		return OrderUpdate{}, normalizeError(err)
	}
	return wrapOrderUpdate(order, false), nil
}

// CreateSpotLimitOrder creates a spot limit order for a base-asset amount at price.
func (client *Client) CreateSpotLimitOrder(symbol string, side types.SpotSide, amount, price float64) (OrderUpdate, error) {
	if err := client.validateConfigured(); err != nil {
		return OrderUpdate{}, err
	}

	if symbol == "" {
		return OrderUpdate{}, apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok || marketInfo.Spot == nil || !*marketInfo.Spot {
		return OrderUpdate{}, apperrors.ErrInvalidSymbol
	}

	if !side.Valid() {
		return OrderUpdate{}, apperrors.ErrInvalidSide
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return OrderUpdate{}, apperrors.ErrInvalidAmount
	}

	if amount <= 0 {
		return OrderUpdate{}, apperrors.ErrInvalidAmount
	}

	if math.IsNaN(price) || math.IsInf(price, 0) {
		return OrderUpdate{}, apperrors.ErrInvalidPrice
	}

	if price <= 0 {
		return OrderUpdate{}, apperrors.ErrInvalidPrice
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return OrderUpdate{}, apperrors.ErrMinimumAmountUnavailable
	}

	if amount < *minAmount {
		return OrderUpdate{}, apperrors.ErrNotEnoughAmount
	}

	order, err := client.iExchange.CreateOrder(symbol, "limit", side.String(), amount, ccxt.WithCreateOrderPrice(price))
	if err != nil {
		return OrderUpdate{}, normalizeError(err)
	}
	return wrapOrderUpdate(order, false), nil
}

// CloseSpotPositionMarket sells the total base-asset balance at market price.
func (client *Client) CloseSpotPositionMarket(symbol string) (OrderUpdate, error) {
	if err := client.validateConfigured(); err != nil {
		return OrderUpdate{}, err
	}

	if symbol == "" {
		return OrderUpdate{}, apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return OrderUpdate{}, apperrors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot ||
		marketInfo.BaseCurrency == nil || *marketInfo.BaseCurrency == "" {
		return OrderUpdate{}, apperrors.ErrInvalidSymbol
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return OrderUpdate{}, normalizeError(err)
	}

	amount := balances.Balances[*marketInfo.BaseCurrency].Total

	if amount == nil {
		return OrderUpdate{}, apperrors.ErrBalanceNotFound
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return OrderUpdate{}, apperrors.ErrMinimumAmountUnavailable
	}

	if *amount < *minAmount {
		return OrderUpdate{}, apperrors.ErrNotEnoughAmount
	}

	order, err := client.iExchange.CreateMarketSellOrder(
		symbol,
		*amount,
	)
	if err != nil {
		return OrderUpdate{}, normalizeError(err)
	}
	return wrapOrderUpdate(order, false), nil
}

// CloseSpotPositionLimit places a limit order to sell the total base-asset balance.
func (client *Client) CloseSpotPositionLimit(symbol string, price float64) (OrderUpdate, error) {
	if err := client.validateConfigured(); err != nil {
		return OrderUpdate{}, err
	}

	if symbol == "" {
		return OrderUpdate{}, apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return OrderUpdate{}, apperrors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot ||
		marketInfo.BaseCurrency == nil || *marketInfo.BaseCurrency == "" {
		return OrderUpdate{}, apperrors.ErrInvalidSymbol
	}

	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
		return OrderUpdate{}, apperrors.ErrInvalidPrice
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return OrderUpdate{}, normalizeError(err)
	}

	amount := balances.Balances[*marketInfo.BaseCurrency].Total

	if amount == nil {
		return OrderUpdate{}, apperrors.ErrBalanceNotFound
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return OrderUpdate{}, apperrors.ErrMinimumAmountUnavailable
	}

	if *amount < *minAmount {
		return OrderUpdate{}, apperrors.ErrNotEnoughAmount
	}

	order, err := client.iExchange.CreateLimitSellOrder(
		symbol,
		*amount,
		price,
	)
	if err != nil {
		return OrderUpdate{}, normalizeError(err)
	}
	return wrapOrderUpdate(order, false), nil
}

// ReduceSpotPositionMarket reduces a spot holding by selling the specified
// base-asset amount at market price.
func (client *Client) ReduceSpotPositionMarket(symbol string, amount float64) (OrderUpdate, error) {
	if err := client.validateConfigured(); err != nil {
		return OrderUpdate{}, err
	}

	if symbol == "" {
		return OrderUpdate{}, apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return OrderUpdate{}, apperrors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot ||
		marketInfo.BaseCurrency == nil || *marketInfo.BaseCurrency == "" {
		return OrderUpdate{}, apperrors.ErrInvalidSymbol
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return OrderUpdate{}, apperrors.ErrInvalidAmount
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return OrderUpdate{}, normalizeError(err)
	}

	freeBalance := balances.Balances[*marketInfo.BaseCurrency].Free

	if freeBalance == nil {
		return OrderUpdate{}, apperrors.ErrBalanceNotFound
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return OrderUpdate{}, apperrors.ErrMinimumAmountUnavailable
	}

	if amount < *minAmount || *freeBalance < amount {
		return OrderUpdate{}, apperrors.ErrNotEnoughAmount
	}

	order, err := client.iExchange.CreateMarketSellOrder(
		symbol,
		amount,
	)
	if err != nil {
		return OrderUpdate{}, normalizeError(err)
	}
	return wrapOrderUpdate(order, false), nil
}

// ReduceSpotPositionLimit reduces a spot holding by placing a limit sell order
// for the specified base-asset amount.
func (client *Client) ReduceSpotPositionLimit(symbol string, amount, price float64) (OrderUpdate, error) {
	if err := client.validateConfigured(); err != nil {
		return OrderUpdate{}, err
	}

	if symbol == "" {
		return OrderUpdate{}, apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return OrderUpdate{}, apperrors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot ||
		marketInfo.BaseCurrency == nil || *marketInfo.BaseCurrency == "" {
		return OrderUpdate{}, apperrors.ErrInvalidSymbol
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return OrderUpdate{}, apperrors.ErrInvalidAmount
	}

	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
		return OrderUpdate{}, apperrors.ErrInvalidPrice
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return OrderUpdate{}, normalizeError(err)
	}

	freeBalance := balances.Balances[*marketInfo.BaseCurrency].Free

	if freeBalance == nil {
		return OrderUpdate{}, apperrors.ErrBalanceNotFound
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return OrderUpdate{}, apperrors.ErrMinimumAmountUnavailable
	}

	if amount < *minAmount || *freeBalance < amount {
		return OrderUpdate{}, apperrors.ErrNotEnoughAmount
	}

	order, err := client.iExchange.CreateLimitSellOrder(
		symbol,
		amount,
		price,
	)
	if err != nil {
		return OrderUpdate{}, normalizeError(err)
	}
	return wrapOrderUpdate(order, false), nil
}
