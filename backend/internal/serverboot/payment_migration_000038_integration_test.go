//go:build integration

package serverboot

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func TestPaymentMigration000038_PaymentSpendReferenceType(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	var version int64
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, version, int64(38), "schema_migrations should reach version 38")

	var migrationName sql.NullString
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT name FROM schema_migrations WHERE version = 38`).Scan(&migrationName)
	})
	require.NoError(t, err)
	require.True(t, migrationName.Valid, "migration 38 must have a recorded name")
	require.Equal(t, "payment_coin_spend_reference_type", migrationName.String)

	userID := uuid.New()
	paymentID := uuid.New()

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, role)
			VALUES ($1, $2, $3, 'user')
			ON CONFLICT (id) DO NOTHING
		`, userID, "fb-"+userID.String()[:8], userID.String()+"@test.local")
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO coins_transactions (
				id, user_id, type, amount, reference_type, reference_id, created_at
			) VALUES ($1, $2, 'spend', $3, 'payment_spend', $4, NOW())
		`, uuid.New(), userID, 1234, paymentID)
		return err
	})
	require.NoError(t, err, "payment_spend must be accepted by the live coins_transactions reference_type check")
}
