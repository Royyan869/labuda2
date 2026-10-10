//go:build integration

package repository_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	notificationentity "github.com/hishumi/backend/internal/interaction/notification/entity"
	notificationrepository "github.com/hishumi/backend/internal/interaction/notification/infrastructure/repository"
	dbpkg "github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/migration"
	"github.com/hishumi/backend/pkg/testdb"
)

// systemCallerID mirrors audit.SystemCallerID: the reserved identity that
// migrations 000086 and 000100 forbid as a persisted human actor. 000114 must
// forbid it here too.
const systemCallerID = "00000000-0000-0000-0000-000000000001"

// actorAuthorityFixture is one migrated test database plus two real users
// (actor + recipient) so the users FK is exercised for real.
type actorAuthorityFixture struct {
	appDB     *dbpkg.DB
	actorID   uuid.UUID
	recipient uuid.UUID
}

func setupActorAuthorityFixture(t *testing.T) actorAuthorityFixture {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	actorID, recipientID := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{actorID, recipientID} {
		_, err := tdb.Pool().Exec(ctx,
			`INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, created_at, updated_at)
			 VALUES ($1, $2, $3, NOW(), 'active', NOW(), NOW())`,
			id, "fb-"+id.String(), id.String()+"@test.invalid",
		)
		require.NoError(t, err, "seed user %s", id)
	}

	return actorAuthorityFixture{
		appDB:     dbpkg.NewFromPool(tdb.Pool()),
		actorID:   actorID,
		recipient: recipientID,
	}
}

func insertNotification(t *testing.T, f actorAuthorityFixture, actor notificationentity.Actor, notifyType string, entityID uuid.UUID) (uuid.UUID, bool) {
	t.Helper()

	var (
		id       uuid.UUID
		inserted bool
	)
	err := f.appDB.WithTx(context.Background(), func(tx dbpkg.Tx) error {
		var err error
		id, inserted, err = notificationrepository.NewNotificationRepository().Insert(
			context.Background(), tx,
			notificationentity.NewNotification(f.recipient, actor, notificationentity.NotificationType(notifyType), entityID, map[string]interface{}{}),
		)
		return err
	})
	require.NoError(t, err, "insert %s", notifyType)
	return id, inserted
}

func getNotification(t *testing.T, f actorAuthorityFixture, id uuid.UUID) *notificationentity.Notification {
	t.Helper()

	var got *notificationentity.Notification
	err := f.appDB.WithTx(context.Background(), func(tx dbpkg.Tx) error {
		var err error
		got, err = notificationrepository.NewNotificationRepository().GetByID(context.Background(), tx, id)
		return err
	})
	require.NoError(t, err, "get notification %s", id)
	return got
}

// TestActorAuthority_ThreeStatesRoundTrip proves the canonical model survives a
// real write→read cycle: the three business truths are stored as three states,
// not as one sentinel that the users FK rejects.
func TestActorAuthority_ThreeStatesRoundTrip(t *testing.T) {
	f := setupActorAuthorityFixture(t)

	userID, inserted := insertNotification(t, f, notificationentity.UserActor(f.actorID), "user.followed", uuid.New())
	require.True(t, inserted, "user actor must persist")
	user := getNotification(t, f, userID)
	require.True(t, user.Actor.IsUser(), "actor kind = %q, want user", user.Actor.Kind())
	require.Equal(t, f.actorID, user.Actor.UserID())

	systemID, inserted := insertNotification(t, f, notificationentity.SystemActor(), "withdrawal.completed", uuid.New())
	require.True(t, inserted, "system actor must persist (this is the row the old schema could not write)")
	system := getNotification(t, f, systemID)
	require.Equal(t, notificationentity.ActorKindSystem, system.Actor.Kind())
	require.Equal(t, uuid.Nil, system.Actor.UserID())

	anonymizedID, inserted := insertNotification(t, f, notificationentity.AnonymizedActor("Penjual"), "order.shipped", uuid.New())
	require.True(t, inserted, "anonymized actor must persist")
	anonymized := getNotification(t, f, anonymizedID)
	require.Equal(t, notificationentity.ActorKindAnonymized, anonymized.Actor.Kind())
	require.Equal(t, uuid.Nil, anonymized.Actor.UserID())
	require.Equal(t, "Penjual", anonymized.Actor.Display())
}

