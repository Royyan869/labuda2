import { useState, useEffect, useCallback } from 'react'
import { getFailedDeliveries } from '@/lib/api'

export interface FailedDelivery {
  id: string
  notification_id: string
  recipient_id: string
  channel: string
  status: string
  reason: string
  metadata: Record<string, unknown> | null
  created_at: string
}

export function useFailedDeliveries(params: { since?: string } = {}) {
  const [deliveries, setDeliveries] = useState<FailedDelivery[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)

  const fetchDeliveries = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getFailedDeliveries({
        page,
        pageSize,
        since: params.since,
      })

      setDeliveries(response.deliveries || [])
      setTotal(response.meta?.total ?? 0)
      setTotalPages(response.meta?.total_pages ?? 0)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch delivery failures'))
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, params.since])

  useEffect(() => {
    fetchDeliveries()
  }, [fetchDeliveries])

  return {
    deliveries,
    loading,
    error,
    page,
    setPage,
    total,
    totalPages,
    refetch: fetchDeliveries,
  }
}
