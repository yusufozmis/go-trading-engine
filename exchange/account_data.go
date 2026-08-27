package exchange

import (
	"strings"

	"github.com/yusufozmis/go-trading-engine/errors"
)

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

// This allows you to check the balance of a given asset.
// Note: The symbol should be in <SYMBOL>/USDT or <SYMBOL>/USDC format,
// i.e., BTC/USDT.
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

	before, _, found := strings.Cut(symbol, "/")
	if !found {
		return 0, errors.ErrInvalidSymbol
	}

	amount := balance.Total[before]

	if amount == nil {
		return 0, errors.ErrBalanceNotFound
	}

	return *amount, nil
}
