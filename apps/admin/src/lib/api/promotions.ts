import { api } from './client'
import type {
  AdminCampaignAnalyticsResponse,
  PromotionPackage,
  PromotionPackageListResponse,
  AdminCampaignListResponse,
} from '@/types/promotion'

// ============================================================================
// ADMIN PROMOTION PACKAGES API
// Capability required: promotion.package.manage
// ============================================================================

/**
 * List all promotion packages (including inactive).
 * GET /api/v1/admin/promotions/packages
 */
export async function adminListPackages(): Promise<PromotionPackageListResponse> {
  const resp = await api.get<{ data: PromotionPackageListResponse }>('/api/v1/admin/promotions/packages')
  return resp.data
}

/**
 * Create a new promotion package.
 * POST /api/v1/admin/promotions/packages
 */
export async function adminCreatePackage(data: {
  name: string
  total_duration_hours: number
  validity_window_hours: number
  price_amount: number
  allowed_target_types: string[]
}): Promise<{ package: PromotionPackage }> {
  const resp = await api.post<{ data: { package: PromotionPackage } }>('/api/v1/admin/promotions/packages', data)
  return resp.data
}

/**
 * Update an existing promotion package.
 * PATCH /api/v1/admin/promotions/packages/:id
 */
export async function adminUpdatePackage(
  id: string,
  data: {
    name: string
    total_duration_hours: number
    validity_window_hours: number
    price_amount: number
    allowed_target_types: string[]
    is_active: boolean
  }
): Promise<{ package: PromotionPackage }> {
  const resp = await api.patch<{ data: { package: PromotionPackage } }>(`/api/v1/admin/promotions/packages/${id}`, data)
  return resp.data
}

/**
 * Enable a promotion package.
 * POST /api/v1/admin/promotions/packages/:id/enable
 */
export async function adminEnablePackage(id: string): Promise<{ package: PromotionPackage }> {
  const resp = await api.post<{ data: { package: PromotionPackage } }>(
    `/api/v1/admin/promotions/packages/${id}/enable`,
    {}
  )
  return resp.data
}

/**
 * Disable a promotion package.
 * POST /api/v1/admin/promotions/packages/:id/disable
 */
export async function adminDisablePackage(id: string): Promise<{ package: PromotionPackage }> {
  const resp = await api.post<{ data: { package: PromotionPackage } }>(
    `/api/v1/admin/promotions/packages/${id}/disable`,
    {}
  )
  return resp.data
}

// ============================================================================
// ADMIN PROMOTION CAMPAIGNS API
// Capability required: promotion.campaign.view / promotion.campaign.stop
// ============================================================================

/**
 * List promotion campaigns (instances) with optional filters.
 * GET /api/v1/admin/promotions/campaigns
 */
export async function adminListCampaigns(params?: {
  status?: string
  target_type?: string
  owner_user_id?: string
  package_id?: string
  limit?: number
  offset?: number
}): Promise<AdminCampaignListResponse> {
  const q = new URLSearchParams()
  if (params?.status) q.append('status', params.status)
  if (params?.target_type) q.append('target_type', params.target_type)
  if (params?.owner_user_id) q.append('owner_user_id', params.owner_user_id)
  if (params?.package_id) q.append('package_id', params.package_id)
  if (params?.limit != null) q.append('limit', String(params.limit))
  if (params?.offset != null) q.append('offset', String(params.offset))
  const qs = q.toString()
  const resp = await api.get<{ data: AdminCampaignListResponse }>(
    `/api/v1/admin/promotions/campaigns${qs ? `?${qs}` : ''}`
  )
  return resp.data
}

/**
 * Force-stop a promotion campaign.
 * POST /api/v1/admin/promotions/campaigns/:id/stop
 */
export async function adminForceStopCampaign(
  id: string,
  reason: string
): Promise<{ message: string }> {
  const resp = await api.post<{ data: { message: string } }>(`/api/v1/admin/promotions/campaigns/${id}/stop`, {
    reason,
  })
  return resp.data
}

/**
 * Get campaign analytics summary.
 * GET /api/v1/admin/promotions/campaigns/:id/analytics
 */
export async function adminGetCampaignAnalytics(
  id: string
): Promise<AdminCampaignAnalyticsResponse> {
  const resp = await api.get<{ data: AdminCampaignAnalyticsResponse }>(
    `/api/v1/admin/promotions/campaigns/${id}/analytics`
  )
  return resp.data
}
