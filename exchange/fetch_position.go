package exchange

import (
	"fmt"
	"math"
	"strconv"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
	"github.com/yusufozmis/go-trading-engine/types"
)

// FetchPositions returns open long and short linear futures positions for the
// requested symbols. Calling it without symbols returns all open positions.
// Hedge mode may return two positions for one symbol. Amounts are converted
// from exchange contracts to base-asset units. Timeframe is left empty because
// it is strategy state rather than exchange position data; callers must set it
// before preloading a position into an engine. Missing TP/SL prices are filled
// from the first matching active protective orders. Orders are fetched once per
// symbol; additional TP/SL levels and pages are not inspected. Missing protection
// remains zero. An empty slice means no matching position is currently open.
// FetchPositionSides is queried once per open symbol to confirm its active sides.
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
	// Both hedge sides reuse the same symbol's order response.
	bySymbol := make(map[string][]OrderUpdate)
	sidesBySymbol := make(map[string][]types.PositionSide)
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
		sides, fetched := sidesBySymbol[symbol]
		if !fetched {
			sides, err = client.FetchPositionSides(symbol)
			if err != nil {
				return nil, err
			}
			sidesBySymbol[symbol] = sides
		}
		if len(sides) == 0 {
			continue
		}
		if position.Side == nil {
			return nil, fmt.Errorf("fetch positions %q: %w", symbol, apperrors.ErrInvalidSide)
		}
		// A symbol can have two hedge records. Match this record to a fetched
		// side rather than assigning the first side to both long and short.
		var positionSide types.PositionSide
		for _, side := range sides {
			if side.String() == *position.Side {
				positionSide = side
				break
			}
		}
		if positionSide == "" {
			// The position may have closed between the two snapshots.
			continue
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

		if wrapped.TP == 0 || wrapped.StopLoss == 0 {
			orders, fetched := bySymbol[symbol]
			if !fetched {
				orders, err = client.FetchOpenOrders(symbol)
				if err != nil {
					return nil, err
				}
				bySymbol[symbol] = orders
			}
			marginMode := types.MarginMode("")
			if position.MarginMode != nil {
				marginMode = types.MarginMode(*position.MarginMode)
			}
			for _, order := range orders {
				if positionProtectionMatches(wrapped, marginMode, order) {
					applyPositionProtection(&wrapped, order)
				}
				if wrapped.TP != 0 && wrapped.StopLoss != 0 {
					break
				}
			}
		}
		result = append(result, wrapped)
	}

	return result, nil
}

// positionProtectionMatches rejects entry orders and the other hedge side.
// One-way orders must explicitly reduce/close the position; matching a symbol
// and a buy/sell direction alone could otherwise select a new entry order.
func positionProtectionMatches(position types.Position, marginMode types.MarginMode, order OrderUpdate) bool {
	if order.Err != nil || order.Symbol != position.Symbol ||
		(order.Status != "" && order.Status != "open") {
		return false
	}
	if (position.Side == types.PositionLong && order.Side != "sell") ||
		(position.Side == types.PositionShort && order.Side != "buy") {
		return false
	}
	if order.MarginMode != "" && order.MarginMode != marginMode {
		return false
	}
	switch order.PositionSide {
	case types.PositionLong, types.PositionShort:
		return order.PositionSide == position.Side
	case "", "net", "both":
		reduceOnly := order.ReduceOnly != nil && *order.ReduceOnly
		return reduceOnly || order.ClosePosition
	default:
		return false
	}
}

// applyPositionProtection fills missing TP/SL fields from one matching order.
// Keep provider-reported position levels and the first matched order levels.
func applyPositionProtection(position *types.Position, order OrderUpdate) {
	tpPrice, slPrice := order.TakeProfitPrice, order.StopLossPrice
	if position.TP == 0 && tpPrice != nil && validPositiveNumber(*tpPrice) {
		position.TP = *tpPrice
	}
	if position.StopLoss == 0 && slPrice != nil && validPositiveNumber(*slPrice) {
		position.StopLoss = *slPrice
	}
}

