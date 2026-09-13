// PMF-02: SELLER SUBSCRIPTION PAYMENT-METHOD FEE CONVERGENCE
//
// These tests prove the settlement money equation for a FEE-BEARING seller
// subscription payment, using the payment's immutable snapshot as the only
// monetary authority:
//
//	A = payment.gross_amount - payment.service_fee_amount   (principal)
//	F = payment.service_fee_amount                          (payment-method fee)
//	A + F = payment.gross_amount                            (gateway gross)
//
//	AmountPaid          = A
//	subscription revenue = A
//	payment-method fee revenue = F
//	PLATFORM_REVENUE    += A + F   (two separate ledger transactions)
//	BANK_SETTLEMENT     -= A + F
//
// To make the authority provable, the ACTIVE config deliberately carries a
// different yearly_fee_rupiah than the snapshot principal. Any settlement that
// still read config as the money authority would fail these assertions. Config
// remains authoritative for duration only.
package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	financeledger "github.com/labuda/backend/internal/finance"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
	paymentRepository "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newProcessServiceForFeeSplit wires a subscription payment service whose
// payment snapshot carries gross (A+F) and fee, while the active config carries
// a deliberately different yearly fee. The divergence is the proof surface:
// config must not leak into any settled monetary value.
func newProcessServiceForFeeSplit(
	t *testing.T,
	gross int64,
	fee int64,
	configYearlyFee int64,
	configDurationDays int,
) (
	*SellerSubscriptionPaymentService,
	*processSubscriptionRepo,
	*processLedgerRepo,
	*processPaymentTx,
	uuid.UUID,
	uuid.UUID,
) {
	t.Helper()

	userID := uuid.New()
	paymentID := uuid.New()

	userPhone := "+628123456789"
	username := "seller-fee"
	bio := "fee split seller"

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
		&processSellerRepo{
			profile: &sellerEntity.SellerProfile{
				ID:        uuid.New(),
				UserID:    userID,
				StoreName: "Toko Fee",
			},
		},
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

	paidAt := time.Date(2026, 12, 1, 10, 0, 0, 0, time.UTC)

	payment := &paymentRepository.Payment{
		ID:               paymentID,
		UserID:           userID,
		Status:           paymentRepository.PaymentStatusSettlement,
		ReferenceType:    paymentRepository.ReferenceTypeSubscription,
		PaidAt:           &paidAt,
		ExpiredAt:        paidAt.Add(24 * time.Hour),
		MidtransOrderID:  "LAB-SUB-FEE",
		GrossAmount:      money.New(gross),
		ServiceFeeAmount: money.New(fee),
	}

	subRepo := &processSubscriptionRepo{}
	sellerRepo := &processSellerRepo{
		profile: &sellerEntity.SellerProfile{
			ID:        uuid.New(),
			UserID:    userID,
			StoreName: "Toko Fee",
		},
	}
	ledger := &processLedgerRepo{}

	svc := NewSellerSubscriptionPaymentService(
		nil,
		&processPaymentRepo{payment: payment},
		subRepo,
		sellerRepo,
		nil,
		onboardingService,
		newProcessFinanceService(t, ledger),
		&mockOutboxRepo{},
		&processConfigRepo{config: &subscriptionEntity.SellerSubscriptionConfig{
			ID:              uuid.New(),
			YearlyFeeRupiah: configYearlyFee,
			DurationDays:    configDurationDays,
			Enabled:         true,
		}},
	)

	return svc, subRepo, ledger, newProcessPaymentTx(paymentID), userID, paymentID
}

