import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { DisputeWorkspacePage } from './DisputeWorkspacePage'
import type { DisputeDetail } from '@/types'

const useDisputeDetailMock = vi.hoisted(() => vi.fn())
const getOrderTimelineMock = vi.hoisted(() => vi.fn())
const navigateMock = vi.hoisted(() => vi.fn())
const refetchMock = vi.hoisted(() => vi.fn())

vi.mock('@/hooks/useDisputes', () => ({
  useDisputeDetail: useDisputeDetailMock,
}))

vi.mock('@/lib/api', () => ({
  getOrderTimeline: getOrderTimelineMock,
  approveDispute: vi.fn(),
  rejectDispute: vi.fn(),
  resolveDisputePartialSplit: vi.fn(),
}))

vi.mock('@/components/disputes/DisputeHeader', () => ({
  DisputeHeader: ({
    dispute,
    onRefresh,
  }: {
    dispute: DisputeDetail
    onRefresh: () => void
  }) => (
    <button type="button" onClick={onRefresh}>
      Refresh timeline for {dispute.order_id}
    </button>
  ),
}))

vi.mock('@/components/disputes/BuyerEvidencePanel', () => ({
  BuyerEvidencePanel: () => <div data-testid="buyer-evidence" />,
}))

vi.mock('@/components/disputes/SellerEvidencePanel', () => ({
  SellerEvidencePanel: () => <div data-testid="seller-evidence" />,
}))

vi.mock('@/components/orders/TimelinePanel', () => ({
  TimelinePanel: () => <div data-testid="timeline-panel" />,
}))

vi.mock('@/components/disputes/DecisionPanel', () => ({
  DecisionPanel: () => <div data-testid="decision-panel" />,
}))

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => navigateMock,
  }
})

function makeDispute(orderId: string): DisputeDetail {
  return {
    id: 'dispute-1',
    order_id: orderId,
    buyer_id: 'buyer-1',
    seller_id: 'seller-1',
    reason: 'item_not_received',
    status: 'under_review',
    opened_at: '2026-07-14T00:00:00.000Z',
    created_at: '2026-07-14T00:00:00.000Z',
    updated_at: '2026-07-14T00:00:00.000Z',
    admin_response_overdue: false,
    resolution_overdue: false,
    buyer_username: 'buyer',
    seller_username: 'seller',
    order_status: 'paid',
    order_escrow_status: 'holding',
  }
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/disputes/dispute-1']}>
      <Routes>
        <Route path="/disputes/:id" element={<DisputeWorkspacePage />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('DisputeWorkspacePage', () => {
  beforeEach(() => {
    useDisputeDetailMock.mockReset()
    getOrderTimelineMock.mockReset()
    navigateMock.mockReset()
    refetchMock.mockReset()
    refetchMock.mockResolvedValue(undefined)
  })

  it('uses the latest dispute order id when refreshing the timeline', async () => {
    let currentDispute = makeDispute('order-1')

    useDisputeDetailMock.mockImplementation(() => ({
      dispute: currentDispute,
      loading: false,
      refetch: refetchMock,
    }))
    getOrderTimelineMock.mockResolvedValue([])

    const user = userEvent.setup()
    const { rerender } = renderPage()

    await waitFor(() => expect(getOrderTimelineMock).toHaveBeenCalledWith('order-1'))

    currentDispute = makeDispute('order-2')
    rerender(
      <MemoryRouter initialEntries={['/disputes/dispute-1']}>
        <Routes>
          <Route path="/disputes/:id" element={<DisputeWorkspacePage />} />
        </Routes>
      </MemoryRouter>
    )

    await waitFor(() => expect(getOrderTimelineMock).toHaveBeenCalledWith('order-2'))

    getOrderTimelineMock.mockClear()
    await user.click(screen.getByRole('button', { name: /Refresh timeline for order-2/i }))

    await waitFor(() => expect(getOrderTimelineMock).toHaveBeenCalledWith('order-2'))
    expect(getOrderTimelineMock).not.toHaveBeenCalledWith('order-1')
  })
})
