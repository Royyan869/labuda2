import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { FinanceSummaryPanel } from './FinanceSummaryPanel'
import type { FinanceSummaryResponse } from '@/types/finance-summary'

const getFinanceSummaryMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  getFinanceSummary: getFinanceSummaryMock,
}))

function makeSummary(overrides: Partial<FinanceSummaryResponse> = {}): FinanceSummaryResponse {
  return {
    generated_at: '2026-07-06T00:00:00.000Z',
    system_account_balances: { GATEWAY_CLEARING: 0 },
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

// PASS_20F: this is the direct regression test for the Reconciliation page
// crash — "Cannot read properties of null (reading 'map')". Root cause:
// backend's queryUserAccountAggregates declared `var out []userAccountAggregate`
// and never initialized it when financial_accounts had zero user-scoped rows
// (a real state in the owner's sandbox), so Go's encoding/json marshaled the
// nil slice as `null` instead of `[]`. This panel then called
// `.map()` on that null directly. Backend now initializes the slice; this
// test guards the frontend side defensively in case any other caller ever
// returns null again.
describe('FinanceSummaryPanel (PASS_20F null-map regression)', () => {
  beforeEach(() => {
    getFinanceSummaryMock.mockReset()
  })

  it('does not crash when aggregate_user_account_balances is null', async () => {
    getFinanceSummaryMock.mockResolvedValue(
      makeSummary({ aggregate_user_account_balances: null as unknown as [] })
    )

    render(<FinanceSummaryPanel />)

    expect(await screen.findByText('Account Balances')).toBeInTheDocument()
  })

  it('renders normally when aggregate_user_account_balances is an empty array', async () => {
    getFinanceSummaryMock.mockResolvedValue(makeSummary())

    render(<FinanceSummaryPanel />)

    expect(await screen.findByText('Account Balances')).toBeInTheDocument()
  })
})
