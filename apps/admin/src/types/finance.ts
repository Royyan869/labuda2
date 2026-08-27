// ============================================================================
// FINANCE / WITHDRAWAL TYPES - Based on backend API responses
// ============================================================================

/**
 * Withdrawal status values from backend
 * Source: backend/internal/finance/application/withdraw_service.go (canonical)
 * All values are UPPERCASE as sent by withdrawalToDetail() in admin_payout_handler.go
 */
export type WithdrawalStatus =
  | 'REQUESTED'       // Initial state when seller requests withdrawal
  | 'PROCESSING'      // Approved by admin, being prepared for gateway
  | 'SUBMITTED'       // Submitted to payment gateway
  | 'SETTLING'        // In transit at payment gateway
  | 'SETTLED'         // Successfully settled by gateway
  | 'COMPLETED'       // Manually marked as paid by admin
  | 'FAILED'          // Payment failed at gateway (may retry)
  | 'FAILED_RETRYABLE'// Transient failure, worker will retry
  | 'FAILED_FINAL'    // Permanent failure, funds returned to seller
  | 'PILOT_BLOCKED'   // Blocked during pilot phase

export const WITHDRAWAL_STATUS = {
  REQUESTED: 'REQUESTED',
  PROCESSING: 'PROCESSING',
  SUBMITTED: 'SUBMITTED',
  SETTLING: 'SETTLING',
  SETTLED: 'SETTLED',
  COMPLETED: 'COMPLETED',
  FAILED: 'FAILED',
  FAILED_RETRYABLE: 'FAILED_RETRYABLE',
  FAILED_FINAL: 'FAILED_FINAL',
  PILOT_BLOCKED: 'PILOT_BLOCKED',
} as const

/**
 * Withdrawal list item (from GET /api/v1/admin/payouts/withdrawals)
 */
export interface WithdrawalListItem {
  id: string
  seller_id: string
  amount: number
  fee_amount: number
  total_debit_amount: number
  status: WithdrawalStatus
  bank_name_snapshot: string
  account_number_snapshot: string
  account_holder_snapshot: string
  created_at: string
  // External gateway reference (CRITICAL - prevents manual completion if set)
  external_reference_id?: string
  // Computed fields for display
  seller_username?: string | null
  seller_farm_name?: string | null
  seller_avatar?: string | null
  seller_email?: string | null
  failure_reason?: string | null
}

/**
 * Withdrawal list response (backend sends flat, not _meta-wrapped)
 */
export interface WithdrawalsListResponse {
  withdrawals: WithdrawalListItem[]
  total: number
  limit: number
  offset: number
}

/**
 * Full withdrawal detail (from GET /api/v1/admin/payouts/withdrawals/:id)
 */
export interface WithdrawalDetail {
  id: string
  seller_id: string
  amount: number
  fee_amount: number
  total_debit_amount: number
  status: WithdrawalStatus
  bank_name_snapshot: string
  account_number_snapshot: string
  account_holder_snapshot: string
  created_at: string
  updated_at: string
  // External gateway reference (CRITICAL - prevents manual completion if set)
  external_reference_id?: string
  gateway_reference_id?: string
  // Processing timestamps
  submitted_at?: string | null
  settled_at?: string | null
  // Failure info
  failure_reason?: string | null
  retry_count?: number
  // Computed fields for display
  seller_username?: string | null
  seller_farm_name?: string | null
  seller_avatar?: string | null
  seller_email?: string | null
}

/**
 * Withdrawal approve request
 */
export interface WithdrawalApproveRequest {
  notes?: string
}

/**
 * Withdrawal reject request
 */
export interface WithdrawalRejectRequest {
  reason: string
}

/**
 * Withdrawal action response
 */
export interface WithdrawalActionResponse {
  withdrawal_id: string
  action: 'approved' | 'rejected'
  message: string
}

/**
 * Withdrawals query parameters
 */
export interface WithdrawalsQueryParams {
  status?: WithdrawalStatus | ''
  date_from?: string
  date_to?: string
  page?: number
  page_size?: number
}

// ============================================================================
// LABELS & VARIANTS
// ============================================================================

export const withdrawalStatusLabels: Record<WithdrawalStatus, string> = {
  REQUESTED: 'Pending Approval',
  PROCESSING: 'Processing',
  SUBMITTED: 'Submitted to Gateway',
  SETTLING: 'In Transit',
  SETTLED: 'Settled by Gateway',
  COMPLETED: 'Manually Paid',
  FAILED: 'Failed',
  FAILED_RETRYABLE: 'Failed (Retrying)',
  FAILED_FINAL: 'Failed (Final)',
  PILOT_BLOCKED: 'Blocked (Pilot)',
}

export const withdrawalStatusVariants: Record<
  WithdrawalStatus,
  'success' | 'warning' | 'error' | 'info' | 'pending'
> = {
  REQUESTED: 'pending',
  PROCESSING: 'info',
  SUBMITTED: 'info',
  SETTLING: 'warning',
  SETTLED: 'success',
  COMPLETED: 'success',
  FAILED: 'error',
  FAILED_RETRYABLE: 'warning',
  FAILED_FINAL: 'error',
  PILOT_BLOCKED: 'warning',
}

// LEDGER TYPES - Based on GET /api/v1/admin/finance/ledger
// ============================================================================

export interface LedgerEntry {
  id: string
  account_id: string
  account_type: string
  entry_type: string
  amount: number
  balance_after: number
}

export interface LedgerTransaction {
  id: string
  idempotency_key: string
  reference_type: string
  reference_id: string | null
  order_id: string | null
  payment_id: string | null
  created_at: string
  entries: LedgerEntry[]
}

export interface LedgerListResponse {
  transactions: LedgerTransaction[]
  total: number
  limit: number
  offset: number
}

// ============================================================================
// PAYOUT WHITELIST AUDIT TYPES - Based on GET /api/v1/admin/payouts/whitelist/audit
// ============================================================================

export interface WhitelistAuditRow {
  id: string
  seller_id?: string
  action: string
  actor_id: string
  reason: string
  source: string
  created_at: string
}

export interface WhitelistAuditResponse {
  audit_log: WhitelistAuditRow[]
  limit: number
  offset: number
  count: number
}

export type WhitelistAuditAction = 'WHITELIST_INITIALIZED' | 'SELLER_ADDED' | 'SELLER_REMOVED'

export const whitelistActionLabels: Record<WhitelistAuditAction, string> = {
  WHITELIST_INITIALIZED: 'Initialized',
  SELLER_ADDED: 'Seller Added',
  SELLER_REMOVED: 'Seller Removed',
}

export const whitelistActionVariants: Record<WhitelistAuditAction, 'info' | 'success' | 'error'> = {
  WHITELIST_INITIALIZED: 'info',
  SELLER_ADDED: 'success',
  SELLER_REMOVED: 'error',
}

// Common failure reason labels
export const withdrawalFailureReasonLabels: Record<string, string> = {
  INSUFFICIENT_BALANCE: 'Insufficient seller balance',
  INVALID_BANK_ACCOUNT: 'Invalid bank account details',
  BANK_TRANSFER_FAILED: 'Bank transfer failed',
  ACCOUNT_FROZEN: 'Seller account is frozen',
  DUPLICATE_WITHDRAWAL: 'Duplicate withdrawal request',
  TIMEOUT: 'Processing timeout',
  OTHER: 'Other reason',
}
