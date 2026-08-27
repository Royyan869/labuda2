import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'

const apiGetMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  api: {
    get: apiGetMock,
  },
}))

import { useWithdrawals, useWithdrawalDetail } from './useWithdrawals'

describe('useWithdrawals (PASS_20E envelope unwrap regression)', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
  })

  it('unwraps the {data, meta} envelope backend actually returns', async () => {
    apiGetMock.mockResolvedValue({
      success: true,
      data: { withdrawals: [{ id: 'w1', seller_id: 's1', amount: 10000 }] },
      meta: { page: 1, per_page: 20, total: 1, total_pages: 1 },
      timestamp: '2026-01-01T00:00:00Z',
    })

    const { result } = renderHook(() => useWithdrawals())

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.withdrawals).toHaveLength(1)
    expect(result.current.withdrawals[0].id).toBe('w1')
    expect(result.current.total).toBe(1)
    expect(result.current.totalPages).toBe(1)
  })

  it('renders an empty list (not an error) when the envelope has zero withdrawals', async () => {
    apiGetMock.mockResolvedValue({
      success: true,
      data: { withdrawals: [] },
      meta: { page: 1, per_page: 20, total: 0, total_pages: 0 },
      timestamp: '2026-01-01T00:00:00Z',
    })

    const { result } = renderHook(() => useWithdrawals())

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.withdrawals).toEqual([])
    expect(result.current.total).toBe(0)
    expect(result.current.error).toBeNull()
  })
})

describe('useWithdrawalDetail (PASS_20E envelope unwrap regression)', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
  })

  it('unwraps data.withdrawal from the detail envelope', async () => {
    apiGetMock.mockResolvedValue({
      success: true,
      data: { withdrawal: { id: 'w1', seller_id: 's1', amount: 5000 } },
      timestamp: '2026-01-01T00:00:00Z',
    })

    const { result } = renderHook(() => useWithdrawalDetail('w1'))

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.withdrawal?.id).toBe('w1')
  })
})
