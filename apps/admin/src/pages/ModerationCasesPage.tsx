import { useState } from 'react'
import { Shield, Eye, Filter } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { CaseDetailModal } from '@/components/moderation/CaseDetailModal'
import { useModerationCases, useCaseAction } from '@/hooks/useModeration'
import { formatDate } from '@/lib/utils'
import type {
  ModerationCase,
  ModerationCaseStatus,
  ResourceType,
} from '@/types'
import {
  moderationCaseStatusLabels,
  resourceTypeLabels,
  moderationCaseStatusVariants,
} from '@/types'

const CASE_STATUSES: { value: ModerationCaseStatus | ''; label: string }[] = [
  { value: '', label: 'All Statuses' },
  { value: 'pending', label: 'Pending' },
  { value: 'approved', label: 'Approved' },
  { value: 'rejected', label: 'Rejected' },
  { value: 'enforced', label: 'Enforced' },
]

const RESOURCE_TYPES: { value: ResourceType | ''; label: string }[] = [
  { value: '', label: 'All Types' },
  { value: 'content', label: 'Content' },
  { value: 'comment', label: 'Comment' },
  { value: 'user', label: 'User' },
  { value: 'chat_message', label: 'Chat Message' },
  { value: 'fixed_price_sale', label: 'Fixed-Price Sale' },
  { value: 'auction', label: 'Auction' },
]

export function ModerationCasesPage() {
  const [statusFilter, setStatusFilter] = useState<ModerationCaseStatus | ''>('')
  const [resourceTypeFilter, setResourceTypeFilter] = useState<ResourceType | ''>('')
  const [selectedCase, setSelectedCase] = useState<ModerationCase | null>(null)
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const { cases, loading, error, count, refetch } = useModerationCases({
    ...(statusFilter && { status: statusFilter }),
    ...(resourceTypeFilter && { resource_type: resourceTypeFilter }),
  })
  const { executeAction, loading: isSubmitting } = useCaseAction()

  const handleViewDetail = (caseItem: ModerationCase) => {
    setSelectedCase(caseItem)
    setActionError(null)
    setIsDetailModalOpen(true)
  }

  const handleAction = async (caseId: string, action: 'approve' | 'reject' | 'enforce', notes?: string) => {
    setActionError(null)
    try {
      await executeAction(caseId, { action, notes })
      setIsDetailModalOpen(false)
      setSelectedCase(null)
      refetch()
    } catch (err) {
      console.error('Failed to execute action:', err)
      const message = err instanceof Error ? err.message : 'Failed to execute action'
      setActionError(message)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading moderation cases...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Moderation Cases</h1>
          <p className="text-gray-600 mt-1">Review and moderate reported content</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading cases: {error.message}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  const pendingCount = cases.filter(c => c.status === 'pending').length

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Moderation Cases</h1>
        <p className="text-gray-600 mt-1">Review and moderate reported content and comments</p>
      </div>

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-600">Total Cases</p>
              <p className="text-3xl font-bold text-primary mt-1">{count}</p>
              <p className="text-xs text-gray-500 mt-1">{pendingCount} pending review</p>
            </div>
            <div className="p-4 rounded-lg bg-blue-100">
              <Shield className="h-8 w-8 text-blue-600" />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center gap-6">
            <div className="flex items-center gap-4">
              <Filter className="h-5 w-5 text-gray-500" />
              <label htmlFor="status-filter" className="text-sm font-medium text-gray-700">
                Status:
              </label>
              <select
                id="status-filter"
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value as ModerationCaseStatus | '')}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {CASE_STATUSES.map((status) => (
                  <option key={status.value} value={status.value}>
                    {status.label}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex items-center gap-4">
              <label htmlFor="resource-type-filter" className="text-sm font-medium text-gray-700">
                Resource Type:
              </label>
              <select
                id="resource-type-filter"
                value={resourceTypeFilter}
                onChange={(e) => setResourceTypeFilter(e.target.value as ResourceType | '')}
                className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {RESOURCE_TYPES.map((type) => (
                  <option key={type.value} value={type.value}>
                    {type.label}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Cases Table */}
      <Card>
        <CardHeader>
          <CardTitle>Cases Queue</CardTitle>
        </CardHeader>
        <CardContent>
          {cases.length === 0 ? (
            <div className="text-center py-12">
              <Shield className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Cases Found</h3>
              <p className="text-gray-600">
                {statusFilter ? 'No cases match the current filter.' : 'No moderation cases pending review.'}
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Case ID</TableHead>
                    <TableHead>Resource Type</TableHead>
                    <TableHead>Reason</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Reported Date</TableHead>
                    <TableHead>Reviewed By</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {cases.map((caseItem) => (
                    <TableRow key={caseItem.id}>
                      <TableCell className="font-mono text-sm">
                        {caseItem.id.slice(0, 8)}
                      </TableCell>
                      <TableCell>
                        <Badge variant="default">
                          {resourceTypeLabels[caseItem.resource_type]}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="max-w-xs truncate">
                          {caseItem.reason}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={moderationCaseStatusVariants[caseItem.status]}>
                          {moderationCaseStatusLabels[caseItem.status]}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {formatDate(caseItem.created_at)}
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {caseItem.reviewed_by ? (
                          <span className="font-mono text-xs">{caseItem.reviewed_by.slice(0, 8)}</span>
                        ) : (
                          <span className="text-gray-400">-</span>
                        )}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          size="sm"
                          onClick={() => handleViewDetail(caseItem)}
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

      {/* Case Detail Modal */}
      <CaseDetailModal
        isOpen={isDetailModalOpen}
        onClose={() => {
          setIsDetailModalOpen(false)
          setSelectedCase(null)
          setActionError(null)
        }}
        caseData={selectedCase}
        onAction={handleAction}
        isSubmitting={isSubmitting}
        actionError={actionError}
      />
    </div>
  )
}
