// Finance/reconciliation summary types — read-only aggregate visibility over
// existing ledger/alert/reconciliation data (PASS_18Z).
// Backend: GET /api/v1/admin/finance/summary (internal/finance/delivery/http
// AdminFinanceHandler.GetSummary). No new accounting model — this only
// formats numbers the ledger already computed.

export interface UserAccountAggregate {
  account_type: string
  total_balance_rupiah: number
  account_count: number
}

export interface GatewayClearingSummary {
  balance_rupiah: number
  is_zero: boolean
  note: string
}

export interface RevenueBreakdownSummary {
  available: boolean
  buyer_payment_fee_revenue_rupiah: number
  commission_revenue_rupiah: number
  other_revenue_rupiah: number
  other_revenue_reference_types?: string[]
  total_platform_revenue_rupiah: number
  note: string
}

export interface InternalReconciliationSummary {
  available: boolean
  last_checked_at?: string
  severity?: string
  mismatched_accounts?: number
  total_accounts?: number
  note: string
}

/**
 * external_gateway_settlement_reconciliation / bank_statement_reconciliation
 * are always "not_implemented" as of PASS_18Z — Labuda does not ingest a
 * Midtrans settlement report or a bank statement. This is intentional, not a
 * bug: see PASS_18X's audit. Never treat these as a green/healthy signal.
 */
export interface ExternalReconciliationSummary {
  external_gateway_settlement_reconciliation: 'not_implemented'
  bank_statement_reconciliation: 'not_implemented'
  note: string
}

export interface FinanceAlertsSummary {
  unresolved_total: number
  unresolved_critical_total: number
  payment_captured_after_expiry_count: number
  unresolved_by_type?: Record<string, number>
}

export interface FinanceSummaryResponse {
  generated_at: string
  system_account_balances: Record<string, number>
  aggregate_user_account_balances: UserAccountAggregate[]
  gateway_clearing: GatewayClearingSummary
  revenue_breakdown: RevenueBreakdownSummary
  internal_reconciliation: InternalReconciliationSummary
  external_reconciliation: ExternalReconciliationSummary
  finance_alerts: FinanceAlertsSummary
}

/** Canonical system account types this UI knows how to label. */
export const ACCOUNT_LABELS: Record<string, string> = {
  GATEWAY_CLEARING: 'Gateway Clearing',
  PLATFORM_REVENUE: 'Platform Revenue',
  BANK_SETTLEMENT: 'Bank Settlement (internal reserve float)',
  PLATFORM_BANK: 'Platform Bank (internal)',
  WITHDRAWAL_PENDING: 'Withdrawal Pending',
  WITHDRAWAL_COMMITTED: 'Withdrawal Committed',
  SELLER_PAYABLE: 'Seller Payable',
  BUYER_REFUNDABLE: 'Buyer Refundable',
  USER_SERVICE_CREDIT: 'User Service Credit',
  AD_REVENUE: 'Ad Revenue',
}
