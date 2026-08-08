package exchange

import (
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/trading-library/errors"
	"github.com/yusufozmis/trading-library/exchange/internal/adapters"
	"github.com/yusufozmis/trading-library/types"
)

const (
	CROSS    = "cross"
	Isolated = "isolated"
)

type FuturesConfigs struct {
	Leverage int64
	Mode     string
	Hedged   bool
}

func (cfg FuturesConfigs) Validate() error {
	if cfg.Leverage <= 0 {
		return errors.ErrInvalidAmount // replace with ErrInvalidLeverage if you add one
	}

	if cfg.Mode != CROSS && cfg.Mode != Isolated {
		return errors.ErrInvalidSide // replace with ErrInvalidMarginMode if you add one
	}

	return nil
}

func (client *Client) SetFuturesConfig(cfg FuturesConfigs) error {
	if err := client.validate(); err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	client.futuresConfigs = cfg

	return nil
}

func (client *Client) CreateFuturesMarketOrder(symbol, side string, amount float64) error {
	return client.createFuturesOrder(adapters.FuturesOrderRequest{
		Symbol:     symbol,
		Side:       side,
		Type:       "market",
		Amount:     amount,
		Leverage:   client.futuresConfigs.Leverage,
		MarginMode: client.futuresConfigs.Mode,
		Hedged:     client.futuresConfigs.Hedged,
	})
}

func (client *Client) CreateFuturesLimitOrder(symbol, side string, amount, price float64) error {
	return client.createFuturesOrder(adapters.FuturesOrderRequest{
		Symbol:     symbol,
		Side:       side,
		Type:       "limit",
		Amount:     amount,
		Price:      price,
		Leverage:   client.futuresConfigs.Leverage,
		MarginMode: client.futuresConfigs.Mode,
		Hedged:     client.futuresConfigs.Hedged,
	})
}

func (client *Client) createFuturesOrder(req adapters.FuturesOrderRequest) error {
	if err := client.validate(); err != nil {
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

	if err := client.futuresAdapter.Prepare(req); err != nil {
		return err
	}

	_, err := client.iExchange.CreateOrder(
		req.Symbol,
		req.Type,
		orderSide(req.Side),
		req.Amount,
		options...,
	)
	return err
}

func (client *Client) validateFuturesOrder(req adapters.FuturesOrderRequest) error {
	if err := client.validate(); err != nil {
		return err
	}

	/*if client.futures == nil {
		return errors.ErrUninitializedClient
	}*/

	if client.futuresAdapter == nil {
		return errors.ErrUnsupportedProvider
	}

	if err := client.futuresConfigs.Validate(); err != nil {
		return err
	}

	if req.Symbol == "" {
		return errors.ErrNilSymbol
	}

	if req.Side != types.LONG && req.Side != types.SHORT {
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

func orderSide(side string) string {
	if side == types.LONG {
		return types.BUY
	}
	return types.SELL
}
