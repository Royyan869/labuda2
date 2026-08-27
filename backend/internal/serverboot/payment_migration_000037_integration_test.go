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

func TestPaymentMigration000037_DropsNetAmount(t *testing.T) {
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
		return tx.QueryRow(ctx, `SELECT name FROM schema_migrations WHERE version = 37`).Scan(&migrationName)
	})
	require.NoError(t, err)
	require.True(t, migrationName.Valid, "migration 37 must have a recorded name")
	require.Equal(t, "payment_net_amount_drop", migrationName.String)

	var hasNetAmount bool
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = 'payments'
				  AND column_name = 'net_amount'
			)
		`).Scan(&hasNetAmount)
	})
	require.NoError(t, err)
	require.False(t, hasNetAmount, "payments.net_amount must not remain on the live schema")
}
