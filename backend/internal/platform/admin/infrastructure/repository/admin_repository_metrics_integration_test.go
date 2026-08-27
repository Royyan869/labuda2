package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	adminrepo "github.com/labuda/backend/internal/platform/admin/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertMetricsTestUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB, role string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, account_status, email_verified_at, created_at, updated_at, role
		)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), NOW(), $4)
	`, id, "fb-"+id.String(), id.String()+"@metrics.test", role)
	require.NoError(t, err)
	return id
}

func insertSellerProfile(t *testing.T, ctx context.Context, tdb *testdb.TestDB, userID uuid.UUID, status string) {
	t.Helper()

	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO seller_profiles (
			id, user_id, store_name, tier, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, 'basic', $4, NOW(), NOW())
	`, uuid.New(), userID, "Metrics Store", status)
	require.NoError(t, err)
}

func insertSellerSubscription(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	userID uuid.UUID,
	status string,
	startedAt, expiresAt time.Time,
) {
	t.Helper()

	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO seller_subscriptions (
			id, user_id, status, started_at, expires_at,
			duration_days, amount_paid, currency, payment_id,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, 365, 70000, 'IDR', $6, NOW(), NOW())
	`, uuid.New(), userID, status, startedAt, expiresAt, uuid.New())
	require.NoError(t, err)
}

func countActiveSellers(t *testing.T, ctx context.Context, tdb *testdb.TestDB) int64 {
	t.Helper()

	repo := &AdminRepositoryImpl{}
	var metrics *adminrepo.DashboardMetrics
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		metrics, err = repo.GetDashboardMetrics(ctx, tx)
		return err
	})
	require.NoError(t, err)
	require.NotNil(t, metrics)
	return metrics.ActiveSellers
}

func TestAdminRepository_GetDashboardMetrics_CountsEligibleSellerIntervals(t *testing.T) {
	now := time.Now().UTC()
	activeWindowStart := now.Add(-1 * time.Hour)
	activeWindowEnd := now.Add(1 * time.Hour)
	staleWindowStart := now.Add(-72 * time.Hour)
	staleWindowEnd := now.Add(-24 * time.Hour)

	t.Run("active_interval_is_counted", func(t *testing.T) {
		tdb, cleanup := testdb.SetupDB(t)
		defer cleanup()

		ctx := context.Background()
		userID := insertMetricsTestUser(t, ctx, tdb, "user")
		insertSellerProfile(t, ctx, tdb, userID, "active")
		insertSellerSubscription(t, ctx, tdb, userID, "active", activeWindowStart, activeWindowEnd)

		assert.EqualValues(t, 1, countActiveSellers(t, ctx, tdb))
	})

	t.Run("inactive_interval_is_not_counted", func(t *testing.T) {
		tdb, cleanup := testdb.SetupDB(t)
		defer cleanup()

		ctx := context.Background()
		userID := insertMetricsTestUser(t, ctx, tdb, "user")
		insertSellerProfile(t, ctx, tdb, userID, "active")
		insertSellerSubscription(t, ctx, tdb, userID, "expired", activeWindowStart, activeWindowEnd)

		assert.EqualValues(t, 0, countActiveSellers(t, ctx, tdb))
	})

	t.Run("stale_active_status_after_interval_end_is_not_counted", func(t *testing.T) {
		tdb, cleanup := testdb.SetupDB(t)
		defer cleanup()

		ctx := context.Background()
		userID := insertMetricsTestUser(t, ctx, tdb, "user")
		insertSellerProfile(t, ctx, tdb, userID, "active")
		insertSellerSubscription(t, ctx, tdb, userID, "active", staleWindowStart, staleWindowEnd)

		assert.EqualValues(t, 0, countActiveSellers(t, ctx, tdb))
	})

	t.Run("multiple_matching_rows_count_once", func(t *testing.T) {
		tdb, cleanup := testdb.SetupDB(t)
		defer cleanup()

		ctx := context.Background()
		userID := insertMetricsTestUser(t, ctx, tdb, "user")
		insertSellerProfile(t, ctx, tdb, userID, "active")
		insertSellerSubscription(t, ctx, tdb, userID, "active", activeWindowStart, activeWindowEnd)

		assert.EqualValues(t, 1, countActiveSellers(t, ctx, tdb))
	})

	t.Run("deleted_or_inactive_user_is_not_counted", func(t *testing.T) {
		tdb, cleanup := testdb.SetupDB(t)
		defer cleanup()

		ctx := context.Background()
		deletedID := insertMetricsTestUser(t, ctx, tdb, "user")
		insertSellerProfile(t, ctx, tdb, deletedID, "active")
		insertSellerSubscription(t, ctx, tdb, deletedID, "active", activeWindowStart, activeWindowEnd)

		inactiveID := insertMetricsTestUser(t, ctx, tdb, "user")
		_, err := tdb.Pool().Exec(ctx, `UPDATE users SET account_status = 'suspended' WHERE id = $1`, inactiveID)
		require.NoError(t, err)
		insertSellerProfile(t, ctx, tdb, inactiveID, "active")
		insertSellerSubscription(t, ctx, tdb, inactiveID, "active", activeWindowStart, activeWindowEnd)

		_, err = tdb.Pool().Exec(ctx, `UPDATE users SET deleted_at = NOW() WHERE id = $1`, deletedID)
		require.NoError(t, err)

		assert.EqualValues(t, 0, countActiveSellers(t, ctx, tdb), "deleted/inactive users must not count")
	})
}
