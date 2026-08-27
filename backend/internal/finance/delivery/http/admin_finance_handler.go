package http

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/finance"
	"github.com/labuda/backend/internal/finance/verifier"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"go.uber.org/zap"
)

// AdminFinanceHandler provides read-only canonical finance visibility endpoints.
//
// All endpoints are protected by RequireAdminMiddleware + finance.admin capability.
// None of these endpoints mutate any data.
type AdminFinanceHandler struct {
	pool             *pgxpool.Pool
	adminAuditLogger audit.AdminAuditLogger
	log              *zap.Logger
}

func NewAdminFinanceHandler(
	pool *pgxpool.Pool,
	adminAuditLogger audit.AdminAuditLogger,
	log *zap.Logger,
) *AdminFinanceHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AdminFinanceHandler{
		pool:             pool,
		adminAuditLogger: adminAuditLogger,
		log:              log,
	}
}

// ============================================================================
// LEDGER EXPORT
// ============================================================================

// ledgerEntryRow is the wire shape for a single ledger entry.
type ledgerEntryRow struct {
	ID           string `json:"id"`
	AccountID    string `json:"account_id"`
	AccountType  string `json:"account_type"`
	EntryType    string `json:"entry_type"`
	Amount       int64  `json:"amount"`
	BalanceAfter int64  `json:"balance_after"`
}

// ledgerTxRow is the wire shape for a single ledger transaction with its entries.
type ledgerTxRow struct {
	ID             string           `json:"id"`
	IdempotencyKey string           `json:"idempotency_key"`
	ReferenceType  string           `json:"reference_type"`
	ReferenceID    *string          `json:"reference_id,omitempty"`
	OrderID        *string          `json:"order_id,omitempty"`
	PaymentID      *string          `json:"payment_id,omitempty"`
	CreatedAt      string           `json:"created_at"`
	Entries        []ledgerEntryRow `json:"entries"`
}

// ListLedger handles GET /api/v1/admin/finance/ledger
//
// Returns paginated canonical ledger transactions with their entries.
// Supports filtering by time range, reference_type, and account_type.
//
// Query parameters:
//   - from          ISO-8601 or Unix timestamp (inclusive)
//   - to            ISO-8601 or Unix timestamp (inclusive)
//   - reference_type filter by ledger transaction reference_type
//   - limit         max rows (default 50, max 200)
//   - offset        pagination offset (default 0)
func (h *AdminFinanceHandler) ListLedger(c *gin.Context) {
	ctx := c.Request.Context()

	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse time range
	fromTime, toTime, err := parseLedgerTimeRange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Parse reference_type filter
	referenceType := c.Query("reference_type")

	// Parse pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	// Query ledger_transactions
	txRows, total, err := h.queryLedgerTransactions(ctx, fromTime, toTime, referenceType, limit, offset)
	if err != nil {
		h.log.Error("Failed to query ledger transactions", zap.Error(err))
		response.InternalServerError(c, "Failed to fetch ledger data")
		return
	}

	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_ledger_exported", "ledger", uuid.Nil,
		map[string]interface{}{
			"from":           fromTime,
			"to":             toTime,
			"reference_type": referenceType,
			"limit":          limit,
			"offset":         offset,
			"result_count":   len(txRows),
		},
	)

	c.JSON(http.StatusOK, gin.H{
		"transactions": txRows,
		"total":        total,
		"limit":        limit,
		"offset":       offset,
	})
}

func parseLedgerTimeRange(c *gin.Context) (time.Time, time.Time, error) {
	fromStr := c.DefaultQuery("from", "")
	toStr := c.DefaultQuery("to", "")

	fromTime := time.Time{}
	toTime := time.Now()

	if fromStr != "" {
		t, err := parseFlexTime(fromStr)
		if err != nil {
			return fromTime, toTime, fmt.Errorf("invalid 'from' parameter: %s", fromStr)
		}
		fromTime = t
	}
	if toStr != "" {
		t, err := parseFlexTime(toStr)
		if err != nil {
			return fromTime, toTime, fmt.Errorf("invalid 'to' parameter: %s", toStr)
		}
		toTime = t
	}
	return fromTime, toTime, nil
}

