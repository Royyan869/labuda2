import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { PaymentMethodsPage } from './PaymentMethodsPage'
import type { PaymentMethodItem } from '@/types/payment-methods'

// ============================================================================
// PaymentMethodsPage tests (PASS_19A rate-source verification labeling)
// ============================================================================

const getPaymentMethodsMock = vi.hoisted(() => vi.fn())
const updatePaymentMethodMock = vi.hoisted(() => vi.fn())
const previewPaymentMethodFeeMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api/paymentMethods', () => ({
  getPaymentMethods: getPaymentMethodsMock,
  updatePaymentMethod: updatePaymentMethodMock,
  previewPaymentMethodFee: previewPaymentMethodFeeMock,
}))

function makeMethod(overrides: Partial<PaymentMethodItem> = {}): PaymentMethodItem {
  return {
    method_code: 'bank_transfer',
    display_name: 'Transfer Bank (Virtual Account)',
    enabled: true,
    fee_type: 'flat',
    flat_amount_rupiah: 4000,
    percent_bps: 0,
    midtrans_channels: ['bca_va'],
    sort_order: 10,
    rate_source: 'public_baseline',
    ...overrides,
  }
}

describe('PaymentMethodsPage (PASS_19A)', () => {
  beforeEach(() => {
    getPaymentMethodsMock.mockReset()
    updatePaymentMethodMock.mockReset()
    previewPaymentMethodFeeMock.mockReset()
  })

  it('shows a rate source badge per method and the no-merchant-verified warning', async () => {
    getPaymentMethodsMock.mockResolvedValue({
      methods: [makeMethod({ method_code: 'bank_transfer' }), makeMethod({ method_code: 'qris', rate_source: 'manual_override' })],
      count: 2,
    })

    render(<PaymentMethodsPage />)

    await waitFor(() => expect(screen.getAllByText('Public Baseline').length).toBeGreaterThan(0))
    expect(screen.getByText('Manual Override')).toBeInTheDocument()
    expect(screen.getByText(/Belum ada metode dengan rate merchant-verified/)).toBeInTheDocument()
  })

  it('hides the warning once at least one method is merchant verified', async () => {
    getPaymentMethodsMock.mockResolvedValue({
      methods: [makeMethod({ rate_source: 'merchant_verified' })],
      count: 1,
    })

    render(<PaymentMethodsPage />)

    await waitFor(() => expect(screen.getByText('Merchant Verified')).toBeInTheDocument())
    expect(screen.queryByText(/Belum ada metode dengan rate merchant-verified/)).not.toBeInTheDocument()
  })

  it('does not claim public-baseline rates are Labuda merchant-final', async () => {
    getPaymentMethodsMock.mockResolvedValue({ methods: [makeMethod()], count: 1 })

    render(<PaymentMethodsPage />)

    await waitFor(() => expect(screen.getByText('Public Baseline')).toBeInTheDocument())
    expect(screen.queryByText(/rate resmi kontrak Labuda/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/final Midtrans merchant Labuda/i)).not.toBeInTheDocument()
  })
})
