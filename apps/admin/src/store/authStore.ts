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

  setUser: (user: AdminUser | null) => void
  setLoading: (isLoading: boolean) => void
  setError: (error: string | null) => void
  signOut: () => void
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: true,
  error: null,

  setUser: (user) => set({ user }),
  setLoading: (isLoading) => set({ isLoading }),
  setError: (error) => set({ error }),
  signOut: () => set({ user: null }),
}))
