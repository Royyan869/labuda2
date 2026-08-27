// Alert types for the admin panel

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

export const ALERT_TYPE = {
  PAYMENT_FAILURE_SPIKE: 'payment_failure_spike',
  PAYMENT_STUCK: 'payment_stuck',
  DISPUTE_SPIKE: 'dispute_spike',
  SELLER_RISK: 'seller_risk',
  COINS_ANOMALY: 'coins_anomaly',
  WITHDRAWAL_ANOMALY: 'withdrawal_anomaly',
  OUTBOX_DLQ_SPIKE: 'outbox_dlq_spike',
  OUTBOX_STUCK: 'outbox_stuck',
  RECONCILIATION_DRIFT: 'reconciliation_drift',
  REFUND_GATEWAY_FAILED: 'refund_gateway_failed',
  STALE_DISPUTE_FREEZE: 'stale_dispute_freeze',
  SUBSCRIPTION_ORPHANED_PAYMENT: 'subscription_orphaned_payment',
  SUBSCRIPTION_CONVERSION_RATE: 'subscription_conversion_rate',
  SUBSCRIPTION_LIFECYCLE: 'subscription_lifecycle',
  ESCROW_STUCK: 'escrow_stuck',
  ORDER_PAID_STUCK: 'order_paid_stuck',
  ORDER_SHIPPED_STUCK: 'order_shipped_stuck',
  DISPUTE_OPEN_STUCK: 'dispute_open_stuck',
  SELLER_NON_SHIPMENT: 'seller_non_shipment',
} as const

export const ALERT_SEVERITY = {
  LOW: 'low',
  MEDIUM: 'medium',
  HIGH: 'high',
  CRITICAL: 'critical',
  WARNING: 'warning',
  INFO: 'info',
} as const

export const ALERT_STATUS = {
  ACTIVE: 'active',
  OPEN: 'open',
  ACKNOWLEDGED: 'acknowledged',
  RESOLVED: 'resolved',
  FALSE_POSITIVE: 'false_positive',
} as const

export interface Alert {
  id: string
  alert_type: AlertType
  severity: AlertSeverity
  entity_type: string
  entity_id: string
  message: string
  metadata: AlertMetadata
  status: AlertStatus
  created_at: string
  updated_at: string
  resolved_at?: string
  resolved_by?: string
  group_key?: string
}

export interface AlertMetadata {
  [key: string]: unknown
  occurrence_count?: number
  first_occurrence?: string
  last_occurrence?: string
  failure_count?: number
  window_minutes?: number
  threshold?: number
  detected_at?: string
  dispute_count?: number
  transaction_count?: number
  total_amount?: number
  withdrawal_count?: number
}

export interface AlertStats {
  total: number
  active: number
  acknowledged: number
  resolved: number
  false_positive: number
  by_severity: Record<string, number>
  by_type: Record<string, number>
  recent_alerts: Alert[]
}

// Labels and variants for UI display
export const alertTypeLabels: Record<AlertType, string> = {
  payment_failure_spike: 'Payment Failure Spike',
  payment_stuck: 'Payment Stuck',
  dispute_spike: 'Dispute Spike',
  seller_risk: 'Seller Risk',
  coins_anomaly: 'Coins Anomaly',
  withdrawal_anomaly: 'Withdrawal Anomaly',
  outbox_dlq_spike: 'Outbox DLQ Spike',
  outbox_stuck: 'Outbox Stuck',
  reconciliation_drift: 'Reconciliation Drift',
  refund_gateway_failed: 'Refund Gateway Failed',
  stale_dispute_freeze: 'Stale Dispute Freeze',
  subscription_orphaned_payment: 'Subscription Orphaned Payment',
  subscription_conversion_rate: 'Subscription Conversion Rate',
  subscription_lifecycle: 'Subscription Lifecycle',
  escrow_stuck: 'Escrow Stuck',
  order_paid_stuck: 'Order Paid Stuck',
  order_shipped_stuck: 'Order Shipped Stuck',
  dispute_open_stuck: 'Dispute Open Stuck',
  seller_non_shipment: 'Seller Non-Shipment',
}

export const alertSeverityLabels: Record<AlertSeverity, string> = {
  low: 'Low',
  medium: 'Medium',
  high: 'High',
  critical: 'Critical',
  warning: 'Warning',
  info: 'Info',
}

export const alertStatusLabels: Record<AlertStatus, string> = {
  active: 'Active',
  open: 'Open',
  acknowledged: 'Acknowledged',
  resolved: 'Resolved',
  false_positive: 'False Positive',
}

