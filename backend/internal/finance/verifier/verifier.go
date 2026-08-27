package verifier

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/finance"
)

type Account struct {
	ID          uuid.UUID
	UserID      *uuid.UUID
	AccountType string
	Balance     int64
}

type LedgerTransaction struct {
	ID             uuid.UUID
	IdempotencyKey string
	ReferenceType  string
	ReferenceID    *uuid.UUID
	OrderID        *uuid.UUID
	PaymentID      *uuid.UUID
	TotalDebit     int64
	TotalCredit    int64
	CreatedAt      int64
}

type LedgerEntry struct {
	ID            uuid.UUID
	TransactionID uuid.UUID
	AccountID     uuid.UUID
	EntryType     string
	Amount        int64
	BalanceAfter  int64
	CreatedAt     int64
	RowOrder      string
	EntrySequence *int64
}

type Payment struct {
	ID            uuid.UUID
	ReferenceType string
	ReferenceID   uuid.UUID
	Status        string
	GrossAmount   int64
	TransactionID *string
	CreatedAt     time.Time
}

type Order struct {
	ID                   uuid.UUID
	BuyerID              uuid.UUID
	SellerID             uuid.UUID
	Status               string
	EscrowStatus         string
	Subtotal             int64
	ShippingTotal        int64
	CommissionAmount     int64
	TotalBeforeCoinsAmount int64 // canonical buyer-funded escrow base = PD + S
	RefundedAmount       int64
	PaymentID            *uuid.UUID
}

type Withdrawal struct {
	ID       uuid.UUID
	SellerID uuid.UUID
	Amount   int64
	Status   string
}

type Refund struct {
	ID                uuid.UUID
	OrderID           uuid.UUID
	BuyerID           uuid.UUID
	SellerID          uuid.UUID
	Status            string
	GatewayStatus     string
	RequestedAmount   int64
	FinalRefundAmount *int64
	RefundedAt        *time.Time
	CreatedAt         time.Time
}

type DisputeFreeze struct {
	ID           uuid.UUID
	DisputeID    uuid.UUID
	OrderID      uuid.UUID
	SellerID     uuid.UUID
	FrozenAmount int64
	Status       string // "active" | "released"
}

type Wallet struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	AvailableBalance int64
	HeldBalance      int64
}

type OutboxEvent struct {
	ID          uuid.UUID
	AggregateID uuid.UUID
	EventType   string
	Status      string
	Archive     bool
}

type Snapshot struct {
	Accounts       []Account
	Transactions   []LedgerTransaction
	Entries        []LedgerEntry
	Payments       []Payment
	Orders         []Order
	Withdrawals    []Withdrawal
	Refunds        []Refund
	DisputeFreezes []DisputeFreeze
	Wallets        []Wallet
	OutboxEvents   []OutboxEvent
}

type Finding struct {
	Section string
	Code    string
	Detail  string
	Class   string
	Level   string
}

type SectionResult struct {
	Name     string
	Passed   bool
	Findings []Finding
}

type Report struct {
	Mode     Mode
	Sections []SectionResult
}

func (r Report) HasFailures() bool {
	for _, s := range r.Sections {
		if !s.Passed {
			return true
		}
	}
	return false
}

func (r Report) Format(title string) string {
	var b strings.Builder
	b.WriteString("============================================================\n")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString("============================================================\n")
	b.WriteString(fmt.Sprintf("MODE: %s\n", r.Mode))
	for _, s := range r.Sections {
		status := "PASS"
		if !s.Passed {
			status = "FAIL"
		}
		b.WriteString(fmt.Sprintf("[%s] %s\n", status, s.Name))
		for _, f := range s.Findings {
			b.WriteString(fmt.Sprintf("  - %s [%s/%s]: %s\n", f.Code, f.Class, f.Level, f.Detail))
		}
	}
	if r.HasFailures() {
		b.WriteString("RESULT: FAIL\n")
	} else {
		b.WriteString("RESULT: PASS\n")
	}
	return b.String()
}

type Mode string

const (
	ModeStrict   Mode = "strict"
	ModeForensic Mode = "forensic"
)

func LoadSnapshot(ctx context.Context, pool *pgxpool.Pool) (*Snapshot, error) {
	s := &Snapshot{}
	var err error
	if s.Accounts, err = loadAccounts(ctx, pool); err != nil {
		return nil, err
	}
	if s.Transactions, err = loadTransactions(ctx, pool); err != nil {
		return nil, err
	}
	if s.Entries, err = loadEntries(ctx, pool); err != nil {
		return nil, err
	}
	if s.Payments, err = loadPayments(ctx, pool); err != nil {
		return nil, err
	}
	if s.Orders, err = loadOrders(ctx, pool); err != nil {
		return nil, err
	}
	if s.Withdrawals, err = loadWithdrawals(ctx, pool); err != nil {
		return nil, err
	}
	if s.Refunds, err = loadRefunds(ctx, pool); err != nil {
		return nil, err
	}
	if s.DisputeFreezes, err = loadDisputeFreezes(ctx, pool); err != nil {
		return nil, err
	}
	if s.Wallets, err = loadWallets(ctx, pool); err != nil {
		return nil, err
	}
	if s.OutboxEvents, err = loadOutboxEvents(ctx, pool); err != nil {
		return nil, err
	}
	return s, nil
}

func Verify(snapshot *Snapshot, mode Mode) Report {
	v := newVerifier(snapshot, mode)
	return Report{
		Mode: mode,
		Sections: []SectionResult{
			v.checkDoubleEntryIntegrity(),
			v.checkAccountBalanceIntegrity(),
			v.checkSettlementReleaseInvariants(),
			v.checkWithdrawalInvariants(),
			v.checkRefundInvariants(),
			v.checkDisputeFreezeInvariants(),
			v.checkOutboxCorrelation(),
		},
	}
}

func BrokenFixtureReports() []struct {
	Name   string
	Report Report
} {
	fixtures := []struct {
		name     string
		snapshot *Snapshot
	}{
		{name: "unbalanced-ledger", snapshot: fixtureUnbalancedLedger()},
		{name: "missing-release", snapshot: fixtureMissingRelease()},
		{name: "duplicate-refund-reversal", snapshot: fixtureDuplicateRefundReversal()},
		{name: "negative-balance", snapshot: fixtureNegativeBalance()},
	}
	out := make([]struct {
		Name   string
		Report Report
	}, 0, len(fixtures))
	for _, f := range fixtures {
		out = append(out, struct {
			Name   string
			Report Report
		}{
			Name:   f.name,
			Report: Verify(f.snapshot, ModeStrict),
		})
	}
	return out
}

