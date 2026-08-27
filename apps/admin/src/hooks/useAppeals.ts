import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'
import type {
  Appeal,
  AppealDetail,
  AppealsResponse,
  AppealDetailResponse,
  AppealReviewRequest,
  AppealReviewResponse,
  AppealsQueryParams,
} from '@/types'

/**
 * Hook for fetching appeals list
 */
export function useAppeals(params: AppealsQueryParams = {}) {
  const [appeals, setAppeals] = useState<Appeal[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [page, setPage] = useState(params.page || 1)
  const [limit, setLimit] = useState(params.limit || 20)
  const [count, setCount] = useState(0)

  const fetchAppeals = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const queryParams = new URLSearchParams()
      if (params.status) queryParams.append('status', params.status)
      queryParams.append('page', page.toString())
      queryParams.append('limit', limit.toString())

      const response = await api.get<{ data: AppealsResponse }>(
        `/api/v1/admin/appeals?${queryParams.toString()}`
      )

      setAppeals(response.data?.appeals || [])
      setCount(response.data?.count ?? 0)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch appeals'))
    } finally {
      setLoading(false)
    }
  }, [params.status, page, limit])

  useEffect(() => {
    fetchAppeals()
  }, [fetchAppeals])

  return {
    appeals,
    loading,
    error,
    page,
    setPage,
    limit,
    setLimit,
    count,
    refetch: fetchAppeals,
  }
}

/**
 * Hook for fetching a single appeal detail
 */
export function useAppeal(appealId: string | null) {
  const [appealDetail, setAppealDetail] = useState<AppealDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const fetchAppeal = useCallback(async () => {
    if (!appealId) return

    setLoading(true)
    setError(null)
    try {
      const response = await api.get<{ data: AppealDetailResponse }>(
        `/api/v1/admin/appeals/${appealId}`
      )
      setAppealDetail(response.data?.appeal ?? null)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch appeal'))
    } finally {
      setLoading(false)
    }
  }, [appealId])

  useEffect(() => {
    fetchAppeal()
  }, [fetchAppeal])

  return {
    appealDetail,
    loading,
    error,
    refetch: fetchAppeal,
  }
}

/**
 * Hook for reviewing appeals
 */
export function useAppealReview() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const reviewAppeal = useCallback(async (
    appealId: string,
    review: AppealReviewRequest
  ): Promise<AppealReviewResponse> => {
    setLoading(true)
    setError(null)
    try {
      const response = await api.put<AppealReviewResponse>(
        `/api/v1/admin/appeals/${appealId}/review`,
        review
      )
      return response
    } catch (err) {
      const error = err instanceof Error ? err : new Error('Failed to review appeal')
      setError(error)
      throw error
    } finally {
      setLoading(false)
    }
  }, [])

  return {
    reviewAppeal,
    loading,
    error,
  }
}
