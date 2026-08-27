import { api } from './client'

// ============================================================================
// AUDIT LOGS API
// ============================================================================

/**
 * Audit logs list response with pagination metadata
 */
export interface PaginatedAuditLogsResponse {
  logs: AuditLogEntry[]
  _meta?: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

/**
 * A single audit log entry
 */
export interface AuditLogEntry {
  id: string
  actor_id: string
  action_type: string
  target_type: string
  target_id: string
  metadata?: Record<string, unknown>
  created_at: string
}

/**
 * Get all audit logs with optional filtering
 * GET /api/v1/admin/audit-logs
 *
 * Backend uses response.SuccessWithMeta: { success, data: { logs }, meta, timestamp }
 */
export async function getAuditLogs(params?: {
  action?: string
  target_type?: string
  admin_id?: string
  target_id?: string
  page?: number
  page_size?: number
}): Promise<PaginatedAuditLogsResponse> {
  const queryParams = new URLSearchParams()
  if (params?.action) queryParams.append('action', params.action)
  if (params?.target_type) queryParams.append('target_type', params.target_type)
  if (params?.admin_id) queryParams.append('admin_id', params.admin_id)
  if (params?.target_id) queryParams.append('target_id', params.target_id)
  queryParams.append('page', String(params?.page ?? 1))
  queryParams.append('page_size', String(params?.page_size ?? 20))

  const resp = await api.get<{
    data: { logs: AuditLogEntry[] }
    meta?: { page: number; per_page: number; total: number; total_pages: number }
  }>(`/api/v1/admin/audit-logs?${queryParams.toString()}`)

  return {
    logs: resp.data?.logs ?? [],
    _meta: resp.meta,
  }
}

// ============================================================================
// ADMIN ME API
// ============================================================================

/**
 * Get current admin user info
 * GET /api/v1/admin/me
 */
export async function getAdminMe() {
  return api.get('/api/v1/admin/me')
}

// ============================================================================
// CAPABILITIES API
// ============================================================================

/**
 * List all available capabilities
 * GET /api/v1/admin/capabilities
 */
export async function getCapabilities() {
  return api.get<{ capabilities: Array<{
    capability: string
    category: string
    description: string
    critical: boolean
  }>}>('/api/v1/admin/capabilities')
}

/**
 * Get user capabilities
 * GET /api/v1/admin/users/:id/capabilities
 */
export async function getUserCapabilities(userId: string) {
  return api.get<{
    user_id: string
    capabilities: Array<{
      capability: string
      granted_by: string
      granted_at: string
    }>
    total: number
  }>(`/api/v1/admin/users/${userId}/capabilities`)
}

/**
 * Assign capability to user
 * POST /api/v1/admin/users/:id/capabilities
 */
export async function assignCapability(userId: string, capability: string) {
  return api.post<{
    message: string
    user_id: string
    capability: string
  }>(`/api/v1/admin/users/${userId}/capabilities`, { capability })
}

/**
 * Revoke capability from user
 * DELETE /api/v1/admin/users/:id/capabilities/:cap
 */
export async function revokeCapability(userId: string, capability: string) {
  return api.delete<{
    message: string
    user_id: string
    capability: string
  }>(`/api/v1/admin/users/${userId}/capabilities/${encodeURIComponent(capability)}`)
}

// ============================================================================
// PLATFORM CONFIG API
// ============================================================================

/**
 * Get all platform config values (read-only)
 * GET /api/v1/admin/config
 *
 * Backend uses response.Success wrapper: { success, data: { configs, count }, timestamp }
 */
export async function getPlatformConfigs() {
  const resp = await api.get<{
    data: import('@/types/platform-config').PlatformConfigResponse
  }>('/api/v1/admin/config')
  return resp.data
}

// ============================================================================
// SELLER SUBSCRIPTION CONFIG API
// ============================================================================

/**
 * Get the active seller subscription config (singleton row).
 * GET /api/v1/admin/seller-subscription-config
 * Requires: config.view
 *
 * yearly_fee_rupiah is a Rupiah integer (70000 = Rp 70,000).
 */
export async function getSellerSubscriptionConfig() {
  const resp = await api.get<{
    data: { config: import('@/types/platform-config').SellerSubscriptionConfig }
  }>('/api/v1/admin/seller-subscription-config')
  return resp.data.config
}

/**
 * Update the active seller subscription config.
 * PUT /api/v1/admin/seller-subscription-config
 * Requires: config.update.financial
 *
 * yearly_fee_rupiah must be a Rupiah integer (e.g., 70000 for Rp 70,000).
 */
export async function updateSellerSubscriptionConfig(
  payload: import('@/types/platform-config').UpdateSellerSubscriptionConfigRequest
) {
  const resp = await api.put<{
    data: { config?: import('@/types/platform-config').SellerSubscriptionConfig; message: string }
  }>('/api/v1/admin/seller-subscription-config', payload)
  return resp.data
}

/**
 * Update a single platform config key.
 * PUT /api/v1/admin/config/:key
 * Requires: config.update.general or config.update.financial (key-dependent)
 *
 * value is always a string; backend parses as numeric if valid decimal.
 */
export async function updatePlatformConfig(key: string, value: string) {
  const resp = await api.put<{
    data: { config: import('@/types/platform-config').PlatformConfigItem; message: string }
  }>(`/api/v1/admin/config/${encodeURIComponent(key)}`, { value })
  return resp.data
}

// ============================================================================
// DELIVERY MONITORING API
// ============================================================================

export async function getFailedDeliveries(params?: {
  page?: number
  pageSize?: number
  since?: string
}) {
  const queryParams = new URLSearchParams()
  queryParams.append('page', String(params?.page ?? 1))
  queryParams.append('page_size', String(params?.pageSize ?? 20))
  if (params?.since) queryParams.append('since', params.since)

  const resp = await api.get<{
    data: {
      deliveries: Array<{
        id: string
        notification_id: string
        recipient_id: string
        channel: string
        status: string
        reason: string
        metadata: Record<string, unknown> | null
        created_at: string
      }>
    }
    meta: {
      page: number
      per_page: number
      total: number
      total_pages: number
    }
  }>(`/api/v1/admin/notifications/failed-deliveries?${queryParams.toString()}`)

  return {
    deliveries: resp.data.deliveries ?? [],
    meta: resp.meta ?? { page: 1, per_page: 20, total: 0, total_pages: 0 },
  }
}

// ============================================================================
// SLA METRICS API
// ============================================================================

/**
 * Get SLA metrics for support and disputes
 * GET /api/v1/admin/sla/metrics
 *
 * Backend handler wraps the payload in its own {"data": ...} gin.H *and*
 * response.Success wraps that again, so the real body is doubly nested:
 * { success, data: { data: <metrics> }, timestamp }. Unwrap both levels
 * here so callers receive the metrics object directly.
 */
export async function getSLAMetrics() {
  const resp = await api.get<{
    data: {
    data: {
      support: {
        avg_first_response_time: number | null
        p95_first_response_time: number | null
        avg_resolution_time: number | null
        p95_resolution_time: number | null
        overdue_rate: number
        overdue_count: number
        total_count: number
        active_count: number
        resolved_count: number
        health_status: 'good' | 'warning' | 'critical'
      } | null
      dispute: {
        avg_first_response_time: number | null
        p95_first_response_time: number | null
        avg_resolution_time: number | null
        p95_resolution_time: number | null
        overdue_rate: number
        overdue_count: number
        total_count: number
        active_count: number
        resolved_count: number
        health_status: 'good' | 'warning' | 'critical'
      } | null
      admin_performance: Array<{
        admin_id: string
        avg_response_time: number | null
        p95_response_time: number | null
        avg_resolution_time: number | null
        p95_resolution_time: number | null
        overdue_count: number
        overdue_rate: number
        handled_tickets: number
        active_workload: number
        health_status: 'good' | 'warning' | 'critical'
      }>
      system_health: {
        status: 'good' | 'warning' | 'critical'
        score: number
        issues: string[]
      }
      trends: {
        last_24_hours: {
          avg_first_response_time: number | null
          p95_first_response_time: number | null
          avg_resolution_time: number | null
          p95_resolution_time: number | null
          overdue_rate: number
          overdue_count: number
          total_count: number
          active_count: number
          resolved_count: number
          health_status: 'good' | 'warning' | 'critical'
        } | null
        previous_24_hours: {
          avg_first_response_time: number | null
          p95_first_response_time: number | null
          avg_resolution_time: number | null
          p95_resolution_time: number | null
          overdue_rate: number
          overdue_count: number
          total_count: number
          active_count: number
          resolved_count: number
          health_status: 'good' | 'warning' | 'critical'
        } | null
        response_time_change: number
        resolution_time_change: number
        overdue_rate_change: number
      } | null
      generated_at: string
    }
    }
  }>('/api/v1/admin/sla/metrics')
  return resp.data.data
}
