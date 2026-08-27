import { useState } from 'react'
import { BadgeCheck, ExternalLink, RefreshCw, ShieldAlert } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Modal } from '@/components/ui/Modal'
import {
  useSellerVerifications,
  useVerificationDetail,
  useVerificationActions,
} from '@/hooks/useSellerVerifications'
import { formatDate } from '@/lib/utils'
import type { SellerVerificationListItem } from '@/types'
import {
  verificationStatusLabels,
  verificationStatusVariants,
  documentTypeLabels,
  documentReviewStatusVariants,
} from '@/types'

// ─── Detail Modal ──────────────────────────────────────────────────────────────

type ActionMode = 'approve' | 'reject' | 'resubmission' | 'suspend' | 'revoke' | 'investigate' | 'restore' | null

interface VerificationDetailModalProps {
  sellerId: string | null
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

function VerificationDetailModal({
  sellerId,
  isOpen,
  onClose,
  onSuccess,
}: VerificationDetailModalProps) {
  const { detail, loading: detailLoading, error: detailError } = useVerificationDetail(
    isOpen ? sellerId : null
  )
  const { approve, reject, requestResubmission, suspend, revoke, investigate, restore, markBankReviewed, loading: actionLoading, error: actionError, clearError } =
    useVerificationActions()

  const [actionMode, setActionMode] = useState<ActionMode>(null)
  const [reason, setReason] = useState('')

  const handleAction = async () => {
    if (!sellerId) return
    let ok = false
    if (actionMode === 'approve') {
      ok = await approve(sellerId, reason || undefined)
    } else if (actionMode === 'reject') {
      ok = await reject(sellerId, reason)
    } else if (actionMode === 'resubmission') {
      ok = await requestResubmission(sellerId, reason)
    } else if (actionMode === 'suspend') {
      ok = await suspend(sellerId, reason)
    } else if (actionMode === 'revoke') {
      ok = await revoke(sellerId, reason)
    } else if (actionMode === 'investigate') {
      ok = await investigate(sellerId, reason)
    } else if (actionMode === 'restore') {
      ok = await restore(sellerId, reason || undefined)
    }
    if (ok) {
      setActionMode(null)
      setReason('')
      onSuccess()
      onClose()
    }
  }

  const openAction = (mode: ActionMode) => {
    clearError()
    setReason('')
    setActionMode(mode)
  }

  const cancelAction = () => {
    clearError()
    setActionMode(null)
    setReason('')
  }

  const reasonRequired = actionMode === 'reject' || actionMode === 'resubmission' || actionMode === 'suspend' || actionMode === 'revoke' || actionMode === 'investigate'
  const canSubmit = !reasonRequired || reason.trim().length >= 3

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Verification Review" size="xl">
      {detailLoading && (
        <div className="flex items-center justify-center py-12">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent" />
        </div>
      )}

      {detailError && !detailLoading && (
        <div className="rounded-lg bg-red-50 p-4 text-sm text-red-700">
          Failed to load verification detail: {detailError.message}
        </div>
      )}

      {detail && !detailLoading && (
        <div className="space-y-6">
          {/* Status header */}
          <div className="flex items-center gap-3">
            <span className="text-sm font-medium text-gray-500">Status:</span>
            <Badge variant={verificationStatusVariants[detail.status]}>
              {verificationStatusLabels[detail.status]}
            </Badge>
            {detail.submitted_at && (
              <span className="text-xs text-gray-400">
                Submitted {formatDate(detail.submitted_at)}
              </span>
            )}
          </div>

          {/* Prior rejection reason */}
          {detail.reason && (
            <div className="rounded-lg border border-orange-200 bg-orange-50 p-3 text-sm text-orange-800">
              <span className="font-medium">Prior reason: </span>{detail.reason}
            </div>
          )}

          {/* Seller identity */}
          <div>
            <h3 className="mb-3 text-sm font-semibold text-gray-700">Seller Identity</h3>
            <div className="rounded-lg border border-gray-200 px-4 py-3">
              <div className="space-y-0.5">
                <p className="text-sm font-medium text-gray-800">
                  {detail.seller_username ? `@${detail.seller_username}` : 'Unknown'}
                </p>
                {detail.seller_farm_name && (
                  <p className="text-sm text-gray-500">{detail.seller_farm_name}</p>
                )}
                <p className="font-mono text-xs text-gray-400">{detail.seller_id}</p>
              </div>
            </div>
          </div>

          {/* Documents */}
          <div>
            <h3 className="mb-3 text-sm font-semibold text-gray-700">Submitted Documents</h3>
            {detail.documents.length === 0 ? (
              <p className="text-sm text-gray-400 italic">No documents submitted.</p>
            ) : (
              <div className="space-y-2">
                {detail.documents.map((doc) => (
                  <div
                    key={doc.id}
                    className="flex items-center justify-between rounded-lg border border-gray-200 px-4 py-3"
                  >
                    <div className="space-y-0.5">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium text-gray-800">
                          {documentTypeLabels[doc.document_type] ?? doc.document_type}
                        </span>
                        <Badge variant={documentReviewStatusVariants[doc.status]} className="text-xs">
                          {doc.status}
                        </Badge>
                      </div>
                      <p className="text-xs text-gray-500">{doc.document_name}</p>
                      {doc.rejection_note && (
                        <p className="text-xs text-red-600">Note: {doc.rejection_note}</p>
                      )}
                    </div>
                    <a
                      href={doc.view_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-1 text-xs text-primary hover:underline"
                    >
                      View <ExternalLink className="h-3 w-3" />
                    </a>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Bank Accounts */}
          <div>
            <h3 className="mb-3 text-sm font-semibold text-gray-700">Registered Bank Accounts</h3>
            {/* Warning: default account not reviewed for payout */}
            {detail.status === 'approved' && detail.bank_accounts.some((ba) => ba.is_default && !ba.is_reviewed_for_payout) && (
              <div className="mb-3 rounded-lg border border-orange-200 bg-orange-50 p-3 text-sm text-orange-800">
                <span className="font-semibold">Warning:</span> The default bank account has not been reviewed for payout.
                Withdrawal requests will be blocked (GUARD 5) until an admin marks it reviewed.
              </div>
            )}
            {detail.bank_accounts.length === 0 ? (
              <p className="text-sm text-gray-400 italic">No bank accounts registered.</p>
            ) : (
              <div className="space-y-2">
                {detail.bank_accounts.map((ba) => (
                  <div
                    key={ba.id}
                    className="flex items-start justify-between rounded-lg border border-gray-200 px-4 py-3"
                  >
                    <div className="space-y-0.5">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium text-gray-800">
                          {ba.bank_name} ({ba.bank_code})
                        </span>
                        {ba.is_default && (
                          <Badge variant="info" className="text-xs">Default</Badge>
                        )}
                        {ba.is_reviewed_for_payout ? (
                          <Badge variant="success" className="text-xs">Reviewed for Payout</Badge>
                        ) : (
                          <Badge variant="warning" className="text-xs">Not Reviewed</Badge>
                        )}
                      </div>
                      <p className="font-mono text-xs text-gray-700">{ba.account_number}</p>
                      <p className="text-xs text-gray-500">{ba.account_holder_name}</p>
                    </div>
                    {/* Mark reviewed action — only for approved sellers + unreviewed active accounts */}
                    {detail.status === 'approved' && !ba.is_reviewed_for_payout && (
                      <Button
                        variant="secondary"
                        size="sm"
                        isLoading={actionLoading}
                        disabled={actionLoading}
                        onClick={async () => {
                          if (!sellerId) return
                          const ok = await markBankReviewed(sellerId, ba.id)
                          if (ok) onSuccess()
                        }}
                      >
                        Mark Reviewed
                      </Button>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Action section */}
          {actionMode === null ? (
            <div className="space-y-3 border-t border-gray-200 pt-4">
              {/* Review actions — only for pending_review */}
              {detail.status === 'pending_review' && (
                <div className="flex gap-3">
                  <Button variant="success" size="sm" onClick={() => openAction('approve')} disabled={actionLoading}>
                    Approve
                  </Button>
                  <Button variant="warning" size="sm" onClick={() => openAction('resubmission')} disabled={actionLoading}>
                    Request Resubmission
                  </Button>
                  <Button variant="danger" size="sm" onClick={() => openAction('reject')} disabled={actionLoading}>
                    Reject
                  </Button>
                </div>
              )}

              {/* Lifecycle actions — based on current status */}
              {detail.status === 'approved' && (
                <div className="flex gap-3">
                  <Button variant="warning" size="sm" onClick={() => openAction('investigate')} disabled={actionLoading}>
                    Investigate
                  </Button>
                  <Button variant="warning" size="sm" onClick={() => openAction('suspend')} disabled={actionLoading}>
                    Suspend
                  </Button>
                  <Button variant="danger" size="sm" onClick={() => openAction('revoke')} disabled={actionLoading}>
                    Revoke
                  </Button>
                </div>
              )}

              {detail.status === 'under_investigation' && (
                <div className="flex gap-3">
                  <Button variant="success" size="sm" onClick={() => openAction('restore')} disabled={actionLoading}>
                    Restore
                  </Button>
                  <Button variant="warning" size="sm" onClick={() => openAction('suspend')} disabled={actionLoading}>
                    Suspend
                  </Button>
                  <Button variant="danger" size="sm" onClick={() => openAction('revoke')} disabled={actionLoading}>
                    Revoke
                  </Button>
                </div>
              )}

              {detail.status === 'suspended' && (
                <div className="flex gap-3">
                  <Button variant="success" size="sm" onClick={() => openAction('restore')} disabled={actionLoading}>
                    Restore
                  </Button>
                </div>
              )}
            </div>
          ) : (
            <div className="space-y-3 border-t border-gray-200 pt-4">
              <p className="text-sm font-medium text-gray-700">
                {actionMode === 'approve' && 'Approve verification (reason optional)'}
                {actionMode === 'reject' && 'Reject verification — reason required'}
                {actionMode === 'resubmission' && 'Request resubmission — reason required'}
                {actionMode === 'suspend' && 'Suspend verification — reason required'}
                {actionMode === 'revoke' && 'Revoke verification (TERMINAL) — reason required'}
                {actionMode === 'investigate' && 'Mark under investigation — reason required'}
                {actionMode === 'restore' && 'Restore to approved (reason optional)'}
              </p>
              {actionMode === 'revoke' && (
                <p className="text-xs text-red-600 font-medium">
                  Warning: Revocation is terminal. There is no recovery path.
                </p>
              )}
              <textarea
                className="w-full rounded-lg border border-gray-300 p-2 text-sm focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                rows={3}
                placeholder={reasonRequired ? 'Enter reason (required, min 3 chars)...' : 'Enter reason (optional)...'}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
              {actionError && (
                <p className="text-sm text-red-600">{actionError}</p>
              )}
              <div className="flex gap-2">
                <Button
                  variant={
                    actionMode === 'approve' || actionMode === 'restore'
                      ? 'success'
                      : actionMode === 'reject' || actionMode === 'revoke'
                      ? 'danger'
                      : 'warning'
                  }
                  size="sm"
                  onClick={handleAction}
                  isLoading={actionLoading}
                  disabled={actionLoading || !canSubmit}
                >
                  Confirm{' '}
                  {actionMode === 'approve' && 'Approval'}
                  {actionMode === 'reject' && 'Rejection'}
                  {actionMode === 'resubmission' && 'Resubmission Request'}
                  {actionMode === 'suspend' && 'Suspension'}
                  {actionMode === 'revoke' && 'Revocation'}
                  {actionMode === 'investigate' && 'Investigation'}
                  {actionMode === 'restore' && 'Restoration'}
                </Button>
                <Button variant="secondary" size="sm" onClick={cancelAction} disabled={actionLoading}>
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

// ─── Status Filter Options ───────────────────────────────────────────────────

const statusFilterOptions = [
  { value: 'pending_review', label: 'Pending Review' },
  { value: 'approved', label: 'Approved' },
  { value: 'under_investigation', label: 'Under Investigation' },
  { value: 'suspended', label: 'Suspended' },
  { value: 'needs_resubmission', label: 'Needs Resubmission' },
  { value: 'rejected', label: 'Rejected' },
  { value: 'revoked', label: 'Revoked' },
] as const

// ─── Page ──────────────────────────────────────────────────────────────────────

export function SellerVerificationsPage() {
  const [statusFilter, setStatusFilter] = useState('pending_review')
  const { items, loading, error, refetch } = useSellerVerifications(statusFilter)
  const [selected, setSelected] = useState<SellerVerificationListItem | null>(null)
  const [isModalOpen, setIsModalOpen] = useState(false)

  const handleView = (item: SellerVerificationListItem) => {
    setSelected(item)
    setIsModalOpen(true)
  }

  const handleClose = () => {
    setIsModalOpen(false)
    setSelected(null)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent" />
          <p className="mt-4 text-gray-600">Loading verifications...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Seller Verifications</h1>
          <p className="text-gray-600 mt-1">Review pending seller verification submissions</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="flex items-center gap-3 text-red-600">
              <ShieldAlert className="h-5 w-5 shrink-0" />
              <p className="text-sm">{error.message}</p>
            </div>
            <Button variant="secondary" size="sm" onClick={refetch} className="mt-4">
              <RefreshCw className="mr-2 h-4 w-4" /> Retry
            </Button>
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
          <h1 className="text-3xl font-bold text-gray-900">Seller Verifications</h1>
          <p className="text-gray-600 mt-1">Review and manage seller verification lifecycle</p>
        </div>
        <Button variant="secondary" size="sm" onClick={refetch}>
          <RefreshCw className="mr-2 h-4 w-4" /> Refresh
        </Button>
      </div>

      {/* Status Filter */}
      <div className="flex items-center gap-3">
        <label htmlFor="status-filter" className="text-sm font-medium text-gray-700">
          Filter by status:
        </label>
        <select
          id="status-filter"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
        >
          {statusFilterOptions.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>

      {/* Summary */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BadgeCheck className="h-5 w-5 text-primary" />
            {statusFilterOptions.find((o) => o.value === statusFilter)?.label ?? 'Verifications'}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-3xl font-bold text-gray-900">{items.length}</p>
          <p className="text-sm text-gray-500 mt-1">records matching filter</p>
        </CardContent>
      </Card>

      {/* Table */}
      <Card>
        <CardContent className="p-0">
          {items.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-gray-400">
              <BadgeCheck className="h-12 w-12 mb-3 text-green-300" />
              <p className="text-lg font-medium">No results</p>
              <p className="text-sm mt-1">No verifications with this status</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Seller</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Submitted</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>
                      {item.seller_username ? (
                        <div className="space-y-0.5">
                          <span className="text-sm font-medium text-gray-800">@{item.seller_username}</span>
                          {item.seller_farm_name && (
                            <div className="text-xs text-gray-500">{item.seller_farm_name}</div>
                          )}
                          <div className="font-mono text-xs text-gray-400">{item.seller_id.slice(0, 8)}…</div>
                        </div>
                      ) : (
                        <span className="font-mono text-xs text-gray-600">{item.seller_id}</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <Badge variant={verificationStatusVariants[item.status]}>
                        {verificationStatusLabels[item.status]}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm text-gray-600">
                      {item.submitted_at ? formatDate(item.submitted_at) : '—'}
                    </TableCell>
                    <TableCell className="text-sm text-gray-600">
                      {formatDate(item.created_at)}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="sm" onClick={() => handleView(item)}>
                        Review
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <VerificationDetailModal
        sellerId={selected?.seller_id ?? null}
        isOpen={isModalOpen}
        onClose={handleClose}
        onSuccess={refetch}
      />
    </div>
  )
}
