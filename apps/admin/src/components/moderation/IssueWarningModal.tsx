import { useState, useEffect } from 'react'
import { AlertTriangle, Info } from 'lucide-react'
import { Modal, ModalFooter } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { useIssueWarning } from '@/hooks/useWarnings'
import type { WarningLevel } from '@/types'
import { warningLevelLabels, warningLevelVariants } from '@/types'

const WARNING_LEVELS: { value: WarningLevel; label: string; description: string; color: string }[] = [
  {
    value: 'info',
    label: 'Info',
    description: 'Minor policy reminder - for first-time or unintentional violations',
    color: 'text-blue-600',
  },
  {
    value: 'warning',
    label: 'Warning',
    description: 'Moderate policy violation - remains on user record',
    color: 'text-amber-600',
  },
  {
    value: 'severe',
    label: 'Severe',
    description: 'Serious policy violation - may lead to account restrictions',
    color: 'text-red-600',
  },
]

interface IssueWarningModalProps {
  isOpen: boolean
  onClose: () => void
  onWarningIssued: () => void
}

const MIN_REASON_LENGTH = 10
const MAX_REASON_LENGTH = 500
const MIN_EXPIRY_DAYS = 1
const MAX_EXPIRY_DAYS = 365

export function IssueWarningModal({ isOpen, onClose, onWarningIssued }: IssueWarningModalProps) {
  const [userId, setUserId] = useState('')
  const [level, setLevel] = useState<WarningLevel>('warning')
  const [reason, setReason] = useState('')
  const [expiresEnabled, setExpiresEnabled] = useState(false)
  const [expiresDays, setExpiresDays] = useState(30)
  const [error, setError] = useState<string | null>(null)
  const [showConfirm, setShowConfirm] = useState(false)

  const { issueWarning, loading } = useIssueWarning()

  // Reset state when modal closes
  useEffect(() => {
    if (!isOpen) {
      const timer = window.setTimeout(() => {
        setUserId('')
        setLevel('warning')
        setReason('')
        setExpiresEnabled(false)
        setExpiresDays(30)
        setError(null)
        setShowConfirm(false)
      }, 0)
      return () => window.clearTimeout(timer)
    }
  }, [isOpen])

  /**
   * Validate form fields
   * Returns error message if invalid, null if valid
   */
  const validateForm = (): string | null => {
    if (!userId.trim()) {
      return 'User ID is required'
    }
    // Basic UUID format validation (simple check)
    const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
    if (!uuidPattern.test(userId.trim())) {
      return 'User ID must be a valid UUID format'
    }
    if (!reason.trim()) {
      return 'Reason is required'
    }
    if (reason.trim().length < MIN_REASON_LENGTH) {
      return `Reason must be at least ${MIN_REASON_LENGTH} characters (current: ${reason.trim().length})`
    }
    if (reason.length > MAX_REASON_LENGTH) {
      return `Reason cannot exceed ${MAX_REASON_LENGTH} characters`
    }
    if (expiresEnabled) {
      if (expiresDays < MIN_EXPIRY_DAYS || expiresDays > MAX_EXPIRY_DAYS) {
        return `Expiry must be between ${MIN_EXPIRY_DAYS} and ${MAX_EXPIRY_DAYS} days`
      }
    }
    return null
  }

  const handleSubmitClick = (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    const validationError = validateForm()
    if (validationError) {
      setError(validationError)
      return
    }

    setShowConfirm(true)
  }

  const handleConfirmIssue = async () => {
    setError(null)

    try {
      const expiresAt = expiresEnabled
        ? Math.floor(Date.now() / 1000) + expiresDays * 86400
        : undefined

      await issueWarning({
        user_id: userId.trim(),
        level,
        reason: reason.trim(),
        expires_at: expiresAt,
      })

      onWarningIssued()
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to issue warning'
      setError(message)
      setShowConfirm(false)
    }
  }

  const handleCancelConfirm = () => {
    setShowConfirm(false)
  }

  const selectedLevelConfig = WARNING_LEVELS.find(wl => wl.value === level)
  const expiresDate = (() => {
    if (!expiresEnabled) return 'Never'
    const date = new Date()
    date.setDate(date.getDate() + expiresDays)
    return date.toLocaleDateString()
  })()

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Issue Warning" size="md">
      <form onSubmit={handleSubmitClick} className="space-y-6">
        {showConfirm ? (
          // Confirmation View
          <div className="space-y-4">
            <div className={`text-center ${selectedLevelConfig?.color}`}>
              <AlertTriangle className="h-12 w-12 mx-auto mb-3" />
              <h3 className="text-lg font-semibold">Confirm Warning Issuance</h3>
            </div>

            <div className="bg-gray-50 rounded-lg p-4 space-y-3">
              <div className="flex justify-between">
                <span className="text-sm text-gray-600">User ID:</span>
                <span className="font-mono text-sm">{userId}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-sm text-gray-600">Level:</span>
                <Badge variant={warningLevelVariants[level]}>
                  {warningLevelLabels[level]}
                </Badge>
              </div>
              <div className="flex justify-between">
                <span className="text-sm text-gray-600">Expires:</span>
                <span className="text-sm font-medium">{expiresDate}</span>
              </div>
              <div className="border-t pt-3 mt-3">
                <span className="text-sm text-gray-600">Reason:</span>
                <p className="text-sm mt-1 bg-white p-2 rounded border break-words">
                  {reason}
                </p>
              </div>
            </div>

            <div className="bg-amber-50 border border-amber-200 text-amber-800 p-3 rounded-lg flex items-start gap-2">
              <AlertTriangle className="h-4 w-4 flex-shrink-0 mt-0.5" />
              <p className="text-sm">
                The user will receive a notification about this warning. This action will be logged.
              </p>
            </div>

            <ModalFooter>
              <Button variant="secondary" onClick={handleCancelConfirm} disabled={loading}>
                Back
              </Button>
              <Button
                variant={level === 'severe' ? 'danger' : level === 'warning' ? 'warning' : 'primary'}
                onClick={handleConfirmIssue}
                disabled={loading}
                isLoading={loading}
              >
                Issue Warning
              </Button>
            </ModalFooter>
          </div>
        ) : (
          // Form View
          <>
            {/* User ID */}
            <div>
              <label htmlFor="user-id" className="text-sm font-medium text-gray-700 mb-2 block">
                User ID <span className="text-red-500">*</span>
              </label>
              <input
                id="user-id"
                type="text"
                value={userId}
                onChange={(e) => setUserId(e.target.value)}
                placeholder="Enter user ID (UUID)"
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary font-mono"
              />
              <p className="text-xs text-gray-500 mt-1">Format: UUID (e.g., 123e4567-e89b-12d3-a456-426614174000)</p>
            </div>

            {/* Warning Level */}
            <div>
              <label className="text-sm font-medium text-gray-700 mb-2 block">
                Warning Level <span className="text-red-500">*</span>
              </label>
              <div className="space-y-2">
                {WARNING_LEVELS.map((wl) => (
                  <label
                    key={wl.value}
                    className={`
                      flex items-start gap-3 p-3 border rounded-lg cursor-pointer transition-colors
                      ${level === wl.value
                        ? 'border-primary bg-primary/5'
                        : 'border-gray-200 hover:border-gray-300'
                      }
                    `}
                  >
                    <input
                      type="radio"
                      name="level"
                      value={wl.value}
                      checked={level === wl.value}
                      onChange={(e) => setLevel(e.target.value as WarningLevel)}
                      className="mt-0.5"
                    />
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{wl.label}</span>
                        <Badge variant={warningLevelVariants[wl.value]} className="text-xs">
                          {warningLevelLabels[wl.value]}
                        </Badge>
                      </div>
                      <p className={`text-sm mt-1 ${wl.color}`}>{wl.description}</p>
                    </div>
                  </label>
                ))}
              </div>
            </div>

            {/* Reason */}
            <div>
              <label htmlFor="reason" className="text-sm font-medium text-gray-700 mb-2 block">
                Reason <span className="text-red-500">*</span>
              </label>
              <textarea
                id="reason"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="Explain why this warning is being issued..."
                rows={4}
                maxLength={MAX_REASON_LENGTH}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary resize-none"
              />
              <div className="flex items-center justify-between mt-1">
                <p className={`text-xs ${reason.trim().length < MIN_REASON_LENGTH && reason.length > 0 ? 'text-amber-600' : 'text-gray-500'}`}>
                  {reason.trim().length < MIN_REASON_LENGTH && reason.length > 0
                    ? `Minimum ${MIN_REASON_LENGTH} characters required`
                    : `${reason.length}/${MAX_REASON_LENGTH} characters`
                  }
                </p>
              </div>
            </div>

            {/* Expiration */}
            <div>
              <div className="flex items-center gap-2 mb-2">
                <input
                  id="expires-enabled"
                  type="checkbox"
                  checked={expiresEnabled}
                  onChange={(e) => setExpiresEnabled(e.target.checked)}
                  className="rounded"
                />
                <label htmlFor="expires-enabled" className="text-sm font-medium text-gray-700">
                  Set expiration date
                </label>
              </div>
              {expiresEnabled && (
                <div className="flex items-center gap-2 ml-6">
                  <input
                    type="number"
                    min={MIN_EXPIRY_DAYS}
                    max={MAX_EXPIRY_DAYS}
                    value={expiresDays}
                    onChange={(e) => {
                      const val = parseInt(e.target.value)
                      if (!isNaN(val) && val >= MIN_EXPIRY_DAYS && val <= MAX_EXPIRY_DAYS) {
                        setExpiresDays(val)
                      }
                    }}
                    className="w-20 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                  />
                  <span className="text-sm text-gray-600">
                    days ({expiresDate})
                  </span>
                </div>
              )}
            </div>

            {/* Info Note */}
            <div className="bg-blue-50 border border-blue-200 text-blue-700 p-3 rounded-lg flex items-start gap-2">
              <Info className="h-4 w-4 flex-shrink-0 mt-0.5" />
              <p className="text-sm">
                The user will be notified of this warning. Severe warnings may impact account standing.
              </p>
            </div>

            {/* Error Message */}
            {error && (
              <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-lg flex items-center gap-2">
                <AlertTriangle className="h-4 w-4 flex-shrink-0" />
                <span className="text-sm">{error}</span>
              </div>
            )}

            {/* Actions */}
            <ModalFooter>
              <Button type="button" variant="secondary" onClick={onClose} disabled={loading}>
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={!userId.trim() || !reason.trim() || loading}
                isLoading={loading}
              >
                Review & Issue
              </Button>
            </ModalFooter>
          </>
        )}
      </form>
    </Modal>
  )
}
