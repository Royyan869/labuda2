// ============================================================================
// ORDER TYPES - Based on backend API responses
// ============================================================================

/**
 * Order status values from backend
 */
export type OrderStatus =
  | 'pending_payment'
  | 'paid'
  | 'shipped'
  | 'delivered'
  | 'completed'
  | 'cancelled'
  | 'cancelled_timeout'
  | 'expired'
  | 'refunded'
  | 'partially_refunded'
  | 'dispute_open'

/**
 * Escrow status values from backend
 */
export type EscrowStatus = 'holding' | 'released' | 'refunded'

/**
 * Source type values from backend
 */
export type SourceType = 'fixed_price_sale' | 'auction' | 'negotiation'

// Status constants to avoid hardcoded strings
export const ORDER_STATUS = {
  PENDING_PAYMENT: 'pending_payment',
  PAID: 'paid',
  SHIPPED: 'shipped',
  DELIVERED: 'delivered',
  COMPLETED: 'completed',
  CANCELLED: 'cancelled',
  CANCELLED_TIMEOUT: 'cancelled_timeout',
  EXPIRED: 'expired',
  REFUNDED: 'refunded',
  PARTIALLY_REFUNDED: 'partially_refunded',
  DISPUTE_OPEN: 'dispute_open',
} as const

export const ESCROW_STATUS = {
  HOLDING: 'holding',
  RELEASED: 'released',
  REFUNDED: 'refunded',
} as const

export const SOURCE_TYPE = {
  FIXED_PRICE_SALE: 'fixed_price_sale',
  AUCTION: 'auction',
  NEGOTIATION: 'negotiation',
} as const

/**
 * Order list item (from GET /api/v1/admin/orders)
 */
export interface OrderListItem {
  id: string
  order_number: string
  buyer_id: string
  seller_id: string
  status: OrderStatus
  escrow_amount: number
  created_at: string
  // Computed fields for display
  buyer_username?: string | null
  buyer_avatar?: string | null
  seller_username?: string | null
  seller_farm_name?: string | null
  seller_avatar?: string | null
}

/**
 * Order list response
 */
export interface OrdersListResponse {
  orders: OrderListItem[]
}

/**
 * Order item detail
 */
export interface OrderItemDetail {
  product_id: string
  product_title: string
  quantity: number
  unit_price: number
  subtotal: number
  snapshot_image_url?: string | null
}

/**
 * Shipping address detail
 */
export interface ShippingAddressDetail {
  id: string
  recipient_name: string
  phone: string
  province: string
  city: string
  address: string
  postal_code: string
}

/**
 * Shipping origin detail (seller's farm/warehouse address snapshot)
 */
export interface ShippingOriginDetail {
  recipient_name: string
  phone: string
  province: string
  city: string
  district?: string | null
  village?: string | null
  address: string
  postal_code: string
}

/**
 * Dispute summary (within order context)
 */
export interface DisputeSummary {
  id: string
  reason: string
  description?: string | null
  status: string
  opened_at: string
  resolved_at?: string | null
}

/**
 * Refund summary (within order context).
 * Surfaces refund_id for POST /admin/refunds/:id/gateway/initiate.
 * Requires: finance.refund.gateway.initiate capability.
 */
export interface RefundSummary {
  id: string
  status: string
  reason: string
  gateway_status: 'unsubmitted' | 'pending' | 'succeeded' | 'failed'
  gateway_attempts: number
  gateway_refund_id?: string | null
  last_gateway_error?: string | null
  requested_amount: number
  created_at: string
  updated_at: string
}

/**
 * Timeline event
 */
export interface TimelineEvent {
  event: string
  timestamp: string
  actor_id?: string | null
  actor_name?: string | null
  metadata?: Record<string, unknown> | null
}

/**
 * Full order detail (from GET /api/v1/admin/orders/:id)
 */
