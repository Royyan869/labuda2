import { useState } from 'react'
import { AlertTriangle, Filter, X } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
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

  const { warnings, loading, error, count, refetch } = useWarnings(
    activeFilter !== null ? { is_active: activeFilter } : {}
  )
  const { revokeWarning } = useRevokeWarning()

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

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading warnings...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">User Warnings</h1>
          <p className="text-gray-600 mt-1">Manage user warnings and policy violations</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading warnings: {error.message}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  const activeCount = warnings.filter(w => w.is_active).length

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-gray-900">User Warnings</h1>
        <p className="text-gray-600 mt-1">Manage user warnings and policy violations</p>
      </div>

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-600">Total Warnings</p>
              <p className="text-3xl font-bold text-primary mt-1">{count}</p>
              <p className="text-xs text-gray-500 mt-1">{activeCount} currently active</p>
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
            <label htmlFor="active-filter" className="text-sm font-medium text-gray-700">
              Filter:
            </label>
            <select
              id="active-filter"
              value={activeFilter === null ? 'null' : activeFilter.toString()}
              onChange={(e) => setActiveFilter(e.target.value === 'null' ? null : e.target.value === 'true')}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            >
              {ACTIVE_FILTERS.map((filter) => (
                <option key={filter.value?.toString() ?? 'null'} value={filter.value?.toString() ?? 'null'}>
                  {filter.label}
                </option>
              ))}
            </select>
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
            <div className="text-center py-12">
              <AlertTriangle className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Warnings Found</h3>
              <p className="text-gray-600">
                {activeFilter === true ? 'No active warnings.' : 'No warnings found.'}
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
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
                      <TableCell className="text-sm text-gray-600">
                        {formatDate(warning.created_at)}
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {warning.expires_at ? formatDate(warning.expires_at) : <span className="text-gray-400">Never</span>}
                      </TableCell>
                      <TableCell className="text-right">
                        {warning.is_active && (
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => handleRevoke(warning.id)}
                            disabled={revokingId === warning.id}
                            className="text-red-600 hover:text-red-700 hover:bg-red-50"
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
    </div>
  )
}
