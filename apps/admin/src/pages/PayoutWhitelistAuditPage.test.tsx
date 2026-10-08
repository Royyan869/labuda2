import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { PayoutWhitelistAuditPage } from './PayoutWhitelistAuditPage'
import type { WhitelistAuditRow } from '@/types/finance'

const getWhitelistAuditMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  getWhitelistAudit: getWhitelistAuditMock,
}))

function row(id: string): WhitelistAuditRow {
  return {
    id,
    seller_id: '11111111-1111-1111-1111-111111111111',
    action: 'SELLER_ADDED',
    actor_id: 'admin-1',
    reason: 'pilot',
    source: 'config',
    created_at: '2026-01-01T00:00:00Z',
  }
}

describe('PayoutWhitelistAuditPage keyset navigation', () => {
  beforeEach(() => {
    getWhitelistAuditMock.mockReset()
  })

  it('loads the newest page on mount without a cursor', async () => {
    getWhitelistAuditMock.mockResolvedValue({
      audit_log: [row('a')],
      limit: 50,
      has_more: true,
      next_cursor: 'cursor-1',
    })

    render(<PayoutWhitelistAuditPage />)

    expect(await screen.findByText('Whitelist Audit Log')).toBeInTheDocument()
    await waitFor(() =>
      expect(getWhitelistAuditMock).toHaveBeenCalledWith(
        expect.objectContaining({ limit: 50, cursor: null })
      )
    )
  })

  it('forwards the continuation token and terminates at end of log', async () => {
    getWhitelistAuditMock
      .mockResolvedValueOnce({
        audit_log: [row('a')],
        limit: 50,
        has_more: true,
        next_cursor: 'cursor-1',
      })
      .mockResolvedValueOnce({
        audit_log: [row('b')],
        limit: 50,
        has_more: false,
        next_cursor: null,
      })

    const user = userEvent.setup()
    render(<PayoutWhitelistAuditPage />)

    await user.click(await screen.findByRole('button', { name: 'Load more' }))

    await waitFor(() =>
      expect(getWhitelistAuditMock).toHaveBeenLastCalledWith(
        expect.objectContaining({ cursor: 'cursor-1' })
      )
    )
    expect(await screen.findByText('End of log')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument()
  })

  it('renders an empty state when no records match', async () => {
    getWhitelistAuditMock.mockResolvedValue({
      audit_log: [],
      limit: 50,
      has_more: false,
      next_cursor: null,
    })

    render(<PayoutWhitelistAuditPage />)

    expect(await screen.findByText('No Audit Records')).toBeInTheDocument()
  })

  it('resets the keyset when the seller filter changes', async () => {
    getWhitelistAuditMock.mockResolvedValue({
      audit_log: [],
      limit: 50,
      has_more: false,
      next_cursor: null,
    })

    const user = userEvent.setup()
    render(<PayoutWhitelistAuditPage />)
    await screen.findByText('No Audit Records')

    await user.type(screen.getByLabelText('Seller ID'), 'seller-1')

    await waitFor(() =>
      expect(getWhitelistAuditMock).toHaveBeenLastCalledWith(
        expect.objectContaining({ seller_id: 'seller-1', cursor: null })
      )
    )
  })
})
