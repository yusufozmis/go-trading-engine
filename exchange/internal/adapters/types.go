package adapters

import (
	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/trading-library/errors"
	"github.com/yusufozmis/trading-library/types"
)

type FuturesExchange interface {
	// These are the provider operations needed while preparing symbol settings.
	// Position mode is handled once at Client level because it is account state,
	// while this interface intentionally contains symbol/provider-specific work.
	FetchMarginMode(symbol string, options ...ccxt.FetchMarginModeOptions) (ccxt.MarginMode, error)
	SetLeverage(leverage int64, options ...ccxt.SetLeverageOptions) (map[string]any, error)
	SetMarginMode(marginMode string, options ...ccxt.SetMarginModeOptions) (map[string]any, error)
}

type FuturesOrderRequest struct {
	Symbol     string
	Side       types.PositionSide
	Type       string
	Amount     float64
	Price      float64
	Leverage   int64
	MarginMode types.MarginMode
	Hedged     bool
	IsClosing  bool
}

type FuturesConfig struct {
	Leverage   int64
	MarginMode types.MarginMode
	Hedged     bool
}

func (cfg FuturesConfig) Validate() error {
	if cfg.Leverage <= 0 {
		return errors.ErrInvalidLeverage
	}

	if !cfg.MarginMode.Valid() {
		return errors.ErrInvalidMarginMode
	}

	return nil
}

type FuturesAdapter interface {
	// Prepare performs provider-specific remote configuration outside the order
	// hot path. OrderParams only translates an already-prepared order.
	Prepare(symbol string, req FuturesConfig) error
	OrderParams(req FuturesOrderRequest) map[string]any
}
