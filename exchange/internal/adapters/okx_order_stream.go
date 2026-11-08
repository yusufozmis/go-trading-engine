package adapters

import (
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// OKXOrderStreamAdapter translates market types into OKX account streams.
type OKXOrderStreamAdapter struct{}

// NewOKXOrderStreamAdapter creates an OKX private-order stream adapter.
func NewOKXOrderStreamAdapter() *OKXOrderStreamAdapter {
	return &OKXOrderStreamAdapter{}
}

// WatcherConfigs includes the separate orders-algo subscription used by OKX
// for futures TP/SL and other conditional orders.
func (*OKXOrderStreamAdapter) WatcherConfigs(
	marketType types.MarketType,
) ([]OrderWatcherConfig, error) {
	switch marketType {
	case types.MarketSpot:
		return []OrderWatcherConfig{{MarketType: "spot"}}, nil
	case types.MarketFutures:
		return []OrderWatcherConfig{
			{MarketType: "swap"},
			{MarketType: "swap", Trigger: true},
		}, nil
	default:
		return nil, apperrors.ErrInvalidMarketType
	}
}

// SubscriptionKey distinguishes the regular and separate algo-order channels.
// This library watches them account-wide, so neither key contains a symbol.
func (*OKXOrderStreamAdapter) SubscriptionKey(config OrderWatcherConfig) string {
	if config.Trigger {
		return "orders-algo"
	}
	return "orders"
}

var _ OrderStreamAdapter = (*OKXOrderStreamAdapter)(nil)
