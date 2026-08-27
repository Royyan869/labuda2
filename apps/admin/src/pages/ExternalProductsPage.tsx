import { useState, useEffect, useCallback } from 'react'
import { Package, Eye, RefreshCw, ExternalLink, AlertTriangle, Filter, Check, X, Clock } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Modal } from '@/components/ui/Modal'
import {
  listAdminExternalProducts,
  getAdminExternalProduct,
  approveExternalProduct,
  rejectExternalProduct,
  requestChangesExternalProduct,
  hideExternalProduct,
} from '@/lib/api/externalProducts'
import { formatDate } from '@/lib/utils'
import type { AdminExternalProduct, ExternalProductReviewStatus } from '@/types/external-product'
import {
  externalProductStatusLabels,
  externalProductStatusVariants,
} from '@/types/external-product'

// ─── Status filter options ────────────────────────────────────────────────────

const STATUS_OPTIONS: { value: ExternalProductReviewStatus | ''; label: string }[] = [
  { value: '', label: 'All Statuses' },
  { value: 'pending_review', label: 'Pending Review' },
  { value: 'approved', label: 'Approved' },
  { value: 'rejected', label: 'Rejected' },
  { value: 'request_changes', label: 'Changes Requested' },
  { value: 'hidden', label: 'Hidden' },
  { value: 'draft', label: 'Draft' },
]

// ─── Detail Modal ─────────────────────────────────────────────────────────────

type ActionMode = 'approve' | 'reject' | 'request_changes' | 'hide' | null

interface DetailModalProps {
  productId: string | null
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

function DetailModal({ productId, isOpen, onClose, onSuccess }: DetailModalProps) {
  const [product, setProduct] = useState<AdminExternalProduct | null>(null)
  const [loading, setLoading] = useState(false)
  const [fetchError, setFetchError] = useState<string | null>(null)

  const [actionMode, setActionMode] = useState<ActionMode>(null)
  const [reason, setReason] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)

  const fetchDetail = useCallback(async () => {
    if (!productId) return
    setLoading(true)
    setFetchError(null)
    try {
      const data = await getAdminExternalProduct(productId)
      setProduct(data)
    } catch (err) {
      setFetchError(err instanceof Error ? err.message : 'Failed to load product')
    } finally {
      setLoading(false)
    }
  }, [productId])

  useEffect(() => {
    if (isOpen && productId) {
      setProduct(null)
      setActionMode(null)
      setReason('')
      setSubmitError(null)
      fetchDetail()
    }
  }, [isOpen, productId, fetchDetail])

  function openAction(mode: ActionMode) {
    setReason('')
    setSubmitError(null)
    setActionMode(mode)
  }

  function cancelAction() {
    setActionMode(null)
    setSubmitError(null)
  }

  async function executeAction() {
    if (!product || !actionMode) return

    const reasonRequired = actionMode === 'reject' || actionMode === 'request_changes'
    if (reasonRequired && !reason.trim()) {
      setSubmitError('Reason is required for this action')
      return
    }

    setSubmitting(true)
    setSubmitError(null)
    try {
      if (actionMode === 'approve') {
        await approveExternalProduct(product.id, reason || undefined)
      } else if (actionMode === 'reject') {
        await rejectExternalProduct(product.id, reason)
      } else if (actionMode === 'request_changes') {
        await requestChangesExternalProduct(product.id, reason)
      } else if (actionMode === 'hide') {
        await hideExternalProduct(product.id, reason || undefined)
      }
      setActionMode(null)
      onSuccess()
      onClose()
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Action failed')
    } finally {
      setSubmitting(false)
    }
  }

  const actionLabels: Record<NonNullable<ActionMode>, string> = {
    approve: 'Approve',
    reject: 'Reject',
    request_changes: 'Request Changes',
    hide: 'Hide',
  }

  const actionVariants: Record<NonNullable<ActionMode>, 'primary' | 'danger' | 'secondary'> = {
    approve: 'primary',
    reject: 'danger',
    request_changes: 'secondary',
    hide: 'danger',
  }

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="External Product Review" size="lg">
      {loading && (
        <div className="flex items-center justify-center py-12">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent" />
        </div>
      )}

      {fetchError && (
        <div className="flex items-center gap-2 text-red-600 py-4">
          <AlertTriangle className="h-4 w-4" />
          <span className="text-sm">{fetchError}</span>
        </div>
      )}

