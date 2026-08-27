//go:build integration

package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	bankaccountrepo "github.com/labuda/backend/internal/finance/bankaccount/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupBankAccountIntegrationService(t *testing.T) (*testdb.TestDB, *BankAccountService, func()) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	svc := &BankAccountService{
		repo: bankaccountrepo.NewBankAccountRepository(),
		log:  zap.NewNop(),
	}
	return tdb, svc, cleanup
}

func seedBankAccountOwner(t *testing.T, ctx context.Context, tx db.Tx, userID uuid.UUID) {
	t.Helper()

	_, err := tx.Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, account_status, created_at, updated_at, role
		)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
	`, userID, userID.String(), userID.String()+"@example.invalid")
	require.NoError(t, err)
}

func seedBankAccountRow(
	t *testing.T,
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	userID uuid.UUID,
	bankName string,
	bankCode string,
	accountNumber string,
	accountHolder string,
	isDefault bool,
	createdAt time.Time,
) {
	t.Helper()

	_, err := tx.Exec(ctx, `
		INSERT INTO bank_accounts (
			id, user_id, bank_name, bank_code, account_number, account_holder_name,
			is_default, status, deleted_at, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, 'active', NULL, $8, $8
		)
	`, id, userID, bankName, bankCode, accountNumber, accountHolder, isDefault, createdAt)
	require.NoError(t, err)
}

func seedActiveWithdrawalRow(
	t *testing.T,
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	status string,
) uuid.UUID {
	t.Helper()

	withdrawalID := uuid.New()
	now := time.Now().UTC()
	_, err := tx.Exec(ctx, `
		INSERT INTO withdrawals (
			id, amount, status, created_at, updated_at, seller_id, idempotency_key,
			bank_name_snapshot, bank_code_snapshot, account_number_snapshot, account_holder_snapshot,
			external_reference_id, gateway_response, failure_reason, submitted_at, settled_at, retry_count, fee_amount
		)
		VALUES (
			$1, 100000, $2, $3, $3, $4, $5,
			'BCA', 'BCA', '1234567890', 'Test Seller',
			'', '', '', 0, 0, 0, 5000
		)
	`, withdrawalID, status, now, sellerID, "seed-"+withdrawalID.String())
	require.NoError(t, err)
	return withdrawalID
}

func countActiveDefaultBankAccounts(t *testing.T, ctx context.Context, tx db.Tx, userID uuid.UUID) int64 {
	t.Helper()

	var count int64
	require.NoError(t, tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM bank_accounts
		WHERE user_id = $1
		  AND deleted_at IS NULL
		  AND is_default = true
	`, userID).Scan(&count))
	return count
}

