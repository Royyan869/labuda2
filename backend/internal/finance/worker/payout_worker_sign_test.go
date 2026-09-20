package worker

// payout_worker_sign_test.go
//
// PROOF TEST: C5.3 sign authority
//
// Verifies that PayoutWorker.markSubmissionFailed(Permanent) emits the
// canonical liability restoration pair:
//   WITHDRAWAL_COMMITTED  +amount (DR decreases)
//   SELLER_PAYABLE        -amount (CR increases)
// An inverted pair also sums to zero but must FAIL this test.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// capturedEntry is one INSERT INTO ledger_entries captured from Exec.
type capturedEntry struct {
	AccountID   uuid.UUID
	EntryType   string // "debit" or "credit"
	Amount      int64  // stored positive (ledger_entries.amount)
	BalanceAfter int64
}

// signSpyTx captures entries with DISTINCT account IDs so the inverted
// pair (both Dr/Cr swapped) is distinguishable. The old failureSpyTx
// collapsed both lookups into one UUID, hiding sign inversion.
type signSpyTx struct {
	withdrawalID uuid.UUID
	sellerID     uuid.UUID
	amount       int64
	status       repository.WithdrawalStatus

	committedID uuid.UUID
	payableID   uuid.UUID

	CapturedIdemKey string
	CapturedEntries []capturedEntry
	CapturedReferenceType string
}

func (s *signSpyTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "FROM withdrawals"):
		return &fspyWithdrawalRow{
			id:     s.withdrawalID,
			seller: s.sellerID,
			amount: s.amount,
			status: s.status,
		}
	case strings.Contains(sql, "idempotency_key"):
		if len(args) > 0 {
			if k, ok := args[0].(string); ok {
				s.CapturedIdemKey = k
			}
		}
		// also capture reference_type when INSERT path is via QueryRow? No,
		// CreateTransaction Idempotency SELECT only has key. Reference type
		// is captured via Exec INSERT INTO ledger_transactions below.
		return &fspyNoRowsRow{}

	case strings.Contains(sql, "SELECT account_type FROM financial_accounts WHERE id"):
		// LedgerRepository accountTypes lookup: args[0] is accountID uuid.
		var acctType string
		if len(args) > 0 {
			if id, ok := args[0].(uuid.UUID); ok {
				if id == s.committedID {
					acctType = "WITHDRAWAL_COMMITTED"
				} else if id == s.payableID {
					acctType = "SELLER_PAYABLE"
				}
			}
		}
		return &singleStringRow{val: acctType}

	case strings.Contains(sql, "financial_accounts") && strings.Contains(sql, "account_type"):
		// GetSystemAccountID / GetOrCreateUserAccount lookups.
		if len(args) > 0 {
			if t, ok := args[0].(string); ok {
				switch t {
				case "WITHDRAWAL_COMMITTED":
					return &fspyUUIDRow{id: s.committedID}
				case "SELLER_PAYABLE":
					return &fspyUUIDRow{id: s.payableID}
				}
			}
		}
		return &fspyUUIDRow{id: s.committedID}
	default:
		return &fspyErrRow{err: fmt.Errorf("signSpyTx: unhandled QueryRow sql=%q args=%v", sql, args)}
	}
}

func (s *signSpyTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "FROM financial_accounts") && strings.Contains(sql, "FOR UPDATE") && strings.Contains(sql, "balance") {
		// SELECT id, balance FROM financial_accounts WHERE id = ANY($1) FOR UPDATE
		// Return both accounts: committed with balance=amount (funds are committed), payable 0.
		return &signSpyAccountRows{
			ids:      []uuid.UUID{s.committedID, s.payableID},
			balances: []int64{s.amount, 0},
		}, nil
	}
	if strings.Contains(sql, "financial_accounts") {
		// Any other financial_accounts query that expects rows (should not happen via Query)
		return &fspyEmptyRows{}, nil
	}
	return &fspyEmptyRows{}, nil
}

