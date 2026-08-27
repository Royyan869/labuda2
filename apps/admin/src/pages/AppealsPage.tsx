import { useState } from 'react'
import { FileText, Eye, Filter } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
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

  const { appeals, loading, error, count, refetch } = useAppeals(
    statusFilter ? { status: statusFilter } : {}
  )

  const handleViewDetail = (appeal: Appeal) => {
    setSelectedAppeal(appeal)
    setIsDetailModalOpen(true)
  }

  const handleReviewComplete = () => {
    setIsDetailModalOpen(false)
    setSelectedAppeal(null)
    refetch()
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading appeals...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Appeals</h1>
          <p className="text-gray-600 mt-1">Review user appeals for moderation decisions</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading appeals: {error.message}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  const pendingCount = appeals.filter(a => a.status === APPEAL_STATUS.PENDING).length

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Appeals</h1>
        <p className="text-gray-600 mt-1">Review user appeals for moderation decisions</p>
      </div>

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-600">Total Appeals</p>
              <p className="text-3xl font-bold text-primary mt-1">{count}</p>
              <p className="text-xs text-gray-500 mt-1">{pendingCount} pending review</p>
            </div>
            <div className="p-4 rounded-lg bg-purple-100">
              <FileText className="h-8 w-8 text-purple-600" />
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
              onChange={(e) => setStatusFilter(e.target.value as AppealStatus | '')}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            >
              {APPEAL_STATUSES.map((status) => (
                <option key={status.value} value={status.value}>
                  {status.label}
                </option>
              ))}
            </select>
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
            <div className="text-center py-12">
              <FileText className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Appeals Found</h3>
              <p className="text-gray-600">
                {statusFilter ? 'No appeals match the current filter.' : 'No appeals pending review.'}
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Appeal ID</TableHead>
                    <TableHead>Report ID</TableHead>
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
                        {appeal.report_id.slice(0, 8)}
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
                      <TableCell className="text-sm text-gray-600">
                        {formatDate(appeal.created_at)}
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {appeal.reviewed_by ? (
                          <span className="font-mono text-xs">{appeal.reviewed_by.slice(0, 8)}</span>
                        ) : (
                          <span className="text-gray-400">-</span>
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
