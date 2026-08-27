package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	financeApp "github.com/labuda/backend/internal/finance/application"
	financeRepo "github.com/labuda/backend/internal/finance/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ── minimal test doubles ─────────────────────────────────────────────────────

type htTransactor struct {
	fn func(ctx context.Context, fn func(db.Tx) error) error
}

func (t *htTransactor) WithTx(ctx context.Context, fn func(db.Tx) error) error {
	if t.fn != nil {
		return t.fn(ctx, fn)
	}
	return fn(&htEmptyTx{})
}

type htEmptyTx struct{}

func (t *htEmptyTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &htEmptyRows{}, nil
}
func (t *htEmptyTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &htCountRow{count: 0}
}
func (t *htEmptyTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0"), nil
}
func (t *htEmptyTx) Commit(ctx context.Context) error   { return nil }
func (t *htEmptyTx) Rollback(ctx context.Context) error { return nil }

type htFilledTx struct {
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (t *htFilledTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if t.queryFn != nil {
		return t.queryFn(ctx, sql, args...)
	}
	return &htEmptyRows{}, nil
}
func (t *htFilledTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if t.queryRowFn != nil {
		return t.queryRowFn(ctx, sql, args...)
	}
	return &htCountRow{count: 0}
}
func (t *htFilledTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("0"), nil
}
func (t *htFilledTx) Commit(ctx context.Context) error   { return nil }
func (t *htFilledTx) Rollback(ctx context.Context) error { return nil }

type htEmptyRows struct{}

func (r *htEmptyRows) Close()                                       {}
func (r *htEmptyRows) Err() error                                   { return nil }
func (r *htEmptyRows) Next() bool                                   { return false }
func (r *htEmptyRows) Scan(dest ...any) error                       { return nil }
func (r *htEmptyRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("0") }
func (r *htEmptyRows) Fields() []pgconn.FieldDescription            { return nil }
func (r *htEmptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *htEmptyRows) RawValues() [][]byte                          { return nil }
func (r *htEmptyRows) Values() ([]any, error)                       { return nil, nil }
func (r *htEmptyRows) Conn() *pgx.Conn                              { return nil }

type htCountRow struct{ count int64 }

func (r *htCountRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if p, ok := dest[0].(*int64); ok {
			*p = r.count
		}
	}
	return nil
}

// htWithdrawalRows implements pgx.Rows for the 20-column withdrawal scan shape.
type htWithdrawalRows struct {
	data    [][]any
	current int
}

func (r *htWithdrawalRows) Close()     {}
func (r *htWithdrawalRows) Err() error { return nil }
func (r *htWithdrawalRows) Next() bool {
	if r.current < len(r.data) {
		r.current++
		return true
	}
	return false
}
func (r *htWithdrawalRows) Scan(dest ...any) error {
	vals := r.data[r.current-1]
	for i, d := range dest {
		if i >= len(vals) {
			break
		}
		v := vals[i]
		switch ptr := d.(type) {
		case *uuid.UUID:
			*ptr = v.(uuid.UUID)
		case *financeRepo.WithdrawalStatus:
			*ptr = financeRepo.WithdrawalStatus(v.(string))
		case *string:
			*ptr = v.(string)
		case *int64:
			*ptr = v.(int64)
		case *int:
			*ptr = v.(int)
		case *time.Time:
			*ptr = v.(time.Time)
		}
	}
	return nil
}
func (r *htWithdrawalRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("0") }
func (r *htWithdrawalRows) Fields() []pgconn.FieldDescription            { return nil }
func (r *htWithdrawalRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *htWithdrawalRows) RawValues() [][]byte                          { return nil }
func (r *htWithdrawalRows) Values() ([]any, error)                       { return nil, nil }
func (r *htWithdrawalRows) Conn() *pgx.Conn                              { return nil }

// ── helpers ──────────────────────────────────────────────────────────────────

// buildHandlerWithTransactor creates a WithdrawalHandlerUnified backed by a
// real WithdrawService whose db layer is replaced by the given transactor.
// walletService is nil — ListWithdrawals must not touch it.
func buildHandlerWithTransactor(tr financeApp.Transactor) *WithdrawalHandlerUnified {
	svc := financeApp.NewWithdrawService(
		tr,
		financeRepo.NewLedgerRepository(),
		financeRepo.NewWithdrawRepository(),
		nil, nil, nil, nil, nil, nil,
	)
	return NewWithdrawalHandlerUnified(svc, nil, zap.NewNop())
}