// FetchClosedPositions returns up to limit completed linear-futures positions
// for one symbol. Entry and exit prices, open and close timestamps, side,
// base-asset amount, trading fees, leverage, and realized net profit are copied
// when the provider supplies them. Timeframe, TP, and SL remain zero-valued
// because they are strategy metadata rather than position-history data.
// SlippageCost also remains zero because the exchange does not report the
// strategy's expected price needed to calculate execution slippage.
//
// State remains UNSPECIFIED because a normal exchange history record does not
// reliably reveal whether TP, SL, duration, or a manual action caused the close.
// The method requires CCXT's fetchPositionsHistory capability, which is
// currently available for OKX but not Binance.
func (client *Client) FetchClosedPositions(symbol string, limit int64) ([]types.Position, error) {
	if err := client.validateConfigured(); err != nil {
		return nil, err
	}
	if symbol == "" {
		return nil, apperrors.ErrNilSymbol
	}
	if err := client.validateFetchPositionMarket(symbol); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, apperrors.ErrInvalidLimit
	}

	supported, _ := client.iExchange.GetHas()["fetchPositionsHistory"].(bool)
	if !supported {
		return nil, apperrors.ErrUnsupportedProvider
	}

	history, err := client.iExchange.FetchPositionsHistory(
		ccxt.WithFetchPositionsHistorySymbols([]string{symbol}),
		ccxt.WithFetchPositionsHistoryLimit(limit),
	)
	if err != nil {
		return nil, normalizeError(err)
	}

	market := client.markets[symbol]
	positions := make([]types.Position, 0, len(history))
	for _, historicalPosition := range history {
		if historicalPosition.Symbol == nil || *historicalPosition.Symbol != symbol {
			continue
		}

		// OKX also includes partial reductions in this history. Types 1 and 4
		// are not fully closed positions, so do not expose them as such.
		closeType, _ := historyString(historicalPosition.Info, "type")
		if closeType == "1" || closeType == "4" {
			continue
		}

		side, err := closedPositionSide(historicalPosition)
		if err != nil {
			return nil, fmt.Errorf("fetch closed positions %q: %w", symbol, err)
		}

		entryPrice, err := requiredHistoryPrice(historicalPosition.EntryPrice)
		if err != nil {
			return nil, fmt.Errorf("fetch closed positions %q entry: %w", symbol, err)
		}
		exitPrice, err := requiredHistoryPrice(historicalPosition.LastPrice)
		if err != nil {
			return nil, fmt.Errorf("fetch closed positions %q exit: %w", symbol, err)
		}

		openTimestamp, err := requiredHistoryTimestamp(historicalPosition.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("fetch closed positions %q open: %w", symbol, err)
		}
		closeTimestamp, err := requiredHistoryTimestamp(
			historicalPosition.LastUpdateTimestamp,
		)
		if err != nil || closeTimestamp < openTimestamp {
			return nil, fmt.Errorf(
				"fetch closed positions %q close: %w",
				symbol,
				apperrors.ErrInvalidTimestamp,
			)
		}

		// CCXT currently leaves Contracts nil for OKX history because OKX names
		// this field closeTotalPos. Prefer the unified field, then use the raw
		// provider value as a compatibility fallback.
		contracts := 0.0
		if historicalPosition.Contracts != nil {
			contracts = *historicalPosition.Contracts
		}
		if !validPositiveNumber(contracts) {
			contracts, _ = historyNumber(historicalPosition.Info, "closeTotalPos")
		}
		amount := contracts * *market.ContractSize
		if !validPositiveNumber(amount) {
			return nil, fmt.Errorf(
				"fetch closed positions %q: %w",
				symbol,
				apperrors.ErrInvalidAmount,
			)
		}

		position := types.Position{
			Symbol:         symbol,
			OpenTimestamp:  openTimestamp,
			CloseTimestamp: closeTimestamp,
			Side:           side,
			EntryPrice:     entryPrice,
			ExitPrice:      exitPrice,
			Amount:         amount,
			State:          types.UNSPECIFIED,
		}

		if historicalPosition.Leverage != nil &&
			validPositiveNumber(*historicalPosition.Leverage) {
			position.Leverage = *historicalPosition.Leverage
		}
		if fee, ok := historyNumber(historicalPosition.Info, "fee"); ok {
			position.Fee = math.Abs(fee)
		}
		if historicalPosition.RealizedPnl != nil {
			if math.IsNaN(*historicalPosition.RealizedPnl) ||
				math.IsInf(*historicalPosition.RealizedPnl, 0) {
				return nil, fmt.Errorf(
					"fetch closed positions %q: %w",
					symbol,
					apperrors.ErrInvalidNetProfit,
				)
			}
			position.NetProfit = *historicalPosition.RealizedPnl
		}

		positions = append(positions, position)
	}

	return positions, nil
}

func closedPositionSide(position ccxt.Position) (types.PositionSide, error) {
	var side types.PositionSide
	if position.Side != nil {
		side = types.PositionSide(*position.Side)
	}

	// OKX returns historical direction in the raw `direction` field. Some CCXT
	// Go responses do not propagate it to the unified Position.Side field.
	if !side.Valid() {
		if direction, ok := historyString(position.Info, "direction"); ok {
			side = types.PositionSide(direction)
		}
	}
	if !side.Valid() {
		return "", apperrors.ErrInvalidSide
	}

	return side, nil
}

func requiredHistoryPrice(price *float64) (float64, error) {
	if price == nil || !validPositiveNumber(*price) {
		return 0, apperrors.ErrInvalidPrice
	}

	return *price, nil
}

func requiredHistoryTimestamp(timestamp *float64) (int64, error) {
	if timestamp == nil || math.IsNaN(*timestamp) ||
		math.IsInf(*timestamp, 0) || *timestamp <= 0 {
		return 0, apperrors.ErrInvalidTimestamp
	}

	return int64(*timestamp), nil
}

func historyNumber(info map[string]any, key string) (float64, bool) {
	value, exists := info[key]
	if !exists || value == nil {
		return 0, false
	}

	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case string:
		parsed, err := strconv.ParseFloat(number, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func historyString(info map[string]any, key string) (string, bool) {
	value, exists := info[key]
	if !exists || value == nil {
		return "", false
	}

	stringValue, ok := value.(string)
	return stringValue, ok
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
