/**
 * Moderation Types — Appeal + Warning only
 *
 * Legacy ModerationCase types removed (Slice 9 cleanup).
 * Canonical Case/Decision/Enforcement types live in types/governance.ts.
 */

// ============================================================================
// APPEALS
// ============================================================================

export type AppealStatus = 'pending' | 'approved' | 'rejected'

export const APPEAL_STATUS = {
  PENDING: 'pending',
  APPROVED: 'approved',
  REJECTED: 'rejected',
} as const

export interface Appeal {
  id: string
  report_id: string
  status: AppealStatus
  message: string
  created_at: string
  admin_response?: string
  reviewed_by?: string
  reviewed_at?: string
}

export interface AppealDetail extends Appeal {
  original_case: OriginalCaseContext
}

export interface OriginalCaseContext {
  id: string
  resource_type: string
  resource_id: string
  status: string
  reason: string
  created_at: string
  decision_status: 'approved' | 'dismissed' | 'enforced'
}

export interface AppealsResponse {
  appeals: Appeal[]
  page: number
  limit: number
  count: number
}

export interface AppealDetailResponse {
  appeal: AppealDetail
}

export interface AppealReviewRequest {
  decision: AppealDecision
  admin_response?: string
}

export interface AppealReviewResponse {
  id: string
  status: AppealStatus
  reviewed_at: string
}

export type AppealDecision = 'approve' | 'reject'

export interface AppealsQueryParams {
  status?: AppealStatus
  page?: number
  limit?: number
}

export const appealStatusLabels: Record<AppealStatus, string> = {
  pending: 'Pending',
  approved: 'Approved',
  rejected: 'Rejected',
}

export const appealStatusVariants: Record<AppealStatus, 'pending' | 'success' | 'warning'> = {
  pending: 'pending',
  approved: 'success',
  rejected: 'warning',
}

// ============================================================================
// WARNINGS
// ============================================================================

export type WarningLevel = 'info' | 'warning' | 'severe'

export const WARNING_LEVEL = {
  INFO: 'info',
  WARNING: 'warning',
  SEVERE: 'severe',
} as const

export interface UserWarning {
  id: string
  user_id: string
  level: WarningLevel
  reason: string
  is_active: boolean
  created_at: string
  expires_at?: string
  revoked_at?: string
}

export interface WarningsResponse {
  warnings: UserWarning[]
  page: number
  limit: number
  count: number
}

export interface IssueWarningRequest {
  user_id: string
  level: WarningLevel
  reason: string
  expires_at?: number
}

export interface IssueWarningResponse {
  id: string
  user_id: string
  level: WarningLevel
  reason: string
  is_active: boolean
  created_at: string
  expires_at?: string
}

export interface RevokeWarningResponse {
  id: string
  is_active: boolean
  revoked_at: string
}

export interface WarningsQueryParams {
  user_id?: string
  is_active?: boolean
  page?: number
  limit?: number
}

export const warningLevelLabels: Record<WarningLevel, string> = {
  info: 'Info',
  warning: 'Warning',
  severe: 'Severe',
}

export const warningLevelVariants: Record<WarningLevel, 'info' | 'pending' | 'error'> = {
  info: 'info',
  pending: 'pending',
  severe: 'error',
}
