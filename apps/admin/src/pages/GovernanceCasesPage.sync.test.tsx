import { MemoryRouter } from 'react-router-dom'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { GovernanceCasesPage } from './GovernanceCasesPage'

// This suite exercises the REAL useGovernanceCases hook (no hook mock) so the
// page→hook→API page-number sync is proven end-to-end. Regression guard for
// the bug where the hook shadowed `params.page` in useState and kept fetching
// page 1 while the UI displayed a later page.
const listGovernanceCasesMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api/governance', () => ({
  listGovernanceCases: listGovernanceCasesMock,
  getGovernanceCase: vi.fn(),
  createGovernanceDecision: vi.fn(),
  getGovernanceCaseAudit: vi.fn(),
}))

const caseItem = {
  id: 'case-1',
  subject_type: 'content',
  subject_id: 'subject-1',
  status: 'open',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-02T00:00:00Z',
}

describe('GovernanceCasesPage page-number sync', () => {
  beforeEach(() => {
    listGovernanceCasesMock.mockReset()
    listGovernanceCasesMock.mockResolvedValue({ cases: [caseItem], count: 25 })
  })

  it('refetches page 2 when Next is pressed', async () => {
    const user = userEvent.setup()
    render(
      <MemoryRouter>
        <GovernanceCasesPage />
      </MemoryRouter>
    )

    expect(await screen.findByText('Page 1 of 2')).toBeInTheDocument()
    expect(listGovernanceCasesMock).toHaveBeenCalledWith(expect.objectContaining({ page: 1, limit: 20 }))

    await user.click(screen.getByRole('button', { name: 'Next' }))

    await waitFor(() =>
      expect(listGovernanceCasesMock).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 2, limit: 20 })
      )
    )
    expect(await screen.findByText('Page 2 of 2')).toBeInTheDocument()
  })
})
