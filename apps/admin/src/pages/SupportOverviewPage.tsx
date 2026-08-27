import { RefreshCw, Users, Inbox } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { useSupportStats, useSupportAdmins } from '@/hooks/useSupportOverview'
import { formatDate } from '@/lib/utils'

export function SupportOverviewPage() {
  const { stats, loading: statsLoading, error: statsError, refetch: refetchStats } = useSupportStats()
  const { admins, loading: adminsLoading, error: adminsError, refetch: refetchAdmins } = useSupportAdmins()

  const loading = statsLoading || adminsLoading
  const error = statsError || adminsError

  const refetch = () => {
    refetchStats()
    refetchAdmins()
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent"></div>
          <p className="mt-4 text-gray-600">Loading support overview...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Support Overview</h1>
          <p className="text-gray-600 mt-1">Ticket statistics and admin workload</p>
        </div>
        <Card>
          <CardContent className="p-6">
            <div className="text-center text-red-600">
              <p>Error: {error.message}</p>
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
          <h1 className="text-3xl font-bold text-gray-900">Support Overview</h1>
          <p className="text-gray-600 mt-1">Ticket statistics and admin workload</p>
        </div>
        <Button variant="secondary" onClick={refetch} className="gap-2">
          <RefreshCw className="h-4 w-4" />
          Refresh
        </Button>
      </div>

      {/* Stats Grid */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-gray-600">Total</p>
              <p className="text-2xl font-bold mt-1">{stats.total_tickets}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-gray-600">Open</p>
              <p className="text-2xl font-bold text-blue-600 mt-1">{stats.open_tickets}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-gray-600">In Progress</p>
              <p className="text-2xl font-bold text-amber-600 mt-1">{stats.in_progress_tickets}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-gray-600">Waiting User</p>
              <p className="text-2xl font-bold text-purple-600 mt-1">{stats.waiting_user_tickets}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-gray-600">Resolved</p>
              <p className="text-2xl font-bold text-green-600 mt-1">{stats.resolved_tickets}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-gray-600">Closed</p>
              <p className="text-2xl font-bold text-gray-600 mt-1">{stats.closed_tickets}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-gray-600">Unassigned</p>
              <p className="text-2xl font-bold text-red-600 mt-1">{stats.unassigned_tickets}</p>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Admin Workload */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Users className="h-5 w-5" />
            Support Admins
          </CardTitle>
        </CardHeader>
        <CardContent>
          {admins.length === 0 ? (
            <div className="text-center py-12">
              <Inbox className="h-12 w-12 text-gray-400 mx-auto mb-4" />
              <h3 className="text-lg font-semibold text-gray-900 mb-2">No Support Admins</h3>
              <p className="text-gray-600">No support admin records found.</p>
            </div>
          ) : (
            <div className="border border-gray-200 rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Admin ID</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Active Tickets</TableHead>
                    <TableHead>Last Assigned</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {admins.map((admin) => (
                    <TableRow key={admin.id}>
                      <TableCell className="font-mono text-sm">
                        {admin.id.slice(0, 8)}...
                      </TableCell>
                      <TableCell>
                        <Badge variant={admin.is_active ? 'success' : 'default'}>
                          {admin.is_active ? 'Active' : 'Inactive'}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <span className={admin.active_ticket_count > 5 ? 'font-bold text-amber-600' : ''}>
                          {admin.active_ticket_count}
                        </span>
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {admin.last_assigned_at ? formatDate(admin.last_assigned_at) : '-'}
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
