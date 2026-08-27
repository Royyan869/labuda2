import { Clock, AlertTriangle, Users, TrendingUp, TrendingDown, RefreshCw, Activity, CheckCircle, XCircle } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { useSLAMetrics } from '@/hooks/useSLA'
import { formatDate } from '@/lib/utils'

/**
 * Format milliseconds to human-readable duration
 */
function formatDuration(ms: number | null): string {
  if (ms === null) return 'N/A'

  const seconds = Math.floor(ms / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (days > 0) {
    return `${days}d ${hours % 24}h`
  }
  if (hours > 0) {
    return `${hours}h ${minutes % 60}m`
  }
  if (minutes > 0) {
    return `${minutes}m`
  }
  return `${seconds}s`
}

/**
 * Format rate as percentage
 */
function formatRate(rate: number): string {
  return `${(rate * 100).toFixed(1)}%`
}

/**
 * Get health badge variant and icon
 */
function getHealthInfo(status: 'good' | 'warning' | 'critical'): { variant: 'success' | 'warning' | 'error'; icon: typeof CheckCircle; color: string } {
  switch (status) {
    case 'good':
      return { variant: 'success', icon: CheckCircle, color: 'text-green-600' }
    case 'warning':
      return { variant: 'warning', icon: AlertTriangle, color: 'text-yellow-600' }
    case 'critical':
      return { variant: 'error', icon: XCircle, color: 'text-red-600' }
  }
}

/**
 * Get trend icon and color
 */
function getTrendInfo(change: number): { icon: typeof TrendingUp | typeof TrendingDown; color: string } {
  if (change > 0) {
    return { icon: TrendingUp, color: 'text-red-600' } // Worsening
  }
  if (change < 0) {
    return { icon: TrendingDown, color: 'text-green-600' } // Improving
  }
  return { icon: TrendingUp, color: 'text-gray-600' } // Neutral
}

export function SLADashboardPage() {
  const { metrics, loading, error, refetch } = useSLAMetrics()

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading SLA metrics...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">SLA Analytics</h1>
          <p className="text-gray-600 mt-1">Service Level Agreement performance metrics</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading SLA metrics: {error.message}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  const supportData = metrics?.support
  const disputeData = metrics?.dispute
  const systemHealth = metrics?.system_health

  return (
    <div className="space-y-6">
      {/* Header with System Health */}
      <div className="flex items-center justify-between">
        <div className="flex-1">
          <h1 className="text-3xl font-bold text-gray-900">SLA Analytics</h1>
          <p className="text-gray-600 mt-1">Service Level Agreement performance metrics</p>
        </div>
        <div className="flex items-center gap-4">
          {/* System Health Badge */}
          {systemHealth && (
            <div className="flex items-center gap-2">
              <Badge variant={systemHealth.status === 'good' ? 'success' : systemHealth.status === 'warning' ? 'warning' : 'error'}>
                {systemHealth.status.toUpperCase()}
              </Badge>
              <span className="text-sm text-gray-600">Score: {systemHealth.score.toFixed(0)}/100</span>
            </div>
          )}
          <Button onClick={refetch} variant="secondary" size="sm">
            <RefreshCw className="h-4 w-4 mr-2" />
            Refresh
          </Button>
        </div>
      </div>

      {/* System Health Alert */}
      {systemHealth && systemHealth.issues.length > 0 && (
        <Card className={systemHealth.status === 'critical' ? 'border-red-500 bg-red-50' : systemHealth.status === 'warning' ? 'border-yellow-500 bg-yellow-50' : 'border-green-500 bg-green-50'}>
          <CardContent className="p-4">
            <div className="flex items-start gap-3">
              <AlertTriangle className={`h-5 w-5 mt-0.5 ${systemHealth.status === 'critical' ? 'text-red-600' : systemHealth.status === 'warning' ? 'text-yellow-600' : 'text-green-600'}`} />
              <div className="flex-1">
                <h3 className={`font-semibold ${systemHealth.status === 'critical' ? 'text-red-900' : systemHealth.status === 'warning' ? 'text-yellow-900' : 'text-green-900'}`}>
                  System Health: {systemHealth.status.toUpperCase()}
                </h3>
                <ul className={`mt-2 text-sm ${systemHealth.status === 'critical' ? 'text-red-800' : systemHealth.status === 'warning' ? 'text-yellow-800' : 'text-green-800'}`}>
                  {systemHealth.issues.map((issue, index) => (
                    <li key={index}>• {issue}</li>
                  ))}
                </ul>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Summary Cards with Health Indicators */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Support Summary */}
        <Card className={supportData?.health_status === 'critical' ? 'border-red-500' : supportData?.health_status === 'warning' ? 'border-yellow-500' : ''}>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium text-gray-600 flex items-center justify-between">
              <div className="flex items-center">
                <Users className="h-4 w-4 mr-2" />
                Support Tickets
              </div>
              {supportData && (
                <Badge variant={supportData.health_status === 'good' ? 'success' : supportData.health_status === 'warning' ? 'warning' : 'error'}>
                  {supportData.health_status}
                </Badge>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <div className="flex items-baseline justify-between">
                <span className="text-2xl font-bold">{supportData?.total_count || 0}</span>
                <span className="text-sm text-gray-600">Total</span>
              </div>
              <div className="flex items-center justify-between text-sm">
                <div className="flex items-center gap-2">
                  <Activity className="h-3 w-3 text-blue-500" />
                  <span>{supportData?.active_count || 0} active</span>
                </div>
                <div className="flex items-center gap-2">
                  <Clock className="h-3 w-3 text-gray-500" />
                  <span>Avg: {formatDuration(supportData?.avg_first_response_time || null)}</span>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Support Overdue (Primary Signal) */}
        <Card className={(supportData?.overdue_count || 0) > 0 ? 'border-red-500 bg-red-50' : 'border-green-500 bg-green-50'}>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium text-gray-600 flex items-center">
              <AlertTriangle className="h-4 w-4 mr-2" />
              Support Overdue
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <div className="flex items-baseline justify-between">
                <span className="text-2xl font-bold text-red-600">{supportData?.overdue_count || 0}</span>
                <span className="text-sm text-gray-600">of {supportData?.total_count || 0}</span>
              </div>
              <div className="text-sm">
                <span className="font-semibold text-red-600">{formatRate(supportData?.overdue_rate || 0)}</span>
                <span className="text-gray-600"> overdue rate</span>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Dispute Summary */}
        <Card className={disputeData?.health_status === 'critical' ? 'border-red-500' : disputeData?.health_status === 'warning' ? 'border-yellow-500' : ''}>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium text-gray-600 flex items-center justify-between">
              <div className="flex items-center">
                <TrendingUp className="h-4 w-4 mr-2" />
                Disputes
              </div>
              {disputeData && (
                <Badge variant={disputeData.health_status === 'good' ? 'success' : disputeData.health_status === 'warning' ? 'warning' : 'error'}>
                  {disputeData.health_status}
                </Badge>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <div className="flex items-baseline justify-between">
                <span className="text-2xl font-bold">{disputeData?.total_count || 0}</span>
                <span className="text-sm text-gray-600">Total</span>
              </div>
              <div className="flex items-center justify-between text-sm">
                <div className="flex items-center gap-2">
                  <Activity className="h-3 w-3 text-blue-500" />
                  <span>{disputeData?.active_count || 0} active</span>
                </div>
                <div className="flex items-center gap-2">
                  <Clock className="h-3 w-3 text-gray-500" />
                  <span>Avg: {formatDuration(disputeData?.avg_resolution_time || null)}</span>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Dispute Overdue (Primary Signal) */}
        <Card className={(disputeData?.overdue_count || 0) > 0 ? 'border-red-500 bg-red-50' : 'border-green-500 bg-green-50'}>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium text-gray-600 flex items-center">
              <AlertTriangle className="h-4 w-4 mr-2" />
              Disputes Overdue
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <div className="flex items-baseline justify-between">
                <span className="text-2xl font-bold text-red-600">{disputeData?.overdue_count || 0}</span>
                <span className="text-sm text-gray-600">of {disputeData?.total_count || 0}</span>
              </div>
              <div className="text-sm">
                <span className="font-semibold text-red-600">{formatRate(disputeData?.overdue_rate || 0)}</span>
                <span className="text-gray-600"> overdue rate</span>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Trend Analysis */}
      {metrics?.trends && (
        <Card>
          <CardHeader>
            <CardTitle>24-Hour Trend Analysis</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              {/* Response Time Trend */}
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm text-gray-600 mb-1">Response Time</p>
                  <p className="text-lg font-semibold">
                    {formatDuration(metrics.trends.last_24_hours?.avg_first_response_time || null)}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  {(() => {
                    const { icon: Icon, color } = getTrendInfo(metrics.trends.response_time_change)
                    return (
                      <>
                        <Icon className={`h-5 w-5 ${color}`} />
                        <span className={`text-sm font-semibold ${color}`}>
                          {Math.abs(metrics.trends.response_time_change).toFixed(1)}%
                        </span>
                      </>
                    )
                  })()}
                </div>
              </div>

              {/* Resolution Time Trend */}
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm text-gray-600 mb-1">Resolution Time</p>
                  <p className="text-lg font-semibold">
                    {formatDuration(metrics.trends.last_24_hours?.avg_resolution_time || null)}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  {(() => {
                    const { icon: Icon, color } = getTrendInfo(metrics.trends.resolution_time_change)
                    return (
                      <>
                        <Icon className={`h-5 w-5 ${color}`} />
                        <span className={`text-sm font-semibold ${color}`}>
                          {Math.abs(metrics.trends.resolution_time_change).toFixed(1)}%
                        </span>
                      </>
                    )
                  })()}
                </div>
              </div>

              {/* Overdue Rate Trend */}
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm text-gray-600 mb-1">Overdue Rate</p>
                  <p className="text-lg font-semibold">
                    {formatRate(metrics.trends.last_24_hours?.overdue_rate || 0)}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  {(() => {
                    const { icon: Icon, color } = getTrendInfo(metrics.trends.overdue_rate_change)
                    return (
                      <>
                        <Icon className={`h-5 w-5 ${color}`} />
                        <span className={`text-sm font-semibold ${color}`}>
                          {Math.abs(metrics.trends.overdue_rate_change).toFixed(1)}%
                        </span>
                      </>
                    )
                  })()}
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Enhanced Performance Metrics */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Support Metrics with P95 */}
        <Card>
          <CardHeader>
            <CardTitle>Support Performance</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-xs text-gray-600 mb-1">Avg First Response</p>
                  <p className="text-lg font-semibold">
                    {formatDuration(supportData?.avg_first_response_time || null)}
                  </p>
                  {supportData?.p95_first_response_time && (
                    <p className="text-xs text-gray-500">P95: {formatDuration(supportData.p95_first_response_time)}</p>
                  )}
                </div>
                <div>
                  <p className="text-xs text-gray-600 mb-1">Avg Resolution</p>
                  <p className="text-lg font-semibold">
                    {formatDuration(supportData?.avg_resolution_time || null)}
                  </p>
                  {supportData?.p95_resolution_time && (
                    <p className="text-xs text-gray-500">P95: {formatDuration(supportData.p95_resolution_time)}</p>
                  )}
                </div>
              </div>
              <div className="pt-4 border-t">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-gray-600">Overdue Rate</span>
                  <Badge variant={supportData?.health_status === 'good' ? 'success' : supportData?.health_status === 'warning' ? 'warning' : 'error'}>
                    {formatRate(supportData?.overdue_rate || 0)}
                  </Badge>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Dispute Metrics with P95 */}
        <Card>
          <CardHeader>
            <CardTitle>Dispute Performance</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-xs text-gray-600 mb-1">Avg First Response</p>
                  <p className="text-lg font-semibold">
                    {formatDuration(disputeData?.avg_first_response_time || null)}
                  </p>
                  {disputeData?.p95_first_response_time && (
                    <p className="text-xs text-gray-500">P95: {formatDuration(disputeData.p95_first_response_time)}</p>
                  )}
                </div>
                <div>
                  <p className="text-xs text-gray-600 mb-1">Avg Resolution</p>
                  <p className="text-lg font-semibold">
                    {formatDuration(disputeData?.avg_resolution_time || null)}
                  </p>
                  {disputeData?.p95_resolution_time && (
                    <p className="text-xs text-gray-500">P95: {formatDuration(disputeData.p95_resolution_time)}</p>
                  )}
                </div>
              </div>
              <div className="pt-4 border-t">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-gray-600">Overdue Rate</span>
                  <Badge variant={disputeData?.health_status === 'good' ? 'success' : disputeData?.health_status === 'warning' ? 'warning' : 'error'}>
                    {formatRate(disputeData?.overdue_rate || 0)}
                  </Badge>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Enhanced Admin Performance Table */}
      <Card>
        <CardHeader>
          <CardTitle>Admin Performance</CardTitle>
        </CardHeader>
        <CardContent>
          {metrics?.admin_performance && metrics.admin_performance.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Admin ID</TableHead>
                  <TableHead className="text-right">Tickets</TableHead>
                  <TableHead className="text-right">Active</TableHead>
                  <TableHead className="text-right">Avg Response</TableHead>
                  <TableHead className="text-right">P95 Response</TableHead>
                  <TableHead className="text-right">Avg Resolution</TableHead>
                  <TableHead className="text-right">P95 Resolution</TableHead>
                  <TableHead className="text-right">Overdue</TableHead>
                  <TableHead className="text-right">Health</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {metrics.admin_performance.map((admin) => {
                  const { icon: HealthIcon, color } = getHealthInfo(admin.health_status)
                  return (
                    <TableRow key={admin.admin_id}>
                      <TableCell className="font-mono text-sm">
                        {admin.admin_id.slice(0, 8)}...
                      </TableCell>
                      <TableCell className="text-right">{admin.handled_tickets}</TableCell>
                      <TableCell className="text-right">
                        <Badge variant={admin.active_workload > 10 ? 'error' : admin.active_workload > 5 ? 'warning' : 'success'}>
                          {admin.active_workload}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        {formatDuration(admin.avg_response_time)}
                      </TableCell>
                      <TableCell className="text-right">
                        {formatDuration(admin.p95_response_time)}
                      </TableCell>
                      <TableCell className="text-right">
                        {formatDuration(admin.avg_resolution_time)}
                      </TableCell>
                      <TableCell className="text-right">
                        {formatDuration(admin.p95_resolution_time)}
                      </TableCell>
                      <TableCell className="text-right">
                        <Badge variant={admin.overdue_count > 0 ? 'error' : 'success'}>
                          {admin.overdue_count} ({formatRate(admin.overdue_rate)})
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex items-center justify-end gap-1">
                          <HealthIcon className={`h-4 w-4 ${color}`} />
                          <span className="text-xs">{admin.health_status}</span>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          ) : (
            <div className="text-center py-8 text-gray-500">
              No admin performance data available
            </div>
          )}
        </CardContent>
      </Card>

      {/* Last Updated */}
      <div className="text-sm text-gray-600 text-right">
        Last updated: {metrics?.generated_at ? formatDate(new Date(metrics.generated_at)) : 'N/A'}
      </div>
    </div>
  )
}
