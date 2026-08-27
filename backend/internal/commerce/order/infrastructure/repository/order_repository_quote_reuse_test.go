package repository

import (
	"testing"

	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/stretchr/testify/require"
)

func TestQuoteBlockingOrderStatuses(t *testing.T) {
	statuses := quoteBlockingOrderStatuses()

	require.ElementsMatch(t, []string{
		string(entity.StatusPending),
		string(entity.StatusPaid),
		string(entity.StatusShipped),
		string(entity.StatusDelivered),
		string(entity.StatusDisputeOpen),
		string(entity.StatusCompleted),
		string(entity.StatusPartiallyRefunded),
	}, statuses)

	for _, status := range []entity.Status{
		entity.StatusCancelled,
		entity.StatusCancelledTimeout,
		entity.StatusRefunded,
		entity.StatusExpired,
	} {
		require.NotContains(t, statuses, string(status))
	}
}


