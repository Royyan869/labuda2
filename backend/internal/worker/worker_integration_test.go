//go:build worker_sql_alignment

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	alertapp "github.com/labuda/backend/internal/platform/alert/application"
	alertentity "github.com/labuda/backend/internal/platform/alert/entity"
	alertrepo "github.com/labuda/backend/internal/platform/alert/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type recordedOutboxEvent struct {
	eventType      string
	idempotencyKey string
	payload        map[string]any
}

type recordingOutbox struct {
	mu     sync.Mutex
	events []recordedOutboxEvent
}

func (o *recordingOutbox) InsertTx(ctx context.Context, tx db.Tx, eventType string, payload any, idempotencyKey string) error {
	m, ok := payload.(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected outbox payload type %T", payload)
	}

	copyPayload := make(map[string]any, len(m))
	for k, v := range m {
		copyPayload[k] = v
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, recordedOutboxEvent{
		eventType:      eventType,
		idempotencyKey: idempotencyKey,
		payload:        copyPayload,
	})
	return nil
}

func (o *recordingOutbox) snapshot() []recordedOutboxEvent {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]recordedOutboxEvent, len(o.events))
	copy(out, o.events)
	return out
}

func seedUser(t *testing.T, ctx context.Context, tx db.Tx, label string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, email_verified_at, phone_verified,
			account_status, created_at, updated_at
		)
		VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW())
	`, id, id.String(), fmt.Sprintf("%s-%s@test.invalid", label, id.String()))
	require.NoError(t, err)
	return id
}

func seedOrder(t *testing.T, ctx context.Context, tx db.Tx, buyerID, sellerID uuid.UUID, status string, readyToShipBy *time.Time) uuid.UUID {
	t.Helper()

	orderID := uuid.New()
	sourceID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO orders (
			id, buyer_id, seller_id, source_type, source_id,
			quantity, unit_price, subtotal, shipping_total,
			commission_percent, commission_amount, status,
			ready_to_ship_by, created_at, updated_at
		)
		VALUES ($1, $2, $3, 'for_sale', $4,
		        1, 100000, 100000, 0,
		        0, 0, $5,
		        $6, NOW(), NOW())
	`, orderID, buyerID, sellerID, sourceID, status, readyToShipBy)
	require.NoError(t, err)
	return orderID
}

func seedWallet(t *testing.T, ctx context.Context, tx db.Tx, userID uuid.UUID) uuid.UUID {
	t.Helper()

	walletID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO wallets (
			id, user_id, available_balance, held_balance, pending_withdrawal,
			created_at, updated_at
		)
		VALUES ($1, $2, 0, 0, 0, NOW(), NOW())
	`, walletID, userID)
	require.NoError(t, err)
	return walletID
}

func seedEscrow(t *testing.T, ctx context.Context, tx db.Tx, orderID, buyerWalletID uuid.UUID, status string, createdAt time.Time) uuid.UUID {
	t.Helper()

	escrowID := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO escrows (
			id, order_id, buyer_wallet_id, seller_wallet_id, amount, status, created_at
		)
		VALUES ($1, $2, $3, NULL, 100000, $4, $5)
	`, escrowID, orderID, buyerWalletID, status, createdAt)
	require.NoError(t, err)
	return escrowID
}

