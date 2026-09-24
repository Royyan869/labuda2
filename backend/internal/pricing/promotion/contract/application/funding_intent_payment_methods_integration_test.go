//go:build integration

package application_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	paymentmethodentity "github.com/labuda/backend/internal/commerce/paymentmethod/entity"
	paymentmethodrepo "github.com/labuda/backend/internal/commerce/paymentmethod/infrastructure/repository"
	"github.com/labuda/backend/internal/pricing/promotion/contract/application"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// PROMOTION FUNDING PAYMENT-METHOD DISCLOSURE — READ-ONLY PROOF
//
// The disclosure endpoint answers one question for the mobile seller: "given
// this exact-shortage obligation, which methods can I pay with, and what will
// each one actually cost me?" It must be a pure read of the canonical
// authorities (payment_methods + CalculateFee + the billing obligation) and it
// must never move money, create a payment, or mutate the intent.
// ============================================================================

// seedCanonicalPaymentMethods re-inserts the four canonical wallet rows via
// ON CONFLICT DO NOTHING. testdb truncates payment_methods after every passing
// test while migrations only run once per binary, so each test reseeds.
func seedCanonicalPaymentMethods(t *testing.T, h *intentHarness) {
	t.Helper()
	err := h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		_, err := tx.Exec(context.Background(), `
			INSERT INTO payment_methods
			    (method_code, display_name, enabled, fee_type, flat_amount_rupiah, percent_bps, min_fee_rupiah, max_fee_rupiah,
			     midtrans_channels, sort_order, rate_source, rate_source_note)
			VALUES
			    ('gopay', 'GoPay', true, 'percent', 0, 150, NULL, NULL,
			        ARRAY['gopay'], 10,
			        'public_baseline', 'test seed'),
			    ('ovo', 'OVO', true, 'percent', 0, 150, NULL, NULL,
			        ARRAY['ovo'], 20,
			        'public_baseline', 'test seed'),
			    ('dana', 'DANA', true, 'percent', 0, 150, NULL, NULL,
			        ARRAY['dana'], 25,
			        'public_baseline', 'test seed'),
			    ('shopeepay', 'ShopeePay', true, 'percent', 0, 150, NULL, NULL,
			        ARRAY['shopeepay'], 30,
			        'public_baseline', 'test seed')
			ON CONFLICT (method_code) DO NOTHING
		`)
		return err
	})
	require.NoError(t, err)
}

// newDisclosureHarness builds the canonical funding harness with the real
// payment method authority wired, exactly as boot does.
func newDisclosureHarness(t *testing.T) *intentHarness {
	t.Helper()
	h := newIntentHarness(t)
	seedCanonicalPaymentMethods(t, h)
	h.svc.SetPaymentMethodReader(paymentmethodrepo.NewPaymentMethodRepository())
	return h
}

// readEnabledMethods reads the canonical method rows the disclosure must mirror.
func readEnabledMethods(t *testing.T, h *intentHarness) []paymentmethodentity.Method {
	t.Helper()
	repo := paymentmethodrepo.NewPaymentMethodRepository()
	var methods []paymentmethodentity.Method
	err := h.tdb.WithTx(context.Background(), func(tx db.Tx) error {
		var err error
		methods, err = repo.ListEnabled(context.Background(), tx)
		return err
	})
	require.NoError(t, err)
	return methods
}

// financialSnapshot captures every table the disclosure must NOT touch.
type financialSnapshot struct {
	promoteBalance int64
	ledgerTxCount  int
	paymentCount   int
	intentCount    int
	billingCount   int
	billingStatus  string
	allocationRows int
}

