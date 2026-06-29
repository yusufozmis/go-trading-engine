package types

type Strategy interface {
	AddBar(candle Candle)
	Calculate(ctx Context) (PlanUpdate, error)
}
