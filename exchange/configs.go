package exchange

import (
	stderrors "errors"
	"fmt"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/exchange/internal/adapters"
	"github.com/yusufozmis/go-trading-engine/types"
)

// ExchangeConfig contains credentials used for authenticated exchange operations.
type ExchangeConfig struct {
	APIKey    string
	SecretKey string
	Password  string // sometimes referred to as 'passphrase'
}

// Validate reports whether the required credential fields are present.
func (cfg *ExchangeConfig) Validate() error {

	if cfg.APIKey == "" {
		return apperrors.ErrNilAPIKey
	}

	if cfg.SecretKey == "" {
		return apperrors.ErrNilSecretKey
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
		return apperrors.ErrInvalidLeverage
	}

	if !marginMode.Valid() {
		return apperrors.ErrInvalidMarginMode
	}

	if len(symbols) == 0 {
		return apperrors.ErrEmptySymbols
	}

	// Validate the complete input before changing either local or remote state.
	for _, symbol := range symbols {
		if symbol == "" {
			return apperrors.ErrNilSymbol
		}

		marketInfo, exists := client.markets[symbol]
		if !exists || marketInfo.Swap == nil || !*marketInfo.Swap {
			return apperrors.ErrInvalidSymbol
		}
	}

	// Configuration, remote preparation, and order submission share this lock.
	// An order therefore cannot run while the account is only partly configured.
	client.futuresMu.Lock()
	defer client.futuresMu.Unlock()

	if client.futuresAdapter == nil {
		return apperrors.ErrUnsupportedProvider
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
		return normalizeError(err)
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
			normalizedErr := normalizeError(err)

			// Broad provider rejections become position-mode-specific only here,
			// where the operation that caused the error is known.
			if stderrors.Is(normalizedErr, apperrors.ErrOperationRejected) {
				return fmt.Errorf(
					"%w: set futures position mode: %w",
					apperrors.ErrPositionModeChangeRejected,
					normalizedErr,
				)
			}

			return fmt.Errorf("set futures position mode: %w", normalizedErr)
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
			return normalizeError(err)
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
		return apperrors.ErrConfigAlreadySet
	}

	required := client.iExchange.GetRequiredCredentials()
	requiresPassword, _ := required["password"].(bool)

	if requiresPassword && exchangeCfg.Password == "" {
		return apperrors.ErrNilPassword
	}

	client.iExchange.SetApiKey(exchangeCfg.APIKey)
	client.iExchange.SetSecret(exchangeCfg.SecretKey)
	client.iExchange.SetPassword(exchangeCfg.Password)

	client.preparedFuturesSymbols = nil
	client.configured = true

	return nil
}
