package application

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	financeApp "github.com/labuda/backend/internal/finance/application"
	financeRepo "github.com/labuda/backend/internal/finance/repository"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	paymentRepository "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type processLedgerRepo struct {
	createCalls int
}

func (m *processLedgerRepo) CreateTransaction(
	_ context.Context,
	_ db.Tx,
	_ string,
	_ string,
	_ uuid.UUID,
	_ *uuid.UUID,
	_ *uuid.UUID,
	_ []financeRepo.Entry,
) error {
	m.createCalls++
	return nil
}

func (m *processLedgerRepo) GetAccountBalance(context.Context, db.Tx, uuid.UUID) (money.Money, error) {
	return money.Zero(), nil
}

func (m *processLedgerRepo) GetAccountBalanceForUpdate(context.Context, db.Tx, uuid.UUID) (money.Money, error) {
	return money.Zero(), nil
}

func (m *processLedgerRepo) GetSystemAccountID(context.Context, db.Tx, string) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (m *processLedgerRepo) GetUserAccountID(context.Context, db.Tx, string, uuid.UUID) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (m *processLedgerRepo) GetOrCreateUserAccount(context.Context, db.Tx, string, uuid.UUID) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (m *processLedgerRepo) CountTransactionsByEntityID(context.Context, db.Tx, uuid.UUID) (int, error) {
	return 0, nil
}

func (m *processLedgerRepo) GetTotalCreditToUserAccount(context.Context, db.Tx, string, uuid.UUID) (int64, error) {
	return 0, nil
}

type processPaymentRepo struct {
	payment *paymentRepository.Payment
}

func (r *processPaymentRepo) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*paymentRepository.Payment, error) {
	return r.payment, nil
}

type processSellerRepo struct {
	profile     *sellerEntity.SellerProfile
	lockCalls   int
	ensureCalls int
}

func (r *processSellerRepo) InsertProfileTx(context.Context, db.Tx, *sellerEntity.SellerProfile) error {
	return nil
}

func (r *processSellerRepo) GetByUserID(context.Context, db.Tx, uuid.UUID) (*sellerEntity.SellerProfile, error) {
	return r.profile, nil
}

func (r *processSellerRepo) GetByUserIDForUpdate(context.Context, db.Tx, uuid.UUID) (*sellerEntity.SellerProfile, error) {
	r.lockCalls++
	return r.profile, nil
}

func (r *processSellerRepo) EnsureProfileExistsTx(context.Context, db.Tx, uuid.UUID, string) (*sellerEntity.SellerProfile, error) {
	r.ensureCalls++
	return r.profile, nil
}

func (r *processSellerRepo) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*sellerEntity.SellerProfile, error) {
	return nil, nil
}

func (r *processSellerRepo) UpdateTierTx(context.Context, db.Tx, uuid.UUID, sellerEntity.Tier) error {
	return nil
}

func (r *processSellerRepo) InsertMonthlyMetricTx(context.Context, db.Tx, *sellerEntity.SellerMonthlyMetric) error {
	return nil
}

func (r *processSellerRepo) UpsertReputationStateTx(context.Context, db.Tx, *sellerEntity.SellerReputationState) error {
	return nil
}

func (r *processSellerRepo) GetReputationStateForUpdate(context.Context, db.Tx, uuid.UUID) (*sellerEntity.SellerReputationState, error) {
	return nil, nil
}

type processSubscriptionRepo struct {
	chainEnd  *subscriptionEntity.SellerSubscription
	inserted  []*subscriptionEntity.SellerSubscription
	insertErr error
}

func (r *processSubscriptionRepo) InsertTx(_ context.Context, tx db.Tx, s *subscriptionEntity.SellerSubscription) error {
	r.inserted = append(r.inserted, s)
	if ptx, ok := tx.(*processPaymentTx); ok {
		ptx.markInserted(s.PaymentID)
	}
	return r.insertErr
}

