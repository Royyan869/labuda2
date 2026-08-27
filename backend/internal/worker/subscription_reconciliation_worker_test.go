//go:build integration

package worker

import (
	"context"
	"testing"

	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestSubscriptionReconciliationWorker_EmptyDatabase_IsHealthy(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	worker := NewSubscriptionReconciliationWorker(
		tdb.Pool(),
		zaptest.NewLogger(t),
		DefaultSubscriptionReconciliationConfig(),
		nil,
	)

	results := worker.runChecks(context.Background())
	require.Len(t, results, 3)

	for _, result := range results {
		require.Equal(t, "OK", result.Severity)
	}

	require.Equal(t, "No orphaned subscription payments", results[0].Message)
	require.Equal(t, "Subscription lifecycle healthy", results[1].Message)
	require.Equal(t, "No subscription payments yet", results[2].Message)
}

