import { RefreshCw } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { AdminLoadingState, AdminErrorState, PageHeader } from '@/components/common'
import { useSupportStats } from '@/hooks/useSupportOverview'

export function SupportOverviewPage() {
  const { stats, loading, error, refetch } = useSupportStats()

  if (loading) {
    return <AdminLoadingState />
  }

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader title="Support Overview" description="Ticket statistics across the support queue" />
        <AdminErrorState title="Failed to load support overview" message={error.message} onRetry={refetch} />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title="Support Overview"
        description="Ticket statistics across the support queue"
        actions={
          <Button variant="secondary" onClick={refetch} className="gap-2">
            <RefreshCw className="h-4 w-4" />
            Refresh
          </Button>
        }
      />

      {/* Stats Grid.
          Agent workload is deliberately NOT a separate admin pool: assignment
          authority is support_tickets.assigned_admin_id, so per-agent load is
          read from the tickets themselves (filter the ticket list by agent). */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-muted-foreground">Total</p>
              <p className="type-metric mt-1">{stats.total_tickets}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-muted-foreground">Open</p>
              <p className="type-metric text-info mt-1">{stats.open_tickets}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-muted-foreground">In Progress</p>
              <p className="type-metric text-warning mt-1">{stats.in_progress_tickets}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-muted-foreground">Waiting User</p>
              <p className="type-metric text-info mt-1">{stats.waiting_user_tickets}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-muted-foreground">Resolved</p>
              <p className="type-metric text-success mt-1">{stats.resolved_tickets}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-muted-foreground">Closed</p>
              <p className="type-metric text-muted-foreground mt-1">{stats.closed_tickets}</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <p className="text-sm font-medium text-muted-foreground">Unassigned</p>
              <p className="type-metric text-destructive mt-1">{stats.unassigned_tickets}</p>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  )
}
