import { useState } from 'react'
import { AlertTriangle, ShieldAlert, CheckCircle2, Gavel } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Textarea } from '@/components/ui/Textarea'
import { Modal, ModalFooter } from '@/components/ui/Modal'
import { Badge } from '@/components/ui/Badge'
import { adminCancelAuction, type AdminCancelAuctionResponse } from '@/lib/api/auctions'
import { ApiError } from '@/lib/api/client'

/**
 * Extracts a clear, backend-specific message from an admin API error.
 *
 * The shared HTTP client (client.ts) already overrides the message/code for
 * 401/403/404 with fixed client-side copy. For every other status (notably
 * 400 and 409 from this route) the raw backend envelope is
 * `{ success: false, error: { code, message } }`, which the client's generic
 * error path does not unwrap — so this reads err.data directly for those
 * cases instead of trusting err.message/err.code blindly.
 */
function extractApiErrorDetail(err: ApiError): { code: string; message: string } {
  if (err.code) {
    // client.ts already assigned a specific code (401/403/404) with a clear,
    // friendly message — trust it, consistent with every other admin page.
    return { code: err.code, message: err.message }
  }
  // Generic path (400/409/500/...): client.ts left code undefined and
  // message as a generic "HTTP <status>: An error occurred" — read the raw
  // backend envelope directly to surface the actual reason.
  const raw = err.data as { error?: { code?: string; message?: string } } | undefined
  return {
    code: raw?.error?.code ?? 'UNKNOWN_ERROR',
    message: raw?.error?.message ?? err.message,
  }
}

export function AuctionEmergencyCancelPage() {
  const [auctionId, setAuctionId] = useState('')
  const [reason, setReason] = useState('')
  const [showConfirm, setShowConfirm] = useState(false)
  const [loading, setLoading] = useState(false)
  const [errorDetail, setErrorDetail] = useState<{ code: string; message: string } | null>(null)
  const [result, setResult] = useState<AdminCancelAuctionResponse | null>(null)

  const trimmedAuctionId = auctionId.trim()
  const trimmedReason = reason.trim()
  const isValid = trimmedAuctionId.length > 0 && trimmedReason.length > 0

  const handleRequestCancel = () => {
    if (!isValid || loading) return
    setErrorDetail(null)
    setShowConfirm(true)
  }

  const handleConfirmCancel = async () => {
    if (loading) return // guard against duplicate submit
    setLoading(true)
    setErrorDetail(null)
    try {
      const response = await adminCancelAuction(trimmedAuctionId, trimmedReason)
      setResult(response)
      setShowConfirm(false)
    } catch (err) {
      if (err instanceof ApiError) {
        setErrorDetail(extractApiErrorDetail(err))
      } else {
        setErrorDetail({ code: 'UNKNOWN_ERROR', message: 'An unexpected error occurred.' })
      }
      setShowConfirm(false)
    } finally {
      setLoading(false)
    }
  }

  const handleReset = () => {
    setAuctionId('')
    setReason('')
    setResult(null)
    setErrorDetail(null)
  }

  return (
    <div className="space-y-6 max-w-2xl">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-gray-900 flex items-center gap-2">
          <Gavel className="h-7 w-7 text-red-600" />
          Emergency Auction Cancel
        </h1>
        <p className="text-gray-600 mt-1">
          Governance-authority override to stop a live auction (e.g. unreachable or abusive
          seller). This is separate from moderation case enforcement — use this only when there
          is no filed moderation case, or the situation requires an immediate stop.
        </p>
      </div>

      <Card>
        <CardContent className="p-6 space-y-4">
          <div className="flex items-start gap-3 p-3 bg-amber-50 border border-amber-200 rounded-lg">
            <AlertTriangle className="h-5 w-5 text-amber-600 shrink-0 mt-0.5" />
            <p className="text-sm text-amber-800">
              This action cannot be undone. The backend remains the source of truth for whether
              cancellation is allowed — auctions that already have an order, or are already in a
              terminal state, will be rejected here and must be handled through the order/dispute
              admin pages instead.
            </p>
          </div>

          <div>
            <label htmlFor="auction-id" className="block text-sm font-medium text-gray-700 mb-1">
              Auction ID
            </label>
            <input
              id="auction-id"
              type="text"
              placeholder="UUID"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              value={auctionId}
              onChange={(e) => setAuctionId(e.target.value)}
              disabled={loading}
            />
          </div>

          <Textarea
            label="Reason (required)"
            placeholder="e.g. seller unreachable, trust & safety stop"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={loading}
            rows={3}
          />

          <Button
            variant="danger"
            onClick={handleRequestCancel}
            disabled={!isValid || loading}
          >
            {loading ? 'Cancelling...' : 'Cancel Auction'}
          </Button>
        </CardContent>
      </Card>

      {/* Error state */}
      {errorDetail && (
        <Card>
          <CardContent className="p-4">
            <div className="flex items-start gap-3">
              <ShieldAlert className="h-5 w-5 text-red-600 shrink-0 mt-0.5" />
              <div>
                <p className="text-sm font-medium text-gray-900">
                  <Badge variant="error" className="mr-2">{errorDetail.code}</Badge>
                  {errorDetail.message}
                </p>
                {errorDetail.code === 'AUCTION_CANCEL_CONFLICT' && (
                  <p className="text-xs text-gray-600 mt-2">
                    This auction cannot be cancelled here — it likely already has an order or is
                    in a terminal state. Use the Orders / Disputes admin pages to handle it through
                    the canonical order/dispute/refund flow instead.
                  </p>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Success state */}
      {result && (
        <Card>
          <CardContent className="p-4">
            <div className="flex items-start gap-3">
              <CheckCircle2 className="h-5 w-5 text-green-600 shrink-0 mt-0.5" />
              <div className="text-sm">
                <p className="font-medium text-gray-900">Auction cancelled</p>
                <p className="text-gray-600 mt-1">
                  {result.auction_id}: <Badge variant="default">{result.status_before}</Badge>
                  {' → '}
                  <Badge variant="success">{result.status_after}</Badge>
                </p>
                <p className="text-gray-500 mt-1">Reason: {result.reason}</p>
              </div>
            </div>
            <Button variant="ghost" size="sm" onClick={handleReset} className="mt-3">
              Cancel Another
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Confirmation modal */}
      <Modal
        isOpen={showConfirm}
        onClose={() => !loading && setShowConfirm(false)}
        title="Confirm Emergency Cancel"
        size="sm"
      >
        <p className="text-sm text-gray-700">
          This will immediately cancel auction <span className="font-mono">{trimmedAuctionId}</span>{' '}
          under governance authority. This action cannot be undone.
        </p>
        <p className="text-sm text-gray-600 mt-3">
          <span className="font-medium">Reason:</span> {trimmedReason}
        </p>
        <ModalFooter>
          <Button variant="secondary" onClick={() => setShowConfirm(false)} disabled={loading}>
            Go Back
          </Button>
          <Button variant="danger" onClick={handleConfirmCancel} disabled={loading}>
            {loading ? 'Cancelling...' : 'Confirm Cancel'}
          </Button>
        </ModalFooter>
      </Modal>
    </div>
  )
}
