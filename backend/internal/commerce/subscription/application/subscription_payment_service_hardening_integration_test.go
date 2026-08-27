package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	sellerRepoImpl "github.com/labuda/backend/internal/commerce/seller/infrastructure/repository"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	subscriptionRepoImpl "github.com/labuda/backend/internal/commerce/subscription/infrastructure/repository"
	"github.com/labuda/backend/internal/finance"
	financeApp "github.com/labuda/backend/internal/finance/application"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	"github.com/labuda/backend/internal/identity/auth"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	paymentRepoImpl "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	outboxRepoImpl "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type onboardingSuccessUserRepo struct{}

func (onboardingSuccessUserRepo) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*userEntity.User, error) {
	phone := "+628123456789"
	now := time.Now().UTC()
	return &userEntity.User{
		ID:            uuid.New(),
		EmailVerified: true,
		PhoneNumber:   &phone,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (onboardingSuccessUserRepo) GetProfileByID(context.Context, db.Tx, uuid.UUID) (*userEntity.UserProfile, error) {
	username := "seller-user"
	bio := "seller bio"
	return &userEntity.UserProfile{
		UserID:   uuid.New(),
		Username: &username,
		Bio:      &bio,
	}, nil
}

type onboardingSuccessSellerRepo struct{}

func (onboardingSuccessSellerRepo) GetByUserID(context.Context, db.Tx, uuid.UUID) (*sellerEntity.SellerProfile, error) {
	return &sellerEntity.SellerProfile{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		StoreName: "Toko Labuda",
		Tier:      sellerEntity.TierBasic,
	}, nil
}

type onboardingSuccessAddressRepo struct{}

func (onboardingSuccessAddressRepo) GetByUserIDFiltered(context.Context, db.Tx, uuid.UUID, string) ([]*addressEntity.Address, error) {
	return []*addressEntity.Address{
		{
			ID:      uuid.New(),
			UserID:  uuid.New(),
			Purpose: addressEntity.AddressPurposeSender,
		},
	}, nil
}

type failingOutboxRepo struct {
	err   error
	calls int
}

func (r *failingOutboxRepo) InsertEvent(
	_ context.Context,
	_ db.Tx,
	_ string,
	_ uuid.UUID,
	_ []byte,
) error {
	r.calls++
	if r.err != nil {
		return r.err
	}
	return fmt.Errorf("outbox failure injected")
}

type renewalHarness struct {
	tdb                 *testdb.TestDB
	svc                 *SellerSubscriptionPaymentService
	paymentRepo         *paymentRepoImpl.PaymentRepository
	subRepo             *subscriptionRepoImpl.SellerSubscriptionRepositoryImpl
	sellerRepo          *sellerRepoImpl.SellerRepositoryImpl
	userID              uuid.UUID
	historicalSubID     uuid.UUID
	chainEnd            time.Time
	systemAccountSeeded bool
}

func newRenewalHarness(t *testing.T) *renewalHarness {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	userID := uuid.New()
	chainEnd := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	historicalSubID := uuid.New()
	historicalPaymentID := uuid.New()

	subRepo := subscriptionRepoImpl.NewSellerSubscriptionRepository()
	sellerRepo := sellerRepoImpl.NewSellerRepository()
	paymentRepo := paymentRepoImpl.NewPaymentRepository()
	financeSvc := financeApp.NewFinanceService()
	outboxRepo := outboxRepoImpl.NewOutboxRepository(nil)
	onboardingSvc := NewSellerOnboardingService(
		onboardingSuccessUserRepo{},
		onboardingSuccessSellerRepo{},
		onboardingSuccessAddressRepo{},
	)

	svc := NewSellerSubscriptionPaymentService(
		tdb,
		paymentRepo,
		subRepo,
		sellerRepo,
		nil,
		onboardingSvc,
		financeSvc,
		outboxRepo,
		subRepo,
	)

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		if err := seedRenewalSellerFixture(ctx, tx, userID, chainEnd, historicalSubID, historicalPaymentID); err != nil {
			return err
		}
		if err := seedLedgerAccounts(ctx, tx); err != nil {
			return err
		}
		if err := seedSubscriptionConfig(ctx, tx); err != nil {
			return err
		}
		return nil
	}))

	return &renewalHarness{
		tdb:             tdb,
		svc:             svc,
		paymentRepo:     paymentRepo,
		subRepo:         subRepo,
		sellerRepo:      sellerRepo,
		userID:          userID,
		historicalSubID: historicalSubID,
		chainEnd:        chainEnd,
	}
}

