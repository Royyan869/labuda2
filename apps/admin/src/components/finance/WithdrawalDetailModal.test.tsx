import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { WithdrawalDetailModal } from './WithdrawalDetailModal'
import type { WithdrawalDetail } from '@/types'

const useAuthMock = vi.fn()
const useWithdrawalDetailMock = vi.fn()
const useWithdrawalActionsMock = vi.fn()

vi.mock('@/hooks/useAuth', () => ({
  useAuth: () => useAuthMock(),
}))

vi.mock('@/hooks/useWithdrawals', () => ({
  useWithdrawalDetail: (...args: unknown[]) => useWithdrawalDetailMock(...args),
  useWithdrawalActions: (...args: unknown[]) => useWithdrawalActionsMock(...args),
}))

function makeWithdrawal(overrides: Partial<WithdrawalDetail> = {}): WithdrawalDetail {
  return {
    id: 'wd_123',
    seller_id: 'seller_1',
    amount: 250000,
    fee_amount: 500000,
    total_debit_amount: 750000,
    status: 'REQUESTED',
    bank_name_snapshot: 'BCA',
    account_number_snapshot: '1234567890',
    account_holder_snapshot: 'Seller One',
    created_at: '2026-01-01T00:00:00.000Z',
    updated_at: '2026-01-01T00:00:00.000Z',
    external_reference_id: undefined,
    gateway_reference_id: undefined,
    submitted_at: null,
    settled_at: null,
    failure_reason: null,
    retry_count: 0,
    seller_username: 'sellerone',
    seller_farm_name: 'Farm One',
    seller_avatar: null,
    seller_email: 'seller@example.com',
    ...overrides,
  }
}

describe('WithdrawalDetailModal', () => {
  beforeEach(() => {
    useAuthMock.mockReturnValue({
      user: { id: 'admin_1' },
      capabilities: ['finance.withdraw.review'],
    })
    useWithdrawalDetailMock.mockReturnValue({
      withdrawal: null,
      loading: false,
      refetch: vi.fn(),
    })
    useWithdrawalActionsMock.mockReturnValue({
      approve: vi.fn(),
      reject: vi.fn(),
      markProcessed: vi.fn(),
      loading: false,
      error: null,
      clearError: vi.fn(),
    })
  })

  it('shows approve and reject actions for requested withdrawals', () => {
    render(
      <WithdrawalDetailModal
        isOpen
        onClose={vi.fn()}
        withdrawalData={makeWithdrawal({ status: 'REQUESTED' })}
        onSuccess={vi.fn()}
      />
    )

    expect(screen.getByText('Approve')).toBeInTheDocument()
    expect(screen.getByText('Reject')).toBeInTheDocument()
    expect(screen.queryByText('Mark as Paid')).not.toBeInTheDocument()
  })

  it('shows manual completion only for processing withdrawals without a gateway ref', () => {
    render(
      <WithdrawalDetailModal
        isOpen
        onClose={vi.fn()}
        withdrawalData={makeWithdrawal({ status: 'PROCESSING' })}
        onSuccess={vi.fn()}
      />
    )

    expect(screen.getByText('Mark as Paid')).toBeInTheDocument()
    expect(screen.queryByText(/submitted to the payment gateway/i)).not.toBeInTheDocument()
  })

  it('hides manual completion once a gateway reference exists', () => {
    render(
      <WithdrawalDetailModal
        isOpen
        onClose={vi.fn()}
        withdrawalData={makeWithdrawal({
          status: 'PROCESSING',
          external_reference_id: 'WD_abc_123',
          gateway_reference_id: 'GW_abc_123',
        })}
        onSuccess={vi.fn()}
      />
    )

    expect(screen.queryByText('Mark as Paid')).not.toBeInTheDocument()
    expect(screen.getByText(/submitted to the payment gateway/i)).toBeInTheDocument()
    expect(screen.getByText('Payment Gateway Reference')).toBeInTheDocument()
  })
})
