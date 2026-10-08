import { useState } from 'react'
import { ShoppingBag, Eye, Filter, RefreshCw, Search } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination, PageHeader } from '@/components/common'
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
  { value: 'for_sale', label: 'For Sale' },
  { value: 'auction', label: 'Auction' },
  { value: 'negotiation', label: 'Negotiation' },
]

export function OrdersPage() {
  const [statusFilter, setStatusFilter] = useState<OrderStatus | ''>('')
  const [sourceFilter, setSourceFilter] = useState<SourceType | ''>('')
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedOrder, setSelectedOrder] = useState<OrderListItem | null>(null)
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false)

  const { orders, loading, error, total, page, setPage, totalPages, refetch } = useOrders(
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

  const hasActiveFilters = statusFilter || sourceFilter || searchQuery

  const handleClearFilters = () => {
    setStatusFilter('')
    setSourceFilter('')
    setSearchQuery('')
    setPage(1)
  }

  if (loading && orders.length === 0) {
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Orders" description="View and manage all marketplace orders" />
        <AdminErrorState title="Failed to load orders" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title="Orders"
        description="View and manage all marketplace orders"
        actions={
          <Button variant="secondary" onClick={refetch} className="gap-2">
            <RefreshCw className="h-4 w-4" />
            Refresh
          </Button>
        }
      />

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-muted-foreground">Total Orders</p>
              <p className="type-metric-lg text-primary mt-1">{total}</p>
            </div>
            <div className="p-4 rounded-lg bg-info-bg">
              <ShoppingBag className="h-8 w-8 text-info" />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-end gap-6 flex-wrap">
            <div className="flex items-end gap-2">
              <Filter className="h-5 w-5 text-muted-foreground mb-2" />
                <Select
                  label="Status:"
                  value={statusFilter}
                  onChange={(e) => { setStatusFilter(e.target.value as OrderStatus | ''); setPage(1) }}
                >
                {ORDER_STATUSES.map((status) => (
                  <option key={status.value} value={status.value}>
                    {status.label}
                  </option>
                ))}
              </Select>
            </div>
            <Select
              label="Source:"
              value={sourceFilter}
              onChange={(e) => { setSourceFilter(e.target.value as SourceType | ''); setPage(1) }}
            >
              {SOURCE_TYPES.map((source) => (
                <option key={source.value} value={source.value}>
                  {source.label}
                </option>
              ))}
            </Select>
            <div className="flex items-center gap-2 ml-auto">
              <Search className="h-4 w-4 text-muted-foreground" />
              <Input
                type="text"
                placeholder="Order number or UUID…"
                value={searchQuery}
                onChange={(e) => { setSearchQuery(e.target.value); setPage(1) }}
                className="w-56"
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
            <AdminEmptyState
              icon={ShoppingBag}
              title="No Orders Found"
              description={
                hasActiveFilters
                  ? 'No orders match the current filters.'
                  : 'No orders in the system.'
              }
              filtered={Boolean(hasActiveFilters)}
              onClearFilters={handleClearFilters}
            />
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
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
                          <span className="type-caption">{order.id.slice(0, 8)}</span>
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
                            <div className="w-6 h-6 rounded-full bg-border" />
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
                            <div className="w-6 h-6 rounded-full bg-border" />
                          )}
                          <div className="min-w-0">
                            <span className="text-sm truncate max-w-[120px] block">
                              {order.seller_username ? `@${order.seller_username}` : 'Unknown'}
                            </span>
                            {order.seller_farm_name && (
                              <span className="type-caption truncate max-w-[120px] block">
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
                        {formatRupiah(order.total_before_coins_amount)}
                      </TableCell>
                      <TableCell className="type-secondary">
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

      {/* Pagination */}
      {totalPages > 1 && (
        <AdminPagination
          page={page}
          totalPages={totalPages}
          onPageChange={setPage}
          disabled={loading}
        />
      )}

      {/* Order Detail Modal */}
      <OrderDetailModal
        isOpen={isDetailModalOpen}
        onClose={handleCloseModal}
        orderData={selectedOrder}
      />
    </div>
  )
}