func seedFinancialAccount(t *testing.T, ctx context.Context, tx db.Tx, userID *uuid.UUID, accountType string, balance int64) uuid.UUID {
	t.Helper()

	accountID := uuid.New()
	name := accountType + " Account"
	var userIDArg any
	if userID != nil {
		userIDArg = *userID
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO financial_accounts (
			id, user_id, account_type, balance, currency, name, is_active, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, 'IDR', $5, true, NOW(), NOW())
	`, accountID, userIDArg, accountType, balance, name)
	require.NoError(t, err)
	return accountID
}

func seedWithdrawalLedgerTx(
	t *testing.T,
	ctx context.Context,
	tx db.Tx,
	sellerAccountID uuid.UUID,
	pendingAccountID uuid.UUID,
	withdrawalID uuid.UUID,
	referenceType string,
	amount int64,
	at time.Time,
	sellerEntryType string,
	pendingEntryType string,
) {
	t.Helper()

	txID := uuid.New()
	idempotencyKey := fmt.Sprintf("%s_%s", referenceType, withdrawalID.String())
	_, err := tx.Exec(ctx, `
		INSERT INTO ledger_transactions (
			id, idempotency_key, reference_type, reference_id, order_id, payment_id,
			total_debit, total_credit, created_at
		)
		VALUES ($1, $2, $3, $4, NULL, NULL, $5, $6, $7)
	`, txID, idempotencyKey, referenceType, withdrawalID, amount, amount, at.Unix())
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		INSERT INTO ledger_entries (
			id, transaction_id, account_id, entry_type, amount, balance_after, created_at
		)
		VALUES ($1, $2, $3, $4, $5, 0, $6)
	`, uuid.New(), txID, sellerAccountID, sellerEntryType, amount, at.Unix())
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		INSERT INTO ledger_entries (
			id, transaction_id, account_id, entry_type, amount, balance_after, created_at
		)
		VALUES ($1, $2, $3, $4, $5, 0, $6)
	`, uuid.New(), txID, pendingAccountID, pendingEntryType, amount, at.Unix())
	require.NoError(t, err)
}

func seedWithdrawalRequest(t *testing.T, ctx context.Context, tx db.Tx, sellerAccountID, pendingAccountID, withdrawalID uuid.UUID, amount int64, at time.Time) {
	t.Helper()
	seedWithdrawalLedgerTx(t, ctx, tx, sellerAccountID, pendingAccountID, withdrawalID, "withdrawal_request", amount, at, "credit", "debit")
}

func seedWithdrawalReject(t *testing.T, ctx context.Context, tx db.Tx, sellerAccountID, pendingAccountID, withdrawalID uuid.UUID, amount int64, at time.Time) {
	t.Helper()
	seedWithdrawalLedgerTx(t, ctx, tx, sellerAccountID, pendingAccountID, withdrawalID, "withdrawal_reject", amount, at, "debit", "credit")
}

func countAlerts(t *testing.T, ctx context.Context, tx db.Tx, alertType string, entityID uuid.UUID) int {
	t.Helper()

	var count int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM system_alerts
		WHERE alert_type = $1 AND entity_id = $2
	`, alertType, entityID).Scan(&count)
	require.NoError(t, err)
	return count
}

func fetchAlertMetadata(t *testing.T, ctx context.Context, tx db.Tx, alertType string, entityID uuid.UUID) map[string]any {
	t.Helper()

	var metadataBytes []byte
	err := tx.QueryRow(ctx, `
		SELECT metadata_json
		FROM system_alerts
		WHERE alert_type = $1 AND entity_id = $2
	`, alertType, entityID).Scan(&metadataBytes)
	require.NoError(t, err)

	var metadata map[string]any
	require.NoError(t, json.Unmarshal(metadataBytes, &metadata))
	return metadata
}

func setupWorkerIntegrationDB(t *testing.T) (*testdb.TestDB, func()) {
	t.Helper()
	tdb, _ := testdb.SetupDB(t)
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = tdb.Pool().Exec(cleanupCtx, `
			TRUNCATE TABLE
				system_alerts,
				outbox,
				order_overdue_reminders,
				escrows,
				orders,
				wallets,
				financial_accounts,
				ledger_entries,
				ledger_transactions
			CASCADE;
		`)
		tdb.Pool().Close()
	}
	return tdb, cleanup
}