func seedRenewalSellerFixture(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	chainEnd time.Time,
	historicalSubID uuid.UUID,
	historicalPaymentID uuid.UUID,
) error {
	now := chainEnd.Add(-365 * 24 * time.Hour)
	email := fmt.Sprintf("%s@example.com", userID.String())

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
		VALUES ($1, $2, $3, 'active', $4, $4, 'user')
	`, userID, "fb-"+userID.String(), email, chainEnd); err != nil {
		return fmt.Errorf("seed user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO seller_profiles (id, user_id, store_name, tier, created_at, updated_at)
		VALUES ($1, $2, $3, 'basic', $4, $4)
	`, uuid.New(), userID, "Toko Labuda", chainEnd); err != nil {
		return fmt.Errorf("seed seller profile: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO seller_subscriptions (
			id, user_id, status, started_at, expires_at,
			duration_days, amount_paid, currency, payment_id,
			created_at, updated_at
		)
		VALUES ($1, $2, 'expired', $3, $4, 365, 70000, 'IDR', $5, $6, $6)
	`, historicalSubID, userID, now, chainEnd, historicalPaymentID, now); err != nil {
		return fmt.Errorf("seed historical subscription: %w", err)
	}

	return nil
}

func seedLedgerAccounts(ctx context.Context, tx db.Tx) error {
	accounts := []struct {
		accountType string
		name        string
		balance     int64
	}{
		{finance.AccountPlatformRevenue, "Platform Revenue", 0},
		{finance.AccountBankSettlement, "Bank Settlement", 1_000_000_000},
	}

	for _, acct := range accounts {
		if _, err := tx.Exec(ctx, `
			INSERT INTO financial_accounts (
				id, user_id, account_type, balance, currency, name, is_active, created_at, updated_at
			)
			VALUES ($1, NULL, $2, $3, 'IDR', $4, true, NOW(), NOW())
			ON CONFLICT DO NOTHING
		`, uuid.New(), acct.accountType, acct.balance, acct.name); err != nil {
			return fmt.Errorf("seed financial account %s: %w", acct.accountType, err)
		}
	}

	return nil
}

func seedSubscriptionConfig(ctx context.Context, tx db.Tx) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO seller_subscription_configs (
			id, yearly_fee_rupiah, duration_days, renewal_reminder_days, enabled, created_at
		)
		VALUES ('a27309fc-586a-4890-a60a-d867db9a03a9', 70000, 365, 7, true, NOW())
		ON CONFLICT (id) DO NOTHING
	`); err != nil {
		return fmt.Errorf("seed subscription config: %w", err)
	}
	return nil
}

func (h *renewalHarness) createSettledPayment(t *testing.T, paidAt time.Time, suffix string, includePaidAt bool) uuid.UUID {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	var paymentID uuid.UUID
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		referenceID := uuid.New()
		payment, err := h.paymentRepo.CreatePayment(ctx, tx, paymentRepoImpl.CreatePaymentInput{
			UserID:           h.userID,
			PaymentNumber:    "PN-" + suffix + "-" + uuid.NewString(),
			MidtransOrderID:  "renewal-" + suffix + "-" + uuid.NewString(),
			GrossAmount:      money.New(70000),
			ServiceFeeAmount: money.Zero(),
			CoinsToUse:       0,
			ReferenceType:    paymentRepoImpl.ReferenceTypeSubscription,
			ReferenceID:      &referenceID,
			ExpiredAt:        paidAt.Add(24 * time.Hour),
		})
		if err != nil {
			return err
		}
		paymentID = payment.ID

		query := `
			UPDATE payments
			SET status = $1,
			    transaction_id = $2,
			    payment_type = $3,
			    updated_at = NOW()
		`
		args := []any{paymentRepoImpl.PaymentStatusSettlement, "txn-" + suffix, "settlement"}
		if includePaidAt {
			query += ", paid_at = $4"
			query += " WHERE id = $5"
			args = append(args, paidAt, paymentID)
		} else {
			query += " WHERE id = $4"
			args = append(args, paymentID)
		}

		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return err
		}
		return nil
	}))

	return paymentID
}

