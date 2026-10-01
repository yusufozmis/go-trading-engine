package exchange

import (
	"fmt"
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// FetchPositions returns open long and short linear futures positions for the
// requested symbols. Calling it without symbols returns all open positions.
// Hedge mode may return two positions for one symbol. Amounts are converted
// from exchange contracts to base-asset units. Timeframe is left empty because
// it is strategy state rather than exchange position data; callers must set it
// before preloading a position into an engine. TP and SL also remain zero when
// the provider does not include attached orders in its position response. An
// empty slice means no matching position is currently open.
func (client *Client) FetchPositions(symbols ...string) ([]types.Position, error) {
	if err := client.validateConfigured(); err != nil {
		return nil, err
	}

	requestedSymbols := make(map[string]bool, len(symbols))
	for _, symbol := range symbols {
		if symbol == "" {
			return nil, apperrors.ErrNilSymbol
		}
		if err := client.validateFetchPositionMarket(symbol); err != nil {
			return nil, err
		}
		requestedSymbols[symbol] = true
	}

	// FetchPositions is used instead of FetchPosition because Binance's unified
	// FetchPosition supports options only, and hedge mode may return two sides
	// for the same perpetual-futures symbol.
	var fetchOptions []ccxt.FetchPositionsOptions
	if len(symbols) > 0 {
		fetchOptions = append(fetchOptions, ccxt.WithFetchPositionsSymbols(symbols))
	}
	positions, err := client.iExchange.FetchPositions(fetchOptions...)
	if err != nil {
		return nil, normalizeError(err)
	}

	result := make([]types.Position, 0, len(positions))
	for _, position := range positions {
		if position.Symbol == nil {
			continue
		}
		symbol := *position.Symbol
		if len(requestedSymbols) > 0 && !requestedSymbols[symbol] {
			continue
		}
		if position.Contracts == nil || !validPositiveNumber(*position.Contracts) {
			continue
		}
		if err := client.validateFetchPositionMarket(symbol); err != nil {
			return nil, err
		}
		market := client.markets[symbol]
		if position.Side == nil {
			return nil, fmt.Errorf("fetch positions %q: %w", symbol, apperrors.ErrInvalidSide)
		}
		positionSide := types.PositionSide(*position.Side)
		if !positionSide.Valid() {
			return nil, fmt.Errorf("fetch positions %q: %w", symbol, apperrors.ErrInvalidSide)
		}
		if position.EntryPrice == nil || !validPositiveNumber(*position.EntryPrice) {
			return nil, fmt.Errorf(
				"fetch positions %q: %w",
				symbol,
				apperrors.ErrInvalidPrice,
			)
		}
		if position.Timestamp == nil || math.IsNaN(*position.Timestamp) ||
			math.IsInf(*position.Timestamp, 0) || *position.Timestamp <= 0 {
			return nil, fmt.Errorf(
				"fetch positions %q: %w",
				symbol,
				apperrors.ErrInvalidTimestamp,
			)
		}

		wrapped := types.Position{
			Symbol:        symbol,
			OpenTimestamp: int64(*position.Timestamp),
			Side:          positionSide,
			EntryPrice:    *position.EntryPrice,
			Amount:        *position.Contracts * *market.ContractSize,
			State:         types.PositionOpen,
		}
		if !validPositiveNumber(wrapped.Amount) {
			return nil, apperrors.ErrInvalidAmount
		}

		if position.StopLossPrice != nil && validPositiveNumber(*position.StopLossPrice) {
			wrapped.StopLoss = *position.StopLossPrice
		}
		if position.TakeProfitPrice != nil && validPositiveNumber(*position.TakeProfitPrice) {
			wrapped.TP = *position.TakeProfitPrice
		}
		// Keep liquidation fields paired so the returned Position preserves its
		// invariant when a provider omits liquidation data, as cross margin may.
		if position.Leverage != nil && validPositiveNumber(*position.Leverage) &&
			*position.Leverage > 1 && position.LiquidationPrice != nil &&
			validPositiveNumber(*position.LiquidationPrice) {
			wrapped.Leverage = *position.Leverage
			wrapped.LiquidationPrice = *position.LiquidationPrice
		}

		result = append(result, wrapped)
	}

	return result, nil
}

func (client *Client) validateFetchPositionMarket(symbol string) error {
	market, exists := client.markets[symbol]
	if !exists || market.Swap == nil || !*market.Swap {
		return apperrors.ErrInvalidSymbol
	}
	// Engine amounts and PnL are base-denominated. Inverse contracts require a
	// price-dependent conversion and are not supported by this method.
	if market.Linear == nil || !*market.Linear {
		return apperrors.ErrUnsupportedFuturesMarket
	}
	if market.ContractSize == nil || !validPositiveNumber(*market.ContractSize) {
		return apperrors.ErrContractSizeUnavailable
	}

	return nil
}

func validPositiveNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}
