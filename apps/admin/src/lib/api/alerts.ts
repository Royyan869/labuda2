import { api } from './client'

// ============================================================================
// ALERTS API
// ============================================================================

/**
 * Alerts list response with pagination metadata
 */
export interface PaginatedAlertsResponse {
  alerts: Alert[]
  _meta?: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

/**
 * Raw envelope shape returned by GET /api/v1/admin/alerts-v1: backend uses
 * response.SuccessWithMeta, so the payload is under "data" and pagination
 * is a sibling "meta" key, not a "_meta" key inside data.
 */
interface AlertsListEnvelope {
  data: { alerts: Alert[] }
  meta?: { page: number; per_page: number; total: number; total_pages: number }
}

/**
 * Alert types
 */
export type AlertType =
  | 'payment_failure_spike'
  | 'payment_stuck'
  | 'dispute_spike'
  | 'seller_risk'
  | 'coins_anomaly'
  | 'withdrawal_anomaly'
  | 'outbox_dlq_spike'
  | 'outbox_stuck'
  | 'reconciliation_drift'
  | 'refund_gateway_failed'
  | 'stale_dispute_freeze'
  | 'subscription_orphaned_payment'
  | 'subscription_conversion_rate'
  | 'subscription_lifecycle'
  | 'escrow_stuck'
  | 'order_paid_stuck'
  | 'order_shipped_stuck'
  | 'dispute_open_stuck'
  | 'seller_non_shipment'

export type AlertSeverity = 'low' | 'medium' | 'high' | 'critical' | 'warning' | 'info'
export type AlertStatus = 'active' | 'open' | 'acknowledged' | 'resolved' | 'false_positive'

/**
 * Alert entity
 */
export interface Alert {
  id: string
  alert_type: AlertType
  severity: AlertSeverity
  entity_type: string
  entity_id: string
  message: string
  metadata: Record<string, unknown>
  status: AlertStatus
  created_at: string
  updated_at: string
  resolved_at?: string
  resolved_by?: string
  group_key?: string
}

/**
 * Alert stats response
 */
export interface AlertStatsResponse {
  total: number
  active: number
  acknowledged: number
  resolved: number
  false_positive: number
  by_severity: Record<string, number>
  by_type: Record<string, number>
  recent_alerts?: Alert[]
}

/**
 * Get all alerts with optional filtering
 * GET /api/v1/admin/alerts
 */
export async function getAlerts(params?: {
  status?: AlertStatus | ''
  severity?: AlertSeverity | ''
  alert_type?: AlertType | ''
  entity_type?: string
  entity_id?: string
  date_from?: string
  date_to?: string
  page?: number
  page_size?: number
}) {
  const queryParams = new URLSearchParams()
  if (params?.status) queryParams.append('status', params.status)
  if (params?.severity) queryParams.append('severity', params.severity)
  if (params?.alert_type) queryParams.append('alert_type', params.alert_type)
  if (params?.entity_type) queryParams.append('entity_type', params.entity_type)
  if (params?.entity_id) queryParams.append('entity_id', params.entity_id)
  if (params?.date_from) queryParams.append('date_from', params.date_from)
  if (params?.date_to) queryParams.append('date_to', params.date_to)
  queryParams.append('page', String(params?.page ?? 1))
  queryParams.append('page_size', String(params?.page_size ?? 20))

  const resp = await api.get<AlertsListEnvelope>(
    `/api/v1/admin/alerts-v1?${queryParams.toString()}`
  )
  return {
    alerts: resp.data?.alerts ?? [],
    _meta: resp.meta,
  }
}

/**
 * Get alert detail by ID
 * GET /api/v1/admin/alerts/:id
 */
export async function getAlertDetail(alertId: string) {
  const resp = await api.get<{ data: Alert }>(`/api/v1/admin/alerts-v1/${alertId}`)
  return resp.data
}

/**
 * Get alert statistics
 * GET /api/v1/admin/alerts/stats
 */
export async function getAlertStats() {
  const resp = await api.get<{ data: AlertStatsResponse }>(`/api/v1/admin/alerts-v1/stats`)
  return resp.data
}

/**
 * Acknowledge an alert
 * POST /api/v1/admin/alerts/:id/acknowledge
 */
export async function acknowledgeAlert(alertId: string, reason?: string) {
  return api.post(`/api/v1/admin/alerts-v1/${alertId}/acknowledge`, reason ? { reason } : {})
}

/**
 * Resolve an alert
 * POST /api/v1/admin/alerts/:id/resolve
 */
export async function resolveAlert(alertId: string, reason?: string) {
  return api.post(`/api/v1/admin/alerts-v1/${alertId}/resolve`, reason ? { reason } : {})
}

/**
 * Mark an alert as false positive
 * POST /api/v1/admin/alerts/:id/false-positive
 */
export async function markAlertAsFalsePositive(alertId: string, reason?: string) {
  return api.post(`/api/v1/admin/alerts-v1/${alertId}/false-positive`, reason ? { reason } : {})
}

/**
 * Cleanup old resolved alerts
 * POST /api/v1/admin/alerts/cleanup
 */
export async function cleanupAlerts(retentionDays?: number) {
  const queryParams = retentionDays
    ? `?retention_days=${retentionDays}`
    : ''
  return api.post<{ data: { deleted: number } }>(`/api/v1/admin/alerts-v1/cleanup${queryParams}`, {})
}
