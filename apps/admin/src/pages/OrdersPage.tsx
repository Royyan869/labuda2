import { useState } from 'react'
import { ShoppingBag, Eye, Filter, RefreshCw, Search } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { OrderDetailModal } from '@/components/orders/OrderDetailModal'
import { useOrders } from '@/hooks/useOrders'
import { formatDate, formatRupiah } from '@/lib/utils'
import type {
  OrderListItem,
  OrderStatus,
  SourceType,
} from '@/types'
import {
  orderStatusLabels,
  orderStatusVariants,
} from '@/types'

const ORDER_STATUSES: { value: OrderStatus | ''; label: string }[] = [
  { value: '', label: 'All Statuses' },
  { value: 'pending_payment', label: 'Pending Payment' },
  { value: 'paid', label: 'Paid' },
  { value: 'shipped', label: 'Shipped' },
  { value: 'delivered', label: 'Delivered' },
  { value: 'completed', label: 'Completed' },
  { value: 'cancelled', label: 'Cancelled' },
  { value: 'cancelled_timeout', label: 'Cancelled (Timeout)' },
  { value: 'expired', label: 'Expired' },
  { value: 'refunded', label: 'Refunded' },
  { value: 'partially_refunded', label: 'Partially Refunded' },
  { value: 'dispute_open', label: 'Dispute Open' },
]

const SOURCE_TYPES: { value: SourceType | ''; label: string }[] = [
  { value: '', label: 'All Sources' },
  { value: 'fixed_price_sale', label: 'Fixed-Price Sale' },
  { value: 'auction', label: 'Auction' },
  { value: 'negotiation', label: 'Negotiation' },
]

export function OrdersPage() {
  const [statusFilter, setStatusFilter] = useState<OrderStatus | ''>('')
  const [sourceFilter, setSourceFilter] = useState<SourceType | ''>('')
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedOrder, setSelectedOrder] = useState<OrderListItem | null>(null)
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false)

  const { orders, loading, error, total, refetch } = useOrders(
    statusFilter || sourceFilter || searchQuery
      ? {
          ...(statusFilter && { status: statusFilter }),
          ...(sourceFilter && { source: sourceFilter }),
          ...(searchQuery && { search: searchQuery }),
        }
      : {}
  )

  const handleViewDetail = (order: OrderListItem) => {
    setSelectedOrder(order)
    setIsDetailModalOpen(true)
  }

  const handleCloseModal = () => {
    setIsDetailModalOpen(false)
    setSelectedOrder(null)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading orders...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Orders</h1>
          <p className="text-gray-600 mt-1">View and manage all marketplace orders</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading orders: {error.message}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Orders</h1>
          <p className="text-gray-600 mt-1">View and manage all marketplace orders</p>
        </div>
        <Button
          variant="secondary"
          onClick={refetch}
          className="gap-2"
        >
          <RefreshCw className="h-4 w-4" />
          Refresh
        </Button>
      </div>

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-600">Total Orders</p>
              <p className="text-3xl font-bold text-primary mt-1">{total}</p>
            </div>
            <div className="p-4 rounded-lg bg-blue-100">
              <ShoppingBag className="h-8 w-8 text-blue-600" />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center gap-6 flex-wrap">
            <div className="flex items-center gap-4">
              <Filter className="h-5 w-5 text-gray-500" />
              <label htmlFor="status-filter" className="text-sm font-medium text-gray-700">
                Status:
              </label>
              <select
                id="status-filter"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value as OrderStatus | '')}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {ORDER_STATUSES.map((status) => (
                  <option key={status.value} value={status.value}>
                    {status.label}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex items-center gap-4">
              <label htmlFor="source-filter" className="text-sm font-medium text-gray-700">
                Source:
              </label>
              <select
                id="source-filter"
                value={sourceFilter}
                onChange={(e) => setSourceFilter(e.target.value as SourceType | '')}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {SOURCE_TYPES.map((source) => (
                  <option key={source.value} value={source.value}>
                    {source.label}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex items-center gap-2 ml-auto">
              <Search className="h-4 w-4 text-gray-400" />
              <input
                type="text"
                placeholder="Order number or UUID…"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm w-56 focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Orders Table */}
      <Card>
        <CardHeader>
          <CardTitle>Orders Queue</CardTitle>
        </CardHeader>
        <CardContent>
          {orders.length === 0 ? (
            <div className="text-center py-12">
              <ShoppingBag className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Orders Found</h3>
              <p className="text-gray-600">
                {statusFilter || sourceFilter
                  ? 'No orders match the current filters.'
                  : 'No orders in the system.'}
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Order #</TableHead>
                    <TableHead>Buyer</TableHead>
                    <TableHead>Seller</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Amount</TableHead>
                    <TableHead>Created At</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {orders.map((order) => (
                    <TableRow key={order.id}>
                      <TableCell className="font-mono text-sm">
                        <div className="flex flex-col gap-0.5">
                          <span className="font-medium">{order.order_number || '—'}</span>
                          <span className="text-xs text-gray-400">{order.id.slice(0, 8)}</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          {order.buyer_avatar ? (
                            <img
                              src={order.buyer_avatar}
                              alt=""
                              className="w-6 h-6 rounded-full object-cover"
                            />
                          ) : (
                            <div className="w-6 h-6 rounded-full bg-gray-200" />
                          )}
                          <div className="min-w-0">
                            <span className="text-sm truncate max-w-[120px] block">
                              {order.buyer_username ? `@${order.buyer_username}` : 'Unknown'}
                            </span>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          {order.seller_avatar ? (
                            <img
                              src={order.seller_avatar}
                              alt=""
                              className="w-6 h-6 rounded-full object-cover"
                            />
                          ) : (
                            <div className="w-6 h-6 rounded-full bg-gray-200" />
                          )}
                          <div className="min-w-0">
                            <span className="text-sm truncate max-w-[120px] block">
                              {order.seller_username ? `@${order.seller_username}` : 'Unknown'}
                            </span>
                            {order.seller_farm_name && (
                              <span className="text-xs text-gray-500 truncate max-w-[120px] block">
                                {order.seller_farm_name}
                              </span>
                            )}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={orderStatusVariants[order.status] || 'info'}>
                          {orderStatusLabels[order.status] || order.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-sm">
                        {formatRupiah(order.escrow_amount)}
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {formatDate(order.created_at)}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          size="sm"
                          onClick={() => handleViewDetail(order)}
                        >
                          <Eye className="h-4 w-4 mr-1" />
                          View
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Order Detail Modal */}
      <OrderDetailModal
        isOpen={isDetailModalOpen}
        onClose={handleCloseModal}
        orderData={selectedOrder}
      />
    </div>
  )
}