func (r *processSubscriptionRepo) UpdateStatusTx(context.Context, db.Tx, uuid.UUID, subscriptionEntity.Status, subscriptionEntity.Status) error {
	return nil
}

func (r *processSubscriptionRepo) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}

func (r *processSubscriptionRepo) GetByID(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}

func (r *processSubscriptionRepo) GetLatestByUserID(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}

func (r *processSubscriptionRepo) GetLatestByUserIDForUpdate(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return r.chainEnd, nil
}

func (r *processSubscriptionRepo) GetActiveByUserID(context.Context, db.Tx, uuid.UUID) (*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}

func (r *processSubscriptionRepo) FetchActiveExpiredBatch(context.Context, db.Tx, time.Time, int) ([]*subscriptionEntity.SellerSubscription, error) {
	return nil, nil
}

func (r *processSubscriptionRepo) FetchActiveExpiredBatchIDs(context.Context, db.Tx, time.Time, int) ([]uuid.UUID, error) {
	return nil, nil
}

func (r *processSubscriptionRepo) ExistsActiveByUserID(context.Context, db.Tx, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *processSubscriptionRepo) GetActiveConfig(context.Context, db.Tx) (*subscriptionEntity.SellerSubscriptionConfig, error) {
	return nil, nil
}

func (r *processSubscriptionRepo) UpdateConfigTx(context.Context, db.Tx, uuid.UUID, int64, int, int, bool) error {
	return nil
}

type processUserRepo struct {
	user    *userEntity.User
	profile *userEntity.UserProfile
}

func (r *processUserRepo) GetByIDForUpdate(context.Context, db.Tx, uuid.UUID) (*userEntity.User, error) {
	return r.user, nil
}

func (r *processUserRepo) GetProfileByID(context.Context, db.Tx, uuid.UUID) (*userEntity.UserProfile, error) {
	return r.profile, nil
}

type processAddressRepo struct {
	addresses []*addressEntity.Address
}

func (r *processAddressRepo) GetByUserIDFiltered(context.Context, db.Tx, uuid.UUID, string) ([]*addressEntity.Address, error) {
	return r.addresses, nil
}

type processConfigRepo struct {
	config *subscriptionEntity.SellerSubscriptionConfig
}

func (r *processConfigRepo) GetActiveConfig(context.Context, db.Tx) (*subscriptionEntity.SellerSubscriptionConfig, error) {
	return r.config, nil
}

type processPaymentTx struct {
	paymentID uuid.UUID
	inserted  map[uuid.UUID]bool
}

func newProcessPaymentTx(paymentID uuid.UUID) *processPaymentTx {
	return &processPaymentTx{
		paymentID: paymentID,
		inserted:  map[uuid.UUID]bool{},
	}
}

func (t *processPaymentTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "SELECT id FROM payments WHERE id = $1 FOR UPDATE"):
		return &processRow{values: []any{t.paymentID}}
	case strings.Contains(sql, "SELECT COUNT(*) FROM seller_subscriptions WHERE payment_id = $1"):
		count := 0
		if len(args) > 0 {
			if paymentID, ok := args[0].(uuid.UUID); ok && t.inserted[paymentID] {
				count = 1
			}
		}
		return &processRow{values: []any{count}}
	default:
		return &processRow{err: fmt.Errorf("unexpected query: %s", sql)}
	}
}

func (t *processPaymentTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, fmt.Errorf("unexpected query")
}

func (t *processPaymentTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *processPaymentTx) Commit(context.Context) error   { return nil }
func (t *processPaymentTx) Rollback(context.Context) error { return nil }

func (t *processPaymentTx) markInserted(paymentID uuid.UUID) {
	t.inserted[paymentID] = true
}

type processRow struct {
	values []any
	err    error
}

