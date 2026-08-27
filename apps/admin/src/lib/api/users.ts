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
 * Set user role
 * PUT /api/v1/admin/users/:id/role
 */
export async function setUserRole(userId: string, role: 'user' | 'seller' | 'admin') {
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

// ============================================================================
// BNR STRIKE ADMIN RESET
// ============================================================================

/**
 * Reset all active BNR strikes for a buyer.
 * POST /api/v1/admin/users/:id/bnr-strikes/reset
 * Requires: governance.bnr.reset
 *
 * Sets admin_reset = TRUE on all active strikes. Rows are kept for audit.
 */
export async function resetBNRByUser(userId: string): Promise<{ buyer_id: string; strikes_reset: number }> {
  const resp = await api.post<{ data: { buyer_id: string; strikes_reset: number } }>(
    `/api/v1/admin/users/${userId}/bnr-strikes/reset`,
    {}
  )
  return resp.data
}

/**
 * Reset a single BNR strike by ID.
 * POST /api/v1/admin/bnr-strikes/:strike_id/reset
 * Requires: governance.bnr.reset
 *
 * Returns false if already reset or decayed.
 */
export async function resetBNRStrike(strikeId: string): Promise<{ strike_id: string; reset: boolean }> {
  const resp = await api.post<{ data: { strike_id: string; reset: boolean } }>(
    `/api/v1/admin/bnr-strikes/${strikeId}/reset`,
    {}
  )
  return resp.data
}
