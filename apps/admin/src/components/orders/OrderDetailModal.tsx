import { useState, useEffect } from 'react'
import { Package, User, MapPin, CreditCard, Clock, AlertTriangle, RefreshCw } from 'lucide-react'
import { Modal, ModalFooter } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { useOrderDetail } from '@/hooks/useOrders'
import { useAuth } from '@/hooks/useAuth'
import { hasCapability } from '@/lib/permissions'
import { initiateGatewayRefund } from '@/lib/api'
import { formatDate, formatRupiah } from '@/lib/utils'
import { AdminLoadingState } from '@/components/common'
import type {
  OrderListItem,
  OrderStatus,
  EscrowStatus,
} from '@/types'
import {
  orderStatusLabels,
  orderStatusVariants,
  escrowStatusLabels,
  escrowStatusVariants,
  sourceTypeLabels,
  sourceStatusLabels,
  sourceStatusVariants,
} from '@/types'

interface OrderDetailModalProps {
  isOpen: boolean
  onClose: () => void
  orderData: OrderListItem | null
}

function gatewayStatusVariant(gs: string): 'success' | 'error' | 'warning' | 'pending' {
  if (gs === 'succeeded') return 'success'
  if (gs === 'failed') return 'error'
  if (gs === 'pending') return 'warning'
  return 'pending'
}

