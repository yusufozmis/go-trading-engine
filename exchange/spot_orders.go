package exchange

import (
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// CreateSpotMarketOrder creates a spot market order for a base-asset amount.
func (client *Client) CreateSpotMarketOrder(symbol string, side types.SpotSide, amount float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok || marketInfo.Spot == nil || !*marketInfo.Spot {
		return apperrors.ErrInvalidSymbol
	}

	if !side.Valid() {
		return apperrors.ErrInvalidSide
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return apperrors.ErrInvalidAmount
	}

	if amount <= 0 {
		return apperrors.ErrInvalidAmount
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return apperrors.ErrMinimumAmountUnavailable
	}

	if amount < *minAmount {
		return apperrors.ErrNotEnoughAmount
	}

	_, err := client.iExchange.CreateOrder(symbol, "market", side.String(), amount)
	return normalizeError(err)
}

// CreateSpotLimitOrder creates a spot limit order for a base-asset amount at price.
func (client *Client) CreateSpotLimitOrder(symbol string, side types.SpotSide, amount, price float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok || marketInfo.Spot == nil || !*marketInfo.Spot {
		return apperrors.ErrInvalidSymbol
	}

	if !side.Valid() {
		return apperrors.ErrInvalidSide
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return apperrors.ErrInvalidAmount
	}

	if amount <= 0 {
		return apperrors.ErrInvalidAmount
	}

	if math.IsNaN(price) || math.IsInf(price, 0) {
		return apperrors.ErrInvalidPrice
	}

	if price <= 0 {
		return apperrors.ErrInvalidPrice
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return apperrors.ErrMinimumAmountUnavailable
	}

	if amount < *minAmount {
		return apperrors.ErrNotEnoughAmount
	}

	_, err := client.iExchange.CreateOrder(symbol, "limit", side.String(), amount, ccxt.WithCreateOrderPrice(price))
	return normalizeError(err)
}

// CloseSpotPositionMarket sells the total base-asset balance at market price.
func (client *Client) CloseSpotPositionMarket(symbol string) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return apperrors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot ||
		marketInfo.BaseCurrency == nil || *marketInfo.BaseCurrency == "" {
		return apperrors.ErrInvalidSymbol
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return normalizeError(err)
	}

	amount := balances.Balances[*marketInfo.BaseCurrency].Total

	if amount == nil {
		return apperrors.ErrBalanceNotFound
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return apperrors.ErrMinimumAmountUnavailable
	}

	if *amount < *minAmount {
		return apperrors.ErrNotEnoughAmount
	}

	_, err = client.iExchange.CreateMarketSellOrder(
		symbol,
		*amount,
	)
	return normalizeError(err)
}

// CloseSpotPositionLimit places a limit order to sell the total base-asset balance.
func (client *Client) CloseSpotPositionLimit(symbol string, price float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return apperrors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot ||
		marketInfo.BaseCurrency == nil || *marketInfo.BaseCurrency == "" {
		return apperrors.ErrInvalidSymbol
	}

	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
		return apperrors.ErrInvalidPrice
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return normalizeError(err)
	}

	amount := balances.Balances[*marketInfo.BaseCurrency].Total

	if amount == nil {
		return apperrors.ErrBalanceNotFound
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return apperrors.ErrMinimumAmountUnavailable
	}

	if *amount < *minAmount {
		return apperrors.ErrNotEnoughAmount
	}

	_, err = client.iExchange.CreateLimitSellOrder(
		symbol,
		*amount,
		price,
	)
	return normalizeError(err)
}

// ReduceSpotPositionMarket reduces a spot holding by selling the specified
// base-asset amount at market price.
func (client *Client) ReduceSpotPositionMarket(symbol string, amount float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return apperrors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot ||
		marketInfo.BaseCurrency == nil || *marketInfo.BaseCurrency == "" {
		return apperrors.ErrInvalidSymbol
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return apperrors.ErrInvalidAmount
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return normalizeError(err)
	}

	freeBalance := balances.Balances[*marketInfo.BaseCurrency].Free

	if freeBalance == nil {
		return apperrors.ErrBalanceNotFound
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return apperrors.ErrMinimumAmountUnavailable
	}

	if amount < *minAmount || *freeBalance < amount {
		return apperrors.ErrNotEnoughAmount
	}

	_, err = client.iExchange.CreateMarketSellOrder(
		symbol,
		amount,
	)
	return normalizeError(err)
}

// ReduceSpotPositionLimit reduces a spot holding by placing a limit sell order
// for the specified base-asset amount.
func (client *Client) ReduceSpotPositionLimit(symbol string, amount, price float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return apperrors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return apperrors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot ||
		marketInfo.BaseCurrency == nil || *marketInfo.BaseCurrency == "" {
		return apperrors.ErrInvalidSymbol
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return apperrors.ErrInvalidAmount
	}

	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
		return apperrors.ErrInvalidPrice
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return normalizeError(err)
	}

	freeBalance := balances.Balances[*marketInfo.BaseCurrency].Free

	if freeBalance == nil {
		return apperrors.ErrBalanceNotFound
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return apperrors.ErrMinimumAmountUnavailable
	}

	if amount < *minAmount || *freeBalance < amount {
		return apperrors.ErrNotEnoughAmount
	}

	_, err = client.iExchange.CreateLimitSellOrder(
		symbol,
		amount,
		price,
	)
	return normalizeError(err)
}