func (r *processRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan arity mismatch: got %d want %d", len(dest), len(r.values))
	}
	for i, value := range r.values {
		switch d := dest[i].(type) {
		case *uuid.UUID:
			*d = value.(uuid.UUID)
		case *int:
			*d = value.(int)
		case *int64:
			*d = value.(int64)
		case *time.Time:
			*d = value.(time.Time)
		case *string:
			*d = value.(string)
		case *bool:
			*d = value.(bool)
		default:
			return fmt.Errorf("unsupported scan destination %T", dest[i])
		}
	}
	return nil
}

func newProcessFinanceService(t *testing.T, ledger *processLedgerRepo) *financeApp.FinanceService {
	t.Helper()

	svc := financeApp.NewFinanceService()
	field := reflect.ValueOf(svc).Elem().FieldByName("ledgerRepo")
	require.True(t, field.IsValid(), "ledgerRepo field should exist")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(ledger))
	return svc
}

func newProcessOnboardingService(userRepo *processUserRepo, sellerRepo *processSellerRepo, addressRepo *processAddressRepo) *SellerOnboardingService {
	return NewSellerOnboardingService(userRepo, sellerRepo, addressRepo)
}

func newProcessServiceForStacking(
	t *testing.T,
	paidAt time.Time,
	chainEnd time.Time,
) (*SellerSubscriptionPaymentService, *processSubscriptionRepo, *processLedgerRepo, *processPaymentTx) {
	t.Helper()

	userID := uuid.New()
	paymentID := uuid.New()

	userPhone := "+628123456789"
	username := "seller-001"
	bio := "seller bio"

	onboardingSellerProfile := &sellerEntity.SellerProfile{
		ID:        uuid.New(),
		UserID:    userID,
		StoreName: "Toko Labuda",
	}

	onboardingService := newProcessOnboardingService(
		&processUserRepo{
			user: &userEntity.User{
				ID:            userID,
				PhoneNumber:   &userPhone,
				EmailVerified: true,
			},
			profile: &userEntity.UserProfile{
				UserID:   userID,
				Username: &username,
				Bio:      &bio,
			},
		},
		&processSellerRepo{profile: onboardingSellerProfile},
		&processAddressRepo{
			addresses: []*addressEntity.Address{
				{
					ID:      uuid.New(),
					UserID:  userID,
					Purpose: addressEntity.AddressPurposeSender,
					Phone:   userPhone,
				},
			},
		},
	)

	payment := &paymentRepository.Payment{
		ID:              paymentID,
		UserID:          userID,
		Status:          paymentRepository.PaymentStatusSettlement,
		ReferenceType:   paymentRepository.ReferenceTypeSubscription,
		PaidAt:          &paidAt,
		ExpiredAt:       paidAt.Add(24 * time.Hour),
		MidtransOrderID: "LAB-SUB-STACK",
		GrossAmount:     money.New(70000),
	}

	subRepo := &processSubscriptionRepo{
		chainEnd: &subscriptionEntity.SellerSubscription{
			UserID:    userID,
			ExpiresAt: chainEnd,
		},
	}
	sellerRepo := &processSellerRepo{
		profile: &sellerEntity.SellerProfile{
			ID:        uuid.New(),
			UserID:    userID,
			StoreName: "Toko Labuda",
		},
	}
	ledger := &processLedgerRepo{}
	financeService := newProcessFinanceService(t, ledger)

	svc := NewSellerSubscriptionPaymentService(
		nil,
		&processPaymentRepo{payment: payment},
		subRepo,
		sellerRepo,
		nil,
		onboardingService,
		financeService,
		&mockOutboxRepo{},
		&processConfigRepo{config: &subscriptionEntity.SellerSubscriptionConfig{
			ID:              uuid.New(),
			YearlyFeeRupiah: 70000,
			DurationDays:    365,
			Enabled:         true,
		}},
	)

	return svc, subRepo, ledger, newProcessPaymentTx(paymentID)
}

