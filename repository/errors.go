package repository

import (
	"order-management/apperror"
	"order-management/metrics"
)

func dbError(err error) error {
	metrics.DBErrors.Inc()
	return apperror.Internal().WithCause(err)
}
