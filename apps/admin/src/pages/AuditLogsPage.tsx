import { useState } from 'react'
import { FileText, Filter, ChevronDown, ChevronUp } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
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

  const { logs, loading, error, count, setPage } = useAuditLogs({
    action: actionFilter,
    target_type: targetFilter,
    page_size: 50,
  })

  const toggleMetadata = (logId: string) => {
    setExpandedMetadata(prev => ({
      ...prev,
      [logId]: !prev[logId],
    }))
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading audit logs...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Admin Activity Logs</h1>
          <p className="text-gray-600 mt-1">Track all admin actions for accountability</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading audit logs: {error.message}</p>
            </div>
          </CardContent>
        </Card>
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
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Admin Activity Logs</h1>
          <p className="text-gray-600 mt-1">Track all admin actions for accountability and compliance</p>
        </div>
      </div>

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-600">Total Logged Actions</p>
              <p className="text-3xl font-bold text-primary mt-1">{count}</p>
              <p className="text-xs text-gray-500 mt-1">Read-only audit trail</p>
            </div>
            <div className="p-4 rounded-lg bg-blue-100">
              <FileText className="h-8 w-8 text-blue-600" />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-wrap items-center gap-4">
            <Filter className="h-5 w-5 text-gray-500" />
            <label htmlFor="action-filter" className="text-sm font-medium text-gray-700">
              Action:
            </label>
            <select
              id="action-filter"
              value={actionFilter}
              onChange={(e) => {
                setActionFilter(e.target.value as AuditActionType | '')
                setPage(1)
              }}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            >
              {ACTION_FILTERS.map((filter) => (
                <option key={filter.value} value={filter.value}>
                  {filter.label}
                </option>
              ))}
            </select>

            <label htmlFor="target-filter" className="text-sm font-medium text-gray-700 ml-4">
              Target:
            </label>
            <select
              id="target-filter"
              value={targetFilter}
              onChange={(e) => {
                setTargetFilter(e.target.value as AuditTargetType | '')
                setPage(1)
              }}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            >
              {TARGET_FILTERS.map((filter) => (
                <option key={filter.value} value={filter.value}>
                  {filter.label}
                </option>
              ))}
            </select>
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
            <div className="text-center py-12">
              <FileText className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Logs Found</h3>
              <p className="text-gray-600">
                No audit logs match the current filters.
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
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
                      <TableCell className="text-sm text-gray-600 whitespace-nowrap">
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
                              className="flex items-center text-xs text-gray-500 hover:text-gray-700"
                            >
                              {expandedMetadata[log.id] ? (
                                <ChevronUp className="h-3 w-3 mr-1" />
                              ) : (
                                <ChevronDown className="h-3 w-3 mr-1" />
                              )}
                              {expandedMetadata[log.id] ? 'Hide' : 'Show'} details
                            </button>
                            {expandedMetadata[log.id] && (
                              <div className="mt-2 p-2 bg-gray-50 rounded text-xs font-mono">
                                {Object.entries(log.metadata).map(([key, value]) => (
                                  <div key={key} className="flex gap-2 py-0.5">
                                    <span className="text-gray-500">{key}:</span>
                                    <span className="text-gray-700 break-all">
                                      {formatMetadataValue(value)}
                                    </span>
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        ) : (
                          <span className="text-xs text-gray-400">No details</span>
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
