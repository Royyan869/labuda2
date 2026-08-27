import { useState, useEffect, useCallback } from 'react'
import { api } from '@/lib/api'
import type {
  OrderListItem,
  OrderDetail,
  OrdersQueryParams,
  TimelineEvent,
} from '@/types'

/**
 * Hook for fetching orders list
 */
export function useOrders(params: OrdersQueryParams = {}) {
  const [orders, setOrders] = useState<OrderListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<Error | null>(null)
  const [page, setPage] = useState(params.page || 1)
  const [pageSize, setPageSize] = useState(params.page_size || 20)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)

  const fetchOrders = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const queryParams = new URLSearchParams()
      if (params.status) queryParams.append('status', params.status)
      if (params.source) queryParams.append('source', params.source)
      if (params.date_from) queryParams.append('date_from', params.date_from)
      if (params.date_to) queryParams.append('date_to', params.date_to)
      if (params.search) queryParams.append('search', params.search)
      queryParams.append('page', page.toString())
      queryParams.append('page_size', pageSize.toString())

      const response = await api.get<{
        data: { orders: OrderListItem[] }
        meta?: {
          page: number
          per_page: number
          total: number
          total_pages: number
        }
      }>(
        `/api/v1/admin/orders?${queryParams.toString()}`
      )

      setOrders(response.data?.orders || [])
      if (response.meta) {
        setTotal(response.meta.total)
        setTotalPages(response.meta.total_pages)
      }
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch orders'))
    } finally {
      setLoading(false)
    }
  }, [params.status, params.source, params.date_from, params.date_to, params.search, page, pageSize])

  useEffect(() => {
    fetchOrders()
  }, [fetchOrders])

  // Auto-refresh every 30s for operational awareness
  useEffect(() => {
    const interval = setInterval(() => {
      fetchOrders()
    }, 30_000)
    return () => clearInterval(interval)
  }, [fetchOrders])

  return {
    orders,
    loading,
    error,
    page,
    setPage,
    pageSize,
    setPageSize,
    total,
    totalPages,
    refetch: fetchOrders,
  }
}

/**
 * Hook for fetching a single order detail
 */
export function useOrderDetail(orderId: string | null) {
  const [order, setOrder] = useState<OrderDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const fetchOrder = useCallback(async () => {
    if (!orderId) return

    setLoading(true)
    setError(null)
    try {
      const response = await api.get<{ data: OrderDetail }>(
        `/api/v1/admin/orders/${orderId}`
      )
      setOrder(response.data)
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch order'))
    } finally {
      setLoading(false)
    }
  }, [orderId])

  useEffect(() => {
    fetchOrder()
  }, [fetchOrder])

  return {
    order,
    loading,
    error,
    refetch: fetchOrder,
  }
}

/**
 * Hook for fetching order timeline
 */
export function useOrderTimeline(orderId: string | null) {
  const [timeline, setTimeline] = useState<TimelineEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const fetchTimeline = useCallback(async () => {
    if (!orderId) return

    setLoading(true)
    setError(null)
    try {
      const response = await api.get<{ data: TimelineEvent[] }>(
        `/api/v1/admin/orders/${orderId}/timeline`
      )
      setTimeline(response.data || [])
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch timeline'))
    } finally {
      setLoading(false)
    }
  }, [orderId])

  useEffect(() => {
    fetchTimeline()
  }, [fetchTimeline])

  return {
    timeline,
    loading,
    error,
    refetch: fetchTimeline,
  }
}
