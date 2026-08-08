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
		return errors.ErrInvalidAmount // replace with ErrInvalidLeverage if you add one
	}

	if cfg.MarginMode != types.MarginModeCross && cfg.MarginMode != types.MarginModeIsolated {
		return errors.ErrInvalidSide // replace with ErrInvalidMarginMode if you add one
	}

	return nil
}

func (client *Client) SetFuturesConfig(cfg FuturesConfigs) error {
	if err := client.validate(); err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	client.futuresConfigs = cfg

	return nil
}