type verifier struct {
	mode              Mode
	snapshot          *Snapshot
	accountByID       map[uuid.UUID]Account
	txByID            map[uuid.UUID]LedgerTransaction
	entriesByTx       map[uuid.UUID][]LedgerEntry
	entriesByAccount  map[uuid.UUID][]LedgerEntry
	paymentByID       map[uuid.UUID]Payment
	orderByID         map[uuid.UUID]Order
	withdrawalByID    map[uuid.UUID]Withdrawal
	refundByID        map[uuid.UUID]Refund
	paymentCutover    *time.Time
	hasEntrySequence  bool
	outboxByTypeAndID map[string]map[uuid.UUID]int
}

func newVerifier(snapshot *Snapshot, mode Mode) *verifier {
	v := &verifier{
		mode:              mode,
		snapshot:          snapshot,
		accountByID:       make(map[uuid.UUID]Account),
		txByID:            make(map[uuid.UUID]LedgerTransaction),
		entriesByTx:       make(map[uuid.UUID][]LedgerEntry),
		entriesByAccount:  make(map[uuid.UUID][]LedgerEntry),
		paymentByID:       make(map[uuid.UUID]Payment),
		orderByID:         make(map[uuid.UUID]Order),
		withdrawalByID:    make(map[uuid.UUID]Withdrawal),
		refundByID:        make(map[uuid.UUID]Refund),
		outboxByTypeAndID: make(map[string]map[uuid.UUID]int),
	}
	for _, a := range snapshot.Accounts {
		v.accountByID[a.ID] = a
	}
	for _, tx := range snapshot.Transactions {
		v.txByID[tx.ID] = tx
		if tx.ReferenceType == "payment_settlement" {
			ts := time.Unix(tx.CreatedAt, 0).UTC()
			if v.paymentCutover == nil || ts.Before(*v.paymentCutover) {
				cutover := ts
				v.paymentCutover = &cutover
			}
		}
	}
	for _, e := range snapshot.Entries {
		v.entriesByTx[e.TransactionID] = append(v.entriesByTx[e.TransactionID], e)
		v.entriesByAccount[e.AccountID] = append(v.entriesByAccount[e.AccountID], e)
		if e.EntrySequence != nil {
			v.hasEntrySequence = true
		}
	}
	for _, p := range snapshot.Payments {
		v.paymentByID[p.ID] = p
	}
	for _, o := range snapshot.Orders {
		v.orderByID[o.ID] = o
	}
	for _, w := range snapshot.Withdrawals {
		v.withdrawalByID[w.ID] = w
	}
	for _, r := range snapshot.Refunds {
		v.refundByID[r.ID] = r
	}
	for _, e := range snapshot.OutboxEvents {
		if _, ok := v.outboxByTypeAndID[e.EventType]; !ok {
			v.outboxByTypeAndID[e.EventType] = make(map[uuid.UUID]int)
		}
		v.outboxByTypeAndID[e.EventType][e.AggregateID]++
	}
	return v
}

func (v *verifier) addFinding(res *SectionResult, class, code, detail string) {
	level := "error"
	if class == "historical_test_residue" || class == "verifier_false_positive" || class == "out_of_scope_event_expectation" || class == "missing_accounting_primitive" {
		if v.mode == ModeForensic {
			level = "warning"
		}
	}
	res.Findings = append(res.Findings, Finding{
		Section: res.Name,
		Code:    code,
		Detail:  detail,
		Class:   class,
		Level:   level,
	})
}

func (v *verifier) finalize(res *SectionResult) SectionResult {
	res.Passed = true
	for _, f := range res.Findings {
		if f.Level == "error" {
			res.Passed = false
			break
		}
	}
	return *res
}

func (v *verifier) checkDoubleEntryIntegrity() SectionResult {
	res := SectionResult{Name: "Double-Entry Integrity"}
	for _, tx := range v.snapshot.Transactions {
		entries := v.entriesByTx[tx.ID]
		if len(entries) == 0 {
			v.addFinding(&res, "real_invariant_bug", "tx_without_entries", fmt.Sprintf("ledger_transaction=%s idempotency_key=%s", tx.ID, tx.IdempotencyKey))
			continue
		}
		var debit, credit int64
		for _, e := range entries {
			if _, ok := v.accountByID[e.AccountID]; !ok {
				v.addFinding(&res, "real_invariant_bug", "orphan_entry_account", fmt.Sprintf("ledger_entry=%s transaction=%s missing_account=%s", e.ID, e.TransactionID, e.AccountID))
			}
			switch e.EntryType {
			case "debit":
				debit += e.Amount
			case "credit":
				credit += e.Amount
			default:
				v.addFinding(&res, "real_invariant_bug", "invalid_entry_type", fmt.Sprintf("ledger_entry=%s transaction=%s entry_type=%s", e.ID, e.TransactionID, e.EntryType))
			}
		}
		if debit != credit {
			v.addFinding(&res, "real_invariant_bug", "unbalanced_transaction", fmt.Sprintf("ledger_transaction=%s debit=%d credit=%d", tx.ID, debit, credit))
		}
		if tx.TotalDebit != 0 || tx.TotalCredit != 0 {
			if tx.TotalDebit != debit || tx.TotalCredit != credit {
				v.addFinding(&res, "real_invariant_bug", "stored_totals_mismatch", fmt.Sprintf("ledger_transaction=%s stored_debit=%d stored_credit=%d actual_debit=%d actual_credit=%d", tx.ID, tx.TotalDebit, tx.TotalCredit, debit, credit))
			}
		}
	}
	for _, e := range v.snapshot.Entries {
		if _, ok := v.txByID[e.TransactionID]; !ok {
			v.addFinding(&res, "real_invariant_bug", "orphan_entry_transaction", fmt.Sprintf("ledger_entry=%s missing_transaction=%s", e.ID, e.TransactionID))
		}
	}
	return v.finalize(&res)
}

