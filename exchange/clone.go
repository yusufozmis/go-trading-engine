package exchange

import (
	"maps"

	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/exchange/options"
)

// Clone creates an independent client for the same provider, credentials, and
// live/demo environment. Fresh CCXT instances load current market metadata;
// connections, channels, request caches, and mutexes are never copied.
// Local futures settings and prepared-symbol state are preserved without
// reapplying account settings or placing orders. Both clients use the same
// remote account; cloning does not duplicate or restore exchange positions.
// Active streams are not restarted: the caller must start the desired streams
// on the clone and retire the original client as appropriate.
// Do not call concurrently with configuration changes.
func (client *Client) Clone() (*Client, error) {
	if err := client.Validate(); err != nil {
		return nil, err
	}
	client.futuresMu.Lock()
	futuresConfig := client.futuresConfigs
	preparedSymbols := maps.Clone(client.preparedFuturesSymbols)
	client.futuresMu.Unlock()

	var opts []options.ClientOption
	if client.clientOptions.PaperTrading {
		opts = append(opts, options.WithPaperTrading())
	}
	var clone *Client
	var err error
	switch client.iExchange.GetId() {
	case "binance":
		clone, err = NewBinance(opts...)
	case "okx":
		clone, err = NewOKX(opts...)
	default:
		return nil, apperrors.ErrUnsupportedProvider
	}
	if err != nil {
		return nil, err
	}
	if client.configured {
		if err := clone.SetConfig(client.exchangeConfig); err != nil {
			return nil, err
		}
	}
	clone.futuresConfigs = futuresConfig
	clone.preparedFuturesSymbols = preparedSymbols
	return clone, nil
}
