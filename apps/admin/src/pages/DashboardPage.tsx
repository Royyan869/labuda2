import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { api } from '@/lib/api'
import {
  LayoutDashboard,
  Users,
  ShoppingCart,
  AlertTriangle,
  ShieldCheck,
  Headphones,
  TrendingUp,
  RefreshCw,
} from 'lucide-react'

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
            <p className="text-sm font-medium text-gray-600">{title}</p>
            <p className="text-2xl font-bold text-gray-900 mt-1">{value}</p>
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
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Dashboard Overview</h1>
          <p className="text-gray-600 mt-1">Welcome to LABUDA Admin Dashboard</p>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {Array.from({ length: 7 }).map((_, i) => (
            <Card key={i}>
              <CardContent className="p-6">
                <div className="animate-pulse">
                  <div className="h-4 bg-gray-200 rounded w-24 mb-3"></div>
                  <div className="h-8 bg-gray-200 rounded w-16"></div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    )
  }

  if (error && !summary) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Dashboard Overview</h1>
          <p className="text-gray-600 mt-1">Welcome to LABUDA Admin Dashboard</p>
        </div>
        <Card>
          <CardContent className="p-12">
            <div className="text-center py-8">
              <AlertTriangle className="h-12 w-12 text-red-400 mx-auto mb-4" />
              <h2 className="text-lg font-semibold text-gray-900 mb-2">Failed to Load Dashboard</h2>
              <p className="text-gray-600 mb-4">{error}</p>
              <Button variant="secondary" onClick={fetchDashboard}>
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
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Dashboard Overview</h1>
          <p className="text-gray-600 mt-1">Welcome to LABUDA Admin Dashboard</p>
        </div>
        <div className="flex items-center gap-3">
          {generatedAt && (
            <span className="text-xs text-gray-500">
              Updated: {new Date(generatedAt).toLocaleTimeString()}
            </span>
          )}
          <Button
            variant="ghost"
            size="sm"
            onClick={fetchDashboard}
            disabled={loading}
          >
            <RefreshCw className={`h-4 w-4 mr-1 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        </div>
      </div>

      {/* Metric Cards */}
      {summary && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <MetricCard
            title="Total Users"
            value={summary.total_users.toLocaleString()}
            icon={Users}
            color="bg-blue-500"
          />
          <MetricCard
            title="Active Users Today"
            value={summary.active_users_today.toLocaleString()}
            icon={TrendingUp}
            color="bg-green-500"
          />
          <MetricCard
            title="Active Sellers"
            value={summary.active_sellers.toLocaleString()}
            icon={ShieldCheck}
            color="bg-purple-500"
          />
          <MetricCard
            title="Total Orders"
            value={summary.total_orders.toLocaleString()}
            icon={ShoppingCart}
            color="bg-indigo-500"
          />
          <MetricCard
            title="Orders Today"
            value={summary.orders_today.toLocaleString()}
            icon={LayoutDashboard}
            color="bg-teal-500"
          />
          <MetricCard
            title="Pending Reports"
            value={summary.pending_reports.toLocaleString()}
            icon={Headphones}
            color="bg-amber-500"
          />
          <MetricCard
            title="Total Revenue"
            value={`Rp ${summary.total_revenue.toLocaleString()}`}
            icon={TrendingUp}
            color="bg-emerald-500"
          />
        </div>
      )}
    </div>
  )
}
