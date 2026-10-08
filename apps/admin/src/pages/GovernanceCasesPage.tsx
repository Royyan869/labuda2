/**
 * Governance Cases Page
 *
 * Canonical admin governance case list.
 * Displays Cases from the canonical cases table with status filter and pagination.
 *
 * Authority: REPORT_GOVERNANCE_ADMIN_BACKEND_IMPLEMENTATION_SLICE_6.md
 */
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Shield, Eye, Filter } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Select } from '@/components/ui/Select'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination, PageHeader } from '@/components/common'
import { useGovernanceCases } from '@/hooks/useGovernance'
import { formatDate } from '@/lib/utils'
import {
  caseStatusLabels,
  targetTypeLabels,
  caseStatusVariants,
} from '@/types/governance'
import type { GovernanceCaseStatus } from '@/types/governance'

const CASE_FILTERS: { value: GovernanceCaseStatus | ''; label: string }[] = [
  { value: '', label: 'All Cases' },
  { value: 'open', label: 'Open' },
  { value: 'resolved', label: 'Resolved' },
]

export function GovernanceCasesPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<GovernanceCaseStatus | ''>('')
  const [page, setPage] = useState(1)

  const { cases, loading, error, count, refetch } = useGovernanceCases({
    ...(statusFilter ? { status: statusFilter } : {}),
    page,
    limit: 20,
  })

  // `count` is the server-side total (SELECT COUNT(*), optionally status-filtered);
  // derive total pages from the existing page size of 20.
  const totalPages = Math.ceil(count / 20)

  const handleViewCase = (caseId: string) => {
    navigate(`/moderation/cases/${caseId}`)
  }

  const handleClearFilters = () => {
    setStatusFilter('')
    setPage(1)
  }

  if (loading && cases.length === 0) {
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Governance Cases" description="Review and decide on reported subjects" />
        <AdminErrorState title="Failed to load cases" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader title="Governance Cases" description="Review and decide on reported subjects" />

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-muted-foreground">Total Cases</p>
              <p className="type-metric-lg text-primary mt-1">{count}</p>
            </div>
            <div className="p-4 rounded-lg bg-info-bg">
              <Shield className="h-8 w-8 text-info" />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center gap-4">
            <Filter className="h-5 w-5 text-muted-foreground" />
            <Select
              label="Status:"
              value={statusFilter}
              onChange={(e) => {
                setStatusFilter(e.target.value as GovernanceCaseStatus | '')
                setPage(1)
              }}
            >
              {CASE_FILTERS.map((f) => (
                <option key={f.value} value={f.value}>
                  {f.label}
                </option>
              ))}
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* Cases Table */}
      <Card>
        <CardHeader>
          <CardTitle>Cases</CardTitle>
        </CardHeader>
        <CardContent>
          {cases.length === 0 ? (
            <AdminEmptyState
              icon={Shield}
              title="No Cases Found"
              description={
                statusFilter
                  ? `No ${statusFilter} cases found.`
                  : 'No governance cases yet.'
              }
              filtered={Boolean(statusFilter)}
              onClearFilters={handleClearFilters}
            />
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Case ID</TableHead>
                    <TableHead>Subject Type</TableHead>
                    <TableHead>Subject ID</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead>Updated</TableHead>
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
                          {targetTypeLabels[caseItem.subject_type]}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-mono text-sm">
                        {caseItem.subject_id.slice(0, 8)}
                      </TableCell>
                      <TableCell>
                        <Badge variant={caseStatusVariants[caseItem.status]}>
                          {caseStatusLabels[caseItem.status]}
                        </Badge>
                      </TableCell>
                      <TableCell className="type-secondary">
                        {formatDate(caseItem.created_at)}
                      </TableCell>
                      <TableCell className="type-secondary">
                        {formatDate(caseItem.updated_at)}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          size="sm"
                          onClick={() => handleViewCase(caseItem.id)}
                        >
                          <Eye className="h-4 w-4 mr-1" />
                          View
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}

          {/* Pagination */}
          {totalPages > 1 && (
            <AdminPagination
              page={page}
              totalPages={totalPages}
              onPageChange={setPage}
              disabled={loading}
              className="mt-4"
            />
          )}
        </CardContent>
      </Card>
    </div>
  )
}