export interface OrderDetail {
  id: string
  order_number: string
  buyer_id: string
  seller_id: string
  source_type: SourceType
  source_id: string
  // source_status: current governance status of the underlying fixed-price sale/auction
  // at query time (read-only). Nil for negotiation orders.
  source_status?: string | null
  status: OrderStatus
  escrow_status: EscrowStatus
  has_dispute: boolean
  dispute_status?: string | null
  subtotal: number
  shipping_total: number
  commission_amount: number
  service_fee_amount?: number
  total_payable_amount?: number
  escrow_amount: number
  refunded_amount: number
  shipping_option?: string | null
  tracking_number?: string | null
  auto_release_at?: string | null
  created_at: string
  updated_at: string
  // Payment info
  payment?: {
    gross_amount: number
    service_fee_amount?: number
  } | null
  // User info
  buyer_username?: string | null
  buyer_avatar?: string | null
  seller_username?: string | null
  seller_farm_name?: string | null
  seller_avatar?: string | null
  // Items
  items?: OrderItemDetail[]
  // Shipping source + origin (I1-C1: where shipping cost originated + seller origin)
  shipping_source?: 'fixed_price_sale' | 'shipping_quote' | null
  shipping_origin?: ShippingOriginDetail | null
  // Shipping address
  shipping_address?: ShippingAddressDetail | null
  // Dispute info
  dispute?: DisputeSummary | null
  // Refund info (if exists)
  refund?: RefundSummary | null
  // Timeline
  timeline?: TimelineEvent[]
}

/**
 * Orders query parameters
 */
export interface OrdersQueryParams {
  status?: OrderStatus | ''
  source?: SourceType | ''
  date_from?: string
  date_to?: string
  page?: number
  page_size?: number
  search?: string
}

// ============================================================================
// LABELS & VARIANTS
// ============================================================================

export const orderStatusLabels: Record<OrderStatus, string> = {
  pending_payment: 'Pending Payment',
  paid: 'Paid',
  shipped: 'Shipped',
  delivered: 'Delivered',
  completed: 'Completed',
  cancelled: 'Cancelled',
  cancelled_timeout: 'Cancelled (Timeout)',
  expired: 'Expired',
  refunded: 'Refunded',
  partially_refunded: 'Partially Refunded',
  dispute_open: 'Dispute Open',
}

export const orderStatusVariants: Record<OrderStatus, 'success' | 'warning' | 'error' | 'info' | 'pending'> = {
  pending_payment: 'pending',
  paid: 'info',
  shipped: 'info',
  delivered: 'warning',
  completed: 'success',
  cancelled: 'error',
  cancelled_timeout: 'error',
  expired: 'error',
  refunded: 'warning',
  partially_refunded: 'warning',
  dispute_open: 'error',
}

export const escrowStatusLabels: Record<EscrowStatus, string> = {
  holding: 'Holding',
  released: 'Released',
  refunded: 'Refunded',
}

export const escrowStatusVariants: Record<EscrowStatus, 'success' | 'warning' | 'error' | 'info' | 'pending'> = {
  holding: 'pending',
  released: 'success',
  refunded: 'warning',
}

export const sourceTypeLabels: Record<SourceType, string> = {
  fixed_price_sale: 'Fixed-Price Sale',
  auction: 'Auction',
  negotiation: 'Negotiation',
}

// Source status labels — covers all fixed-price sale and auction status values.
export const sourceStatusLabels: Record<string, string> = {
  // Fixed-price sale statuses
  draft: 'Draft',
  active: 'Active',
  sold: 'Sold',
  withdrawn: 'Withdrawn',
  // Auction statuses
  scheduled: 'Scheduled',
  waiting_settlement: 'Waiting Settlement',
  expired_bnr: 'Expired (BNR)',
  ended: 'Ended',
  cancelled: 'Cancelled',
}

export const sourceStatusVariants: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
  // Fixed-price sale
  draft: 'default',
  active: 'success',
  sold: 'info',
  withdrawn: 'error',
  // Auction
  scheduled: 'info',
  waiting_settlement: 'warning',
  expired_bnr: 'error',
  ended: 'default',
  cancelled: 'error',
}

// ============================================================================
// DISPUTE TYPES
// ============================================================================

/**
 * Dispute status values from backend
 */
export type DisputeStatus = 'under_review' | 'resolved_refund' | 'resolved_release' | 'resolved_partial'

export const DISPUTE_STATUS = {
  UNDER_REVIEW: 'under_review',
  RESOLVED_REFUND: 'resolved_refund',
  RESOLVED_RELEASE: 'resolved_release',
  RESOLVED_PARTIAL: 'resolved_partial',
} as const

/**
 * Dispute reason values
 */
export type DisputeReason =
  | 'item_not_received'
  | 'item_not_as_described'
  | 'damaged_item'
  | 'wrong_item'
  | 'other'

