import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AlertTriangle, Eye, Filter, RefreshCw, Clock } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { DisputeDetailModal } from '@/components/orders/DisputeDetailModal'
import { useDisputes } from '@/hooks/useDisputes'
import { formatDate } from '@/lib/utils'
import type {
  DisputeListItem,
  DisputeStatus,
} from '@/types'
import {
  disputeStatusLabels,
  disputeStatusVariants,
  disputeReasonLabels,
} from '@/types'

// Helper function to get SLA badge variant
function getSLAVariant(adminOverdue: boolean, resolutionOverdue: boolean): 'success' | 'warning' | 'error' | 'info' | 'pending' {
  if (resolutionOverdue) return 'error'
  if (adminOverdue) return 'warning'
  return 'success'
}

const DISPUTE_STATUSES: { value: DisputeStatus | ''; label: string }[] = [
  { value: '', label: 'All Statuses' },
  { value: 'under_review', label: 'Under Review' },
  { value: 'resolved_refund', label: 'Refunded' },
  { value: 'resolved_release', label: 'Released' },
]

export function DisputesPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<DisputeStatus | ''>('')
  const [selectedDispute, setSelectedDispute] = useState<DisputeListItem | null>(null)
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false)

  const { disputes, loading, error, total, refetch } = useDisputes(
    statusFilter ? { status: statusFilter } : {}
  )

  const handleViewDetail = (dispute: DisputeListItem) => {
    setSelectedDispute(dispute)
    setIsDetailModalOpen(true)
  }

  const handleOpenWorkspace = (disputeId: string) => {
    navigate(`/disputes/${disputeId}`)
  }

  const handleCloseModal = () => {
    setIsDetailModalOpen(false)
    setSelectedDispute(null)
  }

  const handleResolutionComplete = () => {
    refetch()
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading disputes...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Disputes</h1>
          <p className="text-gray-600 mt-1">Review and resolve buyer-seller disputes</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading disputes: {error.message}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  const openedCount = disputes.filter(d => d.status === 'under_review').length

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Disputes</h1>
          <p className="text-gray-600 mt-1">Review and resolve buyer-seller disputes</p>
        </div>
        <Button
          variant="secondary"
          onClick={refetch}
          className="gap-2"
        >
          <RefreshCw className="h-4 w-4" />
          Refresh
        </Button>
      </div>

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-600">Total Disputes</p>
              <p className="text-3xl font-bold text-primary mt-1">{total}</p>
              <p className="text-xs text-gray-500 mt-1">{openedCount} pending resolution</p>
            </div>
            <div className="p-4 rounded-lg bg-orange-100">
              <AlertTriangle className="h-8 w-8 text-orange-600" />
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
              onChange={(e) => setStatusFilter(e.target.value as DisputeStatus | '')}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            >
              {DISPUTE_STATUSES.map((status) => (
                <option key={status.value} value={status.value}>
                  {status.label}
                </option>
              ))}
            </select>
          </div>
        </CardContent>
      </Card>

      {/* Disputes Table */}
      <Card>
        <CardHeader>
          <CardTitle>Disputes Queue</CardTitle>
        </CardHeader>
        <CardContent>
          {disputes.length === 0 ? (
            <div className="text-center py-12">
              <AlertTriangle className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Disputes Found</h3>
              <p className="text-gray-600">
                {statusFilter
                  ? 'No disputes match the current filter.'
                  : 'No disputes in the system.'}
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Dispute ID</TableHead>
                    <TableHead>Order ID</TableHead>
                    <TableHead>Buyer</TableHead>
                    <TableHead>Seller</TableHead>
                    <TableHead>Reason</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>SLA Status</TableHead>
                    <TableHead>Opened At</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {disputes.map((dispute) => (
                    <TableRow key={dispute.id}>
                      <TableCell className="font-mono text-sm">
                        {dispute.id.slice(0, 8)}
                      </TableCell>
                      <TableCell className="font-mono text-sm">
                        {dispute.order_id.slice(0, 8)}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          {dispute.buyer_avatar ? (
                            <img
                              src={dispute.buyer_avatar}
                              alt=""
                              className="w-6 h-6 rounded-full object-cover"
                            />
                          ) : (
                            <div className="w-6 h-6 rounded-full bg-gray-200" />
                          )}
                          <span className="text-sm truncate max-w-[100px]">
                            {dispute.buyer_username || 'Unknown'}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          {dispute.seller_avatar ? (
                            <img
                              src={dispute.seller_avatar}
                              alt=""
                              className="w-6 h-6 rounded-full object-cover"
                            />
                          ) : (
                            <div className="w-6 h-6 rounded-full bg-gray-200" />
                          )}
                          <div className="min-w-0">
                            <p className="text-sm truncate max-w-[100px]">
                              {dispute.seller_username || 'Unknown'}
                            </p>
                            {dispute.seller_farm_name && (
                              <p className="text-xs text-gray-500 truncate max-w-[100px]">
                                {dispute.seller_farm_name}
                              </p>
                            )}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <span className="text-sm">
                          {disputeReasonLabels[dispute.reason] || dispute.reason}
                        </span>
                      </TableCell>
                      <TableCell>
                        <Badge variant={disputeStatusVariants[dispute.status] || 'info'}>
                          {disputeStatusLabels[dispute.status] || dispute.status}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {dispute.resolution_overdue ? (
                          <div className="flex items-center gap-1">
                            <AlertTriangle className="h-3 w-3 text-red-600" />
                            <Badge variant="error" className="text-xs">
                              OVERDUE
                            </Badge>
                          </div>
                        ) : dispute.admin_response_overdue ? (
                          <div className="flex items-center gap-1">
                            <Clock className="h-3 w-3 text-orange-600" />
                            <Badge variant="warning" className="text-xs">
                              Response Late
                            </Badge>
                          </div>
                        ) : (
                          <Badge variant={getSLAVariant(dispute.admin_response_overdue, dispute.resolution_overdue)} className="text-xs">
                            {dispute.sla_summary || 'Within SLA'}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {formatDate(dispute.opened_at)}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex items-center justify-end gap-2">
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => handleViewDetail(dispute)}
                          >
                            <Eye className="h-4 w-4 mr-1" />
                            View
                          </Button>
                          <Button
                            size="sm"
                            onClick={() => handleOpenWorkspace(dispute.id)}
                          >
                            Workspace
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Dispute Detail Modal */}
      <DisputeDetailModal
        isOpen={isDetailModalOpen}
        onClose={handleCloseModal}
        disputeData={selectedDispute}
        onResolutionComplete={handleResolutionComplete}
      />
    </div>
  )
}
