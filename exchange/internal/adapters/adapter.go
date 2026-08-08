package adapters

import ccxt "github.com/ccxt/ccxt/go/v4"

type FuturesExchange interface {
	SetLeverage(leverage int64, options ...ccxt.SetLeverageOptions) (map[string]any, error)
	SetMarginMode(marginMode string, options ...ccxt.SetMarginModeOptions) (map[string]any, error)
	SetPositionMode(hedged bool, options ...ccxt.SetPositionModeOptions) (map[string]any, error)
}

type FuturesOrderRequest struct {
	Symbol     string
	Side       string
	Type       string
	Amount     float64
	Price      float64
	Leverage   int64
	MarginMode string
	Hedged     bool
}