/**
 * Dispute list item (from GET /api/v1/admin/disputes)
 */
export interface DisputeListItem {
  id: string
  order_id: string
  buyer_id: string
  seller_id: string
  reason: string
  description?: string | null
  status: DisputeStatus
  opened_at: string
  resolved_at?: string | null
  resolved_by?: string | null
  resolution_notes?: string | null
  // Computed fields for display
  buyer_username?: string | null
  buyer_avatar?: string | null
  seller_username?: string | null
  seller_farm_name?: string | null
  seller_avatar?: string | null
  // SLA Metrics
  next_action?: string | null
  sla_summary?: string | null
  admin_response_overdue: boolean
  resolution_overdue: boolean
}

/**
 * Dispute list response
 */
export interface DisputesListResponse {
  disputes: DisputeListItem[]
}

/**
 * Evidence item attached to a dispute
 */
export interface EvidenceItem {
  id: string
  type: 'image' | 'document' | 'text'
  url?: string
  content?: string
  submitted_by: 'buyer' | 'seller'
  submitted_at: string
  description?: string
}

/**
 * Evidence summary for a party
 */
export interface PartyEvidence {
  submitted_by: 'buyer' | 'seller'
  evidence: EvidenceItem[]
  statement?: string
}

/**
 * Full dispute detail (from GET /api/v1/admin/disputes/:id)
 */
export interface DisputeDetail {
  id: string
  order_id: string
  buyer_id: string
  seller_id: string
  reason: string
  description?: string | null
  status: DisputeStatus
  opened_at: string
  resolved_at?: string | null
  resolved_by?: string | null
  resolution_notes?: string | null
  created_at: string
  updated_at: string
  // Evidence attachments attached directly to the dispute detail.
  evidence?: string[]
  // Detailed evidence by party
  buyer_evidence?: PartyEvidence
  seller_evidence?: PartyEvidence
  // Related order info
  order_status?: string | null
  order_escrow_status?: string | null // JSON key: order_escrow_status
  escrow_amount?: number              // JSON key: escrow_amount (subtotal + shipping)
  shipping_reference?: string | null  // Tracking number
  shipping_carrier?: string | null    // Carrier/option name
  // Computed fields for display
  buyer_username?: string | null
  buyer_avatar?: string | null
  seller_username?: string | null
  seller_farm_name?: string | null
  seller_avatar?: string | null
  // SLA Metrics
  next_action?: string | null
  sla_summary?: string | null
  admin_response_time?: string | null
  resolution_time?: string | null
  waiting_buyer_time?: string | null
  waiting_seller_time?: string | null
  active_time?: string | null
  admin_response_overdue: boolean
  resolution_overdue: boolean
  admin_response_overdue_duration?: string | null
  resolution_overdue_duration?: string | null
}

/**
 * Dispute decision type
 */
export type DisputeDecision = 'refund_full' | 'refund_partial' | 'reject'

/**
 * Dispute resolution request with decision type
 */
export interface DisputeDecisionRequest {
  decision: DisputeDecision
  refund_amount?: number // For partial refunds
  notes?: string
}

/**
 * Dispute resolution request
 */
export interface DisputeResolutionRequest {
  notes?: string
}

/**
 * Dispute resolution response
 */
export interface DisputeResolutionResponse {
  dispute_id: string
  resolution: string
  message: string
}

/**
 * Disputes query parameters
 */
export interface DisputesQueryParams {
  status?: DisputeStatus | ''
  date_from?: string
  date_to?: string
  page?: number
  page_size?: number
}

// ============================================================================
// DISPUTE LABELS & VARIANTS
// ============================================================================

export const disputeStatusLabels: Record<DisputeStatus, string> = {
  under_review: 'Under Review',
  resolved_refund: 'Refunded',
  resolved_release: 'Released',
  resolved_partial: 'Product-Only Refund',
}

export const disputeStatusVariants: Record<DisputeStatus, 'success' | 'warning' | 'error' | 'info' | 'pending'> = {
  under_review: 'pending',
  resolved_refund: 'warning',
  resolved_release: 'success',
  resolved_partial: 'warning',
}

export const disputeReasonLabels: Record<string, string> = {
  item_not_received: 'Item Not Received',
  item_not_as_described: 'Item Not As Described',
  damaged_item: 'Damaged Item',
  wrong_item: 'Wrong Item Sent',
  other: 'Other',
}
