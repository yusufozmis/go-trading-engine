package exchange

import (
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// CreateSpotMarketOrder creates a spot market order for a base-asset amount.
func (client *Client) CreateSpotMarketOrder(symbol string, side types.SpotSide, amount float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok || marketInfo.Spot == nil || !*marketInfo.Spot {
		return errors.ErrInvalidSymbol
	}

	if !side.Valid() {
		return errors.ErrInvalidSide
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return errors.ErrInvalidAmount
	}

	if amount <= 0 {
		return errors.ErrInvalidAmount
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return errors.ErrMinimumAmountUnavailable
	}

	if amount < *minAmount {
		return errors.ErrNotEnoughAmount
	}

	_, err := client.iExchange.CreateOrder(symbol, "market", side.String(), amount)
	if err != nil {
		return err
	}

	return nil
}

// CreateSpotLimitOrder creates a spot limit order for a base-asset amount at price.
func (client *Client) CreateSpotLimitOrder(symbol string, side types.SpotSide, amount, price float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok || marketInfo.Spot == nil || !*marketInfo.Spot {
		return errors.ErrInvalidSymbol
	}

	if !side.Valid() {
		return errors.ErrInvalidSide
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return errors.ErrInvalidAmount
	}

	if amount <= 0 {
		return errors.ErrInvalidAmount
	}

	if math.IsNaN(price) || math.IsInf(price, 0) {
		return errors.ErrInvalidPrice
	}

	if price <= 0 {
		return errors.ErrInvalidPrice
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return errors.ErrMinimumAmountUnavailable
	}

	if amount < *minAmount {
		return errors.ErrNotEnoughAmount
	}

	_, err := client.iExchange.CreateOrder(symbol, "limit", side.String(), amount, ccxt.WithCreateOrderPrice(price))
	if err != nil {
		return err
	}

	return nil
}

// CloseSpotPositionMarket sells the total base-asset balance at market price.
func (client *Client) CloseSpotPositionMarket(symbol string) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return errors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot ||
		marketInfo.BaseCurrency == nil || *marketInfo.BaseCurrency == "" {
		return errors.ErrInvalidSymbol
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return err
	}

	amount := balances.Balances[*marketInfo.BaseCurrency].Total

	if amount == nil {
		return errors.ErrBalanceNotFound
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return errors.ErrMinimumAmountUnavailable
	}

	if *amount < *minAmount {
		return errors.ErrNotEnoughAmount
	}

	_, err = client.iExchange.CreateMarketSellOrder(
		symbol,
		*amount,
	)
	if err != nil {
		return err
	}

	return nil
}

// CloseSpotPositionLimit places a limit order to sell the total base-asset balance.
func (client *Client) CloseSpotPositionLimit(symbol string, price float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return errors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot ||
		marketInfo.BaseCurrency == nil || *marketInfo.BaseCurrency == "" {
		return errors.ErrInvalidSymbol
	}

	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
		return errors.ErrInvalidPrice
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return err
	}

	amount := balances.Balances[*marketInfo.BaseCurrency].Total

	if amount == nil {
		return errors.ErrBalanceNotFound
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return errors.ErrMinimumAmountUnavailable
	}

	if *amount < *minAmount {
		return errors.ErrNotEnoughAmount
	}

	_, err = client.iExchange.CreateLimitSellOrder(
		symbol,
		*amount,
		price,
	)
	if err != nil {
		return err
	}
	return nil
}

// ReduceSpotPositionMarket reduces a spot holding by selling the specified
// base-asset amount at market price.
func (client *Client) ReduceSpotPositionMarket(symbol string, amount float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return errors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot ||
		marketInfo.BaseCurrency == nil || *marketInfo.BaseCurrency == "" {
		return errors.ErrInvalidSymbol
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return errors.ErrInvalidAmount
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return err
	}

	freeBalance := balances.Balances[*marketInfo.BaseCurrency].Free

	if freeBalance == nil {
		return errors.ErrBalanceNotFound
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return errors.ErrMinimumAmountUnavailable
	}

	if amount < *minAmount || *freeBalance < amount {
		return errors.ErrNotEnoughAmount
	}

	_, err = client.iExchange.CreateMarketSellOrder(
		symbol,
		amount,
	)
	if err != nil {
		return err
	}

	return nil
}

// ReduceSpotPositionLimit reduces a spot holding by placing a limit sell order
// for the specified base-asset amount.
func (client *Client) ReduceSpotPositionLimit(symbol string, amount, price float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return errors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot ||
		marketInfo.BaseCurrency == nil || *marketInfo.BaseCurrency == "" {
		return errors.ErrInvalidSymbol
	}

	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return errors.ErrInvalidAmount
	}

	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
		return errors.ErrInvalidPrice
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return err
	}

	freeBalance := balances.Balances[*marketInfo.BaseCurrency].Free

	if freeBalance == nil {
		return errors.ErrBalanceNotFound
	}

	minAmount := marketInfo.Limits.Amount.Min
	if minAmount == nil {
		return errors.ErrMinimumAmountUnavailable
	}

	if amount < *minAmount || *freeBalance < amount {
		return errors.ErrNotEnoughAmount
	}

	_, err = client.iExchange.CreateLimitSellOrder(
		symbol,
		amount,
		price,
	)
	if err != nil {
		return err
	}
	return nil
}
