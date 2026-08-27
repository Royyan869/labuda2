import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FinanceReconciliationPage } from './FinanceReconciliationPage'
import type { ReconciliationResult } from '@/types/reconciliation'
import type { FinanceSummaryResponse } from '@/types/finance-summary'

const getReconciliationResultsMock = vi.hoisted(() => vi.fn())
const getLatestReconciliationResultMock = vi.hoisted(() => vi.fn())
const getFinanceSummaryMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api/reconciliation', () => ({
  getReconciliationResults: getReconciliationResultsMock,
  getLatestReconciliationResult: getLatestReconciliationResultMock,
}))

// PASS_18Z: FinanceReconciliationPage now also renders FinanceSummaryPanel,
// which calls getFinanceSummary(). Mocked here (like the two reconciliation
// calls above) so this page's own tests stay isolated from that dependency —
// otherwise an unmocked/rejected call would render a second, ambiguous
// "Retry" button and break the retry test below.
vi.mock('@/lib/api/financeSummary', () => ({
  getFinanceSummary: getFinanceSummaryMock,
}))

function makeFinanceSummary(overrides: Partial<FinanceSummaryResponse> = {}): FinanceSummaryResponse {
  return {
    generated_at: '2026-06-01T00:00:00.000Z',
    system_account_balances: { GATEWAY_CLEARING: 0, PLATFORM_REVENUE: 0, BANK_SETTLEMENT: 0 },
    aggregate_user_account_balances: [],
    gateway_clearing: { balance_rupiah: 0, is_zero: true, note: 'note' },
    revenue_breakdown: {
      available: true,
      buyer_payment_fee_revenue_rupiah: 0,
      commission_revenue_rupiah: 0,
      other_revenue_rupiah: 0,
      total_platform_revenue_rupiah: 0,
      note: 'note',
    },
    internal_reconciliation: { available: false, note: 'no runs yet' },
    external_reconciliation: {
      external_gateway_settlement_reconciliation: 'not_implemented',
      bank_statement_reconciliation: 'not_implemented',
      note: 'note',
    },
    finance_alerts: { unresolved_total: 0, unresolved_critical_total: 0, payment_captured_after_expiry_count: 0 },
    ...overrides,
  }
}

function makeResult(overrides: Partial<ReconciliationResult> = {}): ReconciliationResult {
  return {
    id: 'recon-1',
    checked_at: '2026-06-01T00:00:00.000Z',
    severity: 'passed',
    action_taken: 'none',
    auto_repaired: false,
    total_accounts: 0,
    mismatched_accounts: 0,
    details: {},
    created_at: '2026-06-01T00:00:00.000Z',
    ...overrides,
  }
}

describe('FinanceReconciliationPage', () => {
  beforeEach(() => {
    getReconciliationResultsMock.mockReset()
    getLatestReconciliationResultMock.mockReset()
    getFinanceSummaryMock.mockReset()
    getReconciliationResultsMock.mockResolvedValue({
      results: [makeResult()],
      total: 1,
      limit: 50,
      offset: 0,
    })
    getLatestReconciliationResultMock.mockResolvedValue(makeResult())
    getFinanceSummaryMock.mockResolvedValue(makeFinanceSummary())
  })

  it('renders a passed run from the results list', async () => {
    render(<FinanceReconciliationPage />)

    expect(await screen.findByText(/0\/0 accounts mismatched/)).toBeInTheDocument()
  })

  it('surfaces the latest run as a healthy/passed signal', async () => {
    render(<FinanceReconciliationPage />)

    await waitFor(() => {
      expect(getLatestReconciliationResultMock).toHaveBeenCalled()
    })
    expect(await screen.findByText(/Worker is alive/)).toBeInTheDocument()
  })

  it('shows an empty state when no runs match the filters', async () => {
    getReconciliationResultsMock.mockResolvedValue({ results: [], total: 0, limit: 50, offset: 0 })

    render(<FinanceReconciliationPage />)

    expect(await screen.findByText('No Reconciliation Runs')).toBeInTheDocument()
  })

  it('shows an error state and allows retry when the list call fails', async () => {
    getReconciliationResultsMock.mockRejectedValueOnce(new Error('reconciliation unavailable'))
    getReconciliationResultsMock.mockResolvedValueOnce({
      results: [makeResult()],
      total: 1,
      limit: 50,
      offset: 0,
    })

    const user = userEvent.setup()
    render(<FinanceReconciliationPage />)

    expect(await screen.findByText('reconciliation unavailable')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByText(/0\/0 accounts mismatched/)).toBeInTheDocument()
  })

  it('re-fetches with the selected severity filter', async () => {
    const user = userEvent.setup()
    render(<FinanceReconciliationPage />)

    await screen.findByText(/0\/0 accounts mismatched/)
    getReconciliationResultsMock.mockClear()

    await user.selectOptions(screen.getByDisplayValue('All'), 'high')

    await waitFor(() => {
      expect(getReconciliationResultsMock).toHaveBeenCalledWith(
        expect.objectContaining({ severity: 'high' })
      )
    })
  })

  it('expands details for a run with a non-empty details payload', async () => {
    getReconciliationResultsMock.mockResolvedValue({
      results: [makeResult({
        severity: 'high',
        total_accounts: 2,
        mismatched_accounts: 1,
        details: { mismatched_accounts: 1, total_accounts: 2 },
      })],
      total: 1,
      limit: 50,
      offset: 0,
    })

    const user = userEvent.setup()
    render(<FinanceReconciliationPage />)

    const showDetails = await screen.findByText('Show details')
    await user.click(showDetails)

    expect(await screen.findByText(/"mismatched_accounts": 1/)).toBeInTheDocument()
  })

  it('never renders a mutation action (acknowledge/resolve/repair/delete)', async () => {
    render(<FinanceReconciliationPage />)

    await screen.findByText(/0\/0 accounts mismatched/)

    for (const label of ['Acknowledge', 'Resolve', 'Repair', 'Delete', 'False Positive', 'Cleanup']) {
      expect(screen.queryByRole('button', { name: label })).not.toBeInTheDocument()
    }
  })
})