func (v *verifier) checkAccountBalanceIntegrity() SectionResult {
	res := SectionResult{Name: "Account Balance Integrity"}
	for _, account := range v.snapshot.Accounts {
		if account.Balance < 0 {
			v.addFinding(&res, "real_invariant_bug", "negative_account_balance", fmt.Sprintf("account=%s type=%s balance=%d", account.ID, account.AccountType, account.Balance))
		}
		entries := append([]LedgerEntry(nil), v.entriesByAccount[account.ID]...)
		openingBalance, openingClass, ok := v.openingBalanceForAccount(account)
		if !ok {
			v.addFinding(&res, openingClass, "opening_balance_unknown", fmt.Sprintf("account=%s type=%s cannot derive canonical opening balance", account.ID, account.AccountType))
			continue
		}
		ambiguousOrdering := v.accountOrderingAmbiguous(entries)
		if ambiguousOrdering && !v.accountHasUsableSequence(entries) {
			v.addFinding(&res, "missing_accounting_primitive", "account_ordering_ambiguous", fmt.Sprintf("account=%s type=%s has duplicate created_at values and no ledger entry sequence", account.ID, account.AccountType))
			continue
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].EntrySequence != nil && entries[j].EntrySequence != nil && *entries[i].EntrySequence != *entries[j].EntrySequence {
				return *entries[i].EntrySequence < *entries[j].EntrySequence
			}
			if entries[i].CreatedAt != entries[j].CreatedAt {
				return entries[i].CreatedAt < entries[j].CreatedAt
			}
			return entries[i].RowOrder < entries[j].RowOrder
		})
		running := openingBalance
		for _, e := range entries {
			signed := signedAmount(e)
			running += signed
			if e.BalanceAfter != running {
				class := "real_invariant_bug"
				if openingClass == "missing_accounting_primitive" {
					class = openingClass
				}
				v.addFinding(&res, class, "balance_after_mismatch", fmt.Sprintf("account=%s entry=%s expected_balance_after=%d actual_balance_after=%d", account.ID, e.ID, running, e.BalanceAfter))
			}
			if e.BalanceAfter < 0 {
				v.addFinding(&res, "real_invariant_bug", "negative_balance_after", fmt.Sprintf("account=%s entry=%s balance_after=%d", account.ID, e.ID, e.BalanceAfter))
			}
		}
		if running != account.Balance {
			v.addFinding(&res, openingClass, "account_replay_mismatch", fmt.Sprintf("account=%s type=%s opening_balance=%d stored_balance=%d replayed_balance=%d", account.ID, account.AccountType, openingBalance, account.Balance, running))
		}
	}
	return v.finalize(&res)
}

func (v *verifier) checkSettlementReleaseInvariants() SectionResult {
	res := SectionResult{Name: "Settlement / Release Invariants"}

	paymentSettlementByPayment := make(map[uuid.UUID]int)
	orderReleaseByOrder := make(map[uuid.UUID]int)
	gatewayAccountIDs := v.systemAccountIDs(finance.AccountGatewayClearing)

	for _, tx := range v.snapshot.Transactions {
		switch tx.ReferenceType {
		case "payment_settlement":
			if tx.PaymentID != nil {
				paymentSettlementByPayment[*tx.PaymentID]++
			}
		case "order_release":
			if tx.OrderID != nil {
				orderReleaseByOrder[*tx.OrderID]++
			}
		}
	}

	for _, p := range v.snapshot.Payments {
		if p.ReferenceType != "order" {
			continue
		}
		if p.Status == "settlement" || p.Status == "capture" {
			if paymentSettlementByPayment[p.ID] != 1 {
				class := "real_invariant_bug"
				if v.paymentSettlementIsLegacyResidue(p) {
					class = "historical_test_residue"
				}
				v.addFinding(&res, class, "payment_settlement_count", fmt.Sprintf("payment=%s status=%s settlement_tx_count=%d", p.ID, p.Status, paymentSettlementByPayment[p.ID]))
			}
		}
	}

	for _, o := range v.snapshot.Orders {
		if o.Status == "completed" || o.EscrowStatus == "released" {
			if orderReleaseByOrder[o.ID] != 1 {
				v.addFinding(&res, "real_invariant_bug", "order_release_count", fmt.Sprintf("order=%s status=%s escrow_status=%s release_tx_count=%d", o.ID, o.Status, o.EscrowStatus, orderReleaseByOrder[o.ID]))
			}
		}
	}

	type orderGatewayTally struct {
		settlement int64
		release    int64
		refund     int64
		actual     int64
	}
	tallies := make(map[uuid.UUID]*orderGatewayTally)
	for _, tx := range v.snapshot.Transactions {
		if tx.OrderID == nil {
			continue
		}
		t := tallies[*tx.OrderID]
		if t == nil {
			t = &orderGatewayTally{}
			tallies[*tx.OrderID] = t
		}
		for _, e := range v.entriesByTx[tx.ID] {
			if gatewayAccountIDs[e.AccountID] {
				t.actual += signedAmount(e)
				switch tx.ReferenceType {
				case "payment_settlement":
					t.settlement += signedAmount(e)
				case "order_release":
					t.release += -signedAmount(e)
				case "refund_reversal":
					t.refund += -signedAmount(e)
				}
			}
		}
	}
	for orderID, t := range tallies {
		expected := t.settlement - t.release - t.refund
		if t.actual != expected {
			v.addFinding(&res, "real_invariant_bug", "gateway_clearing_mismatch", fmt.Sprintf("order=%s actual_gateway_delta=%d settlement=%d release=%d refund=%d expected=%d", orderID, t.actual, t.settlement, t.release, t.refund, expected))
		}
	}
	return v.finalize(&res)
}

