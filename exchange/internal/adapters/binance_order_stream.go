package adapters

import (
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// BinanceOrderStreamAdapter translates market types into Binance account streams.
type BinanceOrderStreamAdapter struct{}

// NewBinanceOrderStreamAdapter creates a Binance private-order stream adapter.
func NewBinanceOrderStreamAdapter() *BinanceOrderStreamAdapter {
	return &BinanceOrderStreamAdapter{}
}

// WatcherConfigs returns the Binance account stream required by the market.
func (*BinanceOrderStreamAdapter) WatcherConfigs(
	marketType types.MarketType,
) ([]OrderWatcherConfig, error) {
	switch marketType {
	case types.MarketSpot:
		return []OrderWatcherConfig{{MarketType: "spot"}}, nil
	case types.MarketFutures:
		return []OrderWatcherConfig{{MarketType: "future"}}, nil
	default:
		return nil, apperrors.ErrInvalidMarketType
	}
}

// SubscriptionKey follows WatchOrders, which registers the user-data market type
// (spot or future), not its orders result hash. Conditional events share this stream.
func (*BinanceOrderStreamAdapter) SubscriptionKey(config OrderWatcherConfig) string {
	return config.MarketType
}

var _ OrderStreamAdapter = (*BinanceOrderStreamAdapter)(nil)
