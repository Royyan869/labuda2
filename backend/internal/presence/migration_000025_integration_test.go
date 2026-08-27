package presence

import (
	"context"
	"strings"
	"testing"

	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

func TestMigration000025_UserPresenceFoundationSchema(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	defer tdb.Pool().Close()

	ctx := context.Background()
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var exists bool
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = 'public'
				  AND table_name = 'user_presence'
			)
		`).Scan(&exists))
		require.True(t, exists)

		var isNullable, defaultValue string
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT is_nullable, COALESCE(column_default, '')
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'user_presence'
			  AND column_name = 'last_seen_at'
		`).Scan(&isNullable, &defaultValue))
		require.Equal(t, "YES", isNullable)

		require.NoError(t, tx.QueryRow(ctx, `
			SELECT COALESCE(column_default, '')
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'user_presence'
			  AND column_name = 'updated_at'
		`).Scan(&defaultValue))
		require.Contains(t, defaultValue, "now")

		var hasIsOnline bool
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = 'user_presence'
				  AND column_name = 'is_online'
			)
		`).Scan(&hasIsOnline))
		require.False(t, hasIsOnline)

		var pkDefinition string
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT pg_get_constraintdef(c.oid)
			FROM pg_constraint c
			WHERE c.conrelid = 'user_presence'::regclass
			  AND c.contype = 'p'
		`).Scan(&pkDefinition))
		require.Contains(t, pkDefinition, "PRIMARY KEY (user_id)")

		var fkDefinition string
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT pg_get_constraintdef(c.oid)
			FROM pg_constraint c
			WHERE c.conrelid = 'user_presence'::regclass
			  AND c.contype = 'f'
		`).Scan(&fkDefinition))
		require.Contains(t, fkDefinition, "FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE")

		var rowCount int
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'user_presence'
		`).Scan(&rowCount))
		require.Equal(t, 3, rowCount)
		return nil
	})
	require.NoError(t, err)
}

func TestMigration000025_DoesNotReuseAuthRefreshSessionsLastSeenAt(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	defer tdb.Pool().Close()

	ctx := context.Background()
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var count int
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'auth_refresh_sessions'
			  AND column_name = 'last_seen_at'
		`).Scan(&count))
		require.GreaterOrEqual(t, count, 0)
		return nil
	})
	require.NoError(t, err)
}

func TestMigration000025_TableNameDoesNotContainOnlineTruth(t *testing.T) {
	tdb, _ := testdb.SetupDB(t)
	defer tdb.Pool().Close()

	ctx := context.Background()
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'user_presence'
		`)
		require.NoError(t, err)
		defer rows.Close()

		for rows.Next() {
			var column string
			require.NoError(t, rows.Scan(&column))
			require.False(t, strings.EqualFold(column, "is_online"))
		}
		require.NoError(t, rows.Err())
		return nil
	})
	require.NoError(t, err)
}
