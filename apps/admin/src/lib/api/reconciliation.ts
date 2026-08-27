import { api } from './client'
import type {
  ReconciliationListResponse,
  ReconciliationResult,
  ReconciliationSeverity,
  ReconciliationAction,
} from '@/types/reconciliation'

// ============================================================================
// RECONCILIATION VISIBILITY API (read-only)
//
// Reconciliation is verification-only (RUNTIME-INVARIANTS §7.1, ADR-002).
// There is intentionally no mutation/repair endpoint here — these are the
// only three routes that exist server-side.
// ============================================================================

/**
 * List persisted reconciliation results (every worker run, passed or not).
 * GET /api/v1/admin/reconciliation
 */
export async function getReconciliationResults(params?: {
  severity?: ReconciliationSeverity | ''
  action_taken?: ReconciliationAction | ''
  auto_repaired?: boolean
  date_from?: string
  date_to?: string
  limit?: number
  offset?: number
}) {
  const queryParams = new URLSearchParams()
  if (params?.severity) queryParams.append('severity', params.severity)
  if (params?.action_taken) queryParams.append('action_taken', params.action_taken)
  if (params?.auto_repaired !== undefined) queryParams.append('auto_repaired', String(params.auto_repaired))
  if (params?.date_from) queryParams.append('date_from', params.date_from)
  if (params?.date_to) queryParams.append('date_to', params.date_to)
  queryParams.append('limit', String(params?.limit ?? 50))
  queryParams.append('offset', String(params?.offset ?? 0))

  return api.get<ReconciliationListResponse>(
    `/api/v1/admin/reconciliation?${queryParams.toString()}`
  )
}

/**
 * Get a single reconciliation result by ID.
 * GET /api/v1/admin/reconciliation/:id
 */
export async function getReconciliationResult(id: string) {
  return api.get<ReconciliationResult>(`/api/v1/admin/reconciliation/${id}`)
}

/**
 * Get the most recent reconciliation run, whatever its severity — including
 * a PASSED result. This is the positive "the worker is alive and healthy"
 * signal that the alert pipeline alone cannot provide.
 * GET /api/v1/admin/reconciliation/latest
 */
export async function getLatestReconciliationResult() {
  return api.get<ReconciliationResult>(`/api/v1/admin/reconciliation/latest`)
}