func (s *signSpyTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(sql, "INSERT INTO ledger_transactions"):
		// args: $1 id, $2 idempotency_key, $3 reference_type, $4 reference_id ...
		if len(args) >= 3 {
			if rt, ok := args[2].(string); ok {
				s.CapturedReferenceType = rt
			}
			if k, ok := args[1].(string); ok && s.CapturedIdemKey == "" {
				s.CapturedIdemKey = k
			}
		}
		return pgconn.NewCommandTag("INSERT 0 1"), nil

	case strings.Contains(sql, "INSERT INTO ledger_entries"):
		// args: $1 id, $2 transaction_id, $3 account_id, $4 entry_type, $5 amount, $6 balance_after, $7 created_at
		if len(args) >= 6 {
			var acct uuid.UUID
			var eType string
			var amt int64
			if v, ok := args[2].(uuid.UUID); ok {
				acct = v
			}
			if v, ok := args[3].(string); ok {
				eType = v
			}
			if v, ok := args[4].(int64); ok {
				amt = v
			}
			var bal int64
			if v, ok := args[5].(int64); ok {
				bal = v
			}
			s.CapturedEntries = append(s.CapturedEntries, capturedEntry{
				AccountID:    acct,
				EntryType:    eType,
				Amount:       amt,
				BalanceAfter: bal,
			})
		}
		return pgconn.NewCommandTag("INSERT 0 1"), nil

	case strings.Contains(sql, "UPDATE withdrawals") && strings.Contains(sql, "status IN"):
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.Contains(sql, "UPDATE withdrawals"):
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.Contains(sql, "UPDATE financial_accounts"):
		return pgconn.NewCommandTag("UPDATE 1"), nil

	default:
		// INSERT ledger_transactions unique violation is not triggered here;
		// QueryRow NoRows path means insert proceeds.
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
}

func (s *signSpyTx) Commit(_ context.Context) error   { return nil }
func (s *signSpyTx) Rollback(_ context.Context) error { return nil }

// singleStringRow returns a single text column (account_type).
type singleStringRow struct{ val string }

func (r *singleStringRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if p, ok := dest[0].(*string); ok {
			*p = r.val
		}
	}
	return nil
}

// signSpyAccountRows yields (id, balance) for FOR UPDATE lock.
type signSpyAccountRows struct {
	ids      []uuid.UUID
	balances []int64
	idx      int
}

