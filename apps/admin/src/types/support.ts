// ============================================================================
// SUPPORT TYPES - Based on backend API responses
// ============================================================================

/**
 * Support ticket status values from backend
 */
export type SupportTicketStatus =
  | 'open'
  | 'in_progress'
  | 'waiting_user'
  | 'resolved'
  | 'closed'

/**
 * Support ticket category values
 */
export type SupportCategory =
  | 'order_issue'
  | 'payment_issue'
  | 'account_issue'
  | 'listing_issue'
  | 'other'

/**
 * Support ticket priority values
 */
export type SupportPriority = 'low' | 'medium' | 'high' | 'urgent'

/**
 * Support ticket escalation values
 */
export type SupportTicketEscalation = 'none' | 'dispute' | 'finance' | 'ops'

/**
 * SLA metrics for support tickets
 */
export interface SupportTicketSLA {
  first_response_time_seconds: number | null
  first_response_overdue: boolean
  resolution_time_seconds: number | null
  resolution_overdue: boolean
  is_overdue: boolean
  next_action: 'reply' | 'wait' | 'resolve' | 'none'
  waiting_time_seconds?: number
  active_time_seconds?: number
}

// Status constants
export const SUPPORT_TICKET_STATUS = {
  OPEN: 'open',
  IN_PROGRESS: 'in_progress',
  WAITING_USER: 'waiting_user',
  RESOLVED: 'resolved',
  CLOSED: 'closed',
} as const

export const SUPPORT_CATEGORY = {
  ORDER_ISSUE: 'order_issue',
  PAYMENT_ISSUE: 'payment_issue',
  ACCOUNT_ISSUE: 'account_issue',
  LISTING_ISSUE: 'listing_issue',
  OTHER: 'other',
} as const

export const SUPPORT_PRIORITY = {
  LOW: 'low',
  MEDIUM: 'medium',
  HIGH: 'high',
  URGENT: 'urgent',
} as const

/**
 * Support ticket list item (from GET /api/v1/admin/support/tickets)
 */
export interface SupportTicketListItem {
  id: string
  user_id: string
  username?: string | null
  seller_farm_name?: string | null
  category: SupportCategory
  status: SupportTicketStatus
  priority: SupportPriority
  escalation: SupportTicketEscalation
  subject: string
  order_id?: string | null
  created_at: string
  updated_at: string
  assigned_admin_id?: string | null
  assigned_at?: string | null
  resolved_at?: string | null
  closed_at?: string | null
  // SLA metrics
  sla: SupportTicketSLA
  // Computed fields for display
  user_avatar?: string | null
}

/**
 * Support ticket list response
 */
export interface SupportTicketsListResponse {
  tickets: SupportTicketListItem[]
}

/**
 * Order information for support ticket context
 */
export interface SupportTicketOrderInfo {
  order_id: string
  status: string
  escrow_status: string
  has_dispute: boolean
}

/**
 * Dispute information for support ticket context
 */
export interface SupportTicketDisputeInfo {
  dispute_id: string
  status: string
  opened_at: string
  resolved_at?: string | null
  resolved_by?: string | null
}

/**
 * Full support ticket detail (from GET /api/v1/admin/support/tickets/:id)
 */
export interface SupportTicketDetail {
  id: string
  user_id: string
  username?: string | null
  seller_farm_name?: string | null
  category: SupportCategory
  status: SupportTicketStatus
  priority: SupportPriority
  escalation: SupportTicketEscalation
  subject: string
  description: string
  order_id?: string | null
  claimed_by?: string | null
  claimed_at?: string | null
  resolved_at?: string | null
  closed_at?: string | null
  created_at: string
  updated_at: string
  // SLA metrics
  sla: SupportTicketSLA
  // Enriched fields
  order_info?: SupportTicketOrderInfo | null
  dispute_info?: SupportTicketDisputeInfo | null
  // Computed fields for display
  user_avatar?: string | null
  admin_name?: string | null
}

/**
 * Support message item (from GET /api/v1/admin/support/tickets/:id/messages)
 */
export interface SupportMessage {
  id: string
  room_id: string
  sender_id: string
  sender_type: 'user' | 'admin' | 'system'
  message_type: string
  created_at: string
  body?: string
  attachment_json?: Record<string, unknown> | null
}

/**
 * Support messages response
 */
export interface SupportMessagesResponse {
  data: SupportMessage[]
}

/**
 * Send message request (for POST /api/v1/admin/support/tickets/:id/messages)
 */
export interface SendMessageRequest {
  type: 'greeting' | 'system' | 'agent'
  message: string
}

/**
 * Escalate to dispute request (for POST /api/v1/admin/support/tickets/:id/escalate-to-dispute)
 */
export interface EscalateToDisputeRequest {
  reason: string
  description?: string
  reason_code: string
}

/**
 * Update priority request (for PUT /api/v1/admin/support/tickets/:id/priority)
 */
export interface UpdatePriorityRequest {
  priority: SupportPriority
}

/**
 * Update category request (for PUT /api/v1/admin/support/tickets/:id/category)
 */
export interface UpdateCategoryRequest {
  category: SupportCategory
}

/**
 * Support tickets query parameters
 */
export interface SupportTicketsQueryParams {
  status?: SupportTicketStatus | ''
  category?: SupportCategory | ''
  is_overdue?: boolean
  is_unassigned?: boolean
  date_from?: string
  date_to?: string
  page?: number
  page_size?: number
}

// ============================================================================
// LABELS & VARIANTS
// ============================================================================

export const supportTicketStatusLabels: Record<SupportTicketStatus, string> = {
  open: 'Open',
  in_progress: 'In Progress',
  waiting_user: 'Waiting for User',
  resolved: 'Resolved',
  closed: 'Closed',
}

export const supportTicketStatusVariants: Record<SupportTicketStatus, 'success' | 'warning' | 'error' | 'info' | 'pending'> = {
  open: 'pending',
  in_progress: 'info',
  waiting_user: 'warning',
  resolved: 'success',
  closed: 'error',
}

export const supportCategoryLabels: Record<SupportCategory, string> = {
  order_issue: 'Order Issue',
  payment_issue: 'Payment Issue',
  account_issue: 'Account Issue',
  listing_issue: 'Listing Issue',
  other: 'Other',
}

export const supportPriorityLabels: Record<SupportPriority, string> = {
  low: 'Low',
  medium: 'Medium',
  high: 'High',
  urgent: 'Urgent',
}

export const supportPriorityVariants: Record<SupportPriority, 'success' | 'warning' | 'error' | 'info' | 'pending'> = {
  low: 'info',
  medium: 'pending',
  high: 'warning',
  urgent: 'error',
}
