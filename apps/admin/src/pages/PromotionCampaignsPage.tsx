import { useState, useEffect, useCallback } from 'react'
import { BarChart3, Filter, Loader2, Megaphone, RefreshCw, StopCircle } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/Table'
import { Modal } from '@/components/ui/Modal'
import { adminGetCampaignAnalytics, adminListCampaigns, adminForceStopCampaign } from '@/lib/api/promotions'
import { formatDate, formatDateTime } from '@/lib/utils'
import type {
  AdminCampaign,
  AdminCampaignAnalytics,
  CampaignStatus,
} from '@/types/promotion'
import {
  campaignStatusLabels,
  campaignStatusVariants,
  targetTypeLabels,
} from '@/types/promotion'

// ─── Force-Stop Confirm Modal ─────────────────────────────────────────────────

interface ForceStopModalProps {
  campaign: AdminCampaign | null
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

function ForceStopModal({ campaign, isOpen, onClose, onSuccess }: ForceStopModalProps) {
  const [reason, setReason] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (isOpen) {
      setReason('')
      setError(null)
    }
  }, [isOpen])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!campaign) return
    setSubmitting(true)
    setError(null)
    try {
      await adminForceStopCampaign(campaign.id, reason)
      onSuccess()
      onClose()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to stop campaign')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Force-Stop Campaign">
      <form onSubmit={handleSubmit} className="space-y-4">
        {campaign && (
          <div className="rounded bg-gray-50 p-3 text-sm text-gray-700">
            <p>
              <span className="font-medium">Campaign:</span> {campaign.id}
            </p>
            <p>
              <span className="font-medium">Target:</span>{' '}
              {targetTypeLabels[campaign.target_type] ?? campaign.target_type}{' '}
              {campaign.target_id ? `(${campaign.target_id})` : '—'}
            </p>
            <p>
              <span className="font-medium">Package:</span> {campaign.package_name}
            </p>
          </div>
        )}

        <div className="rounded border border-yellow-200 bg-yellow-50 p-3 text-sm text-yellow-800">
          This action terminates the campaign immediately. No automatic refund is issued.
          The consumed duration is baked into the ownership.
        </div>

        {error && (
          <div className="rounded bg-red-50 p-3 text-sm text-red-700">{error}</div>
        )}

        <div>
          <label className="mb-1 block text-sm font-medium text-gray-700">
            Reason <span className="text-red-500">*</span>
          </label>
          <textarea
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            required
            rows={3}
            className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            placeholder="Policy violation / fraudulent promotion / etc."
          />
        </div>

        <div className="flex justify-end gap-3 pt-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" disabled={submitting || !reason.trim()}>
            {submitting ? 'Stopping…' : 'Stop Campaign'}
          </Button>
        </div>
      </form>
    </Modal>
  )
}

interface CampaignAnalyticsModalProps {
  campaign: AdminCampaign | null
  analytics: AdminCampaignAnalytics | null
  isOpen: boolean
  loading: boolean
  error: string | null
  onClose: () => void
  onRetry: () => void
}

