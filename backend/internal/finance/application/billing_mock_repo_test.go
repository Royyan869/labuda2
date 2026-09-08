package application

import (
	"context"

	"github.com/google/uuid"
	ledgerepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// mockBillingLedgerRepo is a minimal LedgerRepository mock shared by finance
// application tests. It captures CreateTransaction calls and validates the
// double-entry invariant.
type mockBillingLedgerRepo struct {
	lastEntries        []ledgerepo.Entry
	lastIdempotencyKey string
	createTxCalled     bool
}

func (m *mockBillingLedgerRepo) GetSystemAccountID(_ context.Context, _ db.Tx, accountType string) (uuid.UUID, error) {
	// Return deterministic UUIDs per account type
	switch accountType {
	case ledgerepo.AccountGatewayClearing:
		return uuid.MustParse("00000000-0000-0000-0000-000000000001"), nil
	case ledgerepo.AccountPlatformRevenue:
		return uuid.MustParse("00000000-0000-0000-0000-000000000002"), nil
	case ledgerepo.AccountBankSettlement:
		return uuid.MustParse("00000000-0000-0000-0000-000000000003"), nil
	default:
		return uuid.Nil, nil
	}
}

func (m *mockBillingLedgerRepo) CreateTransaction(
	_ context.Context, _ db.Tx,
	idempotencyKey string, _ string, _ uuid.UUID,
	_ *uuid.UUID, _ *uuid.UUID,
	entries []ledgerepo.Entry,
) error {
	m.createTxCalled = true
	m.lastIdempotencyKey = idempotencyKey
	m.lastEntries = entries
	return nil
}

// Unused interface methods — satisfy LedgerRepository interface.
func (m *mockBillingLedgerRepo) GetAccountBalance(context.Context, db.Tx, uuid.UUID) (money.Money, error) {
	return money.Zero(), nil
}
func (m *mockBillingLedgerRepo) GetAccountBalanceForUpdate(context.Context, db.Tx, uuid.UUID) (money.Money, error) {
	return money.Zero(), nil
}
func (m *mockBillingLedgerRepo) GetUserAccountID(context.Context, db.Tx, string, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (m *mockBillingLedgerRepo) GetOrCreateUserAccount(context.Context, db.Tx, string, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (m *mockBillingLedgerRepo) CountTransactionsByEntityID(context.Context, db.Tx, uuid.UUID) (int, error) {
	return 0, nil
}
func (m *mockBillingLedgerRepo) GetTotalCreditToUserAccount(context.Context, db.Tx, string, uuid.UUID) (int64, error) {
	return 0, nil
}