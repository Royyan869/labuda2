import { useState, useEffect, useCallback } from 'react'
import { getDisputes, getDisputeDetail, approveDispute, rejectDispute } from '@/lib/api'
import type {
  DisputeListItem,
  DisputeDetail,
  DisputesQueryParams,
} from '@/types'

/**
 * Hook for fetching disputes list
 */
export function useDisputes(params: DisputesQueryParams = {}) {
  const [disputes, setDisputes] = useState<DisputeListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [page, setPage] = useState(params.page || 1)
  const [pageSize, setPageSize] = useState(params.page_size || 20)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)

  const fetchDisputes = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getDisputes({
        status: params.status,
        date_from: params.date_from,
        date_to: params.date_to,
        page,
        page_size: pageSize,
      })

      setDisputes((response.disputes as DisputeListItem[]) || [])

      // Extract pagination from response if available
      if (response._meta) {
        setTotal(response._meta.total)
        setTotalPages(response._meta.total_pages)
      }
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch disputes'))
    } finally {
      setLoading(false)
    }
  }, [params.status, params.date_from, params.date_to, page, pageSize])

  useEffect(() => {
    fetchDisputes()
  }, [fetchDisputes])

  // Auto-refresh every 30s for operational awareness
  useEffect(() => {
    const interval = setInterval(() => {
      fetchDisputes()
    }, 30_000)
    return () => clearInterval(interval)
  }, [fetchDisputes])

  return {
    disputes,
    loading,
    error,
    page,
    setPage,
    pageSize,
    setPageSize,
    total,
    totalPages,
    refetch: fetchDisputes,
  }
}

/**
 * Hook for fetching a single dispute detail
 */
export function useDisputeDetail(disputeId: string | null) {
  const [dispute, setDispute] = useState<DisputeDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const fetchDispute = useCallback(async () => {
    if (!disputeId) return

    setLoading(true)
    setError(null)
    try {
      const data = await getDisputeDetail(disputeId)
      setDispute(data as DisputeDetail)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch dispute'))
    } finally {
      setLoading(false)
    }
  }, [disputeId])

  useEffect(() => {
    fetchDispute()
  }, [fetchDispute])

  return {
    dispute,
    loading,
    error,
    refetch: fetchDispute,
  }
}

/**
 * Hook for resolving disputes (approve/reject)
 */
export function useDisputeResolution() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const resolve = useCallback(async (
    disputeId: string,
    action: 'approve' | 'reject',
    notes?: string
  ) => {
    setLoading(true)
    setError(null)
    try {
      if (action === 'approve') {
        await approveDispute(disputeId, notes)
      } else {
        await rejectDispute(disputeId, notes)
      }
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to resolve dispute')
      setError(error)
      throw error
    } finally {
      setLoading(false)
    }
  }, [])

  return {
    resolve,
    loading,
    error,
  }
}
