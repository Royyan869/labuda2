/**
 * Capability Management Types
 *
 * The capability universe and its clusters are owned by the backend
 * (capability.AllCapabilities() -> capability.Catalog()). The dashboard must
 * therefore treat categories as OPEN strings and render whatever the API
 * returns — never a hardcoded list, which is how whole clusters used to vanish
 * from the UI.
 */

/**
 * Capability category (cluster), e.g. "Finance", "Promotion", "Support".
 * Open by design: a new backend cluster must render without a frontend change.
 */
export type CapabilityCategory = string

/**
 * A single capability definition
 */
export interface CapabilityDefinition {
  capability: string
  category: CapabilityCategory
  description: string
  critical: boolean
}

/**
 * A user's assigned capability with metadata
 */
export interface UserCapability {
  capability: string
  granted_by?: string | null
  granted_at: string
}

/**
 * Response from GET /api/v1/admin/capabilities
 */
export interface ListCapabilitiesResponse {
  capabilities: CapabilityDefinition[]
}

/**
 * Response from GET /api/v1/admin/users/:id/capabilities
 *
 * full_access is DERIVED by the backend (admin role + active coverage of the
 * entire canonical universe). The dashboard displays it; it must never
 * re-derive it with its own set comparison.
 */
export interface GetUserCapabilitiesResponse {
  user_id: string
  role: 'user' | 'admin'
  is_admin: boolean
  capabilities: UserCapability[]
  total: number
  full_access: boolean
  missing_capabilities: string[]
}

/**
 * Request body for POST /api/v1/admin/users/:id/capabilities
 */
export interface AssignCapabilityRequest {
  capability: string
}

/**
 * Response from assign/revoke capability operations
 */
export interface CapabilityActionResponse {
  message: string
  user_id: string
  capability: string
}

/**
 * Admin user for the admin list
 */
export interface AdminListItem {
  id: string
  display_name: string
  email: string
  account_status: 'active' | 'suspended' | 'banned'
  is_admin: boolean
  created_at: string
  last_active_at?: string
  total_capabilities?: number
}

/**
 * Curated cluster descriptions. Presentation only — a cluster WITHOUT an entry
 * still renders, using its raw name. Missing metadata never hides a capability.
 */
const CAPABILITY_GROUP_DESCRIPTIONS: Record<string, string> = {
  Finance: 'Financial operations and withdrawal management',
  Governance: 'User management and system administration',
  Moderation: 'Content moderation and community management',
  Promotion: 'Promotion packages, campaigns, and external product review',
  Seller: 'Seller verification and subscription operations',
  Order: 'Order management',
  Config: 'Platform configuration',
  Support: 'Customer support and ticket management',
  Other: 'Other system permissions',
}

export function capabilityGroupDescription(category: string): string {
  return CAPABILITY_GROUP_DESCRIPTIONS[category] ?? `${category} capabilities`
}

/**
 * Group capability definitions by their backend-provided category, preserving
 * the canonical order the API returned them in.
 */
export function groupCapabilitiesByCategory(
  capabilities: CapabilityDefinition[]
): Array<[string, CapabilityDefinition[]]> {
  const groups = new Map<string, CapabilityDefinition[]>()
  for (const cap of capabilities) {
    const key = cap.category || 'Other'
    const existing = groups.get(key)
    if (existing) {
      existing.push(cap)
    } else {
      groups.set(key, [cap])
    }
  }
  return Array.from(groups.entries())
}
