import { useState } from 'react'
import { MailWarning, RefreshCw } from 'lucide-react'
import { AdminLoadingState, AdminErrorState, AdminEmptyState, AdminPagination, PageHeader } from '@/components/common'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Select } from '@/components/ui/Select'
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

  // Pass the scalar lookback window through; the hook resolves it to a
  // timestamp at request time so this page does not create a fresh, unstable
  // value on every render (which caused an infinite refetch loop).
  const { deliveries, loading, error, total, refetch, page, setPage, totalPages } =
    useFailedDeliveries({ sinceHours })

  const handleSinceChange = (hours: number) => {
    setSinceHours(hours)
    setPage(1)
  }

  // One description for every render state, so the header never changes copy
  // depending on whether the request succeeded.
  const description = total > 0
    ? `Notification delivery failures (${total} total)`
    : 'Notification delivery failures'

  if (loading && deliveries.length === 0) {
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Failed Deliveries" description={description} />
        <AdminErrorState title="Failed to load failed deliveries" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title="Failed Deliveries"
        description={description}
        actions={
          <Button variant="secondary" onClick={refetch} className="gap-2">
            <RefreshCw className="h-4 w-4" />
            Refresh
          </Button>
        }
      />

      {/* Time filter */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center gap-4">
            <Select
              label="Since:"
              value={sinceHours}
              onChange={(e) => handleSinceChange(Number(e.target.value))}
            >
              <option value={1}>Last 1 hour</option>
              <option value={6}>Last 6 hours</option>
              <option value={24}>Last 24 hours</option>
              <option value={72}>Last 3 days</option>
              <option value={168}>Last 7 days</option>
            </Select>
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
            <AdminEmptyState
              icon={MailWarning}
              title="No Failed Deliveries"
              description="No notification delivery failures in the selected time range."
              filtered
            />
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
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
                      <TableCell className="max-w-[300px] truncate type-body">
                        {d.reason || '-'}
                      </TableCell>
                      <TableCell className="font-mono text-sm">
                        {d.notification_id.slice(0, 8)}...
                      </TableCell>
                      <TableCell className="type-secondary">
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
