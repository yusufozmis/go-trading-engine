package adapters

import (
	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/trading-library/types"
)

type BinanceFuturesAdapter struct {
	futures FuturesExchange
}

func NewBinanceFuturesAdapter(exchange FuturesExchange) *BinanceFuturesAdapter {
	return &BinanceFuturesAdapter{
		futures: exchange,
	}
}

func (adapter *BinanceFuturesAdapter) Prepare(req FuturesOrderRequest) error {

	_, err := adapter.futures.SetMarginMode(
		req.MarginMode.String(),
		ccxt.WithSetMarginModeSymbol(req.Symbol),
	)
	if err != nil {
		return err
	}

	_, err = adapter.futures.SetPositionMode(
		req.Hedged,
		ccxt.WithSetPositionModeSymbol(req.Symbol),
	)
	if err != nil {
		return err
	}

	_, err = adapter.futures.SetLeverage(
		req.Leverage,
		ccxt.WithSetLeverageSymbol(req.Symbol),
	)
	return err
}

func (adapter *BinanceFuturesAdapter) OrderParams(req FuturesOrderRequest) map[string]any {
	params := map[string]any{}

	if req.Hedged {
		if req.Side == types.PositionLong {
			params["positionSide"] = "LONG"
		} else {
			params["positionSide"] = "SHORT"
		}
	}

	return params
}
