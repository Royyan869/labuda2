import { useState, useEffect } from 'react'
import { AlertTriangle, CheckCircle, XCircle, Gavel, RefreshCw, Trash2, MessageSquare, Eye } from 'lucide-react'
import { Modal, ModalFooter } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { useAuth } from '@/hooks/useAuth'
import { useModerationCase } from '@/hooks/useModeration'
import { hasCapability } from '@/lib/permissions'
import { getModerationCaseEvidence } from '@/lib/api/moderation'
import { formatDate } from '@/lib/utils'
import type {
  ModerationCase,
  ModerationCaseEvidence,
} from '@/types'
import {
  moderationCaseStatusLabels,
  resourceTypeLabels,
  moderationCaseStatusVariants,
} from '@/types'

interface CaseDetailModalProps {
  isOpen: boolean
  onClose: () => void
  caseData: ModerationCase | null
  onAction: (caseId: string, action: 'approve' | 'reject' | 'enforce', notes?: string) => void
  isSubmitting?: boolean
  actionError?: string | null
}

// Confirmation messages for each action type
const ACTION_CONFIRMATIONS = {
  approve: {
    title: 'Approve Content',
    message: 'This will mark the reported content as approved and dismiss the report. Continue?',
    variant: 'info' as const,
  },
  reject: {
    title: 'Reject Report',
    message: 'This will dismiss the report without taking action on the content. Continue?',
    variant: 'warning' as const,
  },
  enforce: {
    title: 'Enforce Action',
    message: 'This will enforce governance actions on the reported content. This action cannot be undone. Continue?',
    variant: 'danger' as const,
  },
}

