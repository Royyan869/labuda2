import { AlertTriangle, RefreshCw, ArrowLeft, Clock, Activity } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import type { DisputeDetail, DisputeStatus } from '@/types'
import {
  disputeStatusLabels,
  disputeStatusVariants,
  disputeReasonLabels,
} from '@/types'
import { formatDate, formatRupiah } from '@/lib/utils'

// Helper function to get SLA badge variant
function getSLAVariant(adminOverdue: boolean, resolutionOverdue: boolean): 'success' | 'warning' | 'error' | 'info' | 'pending' {
  if (resolutionOverdue) return 'error'
  if (adminOverdue) return 'warning'
  return 'success'
}

// Helper function to get next action label
function getNextActionLabel(nextAction?: string | null): string {
  if (!nextAction) return 'Unknown'
  switch (nextAction) {
    case 'review':
      return 'Review Required'
    case 'wait_buyer':
      return 'Waiting for Buyer'
    case 'wait_seller':
      return 'Waiting for Seller'
    case 'resolved':
      return 'Resolved'
    case 'auto_resolve':
      return 'Auto-Resolution Pending'
    default:
      return nextAction
  }
}

// Helper function to get next action variant
function getNextActionVariant(nextAction?: string | null): 'success' | 'warning' | 'error' | 'info' | 'pending' {
  if (!nextAction) return 'info'
  switch (nextAction) {
    case 'review':
      return 'pending'
    case 'wait_buyer':
    case 'wait_seller':
      return 'info'
    case 'resolved':
      return 'success'
    case 'auto_resolve':
      return 'warning'
    default:
      return 'info'
  }
}

interface DisputeHeaderProps {
  dispute: DisputeDetail
  onRefresh: () => void
  refreshing?: boolean
  onBack?: () => void
}

