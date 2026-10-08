import { useState } from 'react'
import { AlertTriangle, Filter, X } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Select } from '@/components/ui/Select'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination, PageHeader } from '@/components/common'
import { useWarnings, useRevokeWarning } from '@/hooks/useWarnings'
import { formatDate } from '@/lib/utils'
import {
  warningLevelLabels,
  warningLevelVariants,
} from '@/types'

const ACTIVE_FILTERS: { value: boolean | null; label: string }[] = [
  { value: null, label: 'All Warnings' },
  { value: true, label: 'Active Only' },
  { value: false, label: 'Inactive' },
]

export function WarningsPage() {
  const [activeFilter, setActiveFilter] = useState<boolean | null>(true)
  const [revokingId, setRevokingId] = useState<string | null>(null)

  const { warnings, loading, error, page, setPage, limit, count, refetch } = useWarnings(
    activeFilter !== null ? { is_active: activeFilter } : {}
  )
  const { revokeWarning } = useRevokeWarning()

  // `count` is the truthful server-side total for the current filter.
  const totalPages = limit > 0 ? Math.ceil(count / limit) : 0

  const handleClearFilters = () => {
    setActiveFilter(null)
    setPage(1)
  }

  const handleRevoke = async (warningId: string) => {
    if (!confirm('Are you sure you want to revoke this warning?')) {
      return
    }

    setRevokingId(warningId)
    try {
      await revokeWarning(warningId)
      refetch()
    } catch (err) {
      console.error('Failed to revoke warning:', err)
      alert(err instanceof Error ? err.message : 'Failed to revoke warning')
    } finally {
      setRevokingId(null)
    }
  }

  if (loading && warnings.length === 0) {
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="User Warnings" description="Manage user warnings and policy violations" />
        <AdminErrorState title="Failed to load warnings" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  const activeCount = warnings.filter(w => w.is_active).length

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader title="User Warnings" description="Manage user warnings and policy violations" />

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-muted-foreground">Total Warnings</p>
              <p className="type-metric-lg text-primary mt-1">{count}</p>
              <p className="type-caption mt-1">{activeCount} currently active</p>
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
              label="Filter:"
              value={activeFilter === null ? 'null' : activeFilter.toString()}
              onChange={(e) => {
                setActiveFilter(e.target.value === 'null' ? null : e.target.value === 'true')
                setPage(1)
              }}
            >
              {ACTIVE_FILTERS.map((filter) => (
                <option key={filter.value?.toString() ?? 'null'} value={filter.value?.toString() ?? 'null'}>
                  {filter.label}
                </option>
              ))}
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* Warnings Table */}
      <Card>
        <CardHeader>
          <CardTitle>Warnings List</CardTitle>
        </CardHeader>
        <CardContent>
          {warnings.length === 0 ? (
            <AdminEmptyState
              icon={AlertTriangle}
              title="No Warnings Found"
              description={
                activeFilter === true
                  ? 'No active warnings.'
                  : activeFilter === false
                  ? 'No inactive warnings.'
                  : 'No warnings in the system.'
              }
              filtered={activeFilter !== null}
              onClearFilters={handleClearFilters}
            />
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Warning ID</TableHead>
                    <TableHead>User ID</TableHead>
                    <TableHead>Level</TableHead>
                    <TableHead>Reason</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Issued Date</TableHead>
                    <TableHead>Expires</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {warnings.map((warning) => (
                    <TableRow key={warning.id}>
                      <TableCell className="font-mono text-sm">
                        {warning.id.slice(0, 8)}
                      </TableCell>
                      <TableCell className="font-mono text-sm">
                        {warning.user_id}
                      </TableCell>
                      <TableCell>
                        <Badge variant={warningLevelVariants[warning.level]}>
                          {warningLevelLabels[warning.level]}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="max-w-xs truncate">
                          {warning.reason}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={warning.is_active ? 'success' : 'default'}>
                          {warning.is_active ? 'Active' : 'Inactive'}
                        </Badge>
                      </TableCell>
                      <TableCell className="type-secondary">
                        {formatDate(warning.created_at)}
                      </TableCell>
                      <TableCell className="type-secondary">
                        {warning.expires_at ? formatDate(warning.expires_at) : <span className="text-muted-foreground">Never</span>}
                      </TableCell>
                      <TableCell className="text-right">
                        {warning.is_active && (
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => handleRevoke(warning.id)}
                            disabled={revokingId === warning.id}
                            className="text-destructive hover:text-destructive hover:bg-destructive-bg"
                          >
                            {revokingId === warning.id ? (
                              'Revoking...'
                            ) : (
                              <>
                                <X className="h-4 w-4 mr-1" />
                                Revoke
                              </>
                            )}
                          </Button>
                        )}
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
    </div>
  )
}