func (v *verifier) checkWithdrawalInvariants() SectionResult {
	res := SectionResult{Name: "Withdrawal Invariants"}

	requestCount := v.referenceTypeCount("withdrawal_request")
	commitCount := v.referenceTypeCount("withdrawal_commit")
	rejectCount := v.referenceTypeCount("withdrawal_reject")
	restoreCount := v.referenceTypeCount("withdrawal_restore")
	completeCount := v.referenceTypeCount("withdrawal_complete")

	var expectedPending int64
	var expectedCommitted int64

	for _, w := range v.snapshot.Withdrawals {
		switch w.Status {
		case "pending":
			expectedPending += w.Amount
			if requestCount[w.ID] != 1 {
				v.addFinding(&res, "real_invariant_bug", "withdrawal_request_count", fmt.Sprintf("withdrawal=%s status=pending request_tx_count=%d", w.ID, requestCount[w.ID]))
			}
		case "approved":
			expectedCommitted += w.Amount
			if commitCount[w.ID] != 1 {
				v.addFinding(&res, "real_invariant_bug", "withdrawal_commit_count", fmt.Sprintf("withdrawal=%s status=approved commit_tx_count=%d", w.ID, commitCount[w.ID]))
			}
		case "completed":
			if completeCount[w.ID] != 1 {
				v.addFinding(&res, "real_invariant_bug", "withdrawal_complete_count", fmt.Sprintf("withdrawal=%s status=completed complete_tx_count=%d", w.ID, completeCount[w.ID]))
			}
		case "rejected":
			pathCount := rejectCount[w.ID] + restoreCount[w.ID]
			if pathCount != 1 {
				v.addFinding(&res, "real_invariant_bug", "withdrawal_reject_restore_count", fmt.Sprintf("withdrawal=%s status=rejected reject_tx_count=%d restore_tx_count=%d", w.ID, rejectCount[w.ID], restoreCount[w.ID]))
			}
		case "REQUESTED", "PROCESSING", "SUBMITTED", "SETTLING", "SETTLED",
			"FAILED_RETRYABLE", "FAILED_FINAL", "PILOT_BLOCKED":
			// Payout-system statuses. Ledger invariants for these require
			// payout worker accounting integration (not yet activated).
			// Classified as informational until accounting is wired.
			v.addFinding(&res, "missing_accounting_primitive", "payout_status_unaudited",
				fmt.Sprintf("withdrawal=%s status=%s â€” payout accounting invariants not yet implemented", w.ID, w.Status))
		default:
			v.addFinding(&res, "real_invariant_bug", "unknown_withdrawal_status", fmt.Sprintf("withdrawal=%s status=%s", w.ID, w.Status))
		}
	}

	pendingBalance := v.systemBalance(finance.AccountWithdrawalPending)
	committedBalance := v.systemBalance(finance.AccountWithdrawalCommitted)
	if pendingBalance != expectedPending {
		v.addFinding(&res, "real_invariant_bug", "withdrawal_pending_balance_mismatch", fmt.Sprintf("WITHDRAWAL_PENDING stored=%d expected=%d", pendingBalance, expectedPending))
	}
	if committedBalance != expectedCommitted {
		v.addFinding(&res, "real_invariant_bug", "withdrawal_committed_balance_mismatch", fmt.Sprintf("WITHDRAWAL_COMMITTED stored=%d expected=%d", committedBalance, expectedCommitted))
	}
	return v.finalize(&res)
}

