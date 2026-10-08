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
// Both money AND duration are read from the payment snapshot. The service no
// longer takes a config repository at all, so no live config value can leak
// into a settled subscription.
package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
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
	durationDays int,
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
			primary: &addressEntity.Address{
				ID:        uuid.New(),
				UserID:    userID,
				IsPrimary: true,
				Phone:     userPhone,
			},
		},
	)

	paidAt := time.Date(2026, 12, 1, 10, 0, 0, 0, time.UTC)

	payment := &paymentRepository.Payment{
		ID:                       paymentID,
		UserID:                   userID,
		Status:                   paymentRepository.PaymentStatusSettlement,
		ReferenceType:            paymentRepository.ReferenceTypeSubscription,
		PaidAt:                   &paidAt,
		ExpiredAt:                paidAt.Add(24 * time.Hour),
		MidtransOrderID:          "LAB-SUB-FEE",
		GrossAmount:              money.New(gross),
		ServiceFeeAmount:         money.New(fee),
		SubscriptionDurationDays: &durationDays,
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
	)

	return svc, subRepo, ledger, newProcessPaymentTx(paymentID), userID, paymentID
}

// TestProcessSuccessfulPaymentTx_SplitsPrincipalAndFeeFromSnapshot is the
// canonical PMF-02 settlement proof for a fee-bearing payment.
func TestProcessSuccessfulPaymentTx_SplitsPrincipalAndFeeFromSnapshot(t *testing.T) {
	const (
		principal = int64(100000)
		fee       = int64(7000)
		gross     = principal + fee // 107000
		duration  = 400
	)

	svc, subRepo, ledger, tx, userID, paymentID := newProcessServiceForFeeSplit(
		t, gross, fee, duration,
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
	assert.Equal(t, duration, inserted.DurationDays,
		"duration must come from the payment snapshot, never live config")
	assert.Equal(t, paymentID, inserted.PaymentID)

	// Principal and fee must be separate ledger transactions.
	require.Len(t, ledger.calls, 2,
		"principal revenue and payment-method fee revenue must be separate ledger transactions")

	platformRevenue := uuid.NewSHA1(uuid.NameSpaceOID, []byte(financeledger.AccountPlatformRevenue))
	bankSettlement := uuid.NewSHA1(uuid.NameSpaceOID, []byte(financeledger.AccountBankSettlement))

	// CANONICAL SIGN ARCHITECTURE (finance/account_types.go, and the balance
	// formula in finance/infrastructure/repository/ledger_repository.go):
	//
	//   Asset/Expense:     Δbalance = +entry.Amount
	//   Liability/Revenue: Δbalance = -entry.Amount
	//
	// PLATFORM_REVENUE is ClassRevenue and BANK_SETTLEMENT is ClassLiability,
	// so BOTH move opposite to the raw DR/CR amount. Raw entry amounts must be
	// read through that formula before they can be called a "balance" — summing
	// raw amounts and comparing them to a balance figure is what made this proof
	// look sign-flipped.
	perTxn := make(map[string]map[uuid.UUID]int64, len(ledger.calls))
	var platformTotal, bankTotal int64
	for _, call := range ledger.calls {
		balances := map[uuid.UUID]int64{}
		var sum int64
		for _, entry := range call.entries {
			sum += entry.Amount.Int64()
			balances[entry.AccountID] += -entry.Amount.Int64()
		}
		assert.Zero(t, sum, "every ledger transaction must balance (Σ entries = 0)")
		platformTotal += balances[platformRevenue]
		bankTotal += balances[bankSettlement]
		perTxn[call.idempotencyKey] = balances
	}

	// Total ledger economics: PLATFORM_REVENUE += A+F, BANK_SETTLEMENT -= A+F.
	assert.Equal(t, gross, platformTotal, "PLATFORM_REVENUE balance must receive A + F")
	assert.Equal(t, -gross, bankTotal, "BANK_SETTLEMENT balance must drain A + F")

	// The fee transaction is keyed on the payment id; it must carry F only.
	feeKey := "subscription_fee_revenue_" + paymentID.String()
	feeBalances, ok := perTxn[feeKey]
	require.True(t, ok, "fee revenue must be booked under the payment-id idempotency key")
	assert.Equal(t, fee, feeBalances[platformRevenue], "fee transaction must raise PLATFORM_REVENUE balance by F")
	assert.Equal(t, -fee, feeBalances[bankSettlement], "fee transaction must drain BANK_SETTLEMENT balance by F")

	// The principal transaction is keyed on the payment identity (PMF02-A1),
	// never on the caller-supplied provider event id passed to activation.
	principalKey := "seller_subscription_payment_" + paymentID.String()
	principalBalances, ok := perTxn[principalKey]
	require.True(t, ok, "subscription revenue must be booked under the payment identity key")
	assert.Equal(t, principal, principalBalances[platformRevenue],
		"principal transaction must raise PLATFORM_REVENUE balance by A only")
	assert.Equal(t, -principal, principalBalances[bankSettlement])
}

// TestProcessSuccessfulPaymentTx_ZeroFeeBooksNoFeeTransaction proves a zero-fee
// method still settles as AmountPaid = gross with no useless fee ledger effect.
func TestProcessSuccessfulPaymentTx_ZeroFeeBooksNoFeeTransaction(t *testing.T) {
	const principal = int64(70000)

	svc, subRepo, ledger, tx, userID, paymentID := newProcessServiceForFeeSplit(
		t, principal, 0, 365,
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
		t, principal+fee, fee, 365,
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
