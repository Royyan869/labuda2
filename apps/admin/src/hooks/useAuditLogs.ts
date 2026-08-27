import { useState, useEffect, useCallback } from 'react'
import { getAuditLogs } from '@/lib/api'
import type {
  AuditLog,
  AuditLogsQueryParams,
  AuditActionType,
  AuditTargetType,
} from '@/types'

/**
 * Hook for fetching audit logs list
 */
export function useAuditLogs(params: AuditLogsQueryParams = {}) {
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [page, setPage] = useState(params.page || 1)
  const [limit, setLimit] = useState(params.page_size || 20)
  const [count, setCount] = useState(0)

  const fetchLogs = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getAuditLogs({
        action: params.action,
        target_type: params.target_type,
        admin_id: params.admin_id,
        page,
        page_size: limit,
      })

      // Convert API response to AuditLog format
      setLogs(response.logs.map((log) => ({
        id: log.id,
        actor_id: log.actor_id,
        action_type: log.action_type as AuditActionType,
        target_type: log.target_type as AuditTargetType,
        target_id: log.target_id,
        metadata: log.metadata,
        created_at: log.created_at,
      })))
      setCount(response._meta?.total || response.logs.length)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch audit logs'))
    } finally {
      setLoading(false)
    }
  }, [params.action, params.target_type, params.admin_id, page, limit])

  useEffect(() => {
    fetchLogs()
  }, [fetchLogs])

  return {
    logs,
    loading,
    error,
    page,
    setPage,
    limit,
    setLimit,
    count,
    refetch: fetchLogs,
  }
}
