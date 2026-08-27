package worker

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	finance "github.com/labuda/backend/internal/finance"
	"github.com/labuda/backend/internal/finance/entity"
	"go.uber.org/zap"
)

// TestCheckTransactionBalance_BalancedDebitCredit verifies that when
// no unbalanced transactions exist (empty result set), zero issues are returned.
func TestCheckTransactionBalance_BalancedDebitCredit(t *testing.T) {
	w := newTestWorker(t)

	tx := &mockTx{
		QueryFunc: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			// No unbalanced transactions → empty result set
			return &mockRows{rows: [][]any{}}, nil
		},
	}

	issues := w.checkTransactionBalance(context.Background(), tx)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for balanced ledger, got %d", len(issues))
	}
}

// TestCheckTransactionBalance_UnbalancedTransaction verifies that a transaction
// with non-zero signed SUM is flagged as CRITICAL.
func TestCheckTransactionBalance_UnbalancedTransaction(t *testing.T) {
	w := newTestWorker(t)
	txID := uuid.New()

	tx := &mockTx{
		QueryFunc: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			return &mockRows{rows: [][]any{
				{txID},
			}}, nil
		},
	}

	issues := w.checkTransactionBalance(context.Background(), tx)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Type != "transaction_balance" {
		t.Errorf("expected type transaction_balance, got %s", issues[0].Type)
	}
	if issues[0].Severity != entity.SeverityReconcileCritical {
		t.Errorf("expected CRITICAL severity, got %s", issues[0].Severity)
	}
}

// TestCheckAccountBalances_MatchesStoredBalance verifies that when ledger net
// movement matches the stored balance, no issues are reported.
func TestCheckAccountBalances_MatchesStoredBalance(t *testing.T) {
	w := newTestWorker(t)
	acctID := uuid.New()

	queryCount := 0
	tx := &mockTx{
		QueryFunc: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "ledger_entries") {
				// One account with net movement of +50000
				return &mockRows{rows: [][]any{
					{acctID, int64(50000)},
				}}, nil
			}
			return &mockRows{}, nil
		},
		QueryRowFunc: func(_ context.Context, sql string, args ...any) pgx.Row {
			queryCount++
			// financial_accounts lookup: balance=50000, type=SELLER_PAYABLE, user_id=nil
			return &mockRowNullable{values: []any{int64(50000), "SELLER_PAYABLE", (*uuid.UUID)(nil)}}
		},
	}

	issues := w.checkAccountBalances(context.Background(), tx)
	for _, issue := range issues {
		t.Logf("unexpected issue: %s — %s", issue.Type, issue.Message)
	}
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for matching balance, got %d", len(issues))
	}
}

// TestCheckAccountBalances_BankSettlementSeedOffset verifies that
// BANK_SETTLEMENT accounts are compared against seed + net movement,
// avoiding a false positive from the bootstrapped initial balance.
func TestCheckAccountBalances_BankSettlementSeedOffset(t *testing.T) {
	w := newTestWorker(t)
	bankAcctID := uuid.New()

	// Net ledger movement: -1,000,000 (credits from settlements draining reserve)
	netMovement := int64(-1_000_000)
	// Stored balance should be seed + net movement
	storedBalance := bankSettlementSeedBalance + netMovement

	tx := &mockTx{
		QueryFunc: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "ledger_entries") {
				return &mockRows{rows: [][]any{
					{bankAcctID, netMovement},
				}}, nil
			}
			return &mockRows{}, nil
		},
		QueryRowFunc: func(_ context.Context, sql string, _ ...any) pgx.Row {
			return &mockRowNullable{values: []any{storedBalance, finance.AccountBankSettlement, (*uuid.UUID)(nil)}}
		},
	}

	issues := w.checkAccountBalances(context.Background(), tx)
	for _, issue := range issues {
		t.Logf("unexpected issue: %s — %s", issue.Type, issue.Message)
	}
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for BANK_SETTLEMENT with seed offset, got %d", len(issues))
	}

	// Now verify that WITHOUT the seed offset, the same data WOULD be flagged
	// (i.e. if stored == seed + movement but we compare against raw movement)
	txBad := &mockTx{
		QueryFunc: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			if strings.Contains(sql, "ledger_entries") {
				return &mockRows{rows: [][]any{
					{bankAcctID, netMovement},
				}}, nil
			}
			return &mockRows{}, nil
		},
		QueryRowFunc: func(_ context.Context, sql string, _ ...any) pgx.Row {
			// Return a balance that matches raw movement (wrong) — should flag mismatch
			return &mockRowNullable{values: []any{netMovement, finance.AccountBankSettlement, (*uuid.UUID)(nil)}}
		},
	}

	issues2 := w.checkAccountBalances(context.Background(), txBad)
	if len(issues2) == 0 {
		t.Fatal("expected mismatch when stored balance != seed + net movement")
	}
}

// mockRowNullable extends mockRow with support for **uuid.UUID (nullable columns).
type mockRowNullable struct {
	values []any
	err    error
}

func (r *mockRowNullable) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) != len(dest) {
		return pgx.ErrNoRows
	}
	for i, v := range r.values {
		switch d := dest[i].(type) {
		case *int64:
			if val, ok := v.(int64); ok {
				*d = val
			}
		case *string:
			if val, ok := v.(string); ok {
				*d = val
			}
		case *uuid.UUID:
			if val, ok := v.(uuid.UUID); ok {
				*d = val
			}
		case **uuid.UUID:
			if v == nil {
				*d = nil
			} else if val, ok := v.(*uuid.UUID); ok {
				*d = val
			}
		default:
			// skip
		}
	}
	return nil
}

func newTestWorker(t *testing.T) *ReconciliationWorkerV2 {
	t.Helper()
	mockDB := &mockDB{}
	mockRepo := NewMockReconciliationRepository()
	mockAlerts := NewMockAlertService()
	return NewReconciliationWorkerV2(
		mockDB,
		zap.NewNop(),
		mockRepo,
		mockAlerts,
		DefaultReconciliationConfigV2(),
	)
}


