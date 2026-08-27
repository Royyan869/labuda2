import { describe, expect, it, vi, beforeEach } from 'vitest'

const apiGetMock = vi.hoisted(() => vi.fn())

vi.mock('./client', () => ({
  api: {
    get: apiGetMock,
  },
}))

import { getModerationCases, getModerationCaseEvidence } from './moderation'

describe('getModerationCases', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
    apiGetMock.mockResolvedValue({ cases: [], count: 0 })
  })

  it('serializes resource_type into the moderation cases request', async () => {
    await getModerationCases({
      status: 'pending',
      resource_type: 'fixed_price_sale',
      page: 2,
      limit: 25,
    })

    expect(apiGetMock).toHaveBeenCalledWith(
      '/api/v1/admin/moderation/cases?status=pending&resource_type=fixed_price_sale&page=2&limit=25'
    )
  })

  it('requests case evidence from the dedicated evidence endpoint', async () => {
    apiGetMock.mockResolvedValue({
      data: {
        evidence: {
          case_id: 'case-1',
          resource_type: 'chat_message',
          resource_id: 'message-1',
          message_id: 'message-1',
          room_id: 'room-1',
          room_type: 'normal',
          sender_id: 'user-1',
          created_at: '2026-07-14T00:00:00.000Z',
        },
      },
    })

    await getModerationCaseEvidence('case-1')

    expect(apiGetMock).toHaveBeenCalledWith('/api/v1/admin/moderation/cases/case-1/evidence')
  })
})