func (v *verifier) checkRefundInvariants() SectionResult {
	res := SectionResult{Name: "Refund Invariants"}

	refundTxs := v.referenceTypeCount("refund_reversal")
	accountTypeByID := make(map[uuid.UUID]string, len(v.snapshot.Accounts))
	for _, a := range v.snapshot.Accounts {
		accountTypeByID[a.ID] = a.AccountType
	}
	refundsByOrder := make(map[uuid.UUID][]Refund)
	for _, r := range v.snapshot.Refunds {
		refundsByOrder[r.OrderID] = append(refundsByOrder[r.OrderID], r)
	}
	for orderID := range refundsByOrder {
		sort.Slice(refundsByOrder[orderID], func(i, j int) bool {
			li := refundOrderingTime(refundsByOrder[orderID][i])
			lj := refundOrderingTime(refundsByOrder[orderID][j])
			if !li.Equal(lj) {
				return li.Before(lj)
			}
			return refundsByOrder[orderID][i].ID.String() < refundsByOrder[orderID][j].ID.String()
		})
	}
	previousByRefund := make(map[uuid.UUID]int64, len(v.snapshot.Refunds))
	for orderID, refunds := range refundsByOrder {
		var cumulative int64
		for _, refund := range refunds {
			previousByRefund[refund.ID] = cumulative
			if refund.GatewayStatus == "succeeded" || refund.Status == "refunded" || refund.RefundedAt != nil {
				if refund.FinalRefundAmount != nil {
					cumulative += *refund.FinalRefundAmount
				}
			}
		}
		_ = orderID
	}

	for _, r := range v.snapshot.Refunds {
		order, ok := v.orderByID[r.OrderID]
		if !ok {
			res.Findings = append(res.Findings, Finding{
				Section: res.Name,
				Code:    "refund_missing_order",
				Detail:  fmt.Sprintf("refund=%s order=%s", r.ID, r.OrderID),
				Class:   "real_invariant_bug",
				Level:   "error",
			})
			continue
		}
		// CANONICAL BUYER-FUNDED ESCROW BASE: orderGross = total_before_coins_amount
		// = PD + S. orders.escrow_amount is NOT authoritative (never persisted).
		orderGross := order.TotalBeforeCoinsAmount
		if orderGross <= 0 {
			orderGross = order.Subtotal + order.ShippingTotal
		}
		if r.RequestedAmount > orderGross {
			v.addFinding(&res, "real_invariant_bug", "refund_requested_exceeds_order", fmt.Sprintf("refund=%s requested_amount=%d order_escrow=%d", r.ID, r.RequestedAmount, orderGross))
		}
		if r.FinalRefundAmount != nil && *r.FinalRefundAmount > orderGross {
			v.addFinding(&res, "real_invariant_bug", "refund_final_exceeds_order", fmt.Sprintf("refund=%s final_refund_amount=%d order_escrow=%d", r.ID, *r.FinalRefundAmount, orderGross))
		}
		if r.GatewayStatus == "succeeded" || r.Status == "refunded" || r.RefundedAt != nil {
			if refundTxs[r.ID] != 1 {
				v.addFinding(&res, "real_invariant_bug", "refund_reversal_count", fmt.Sprintf("refund=%s status=%s gateway_status=%s refund_reversal_tx_count=%d", r.ID, r.Status, r.GatewayStatus, refundTxs[r.ID]))
				continue
			}
		} else {
			continue
		}

		tx := v.findTransaction("refund_reversal", r.ID)
		if tx == nil {
			continue
		}
		if r.FinalRefundAmount == nil {
			v.addFinding(&res, "real_invariant_bug", "refund_success_missing_final_amount", fmt.Sprintf("refund=%s", r.ID))
			continue
		}
		previouslyRefunded := previousByRefund[r.ID]
		if previouslyRefunded+*r.FinalRefundAmount > orderGross {
			v.addFinding(&res, "real_invariant_bug", "cumulative_refund_exceeds_order", fmt.Sprintf("order=%s refund=%s previous=%d refund_amount=%d order_gross=%d", r.OrderID, r.ID, previouslyRefunded, *r.FinalRefundAmount, orderGross))
			continue
		}
		// CANONICAL COMMISSION CONVERGENCE: the commission denominator is
		// PD = TotalBeforeCoins - Shipping (product-only, discounted buyer
		// base minus shipping), NOT EscrowAmount and NOT a dead discount
		// column. This matches refund_math.go / refund_gateway.go.
		pd := order.TotalBeforeCoinsAmount - order.ShippingTotal
		if pd <= 0 {
			pd = order.Subtotal
		}
		if pd <= 0 {
			v.addFinding(&res, "real_invariant_bug", "order_invalid_pd", fmt.Sprintf("order=%s pd=%d", order.ID, pd))
			continue
		}
		expectedCommissionBefore := verifierProportionalCommissionPD(previouslyRefunded, order.CommissionAmount, pd)
		expectedCommissionAfter := verifierProportionalCommissionPD(previouslyRefunded+*r.FinalRefundAmount, order.CommissionAmount, pd)
		expectedCommissionComponent := expectedCommissionAfter - expectedCommissionBefore
		expectedSellerComponent := *r.FinalRefundAmount - expectedCommissionComponent
		entries := v.entriesByTx[tx.ID]
		var gatewayDelta int64
		var sellerDelta int64
		var platformDelta int64
		var bankDelta int64
		for _, e := range entries {
			switch accountTypeByID[e.AccountID] {
			case finance.AccountGatewayClearing:
				gatewayDelta += signedAmount(e)
			case finance.AccountSellerPayable:
				sellerDelta += signedAmount(e)
			case finance.AccountPlatformRevenue:
				platformDelta += signedAmount(e)
			case finance.AccountBankSettlement:
				bankDelta += signedAmount(e)
			}
		}

		beforeRelease := gatewayDelta != 0
		if beforeRelease {
			if gatewayDelta != -*r.FinalRefundAmount {
				v.addFinding(&res, "real_invariant_bug", "before_release_gateway_mismatch", fmt.Sprintf("refund=%s gateway=%d expected=%d", r.ID, gatewayDelta, -*r.FinalRefundAmount))
			}
			if sellerDelta != 0 || platformDelta != 0 || bankDelta != 0 {
				v.addFinding(&res, "real_invariant_bug", "before_release_touched_seller_or_platform", fmt.Sprintf("refund=%s gateway=%d seller=%d platform=%d bank=%d", r.ID, gatewayDelta, sellerDelta, platformDelta, bankDelta))
			}
		} else {
			expectedSellerDelta := -expectedSellerComponent
			expectedPlatformDelta := -expectedCommissionComponent
			if sellerDelta == 0 && bankDelta == 0 {
				v.addFinding(&res, "real_invariant_bug", "after_release_missing_seller_reversal", fmt.Sprintf("refund=%s seller_delta=0 bank_delta=0", r.ID))
			}
			if platformDelta != expectedPlatformDelta {
				v.addFinding(&res, "real_invariant_bug", "after_release_platform_mismatch", fmt.Sprintf("refund=%s platform_delta=%d expected=%d", r.ID, platformDelta, expectedPlatformDelta))
			}
			if sellerDelta != expectedSellerDelta {
				v.addFinding(&res, "real_invariant_bug", "after_release_seller_mismatch", fmt.Sprintf("refund=%s seller_delta=%d expected=%d", r.ID, sellerDelta, expectedSellerDelta))
			}
		}
	}
	return v.finalize(&res)
}

func refundOrderingTime(r Refund) time.Time {
	if r.RefundedAt != nil {
		return *r.RefundedAt
	}
	return r.CreatedAt
}

// verifierProportionalCommissionPD computes the product-proportional commission
// reversal for a cumulative refunded amount, using the CANONICAL denominator
// PD = Subtotal - Discount. This mirrors refund_math.go's proportionalFloor
// (floor division) so the verifier checks ledger entries against the same
// allocation the refund pipeline actually posts. It is a derived expectation,
// NOT a commission identity — the identity is order.CommissionAmount.
func verifierProportionalCommissionPD(amount int64, orderCommission int64, pd int64) int64 {
	if amount <= 0 || orderCommission <= 0 || pd <= 0 {
		return 0
	}
	return (amount * orderCommission) / pd
}

// checkDisputeFreezeInvariants validates:
// 1. frozen_amount > 0 for every row
// 2. frozen_amount â‰¤ order gross (subtotal + shipping) for known orders
// 3. active freeze count per dispute â‰¤ 1 (UNIQUE enforced by DB, verified here)
// 4. active freezes reduce seller withdrawable (informational warning in forensic mode)
func (v *verifier) checkDisputeFreezeInvariants() SectionResult {
	res := SectionResult{Name: "Dispute Freeze Invariants"}

	orderGrossByID := make(map[uuid.UUID]int64)
	for _, o := range v.snapshot.Orders {
		orderGrossByID[o.ID] = o.Subtotal + o.ShippingTotal
	}

	activeByDispute := make(map[uuid.UUID]int)
	activeFreezesBySeller := make(map[uuid.UUID]int64)

	for _, f := range v.snapshot.DisputeFreezes {
		if f.FrozenAmount <= 0 {
			v.addFinding(&res, "real_invariant_bug", "dispute_freeze_non_positive",
				fmt.Sprintf("freeze=%s dispute=%s amount=%d", f.ID, f.DisputeID, f.FrozenAmount))
		}
		if gross, ok := orderGrossByID[f.OrderID]; ok && f.FrozenAmount > gross {
			v.addFinding(&res, "real_invariant_bug", "dispute_freeze_exceeds_order_gross",
				fmt.Sprintf("freeze=%s dispute=%s frozen=%d order_gross=%d", f.ID, f.DisputeID, f.FrozenAmount, gross))
		}
		if f.Status == "active" {
			activeByDispute[f.DisputeID]++
			activeFreezesBySeller[f.SellerID] += f.FrozenAmount
		}
	}

	for disputeID, count := range activeByDispute {
		if count > 1 {
			v.addFinding(&res, "real_invariant_bug", "dispute_multiple_active_freezes",
				fmt.Sprintf("dispute=%s active_freeze_count=%d", disputeID, count))
		}
	}

	for sellerID, freezeTotal := range activeFreezesBySeller {
		payable := v.userBalance(finance.AccountSellerPayable, sellerID)
		if freezeTotal > payable {
			v.addFinding(&res, "real_invariant_bug", "dispute_freeze_exceeds_payable",
				fmt.Sprintf("seller=%s active_freeze=%d payable_balance=%d", sellerID, freezeTotal, payable))
		}
	}

	return v.finalize(&res)
}

