import { create } from 'zustand'

export interface AdminUser {
  id: string
  email: string
  username: string
  isAdmin: boolean
  capabilities?: string[]
}

interface AuthState {
  user: AdminUser | null
  isLoading: boolean
  error: string | null

  /**
   * The exact credential for which `user` was successfully validated by the
   * canonical session-validation authority (`@/hooks/useAuth`).
   *
   * This is the ONE lifecycle marker for the admin session. It lives in the
   * canonical auth state authority so that "the store session is already
   * validated" is a fact about the store itself, not a second cache beside it.
   *
   *   sessionToken === current token  → store session is validated (zero requests)
   *   sessionToken !== current token  → this credential has not established the
   *                                     current session (validation required)
   */
  sessionToken: string | null

  setUser: (user: AdminUser | null) => void
  setLoading: (isLoading: boolean) => void
  setError: (error: string | null) => void
  signOut: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: true,
  error: null,
  sessionToken: null,

  setUser: (user) => set({ user }),
  setLoading: (isLoading) => set({ isLoading }),
  setError: (error) => set({ error }),

  // Canonical auth-state invalidation. The session and the lifecycle marker are
  // cleared together, so no consumer can observe a validated-looking session
  // after logout or a 401 — independent of any page reload.
  signOut: () => set({ user: null, sessionToken: null, error: null }),
}))
