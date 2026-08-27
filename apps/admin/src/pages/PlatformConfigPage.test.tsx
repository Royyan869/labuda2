import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { PlatformConfigPage } from './PlatformConfigPage'

type SellerSubscriptionConfig = {
  id: string
  yearly_fee_rupiah: number
  duration_days: number
  renewal_reminder_days: number
  enabled: boolean
  created_at: string
}

const getPlatformConfigsMock = vi.hoisted(() => vi.fn())
const getSellerSubscriptionConfigMock = vi.hoisted(() => vi.fn())
const updateSellerSubscriptionConfigMock = vi.hoisted(() => vi.fn())
const useAuthMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  getPlatformConfigs: getPlatformConfigsMock,
  getSellerSubscriptionConfig: getSellerSubscriptionConfigMock,
  updateSellerSubscriptionConfig: updateSellerSubscriptionConfigMock,
  updatePlatformConfig: vi.fn(),
}))

vi.mock('@/hooks/useAuth', () => ({
  useAuth: useAuthMock,
}))

function makeSellerConfig(overrides: Partial<SellerSubscriptionConfig> = {}): SellerSubscriptionConfig {
  return {
    id: 'seller-subscription-config-1',
    yearly_fee_rupiah: 70000,
    duration_days: 365,
    renewal_reminder_days: 7,
    enabled: true,
    created_at: '2026-07-16T00:00:00Z',
    ...overrides,
  }
}

describe('PlatformConfigPage seller subscription config', () => {
  beforeEach(() => {
    getPlatformConfigsMock.mockReset()
    getSellerSubscriptionConfigMock.mockReset()
    updateSellerSubscriptionConfigMock.mockReset()
    useAuthMock.mockReset()

    useAuthMock.mockReturnValue({
      capabilities: ['config.update.financial'],
    })
    getPlatformConfigsMock.mockResolvedValue({ configs: [], count: 0 })
    getSellerSubscriptionConfigMock.mockResolvedValue(makeSellerConfig())
    updateSellerSubscriptionConfigMock.mockResolvedValue({
      config: makeSellerConfig(),
      message: 'Subscription config updated',
    })
  })

  it('renders and saves the canonical yearly fee without any 100x conversion', async () => {
    const user = userEvent.setup()

    const { unmount } = render(<PlatformConfigPage />)

    await waitFor(() => expect(screen.getByText('Rp70.000')).toBeInTheDocument())
    expect(screen.getByText('365 days')).toBeInTheDocument()
    expect(screen.getByText('7 days before expiry')).toBeInTheDocument()
    expect(screen.getByText('Enabled')).toBeInTheDocument()
    expect(screen.queryByText('Rp7.000.000')).not.toBeInTheDocument()
    expect(screen.queryByText('seller_subscription_price')).not.toBeInTheDocument()
    expect(screen.queryByText('seller_subscription_duration_days')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Edit' }))

    await user.click(screen.getByRole('button', { name: 'Save' }))
    await user.click(screen.getByRole('button', { name: 'Confirm Change' }))

    await waitFor(() =>
      expect(updateSellerSubscriptionConfigMock).toHaveBeenCalledWith({
        yearly_fee_rupiah: 70000,
        duration_days: 365,
        renewal_reminder_days: 7,
        enabled: true,
      })
    )

    await waitFor(() => expect(screen.getByText('Rp70.000')).toBeInTheDocument())
    expect(screen.queryByText('Rp7.000.000')).not.toBeInTheDocument()

    unmount()

    render(<PlatformConfigPage />)
    await waitFor(() => expect(screen.getByText('Rp70.000')).toBeInTheDocument())
  })
})
