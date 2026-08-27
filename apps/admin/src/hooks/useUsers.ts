import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'
import type {
  UserListItem,
  UserDetail,
  UsersQueryParams,
  UserActionResponse,
  UserRole,
} from '@/types'

/**
 * Hook for fetching users list
 */
export function useUsers(params: UsersQueryParams = {}) {
  const [users, setUsers] = useState<UserListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [page, setPage] = useState(params.page || 1)
  const [pageSize, setPageSize] = useState(params.page_size || 20)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)

  const fetchUsers = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const queryParams = new URLSearchParams()
      if (params.status) queryParams.append('status', params.status)
      if (params.role) queryParams.append('role', params.role)
      if (params.is_verified) queryParams.append('is_verified', params.is_verified)
      if (params.search) queryParams.append('search', params.search)
      queryParams.append('page', page.toString())
      queryParams.append('page_size', pageSize.toString())

      const response = await api.get<{
        data: { users: UserListItem[] }
        meta?: {
          page: number
          per_page: number
          total: number
          total_pages: number
        }
      }>(
        `/api/v1/admin/users?${queryParams.toString()}`
      )

      setUsers(response.data?.users || [])
      if (response.meta) {
        setTotal(response.meta.total)
        setTotalPages(response.meta.total_pages)
      }
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch users'))
    } finally {
      setLoading(false)
    }
  }, [params.status, params.role, params.is_verified, params.search, page, pageSize])

  useEffect(() => {
    fetchUsers()
  }, [fetchUsers])

  return {
    users,
    loading,
    error,
    page,
    setPage,
    pageSize,
    setPageSize,
    total,
    totalPages,
    refetch: fetchUsers,
  }
}

/**
 * Hook for fetching a single user detail
 */
export function useUserDetail(userId: string | null) {
  const [user, setUser] = useState<UserDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const fetchUser = useCallback(async () => {
    if (!userId) return

    setLoading(true)
    setError(null)
    try {
      const response = await api.get<{ data: UserDetail }>(
        `/api/v1/admin/users/${userId}`
      )
      setUser(response.data ?? null)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch user'))
    } finally {
      setLoading(false)
    }
  }, [userId])

  useEffect(() => {
    fetchUser()
  }, [fetchUser])

  return {
    user,
    loading,
    error,
    refetch: fetchUser,
  }
}

/**
 * Hook for user actions (suspend/activate/ban)
 */
export function useUserActions(userId: string | null) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const suspend = useCallback(async (reason: string, until?: string): Promise<UserActionResponse | null> => {
    if (!userId) {
      setError('No user ID provided')
      return null
    }

    setLoading(true)
    setError(null)
    try {
      const response = await api.post<UserActionResponse>(
        `/api/v1/admin/users/${userId}/suspend`,
        until ? { reason, until } : { reason }
      )
      return response
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to suspend user'
      setError(message)
      return null
    } finally {
      setLoading(false)
    }
  }, [userId])

  const activate = useCallback(async (): Promise<UserActionResponse | null> => {
    if (!userId) {
      setError('No user ID provided')
      return null
    }

    setLoading(true)
    setError(null)
    try {
      const response = await api.post<UserActionResponse>(
        `/api/v1/admin/users/${userId}/activate`,
        {}
      )
      return response
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to activate user'
      setError(message)
      return null
    } finally {
      setLoading(false)
    }
  }, [userId])

  const ban = useCallback(async (reason: string): Promise<UserActionResponse | null> => {
    if (!userId) {
      setError('No user ID provided')
      return null
    }

    setLoading(true)
    setError(null)
    try {
      const response = await api.post<UserActionResponse>(
        `/api/v1/admin/users/${userId}/ban`,
        { reason }
      )
      return response
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to ban user'
      setError(message)
      return null
    } finally {
      setLoading(false)
    }
  }, [userId])

  const unban = useCallback(async (reason: string): Promise<UserActionResponse | null> => {
    if (!userId) {
      setError('No user ID provided')
      return null
    }

    setLoading(true)
    setError(null)
    try {
      const response = await api.post<UserActionResponse>(
        `/api/v1/admin/users/${userId}/unban`,
        { reason }
      )
      return response
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to unban user'
      setError(message)
      return null
    } finally {
      setLoading(false)
    }
  }, [userId])

  const setRole = useCallback(async (role: UserRole): Promise<{ user_id: string; role: string; message: string } | null> => {
    if (!userId) {
      setError('No user ID provided')
      return null
    }

    setLoading(true)
    setError(null)
    try {
      const response = await api.put<{ user_id: string; role: string; message: string }>(
        `/api/v1/admin/users/${userId}/role`,
        { role }
      )
      return response
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to set user role'
      setError(message)
      return null
    } finally {
      setLoading(false)
    }
  }, [userId])

  return {
    suspend,
    activate,
    ban,
    unban,
    setRole,
    loading,
    error,
    clearError: () => setError(null),
  }
}
