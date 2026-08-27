package exchange

import (
	"fmt"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/exchange/internal/adapters"
	"github.com/yusufozmis/go-trading-engine/types"
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

	return nil
}

// SetFuturesConfig stores the desired futures configuration and applies the
// corresponding account and symbol settings before futures orders are allowed.
// SetConfig must be called first because this method performs authenticated
// exchange requests.
func (client *Client) SetFuturesConfig(leverage int64, marginMode types.MarginMode, hedged bool, symbols ...string) error {
	if err := client.validateConfigured(); err != nil {
		return err
	}

	if leverage <= 0 {
		return errors.ErrInvalidLeverage
	}

	if !marginMode.Valid() {
		return errors.ErrInvalidMarginMode
	}

	if len(symbols) == 0 {
		return errors.ErrEmptySymbols
	}

	// Validate the complete input before changing either local or remote state.
	for _, symbol := range symbols {
		if symbol == "" {
			return errors.ErrNilSymbol
		}

		marketInfo, exists := client.markets[symbol]
		if !exists || marketInfo.Swap == nil || !*marketInfo.Swap {
			return errors.ErrInvalidSymbol
		}
	}

	// Configuration, remote preparation, and order submission share this lock.
	// An order therefore cannot run while the account is only partly configured.
	client.futuresMu.Lock()
	defer client.futuresMu.Unlock()

	if client.futuresAdapter == nil {
		return errors.ErrUnsupportedProvider
	}

	// Invalidate the previous symbol preparation before making remote changes.
	// If any request below fails, order creation stays disabled instead of using
	// a configuration that may have been only partially applied.
	client.futuresConfigs = adapters.FuturesConfig{}
	client.preparedFuturesSymbols = nil

	positionMode, err := client.iExchange.FetchPositionMode(
		ccxt.WithFetchPositionModeSymbol(symbols[0]),
	)
	if err != nil {
		return err
	}

	currentHedged, ok := positionMode["hedged"].(bool)
	if !ok {
		return fmt.Errorf("exchange: position mode response is missing hedged value")
	}

	// If account already is in the requested mode, don't send request.
	if currentHedged != hedged {
		_, err = client.iExchange.SetPositionMode(
			hedged,
			ccxt.WithSetPositionModeSymbol(symbols[0]),
		)
		if err != nil {
			return fmt.Errorf("set futures position mode: %w", err)
		}
	}

	cfg := adapters.FuturesConfig{
		Leverage:   leverage,
		MarginMode: marginMode,
		Hedged:     hedged,
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	preparedSymbols := make(map[string]bool, len(symbols))
	for _, symbol := range symbols {
		// Do not repeat provider requests when the same symbol appears twice.
		if preparedSymbols[symbol] {
			continue
		}

		if err := client.futuresAdapter.Prepare(symbol, cfg); err != nil {
			return err
		}

		preparedSymbols[symbol] = true
	}

	client.futuresConfigs = cfg
	client.preparedFuturesSymbols = preparedSymbols

	return nil
}

// SetConfig configures the credentials used for authenticated exchange requests.
// It must be called before SetFuturesConfig and any order, position, or balance method.
// SetConfig must not be called concurrently with any other Client method.
func (client *Client) SetConfig(exchangeCfg ExchangeConfig) error {

	if err := client.Validate(); err != nil {
		return err
	}

	if err := exchangeCfg.Validate(); err != nil {
		return err
	}

	if client.configured {
		return errors.ErrConfigAlreadySet
	}

	required := client.iExchange.GetRequiredCredentials()
	requiresPassword, _ := required["password"].(bool)

	if requiresPassword && exchangeCfg.Password == "" {
		return errors.ErrNilPassword
	}

	client.iExchange.SetApiKey(exchangeCfg.ApiKey)
	client.iExchange.SetSecret(exchangeCfg.SecretKey)
	client.iExchange.SetPassword(exchangeCfg.Password)

	client.preparedFuturesSymbols = nil
	client.configured = true

	return nil
}
