import { StrictMode } from 'react'
import { render, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuth } from './useAuth'
import { useAuthStore } from '@/store/authStore'

const apiGetMock = vi.fn()

vi.mock('@/lib/api', () => ({
  api: {
    get: (...args: unknown[]) => apiGetMock(...args),
  },
  // Mirrors the real accessor (localStorage-backed) so the session lifecycle is
  // exercised through the same credential source the app uses.
  getAuthToken: () => localStorage.getItem('admin_token'),
}))

const USERS_ME = '/api/v1/users/me'
const ADMIN_ME = '/api/v1/admin/me'

/**
 * Every test in this file deliberately reuses the SAME credential value. Test
 * isolation must come from the canonical lifecycle reset (store state), never
 * from unique token strings hiding leaked module state.
 */
const TOKEN = 'canonical-session-token'

function countRequests(url: string): number {
  return apiGetMock.mock.calls.filter(([requested]) => requested === url).length
}

/**
 * Canonical identity fixture that matches the REAL backend wire shape.
 *
 * GET /api/v1/users/me returns:
 *   { data: { user: { id, email }, profile: { id, user_id, username, avatar_url } } }
 *
 * The old mock incorrectly put username on data.user. This fixture matches the
 * actual backend DTO structure so tests exercise the real mapping path.
 */
const ADMIN_IDENTITY = {
  user: { id: 'admin-1', email: 'admin@labuda.com' },
  profile: { id: 'profile-1', user_id: 'admin-1', username: 'admin', avatar_url: null as string | null },
}

const ADMIN_ME_DATA = {
  id: 'admin-1',
  email: 'admin@labuda.com',
  username: 'admin',
  role: 'admin',
  is_admin: true,
  capabilities: ['finance.withdraw.read', 'governance.alert.read'],
}

