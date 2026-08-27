import { describe, expect, it, vi, beforeEach } from 'vitest'

const apiPostMock = vi.hoisted(() => vi.fn())

vi.mock('./client', () => ({
  api: {
    post: apiPostMock,
  },
}))

import { adminCancelAuction } from './auctions'

describe('adminCancelAuction', () => {
  beforeEach(() => {
    apiPostMock.mockReset()
    // Real backend shape: response.Success wraps the payload as { data: ... }.
    apiPostMock.mockResolvedValue({
      data: {
        auction_id: 'auction-1',
        status_before: 'active',
        status_after: 'cancelled',
        reason: 'seller unreachable',
      },
    })
  })

  it('posts the reason to the admin auction cancel endpoint', async () => {
    await adminCancelAuction('auction-1', 'seller unreachable')

    expect(apiPostMock).toHaveBeenCalledWith(
      '/api/v1/admin/auctions/auction-1/cancel',
      { reason: 'seller unreachable' }
    )
  })

  it('returns the cancelled auction status transition', async () => {
    const result = await adminCancelAuction('auction-1', 'seller unreachable')

    expect(result.status_before).toBe('active')
    expect(result.status_after).toBe('cancelled')
  })
})