func (h *renewalHarness) createPaymentWithStatus(t *testing.T, status string, paidAt *time.Time, suffix string) uuid.UUID {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	var paymentID uuid.UUID
	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		referenceID := uuid.New()
		payment, err := h.paymentRepo.CreatePayment(ctx, tx, paymentRepoImpl.CreatePaymentInput{
			UserID:           h.userID,
			PaymentNumber:    "PN-" + suffix + "-" + uuid.NewString(),
			MidtransOrderID:  "state-" + suffix + "-" + uuid.NewString(),
			GrossAmount:      money.New(70000),
			ServiceFeeAmount: money.Zero(),
			CoinsToUse:       0,
			ReferenceType:    paymentRepoImpl.ReferenceTypeSubscription,
			ReferenceID:      &referenceID,
			ExpiredAt:        time.Now().Add(24 * time.Hour),
		})
		if err != nil {
			return err
		}
		paymentID = payment.ID

		query := `
			UPDATE payments
			SET status = $1,
			    transaction_id = $2,
			    payment_type = $3,
			    updated_at = NOW()
		`
		args := []any{status, "txn-" + suffix, "settlement"}
		if paidAt != nil {
			query += ", paid_at = $4 WHERE id = $5"
			args = append(args, *paidAt, paymentID)
		} else {
			query += " WHERE id = $4"
			args = append(args, paymentID)
		}

		if _, err := tx.Exec(ctx, query, args...); err != nil {
			return err
		}
		return nil
	}))

	return paymentID
}

func countRows(t *testing.T, tdb *testdb.TestDB, query string, args ...any) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var count int
	require.NoError(t, tdb.Pool().QueryRow(ctx, query, args...).Scan(&count))
	return count
}

func fetchActiveSubscriptions(t *testing.T, tdb *testdb.TestDB, userID uuid.UUID) []subscriptionEntity.SellerSubscription {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	rows, err := tdb.Pool().Query(ctx, `
		SELECT id, user_id, status, started_at, expires_at, duration_days,
		       amount_paid, currency, payment_id, created_at, updated_at
		FROM seller_subscriptions
		WHERE user_id = $1
		  AND status = 'active'
		ORDER BY started_at ASC
	`, userID)
	require.NoError(t, err)
	defer rows.Close()

	var subs []subscriptionEntity.SellerSubscription
	for rows.Next() {
		var sub subscriptionEntity.SellerSubscription
		var amountPaid int64
		require.NoError(t, rows.Scan(
			&sub.ID, &sub.UserID, &sub.Status, &sub.StartedAt, &sub.ExpiresAt, &sub.DurationDays,
			&amountPaid, &sub.Currency, &sub.PaymentID, &sub.CreatedAt, &sub.UpdatedAt,
		))
		sub.AmountPaid = money.New(amountPaid)
		subs = append(subs, sub)
	}
	require.NoError(t, rows.Err())
	return subs
}

func TestProcessSuccessfulPaymentTx_PaymentStateMatrix(t *testing.T) {
	h := newRenewalHarness(t)

	ctx := context.Background()
	missingPaidAtPaymentID := h.createPaymentWithStatus(t, paymentRepoImpl.PaymentStatusSettlement, nil, "missing-paid-at")
	pendingPaymentID := h.createPaymentWithStatus(t, paymentRepoImpl.PaymentStatusPending, nil, "pending")
	failedPaymentID := h.createPaymentWithStatus(t, paymentRepoImpl.PaymentStatusDeny, nil, "failed")
	cancelledPaymentID := h.createPaymentWithStatus(t, paymentRepoImpl.PaymentStatusCancel, nil, "cancelled")
	validPaidAt := h.chainEnd.Add(-2 * time.Hour)
	validPaymentID := h.createSettledPayment(t, validPaidAt, "valid", true)

	cases := []struct {
		name             string
		paymentID        uuid.UUID
		wantErr          error
		wantEntitlements int
	}{
		{name: "settled_missing_paid_at", paymentID: missingPaidAtPaymentID, wantErr: ErrMissingSettlementTimestamp, wantEntitlements: 0},
		{name: "pending_rejected", paymentID: pendingPaymentID, wantErr: nil, wantEntitlements: 0},
		{name: "failed_rejected", paymentID: failedPaymentID, wantErr: nil, wantEntitlements: 0},
		{name: "cancelled_rejected", paymentID: cancelledPaymentID, wantErr: nil, wantEntitlements: 0},
		{name: "settled_valid", paymentID: validPaymentID, wantErr: nil, wantEntitlements: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := h.svc.ProcessSuccessfulPayment(ctx, tc.paymentID, h.userID, "state-"+tc.name)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else if tc.wantEntitlements == 0 {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			subCount := countRows(t, h.tdb, `SELECT COUNT(*) FROM seller_subscriptions WHERE payment_id = $1`, tc.paymentID)
			assert.Equal(t, tc.wantEntitlements, subCount)
		})
	}
}

