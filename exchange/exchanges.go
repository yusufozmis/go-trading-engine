package exchange

import (
	"sync"

	ccxt "github.com/ccxt/ccxt/go/v4"
	ccxtpro "github.com/ccxt/ccxt/go/v4/pro"
	"github.com/yusufozmis/go-trading-engine/errors"
	"github.com/yusufozmis/go-trading-engine/exchange/internal/adapters"
)

type Client struct {
	iExchange      ccxt.IExchange
	futuresAdapter adapters.FuturesAdapter

	stream *candleStream

	// futuresMu serializes futures configuration and order submission. Holding one
	// lock prevents orders from observing a half-applied remote configuration.
	futuresMu      sync.Mutex
	futuresConfigs adapters.FuturesConfig

	// preparedFuturesSymbols contains only symbols successfully configured by the
	// most recent SetFuturesConfig call. The bool value keeps order checks cheap.
	preparedFuturesSymbols map[string]bool
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
