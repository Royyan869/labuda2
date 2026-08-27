import { useState, useEffect, useCallback } from 'react'
import {
  getAlerts,
  getAlertStats,
  acknowledgeAlert,
  resolveAlert,
  markAlertAsFalsePositive,
  cleanupAlerts,
  type Alert,
  type AlertStatus,
  type AlertSeverity,
  type AlertType,
  type AlertStatsResponse,
} from '@/lib/api'
import type { ApiError } from '@/lib/api'

interface UseAlertsParams {
  status?: AlertStatus | ''
  severity?: AlertSeverity | ''
  alert_type?: AlertType | ''
  entity_type?: string
  entity_id?: string
  date_from?: string
  date_to?: string
  page?: number
  page_size?: number
}

interface UseAlertsResult {
  alerts: Alert[]
  loading: boolean
  error: Error | null
  count: number
  totalPages: number
  refetch: () => Promise<void>
}

export function useAlerts(params: UseAlertsParams = {}): UseAlertsResult {
  const [alerts, setAlerts] = useState<Alert[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [count, setCount] = useState(0)
  const [totalPages, setTotalPages] = useState(0)

  const fetchAlerts = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getAlerts(params)
      setAlerts(response.alerts ?? [])
      setCount(response._meta?.total ?? 0)
      setTotalPages(response._meta?.total_pages ?? 0)
    } catch (err) {
      const error = err as ApiError
      setError(new Error(error.message))
    } finally {
      setLoading(false)
    }
  }, [params])

  useEffect(() => {
    fetchAlerts()
  }, [fetchAlerts])

  return {
    alerts,
    loading,
    error,
    count,
    totalPages,
    refetch: fetchAlerts,
  }
}

interface UseAlertStatsResult {
  stats: AlertStatsResponse | null
  loading: boolean
  error: Error | null
  refetch: () => Promise<void>
}

export function useAlertStats(): UseAlertStatsResult {
  const [stats, setStats] = useState<AlertStatsResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)

  const fetchStats = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getAlertStats()
      setStats(response)
    } catch (err) {
      const error = err as ApiError
      setError(new Error(error.message))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchStats()
  }, [fetchStats])

  return {
    stats,
    loading,
    error,
    refetch: fetchStats,
  }
}

interface UseAlertActionsResult {
  acknowledge: (alertId: string, reason?: string) => Promise<void>
  resolve: (alertId: string, reason?: string) => Promise<void>
  markFalsePositive: (alertId: string, reason?: string) => Promise<void>
  cleanup: (retentionDays?: number) => Promise<number>
  loading: boolean
  error: Error | null
}

export function useAlertActions(): UseAlertActionsResult {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const acknowledge = useCallback(async (alertId: string, reason?: string) => {
    setLoading(true)
    setError(null)
    try {
      await acknowledgeAlert(alertId, reason)
    } catch (err) {
      const error = err as ApiError
      setError(new Error(error.message))
      throw error
    } finally {
      setLoading(false)
    }
  }, [])

  const resolve = useCallback(async (alertId: string, reason?: string) => {
    setLoading(true)
    setError(null)
    try {
      await resolveAlert(alertId, reason)
    } catch (err) {
      const error = err as ApiError
      setError(new Error(error.message))
      throw error
    } finally {
      setLoading(false)
    }
  }, [])

  const markFalsePositive = useCallback(async (alertId: string, reason?: string) => {
    setLoading(true)
    setError(null)
    try {
      await markAlertAsFalsePositive(alertId, reason)
    } catch (err) {
      const error = err as ApiError
      setError(new Error(error.message))
      throw error
    } finally {
      setLoading(false)
    }
  }, [])

  const cleanup = useCallback(async (retentionDays?: number) => {
    setLoading(true)
    setError(null)
    try {
      const response = await cleanupAlerts(retentionDays)
      return response.data.deleted as number
    } catch (err) {
      const error = err as ApiError
      setError(new Error(error.message))
      throw error
    } finally {
      setLoading(false)
    }
  }, [])

  return {
    acknowledge,
    resolve,
    markFalsePositive,
    cleanup,
    loading,
    error,
  }
}