func callListWithdrawalsHandler(h *WithdrawalHandlerUnified, sellerID uuid.UUID, queryStr string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	url := "/withdraw/history"
	if queryStr != "" {
		url += "?" + queryStr
	}
	req, _ := nethttp.NewRequest(nethttp.MethodGet, url, nil)
	c.Request = req
	c.Set("userID", sellerID)
	h.ListWithdrawals(c)
	return w
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestListWithdrawals_WalletServiceUnused proves that ListWithdrawals works when
// walletService is nil.  If the handler still called walletService.Get*, this
// test would panic.
func TestListWithdrawals_WalletServiceUnused(t *testing.T) {
	h := buildHandlerWithTransactor(&htTransactor{})
	w := callListWithdrawalsHandler(h, uuid.New(), "")

	if w.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestListWithdrawals_JSONCompatibility checks all JSON key names, types, and
// values that the mobile client depends on.
func TestListWithdrawals_JSONCompatibility(t *testing.T) {
	sellerID := uuid.New()
	withdrawalID := uuid.New()
	created := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	settled := time.Date(2024, 6, 2, 10, 0, 0, 0, time.UTC)

	rows := &htWithdrawalRows{
		data: [][]any{{
			withdrawalID, sellerID, "seller-username", "seller-farm",
			int64(500000), int64(5000), "SETTLED", "idem-key",
			"BCA", "014", "1234567890", "John Doe",
			"WD_REF_001", "", "",
			int64(0), settled.Unix(), int(0),
			created, created,
		}},
	}

	tr := &htTransactor{
		fn: func(ctx context.Context, fn func(db.Tx) error) error {
			return fn(&htFilledTx{
				queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return rows, nil
				},
				queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &htCountRow{count: 1}
				},
			})
		},
	}

	h := buildHandlerWithTransactor(tr)
	w := callListWithdrawalsHandler(h, sellerID, "")

	if w.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var envelope struct {
		Data struct {
			Withdrawals []map[string]interface{} `json:"withdrawals"`
			Total       float64                  `json:"total"`
			Limit       float64                  `json:"limit"`
			Offset      float64                  `json:"offset"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, w.Body.String())
	}

	if len(envelope.Data.Withdrawals) != 1 {
		t.Fatalf("len(withdrawals): want 1, got %d", len(envelope.Data.Withdrawals))
	}
	item := envelope.Data.Withdrawals[0]

	// withdrawal_id preserved
	if item["withdrawal_id"] != withdrawalID.String() {
		t.Errorf("withdrawal_id: want %s, got %v", withdrawalID, item["withdrawal_id"])
	}
	// amount preserved (requested withdrawal amount)
	if item["amount"] != float64(500000) {
		t.Errorf("amount: want 500000, got %v", item["amount"])
	}
	if item["fee_amount"] != float64(5000) {
		t.Errorf("fee_amount: want 5000, got %v", item["fee_amount"])
	}
	// PASS_18H: total_debit_amount equals the requested amount only —
	// the fee is deducted FROM it, never added on top.
	if item["total_debit_amount"] != float64(500000) {
		t.Errorf("total_debit_amount: want 500000, got %v", item["total_debit_amount"])
	}
	// net_payout_amount = amount - fee_amount = what actually reaches the
	// seller's bank account.
	if item["net_payout_amount"] != float64(495000) {
		t.Errorf("net_payout_amount: want 495000, got %v", item["net_payout_amount"])
	}
	// status now uppercase canonical finance status
	if item["status"] != "SETTLED" {
		t.Errorf("status: want SETTLED, got %v", item["status"])
	}
	// reference_code key preserved, mapped from ExternalReferenceID
	if item["reference_code"] != "WD_REF_001" {
		t.Errorf("reference_code: want WD_REF_001, got %v", item["reference_code"])
	}
	// requested_at is RFC3339 from finance created_at
	if item["requested_at"] != "2024-06-01T10:00:00Z" {
		t.Errorf("requested_at: want 2024-06-01T10:00:00Z, got %v", item["requested_at"])
	}
	// processed_at is RFC3339 from finance settled_at
	if item["processed_at"] != "2024-06-02T10:00:00Z" {
		t.Errorf("processed_at: want 2024-06-02T10:00:00Z, got %v", item["processed_at"])
	}
	// bank snapshot fields are additive
	if item["bank_name_snapshot"] != "BCA" {
		t.Errorf("bank_name_snapshot: want BCA, got %v", item["bank_name_snapshot"])
	}
	if item["account_number_snapshot"] != "1234567890" {
		t.Errorf("account_number_snapshot: want 1234567890, got %v", item["account_number_snapshot"])
	}
	// pagination envelope
	if envelope.Data.Total != 1 {
		t.Errorf("total: want 1, got %v", envelope.Data.Total)
	}
	if envelope.Data.Limit != 20 {
		t.Errorf("limit: want 20, got %v", envelope.Data.Limit)
	}
	if envelope.Data.Offset != 0 {
		t.Errorf("offset: want 0, got %v", envelope.Data.Offset)
	}
}

// TestListWithdrawals_ReferenceCodeNilWhenEmpty proves reference_code is null
// (not absent) when ExternalReferenceID is empty.
func TestListWithdrawals_ReferenceCodeNilWhenEmpty(t *testing.T) {
	sellerID := uuid.New()
	withdrawalID := uuid.New()
	created := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)

	rows := &htWithdrawalRows{
		data: [][]any{{
			withdrawalID, sellerID, "seller-username", "seller-farm",
			int64(100000), int64(500000), "REQUESTED", "idem-key",
			"", "", "", "",
			"", "", "", // empty ExternalReferenceID
			int64(0), int64(0), int(0),
			created, created,
		}},
	}

	tr := &htTransactor{
		fn: func(ctx context.Context, fn func(db.Tx) error) error {
			return fn(&htFilledTx{
				queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return rows, nil
				},
				queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &htCountRow{count: 1}
				},
			})
		},
	}

	h := buildHandlerWithTransactor(tr)
	w := callListWithdrawalsHandler(h, sellerID, "")

	if w.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse raw to check null vs absent
	var envelope struct {
		Data struct {
			Withdrawals []map[string]interface{} `json:"withdrawals"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &envelope)

	if len(envelope.Data.Withdrawals) == 0 {
		t.Fatal("expected 1 item")
	}
	item := envelope.Data.Withdrawals[0]

	// reference_code must be present as null (mobile relies on key existing)
	if val, exists := item["reference_code"]; !exists || val != nil {
		t.Errorf("reference_code: want null, got exists=%v val=%v", exists, val)
	}
	// processed_at must be null when settled_at == 0
	if val, exists := item["processed_at"]; !exists || val != nil {
		t.Errorf("processed_at: want null, got exists=%v val=%v", exists, val)
	}
}

// TestListWithdrawals_PaginationPreserved verifies limit/offset params are
// reflected in the response envelope.
func TestListWithdrawals_PaginationPreserved(t *testing.T) {
	h := buildHandlerWithTransactor(&htTransactor{})
	w := callListWithdrawalsHandler(h, uuid.New(), "limit=10&offset=20")

	if w.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var envelope struct {
		Data struct {
			Limit  float64 `json:"limit"`
			Offset float64 `json:"offset"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &envelope)

	if envelope.Data.Limit != 10 {
		t.Errorf("limit: want 10, got %v", envelope.Data.Limit)
	}
	if envelope.Data.Offset != 20 {
		t.Errorf("offset: want 20, got %v", envelope.Data.Offset)
	}
}

// callRequestWithdrawHandler sends POST /withdraw to the handler.
// sellerID nil simulates missing auth context (unauthenticated request).
func callRequestWithdrawHandler(h *WithdrawalHandlerUnified, sellerID *uuid.UUID, bodyJSON string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := nethttp.NewRequest(nethttp.MethodPost, "/withdraw", strings.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	if sellerID != nil {
		c.Set("userID", *sellerID)
	}
	h.RequestWithdraw(c)
	return w
}

// TestRequestWithdraw_MissingUserID_Returns401 proves that the handler rejects
// requests where the auth middleware did not inject a userID into context.
// This is the primary safety net if RequireActiveAccount is somehow bypassed.
func TestRequestWithdraw_MissingUserID_Returns401(t *testing.T) {
	h := buildHandlerWithTransactor(&htTransactor{})
	w := callRequestWithdrawHandler(h, nil, `{"amount": 100000}`)
	if w.Code != nethttp.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRequestWithdraw_InvalidBody_Returns400 proves the handler validates
// the request body before delegating to the service layer.
func TestRequestWithdraw_InvalidBody_Returns400(t *testing.T) {
	h := buildHandlerWithTransactor(&htTransactor{})
	id := uuid.New()
	w := callRequestWithdrawHandler(h, &id, `not valid json`)
	if w.Code != nethttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRequestWithdraw_NilService_Returns500 proves the handler guards against
// a nil WithdrawService (wiring failure at boot time).
func TestRequestWithdraw_NilService_Returns500(t *testing.T) {
	h := NewWithdrawalHandlerUnified(nil, nil, zap.NewNop())
	id := uuid.New()
	w := callRequestWithdrawHandler(h, &id, `{"amount": 100000}`)
	if w.Code != nethttp.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRequestWithdraw_SellerNotVerified_Returns403 proves that when the service
// returns ErrSellerNotVerified (GUARD 1), the handler maps it to HTTP 403.
// This is the primary payout gate: verification approved is required to withdraw,
// regardless of subscription status (doctrine: payout authority ≠ selling authority).
func TestRequestWithdraw_SellerNotVerified_Returns403(t *testing.T) {
	// Build a transactor whose tx returns ErrNoRows for every query, causing
	// IsSellerVerifiedTx to get no verification row → verified=false → ErrSellerNotVerified.
	tr := &htTransactor{
		fn: func(ctx context.Context, fn func(db.Tx) error) error {
			return fn(&htEmptyTx{})
		},
	}
	// The service built by buildHandlerWithTransactor has verificationService=nil,
	// so we need to use NewWithdrawService directly with a stub verification checker
	// that returns not-verified. Account status checker is nil — GUARD 0 would panic.
	// Instead wire an active account checker.
	svc := financeApp.NewWithdrawService(
		tr,
		financeRepo.NewLedgerRepository(),
		financeRepo.NewWithdrawRepository(),
		nil,                             // bankAccountRepo: not reached (fails at GUARD 1)
		nil,                             // roleChecker: unused in RequestWithdrawal
		&htActiveAccountChecker{},       // GUARD 0: always active
		nil,                             // adminAuditLogger
		&htUnverifiedChecker{},          // GUARD 1: always not verified
		nil,                             // outboxRepo
	)
	// canonicalAuthority nil would short-circuit before GUARD 0; set it to a
	// non-nil zero value — GUARD 1 fires before any authority method is called.
	svc.SetCanonicalAuthority(&financeApp.FinanceService{})
	h := NewWithdrawalHandlerUnified(svc, nil, zap.NewNop())

	id := uuid.New()
	w := callRequestWithdrawHandler(h, &id, `{"amount": 100000}`)
	if w.Code != nethttp.StatusForbidden {
		t.Fatalf("expected 403 for unverified seller, got %d: %s", w.Code, w.Body.String())
	}
}

// htActiveAccountChecker satisfies auth.AccountStatusChecker with an always-active response.
type htActiveAccountChecker struct{}

func (h *htActiveAccountChecker) EnsureActive(ctx context.Context, userID uuid.UUID) error {
	return nil
}
func (h *htActiveAccountChecker) IsBanned(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil
}
func (h *htActiveAccountChecker) GetStatus(ctx context.Context, userID uuid.UUID) (string, error) {
	return "active", nil
}

// htUnverifiedChecker satisfies financeApp.WithdrawVerificationChecker with
// an always-unverified response. Simulates a seller who has not yet received
// KYC approval (or whose verification has been suspended/revoked).
type htUnverifiedChecker struct{}

func (h *htUnverifiedChecker) IsSellerVerifiedTx(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (bool, error) {
	return false, nil
}
func (h *htUnverifiedChecker) IsReviewedBankAccountTx(ctx context.Context, tx db.Tx, sellerID, bankAccountID uuid.UUID) (bool, error) {
	return false, nil
}

// TestListWithdrawals_ExpiredSubscriptionSellerCanReadOwnHistory proves that
// the withdrawal history endpoint does not require an active seller subscription.
// Historical finance records (earned balance, past withdrawals) must remain
// visible even after subscription expiry per the payout authority doctrine:
// "Payout authority ≠ Selling authority."
// The handler sources sellerID exclusively from the auth context; no cross-seller
// data leak is possible regardless of subscription status.
func TestListWithdrawals_ExpiredSubscriptionSellerCanReadOwnHistory(t *testing.T) {
	h := buildHandlerWithTransactor(&htTransactor{})
	// Any authenticated sellerID — no subscription check in handler or service read path.
	sellerID := uuid.New()
	w := callListWithdrawalsHandler(h, sellerID, "")
	if w.Code != nethttp.StatusOK {
		t.Fatalf("expired-subscription seller: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestListWithdrawals_RFC3339Timestamps verifies timestamps are ISO-8601 strings.
func TestListWithdrawals_RFC3339Timestamps(t *testing.T) {
	sellerID := uuid.New()
	created := time.Date(2024, 1, 15, 8, 30, 0, 0, time.UTC)

	rows := &htWithdrawalRows{
		data: [][]any{{
			uuid.New(), sellerID, "seller-username", "seller-farm",
			int64(50000), int64(500000), "REQUESTED", "k",
			"", "", "", "", "", "", "",
			int64(0), int64(0), int(0),
			created, created,
		}},
	}

	tr := &htTransactor{
		fn: func(ctx context.Context, fn func(db.Tx) error) error {
			return fn(&htFilledTx{
				queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return rows, nil
				},
				queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &htCountRow{count: 1}
				},
			})
		},
	}

	h := buildHandlerWithTransactor(tr)
	w := callListWithdrawalsHandler(h, sellerID, "")

	var envelope struct {
		Data struct {
			Withdrawals []map[string]interface{} `json:"withdrawals"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &envelope)

	item := envelope.Data.Withdrawals[0]
	ra, _ := item["requested_at"].(string)
	if _, err := time.Parse(time.RFC3339, ra); err != nil {
		t.Errorf("requested_at %q is not RFC3339: %v", ra, err)
	}
}


