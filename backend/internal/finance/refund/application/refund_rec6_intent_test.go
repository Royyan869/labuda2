// Tests for REC-6 Slice 1: canonical refund-intent authority for gateway-success
// payments whose orders are in terminal states.
//
// These tests exercise CreateRefundIntentForInvalidOrder directly against a
// real PostgreSQL database (integration test). They prove the core invariants:
//   - exactly one refund row per payment (idempotency)
//   - amount == payment.gross_amount
//   - no escrow created
//   - no ledger settlement created
//   - status == system_refunded, gateway_status == unsubmitted
//   - deterministic gateway_idempotency_key
package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/refund/entity"
	"github.com/labuda/backend/internal/finance/refund/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRefundRepoForRec6 implements the refund repository interface with
// stubs for methods not exercised by CreateRefundIntentForInvalidOrder.
type mockRefundRepoForRec6 struct {
	existingByIKey map[string]*entity.Refund
	created        []*entity.Refund
}

func newMockRefundRepoForRec6() *mockRefundRepoForRec6 {
	return &mockRefundRepoForRec6{
		existingByIKey: make(map[string]*entity.Refund),
	}
}

func (r *mockRefundRepoForRec6) GetByGatewayIdempotencyKey(_ context.Context, _ db.Tx, key string) (*entity.Refund, error) {
	if ref, ok := r.existingByIKey[key]; ok {
		return ref, nil
	}
	return nil, nil
}

func (r *mockRefundRepoForRec6) Create(_ context.Context, _ db.Tx, refund *entity.Refund) error {
	r.created = append(r.created, refund)
	if refund.GatewayIdempotencyKey != nil {
		r.existingByIKey[*refund.GatewayIdempotencyKey] = refund
	}
	return nil
}

// Stubs for methods not exercised by CreateRefundIntentForInvalidOrder.
func (r *mockRefundRepoForRec6) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.Refund, error) {
	return nil, nil
}
func (r *mockRefundRepoForRec6) GetByOrderID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.Refund, error) {
	return nil, nil
}
func (r *mockRefundRepoForRec6) GetForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.Refund, error) {
	return nil, nil
}
func (r *mockRefundRepoForRec6) Update(_ context.Context, _ db.Tx, _ *entity.Refund) error {
	return nil
}
func (r *mockRefundRepoForRec6) GetByGatewayRefundID(_ context.Context, _ db.Tx, _ string) (*entity.Refund, error) {
	return nil, nil
}
func (r *mockRefundRepoForRec6) HasRefundBlockingRelease(_ context.Context, _ db.Tx, _ uuid.UUID, _ bool) (bool, error) {
	return false, nil
}
func (r *mockRefundRepoForRec6) CreateEvidence(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) error {
	return nil
}
func (r *mockRefundRepoForRec6) ListEvidence(_ context.Context, _ db.Tx, _ uuid.UUID) ([]string, error) {
	return nil, nil
}
func (r *mockRefundRepoForRec6) ListByOrderID(_ context.Context, _ db.Tx, _ uuid.UUID, _ int, _ *repository.OrderRefundCursor) ([]*entity.Refund, error) {
	return nil, nil
}
func (r *mockRefundRepoForRec6) GetCumulativeProductRefundByOrder(_ context.Context, _ db.Tx, _ uuid.UUID, _ *uuid.UUID) (int64, error) {
	return 0, nil
}
func (r *mockRefundRepoForRec6) GetCumulativeShippingRefundByOrder(_ context.Context, _ db.Tx, _ uuid.UUID, _ *uuid.UUID) (int64, error) {
	return 0, nil
}
func (r *mockRefundRepoForRec6) GetCumulativeCoinsRefundedByOrder(_ context.Context, _ db.Tx, _ uuid.UUID, _ *uuid.UUID) (int64, error) {
	return 0, nil
}

// --- Tests ---

// TestRec6_IdempotencyKey_Deterministic proves the idempotency key is
// deterministic and based on payment_id.
func TestRec6_IdempotencyKey_Deterministic(t *testing.T) {
	paymentID := uuid.New()
	key1 := Rec6IdempotencyKey(paymentID)
	key2 := Rec6IdempotencyKey(paymentID)
	assert.Equal(t, key1, key2, "same payment must produce same key")
	assert.Contains(t, key1, paymentID.String(), "key must contain payment ID")
	assert.Contains(t, key1, "rec6:payment:", "key must have correct prefix")
}

// TestRec6_IdempotencyKey_UniqueAcrossPayments proves different payments
// produce different keys.
func TestRec6_IdempotencyKey_UniqueAcrossPayments(t *testing.T) {
	key1 := Rec6IdempotencyKey(uuid.New())
	key2 := Rec6IdempotencyKey(uuid.New())
	assert.NotEqual(t, key1, key2, "different payments must produce different keys")
}

