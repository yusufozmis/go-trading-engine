package exchange

import (
	"math"

	ccxt "github.com/ccxt/ccxt/go/v4"
	ccxtpro "github.com/ccxt/ccxt/go/v4/pro"
	"github.com/yusufozmis/trading-library/errors"
	"github.com/yusufozmis/trading-library/types"
)

type futuresAdapter interface {
	Prepare(req futuresOrderRequest) error
	OrderParams(req futuresOrderRequest) map[string]any
}

type FuturesExchange interface {
	SetLeverage(leverage int64, options ...ccxt.SetLeverageOptions) (map[string]any, error)
	SetMarginMode(marginMode string, options ...ccxt.SetMarginModeOptions) (map[string]any, error)
	SetPositionMode(hedged bool, options ...ccxt.SetPositionModeOptions) (map[string]any, error)
}

type Client struct {
	iExchange ccxt.IExchange
	adapter   futuresAdapter

	stream *candleStream

	exchangeConfig ExchangeConfig
	futuresConfigs FuturesConfigs
}

func NewBinance() *Client {
	pro := ccxtpro.NewBinance(nil)
	core := ccxt.NewBinanceFromCore(pro.Core.BinanceCore)

	return &Client{
		iExchange: pro,
		adapter:   NewBinanceFuturesAdapter(core),
	}
}

func NewOkx() *Client {
	pro := ccxtpro.NewOkx(nil)
	core := ccxt.NewOkxFromCore(pro.Core.OkxCore)

	return &Client{
		iExchange: pro,
		adapter:   NewOkxFuturesAdapter(core),
	}
}

func (client *Client) validate() error {

	if client == nil {
		return errors.ErrNilClient
	}

	if client.iExchange == nil {
		return errors.ErrUninitializedClient
	}

	return nil
}

func (client *Client) wrapOHLCV(symbol, timeframe string, ohlcv ccxt.OHLCV) types.Candle {
	return types.Candle{
		Symbol:    symbol,
		Timeframe: timeframe,
		Timestamp: ohlcv.Timestamp,
		PriceData: types.Prices{
			OpenPrice:  ohlcv.Open,
			HighPrice:  ohlcv.High,
			LowPrice:   ohlcv.Low,
			ClosePrice: ohlcv.Close,
		},
		Volume: ohlcv.Volume,
	}
}

// FetchCandles returns up to limit fully formed candles for the given symbol and timeframe,
// conservatively dropping the newest fetched candle because it may still be in progress.
func (client *Client) FetchCandles(symbol, timeframe string, limit int64) ([]types.Candle, error) {

	if err := client.validate(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		return nil, errors.ErrInvalidLimit
	}

	if limit == math.MaxInt64 {
		return nil, errors.ErrLimitTooLarge
	}

	candles, err := client.iExchange.FetchOHLCV(
		symbol,
		ccxt.WithFetchOHLCVTimeframe(timeframe),
		ccxt.WithFetchOHLCVLimit(limit+1),
	)
	if err != nil {
		return nil, err
	}

	if len(candles) == 0 {
		return nil, errors.ErrNoCandles
	}

	// Never trust the newest candle.
	// It may still be forming, and without an exchange-specific confirm flag
	// we cannot know for sure.
	candles = candles[:len(candles)-1]

	if limit > 0 && int64(len(candles)) > limit {
		candles = candles[len(candles)-int(limit):]
	}

	var wrapped []types.Candle

	for _, ohlcv := range candles {
		wrapped = append(wrapped, client.wrapOHLCV(symbol, timeframe, ohlcv))
	}

	return wrapped, nil
}
