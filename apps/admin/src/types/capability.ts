/**
 * Capability Management Types
 */

/**
 * Capability category
 */
export type CapabilityCategory =
  | 'Finance'
  | 'Governance'
  | 'Moderation'
  | 'Support'
  | 'Other'

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
  granted_by: string
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
 */
export interface GetUserCapabilitiesResponse {
  user_id: string
  capabilities: UserCapability[]
  total: number
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
 * Capability groups for UI organization
 */
export const CAPABILITY_GROUPS: Record<
  CapabilityCategory,
  { label: string; description: string }
> = {
  Finance: {
    label: 'Finance',
    description: 'Financial operations and withdrawal management',
  },
  Governance: {
    label: 'Governance',
    description: 'User management and system administration',
  },
  Moderation: {
    label: 'Moderation',
    description: 'Content moderation and community management',
  },
  Support: {
    label: 'Support',
    description: 'Customer support and ticket management',
  },
  Other: {
    label: 'Other',
    description: 'Other system permissions',
  },
}
