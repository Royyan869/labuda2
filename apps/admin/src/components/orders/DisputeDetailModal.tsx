import { useState, useEffect } from 'react'
import { AlertTriangle, User, RefreshCw, CheckCircle, XCircle, FileText } from 'lucide-react'
import { Modal, ModalFooter } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Textarea } from '@/components/ui/Textarea'
import { useDisputeDetail, useDisputeResolution } from '@/hooks/useDisputes'
import { formatDate, formatRupiah } from '@/lib/utils'
import { AdminLoadingState } from '@/components/common'
import { hasCapability } from '@/lib/permissions'
import { useAuth } from '@/hooks/useAuth'
import type {
  DisputeListItem,
  DisputeStatus,
} from '@/types'
import {
  disputeStatusLabels,
  disputeStatusVariants,
  disputeReasonLabels,
} from '@/types'

interface DisputeDetailModalProps {
  isOpen: boolean
  onClose: () => void
  disputeData: DisputeListItem | null
  onResolutionComplete?: () => void
}

// Confirmation messages for each action type
const ACTION_CONFIRMATIONS = {
  approve: {
    title: 'Approve Dispute (Refund Buyer)',
    message: 'This will refund the buyer and release funds from escrow. The seller will not receive payment. This action cannot be undone. Continue?',
    variant: 'warning' as const,
  },
  reject: {
    title: 'Reject Dispute (Release to Seller)',
    message: 'This will reject the dispute and release payment to the seller. The buyer will not receive a refund. This action cannot be undone. Continue?',
    variant: 'danger' as const,
  },
}

