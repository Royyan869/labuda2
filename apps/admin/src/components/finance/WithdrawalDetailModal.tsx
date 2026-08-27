import { useState, useEffect, useRef } from 'react'
import {
  User,
  Building2,
  DollarSign,
  Calendar,
  AlertTriangle,
  RefreshCw,
  CheckCircle,
  XCircle,
  Ban,
  CreditCard,
} from 'lucide-react'
import { Modal, ModalFooter } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { useWithdrawalDetail, useWithdrawalActions } from '@/hooks/useWithdrawals'
import { formatDate, formatRupiah } from '@/lib/utils'
import { hasCapability } from '@/lib/permissions'
import { useAuth } from '@/hooks/useAuth'
import type {
  WithdrawalListItem,
  WithdrawalStatus,
} from '@/types'
import {
  withdrawalStatusLabels,
  withdrawalStatusVariants,
} from '@/types'

interface WithdrawalDetailModalProps {
  isOpen: boolean
  onClose: () => void
  withdrawalData: WithdrawalListItem | null
  onSuccess?: () => void
}

type ActionState = 'idle' | 'confirm-approve' | 'confirm-reject' | 'confirm-mark-processed' | 'submitting'

const FINAL_STATUSES: WithdrawalStatus[] = ['SETTLED', 'COMPLETED', 'FAILED', 'FAILED_FINAL', 'FAILED_RETRYABLE', 'PILOT_BLOCKED']

// Action confirmation configurations
const ACTION_CONFIRMATIONS = {
  'confirm-approve': {
    title: 'Approve Withdrawal',
    icon: CheckCircle,
    iconColor: 'text-green-600',
    bgColor: 'bg-green-50',
    borderColor: 'border-green-200',
    message: (
      <>
        <p className="font-semibold text-lg mb-2">You are about to approve this withdrawal request.</p>
        <p className="text-sm">The withdrawal will be submitted to the payment gateway for processing. This action cannot be undone.</p>
      </>
    ),
  },
  'confirm-reject': {
    title: 'Reject Withdrawal',
    icon: XCircle,
    iconColor: 'text-red-600',
    bgColor: 'bg-red-50',
    borderColor: 'border-red-200',
    message: (
      <>
        <p className="font-semibold text-lg mb-2">You are about to reject this withdrawal request.</p>
        <p className="text-sm">The funds will be returned to the seller&apos;s available balance. Please provide a reason.</p>
      </>
    ),
  },
  'confirm-mark-processed': {
    title: 'Mark Withdrawal as Paid',
    icon: CreditCard,
    iconColor: 'text-blue-600',
    bgColor: 'bg-blue-50',
    borderColor: 'border-blue-200',
    message: (
      <>
        <p className="font-semibold text-lg mb-2">You are about to mark this withdrawal as manually paid.</p>
        <p className="text-sm font-semibold text-red-600 mb-2">⚠️ CRITICAL: Use this ONLY if you have manually sent the funds outside the payment gateway.</p>
        <p className="text-xs text-gray-600">This action cannot be undone. The seller will receive the funds and the withdrawal will be marked as completed.</p>
      </>
    ),
  },
} as const