func TestOrderOverdueReminderRepository_OrdersByDeadlineAndExcludesReminded(t *testing.T) {
	tdb, cleanup := setupWorkerIntegrationDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewOrderOverdueReminderRepository()

	var oldestOrderID, newerOrderID, remindedOrderID, cancelledOrderID uuid.UUID
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		buyerID := seedUser(t, ctx, tx, "buyer")
		sellerID := seedUser(t, ctx, tx, "seller")

		oldest := time.Now().UTC().Add(-3 * time.Hour)
		newer := time.Now().UTC().Add(-2 * time.Hour)
		reminded := time.Now().UTC().Add(-90 * time.Minute)
		cancelled := time.Now().UTC().Add(-4 * time.Hour)

		oldestOrderID = seedOrder(t, ctx, tx, buyerID, sellerID, "paid", &oldest)
		newerOrderID = seedOrder(t, ctx, tx, buyerID, sellerID, "paid", &newer)
		remindedOrderID = seedOrder(t, ctx, tx, buyerID, sellerID, "paid", &reminded)
		cancelledOrderID = seedOrder(t, ctx, tx, buyerID, sellerID, "cancelled_timeout", &cancelled)

		_, err := tx.Exec(ctx, `
			INSERT INTO order_overdue_reminders (id, order_id, tier, sent_at)
			VALUES ($1, $2, $3, NOW())
		`, uuid.New(), remindedOrderID, string(ReminderTier1))
		require.NoError(t, err)

		ids, err := repo.FindOrdersNeedingReminder(ctx, tx, ReminderTier1, 10)
		require.NoError(t, err)
		require.Len(t, ids, 2)
		require.Equal(t, oldestOrderID, ids[0])
		require.Equal(t, newerOrderID, ids[1])
		require.NotContains(t, ids, remindedOrderID)
		require.NotContains(t, ids, cancelledOrderID)
		return nil
	})
	require.NoError(t, err)
}

func TestOrderOverdueReminderRepository_SkipsLockedRows(t *testing.T) {
	tdb, cleanup := setupWorkerIntegrationDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewOrderOverdueReminderRepository()

	var lockedOrderID, unlockedOrderID uuid.UUID
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		buyerID := seedUser(t, ctx, tx, "buyer")
		sellerID := seedUser(t, ctx, tx, "seller")
		lockedDeadline := time.Now().UTC().Add(-3 * time.Hour)
		unlockedDeadline := time.Now().UTC().Add(-2 * time.Hour)
		lockedOrderID = seedOrder(t, ctx, tx, buyerID, sellerID, "paid", &lockedDeadline)
		unlockedOrderID = seedOrder(t, ctx, tx, buyerID, sellerID, "paid", &unlockedDeadline)
		return nil
	})
	require.NoError(t, err)

	lockTx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer lockTx.Rollback(ctx)

	var locked uuid.UUID
	err = lockTx.QueryRow(ctx, `
		SELECT id
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, lockedOrderID).Scan(&locked)
	require.NoError(t, err)
	require.Equal(t, lockedOrderID, locked)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		ids, err := repo.FindOrdersNeedingReminder(ctx, tx, ReminderTier1, 10)
		require.NoError(t, err)
		require.Len(t, ids, 1)
		require.Equal(t, unlockedOrderID, ids[0])
		return nil
	})
	require.NoError(t, err)
}

func TestOrderOverdueReminderWorker_ProcessTier1Reminders_IsIdempotent(t *testing.T) {
	tdb, cleanup := setupWorkerIntegrationDB(t)
	defer cleanup()

	ctx := context.Background()
	outbox := &recordingOutbox{}
	worker := NewOrderOverdueReminderWorker(tdb, NewOrderOverdueReminderRepository(), outbox, zaptest.NewLogger(t))
	worker.SetBatchSize(10)

	var firstOrderID, secondOrderID uuid.UUID
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		buyerID := seedUser(t, ctx, tx, "buyer")
		sellerID := seedUser(t, ctx, tx, "seller")
		firstDeadline := time.Now().UTC().Add(-3 * time.Hour)
		secondDeadline := time.Now().UTC().Add(-2 * time.Hour)
		firstOrderID = seedOrder(t, ctx, tx, buyerID, sellerID, "paid", &firstDeadline)
		secondOrderID = seedOrder(t, ctx, tx, buyerID, sellerID, "paid", &secondDeadline)
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, worker.ProcessTier1Reminders(ctx))
	require.NoError(t, worker.ProcessTier1Reminders(ctx))

	events := outbox.snapshot()
	require.Len(t, events, 2)
	require.Equal(t, "order.overdue_reminder.seller", events[0].eventType)
	require.Equal(t, "order.overdue_reminder.seller", events[1].eventType)
	require.Equal(t, firstOrderID, events[0].payload["order_id"])
	require.Equal(t, secondOrderID, events[1].payload["order_id"])

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		var reminderCount int
		err := tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM order_overdue_reminders
			WHERE tier = $1
		`, string(ReminderTier1)).Scan(&reminderCount)
		require.NoError(t, err)
		require.Equal(t, 2, reminderCount)
		return nil
	})
	require.NoError(t, err)
}