export function DisputeDetailModal({ isOpen, onClose, disputeData, onResolutionComplete }: DisputeDetailModalProps) {
  const [actionNotes, setActionNotes] = useState('')
  const [selectedAction, setSelectedAction] = useState<'approve' | 'reject' | null>(null)
  const [showConfirm, setShowConfirm] = useState(false)
  const [staleStatus, setStaleStatus] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const { dispute, loading, refetch } = useDisputeDetail(disputeData?.id || null)
  const { resolve, loading: isSubmitting } = useDisputeResolution()
  const { capabilities } = useAuth()

  const canResolveDisputes = hasCapability(capabilities, 'finance.dispute.resolve')
  const requiredCapability = 'finance.dispute.resolve'

  // Reset state when modal closes or dispute changes
  useEffect(() => {
    if (!isOpen) {
      const timer = window.setTimeout(() => {
        setActionNotes('')
        setSelectedAction(null)
        setShowConfirm(false)
        setStaleStatus(null)
        setError(null)
      }, 0)
      return () => window.clearTimeout(timer)
    }
  }, [isOpen])

  if (!disputeData) return null

  const displayData = dispute || disputeData

  /**
   * Verify dispute status hasn't changed before action (data freshness check)
   * Returns true if status is valid for action, false if stale
   */
  const verifyStatusFresh = (): boolean => {
    // If we have fresh detail data, check status
    if (dispute && dispute.status !== disputeData.status) {
      setStaleStatus(`Dispute status has changed from "${disputeData.status}" to "${dispute.status}". Please refresh and review the current state before taking action.`)
      return false
    }
    // Only opened disputes can be acted upon
    if (dispute && dispute.status !== 'under_review') {
      setStaleStatus(`This dispute is no longer opened (current status: "${dispute.status}"). Actions can only be taken on opened disputes.`)
      return false
    }
    if (!dispute && disputeData.status !== 'under_review') {
      setStaleStatus(`This dispute is no longer opened. Actions can only be taken on opened disputes.`)
      return false
    }
    return true
  }

  const handleActionClick = async (action: 'approve' | 'reject') => {
    setError(null)
    setStaleStatus(null)

    // Re-fetch latest data before action (data freshness)
    try {
      await refetch()
    } catch {
      // Continue anyway, verifyStatusFresh will handle status mismatch
    }

    if (!verifyStatusFresh()) {
      return
    }

    setSelectedAction(action)
    setShowConfirm(true)
  }

  const handleConfirmAction = async () => {
    if (!selectedAction || !disputeData.id) return

    // Final status check before submitting
    if (!verifyStatusFresh()) {
      setShowConfirm(false)
      return
    }

    try {
      await resolve(disputeData.id, selectedAction, actionNotes || undefined)
      setShowConfirm(false)
      setSelectedAction(null)
      onClose()
      onResolutionComplete?.()
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to resolve dispute'
      setError(message)
      setShowConfirm(false)
    }
  }

  const handleCancelConfirm = () => {
    setShowConfirm(false)
    setSelectedAction(null)
  }

  const isOpened = (dispute || disputeData).status === 'under_review'
  const confirmConfig = selectedAction ? ACTION_CONFIRMATIONS[selectedAction] : null

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Dispute Details" size="lg">
      {loading ? (
        <AdminLoadingState embedded label="Loading dispute details" />
      ) : (
        <div className="space-y-6">
          {/* Status Badge with Refresh */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Badge variant={disputeStatusVariants[displayData.status as DisputeStatus] || 'info'}>
                {disputeStatusLabels[displayData.status as DisputeStatus] || displayData.status}
              </Badge>
              {dispute?.resolution_overdue && (
                <Badge variant="error">Resolution Overdue</Badge>
              )}
              {!dispute?.resolution_overdue && dispute?.admin_response_overdue && (
                <Badge variant="warning">Response Overdue</Badge>
              )}
              <button
                onClick={() => {
                  setStaleStatus(null)
                  refetch()
                }}
                className="text-muted-foreground hover:text-muted-foreground transition-colors"
                title="Refresh dispute data"
              >
                <RefreshCw className="h-4 w-4" />
              </button>
            </div>
            <span className="type-secondary">
              Dispute ID: <span className="font-mono">{displayData.id}</span>
            </span>
          </div>

          {/* Stale Status Warning */}
          {staleStatus && (
            <div className="bg-warning-bg border border-warning text-warning p-4 rounded-lg flex items-start gap-2">
              <AlertTriangle className="h-5 w-5 flex-shrink-0 mt-0.5" />
              <div className="flex-1">
                <p className="font-medium text-sm">Status Changed</p>
                <p className="text-sm mt-1">{staleStatus}</p>
              </div>
            </div>
          )}

          {/* Error Message */}
          {error && (
            <div className="bg-destructive-bg border border-destructive text-destructive p-3 rounded-lg flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 flex-shrink-0" />
              <span className="text-sm">{error}</span>
            </div>
          )}

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
                  <p className="type-secondary">Buyer (Complainant)</p>
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
                      <p className="font-medium">{displayData.buyer_username || 'Unknown'}</p>
                      <p className="font-mono type-caption">{displayData.buyer_id}</p>
                    </div>
                  </div>
                </div>

                {/* Seller */}
                <div className="space-y-2">
                  <p className="type-secondary">Seller (Respondent)</p>
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
                      <p className="font-medium">{displayData.seller_username || 'Unknown'}</p>
                      {displayData.seller_farm_name && (
                        <p className="type-secondary">{displayData.seller_farm_name}</p>
                      )}
                      <p className="font-mono type-caption">{displayData.seller_id}</p>
                    </div>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Dispute Information */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg flex items-center gap-2">
                <FileText className="h-5 w-5" />
                Dispute Information
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="type-secondary">Order ID</p>
                  <p className="font-mono text-sm">{displayData.order_id}</p>
                </div>
                <div>
                  <p className="type-secondary">Opened Date</p>
                  <p className="text-sm">{formatDate(displayData.opened_at)}</p>
                </div>
              </div>
              <div>
                <p className="type-secondary">Reason</p>
                <p className="font-medium">
                  {disputeReasonLabels[displayData.reason] || displayData.reason}
                </p>
              </div>
              {displayData.description && (
                <div>
                  <p className="type-secondary mb-1">Description</p>
                  <p className="text-foreground bg-surface-muted p-3 rounded-lg whitespace-pre-wrap break-words">
                    {displayData.description}
                  </p>
                </div>
              )}

              {/* Resolution Info (if resolved) */}
              {displayData.resolved_at && (
                <>
                  <div className="border-t border-border pt-4">
                    <p className="type-secondary">Resolution</p>
                    <p className="font-medium">{displayData.status === 'resolved_refund' ? 'Refunded to Buyer' : 'Released to Seller'}</p>
                  </div>
                  {displayData.resolution_notes && (
                    <div>
                      <p className="type-secondary mb-1">Admin Notes</p>
                      <p className="text-foreground bg-surface-muted p-3 rounded-lg whitespace-pre-wrap break-words">
                        {displayData.resolution_notes}
                      </p>
                    </div>
                  )}
                  <div>
                    <p className="type-secondary">Resolved At</p>
                    <p className="text-sm">{formatDate(displayData.resolved_at)}</p>
                  </div>
                </>
              )}

              {/* Order Context */}
              {(dispute?.order_status || dispute?.total_before_coins_amount != null || dispute?.shipping_reference) && (
                <div className="border-t border-border pt-4 space-y-3">
                  <div className="grid grid-cols-2 gap-4">
                    {dispute?.order_status && (
                      <div>
                        <p className="type-secondary">Order Status</p>
                        <p className="text-sm capitalize">{dispute.order_status.replace(/_/g, ' ')}</p>
                      </div>
                    )}
                    {dispute?.order_escrow_status && (
                      <div>
                        <p className="type-secondary">Escrow Status</p>
                        <p className="text-sm capitalize">{dispute.order_escrow_status.replace(/_/g, ' ')}</p>
                      </div>
                    )}
                    {dispute?.total_before_coins_amount != null && (
                      <div>
                        <p className="type-secondary">Escrow at Risk</p>
                        <p className="text-sm font-semibold text-warning">{formatRupiah(dispute.total_before_coins_amount)}</p>
                      </div>
                    )}
                    {dispute?.shipping_carrier && (
                      <div>
                        <p className="type-secondary">Carrier</p>
                        <p className="text-sm">{dispute.shipping_carrier}</p>
                      </div>
                    )}
                  </div>
                  {dispute?.shipping_reference && (
                    <div>
                      <p className="type-secondary">Tracking / Shipping Reference</p>
                      <p className="text-sm font-mono">{dispute.shipping_reference}</p>
                    </div>
                  )}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Evidence */}
          {dispute?.evidence && dispute.evidence.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">Evidence</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 gap-3">
                  {dispute.evidence.map((url, index) => (
                    <a
                      key={index}
                      href={url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="block"
                    >
                      <img
                        src={url}
                        alt={`Evidence ${index + 1}`}
                        className="w-full h-40 object-cover rounded-lg hover:opacity-90 transition-opacity"
                      />
                    </a>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Action Notes */}
          {isOpened && (
            <div>
              <Textarea
                label="Resolution Notes (Optional)"
                value={actionNotes}
                onChange={(e) => setActionNotes(e.target.value)}
                placeholder="Add notes explaining your decision..."
                rows={3}
                maxLength={1000}
                className="resize-none"
              />
              <p className="type-caption mt-1">{actionNotes.length}/1000 characters</p>
            </div>
          )}

          {/* Actions */}
          <ModalFooter>
            {showConfirm ? (
              <>
                <div className={`flex-1 ${confirmConfig?.variant === 'danger' ? 'text-destructive' : 'text-warning'}`}>
                  <AlertTriangle className="h-5 w-5 inline mr-2" />
                  {confirmConfig?.message}
                </div>
                <Button variant="secondary" onClick={handleCancelConfirm} disabled={isSubmitting}>
                  Cancel
                </Button>
                <Button
                  variant={confirmConfig?.variant === 'danger' ? 'danger' : 'warning'}
                  onClick={handleConfirmAction}
                  disabled={isSubmitting}
                  isLoading={isSubmitting}
                >
                  Confirm {confirmConfig?.title}
                </Button>
              </>
            ) : (
              <>
                <Button variant="secondary" onClick={onClose} disabled={isSubmitting}>
                  Close
                </Button>
                {isOpened && (
                  <>
                    <Button
                      variant="secondary"
                      onClick={() => handleActionClick('reject')}
                      disabled={isSubmitting || !canResolveDisputes}
                      className="border-warning text-warning hover:bg-warning-bg"
                      title={!canResolveDisputes ? `Requires: ${requiredCapability}` : ''}
                    >
                      <XCircle className="h-4 w-4 mr-2" />
                      Reject (Release to Seller)
                    </Button>
                    <Button
                      variant="warning"
                      onClick={() => handleActionClick('approve')}
                      disabled={isSubmitting || !canResolveDisputes}
                      title={!canResolveDisputes ? `Requires: ${requiredCapability}` : ''}
                    >
                      <CheckCircle className="h-4 w-4 mr-2" />
                      Approve (Refund Buyer)
                    </Button>
                  </>
                )}
              </>
            )}
          </ModalFooter>
        </div>
      )}
    </Modal>
  )
}