func (r *signSpyAccountRows) Close()                                       {}
func (r *signSpyAccountRows) Err() error                                   { return nil }
func (r *signSpyAccountRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT 2") }
func (r *signSpyAccountRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *signSpyAccountRows) RawValues() [][]byte                          { return nil }
func (r *signSpyAccountRows) Values() ([]any, error)                       { return nil, nil }
func (r *signSpyAccountRows) Conn() *pgx.Conn                              { return nil }
func (r *signSpyAccountRows) Next() bool {
	return r.idx < len(r.ids)
}
func (r *signSpyAccountRows) Scan(dest ...any) error {
	if r.idx >= len(r.ids) {
		return fmt.Errorf("no more rows")
	}
	*dest[0].(*uuid.UUID) = r.ids[r.idx]
	*dest[1].(*int64) = r.balances[r.idx]
	r.idx++
	return nil
}

// signSpyTransactor injects signSpyTx into worker.WithTx.
type signSpyTransactor struct{ spy *signSpyTx }

func (t *signSpyTransactor) WithTx(_ context.Context, fn func(db.Tx) error) error {
	return fn(t.spy)
}

func newWorkerForSignSpy(spy *signSpyTx) *PayoutWorker {
	return &PayoutWorker{
		db:           &signSpyTransactor{spy: spy},
		withdrawRepo: repository.NewWithdrawRepository(),
		ledgerRepo:   repository.NewLedgerRepository(),
		log:          zap.NewNop(),
		metrics:      NewPayoutMetrics(zap.NewNop()),
	}
}

// TestMarkSubmissionFailed_Permanent_EmitsCanonicalRestorationSign is the
// MINIMUM proof that the worker no longer emits the inverted pair.
// Both entries are captured via distinct account IDs; an inverted pair
// also sums to zero but fails the per-account signed assertion.
func TestMarkSubmissionFailed_Permanent_EmitsCanonicalRestorationSign(t *testing.T) {
	withdrawalID := uuid.New()
	sellerID := uuid.New()
	committedID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	payableID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	amount := int64(100_000)

	spy := &signSpyTx{
		withdrawalID: withdrawalID,
		sellerID:     sellerID,
		amount:       amount,
		status:       repository.WithdrawalStatusSubmitted,
		committedID:  committedID,
		payableID:    payableID,
	}

	w := newWorkerForSignSpy(spy)

	err := w.markSubmissionFailed(context.Background(), withdrawalID, ErrorTypePermanent, "permanent gateway rejection")
	require.NoError(t, err, "markSubmissionFailed(Permanent) must succeed with sign spy")

	// 1. Idempotency key & reference_type convergence (same as webhook).
	wantKey := fmt.Sprintf(gatewayRestoreKeyFmt, withdrawalID.String())
	assert.Equal(t, wantKey, spy.CapturedIdemKey, "idempotency key must be withdrawal_gateway_restore_<id>")
	assert.Equal(t, "WITHDRAWAL_FAIL_RETURN", spy.CapturedReferenceType, "reference_type must be WITHDRAWAL_FAIL_RETURN")

	// 2. Exactly two ledger entries.
	require.Len(t, spy.CapturedEntries, 2, "must emit exactly 2 ledger entries (WC + SP)")

	// 3. Locate entries by account ID (distinguishes inverted pair).
	var wcEntry *capturedEntry
	var spEntry *capturedEntry
	for i := range spy.CapturedEntries {
		e := &spy.CapturedEntries[i]
		if e.AccountID == committedID {
			wcEntry = e
		} else if e.AccountID == payableID {
			spEntry = e
		}
	}
	require.NotNil(t, wcEntry, "must have entry for WITHDRAWAL_COMMITTED %s", committedID)
	require.NotNil(t, spEntry, "must have entry for SELLER_PAYABLE %s", payableID)

	// 4-5. Canonical signed pair: WC +amount (debit), SP -amount would be credit.
	//    ledger_entries stores entry_type + positive amount, while
	//    LedgerRepository derives entry_type from Amount sign:
	//    Amount>0 => debit, Amount<0 => credit (ledger_repository.go:201).
	//    Our spy captures the INSERT's entry_type/amount directly, so assert
	//    both representations are consistent.
	//
	//    WC: entry_type debit, amount == +amount  -> newBalance = old - amount (decreases)
	//    SP: entry_type credit, amount == +amount -> newBalance = old + amount (increases) but stored as credit
	//    However ledger_repository maps Amount sign to entry_type:
	//      Amount>0 => entry_type debit, stored amount = Amount
	//      Amount<0 => entry_type credit, stored amount = -Amount
	//    So WC Amount>0 => debit, SP Amount<0 => credit with stored +amount.
	//    Assert the *signed* Amount intent via entry_type+amount combination.
	assert.Equal(t, "debit", wcEntry.EntryType, "WITHDRAWAL_COMMITTED must be debit (DR decreases liability)")
	assert.Equal(t, amount, wcEntry.Amount, "WITHDRAWAL_COMMITTED stored amount must be +amount")

	assert.Equal(t, "credit", spEntry.EntryType, "SELLER_PAYABLE must be credit (CR increases liability)")
	assert.Equal(t, amount, spEntry.Amount, "SELLER_PAYABLE stored amount must be +amount (credit stores positive)")

	// 6. Signed sum zero via Amount sign reconstruction.
	//    Reconstruct signed Amount: debit => +Amount, credit => -Amount.
	wcSigned := wcEntry.Amount
	if wcEntry.EntryType == "credit" {
		wcSigned = -wcSigned
	}
	spSigned := spEntry.Amount
	if spEntry.EntryType == "credit" {
		spSigned = -spSigned
	}
	assert.Equal(t, int64(amount), wcSigned, "WC signed amount must be +amount")
	assert.Equal(t, int64(-amount), spSigned, "SP signed amount must be -amount")
	assert.Equal(t, int64(0), wcSigned+spSigned, "signed sum must be zero (but per-account check already proves not inverted)")

	// 7. Explicitly guard against the old bug: inverted pair would have
	//    WC credit / SP debit.
	assert.NotEqual(t, "credit", wcEntry.EntryType, "WC must not be credit — inverted bug is WC -amount (credit)")
	assert.NotEqual(t, "debit", spEntry.EntryType, "SP must not be debit — inverted bug is SP +amount (debit)")

	// BalanceAfter is account balance, not timestamp; no sanity check needed.
}
