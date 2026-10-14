package exchange

import (
	"fmt"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// FetchPositionSides returns the sides of open linear futures positions for
// symbol. No open position returns an empty slice; hedge mode may return both
// long and short, in that order. Duplicate sides are returned only once.
// It fetches positions only, without fetching protective orders or requiring
// entry-price and timestamp data to determine whether a position is open.
func (client *Client) FetchPositionSides(symbol string) ([]types.PositionSide, error) {
	if err := client.validateConfigured(); err != nil {
		return nil, err
	}
	if symbol == "" {
		return nil, apperrors.ErrNilSymbol
	}
	if err := client.validateFetchPositionMarket(symbol); err != nil {
		return nil, err
	}

	positions, err := client.iExchange.FetchPositions(
		ccxt.WithFetchPositionsSymbols([]string{symbol}),
	)
	if err != nil {
		return nil, normalizeError(err)
	}

	var hasLong, hasShort bool
	for _, position := range positions {
		if position.Symbol == nil || *position.Symbol != symbol ||
			position.Contracts == nil || !validPositiveNumber(*position.Contracts) {
			continue
		}
		if position.Side == nil {
			return nil, fmt.Errorf("fetch position sides %q: %w", symbol, apperrors.ErrInvalidSide)
		}
		switch types.PositionSide(*position.Side) {
		case types.PositionLong:
			hasLong = true
		case types.PositionShort:
			hasShort = true
		default:
			return nil, fmt.Errorf("fetch position sides %q: %w", symbol, apperrors.ErrInvalidSide)
		}
	}

	sides := make([]types.PositionSide, 0, 2)
	if hasLong {
		sides = append(sides, types.PositionLong)
	}
	if hasShort {
		sides = append(sides, types.PositionShort)
	}
	return sides, nil
}