func TestAlertDetectionWorker_ManualProcess_DetectsAndDeduplicatesWithdrawalAndEscrowAlerts(t *testing.T) {
	tdb, cleanup := setupWorkerIntegrationDB(t)
	defer cleanup()

	ctx := context.Background()
	alertService := alertapp.NewAlertService(tdb, alertrepo.NewAlertRepository(), zaptest.NewLogger(t))
	worker := NewAlertDetectionWorker(tdb, alertService, zaptest.NewLogger(t), DefaultAlertDetectionConfig())

	var withdrawalUserID uuid.UUID
	var oldestEscrowOrderID uuid.UUID

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		// Withdrawal anomaly seed data.
		withdrawalUserID = seedUser(t, ctx, tx, "withdrawal-user")
		otherUserID := seedUser(t, ctx, tx, "other-user")

		withdrawalAccountID := seedFinancialAccount(t, ctx, tx, &withdrawalUserID, finance.AccountSellerPayable, 2_000_000)
		otherAccountID := seedFinancialAccount(t, ctx, tx, &otherUserID, finance.AccountSellerPayable, 2_000_000)
		pendingAccountID := seedFinancialAccount(t, ctx, tx, nil, finance.AccountWithdrawalPending, 0)

		now := time.Now().UTC()
		seedWithdrawalRequest(t, ctx, tx, withdrawalAccountID, pendingAccountID, uuid.New(), 600_000, now.Add(-30*time.Minute))
		seedWithdrawalRequest(t, ctx, tx, withdrawalAccountID, pendingAccountID, uuid.New(), 500_000, now.Add(-20*time.Minute))
		seedWithdrawalReject(t, ctx, tx, withdrawalAccountID, pendingAccountID, uuid.New(), 700_000, now.Add(-10*time.Minute))
		seedWithdrawalRequest(t, ctx, tx, withdrawalAccountID, pendingAccountID, uuid.New(), 2_000_000, now.Add(-2*time.Hour))
		seedWithdrawalRequest(t, ctx, tx, otherAccountID, pendingAccountID, uuid.New(), 900_000, now.Add(-15*time.Minute))

		// Escrow stuck seed data.
		buyerOne := seedUser(t, ctx, tx, "buyer-one")
		sellerOne := seedUser(t, ctx, tx, "seller-one")
		buyerTwo := seedUser(t, ctx, tx, "buyer-two")
		sellerTwo := seedUser(t, ctx, tx, "seller-two")
		buyerThree := seedUser(t, ctx, tx, "buyer-three")
		sellerThree := seedUser(t, ctx, tx, "seller-three")

		oldestDeadline := time.Now().UTC().Add(-15 * 24 * time.Hour)
		secondDeadline := time.Now().UTC().Add(-8 * 24 * time.Hour)
		releasedDeadline := time.Now().UTC().Add(-20 * 24 * time.Hour)
		cancelledDeadline := time.Now().UTC().Add(-17 * 24 * time.Hour)

		oldestEscrowOrderID = seedOrder(t, ctx, tx, buyerOne, sellerOne, "paid", &oldestDeadline)
		secondOrderID := seedOrder(t, ctx, tx, buyerTwo, sellerTwo, "paid", &secondDeadline)
		releasedOrderID := seedOrder(t, ctx, tx, buyerThree, sellerThree, "paid", &releasedDeadline)
		cancelledOrderID := seedOrder(t, ctx, tx, buyerThree, sellerThree, "cancelled_timeout", &cancelledDeadline)

		oldestWalletID := seedWallet(t, ctx, tx, buyerOne)
		secondWalletID := seedWallet(t, ctx, tx, buyerTwo)
		buyerThreeWalletID := seedWallet(t, ctx, tx, buyerThree)

		seedEscrow(t, ctx, tx, oldestEscrowOrderID, oldestWalletID, "holding", time.Now().UTC().Add(-15*24*time.Hour))
		seedEscrow(t, ctx, tx, secondOrderID, secondWalletID, "holding", time.Now().UTC().Add(-8*24*time.Hour))
		seedEscrow(t, ctx, tx, releasedOrderID, buyerThreeWalletID, "released", time.Now().UTC().Add(-20*24*time.Hour))
		seedEscrow(t, ctx, tx, cancelledOrderID, buyerThreeWalletID, "holding", time.Now().UTC().Add(-17*24*time.Hour))

		return nil
	})
	require.NoError(t, err)

	require.NoError(t, worker.ManualProcess(ctx))
	require.NoError(t, worker.ManualProcess(ctx))

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		require.Equal(t, 1, countAlerts(t, ctx, tx, string(alertentity.AlertTypeWithdrawalAnomaly), withdrawalUserID))
		require.Equal(t, 1, countAlerts(t, ctx, tx, string(alertentity.AlertTypeEscrowStuck), uuid.Nil))

		withdrawalMetadata := fetchAlertMetadata(t, ctx, tx, string(alertentity.AlertTypeWithdrawalAnomaly), withdrawalUserID)
		require.Equal(t, withdrawalUserID.String(), withdrawalMetadata["user_id"])
		require.Equal(t, finance.AccountSellerPayable, withdrawalMetadata["account_type"])
		require.Equal(t, "withdrawal_request", withdrawalMetadata["reference_type"])
		require.Equal(t, float64(2), withdrawalMetadata["withdrawal_count"])
		require.Equal(t, float64(1_100_000), withdrawalMetadata["total_amount"])
		require.Equal(t, float64(WithdrawalAnomalyWindowHours), withdrawalMetadata["window_hours"])
		require.Equal(t, float64(WithdrawalAnomalyThreshold), withdrawalMetadata["threshold"])

		escrowMetadata := fetchAlertMetadata(t, ctx, tx, string(alertentity.AlertTypeEscrowStuck), uuid.Nil)
		require.Equal(t, float64(2), escrowMetadata["stuck_count"])
		require.Equal(t, oldestEscrowOrderID.String(), escrowMetadata["oldest_order_id"])
		require.Equal(t, float64(EscrowStuckThresholdDays), escrowMetadata["threshold_days"])
		require.Equal(t, float64(EscrowStuckCriticalDays), escrowMetadata["critical_days"])

		var severity string
		err := tx.QueryRow(ctx, `
			SELECT severity
			FROM system_alerts
			WHERE alert_type = $1 AND entity_id = $2
		`, string(alertentity.AlertTypeEscrowStuck), uuid.Nil).Scan(&severity)
		require.NoError(t, err)
		require.Equal(t, string(alertentity.SeverityCritical), severity)
		return nil
	})
	require.NoError(t, err)
}
