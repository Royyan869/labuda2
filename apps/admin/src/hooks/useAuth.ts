import { useEffect } from 'react'
import { api, getAuthToken } from '@/lib/api'
import { useAuthStore, type AdminUser } from '@/store/authStore'

interface AdminMeResponse {
  id: string
  email: string
  username: string
  role: string
  is_admin: boolean
  capabilities: string[]
}

interface UserMeResponse {
  user: {
    id: string
    email?: string | null
    username: string
  }
}

/**
 * Custom hook for authentication.
 * On mount, validates the stored Firebase ID token by loading /users/me
 * first and then confirming admin capability via /admin/me.
 */
export function useAuth() {
  const { user, isLoading, error, setUser, setLoading } = useAuthStore()

  useEffect(() => {
    const validateAuth = async () => {
      const token = getAuthToken()

      if (!token) {
        setLoading(false)
        return
      }

      try {
        const userResp = await api.get<{ data: UserMeResponse }>('/api/v1/users/me')
        const identity = userResp.data.user

        const resp = await api.get<{ data: AdminMeResponse }>('/api/v1/admin/me')
        const me = resp.data
        const adminUser: AdminUser = {
          id: identity.id,
          email: identity.email ?? '',
          username: identity.username,
          isAdmin: me.is_admin,
          capabilities: me.capabilities,
        }
        setUser(adminUser)
      } catch {
        // Token invalid, expired, or not an admin
        setUser(null)
      } finally {
        setLoading(false)
      }
    }

    validateAuth()
  }, [setUser, setLoading])

  return {
    user,
    isLoading,
    error,
    isAdmin: user?.isAdmin ?? false,
    isAuthenticated: !!user,
    capabilities: user?.capabilities ?? [],
  }
}