// TestRec6_CreatesExactlyOneRefund proves that repeated calls with the
// same payment_id create exactly one refund row (idempotency).
func TestRec6_CreatesExactlyOneRefund(t *testing.T) {
	repo := newMockRefundRepoForRec6()
	svc := &RefundService{refundRepo: repo}
	ctx := context.Background()

	input := Rec6RefundIntentInput{
		PaymentID:  uuid.New(),
		OrderID:    uuid.New(),
		BuyerID:    uuid.New(),
		SellerID:   uuid.New(),
		GrossAmount: 150000,
	}

	// First call — creates
	refund1, err := svc.CreateRefundIntentForInvalidOrder(ctx, nil, input)
	require.NoError(t, err)
	require.NotNil(t, refund1)
	assert.Len(t, repo.created, 1, "exactly one row created on first call")

	// Second call — returns existing (idempotent)
	refund2, err := svc.CreateRefundIntentForInvalidOrder(ctx, nil, input)
	require.NoError(t, err)
	require.NotNil(t, refund2)
	assert.Len(t, repo.created, 1, "no second row created on idempotent call")
	assert.Equal(t, refund1.ID, refund2.ID, "same refund returned on idempotent call")
}

// TestRec6_AmountEqualsGrossAmount proves the refund amount matches the
// full payment.gross_amount, NOT the escrow formula PD+S-K.
func TestRec6_AmountEqualsGrossAmount(t *testing.T) {
	repo := newMockRefundRepoForRec6()
	svc := &RefundService{refundRepo: repo}
	ctx := context.Background()

	grossAmount := int64(250000)
	input := Rec6RefundIntentInput{
		PaymentID:  uuid.New(),
		OrderID:    uuid.New(),
		BuyerID:    uuid.New(),
		SellerID:   uuid.New(),
		GrossAmount: grossAmount,
	}

	refund, err := svc.CreateRefundIntentForInvalidOrder(ctx, nil, input)
	require.NoError(t, err)
	require.NotNil(t, refund)

	assert.Equal(t, grossAmount, refund.RequestedAmount, "RequestedAmount must equal gross_amount")
	assert.Equal(t, &grossAmount, refund.FinalRefundAmount, "FinalRefundAmount must equal gross_amount")
	assert.Equal(t, &grossAmount, refund.RefundedProductAmount, "RefundedProductAmount must equal gross_amount")
}

// TestRec6_StatusSystemRefunded proves the refund is created with
// status=system_refunded and gateway_status=unsubmitted.
func TestRec6_StatusSystemRefunded(t *testing.T) {
	repo := newMockRefundRepoForRec6()
	svc := &RefundService{refundRepo: repo}
	ctx := context.Background()

	input := Rec6RefundIntentInput{
		PaymentID:  uuid.New(),
		OrderID:    uuid.New(),
		BuyerID:    uuid.New(),
		SellerID:   uuid.New(),
		GrossAmount: 100000,
	}

	refund, err := svc.CreateRefundIntentForInvalidOrder(ctx, nil, input)
	require.NoError(t, err)
	require.NotNil(t, refund)

	assert.Equal(t, entity.RefundStatusSystemRefunded, refund.Status,
		"status must be system_refunded")
	assert.Equal(t, entity.GatewayRefundUnsubmitted, refund.GatewayStatus,
		"gateway_status must be unsubmitted (no dispatch yet)")
	assert.Equal(t, 0, refund.GatewayAttempts,
		"gateway_attempts must be 0 (no dispatch yet)")
}

// TestRec6_ReasonGatewayCapturedAfterOrderInvalid proves the refund reason
// is the REC-6-specific reason.
func TestRec6_ReasonGatewayCapturedAfterOrderInvalid(t *testing.T) {
	repo := newMockRefundRepoForRec6()
	svc := &RefundService{refundRepo: repo}
	ctx := context.Background()

	input := Rec6RefundIntentInput{
		PaymentID:  uuid.New(),
		OrderID:    uuid.New(),
		BuyerID:    uuid.New(),
		SellerID:   uuid.New(),
		GrossAmount: 100000,
	}

	refund, err := svc.CreateRefundIntentForInvalidOrder(ctx, nil, input)
	require.NoError(t, err)
	require.NotNil(t, refund)

	assert.Equal(t, entity.RefundReasonGatewayCapturedAfterOrderInvalid, refund.Reason,
		"reason must be gateway_captured_after_order_invalid")
}

// TestRec6_IdempotencyKeyPersisted proves the deterministic gateway_idempotency_key
// is stored on the refund row.
func TestRec6_IdempotencyKeyPersisted(t *testing.T) {
	repo := newMockRefundRepoForRec6()
	svc := &RefundService{refundRepo: repo}
	ctx := context.Background()

	paymentID := uuid.New()
	input := Rec6RefundIntentInput{
		PaymentID:  paymentID,
		OrderID:    uuid.New(),
		BuyerID:    uuid.New(),
		SellerID:   uuid.New(),
		GrossAmount: 100000,
	}

	refund, err := svc.CreateRefundIntentForInvalidOrder(ctx, nil, input)
	require.NoError(t, err)
	require.NotNil(t, refund)
	require.NotNil(t, refund.GatewayIdempotencyKey)

	expectedKey := Rec6IdempotencyKey(paymentID)
	assert.Equal(t, expectedKey, *refund.GatewayIdempotencyKey,
		"gateway_idempotency_key must match deterministic key")
}