export function DisputeHeader({ dispute, onRefresh, refreshing, onBack }: DisputeHeaderProps) {
  const isOpened = dispute.status === 'under_review'

  return (
    <div className="space-y-4">
      {/* Top row: Back button, ID, Status, Refresh */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          {onBack && (
            <Button
              variant="secondary"
              size="sm"
              onClick={onBack}
              className="gap-2"
            >
              <ArrowLeft className="h-4 w-4" />
              Back
            </Button>
          )}
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Dispute Workspace</h1>
            <p className="text-sm text-gray-500 mt-0.5">
              Dispute ID: <span className="font-mono">{dispute.id}</span>
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <Badge variant={disputeStatusVariants[dispute.status as DisputeStatus] || 'info'}>
            {disputeStatusLabels[dispute.status as DisputeStatus] || dispute.status}
          </Badge>
          <button
            onClick={onRefresh}
            disabled={refreshing}
            className="text-gray-400 hover:text-gray-600 transition-colors disabled:opacity-50"
            title="Refresh dispute data"
          >
            <RefreshCw className={`h-5 w-5 ${refreshing ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Warning if not opened */}
      {!isOpened && (
        <div className="bg-amber-50 border border-amber-200 text-amber-800 p-3 rounded-lg flex items-start gap-2">
          <AlertTriangle className="h-5 w-5 flex-shrink-0 mt-0.5" />
          <div className="flex-1">
            <p className="font-medium text-sm">Dispute Already Resolved</p>
            <p className="text-sm mt-1">
              This dispute was resolved on {formatDate(dispute.resolved_at || '')}.
              No further actions can be taken.
            </p>
          </div>
        </div>
      )}

      {/* Dispute Info Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {/* Order ID */}
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <p className="text-xs text-gray-500 uppercase tracking-wide">Order ID</p>
          <p className="font-mono text-sm mt-1">{dispute.order_id.slice(0, 12)}...</p>
        </div>

        {/* Reason */}
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <p className="text-xs text-gray-500 uppercase tracking-wide">Reason</p>
          <p className="font-medium text-sm mt-1">
            {disputeReasonLabels[dispute.reason] || dispute.reason}
          </p>
        </div>

        {/* Escrow Amount (money at risk) */}
        {dispute.escrow_amount != null && (
          <div className="bg-white rounded-lg border border-gray-200 p-4">
            <p className="text-xs text-gray-500 uppercase tracking-wide">Escrow at Risk</p>
            <p className="font-semibold text-sm mt-1">{formatRupiah(dispute.escrow_amount)}</p>
          </div>
        )}

        {/* Opened Date */}
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <p className="text-xs text-gray-500 uppercase tracking-wide">Opened</p>
          <p className="text-sm mt-1">{formatDate(dispute.opened_at)}</p>
        </div>
      </div>

      {/* Description */}
      {dispute.description && (
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <p className="text-xs text-gray-500 uppercase tracking-wide mb-2">Description</p>
          <p className="text-sm text-gray-900 whitespace-pre-wrap">{dispute.description}</p>
        </div>
      )}

      {/* SLA Metrics Panel */}
      <div className={`rounded-lg border p-4 ${
        dispute.resolution_overdue
          ? 'bg-red-50 border-red-200'
          : dispute.admin_response_overdue
          ? 'bg-orange-50 border-orange-200'
          : 'bg-white border-gray-200'
      }`}>
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <Clock className={`h-4 w-4 ${
              dispute.resolution_overdue
                ? 'text-red-600'
                : dispute.admin_response_overdue
                ? 'text-orange-600'
                : 'text-gray-600'
            }`} />
            <p className="text-xs font-semibold uppercase tracking-wide">
              SLA Status
            </p>
          </div>
          <Badge variant={getSLAVariant(dispute.admin_response_overdue, dispute.resolution_overdue)}>
            {dispute.sla_summary || 'Within SLA'}
          </Badge>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {/* Next Action */}
          <div>
            <p className="text-xs text-gray-600 mb-1">Next Action</p>
            <Badge variant={getNextActionVariant(dispute.next_action)} className="text-xs">
              {getNextActionLabel(dispute.next_action)}
            </Badge>
          </div>

          {/* Admin Response Time */}
          {dispute.admin_response_time && (
            <div>
              <p className="text-xs text-gray-600 mb-1">Admin Response</p>
              <p className="text-sm font-medium">{dispute.admin_response_time}</p>
              {dispute.admin_response_overdue && dispute.admin_response_overdue_duration && (
                <p className="text-xs text-red-600 mt-0.5">
                  Overdue by {dispute.admin_response_overdue_duration}
                </p>
              )}
            </div>
          )}

          {/* Resolution Time */}
          {dispute.resolution_time && (
            <div>
              <p className="text-xs text-gray-600 mb-1">Resolution Time</p>
              <p className="text-sm font-medium">{dispute.resolution_time}</p>
              {dispute.resolution_overdue && dispute.resolution_overdue_duration && (
                <p className="text-xs text-red-600 mt-0.5">
                  Overdue by {dispute.resolution_overdue_duration}
                </p>
              )}
            </div>
          )}

          {/* Active Time */}
          {dispute.active_time && (
            <div>
              <p className="text-xs text-gray-600 mb-1">Active Time</p>
              <p className="text-sm font-medium flex items-center gap-1">
                <Activity className="h-3 w-3" />
                {dispute.active_time}
              </p>
            </div>
          )}
        </div>

        {/* Warning for overdue disputes */}
        {dispute.resolution_overdue && (
          <div className="mt-3 pt-3 border-t border-red-200">
            <div className="flex items-start gap-2">
              <AlertTriangle className="h-4 w-4 text-red-600 flex-shrink-0 mt-0.5" />
              <div>
                <p className="text-sm font-medium text-red-800">Resolution SLA Breached</p>
                <p className="text-xs text-red-700 mt-0.5">
                  This dispute has exceeded the 48-hour resolution SLA.
                  {dispute.resolution_overdue_duration && ` Overdue by ${dispute.resolution_overdue_duration}.`}
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Warning for admin response overdue */}
        {dispute.admin_response_overdue && !dispute.resolution_overdue && (
          <div className="mt-3 pt-3 border-t border-orange-200">
            <div className="flex items-start gap-2">
              <AlertTriangle className="h-4 w-4 text-orange-600 flex-shrink-0 mt-0.5" />
              <div>
                <p className="text-sm font-medium text-orange-800">Admin Response SLA Breached</p>
                <p className="text-xs text-orange-700 mt-0.5">
                  This dispute has exceeded the 2-hour admin response SLA.
                  {dispute.admin_response_overdue_duration && ` Overdue by ${dispute.admin_response_overdue_duration}.`}
                </p>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