func countSellerBankAccounts(t *testing.T, ctx context.Context, tx db.Tx, userID uuid.UUID) int64 {
	t.Helper()

	var count int64
	require.NoError(t, tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM bank_accounts
		WHERE user_id = $1
		  AND deleted_at IS NULL
	`, userID).Scan(&count))
	return count
}

func fetchDefaultBankAccountID(t *testing.T, ctx context.Context, tx db.Tx, userID uuid.UUID) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	require.NoError(t, tx.QueryRow(ctx, `
		SELECT id
		FROM bank_accounts
		WHERE user_id = $1
		  AND deleted_at IS NULL
		  AND is_default = true
		LIMIT 1
	`, userID).Scan(&id))
	return id
}

func loadPrimaryInvariantProbeMigrationSQL(t *testing.T) string {
	t.Helper()

	path, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "migrations", "000028_bank_account_primary_invariant_hardening.up.sql"))
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	sql := string(data)
	sql = strings.ReplaceAll(sql, "public.bank_accounts", "bank_accounts_migration_probe")
	sql = strings.ReplaceAll(sql, "idx_bank_accounts_user_active_default_unique", "idx_bank_accounts_migration_probe_unique")
	return sql
}

func TestBankAccountPrimaryInvariant_FirstAccountBecomesDefaultAndSecondDoesNotCreateDoublePrimary(t *testing.T) {
	tdb, svc, cleanup := setupBankAccountIntegrationService(t)
	defer cleanup()

	ctx := context.Background()
	sellerID := uuid.New()

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		seedBankAccountOwner(t, ctx, tx, sellerID)

		first, err := svc.CreateBankAccount(ctx, tx, CreateBankAccountInput{
			UserID:            sellerID,
			BankName:          "Bank Central Asia",
			BankCode:          "BCA",
			AccountNumber:     "1234567890",
			AccountHolderName: "Seller One",
			IsDefault:         false,
		})
		require.NoError(t, err)
		require.True(t, first.IsDefault)

		second, err := svc.CreateBankAccount(ctx, tx, CreateBankAccountInput{
			UserID:            sellerID,
			BankName:          "Bank Mandiri",
			BankCode:          "MANDIRI",
			AccountNumber:     "9876543210",
			AccountHolderName: "Seller One",
			IsDefault:         false,
		})
		require.NoError(t, err)
		require.False(t, second.IsDefault)

		require.Equal(t, int64(2), countSellerBankAccounts(t, ctx, tx, sellerID))
		require.Equal(t, int64(1), countActiveDefaultBankAccounts(t, ctx, tx, sellerID))
		return nil
	})
	require.NoError(t, err)
}

func TestBankAccountPrimaryInvariant_ConcurrentSetDefaultLeavesAtMostOnePrimary(t *testing.T) {
	tdb, svc, cleanup := setupBankAccountIntegrationService(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sellerID := uuid.New()
	var firstID uuid.UUID
	var secondID uuid.UUID

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		seedBankAccountOwner(t, ctx, tx, sellerID)

		first, err := svc.CreateBankAccount(ctx, tx, CreateBankAccountInput{
			UserID:            sellerID,
			BankName:          "Bank Central Asia",
			BankCode:          "BCA",
			AccountNumber:     "1234567890",
			AccountHolderName: "Seller One",
			IsDefault:         false,
		})
		require.NoError(t, err)
		firstID = first.ID

		second, err := svc.CreateBankAccount(ctx, tx, CreateBankAccountInput{
			UserID:            sellerID,
			BankName:          "Bank Mandiri",
			BankCode:          "MANDIRI",
			AccountNumber:     "9876543210",
			AccountHolderName: "Seller One",
			IsDefault:         false,
		})
		require.NoError(t, err)
		secondID = second.ID
		return nil
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	startCh := make(chan struct{})
	errCh := make(chan error, 2)

	for _, accountID := range []uuid.UUID{firstID, secondID} {
		accountID := accountID
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			errCh <- tdb.WithTx(ctx, func(tx db.Tx) error {
				return svc.SetDefaultBankAccount(ctx, tx, accountID, sellerID)
			})
		}()
	}

	close(startCh)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	require.Equal(t, int64(2), countSellerBankAccounts(t, ctx, tx, sellerID))
	require.Equal(t, int64(1), countActiveDefaultBankAccounts(t, ctx, tx, sellerID))
	defaultID := fetchDefaultBankAccountID(t, ctx, tx, sellerID)
	require.True(t, defaultID == firstID || defaultID == secondID)
}

func TestBankAccountUpdateAndDelete_RejectOtherUserAndBlockActiveWithdrawals(t *testing.T) {
	tdb, svc, cleanup := setupBankAccountIntegrationService(t)
	defer cleanup()

	ctx := context.Background()
	sellerID := uuid.New()
	otherSellerID := uuid.New()
	accountID := uuid.New()

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		seedBankAccountOwner(t, ctx, tx, sellerID)
		seedBankAccountOwner(t, ctx, tx, otherSellerID)
		seedBankAccountRow(t, ctx, tx, accountID, sellerID, "Bank Central Asia", "BCA", "1234567890", "Seller One", true, time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC))
		return nil
	})
	require.NoError(t, err)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := svc.UpdateBankAccount(ctx, tx, UpdateBankAccountInput{
			UserID:            otherSellerID,
			BankAccountID:     accountID,
			BankName:          "Bank Mandiri",
			BankCode:          "MANDIRI",
			AccountNumber:     "9876543210",
			AccountHolderName: "Intruder",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not belong to seller")

		err = svc.DeleteBankAccount(ctx, tx, accountID, otherSellerID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not belong to seller")

		seedActiveWithdrawalRow(t, ctx, tx, sellerID, "REQUESTED")

		_, err = svc.UpdateBankAccount(ctx, tx, UpdateBankAccountInput{
			UserID:            sellerID,
			BankAccountID:     accountID,
			BankName:          "Bank Mandiri",
			BankCode:          "MANDIRI",
			AccountNumber:     "9876543210",
			AccountHolderName: "Seller One",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot update while 1 active withdrawal(s) exist")

		err = svc.DeleteBankAccount(ctx, tx, accountID, sellerID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot delete while 1 active withdrawal(s) exist")
		return nil
	})
	require.NoError(t, err)
}

func TestBankAccountUpdate_MutatesExistingRowWithoutCreatingNewOne(t *testing.T) {
	tdb, svc, cleanup := setupBankAccountIntegrationService(t)
	defer cleanup()

	ctx := context.Background()
	sellerID := uuid.New()
	accountID := uuid.New()

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		seedBankAccountOwner(t, ctx, tx, sellerID)
		seedBankAccountRow(t, ctx, tx, accountID, sellerID, "Bank Central Asia", "BCA", "1234567890", "Seller One", true, time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC))
		return nil
	})
	require.NoError(t, err)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		before := countSellerBankAccounts(t, ctx, tx, sellerID)
		require.Equal(t, int64(1), before)

		updated, err := svc.UpdateBankAccount(ctx, tx, UpdateBankAccountInput{
			UserID:            sellerID,
			BankAccountID:     accountID,
			BankName:          "Bank Mandiri",
			BankCode:          "MANDIRI",
			AccountNumber:     "9876543210",
			AccountHolderName: "Seller One Updated",
		})
		require.NoError(t, err)
		require.Equal(t, accountID, updated.ID)
		require.Equal(t, "Bank Mandiri", updated.BankName)
		require.Equal(t, "MANDIRI", updated.BankCode)
		require.Equal(t, "9876543210", updated.AccountNumber)

		after := countSellerBankAccounts(t, ctx, tx, sellerID)
		require.Equal(t, int64(1), after)
		return nil
	})
	require.NoError(t, err)
}

func TestBankAccountPrimaryInvariantMigration_RepairsProbeTableAndEnforcesConstraint(t *testing.T) {
	tdb, _, cleanup := setupBankAccountIntegrationService(t)
	defer cleanup()

	ctx := context.Background()
	migrationSQL := loadPrimaryInvariantProbeMigrationSQL(t)
	realIndexName := "idx_bank_accounts_user_active_default_unique"

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		assertUniqueIndex := func() {
			var exists bool
			require.NoError(t, tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1
					FROM pg_indexes
					WHERE schemaname = 'public'
					  AND tablename = 'bank_accounts'
					  AND indexname = $1
				)
			`, realIndexName).Scan(&exists))
			require.True(t, exists)
		}
		assertUniqueIndex()

		_, err := tx.Exec(ctx, `
			CREATE TEMP TABLE bank_accounts_migration_probe (
				id uuid PRIMARY KEY,
				user_id uuid NOT NULL,
				is_default boolean NOT NULL DEFAULT false,
				deleted_at timestamp with time zone,
				created_at timestamp with time zone NOT NULL,
				updated_at timestamp with time zone NOT NULL
			) ON COMMIT DROP
		`)
		require.NoError(t, err)

		missingDefaultUser := uuid.New()
		multiDefaultUser := uuid.New()
		otherUser := uuid.New()
		oldestMissing := uuid.MustParse("10000000-0000-0000-0000-000000000001")
		oldestPrimary := uuid.MustParse("20000000-0000-0000-0000-000000000001")
		newerPrimary := uuid.MustParse("20000000-0000-0000-0000-000000000002")
		otherPrimary := uuid.MustParse("30000000-0000-0000-0000-000000000001")

		_, err = tx.Exec(ctx, `
			INSERT INTO bank_accounts_migration_probe (
				id, user_id, is_default, deleted_at, created_at, updated_at
			)
			VALUES
				($1, $2, false, NULL, $3, $3),
				($4, $2, false, NULL, $5, $5),
				($6, $7, true, NULL, $8, $8),
				($9, $7, true, NULL, $10, $10),
				($11, $12, true, NULL, $13, $13)
		`, oldestMissing, missingDefaultUser, time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC),
			uuid.New(), missingDefaultUser, time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC),
			oldestPrimary, multiDefaultUser, time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC),
			newerPrimary, multiDefaultUser, time.Date(2026, 8, 1, 8, 30, 0, 0, time.UTC),
			otherPrimary, otherUser, time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC))
		require.NoError(t, err)

		_, err = tx.Exec(ctx, migrationSQL)
		require.NoError(t, err)

		var missingDefaultCount int64
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM bank_accounts_migration_probe
			WHERE user_id = $1
			  AND deleted_at IS NULL
			  AND is_default = true
		`, missingDefaultUser).Scan(&missingDefaultCount))
		require.Equal(t, int64(1), missingDefaultCount)

		var missingDefaultKeeper uuid.UUID
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT id
			FROM bank_accounts_migration_probe
			WHERE user_id = $1
			  AND deleted_at IS NULL
			  AND is_default = true
		`, missingDefaultUser).Scan(&missingDefaultKeeper))
		require.Equal(t, oldestMissing, missingDefaultKeeper)

		var multiDefaultCount int64
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM bank_accounts_migration_probe
			WHERE user_id = $1
			  AND deleted_at IS NULL
			  AND is_default = true
		`, multiDefaultUser).Scan(&multiDefaultCount))
		require.Equal(t, int64(1), multiDefaultCount)

		var multiDefaultKeeper uuid.UUID
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT id
			FROM bank_accounts_migration_probe
			WHERE user_id = $1
			  AND deleted_at IS NULL
			  AND is_default = true
		`, multiDefaultUser).Scan(&multiDefaultKeeper))
		require.Equal(t, oldestPrimary, multiDefaultKeeper)

		var probeIndexExists bool
		require.NoError(t, tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM pg_indexes
				WHERE tablename = 'bank_accounts_migration_probe'
				  AND indexname = $1
			)
		`, "idx_bank_accounts_migration_probe_unique").Scan(&probeIndexExists))
		require.True(t, probeIndexExists)

		_, err = tx.Exec(ctx, `
			INSERT INTO bank_accounts_migration_probe (
				id, user_id, is_default, deleted_at, created_at, updated_at
			)
			VALUES ($1, $2, true, NULL, $3, $3)
		`, uuid.New(), multiDefaultUser, time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC))
		require.Error(t, err)
		var pgErr *pgconn.PgError
		require.True(t, errors.As(err, &pgErr))
		require.Equal(t, "23505", pgErr.Code)

		// Ensure the migration SQL text we exercised really was the exact
		// hardened migration body from disk.
		require.Contains(t, migrationSQL, "missing_default_users")
		require.Contains(t, migrationSQL, "default_keeper")
		require.Contains(t, migrationSQL, "idx_bank_accounts_migration_probe_unique")
		return nil
	})
	require.NoError(t, err)
}
