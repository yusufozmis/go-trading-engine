package exchange

import (
	stderrors "errors"

	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/yusufozmis/go-trading-engine/apperrors"
)

// normalizeError exposes a stable library error while preserving the original
// CCXT error for callers that need provider-specific details.
func normalizeError(err error) error {
	if err == nil {
		return nil
	}

	var exchangeErr *ccxt.Error
	if !stderrors.As(err, &exchangeErr) {
		return err
	}

	var normalized error
	switch exchangeErr.Type {
	case ccxt.ExchangeErrorErrType,
		ccxt.OperationRejectedErrType,
		ccxt.BadRequestErrType:
		normalized = apperrors.ErrOperationRejected
	case ccxt.InsufficientFundsErrType:
		normalized = apperrors.ErrInsufficientFunds
	case ccxt.AuthenticationErrorErrType:
		normalized = apperrors.ErrAuthenticationFailed
	case ccxt.PermissionDeniedErrType:
		normalized = apperrors.ErrPermissionDenied
	case ccxt.RateLimitExceededErrType:
		normalized = apperrors.ErrRateLimitExceeded
	case ccxt.BadSymbolErrType:
		normalized = apperrors.ErrInvalidSymbol
	case ccxt.InvalidOrderErrType:
		normalized = apperrors.ErrInvalidOrder
	default:
		return err
	}

	return normalized
}
