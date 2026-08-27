import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { CaseDetailModal } from './CaseDetailModal'
import type { ModerationCase } from '@/types/moderation'

const useModerationCaseMock = vi.hoisted(() => vi.fn())
const useAuthMock = vi.hoisted(() => vi.fn())
const getModerationCaseEvidenceMock = vi.hoisted(() => vi.fn())

vi.mock('@/hooks/useModeration', () => ({
  useModerationCase: useModerationCaseMock,
}))

vi.mock('@/hooks/useAuth', () => ({
  useAuth: useAuthMock,
}))

vi.mock('@/lib/api/moderation', () => ({
  getModerationCaseEvidence: getModerationCaseEvidenceMock,
}))

function makeCase(overrides: Partial<ModerationCase> = {}): ModerationCase {
  return {
    id: 'case-1',
    resource_type: 'chat_message',
    resource_id: 'message-1',
    status: 'pending',
    reported_by: 'reporter-1',
    reason: 'spam',
    created_at: '2026-07-14T00:00:00.000Z',
    ...overrides,
  }
}

function makeCaseDetail() {
  return {
    ...makeCase(),
    resource_preview: {
      author_id: 'sender-1',
      author_username: 'hidden-user',
      content_type: 'text',
      is_deleted: true,
      evidence_available: true,
      evidence_requires_capability: 'moderation.evidence.read',
      room_id: 'room-1',
      room_type: 'normal',
      sent_at: '2026-07-14T00:01:00.000Z',
    },
  }
}

describe('CaseDetailModal - evidence access flow', () => {
  beforeEach(() => {
    useModerationCaseMock.mockReturnValue({
      caseDetail: makeCaseDetail(),
      loading: false,
      refetch: vi.fn(),
    })
    useAuthMock.mockReturnValue({ capabilities: ['moderation.case.read', 'moderation.evidence.read'] })
    getModerationCaseEvidenceMock.mockReset()
  })

  it('shows the evidence button only for admins with moderation.evidence.read', () => {
    useAuthMock.mockReturnValue({ capabilities: ['moderation.case.read'] })

    render(
      <CaseDetailModal
        isOpen={true}
        onClose={vi.fn()}
        caseData={makeCase()}
        onAction={vi.fn()}
      />
    )

    expect(screen.queryByRole('button', { name: /Lihat bukti asli/i })).not.toBeInTheDocument()
  })

  it('fetches evidence only after explicit click and renders it separately', async () => {
    getModerationCaseEvidenceMock.mockResolvedValue({
      case_id: 'case-1',
      resource_type: 'chat_message',
      resource_id: 'message-1',
      message_id: 'message-1',
      room_id: 'room-1',
      room_type: 'normal',
      sender_id: 'sender-1',
      author_username: 'hidden-user',
      created_at: '2026-07-14T00:01:00.000Z',
      original_body: 'hidden original body',
      original_attachment: { media_url: 'https://cdn.example.com/evidence.png' },
    })

    const user = userEvent.setup()

    render(
      <CaseDetailModal
        isOpen={true}
        onClose={vi.fn()}
        caseData={makeCase()}
        onAction={vi.fn()}
      />
    )

    expect(getModerationCaseEvidenceMock).not.toHaveBeenCalled()
    expect(screen.queryByText(/hidden original body/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/media_url/i)).not.toBeInTheDocument()
    expect(screen.getByText(/Access to this evidence is audited/i)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Lihat bukti asli/i }))

    expect(getModerationCaseEvidenceMock).toHaveBeenCalledWith('case-1')
    expect(await screen.findByText(/Original Hidden Evidence/i)).toBeInTheDocument()
    expect(screen.getByText(/hidden original body/i)).toBeInTheDocument()
    expect(screen.getByText(/media_url/i)).toBeInTheDocument()
  })

  it('clears evidence state when the modal closes', async () => {
    getModerationCaseEvidenceMock.mockResolvedValue({
      case_id: 'case-1',
      resource_type: 'chat_message',
      resource_id: 'message-1',
      message_id: 'message-1',
      room_id: 'room-1',
      room_type: 'normal',
      sender_id: 'sender-1',
      created_at: '2026-07-14T00:01:00.000Z',
      original_body: 'hidden original body',
      original_attachment: null,
    })

    const user = userEvent.setup()
    const { rerender } = render(
      <CaseDetailModal
        isOpen={true}
        onClose={vi.fn()}
        caseData={makeCase()}
        onAction={vi.fn()}
      />
    )

    await user.click(screen.getByRole('button', { name: /Lihat bukti asli/i }))
    expect(await screen.findByText(/Original Hidden Evidence/i)).toBeInTheDocument()

    rerender(
      <CaseDetailModal
        isOpen={false}
        onClose={vi.fn()}
        caseData={makeCase()}
        onAction={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.queryByText(/Original Hidden Evidence/i)).not.toBeInTheDocument()
    })

    rerender(
      <CaseDetailModal
        isOpen={true}
        onClose={vi.fn()}
        caseData={makeCase()}
        onAction={vi.fn()}
      />
    )

    expect(screen.queryByText(/Original Hidden Evidence/i)).not.toBeInTheDocument()
  })

  it('shows an error when evidence fetch fails and does not fall back to normal content', async () => {
    getModerationCaseEvidenceMock.mockRejectedValue(new Error('Access denied. You do not have sufficient permissions for this action.'))

    const user = userEvent.setup()

    render(
      <CaseDetailModal
        isOpen={true}
        onClose={vi.fn()}
        caseData={makeCase()}
        onAction={vi.fn()}
      />
    )

    await user.click(screen.getByRole('button', { name: /Lihat bukti asli/i }))

    expect(await screen.findByText(/Access denied/i)).toBeInTheDocument()
    expect(screen.queryByText(/hidden original body/i)).not.toBeInTheDocument()
  })
})
