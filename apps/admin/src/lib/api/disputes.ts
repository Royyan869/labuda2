import { api } from './client'

// ============================================================================
// DISPUTES API
// ============================================================================

/**
 * Disputes list response with pagination metadata
 */
export interface PaginatedDisputesResponse {
  disputes: unknown[]
  _meta?: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

/**
 * Get all disputes with optional filtering
 * GET /api/v1/admin/disputes
 *
 * Backend uses response.SuccessWithMeta: { success, data: { disputes }, meta, timestamp }
 */
export async function getDisputes(params?: {
  status?: string
  date_from?: string
  date_to?: string
  page?: number
  page_size?: number
}): Promise<PaginatedDisputesResponse> {
  const queryParams = new URLSearchParams()
  if (params?.status) queryParams.append('status', params.status)
  if (params?.date_from) queryParams.append('date_from', params.date_from)
  if (params?.date_to) queryParams.append('date_to', params.date_to)
  queryParams.append('page', String(params?.page ?? 1))
  queryParams.append('page_size', String(params?.page_size ?? 20))

  const resp = await api.get<{
    data: { disputes: unknown[] }
    meta?: { page: number; per_page: number; total: number; total_pages: number }
  }>(`/api/v1/admin/disputes?${queryParams.toString()}`)

  return {
    disputes: resp.data?.disputes ?? [],
    _meta: resp.meta,
  }
}

/**
 * Get dispute detail by ID
 * GET /api/v1/admin/disputes/:id
 *
 * Backend uses response.Success: { success, data: <detail>, timestamp }
 */
export async function getDisputeDetail(disputeId: string) {
  const resp = await api.get<{ data: unknown }>(`/api/v1/admin/disputes/${disputeId}`)
  return resp.data
}

/**
 * Approve dispute (refund buyer)
 * POST /api/v1/admin/disputes/:id/approve
 */
export async function approveDispute(disputeId: string, notes?: string) {
  return api.post(`/api/v1/admin/disputes/${disputeId}/approve`, notes ? { notes } : {})
}

/**
 * Reject dispute (release to seller)
 * POST /api/v1/admin/disputes/:id/reject
 */
export async function rejectDispute(disputeId: string, notes?: string) {
  return api.post(`/api/v1/admin/disputes/${disputeId}/reject`, notes ? { notes } : {})
}

/**
 * Product-only refund: buyer gets item price, seller keeps shipping fee.
 * POST /api/v1/admin/disputes/:id/partial-split
 */
export async function resolveDisputePartialSplit(disputeId: string, notes?: string) {
  return api.post(`/api/v1/admin/disputes/${disputeId}/partial-split`, notes ? { notes } : {})
}