/** Resolves the canonical pair for the given session identity. */
function mockSessionSuccess(
  identity = ADMIN_IDENTITY,
  adminData = ADMIN_ME_DATA,
) {
  apiGetMock.mockImplementation((url: string) => {
    if (url === USERS_ME) {
      return Promise.resolve({ data: identity })
    }
    if (url === ADMIN_ME) {
      return Promise.resolve({ data: adminData })
    }
    return Promise.reject(new Error(`unexpected request: ${url}`))
  })
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function Consumer() {
  useAuth()
  return null
}

function renderConsumers(count: number) {
  return render(
    <>
      {Array.from({ length: count }, (_, index) => (
        <Consumer key={index} />
      ))}
    </>
  )
}

function setSessionToken(token: string) {
  localStorage.setItem('admin_token', token)
}

function resetCanonicalAuth() {
  localStorage.clear()
  useAuthStore.setState({
    user: null,
    isLoading: true,
    error: null,
    sessionToken: null,
  })
}

describe('useAuth canonical session validation authority', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
    resetCanonicalAuth()
  })

  it('A. issues one request pair for many consumers mounted together', async () => {
    mockSessionSuccess()
    setSessionToken(TOKEN)

    renderConsumers(4)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.id).toBe('admin-1')
    })

    expect(countRequests(USERS_ME)).toBe(1)
    expect(countRequests(ADMIN_ME)).toBe(1)
    expect(useAuthStore.getState().sessionToken).toBe(TOKEN)
    expect(useAuthStore.getState().isLoading).toBe(false)
  })

  it('B. does not refetch when a consumer mounts after validation completed', async () => {
    mockSessionSuccess()
    setSessionToken(TOKEN)

    const first = renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.id).toBe('admin-1')
    })
    expect(countRequests(USERS_ME)).toBe(1)

    // Second consumer mounts into a *new* container after hydration finished.
    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().isLoading).toBe(false)
    })

    expect(countRequests(USERS_ME)).toBe(1)
    expect(countRequests(ADMIN_ME)).toBe(1)

    first.unmount()
  })

  it('C. shares one in-flight validation while it is still pending', async () => {
    const usersMe = deferred<unknown>()
    apiGetMock.mockImplementation((url: string) => {
      if (url === USERS_ME) return usersMe.promise
      if (url === ADMIN_ME) {
        return Promise.resolve({
          data: { ...ADMIN_ME_DATA, capabilities: ['finance.withdraw.read'] },
        })
      }
      return Promise.reject(new Error(`unexpected request: ${url}`))
    })
    setSessionToken(TOKEN)

    renderConsumers(4)

    // Only the first leg has been issued: the admin leg follows identity.
    expect(countRequests(USERS_ME)).toBe(1)
    expect(countRequests(ADMIN_ME)).toBe(0)

    usersMe.resolve({ data: ADMIN_IDENTITY })

    await waitFor(() => {
      expect(useAuthStore.getState().user?.id).toBe('admin-1')
    })

    expect(countRequests(USERS_ME)).toBe(1)
    expect(countRequests(ADMIN_ME)).toBe(1)
  })

  it('D. issues one request pair under React StrictMode', async () => {
    mockSessionSuccess()
    setSessionToken(TOKEN)

    render(
      <StrictMode>
        <Consumer />
        <Consumer />
        <Consumer />
      </StrictMode>
    )

    await waitFor(() => {
      expect(useAuthStore.getState().user?.id).toBe('admin-1')
    })

    expect(countRequests(USERS_ME)).toBe(1)
    expect(countRequests(ADMIN_ME)).toBe(1)
  })

  it('E. reaches a terminal error state and never retries on its own', async () => {
    apiGetMock.mockRejectedValue(new Error('network down'))
    setSessionToken(TOKEN)

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().isLoading).toBe(false)
    })
    expect(useAuthStore.getState().user).toBeNull()
    expect(useAuthStore.getState().sessionToken).toBeNull()
    expect(countRequests(USERS_ME)).toBe(1)

    // Idle window: no timer or effect dependency may re-issue the request.
    await new Promise((resolve) => setTimeout(resolve, 60))

    expect(countRequests(USERS_ME)).toBe(1)
    expect(countRequests(ADMIN_ME)).toBe(0)

    // A failure is not cached: an explicit new mount may retry exactly once,
    // which keeps a transient failure recoverable without a reload.
    renderConsumers(1)

    await waitFor(() => {
      expect(countRequests(USERS_ME)).toBe(2)
    })

    await new Promise((resolve) => setTimeout(resolve, 60))
    expect(countRequests(USERS_ME)).toBe(2)
  })

  it('F. does not let an in-flight validation for token A satisfy token B', async () => {
    const firstLeg = deferred<unknown>()
    apiGetMock.mockImplementation((url: string) => {
      if (url === USERS_ME) return firstLeg.promise
      if (url === ADMIN_ME) {
        return Promise.resolve({
          data: { ...ADMIN_ME_DATA, capabilities: [] },
        })
      }
      return Promise.reject(new Error(`unexpected request: ${url}`))
    })

    setSessionToken('token-a')
    renderConsumers(1)
    expect(countRequests(USERS_ME)).toBe(1)

    // The credential is replaced while token A's validation is in flight.
    setSessionToken('token-b')

    firstLeg.resolve({ data: ADMIN_IDENTITY })

    await waitFor(() => {
      expect(useAuthStore.getState().isLoading).toBe(false)
    })

    // Token A's result must never be published for token B.
    expect(useAuthStore.getState().user).toBeNull()
    expect(useAuthStore.getState().sessionToken).toBeNull()

    // Token B must run its OWN validation rather than merely awaiting token A.
    const freshIdentity = {
      user: { id: 'admin-fresh', email: 'fresh@labuda.com' },
      profile: { id: 'p-fresh', user_id: 'admin-fresh', username: 'fresh', avatar_url: null },
    }
    const freshAdminData = {
      id: 'admin-fresh', email: 'fresh@labuda.com', username: 'fresh',
      role: 'admin', is_admin: true, capabilities: [],
    }
    mockSessionSuccess(freshIdentity, freshAdminData)

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.id).toBe('admin-fresh')
    })
    expect(useAuthStore.getState().sessionToken).toBe('token-b')
    expect(countRequests(USERS_ME)).toBe(2)
    expect(countRequests(ADMIN_ME)).toBe(2)
  })

  it('G. invalidates user and sessionToken explicitly, and clears a stale session without a token', async () => {
    mockSessionSuccess()
    setSessionToken(TOKEN)

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.id).toBe('admin-1')
    })
    expect(useAuthStore.getState().sessionToken).toBe(TOKEN)

    // Canonical logout: logoutAdmin() clears the credential and calls signOut().
    localStorage.removeItem('admin_token')
    useAuthStore.getState().signOut()

    expect(useAuthStore.getState().user).toBeNull()
    expect(useAuthStore.getState().sessionToken).toBeNull()
    expect(useAuthStore.getState().error).toBeNull()

    // A stale in-memory session with no credential must also be invalidated on
    // the next mount, with ZERO requests.
    useAuthStore.setState({
      user: { ...ADMIN_IDENTITY.user, username: 'admin', isAdmin: true },
      sessionToken: 'stale-token',
      isLoading: true,
    })

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().isLoading).toBe(false)
    })
    expect(useAuthStore.getState().user).toBeNull()
    expect(useAuthStore.getState().sessionToken).toBeNull()
    expect(countRequests(USERS_ME)).toBe(1)
    expect(countRequests(ADMIN_ME)).toBe(1)
  })

  it('H. revalidates the same credential value after canonical invalidation', async () => {
    mockSessionSuccess()
    setSessionToken(TOKEN)

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.id).toBe('admin-1')
    })
    expect(countRequests(USERS_ME)).toBe(1)

    // The credential string is deliberately reused: the gate is the store-owned
    // lifecycle marker, not a module cache keyed by a unique value. This is the
    // isolation the test suite relies on.
    localStorage.removeItem('admin_token')
    useAuthStore.getState().signOut()
    setSessionToken(TOKEN)

    renderConsumers(1)

    await waitFor(() => {
      expect(countRequests(USERS_ME)).toBe(2)
    })
    expect(countRequests(ADMIN_ME)).toBe(2)
    expect(useAuthStore.getState().sessionToken).toBe(TOKEN)
  })

  it('I. publishes user and sessionToken atomically and exposes capabilities', async () => {
    mockSessionSuccess()
    setSessionToken(TOKEN)

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.id).toBe('admin-1')
    })

    const state = useAuthStore.getState()
    expect(state.user?.isAdmin).toBe(true)
    expect(state.user?.capabilities).toEqual(['finance.withdraw.read', 'governance.alert.read'])
    expect(state.sessionToken).toBe(TOKEN)
  })
})

