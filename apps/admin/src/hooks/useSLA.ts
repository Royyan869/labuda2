import { useState, useEffect, useCallback } from 'react'
import { getSLAMetrics } from '@/lib/api'
import type { SLAMetrics } from '@/types'

/**
 * Hook for fetching SLA metrics
 */
export function useSLAMetrics() {
  const [metrics, setMetrics] = useState<SLAMetrics | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)

  const fetchMetrics = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const metrics = await getSLAMetrics()
      setMetrics(metrics)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch SLA metrics'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchMetrics()
  }, [fetchMetrics])

  return {
    metrics,
    loading,
    error,
    refetch: fetchMetrics,
  }
}
