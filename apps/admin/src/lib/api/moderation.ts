import { api } from './client'
import type {
  CaseAction,
  ModerationCase,
  ModerationCaseDetail,
  ModerationCaseEvidence,
  ModerationCaseEvidenceResponse,
  ModerationCaseStatus,
  ResourceType,
} from '@/types'

// ============================================================================
// APPEALS API
// ============================================================================

/**
 * Appeals list response with pagination metadata
 */
export interface PaginatedAppealsResponse {
  appeals: unknown[]
  count?: number
  _meta?: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

/**
 * Get all appeals with optional filtering
 * GET /api/v1/admin/appeals
 */
export async function getAppeals(params?: {
  status?: string
  page?: number
  limit?: number
}) {
  const queryParams = new URLSearchParams()
  if (params?.status) queryParams.append('status', params.status)
  queryParams.append('page', String(params?.page ?? 1))
  queryParams.append('limit', String(params?.limit ?? 20))

  return api.get<PaginatedAppealsResponse>(
    `/api/v1/admin/appeals?${queryParams.toString()}`
  )
}

/**
 * Get appeal detail by ID
 * GET /api/v1/admin/appeals/:id
 */
export async function getAppealDetail(appealId: string) {
  return api.get(`/api/v1/admin/appeals/${appealId}`)
}

/**
 * Review appeal (approve/reject)
 * PUT /api/v1/admin/appeals/:id/review
 */
export async function reviewAppeal(appealId: string, decision: 'approve' | 'reject', notes?: string) {
  return api.put(`/api/v1/admin/appeals/${appealId}/review`, { decision, notes })
}

// ============================================================================
// WARNINGS API
// ============================================================================

/**
 * Warnings list response with pagination metadata
 */
export interface PaginatedWarningsResponse {
  warnings: unknown[]
  count?: number
  _meta?: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

/**
 * Get all warnings with optional filtering
 * GET /api/v1/admin/warnings
 */
export async function getWarnings(params?: {
  user_id?: string
  is_active?: boolean
  page?: number
  limit?: number
}) {
  const queryParams = new URLSearchParams()
  if (params?.user_id) queryParams.append('user_id', params.user_id)
  if (params?.is_active !== undefined) queryParams.append('is_active', params.is_active.toString())
  queryParams.append('page', String(params?.page ?? 1))
  queryParams.append('limit', String(params?.limit ?? 20))

  return api.get<PaginatedWarningsResponse>(
    `/api/v1/admin/warnings?${queryParams.toString()}`
  )
}

/**
 * Issue a warning to a user
 * POST /api/v1/admin/warnings
 */
export async function issueWarning(data: {
  user_id: string
  reason: string
  level: 'info' | 'warning' | 'severe'
  expires_at?: number
}) {
  return api.post('/api/v1/admin/warnings', data)
}

/**
 * Revoke a warning
 * DELETE /api/v1/admin/warnings/:id/revoke
 */
export async function revokeWarning(warningId: string) {
  return api.delete(`/api/v1/admin/warnings/${warningId}/revoke`)
}

// ============================================================================
// MODERATION CASES API
// ============================================================================

/**
 * Moderation cases list response with pagination metadata
 */
export interface PaginatedModerationCasesResponse {
  cases: ModerationCase[]
  count: number
  _meta?: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

/**
 * Get all moderation cases with optional filtering
 * GET /api/v1/admin/moderation/cases
 */
export async function getModerationCases(params?: {
  status?: ModerationCaseStatus
  resource_type?: ResourceType
  page?: number
  limit?: number
}): Promise<PaginatedModerationCasesResponse> {
  const queryParams = new URLSearchParams()
  if (params?.status) queryParams.append('status', params.status)
  if (params?.resource_type) queryParams.append('resource_type', params.resource_type)
  queryParams.append('page', String(params?.page ?? 1))
  queryParams.append('limit', String(params?.limit ?? 20))

  const resp = await api.get<{ data: PaginatedModerationCasesResponse }>(
    `/api/v1/admin/moderation/cases?${queryParams.toString()}`
  )
  return resp.data ?? { cases: [], count: 0 }
}

/**
 * Get moderation case detail by ID
 * GET /api/v1/admin/moderation/cases/:id
 */
export async function getModerationCase(caseId: string) {
  return api.get<{ case: ModerationCaseDetail }>(`/api/v1/admin/moderation/cases/${caseId}`)
}

/**
 * Get hidden evidence for a moderation case by ID.
 * GET /api/v1/admin/moderation/cases/:id/evidence
 */
export async function getModerationCaseEvidence(caseId: string): Promise<ModerationCaseEvidence> {
  const resp = await api.get<{ data: ModerationCaseEvidenceResponse }>(
    `/api/v1/admin/moderation/cases/${caseId}/evidence`
  )

  const evidence = resp.data?.evidence
  if (!evidence) {
    throw new Error('Failed to fetch moderation evidence')
  }

  return evidence
}

/**
 * Execute action on moderation case
 * POST /api/v1/admin/moderation/cases/:id/action
 */
export async function executeCaseAction(caseId: string, action: {
  action: CaseAction
  notes?: string
}): Promise<{ case_id: string; status: ModerationCaseStatus; action_applied: CaseAction; reviewed_at: string }> {
  return api.post<{ case_id: string; status: ModerationCaseStatus; action_applied: CaseAction; reviewed_at: string }>(
    `/api/v1/admin/moderation/cases/${caseId}/action`,
    action
  )
}
