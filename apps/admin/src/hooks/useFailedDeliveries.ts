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

export function useFailedDeliveries(params: { sinceHours?: number } = {}) {
  // The lookback window is a scalar input and is resolved to a timestamp at
  // request time. It must NOT be computed during render and passed in as an
  // ISO string: a render-time `new Date()` yields a new value on every
  // render, which changed fetchDeliveries' identity every render and made the
  // effect below refetch forever (the "Loading failed deliveries..." spinner
  // never reached a terminal state).
  const sinceHours = params.sinceHours ?? 24

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
      const since = new Date(Date.now() - sinceHours * 60 * 60 * 1000).toISOString()
      const response = await getFailedDeliveries({
        page,
        pageSize,
        since,
      })

      setDeliveries(response.deliveries || [])
      setTotal(response.meta?.total ?? 0)
      setTotalPages(response.meta?.total_pages ?? 0)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch delivery failures'))
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, sinceHours])

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
