import { describe, expect, it, vi, beforeEach } from 'vitest'

const apiGetMock = vi.hoisted(() => vi.fn())
const apiPostMock = vi.hoisted(() => vi.fn())

vi.mock('./client', () => ({
  api: {
    get: apiGetMock,
    post: apiPostMock,
  },
}))

import {
  getSupportTicketMessages,
  sendSupportTicketMessage,
} from './support'

const message = {
  id: 'msg-1',
  room_id: 'room-1',
  sender_id: 'admin-1',
  sender_type: 'admin' as const,
  message_type: 'text',
  body: 'kami cek dulu ya',
  created_at: '2026-09-19T00:00:00Z',
}

describe('support conversation API client', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
    apiPostMock.mockReset()
  })

  it('unwraps the canonical nested data envelope for ticket messages', async () => {
    apiGetMock.mockResolvedValue({
      success: true,
      data: { data: [message] },
    })

    const messages = await getSupportTicketMessages('ticket-1')

    expect(apiGetMock).toHaveBeenCalledWith(
      '/api/v1/admin/support/tickets/ticket-1/messages'
    )
    expect(messages).toHaveLength(1)
    // The canonical sender taxonomy comes from the server, never inferred.
    expect(messages[0].sender_type).toBe('admin')
    expect(messages[0].sender_id).toBe('admin-1')
  })

  it('returns an empty list when no conversation exists', async () => {
    apiGetMock.mockResolvedValue({ success: true, data: { data: [] } })

    const messages = await getSupportTicketMessages('ticket-2')

    expect(messages).toEqual([])
  })

  it('sends an agent reply through the admin conversation endpoint', async () => {
    apiPostMock.mockResolvedValue({
      success: true,
      message: 'Message sent',
      data: { ticket_id: 'ticket-1', chat_room_id: 'room-1' },
    })

    await sendSupportTicketMessage('ticket-1', {
      type: 'agent',
      message: 'kami cek dulu ya',
    })

    expect(apiPostMock).toHaveBeenCalledWith(
      '/api/v1/admin/support/tickets/ticket-1/messages',
      { type: 'agent', message: 'kami cek dulu ya' }
    )
  })
})