func (v *verifier) checkOutboxCorrelation() SectionResult {
	res := SectionResult{Name: "Outbox Correlation"}
	for _, o := range v.snapshot.Orders {
		if o.Status == "completed" {
			if v.outboxCount("money.released", o.ID) != 1 {
				v.addFinding(&res, "real_invariant_bug", "missing_money_released_event", fmt.Sprintf("order=%s status=completed money.released_count=%d", o.ID, v.outboxCount("money.released", o.ID)))
			}
		}
	}
	for _, r := range v.snapshot.Refunds {
		if r.GatewayStatus == "succeeded" || r.Status == "refunded" || r.RefundedAt != nil {
			if v.outboxCount("money.refund_succeeded", r.ID) != 1 {
				v.addFinding(&res, "real_invariant_bug", "missing_money_refund_succeeded_event", fmt.Sprintf("refund=%s money.refund_succeeded_count=%d", r.ID, v.outboxCount("money.refund_succeeded", r.ID)))
			}
		}
	}
	for _, w := range v.snapshot.Withdrawals {
		switch w.Status {
		case "approved":
			if c := v.outboxCount("withdrawal.approved", w.ID); c != 1 && v.outboxContractObserved("withdrawal.approved") {
				v.addFinding(&res, "out_of_scope_event_expectation", "missing_withdrawal_approved_event", fmt.Sprintf("withdrawal=%s withdrawal.approved_count=%d", w.ID, c))
			}
		case "rejected":
			if c := v.outboxCount("withdrawal.rejected", w.ID); c != 1 && v.outboxContractObserved("withdrawal.rejected") {
				v.addFinding(&res, "out_of_scope_event_expectation", "missing_withdrawal_rejected_event", fmt.Sprintf("withdrawal=%s withdrawal.rejected_count=%d", w.ID, c))
			}
		case "completed":
			if c := v.outboxCount("withdrawal.completed", w.ID); c != 1 && v.outboxContractObserved("withdrawal.completed") {
				v.addFinding(&res, "out_of_scope_event_expectation", "missing_withdrawal_completed_event", fmt.Sprintf("withdrawal=%s withdrawal.completed_count=%d", w.ID, c))
			}
		}
	}
	return v.finalize(&res)
}

func signedAmount(e LedgerEntry) int64 {
	if e.EntryType == "credit" {
		return -e.Amount
	}
	return e.Amount
}

func (v *verifier) systemAccountIDs(accountType string) map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	for _, a := range v.snapshot.Accounts {
		if a.AccountType == accountType && a.UserID == nil {
			out[a.ID] = true
		}
	}
	return out
}

func (v *verifier) systemBalance(accountType string) int64 {
	var total int64
	for _, a := range v.snapshot.Accounts {
		if a.AccountType == accountType && a.UserID == nil {
			total += a.Balance
		}
	}
	return total
}

func (v *verifier) userBalance(accountType string, userID uuid.UUID) int64 {
	for _, a := range v.snapshot.Accounts {
		if a.AccountType == accountType && a.UserID != nil && *a.UserID == userID {
			return a.Balance
		}
	}
	return 0
}

func (v *verifier) referenceTypeCount(referenceType string) map[uuid.UUID]int {
	out := map[uuid.UUID]int{}
	for _, tx := range v.snapshot.Transactions {
		if tx.ReferenceType != referenceType || tx.ReferenceID == nil {
			continue
		}
		out[*tx.ReferenceID]++
	}
	return out
}

func (v *verifier) findTransaction(referenceType string, referenceID uuid.UUID) *LedgerTransaction {
	for _, tx := range v.snapshot.Transactions {
		if tx.ReferenceType == referenceType && tx.ReferenceID != nil && *tx.ReferenceID == referenceID {
			copyTx := tx
			return &copyTx
		}
	}
	return nil
}

func (v *verifier) outboxCount(eventType string, aggregateID uuid.UUID) int {
	if ids, ok := v.outboxByTypeAndID[eventType]; ok {
		return ids[aggregateID]
	}
	return 0
}

func (v *verifier) outboxContractObserved(eventType string) bool {
	ids, ok := v.outboxByTypeAndID[eventType]
	return ok && len(ids) > 0
}

func (v *verifier) openingBalanceForAccount(account Account) (int64, string, bool) {
	if account.UserID != nil {
		return 0, "real_invariant_bug", true
	}
	switch account.AccountType {
	case finance.AccountBankSettlement:
		return 9_000_000_000_000_000, "real_invariant_bug", true
	case finance.AccountPlatformBank:
		// PLATFORM_BANK carries a reserve float representing the platform's
		// own bank holdings, the source of platform-funded buyer benefits (K
		// coin funding). Mirrors BANK_SETTLEMENT's reserve-float opening.
		return 9_000_000_000_000_000, "real_invariant_bug", true
	case finance.AccountGatewayClearing, finance.AccountEscrow, finance.AccountSellerPayable, finance.AccountPlatformRevenue, finance.AccountWithdrawalPending, finance.AccountWithdrawalCommitted, finance.AccountBuyerRefundable, finance.AccountUserServiceCredit, finance.AccountAdRevenue:
		return 0, "real_invariant_bug", true
	default:
		return 0, "missing_accounting_primitive", false
	}
}

func (v *verifier) accountOrderingAmbiguous(entries []LedgerEntry) bool {
	counts := map[int64]int{}
	for _, e := range entries {
		counts[e.CreatedAt]++
		if counts[e.CreatedAt] > 1 {
			return true
		}
	}
	return false
}

