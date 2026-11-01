package engine

import (
	"math"
	"strconv"
	"time"

	"github.com/yusufozmis/go-trading-engine/apperrors"
)

// nextCandleTimestamp advances unified timeframes without depending on an exchange.
func nextCandleTimestamp(timestamp int64, timeframe string) (int64, error) {
	if len(timeframe) < 2 {
		return 0, apperrors.ErrInvalidTimeframe
	}
	count, err := strconv.ParseInt(timeframe[:len(timeframe)-1], 10, 64)
	if err != nil || count <= 0 {
		return 0, apperrors.ErrInvalidTimeframe
	}

	var unitMilliseconds int64
	switch timeframe[len(timeframe)-1] {
	case 's':
		unitMilliseconds = 1000
	case 'm':
		unitMilliseconds = 60 * 1000
	case 'h':
		unitMilliseconds = 60 * 60 * 1000
	case 'd':
		unitMilliseconds = 24 * 60 * 60 * 1000
	case 'w':
		unitMilliseconds = 7 * 24 * 60 * 60 * 1000
	case 'M':
		// Bound the calendar offset before converting it to int for AddDate.
		if count > math.MaxInt64/(31*24*60*60*1000) {
			return 0, apperrors.ErrInvalidTimeframe
		}
		next := time.UnixMilli(timestamp).UTC().AddDate(0, int(count), 0).UnixMilli()
		if next <= timestamp {
			return 0, apperrors.ErrInvalidTimestamp
		}
		return next, nil
	default:
		return 0, apperrors.ErrInvalidTimeframe
	}
	if count > math.MaxInt64/unitMilliseconds {
		return 0, apperrors.ErrInvalidTimeframe
	}
	interval := count * unitMilliseconds
	if timestamp > math.MaxInt64-interval {
		return 0, apperrors.ErrInvalidTimestamp
	}
	return timestamp + interval, nil
}
