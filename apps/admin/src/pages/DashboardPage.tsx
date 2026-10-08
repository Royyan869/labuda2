import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { api } from '@/lib/api'
import {
  LayoutDashboard,
  Users,
  ShoppingCart,
  ShieldCheck,
  Headphones,
  TrendingUp,
  RefreshCw,
} from 'lucide-react'
import { AdminLoadingState, AdminErrorState, PageHeader } from '@/components/common'

interface DashboardSummary {
  total_users: number
  active_users_today: number
  active_sellers: number
  total_orders: number
  orders_today: number
  pending_reports: number
  total_revenue: number
}

interface DashboardResponse {
  success: boolean
  data: {
    data: {
      summary: DashboardSummary
      generated_at: string
    }
  }
  timestamp: string
}

function MetricCard({
  title,
  value,
  icon: Icon,
  color,
}: {
  title: string
  value: number | string
  icon: React.ComponentType<{ className?: string }>
  color: string
}) {
  return (
    <Card>
      <CardContent className="p-6">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium text-muted-foreground">{title}</p>
            <p className="type-metric text-foreground mt-1">{value}</p>
          </div>
          <div className={`p-3 rounded-full ${color}`}>
            <Icon className="h-6 w-6 text-white" />
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export function DashboardPage() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null)
  const [generatedAt, setGeneratedAt] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchDashboard = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await api.get<DashboardResponse>('/api/v1/admin/dashboard')
      const metrics = response?.data?.data
      if (metrics?.summary) {
        setSummary(metrics.summary)
        setGeneratedAt(metrics.generated_at)
      } else {
        setError('Unexpected response shape from dashboard endpoint')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch dashboard metrics')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchDashboard()
  }, [fetchDashboard])

  if (loading && !summary) {
    return (
      <div className="space-y-6">
        <PageHeader title="Dashboard Overview" description="Welcome to LABUDA Admin Dashboard" />
        <AdminLoadingState label="Loading dashboard" />
      </div>
    )
  }

  if (error && !summary) {
    return (
      <div className="space-y-6">
        <PageHeader title="Dashboard Overview" description="Welcome to LABUDA Admin Dashboard" />
        <AdminErrorState
          title="Failed to Load Dashboard"
          message={error}
          onRetry={fetchDashboard}
        />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <PageHeader
        title="Dashboard Overview"
        description="Welcome to LABUDA Admin Dashboard"
        actions={
          <>
            {generatedAt && (
              <span className="type-caption">
                Updated: {new Date(generatedAt).toLocaleTimeString()}
              </span>
            )}
            <Button variant="ghost" size="sm" onClick={fetchDashboard} disabled={loading}>
              <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </Button>
          </>
        }
      />

      {/* Metric Cards */}
      {summary && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <MetricCard
            title="Total Users"
            value={summary.total_users.toLocaleString()}
            icon={Users}
            color="bg-chart-1"
          />
          <MetricCard
            title="Active Users Today"
            value={summary.active_users_today.toLocaleString()}
            icon={TrendingUp}
            color="bg-chart-2"
          />
          <MetricCard
            title="Active Sellers"
            value={summary.active_sellers.toLocaleString()}
            icon={ShieldCheck}
            color="bg-chart-3"
          />
          <MetricCard
            title="Total Orders"
            value={summary.total_orders.toLocaleString()}
            icon={ShoppingCart}
            color="bg-chart-4"
          />
          <MetricCard
            title="Orders Today"
            value={summary.orders_today.toLocaleString()}
            icon={LayoutDashboard}
            color="bg-chart-5"
          />
          <MetricCard
            title="Pending Reports"
            value={summary.pending_reports.toLocaleString()}
            icon={Headphones}
            color="bg-chart-6"
          />
          <MetricCard
            title="Total Revenue"
            value={`Rp ${summary.total_revenue.toLocaleString()}`}
            icon={TrendingUp}
            color="bg-chart-7"
          />
        </div>
      )}
    </div>
  )
}
