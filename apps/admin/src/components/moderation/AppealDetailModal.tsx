import { useState, useEffect } from 'react'
import { CheckCircle, XCircle, AlertTriangle, RefreshCw } from 'lucide-react'
import { Modal, ModalFooter } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { useAppeal, useAppealReview } from '@/hooks/useAppeals'
import { useAuth } from '@/hooks/useAuth'
import { hasCapability } from '@/lib/permissions'
import { formatDate } from '@/lib/utils'
import type {
  Appeal,
} from '@/types'
import {
  appealStatusLabels,
  moderationCaseStatusLabels,
  resourceTypeLabels,
  appealStatusVariants,
  moderationCaseStatusVariants,
} from '@/types'

interface AppealDetailModalProps {
  isOpen: boolean
  onClose: () => void
  appeal: Appeal | null
  onReviewComplete: () => void
}

// Confirmation messages for each appeal decision
const APPEAL_CONFIRMATIONS = {
  approve: {
    title: 'Approve Appeal',
    message: 'This will approve the appeal and reverse the original moderation decision. Continue?',
    variant: 'info' as const,
  },
  reject: {
    title: 'Reject Appeal',
    message: 'This will reject the appeal and uphold the original moderation decision. Continue?',
    variant: 'warning' as const,
  },
}

