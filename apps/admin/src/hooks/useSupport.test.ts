import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import type { SupportTicketsQueryParams } from '@/types/support'

const listSupportTicketsMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  claimSupportTicket: vi.fn(),
  closeSupportTicket: vi.fn(),
  escalateSupportTicketToDispute: vi.fn(),
  getSupportTicket: vi.fn(),
  getSupportTicketMessages: vi.fn(),
  listSupportTickets: listSupportTicketsMock,
  resolveSupportTicket: vi.fn(),
  sendSupportTicketMessage: vi.fn(),
  setSupportTicketWaitingForUser: vi.fn(),
  updateSupportTicketCategory: vi.fn(),
  updateSupportTicketPriority: vi.fn(),
}))

import { useSupportTickets } from './useSupport'

describe('useSupportTickets', () => {
  beforeEach(() => {
    listSupportTicketsMock.mockReset()
  })

  it('reloads support tickets when the filter params change', async () => {
    listSupportTicketsMock.mockResolvedValue({ tickets: [] })

    type HookProps = { params: SupportTicketsQueryParams }
    const initialParams: SupportTicketsQueryParams = {
      status: 'open',
      category: 'order_issue',
    }

    const { rerender, result } = renderHook(
      ({ params }: HookProps) => useSupportTickets(params),
      {
        initialProps: { params: initialParams },
      }
    )

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(listSupportTicketsMock).toHaveBeenCalledTimes(1)
    expect(listSupportTicketsMock).toHaveBeenLastCalledWith(
      expect.objectContaining({
        status: 'open',
        category: 'order_issue',
      })
    )

    rerender({
      params: {
        status: 'resolved',
        category: 'payment_issue',
      },
    })

    await waitFor(() => expect(listSupportTicketsMock).toHaveBeenCalledTimes(2))
    expect(listSupportTicketsMock).toHaveBeenLastCalledWith(
      expect.objectContaining({
        status: 'resolved',
        category: 'payment_issue',
      })
    )
  })
})
