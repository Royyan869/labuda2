import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AlertTriangle, Eye, Filter, RefreshCw, Clock } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Select } from '@/components/ui/Select'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination, PageHeader } from '@/components/common'
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

  const { disputes, loading, error, total, page, setPage, totalPages, refetch } = useDisputes(
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

  const handleClearFilters = () => {
    setStatusFilter('')
    setPage(1)
  }

  if (loading && disputes.length === 0) {
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Disputes" description="Review and resolve buyer-seller disputes" />
        <AdminErrorState title="Failed to load disputes" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  const openedCount = disputes.filter(d => d.status === 'under_review').length

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title="Disputes"
        description="Review and resolve buyer-seller disputes"
        actions={
          <Button variant="secondary" onClick={refetch} className="gap-2">
            <RefreshCw className="h-4 w-4" />
            Refresh
          </Button>
        }
      />

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-muted-foreground">Total Disputes</p>
              <p className="type-metric-lg text-primary mt-1">{total}</p>
              <p className="type-caption mt-1">{openedCount} pending resolution</p>
            </div>
            <div className="p-4 rounded-lg bg-warning-bg">
              <AlertTriangle className="h-8 w-8 text-warning" />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-end gap-4">
            <Filter className="h-5 w-5 text-muted-foreground mb-2" />
            <Select
              label="Status:"
              value={statusFilter}
              onChange={(e) => { setStatusFilter(e.target.value as DisputeStatus | ''); setPage(1) }}
            >
              {DISPUTE_STATUSES.map((status) => (
                <option key={status.value} value={status.value}>
                  {status.label}
                </option>
              ))}
            </Select>
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
            <AdminEmptyState
              icon={AlertTriangle}
              title="No Disputes Found"
              description={
                statusFilter
                  ? 'No disputes match the current filter.'
                  : 'No disputes in the system.'
              }
              filtered={Boolean(statusFilter)}
              onClearFilters={handleClearFilters}
            />
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
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
                            <div className="w-6 h-6 rounded-full bg-surface-muted" />
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
                            <div className="w-6 h-6 rounded-full bg-surface-muted" />
                          )}
                          <div className="min-w-0">
                            <p className="text-sm truncate max-w-[100px]">
                              {dispute.seller_username || 'Unknown'}
                            </p>
                            {dispute.seller_farm_name && (
                              <p className="type-caption truncate max-w-[100px]">
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
                            <AlertTriangle className="h-3 w-3 text-destructive" />
                            <Badge variant="error" className="text-xs">
                              OVERDUE
                            </Badge>
                          </div>
                        ) : dispute.admin_response_overdue ? (
                          <div className="flex items-center gap-1">
                            <Clock className="h-3 w-3 text-warning" />
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
                      <TableCell className="type-secondary">
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

      {/* Pagination */}
      {totalPages > 1 && (
        <AdminPagination
          page={page}
          totalPages={totalPages}
          onPageChange={setPage}
          disabled={loading}
        />
      )}

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
