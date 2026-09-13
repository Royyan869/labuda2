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
 * Outcome of the canonical session validation.
 *
 * - `validated`  : the store session for the current credential is established
 *                  (either it already was, or this call just established it).
 * - `no_session` : there is no stored credential; nothing to validate.
 * - `superseded` : a credential change was observed mid-flight, so the result
 *                  was deliberately not published.
 *
 * Transport/HTTP failures are thrown, never encoded here: the state authority
 * has already been invalidated by the time the error surfaces.
 */
export type SessionValidationResult =
  | { status: 'validated'; user: AdminUser }
  | { status: 'no_session' }
  | { status: 'superseded' }

/**
 * TRANSIENT single-flight transport state — not a lifecycle authority.
 *
 * It holds no decision that outlives the requests it represents: it is keyed by
 * the credential it is validating, it is released as soon as that validation
 * settles, and it is never consulted to decide whether a session is valid
 * (`authStore.sessionToken` owns that decision).
 */
let inFlightValidation: { token: string; promise: Promise<SessionValidationResult> } | null = null

/**
 * The canonical session-validation authority.
 *
 * Fetches identity, then admin authorization metadata, maps them to the single
 * `AdminUser` shape and publishes the result together with the credential it
 * was validated for. Order and error semantics are unchanged from the previous
 * implementation; only the ownership moved here.
 *
 * A credential change observed while the requests are in flight makes the
 * response unpublishable.
 */
async function validateSession(token: string): Promise<SessionValidationResult> {
  useAuthStore.getState().setLoading(true)

  try {
    const userResp = await api.get<{ data: UserMeResponse }>('/api/v1/users/me')
    const identity = userResp.data.user

    const resp = await api.get<{ data: AdminMeResponse }>('/api/v1/admin/me')
    const me = resp.data

    if (getAuthToken() !== token) {
      // The response belongs to a credential that is no longer current.
      return { status: 'superseded' }
    }

    const user: AdminUser = {
      id: identity.id,
      email: identity.email ?? '',
      username: identity.username,
      isAdmin: me.is_admin,
      capabilities: me.capabilities,
    }

    // Atomic publish: the session state and the lifecycle marker are written in
    // one update, so no consumer can observe a user without the credential that
    // produced it.
    useAuthStore.setState({ user, sessionToken: token })
    return { status: 'validated', user }
  } catch (error) {
    if (getAuthToken() !== token) {
      return { status: 'superseded' }
    }
    // Terminal failure: invalidate the canonical session and let the caller
    // decide how to surface it. Nothing is cached and no timer is started, so a
    // single consumer never retries on its own.
    useAuthStore.getState().signOut()
    throw error
  } finally {
    useAuthStore.getState().setLoading(false)
  }
}

/**
 * Ensures the stored credential has been validated once.
 *
 * This is the only way a session gets validated anywhere in the Admin app:
 * `useAuth()` consumers defer to it on mount, and `LoginPage` calls it after
 * the Firebase exchange establishes the credential.
 *
 * Typed as `async` so a rejected shared validation is always delivered to every
 * caller as a rejection they can handle — the promise itself is deduplicated.
 */
export async function ensureSessionValidated(): Promise<SessionValidationResult> {
  const token = getAuthToken()
  const store = useAuthStore.getState()

  if (!token) {
    // No credential: there is no session to validate. Invalidate the canonical
    // state (idempotent) and stop loading. ZERO requests.
    if (store.user !== null || store.sessionToken !== null) {
      store.signOut()
    }
    store.setLoading(false)
    return { status: 'no_session' }
  }

  if (store.sessionToken === token && store.user) {
    // Already validated for this exact credential.
    return { status: 'validated', user: store.user }
  }

  // Join an in-flight validation ONLY for the same credential: a promise for
  // token A must never satisfy a validation request for token B.
  if (inFlightValidation && inFlightValidation.token === token) {
    return inFlightValidation.promise
  }

  const promise = validateSession(token).finally(() => {
    if (inFlightValidation && inFlightValidation.token === token) {
      inFlightValidation = null
    }
  })
  inFlightValidation = { token, promise }
  return promise
}

/**
 * Admin authentication state.
 *
 * Mounting this hook is not an auth lifecycle event: it subscribes to the
 * canonical store and defers to the canonical validation authority, which
 * decides whether the stored credential still needs validating.
 */
export function useAuth() {
  const { user, isLoading, error } = useAuthStore()

  useEffect(() => {
    // Failures are terminal inside the authority; nothing here retries.
    void ensureSessionValidated().catch(() => {})
  }, [])

  return {
    user,
    isLoading,
    error,
    isAdmin: user?.isAdmin ?? false,
    isAuthenticated: !!user,
    capabilities: user?.capabilities ?? [],
  }
}
