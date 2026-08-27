import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'
import type {
  WithdrawalListItem,
  WithdrawalDetail,
  WithdrawalsQueryParams,
  WithdrawalActionResponse,
} from '@/types'

/**
 * Hook for fetching withdrawals list
 */
export function useWithdrawals(params: WithdrawalsQueryParams = {}) {
  const [withdrawals, setWithdrawals] = useState<WithdrawalListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [page, setPage] = useState(params.page || 1)
  const [pageSize, setPageSize] = useState(params.page_size || 20)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)

  const fetchWithdrawals = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const queryParams = new URLSearchParams()
      if (params.status) queryParams.append('status', params.status)
      if (params.date_from) queryParams.append('date_from', params.date_from)
      if (params.date_to) queryParams.append('date_to', params.date_to)
      queryParams.append('page', page.toString())
      queryParams.append('page_size', pageSize.toString())

      const response = await api.get<{
        data: { withdrawals: WithdrawalListItem[] }
        meta?: { page: number; per_page: number; total: number; total_pages: number }
      }>(
        `/api/v1/admin/payouts/withdrawals?${queryParams.toString()}`
      )

      setWithdrawals(response.data?.withdrawals || [])
      setTotal(response.meta?.total ?? 0)
      setTotalPages(response.meta?.total_pages ?? (pageSize > 0 ? Math.ceil((response.meta?.total ?? 0) / pageSize) : 0))
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch withdrawals'))
    } finally {
      setLoading(false)
    }
  }, [params.status, params.date_from, params.date_to, page, pageSize])

  useEffect(() => {
    fetchWithdrawals()
  }, [fetchWithdrawals])

  // Auto-refresh every 30s for operational awareness
  useEffect(() => {
    const interval = setInterval(() => {
      fetchWithdrawals()
    }, 30_000)
    return () => clearInterval(interval)
  }, [fetchWithdrawals])

  return {
    withdrawals,
    loading,
    error,
    page,
    setPage,
    pageSize,
    setPageSize,
    total,
    totalPages,
    refetch: fetchWithdrawals,
  }
}

/**
 * Hook for fetching a single withdrawal detail
 */
export function useWithdrawalDetail(withdrawalId: string | null) {
  const [withdrawal, setWithdrawal] = useState<WithdrawalDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const fetchWithdrawal = useCallback(async () => {
    if (!withdrawalId) return

    setLoading(true)
    setError(null)
    try {
      const response = await api.get<{ data: { withdrawal: WithdrawalDetail } }>(
        `/api/v1/admin/payouts/withdrawals/${withdrawalId}`
      )
      setWithdrawal(response.data.withdrawal)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch withdrawal'))
    } finally {
      setLoading(false)
    }
  }, [withdrawalId])

  useEffect(() => {
    fetchWithdrawal()
  }, [fetchWithdrawal])

  return {
    withdrawal,
    loading,
    error,
    refetch: fetchWithdrawal,
  }
}

/**
 * Hook for withdrawal actions (approve/reject/mark-processed)
 */
export function useWithdrawalActions(withdrawalId: string | null) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const approve = useCallback(async (notes?: string): Promise<WithdrawalActionResponse | null> => {
    if (!withdrawalId) {
      setError('No withdrawal ID provided')
      return null
    }

    setLoading(true)
    setError(null)
    try {
      const response = await api.post<WithdrawalActionResponse>(
        `/api/v1/admin/payouts/withdrawals/${withdrawalId}/approve`,
        notes ? { notes } : {}
      )
      return response
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to approve withdrawal'
      setError(message)
      return null
    } finally {
      setLoading(false)
    }
  }, [withdrawalId])

  const reject = useCallback(async (reason: string): Promise<WithdrawalActionResponse | null> => {
    if (!withdrawalId) {
      setError('No withdrawal ID provided')
      return null
    }

    setLoading(true)
    setError(null)
    try {
      const response = await api.post<WithdrawalActionResponse>(
        `/api/v1/admin/payouts/withdrawals/${withdrawalId}/reject`,
        { reason }
      )
      return response
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to reject withdrawal'
      setError(message)
      return null
    } finally {
      setLoading(false)
    }
  }, [withdrawalId])

  const markProcessed = useCallback(async (): Promise<WithdrawalActionResponse | null> => {
    if (!withdrawalId) {
      setError('No withdrawal ID provided')
      return null
    }

    setLoading(true)
    setError(null)
    try {
      const response = await api.post<WithdrawalActionResponse>(
        `/api/v1/admin/payouts/withdrawals/${withdrawalId}/mark-processed`,
        {}
      )
      return response
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to mark withdrawal as processed'
      setError(message)
      return null
    } finally {
      setLoading(false)
    }
  }, [withdrawalId])

  return {
    approve,
    reject,
    markProcessed,
    loading,
    error,
    clearError: () => setError(null),
  }
}
