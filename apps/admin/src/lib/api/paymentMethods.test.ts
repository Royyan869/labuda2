import { describe, expect, it, vi, beforeEach } from 'vitest'

const apiGetMock = vi.hoisted(() => vi.fn())
const apiPutMock = vi.hoisted(() => vi.fn())
const apiPostMock = vi.hoisted(() => vi.fn())

vi.mock('./client', () => ({
  api: {
    get: apiGetMock,
    put: apiPutMock,
    post: apiPostMock,
  },
}))

import { getPaymentMethods, getPaymentMethod, updatePaymentMethod, previewPaymentMethodFee } from './paymentMethods'

describe('payment methods API client (PASS_18W)', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
    apiPutMock.mockReset()
    apiPostMock.mockReset()
  })

  it('lists all payment methods', async () => {
    apiGetMock.mockResolvedValue({ data: { methods: [], count: 0 } })

    const result = await getPaymentMethods()

    expect(apiGetMock).toHaveBeenCalledWith('/api/v1/admin/payment-methods')
    expect(result).toEqual({ methods: [], count: 0 })
  })

  it('gets a single method by code, URL-encoded', async () => {
    apiGetMock.mockResolvedValue({ data: { method: { method_code: 'bank transfer' } } })

    await getPaymentMethod('bank transfer')

    expect(apiGetMock).toHaveBeenCalledWith('/api/v1/admin/payment-methods/bank%20transfer')
  })

  it('updates a method with the given payload, never including method_code in the body', async () => {
    const payload = {
      display_name: 'Transfer Bank',
      enabled: true,
      fee_type: 'flat' as const,
      flat_amount_rupiah: 4000,
      percent_bps: 0,
      midtrans_channels: ['bca_va'],
      sort_order: 10,
      rate_source: 'public_baseline' as const,
    }
    apiPutMock.mockResolvedValue({ data: { method: { ...payload, method_code: 'bank_transfer' }, message: 'ok' } })

    await updatePaymentMethod('bank_transfer', payload)

    expect(apiPutMock).toHaveBeenCalledWith('/api/v1/admin/payment-methods/bank_transfer', payload)
    // Structural proof: the payload sent to the backend has no method_code field —
    // it is immutable and comes from the URL, not the body (PASS_18W doctrine).
    expect(Object.keys(payload)).not.toContain('method_code')
  })

  it('previews a fee calculation against the given base amount', async () => {
    apiPostMock.mockResolvedValue({
      data: {
        method_code: 'qris',
        base_amount_rupiah: 103000,
        buyer_payment_fee_rupiah: 721,
        gross_amount_rupiah: 103721,
        clamped: false,
        formula: 'ceil(base * 70 bps)',
      },
    })

    const result = await previewPaymentMethodFee('qris', {
      fee_type: 'percent',
      percent_bps: 70,
      base_amount_rupiah: 103000,
    })

    expect(apiPostMock).toHaveBeenCalledWith('/api/v1/admin/payment-methods/qris/preview', {
      fee_type: 'percent',
      percent_bps: 70,
      base_amount_rupiah: 103000,
    })
    expect(result.buyer_payment_fee_rupiah).toBe(721)
  })
})
