/**
 * Canonical Governance API Client
 *
 * Implements the canonical admin governance backend endpoints:
 *   GET  /admin/governance/cases
 *   GET  /admin/governance/cases/:id
 *   POST /admin/governance/cases/:id/decisions
 *   GET  /admin/governance/decisions/:id
 *   GET  /admin/governance/decisions/:id/enforcement
 *
 * Authority: REPORT_GOVERNANCE_ADMIN_BACKEND_IMPLEMENTATION_SLICE_6.md
 */
import { api } from './client'
import type {
  GovernanceCaseListResponse,
  GovernanceCaseDetailResponse,
  GovernanceDecisionDetailResponse,
  GovernanceEnforcementResponse,
  CreateDecisionRequest,
  GovernanceCaseStatus,
} from '@/types/governance'

// ============================================================================
// CASES
// ============================================================================

/**
 * List governance cases with optional status filter and pagination.
 * GET /api/v1/admin/governance/cases
 */
export async function listGovernanceCases(params?: {
  status?: GovernanceCaseStatus
  page?: number
  limit?: number
}): Promise<GovernanceCaseListResponse> {
  const queryParams = new URLSearchParams()
  if (params?.status) queryParams.append('status', params.status)
  queryParams.append('page', String(params?.page ?? 1))
  queryParams.append('limit', String(params?.limit ?? 20))

  const resp = await api.get<{ data: GovernanceCaseListResponse }>(
    `/api/v1/admin/governance/cases?${queryParams.toString()}`
  )
  return resp.data ?? { cases: [], page: 1, limit: 20, count: 0 }
}

/**
 * Get governance case detail with reports, decisions, and enforcement.
 * GET /api/v1/admin/governance/cases/:id
 */
export async function getGovernanceCase(caseId: string): Promise<GovernanceCaseDetailResponse> {
  const resp = await api.get<{ data: GovernanceCaseDetailResponse }>(
    `/api/v1/admin/governance/cases/${caseId}`
  )
  return resp.data
}

// ============================================================================
// DECISIONS
// ============================================================================

/**
 * Create a Decision for a Case through the canonical DecisionService.
 * POST /api/v1/admin/governance/cases/:id/decisions
 *
 * For violation: target_type and target_id are required.
 * For no_violation: target_type and target_id are ignored.
 */
export async function createGovernanceDecision(
  caseId: string,
  request: CreateDecisionRequest
): Promise<{ decision: unknown }> {
  return api.post<{ data: { decision: unknown } }>(
    `/api/v1/admin/governance/cases/${caseId}/decisions`,
    request
  ).then(resp => resp.data)
}

/**
 * Get decision detail.
 * GET /api/v1/admin/governance/decisions/:id
 */
export async function getGovernanceDecision(decisionId: string): Promise<GovernanceDecisionDetailResponse> {
  const resp = await api.get<{ data: GovernanceDecisionDetailResponse }>(
    `/api/v1/admin/governance/decisions/${decisionId}`
  )
  return resp.data
}

/**
 * Get enforcement status for a Decision.
 * GET /api/v1/admin/governance/decisions/:id/enforcement
 */
export async function getGovernanceEnforcement(decisionId: string): Promise<GovernanceEnforcementResponse> {
  const resp = await api.get<{ data: GovernanceEnforcementResponse }>(
    `/api/v1/admin/governance/decisions/${decisionId}/enforcement`
  )
  return resp.data
}
