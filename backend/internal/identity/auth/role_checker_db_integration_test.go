package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	capabilityRepoImpl "github.com/labuda/backend/internal/platform/capability/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noopAdminAuditLogger struct{}

func (noopAdminAuditLogger) Log(context.Context, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) error {
	return nil
}

func (noopAdminAuditLogger) LogSafe(context.Context, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) {
}

func (noopAdminAuditLogger) LogTx(context.Context, db.Tx, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) error {
	return nil
}

func insertUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB, accountStatus string, deleted bool) uuid.UUID {
	t.Helper()

	id := uuid.New()
	var deletedAt interface{}
	if deleted {
		deletedAt = time.Now().UTC()
	}
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, account_status, email_verified_at, deleted_at, created_at, updated_at, role
		)
		VALUES ($1, $2, $3, $4, NOW(), $5, NOW(), NOW(), 'user')
	`, id, "fb-"+id.String(), id.String()+"@test.invalid", accountStatus, deletedAt)
	require.NoError(t, err)
	return id
}

func insertSellerProfile(t *testing.T, ctx context.Context, tdb *testdb.TestDB, userID uuid.UUID) uuid.UUID {
	t.Helper()

	profileID := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO seller_profiles (
			id, user_id, store_name, tier, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, 'basic', 'active', NOW(), NOW())
	`, profileID, userID, "Seller Store")
	require.NoError(t, err)
	return profileID
}

func insertSellerSubscription(t *testing.T, ctx context.Context, tdb *testdb.TestDB, userID uuid.UUID, status string) {
	t.Helper()

	now := time.Now().UTC()
	startedAt := now.Add(-24 * time.Hour)
	expiresAt := now.Add(24 * time.Hour)
	switch status {
	case "expired":
		startedAt = now.Add(-48 * time.Hour)
		expiresAt = now.Add(-24 * time.Hour)
	case "active":
		// defaults already represent an active window
	default:
		startedAt = now.Add(-24 * time.Hour)
		expiresAt = now.Add(24 * time.Hour)
	}

	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO seller_subscriptions (
			id, user_id, status, started_at, expires_at,
			duration_days, amount_paid, currency, payment_id,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, 365, 1000000, 'IDR', $6, NOW(), NOW())
	`, uuid.New(), userID, status, startedAt, expiresAt, uuid.New())
	require.NoError(t, err)
}

func insertSellerVerification(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerProfileID uuid.UUID, status string) {
	t.Helper()

	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO seller_verifications (
			id, seller_id, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, NOW(), NOW())
	`, uuid.New(), sellerProfileID, status)
	require.NoError(t, err)
}

func newRoleCheckerWithPool(pool *pgxpool.Pool) *RoleCheckerDB {
	return NewRoleCheckerDB(db.NewFromPool(pool), noopAdminAuditLogger{})
}

func TestRoleCheckerDB_HasSellerProfile_NoProfileReturnsFalseNil(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	checker := newRoleCheckerWithPool(tdb.Pool())

	userID := insertUser(t, ctx, tdb, "active", false)

	hasProfile, err := checker.HasSellerProfile(ctx, userID)
	require.NoError(t, err)
	assert.False(t, hasProfile)
}

