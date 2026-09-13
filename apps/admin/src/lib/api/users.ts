import { api } from './client'

// ============================================================================
// USERS API
// ============================================================================

/**
 * Get all users with optional filtering
 * GET /api/v1/admin/users
 */
export async function getUsers(params?: {
  status?: string
  role?: string
  is_verified?: string
  search?: string
  page?: number
  page_size?: number
}) {
  const queryParams = new URLSearchParams()
  if (params?.status) queryParams.append('status', params.status)
  if (params?.role) queryParams.append('role', params.role)
  if (params?.is_verified) queryParams.append('is_verified', params.is_verified)
  if (params?.search) queryParams.append('search', params.search)
  queryParams.append('page', String(params?.page ?? 1))
  queryParams.append('page_size', String(params?.page_size ?? 20))

  return api.get(`/api/v1/admin/users?${queryParams.toString()}`)
}

/**
 * Get user detail by ID
 * GET /api/v1/admin/users/:id
 */
export async function getUserDetail(userId: string) {
  return api.get(`/api/v1/admin/users/${userId}`)
}

/**
 * Suspend a user
 * POST /api/v1/admin/users/:id/suspend
 */
export async function suspendUser(userId: string, reason: string, until?: string) {
  return api.post(`/api/v1/admin/users/${userId}/suspend`, until ? { reason, until } : { reason })
}

/**
 * Activate a user (remove suspension)
 * POST /api/v1/admin/users/:id/activate
 */
export async function activateUser(userId: string) {
  return api.post(`/api/v1/admin/users/${userId}/activate`, {})
}

/**
 * Ban a user
 * POST /api/v1/admin/users/:id/ban
 */
export async function banUser(userId: string, reason: string) {
  return api.post(`/api/v1/admin/users/${userId}/ban`, { reason })
}

/**
 * Unban a user (reverses a ban)
 * POST /api/v1/admin/users/:id/unban
 */
export async function unbanUser(userId: string, reason: string) {
  return api.post(`/api/v1/admin/users/${userId}/unban`, { reason })
}

/**
 * Set user role (promote / demote admin membership)
 * PUT /api/v1/admin/users/:id/role
 *
 * Requires governance.role.assign. Canonical roles are exactly user|admin.
 * Demoting the last full-access admin is refused by the backend invariant.
 */
export async function setUserRole(userId: string, role: 'user' | 'admin') {
  return api.put<{ user_id: string; role: string; message: string }>(
    `/api/v1/admin/users/${userId}/role`,
    { role }
  )
}

/**
 * Get user block list (admin view)
 * GET /api/v1/admin/users/:id/blocks
 */
export async function getUserBlocks(userId: string, params?: { limit?: number; cursor?: string }) {
  const queryParams = new URLSearchParams()
  if (params?.limit) queryParams.append('limit', String(params.limit))
  if (params?.cursor) queryParams.append('cursor', params.cursor)
  const qs = queryParams.toString()
  return api.get<{ blocked: string[]; limit: number }>(
    `/api/v1/admin/users/${userId}/blocks${qs ? `?${qs}` : ''}`
  )
}

