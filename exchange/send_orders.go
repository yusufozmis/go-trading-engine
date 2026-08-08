package exchange

import (
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/trading-library/errors"
	"github.com/yusufozmis/trading-library/types"
)

type ExchangeConfig struct {
	ApiKey    string
	SecretKey string
	Password  string // sometimes referred to as 'passphrase'
}

func (cfg *ExchangeConfig) Validate() error {

	if cfg.ApiKey == "" {
		return errors.ErrNilApiKey
	}

	if cfg.SecretKey == "" {
		return errors.ErrNilSecretKey
	}

	if cfg.Password == "" {
		return errors.ErrNilPassword
	}

	return nil
}

func (client *Client) SetConfig(exchangeCfg ExchangeConfig) error {

	if err := client.validate(); err != nil {
		return err
	}

	if err := exchangeCfg.Validate(); err != nil {
		return err
	}

	client.iExchange.SetApiKey(exchangeCfg.ApiKey)
	client.iExchange.SetSecret(exchangeCfg.SecretKey)
	client.iExchange.SetPassword(exchangeCfg.Password)

	return nil
}

func (client *Client) CreateSpotMarketOrder(symbol string, side types.SpotSide, amount float64) error {
	if err := client.validate(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	if side != types.SpotBuy && side != types.SpotSell {
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
	if err := client.validate(); err != nil {
		return err
	}

	if symbol == "" {
		return errors.ErrNilSymbol
	}

	if side != types.SpotBuy && side != types.SpotSell {
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
