package adapters

import (
	"fmt"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/types"
)

type BinanceFuturesAdapter struct {
	futures FuturesExchange
}

func NewBinanceFuturesAdapter(exchange FuturesExchange) *BinanceFuturesAdapter {
	return &BinanceFuturesAdapter{
		futures: exchange,
	}
}

func (adapter *BinanceFuturesAdapter) Prepare(symbol string, req FuturesConfig) error {
	// Binance may reject setting a margin mode that is already active. Fetching it
	// during explicit preparation lets us avoid that unnecessary SetMarginMode call.
	marginMode, err := adapter.futures.FetchMarginMode(symbol)
	if err != nil {
		return err
	}
	if marginMode.MarginMode == nil {
		return fmt.Errorf("binance: margin mode response is missing marginMode")
	}

	// Margin mode is symbol-specific on this adapter, so change it only when the
	// exchange reports a value different from the requested configuration.
	if *marginMode.MarginMode != req.MarginMode.String() {
		if _, err := adapter.futures.SetMarginMode(
			req.MarginMode.String(),
			ccxt.WithSetMarginModeSymbol(symbol),
		); err != nil {
			return err
		}
	}

	// Leverage is applied after margin mode so the setting belongs to the final
	// mode. Prepare may run during configuration or when an order adds a new symbol.
	_, err = adapter.futures.SetLeverage(
		req.Leverage,
		ccxt.WithSetLeverageSymbol(symbol),
	)
	return err
}

func (adapter *BinanceFuturesAdapter) OrderParams(req FuturesOrderRequest) map[string]any {
	params := map[string]any{}

	// Let CCXT translate a reduce-only hedge order into Binance's required
	// positionSide without sending the unsupported reduceOnly parameter.
	if req.IsClosing {
		params["hedged"] = req.Hedged
		return params
	}

	if req.Hedged {
		if req.Side == types.PositionLong {
			params["positionSide"] = "LONG"
		} else {
			params["positionSide"] = "SHORT"
		}
	}

	return params
}
