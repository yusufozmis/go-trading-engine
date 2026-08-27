package exchange

import (
	"strings"

	"github.com/yusufozmis/go-trading-engine/errors"
)

// FetchUSDTBalance returns the account's total USDT balance.
func (client *Client) FetchUSDTBalance() (float64, error) {

	if err := client.Validate(); err != nil {
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

// FetchBalance returns the account's total balance for the base asset in symbol.
// The symbol must use a BASE/QUOTE format such as BTC/USDT.
func (client *Client) FetchBalance(symbol string) (float64, error) {

	if err := client.Validate(); err != nil {
		return 0, err
	}

	if symbol == "" {
		return 0, errors.ErrNilSymbol
	}

	balance, err := client.iExchange.FetchBalance()
	if err != nil {
		return 0, err
	}

	base, quote, found := strings.Cut(symbol, "/")
	if !found {
		return 0, errors.ErrInvalidSymbol
	}

	if base == "" {
		return 0, errors.ErrInvalidSymbol
	}
	if quote == "" {
		return 0, errors.ErrInvalidSymbol
	}

	amount := balance.Total[base]

	if amount == nil {
		return 0, errors.ErrBalanceNotFound
	}

	return *amount, nil
}