function CampaignAnalyticsModal({
  campaign,
  analytics,
  isOpen,
  loading,
  error,
  onClose,
  onRetry,
}: CampaignAnalyticsModalProps) {
  const hasAnalytics = !!analytics
  const isZeroSummary =
    !!analytics &&
    analytics.impressions_total === 0 &&
    analytics.clicks_total === 0 &&
    analytics.feed_impressions === 0 &&
    analytics.feed_clicks === 0 &&
    analytics.search_impressions === 0 &&
    analytics.search_clicks === 0 &&
    analytics.explore_impressions === 0 &&
    analytics.explore_clicks === 0

  const formatCount = (value: number) => new Intl.NumberFormat('en-US').format(value)
  const formatPercent = (value: number) =>
    new Intl.NumberFormat('en-US', {
      style: 'percent',
      minimumFractionDigits: 1,
      maximumFractionDigits: 1,
    }).format(value)

  const windowLabel = analytics?.window_from || analytics?.window_to
    ? `${analytics.window_from ? formatDateTime(analytics.window_from) : 'Start'} - ${analytics.window_to ? formatDateTime(analytics.window_to) : 'Now'}`
    : 'All time'

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Campaign Analytics" size="lg">
      {!campaign || !campaign.id ? (
        <div className="space-y-4">
          <div className="rounded border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
            Campaign data is unavailable. Please close this panel and choose a campaign again.
          </div>
          <div className="flex justify-end">
            <Button variant="secondary" onClick={onClose}>
              Close
            </Button>
          </div>
        </div>
      ) : loading ? (
        <div className="flex items-center justify-center py-12 text-sm text-gray-500">
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          Loading analytics...
        </div>
      ) : error ? (
        <div className="space-y-4">
          <div className="rounded border border-red-200 bg-red-50 p-4 text-sm text-red-700">
            {error}
          </div>
          <div className="flex items-center justify-end gap-3">
            <Button variant="secondary" onClick={onRetry}>
              Retry
            </Button>
            <Button variant="secondary" onClick={onClose}>
              Close
            </Button>
          </div>
        </div>
      ) : hasAnalytics ? (
        <div className="space-y-5">
          <div className="rounded border border-gray-200 bg-gray-50 p-4 text-sm text-gray-700">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="font-medium text-gray-900">
                  {targetTypeLabels[campaign.target_type] ?? campaign.target_type}
                  {campaign.target_id ? ` · ${campaign.target_id.slice(0, 8)}…` : ''}
                </p>
                <p className="text-xs text-gray-500">
                  Campaign {campaign.id.slice(0, 8)}… · {campaign.package_name}
                </p>
              </div>
              <Badge variant={campaignStatusVariants[campaign.status as CampaignStatus] ?? 'default'}>
                {campaignStatusLabels[campaign.status as CampaignStatus] ?? campaign.status}
              </Badge>
            </div>
            <p className="mt-3 text-xs text-gray-500">Window: {windowLabel}</p>
          </div>

          {isZeroSummary && (
            <div className="rounded border border-gray-200 bg-white p-3 text-sm text-gray-600">
              No analytics have been recorded yet for this campaign.
            </div>
          )}

          <div className="grid gap-3 md:grid-cols-3">
            <Card>
              <CardContent className="p-4">
                <p className="text-xs uppercase tracking-wide text-gray-500">Impressions</p>
                <p className="mt-1 text-2xl font-semibold text-gray-900" data-testid="analytics-impressions">
                  {formatCount(analytics.impressions_total)}
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="p-4">
                <p className="text-xs uppercase tracking-wide text-gray-500">Clicks</p>
                <p className="mt-1 text-2xl font-semibold text-gray-900" data-testid="analytics-clicks">
                  {formatCount(analytics.clicks_total)}
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardContent className="p-4">
                <p className="text-xs uppercase tracking-wide text-gray-500">CTR</p>
                <p className="mt-1 text-2xl font-semibold text-gray-900" data-testid="analytics-ctr">
                  {formatPercent(analytics.ctr)}
                </p>
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <BarChart3 className="h-4 w-4" />
                Surface Breakdown
              </CardTitle>
            </CardHeader>
            <CardContent className="pt-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Surface</TableHead>
                    <TableHead>Impressions</TableHead>
                    <TableHead>Clicks</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell>Feed</TableCell>
                    <TableCell data-testid="analytics-feed-impressions">
                      {formatCount(analytics.feed_impressions)}
                    </TableCell>
                    <TableCell data-testid="analytics-feed-clicks">
                      {formatCount(analytics.feed_clicks)}
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>Search</TableCell>
                    <TableCell data-testid="analytics-search-impressions">
                      {formatCount(analytics.search_impressions)}
                    </TableCell>
                    <TableCell data-testid="analytics-search-clicks">
                      {formatCount(analytics.search_clicks)}
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>Explore</TableCell>
                    <TableCell data-testid="analytics-explore-impressions">
                      {formatCount(analytics.explore_impressions)}
                    </TableCell>
                    <TableCell data-testid="analytics-explore-clicks">
                      {formatCount(analytics.explore_clicks)}
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          <div className="flex justify-end">
            <Button variant="secondary" onClick={onClose}>
              Close
            </Button>
          </div>
        </div>
      ) : (
        <div className="space-y-4">
          <div className="rounded border border-gray-200 bg-gray-50 p-4 text-sm text-gray-600">
            Analytics are not available for this campaign.
          </div>
          <div className="flex justify-end">
            <Button variant="secondary" onClick={onClose}>
              Close
            </Button>
          </div>
        </div>
      )}
    </Modal>
  )
}

// ─── Filter bar ───────────────────────────────────────────────────────────────

