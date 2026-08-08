package exchange

import (
	ccxt "github.com/ccxt/ccxt/go/v4"
	ccxtpro "github.com/ccxt/ccxt/go/v4/pro"
	"github.com/yusufozmis/trading-library/errors"
	"github.com/yusufozmis/trading-library/exchange/internal/adapters"
)

type Client struct {
	iExchange      ccxt.IExchange
	futuresAdapter adapters.FuturesAdapter

	stream *candleStream

	futuresConfigs FuturesConfigs
}

func NewBinance() *Client {
	pro := ccxtpro.NewBinance(nil)
	core := ccxt.NewBinanceFromCore(pro.Core.BinanceCore)

	return &Client{
		iExchange:      pro,
		futuresAdapter: adapters.NewBinanceFuturesAdapter(core),
	}
}

func NewOkx() *Client {
	pro := ccxtpro.NewOkx(nil)
	core := ccxt.NewOkxFromCore(pro.Core.OkxCore)

	return &Client{
		iExchange:      pro,
		futuresAdapter: adapters.NewOkxFuturesAdapter(core),
	}
}

func (client *Client) Validate() error {

	if client == nil {
		return errors.ErrNilClient
	}

	if client.iExchange == nil {
		return errors.ErrUninitializedClient
	}

	return nil
}
