package exchange

import (
	"github.com/yusufozmis/trading-library/errors"
	"github.com/yusufozmis/trading-library/types"
)

type FuturesConfigs struct {
	Leverage   int64
	MarginMode types.MarginMode
	Hedged     bool
}

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

	return nil
}

func (cfg FuturesConfigs) Validate() error {
	if cfg.Leverage <= 0 {
		return errors.ErrInvalidLeverage
	}

	if !cfg.MarginMode.Valid() {
		return errors.ErrInvalidMarginMode
	}

	return nil
}

func (client *Client) SetFuturesConfig(cfg FuturesConfigs) error {
	if err := client.Validate(); err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	client.futuresMu.Lock()
	defer client.futuresMu.Unlock()

	client.futuresConfigs = cfg

	return nil
}

func (client *Client) SetConfig(exchangeCfg ExchangeConfig) error {

	if err := client.Validate(); err != nil {
		return err
	}

	if err := exchangeCfg.Validate(); err != nil {
		return err
	}

	required := client.iExchange.GetRequiredCredentials()
	requiresPassword, _ := required["password"].(bool)

	if requiresPassword && exchangeCfg.Password == "" {
		return errors.ErrNilPassword
	}

	client.iExchange.SetApiKey(exchangeCfg.ApiKey)
	client.iExchange.SetSecret(exchangeCfg.SecretKey)
	client.iExchange.SetPassword(exchangeCfg.Password)

	return nil
}
