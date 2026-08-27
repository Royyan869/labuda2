import { api } from './client'

// ============================================================================
// FINANCE / WITHDRAWALS API
// ============================================================================

/**
 * Withdrawals list response with pagination metadata
 */
export interface PaginatedWithdrawalsResponse {
  withdrawals: unknown[]
  _meta?: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

/**
 * Get all withdrawals with optional filtering
 * GET /api/v1/admin/payouts/withdrawals
 */
export async function getWithdrawals(params?: {
  status?: string
  date_from?: string
  date_to?: string
  page?: number
  page_size?: number
}) {
  const queryParams = new URLSearchParams()
  if (params?.status) queryParams.append('status', params.status)
  if (params?.date_from) queryParams.append('date_from', params.date_from)
  if (params?.date_to) queryParams.append('date_to', params.date_to)
  queryParams.append('page', String(params?.page ?? 1))
  queryParams.append('page_size', String(params?.page_size ?? 20))

  return api.get<PaginatedWithdrawalsResponse>(
    `/api/v1/admin/payouts/withdrawals?${queryParams.toString()}`
  )
}

/**
 * Get withdrawal detail by ID
 * GET /api/v1/admin/payouts/withdrawals/:id
 */
export async function getWithdrawalDetail(withdrawalId: string) {
  return api.get(`/api/v1/admin/payouts/withdrawals/${withdrawalId}`)
}

/**
 * Approve withdrawal request
 * POST /api/v1/admin/payouts/withdrawals/:id/approve
 */
export async function approveWithdrawal(withdrawalId: string, notes?: string) {
  return api.post(`/api/v1/admin/payouts/withdrawals/${withdrawalId}/approve`, notes ? { notes } : {})
}

/**
 * Reject withdrawal request
 * POST /api/v1/admin/payouts/withdrawals/:id/reject
 */
export async function rejectWithdrawal(withdrawalId: string, reason: string) {
  return api.post(`/api/v1/admin/payouts/withdrawals/${withdrawalId}/reject`, { reason })
}

/**
 * Mark withdrawal as processed (manual completion)
 * POST /api/v1/admin/payouts/withdrawals/:id/mark-processed
 *
 * CRITICAL: This is for MANUAL COMPLETION ONLY when:
 * 1. External bank transfer was done manually (outside payment gateway)
 * 2. Exceptional recovery requiring admin intervention
 *
 * GUARD: Will fail if external_reference_id is set (payout already submitted to gateway)
 */
export async function markWithdrawalProcessed(withdrawalId: string) {
  return api.post(`/api/v1/admin/payouts/withdrawals/${withdrawalId}/mark-processed`, {})
}

// FINANCE LEDGER API
// ============================================================================

/**
 * Get ledger transactions (read-only)
 * GET /api/v1/admin/finance/ledger
 */
export async function getLedgerTransactions(params?: {
  from?: string
  to?: string
  reference_type?: string
  limit?: number
  offset?: number
}) {
  const queryParams = new URLSearchParams()
  if (params?.from) queryParams.append('from', params.from)
  if (params?.to) queryParams.append('to', params.to)
  if (params?.reference_type) queryParams.append('reference_type', params.reference_type)
  queryParams.append('limit', String(params?.limit ?? 50))
  queryParams.append('offset', String(params?.offset ?? 0))

  return api.get<import('@/types/finance').LedgerListResponse>(
    `/api/v1/admin/finance/ledger?${queryParams.toString()}`
  )
}

// ============================================================================
// PAYOUT WHITELIST AUDIT API
// ============================================================================

/**
 * Get payout whitelist audit log (read-only)
 * GET /api/v1/admin/payouts/whitelist/audit
 */
export async function getWhitelistAudit(params?: {
  seller_id?: string
  limit?: number
  offset?: number
}) {
  const queryParams = new URLSearchParams()
  if (params?.seller_id) queryParams.append('seller_id', params.seller_id)
  queryParams.append('limit', String(params?.limit ?? 50))
  queryParams.append('offset', String(params?.offset ?? 0))

  return api.get<import('@/types/finance').WhitelistAuditResponse>(
    `/api/v1/admin/payouts/whitelist/audit?${queryParams.toString()}`
  )
}

// ============================================================================
// GATEWAY REFUND API
// ============================================================================

/**
 * Initiate gateway refund for a refund row (admin manual retry).
 * POST /api/v1/admin/refunds/:refund_id/gateway/initiate
 * Requires: finance.refund.gateway.initiate
 *
 * Feature-gated: ENABLE_GATEWAY_REFUND_PHASE2 must be enabled server-side.
 * Returns 503 FEATURE_DISABLED when the flag is off.
 * Returns 409 REFUND_ALREADY_SETTLED if gateway already succeeded.
 *
 * A 200 with gateway_status=failed means the orchestration ran but the
 * gateway declined — the admin needs to see that and decide whether to retry.
 */
export async function initiateGatewayRefund(
  refundId: string,
  amount: number,
  reason: string,
  idempotencyKey: string
): Promise<{
  refund_id: string
  order_id: string
  gateway_status: string
  gateway_attempts: number
  gateway_refund_id?: string | null
  gateway_idempotency_key?: string | null
  last_gateway_error?: string | null
}> {
  const resp = await api.post<{ data: {
    refund_id: string
    order_id: string
    gateway_status: string
    gateway_attempts: number
    gateway_refund_id?: string | null
    gateway_idempotency_key?: string | null
    last_gateway_error?: string | null
  } }>(`/api/v1/admin/refunds/${encodeURIComponent(refundId)}/gateway/initiate`, {
    amount,
    reason,
    idempotency_key: idempotencyKey,
  })
  return resp.data
}
