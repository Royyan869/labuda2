import { useState, useEffect, useCallback } from 'react'
import {
  listPendingVerifications,
  getVerificationDetail,
  approveVerification,
  rejectVerification,
  requestVerificationResubmission,
  suspendVerification,
  revokeVerification,
  investigateVerification,
  restoreVerification,
  markBankAccountReviewed,
} from '@/lib/api'
import type { SellerVerificationListItem, SellerVerificationDetail } from '@/types'

export function useSellerVerifications(status?: string) {
  const [items, setItems] = useState<SellerVerificationListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)

  const fetchList = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const resp = await listPendingVerifications(status)
      setItems(resp.items)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to load verifications'))
    } finally {
      setLoading(false)
    }
  }, [status])

  useEffect(() => {
    fetchList()
  }, [fetchList])

  // Auto-refresh every 30s for operational awareness
  useEffect(() => {
    const interval = setInterval(() => {
      fetchList()
    }, 30_000)
    return () => clearInterval(interval)
  }, [fetchList])

  return { items, loading, error, refetch: fetchList }
}

export function useVerificationDetail(sellerId: string | null) {
  const [detail, setDetail] = useState<SellerVerificationDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const fetchDetail = useCallback(async (id: string) => {
    setLoading(true)
    setError(null)
    try {
      const resp = await getVerificationDetail(id)
      setDetail(resp)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to load detail'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (sellerId) {
      fetchDetail(sellerId)
    } else {
      setDetail(null)
    }
  }, [sellerId, fetchDetail])

  return { detail, loading, error }
}

export function useVerificationActions() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const clearError = () => setError(null)

  const approve = async (sellerId: string, reason?: string): Promise<boolean> => {
    setLoading(true)
    setError(null)
    try {
      await approveVerification(sellerId, reason)
      return true
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to approve')
      return false
    } finally {
      setLoading(false)
    }
  }

  const reject = async (sellerId: string, reason: string): Promise<boolean> => {
    setLoading(true)
    setError(null)
    try {
      await rejectVerification(sellerId, reason)
      return true
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to reject')
      return false
    } finally {
      setLoading(false)
    }
  }

  const requestResubmission = async (sellerId: string, reason: string): Promise<boolean> => {
    setLoading(true)
    setError(null)
    try {
      await requestVerificationResubmission(sellerId, reason)
      return true
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to request resubmission')
      return false
    } finally {
      setLoading(false)
    }
  }

  const suspend = async (sellerId: string, reason: string): Promise<boolean> => {
    setLoading(true)
    setError(null)
    try {
      await suspendVerification(sellerId, reason)
      return true
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to suspend')
      return false
    } finally {
      setLoading(false)
    }
  }

  const revoke = async (sellerId: string, reason: string): Promise<boolean> => {
    setLoading(true)
    setError(null)
    try {
      await revokeVerification(sellerId, reason)
      return true
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke')
      return false
    } finally {
      setLoading(false)
    }
  }

  const investigate = async (sellerId: string, reason: string): Promise<boolean> => {
    setLoading(true)
    setError(null)
    try {
      await investigateVerification(sellerId, reason)
      return true
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to investigate')
      return false
    } finally {
      setLoading(false)
    }
  }

  const restore = async (sellerId: string, reason?: string): Promise<boolean> => {
    setLoading(true)
    setError(null)
    try {
      await restoreVerification(sellerId, reason)
      return true
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to restore')
      return false
    } finally {
      setLoading(false)
    }
  }

  const markBankReviewed = async (sellerId: string, bankAccountId: string): Promise<boolean> => {
    setLoading(true)
    setError(null)
    try {
      await markBankAccountReviewed(sellerId, bankAccountId)
      return true
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to mark bank account as reviewed')
      return false
    } finally {
      setLoading(false)
    }
  }

  return { approve, reject, requestResubmission, suspend, revoke, investigate, restore, markBankReviewed, loading, error, clearError }
}