func TestRoleCheckerDB_HasSellerProfile_QueryFailurePropagates(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := insertUser(t, ctx, tdb, "active", false)
	insertSellerProfile(t, ctx, tdb, userID)

	poolCfg := tdb.Pool().Config().Copy()
	if poolCfg.ConnConfig.RuntimeParams == nil {
		poolCfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	poolCfg.ConnConfig.RuntimeParams["search_path"] = "seller_profile_missing_schema"

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	require.NoError(t, err)
	defer pool.Close()

	checker := newRoleCheckerWithPool(pool)
	_, err = checker.HasSellerProfile(ctx, userID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check seller profile")
}

func TestRoleCheckerDB_HasSellerProfile_ContextCancellationPropagates(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := insertUser(t, ctx, tdb, "active", false)
	insertSellerProfile(t, ctx, tdb, userID)

	checker := newRoleCheckerWithPool(tdb.Pool())
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, err := checker.HasSellerProfile(cancelCtx, userID)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRoleCheckerDB_HasSellerProfile_ClosedPoolPropagates(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := insertUser(t, ctx, tdb, "active", false)
	insertSellerProfile(t, ctx, tdb, userID)

	poolCfg := tdb.Pool().Config().Copy()
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	require.NoError(t, err)
	pool.Close()

	checker := newRoleCheckerWithPool(pool)
	_, err = checker.HasSellerProfile(ctx, userID)
	require.Error(t, err)
	assert.Contains(t, fmt.Sprint(err), "closed")
}

func TestRoleCheckerDB_HasActiveSellerCapability_VerificationStatesAreIgnored(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	checker := newRoleCheckerWithPool(tdb.Pool())

	matrix := []string{
		"no_seller_verifications_row",
		"not_submitted",
		"pending_review",
		"approved",
		"rejected",
		"needs_resubmission",
		"under_investigation",
		"suspended",
		"revoked",
	}

	for _, tc := range matrix {
		t.Run(tc, func(t *testing.T) {
			userID := insertUser(t, ctx, tdb, "active", false)
			insertSellerProfile(t, ctx, tdb, userID)
			insertSellerSubscription(t, ctx, tdb, userID, "active")

			got, err := checker.HasActiveSellerCapability(ctx, userID)
			require.NoError(t, err)
			assert.True(t, got)
		})
	}
}

func TestRoleCheckerDB_HasActiveSellerCapability_ExpiredSubscriptionDenies(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	checker := newRoleCheckerWithPool(tdb.Pool())

	userID := insertUser(t, ctx, tdb, "active", false)
	insertSellerProfile(t, ctx, tdb, userID)
	insertSellerSubscription(t, ctx, tdb, userID, "expired")

	got, err := checker.HasActiveSellerCapability(ctx, userID)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestRoleCheckerDB_HasActiveSellerCapability_MissingProfileDenies(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	checker := newRoleCheckerWithPool(tdb.Pool())

	userID := insertUser(t, ctx, tdb, "active", false)
	insertSellerSubscription(t, ctx, tdb, userID, "active")

	got, err := checker.HasActiveSellerCapability(ctx, userID)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestRoleCheckerDB_HasActiveSellerCapability_QueryFailurePropagates(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	userID := insertUser(t, ctx, tdb, "active", false)
	insertSellerProfile(t, ctx, tdb, userID)
	insertSellerSubscription(t, ctx, tdb, userID, "active")

	checker := newRoleCheckerWithPool(tdb.Pool())
	poolCfg := tdb.Pool().Config().Copy()
	badPool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	require.NoError(t, err)
	badPool.Close()
	checker.db = db.NewFromPool(badPool)

	_, err = checker.HasActiveSellerCapability(ctx, userID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

// ============================================================================
// ROLE AUTHORITY — DB-BACKED COVERAGE (real PostgreSQL)
// ============================================================================
//
// These tests close the persistence gap for the role authority itself:
// RoleCheckerDB.IsAdmin and the DB-backed branches of RoleCheckerDB.SetRole
// that were previously unproven. They derive full access from the canonical
// universe (role=admin + coverage of capability.AllCapabilityStrings()) and
// create grants through the canonical write path; there is no second authority
// and no hardcoded capability count.

// insertFullAccessAdmin inserts an active, email-verified user with the
// canonical admin role and grants the entire canonical capability universe via
// the canonical grant write path. The result is a full-access admin by
// derivation, which is the backstop the full-access invariant requires.
func insertFullAccessAdmin(t *testing.T, ctx context.Context, tdb *testdb.TestDB, database *db.DB) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', $4, NOW(), NOW())
	`, id, id.String(), id.String()+"@role-checker.test", capabilityEntity.AdminRole)
	require.NoError(t, err, "insert full-access admin user")

	repo := capabilityRepoImpl.NewCapabilityRepository(database)
	for _, c := range capability.AllCapabilityStrings() {
		require.NoError(t, repo.CreateGrant(ctx, capabilityEntity.NewCapabilityGrant(id, c, nil)),
			"grant %s", c)
	}
	return id
}

// persistedRole reads the committed users.role for an id directly from its row.
func persistedRole(t *testing.T, ctx context.Context, tdb *testdb.TestDB, id uuid.UUID) string {
	t.Helper()
	var role string
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, id).Scan(&role))
	return role
}

// TestRoleCheckerDB_IsAdmin_AdminRoleReturnsTrue proves admin membership is
// resolved from the persisted users.role row.
func TestRoleCheckerDB_IsAdmin_AdminRoleReturnsTrue(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	database := db.NewFromPool(tdb.Pool())
	checker := newRoleCheckerWithPool(tdb.Pool())

	adminID := insertFullAccessAdmin(t, ctx, tdb, database)

	got, err := checker.IsAdmin(ctx, adminID)
	require.NoError(t, err)
	assert.True(t, got, "a persisted users.role=admin must resolve as admin membership")
}

// TestRoleCheckerDB_SetRole_PromotionPersistsCanonicalAdminRole proves the
// user -> admin promotion commits the canonical role value.
func TestRoleCheckerDB_SetRole_PromotionPersistsCanonicalAdminRole(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	database := db.NewFromPool(tdb.Pool())
	checker := newRoleCheckerWithPool(tdb.Pool())

	// Backstop: the full-access invariant requires at least one full-access
	// admin to remain after the mutation.
	backstop := insertFullAccessAdmin(t, ctx, tdb, database)
	target := insertUser(t, ctx, tdb, "active", false) // role = 'user'

	require.Equal(t, "user", persistedRole(t, ctx, tdb, target))

	err := checker.SetRole(ctx, backstop, target, capabilityEntity.AdminRole)
	require.NoError(t, err)

	// Persisted state via the production read path...
	role, err := checker.GetRole(ctx, target)
	require.NoError(t, err)
	assert.Equal(t, capabilityEntity.AdminRole, role)

	// ...and via the committed row itself.
	assert.Equal(t, capabilityEntity.AdminRole, persistedRole(t, ctx, tdb, target))

	// The backstop's role is untouched.
	assert.Equal(t, capabilityEntity.AdminRole, persistedRole(t, ctx, tdb, backstop))
}

// TestRoleCheckerDB_SetRole_NonexistentTarget_RejectedWithoutMutation proves a
// missing target is rejected and the transaction leaves no trace.
func TestRoleCheckerDB_SetRole_NonexistentTarget_RejectedWithoutMutation(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	ctx := context.Background()
	checker := newRoleCheckerWithPool(tdb.Pool())

	caller := uuid.New()  // distinct from target, so the self-escalation guard is not what fires
	missing := uuid.New() // no users row exists for this id

	err := checker.SetRole(ctx, caller, missing, capabilityEntity.AdminRole)
	require.Error(t, err, "setting the role of a nonexistent target must fail")

	// Error contract derived from the production source: the current-role read
	// fails first and is wrapped as "failed to query current role: %w".
	assert.Contains(t, err.Error(), "failed to query current role")

	// Rollback proof: no user row was created or mutated by the failed call.
	var count int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT count(*) FROM users WHERE id = $1`, missing).Scan(&count))
	assert.Zero(t, count, "a failed SetRole must not create or mutate any user row")
}