// ============================================================================
// IDENTITY CONVERGENCE TESTS
//
// Proves that the Admin frontend reads canonical identity from the same
// backend wire shape as Mobile: data.profile.username, data.profile.avatar_url.
// These tests pin the fix for the @40448f54 bug where username was read from
// data.user.username (which does not exist on UserDTO).
// ============================================================================

describe('useAuth identity convergence — canonical username/avatar from profile', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
    resetCanonicalAuth()
  })

  it('J. maps profile.username to AdminUser.username', async () => {
    mockSessionSuccess()
    setSessionToken(TOKEN)

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.username).toBe('admin')
    })

    // Verify the request was made to the canonical endpoint
    expect(countRequests(USERS_ME)).toBe(1)
  })

  it('K. maps profile.avatar_url to AdminUser.avatarUrl when present', async () => {
    const identityWithAvatar = {
      user: { id: 'admin-1', email: 'admin@labuda.com' },
      profile: {
        id: 'profile-1',
        user_id: 'admin-1',
        username: 'admin',
        avatar_url: 'https://cdn.labuda.com/avatars/admin-1.jpg',
      },
    }
    mockSessionSuccess(identityWithAvatar)
    setSessionToken(TOKEN)

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.avatarUrl).toBe(
        'https://cdn.labuda.com/avatars/admin-1.jpg',
      )
    })
  })

  it('L. sets avatarUrl to undefined when profile.avatar_url is null', async () => {
    mockSessionSuccess()
    setSessionToken(TOKEN)

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.username).toBe('admin')
    })

    // avatar_url is null in the default fixture → avatarUrl should be undefined
    expect(useAuthStore.getState().user?.avatarUrl).toBeUndefined()
  })

  it('M. does NOT read username from data.user (UserDTO has no username)', async () => {
    // Explicitly return a response where data.user has NO username field
    // (matching the real backend UserDTO), and profile has the username.
    apiGetMock.mockImplementation((url: string) => {
      if (url === USERS_ME) {
        return Promise.resolve({
          data: {
            user: { id: 'admin-1', email: 'admin@labuda.com' },
            profile: { id: 'p-1', user_id: 'admin-1', username: 'canonical-name', avatar_url: null },
          },
        })
      }
      if (url === ADMIN_ME) {
        return Promise.resolve({
          data: { ...ADMIN_ME_DATA, username: 'canonical-name' },
        })
      }
      return Promise.reject(new Error(`unexpected request: ${url}`))
    })
    setSessionToken(TOKEN)

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.id).toBe('admin-1')
    })

    // Username MUST come from profile, not user
    expect(useAuthStore.getState().user?.username).toBe('canonical-name')
    expect(useAuthStore.getState().user?.id).toBe('admin-1')
  })

  it('N. preserves identity across logout/login cycle', async () => {
    mockSessionSuccess()
    setSessionToken(TOKEN)

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.username).toBe('admin')
    })

    // Logout
    localStorage.removeItem('admin_token')
    useAuthStore.getState().signOut()
    expect(useAuthStore.getState().user).toBeNull()

    // Login again with the same credential
    mockSessionSuccess()
    setSessionToken(TOKEN)

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.username).toBe('admin')
    })

    expect(useAuthStore.getState().user?.id).toBe('admin-1')
    expect(useAuthStore.getState().user?.email).toBe('admin@labuda.com')
  })

  it('O. identity maps correctly when profile has empty username', async () => {
    const identityEmptyUsername = {
      user: { id: 'admin-incomplete', email: 'incomplete@test.com' },
      profile: { id: 'p-inc', user_id: 'admin-incomplete', username: '', avatar_url: null },
    }
    const adminIncomplete = {
      id: 'admin-incomplete', email: 'incomplete@test.com', username: '',
      role: 'admin', is_admin: true, capabilities: [],
    }
    mockSessionSuccess(identityEmptyUsername, adminIncomplete)
    setSessionToken(TOKEN)

    renderConsumers(1)

    await waitFor(() => {
      expect(useAuthStore.getState().user?.id).toBe('admin-incomplete')
    })

    // Empty username is mapped faithfully — not replaced by ID or email
    expect(useAuthStore.getState().user?.username).toBe('')
  })
})