export function AppealDetailModal({ isOpen, onClose, appeal, onReviewComplete }: AppealDetailModalProps) {
  const [adminResponse, setAdminResponse] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [selectedDecision, setSelectedDecision] = useState<'approve' | 'reject' | null>(null)
  const [showConfirm, setShowConfirm] = useState(false)
  const [staleStatus, setStaleStatus] = useState<string | null>(null)

  const { appealDetail, loading, refetch: refetchDetail } = useAppeal(appeal?.id || null)
  const { reviewAppeal, loading: submitting } = useAppealReview()
  const { capabilities } = useAuth()
  // PASS_13B: viewing this modal only requires moderation.appeal.read (see
  // App.tsx route guard); submitting a decision requires the higher-trust
  // moderation.appeal.review capability. A read-only appeal admin can open
  // and inspect this modal but must not see/use the decision controls.
  const canReview = hasCapability(capabilities, 'moderation.appeal.review')

  // Reset state when modal closes or appeal changes
  useEffect(() => {
    if (!isOpen) {
      const timer = window.setTimeout(() => {
        setAdminResponse('')
        setError(null)
        setSelectedDecision(null)
        setShowConfirm(false)
        setStaleStatus(null)
      }, 0)
      return () => window.clearTimeout(timer)
    }
  }, [isOpen])

  if (!appeal) return null

  const displayData = appealDetail || appeal

  /**
   * Verify appeal status hasn't changed before action (data freshness check)
   * Returns true if status is valid for action, false if stale
   */
  const verifyStatusFresh = (): boolean => {
    // If we have fresh detail data, check status
    if (appealDetail && appealDetail.status !== appeal.status) {
      setStaleStatus(`Appeal status has changed from "${appeal.status}" to "${appealDetail.status}". Please refresh and review the current state before taking action.`)
      return false
    }
    // Only pending appeals can be reviewed
    if (appealDetail && appealDetail.status !== 'pending') {
      setStaleStatus(`This appeal is no longer pending (current status: "${appealDetail.status}"). Actions can only be taken on pending appeals.`)
      return false
    }
    if (!appealDetail && appeal.status !== 'pending') {
      setStaleStatus(`This appeal is no longer pending. Actions can only be taken on pending appeals.`)
      return false
    }
    return true
  }

  const handleReviewClick = async (decision: 'approve' | 'reject') => {
    setError(null)
    setStaleStatus(null)

    // Re-fetch latest data before action (data freshness)
    try {
      await refetchDetail()
    } catch {
      // Continue anyway, verifyStatusFresh will handle status mismatch
    }

    if (!verifyStatusFresh()) {
      return
    }

    setSelectedDecision(decision)
    setShowConfirm(true)
  }

  const handleConfirmReview = async () => {
    if (!selectedDecision) return

    // Final status check before submitting
    if (!verifyStatusFresh()) {
      setShowConfirm(false)
      setSelectedDecision(null)
      return
    }

    setError(null)
    try {
      await reviewAppeal(appeal.id, {
        decision: selectedDecision,
        admin_response: adminResponse || undefined,
      })
      onReviewComplete()
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to submit review'
      setError(message)
      setShowConfirm(false)
      setSelectedDecision(null)
    }
  }

  const handleCancelConfirm = () => {
    setShowConfirm(false)
    setSelectedDecision(null)
  }

  const isPending = (appealDetail || appeal).status === 'pending'
  const confirmConfig = selectedDecision ? APPEAL_CONFIRMATIONS[selectedDecision] : null

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Appeal Details" size="lg">
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
        </div>
      ) : (
        <div className="space-y-6">
          {/* Status Badge with Refresh */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Badge variant={appealStatusVariants[displayData.status]} className="text-sm px-3 py-1">
                {appealStatusLabels[displayData.status]}
              </Badge>
              <button
                onClick={() => {
                  setStaleStatus(null)
                  refetchDetail()
                }}
                className="text-gray-400 hover:text-gray-600 transition-colors"
                title="Refresh appeal data"
              >
                <RefreshCw className="h-4 w-4" />
              </button>
            </div>
            <span className="text-sm text-gray-500">
              Appeal ID: <span className="font-mono">{displayData.id}</span>
            </span>
          </div>

          {/* Stale Status Warning */}
          {staleStatus && (
            <div className="bg-amber-50 border border-amber-200 text-amber-800 p-4 rounded-lg flex items-start gap-2">
              <AlertTriangle className="h-5 w-5 flex-shrink-0 mt-0.5" />
              <div className="flex-1">
                <p className="font-medium text-sm">Status Changed</p>
                <p className="text-sm mt-1">{staleStatus}</p>
              </div>
            </div>
          )}

          {/* Appeal Information */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Appeal Information</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-sm text-gray-500">Report ID</p>
                  <p className="font-mono text-sm break-all">{appeal.report_id}</p>
                </div>
                <div>
                  <p className="text-sm text-gray-500">Submitted Date</p>
                  <p className="text-sm">{formatDate(appeal.created_at)}</p>
                </div>
              </div>
              <div>
                <p className="text-sm text-gray-500 mb-1">Appeal Message</p>
                <p className="text-gray-900 bg-gray-50 p-3 rounded-lg whitespace-pre-wrap break-words">
                  {appeal.message}
                </p>
              </div>

              {displayData.reviewed_by && (
                <>
                  <div>
                    <p className="text-sm text-gray-500">Reviewed By</p>
                    <p className="font-mono text-sm break-all">{displayData.reviewed_by}</p>
                  </div>
                  {displayData.admin_response && (
                    <div>
                      <p className="text-sm text-gray-500 mb-1">Admin Response</p>
                      <p className="text-gray-900 bg-gray-50 p-3 rounded-lg whitespace-pre-wrap break-words">
                        {displayData.admin_response}
                      </p>
                    </div>
                  )}
                  {displayData.reviewed_at && (
                    <div>
                      <p className="text-sm text-gray-500">Reviewed At</p>
                      <p className="text-sm">{formatDate(displayData.reviewed_at)}</p>
                    </div>
                  )}
                </>
              )}
            </CardContent>
          </Card>

          {/* Original Case Context */}
          {appealDetail?.original_case && (
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">Original Case Context</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm text-gray-500">Case ID</p>
                    <p className="font-mono text-sm break-all">{appealDetail.original_case.id}</p>
                  </div>
                  <div>
                    <p className="text-sm text-gray-500">Resource Type</p>
                    <p className="text-sm">{resourceTypeLabels[appealDetail.original_case.resource_type]}</p>
                  </div>
                </div>
                <div>
                  <p className="text-sm text-gray-500">Original Reason</p>
                  <p className="text-gray-900 bg-gray-50 p-3 rounded-lg whitespace-pre-wrap break-words">
                    {appealDetail.original_case.reason}
                  </p>
                </div>
                <div className="flex items-center gap-4">
                  <div>
                    <p className="text-sm text-gray-500">Case Status</p>
                    <Badge variant={moderationCaseStatusVariants[appealDetail.original_case.status]}>
                      {moderationCaseStatusLabels[appealDetail.original_case.status]}
                    </Badge>
                  </div>
                  <div>
                    <p className="text-sm text-gray-500">Decision</p>
                    <p className="text-sm font-medium capitalize">
                      {appealDetail.original_case.decision_status}
                    </p>
                  </div>
                  <div>
                    <p className="text-sm text-gray-500">Created</p>
                    <p className="text-sm">{formatDate(appealDetail.original_case.created_at)}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Admin Response */}
          {isPending && canReview && (
            <div>
              <label className="text-sm font-medium text-gray-700 mb-2 block">
                Admin Response (Optional)
              </label>
              <textarea
                value={adminResponse}
                onChange={(e) => setAdminResponse(e.target.value)}
                placeholder="Provide a response to the user explaining your decision..."
                rows={3}
                maxLength={2000}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary resize-none"
              />
              <p className="text-xs text-gray-500 mt-1">{adminResponse.length}/2000 characters</p>
            </div>
          )}

          {/* Read-only notice: pending appeal, but this admin cannot submit a decision */}
          {isPending && !canReview && (
            <div className="bg-gray-50 border border-gray-200 text-gray-600 p-3 rounded-lg flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 flex-shrink-0" />
              <span className="text-sm">
                You can view this appeal but do not have permission to submit a decision
                (requires moderation.appeal.review).
              </span>
            </div>
          )}

          {/* Error Message */}
          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 flex-shrink-0" />
              <span className="text-sm">{error}</span>
            </div>
          )}

          {/* Actions */}
          <ModalFooter>
            {showConfirm ? (
              <>
                <div className={`flex-1 ${confirmConfig?.variant === 'warning' ? 'text-amber-600' : 'text-blue-600'}`}>
                  <AlertTriangle className="h-5 w-5 inline mr-2" />
                  {confirmConfig?.message}
                </div>
                <Button variant="secondary" onClick={handleCancelConfirm} disabled={submitting}>
                  Cancel
                </Button>
                <Button
                  variant={confirmConfig?.variant === 'warning' ? 'warning' : 'primary'}
                  onClick={handleConfirmReview}
                  disabled={submitting}
                  isLoading={submitting}
                >
                  Confirm {confirmConfig?.title}
                </Button>
              </>
            ) : (
              <>
                <Button variant="secondary" onClick={onClose} disabled={submitting}>
                  Close
                </Button>
                {isPending && canReview && (
                  <>
                    <Button
                      variant="secondary"
                      onClick={() => handleReviewClick('reject')}
                      disabled={submitting}
                      className="border-orange-200 text-orange-700 hover:bg-orange-50"
                    >
                      <XCircle className="h-4 w-4 mr-2" />
                      Reject Appeal
                    </Button>
                    <Button
                      variant="secondary"
                      onClick={() => handleReviewClick('approve')}
                      disabled={submitting}
                      className="border-green-200 text-green-700 hover:bg-green-50"
                    >
                      <CheckCircle className="h-4 w-4 mr-2" />
                      Approve Appeal
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