func parseFlexTime(s string) (time.Time, error) {
	// Try Unix timestamp first
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(n, 0), nil
	}
	// Try ISO-8601
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised time format")
}

func (h *AdminFinanceHandler) queryLedgerTransactions(
	ctx context.Context,
	from, to time.Time,
	referenceType string,
	limit, offset int,
) ([]ledgerTxRow, int64, error) {
	// Build WHERE clause
	args := []interface{}{}
	where := "WHERE lt.created_at <= $1"
	args = append(args, to.Unix())

	if !from.IsZero() {
		args = append(args, from.Unix())
		where += fmt.Sprintf(" AND lt.created_at >= $%d", len(args))
	}
	if referenceType != "" {
		args = append(args, referenceType)
		where += fmt.Sprintf(" AND lt.reference_type = $%d", len(args))
	}

	// Count
	countSQL := "SELECT COUNT(*) FROM ledger_transactions lt " + where
	var total int64
	if err := h.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count ledger_transactions: %w", err)
	}

	// Fetch transactions
	args = append(args, limit, offset)
	txSQL := fmt.Sprintf(`
		SELECT lt.id, lt.idempotency_key, lt.reference_type, lt.reference_id,
		       lt.order_id, lt.payment_id, lt.created_at
		FROM ledger_transactions lt
		%s
		ORDER BY lt.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	txRows, err := h.pool.Query(ctx, txSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query ledger_transactions: %w", err)
	}
	defer txRows.Close()

	var results []ledgerTxRow
	var txIDs []uuid.UUID
	txByID := map[uuid.UUID]*ledgerTxRow{}

	for txRows.Next() {
		var row ledgerTxRow
		var id uuid.UUID
		var createdAt int64
		var refID, orderID, paymentID *uuid.UUID

		if err := txRows.Scan(&id, &row.IdempotencyKey, &row.ReferenceType,
			&refID, &orderID, &paymentID, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan ledger_transaction: %w", err)
		}
		row.ID = id.String()
		row.CreatedAt = time.Unix(createdAt, 0).UTC().Format(time.RFC3339)
		if refID != nil {
			s := refID.String()
			row.ReferenceID = &s
		}
		if orderID != nil {
			s := orderID.String()
			row.OrderID = &s
		}
		if paymentID != nil {
			s := paymentID.String()
			row.PaymentID = &s
		}
		results = append(results, row)
		txIDs = append(txIDs, id)
		txByID[id] = &results[len(results)-1]
	}
	if err := txRows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate ledger_transactions: %w", err)
	}

	if len(txIDs) == 0 {
		return results, total, nil
	}

	// Fetch entries for these transactions in one shot
	entrySQL := `
		SELECT le.id, le.transaction_id, le.account_id, fa.account_type,
		       le.entry_type, le.amount, le.balance_after
		FROM ledger_entries le
		JOIN financial_accounts fa ON fa.id = le.account_id
		WHERE le.transaction_id = ANY($1)
		ORDER BY le.transaction_id, le.id
	`
	entryRows, err := h.pool.Query(ctx, entrySQL, txIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("query ledger_entries: %w", err)
	}
	defer entryRows.Close()

	for entryRows.Next() {
		var entry ledgerEntryRow
		var entryID, txID, accountID uuid.UUID
		if err := entryRows.Scan(&entryID, &txID, &accountID, &entry.AccountType,
			&entry.EntryType, &entry.Amount, &entry.BalanceAfter); err != nil {
			return nil, 0, fmt.Errorf("scan ledger_entry: %w", err)
		}
		entry.ID = entryID.String()
		entry.AccountID = accountID.String()
		if tx, ok := txByID[txID]; ok {
			tx.Entries = append(tx.Entries, entry)
		}
	}
	if err := entryRows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate ledger_entries: %w", err)
	}

	return results, total, nil
}

// ============================================================================
// INVARIANT VERIFIER
// ============================================================================

// verifierSectionResult is the wire shape for a single verifier section.
type verifierSectionResult struct {
	Name     string            `json:"name"`
	Passed   bool              `json:"passed"`
	Findings []verifierFinding `json:"findings,omitempty"`
}

type verifierFinding struct {
	Code   string `json:"code"`
	Level  string `json:"level"`
	Class  string `json:"class"`
	Detail string `json:"detail"`
}

// verifierResponse is the wire shape for the full verifier result.
type verifierResponse struct {
	Mode         string                  `json:"mode"`
	Passed       bool                    `json:"passed"`
	ErrorCount   int                     `json:"error_count"`
	WarningCount int                     `json:"warning_count"`
	Sections     []verifierSectionResult `json:"sections"`
}

// RunVerifier handles POST /api/v1/admin/finance/verify
//
// Loads a DB snapshot and runs the invariant verifier. Read-only.
// Defaults to forensic mode; pass ?mode=strict to raise all findings to errors.
//
// Panics from the verifier (e.g. strict mode on critical findings) are caught
// and returned as HTTP 500 so they never crash the server.
func (h *AdminFinanceHandler) RunVerifier(c *gin.Context) {
	ctx := c.Request.Context()

	actorID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	modeStr := c.DefaultQuery("mode", "forensic")
	var mode verifier.Mode
	switch modeStr {
	case "strict":
		mode = verifier.ModeStrict
	case "forensic":
		mode = verifier.ModeForensic
	default:
		response.BadRequest(c, "mode must be 'forensic' or 'strict'")
		return
	}

	// 30-second timeout so a large DB doesn't block the request indefinitely.
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Catch any verifier panic (strict mode panics on CRITICAL findings).
	report, panicErr := runVerifierSafe(runCtx, h.pool, mode)
	if panicErr != "" {
		h.log.Error("Verifier panicked", zap.String("panic", panicErr), zap.String("mode", modeStr))
		response.InternalServerError(c, "Verifier panic: "+panicErr)
		return
	}

	res := buildVerifierResponse(report)

	h.adminAuditLogger.LogSafe(ctx, actorID,
		"admin_finance_verified", "finance", uuid.Nil,
		map[string]interface{}{
			"mode":          modeStr,
			"passed":        res.Passed,
			"error_count":   res.ErrorCount,
			"warning_count": res.WarningCount,
		},
	)

	status := http.StatusOK
	if !res.Passed {
		status = http.StatusUnprocessableEntity
	}
	c.JSON(status, res)
}

// runVerifierSafe runs the verifier and recovers from any panic.
// Returns the report and an empty string on success, or an empty report and
// the panic message on failure.
func runVerifierSafe(ctx context.Context, pool *pgxpool.Pool, mode verifier.Mode) (report verifier.Report, panicMsg string) {
	defer func() {
		if r := recover(); r != nil {
			panicMsg = fmt.Sprintf("%v", r)
		}
	}()
	snapshot, err := verifier.LoadSnapshot(ctx, pool)
	if err != nil {
		panicMsg = "snapshot load failed: " + err.Error()
		return
	}
	report = verifier.Verify(snapshot, mode)
	return
}

// ============================================================================
// FINANCE SUMMARY (PASS_18Z)
// ============================================================================
//
// GetSummary is a read-only visibility surface over data that already exists
// in the ledger — it introduces no new accounting model, no gateway-fee
// expense/liability account, and no guessed Midtrans numbers. It answers,
// without a DB query, exactly the questions PASS_18Z's owner outcome names:
// current GATEWAY_CLEARING, buyer payment fee revenue collected, commission
// revenue collected, seller payable total, unresolved finance alerts, and
// whether finance is internally balanced — while being explicit that
// external Midtrans/bank settlement reconciliation is NOT implemented
// (PASS_18X finding), never faking a green status for it.

// userAccountAggregate summarizes a per-user account type (SELLER_PAYABLE,
// BUYER_REFUNDABLE, USER_SERVICE_CREDIT, ...) across every user.
type userAccountAggregate struct {
	AccountType  string `json:"account_type"`
	TotalBalance int64  `json:"total_balance_rupiah"`
	AccountCount int    `json:"account_count"`
}

type gatewayClearingSummary struct {
	BalanceRupiah int64  `json:"balance_rupiah"`
	IsZero        bool   `json:"is_zero"`
	Note          string `json:"note"`
}

type revenueBreakdownSummary struct {
	Available                  bool     `json:"available"`
	BuyerPaymentFeeRevenue     int64    `json:"buyer_payment_fee_revenue_rupiah"`
	CommissionRevenue          int64    `json:"commission_revenue_rupiah"`
	OtherRevenue               int64    `json:"other_revenue_rupiah"`
	OtherRevenueReferenceTypes []string `json:"other_revenue_reference_types,omitempty"`
	TotalPlatformRevenue       int64    `json:"total_platform_revenue_rupiah"`
	Note                       string   `json:"note"`
}

type internalReconciliationSummary struct {
	Available          bool   `json:"available"`
	LastCheckedAt      string `json:"last_checked_at,omitempty"`
	Severity           string `json:"severity,omitempty"`
	MismatchedAccounts int    `json:"mismatched_accounts,omitempty"`
	TotalAccounts      int    `json:"total_accounts,omitempty"`
	Note               string `json:"note"`
}

type externalReconciliationSummary struct {
	GatewaySettlementReconciliation string `json:"external_gateway_settlement_reconciliation"`
	BankStatementReconciliation     string `json:"bank_statement_reconciliation"`
	Note                            string `json:"note"`
}

type financeAlertsSummary struct {
	UnresolvedTotal                 int            `json:"unresolved_total"`
	UnresolvedCriticalTotal         int            `json:"unresolved_critical_total"`
	PaymentCapturedAfterExpiryCount int            `json:"payment_captured_after_expiry_count"`
	UnresolvedByType                map[string]int `json:"unresolved_by_type,omitempty"`
}

// financeSummaryResponse is the full wire shape for GET /admin/finance/summary.
type financeSummaryResponse struct {
	GeneratedAt            string                        `json:"generated_at"`
	SystemAccountBalances  map[string]int64              `json:"system_account_balances"`
	AggregateUserAccounts  []userAccountAggregate        `json:"aggregate_user_account_balances"`
	GatewayClearing        gatewayClearingSummary        `json:"gateway_clearing"`
	RevenueBreakdown       revenueBreakdownSummary       `json:"revenue_breakdown"`
	InternalReconciliation internalReconciliationSummary `json:"internal_reconciliation"`
	ExternalReconciliation externalReconciliationSummary `json:"external_reconciliation"`
	FinanceAlerts          financeAlertsSummary          `json:"finance_alerts"`
}

// reconciliationRow is the plain-Go shape of the latest reconciliation_results
// row, decoupled from SQL scanning so buildFinanceSummaryResponse is
// unit-testable without a database.
type reconciliationRow struct {
	Found              bool
	CheckedAt          time.Time
	Severity           string
	MismatchedAccounts int
	TotalAccounts      int
}

// alertCounts is the plain-Go shape of the alert aggregation query.
type alertCounts struct {
	UnresolvedTotal                 int
	UnresolvedCriticalTotal         int
	PaymentCapturedAfterExpiryCount int
	UnresolvedByType                map[string]int
}

// revenueReferenceType -> bucket mapping. Every reference_type that ever
// posts to PLATFORM_REVENUE (see finance_service.go's CreateTransaction call
// sites: "payment_fee_revenue", "order_release", "partial_refund_release",
// "refund_reversal", "billing", "seller_subscription_payment") is accounted
// for explicitly below — nothing is silently dropped, and anything NOT in
// this map surfaces honestly under "other_revenue_reference_types" rather
// than being hidden.
const (
	refTypeBuyerPaymentFee = "payment_fee_revenue"
)

var commissionReferenceTypes = map[string]bool{
	"order_release":          true,
	"partial_refund_release": true,
	"refund_reversal":        true, // commission reversal on after-release refunds (negative)
}

// buildFinanceSummaryResponse is a pure function: given already-queried data,
// it produces the exact response shape. No DB access, no accounting
// authority — it only formats numbers the ledger already computed
// (FinanceService in internal/finance/application owns all money movement).
func buildFinanceSummaryResponse(
	now time.Time,
	systemBalances map[string]int64,
	userAggregates []userAccountAggregate,
	revenueByRefType map[string]int64,
	recon reconciliationRow,
	alerts alertCounts,
) financeSummaryResponse {
	gatewayClearing := systemBalances[finance.AccountGatewayClearing]

	var buyerFee, commission, other int64
	var otherRefTypes []string
	for refType, net := range revenueByRefType {
		switch {
		case refType == refTypeBuyerPaymentFee:
			buyerFee += net
		case commissionReferenceTypes[refType]:
			commission += net
		default:
			other += net
			otherRefTypes = append(otherRefTypes, refType)
		}
	}
	sort.Strings(otherRefTypes)

	resp := financeSummaryResponse{
		GeneratedAt:           now.UTC().Format(time.RFC3339),
		SystemAccountBalances: systemBalances,
		AggregateUserAccounts: userAggregates,
		GatewayClearing: gatewayClearingSummary{
			BalanceRupiah: gatewayClearing,
			IsZero:        gatewayClearing == 0,
			Note: "Non-zero GATEWAY_CLEARING is normal: it holds every order's " +
				"escrow-equivalent amount between payment settlement and order " +
				"release. Zero does not imply idle; non-zero does not imply a problem.",
		},
		RevenueBreakdown: revenueBreakdownSummary{
			Available:                  true,
			BuyerPaymentFeeRevenue:     buyerFee,
			CommissionRevenue:          commission,
			OtherRevenue:               other,
			OtherRevenueReferenceTypes: otherRefTypes,
			TotalPlatformRevenue:       systemBalances[finance.AccountPlatformRevenue],
			Note: "Derived from ledger_entries grouped by the originating " +
				"transaction's reference_type. buyer_payment_fee_revenue = " +
				"payment_fee_revenue entries; commission_revenue = order_release " +
				"+ partial_refund_release + refund_reversal entries (commission " +
				"reversals net out automatically); other_revenue = anything else " +
				"posted to PLATFORM_REVENUE (e.g. billing/subscription), listed " +
				"explicitly in other_revenue_reference_types rather than hidden.",
		},
		ExternalReconciliation: externalReconciliationSummary{
			GatewaySettlementReconciliation: "not_implemented",
			BankStatementReconciliation:     "not_implemented",
			Note: "PASS_18X finding: Labuda does not yet ingest a Midtrans " +
				"settlement report or bank statement, and BANK_SETTLEMENT is an " +
				"internal reserve-float abstraction, not real bank data. " +
				"Internal reconciliation below proves Labuda's own ledger is " +
				"self-consistent; it does NOT prove the real bank balance " +
				"matches this ledger.",
		},
		FinanceAlerts: financeAlertsSummary{
			UnresolvedTotal:                 alerts.UnresolvedTotal,
			UnresolvedCriticalTotal:         alerts.UnresolvedCriticalTotal,
			PaymentCapturedAfterExpiryCount: alerts.PaymentCapturedAfterExpiryCount,
			UnresolvedByType:                alerts.UnresolvedByType,
		},
	}

	if recon.Found {
		resp.InternalReconciliation = internalReconciliationSummary{
			Available:          true,
			LastCheckedAt:      recon.CheckedAt.UTC().Format(time.RFC3339),
			Severity:           recon.Severity,
			MismatchedAccounts: recon.MismatchedAccounts,
			TotalAccounts:      recon.TotalAccounts,
			Note: "Reflects the most recent ReconciliationWorkerV2 run: Labuda's " +
				"own ledger tables compared against each other for double-entry " +
				"and account-balance consistency. This is internal-only.",
		}
	} else {
		resp.InternalReconciliation = internalReconciliationSummary{
			Available: false,
			Note:      "No reconciliation run has completed yet.",
		}
	}

	return resp
}

// GetSummary handles GET /api/v1/admin/finance/summary
//
// Read-only aggregate visibility over existing ledger/alert/reconciliation
// data. Mutates nothing. See buildFinanceSummaryResponse for the response
// shape and the accounting rationale behind each field.
func (h *AdminFinanceHandler) GetSummary(c *gin.Context) {
	ctx := c.Request.Context()

	if _, ok := middleware.MustGetUserIDFromContext(c); !ok {
		return
	}

	systemBalances, err := h.querySystemAccountBalances(ctx)
	if err != nil {
		h.log.Error("Failed to query system account balances", zap.Error(err))
		response.InternalServerError(c, "Failed to fetch finance summary")
		return
	}

	userAggregates, err := h.queryUserAccountAggregates(ctx)
	if err != nil {
		h.log.Error("Failed to query user account aggregates", zap.Error(err))
		response.InternalServerError(c, "Failed to fetch finance summary")
		return
	}

	revenueByRefType, err := h.queryPlatformRevenueByReferenceType(ctx)
	if err != nil {
		h.log.Error("Failed to query platform revenue breakdown", zap.Error(err))
		response.InternalServerError(c, "Failed to fetch finance summary")
		return
	}

	recon, err := h.queryLatestReconciliation(ctx)
	if err != nil {
		h.log.Error("Failed to query latest reconciliation result", zap.Error(err))
		response.InternalServerError(c, "Failed to fetch finance summary")
		return
	}

	alerts, err := h.queryAlertCounts(ctx)
	if err != nil {
		h.log.Error("Failed to query alert counts", zap.Error(err))
		response.InternalServerError(c, "Failed to fetch finance summary")
		return
	}

	resp := buildFinanceSummaryResponse(time.Now(), systemBalances, userAggregates, revenueByRefType, recon, alerts)
	c.JSON(http.StatusOK, resp)
}

func (h *AdminFinanceHandler) querySystemAccountBalances(ctx context.Context) (map[string]int64, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT account_type, balance FROM financial_accounts WHERE user_id IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("query system account balances: %w", err)
	}
	defer rows.Close()

	balances := make(map[string]int64)
	for rows.Next() {
		var accountType string
		var balance int64
		if err := rows.Scan(&accountType, &balance); err != nil {
			return nil, fmt.Errorf("scan system account balance: %w", err)
		}
		balances[accountType] = balance
	}
	return balances, rows.Err()
}

func (h *AdminFinanceHandler) queryUserAccountAggregates(ctx context.Context) ([]userAccountAggregate, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT account_type, SUM(balance), COUNT(*)
		FROM financial_accounts
		WHERE user_id IS NOT NULL
		GROUP BY account_type
		ORDER BY account_type
	`)
	if err != nil {
		return nil, fmt.Errorf("query user account aggregates: %w", err)
	}
	defer rows.Close()

	// Initialized (not nil) so an empty result set marshals as [], not null —
	// the admin frontend maps over this field directly.
	out := make([]userAccountAggregate, 0)
	for rows.Next() {
		var agg userAccountAggregate
		if err := rows.Scan(&agg.AccountType, &agg.TotalBalance, &agg.AccountCount); err != nil {
			return nil, fmt.Errorf("scan user account aggregate: %w", err)
		}
		out = append(out, agg)
	}
	return out, rows.Err()
}