// TestActorAuthority_DedupSurvivesNullActor proves the unique key kept its
// idempotency meaning after actor_id became nullable: the generated actor_key
// gives system/anonymized notifications one stable key instead of PostgreSQL's
// "NULLs are distinct" trap.
func TestActorAuthority_DedupSurvivesNullActor(t *testing.T) {
	f := setupActorAuthorityFixture(t)

	for _, tc := range []struct {
		name  string
		actor notificationentity.Actor
	}{
		{"user", notificationentity.UserActor(f.actorID)},
		{"system", notificationentity.SystemActor()},
		{"anonymized", notificationentity.AnonymizedActor("Penjual")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entityID := uuid.New()
			firstID, inserted := insertNotification(t, f, tc.actor, "negotiation.expired", entityID)
			require.True(t, inserted, "first insert must write a row")
			require.NotEqual(t, uuid.Nil, firstID)

			replayID, inserted := insertNotification(t, f, tc.actor, "negotiation.expired", entityID)
			require.False(t, inserted, "replay must be a dedup no-op")
			require.Equal(t, uuid.Nil, replayID, "dedup no-op returns uuid.Nil")
		})
	}
}

// TestActorAuthority_DeletedActorDemotedToSystem proves deleting a user keeps
// the recipient's history: the trigger demotes the row to a system actor before
// the FK's SET NULL, instead of cascading the notification away.
func TestActorAuthority_DeletedActorDemotedToSystem(t *testing.T) {
	f := setupActorAuthorityFixture(t)

	id, inserted := insertNotification(t, f, notificationentity.UserActor(f.actorID), "order.created", uuid.New())
	require.True(t, inserted)

	_, err := f.appDB.Pool().Exec(context.Background(), "DELETE FROM users WHERE id = $1", f.actorID)
	require.NoError(t, err, "delete actor")

	var kind string
	var actorID *uuid.UUID
	var display string
	err = f.appDB.Pool().QueryRow(context.Background(),
		"SELECT actor_kind::text, actor_id, actor_display FROM notifications WHERE id = $1", id,
	).Scan(&kind, &actorID, &display)
	require.NoError(t, err, "notification must survive its actor's deletion")
	require.Equal(t, string(notificationentity.ActorKindSystem), kind)
	require.Nil(t, actorID)
	require.Equal(t, "", display)
}

// TestActorAuthority_SchemaRejectsInexpressibleRows proves the database itself
// refuses the shapes the old model allowed or the sentinel produced: no
// "user without id", no "system with id", and never the reserved system caller
// as a persisted human identity.
func TestActorAuthority_SchemaRejectsInexpressibleRows(t *testing.T) {
	f := setupActorAuthorityFixture(t)
	ctx := context.Background()

	const insertSQL = `INSERT INTO notifications
		(id, recipient_id, actor_id, type, entity_id, data, is_read, created_at, actor_kind, actor_display)
		VALUES ($1, $2, $3, 'test.type', $4, '{}'::jsonb, false, NOW(), $5, '')`

	cases := []struct {
		name        string
		actorID     any
		kind        string
		wantErrPart string
	}{
		{"user kind without id", nil, "user", "notifications_actor_kind_shape"},
		{"system kind with id", f.actorID, "system", "notifications_actor_kind_shape"},
		{"anonymized kind with id", f.actorID, "anonymized", "notifications_actor_kind_shape"},
		{"reserved system caller as user", systemCallerID, "user", "notifications_actor_not_system_caller"},
		{"unknown actor kind", nil, "robot", "invalid input value for enum"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.appDB.Pool().Exec(ctx, insertSQL, uuid.New(), f.recipient, tc.actorID, uuid.New(), tc.kind)
			require.Error(t, err, "insert must be rejected")
			require.Contains(t, err.Error(), tc.wantErrPart)
		})
	}
}

// TestActorAuthority_NoRowsForDeletedSystemCallerUser guards the assumption the
// CHECK constraint encodes: the reserved system caller has no users row, so it
// can never satisfy the FK either.
func TestActorAuthority_NoRowsForDeletedSystemCallerUser(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	var exists bool
	err := tdb.Pool().QueryRow(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)", systemCallerID,
	).Scan(&exists)
	require.NoError(t, err)
	require.False(t, exists, "reserved system caller must not exist as a user (see 000086)")
}

// TestActorAuthority_DownUpReplay proves the reverse migration is real and the
// forward migration is repeatable: applying 000114 down then up inside one
// transaction succeeds and leaves the canonical shape in place. The transaction
// is rolled back, so the suite's schema is untouched.
func TestActorAuthority_DownUpReplay(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	dir, err := migration.ResolveDir(".")
	require.NoError(t, err, "resolve migration chain")
	ctx := context.Background()

	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	for _, name := range []string{
		"000114_notification_actor_authority.down.sql",
		"000114_notification_actor_authority.up.sql",
	} {
		raw, readErr := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, readErr, "read %s", name)
		for _, stmt := range migration.Split(string(raw)) {
			_, execErr := tx.Exec(ctx, stmt)
			require.NoError(t, execErr, "%s: %s", name, stmt)
		}
	}

	var actorKind string
	err = tx.QueryRow(ctx, `
		SELECT data_type FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'notifications' AND column_name = 'actor_kind'`).Scan(&actorKind)
	require.NoError(t, err, "actor_kind must exist after a down/up replay")
	require.Equal(t, "USER-DEFINED", actorKind)
}


