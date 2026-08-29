package adapters

import (
	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/types"
)

// OKXFuturesAdapter translates futures configuration and order parameters for OKX.
type OKXFuturesAdapter struct {
	exchange FuturesExchange
}

// NewOKXFuturesAdapter creates an OKX futures adapter.
func NewOKXFuturesAdapter(exchange FuturesExchange) *OKXFuturesAdapter {
	return &OKXFuturesAdapter{
		exchange: exchange,
	}
}

// Prepare applies leverage settings for an OKX symbol and position mode.
func (adapter *OKXFuturesAdapter) Prepare(symbol string, req FuturesConfig) error {
	// In OKX isolated hedge mode, long and short leverage are separate settings.
	// Preparing both here means the first long and first short orders are equally
	// ready and neither has to call SetLeverage on the latency-sensitive order path.
	if req.Hedged && req.MarginMode == types.MarginModeIsolated {
		for _, side := range []types.PositionSide{types.PositionLong, types.PositionShort} {
			if err := adapter.setLeverage(symbol, req, side.String()); err != nil {
				return err
			}
		}
		return nil
	}

	// Cross margin and one-way position mode use the net position side rather than
	// separate long/short leverage configuration.
	return adapter.setLeverage(symbol, req, "net")
}

func (adapter *OKXFuturesAdapter) setLeverage(symbol string, req FuturesConfig, positionSide string) error {
	// OKX calls this field marginMode in CCXT's unified parameters. posSide is only
	// required for isolated leverage; sending it for cross mode would be incorrect.
	params := map[string]any{
		"marginMode": req.MarginMode.String(),
	}
	if req.MarginMode == types.MarginModeIsolated {
		params["posSide"] = positionSide
	}

	// This is the actual remote preparation request. It completes before the
	// caller marks the symbol as prepared in the Client's local setup state.
	_, err := adapter.exchange.SetLeverage(
		req.Leverage,
		ccxt.WithSetLeverageSymbol(symbol),
		ccxt.WithSetLeverageParams(params),
	)
	return err
}

// OrderParams returns OKX-specific parameters for a futures order.
func (adapter *OKXFuturesAdapter) OrderParams(req FuturesOrderRequest) map[string]any {

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

// AttachSL uses CCXT's unified attached-order shape. CCXT translates this
// value into OKX attachAlgoOrds on the same request as the entry order.
func (adapter *OKXFuturesAdapter) AttachSL(req FuturesOrderRequest) (map[string]any, error) {
	return map[string]any{
		"stopLoss": map[string]any{
			"triggerPrice": *req.StopLoss,
			"type":         "market",
		},
	}, nil
}

// AttachTP uses CCXT's unified attached-order shape. CCXT translates this
// value into OKX attachAlgoOrds on the same request as the entry order.
func (adapter *OKXFuturesAdapter) AttachTP(req FuturesOrderRequest) (map[string]any, error) {
	return map[string]any{
		"takeProfit": map[string]any{
			"triggerPrice": *req.TakeProfit,
			"type":         "market",
		},
	}, nil
}
