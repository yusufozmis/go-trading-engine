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

	// SetConfig can only be applied once.
	configured bool
	// preparedFuturesSymbols contains symbols successfully prepared for the
	// current futures configuration. The bool value keeps order checks cheap.
	preparedFuturesSymbols map[string]bool

	// Exchanges metadata
	markets map[string]ccxt.MarketInterface
}

func NewBinance() (*Client, error) {
	pro := ccxtpro.NewBinance(nil)
	core := ccxt.NewBinanceFromCore(pro.Core.BinanceCore)

	markets, err := pro.LoadMarkets()
	if err != nil {
		return nil, err
	}

	return &Client{
		iExchange:      pro,
		futuresAdapter: adapters.NewBinanceFuturesAdapter(core),
		markets:        markets,
	}, nil
}

func NewOkx() (*Client, error) {
	pro := ccxtpro.NewOkx(nil)
	core := ccxt.NewOkxFromCore(pro.Core.OkxCore)

	markets, err := pro.LoadMarkets()
	if err != nil {
		return nil, err
	}

	return &Client{
		iExchange:      pro,
		futuresAdapter: adapters.NewOkxFuturesAdapter(core),
		markets:        markets,
	}, nil
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

func (client *Client) validateConfigured() error {
	if err := client.Validate(); err != nil {
		return err
	}

	if !client.configured {
		return errors.ErrClientNotConfigured
	}

	return nil
}
