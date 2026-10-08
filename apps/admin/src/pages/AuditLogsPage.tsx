import { useState } from 'react'
import { FileText, Filter, ChevronDown, ChevronUp } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Select } from '@/components/ui/Select'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination, PageHeader } from '@/components/common'
import { useAuditLogs } from '@/hooks/useAuditLogs'
import { formatDateTime } from '@/lib/utils'
import type {
  AuditActionType,
  AuditTargetType,
} from '@/types'
import {
  auditActionLabels,
  auditTargetTypeLabels,
  auditActionVariants,
  auditTargetVariants,
} from '@/types'

// Available action types for filter (excluding view actions to reduce noise)
const ACTION_FILTERS = [
  { label: 'All Actions', value: '' },
  { label: 'Withdrawals', value: 'withdraw_approved' },
  { label: 'Withdrawals Rejected', value: 'withdraw_rejected' },
  { label: 'Dispute Approved (Buyer)', value: 'dispute_resolved_approved' },
  { label: 'Dispute Rejected (Seller)', value: 'dispute_resolved_rejected' },
  { label: 'User Suspended', value: 'user_suspended' },
  { label: 'User Banned', value: 'user_banned' },
  { label: 'User Activated', value: 'user_activated' },
  { label: 'Role Changed', value: 'role_changed' },
  { label: 'Account Status Changed', value: 'account_status_changed' },
]

// Available target types for filter
const TARGET_FILTERS = [
  { label: 'All Targets', value: '' },
  { label: 'User', value: 'user' },
  { label: 'Withdrawal', value: 'withdrawal' },
  { label: 'Dispute', value: 'dispute' },
  { label: 'Refund', value: 'refund' },
  { label: 'Auction', value: 'auction' },
]

export function AuditLogsPage() {
  const [actionFilter, setActionFilter] = useState<AuditActionType | ''>('')
  const [targetFilter, setTargetFilter] = useState<AuditTargetType | ''>('')
  const [expandedMetadata, setExpandedMetadata] = useState<Record<string, boolean>>({})

  const { logs, loading, error, page, setPage, limit, count, refetch } = useAuditLogs({
    action: actionFilter,
    target_type: targetFilter,
    page_size: 50,
  })

  // `count` is the truthful server-side total.
  const totalPages = limit > 0 ? Math.ceil(count / limit) : 0

  const toggleMetadata = (logId: string) => {
    setExpandedMetadata(prev => ({
      ...prev,
      [logId]: !prev[logId],
    }))
  }

  const hasActiveFilters = actionFilter || targetFilter

  const handleClearFilters = () => {
    setActionFilter('')
    setTargetFilter('')
    setPage(1)
  }

  if (loading && logs.length === 0) {
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Admin Activity Logs" description="Track all admin actions for accountability and compliance" />
        <AdminErrorState title="Failed to load audit logs" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  const getActionLabel = (actionType: string) => {
    return auditActionLabels[actionType as AuditActionType] || actionType.replace(/_/g, ' ')
  }

  const getTargetLabel = (targetType: string) => {
    return auditTargetTypeLabels[targetType as AuditTargetType] || targetType
  }

  const getActionVariant = (actionType: string) => {
    return auditActionVariants[actionType as AuditActionType] || 'default'
  }

  const getTargetVariant = (targetType: string) => {
    return auditTargetVariants[targetType as AuditTargetType] || 'default'
  }

  // Format metadata for display
  const formatMetadataValue = (value: unknown): string => {
    if (value === null) return 'null'
    if (value === undefined) return 'undefined'
    if (typeof value === 'string') return value
    if (typeof value === 'number') return value.toString()
    if (typeof value === 'boolean') return value ? 'true' : 'false'
    return JSON.stringify(value)
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader title="Admin Activity Logs" description="Track all admin actions for accountability and compliance" />

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-muted-foreground">Total Logged Actions</p>
              <p className="type-metric-lg text-primary mt-1">{count}</p>
              <p className="type-caption mt-1">Read-only audit trail</p>
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
          <div className="flex flex-wrap items-end gap-4">
            <Filter className="h-5 w-5 text-muted-foreground mb-2" />
            <Select
              label="Action:"
              value={actionFilter}
              onChange={(e) => {
                setActionFilter(e.target.value as AuditActionType | '')
                setPage(1)
              }}
            >
              {ACTION_FILTERS.map((filter) => (
                <option key={filter.value} value={filter.value}>
                  {filter.label}
                </option>
              ))}
            </Select>

            <Select
              label="Target:"
              value={targetFilter}
              onChange={(e) => {
                setTargetFilter(e.target.value as AuditTargetType | '')
                setPage(1)
              }}
            >
              {TARGET_FILTERS.map((filter) => (
                <option key={filter.value} value={filter.value}>
                  {filter.label}
                </option>
              ))}
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* Audit Logs Table */}
      <Card>
        <CardHeader>
          <CardTitle>Activity Log</CardTitle>
        </CardHeader>
        <CardContent>
          {logs.length === 0 ? (
            <AdminEmptyState
              icon={FileText}
              title="No Logs Found"
              description={
                hasActiveFilters
                  ? 'No audit logs match the current filters.'
                  : 'No audit logs have been recorded yet.'
              }
              filtered={Boolean(hasActiveFilters)}
              onClearFilters={handleClearFilters}
            />
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Timestamp</TableHead>
                    <TableHead>Admin ID</TableHead>
                    <TableHead>Action</TableHead>
                    <TableHead>Target</TableHead>
                    <TableHead>Target ID</TableHead>
                    <TableHead>Details</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {logs.map((log) => (
                    <TableRow key={log.id}>
                      <TableCell className="type-secondary whitespace-nowrap">
                        {formatDateTime(log.created_at)}
                      </TableCell>
                      <TableCell className="font-mono text-sm">
                        {log.actor_id.slice(0, 8)}
                      </TableCell>
                      <TableCell>
                        <Badge variant={getActionVariant(log.action_type)}>
                          {getActionLabel(log.action_type)}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={getTargetVariant(log.target_type)}>
                          {getTargetLabel(log.target_type)}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-mono text-sm">
                        {log.target_id.slice(0, 8)}
                      </TableCell>
                      <TableCell>
                        {log.metadata && Object.keys(log.metadata).length > 0 ? (
                          <div className="max-w-md">
                            <button
                              onClick={() => toggleMetadata(log.id)}
                              className="flex items-center type-caption hover:text-foreground"
                            >
                              {expandedMetadata[log.id] ? (
                                <ChevronUp className="h-3 w-3 mr-1" />
                              ) : (
                                <ChevronDown className="h-3 w-3 mr-1" />
                              )}
                              {expandedMetadata[log.id] ? 'Hide' : 'Show'} details
                            </button>
                            {expandedMetadata[log.id] && (
                              <div className="mt-2 p-2 bg-surface-muted rounded text-xs font-mono">
                                {Object.entries(log.metadata).map(([key, value]) => (
                                  <div key={key} className="flex gap-2 py-0.5">
                                    <span className="text-muted-foreground">{key}:</span>
                                    <span className="text-foreground break-all">
                                      {formatMetadataValue(value)}
                                    </span>
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        ) : (
                          <span className="type-caption">No details</span>
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