export function OrderDetailModal({ isOpen, onClose, orderData }: OrderDetailModalProps) {
  const [error, setError] = useState<string | null>(null)
  const [gatewayRetryOpen, setGatewayRetryOpen] = useState(false)
  const [gatewayRetryAmount, setGatewayRetryAmount] = useState('')
  const [gatewayRetryReason, setGatewayRetryReason] = useState('')
  const [gatewayRetrying, setGatewayRetrying] = useState(false)
  const [gatewayRetryResult, setGatewayRetryResult] = useState<{
    gateway_status: string; gateway_attempts: number
  } | null>(null)
  const [gatewayRetryError, setGatewayRetryError] = useState<string | null>(null)

  const { order, loading, refetch } = useOrderDetail(orderData?.id || null)
  const { capabilities } = useAuth()
  const canInitiateGatewayRefund = hasCapability(capabilities, 'finance.refund.gateway.initiate')

  // Reset state when modal closes
  useEffect(() => {
    if (!isOpen) {
      setError(null)
      setGatewayRetryOpen(false)
      setGatewayRetryResult(null)
      setGatewayRetryError(null)
    }
  }, [isOpen])

  const handleGatewayRetry = async () => {
    if (!order?.refund) return
    const amount = parseInt(gatewayRetryAmount, 10)
    if (isNaN(amount) || amount <= 0) {
      setGatewayRetryError('Amount must be a positive integer')
      return
    }
    if (!gatewayRetryReason.trim()) {
      setGatewayRetryError('Reason is required')
      return
    }
    setGatewayRetrying(true)
    setGatewayRetryError(null)
    setGatewayRetryResult(null)
    try {
      const idempotencyKey = `admin-retry-${order.refund.id}-${Date.now()}`
      const result = await initiateGatewayRefund(
        order.refund.id, amount, gatewayRetryReason.trim(), idempotencyKey
      )
      setGatewayRetryResult({ gateway_status: result.gateway_status, gateway_attempts: result.gateway_attempts })
      await refetch()
    } catch (err) {
      setGatewayRetryError(err instanceof Error ? err.message : 'Failed to initiate gateway refund')
    } finally {
      setGatewayRetrying(false)
    }
  }

  if (!orderData) return null

  const displayData = order || orderData
  const buyerIdentity = displayData.buyer_username ? `@${displayData.buyer_username}` : 'Unknown'
  const sellerIdentity = displayData.seller_username ? `@${displayData.seller_username}` : 'Unknown'

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Order Details" size="xl">
      {loading ? (
        <AdminLoadingState embedded label="Loading order details" />
      ) : (
        <div className="space-y-6">
          {/* Error Message */}
          {error && (
            <div className="bg-destructive-bg border border-destructive text-destructive p-3 rounded-lg flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 flex-shrink-0" />
              <span className="text-sm">{error}</span>
            </div>
          )}

          {/* Header with Status Badges */}
          <div className="flex items-center justify-between flex-wrap gap-3">
            <div className="flex items-center gap-3">
              <Badge variant={orderStatusVariants[displayData.status as OrderStatus] || 'info'}>
                {orderStatusLabels[displayData.status as OrderStatus] || displayData.status}
              </Badge>
              {order?.escrow_status && (
                <Badge variant={escrowStatusVariants[order.escrow_status as EscrowStatus] || 'info'}>
                  Escrow: {escrowStatusLabels[order.escrow_status as EscrowStatus] || order.escrow_status}
                </Badge>
              )}
              {order?.has_dispute && (
                <Badge variant="error">Has Dispute</Badge>
              )}
            </div>
            <div className="flex items-center gap-3">
              {order?.order_number && (
                <span className="text-sm font-semibold text-foreground font-mono">
                  {order.order_number}
                </span>
              )}
              <span className="type-caption font-mono" title={displayData.id}>
                {displayData.id.slice(0, 8)}…
              </span>
              <button
                onClick={() => {
                  setError(null)
                  refetch()
                }}
                className="text-muted-foreground hover:text-muted-foreground transition-colors"
                title="Refresh order data"
              >
                <RefreshCw className="h-4 w-4" />
              </button>
            </div>
          </div>

          {/* Parties Information */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg flex items-center gap-2">
                <User className="h-5 w-5" />
                Parties
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 gap-6">
                {/* Buyer */}
                <div className="space-y-2">
                  <p className="type-secondary">Buyer</p>
                  <div className="flex items-center gap-3">
                    {displayData.buyer_avatar ? (
                      <img
                        src={displayData.buyer_avatar}
                        alt=""
                        className="w-10 h-10 rounded-full object-cover"
                      />
                    ) : (
                      <div className="w-10 h-10 rounded-full bg-surface-muted flex items-center justify-center">
                        <User className="h-5 w-5 text-muted-foreground" />
                      </div>
                    )}
                    <div>
                      <p className="font-medium">{buyerIdentity}</p>
                      <p className="font-mono type-caption">{displayData.buyer_id}</p>
                    </div>
                  </div>
                </div>

                {/* Seller */}
                <div className="space-y-2">
                  <p className="type-secondary">Seller</p>
                  <div className="flex items-center gap-3">
                    {displayData.seller_avatar ? (
                      <img
                        src={displayData.seller_avatar}
                        alt=""
                        className="w-10 h-10 rounded-full object-cover"
                      />
                    ) : (
                      <div className="w-10 h-10 rounded-full bg-surface-muted flex items-center justify-center">
                        <User className="h-5 w-5 text-muted-foreground" />
                      </div>
                    )}
                    <div>
                      <p className="font-medium">{sellerIdentity}</p>
                      {displayData.seller_farm_name && (
                        <p className="type-secondary">{displayData.seller_farm_name}</p>
                      )}
                      <p className="font-mono type-caption">{displayData.seller_id}</p>
                    </div>
                  </div>
                </div>
              </div>

              {/* Order Metadata */}
              {order && (
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4 pt-4 border-t border-border">
                  <div>
                    <p className="type-secondary">Source Type</p>
                    <p className="text-sm font-medium">
                      {sourceTypeLabels[order.source_type] || order.source_type}
                    </p>
                    {order.source_id && (
                      <p className="font-mono type-caption mt-0.5 truncate" title={order.source_id}>
                        {order.source_id.slice(0, 8)}…
                      </p>
                    )}
                  </div>
                  <div>
                    <p className="type-secondary mb-1">Source Status</p>
                    {order.source_status ? (
                      <Badge variant={sourceStatusVariants[order.source_status] || 'default'}>
                        {sourceStatusLabels[order.source_status] || order.source_status}
                      </Badge>
                    ) : (
                      <span className="type-caption">—</span>
                    )}
                  </div>
                  <div>
                    <p className="type-secondary">Created At</p>
                    <p className="text-sm">{formatDate(order.created_at)}</p>
                  </div>
                  <div>
                    <p className="type-secondary">Updated At</p>
                    <p className="text-sm">{formatDate(order.updated_at)}</p>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Order Items */}
          {order?.items && order.items.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="text-lg flex items-center gap-2">
                  <Package className="h-5 w-5" />
                  Items ({order.items.length})
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  {order.items.map((item, index) => (
                    <div key={index} className="flex items-center gap-4 p-3 bg-surface-muted rounded-lg">
                      {item.snapshot_image_url ? (
                        <img
                          src={item.snapshot_image_url}
                          alt={item.product_title}
                          className="w-16 h-16 rounded object-cover"
                        />
                      ) : (
                        <div className="w-16 h-16 rounded bg-surface-muted flex items-center justify-center">
                          <Package className="h-8 w-8 text-muted-foreground" />
                        </div>
                      )}
                      <div className="flex-1 min-w-0">
                        <p className="font-medium truncate">{item.product_title}</p>
                        <p className="type-secondary">Qty: {item.quantity} × {formatRupiah(item.unit_price)}</p>
                      </div>
                      <div className="text-right">
                        <p className="font-medium">{formatRupiah(item.subtotal)}</p>
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Pricing Breakdown */}
          {order && (
            <Card>
              <CardHeader>
                <CardTitle className="text-lg flex items-center gap-2">
                  <CreditCard className="h-5 w-5" />
                  Pricing Breakdown
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Subtotal</span>
                    <span>{formatRupiah(order.subtotal)}</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Shipping</span>
                    <span>{formatRupiah(order.shipping_total)}</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Commission</span>
                    <span>{formatRupiah(order.commission_amount)}</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Service Fee</span>
                    <span>{formatRupiah(order.service_fee_amount ?? 0)}</span>
                  </div>
                  <div className="border-t border-border my-2" />
                  <div className="flex justify-between font-medium">
                    <span>Buyer Gross Total</span>
                    <span className="text-primary">
                      {formatRupiah(order.total_payable_amount ?? order.total_before_coins_amount)}
                    </span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Escrow / Seller Eligible</span>
                    <span>{formatRupiah(order.total_before_coins_amount)}</span>
                  </div>
                  {order.refunded_amount > 0 && (
                    <div className="flex justify-between text-sm text-warning">
                      <span>Refunded</span>
                      <span>-{formatRupiah(order.refunded_amount)}</span>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Shipping Address */}
          {order?.shipping_address && (
            <Card>
              <CardHeader>
                <CardTitle className="text-lg flex items-center gap-2">
                  <MapPin className="h-5 w-5" />
                  Shipping Address
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-1">
                  <p className="font-medium">{order.shipping_address.recipient_name}</p>
                  <p className="type-secondary">{order.shipping_address.phone}</p>
                  <p className="type-secondary">{order.shipping_address.address}</p>
                  <p className="type-secondary">
                    {order.shipping_address.city}, {order.shipping_address.province} {order.shipping_address.postal_code}
                  </p>
                </div>
                {order.tracking_number && (
                  <div className="mt-3 p-2 bg-primary/10 rounded">
                    <p className="type-secondary">Shipping Option: {order.shipping_option || 'Standard'}</p>
                    <p className="type-secondary">Tracking: {order.tracking_number}</p>
                  </div>
                )}
              </CardContent>
            </Card>
          )}

          {/* Shipping Origin (Seller's farm/warehouse) */}
          {order?.shipping_origin && (
            <Card>
              <CardHeader>
                <CardTitle className="text-lg flex items-center gap-2">
                  <MapPin className="h-5 w-5" />
                  Shipping Origin (Seller)
                  {order.shipping_source && (
                    <Badge variant="info">{order.shipping_source === 'shipping_quote' ? 'Manual Quote' : 'Fixed-Price Sale Option'}</Badge>
                  )}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-1">
                  <p className="font-medium">{order.shipping_origin.recipient_name}</p>
                  <p className="type-secondary">{order.shipping_origin.phone}</p>
                  <p className="type-secondary">{order.shipping_origin.address}</p>
                  <p className="type-secondary">
                    {order.shipping_origin.city}, {order.shipping_origin.province} {order.shipping_origin.postal_code}
                  </p>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Dispute Info */}
          {order?.dispute && (
            <Card>
              <CardHeader>
                <CardTitle className="text-lg text-warning">Dispute Information</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  <div className="flex justify-between">
                    <span className="type-secondary">Dispute ID</span>
                    <span className="font-mono text-sm">{order.dispute.id}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="type-secondary">Reason</span>
                    <span className="text-sm">{order.dispute.reason}</span>
                  </div>
                  {order.dispute.description && (
                    <div>
                      <p className="type-secondary mb-1">Description</p>
                      <p className="text-sm bg-surface-muted p-2 rounded">{order.dispute.description}</p>
                    </div>
                  )}
                  <div className="flex justify-between">
                    <span className="type-secondary">Status</span>
                    <span className="text-sm">{order.dispute.status}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="type-secondary">Opened</span>
                    <span className="text-sm">{formatDate(order.dispute.opened_at)}</span>
                  </div>
                  {order.dispute.resolved_at && (
                    <div className="flex justify-between">
                      <span className="type-secondary">Resolved</span>
                      <span className="text-sm">{formatDate(order.dispute.resolved_at)}</span>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Refund Info */}
          {order?.refund && (
            <Card className={order.refund.gateway_status === 'failed' ? 'border-destructive' : undefined}>
              <CardHeader>
                <CardTitle className="text-lg flex items-center gap-2">
                  <CreditCard className="h-5 w-5" />
                  Refund
                  <Badge variant={gatewayStatusVariant(order.refund.gateway_status)}>
                    Gateway: {order.refund.gateway_status}
                  </Badge>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Refund ID</span>
                    <span className="font-mono text-xs">{order.refund.id}</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Status</span>
                    <span className="capitalize">{order.refund.status.replace(/_/g, ' ')}</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Reason</span>
                    <span className="capitalize">{order.refund.reason.replace(/_/g, ' ')}</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Requested Amount</span>
                    <span>{formatRupiah(order.refund.requested_amount)}</span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Gateway Attempts</span>
                    <span>{order.refund.gateway_attempts}</span>
                  </div>
                  {order.refund.gateway_refund_id && (
                    <div className="flex justify-between text-sm">
                      <span className="text-muted-foreground">Gateway Refund ID</span>
                      <span className="font-mono text-xs">{order.refund.gateway_refund_id}</span>
                    </div>
                  )}
                  {order.refund.last_gateway_error && (
                    <div className="bg-destructive-bg border border-destructive rounded p-2 mt-1">
                      <p className="text-xs font-medium text-destructive">Last Gateway Error</p>
                      <p className="text-xs text-destructive mt-0.5">{order.refund.last_gateway_error}</p>
                    </div>
                  )}
                  {order.refund.gateway_status === 'succeeded' && (
                    <div className="bg-success-bg border border-success rounded p-2 mt-1">
                      <p className="text-xs text-success">Gateway refund succeeded — funds reversed to buyer&apos;s payment method.</p>
                    </div>
                  )}

                  {/* Gateway retry — gated on capability + not already succeeded */}
                  {canInitiateGatewayRefund && order.refund.gateway_status !== 'succeeded' && (
                    <div className="mt-3 pt-3 border-t border-border">
                      {!gatewayRetryOpen ? (
                        <Button
                          size="sm"
                          variant="warning"
                          onClick={() => {
                            setGatewayRetryAmount(String(order.refund!.requested_amount))
                            setGatewayRetryReason(order.refund!.reason)
                            setGatewayRetryOpen(true)
                            setGatewayRetryError(null)
                            setGatewayRetryResult(null)
                          }}
                        >
                          <RefreshCw className="h-3 w-3 mr-1" />
                          Retry Gateway Refund
                        </Button>
                      ) : (
                        <div className="space-y-3">
                          <div className="bg-warning-bg border border-warning rounded p-3">
                            <p className="text-sm font-semibold text-warning mb-1">Retry Gateway Refund</p>
                            <p className="text-xs text-warning">This may re-attempt refund processing via the payment gateway. An idempotency key is generated automatically to prevent double-dispatch.</p>
                          </div>
                          <div className="space-y-2">
                            <Input
                              type="number"
                              label="Amount (IDR, smallest unit)"
                              size="compact"
                              value={gatewayRetryAmount}
                              onChange={e => setGatewayRetryAmount(e.target.value)}
                              min={1}
                            />
                            <Input
                              type="text"
                              label="Reason"
                              size="compact"
                              value={gatewayRetryReason}
                              onChange={e => setGatewayRetryReason(e.target.value)}
                              placeholder="e.g. admin_retry"
                            />
                          </div>
                          {gatewayRetryError && (
                            <div className="bg-destructive-bg border border-destructive text-destructive p-2 rounded text-xs">
                              {gatewayRetryError}
                            </div>
                          )}
                          {gatewayRetryResult && (
                            <div className="bg-success-bg border border-success text-success p-2 rounded text-xs">
                              Dispatched. Gateway status: <strong>{gatewayRetryResult.gateway_status}</strong>. Attempts: {gatewayRetryResult.gateway_attempts}.
                            </div>
                          )}
                          <div className="flex gap-2">
                            <Button
                              size="sm"
                              variant="warning"
                              onClick={handleGatewayRetry}
                              isLoading={gatewayRetrying}
                              disabled={gatewayRetrying || !gatewayRetryAmount || !gatewayRetryReason.trim()}
                            >
                              Confirm Retry
                            </Button>
                            <Button
                              size="sm"
                              onClick={() => {
                                setGatewayRetryOpen(false)
                                setGatewayRetryError(null)
                                setGatewayRetryResult(null)
                              }}
                              disabled={gatewayRetrying}
                            >
                              Cancel
                            </Button>
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Timeline */}
          {order?.timeline && order.timeline.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="text-lg flex items-center gap-2">
                  <Clock className="h-5 w-5" />
                  Timeline
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  {order.timeline.map((event, index) => (
                    <div key={index} className="flex gap-3">
                      <div className="flex flex-col items-center">
                        <div className="w-2 h-2 rounded-full bg-primary" />
                        {index < order.timeline!.length - 1 && (
                          <div className="w-0.5 flex-1 bg-surface-muted min-h-[40px]" />
                        )}
                      </div>
                      <div className="flex-1 pb-4">
                        <p className="text-sm font-medium">{event.event}</p>
                        <p className="type-caption">{formatDate(event.timestamp)}</p>
                        {event.actor_name && (
                          <p className="type-caption">by {event.actor_name}</p>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Footer */}
          <ModalFooter>
            <Button onClick={onClose}>Close</Button>
          </ModalFooter>
        </div>
      )}
    </Modal>
  )
}
