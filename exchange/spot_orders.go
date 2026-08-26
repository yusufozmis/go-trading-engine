package exchange

import (
	"math"
	"strings"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/types"
)

func (client *Client) CreateSpotMarketOrder(symbol string, side types.SpotSide, amount float64) error {
	if err := client.Validate(); err != nil {
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
	if err := client.Validate(); err != nil {
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

func (client *Client) CloseSpotPositionMarket(symbol string, side types.SpotSide) error {
	if err := client.Validate(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	if !side.Valid() {
		return errors.ErrInvalidSide
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return err
	}

	before, _, _ := strings.Cut(symbol, "/")
	amount := *balances.Balances[before].Total

	if amount < 0.00001 {
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

func (client *Client) CloseSpotPositionLimit(symbol string, side types.SpotSide, price float64) error {
	if err := client.Validate(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	if !side.Valid() {
		return errors.ErrInvalidSide
	}

	balances, err := client.iExchange.FetchBalance()
	if err != nil {
		return err
	}

	before, _, _ := strings.Cut(symbol, "/")
	amount := *balances.Balances[before].Total

	if amount < 0.00001 {
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