func (v *verifier) accountHasUsableSequence(entries []LedgerEntry) bool {
	if len(entries) == 0 {
		return true
	}
	for _, e := range entries {
		if e.EntrySequence == nil {
			return false
		}
	}
	return true
}

func (v *verifier) paymentSettlementIsLegacyResidue(payment Payment) bool {
	if v.paymentCutover == nil {
		return false
	}
	return payment.CreatedAt.Before(*v.paymentCutover)
}

func loadAccounts(ctx context.Context, pool *pgxpool.Pool) ([]Account, error) {
	rows, err := pool.Query(ctx, `SELECT id, user_id, account_type, balance FROM financial_accounts`)
	if err != nil {
		return nil, fmt.Errorf("load financial_accounts: %w", err)
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.UserID, &a.AccountType, &a.Balance); err != nil {
			return nil, fmt.Errorf("scan financial_accounts: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func loadTransactions(ctx context.Context, pool *pgxpool.Pool) ([]LedgerTransaction, error) {
	rows, err := pool.Query(ctx, `SELECT id, idempotency_key, reference_type, reference_id, order_id, payment_id, total_debit, total_credit, created_at FROM ledger_transactions`)
	if err != nil {
		return nil, fmt.Errorf("load ledger_transactions: %w", err)
	}
	defer rows.Close()
	var out []LedgerTransaction
	for rows.Next() {
		var tx LedgerTransaction
		if err := rows.Scan(&tx.ID, &tx.IdempotencyKey, &tx.ReferenceType, &tx.ReferenceID, &tx.OrderID, &tx.PaymentID, &tx.TotalDebit, &tx.TotalCredit, &tx.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ledger_transactions: %w", err)
		}
		out = append(out, tx)
	}
	return out, rows.Err()
}

func loadEntries(ctx context.Context, pool *pgxpool.Pool) ([]LedgerEntry, error) {
	hasSequence, err := columnExists(ctx, pool, "ledger_entries", "entry_sequence")
	if err != nil {
		return nil, err
	}
	query := `SELECT id, transaction_id, account_id, entry_type, amount, balance_after, created_at, ctid::text`
	if hasSequence {
		query += `, entry_sequence`
	} else {
		query += `, NULL::bigint AS entry_sequence`
	}
	query += ` FROM ledger_entries`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load ledger_entries: %w", err)
	}
	defer rows.Close()
	var out []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.EntryType, &e.Amount, &e.BalanceAfter, &e.CreatedAt, &e.RowOrder, &e.EntrySequence); err != nil {
			return nil, fmt.Errorf("scan ledger_entries: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func loadPayments(ctx context.Context, pool *pgxpool.Pool) ([]Payment, error) {
	rows, err := pool.Query(ctx, `SELECT id, reference_type, reference_id, status::text, gross_amount, transaction_id, created_at FROM payments`)
	if err != nil {
		return nil, fmt.Errorf("load payments: %w", err)
	}
	defer rows.Close()
	var out []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.ReferenceType, &p.ReferenceID, &p.Status, &p.GrossAmount, &p.TransactionID, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan payments: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func loadOrders(ctx context.Context, pool *pgxpool.Pool) ([]Order, error) {
	hasPaymentID, err := columnExists(ctx, pool, "orders", "payment_id")
	if err != nil {
		return nil, err
	}
	query := `SELECT id, buyer_id, seller_id, status::text, escrow_status::text, subtotal, shipping_total, commission_amount, total_before_coins_amount, refunded_amount`
	if hasPaymentID {
		query += `, payment_id`
	} else {
		query += `, NULL::uuid AS payment_id`
	}
	query += ` FROM orders`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load orders: %w", err)
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.BuyerID, &o.SellerID, &o.Status, &o.EscrowStatus, &o.Subtotal, &o.ShippingTotal, &o.CommissionAmount, &o.TotalBeforeCoinsAmount, &o.RefundedAmount, &o.PaymentID); err != nil {
			return nil, fmt.Errorf("scan orders: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func loadWithdrawals(ctx context.Context, pool *pgxpool.Pool) ([]Withdrawal, error) {
	hasSellerID, err := columnExists(ctx, pool, "withdrawals", "seller_id")
	if err != nil {
		return nil, err
	}
	query := `SELECT id, `
	if hasSellerID {
		query += `seller_id`
	} else {
		query += `user_id`
	}
	query += `, amount, status::text FROM withdrawals`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load withdrawals: %w", err)
	}
	defer rows.Close()
	var out []Withdrawal
	for rows.Next() {
		var w Withdrawal
		if err := rows.Scan(&w.ID, &w.SellerID, &w.Amount, &w.Status); err != nil {
			return nil, fmt.Errorf("scan withdrawals: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func loadRefunds(ctx context.Context, pool *pgxpool.Pool) ([]Refund, error) {
	hasGatewayStatus, err := columnExists(ctx, pool, "refunds", "gateway_status")
	if err != nil {
		return nil, err
	}
	query := `SELECT id, order_id, buyer_id, seller_id, status::text`
	if hasGatewayStatus {
		query += `, gateway_status`
	} else {
		query += `, ''::text AS gateway_status`
	}
	query += `, requested_amount, final_refund_amount, refunded_at, created_at FROM refunds`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load refunds: %w", err)
	}
	defer rows.Close()
	var out []Refund
	for rows.Next() {
		var r Refund
		if err := rows.Scan(&r.ID, &r.OrderID, &r.BuyerID, &r.SellerID, &r.Status, &r.GatewayStatus, &r.RequestedAmount, &r.FinalRefundAmount, &r.RefundedAt, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan refunds: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func loadDisputeFreezes(ctx context.Context, pool *pgxpool.Pool) ([]DisputeFreeze, error) {
	// Table may not exist before migration 000131 — tolerate absence gracefully.
	rows, err := pool.Query(ctx, `
		SELECT id, dispute_id, order_id, seller_id, frozen_amount, status
		FROM dispute_freezes
	`)
	if err != nil {
		// Migration not yet applied — treat as empty, not an error.
		return nil, nil
	}
	defer rows.Close()
	var out []DisputeFreeze
	for rows.Next() {
		var f DisputeFreeze
		if err := rows.Scan(&f.ID, &f.DisputeID, &f.OrderID, &f.SellerID, &f.FrozenAmount, &f.Status); err != nil {
			return nil, fmt.Errorf("scan dispute_freezes: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func loadWallets(ctx context.Context, pool *pgxpool.Pool) ([]Wallet, error) {
	rows, err := pool.Query(ctx, `SELECT id, user_id, available_balance, held_balance FROM wallets`)
	if err != nil {
		return nil, fmt.Errorf("load wallets: %w", err)
	}
	defer rows.Close()
	var out []Wallet
	for rows.Next() {
		var w Wallet
		if err := rows.Scan(&w.ID, &w.UserID, &w.AvailableBalance, &w.HeldBalance); err != nil {
			return nil, fmt.Errorf("scan wallets: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func loadOutboxEvents(ctx context.Context, pool *pgxpool.Pool) ([]OutboxEvent, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, aggregate_id, event_type, status::text, false AS archive FROM outbox
		UNION ALL
		SELECT id, aggregate_id, event_type, status::text, true AS archive FROM outbox_archive
	`)
	if err != nil {
		return nil, fmt.Errorf("load outbox events: %w", err)
	}
	defer rows.Close()
	var out []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err := rows.Scan(&e.ID, &e.AggregateID, &e.EventType, &e.Status, &e.Archive); err != nil {
			return nil, fmt.Errorf("scan outbox events: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func columnExists(ctx context.Context, pool *pgxpool.Pool, tableName, columnName string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = $1
			  AND column_name = $2
		)
	`, tableName, columnName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check column %s.%s: %w", tableName, columnName, err)
	}
	return exists, nil
}

func fixtureUnbalancedLedger() *Snapshot {
	account1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	account2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	txID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	return &Snapshot{
		Accounts: []Account{
			{ID: account1, AccountType: finance.AccountGatewayClearing, Balance: 100},
			{ID: account2, AccountType: finance.AccountBankSettlement, Balance: 0},
		},
		Transactions: []LedgerTransaction{
			{ID: txID, IdempotencyKey: "fixture-unbalanced", ReferenceType: "payment_settlement", TotalDebit: 0, TotalCredit: 0, CreatedAt: 1},
		},
		Entries: []LedgerEntry{
			{ID: uuid.MustParse("44444444-4444-4444-4444-444444444444"), TransactionID: txID, AccountID: account1, EntryType: "debit", Amount: 100, BalanceAfter: 100, CreatedAt: 1},
			{ID: uuid.MustParse("55555555-5555-5555-5555-555555555555"), TransactionID: txID, AccountID: account2, EntryType: "credit", Amount: 90, BalanceAfter: -90, CreatedAt: 1},
		},
	}
}

func fixtureMissingRelease() *Snapshot {
	orderID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	paymentID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	return &Snapshot{
		Orders: []Order{
			{ID: orderID, BuyerID: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), SellerID: uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"), Status: "completed", EscrowStatus: "released", Subtotal: 1000, ShippingTotal: 100, CommissionAmount: 50, TotalBeforeCoinsAmount: 1100},
		},
		Payments: []Payment{
			{ID: paymentID, ReferenceType: "order", ReferenceID: orderID, Status: "settlement", GrossAmount: 1150},
		},
	}
}

func fixtureDuplicateRefundReversal() *Snapshot {
	refundID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	orderID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	acc1 := uuid.MustParse("12121212-1212-1212-1212-121212121212")
	acc2 := uuid.MustParse("13131313-1313-1313-1313-131313131313")
	tx1 := uuid.MustParse("14141414-1414-1414-1414-141414141414")
	tx2 := uuid.MustParse("15151515-1515-1515-1515-151515151515")
	return &Snapshot{
		Accounts: []Account{
			{ID: acc1, AccountType: finance.AccountGatewayClearing, Balance: 0},
			{ID: acc2, AccountType: finance.AccountBuyerRefundable, Balance: 2000},
		},
		Orders: []Order{
			{ID: orderID, BuyerID: uuid.MustParse("16161616-1616-1616-1616-161616161616"), SellerID: uuid.MustParse("17171717-1717-1717-1717-171717171717"), Status: "refunded", EscrowStatus: "refunded", Subtotal: 1500, ShippingTotal: 300, CommissionAmount: 200, TotalBeforeCoinsAmount: 1800},
		},
		Refunds: []Refund{
			{ID: refundID, OrderID: orderID, BuyerID: uuid.MustParse("16161616-1616-1616-1616-161616161616"), SellerID: uuid.MustParse("17171717-1717-1717-1717-171717171717"), Status: "refunded", GatewayStatus: "succeeded", RequestedAmount: 2000},
		},
		Transactions: []LedgerTransaction{
			{ID: tx1, IdempotencyKey: "refund_reversal_1", ReferenceType: "refund_reversal", ReferenceID: &refundID, OrderID: &orderID, CreatedAt: 1},
			{ID: tx2, IdempotencyKey: "refund_reversal_2", ReferenceType: "refund_reversal", ReferenceID: &refundID, OrderID: &orderID, CreatedAt: 2},
		},
		Entries: []LedgerEntry{
			{ID: uuid.MustParse("18181818-1818-1818-1818-181818181818"), TransactionID: tx1, AccountID: acc1, EntryType: "credit", Amount: 1000, BalanceAfter: -1000, CreatedAt: 1},
			{ID: uuid.MustParse("19191919-1919-1919-1919-191919191919"), TransactionID: tx1, AccountID: acc2, EntryType: "debit", Amount: 1000, BalanceAfter: 1000, CreatedAt: 1},
			{ID: uuid.MustParse("20202020-2020-2020-2020-202020202020"), TransactionID: tx2, AccountID: acc1, EntryType: "credit", Amount: 1000, BalanceAfter: -2000, CreatedAt: 2},
			{ID: uuid.MustParse("21212121-2121-2121-2121-212121212121"), TransactionID: tx2, AccountID: acc2, EntryType: "debit", Amount: 1000, BalanceAfter: 2000, CreatedAt: 2},
		},
	}
}

func fixtureNegativeBalance() *Snapshot {
	accountID := uuid.MustParse("23232323-2323-2323-2323-232323232323")
	return &Snapshot{
		Accounts: []Account{
			{ID: accountID, AccountType: finance.AccountSellerPayable, Balance: -50},
		},
	}
}


