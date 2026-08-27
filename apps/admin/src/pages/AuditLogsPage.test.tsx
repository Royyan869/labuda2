import { render, screen, within } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AuditLogsPage } from './AuditLogsPage'

const useAuditLogsMock = vi.hoisted(() => vi.fn())

vi.mock('@/hooks/useAuditLogs', () => ({
  useAuditLogs: useAuditLogsMock,
}))

describe('AuditLogsPage target type filter', () => {
  beforeEach(() => {
    useAuditLogsMock.mockReturnValue({
      logs: [],
      loading: false,
      error: null,
      count: 0,
      setPage: vi.fn(),
    })
  })

  it('offers only real backend target types, excluding the phantom "trade" option', () => {
    render(<AuditLogsPage />)

    const targetSelect = screen.getByLabelText('Target:')
    const optionLabels = within(targetSelect)
      .getAllByRole('option')
      .map((opt) => opt.textContent)

    expect(optionLabels).not.toContain('Trade')
    expect(optionLabels).toEqual(
      expect.arrayContaining(['All Targets', 'User', 'Withdrawal', 'Dispute', 'Refund', 'Auction'])
    )
  })
})
