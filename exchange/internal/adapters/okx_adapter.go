package adapters

import ccxt "github.com/ccxt/ccxt/go/v4"

type OkxFuturesAdapter struct {
	exchange FuturesExchange
}

func NewOkxFuturesAdapter(exchange FuturesExchange) *OkxFuturesAdapter {
	return &OkxFuturesAdapter{
		exchange: exchange,
	}
}

func (adapter *OkxFuturesAdapter) Prepare(req FuturesOrderRequest) error {

	posSide := "net"
	if req.Hedged {
		posSide = req.Side.String()
	}

	_, err := adapter.exchange.SetPositionMode(
		req.Hedged,
		ccxt.WithSetPositionModeSymbol(req.Symbol),
	)
	if err != nil {
		return err
	}

	_, err = adapter.exchange.SetLeverage(
		req.Leverage,
		ccxt.WithSetLeverageSymbol(req.Symbol),
		ccxt.WithSetLeverageParams(map[string]any{
			"marginMode": req.MarginMode.String(),
			"posSide":    posSide,
		}),
	)
	return err
}

func (adapter *OkxFuturesAdapter) OrderParams(req FuturesOrderRequest) map[string]any {

	params := map[string]any{
		"marginMode": req.MarginMode.String(),
	}

	// Let CCXT derive OKX's closing posSide from the reduce-only order side.
	if req.IsClosing {
		params["hedged"] = req.Hedged
		return params
	}

	if req.Hedged {
		params["positionSide"] = req.Side.String()
	}

	return params
}
