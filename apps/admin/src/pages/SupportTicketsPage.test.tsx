import { MemoryRouter } from 'react-router-dom'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { RequireCapability } from '@/components/auth/RequireCapability'
import { SupportTicketsPage } from './SupportTicketsPage'

const useSupportTicketsMock = vi.hoisted(() => vi.fn())
const useAuthMock = vi.hoisted(() => vi.fn())
const navigateMock = vi.hoisted(() => vi.fn())

vi.mock('@/hooks/useSupport', () => ({
  useSupportTickets: useSupportTicketsMock,
}))

vi.mock('@/hooks/useAuth', () => ({
  useAuth: useAuthMock,
}))

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => navigateMock,
  }
})

function renderSupportPage() {
  return render(
    <MemoryRouter>
      <RequireCapability cap="support.ticket.read">
        <SupportTicketsPage />
      </RequireCapability>
    </MemoryRouter>
  )
}

function makeTicket() {
  return {
    id: 'ticket-1',
    user_id: 'user-1',
    username: 'support-user',
    seller_farm_name: 'Green Farm',
    category: 'order_issue' as const,
    status: 'open' as const,
    priority: 'high' as const,
    escalation: 'none' as const,
    subject: 'Order has not arrived',
    order_id: 'order-1',
    created_at: '2026-07-14T00:00:00.000Z',
    updated_at: '2026-07-14T00:05:00.000Z',
    sla: {
      first_response_time_seconds: 120,
      first_response_overdue: false,
      resolution_time_seconds: null,
      resolution_overdue: false,
      is_overdue: false,
      next_action: 'reply' as const,
    },
    user_avatar: null,
  }
}

describe('SupportTicketsPage', () => {
  beforeEach(() => {
    useSupportTicketsMock.mockReset()
    useAuthMock.mockReset()
    navigateMock.mockReset()
  })

  it('blocks the page when the user lacks support.ticket.read', () => {
    useAuthMock.mockReturnValue({
      user: { id: 'admin-1' },
      capabilities: ['governance.dashboard.view'],
    })
    useSupportTicketsMock.mockReturnValue({
      tickets: [],
      loading: false,
      error: null,
      total: 0,
      refetch: vi.fn(),
    })

    renderSupportPage()

    expect(screen.getByText('Access Denied')).toBeInTheDocument()
    expect(screen.getByText('support.ticket.read')).toBeInTheDocument()
  })

  it('loads and displays the current ticket data for an authorized admin', () => {
    useAuthMock.mockReturnValue({
      user: { id: 'admin-1' },
      capabilities: ['support.ticket.read'],
    })
    useSupportTicketsMock.mockReturnValue({
      tickets: [makeTicket()],
      loading: false,
      error: null,
      total: 1,
      refetch: vi.fn(),
    })

    renderSupportPage()

    expect(screen.getByText('Order has not arrived')).toBeInTheDocument()
    expect(screen.getByText('@support-user')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'View' })).toBeInTheDocument()
  })

  it('renders the error state instead of an empty-success state when loading fails', () => {
    useAuthMock.mockReturnValue({
      user: { id: 'admin-1' },
      capabilities: ['support.ticket.read'],
    })
    useSupportTicketsMock.mockReturnValue({
      tickets: [],
      loading: false,
      error: new Error('HTTP 500: support tickets unavailable'),
      total: 0,
      refetch: vi.fn(),
    })

    renderSupportPage()

    expect(screen.getByText(/Error loading tickets/i)).toBeInTheDocument()
    expect(screen.queryByText('No support tickets in the system.')).not.toBeInTheDocument()
  })
})
