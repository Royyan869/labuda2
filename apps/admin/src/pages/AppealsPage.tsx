import { useState } from 'react'
import { FileText, Eye, Filter } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Select } from '@/components/ui/Select'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination, PageHeader } from '@/components/common'
import { AppealDetailModal } from '@/components/moderation/AppealDetailModal'
import { useAppeals } from '@/hooks/useAppeals'
import { formatDate } from '@/lib/utils'
import type {
  Appeal,
  AppealStatus,
} from '@/types'
import {
  appealStatusLabels,
  appealStatusVariants,
  APPEAL_STATUS,
} from '@/types'

const APPEAL_STATUSES: { value: AppealStatus | ''; label: string }[] = [
  { value: '', label: 'All Statuses' },
  { value: APPEAL_STATUS.PENDING, label: 'Pending' },
  { value: APPEAL_STATUS.APPROVED, label: 'Approved' },
  { value: APPEAL_STATUS.REJECTED, label: 'Rejected' },
]

export function AppealsPage() {
  const [statusFilter, setStatusFilter] = useState<AppealStatus | ''>('')
  const [selectedAppeal, setSelectedAppeal] = useState<Appeal | null>(null)
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false)

  const { appeals, loading, error, page, setPage, limit, count, refetch } = useAppeals(
    statusFilter ? { status: statusFilter } : {}
  )

  // `count` is the truthful server-side total for the current filter.
  const totalPages = limit > 0 ? Math.ceil(count / limit) : 0

  const handleViewDetail = (appeal: Appeal) => {
    setSelectedAppeal(appeal)
    setIsDetailModalOpen(true)
  }

  const handleReviewComplete = () => {
    setIsDetailModalOpen(false)
    setSelectedAppeal(null)
    refetch()
  }

  const handleClearFilters = () => {
    setStatusFilter('')
    setPage(1)
  }

  if (loading && appeals.length === 0) {
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Appeals" description="Review user appeals for moderation decisions" />
        <AdminErrorState title="Failed to load appeals" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  const pendingCount = appeals.filter(a => a.status === APPEAL_STATUS.PENDING).length

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader title="Appeals" description="Review user appeals for moderation decisions" />

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-muted-foreground">Total Appeals</p>
              <p className="type-metric-lg text-primary mt-1">{count}</p>
              <p className="type-caption mt-1">{pendingCount} pending review</p>
            </div>
            <div className="p-4 rounded-lg bg-info-bg">
              <FileText className="h-8 w-8 text-info" />
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
              onChange={(e) => { setStatusFilter(e.target.value as AppealStatus | ''); setPage(1) }}
            >
              {APPEAL_STATUSES.map((status) => (
                <option key={status.value} value={status.value}>
                  {status.label}
                </option>
              ))}
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* Appeals Table */}
      <Card>
        <CardHeader>
          <CardTitle>Appeals Queue</CardTitle>
        </CardHeader>
        <CardContent>
          {appeals.length === 0 ? (
            <AdminEmptyState
              icon={FileText}
              title="No Appeals Found"
              description={
                statusFilter
                  ? 'No appeals match the current filter.'
                  : 'No appeals pending review.'
              }
              filtered={Boolean(statusFilter)}
              onClearFilters={handleClearFilters}
            />
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Appeal ID</TableHead>
                    <TableHead>Decision ID</TableHead>
                    <TableHead>Message</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Submitted Date</TableHead>
                    <TableHead>Reviewed By</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {appeals.map((appeal) => (
                    <TableRow key={appeal.id}>
                      <TableCell className="font-mono text-sm">
                        {appeal.id.slice(0, 8)}
                      </TableCell>
                      <TableCell className="font-mono text-sm">
                        {appeal.decision_id.slice(0, 8)}
                      </TableCell>
                      <TableCell>
                        <div className="max-w-xs truncate">
                          {appeal.message}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={appealStatusVariants[appeal.status]}>
                          {appealStatusLabels[appeal.status]}
                        </Badge>
                      </TableCell>
                      <TableCell className="type-secondary">
                        {formatDate(appeal.created_at)}
                      </TableCell>
                      <TableCell className="type-secondary">
                        {appeal.reviewed_by ? (
                          <span className="font-mono text-xs">{appeal.reviewed_by.slice(0, 8)}</span>
                        ) : (
                          <span className="text-muted-foreground">-</span>
                        )}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          size="sm"
                          onClick={() => handleViewDetail(appeal)}
                        >
                          <Eye className="h-4 w-4 mr-1" />
                          Review
                        </Button>
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

      {/* Appeal Detail Modal */}
      <AppealDetailModal
        isOpen={isDetailModalOpen}
        onClose={() => {
          setIsDetailModalOpen(false)
          setSelectedAppeal(null)
        }}
        appeal={selectedAppeal}
        onReviewComplete={handleReviewComplete}
      />
    </div>
  )
}
