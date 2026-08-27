// ============================================================================
// PLATFORM CONFIG TYPES - Based on GET /api/v1/admin/config
// ============================================================================

/**
 * A single platform config entry.
 *
 * Backend uses response.Success wrapper, so wire shape is:
 *   { success, data: { configs: [...], count }, timestamp }
 *
 * Fields use omitempty — value_numeric, value_text, updated_by may be absent.
 */
export interface PlatformConfigItem {
  key: string
  value_numeric?: string
  value_text?: string
  updated_by?: string
  updated_at: number
}

export interface PlatformConfigResponse {
  configs: PlatformConfigItem[]
  count: number
}

// ============================================================================
// SELLER SUBSCRIPTION CONFIG - Based on GET/PUT /api/v1/admin/seller-subscription-config
// ============================================================================

/**
 * Seller subscription configuration (singleton row in seller_subscription_configs).
 *
 * yearly_fee_rupiah is a Rupiah integer — no conversion needed for IDR display.
 * Example: yearly_fee_rupiah = 70000 → Rp 70,000
 */
export interface SellerSubscriptionConfig {
  id: string
  yearly_fee_rupiah: number
  duration_days: number
  renewal_reminder_days: number
  enabled: boolean
  created_at: string
}

export interface UpdateSellerSubscriptionConfigRequest {
  yearly_fee_rupiah: number
  duration_days: number
  renewal_reminder_days: number
  enabled: boolean
}
