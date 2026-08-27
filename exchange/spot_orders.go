package exchange

import (
	"math"
	"strings"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

func (client *Client) CreateSpotMarketOrder(symbol string, side types.SpotSide, amount float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
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

	_, err := client.iExchange.CreateOrder(symbol, "market", side.String(), amount)
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) CreateSpotLimitOrder(symbol string, side types.SpotSide, amount, price float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
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

	_, err := client.iExchange.CreateOrder(symbol, "limit", side.String(), amount, ccxt.WithCreateOrderPrice(price))
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) CloseSpotPositionMarket(symbol string) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	base, quote, found := strings.Cut(symbol, "/")
	if !found || base == "" || quote == "" ||
		strings.Contains(quote, "/") ||
		strings.Contains(symbol, ":") {
		return errors.ErrInvalidSymbol
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return err
	}

	amount := balances.Balances[base].Total

	if amount == nil {
		return errors.ErrBalanceNotFound
	}

	if *amount < 0.00001 {
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

func (client *Client) CloseSpotPositionLimit(symbol string, price float64) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	base, quote, found := strings.Cut(symbol, "/")
	if !found || base == "" || quote == "" ||
		strings.Contains(quote, "/") ||
		strings.Contains(symbol, ":") {
		return errors.ErrInvalidSymbol
	}

	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
		return errors.ErrInvalidPrice
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return err
	}

	amount := balances.Balances[base].Total

	if amount == nil {
		return errors.ErrBalanceNotFound
	}

	if *amount < 0.00001 {
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
