import { api } from './client'

// ============================================================================
// SELLER VERIFICATION API
// ============================================================================

/**
 * List seller verification submissions filtered by status.
 * GET /api/v1/admin/seller-verifications/pending?status=<status>
 * When status is omitted, defaults to pending_review (backward compat).
 */
export async function listPendingVerifications(status?: string) {
  const params = status ? `?status=${encodeURIComponent(status)}` : ''
  const resp = await api.get<{
    data: { items: import('@/types').SellerVerificationListItem[]; count: number }
  }>(`/api/v1/admin/seller-verifications/pending${params}`)
  return resp.data
}

/**
 * Get full verification detail for a seller (includes document URLs)
 * GET /api/v1/admin/seller-verifications/:seller_id
 */
export async function getVerificationDetail(sellerId: string) {
  const resp = await api.get<{
    data: import('@/types').SellerVerificationDetail
  }>(`/api/v1/admin/seller-verifications/${sellerId}`)
  return resp.data
}

/**
 * Approve a seller's verification
 * POST /api/v1/admin/seller-verifications/:seller_id/approve
 */
export async function approveVerification(sellerId: string, reason?: string) {
  const resp = await api.post<{
    data: { message: string; seller_id: string; status: string }
  }>(
    `/api/v1/admin/seller-verifications/${sellerId}/approve`,
    reason ? { reason } : {}
  )
  return resp.data
}

/**
 * Reject a seller's verification (reason mandatory)
 * POST /api/v1/admin/seller-verifications/:seller_id/reject
 */
export async function rejectVerification(sellerId: string, reason: string) {
  const resp = await api.post<{
    data: { message: string; seller_id: string; status: string; reason: string }
  }>(
    `/api/v1/admin/seller-verifications/${sellerId}/reject`,
    { reason }
  )
  return resp.data
}

/**
 * Request resubmission from a seller (reason mandatory)
 * POST /api/v1/admin/seller-verifications/:seller_id/request-resubmission
 */
export async function requestVerificationResubmission(sellerId: string, reason: string) {
  const resp = await api.post<{
    data: { message: string; seller_id: string; status: string; reason: string }
  }>(
    `/api/v1/admin/seller-verifications/${sellerId}/request-resubmission`,
    { reason }
  )
  return resp.data
}

/**
 * Suspend a seller's verification (reason mandatory)
 * POST /api/v1/admin/seller-verifications/:seller_id/suspend
 */
export async function suspendVerification(sellerId: string, reason: string) {
  const resp = await api.post<{
    data: { message: string; seller_id: string; status: string }
  }>(
    `/api/v1/admin/seller-verifications/${sellerId}/suspend`,
    { reason }
  )
  return resp.data
}

/**
 * Revoke a seller's verification (reason mandatory)
 * POST /api/v1/admin/seller-verifications/:seller_id/revoke
 */
export async function revokeVerification(sellerId: string, reason: string) {
  const resp = await api.post<{
    data: { message: string; seller_id: string; status: string }
  }>(
    `/api/v1/admin/seller-verifications/${sellerId}/revoke`,
    { reason }
  )
  return resp.data
}

/**
 * Investigate a seller's verification (reason mandatory)
 * POST /api/v1/admin/seller-verifications/:seller_id/investigate
 */
export async function investigateVerification(sellerId: string, reason: string) {
  const resp = await api.post<{
    data: { message: string; seller_id: string; status: string }
  }>(
    `/api/v1/admin/seller-verifications/${sellerId}/investigate`,
    { reason }
  )
  return resp.data
}

/**
 * Restore a seller's verification to approved (reason optional)
 * POST /api/v1/admin/seller-verifications/:seller_id/restore
 */
export async function restoreVerification(sellerId: string, reason?: string) {
  const resp = await api.post<{
    data: { message: string; seller_id: string; status: string }
  }>(
    `/api/v1/admin/seller-verifications/${sellerId}/restore`,
    reason ? { reason } : {}
  )
  return resp.data
}

/**
 * Mark a specific bank account as reviewed for payout without full re-KYC.
 * POST /api/v1/admin/seller-verifications/:seller_id/bank-accounts/:bank_account_id/mark-reviewed
 *
 * Use when a seller adds a new bank account post-approval and an admin has
 * manually confirmed the account belongs to the same KYC-approved identity.
 * Idempotent: marking an already-reviewed account returns 200 with no side effects.
 * Requires: seller is in approved status; account is active and belongs to seller.
 */
export async function markBankAccountReviewed(sellerId: string, bankAccountId: string) {
  const resp = await api.post<{
    data: { message: string; seller_id: string; bank_account_id: string }
  }>(
    `/api/v1/admin/seller-verifications/${sellerId}/bank-accounts/${bankAccountId}/mark-reviewed`,
    {}
  )
  return resp.data
}

// ============================================================================
// SELLER SUBSCRIPTION RECOVERY
// ============================================================================

/**
 * Recover a settled subscription payment that has no seller_subscriptions row.
 * POST /api/v1/admin/seller-subscriptions/recover/:payment_id
 * Requires: seller.subscription.recover
 */
export async function recoverSellerSubscription(paymentId: string) {
  const resp = await api.post<{
    data: { message: string; payment_id: string; user_id: string }
  }>(`/api/v1/admin/seller-subscriptions/recover/${encodeURIComponent(paymentId)}`, {})
  return resp.data
}