func snapshotFinancials(t *testing.T, h *intentHarness, seller uuid.UUID, billingID string) financialSnapshot {
	t.Helper()
	ctx := context.Background()
	var snap financialSnapshot

	require.NoError(t, h.tdb.Pool().QueryRow(ctx, `
		SELECT COALESCE(balance, 0) FROM financial_accounts
		WHERE account_type = 'PROMOTE_BALANCE' AND user_id = $1 AND holder_id IS NULL`,
		seller).Scan(&snap.promoteBalance))
	require.NoError(t, h.tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM ledger_transactions`).Scan(&snap.ledgerTxCount))
	require.NoError(t, h.tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM payments`).Scan(&snap.paymentCount))
	require.NoError(t, h.tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM promotion_funding_intents`).Scan(&snap.intentCount))
	require.NoError(t, h.tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM billing_transactions`).Scan(&snap.billingCount))
	require.NoError(t, h.tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM financial_accounts WHERE account_type = 'PROMOTION_ALLOCATION'`).Scan(&snap.allocationRows))
	require.NoError(t, h.tdb.Pool().QueryRow(ctx, `SELECT status FROM billing_transactions WHERE id = $1`, billingID).Scan(&snap.billingStatus))
	return snap
}

// ============================================================================
// TEST 1: owner + pending intent → methods + canonical fee/total, zero mutation
// ============================================================================

func TestFundingPaymentMethods_OwnerPendingIntent_MethodsAndExactFee(t *testing.T) {
	h := newDisclosureHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.fundSeller(t, seller, 10_000)

	// budget 45_000, balance 10_000 → exact shortage 35_000
	intent, err := h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 45_000,
		DurationDays: 3,
	})
	require.NoError(t, err)
	require.True(t, intent.PaymentRequired)
	require.Equal(t, int64(35_000), intent.Shortage)

	before := snapshotFinancials(t, h, seller, intent.BillingID)

	result, err := h.svc.PaymentMethods(context.Background(), application.PaymentMethodsInput{
		IntentID: uuid.MustParse(intent.IntentID),
		CallerID: seller,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// The obligation, not the promotion budget, is the disclosed principal.
	assert.Equal(t, int64(35_000), result.ShortageAmount,
		"disclosed principal must be the exact shortage obligation, never budget_rupiah (45_000)")
	assert.Equal(t, "IDR", result.Currency)

	// Every enabled canonical method is disclosed, in canonical order, with the
	// canonical fee F = CalculateFee(shortage, method) and gross = shortage + F.
	canonical := readEnabledMethods(t, h)
	require.NotEmpty(t, canonical)
	require.Len(t, result.Methods, len(canonical))

	for i, method := range canonical {
		option := result.Methods[i]
		assert.Equal(t, method.Code, option.MethodCode, "method order must follow ListEnabled")
		assert.Equal(t, method.DisplayName, option.DisplayName)

		expectedFee, feeErr := paymentmethodentity.CalculateFee(money.New(intent.Shortage), method)
		require.NoError(t, feeErr)
		assert.Equal(t, expectedFee.Int64(), option.ServiceFeeAmount,
			"fee for %s must come from CalculateFee on the shortage obligation", method.Code)
		assert.Equal(t, intent.Shortage+expectedFee.Int64(), option.GrossAmount,
			"gross for %s must be exactly shortage + fee", method.Code)
	}

	// READ-ONLY PROOF: nothing financial moved and nothing was created.
	after := snapshotFinancials(t, h, seller, intent.BillingID)
	assert.Equal(t, before, after,
		"disclosure must not create payments, ledger rows, intents, allocations, or change the balance/billing status")
	assert.Equal(t, int64(10_000), h.balance(t, seller))
	assert.Equal(t, "pending", after.billingStatus)
}

// ============================================================================
// TEST 2: fee parity with the canonical initiation engine
// ============================================================================

func TestFundingPaymentMethods_FeeParityWithInitiationEngine(t *testing.T) {
	h := newDisclosureHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.seedUser(t, seller) // zero balance → full cost is the shortage

	intent, err := h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 15_000,
		DurationDays: 1,
	})
	require.NoError(t, err)
	require.True(t, intent.PaymentRequired)

	// Read the obligation straight from the DB — the exact base
	// CorePaymentHandler.InitiateBillingPayment charges its fee on
	// (`billingPrincipal := billing.GrossAmount`).
	var billingGross int64
	require.NoError(t, h.tdb.Pool().QueryRow(context.Background(),
		`SELECT gross_amount FROM billing_transactions WHERE id = $1`, intent.BillingID,
	).Scan(&billingGross))
	require.Equal(t, intent.Shortage, billingGross)

	result, err := h.svc.PaymentMethods(context.Background(), application.PaymentMethodsInput{
		IntentID: uuid.MustParse(intent.IntentID),
		CallerID: seller,
	})
	require.NoError(t, err)
	assert.Equal(t, billingGross, result.ShortageAmount,
		"disclosure principal must equal billing.gross_amount — the engine's fee base")

	// Recompute what initiation will snapshot, independently of the service.
	engineMethods := readEnabledMethods(t, h)
	require.NotEmpty(t, engineMethods)
	for _, method := range engineMethods {
		engineFee, feeErr := paymentmethodentity.CalculateFee(money.New(billingGross), method)
		require.NoError(t, feeErr)

		var found bool
		for _, option := range result.Methods {
			if option.MethodCode != method.Code {
				continue
			}
			found = true
			assert.Equal(t, engineFee.Int64(), option.ServiceFeeAmount,
				"disclosed fee for %s must equal what initiation will charge", method.Code)
			assert.Equal(t, billingGross+engineFee.Int64(), option.GrossAmount,
				"disclosed gross for %s must equal what initiation will charge", method.Code)
		}
		assert.True(t, found, "method %s must be disclosed", method.Code)
	}
}

// ============================================================================
// TEST 3: non-owner → reject
// ============================================================================

func TestFundingPaymentMethods_NonOwnerRejected(t *testing.T) {
	h := newDisclosureHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.seedUser(t, seller)
	attacker := uuid.New()
	h.seedUser(t, attacker)

	intent, err := h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 15_000,
		DurationDays: 1,
	})
	require.NoError(t, err)

	_, err = h.svc.PaymentMethods(context.Background(), application.PaymentMethodsInput{
		IntentID: uuid.MustParse(intent.IntentID),
		CallerID: attacker,
	})
	require.ErrorIs(t, err, application.ErrFundingIntentNotOwnedByCaller)
}

// ============================================================================
// TEST 4: unknown intent → reject
// ============================================================================

func TestFundingPaymentMethods_UnknownIntentRejected(t *testing.T) {
	h := newDisclosureHarness(t)
	h.seedConfig(t, 7500, 10_000)

	_, err := h.svc.PaymentMethods(context.Background(), application.PaymentMethodsInput{
		IntentID: uuid.New(),
		CallerID: uuid.New(),
	})
	require.ErrorIs(t, err, application.ErrFundingIntentNotFound)
}

// ============================================================================
// TEST 5: settled (non-pending) intent → reject
// ============================================================================

func TestFundingPaymentMethods_NonPendingIntentRejected(t *testing.T) {
	h := newDisclosureHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.seedUser(t, seller)

	intent, err := h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 15_000,
		DurationDays: 1,
	})
	require.NoError(t, err)

	_, err = h.tdb.Pool().Exec(context.Background(),
		`UPDATE billing_transactions SET status = 'paid' WHERE id = $1`, intent.BillingID)
	require.NoError(t, err)

	_, err = h.svc.PaymentMethods(context.Background(), application.PaymentMethodsInput{
		IntentID: uuid.MustParse(intent.IntentID),
		CallerID: seller,
	})
	require.ErrorIs(t, err, application.ErrFundingIntentBillingNotPending)
}

// ============================================================================
// TEST 6: intent with no linked billing → reject
// ============================================================================

func TestFundingPaymentMethods_UnlinkedIntentRejected(t *testing.T) {
	h := newDisclosureHarness(t)
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.seedUser(t, seller)

	intentID := uuid.New()
	_, err := h.tdb.Pool().Exec(context.Background(), `
		INSERT INTO promotion_funding_intents
			(id, seller_id, kind, budget_rupiah, duration_days, city_ids, shortage_amount, billing_transaction_id, created_at)
		VALUES ($1, $2, 'internal', 20000, 2, NULL, 15000, NULL, NOW())`,
		intentID, seller)
	require.NoError(t, err)

	_, err = h.svc.PaymentMethods(context.Background(), application.PaymentMethodsInput{
		IntentID: intentID,
		CallerID: seller,
	})
	require.ErrorIs(t, err, application.ErrFundingIntentNoBilling)
}

// ============================================================================
// TEST 7: no payment-method authority wired → fail closed, never invent
// ============================================================================

func TestFundingPaymentMethods_UnwiredAuthorityFailsClosed(t *testing.T) {
	h := newIntentHarness(t) // deliberately NOT wired with a method reader
	h.seedConfig(t, 7500, 10_000)

	seller := uuid.New()
	h.seedUser(t, seller)

	intent, err := h.svc.CreateFundingIntent(context.Background(), application.CreatePromotionInput{
		SellerID:     seller,
		Kind:         entity.KindInternal,
		BudgetRupiah: 15_000,
		DurationDays: 1,
	})
	require.NoError(t, err)

	_, err = h.svc.PaymentMethods(context.Background(), application.PaymentMethodsInput{
		IntentID: uuid.MustParse(intent.IntentID),
		CallerID: seller,
	})
	require.Error(t, err, "an unwired authority must fail closed rather than invent methods or fees")
}
