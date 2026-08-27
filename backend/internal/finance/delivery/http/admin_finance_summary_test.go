package http

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/labuda/backend/internal/finance"
)

// ============================================================================
// PASS_18Z: admin finance/reconciliation summary — pure-function tests.
//
// buildFinanceSummaryResponse takes already-queried plain Go data (no DB
// access), so every scenario below is a fast, DB-free unit test. The SQL
// query helpers themselves (querySystemAccountBalances, etc.) follow the
// same untested-glue convention as this file's existing ListLedger/
// RunVerifier query methods — they have no branching logic worth locking,
// only the shaping in buildFinanceSummaryResponse does.
// ============================================================================

func TestBuildFinanceSummaryResponse_ReturnsKeyAccountBalances(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	systemBalances := map[string]int64{
		finance.AccountGatewayClearing: 125_000,
		finance.AccountPlatformRevenue: 9_000,
		finance.AccountBankSettlement:  9_000_000_000_000_000 - 129_000,
		finance.AccountPlatformBank:    0,
	}
	userAggregates := []userAccountAggregate{
		{AccountType: finance.AccountSellerPayable, TotalBalance: 120_000, AccountCount: 1},
		{AccountType: finance.AccountBuyerRefundable, TotalBalance: 0, AccountCount: 0},
	}

	resp := buildFinanceSummaryResponse(now, systemBalances, userAggregates, map[string]int64{}, reconciliationRow{}, alertCounts{})

	if resp.SystemAccountBalances[finance.AccountGatewayClearing] != 125_000 {
		t.Errorf("GATEWAY_CLEARING = %d, want 125000", resp.SystemAccountBalances[finance.AccountGatewayClearing])
	}
	if resp.SystemAccountBalances[finance.AccountPlatformRevenue] != 9_000 {
		t.Errorf("PLATFORM_REVENUE = %d, want 9000", resp.SystemAccountBalances[finance.AccountPlatformRevenue])
	}
	if len(resp.AggregateUserAccounts) != 2 {
		t.Fatalf("expected 2 aggregate user account rows, got %d", len(resp.AggregateUserAccounts))
	}
	if resp.AggregateUserAccounts[0].TotalBalance != 120_000 {
		t.Errorf("SELLER_PAYABLE aggregate = %d, want 120000", resp.AggregateUserAccounts[0].TotalBalance)
	}
	if resp.GeneratedAt != now.Format(time.RFC3339) {
		t.Errorf("generated_at = %q, want %q", resp.GeneratedAt, now.Format(time.RFC3339))
	}
}

func TestBuildFinanceSummaryResponse_GatewayClearingNonZero_IsReportedNotHidden(t *testing.T) {
	systemBalances := map[string]int64{finance.AccountGatewayClearing: 125_000}

	resp := buildFinanceSummaryResponse(time.Now(), systemBalances, nil, nil, reconciliationRow{}, alertCounts{})

	if resp.GatewayClearing.IsZero {
		t.Fatal("is_zero must be false when GATEWAY_CLEARING is non-zero")
	}
	if resp.GatewayClearing.BalanceRupiah != 125_000 {
		t.Errorf("balance_rupiah = %d, want 125000 (must not be hidden/zeroed)", resp.GatewayClearing.BalanceRupiah)
	}
	if !strings.Contains(resp.GatewayClearing.Note, "normal") {
		t.Error("note must explain that non-zero clearing can be normal for paid-but-not-released orders")
	}
}

func TestBuildFinanceSummaryResponse_GatewayClearingZero_ReportsZero(t *testing.T) {
	systemBalances := map[string]int64{finance.AccountGatewayClearing: 0}
	resp := buildFinanceSummaryResponse(time.Now(), systemBalances, nil, nil, reconciliationRow{}, alertCounts{})
	if !resp.GatewayClearing.IsZero {
		t.Fatal("is_zero must be true when GATEWAY_CLEARING is 0")
	}
	if resp.GatewayClearing.BalanceRupiah != 0 {
		t.Errorf("balance_rupiah = %d, want 0", resp.GatewayClearing.BalanceRupiah)
	}
}

