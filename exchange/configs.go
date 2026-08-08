package exchange

import (
	"github.com/yusufozmis/trading-library/errors"
	"github.com/yusufozmis/trading-library/types"
)

type FuturesConfigs struct {
	Leverage   int64
	MarginMode types.MarginMode
	Hedged     bool
}

func (cfg FuturesConfigs) Validate() error {
	if cfg.Leverage <= 0 {
		return errors.ErrInvalidLeverage
	}

	if !cfg.MarginMode.Valid() {
		return errors.ErrInvalidMarginMode
	}

	return nil
}

func (client *Client) SetFuturesConfig(cfg FuturesConfigs) error {
	if err := client.Validate(); err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	client.futuresConfigs = cfg

	return nil
}
