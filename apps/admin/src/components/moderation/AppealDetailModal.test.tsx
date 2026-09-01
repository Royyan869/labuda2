import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AppealDetailModal } from './AppealDetailModal'
import type { Appeal } from '@/types/moderation'

const useAppealMock = vi.hoisted(() => vi.fn())
const useAppealReviewMock = vi.hoisted(() => vi.fn())
const useAuthMock = vi.hoisted(() => vi.fn())

vi.mock('@/hooks/useAppeals', () => ({
  useAppeal: useAppealMock,
  useAppealReview: useAppealReviewMock,
}))

vi.mock('@/hooks/useAuth', () => ({
  useAuth: useAuthMock,
}))

function makeAppeal(overrides: Partial<Appeal> = {}): Appeal {
  return {
    id: 'appeal-1',
    decision_id: 'decision-1',
    status: 'pending',
    message: 'I believe this decision was incorrect.',
    created_at: '2026-06-01T00:00:00.000Z',
    ...overrides,
  }
}

describe('AppealDetailModal — capability-gated review action', () => {
  beforeEach(() => {
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

  it('shows Approve/Reject controls for an admin with moderation.appeal.review', () => {
    useAuthMock.mockReturnValue({ capabilities: ['moderation.appeal.read', 'moderation.appeal.review'] })

    render(
      <AppealDetailModal
        isOpen={true}
        onClose={vi.fn()}
        appeal={makeAppeal()}
        onReviewComplete={vi.fn()}
      />
    )

    expect(screen.getByRole('button', { name: /Approve Appeal/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Reject Appeal/i })).toBeInTheDocument()
    expect(screen.queryByText(/do not have permission to submit a decision/i)).not.toBeInTheDocument()
  })

  it('hides Approve/Reject controls and shows a read-only notice for an admin with only moderation.appeal.read', () => {
    useAuthMock.mockReturnValue({ capabilities: ['moderation.appeal.read'] })

    render(
      <AppealDetailModal
        isOpen={true}
        onClose={vi.fn()}
        appeal={makeAppeal()}
        onReviewComplete={vi.fn()}
      />
    )

    expect(screen.queryByRole('button', { name: /Approve Appeal/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Reject Appeal/i })).not.toBeInTheDocument()
    expect(screen.getByText(/do not have permission to submit a decision/i)).toBeInTheDocument()
  })

  it('still allows the full review flow for an admin with both read and review', async () => {
    useAuthMock.mockReturnValue({ capabilities: ['moderation.appeal.read', 'moderation.appeal.review'] })
    const reviewAppeal = vi.fn().mockResolvedValue(undefined)
    useAppealReviewMock.mockReturnValue({ reviewAppeal, loading: false })

    const user = userEvent.setup()
    const onReviewComplete = vi.fn()

    render(
      <AppealDetailModal
        isOpen={true}
        onClose={vi.fn()}
        appeal={makeAppeal()}
        onReviewComplete={onReviewComplete}
      />
    )

    await user.click(screen.getByRole('button', { name: /Approve Appeal/i }))
    await user.click(screen.getByRole('button', { name: /Confirm Approve Appeal/i }))

    expect(reviewAppeal).toHaveBeenCalledWith('appeal-1', {
      decision: 'approve',
      admin_response: undefined,
    })
  })

  it('does not show the admin-response input for a read-only appeal admin', () => {
    useAuthMock.mockReturnValue({ capabilities: ['moderation.appeal.read'] })

    render(
      <AppealDetailModal
        isOpen={true}
        onClose={vi.fn()}
        appeal={makeAppeal()}
        onReviewComplete={vi.fn()}
      />
    )

    expect(screen.queryByPlaceholderText(/Provide a response to the user/i)).not.toBeInTheDocument()
  })
})
