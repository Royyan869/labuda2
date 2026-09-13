import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/lib/firebase', () => ({ auth: {} }))
vi.mock('firebase/auth', () => ({ signOut: vi.fn().mockResolvedValue(undefined) }))

import { signOut } from 'firebase/auth'
import { api, logoutAdmin } from './client'
import { useAuthStore } from '@/store/authStore'

const originalLocation = window.location

function stubLocation() {
  try {
    Object.defineProperty(window, 'location', {
      configurable: true,
      writable: true,
      value: { href: '' },
    })
  } catch {
    // jsdom can mark `location` unforgeable; the redirect is then a no-op and
    // invalidation has already happened before it runs.
  }
}

function mockFetch(status: number, body: unknown = {}) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: status >= 200 && status < 300,
      status,
      headers: { get: () => 'application/json' },
      json: () => Promise.resolve(body),
    })
  )
}

function seedCanonicalSession() {
  localStorage.setItem('admin_token', 'stored-credential')
  useAuthStore.setState({
    user: { id: 'admin-1', email: 'admin@labuda.com', username: 'admin', isAdmin: true },
    sessionToken: 'stored-credential',
    isLoading: false,
    error: null,
  })
}

describe('canonical auth invalidation at the credential boundary', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    useAuthStore.setState({ user: null, isLoading: false, error: null, sessionToken: null })
    stubLocation()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    try {
      Object.defineProperty(window, 'location', {
        configurable: true,
        writable: true,
        value: originalLocation,
      })
    } catch {
      // ignore: location is not forgeable in this environment
    }
  })

  it('logoutAdmin clears the credential and the canonical session', async () => {
    seedCanonicalSession()

    await logoutAdmin()

    expect(signOut).toHaveBeenCalled()
    expect(localStorage.getItem('admin_token')).toBeNull()
    expect(useAuthStore.getState().user).toBeNull()
    expect(useAuthStore.getState().sessionToken).toBeNull()
  })

  it('invalidates the canonical session on 401 alongside the redirect', async () => {
    seedCanonicalSession()
    mockFetch(401)

    await expect(api.get('/api/v1/admin/alerts')).rejects.toThrow()

    // The canonical auth-state invalidation must not depend on the reload.
    expect(localStorage.getItem('admin_token')).toBeNull()
    expect(useAuthStore.getState().user).toBeNull()
    expect(useAuthStore.getState().sessionToken).toBeNull()
  })

  it('does not invalidate the canonical session on a successful request', async () => {
    seedCanonicalSession()
    mockFetch(200, { data: { ok: true } })

    await expect(api.get('/api/v1/admin/alerts')).resolves.toEqual({ data: { ok: true } })

    expect(useAuthStore.getState().user?.id).toBe('admin-1')
    expect(useAuthStore.getState().sessionToken).toBe('stored-credential')
  })
})
