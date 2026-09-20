// Package exchange provides unified market-data, streaming, account, and order
// operations for supported exchange providers.
package exchange

import (
	"sync"

	ccxt "github.com/ccxt/ccxt/go/v4"
	ccxtpro "github.com/ccxt/ccxt/go/v4/pro"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/exchange/internal/adapters"
	"github.com/yusufozmis/go-trading-engine/exchange/options"
)

// Client provides market-data, streaming, account, and order operations for a
// configured exchange provider.
type Client struct {
	iExchange      ccxt.IExchange
	futuresAdapter adapters.FuturesAdapter
	ohlcvPageSize  int64

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

// NewBinance creates a Binance client and loads its current market metadata.
func NewBinance(opts ...options.ClientOption) (*Client, error) {
	cfg := options.ClientOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	pro := ccxtpro.NewBinance(nil)

	// Binance demo trading is distinct from its legacy sandbox/testnet.
	if cfg.PaperTrading {
		pro.EnableDemoTrading(true)
	}

	core := ccxt.NewBinanceFromCore(pro.Core.BinanceCore)

	markets, err := pro.LoadMarkets()
	if err != nil {
		return nil, normalizeError(err)
	}

	return &Client{
		iExchange:      pro,
		futuresAdapter: adapters.NewBinanceFuturesAdapter(core),
		ohlcvPageSize:  binanceOHLCVPageSize,
		markets:        markets,
	}, nil
}

// NewOKX creates an OKX client and loads its current market metadata.
func NewOKX(opts ...options.ClientOption) (*Client, error) {
	cfg := options.ClientOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	pro := ccxtpro.NewOkx(nil)

	// CCXT maps OKX sandbox mode to the provider's demo-trading environment.
	if cfg.PaperTrading {
		pro.SetSandboxMode(true)
	}

	core := ccxt.NewOkxFromCore(pro.Core.OkxCore)

	markets, err := pro.LoadMarkets()
	if err != nil {
		return nil, normalizeError(err)
	}

	return &Client{
		iExchange:      pro,
		futuresAdapter: adapters.NewOKXFuturesAdapter(core),
		ohlcvPageSize:  okxOHLCVPageSize,
		markets:        markets,
	}, nil
}

// Validate reports whether the client and its underlying exchange are initialized.
func (client *Client) Validate() error {

	if client == nil {
		return apperrors.ErrNilClient
	}

	if client.iExchange == nil {
		return apperrors.ErrUninitializedClient
	}

	return nil
}

func (client *Client) validateConfigured() error {
	if err := client.Validate(); err != nil {
		return err
	}

	if !client.configured {
		return apperrors.ErrClientNotConfigured
	}

	return nil
}