// TestBuildFinanceSummaryResponse_ExternalReconciliation_ExplicitlyNotImplemented
// is the central "do not fake a green status" guard from PASS_18X/18Z.
func TestBuildFinanceSummaryResponse_ExternalReconciliation_ExplicitlyNotImplemented(t *testing.T) {
	resp := buildFinanceSummaryResponse(time.Now(), map[string]int64{}, nil, nil, reconciliationRow{}, alertCounts{})

	if resp.ExternalReconciliation.GatewaySettlementReconciliation != "not_implemented" {
		t.Errorf("external_gateway_settlement_reconciliation = %q, want %q",
			resp.ExternalReconciliation.GatewaySettlementReconciliation, "not_implemented")
	}
	if resp.ExternalReconciliation.BankStatementReconciliation != "not_implemented" {
		t.Errorf("bank_statement_reconciliation = %q, want %q",
			resp.ExternalReconciliation.BankStatementReconciliation, "not_implemented")
	}
	if !strings.Contains(resp.ExternalReconciliation.Note, "does NOT prove the real bank balance") {
		t.Error("note must explicitly warn that internal reconciliation does not validate real bank/gateway state")
	}
}

// TestBuildFinanceSummaryResponse_RevenueBreakdown_SeparatesBuyerFeeFromCommission
// proves the exact worked example from PASS_18X/18Y: buyer payment fee
// revenue and commission revenue are distinguishable from real ledger data
// (not guessed), and are correctly separated.
func TestBuildFinanceSummaryResponse_RevenueBreakdown_SeparatesBuyerFeeFromCommission(t *testing.T) {
	systemBalances := map[string]int64{finance.AccountPlatformRevenue: 9_000}
	revenueByRefType := map[string]int64{
		"payment_fee_revenue": 4_000,
		"order_release":       5_000,
	}

	resp := buildFinanceSummaryResponse(time.Now(), systemBalances, nil, revenueByRefType, reconciliationRow{}, alertCounts{})

	if !resp.RevenueBreakdown.Available {
		t.Fatal("breakdown must be available — it is derived from real ledger_entries, not guessed")
	}
	if resp.RevenueBreakdown.BuyerPaymentFeeRevenue != 4_000 {
		t.Errorf("buyer_payment_fee_revenue = %d, want 4000", resp.RevenueBreakdown.BuyerPaymentFeeRevenue)
	}
	if resp.RevenueBreakdown.CommissionRevenue != 5_000 {
		t.Errorf("commission_revenue = %d, want 5000", resp.RevenueBreakdown.CommissionRevenue)
	}
	if resp.RevenueBreakdown.OtherRevenue != 0 {
		t.Errorf("other_revenue = %d, want 0", resp.RevenueBreakdown.OtherRevenue)
	}
	if resp.RevenueBreakdown.TotalPlatformRevenue != 9_000 {
		t.Errorf("total_platform_revenue_rupiah = %d, want 9000", resp.RevenueBreakdown.TotalPlatformRevenue)
	}
}

// TestBuildFinanceSummaryResponse_RevenueBreakdown_CommissionReversalNetsOut proves
// that an after-release refund's commission reversal (reference_type=
// "refund_reversal", negative amount) correctly nets against order_release's
// commission rather than being miscategorized as "other" or ignored.
func TestBuildFinanceSummaryResponse_RevenueBreakdown_CommissionReversalNetsOut(t *testing.T) {
	revenueByRefType := map[string]int64{
		"order_release":   5_000,
		"refund_reversal": -5_000, // full commission reversed on after-release refund
	}
	resp := buildFinanceSummaryResponse(time.Now(), map[string]int64{}, nil, revenueByRefType, reconciliationRow{}, alertCounts{})

	if resp.RevenueBreakdown.CommissionRevenue != 0 {
		t.Errorf("commission_revenue = %d, want 0 (5000 order_release - 5000 refund_reversal)", resp.RevenueBreakdown.CommissionRevenue)
	}
}

// TestBuildFinanceSummaryResponse_RevenueBreakdown_UnmappedReferenceTypeSurfacedAsOther
// proves an unrecognized reference_type (e.g. billing/subscription revenue)
// is never silently dropped — it must show up, explicitly labeled.
func TestBuildFinanceSummaryResponse_RevenueBreakdown_UnmappedReferenceTypeSurfacedAsOther(t *testing.T) {
	revenueByRefType := map[string]int64{
		"billing":                     70_000,
		"seller_subscription_payment": 50_000,
	}
	resp := buildFinanceSummaryResponse(time.Now(), map[string]int64{}, nil, revenueByRefType, reconciliationRow{}, alertCounts{})

	if resp.RevenueBreakdown.OtherRevenue != 120_000 {
		t.Errorf("other_revenue = %d, want 120000", resp.RevenueBreakdown.OtherRevenue)
	}
	if len(resp.RevenueBreakdown.OtherRevenueReferenceTypes) != 2 {
		t.Fatalf("expected 2 other-revenue reference types listed, got %v", resp.RevenueBreakdown.OtherRevenueReferenceTypes)
	}
}

