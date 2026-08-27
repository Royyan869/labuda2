// Promotion admin types — matches backend admin promotion API shapes.

// ─── Packages ─────────────────────────────────────────────────────────────────

export interface PromotionPackage {
  id: string
  name: string
  total_duration_hours: number
  validity_window_hours: number
  price_amount: number
  allowed_target_types: string[]
  is_active: boolean
  created_at: string
}

export interface PromotionPackageListResponse {
  packages: PromotionPackage[]
}

// ─── Campaigns (Instances) ────────────────────────────────────────────────────

export type CampaignStatus = 'inactive' | 'active' | 'paused' | 'expired' | 'cancelled'

export interface AdminCampaign {
  id: string
  ownership_id: string
  user_id: string
  target_type: string
  target_id: string | null
  status: CampaignStatus
  activated_at?: string
  stopped_at?: string
  stop_reason?: string
  created_at: string
  updated_at: string
  // Joined from ownership + package
  package_id: string
  package_name: string
  ownership_total_hours: number
  ownership_consumed_hours: number
}

export interface AdminCampaignListResponse {
  campaigns: AdminCampaign[] 
  total: number
  limit: number
  offset: number
}

export interface AdminCampaignAnalytics {
  instance_id: string
  window_from?: string
  window_to?: string
  impressions_total: number
  clicks_total: number
  ctr: number
  feed_impressions: number
  feed_clicks: number
  search_impressions: number
  search_clicks: number
  explore_impressions: number
  explore_clicks: number
}

export interface AdminCampaignAnalyticsResponse {
  analytics: AdminCampaignAnalytics
}

// ─── Display helpers ──────────────────────────────────────────────────────────

export const campaignStatusLabels: Record<CampaignStatus, string> = {
  inactive: 'Inactive',
  active: 'Active',
  paused: 'Paused',
  expired: 'Expired',
  cancelled: 'Cancelled',
}

export const campaignStatusVariants: Record<
  CampaignStatus,
  'default' | 'pending' | 'success' | 'error' | 'warning' | 'info'
> = {
  inactive: 'default',
  active: 'success',
  paused: 'warning',
  expired: 'info',
  cancelled: 'error',
}

export const targetTypeLabels: Record<string, string> = {
  fixed_price_sale: 'Fixed Price Sale',
  auction: 'Auction',
  external_product: 'External Product',
}