// TestProcessSuccessfulPaymentTx_SplitsPrincipalAndFeeFromSnapshot is the
// canonical PMF-02 settlement proof for a fee-bearing payment.
func TestProcessSuccessfulPaymentTx_SplitsPrincipalAndFeeFromSnapshot(t *testing.T) {
	const (
		principal       = int64(100000)
		fee             = int64(7000)
		gross           = principal + fee // 107000
		configYearlyFee = int64(999999)   // deliberately divergent from the snapshot
		configDuration  = 400
	)

	svc, subRepo, ledger, tx, userID, paymentID := newProcessServiceForFeeSplit(
		t, gross, fee, configYearlyFee, configDuration,
	)

	err := svc.ProcessSuccessfulPaymentTx(
		context.Background(), tx, paymentID, userID, "provider-event-fee",
	)
	require.NoError(t, err)

	// Subscription row: AmountPaid is the snapshot principal, and duration still
	// comes from config — only the MONEY authority moved to the snapshot.
	require.Len(t, subRepo.inserted, 1)
	inserted := subRepo.inserted[0]
	assert.Equal(t, principal, inserted.AmountPaid.Int64(),
		"AmountPaid must be the snapshot principal (gross - fee), never config.YearlyFeeRupiah")
	assert.Equal(t, configDuration, inserted.DurationDays,
		"duration is still a config authority")
	assert.Equal(t, paymentID, inserted.PaymentID)

	// Principal and fee must be separate ledger transactions.
	require.Len(t, ledger.calls, 2,
		"principal revenue and payment-method fee revenue must be separate ledger transactions")

	platformRevenue := uuid.NewSHA1(uuid.NameSpaceOID, []byte(financeledger.AccountPlatformRevenue))
	bankSettlement := uuid.NewSHA1(uuid.NameSpaceOID, []byte(financeledger.AccountBankSettlement))

	perTxn := make(map[string]map[uuid.UUID]int64, len(ledger.calls))
	var platformTotal, bankTotal int64
	for _, call := range ledger.calls {
		balances := map[uuid.UUID]int64{}
		var sum int64
		for _, entry := range call.entries {
			balances[entry.AccountID] += entry.Amount.Int64()
			sum += entry.Amount.Int64()
		}
		assert.Zero(t, sum, "every ledger transaction must balance (Σ entries = 0)")
		platformTotal += balances[platformRevenue]
		bankTotal += balances[bankSettlement]
		perTxn[call.idempotencyKey] = balances
	}

	// Total ledger economics: PLATFORM_REVENUE += A+F, BANK_SETTLEMENT -= A+F.
	assert.Equal(t, gross, platformTotal, "PLATFORM_REVENUE must receive A + F")
	assert.Equal(t, -gross, bankTotal, "BANK_SETTLEMENT must drain A + F")

	// The fee transaction is keyed on the payment id; it must carry F only.
	feeKey := "subscription_fee_revenue_" + paymentID.String()
	feeBalances, ok := perTxn[feeKey]
	require.True(t, ok, "fee revenue must be booked under the payment-id idempotency key")
	assert.Equal(t, fee, feeBalances[platformRevenue], "fee transaction must debit PLATFORM_REVENUE with F")
	assert.Equal(t, -fee, feeBalances[bankSettlement], "fee transaction must credit BANK_SETTLEMENT with F")

	// The principal transaction is keyed on the payment identity (PMF02-A1),
	// never on the caller-supplied provider event id passed to activation.
	principalKey := "seller_subscription_payment_" + paymentID.String()
	principalBalances, ok := perTxn[principalKey]
	require.True(t, ok, "subscription revenue must be booked under the payment identity key")
	assert.Equal(t, principal, principalBalances[platformRevenue],
		"principal transaction must debit PLATFORM_REVENUE with A only")
	assert.Equal(t, -principal, principalBalances[bankSettlement])
}

// TestProcessSuccessfulPaymentTx_ZeroFeeBooksNoFeeTransaction proves a zero-fee
// method still settles as AmountPaid = gross with no useless fee ledger effect.
func TestProcessSuccessfulPaymentTx_ZeroFeeBooksNoFeeTransaction(t *testing.T) {
	const principal = int64(70000)

	svc, subRepo, ledger, tx, userID, paymentID := newProcessServiceForFeeSplit(
		t, principal, 0, principal, 365,
	)

	err := svc.ProcessSuccessfulPaymentTx(
		context.Background(), tx, paymentID, userID, "provider-event-zero",
	)
	require.NoError(t, err)

	require.Len(t, subRepo.inserted, 1)
	assert.Equal(t, principal, subRepo.inserted[0].AmountPaid.Int64(),
		"zero fee: AmountPaid must equal gross")
	assert.Equal(t, 1, ledger.createCalls,
		"zero fee must not create a second (fee) ledger transaction")
	assert.Equal(t, "seller_subscription_payment_"+paymentID.String(), ledger.calls[0].idempotencyKey)
}

// TestProcessSuccessfulPaymentTx_FeeRevenueNotDuplicatedOnReplay proves a
// duplicate activation of the same settled payment cannot double-book either the
// principal or the payment-method fee revenue.
func TestProcessSuccessfulPaymentTx_FeeRevenueNotDuplicatedOnReplay(t *testing.T) {
	const (
		principal = int64(100000)
		fee       = int64(7000)
	)

	svc, subRepo, ledger, tx, userID, paymentID := newProcessServiceForFeeSplit(
		t, principal+fee, fee, principal, 365,
	)

	require.NoError(t, svc.ProcessSuccessfulPaymentTx(
		context.Background(), tx, paymentID, userID, "provider-event-replay",
	))
	require.Len(t, ledger.calls, 2)

	require.NoError(t, svc.ProcessSuccessfulPaymentTx(
		context.Background(), tx, paymentID, userID, "provider-event-replay",
	))

	require.Len(t, subRepo.inserted, 1, "replay must not mint a second subscription interval")
	assert.Len(t, ledger.calls, 2, "replay must not book principal or fee revenue twice")
}
