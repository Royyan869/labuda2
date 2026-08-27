import { MemoryRouter } from 'react-router-dom'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AppealsPage } from './AppealsPage'
import { RequireCapability } from '@/components/auth/RequireCapability'

const useAppealsMock = vi.hoisted(() => vi.fn())
const useAppealMock = vi.hoisted(() => vi.fn())
const useAppealReviewMock = vi.hoisted(() => vi.fn())
const useAuthMock = vi.fn()

vi.mock('@/hooks/useAppeals', () => ({
  useAppeals: useAppealsMock,
  // AppealsPage always mounts AppealDetailModal (even when closed), which
  // calls these hooks unconditionally.
  useAppeal: useAppealMock,
  useAppealReview: useAppealReviewMock,
}))

vi.mock('@/hooks/useAuth', () => ({
  useAuth: () => useAuthMock(),
}))

describe('AppealsPage route guard (PASS_13B)', () => {
  beforeEach(() => {
    useAppealsMock.mockReturnValue({
      appeals: [],
      loading: false,
      error: null,
      count: 0,
      refetch: vi.fn(),
    })
    useAppealMock.mockReturnValue({
      appealDetail: null,
      loading: false,
      refetch: vi.fn(),
    })
    useAppealReviewMock.mockReturnValue({
      reviewAppeal: vi.fn(),
      loading: false,
    })
  })

  it('is visible to an admin with moderation.appeal.read', () => {
    useAuthMock.mockReturnValue({
      user: { id: 'admin-1' },
      capabilities: ['moderation.appeal.read'],
    })

    render(
      <MemoryRouter>
        <RequireCapability cap="moderation.appeal.read">
          <AppealsPage />
        </RequireCapability>
      </MemoryRouter>
    )

    expect(screen.getByText('Appeals')).toBeInTheDocument()
    expect(screen.queryByText('Access Denied')).not.toBeInTheDocument()
  })

  it('is NOT visible to an admin with only moderation.case.read', () => {
    useAuthMock.mockReturnValue({
      user: { id: 'admin-2' },
      capabilities: ['moderation.case.read'],
    })

    render(
      <MemoryRouter>
        <RequireCapability cap="moderation.appeal.read">
          <AppealsPage />
        </RequireCapability>
      </MemoryRouter>
    )

    expect(screen.getByText('Access Denied')).toBeInTheDocument()
    expect(screen.queryByText('Review user appeals for moderation decisions')).not.toBeInTheDocument()
  })

  it('is also visible to an admin with both appeal.read and appeal.review', () => {
    useAuthMock.mockReturnValue({
      user: { id: 'admin-3' },
      capabilities: ['moderation.appeal.read', 'moderation.appeal.review'],
    })

    render(
      <MemoryRouter>
        <RequireCapability cap="moderation.appeal.read">
          <AppealsPage />
        </RequireCapability>
      </MemoryRouter>
    )

    expect(screen.getByText('Appeals')).toBeInTheDocument()
  })
})
