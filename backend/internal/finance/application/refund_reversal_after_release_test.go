package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

type afterReleaseRefundLedgerRepo struct {
	mockBillingLedgerRepo
	userAccounts     map[string]uuid.UUID
	sellerPayableBal int64
}

func newAfterReleaseRefundLedgerRepo(balance int64) *afterReleaseRefundLedgerRepo {
	return &afterReleaseRefundLedgerRepo{
		userAccounts:     make(map[string]uuid.UUID),
		sellerPayableBal: balance,
	}
}

func (m *afterReleaseRefundLedgerRepo) GetOrCreateUserAccount(_ context.Context, _ db.Tx, accountType string, userID uuid.UUID) (uuid.UUID, error) {
	key := accountType + "_" + userID.String()
	if id, ok := m.userAccounts[key]; ok {
		return id, nil
	}
	id := uuid.New()
	m.userAccounts[key] = id
	return id, nil
}

func (m *afterReleaseRefundLedgerRepo) GetAccountBalanceForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (money.Money, error) {
	return money.New(m.sellerPayableBal), nil
}

func (m *afterReleaseRefundLedgerRepo) CountTransactionsByEntityID(_ context.Context, _ db.Tx, _ uuid.UUID) (int, error) {
	return 0, nil
}

func TestRecordRefundReversal_AfterReleaseInsufficientSellerPayable_DoesNotCreateTransaction(t *testing.T) {
	ledger := newAfterReleaseRefundLedgerRepo(50)
	svc := &FinanceService{ledgerRepo: ledger, logger: zap.NewNop()}

	refundID := uuid.New()
	orderID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	_, err := svc.RecordRefundReversal(context.Background(), nil, RecordRefundReversalInput{
		RefundID:            refundID,
		OrderID:             orderID,
		BuyerID:             buyerID,
		SellerID:            sellerID,
		RefundAmount:        100,
		SellerComponent:     70,
		CommissionComponent: 30,
		OrderGross:          100,
		CumulativeRefunded:  100,
		AfterRelease:        true,
	})
	if !errors.Is(err, ErrSellerPayableInsufficient) {
		t.Fatalf("expected ErrSellerPayableInsufficient, got %v", err)
	}
	if ledger.createTxCalled {
		t.Fatal("CreateTransaction must not run when after-release seller payable is insufficient")
	}
	if len(ledger.lastEntries) != 0 {
		t.Fatalf("expected no ledger entries, got %d", len(ledger.lastEntries))
	}
}


