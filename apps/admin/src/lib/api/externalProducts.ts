import { api } from './client'
import type {
  AdminExternalProduct,
  AdminExternalProductListResponse,
  ExternalProductReviewHistory,
  ExternalProductReviewStatus,
} from '@/types/external-product'

// ============================================================================
// ADMIN EXTERNAL PRODUCTS API
// Capability required: promotion.external_product.review
// ============================================================================

/**
 * List external products in the admin review queue.
 * GET /api/v1/admin/external-products
 */
export async function listAdminExternalProducts(params?: {
  status?: ExternalProductReviewStatus | ''
  page?: number
  limit?: number
}): Promise<AdminExternalProductListResponse> {
  const q = new URLSearchParams()
  if (params?.status) q.append('status', params.status)
  if (params?.page) q.append('page', String(params.page))
  if (params?.limit) q.append('limit', String(params.limit))
  const qs = q.toString()
  const resp = await api.get<{ data: AdminExternalProductListResponse }>(
    `/api/v1/admin/external-products${qs ? `?${qs}` : ''}`
  )
  return resp.data
}

/**
 * Get admin detail for a single external product (includes review history).
 * GET /api/v1/admin/external-products/:id
 */
export async function getAdminExternalProduct(id: string): Promise<AdminExternalProduct> {
  const resp = await api.get<{ data: AdminExternalProduct }>(`/api/v1/admin/external-products/${id}`)
  return resp.data
}

/**
 * Get review history for an external product.
 * GET /api/v1/admin/external-products/:id/reviews
 */
export async function listAdminExternalProductReviews(id: string): Promise<{
  items: ExternalProductReviewHistory[]
  count: number
}> {
  const resp = await api.get<{ data: { items: ExternalProductReviewHistory[]; count: number } }>(
    `/api/v1/admin/external-products/${id}/reviews`
  )
  return resp.data
}

/**
 * Approve an external product for promotion discovery.
 * POST /api/v1/admin/external-products/:id/approve
 */
export async function approveExternalProduct(
  id: string,
  reason?: string
): Promise<AdminExternalProduct> {
  const resp = await api.post<{ data: AdminExternalProduct }>(
    `/api/v1/admin/external-products/${id}/approve`,
    { reason: reason ?? null }
  )
  return resp.data
}

/**
 * Reject an external product (seller is notified).
 * POST /api/v1/admin/external-products/:id/reject
 */
export async function rejectExternalProduct(
  id: string,
  reason: string
): Promise<AdminExternalProduct> {
  const resp = await api.post<{ data: AdminExternalProduct }>(
    `/api/v1/admin/external-products/${id}/reject`,
    { reason }
  )
  return resp.data
}

/**
 * Request changes from the seller before approval.
 * POST /api/v1/admin/external-products/:id/request-changes
 */
export async function requestChangesExternalProduct(
  id: string,
  reason: string
): Promise<AdminExternalProduct> {
  const resp = await api.post<{ data: AdminExternalProduct }>(
    `/api/v1/admin/external-products/${id}/request-changes`,
    { reason }
  )
  return resp.data
}

/**
 * Hide an approved external product from promotion discovery.
 * POST /api/v1/admin/external-products/:id/hide
 */
export async function hideExternalProduct(
  id: string,
  reason?: string
): Promise<AdminExternalProduct> {
  const resp = await api.post<{ data: AdminExternalProduct }>(
    `/api/v1/admin/external-products/${id}/hide`,
    { reason: reason ?? null }
  )
  return resp.data
}