func TestProcessSuccessfulPaymentTx_ConcurrentDistinctPayments_StackAtChainEnd(t *testing.T) {
	h := newRenewalHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	paymentA := h.createSettledPayment(t, h.chainEnd.Add(-2*time.Hour), "A", true)
	paymentB := h.createSettledPayment(t, h.chainEnd.Add(-1*time.Hour), "B", true)

	var start sync.WaitGroup
	start.Add(2)
	begin := make(chan struct{})
	errs := make(chan error, 2)

	run := func(paymentID uuid.UUID, providerEventID string) {
		errs <- h.tdb.WithTx(ctx, func(tx db.Tx) error {
			start.Done()
			<-begin
			return h.svc.ProcessSuccessfulPaymentTx(ctx, tx, paymentID, h.userID, providerEventID)
		})
	}

	go run(paymentA, "concurrent-a")
	go run(paymentB, "concurrent-b")

	start.Wait()
	close(begin)

	err1 := <-errs
	err2 := <-errs
	require.NoError(t, err1)
	require.NoError(t, err2)

	subs := fetchActiveSubscriptions(t, h.tdb, h.userID)
	require.Len(t, subs, 2)

	assert.True(t, subs[0].StartedAt.Equal(h.chainEnd))
	assert.True(t, subs[0].ExpiresAt.Equal(h.chainEnd.Add(365*24*time.Hour)))
	assert.True(t, subs[0].ExpiresAt.Equal(subs[1].StartedAt))
	assert.True(t, subs[1].ExpiresAt.Equal(h.chainEnd.Add(730*24*time.Hour)))
	seenPayments := map[uuid.UUID]bool{
		subs[0].PaymentID: true,
		subs[1].PaymentID: true,
	}
	assert.True(t, seenPayments[paymentA])
	assert.True(t, seenPayments[paymentB])

	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		chainEnd, err := h.subRepo.GetLatestByUserIDForUpdate(ctx, tx, h.userID)
		require.NoError(t, err)
		require.NotNil(t, chainEnd)
		assert.True(t, chainEnd.ExpiresAt.Equal(h.chainEnd.Add(730*24*time.Hour)))

		historical, err := h.subRepo.GetByID(ctx, tx, h.historicalSubID)
		require.NoError(t, err)
		require.NotNil(t, historical)
		assert.Equal(t, subscriptionEntity.StatusExpired, historical.Status)
		assert.True(t, historical.StartedAt.Equal(h.chainEnd.Add(-365*24*time.Hour)))
		return nil
	}))

	assert.Equal(t, 2, countRows(t, h.tdb, `SELECT COUNT(*) FROM ledger_transactions WHERE reference_type = 'seller_subscription_payment'`))
	assert.Equal(t, 2, countRows(t, h.tdb, `SELECT COUNT(*) FROM outbox WHERE event_type = 'seller.subscription.activated'`))

	checker := auth.NewRoleCheckerDB(db.NewFromPool(h.tdb.Pool()), nil)
	ok, err := checker.HasActiveSellerCapability(ctx, h.userID)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestProcessSuccessfulPaymentTx_ConcurrentSamePayment_ReplayIdempotent(t *testing.T) {
	h := newRenewalHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	paymentID := h.createSettledPayment(t, h.chainEnd.Add(-2*time.Hour), "same", true)

	var start sync.WaitGroup
	start.Add(2)
	begin := make(chan struct{})
	errs := make(chan error, 2)

	run := func(providerEventID string) {
		errs <- h.tdb.WithTx(ctx, func(tx db.Tx) error {
			start.Done()
			<-begin
			return h.svc.ProcessSuccessfulPaymentTx(ctx, tx, paymentID, h.userID, providerEventID)
		})
	}

	go run("same-payment")
	go run("same-payment")

	start.Wait()
	close(begin)

	err1 := <-errs
	err2 := <-errs
	require.NoError(t, err1)
	require.NoError(t, err2)

	assert.Equal(t, 1, countRows(t, h.tdb, `SELECT COUNT(*) FROM seller_subscriptions WHERE payment_id = $1`, paymentID))
	assert.Equal(t, 1, countRows(t, h.tdb, `SELECT COUNT(*) FROM ledger_transactions WHERE reference_id = $1`, paymentID))
	assert.Equal(t, 1, countRows(t, h.tdb, `SELECT COUNT(*) FROM outbox WHERE event_type = 'seller.subscription.activated'`))
}

