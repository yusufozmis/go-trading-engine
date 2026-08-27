package exchange

import (
	"strings"

	"github.com/yusufozmis/go-trading-engine/errors"
)

// FetchSpotUSDTBalance returns the account's total USDT balance.
func (client *Client) FetchSpotUSDTBalance() (float64, error) {

	if err := client.validateConfigured(); err != nil {
		return 0, err
	}

	balance, err := client.iExchange.FetchBalance()
	if err != nil {
		return 0, err
	}

	amount := balance.Total["USDT"]

	if amount == nil {
		return 0, errors.ErrBalanceNotFound
	}

	return *amount, nil
}

// FetchSpotBalance returns the account's total balance for the base asset in symbol.
// The symbol must use a BASE/QUOTE format such as BTC/USDT.
func (client *Client) FetchSpotBalance(symbol string) (float64, error) {

	if err := client.validateConfigured(); err != nil {
		return 0, err
	}

	if symbol == "" {
		return 0, errors.ErrNilSymbol
	}

	marketInfo, ok := client.markets[symbol]
	if !ok {
		return 0, errors.ErrInvalidSymbol
	}

	if marketInfo.Spot == nil || !*marketInfo.Spot {
		return 0, errors.ErrInvalidSymbol
	}

	base, quote, found := strings.Cut(symbol, "/")
	if !found || base == "" || quote == "" ||
		strings.Contains(quote, "/") ||
		strings.Contains(symbol, ":") {
		return 0, errors.ErrInvalidSymbol
	}

	balance, err := client.iExchange.FetchBalance()
	if err != nil {
		return 0, err
	}

	amount := balance.Total[base]

	if amount == nil {
		return 0, errors.ErrBalanceNotFound
	}

	return *amount, nil
}

// FetchFuturesUSDTBalance returns total USDT collateral in the swap account.
func (client *Client) FetchFuturesUSDTBalance() (float64, error) {
	if err := client.validateConfigured(); err != nil {
		return 0, err
	}

	balance, err := client.iExchange.FetchBalance(
		map[string]any{"type": "swap"},
	)
	if err != nil {
		return 0, err
	}

	amount := balance.Total["USDT"]
	if amount == nil {
		return 0, errors.ErrBalanceNotFound
	}

	return *amount, nil
}