const STATUS_OPTIONS: { value: string; label: string }[] = [
  { value: '', label: 'All Statuses' },
  { value: 'active', label: 'Active' },
  { value: 'paused', label: 'Paused' },
  { value: 'inactive', label: 'Inactive' },
  { value: 'expired', label: 'Expired' },
  { value: 'cancelled', label: 'Cancelled' },
]

const TARGET_TYPE_OPTIONS: { value: string; label: string }[] = [
  { value: '', label: 'All Types' },
  { value: 'fixed_price_sale', label: 'Fixed Price Sale' },
  { value: 'auction', label: 'Auction' },
  { value: 'external_product', label: 'External Product' },
]

const PAGE_SIZE = 50

// ─── Page ─────────────────────────────────────────────────────────────────────

export function PromotionCampaignsPage() {
  const [campaigns, setCampaigns] = useState<AdminCampaign[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [fetchError, setFetchError] = useState<string | null>(null)
  const [offset, setOffset] = useState(0)

  const [filterStatus, setFilterStatus] = useState('')
  const [filterTargetType, setFilterTargetType] = useState('')
  const [filterOwnerID, setFilterOwnerID] = useState('')

  const [stopTarget, setStopTarget] = useState<AdminCampaign | null>(null)
  const [stopModalOpen, setStopModalOpen] = useState(false)
  const [analyticsTarget, setAnalyticsTarget] = useState<AdminCampaign | null>(null)
  const [analyticsModalOpen, setAnalyticsModalOpen] = useState(false)
  const [analyticsLoading, setAnalyticsLoading] = useState(false)
  const [analyticsError, setAnalyticsError] = useState<string | null>(null)
  const [analyticsData, setAnalyticsData] = useState<AdminCampaignAnalytics | null>(null)

  const fetchCampaigns = useCallback(async (off = 0) => {
    setLoading(true)
    setFetchError(null)
    try {
      const data = await adminListCampaigns({
        status: filterStatus || undefined,
        target_type: filterTargetType || undefined,
        owner_user_id: filterOwnerID.trim() || undefined,
        limit: PAGE_SIZE,
        offset: off,
      })
      setCampaigns(data.campaigns ?? [])
      setTotal(data.total ?? 0)
    } catch (err: unknown) {
      setFetchError(err instanceof Error ? err.message : 'Failed to load campaigns')
    } finally {
      setLoading(false)
    }
  }, [filterStatus, filterTargetType, filterOwnerID])

  useEffect(() => {
    setOffset(0)
    fetchCampaigns(0)
  }, [fetchCampaigns])

  const handleFilter = (e: React.FormEvent) => {
    e.preventDefault()
    setOffset(0)
    fetchCampaigns(0)
  }

  const handleStop = (campaign: AdminCampaign) => {
    setStopTarget(campaign)
    setStopModalOpen(true)
  }

  const handleViewAnalytics = async (campaign: AdminCampaign | null) => {
    setAnalyticsTarget(campaign)
    setAnalyticsModalOpen(true)
    setAnalyticsLoading(true)
    setAnalyticsError(null)
    setAnalyticsData(null)

    if (!campaign?.id) {
      setAnalyticsLoading(false)
      setAnalyticsError('Campaign data is unavailable.')
      return
    }

    try {
      const data = await adminGetCampaignAnalytics(campaign.id)
      setAnalyticsData(data.analytics)
    } catch (err: unknown) {
      setAnalyticsError(err instanceof Error ? err.message : 'Failed to load campaign analytics')
    } finally {
      setAnalyticsLoading(false)
    }
  }

  const closeAnalyticsModal = () => {
    setAnalyticsModalOpen(false)
    setAnalyticsTarget(null)
    setAnalyticsLoading(false)
    setAnalyticsError(null)
    setAnalyticsData(null)
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1

  const goPage = (page: number) => {
    const newOffset = (page - 1) * PAGE_SIZE
    setOffset(newOffset)
    fetchCampaigns(newOffset)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Promotion Campaigns</h1>
          <p className="mt-1 text-sm text-gray-500">
            View active and historical promotion instances. Force-stop violating campaigns.
          </p>
        </div>
        <Button variant="secondary" onClick={() => fetchCampaigns(offset)} disabled={loading}>
          <RefreshCw className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Filters */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Filter className="h-4 w-4" />
            Filters
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleFilter} className="flex flex-wrap items-end gap-4">
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-600">Status</label>
              <select
                value={filterStatus}
                onChange={(e) => setFilterStatus(e.target.value)}
                className="rounded border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {STATUS_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-600">Target Type</label>
              <select
                value={filterTargetType}
                onChange={(e) => setFilterTargetType(e.target.value)}
                className="rounded border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                {TARGET_TYPE_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-600">Owner User ID</label>
              <input
                type="text"
                value={filterOwnerID}
                onChange={(e) => setFilterOwnerID(e.target.value)}
                placeholder="UUID"
                className="rounded border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
            <Button type="submit" size="sm">
              Apply
            </Button>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => {
                setFilterStatus('')
                setFilterTargetType('')
                setFilterOwnerID('')
              }}
            >
              Clear
            </Button>
          </form>
        </CardContent>
      </Card>

      {fetchError && (
        <div className="rounded border border-red-200 bg-red-50 p-4 text-sm text-red-700">
          {fetchError}
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Megaphone className="h-5 w-5" />
            Campaigns ({total} total)
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {loading ? (
            <div className="p-8 text-center text-sm text-gray-500">Loading…</div>
          ) : campaigns.length === 0 ? (
            <div className="p-8 text-center text-sm text-gray-500">
              No campaigns match the current filters.
            </div>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>Owner</TableHead>
                    <TableHead>Target</TableHead>
                    <TableHead>Package</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Duration</TableHead>
                    <TableHead>Activated</TableHead>
                    <TableHead>Stopped</TableHead>
                    <TableHead>Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {campaigns.map((c) => (
                    <TableRow key={c.id}>
                      <TableCell className="font-mono text-xs text-gray-500">
                        {c.id.slice(0, 8)}…
                      </TableCell>
                      <TableCell className="font-mono text-xs text-gray-500">
                        {c.user_id.slice(0, 8)}…
                      </TableCell>
                      <TableCell>
                        <div className="text-sm">
                          <span className="font-medium">
                            {targetTypeLabels[c.target_type] ?? c.target_type}
                          </span>
                          {c.target_id && (
                            <div className="font-mono text-xs text-gray-400">
                              {c.target_id.slice(0, 8)}…
                            </div>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-sm">{c.package_name}</TableCell>
                      <TableCell>
                        <Badge variant={campaignStatusVariants[c.status as CampaignStatus] ?? 'default'}>
                          {campaignStatusLabels[c.status as CampaignStatus] ?? c.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-sm text-gray-600">
                        {c.ownership_consumed_hours}h / {c.ownership_total_hours}h
                      </TableCell>
                      <TableCell className="text-sm text-gray-500">
                        {c.activated_at ? formatDate(c.activated_at) : '—'}
                      </TableCell>
                      <TableCell className="text-sm text-gray-500">
                        {c.stopped_at ? (
                          <span title={c.stop_reason ?? ''}>
                            {formatDate(c.stopped_at)}
                          </span>
                        ) : (
                          '—'
                        )}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center justify-end gap-2">
                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={() => handleViewAnalytics(c)}
                            title="View campaign analytics"
                          >
                            <BarChart3 className="mr-1.5 h-3.5 w-3.5" />
                            Analytics
                          </Button>
                          {(c.status === 'active' || c.status === 'paused') && (
                            <Button
                              variant="secondary"
                              size="sm"
                              onClick={() => handleStop(c)}
                              title="Force-stop campaign"
                            >
                              <StopCircle className="h-3.5 w-3.5 text-red-500" />
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              {/* Pagination */}
              {totalPages > 1 && (
                <div className="flex items-center justify-between border-t px-4 py-3">
                  <p className="text-sm text-gray-500">
                    Page {currentPage} of {totalPages} ({total} total)
                  </p>
                  <div className="flex gap-2">
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={currentPage <= 1}
                      onClick={() => goPage(currentPage - 1)}
                    >
                      Previous
                    </Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={currentPage >= totalPages}
                      onClick={() => goPage(currentPage + 1)}
                    >
                      Next
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>

      <ForceStopModal
        campaign={stopTarget}
        isOpen={stopModalOpen}
        onClose={() => setStopModalOpen(false)}
        onSuccess={() => fetchCampaigns(offset)}
      />

      <CampaignAnalyticsModal
        campaign={analyticsTarget}
        analytics={analyticsData}
        isOpen={analyticsModalOpen}
        loading={analyticsLoading}
        error={analyticsError}
        onClose={closeAnalyticsModal}
        onRetry={() => void handleViewAnalytics(analyticsTarget)}
      />
    </div>
  )
}
