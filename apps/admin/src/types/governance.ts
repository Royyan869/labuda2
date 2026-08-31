/**
 * Canonical Governance Types
 *
 * Based on the canonical backend API contract:
 *   GET  /admin/governance/cases
 *   GET  /admin/governance/cases/:id
 *   POST /admin/governance/cases/:id/decisions
 *   GET  /admin/governance/decisions/:id
 *   GET  /admin/governance/decisions/:id/enforcement
 *
 * Authority: REPORT_GOVERNANCE_ADMIN_BACKEND_IMPLEMENTATION_SLICE_6.md
 */

// ============================================================================
// CANONICAL ENUMS
// ============================================================================

/** Canonical Case lifecycle — only two values. */
export type GovernanceCaseStatus = 'open' | 'resolved'

/** Canonical Decision outcome — only two values. */
export type DecisionOutcome = 'no_violation' | 'violation'

/** Canonical Enforcement lifecycle — four values. */
export type EnforcementStatus = 'pending' | 'processing' | 'succeeded' | 'failed'

/** Canonical Report target type — five values. */
export type GovernanceTargetType = 'content' | 'comment' | 'for_sale' | 'auction' | 'user'

// ============================================================================
// CASE
// ============================================================================

export interface GovernanceCase {
  id: string
  subject_type: GovernanceTargetType
  subject_id: string
  status: GovernanceCaseStatus
  created_at: string
  updated_at: string
  closed_at?: string
}

// ============================================================================
// REPORT (as returned by Case detail)
// ============================================================================

export interface GovernanceReport {
  id: string
  reporter_id: string
  subject_type: GovernanceTargetType
  subject_id: string
  reason_code: string
  reason_note?: string
  evidence_snapshot?: {
    author_id?: string
    author_username?: string
    title?: string
    text?: string
    status?: string
    content_type?: string
    is_deleted?: boolean
  }
  case_id?: string
  created_at: string
}

// ============================================================================
// DECISION
// ============================================================================

export interface GovernanceDecision {
  id: string
  case_id: string
  decided_by: string
  outcome: DecisionOutcome
  decision_note?: string
  created_at: string
  enforcements?: GovernanceEnforcement[]
}

// ============================================================================
// ENFORCEMENT
// ============================================================================

export interface GovernanceEnforcement {
  id: string
  decision_id: string
  target_type: GovernanceTargetType
  target_id: string
  status: EnforcementStatus
  attempt_count: number
  requested_at: string
  started_at?: string
  finished_at?: string
  last_error?: string
  next_attempt_at?: string
  created_at: string
  updated_at: string
}

// ============================================================================
// AUDIT EVENTS
// ============================================================================

/** Governance audit event as returned by GET /admin/governance/cases/:id/audit */
export interface GovernanceAuditEvent {
  id: string
  event_type: string
  actor_type: string
  actor_id?: string
  actor_name?: string
  outcome?: string
  case_id?: string
  target_type?: string
  target_id?: string
  decision_note?: string
  created_at: string
}

// ============================================================================
// API RESPONSES
// ============================================================================

export interface GovernanceCaseListResponse {
  cases: GovernanceCase[]
  page: number
  limit: number
  count: number
}

export interface GovernanceCaseDetailResponse {
  case: GovernanceCase
  reports: GovernanceReport[]
  decisions: GovernanceDecision[]
}

export interface GovernanceDecisionDetailResponse {
  decision: GovernanceDecision
}

export interface GovernanceEnforcementResponse {
  enforcements: GovernanceEnforcement[]
  message?: string
}

export interface GovernanceAuditResponse {
  events: GovernanceAuditEvent[]
  count: number
}

// ============================================================================
// REQUESTS
// ============================================================================

export interface CreateDecisionRequest {
  outcome: DecisionOutcome
  target_type?: GovernanceTargetType
  target_id?: string
  decision_note?: string
}

// ============================================================================
// LABELS AND CONSTANTS
// ============================================================================

export const GOVERNANCE_CASE_STATUS = {
  OPEN: 'open',
  RESOLVED: 'resolved',
} as const

export const DECISION_OUTCOME = {
  NO_VIOLATION: 'no_violation',
  VIOLATION: 'violation',
} as const

export const ENFORCEMENT_STATUS = {
  PENDING: 'pending',
  PROCESSING: 'processing',
  SUCCEEDED: 'succeeded',
  FAILED: 'failed',
} as const

export const GOVERNANCE_TARGET_TYPE = {
  CONTENT: 'content',
  COMMENT: 'comment',
  FOR_SALE: 'for_sale',
  AUCTION: 'auction',
  USER: 'user',
} as const

// ============================================================================
// LABELS
// ============================================================================

export const caseStatusLabels: Record<GovernanceCaseStatus, string> = {
  open: 'Open',
  resolved: 'Resolved',
}

export const decisionOutcomeLabels: Record<DecisionOutcome, string> = {
  no_violation: 'No Violation',
  violation: 'Violation',
}

export const enforcementStatusLabels: Record<EnforcementStatus, string> = {
  pending: 'Pending',
  processing: 'Processing',
  succeeded: 'Succeeded',
  failed: 'Failed',
}

export const targetTypeLabels: Record<GovernanceTargetType, string> = {
  content: 'Content',
  comment: 'Comment',
  for_sale: 'For Sale',
  auction: 'Auction',
  user: 'User',
}

// ============================================================================
// BADGE VARIANTS
// ============================================================================

export const caseStatusVariants: Record<GovernanceCaseStatus, 'info' | 'success'> = {
  open: 'info',
  resolved: 'success',
}

export const decisionOutcomeVariants: Record<DecisionOutcome, 'warning' | 'success'> = {
  no_violation: 'success',
  violation: 'warning',
}

export const enforcementStatusVariants: Record<EnforcementStatus, 'pending' | 'info' | 'success' | 'error'> = {
  pending: 'pending',
  processing: 'info',
  succeeded: 'success',
  failed: 'error',
}

// ============================================================================
// QUERY PARAMS
// ============================================================================

export interface GovernanceCasesQueryParams {
  status?: GovernanceCaseStatus
  page?: number
  limit?: number
}
