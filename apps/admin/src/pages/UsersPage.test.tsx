import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { UsersPage } from './UsersPage'

const apiGetMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  api: {
    get: apiGetMock,
  },
  getAuthToken: () => null,
}))

// PASS_20F: reproduces the reported "Error loading users: HTTP 500" —
// the actual backend bug (wrong table name + a NULL scan on a LEFT JOIN
// column) is fixed and proven with a Go integration test; these tests
// cover the frontend contract: empty state renders cleanly, and a genuine
// 500 shows the error panel rather than a blank page.
describe('UsersPage (PASS_20F retest)', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
  })

  it('renders the empty state when the backend returns zero users', async () => {
    apiGetMock.mockResolvedValue({
      success: true,
      data: { users: [] },
      meta: { page: 1, per_page: 20, total: 0, total_pages: 0 },
      timestamp: '2026-07-06T00:00:00Z',
    })

    render(<UsersPage />)

    expect(await screen.findByText('No users in the system.')).toBeInTheDocument()
  })

  it('renders seeded users once the backend returns them', async () => {
    apiGetMock.mockResolvedValue({
      success: true,
      data: {
        users: [
          {
            id: 'u1',
            email: 'seller@test.local',
            username: null,
            role: 'seller',
            account_status: 'active',
            is_verified: false,
            created_at: '2026-07-02T19:57:12Z',
          },
        ],
      },
      meta: { page: 1, per_page: 20, total: 1, total_pages: 1 },
      timestamp: '2026-07-06T00:00:00Z',
    })

    render(<UsersPage />)

    expect(await screen.findByText('seller@test.local')).toBeInTheDocument()
  })

  it('shows an error panel (not a blank page) when the backend 500s', async () => {
    apiGetMock.mockRejectedValue(new Error('HTTP 500: An error occurred'))

    render(<UsersPage />)

    await waitFor(() =>
      expect(screen.getByText(/Error loading users/)).toBeInTheDocument()
    )
  })
})
