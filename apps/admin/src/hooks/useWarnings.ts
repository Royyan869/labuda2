import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'
import type {
  UserWarning,
  WarningsResponse,
  IssueWarningRequest,
  IssueWarningResponse,
  RevokeWarningResponse,
  WarningsQueryParams,
} from '@/types'

/**
 * Hook for fetching warnings list
 */
export function useWarnings(params: WarningsQueryParams = {}) {
  const [warnings, setWarnings] = useState<UserWarning[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [page, setPage] = useState(params.page || 1)
  const [limit, setLimit] = useState(params.limit || 20)
  const [count, setCount] = useState(0)

  const fetchWarnings = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const queryParams = new URLSearchParams()
      if (params.user_id) queryParams.append('user_id', params.user_id)
      if (params.is_active !== undefined) queryParams.append('is_active', params.is_active.toString())
      queryParams.append('page', page.toString())
      queryParams.append('limit', limit.toString())

      const response = await api.get<{ data: WarningsResponse }>(
        `/api/v1/admin/warnings?${queryParams.toString()}`
      )

      setWarnings(response.data?.warnings || [])
      setCount(response.data?.count ?? 0)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch warnings'))
    } finally {
      setLoading(false)
    }
  }, [params.user_id, params.is_active, page, limit])

  useEffect(() => {
    fetchWarnings()
  }, [fetchWarnings])

  return {
    warnings,
    loading,
    error,
    page,
    setPage,
    limit,
    setLimit,
    count,
    refetch: fetchWarnings,
  }
}

/**
 * Hook for issuing warnings
 */
export function useIssueWarning() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const issueWarning = useCallback(async (
    data: IssueWarningRequest
  ): Promise<IssueWarningResponse> => {
    setLoading(true)
    setError(null)
    try {
      const response = await api.post<IssueWarningResponse>(
        '/api/v1/admin/warnings',
        data
      )
      return response
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to issue warning')
      setError(error)
      throw error
    } finally {
      setLoading(false)
    }
  }, [])

  return {
    issueWarning,
    loading,
    error,
  }
}

/**
 * Hook for revoking warnings
 */
export function useRevokeWarning() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const revokeWarning = useCallback(async (
    warningId: string
  ): Promise<RevokeWarningResponse> => {
    setLoading(true)
    setError(null)
    try {
      const response = await api.delete<RevokeWarningResponse>(
        `/api/v1/admin/warnings/${warningId}/revoke`
      )
      return response
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to revoke warning')
      setError(error)
      throw error
    } finally {
      setLoading(false)
    }
  }, [])

  return {
    revokeWarning,
    loading,
    error,
  }
}
