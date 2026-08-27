import { useState, useEffect, useCallback } from 'react'
import {
  getCapabilities,
  getUserCapabilities,
  assignCapability,
  revokeCapability,
} from '@/lib/api'
import type {
  CapabilityDefinition,
  UserCapability,
} from '@/types/capability'

/**
 * Hook for fetching all available capabilities
 */
export function useCapabilities() {
  const [capabilities, setCapabilities] = useState<CapabilityDefinition[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)

  const fetchCapabilities = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await getCapabilities()
      setCapabilities((response.capabilities || []) as CapabilityDefinition[])
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch capabilities'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchCapabilities()
  }, [fetchCapabilities])

  return {
    capabilities,
    loading,
    error,
    refetch: fetchCapabilities,
  }
}

/**
 * Hook for fetching a user's capabilities
 */
export function useUserCapabilities(userId: string | null) {
  const [userCapabilities, setUserCapabilities] = useState<UserCapability[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [total, setTotal] = useState(0)

  const fetchUserCapabilities = useCallback(async () => {
    if (!userId) return

    setLoading(true)
    setError(null)
    try {
      const response = await getUserCapabilities(userId)
      setUserCapabilities(response.capabilities || [])
      setTotal(response.total || 0)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch user capabilities'))
    } finally {
      setLoading(false)
    }
  }, [userId])

  useEffect(() => {
    fetchUserCapabilities()
  }, [fetchUserCapabilities])

  return {
    userCapabilities,
    loading,
    error,
    total,
    refetch: fetchUserCapabilities,
  }
}

/**
 * Hook for capability management (assign/revoke)
 */
export function useCapabilityActions(userId: string | null) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const assign = useCallback(async (capability: string): Promise<boolean> => {
    if (!userId) {
      setError('No user ID provided')
      return false
    }

    setLoading(true)
    setError(null)
    try {
      await assignCapability(userId, capability)
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to assign capability'
      setError(message)
      return false
    } finally {
      setLoading(false)
    }
  }, [userId])

  const revoke = useCallback(async (capability: string): Promise<boolean> => {
    if (!userId) {
      setError('No user ID provided')
      return false
    }

    setLoading(true)
    setError(null)
    try {
      await revokeCapability(userId, capability)
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to revoke capability'
      setError(message)
      return false
    } finally {
      setLoading(false)
    }
  }, [userId])

  return {
    assign,
    revoke,
    loading,
    error,
    clearError: () => setError(null),
  }
}
