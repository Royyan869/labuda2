/**
 * Moderation Types
 * Based on backend API contracts
 */

// ============================================================================
// ENUMS
// ============================================================================

export type ModerationCaseStatus = 'pending' | 'approved' | 'rejected' | 'enforced'
export type ResourceType = 'content' | 'comment' | 'user' | 'chat_message' | 'fixed_price_sale' | 'auction'
export type AppealStatus = 'pending' | 'approved' | 'rejected'
export type WarningLevel = 'info' | 'warning' | 'severe'
export type CaseAction = 'approve' | 'reject' | 'enforce'
export type AppealDecision = 'approve' | 'reject'

// Status constants to avoid hardcoded strings
export const MODERATION_CASE_STATUS = {
  PENDING: 'pending',
  APPROVED: 'approved',
  REJECTED: 'rejected',
  ENFORCED: 'enforced',
} as const

export const APPEAL_STATUS = {
  PENDING: 'pending',
  APPROVED: 'approved',
  REJECTED: 'rejected',
} as const

export const WARNING_LEVEL = {
  INFO: 'info',
  WARNING: 'warning',
  SEVERE: 'severe',
} as const

// ============================================================================
// MODERATION CASES
// ============================================================================

export interface ModerationCase {
  id: string
  resource_type: ResourceType
  resource_id: string
  status: ModerationCaseStatus
  reported_by: string
  reason: string
  created_at: string
  reviewed_by?: string
  decision_note?: string
  reviewed_at?: string
}

export interface ModerationCaseDetail extends ModerationCase {
  resource_preview?: ResourcePreview
}

export interface ResourcePreview {
  author_id: string
  author_username: string
  title?: string
  status?: string
  content_text?: string
  content_type: string
  is_deleted: boolean
  deleted_at?: string
  deletion_reason?: string
  evidence_available?: boolean
  evidence_requires_capability?: string
  // Chat-message-specific fields (omitted for other resource types)
  room_id?: string
  room_type?: string   // normal, support, negotiation
  sent_at?: string     // ISO 8601 timestamp
}

export interface ModerationCaseEvidence {
  case_id: string
  resource_type: ResourceType
  resource_id: string
  message_id: string
  room_id: string
  room_type: string
  sender_id: string
  author_username?: string
  created_at: string
  deleted_at?: string
  deletion_reason?: string
  original_body?: string | null
  original_attachment?: Record<string, unknown> | null
}

export interface ModerationCaseEvidenceResponse {
  evidence: ModerationCaseEvidence
}

export interface ModerationCasesResponse {
  cases: ModerationCase[]
  page: number
  limit: number
  count: number
}

export interface ModerationCaseDetailResponse {
  case: ModerationCaseDetail
}

export interface CaseActionRequest {
  action: CaseAction
  notes?: string
}

export interface CaseActionResponse {
  case_id: string
  status: ModerationCaseStatus
  action_applied: CaseAction
  reviewed_at: string
}

// ============================================================================
// APPEALS
// ============================================================================

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
  resource_type: ResourceType
  resource_id: string
  status: ModerationCaseStatus
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

// ============================================================================
// WARNINGS
// ============================================================================

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

// ============================================================================
// QUERY PARAMS
// ============================================================================

export interface CasesQueryParams {
  status?: ModerationCaseStatus
  resource_type?: ResourceType
  page?: number
  limit?: number
}

export interface AppealsQueryParams {
  status?: AppealStatus
  page?: number
  limit?: number
}

export interface WarningsQueryParams {
  user_id?: string
  is_active?: boolean
  page?: number
  limit?: number
}

// ============================================================================
// LABELS
// ============================================================================

export const moderationCaseStatusLabels: Record<ModerationCaseStatus, string> = {
  pending: 'Pending',
  approved: 'Approved',
  rejected: 'Rejected',
  enforced: 'Enforced',
}

export const appealStatusLabels: Record<AppealStatus, string> = {
  pending: 'Pending',
  approved: 'Approved',
  rejected: 'Rejected',
}

export const warningLevelLabels: Record<WarningLevel, string> = {
  info: 'Info',
  warning: 'Warning',
  severe: 'Severe',
}

export const resourceTypeLabels: Record<ResourceType, string> = {
  content: 'Content',
  comment: 'Comment',
  user: 'User',
  chat_message: 'Chat Message',
  fixed_price_sale: 'Fixed-Price Sale',
  auction: 'Auction',
}

export const caseActionLabels: Record<CaseAction, string> = {
  approve: 'Approve',
  reject: 'Reject',
  enforce: 'Enforce',
}

// Badge variants for statuses
export const moderationCaseStatusVariants: Record<ModerationCaseStatus, 'pending' | 'success' | 'warning' | 'error'> = {
  pending: 'pending',
  approved: 'success',
  rejected: 'warning',
  enforced: 'error',
}

export const appealStatusVariants: Record<AppealStatus, 'pending' | 'success' | 'warning'> = {
  pending: 'pending',
  approved: 'success',
  rejected: 'warning',
}

export const warningLevelVariants: Record<WarningLevel, 'info' | 'pending' | 'error'> = {
  info: 'info',
  warning: 'pending',
  severe: 'error',
}
