import { describe, expect, it, vi, beforeEach } from 'vitest'

const apiGetMock = vi.hoisted(() => vi.fn())

vi.mock('./client', () => ({
  api: {
    get: apiGetMock,
  },
}))

import { adminGetCampaignAnalytics } from './promotions'

describe('adminGetCampaignAnalytics', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
    apiGetMock.mockResolvedValue({ analytics: null })
  })

  it('calls the campaign analytics endpoint', async () => {
    await adminGetCampaignAnalytics('campaign_123')

    expect(apiGetMock).toHaveBeenCalledWith('/api/v1/admin/promotions/campaigns/campaign_123/analytics')
  })
})
