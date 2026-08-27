import { useState } from 'react'
import { ChevronLeft, ChevronRight, MailWarning, RefreshCw } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { useFailedDeliveries } from '@/hooks/useFailedDeliveries'
import { formatDate } from '@/lib/utils'

const CHANNEL_VARIANTS: Record<string, 'info' | 'warning' | 'error' | 'success'> = {
  push: 'info',
  push_retry: 'warning',
  email: 'success',
  in_app: 'info',
}

export function FailedDeliveriesPage() {
  const [sinceHours, setSinceHours] = useState<number>(24)

  const since = (() => {
    const date = new Date()
    date.setHours(date.getHours() - sinceHours)
    return date.toISOString()
  })()

  const { deliveries, loading, error, total, refetch, page, setPage, totalPages } =
    useFailedDeliveries({ since })

  const handleSinceChange = (hours: number) => {
    setSinceHours(hours)
    setPage(1)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading failed deliveries...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Failed Deliveries</h1>
          <p className="text-gray-600 mt-1">Notification delivery failures</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error loading failed deliveries: {error.message}</p>
              <Button variant="secondary" onClick={refetch} className="mt-4 gap-2">
                <RefreshCw className="h-4 w-4" />
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
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Failed Deliveries</h1>
          <p className="text-gray-600 mt-1">Notification delivery failures ({total} total)</p>
        </div>
        <Button variant="secondary" onClick={refetch} className="gap-2">
          <RefreshCw className="h-4 w-4" />
          Refresh
        </Button>
      </div>

      {/* Time filter */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center gap-4">
            <label htmlFor="since-filter" className="text-sm font-medium text-gray-700">
              Since:
            </label>
            <select
              id="since-filter"
              value={sinceHours}
              onChange={(e) => handleSinceChange(Number(e.target.value))}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            >
              <option value={1}>Last 1 hour</option>
              <option value={6}>Last 6 hours</option>
              <option value={24}>Last 24 hours</option>
              <option value={72}>Last 3 days</option>
              <option value={168}>Last 7 days</option>
            </select>
          </div>
        </CardContent>
      </Card>

      {/* Table */}
      <Card>
        <CardHeader>
          <CardTitle>Delivery Failures</CardTitle>
        </CardHeader>
        <CardContent>
          {deliveries.length === 0 ? (
            <div className="text-center py-12">
              <MailWarning className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Failed Deliveries</h3>
              <p className="text-gray-600">
                No notification delivery failures in the selected time range.
              </p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Recipient</TableHead>
                    <TableHead>Channel</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Reason</TableHead>
                    <TableHead>Notification ID</TableHead>
                    <TableHead>Failed At</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {deliveries.map((d) => (
                    <TableRow key={d.id}>
                      <TableCell className="font-mono text-sm">
                        {d.recipient_id.slice(0, 8)}...
                      </TableCell>
                      <TableCell>
                        <Badge variant={CHANNEL_VARIANTS[d.channel] || 'info'}>
                          {d.channel}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant="error">{d.status}</Badge>
                      </TableCell>
                      <TableCell className="max-w-[300px] truncate text-sm text-gray-700">
                        {d.reason || '-'}
                      </TableCell>
                      <TableCell className="font-mono text-sm">
                        {d.notification_id.slice(0, 8)}...
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {formatDate(d.created_at)}
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
        <div className="flex items-center justify-center gap-4">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setPage(p => p - 1)}
            disabled={page <= 1}
          >
            <ChevronLeft className="h-4 w-4 mr-1" />
            Previous
          </Button>
          <span className="text-sm text-gray-600 font-medium">
            Page {page} of {totalPages}
          </span>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setPage(p => p + 1)}
            disabled={page >= totalPages}
          >
            Next
            <ChevronRight className="h-4 w-4 ml-1" />
          </Button>
        </div>
      )}
    </div>
  )
}