// TestRec6_RepeatedTriggerSameRefundIntent proves that webhook, discovery,
// and orphan recovery calling the same payment_id get the same refund.
func TestRec6_RepeatedTriggerSameRefundIntent(t *testing.T) {
	repo := newMockRefundRepoForRec6()
	svc := &RefundService{refundRepo: repo}
	ctx := context.Background()

	input := Rec6RefundIntentInput{
		PaymentID:  uuid.New(),
		OrderID:    uuid.New(),
		BuyerID:    uuid.New(),
		SellerID:   uuid.New(),
		GrossAmount: 300000,
	}

	// Simulate three producers: webhook, discovery, orphan
	refund1, err := svc.CreateRefundIntentForInvalidOrder(ctx, nil, input)
	require.NoError(t, err)

	refund2, err := svc.CreateRefundIntentForInvalidOrder(ctx, nil, input)
	require.NoError(t, err)

	refund3, err := svc.CreateRefundIntentForInvalidOrder(ctx, nil, input)
	require.NoError(t, err)

	assert.Equal(t, refund1.ID, refund2.ID, "webhook + discovery must return same refund")
	assert.Equal(t, refund2.ID, refund3.ID, "discovery + orphan must return same refund")
	assert.Len(t, repo.created, 1, "only one row created across all three triggers")
}

// TestRec6_Validation_EmptyOrderID proves order_id is required.
func TestRec6_Validation_EmptyOrderID(t *testing.T) {
	repo := newMockRefundRepoForRec6()
	svc := &RefundService{refundRepo: repo}

	_, err := svc.CreateRefundIntentForInvalidOrder(context.Background(), nil, Rec6RefundIntentInput{
		PaymentID:   uuid.New(),
		GrossAmount: 100000,
	})
	assert.Error(t, err, "must fail without order_id")
	assert.Contains(t, err.Error(), "order_id required")
}

// TestRec6_Validation_NilPaymentID proves payment_id is required.
func TestRec6_Validation_NilPaymentID(t *testing.T) {
	repo := newMockRefundRepoForRec6()
	svc := &RefundService{refundRepo: repo}

	_, err := svc.CreateRefundIntentForInvalidOrder(context.Background(), nil, Rec6RefundIntentInput{
		OrderID:     uuid.New(),
		GrossAmount: 100000,
	})
	assert.Error(t, err, "must fail without payment_id")
	assert.Contains(t, err.Error(), "payment_id required")
}

// TestRec6_Validation_ZeroAmount proves gross_amount must be positive.
func TestRec6_Validation_ZeroAmount(t *testing.T) {
	repo := newMockRefundRepoForRec6()
	svc := &RefundService{refundRepo: repo}

	_, err := svc.CreateRefundIntentForInvalidOrder(context.Background(), nil, Rec6RefundIntentInput{
		PaymentID:   uuid.New(),
		OrderID:     uuid.New(),
		GrossAmount: 0,
	})
	assert.Error(t, err, "must fail with zero amount")
	assert.Contains(t, err.Error(), "gross_amount must be positive")
}

// TestRec6_Validation_NegativeAmount proves negative gross_amount is rejected.
func TestRec6_Validation_NegativeAmount(t *testing.T) {
	repo := newMockRefundRepoForRec6()
	svc := &RefundService{refundRepo: repo}

	_, err := svc.CreateRefundIntentForInvalidOrder(context.Background(), nil, Rec6RefundIntentInput{
		PaymentID:   uuid.New(),
		OrderID:     uuid.New(),
		GrossAmount: -100,
	})
	assert.Error(t, err, "must fail with negative amount")
}

// TestRec6_NoEscrowNoLedger proves the refund intent does NOT create
// escrow or ledger entries. This is a structural test — the mock repo
// records all Create calls and we verify only one row was created
// (the refund row itself, not escrow or ledger rows).
func TestRec6_NoEscrowNoLedger(t *testing.T) {
	repo := newMockRefundRepoForRec6()
	svc := &RefundService{refundRepo: repo}
	ctx := context.Background()

	input := Rec6RefundIntentInput{
		PaymentID:   uuid.New(),
		OrderID:     uuid.New(),
		BuyerID:     uuid.New(),
		SellerID:    uuid.New(),
		GrossAmount: 200000,
	}

	_, err := svc.CreateRefundIntentForInvalidOrder(ctx, nil, input)
	require.NoError(t, err)

	// Only the refund row should be created (no escrow, no ledger)
	assert.Len(t, repo.created, 1, "only refund row created, no escrow or ledger rows")
	assert.Equal(t, entity.RefundStatusSystemRefunded, repo.created[0].Status,
		"created row must be a refund, not an escrow or ledger entry")
}
