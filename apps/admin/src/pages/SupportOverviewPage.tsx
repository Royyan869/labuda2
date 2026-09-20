import { RefreshCw } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { useSupportStats } from '@/hooks/useSupportOverview'

export function SupportOverviewPage() {
  const { stats, loading, error, refetch } = useSupportStats()

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
          <p className="text-gray-600 mt-1">Ticket statistics across the support queue</p>
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
          <p className="text-gray-600 mt-1">Ticket statistics across the support queue</p>
        </div>
        <Button variant="secondary" onClick={refetch} className="gap-2">
          <RefreshCw className="h-4 w-4" />
          Refresh
        </Button>
      </div>

      {/* Stats Grid.
          Agent workload is deliberately NOT a separate admin pool: assignment
          authority is support_tickets.assigned_admin_id, so per-agent load is
          read from the tickets themselves (filter the ticket list by agent). */}
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
    </div>
  )
}
