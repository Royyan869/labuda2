/**
 * Audit Log Types
 * Based on backend API contracts for admin audit logs
 */

// ============================================================================
// ACTION TYPES
// ============================================================================

/**
 * All possible admin action types that can be logged
 */
export type AuditActionType =
  | 'withdraw_approved'
  | 'withdraw_rejected'
  | 'withdraw_processed'
  | 'dispute_resolved_approved'
  | 'dispute_resolved_rejected'
  | 'role_changed'
  | 'account_status_changed'
  | 'admin_users_listed'
  | 'admin_user_viewed'
  | 'admin_dashboard_viewed'
  | 'admin_audit_logs_viewed'
  | 'admin_metrics_viewed'
  | 'admin_alerts_viewed'
  | 'user_suspended'
  | 'user_activated'
  | 'user_banned'
  // Add more as backend adds new actions

/**
 * All possible target types for audit logs
 */
export type AuditTargetType =
  | 'user'
  | 'withdrawal'
  | 'dispute'
  | 'refund'
  | 'auction'
  | 'user_list'
  | 'dashboard'
  | 'audit_logs'
  | 'metrics'
  | 'alerts'

// ============================================================================
// AUDIT LOG ENTITY
// ============================================================================

/**
 * A single audit log entry representing an admin action
 */
export interface AuditLog {
  id: string
  actor_id: string       // Admin who performed the action
  action_type: AuditActionType
  target_type: AuditTargetType
  target_id: string      // ID of the entity acted upon
  metadata?: Record<string, unknown>
  created_at: string
}

/**
 * Admin info embedded in audit log (for display)
 * Note: Backend only returns actor_id, admin name/email would need
 * separate lookup or could be included in metadata
 */
export interface AuditLogWithAdmin extends AuditLog {
  admin_name?: string
  admin_email?: string
}

// ============================================================================
// RESPONSE TYPES
// ============================================================================

/**
 * Response from GET /api/v1/admin/audit-logs
 */
export interface AuditLogsResponse {
  logs: AuditLog[]
  _meta?: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

// ============================================================================
// QUERY PARAMS
// ============================================================================

/**
 * Query parameters for fetching audit logs
 */
export interface AuditLogsQueryParams {
  action?: AuditActionType | ''
  target_type?: AuditTargetType | ''
  admin_id?: string
  date_from?: string
  date_to?: string
  page?: number
  page_size?: number
}

// ============================================================================
// LABELS & VARIANTS
// ============================================================================

/**
 * Human-readable labels for action types
 */
export const auditActionLabels: Partial<Record<AuditActionType, string>> = {
  withdraw_approved: 'Withdrawal Approved',
  withdraw_rejected: 'Withdrawal Rejected',
  withdraw_processed: 'Withdrawal Processed',
  dispute_resolved_approved: 'Dispute Approved (Buyer)',
  dispute_resolved_rejected: 'Dispute Rejected (Seller)',
  role_changed: 'Role Changed',
  account_status_changed: 'Account Status Changed',
  admin_users_listed: 'Viewed Users List',
  admin_user_viewed: 'Viewed User Details',
  admin_dashboard_viewed: 'Viewed Dashboard',
  admin_audit_logs_viewed: 'Viewed Audit Logs',
  admin_metrics_viewed: 'Viewed Metrics',
  admin_alerts_viewed: 'Viewed Alerts',
  user_suspended: 'User Suspended',
  user_activated: 'User Activated',
  user_banned: 'User Banned',
}

/**
 * Grouped action types for filter dropdown
 */
export const auditActionGroups = [
  { label: 'All Actions', value: '' },
  { label: 'Withdrawals', value: ['withdraw_approved', 'withdraw_rejected', 'withdraw_processed'] },
  { label: 'Disputes', value: ['dispute_resolved_approved', 'dispute_resolved_rejected'] },
  { label: 'User Management', value: ['role_changed', 'account_status_changed', 'user_suspended', 'user_activated', 'user_banned'] },
  { label: 'Admin Views', value: ['admin_users_listed', 'admin_user_viewed', 'admin_dashboard_viewed', 'admin_audit_logs_viewed'] },
]

/**
 * Human-readable labels for target types
 */
export const auditTargetTypeLabels: Partial<Record<AuditTargetType, string>> = {
  user: 'User',
  withdrawal: 'Withdrawal',
  dispute: 'Dispute',
  refund: 'Refund',
  auction: 'Auction',
  user_list: 'User List',
  dashboard: 'Dashboard',
  audit_logs: 'Audit Logs',
  metrics: 'Metrics',
  alerts: 'Alerts',
}

/**
 * Badge variants for different action categories
 */
export const auditActionVariants: Partial<Record<AuditActionType, 'success' | 'warning' | 'error' | 'info' | 'pending' | 'default'>> = {
  withdraw_approved: 'success',
  withdraw_rejected: 'error',
  withdraw_processed: 'info',
  dispute_resolved_approved: 'success',
  dispute_resolved_rejected: 'warning',
  role_changed: 'info',
  account_status_changed: 'warning',
  user_suspended: 'warning',
  user_activated: 'success',
  user_banned: 'error',
}

/**
 * Badge variants for target types
 */
export const auditTargetVariants: Partial<Record<AuditTargetType, 'success' | 'warning' | 'error' | 'info' | 'pending' | 'default'>> = {
  user: 'info',
  withdrawal: 'warning',
  dispute: 'error',
  refund: 'warning',
  auction: 'info',
  dashboard: 'default',
  audit_logs: 'default',
  metrics: 'default',
  alerts: 'error',
}
