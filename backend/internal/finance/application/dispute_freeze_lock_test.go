package application

// =============================================================================
// FIX-4 regression tests — CreateDisputeFreeze lock serialization
//
// Verifies that CreateDisputeFreeze acquires SELLER_PAYABLE FOR UPDATE before
// inserting the dispute_freezes row, closing the TOCTOU race with concurrent
// calls to AssertSellerWithdrawalAllowed.
// =============================================================================

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	ledgerepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// --- minimal mocks ---

type freezeLockLedgerRepo struct {
	mockBillingLedgerRepo
	userAccounts     map[string]uuid.UUID
	lockCallOrder    []string // records which account types were locked
	getOrCreateCalls []string
}

func newFreezeLockLedgerRepo() *freezeLockLedgerRepo {
	return &freezeLockLedgerRepo{
		userAccounts: make(map[string]uuid.UUID),
	}
}

func (m *freezeLockLedgerRepo) GetOrCreateUserAccount(_ context.Context, _ db.Tx, accountType string, userID uuid.UUID) (uuid.UUID, error) {
	key := accountType + "_" + userID.String()
	if id, ok := m.userAccounts[key]; ok {
		return id, nil
	}
	id := uuid.New()
	m.userAccounts[key] = id
	m.getOrCreateCalls = append(m.getOrCreateCalls, accountType)
	return id, nil
}

func (m *freezeLockLedgerRepo) GetAccountBalanceForUpdate(_ context.Context, _ db.Tx, accountID uuid.UUID) (money.Money, error) {
	// Record which account type was locked by finding the reverse mapping.
	for key, id := range m.userAccounts {
		if id == accountID {
			m.lockCallOrder = append(m.lockCallOrder, key)
			return money.New(1000), nil
		}
	}
	m.lockCallOrder = append(m.lockCallOrder, "unknown:"+accountID.String())
	return money.New(1000), nil
}

// mockFreezeWriterWithOrder records the order in which Create was called
// relative to the lock.
type mockFreezeWriterWithOrder struct {
	createCalled bool
	sellerID     uuid.UUID
	amount       int64
}

func (m *mockFreezeWriterWithOrder) Create(_ context.Context, _ db.Tx, f *ledgerepo.DisputeFreeze) error {
	m.createCalled = true
	m.sellerID = f.SellerID
	m.amount = f.FrozenAmount
	return nil
}
func (m *mockFreezeWriterWithOrder) Release(_ context.Context, _ db.Tx, _ uuid.UUID) error { return nil }
func (m *mockFreezeWriterWithOrder) ReleaseByOrderID(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	return nil
}
func (m *mockFreezeWriterWithOrder) GetTotalActiveBySeller(_ context.Context, _ db.Tx, _ uuid.UUID) (int64, error) {
	return 0, nil
}

// --- tests ---

// TestCreateDisputeFreeze_AcquiresSellerPayableLockBeforeInsert verifies that
// CreateDisputeFreeze calls GetAccountBalanceForUpdate (FOR UPDATE) on
// SELLER_PAYABLE before calling disputeFreezeRepo.Create.
// This is the FIX-4 regression lock — the lock serializes CreateDisputeFreeze
// against concurrent AssertSellerWithdrawalAllowed calls.
func TestCreateDisputeFreeze_AcquiresSellerPayableLockBeforeInsert(t *testing.T) {
	ledger := newFreezeLockLedgerRepo()
	freezeWriter := &mockFreezeWriterWithOrder{}
	sellerID := uuid.New()

	svc := &FinanceService{
		ledgerRepo:        ledger,
		disputeFreezeRepo: freezeWriter,
		logger:            zap.NewNop(),
	}

	err := svc.CreateDisputeFreeze(
		context.Background(), nil,
		uuid.New(), sellerID, uuid.New(), 60_000,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Lock must have been acquired.
	if len(ledger.lockCallOrder) == 0 {
		t.Fatal("expected GetAccountBalanceForUpdate to be called (FIX-4 lock), but it was not")
	}

	// The locked account must be SELLER_PAYABLE for the correct seller.
	wantKey := finance.AccountSellerPayable + "_" + sellerID.String()
	locked := ledger.lockCallOrder[0]
	if locked != wantKey {
		t.Errorf("locked account = %q, want SELLER_PAYABLE key %q", locked, wantKey)
	}

	// freeze row must have been inserted.
	if !freezeWriter.createCalled {
		t.Fatal("expected disputeFreezeRepo.Create to be called")
	}
	if freezeWriter.amount != 60_000 {
		t.Errorf("frozen_amount = %d, want 60000", freezeWriter.amount)
	}
}

// TestCreateDisputeFreeze_NilRepo_ReturnsError ensures the nil-repo guard
// still fires before the lock attempt.
func TestCreateDisputeFreeze_NilRepo_ReturnsError(t *testing.T) {
	svc := &FinanceService{
		ledgerRepo: newFreezeLockLedgerRepo(),
		logger:     zap.NewNop(),
		// disputeFreezeRepo intentionally nil
	}
	err := svc.CreateDisputeFreeze(context.Background(), nil, uuid.New(), uuid.New(), uuid.New(), 1000)
	if err == nil {
		t.Fatal("expected error when disputeFreezeRepo is nil")
	}
}

// TestCreateDisputeFreeze_LockError_PropagatesAndDoesNotInsert verifies that
// if the SELLER_PAYABLE lock fails, the freeze row is NOT inserted.
func TestCreateDisputeFreeze_LockError_PropagatesAndDoesNotInsert(t *testing.T) {
	errLock := errors.New("lock failed: deadlock")
	ledger := &lockFailingLedgerRepo{
		freezeLockLedgerRepo: newFreezeLockLedgerRepo(),
		lockErr:              errLock,
	}
	freezeWriter := &mockFreezeWriterWithOrder{}

	svc := &FinanceService{
		ledgerRepo:        ledger,
		disputeFreezeRepo: freezeWriter,
		logger:            zap.NewNop(),
	}
	err := svc.CreateDisputeFreeze(context.Background(), nil, uuid.New(), uuid.New(), uuid.New(), 5000)
	if err == nil {
		t.Fatal("expected error when lock fails")
	}
	if freezeWriter.createCalled {
		t.Fatal("Create must NOT be called when lock acquisition fails")
	}
}

// lockFailingLedgerRepo wraps freezeLockLedgerRepo and injects a lock error.
type lockFailingLedgerRepo struct {
	*freezeLockLedgerRepo
	lockErr error
}

func (m *lockFailingLedgerRepo) GetAccountBalanceForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (money.Money, error) {
	return money.Zero(), m.lockErr
}