func (h *AdminFinanceHandler) queryPlatformRevenueByReferenceType(ctx context.Context) (map[string]int64, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT lt.reference_type,
		       SUM(CASE WHEN le.entry_type = 'debit' THEN le.amount ELSE -le.amount END)
		FROM ledger_entries le
		JOIN ledger_transactions lt ON lt.id = le.transaction_id
		JOIN financial_accounts fa ON fa.id = le.account_id
		WHERE fa.account_type = $1 AND fa.user_id IS NULL
		GROUP BY lt.reference_type
	`, finance.AccountPlatformRevenue)
	if err != nil {
		return nil, fmt.Errorf("query platform revenue breakdown: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var refType string
		var net int64
		if err := rows.Scan(&refType, &net); err != nil {
			return nil, fmt.Errorf("scan platform revenue breakdown row: %w", err)
		}
		out[refType] = net
	}
	return out, rows.Err()
}

func (h *AdminFinanceHandler) queryLatestReconciliation(ctx context.Context) (reconciliationRow, error) {
	var row reconciliationRow
	err := h.pool.QueryRow(ctx, `
		SELECT checked_at, severity, mismatched_accounts, total_accounts
		FROM reconciliation_results
		ORDER BY checked_at DESC
		LIMIT 1
	`).Scan(&row.CheckedAt, &row.Severity, &row.MismatchedAccounts, &row.TotalAccounts)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return reconciliationRow{Found: false}, nil
		}
		return reconciliationRow{}, fmt.Errorf("query latest reconciliation result: %w", err)
	}
	row.Found = true
	return row, nil
}

func (h *AdminFinanceHandler) queryAlertCounts(ctx context.Context) (alertCounts, error) {
	var out alertCounts

	err := h.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM system_alerts WHERE status NOT IN ('resolved', 'false_positive')
	`).Scan(&out.UnresolvedTotal)
	if err != nil {
		return out, fmt.Errorf("count unresolved alerts: %w", err)
	}

	err = h.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM system_alerts
		WHERE status NOT IN ('resolved', 'false_positive') AND severity = 'critical'
	`).Scan(&out.UnresolvedCriticalTotal)
	if err != nil {
		return out, fmt.Errorf("count unresolved critical alerts: %w", err)
	}

	err = h.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM system_alerts
		WHERE status NOT IN ('resolved', 'false_positive') AND alert_type = 'payment_captured_after_expiry'
	`).Scan(&out.PaymentCapturedAfterExpiryCount)
	if err != nil {
		return out, fmt.Errorf("count payment_captured_after_expiry alerts: %w", err)
	}

	rows, err := h.pool.Query(ctx, `
		SELECT alert_type, COUNT(*) FROM system_alerts
		WHERE status NOT IN ('resolved', 'false_positive')
		GROUP BY alert_type
	`)
	if err != nil {
		return out, fmt.Errorf("count unresolved alerts by type: %w", err)
	}
	defer rows.Close()

	out.UnresolvedByType = make(map[string]int)
	for rows.Next() {
		var alertType string
		var count int
		if err := rows.Scan(&alertType, &count); err != nil {
			return out, fmt.Errorf("scan alert type count: %w", err)
		}
		out.UnresolvedByType[alertType] = count
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	return out, nil
}

func buildVerifierResponse(report verifier.Report) verifierResponse {
	res := verifierResponse{
		Mode:   string(report.Mode),
		Passed: !report.HasFailures(),
	}
	for _, sec := range report.Sections {
		s := verifierSectionResult{
			Name:   sec.Name,
			Passed: sec.Passed,
		}
		for _, f := range sec.Findings {
			s.Findings = append(s.Findings, verifierFinding{
				Code:   f.Code,
				Level:  f.Level,
				Class:  f.Class,
				Detail: f.Detail,
			})
			if f.Level == "error" {
				res.ErrorCount++
			} else {
				res.WarningCount++
			}
		}
		res.Sections = append(res.Sections, s)
	}
	return res
}
