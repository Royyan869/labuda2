//go:build integration

package serverboot

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestPaymentMigration000036_Applied(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	var version int64
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	})
	require.NoError(t, err)
	require.Equal(t, int64(38), version, "schema_migrations should reach version 38")

	var migrationName sql.NullString
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT name FROM schema_migrations WHERE version = 36`).Scan(&migrationName)
	})
	require.NoError(t, err)
	require.True(t, migrationName.Valid, "migration 36 must have a recorded name")
	require.Equal(t, "payment_coins_to_use_rename", migrationName.String)

	var hasNewColumn bool
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = 'payments'
				  AND column_name = 'coins_to_use'
			)
		`).Scan(&hasNewColumn)
	})
	require.NoError(t, err)
	require.True(t, hasNewColumn, "payments.coins_to_use column must exist")

	var hasOldColumn bool
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = 'payments'
				  AND column_name = 'coin_discount'
			)
		`).Scan(&hasOldColumn)
	})
	require.NoError(t, err)
	require.False(t, hasOldColumn, "payments.coin_discount must not remain on the live schema")
}
