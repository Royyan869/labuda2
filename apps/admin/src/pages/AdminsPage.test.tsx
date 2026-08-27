import { MemoryRouter } from 'react-router-dom'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { AdminsPage } from './AdminsPage'
import type { UserListItem } from '@/types'

const mockUser: UserListItem = {
  id: 'admin-123',
  username: 'ops-admin',
  email: 'ops-admin@labuda.test',
  account_status: 'active',
  is_seller: false,
  is_buyer: false,
  is_admin: true,
  created_at: '2026-01-01T00:00:00Z',
}

vi.mock('@/hooks/useUsers', () => ({
  useUsers: () => ({
    users: [mockUser],
    loading: false,
    error: null,
    total: 1,
    refetch: vi.fn(),
  }),
}))

// P5-05: the "Manage" link must point at the actual registered route
// (/users/admins/:id per App.tsx) — the previous /admin/users/admins/:id
// link 404'd because no such route exists.
describe('AdminsPage', () => {
  it('links Manage to the registered /users/admins/:id route', () => {
    render(
      <MemoryRouter>
        <AdminsPage />
      </MemoryRouter>
    )

    const manageLink = screen.getByRole('link', { name: /manage/i })
    expect(manageLink).toHaveAttribute('href', '/users/admins/admin-123')
  })
})
