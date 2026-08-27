import { describe, expect, it, vi, beforeEach } from 'vitest'

const apiGetMock = vi.hoisted(() => vi.fn())

vi.mock('./client', () => ({
  api: {
    get: apiGetMock,
  },
}))

import { getFinanceSummary } from './financeSummary'

describe('finance summary API client (PASS_18Z)', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
  })

  it('fetches the finance summary from the correct endpoint', async () => {
    apiGetMock.mockResolvedValue({
      generated_at: '2026-07-06T00:00:00Z',
      system_account_balances: { GATEWAY_CLEARING: 125000 },
      aggregate_user_account_balances: [],
      gateway_clearing: { balance_rupiah: 125000, is_zero: false, note: 'x' },
      revenue_breakdown: {
        available: true,
        buyer_payment_fee_revenue_rupiah: 4000,
        commission_revenue_rupiah: 5000,
        other_revenue_rupiah: 0,
        total_platform_revenue_rupiah: 9000,
        note: 'x',
      },
      internal_reconciliation: { available: false, note: 'x' },
      external_reconciliation: {
        external_gateway_settlement_reconciliation: 'not_implemented',
        bank_statement_reconciliation: 'not_implemented',
        note: 'x',
      },
      finance_alerts: { unresolved_total: 0, unresolved_critical_total: 0, payment_captured_after_expiry_count: 0 },
    })

    const result = await getFinanceSummary()

    expect(apiGetMock).toHaveBeenCalledWith('/api/v1/admin/finance/summary')
    expect(result.gateway_clearing.balance_rupiah).toBe(125000)
    // External reconciliation must never be reported as anything other than
    // not_implemented — this is a compile-time guarantee via the literal
    // union type, re-asserted here at runtime against the actual response.
    expect(result.external_reconciliation.external_gateway_settlement_reconciliation).toBe('not_implemented')
    expect(result.external_reconciliation.bank_statement_reconciliation).toBe('not_implemented')
  })
})
