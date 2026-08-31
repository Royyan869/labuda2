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

  const handleViewCase = (caseId: string) => {
    navigate(`/moderation/cases/${caseId}`)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading governance cases...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Governance Cases</h1>
          <p className="text-gray-600 mt-1">Review and decide on reported subjects</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p className="font-medium">Error loading cases</p>
              <p className="text-sm mt-1">{error.message}</p>
              <Button
                variant="secondary"
                className="mt-4"
                onClick={refetch}
              >
                Retry
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-gray-900">Governance Cases</h1>
        <p className="text-gray-600 mt-1">Review and decide on reported subjects</p>
      </div>

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-600">Total Cases</p>
              <p className="text-3xl font-bold text-primary mt-1">{count}</p>
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
          <div className="flex items-center gap-4">
            <Filter className="h-5 w-5 text-gray-500" />
            <label htmlFor="status-filter" className="text-sm font-medium text-gray-700">
              Status:
            </label>
            <select
              id="status-filter"
              value={statusFilter}
              onChange={(e) => {
                setStatusFilter(e.target.value as GovernanceCaseStatus | '')
                setPage(1)
              }}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            >
              {CASE_FILTERS.map((f) => (
                <option key={f.value} value={f.value}>
                  {f.label}
                </option>
              ))}
            </select>
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
            <div className="text-center py-12">
              <Shield className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Cases Found</h3>
              <p className="text-gray-600">
                {statusFilter ? `No ${statusFilter} cases found.` : 'No governance cases yet.'}
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
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
                      <TableCell className="text-sm text-gray-600">
                        {formatDate(caseItem.created_at)}
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
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
          {count > 20 && (
            <div className="flex items-center justify-between mt-4">
              <p className="text-sm text-gray-600">
                Showing {(page - 1) * 20 + 1}–{Math.min(page * 20, count)} of {count}
              </p>
              <div className="flex gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={page <= 1}
                  onClick={() => setPage(p => p - 1)}
                >
                  Previous
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={page * 20 >= count}
                  onClick={() => setPage(p => p + 1)}
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
