package invariant_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	capabilityRepoImpl "github.com/labuda/backend/internal/platform/capability/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/capability/invariant"
	capabilityRepo "github.com/labuda/backend/internal/platform/capability/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

type noopAuditLogger struct{}

func (noopAuditLogger) Log(context.Context, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) error {
	return nil
}
func (noopAuditLogger) LogSafe(context.Context, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) {
}
func (noopAuditLogger) LogTx(context.Context, db.Tx, uuid.UUID, string, string, uuid.UUID, map[string]interface{}) error {
	return nil
}

// insertAdmin creates an active, email-verified admin with the entire canonical
// capability universe — a canonical full-access admin — via the canonical grant
// write path, which also proves that path is actually functional.
func insertAdmin(t *testing.T, ctx context.Context, tdb *testdb.TestDB, repo capabilityRepo.CapabilityRepository) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', 'admin', NOW(), NOW())
	`, id, id.String(), id.String()+"@test.local")
	require.NoError(t, err, "insert admin user")

	for _, c := range capability.AllCapabilityStrings() {
		grant := capabilityEntity.NewCapabilityGrant(id, c, nil)
		require.NoError(t, repo.CreateGrant(ctx, grant), "grant %s", c)
	}
	return id
}

func newRepo(t *testing.T, tdb *testdb.TestDB) (capabilityRepo.CapabilityRepository, *db.DB) {
	t.Helper()
	database := db.NewFromPool(tdb.Pool())
	return capabilityRepoImpl.NewCapabilityRepository(database), database
}

func insertPlainAdmin(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', 'admin', NOW(), NOW())
	`, id, id.String(), id.String()+"@test.local")
	require.NoError(t, err)
	return id
}

func firstActiveGrant(t *testing.T, ctx context.Context, tdb *testdb.TestDB, userID uuid.UUID) uuid.UUID {
	t.Helper()
	var grantID uuid.UUID
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT id FROM user_capabilities
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY capability
		LIMIT 1
	`, userID).Scan(&grantID))
	return grantID
}

// TestInvariant_RemovingOneOfTwo_Succeeds proves a reduction is allowed while
// another full-access admin remains.
func TestInvariant_RemovingOneOfTwo_Succeeds(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo, database := newRepo(t, tdb)
	a := insertAdmin(t, ctx, tdb, repo)
	_ = insertAdmin(t, ctx, tdb, repo)

	rc := auth.NewRoleCheckerDB(database, noopAuditLogger{})
	require.NoError(t, rc.SetRole(ctx, adminCaller(), a, "user"),
		"demoting A is allowed because another full-access admin remains")

	role, err := rc.GetRole(ctx, a)
	require.NoError(t, err)
	require.Equal(t, "user", role)
}

// TestInvariant_LastFullAccessAdminDemotion_IsRejected proves the final
// full-access admin cannot be demoted, and that the refusal is atomic.
func TestInvariant_LastFullAccessAdminDemotion_IsRejected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo, database := newRepo(t, tdb)
	a := insertAdmin(t, ctx, tdb, repo)

	rc := auth.NewRoleCheckerDB(database, noopAuditLogger{})
	err := rc.SetRole(ctx, adminCaller(), a, "user")
	require.Error(t, err, "demoting the last full-access admin must be rejected")
	require.True(t, errors.Is(err, invariant.ErrLastFullAccessAdmin),
		"expected ErrLastFullAccessAdmin, got %v", err)

	role, err := rc.GetRole(ctx, a)
	require.NoError(t, err)
	require.Equal(t, capabilityEntity.AdminRole, role, "role change must be rolled back")
}

// TestInvariant_LastFullAccessAdminCapabilityRevocation_IsRejected proves the
// final full-access admin cannot lose the capability that breaks coverage.
func TestInvariant_LastFullAccessAdminCapabilityRevocation_IsRejected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo, _ := newRepo(t, tdb)
	a := insertAdmin(t, ctx, tdb, repo)
	grantID := firstActiveGrant(t, ctx, tdb, a)

	err := repo.RevokeGuarded(ctx, grantID)
	require.Error(t, err, "revoking from the last full-access admin must be rejected")
	require.True(t, errors.Is(err, invariant.ErrLastFullAccessAdmin),
		"expected ErrLastFullAccessAdmin, got %v", err)

	var revokedAt *time.Time
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT revoked_at FROM user_capabilities WHERE id = $1`, grantID).Scan(&revokedAt))
	require.Nil(t, revokedAt, "revocation must be rolled back")
}

// TestInvariant_RevocationAllowedWhenAnotherFullAccessAdminRemains proves the
// guard is not a blanket ban.
func TestInvariant_RevocationAllowedWhenAnotherFullAccessAdminRemains(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo, _ := newRepo(t, tdb)
	a := insertAdmin(t, ctx, tdb, repo)
	_ = insertAdmin(t, ctx, tdb, repo)

	require.NoError(t, repo.RevokeGuarded(ctx, firstActiveGrant(t, ctx, tdb, a)),
		"reduction is allowed while another full-access admin remains")
}

// TestInvariant_CountRequiresCoverageNotJustAdminship proves the invariant is
// coverage based: an admin row without the full universe does not satisfy it.
func TestInvariant_CountRequiresCoverageNotJustAdminship(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo, _ := newRepo(t, tdb)
	full := insertAdmin(t, ctx, tdb, repo)
	_ = insertPlainAdmin(t, ctx, tdb)

	// User set is one full-coverage admin.
	var count int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT count(*) FROM users WHERE role='admin'`).Scan(&count))
	require.Equal(t, 2, count, "two admin rows exist")

	var fullCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT count(*) FROM users u
		WHERE u.role='admin' AND u.deleted_at IS NULL AND u.account_status='active'
		  AND NOT EXISTS (
		      SELECT 1 FROM unnest($1::text[]) AS required(cap)
		      WHERE NOT EXISTS (
		          SELECT 1 FROM user_capabilities uc
		          WHERE uc.user_id=u.id AND uc.capability=required.cap AND uc.revoked_at IS NULL
		      )
		  )
	`, capability.AllCapabilityStrings()).Scan(&fullCount))
	require.Equal(t, 1, fullCount, "only the coverage-complete admin is full access")

	// Removing the only full-access admin's coverage is still refused.
	require.Error(t, repo.RevokeGuarded(ctx, firstActiveGrant(t, ctx, tdb, full)),
		"a bare admin row must not satisfy the invariant")
}

// TestInvariant_SupportsRoleIsTheOnlyAdminAuthority proves there is no hidden
// identity bypass: a non-admin user passes IsAdmin only by having the role.
func TestInvariant_AdminMembershipHasNoIdentityBypass(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	_, database := newRepo(t, tdb)
	rc := auth.NewRoleCheckerDB(database, noopAuditLogger{})

	// The reserved system-caller UUID is not a users row (migration 000086),
	// and even if it were, it must not confer admin membership.
	isAdmin, err := rc.IsAdmin(ctx, auth.SystemCallerID)
	require.Error(t, err, "no user row may occupy the reserved system-caller UUID")
	require.False(t, isAdmin)

	plain := uuid.New()
	_, execErr := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', 'user', NOW(), NOW())
	`, plain, plain.String(), plain.String()+"@test.local")
	require.NoError(t, execErr)

	isAdmin, err = rc.IsAdmin(ctx, plain)
	require.NoError(t, err)
	require.False(t, isAdmin, "role=user must never be admin")
}

// adminCaller is a distinct actor id (never the target, so the self-escalation
// guard is not what is being exercised).
func adminCaller() uuid.UUID {
	return uuid.MustParse("99999999-9999-9999-9999-999999999999")
}
