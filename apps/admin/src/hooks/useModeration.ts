import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'
import { getModerationCases } from '@/lib/api/moderation'
import type {
  ModerationCase,
  ModerationCaseDetail,
  ModerationCaseDetailResponse,
  CaseActionRequest,
  CasesQueryParams,
} from '@/types'

/**
 * Hook for fetching moderation cases list
 */
export function useModerationCases(params: CasesQueryParams = {}) {
  const [cases, setCases] = useState<ModerationCase[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [page, setPage] = useState(params.page || 1)
  const [limit, setLimit] = useState(params.limit || 20)
  const [count, setCount] = useState(0)

  const fetchCases = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getModerationCases({
        status: params.status,
        resource_type: params.resource_type,
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
  }, [params.status, params.resource_type, page, limit])

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
    setLimit,
    count,
    refetch: fetchCases,
  }
}

/**
 * Hook for fetching a single moderation case detail
 */
export function useModerationCase(caseId: string | null) {
  const [caseDetail, setCaseDetail] = useState<ModerationCaseDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const fetchCase = useCallback(async () => {
    if (!caseId) return

    setLoading(true)
    setError(null)
    try {
      const response = await api.get<{ data: ModerationCaseDetailResponse }>(
        `/api/v1/admin/moderation/cases/${caseId}`
      )
      setCaseDetail(response.data?.case ?? null)
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
    caseDetail,
    loading,
    error,
    refetch: fetchCase,
  }
}

/**
 * Hook for executing case actions
 */
export function useCaseAction() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const executeAction = useCallback(async (caseId: string, action: CaseActionRequest) => {
    setLoading(true)
    setError(null)
    try {
      const response = await api.post<ActionResponse>(
        `/api/v1/admin/moderation/cases/${caseId}/action`,
        action
      )
      return response
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to execute action')
      setError(error)
      throw error
    } finally {
      setLoading(false)
    }
  }, [])

  return {
    executeAction,
    loading,
    error,
  }
}

interface ActionResponse {
  case_id: string
  status: string
  action_applied: string
  reviewed_at: string
}