func TestBuildFinanceSummaryResponse_InternalReconciliation_AvailableWhenFound(t *testing.T) {
	checkedAt := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	recon := reconciliationRow{
		Found:              true,
		CheckedAt:          checkedAt,
		Severity:           "passed",
		MismatchedAccounts: 0,
		TotalAccounts:      42,
	}
	resp := buildFinanceSummaryResponse(time.Now(), map[string]int64{}, nil, nil, recon, alertCounts{})

	if !resp.InternalReconciliation.Available {
		t.Fatal("expected internal reconciliation to be available")
	}
	if resp.InternalReconciliation.Severity != "passed" {
		t.Errorf("severity = %q, want %q", resp.InternalReconciliation.Severity, "passed")
	}
	if resp.InternalReconciliation.TotalAccounts != 42 {
		t.Errorf("total_accounts = %d, want 42", resp.InternalReconciliation.TotalAccounts)
	}
	if resp.InternalReconciliation.LastCheckedAt != checkedAt.Format(time.RFC3339) {
		t.Errorf("last_checked_at = %q, want %q", resp.InternalReconciliation.LastCheckedAt, checkedAt.Format(time.RFC3339))
	}
}

func TestBuildFinanceSummaryResponse_InternalReconciliation_UnavailableWhenNoRunsYet(t *testing.T) {
	resp := buildFinanceSummaryResponse(time.Now(), map[string]int64{}, nil, nil, reconciliationRow{Found: false}, alertCounts{})
	if resp.InternalReconciliation.Available {
		t.Fatal("expected internal reconciliation to be unavailable when no run has completed")
	}
}

func TestBuildFinanceSummaryResponse_FinanceAlerts_IncludesCriticalAndCapturedAfterExpiry(t *testing.T) {
	alerts := alertCounts{
		UnresolvedTotal:                 3,
		UnresolvedCriticalTotal:         1,
		PaymentCapturedAfterExpiryCount: 1,
		UnresolvedByType:                map[string]int{"payment_captured_after_expiry": 1, "escrow_integrity_mismatch": 2},
	}
	resp := buildFinanceSummaryResponse(time.Now(), map[string]int64{}, nil, nil, reconciliationRow{}, alerts)

	if resp.FinanceAlerts.UnresolvedTotal != 3 {
		t.Errorf("unresolved_total = %d, want 3", resp.FinanceAlerts.UnresolvedTotal)
	}
	if resp.FinanceAlerts.UnresolvedCriticalTotal != 1 {
		t.Errorf("unresolved_critical_total = %d, want 1", resp.FinanceAlerts.UnresolvedCriticalTotal)
	}
	if resp.FinanceAlerts.PaymentCapturedAfterExpiryCount != 1 {
		t.Errorf("payment_captured_after_expiry_count = %d, want 1", resp.FinanceAlerts.PaymentCapturedAfterExpiryCount)
	}
	if resp.FinanceAlerts.UnresolvedByType["escrow_integrity_mismatch"] != 2 {
		t.Errorf("unresolved_by_type[escrow_integrity_mismatch] = %d, want 2", resp.FinanceAlerts.UnresolvedByType["escrow_integrity_mismatch"])
	}
}

func TestBuildFinanceSummaryResponse_NoAlerts_ReportsZero(t *testing.T) {
	resp := buildFinanceSummaryResponse(time.Now(), map[string]int64{}, nil, nil, reconciliationRow{}, alertCounts{})
	if resp.FinanceAlerts.UnresolvedTotal != 0 || resp.FinanceAlerts.PaymentCapturedAfterExpiryCount != 0 {
		t.Error("expected zero alert counts when none exist")
	}
}

// ============================================================================
// Zombie design guard (PASS_18V/18W/18X/18Y doctrine, scoped to this file)
// ============================================================================

func TestAdminFinanceHandler_NoFlatFeeOrClientAuthorityResidue(t *testing.T) {
	raw, err := os.ReadFile("admin_finance_handler.go")
	if err != nil {
		t.Fatalf("read admin_finance_handler.go: %v", err)
	}
	src := string(raw)

	forbidden := []string{"checkoutfee", "BuyerServiceFeeAmount", "/100", "*100"}
	for _, f := range forbidden {
		if strings.Contains(src, f) {
			t.Errorf("REGRESSION: admin_finance_handler.go must not contain %q", f)
		}
	}

	if !strings.Contains(src, `"not_implemented"`) {
		t.Error("MISSING: external reconciliation must be hardcoded as not_implemented, not a computed/faked status")
	}
}
