import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { PromotionCampaignsPage } from './PromotionCampaignsPage'
import type { AdminCampaign, AdminCampaignAnalyticsResponse } from '@/types/promotion'

const adminListCampaignsMock = vi.hoisted(() => vi.fn())
const adminForceStopCampaignMock = vi.hoisted(() => vi.fn())
const adminGetCampaignAnalyticsMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api/promotions', () => ({
  adminListCampaigns: adminListCampaignsMock,
  adminForceStopCampaign: adminForceStopCampaignMock,
  adminGetCampaignAnalytics: adminGetCampaignAnalyticsMock,
}))

function makeCampaign(overrides: Partial<AdminCampaign> = {}): AdminCampaign {
  return {
    id: 'camp_1234567890',
    ownership_id: 'own_123',
    user_id: 'user_123',
    target_type: 'fixed_price_sale',
    target_id: 'list_123',
    status: 'active',
    activated_at: '2026-06-01T00:00:00.000Z',
    stopped_at: undefined,
    stop_reason: undefined,
    created_at: '2026-06-01T00:00:00.000Z',
    updated_at: '2026-06-01T00:00:00.000Z',
    package_id: 'pkg_123',
    package_name: 'Starter Boost',
    ownership_total_hours: 24,
    ownership_consumed_hours: 2,
    ...overrides,
  }
}

function makeAnalytics(
  overrides: Partial<AdminCampaignAnalyticsResponse['analytics']> = {}
): AdminCampaignAnalyticsResponse {
  return {
    analytics: {
      instance_id: 'camp_1234567890',
      impressions_total: 0,
      clicks_total: 0,
      ctr: 0,
      feed_impressions: 0,
      feed_clicks: 0,
      search_impressions: 0,
      search_clicks: 0,
      explore_impressions: 0,
      explore_clicks: 0,
      ...overrides,
    },
  }
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('PromotionCampaignsPage', () => {
  beforeEach(() => {
    adminListCampaignsMock.mockReset()
    adminForceStopCampaignMock.mockReset()
    adminGetCampaignAnalyticsMock.mockReset()
    adminListCampaignsMock.mockResolvedValue({
      campaigns: [makeCampaign()],
      total: 1,
      limit: 50,
      offset: 0,
    })
    adminForceStopCampaignMock.mockResolvedValue({ message: 'ok' })
    adminGetCampaignAnalyticsMock.mockResolvedValue(makeAnalytics())
  })

  it('calls the analytics endpoint with the selected campaign id', async () => {
    const user = userEvent.setup()
    render(<PromotionCampaignsPage />)

    await screen.findByText('Starter Boost')
    await user.click(screen.getByRole('button', { name: 'Analytics' }))

    await waitFor(() => {
      expect(adminGetCampaignAnalyticsMock).toHaveBeenCalledWith('camp_1234567890')
    })
  })

  it('renders a zero analytics summary', async () => {
    const user = userEvent.setup()
    render(<PromotionCampaignsPage />)

    await screen.findByText('Starter Boost')
    await user.click(screen.getByRole('button', { name: 'Analytics' }))

    expect(await screen.findByText('No analytics have been recorded yet for this campaign.')).toBeInTheDocument()
    expect(screen.getByTestId('analytics-impressions')).toHaveTextContent('0')
    expect(screen.getByTestId('analytics-clicks')).toHaveTextContent('0')
    expect(screen.getByTestId('analytics-ctr')).toHaveTextContent('0.0%')
  })

  it('renders non-zero analytics and ctr', async () => {
    const user = userEvent.setup()
    adminGetCampaignAnalyticsMock.mockResolvedValueOnce(
      makeAnalytics({
        impressions_total: 10,
        clicks_total: 4,
        ctr: 0.4,
        feed_impressions: 6,
        feed_clicks: 2,
        search_impressions: 3,
        search_clicks: 1,
        explore_impressions: 1,
        explore_clicks: 1,
      })
    )

    render(<PromotionCampaignsPage />)

    await screen.findByText('Starter Boost')
    await user.click(screen.getByRole('button', { name: 'Analytics' }))

    expect(await screen.findByText('40.0%')).toBeInTheDocument()
    expect(screen.getByTestId('analytics-impressions')).toHaveTextContent('10')
    expect(screen.getByTestId('analytics-clicks')).toHaveTextContent('4')
    expect(screen.getByTestId('analytics-feed-impressions')).toHaveTextContent('6')
    expect(screen.getByTestId('analytics-search-clicks')).toHaveTextContent('1')
    expect(screen.getByTestId('analytics-explore-clicks')).toHaveTextContent('1')
  })

  it('shows loading and error states without crashing', async () => {
    const user = userEvent.setup()
    const deferred = createDeferred<AdminCampaignAnalyticsResponse>()
    adminGetCampaignAnalyticsMock.mockReturnValueOnce(deferred.promise)
    adminGetCampaignAnalyticsMock.mockRejectedValueOnce(new Error('analytics unavailable'))

    render(<PromotionCampaignsPage />)

    await screen.findByText('Starter Boost')
    await user.click(screen.getByRole('button', { name: 'Analytics' }))

    expect(screen.getByText('Loading analytics...')).toBeInTheDocument()

    deferred.resolve(makeAnalytics())
    expect(await screen.findByText('No analytics have been recorded yet for this campaign.')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Analytics' }))
    expect(await screen.findByText('analytics unavailable')).toBeInTheDocument()
  })

  it('keeps the campaign list actions available', async () => {
    render(<PromotionCampaignsPage />)

    await screen.findByText('Starter Boost')
    expect(screen.getByRole('button', { name: 'Analytics' })).toBeInTheDocument()
    expect(screen.getByTitle('Force-stop campaign')).toBeInTheDocument()
  })
})
