import { useState } from 'react'
import {
  AlertTriangle,
  CheckCircle,
  XCircle,
  DollarSign,
  Lock,
  AlertCircle,
} from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import type { DisputeDetail, DisputeDecision } from '@/types'
import { hasCapability } from '@/lib/permissions'
import { useAuth } from '@/hooks/useAuth'

interface DecisionPanelProps {
  dispute: DisputeDetail
  onSubmit: (decision: DisputeDecision, notes: string) => void
  submitting?: boolean
}

type DecisionStep = 'select' | 'confirm'

const MIN_NOTES_LENGTH = 10
const MAX_NOTES_LENGTH = 1000

export function DecisionPanel({ dispute, onSubmit, submitting }: DecisionPanelProps) {
  const [step, setStep] = useState<DecisionStep>('select')
  const [selectedDecision, setSelectedDecision] = useState<DisputeDecision | null>(null)
  const [notes, setNotes] = useState('')
  const [notesError, setNotesError] = useState<string | null>(null)

  // Only opened disputes can be acted upon
  const isLocked = dispute.status !== 'under_review'

  const { capabilities } = useAuth()
  const canResolveDisputes = hasCapability(capabilities, 'finance.dispute.resolve')
  const requiredCapability = 'finance.dispute.resolve'

  const validateNotes = (value: string): boolean => {
    if (value.trim().length < MIN_NOTES_LENGTH) {
      setNotesError(`Notes must be at least ${MIN_NOTES_LENGTH} characters`)
      return false
    }
    setNotesError(null)
    return true
  }

  const handleNotesChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const value = e.target.value
    setNotes(value)
    if (notesError) validateNotes(value)
  }

  const handleDecisionClick = (decision: DisputeDecision) => {
    if (!validateNotes(notes)) return
    setSelectedDecision(decision)
    setStep('confirm')
  }

  const handleConfirmSubmit = () => {
    if (!selectedDecision) return
    if (!validateNotes(notes)) {
      setStep('select')
      return
    }
    onSubmit(selectedDecision, notes.trim())
  }

  const handleBack = () => {
    setStep('select')
    setSelectedDecision(null)
  }

  // Resolved / locked state
  if (isLocked) {
    const statusConfig: Record<string, { label: string; variant: 'success' | 'warning' | 'info' }> = {
      resolved_refund:   { label: 'Resolved — Full Refund to Buyer',                           variant: 'success' },
      resolved_release:  { label: 'Resolved — Funds Released to Seller',                       variant: 'info'    },
      resolved_partial:  { label: 'Resolved — Product-Only Refund (Shipping Released to Seller)', variant: 'warning' },
    }

    const config = statusConfig[dispute.status] || {
      label: `Resolved (${dispute.status})`,
      variant: 'info' as const,
    }

    return (
      <div className="fixed bottom-0 left-0 right-0 bg-gray-50 border-t border-gray-200 shadow-lg p-4 z-10">
        <div className="max-w-7xl mx-auto flex items-center justify-center gap-3">
          <Lock className="h-5 w-5 text-gray-500" />
          <Badge variant={config.variant}>{config.label}</Badge>
          <p className="text-sm text-gray-600">
            This dispute was resolved on{' '}
            {dispute.resolved_at
              ? new Date(dispute.resolved_at).toLocaleDateString('id-ID')
              : 'a previous date'}
            . No further actions can be taken.
          </p>
          {dispute.resolution_notes && (
            <div className="ml-4 px-3 py-2 bg-white rounded border border-gray-200 max-w-md">
              <p className="text-xs text-gray-500 mb-1">Admin Notes:</p>
              <p className="text-sm text-gray-900">{dispute.resolution_notes}</p>
            </div>
          )}
        </div>
      </div>
    )
  }

  // Confirmation step
  if (step === 'confirm' && selectedDecision) {
    const getDecisionSummary = () => {
      switch (selectedDecision) {
        case 'refund_full':
          return {
            title: 'Full Refund to Buyer',
            variant: 'warning' as const,
            icon: <CheckCircle className="h-5 w-5 text-amber-600" />,
            description: 'Buyer receives the full order amount (subtotal + shipping). Seller receives nothing.',
            warning: 'The seller will not receive any payment from this order.',
          }
        case 'refund_partial':
          return {
            title: 'Product-Only Refund',
            variant: 'warning' as const,
            icon: <DollarSign className="h-5 w-5 text-amber-600" />,
            description: 'Buyer receives the item price (subtotal) only. Shipping fee is released to the seller.',
            warning: 'The split amount is calculated from the order. Buyer gets subtotal, seller keeps shipping.',
          }
        case 'reject':
          return {
            title: 'Release to Seller',
            variant: 'danger' as const,
            icon: <XCircle className="h-5 w-5 text-red-600" />,
            description: 'All funds are released to the seller. Buyer receives no refund.',
            warning: 'The buyer will not receive a refund. Full payment goes to the seller.',
          }
      }
    }

    const summary = getDecisionSummary()

    return (
      <div className="fixed bottom-0 left-0 right-0 bg-white border-t border-gray-200 shadow-lg p-4 z-10">
        <div className="max-w-7xl mx-auto">
          <div className="flex items-center gap-6">
            <div className="flex items-start gap-3 flex-1">
              <div className={`p-2 rounded-lg ${
                summary.variant === 'danger' ? 'bg-red-50' : 'bg-amber-50'
              }`}>
                {summary.icon}
              </div>

              <div className="flex-1">
                <p className={`font-semibold ${
                  summary.variant === 'danger' ? 'text-red-700' : 'text-amber-700'
                }`}>
                  {summary.title}
                </p>
                <p className="text-sm text-gray-600 mt-1">{summary.description}</p>
                <div className={`flex items-start gap-2 mt-2 text-sm ${
                  summary.variant === 'danger' ? 'text-red-600' : 'text-amber-600'
                }`}>
                  <AlertTriangle className="h-4 w-4 flex-shrink-0 mt-0.5" />
                  <span>{summary.warning}</span>
                </div>
                {notes && (
                  <div className="mt-3 p-2 bg-gray-50 rounded border border-gray-200">
                    <p className="text-xs text-gray-500 mb-1">Your notes:</p>
                    <p className="text-sm text-gray-900 line-clamp-2">{notes}</p>
                  </div>
                )}
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Button variant="secondary" onClick={handleBack} disabled={submitting}>
                Back
              </Button>
              <Button
                variant={summary.variant === 'danger' ? 'danger' : 'warning'}
                onClick={handleConfirmSubmit}
                disabled={submitting || !canResolveDisputes}
                isLoading={submitting}
                title={!canResolveDisputes ? `Requires: ${requiredCapability}` : ''}
              >
                Confirm & Submit
              </Button>
            </div>
          </div>
        </div>
      </div>
    )
  }

  // Selection step
  return (
    <div className="fixed bottom-0 left-0 right-0 bg-white border-t border-gray-200 shadow-lg p-4 z-10">
      <div className="max-w-7xl mx-auto">
        <div className="flex items-center justify-between gap-6">
          {/* Mandatory notes */}
          <div className="flex-1 max-w-lg">
            <label className="text-sm font-medium text-gray-700 flex items-center gap-1 mb-1">
              Resolution Notes
              <span className="text-red-500">*</span>
            </label>
            <textarea
              value={notes}
              onChange={handleNotesChange}
              placeholder="Explain your decision. Include details like evidence reviewed, reason for decision, etc. (minimum 10 characters)"
              rows={3}
              maxLength={MAX_NOTES_LENGTH}
              className={`w-full px-3 py-2 border rounded-lg text-sm focus:outline-none focus:ring-2 resize-none ${
                notesError
                  ? 'border-red-300 focus:ring-red-500'
                  : 'border-gray-300 focus:ring-primary'
              }`}
            />
            <div className="flex items-center justify-between mt-1">
              {notesError ? (
                <div className="flex items-center gap-1 text-red-600 text-xs">
                  <AlertCircle className="h-3 w-3" />
                  <span>{notesError}</span>
                </div>
              ) : (
                <p className="text-xs text-gray-500">
                  Minimum {MIN_NOTES_LENGTH} characters required
                </p>
              )}
              <p className={`text-xs ${notes.length < MIN_NOTES_LENGTH ? 'text-red-500' : 'text-gray-500'}`}>
                {notes.length}/{MAX_NOTES_LENGTH}
              </p>
            </div>
          </div>

          {/* Decision buttons */}
          <div className="flex items-center gap-3">
            {/* RELEASE_TO_SELLER */}
            <Button
              variant="secondary"
              onClick={() => handleDecisionClick('reject')}
              disabled={submitting || notes.trim().length < MIN_NOTES_LENGTH || !canResolveDisputes}
              className="border-orange-200 text-orange-700 hover:bg-orange-50"
              title={!canResolveDisputes ? `Requires: ${requiredCapability}` : 'Funds released to seller. Buyer gets no refund.'}
            >
              <XCircle className="h-4 w-4 mr-2" />
              Release to Seller
            </Button>

            {/* PRODUCT_ONLY_REFUND */}
            <Button
              variant="secondary"
              onClick={() => handleDecisionClick('refund_partial')}
              disabled={submitting || notes.trim().length < MIN_NOTES_LENGTH || !canResolveDisputes}
              className="border-amber-200 text-amber-700 hover:bg-amber-50"
              title={!canResolveDisputes ? `Requires: ${requiredCapability}` : 'Buyer gets item price; shipping stays with seller.'}
            >
              <DollarSign className="h-4 w-4 mr-2" />
              Product-Only Refund
            </Button>

            {/* FULL_REFUND */}
            <Button
              variant="warning"
              onClick={() => handleDecisionClick('refund_full')}
              disabled={submitting || notes.trim().length < MIN_NOTES_LENGTH || !canResolveDisputes}
              title={!canResolveDisputes ? `Requires: ${requiredCapability}` : 'Buyer gets subtotal + shipping. Seller receives nothing.'}
            >
              <CheckCircle className="h-4 w-4 mr-2" />
              Full Refund
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