export function CaseDetailModal({ isOpen, onClose, caseData, onAction, isSubmitting = false, actionError = null }: CaseDetailModalProps) {
  const [actionNotes, setActionNotes] = useState('')
  const [selectedAction, setSelectedAction] = useState<'approve' | 'reject' | 'enforce' | null>(null)
  const [showConfirm, setShowConfirm] = useState(false)
  const [staleStatus, setStaleStatus] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [evidence, setEvidence] = useState<ModerationCaseEvidence | null>(null)
  const [evidenceLoading, setEvidenceLoading] = useState(false)
  const [evidenceError, setEvidenceError] = useState<string | null>(null)

  const { caseDetail, loading, refetch: refetchDetail } = useModerationCase(caseData?.id || null)
  const { capabilities } = useAuth()

  // Reset state when modal closes or case changes
  useEffect(() => {
    setEvidence(null)
    setEvidenceLoading(false)
    setEvidenceError(null)
    if (!isOpen) {
      setActionNotes('')
      setSelectedAction(null)
      setShowConfirm(false)
      setStaleStatus(null)
      setError(null)
    }
  }, [isOpen, caseData?.id])

  // Sync actionError from parent into local error state
  useEffect(() => {
    if (actionError) {
      setError(actionError)
    }
  }, [actionError])

  if (!caseData) return null

  const displayData = caseDetail || caseData
  const resourcePreview = caseDetail?.resource_preview
  const isChatMessage = displayData.resource_type === 'chat_message'
  const evidenceCapability = resourcePreview?.evidence_requires_capability || 'moderation.evidence.read'
  const canReadEvidence = isChatMessage && hasCapability(capabilities, evidenceCapability)
  const evidenceAvailable = resourcePreview?.evidence_available !== false

  /**
   * Verify case status hasn't changed before action (data freshness check)
   * Returns true if status is valid for action, false if stale
   */
  const verifyStatusFresh = (): boolean => {
    // If we have fresh detail data, check status
    if (caseDetail && caseDetail.status !== caseData.status) {
      setStaleStatus(`Case status has changed from "${caseData.status}" to "${caseDetail.status}". Please refresh and review the current state before taking action.`)
      return false
    }
    // Only pending cases can be acted upon
    if (caseDetail && caseDetail.status !== 'pending') {
      setStaleStatus(`This case is no longer pending (current status: "${caseDetail.status}"). Actions can only be taken on pending cases.`)
      return false
    }
    if (!caseDetail && caseData.status !== 'pending') {
      setStaleStatus(`This case is no longer pending. Actions can only be taken on pending cases.`)
      return false
    }
    return true
  }

  const handleActionClick = async (action: 'approve' | 'reject' | 'enforce') => {
    setError(null)
    setStaleStatus(null)

    // UX guard: enforce requires non-empty notes (backend is authoritative).
    if (action === 'enforce' && actionNotes.trim() === '') {
      setError('Enforce action requires notes explaining the reason for enforcement.')
      return
    }

    // Re-fetch latest data before action (data freshness)
    try {
      await refetchDetail()
    } catch {
      // Continue anyway, verifyStatusFresh will handle status mismatch
    }

    if (!verifyStatusFresh()) {
      return
    }

    setSelectedAction(action)
    setShowConfirm(true)
  }

  const handleConfirmAction = () => {
    if (!selectedAction) return

    // Final status check before submitting
    if (!verifyStatusFresh()) {
      setShowConfirm(false)
      return
    }

    onAction(caseData.id, selectedAction, actionNotes || undefined)
    setShowConfirm(false)
  }

  const handleCancelConfirm = () => {
    setShowConfirm(false)
    setSelectedAction(null)
  }

  const handleEvidenceClick = async () => {
    if (!caseData || !canReadEvidence || evidenceLoading) return

    setEvidenceError(null)
    setEvidenceLoading(true)
    try {
      const fetchedEvidence = await getModerationCaseEvidence(caseData.id)
      setEvidence(fetchedEvidence)
    } catch (err) {
      setEvidence(null)
      setEvidenceError(err instanceof Error ? err.message : 'Failed to fetch moderation evidence')
    } finally {
      setEvidenceLoading(false)
    }
  }

  const isPending = (caseDetail || caseData).status === 'pending'
  const isMarketplaceResource = displayData.resource_type === 'fixed_price_sale' || displayData.resource_type === 'auction'
  const isUserResource = displayData.resource_type === 'user'
  const confirmConfig = selectedAction ? ACTION_CONFIRMATIONS[selectedAction] : null

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Case Details" size="lg">
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
        </div>
      ) : (
        <div className="space-y-6">
          {/* Status Badge with Last Updated indicator */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Badge variant={moderationCaseStatusVariants[displayData.status]} className="text-sm px-3 py-1">
                {moderationCaseStatusLabels[displayData.status]}
              </Badge>
              <button
                onClick={() => {
                  setStaleStatus(null)
                  refetchDetail()
                }}
                className="text-gray-400 hover:text-gray-600 transition-colors"
                title="Refresh case data"
              >
                <RefreshCw className="h-4 w-4" />
              </button>
            </div>
            <span className="text-sm text-gray-500">
              Case ID: <span className="font-mono">{displayData.id}</span>
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

          {/* Error Message */}
          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 flex-shrink-0" />
              <span className="text-sm">{error}</span>
            </div>
          )}

          {/* Case Information */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Report Information</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-sm text-gray-500">Resource Type</p>
                  <p className="font-medium">
                    {resourceTypeLabels[displayData.resource_type]}
                  </p>
                </div>
                <div>
                  <p className="text-sm text-gray-500">Resource ID</p>
                  <p className="font-mono text-sm break-all">{displayData.resource_id}</p>
                </div>
                <div>
                  <p className="text-sm text-gray-500">Reported By</p>
                  <p className="font-mono text-sm break-all">{displayData.reported_by}</p>
                </div>
                <div>
                  <p className="text-sm text-gray-500">Reported Date</p>
                  <p className="text-sm">{formatDate(displayData.created_at)}</p>
                </div>
              </div>
              <div>
                <p className="text-sm text-gray-500 mb-1">Reason</p>
                <p className="text-gray-900 bg-gray-50 p-3 rounded-lg whitespace-pre-wrap break-words">
                  {displayData.reason}
                </p>
              </div>

              {displayData.reviewed_by && (
                <>
                  <div>
                    <p className="text-sm text-gray-500">Reviewed By</p>
                    <p className="font-mono text-sm break-all">{displayData.reviewed_by}</p>
                  </div>
                  {displayData.decision_note && (
                    <div>
                      <p className="text-sm text-gray-500 mb-1">Decision Note</p>
                      <p className="text-gray-900 bg-gray-50 p-3 rounded-lg whitespace-pre-wrap break-words">
                        {displayData.decision_note}
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

          {/* Resource Preview - Full Content (No Truncation) */}
          {resourcePreview && (
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">
                  {isChatMessage ? (
                    <span className="flex items-center gap-2"><MessageSquare className="h-5 w-5" /> Chat Message Preview</span>
                  ) : isMarketplaceResource ? 'Marketplace Resource Preview'
                    : isUserResource ? 'Reported User Preview'
                    : 'Reported Content Preview'}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm text-gray-500">
                      {isChatMessage ? 'Sender' : isMarketplaceResource ? 'Seller' : isUserResource ? 'User' : 'Author'}
                    </p>
                    <p className="font-medium">{resourcePreview.author_username || <em className="text-gray-400">Unknown</em>}</p>
                    <p className="font-mono text-xs text-gray-500 break-all">{resourcePreview.author_id}</p>
                  </div>
                  {!isUserResource && (
                    <div>
                      <p className="text-sm text-gray-500">
                        {isChatMessage ? 'Message Type' : 'Content Type'}
                      </p>
                      <p className="text-sm">{resourcePreview.content_type}</p>
                    </div>
                  )}
                </div>

                {isMarketplaceResource && (
                  <div className="grid grid-cols-2 gap-4">
                    {resourcePreview.title && (
                      <div>
                        <p className="text-sm text-gray-500">Title</p>
                        <p className="text-sm font-medium break-words">{resourcePreview.title}</p>
                      </div>
                    )}
                    {resourcePreview.status && (
                      <div>
                        <p className="text-sm text-gray-500">Status</p>
                        <p className="text-sm">{resourcePreview.status}</p>
                      </div>
                    )}
                  </div>
                )}

                {isUserResource && resourcePreview.status && (
                  <div>
                    <p className="text-sm text-gray-500">Account Status</p>
                    <p className="text-sm font-medium">{resourcePreview.status}</p>
                  </div>
                )}

                {/* Chat-message-specific context */}
                {isChatMessage && (
                  <div className="grid grid-cols-2 gap-4">
                    {resourcePreview.room_id && (
                      <div>
                        <p className="text-sm text-gray-500">Room ID</p>
                        <p className="font-mono text-xs break-all">{resourcePreview.room_id}</p>
                      </div>
                    )}
                    {resourcePreview.room_type && (
                      <div>
                        <p className="text-sm text-gray-500">Room Type</p>
                        <p className="text-sm">{resourcePreview.room_type}</p>
                      </div>
                    )}
                    {resourcePreview.sent_at && (
                      <div>
                        <p className="text-sm text-gray-500">Sent At</p>
                        <p className="text-sm">{formatDate(resourcePreview.sent_at)}</p>
                      </div>
                    )}
                  </div>
                )}

                {!isUserResource && (
                  <div>
                    <p className="text-sm text-gray-500 mb-1">
                      {isChatMessage ? 'Message Body' : 'Content'}
                    </p>
                    {isChatMessage ? (
                      <div className="space-y-3">
                        {resourcePreview.is_deleted && (
                          <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg flex items-center gap-2">
                            <Trash2 className="h-4 w-4" />
                            <span>This message has been deleted</span>
                          </div>
                        )}
                        <div className="bg-slate-50 border border-slate-200 text-slate-700 p-4 rounded-lg space-y-3">
                          <div className="flex items-start gap-3">
                            <MessageSquare className="h-4 w-4 mt-0.5 flex-shrink-0" />
                            <div className="space-y-1">
                              <p className="font-medium text-sm">The normal case view never includes the original body or attachment.</p>
                              <p className="text-sm text-slate-600">
                                Open the audited evidence panel only when you need to inspect the hidden chat payload.
                              </p>
                            </div>
                          </div>

                          {canReadEvidence && evidenceAvailable && (
                            <div className="flex flex-wrap items-center gap-3">
                              <Button
                                variant="secondary"
                                onClick={handleEvidenceClick}
                                disabled={evidenceLoading}
                                isLoading={evidenceLoading}
                                className="border-slate-300 text-slate-800 hover:bg-slate-100"
                              >
                                <Eye className="h-4 w-4 mr-2" />
                                Lihat bukti asli
                              </Button>
                              <span className="text-xs text-slate-500">
                                Access to this evidence is audited.
                              </span>
                            </div>
                          )}
                        </div>

                        {evidenceError && (
                          <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg flex items-center gap-2">
                            <AlertTriangle className="h-4 w-4 flex-shrink-0" />
                            <span className="text-sm">{evidenceError}</span>
                          </div>
                        )}
                      </div>
                    ) : (
                      <div className="text-gray-900 bg-gray-50 p-3 rounded-lg whitespace-pre-wrap break-words max-h-[400px] overflow-y-auto">
                        {resourcePreview.content_text || <em className="text-gray-500">No text content</em>}
                      </div>
                    )}
                  </div>
                )}
                {isUserResource && resourcePreview.is_deleted && (
                  <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg flex items-center gap-2">
                    <Trash2 className="h-4 w-4" />
                    <span>This account has been deleted</span>
                  </div>
                )}

                {evidence && (
                  <div className="border border-indigo-200 bg-indigo-50/40 rounded-lg p-4 space-y-4">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-sm font-semibold text-indigo-900">Original Hidden Evidence</p>
                        <p className="text-xs text-indigo-700 mt-1">
                          This payload was retrieved through the explicit audited evidence path.
                        </p>
                      </div>
                      <Badge variant="default">Audited</Badge>
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <p className="text-sm text-gray-500">Message ID</p>
                        <p className="font-mono text-xs break-all">{evidence.message_id}</p>
                      </div>
                      <div>
                        <p className="text-sm text-gray-500">Room ID</p>
                        <p className="font-mono text-xs break-all">{evidence.room_id}</p>
                      </div>
                      <div>
                        <p className="text-sm text-gray-500">Sender</p>
                        <p className="font-mono text-xs break-all">{evidence.sender_id}</p>
                      </div>
                      <div>
                        <p className="text-sm text-gray-500">Created At</p>
                        <p className="text-sm">{formatDate(evidence.created_at)}</p>
                      </div>
                    </div>

                    <div>
                      <p className="text-sm text-gray-500 mb-1">Original Body</p>
                      <div className="bg-white border border-indigo-100 rounded-lg p-3 whitespace-pre-wrap break-words">
                        {evidence.original_body || <em className="text-gray-500">No body available</em>}
                      </div>
                    </div>

                    <div>
                      <p className="text-sm text-gray-500 mb-1">Original Attachment</p>
                      <div className="bg-white border border-indigo-100 rounded-lg p-3">
                        {evidence.original_attachment ? (
                          <pre className="text-xs whitespace-pre-wrap break-words overflow-x-auto">
                            {JSON.stringify(evidence.original_attachment, null, 2)}
                          </pre>
                        ) : (
                          <em className="text-gray-500">No attachment available</em>
                        )}
                      </div>
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>
          )}

          {/* Action Notes */}
          {isPending && (
            <div>
              <label className="text-sm font-medium text-gray-700 mb-2 block">
                Decision Notes <span className="font-normal text-gray-500">(optional — required for Enforce)</span>
              </label>
              <textarea
                value={actionNotes}
                onChange={(e) => setActionNotes(e.target.value)}
                placeholder="Add notes explaining your decision..."
                rows={3}
                maxLength={1000}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary resize-none"
              />
              <p className="text-xs text-gray-500 mt-1">{actionNotes.length}/1000 characters</p>
            </div>
          )}

          {/* Actions */}
          <ModalFooter>
            {showConfirm ? (
              <>
                <div className={`flex-1 ${confirmConfig?.variant === 'danger' ? 'text-red-600' : 'text-amber-600'}`}>
                  <AlertTriangle className="h-5 w-5 inline mr-2" />
                  {confirmConfig?.message}
                </div>
                <Button variant="secondary" onClick={handleCancelConfirm} disabled={isSubmitting}>
                  Cancel
                </Button>
                <Button
                  variant={confirmConfig?.variant === 'danger' ? 'danger' : confirmConfig?.variant === 'warning' ? 'warning' : 'primary'}
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
                {isPending && (
                  <>
                    <Button
                      variant="secondary"
                      onClick={() => handleActionClick('reject')}
                      disabled={isSubmitting}
                      className="border-orange-200 text-orange-700 hover:bg-orange-50"
                    >
                      <XCircle className="h-4 w-4 mr-2" />
                      Reject Report
                    </Button>
                    <Button
                      variant="secondary"
                      onClick={() => handleActionClick('approve')}
                      disabled={isSubmitting}
                      className="border-green-200 text-green-700 hover:bg-green-50"
                    >
                      <CheckCircle className="h-4 w-4 mr-2" />
                      Approve Content
                    </Button>
                    <Button
                      variant="danger"
                      onClick={() => handleActionClick('enforce')}
                      disabled={isSubmitting}
                    >
                      <Gavel className="h-4 w-4 mr-2" />
                      Enforce Action
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
