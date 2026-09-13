package invariant_test

// ACCOUNT-STATUS LIFECYCLE INVARIANT PROOF
//
// Full access is derived: users.role == 'admin' AND the active capability set
// covers the entire canonical universe. The invariant under test is therefore
// about account_status, not about roles or capabilities:
//
//	At least one ACTIVE full-access admin must always remain.
//
// These tests drive the canonical admin lifecycle authority
// (AdminService.SuspendUser / AdminService.BanUser) against real PostgreSQL,
// because the guard is a transaction-scoped advisory lock plus a count query —
// a mock cannot prove it.

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	adminApp "github.com/labuda/backend/internal/platform/admin/application"
	adminInfra "github.com/labuda/backend/internal/platform/admin/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/labuda/backend/internal/platform/capability/invariant"
	"github.com/labuda/backend/pkg/testdb"
)

// governanceActorContext injects an admin actor holding exactly one capability,
// mirroring how the admin endpoints authorize the lifecycle mutation.
func governanceActorContext(cap string) context.Context {
	actor := &capabilityEntity.Actor{
		ID:           adminCaller(),
		Role:         capabilityEntity.AdminRole,
		Capabilities: []string{cap},
	}
	return capability.WithActor(context.Background(), actor)
}

func accountStatus(t *testing.T, ctx context.Context, tdb *testdb.TestDB, id uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT account_status FROM users WHERE id = $1`, id).Scan(&status))
	return status
}

func insertOrdinaryUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', 'user', NOW(), NOW())
	`, id, id.String(), id.String()+"@test.local")
	require.NoError(t, err)
	return id
}

type lifecycleOp struct {
	name       string
	cap        string
	wantStatus string
	run        func(svc *adminApp.AdminService, ctx context.Context, target uuid.UUID) error
}

func lifecycleOps() []lifecycleOp {
	return []lifecycleOp{
		{
			name:       "suspend",
			cap:        capability.CapGovernanceUserSuspend.String(),
			wantStatus: "suspended",
			run: func(svc *adminApp.AdminService, ctx context.Context, target uuid.UUID) error {
				return svc.SuspendUser(ctx, adminCaller(), target, adminApp.SuspendUserRequest{Reason: "test"})
			},
		},
		{
			name:       "ban",
			cap:        capability.CapGovernanceUserBan.String(),
			wantStatus: "banned",
			run: func(svc *adminApp.AdminService, ctx context.Context, target uuid.UUID) error {
				return svc.BanUser(ctx, adminCaller(), target, adminApp.BanUserRequest{Reason: "test"})
			},
		},
	}
}

// TestAccountStatusInvariant_LastFullAccessAdmin_SuspendAndBanRejected proves
// Case A: the sole active full-access admin cannot be suspended or banned, the
// canonical invariant error is returned, and the mutation is rolled back.
func TestAccountStatusInvariant_LastFullAccessAdmin_SuspendAndBanRejected(t *testing.T) {
	for _, op := range lifecycleOps() {
		t.Run(op.name, func(t *testing.T) {
			tdb, cleanup := testdb.SetupDB(t)
			defer cleanup()
			ctx := context.Background()

			repo, database := newRepo(t, tdb)
			target := insertAdmin(t, ctx, tdb, repo)

			svc := adminApp.NewAdminService(database, adminInfra.NewAdminRepository(), noopAuditLogger{}, nil)

			err := op.run(svc, governanceActorContext(op.cap), target)
			require.Error(t, err, "sole full-access admin must not be %sd", op.name)
			require.True(t, errors.Is(err, invariant.ErrLastFullAccessAdmin),
				"expected ErrLastFullAccessAdmin, got %v", err)

			require.Equal(t, "active", accountStatus(t, ctx, tdb, target),
				"account_status must be unchanged after a refused %s", op.name)
		})
	}
}

// TestAccountStatusInvariant_AllowedWhenAnotherFullAccessAdminRemains proves
// Case B: the guard is not a blanket ban — reducing one full-access admin is
// allowed while another active full-access admin remains.
func TestAccountStatusInvariant_AllowedWhenAnotherFullAccessAdminRemains(t *testing.T) {
	for _, op := range lifecycleOps() {
		t.Run(op.name, func(t *testing.T) {
			tdb, cleanup := testdb.SetupDB(t)
			defer cleanup()
			ctx := context.Background()

			repo, database := newRepo(t, tdb)
			target := insertAdmin(t, ctx, tdb, repo)
			_ = insertAdmin(t, ctx, tdb, repo) // second active full-access admin

			svc := adminApp.NewAdminService(database, adminInfra.NewAdminRepository(), noopAuditLogger{}, nil)

			require.NoError(t, op.run(svc, governanceActorContext(op.cap), target),
				"reduction is allowed while another full-access admin remains")
			require.Equal(t, op.wantStatus, accountStatus(t, ctx, tdb, target))
		})
	}
}

// TestAccountStatusInvariant_NonFullAccessTarget_Unaffected proves Case C:
// lifecycle enforcement is unchanged for ordinary users and for admins that do
// not cover the whole capability universe. A full-access admin is present so
// the invariant is satisfiable.
func TestAccountStatusInvariant_NonFullAccessTarget_Unaffected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo, database := newRepo(t, tdb)
	_ = insertAdmin(t, ctx, tdb, repo) // keeps >=1 active full-access admin

	svc := adminApp.NewAdminService(database, adminInfra.NewAdminRepository(), noopAuditLogger{}, nil)

	targets := []struct {
		name string
		mk   func() uuid.UUID
	}{
		{"ordinary_user", func() uuid.UUID { return insertOrdinaryUser(t, ctx, tdb) }},
		{"partial_capability_admin", func() uuid.UUID { return insertPlainAdmin(t, ctx, tdb) }},
	}

	for _, op := range lifecycleOps() {
		for _, target := range targets {
			t.Run(op.name+"_"+target.name, func(t *testing.T) {
				id := target.mk()
				require.NoError(t, op.run(svc, governanceActorContext(op.cap), id))
				require.Equal(t, op.wantStatus, accountStatus(t, ctx, tdb, id))
			})
		}
	}
}
