package exchange

import ccxt "github.com/ccxt/ccxt/go/v4"

type OkxFuturesAdapter struct {
	exchange FuturesExchange
}

func NewOkxFuturesAdapter(exchange FuturesExchange) *OkxFuturesAdapter {
	return &OkxFuturesAdapter{
		exchange: exchange,
	}
}

func (adapter *OkxFuturesAdapter) Prepare(req futuresOrderRequest) error {

	posSide := "net"
	if req.Hedged {
		posSide = req.Side
	}

	_, err := adapter.exchange.SetLeverage(
		req.Leverage,
		ccxt.WithSetLeverageSymbol(req.Symbol),
		ccxt.WithSetLeverageParams(map[string]any{
			"marginMode": req.MarginMode,
			"posSide":    posSide,
		}),
	)
	return err
}

func (adapter *OkxFuturesAdapter) OrderParams(req futuresOrderRequest) map[string]any {

	params := map[string]any{
		"marginMode": req.MarginMode,
	}

	if req.Hedged {
		params["positionSide"] = req.Side
	}

	return params
}
