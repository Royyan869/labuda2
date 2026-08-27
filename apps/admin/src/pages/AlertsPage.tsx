import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { useAlerts, useAlertStats, useAlertActions } from '@/hooks/useAlerts'
import {
  alertTypeLabels,
  alertSeverityLabels,
  alertStatusLabels,
} from '@/types/alert'
import type { AlertStatus, AlertSeverity, AlertType, Alert } from '@/types/alert'
import {
  RefreshCw,
  Bell,
  CheckCircle,
  XCircle,
  Eye,
  ShieldAlert,
  Activity,
  ChevronDown,
  ChevronRight,
  Copy,
  ExternalLink,
} from 'lucide-react'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination } from '@/components/common'

/**
 * Returns the admin navigation path for a given alert entity_type + entity_id.
 * - dispute: direct detail page (/disputes/:id)
 * - order: orders list (/orders) — no direct /orders/:id route exists
 * - seller / user: users list (/users) — no direct /users/:id route exists
 * - withdrawal: withdrawals list (/finance/withdrawals)
 * Returns null for unknown / system-level entity types.
 */
function getEntityPath(entityType: string, entityId: string): string | null {
  switch (entityType) {
    case 'dispute':
      return `/disputes/${entityId}`
    case 'order':
      return '/orders'
    case 'seller':
    case 'user':
      return '/users'
    case 'withdrawal':
      return '/finance/withdrawals'
    default:
      return null
  }
}

const severityBadgeVariant: Record<AlertSeverity, 'default' | 'info' | 'warning' | 'error'> = {
  low: 'default',
  medium: 'info',
  high: 'warning',
  critical: 'error',
  warning: 'warning',
  info: 'info',
}

const statusBadgeVariant: Record<AlertStatus, 'error' | 'warning' | 'success' | 'default'> = {
  active: 'error',
  open: 'error',
  acknowledged: 'warning',
  resolved: 'success',
  false_positive: 'default',
}

