import { MemoryRouter } from 'react-router-dom'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { GovernanceCasesPage } from './GovernanceCasesPage'

const useGovernanceCasesMock = vi.hoisted(() => vi.fn())

vi.mock('@/hooks/useGovernance', () => ({
  useGovernanceCases: useGovernanceCasesMock,
}))

const caseItem = {
  id: 'case-1',
  subject_type: 'content',
  subject_id: 'subject-1',
  status: 'open',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-02T00:00:00Z',
}

describe('GovernanceCasesPage pagination (Task 5D)', () => {
  beforeEach(() => {
    useGovernanceCasesMock.mockReturnValue({
      cases: [caseItem],
      loading: false,
      error: null,
      count: 25,
      refetch: vi.fn(),
    })
  })

  it('derives truthful total pages from the server total and uses canonical pagination', async () => {
    render(
      <MemoryRouter>
        <GovernanceCasesPage />
      </MemoryRouter>
    )

    // count=25 with page size 20 -> 2 pages; no fabricated "Page X" without total
    expect(screen.getByText('Page 1 of 2')).toBeInTheDocument()
    expect(screen.queryByText(/Showing/)).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Next' }))

    expect(screen.getByText('Page 2 of 2')).toBeInTheDocument()
  })

  it('hides pagination when the server total fits on a single page', () => {
    useGovernanceCasesMock.mockReturnValue({
      cases: [caseItem],
      loading: false,
      error: null,
      count: 20,
      refetch: vi.fn(),
    })

    render(
      <MemoryRouter>
        <GovernanceCasesPage />
      </MemoryRouter>
    )

    expect(screen.queryByText(/Page \d+ of/)).not.toBeInTheDocument()
  })
})
