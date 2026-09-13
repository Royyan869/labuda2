//go:build integration

package worker

// MODERATION ACCOUNT-STATUS INVARIANT PROOF
//
// The moderation enforcement handler can suspend an account
// (moderation.user.suspended -> handleUserAction -> userRepo.Update). That is a
// full-access-reducing mutation, so it must be serialized against the canonical
// full-access admin invariant exactly like role changes and capability
// revocations.
//
// These tests drive the real Handle() entry point (and therefore the real
// transaction boundary and real identity/user repository) against real
// PostgreSQL. A mock cannot prove the advisory-lock + count-query guard.

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	userInfraRepo "github.com/labuda/backend/internal/identity/user/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	capabilityRepoImpl "github.com/labuda/backend/internal/platform/capability/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/capability/invariant"
	platformevent "github.com/labuda/backend/internal/platform/event"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// insertModerationInvariantUser inserts an active user with the given role.
// When fullAccess is true the user is granted the entire canonical capability
// universe through the canonical grant path, making them a full-access admin.
func insertModerationInvariantUser(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	appDB *db.DB,
	role string,
	fullAccess bool,
) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), 'active', $4, NOW(), NOW())
	`, id, id.String(), id.String()+"@moderation-invariant.test", role)
	require.NoError(t, err, "insert user")

	if !fullAccess {
		return id
	}

	repo := capabilityRepoImpl.NewCapabilityRepository(appDB)
	for _, c := range capability.AllCapabilityStrings() {
		require.NoError(t, repo.CreateGrant(ctx, capabilityEntity.NewCapabilityGrant(id, c, nil)),
			"grant %s", c)
	}
	return id
}

func newModerationEnforcementHandler(t *testing.T, appDB *db.DB) *ModerationEventHandler {
	t.Helper()
	userRepo := userInfraRepo.NewUserRepository(appDB)
	return NewModerationEventHandler(appDB, nil, nil, nil, nil, userRepo, nil, zaptest.NewLogger(t))
}

// moderationUserSuspendEvent builds the canonical moderation.user.suspended
// event. No enforcement_id, so the enforcement lifecycle rows are skipped and
// the account mutation is exercised directly.
func moderationUserSuspendEvent(userID uuid.UUID) platformevent.OutboxEvent {
	payload, _ := json.Marshal(moderationRemovedPayload{
		CaseID:       uuid.New().String(),
		ResourceType: "user",
		ResourceID:   userID.String(),
	})
	return platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "moderation.user.suspended",
		Payload:   payload,
	}
}

func moderationAccountStatus(t *testing.T, ctx context.Context, tdb *testdb.TestDB, id uuid.UUID) string {
	t.Helper()
	var status string
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT account_status FROM users WHERE id = $1`, id).Scan(&status))
	return status
}

// TestModerationSuspendInvariant_LastFullAccessAdmin_Rejected proves Case A: the
// sole active full-access admin cannot be suspended by moderation enforcement,
// the canonical invariant error surfaces, and the mutation is rolled back.
func TestModerationSuspendInvariant_LastFullAccessAdmin_Rejected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	target := insertModerationInvariantUser(t, ctx, tdb, appDB, capabilityEntity.AdminRole, true)
	handler := newModerationEnforcementHandler(t, appDB)

	err := handler.Handle(ctx, moderationUserSuspendEvent(target))
	require.Error(t, err, "sole full-access admin must not be suspended by moderation")
	require.True(t, errors.Is(err, invariant.ErrLastFullAccessAdmin),
		"expected ErrLastFullAccessAdmin, got %v", err)

	require.Equal(t, "active", moderationAccountStatus(t, ctx, tdb, target),
		"account_status must be unchanged after a refused moderation suspension")
}

// TestModerationSuspendInvariant_AllowedWhenAnotherFullAccessAdminRemains proves
// Case B: the guard is not a blanket ban — moderation may suspend one of two
// active full-access admins.
func TestModerationSuspendInvariant_AllowedWhenAnotherFullAccessAdminRemains(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	target := insertModerationInvariantUser(t, ctx, tdb, appDB, capabilityEntity.AdminRole, true)
	_ = insertModerationInvariantUser(t, ctx, tdb, appDB, capabilityEntity.AdminRole, true) // second full-access admin
	handler := newModerationEnforcementHandler(t, appDB)

	require.NoError(t, handler.Handle(ctx, moderationUserSuspendEvent(target)),
		"moderation suspension is allowed while another full-access admin remains")
	require.Equal(t, "suspended", moderationAccountStatus(t, ctx, tdb, target))
}

// TestModerationSuspendInvariant_NonFullAccessTarget_Unaffected proves Case C:
// moderation enforcement behavior is unchanged for ordinary users and for
// admins that do not cover the whole capability universe. A full-access admin is
// present so the invariant stays satisfiable.
func TestModerationSuspendInvariant_NonFullAccessTarget_Unaffected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	appDB := db.NewFromPool(tdb.Pool())

	_ = insertModerationInvariantUser(t, ctx, tdb, appDB, capabilityEntity.AdminRole, true) // keeps >=1 active full-access admin
	handler := newModerationEnforcementHandler(t, appDB)

	targets := []struct {
		name string
		role string
	}{
		{"ordinary_user", "user"},
		{"partial_capability_admin", capabilityEntity.AdminRole},
	}

	for _, tc := range targets {
		t.Run(tc.name, func(t *testing.T) {
			id := insertModerationInvariantUser(t, ctx, tdb, appDB, tc.role, false)
			require.NoError(t, handler.Handle(ctx, moderationUserSuspendEvent(id)))
			require.Equal(t, "suspended", moderationAccountStatus(t, ctx, tdb, id))
		})
	}
}
