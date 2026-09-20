package options

// ClientOptions contains options applied while an exchange client is created.
type ClientOptions struct {
	PaperTrading bool
}

// ClientOption configures an exchange client before its first provider request.
type ClientOption func(*ClientOptions)

// WithPaperTrading routes requests to the provider-hosted demo environment.
// It requires demo-specific API credentials. Providers may also require account
// setup outside the API; for example, OKX users must select a futures-capable
// account mode in the Demo Trading UI before submitting futures orders.
func WithPaperTrading() ClientOption {
	return func(options *ClientOptions) {
		options.PaperTrading = true
	}
}