function StatCard({ label, value, icon: Icon, color }: {
  label: string
  value: number
  icon: React.ComponentType<{ className?: string }>
  color: string
}) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center gap-3">
          <div className={`p-2 rounded-lg ${color}`}>
            <Icon className="h-5 w-5 text-white" />
          </div>
          <div>
            <p className="text-2xl font-bold text-gray-900">{value}</p>
            <p className="text-xs text-gray-500">{label}</p>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export function AlertsPage() {
  const [statusFilter, setStatusFilter] = useState<AlertStatus | ''>('')
  const [severityFilter, setSeverityFilter] = useState<AlertSeverity | ''>('')
  const [typeFilter, setTypeFilter] = useState<AlertType | ''>('')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [page, setPage] = useState(1)

  const { alerts, loading, error, count, totalPages, refetch } = useAlerts({
    status: statusFilter || undefined,
    severity: severityFilter || undefined,
    alert_type: typeFilter || undefined,
    date_from: dateFrom || undefined,
    date_to: dateTo || undefined,
    page,
    page_size: 20,
  })
  const { stats, refetch: refetchStats } = useAlertStats()
  const { acknowledge, resolve, markFalsePositive, loading: actionLoading } = useAlertActions()

  const handleAction = async (action: 'acknowledge' | 'resolve' | 'false_positive', alertId: string) => {
    try {
      if (action === 'acknowledge') await acknowledge(alertId)
      else if (action === 'resolve') await resolve(alertId)
      else await markFalsePositive(alertId)
      await refetch()
      await refetchStats()
    } catch {
      // Error is surfaced by the hook
    }
  }

  const handleRefresh = () => {
    refetch()
    refetchStats()
  }

  const handleClearFilters = () => {
    setStatusFilter('')
    setSeverityFilter('')
    setTypeFilter('')
    setDateFrom('')
    setDateTo('')
    setPage(1)
  }

  const hasActiveFilters = statusFilter || severityFilter || typeFilter || dateFrom || dateTo

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">System Alerts</h1>
          <p className="text-gray-600 mt-1">Monitor and manage platform alerts</p>
        </div>
        <Button variant="ghost" size="sm" onClick={handleRefresh} disabled={loading}>
          <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Stats Cards */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <StatCard label="Active" value={stats.active} icon={ShieldAlert} color="bg-red-500" />
          <StatCard label="Acknowledged" value={stats.acknowledged} icon={Eye} color="bg-amber-500" />
          <StatCard label="Resolved" value={stats.resolved} icon={CheckCircle} color="bg-green-500" />
          <StatCard label="Total" value={stats.total} icon={Activity} color="bg-blue-500" />
        </div>
      )}

      {/* Filters */}
      <Card>
        <CardContent className="p-4">
          <div className="flex flex-wrap items-end gap-4">
            <div>
              <label className="text-xs font-medium text-gray-600 block mb-1">Status</label>
              <select
                className="border border-gray-300 rounded-md px-3 py-1.5 text-sm"
                value={statusFilter}
                onChange={(e) => { setStatusFilter(e.target.value as AlertStatus | ''); setPage(1) }}
              >
                <option value="">All</option>
                <option value="active">Active</option>
                <option value="open">Open</option>
                <option value="acknowledged">Acknowledged</option>
                <option value="resolved">Resolved</option>
                <option value="false_positive">False Positive</option>
              </select>
            </div>
            <div>
              <label className="text-xs font-medium text-gray-600 block mb-1">Severity</label>
              <select
                className="border border-gray-300 rounded-md px-3 py-1.5 text-sm"
                value={severityFilter}
                onChange={(e) => { setSeverityFilter(e.target.value as AlertSeverity | ''); setPage(1) }}
              >
                <option value="">All</option>
                <option value="critical">Critical</option>
                <option value="high">High</option>
                <option value="warning">Warning</option>
                <option value="medium">Medium</option>
                <option value="info">Info</option>
                <option value="low">Low</option>
              </select>
            </div>
            <div>
              <label className="text-xs font-medium text-gray-600 block mb-1">Type</label>
              <select
                className="border border-gray-300 rounded-md px-3 py-1.5 text-sm"
                value={typeFilter}
                onChange={(e) => { setTypeFilter(e.target.value as AlertType | ''); setPage(1) }}
              >
                <option value="">All</option>
                {Object.entries(alertTypeLabels).map(([value, label]) => (
                  <option key={value} value={value}>{label}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-xs font-medium text-gray-600 block mb-1">From</label>
              <input
                type="date"
                className="border border-gray-300 rounded-md px-3 py-1.5 text-sm"
                value={dateFrom}
                onChange={(e) => { setDateFrom(e.target.value); setPage(1) }}
              />
            </div>
            <div>
              <label className="text-xs font-medium text-gray-600 block mb-1">To</label>
              <input
                type="date"
                className="border border-gray-300 rounded-md px-3 py-1.5 text-sm"
                value={dateTo}
                onChange={(e) => { setDateTo(e.target.value); setPage(1) }}
              />
            </div>
            {hasActiveFilters && (
              <Button variant="ghost" size="sm" onClick={handleClearFilters}>
                Clear filters
              </Button>
            )}
            <div className="ml-auto text-sm text-gray-500 self-end pb-1.5">
              {count} alert{count !== 1 ? 's' : ''}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Error State */}
      {error && (
        <AdminErrorState
          title="Failed to load alerts"
          message={error.message}
          onRetry={handleRefresh}
        />
      )}

      {/* Loading State */}
      {loading && alerts.length === 0 && !error && <AdminLoadingState />}

      {/* Empty State */}
      {!loading && !error && alerts.length === 0 && (
        <AdminEmptyState
          icon={Bell}
          title="No Alerts"
          description="No alerts match the current filters."
        />
      )}

      {/* Alert List */}
      {alerts.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Alerts</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <div className="divide-y divide-gray-200">
              {alerts.map((alert) => (
                <AlertRow
                  key={alert.id}
                  alert={alert}
                  onAction={handleAction}
                  actionLoading={actionLoading}
                />
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <AdminPagination
          page={page}
          totalPages={totalPages}
          onPageChange={setPage}
        />
      )}
    </div>
  )
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text).catch(() => {})
}

function AlertRow({ alert, onAction, actionLoading }: {
  alert: Alert
  onAction: (action: 'acknowledge' | 'resolve' | 'false_positive', id: string) => void
  actionLoading: boolean
}) {
  const [metaExpanded, setMetaExpanded] = useState(false)
  const navigate = useNavigate()

  const canAcknowledge = alert.status === 'active' || alert.status === 'open'
  const canResolve = alert.status === 'active' || alert.status === 'open' || alert.status === 'acknowledged'
  const canMarkFalse = alert.status === 'active' || alert.status === 'open' || alert.status === 'acknowledged'
  const hasMetadata = alert.metadata && Object.keys(alert.metadata).length > 0
  const isTerminal = alert.status === 'resolved' || alert.status === 'false_positive'

  return (
    <div className="px-6 py-4">
      <div className="flex items-start gap-4">
        {/* Main content */}
        <div className="flex flex-col gap-1.5 min-w-0 flex-1">
          {/* Badges + type */}
          <div className="flex items-center gap-2 flex-wrap">
            <Badge variant={severityBadgeVariant[alert.severity] ?? 'default'}>
              {alertSeverityLabels[alert.severity] ?? alert.severity}
            </Badge>
            <Badge variant={statusBadgeVariant[alert.status]}>
              {alertStatusLabels[alert.status]}
            </Badge>
            <span className="text-xs text-gray-500">
              {alertTypeLabels[alert.alert_type] ?? alert.alert_type}
            </span>
          </div>

          {/* Message */}
          <p className="text-sm text-gray-900 font-medium">{alert.message}</p>

          {/* Meta row: created_at, entity, group_key */}
          <div className="flex flex-wrap items-center gap-3 text-xs text-gray-500">
            <span title={alert.created_at}>
              Created: {new Date(alert.created_at).toLocaleString()}
            </span>
            {alert.entity_type && alert.entity_type !== 'system' && (() => {
              const path = getEntityPath(alert.entity_type, alert.entity_id)
              return (
                <span className="flex items-center gap-1">
                  {alert.entity_type}:
                  <code className="font-mono bg-gray-100 px-1 rounded text-gray-700">
                    {alert.entity_id.slice(0, 8)}…
                  </code>
                  <button
                    onClick={() => copyToClipboard(alert.entity_id)}
                    title={`Copy ${alert.entity_type} ID: ${alert.entity_id}`}
                    className="text-gray-400 hover:text-gray-600"
                  >
                    <Copy className="h-3 w-3" />
                  </button>
                  {path && (
                    <button
                      onClick={() => navigate(path)}
                      title={`Navigate to ${alert.entity_type}${alert.entity_type === 'dispute' ? '' : ' list (copy ID to filter)'}`}
                      className="text-blue-500 hover:text-blue-700"
                    >
                      <ExternalLink className="h-3 w-3" />
                    </button>
                  )}
                </span>
              )
            })()}
            {alert.group_key && (
              <span>Group: {alert.group_key}</span>
            )}
          </div>

          {/* Resolution info */}
          {isTerminal && alert.resolved_at && (
            <div className="text-xs text-gray-500">
              {alert.status === 'resolved' ? 'Resolved' : 'Closed'}:{' '}
              {new Date(alert.resolved_at).toLocaleString()}
              {alert.resolved_by && (
                <span className="ml-2 font-mono bg-gray-100 px-1 rounded">
                  by {alert.resolved_by.slice(0, 8)}…
                </span>
              )}
            </div>
          )}

          {/* Metadata expand */}
          {hasMetadata && (
            <div>
              <button
                onClick={() => setMetaExpanded((v) => !v)}
                className="flex items-center gap-1 text-xs text-blue-600 hover:text-blue-800"
              >
                {metaExpanded
                  ? <ChevronDown className="h-3 w-3" />
                  : <ChevronRight className="h-3 w-3" />
                }
                {metaExpanded ? 'Hide' : 'Show'} metadata
              </button>
              {metaExpanded && (
                <pre className="mt-2 text-xs bg-gray-50 border border-gray-200 rounded p-2 overflow-auto max-h-48 text-gray-700">
                  {JSON.stringify(alert.metadata, null, 2)}
                </pre>
              )}
            </div>
          )}
        </div>

        {/* Actions */}
        <div className="flex items-center gap-2 shrink-0">
          {canAcknowledge && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onAction('acknowledge', alert.id)}
              disabled={actionLoading}
              title="Acknowledge"
            >
              <Eye className="h-4 w-4" />
            </Button>
          )}
          {canResolve && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onAction('resolve', alert.id)}
              disabled={actionLoading}
              title="Resolve"
            >
              <CheckCircle className="h-4 w-4" />
            </Button>
          )}
          {canMarkFalse && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onAction('false_positive', alert.id)}
              disabled={actionLoading}
              title="Mark False Positive"
            >
              <XCircle className="h-4 w-4" />
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}
