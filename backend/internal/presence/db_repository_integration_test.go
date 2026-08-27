package presence

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

func setupPresenceDBTest(t *testing.T) (*testdb.TestDB, func()) {
	t.Helper()
	tdb, _ := testdb.SetupDB(t)
	return tdb, func() { tdb.Pool().Close() }
}

func readPresenceLastSeen(t *testing.T, tdb *testdb.TestDB, userID uuid.UUID) *time.Time {
	t.Helper()

	ctx := context.Background()
	var seen sql.NullTime
	err := tdb.Pool().QueryRow(ctx, `
		SELECT last_seen_at
		FROM user_presence
		WHERE user_id = $1
	`, userID).Scan(&seen)
	require.NoError(t, err)
	if !seen.Valid {
		return nil
	}
	v := seen.Time.UTC()
	return &v
}

func TestDBRepository_UpsertLastSeen_CreatesRowAndReturnsValue(t *testing.T) {
	tdb, cleanup := setupPresenceDBTest(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewDBRepository(nil)
	userID := seedPresenceUserCommitted(t, tdb, "presence-create")
	occurredAt := time.Date(2026, time.July, 29, 11, 0, 0, 0, time.UTC)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.UpsertLastSeen(ctx, tx, userID, occurredAt)
	})
	require.NoError(t, err)

	seen := readPresenceLastSeen(t, tdb, userID)
	require.NotNil(t, seen)
	require.True(t, occurredAt.Equal(*seen))
}

func TestDBRepository_UpsertLastSeen_NewerReplacesOlder(t *testing.T) {
	tdb, cleanup := setupPresenceDBTest(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewDBRepository(nil)
	userID := seedPresenceUserCommitted(t, tdb, "presence-newer")
	older := time.Date(2026, time.July, 29, 9, 0, 0, 0, time.UTC)
	newer := time.Date(2026, time.July, 29, 10, 0, 0, 0, time.UTC)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		require.NoError(t, repo.UpsertLastSeen(ctx, tx, userID, older))
		return repo.UpsertLastSeen(ctx, tx, userID, newer)
	})
	require.NoError(t, err)

	seen := readPresenceLastSeen(t, tdb, userID)
	require.NotNil(t, seen)
	require.True(t, newer.Equal(*seen))
}

func TestDBRepository_UpsertLastSeen_OlderIsIgnored(t *testing.T) {
	tdb, cleanup := setupPresenceDBTest(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewDBRepository(nil)
	userID := seedPresenceUserCommitted(t, tdb, "presence-older")
	older := time.Date(2026, time.July, 29, 9, 0, 0, 0, time.UTC)
	newer := time.Date(2026, time.July, 29, 10, 0, 0, 0, time.UTC)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		require.NoError(t, repo.UpsertLastSeen(ctx, tx, userID, newer))
		return repo.UpsertLastSeen(ctx, tx, userID, older)
	})
	require.NoError(t, err)

	seen := readPresenceLastSeen(t, tdb, userID)
	require.NotNil(t, seen)
	require.True(t, newer.Equal(*seen))
}

func TestDBRepository_UpsertLastSeen_EqualTimestampIsIdempotent(t *testing.T) {
	tdb, cleanup := setupPresenceDBTest(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewDBRepository(nil)
	userID := seedPresenceUserCommitted(t, tdb, "presence-idempotent")
	occurredAt := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		require.NoError(t, repo.UpsertLastSeen(ctx, tx, userID, occurredAt))
		return repo.UpsertLastSeen(ctx, tx, userID, occurredAt)
	})
	require.NoError(t, err)

	seen := readPresenceLastSeen(t, tdb, userID)
	require.NotNil(t, seen)
	require.True(t, occurredAt.Equal(*seen))
}

func TestDBRepository_GetLastSeenBatch_ReturnsMissingAsNil(t *testing.T) {
	tdb, cleanup := setupPresenceDBTest(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewDBRepository(nil)
	presentID := uuid.New()
	missingID := uuid.New()
	occurredAt := time.Date(2026, time.July, 29, 13, 0, 0, 0, time.UTC)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (
				id, firebase_uid, email, email_verified_at, phone_verified,
				account_status, created_at, updated_at
			)
			VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW())
		`, presentID, presentID.String(), "presence-batch-"+presentID.String()+"@test.invalid")
		require.NoError(t, err)
		return repo.UpsertLastSeen(ctx, tx, presentID, occurredAt)
	})
	require.NoError(t, err)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		results, err := repo.GetLastSeenBatch(ctx, tx, []uuid.UUID{presentID, missingID})
		require.NoError(t, err)
		require.Len(t, results, 2)
		require.Contains(t, results, presentID)
		require.Contains(t, results, missingID)
		require.NotNil(t, results[presentID])
		require.True(t, occurredAt.Equal(*results[presentID]))
		require.Nil(t, results[missingID])
		return nil
	})
	require.NoError(t, err)
}

func TestDBRepository_DeleteUser_CascadesPresenceRow(t *testing.T) {
	tdb, cleanup := setupPresenceDBTest(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewDBRepository(nil)
	userID := uuid.New()
	occurredAt := time.Date(2026, time.July, 29, 14, 0, 0, 0, time.UTC)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (
				id, firebase_uid, email, email_verified_at, phone_verified,
				account_status, created_at, updated_at
			)
			VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW())
		`, userID, userID.String(), "presence-delete-"+userID.String()+"@test.invalid")
		require.NoError(t, err)
		return repo.UpsertLastSeen(ctx, tx, userID, occurredAt)
	})
	require.NoError(t, err)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
		require.NoError(t, err)
		var count int
		require.NoError(t, tx.QueryRow(ctx, `SELECT COUNT(*) FROM user_presence WHERE user_id = $1`, userID).Scan(&count))
		require.Equal(t, 0, count)
		return nil
	})
	require.NoError(t, err)
}

func TestDBRepository_GetLastSeen_MissingReturnsNil(t *testing.T) {
	tdb, cleanup := setupPresenceDBTest(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewDBRepository(nil)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		userID := uuid.New()
		lastSeen, err := repo.GetLastSeen(ctx, tx, userID)
		require.NoError(t, err)
		require.Nil(t, lastSeen)
		return nil
	})
	require.NoError(t, err)
}