func TestProcessSuccessfulPaymentTx_RollbackOnOutboxFailure_RevertsWrites(t *testing.T) {
	h := newRenewalHarness(t)
	ctx := context.Background()
	paymentID := h.createSettledPayment(t, h.chainEnd.Add(-2*time.Hour), "rollback", true)

	failingOutbox := &failingOutboxRepo{err: errors.New("forced outbox failure")}
	h.svc.outboxRepo = failingOutbox

	err := h.svc.ProcessSuccessfulPayment(ctx, paymentID, h.userID, "rollback-event")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "emit outbox event failed")

	assert.Equal(t, 0, countRows(t, h.tdb, `SELECT COUNT(*) FROM seller_subscriptions WHERE payment_id = $1`, paymentID))
	assert.Equal(t, 0, countRows(t, h.tdb, `SELECT COUNT(*) FROM ledger_transactions WHERE reference_id = $1`, paymentID))
	assert.Equal(t, 0, countRows(t, h.tdb, `SELECT COUNT(*) FROM outbox WHERE event_type = 'seller.subscription.activated'`))

	h.svc.outboxRepo = outboxRepoImpl.NewOutboxRepository(nil)
	require.NoError(t, h.svc.ProcessSuccessfulPayment(ctx, paymentID, h.userID, "rollback-event-retry"))

	assert.Equal(t, 1, countRows(t, h.tdb, `SELECT COUNT(*) FROM seller_subscriptions WHERE payment_id = $1`, paymentID))
	assert.Equal(t, 1, countRows(t, h.tdb, `SELECT COUNT(*) FROM ledger_transactions WHERE reference_id = $1`, paymentID))
	assert.Equal(t, 1, countRows(t, h.tdb, `SELECT COUNT(*) FROM outbox WHERE event_type = 'seller.subscription.activated'`))
}

func TestProcessSuccessfulPaymentTx_HistoricalAndFutureChain(t *testing.T) {
	h := newRenewalHarness(t)
	ctx := context.Background()

	require.NoError(t, h.tdb.WithTx(ctx, func(tx db.Tx) error {
		historical, err := h.subRepo.GetByID(ctx, tx, h.historicalSubID)
		require.NoError(t, err)
		require.NotNil(t, historical)
		assert.Equal(t, subscriptionEntity.StatusExpired, historical.Status)
		return nil
	}))

	paymentID := h.createSettledPayment(t, h.chainEnd.Add(-2*time.Hour), "history", true)
	require.NoError(t, h.svc.ProcessSuccessfulPayment(ctx, paymentID, h.userID, "history-event"))

	subs := fetchActiveSubscriptions(t, h.tdb, h.userID)
	require.Len(t, subs, 1)
	assert.True(t, subs[0].StartedAt.Equal(h.chainEnd))
	assert.True(t, subs[0].ExpiresAt.Equal(h.chainEnd.Add(365*24*time.Hour)))

	nextPaymentID := h.createSettledPayment(t, h.chainEnd.Add(-time.Hour), "future", true)
	require.NoError(t, h.svc.ProcessSuccessfulPayment(ctx, nextPaymentID, h.userID, "history-event-2"))

	subs = fetchActiveSubscriptions(t, h.tdb, h.userID)
	require.Len(t, subs, 2)
	assert.True(t, subs[1].ExpiresAt.Equal(h.chainEnd.Add(730*24*time.Hour)))
}