      {product && !loading && (
        <div className="space-y-6">
          {/* Status & URL */}
          <div className="flex items-center justify-between gap-4">
            <Badge variant={externalProductStatusVariants[product.review_status]}>
              {externalProductStatusLabels[product.review_status]}
            </Badge>
            {product.unsafe_url_flag && (
              <Badge variant="error">
                <AlertTriangle className="h-3 w-3 mr-1" />
                Unsafe URL Flagged
              </Badge>
            )}
            <a
              href={product.external_url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1 text-sm text-blue-600 hover:underline"
            >
              <ExternalLink className="h-3 w-3" />
              Open URL
            </a>
          </div>

          {/* Core Info */}
          <dl className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
            <dt className="font-medium text-gray-600">Title</dt>
            <dd className="font-semibold text-gray-900">{product.title}</dd>

            {product.description && (
              <>
                <dt className="font-medium text-gray-600">Description</dt>
                <dd className="text-gray-900 break-words">{product.description}</dd>
              </>
            )}

            <dt className="font-medium text-gray-600">Owner ID</dt>
            <dd className="font-mono text-xs text-gray-700">{product.owner_user_id}</dd>

            <dt className="font-medium text-gray-600">Product ID</dt>
            <dd className="font-mono text-xs text-gray-700">{product.id}</dd>

            <dt className="font-medium text-gray-600">Normalized URL</dt>
            <dd className="font-mono text-xs text-gray-700 break-all">
              {product.normalized_external_url}
            </dd>

            <dt className="font-medium text-gray-600">Submitted</dt>
            <dd className="text-gray-700">{product.submitted_at ? formatDate(product.submitted_at) : '-'}</dd>

            {product.approved_at && (
              <>
                <dt className="font-medium text-gray-600">Approved</dt>
                <dd className="text-gray-700">{formatDate(product.approved_at)}</dd>
              </>
            )}

            {product.rejected_at && (
              <>
                <dt className="font-medium text-gray-600">Rejected</dt>
                <dd className="text-gray-700">{formatDate(product.rejected_at)}</dd>
              </>
            )}

            {product.hidden_at && (
              <>
                <dt className="font-medium text-gray-600">Hidden</dt>
                <dd className="text-gray-700">{formatDate(product.hidden_at)}</dd>
              </>
            )}

            {product.rejection_reason && (
              <>
                <dt className="font-medium text-gray-600">Last Reason</dt>
                <dd className="text-gray-700">{product.rejection_reason}</dd>
              </>
            )}

            {product.last_reviewed_by && (
              <>
                <dt className="font-medium text-gray-600">Reviewed By</dt>
                <dd className="font-mono text-xs text-gray-700">
                  {product.last_reviewed_by.slice(0, 8)}…
                </dd>
              </>
            )}
          </dl>

          {/* Media */}
          {product.media && product.media.length > 0 && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">
                Media ({product.media.length})
              </h3>
              <div className="flex flex-wrap gap-2">
                {product.media.map((m) => (
                  <a
                    key={m.id}
                    href={m.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="block h-16 w-16 rounded border border-gray-200 overflow-hidden bg-gray-50 flex items-center justify-center hover:opacity-80"
                    title={m.media_type}
                  >
                    {m.media_type === 'image' ? (
                      <img src={m.thumbnail_url ?? m.url} alt="" className="h-full w-full object-cover" />
                    ) : (
                      <span className="text-xs text-gray-500">Video</span>
                    )}
                  </a>
                ))}
              </div>
            </div>
          )}

          {/* Review History */}
          {product.review_history && product.review_history.length > 0 && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">Review History</h3>
              <div className="space-y-2">
                {product.review_history.map((h) => (
                  <div key={h.id} className="flex items-start gap-2 text-xs text-gray-600 bg-gray-50 rounded p-2">
                    <Clock className="h-3 w-3 mt-0.5 flex-shrink-0 text-gray-400" />
                    <div>
                      <span className="font-medium text-gray-800">
                        {h.from_status ?? 'initial'} → {h.to_status}
                      </span>
                      {h.reason && <span className="ml-2 text-gray-500">"{h.reason}"</span>}
                      <div className="text-gray-400 mt-0.5">{formatDate(h.created_at)}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Action Buttons */}
          {!actionMode && (
            <div className="flex flex-wrap items-center gap-2 pt-2 border-t border-gray-200">
              {product.can_approve && (
                <Button size="sm" onClick={() => openAction('approve')}>
                  <Check className="h-4 w-4 mr-1" />
                  Approve
                </Button>
              )}
              {product.can_reject && (
                <Button size="sm" variant="danger" onClick={() => openAction('reject')}>
                  <X className="h-4 w-4 mr-1" />
                  Reject
                </Button>
              )}
              {product.can_approve && (
                <Button size="sm" variant="secondary" onClick={() => openAction('request_changes')}>
                  Request Changes
                </Button>
              )}
              {product.can_hide && (
                <Button size="sm" variant="danger" onClick={() => openAction('hide')}>
                  Hide
                </Button>
              )}
            </div>
          )}

          {/* Action Form */}
          {actionMode && (
            <div className="border border-gray-200 rounded-lg p-4 space-y-3 bg-gray-50">
              <h3 className="text-sm font-semibold text-gray-800">
                {actionLabels[actionMode]}
                {(actionMode === 'reject' || actionMode === 'request_changes') && (
                  <span className="ml-1 text-red-500">*</span>
                )}
              </h3>
              <textarea
                rows={3}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder={
                  actionMode === 'reject'
                    ? 'Reason for rejection (required)'
                    : actionMode === 'request_changes'
                    ? 'Describe what needs to be changed (required)'
                    : 'Optional note'
                }
                className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              {submitError && (
                <p className="text-sm text-red-600 flex items-center gap-1">
                  <AlertTriangle className="h-4 w-4" />
                  {submitError}
                </p>
              )}
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  variant={actionVariants[actionMode]}
                  onClick={executeAction}
                  disabled={submitting}
                >
                  {submitting ? 'Saving…' : `Confirm ${actionLabels[actionMode]}`}
                </Button>
                <Button size="sm" variant="ghost" onClick={cancelAction} disabled={submitting}>
                  Cancel
                </Button>
              </div>
            </div>
          )}
        </div>
      )}
    </Modal>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export function ExternalProductsPage() {
  const [statusFilter, setStatusFilter] = useState<ExternalProductReviewStatus | ''>('pending_review')
  const [products, setProducts] = useState<AdminExternalProduct[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [isModalOpen, setIsModalOpen] = useState(false)

  const fetchProducts = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await listAdminExternalProducts({ status: statusFilter || undefined })
      setProducts(res.items ?? [])
      setTotal(res.count ?? 0)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load products')
    } finally {
      setLoading(false)
    }
  }, [statusFilter])

  useEffect(() => {
    fetchProducts()
  }, [fetchProducts])

  function openDetail(id: string) {
    setSelectedId(id)
    setIsModalOpen(true)
  }

  const pendingCount = products.filter((p) => p.review_status === 'pending_review').length

  if (loading && products.length === 0) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent" />
          <p className="mt-4 text-gray-600">Loading external products…</p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">External Product Review</h1>
          <p className="text-gray-600 mt-1">
            Review seller-submitted external products for promotion discovery.
          </p>
        </div>
        <Button variant="ghost" size="sm" onClick={fetchProducts} disabled={loading}>
          <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Stats */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-600">
                {statusFilter ? externalProductStatusLabels[statusFilter as ExternalProductReviewStatus] : 'All'} Products
              </p>
              <p className="text-3xl font-bold text-primary mt-1">{total}</p>
              {!statusFilter && (
                <p className="text-xs text-gray-500 mt-1">{pendingCount} pending review</p>
              )}
            </div>
            <div className="p-4 rounded-lg bg-purple-100">
              <Package className="h-8 w-8 text-purple-600" />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center gap-4">
            <Filter className="h-5 w-5 text-gray-500" />
            <label htmlFor="status-filter" className="text-sm font-medium text-gray-700">
              Status:
            </label>
            <select
              id="status-filter"
              value={statusFilter}
              onChange={(e) =>
                setStatusFilter(e.target.value as ExternalProductReviewStatus | '')
              }
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            >
              {STATUS_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </div>
        </CardContent>
      </Card>

      {/* Error */}
      {error && (
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center gap-2 text-red-600">
              <AlertTriangle className="h-5 w-5" />
              <p className="text-sm">{error}</p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Table */}
      <Card>
        <CardHeader>
          <CardTitle>Review Queue</CardTitle>
        </CardHeader>
        <CardContent>
          {products.length === 0 && !loading ? (
            <div className="text-center py-12">
              <Package className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Products Found</h3>
              <p className="text-gray-600">
                {statusFilter
                  ? `No external products with status "${externalProductStatusLabels[statusFilter as ExternalProductReviewStatus]}".`
                  : 'No external products in the review queue.'}
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>Title</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>URL</TableHead>
                    <TableHead>Owner</TableHead>
                    <TableHead>Submitted</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {products.map((p) => (
                    <TableRow key={p.id}>
                      <TableCell className="font-mono text-xs text-gray-600">
                        {p.id.slice(0, 8)}
                      </TableCell>
                      <TableCell>
                        <div className="max-w-xs">
                          <div className="font-medium text-gray-900 truncate">{p.title}</div>
                          {p.unsafe_url_flag && (
                            <Badge variant="error" className="mt-1 text-xs">
                              Unsafe URL
                            </Badge>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={externalProductStatusVariants[p.review_status]}>
                          {externalProductStatusLabels[p.review_status]}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <a
                          href={p.external_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex items-center gap-1 text-xs text-blue-600 hover:underline max-w-[120px] truncate"
                          title={p.external_url}
                        >
                          <ExternalLink className="h-3 w-3 flex-shrink-0" />
                          {p.normalized_external_url.replace(/^https?:\/\//, '').slice(0, 30)}
                        </a>
                      </TableCell>
                      <TableCell className="font-mono text-xs text-gray-600">
                        {p.owner_user_id.slice(0, 8)}
                      </TableCell>
                      <TableCell className="text-xs text-gray-600">
                        {p.submitted_at ? formatDate(p.submitted_at) : '-'}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button size="sm" onClick={() => openDetail(p.id)}>
                          <Eye className="h-4 w-4 mr-1" />
                          Review
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

      {/* Detail Modal */}
      <DetailModal
        productId={selectedId}
        isOpen={isModalOpen}
        onClose={() => {
          setIsModalOpen(false)
          setSelectedId(null)
        }}
        onSuccess={fetchProducts}
      />
    </div>
  )
}
