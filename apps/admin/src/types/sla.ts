/**
 * SLA (Service Level Agreement) Metrics Types
 */

export interface SLAMetricBreakdown {
  // Timing metrics
  avg_first_response_time: number | null // milliseconds
  p95_first_response_time: number | null // milliseconds
  avg_resolution_time: number | null // milliseconds
  p95_resolution_time: number | null // milliseconds

  // Status metrics
  overdue_rate: number
  overdue_count: number
  total_count: number

  // Context metrics
  active_count: number
  resolved_count: number

  // Health status
  health_status: 'good' | 'warning' | 'critical'
}

export interface AdminPerformanceMetrics {
  admin_id: string
  avg_response_time: number | null // milliseconds
  p95_response_time: number | null // milliseconds
  avg_resolution_time: number | null // milliseconds
  p95_resolution_time: number | null // milliseconds
  overdue_count: number
  overdue_rate: number
  handled_tickets: number
  active_workload: number
  health_status: 'good' | 'warning' | 'critical'
}

export interface SystemHealthStatus {
  status: 'good' | 'warning' | 'critical'
  score: number // 0-100
  issues: string[]
}

export interface TrendComparison {
  last_24_hours: SLAMetricBreakdown | null
  previous_24_hours: SLAMetricBreakdown | null
  response_time_change: number // percentage
  resolution_time_change: number // percentage
  overdue_rate_change: number // percentage
}

export interface SLAMetrics {
  support: SLAMetricBreakdown | null
  dispute: SLAMetricBreakdown | null
  admin_performance: AdminPerformanceMetrics[]
  system_health: SystemHealthStatus
  trends: TrendComparison | null
  generated_at: string
}
