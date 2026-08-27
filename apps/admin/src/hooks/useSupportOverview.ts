import { useState, useEffect, useCallback } from 'react'
import { getSupportAdmins, getSupportStatistics } from '@/lib/api'
import type { SupportAdmin, SupportStats } from '@/lib/api'
export type { SupportStats, SupportAdmin } from '@/lib/api'

export function useSupportStats() {
  const [stats, setStats] = useState<SupportStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)

  const fetch = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getSupportStatistics()
      setStats(response)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch support statistics'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetch()
  }, [fetch])

  return { stats, loading, error, refetch: fetch }
}

export function useSupportAdmins() {
  const [admins, setAdmins] = useState<SupportAdmin[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)

  const fetch = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getSupportAdmins()
      setAdmins(response || [])
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch support admins'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetch()
  }, [fetch])

  return { admins, loading, error, refetch: fetch }
}
