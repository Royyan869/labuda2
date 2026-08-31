/**
 * GovernanceCaseDetailPage capability-gating tests
 *
 * Proves:
 *   - read-only admin (moderation.case.read only) → Create Decision NOT visible
 *   - governance admin (moderation.case.read + moderation.case.resolve) → Create Decision visible
 *   - read-only admin sees read-only notice
 */
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { GovernanceCaseDetailPage } from './GovernanceCaseDetailPage'

// ── Mocks ──────────────────────────────────────────────────

const useGovernanceCaseMock = vi.hoisted(() => vi.fn())
const useCreateDecisionMock = vi.hoisted(() => vi.fn())
const useGovernanceCaseAuditMock = vi.hoisted(() => vi.fn())
const useAuthMock = vi.hoisted(() => vi.fn())

vi.mock('@/hooks/useGovernance', () => ({
  useGovernanceCase: useGovernanceCaseMock,
  useCreateDecision: useCreateDecisionMock,
  useGovernanceCaseAudit: useGovernanceCaseAuditMock,
}))

vi.mock('@/hooks/useAuth', () => ({
  useAuth: useAuthMock,
}))

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    useParams: () => ({ id: 'case-abc-123' }),
    useNavigate: () => vi.fn(),
  }
})

// ── Fixtures ───────────────────────────────────────────────

function makeCaseDetail() {
  return {
    case: {
      id: 'case-abc-123',
      subject_type: 'content',
      subject_id: 'content-xyz',
      status: 'open',
      created_at: '2026-09-01T00:00:00Z',
      updated_at: '2026-09-01T00:00:00Z',
    },
    reports: [
      {
        id: 'report-1',
        reporter_id: 'user-1',
        subject_type: 'content',
        subject_id: 'content-xyz',
        reason_code: 'prohibited_content',
        created_at: '2026-09-01T00:00:00Z',
      },
    ],
    decisions: [],
  }
}

// ── Tests ──────────────────────────────────────────────────

describe('GovernanceCaseDetailPage — capability gating', () => {
  beforeEach(() => {
    useGovernanceCaseMock.mockReturnValue({
      data: makeCaseDetail(),
      loading: false,
      error: null,
      refetch: vi.fn(),
    })
    useCreateDecisionMock.mockReturnValue({
      createDecision: vi.fn(),
      loading: false,
      error: null,
    })
    useGovernanceCaseAuditMock.mockReturnValue({
      events: [],
      loading: false,
      error: null,
      count: 0,
      refetch: vi.fn(),
    })
  })

  it('shows Create Decision button for admin with moderation.case.resolve', () => {
    useAuthMock.mockReturnValue({
      capabilities: ['moderation.case.read', 'moderation.case.resolve'],
    })

    render(
      <MemoryRouter initialEntries={['/moderation/cases/case-abc-123']}>
        <GovernanceCaseDetailPage />
      </MemoryRouter>
    )

    expect(screen.getByRole('button', { name: /Create Decision/i })).toBeInTheDocument()
    expect(screen.queryByText(/do not have permission to create decisions/i)).not.toBeInTheDocument()
  })

  it('hides Create Decision button for admin with only moderation.case.read', () => {
    useAuthMock.mockReturnValue({
      capabilities: ['moderation.case.read'],
    })

    render(
      <MemoryRouter initialEntries={['/moderation/cases/case-abc-123']}>
        <GovernanceCaseDetailPage />
      </MemoryRouter>
    )

    expect(screen.queryByRole('button', { name: /Create Decision/i })).not.toBeInTheDocument()
    expect(screen.getByText(/do not have permission to create decisions/i)).toBeInTheDocument()
  })

  it('hides Create Decision button when case is resolved (regardless of capability)', () => {
    useAuthMock.mockReturnValue({
      capabilities: ['moderation.case.read', 'moderation.case.resolve'],
    })
    useGovernanceCaseMock.mockReturnValue({
      data: {
        ...makeCaseDetail(),
        case: { ...makeCaseDetail().case, status: 'resolved' },
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    })

    render(
      <MemoryRouter initialEntries={['/moderation/cases/case-abc-123']}>
        <GovernanceCaseDetailPage />
      </MemoryRouter>
    )

    expect(screen.queryByRole('button', { name: /Create Decision/i })).not.toBeInTheDocument()
  })

  it('shows case detail and reports for read-only admin', () => {
    useAuthMock.mockReturnValue({
      capabilities: ['moderation.case.read'],
    })

    render(
      <MemoryRouter initialEntries={['/moderation/cases/case-abc-123']}>
        <GovernanceCaseDetailPage />
      </MemoryRouter>
    )

    expect(screen.getByText('Case Detail')).toBeInTheDocument()
    expect(screen.getByText('Reports (1)')).toBeInTheDocument()
    expect(screen.getByText('Decisions (0)')).toBeInTheDocument()
  })
})