export function WithdrawalDetailModal({ isOpen, onClose, withdrawalData, onSuccess }: WithdrawalDetailModalProps) {
  const [actionState, setActionState] = useState<ActionState>('idle')
  const [error, setError] = useState<string | null>(null)
  const [rejectionReason, setRejectionReason] = useState('')
  const [isDataStale, setIsDataStale] = useState(false)
  const lastKnownStatus = useRef<WithdrawalStatus | null>(null)

  const { withdrawal, loading, refetch } = useWithdrawalDetail(withdrawalData?.id || null)
  const { approve, reject, markProcessed, loading: actionLoading } = useWithdrawalActions(withdrawalData?.id || null)
  const { capabilities } = useAuth()

  const canReviewWithdrawals = hasCapability(capabilities, 'finance.withdraw.review')
  const requiredCapability = 'finance.withdraw.review'

  // Track status changes for staleness detection
  useEffect(() => {
    if (withdrawal) {
      if (lastKnownStatus.current && lastKnownStatus.current !== withdrawal.status) {
        const timer = window.setTimeout(() => setIsDataStale(true), 0)
        lastKnownStatus.current = withdrawal.status
        return () => window.clearTimeout(timer)
      }
      lastKnownStatus.current = withdrawal.status
    } else if (withdrawalData) {
      lastKnownStatus.current = withdrawalData.status
    }
  }, [withdrawal, withdrawalData])

  // Reset state when modal closes
  useEffect(() => {
    if (!isOpen) {
      const timer = window.setTimeout(() => {
        setActionState('idle')
        setError(null)
        setRejectionReason('')
        setIsDataStale(false)
      }, 0)
      return () => window.clearTimeout(timer)
    }
  }, [isOpen])

  // Refresh data before showing action confirmation
  const prepareAction = async (action: 'approve' | 'reject' | 'mark-processed') => {
    setError(null)
    setIsDataStale(false)

    // Re-fetch to ensure we have the latest data
    await refetch()

    // Check if action is still valid after refresh
    const currentStatus = withdrawal?.status || withdrawalData?.status
    if (FINAL_STATUSES.includes(currentStatus as WithdrawalStatus)) {
      setError(`Cannot modify withdrawal with status: ${withdrawalStatusLabels[currentStatus as WithdrawalStatus]}`)
      return
    }

    // CRITICAL: For mark-processed, check external_reference_id
    if (action === 'mark-processed') {
      const externalRef = withdrawal?.external_reference_id || withdrawalData?.external_reference_id
      if (externalRef) {
        setError(`Cannot manually complete - payout already submitted to payment gateway (Ref: ${externalRef}). Must wait for webhook settlement.`)
        return
      }
    }

    if (action === 'approve') {
      setActionState('confirm-approve')
    } else if (action === 'reject') {
      setActionState('confirm-reject')
    } else {
      setActionState('confirm-mark-processed')
    }
  }

  const handleConfirmApprove = async () => {
    setActionState('submitting')
    setError(null)

    const result = await approve()
    if (result) {
      onSuccess?.()
      onClose()
    } else {
      setActionState('idle')
    }
  }

  const handleConfirmReject = async () => {
    if (!rejectionReason.trim()) {
      setError('Please provide a reason for rejection')
      return
    }

    setActionState('submitting')
    setError(null)

    const result = await reject(rejectionReason.trim())
    if (result) {
      onSuccess?.()
      onClose()
    } else {
      setActionState('idle')
    }
  }

  const handleConfirmMarkProcessed = async () => {
    setActionState('submitting')
    setError(null)

    const result = await markProcessed()
    if (result) {
      onSuccess?.()
      onClose()
    } else {
      setActionState('idle')
    }
  }

  const handleCancelConfirm = () => {
    setActionState('idle')
    setRejectionReason('')
    setError(null)
  }

  if (!withdrawalData) return null

  const displayData = withdrawal || withdrawalData
  const canModify = !FINAL_STATUSES.includes(displayData.status)
  const confirmConfig = actionState !== 'idle' && actionState !== 'submitting' ? ACTION_CONFIRMATIONS[actionState] : null
  const isSubmitting = actionLoading || actionState === 'submitting'
  const sellerUsername = displayData.seller_username?.trim()
  const sellerFarmName = displayData.seller_farm_name?.trim()
  const hasSellerUsername = !!sellerUsername

  // CRITICAL: Check if external_reference_id exists (prevents manual completion)
  const hasExternalRef = !!(displayData.external_reference_id)
  const canMarkProcessed = displayData.status === 'PROCESSING' && !hasExternalRef

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Withdrawal Details" size="xl">
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent" />
        </div>
      ) : (
        <div className="space-y-6">
          {/* Status bar with action state */}
          {actionState !== 'idle' && confirmConfig ? (
            <div className={`p-4 rounded-lg border ${confirmConfig.bgColor} ${confirmConfig.borderColor}`}>
              <div className="flex items-start gap-3">
                <confirmConfig.icon className={`h-6 w-6 flex-shrink-0 mt-0.5 ${confirmConfig.iconColor}`} />
                <div className="flex-1">
                  <h3 className={`font-semibold ${confirmConfig.iconColor}`}>{confirmConfig.title}</h3>
                  <div className="mt-2 text-gray-700">
                    {confirmConfig.message}
                  </div>

                  {/* Rejection reason input */}
                  {actionState === 'confirm-reject' && (
                    <div className="mt-4">
                      <label htmlFor="reject-reason" className="block text-sm font-medium text-gray-700 mb-1">
                        Reason for Rejection <span className="text-red-600">*</span>
                      </label>
                      <textarea
                        id="reject-reason"
                        value={rejectionReason}
                        onChange={(e) => setRejectionReason(e.target.value)}
                        placeholder="e.g., Invalid bank account details, insufficient verification, etc."
                        rows={3}
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-red-500 focus:border-red-500"
                        autoFocus
                      />
                    </div>
                  )}

                  {/* Double confirmation for mark-processed */}
                  {actionState === 'confirm-mark-processed' && (
                    <div className="mt-4 p-3 bg-red-50 border border-red-200 rounded-lg">
                      <label className="flex items-center gap-2 text-sm font-medium text-red-700">
                        <input
                          type="checkbox"
                          id="confirm-manual-payment"
                          className="rounded border-red-300 text-red-600 focus:ring-red-500"
                          onChange={(e) => {
                            const button = document.getElementById('confirm-mark-processed-btn') as HTMLButtonElement
                            if (button) button.disabled = !e.target.checked
                          }}
                        />
                        <span>I have manually sent the funds to the seller&apos;s bank account</span>
                      </label>
                    </div>
                  )}

                  {/* Action confirmation buttons */}
                  <div className="mt-4 flex items-center gap-3">
                    <Button
                      variant="secondary"
                      onClick={handleCancelConfirm}
                      disabled={isSubmitting}
                    >
                      Cancel
                    </Button>
                    {actionState === 'confirm-mark-processed' ? (
                      <Button
                        variant="success"
                        onClick={handleConfirmMarkProcessed}
                        isLoading={isSubmitting}
                        disabled={isSubmitting}
                        id="confirm-mark-processed-btn"
                      >
                        Confirm Payment Sent
                      </Button>
                    ) : (
                      <Button
                        variant={actionState === 'confirm-approve' ? 'success' : 'danger'}
                        onClick={actionState === 'confirm-approve' ? handleConfirmApprove : handleConfirmReject}
                        isLoading={isSubmitting}
                        disabled={isSubmitting || (actionState === 'confirm-reject' && !rejectionReason.trim())}
                      >
                        {actionState === 'confirm-approve' ? 'Confirm Approval' : 'Confirm Rejection'}
                      </Button>
                    )}
                  </div>
                </div>
              </div>
            </div>
          ) : (
            <>
              {/* Stale data warning */}
              {isDataStale && (
                <div className="bg-amber-50 border border-amber-200 text-amber-800 p-3 rounded-lg flex items-center gap-2">
                  <AlertTriangle className="h-4 w-4 flex-shrink-0" />
                  <span className="text-sm">This withdrawal&apos;s status has changed. Refresh to see the latest data.</span>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => {
                      refetch()
                      setIsDataStale(false)
                    }}
                    className="ml-auto"
                  >
                    <RefreshCw className="h-3 w-3 mr-1" />
                    Refresh
                  </Button>
                </div>
              )}

              {/* Error Message */}
              {error && (
                <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg flex items-center gap-2">
                  <AlertTriangle className="h-4 w-4 flex-shrink-0" />
                  <span className="text-sm">{error}</span>
                </div>
              )}

              {/* Header with Status Badges */}
              <div className="flex items-center justify-between flex-wrap gap-3">
                <div className="flex items-center gap-3">
                  <Badge variant={withdrawalStatusVariants[displayData.status] || 'info'}>
                    {withdrawalStatusLabels[displayData.status] || displayData.status}
                  </Badge>
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-sm text-gray-500">
                    Withdrawal ID: <span className="font-mono">{displayData.id}</span>
                  </span>
                  <button
                    onClick={() => {
                      setError(null)
                      setIsDataStale(false)
                      refetch()
                    }}
                    className="text-gray-400 hover:text-gray-600 transition-colors"
                    title="Refresh withdrawal data"
                  >
                    <RefreshCw className="h-4 w-4" />
                  </button>
                </div>
              </div>

              {/* Amount Display - Prominent */}
              <Card className="border-2 border-primary/20">
                <CardContent className="pt-6">
                  <div className="space-y-4">
                    <div className="flex items-center justify-center gap-4">
                      <div className="p-4 rounded-full bg-primary/10">
                        <DollarSign className="h-8 w-8 text-primary" />
                      </div>
                      <div>
                        <p className="text-sm text-gray-600">Jumlah diterima seller</p>
                        <p className="text-3xl font-bold text-primary">{formatRupiah(displayData.amount)}</p>
                      </div>
                    </div>
                    <div className="grid gap-3 rounded-lg bg-gray-50 p-4 md:grid-cols-3">
                      <div>
                        <p className="text-xs uppercase tracking-wide text-gray-500">Biaya penarikan</p>
                        <p className="mt-1 text-lg font-semibold text-gray-900">{formatRupiah(displayData.fee_amount)}</p>
                      </div>
                      <div>
                        <p className="text-xs uppercase tracking-wide text-gray-500">Total dipotong saldo</p>
                        <p className="mt-1 text-lg font-semibold text-gray-900">{formatRupiah(displayData.total_debit_amount)}</p>
                      </div>
                      <div>
                        <p className="text-xs uppercase tracking-wide text-gray-500">Gateway payout</p>
                        <p className="mt-1 text-lg font-semibold text-gray-900">{formatRupiah(displayData.amount)}</p>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>

              {/* Seller Information */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg flex items-center gap-2">
                    <User className="h-5 w-5" />
                    Seller Information
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center gap-4">
                    {displayData.seller_avatar ? (
                      <img
                        src={displayData.seller_avatar}
                        alt=""
                        className="w-12 h-12 rounded-full object-cover"
                      />
                    ) : (
                      <div className="w-12 h-12 rounded-full bg-gray-200 flex items-center justify-center">
                        <User className="h-6 w-6 text-gray-500" />
                      </div>
                    )}
                    <div className="flex-1">
                      {hasSellerUsername ? (
                        <>
                          <p className="font-medium text-lg">@{sellerUsername}</p>
                          {sellerFarmName && <p className="text-sm text-gray-600">{sellerFarmName}</p>}
                          <p className="font-mono text-sm text-gray-500">{displayData.seller_id}</p>
                        </>
                      ) : (
                        <p className="font-medium text-lg text-gray-500">{displayData.seller_id}</p>
                      )}
                      {withdrawal?.seller_email && (
                        <p className="text-sm text-gray-600">{withdrawal.seller_email}</p>
                      )}
                    </div>
                  </div>
                </CardContent>
              </Card>

              {/* Bank Information - Critical for review */}
              <Card className="border-amber-200">
                <CardHeader>
                  <CardTitle className="text-lg flex items-center gap-2 text-amber-700">
                    <Building2 className="h-5 w-5" />
                    Bank Account Information
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-3 bg-amber-50 p-4 rounded-lg">
                    <div className="flex justify-between">
                      <span className="text-sm font-medium text-gray-700">Bank Name</span>
                      <span className="font-semibold">{displayData.bank_name_snapshot}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-sm font-medium text-gray-700">Account Holder</span>
                      <span className="font-semibold">{displayData.account_holder_snapshot}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-sm font-medium text-gray-700">Account Number</span>
                      <span className="font-mono font-semibold">{displayData.account_number_snapshot}</span>
                    </div>

                    {/* External Reference Warning */}
                    {displayData.external_reference_id && (
                      <div className="mt-4 p-3 bg-blue-50 border border-blue-200 rounded-lg">
                        <div className="flex items-start gap-2">
                          <CreditCard className="h-4 w-4 text-blue-600 mt-0.5 flex-shrink-0" />
                          <div className="flex-1">
                            <p className="text-sm font-medium text-blue-700">Payment Gateway Reference</p>
                            <p className="text-xs text-blue-600 mt-1">
                              This payout has been submitted to the payment gateway (Ref: {displayData.external_reference_id}).
                              Manual completion is disabled - must wait for webhook settlement.
                            </p>
                          </div>
                        </div>
                      </div>
                    )}
                  </div>
                  <p className="text-xs text-amber-700 mt-3 flex items-center gap-1">
                    <AlertTriangle className="h-3 w-3" />
                    Verify these details match the seller&apos;s verified bank account before approving.
                  </p>
                </CardContent>
              </Card>

              {/* Timestamps */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg flex items-center gap-2">
                    <Calendar className="h-5 w-5" />
                    Timeline
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-sm text-gray-600">Created At</p>
                      <p className="text-sm font-medium">{formatDate(displayData.created_at)}</p>
                    </div>
                    <div>
                      <p className="text-sm text-gray-600">Last Updated</p>
                      <p className="text-sm font-medium">{withdrawal?.updated_at ? formatDate(withdrawal.updated_at) : formatDate(displayData.created_at)}</p>
                    </div>
                    {withdrawal?.submitted_at && (
                      <div>
                        <p className="text-sm text-gray-600">Submitted to Gateway</p>
                        <p className="text-sm font-medium">{formatDate(withdrawal.submitted_at)}</p>
                      </div>
                    )}
                    {withdrawal?.settled_at && (
                      <div>
                        <p className="text-sm text-gray-600">Settled</p>
                        <p className="text-sm font-medium">{formatDate(withdrawal.settled_at)}</p>
                      </div>
                    )}
                  </div>

                  {/* Gateway Reference */}
                  {withdrawal?.gateway_reference_id && (
                    <div className="mt-4 pt-4 border-t border-gray-100">
                      <p className="text-sm text-gray-600">Gateway Reference</p>
                      <p className="text-sm font-mono font-medium">{withdrawal.gateway_reference_id}</p>
                    </div>
                  )}

                  {/* Retry count */}
                  {withdrawal?.retry_count !== undefined && withdrawal.retry_count > 0 && (
                    <div className="mt-4 pt-4 border-t border-gray-100">
                      <p className="text-sm text-gray-600">Retry Attempts</p>
                      <p className="text-sm font-medium">{withdrawal.retry_count}</p>
                    </div>
                  )}
                </CardContent>
              </Card>

              {/* Failure Reason (if applicable) */}
              {displayData.failure_reason && (
                <Card className="border-red-200">
                  <CardHeader>
                    <CardTitle className="text-lg text-red-600 flex items-center gap-2">
                      <AlertTriangle className="h-5 w-5" />
                      Failure Information
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-2">
                      <div>
                        <p className="text-sm text-gray-600">Reason</p>
                        <p className="text-sm bg-red-50 p-3 rounded border border-red-200">
                          {displayData.failure_reason}
                        </p>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              )}
            </>
          )}

          {/* Footer with action buttons (only in idle state) */}
          {actionState === 'idle' && (
            <ModalFooter className="flex items-center justify-between">
              <div className="text-sm text-gray-500">
                {canModify ? (
                  <span className="flex items-center gap-1">
                    <AlertTriangle className="h-4 w-4 text-amber-500" />
                    Review all information before taking action
                  </span>
                ) : (
                  <span className="flex items-center gap-1">
                    <Ban className="h-4 w-4 text-gray-400" />
                    This withdrawal cannot be modified
                  </span>
                )}
              </div>
              <div className="flex items-center gap-3">
                <Button variant="secondary" onClick={onClose}>
                  Close
                </Button>
                {canModify && (
                  <>
                    {/* REQUESTED status: Can approve or reject */}
                    {displayData.status === 'REQUESTED' && (
                      <>
                        <Button
                          variant="danger"
                          onClick={() => prepareAction('reject')}
                          disabled={loading || !canReviewWithdrawals}
                          title={!canReviewWithdrawals ? `Requires: ${requiredCapability}` : ''}
                        >
                          <XCircle className="h-4 w-4 mr-2" />
                          Reject
                        </Button>
                        <Button
                          variant="success"
                          onClick={() => prepareAction('approve')}
                          disabled={loading || !canReviewWithdrawals}
                          title={!canReviewWithdrawals ? `Requires: ${requiredCapability}` : ''}
                        >
                          <CheckCircle className="h-4 w-4 mr-2" />
                          Approve
                        </Button>
                      </>
                    )}

                    {/* PROCESSING status: Can mark as processed (only if no external ref) */}
                    {displayData.status === 'PROCESSING' && canMarkProcessed && (
                      <Button
                        variant="success"
                        onClick={() => prepareAction('mark-processed')}
                        disabled={loading || !canReviewWithdrawals}
                        title={!canReviewWithdrawals ? `Requires: ${requiredCapability}` : ''}
                      >
                        <CreditCard className="h-4 w-4 mr-2" />
                        Mark as Paid
                      </Button>
                    )}
                  </>
                )}
              </div>
            </ModalFooter>
          )}
        </div>
      )}
    </Modal>
  )
}
