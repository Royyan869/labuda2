import type { TimelineEvent } from '@/types'
import { api } from './client'

// ============================================================================
// ORDERS API
// ============================================================================

/**
 * Orders list response with pagination metadata
 */
export interface PaginatedOrdersResponse {
  orders: unknown[]
  _meta?: {
    page: number
    per_page: number
    total: number
    total_pages: number
  }
}

/**
 * Get all orders with optional filtering
 * GET /api/v1/admin/orders
 */
export async function getOrders(params?: {
  status?: string
  source?: string
  date_from?: string
  date_to?: string
  page?: number
  page_size?: number
}) {
  const queryParams = new URLSearchParams()
  if (params?.status) queryParams.append('status', params.status)
  if (params?.source) queryParams.append('source', params.source)
  if (params?.date_from) queryParams.append('date_from', params.date_from)
  if (params?.date_to) queryParams.append('date_to', params.date_to)
  queryParams.append('page', String(params?.page ?? 1))
  queryParams.append('page_size', String(params?.page_size ?? 20))

  return api.get<PaginatedOrdersResponse>(
    `/api/v1/admin/orders?${queryParams.toString()}`
  )
}

/**
 * Get order detail by ID
 * GET /api/v1/admin/orders/:id
 */
export async function getOrderDetail(orderId: string) {
  return api.get(`/api/v1/admin/orders/${orderId}`)
}

/**
 * Get order timeline by ID
 * GET /api/v1/admin/orders/:id/timeline
 */
export async function getOrderTimeline(orderId: string): Promise<TimelineEvent[]> {
  const resp = await api.get<{ data: TimelineEvent[] }>(`/api/v1/admin/orders/${orderId}/timeline`)
  return resp.data ?? []
}