func TestProcessSuccessfulPaymentTx_StacksAtChainEnd(t *testing.T) {
	type testCase struct {
		name          string
		paidAt        time.Time
		chainEnd      time.Time
		wantStart     time.Time
		wantExpiresAt time.Time
	}

	cases := []testCase{
		{
			name:          "chain end dominates earlier payment activation",
			paidAt:        time.Date(2026, 12, 1, 10, 0, 0, 0, time.UTC),
			chainEnd:      time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			wantStart:     time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			wantExpiresAt: time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "payment activation dominates when it is later than chain end",
			paidAt:        time.Date(2027, 3, 10, 10, 0, 0, 0, time.UTC),
			chainEnd:      time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			wantStart:     time.Date(2027, 3, 10, 10, 0, 0, 0, time.UTC),
			wantExpiresAt: time.Date(2028, 3, 9, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, subRepo, ledger, tx := newProcessServiceForStacking(t, tc.paidAt, tc.chainEnd)
			userID := subRepo.chainEnd.UserID
			paymentID := uuid.New()
			tx.paymentID = paymentID
			svc.paymentRepo = &processPaymentRepo{
				payment: &paymentRepository.Payment{
					ID:              paymentID,
					UserID:          userID,
					Status:          paymentRepository.PaymentStatusSettlement,
					ReferenceType:   paymentRepository.ReferenceTypeSubscription,
					PaidAt:          &tc.paidAt,
					ExpiredAt:       tc.paidAt.Add(24 * time.Hour),
					MidtransOrderID: "LAB-SUB-STACK",
					GrossAmount:     money.New(70000),
				},
			}
			subRepo.chainEnd = &subscriptionEntity.SellerSubscription{
				UserID:    userID,
				ExpiresAt: tc.chainEnd,
			}
			tx.inserted = map[uuid.UUID]bool{}

			err := svc.ProcessSuccessfulPaymentTx(context.Background(), tx, paymentID, userID, "provider-event-1")
			require.NoError(t, err)

			require.Len(t, subRepo.inserted, 1)
			inserted := subRepo.inserted[0]
			assert.Equal(t, tc.wantStart, inserted.StartedAt)
			assert.Equal(t, tc.wantExpiresAt, inserted.ExpiresAt)
			assert.Equal(t, int64(70000), inserted.AmountPaid.Int64())
			assert.Equal(t, 365, inserted.DurationDays)
			assert.Equal(t, paymentID, inserted.PaymentID)
			assert.Equal(t, 1, ledger.createCalls)
		})
	}
}

func TestProcessSuccessfulPaymentTx_ReplaySkipsSecondInterval(t *testing.T) {
	paidAt := time.Date(2026, 12, 1, 10, 0, 0, 0, time.UTC)
	chainEnd := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	svc, subRepo, ledger, tx := newProcessServiceForStacking(t, paidAt, chainEnd)
	userID := subRepo.chainEnd.UserID
	paymentID := uuid.New()
	tx.paymentID = paymentID
	svc.paymentRepo = &processPaymentRepo{
		payment: &paymentRepository.Payment{
			ID:              paymentID,
			UserID:          userID,
			Status:          paymentRepository.PaymentStatusSettlement,
			ReferenceType:   paymentRepository.ReferenceTypeSubscription,
			PaidAt:          &paidAt,
			ExpiredAt:       paidAt.Add(24 * time.Hour),
			MidtransOrderID: "LAB-SUB-REPLAY",
			GrossAmount:     money.New(70000),
		},
	}

	err := svc.ProcessSuccessfulPaymentTx(context.Background(), tx, paymentID, userID, "provider-event-1")
	require.NoError(t, err)
	require.Len(t, subRepo.inserted, 1)
	require.Equal(t, 1, ledger.createCalls)

	err = svc.ProcessSuccessfulPaymentTx(context.Background(), tx, paymentID, userID, "provider-event-1")
	require.NoError(t, err)
	require.Len(t, subRepo.inserted, 1, "duplicate replay must not mint a second interval")
	require.Equal(t, 1, ledger.createCalls, "duplicate replay must not book revenue twice")
}
