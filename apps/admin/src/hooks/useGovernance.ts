/**
 * Canonical Governance Hooks
 *
 * Hooks for the canonical admin governance workflow:
 *   Case List → Case Detail → Decision Creation → Enforcement Status
 *
 * Authority: REPORT_GOVERNANCE_ADMIN_BACKEND_IMPLEMENTATION_SLICE_6.md
 */
import { useState, useEffect, useCallback } from 'react'
import {
  listGovernanceCases,
  getGovernanceCase,
  createGovernanceDecision,
  getGovernanceCaseAudit,
} from '@/lib/api/governance'
import type {
  GovernanceCase,
  GovernanceCaseDetailResponse,
  GovernanceAuditEvent,
  GovernanceCaseStatus,
  CreateDecisionRequest,
} from '@/types/governance'

// ============================================================================
// CASE LIST HOOK
// ============================================================================

/**
 * Hook for fetching the governance case list.
 * Supports status filter and pagination.
 */
export function useGovernanceCases(params: {
  status?: GovernanceCaseStatus
  page?: number
  limit?: number
} = {}) {
  const [cases, setCases] = useState<GovernanceCase[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [page, setPage] = useState(params.page || 1)
  const [limit] = useState(params.limit || 20)
  const [count, setCount] = useState(0)

  const fetchCases = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await listGovernanceCases({
        status: params.status,
        page,
        limit,
      })
      setCases(response.cases)
      setCount(response.count)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch cases'))
    } finally {
      setLoading(false)
    }
  }, [params.status, page, limit])

  useEffect(() => {
    fetchCases()
  }, [fetchCases])

  return {
    cases,
    loading,
    error,
    page,
    setPage,
    limit,
    count,
    refetch: fetchCases,
  }
}

// ============================================================================
// CASE DETAIL HOOK
// ============================================================================

/**
 * Hook for fetching a single governance case with reports, decisions, and enforcement.
 */
export function useGovernanceCase(caseId: string | null) {
  const [data, setData] = useState<GovernanceCaseDetailResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const fetchCase = useCallback(async () => {
    if (!caseId) return

    setLoading(true)
    setError(null)
    try {
      const response = await getGovernanceCase(caseId)
      setData(response)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch case'))
    } finally {
      setLoading(false)
    }
  }, [caseId])

  useEffect(() => {
    fetchCase()
  }, [fetchCase])

  return {
    data,
    loading,
    error,
    refetch: fetchCase,
  }
}

// ============================================================================
// CREATE DECISION HOOK
// ============================================================================

/**
 * Hook for creating a Decision against a Case.
 * Delegates to POST /admin/governance/cases/:id/decisions.
 */
export function useCreateDecision() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const createDecision = useCallback(async (caseId: string, request: CreateDecisionRequest) => {
    setLoading(true)
    setError(null)
    try {
      const result = await createGovernanceDecision(caseId, request)
      return result
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to create decision')
      setError(error)
      throw error
    } finally {
      setLoading(false)
    }
  }, [])

  return {
    createDecision,
    loading,
    error,
  }
}

// ============================================================================
// CASE AUDIT HOOK
// ============================================================================

/**
 * Hook for fetching governance audit events for a Case.
 * GET /admin/governance/cases/:id/audit
 */
export function useGovernanceCaseAudit(caseId: string | null) {
  const [events, setEvents] = useState<GovernanceAuditEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)
  const [count, setCount] = useState(0)

  const fetchAudit = useCallback(async () => {
    if (!caseId) return

    setLoading(true)
    setError(null)
    try {
      const response = await getGovernanceCaseAudit(caseId)
      setEvents(response.events)
      setCount(response.count)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch audit events'))
    } finally {
      setLoading(false)
    }
  }, [caseId])

  useEffect(() => {
    fetchAudit()
  }, [fetchAudit])

  return {
    events,
    loading,
    error,
    count,
    refetch: fetchAudit,
  }
}
