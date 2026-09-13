import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'

const getFailedDeliveriesMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  getFailedDeliveries: getFailedDeliveriesMock,
}))

import { useFailedDeliveries } from './useFailedDeliveries'

/**
 * Regression guard for the "Loading failed deliveries..." never-ending spinner.
 *
 * FailedDeliveriesPage used to compute `since` with `new Date()` during render
 * and pass the resulting ISO string into this hook. That string changed on
 * every render, which changed fetchDeliveries' identity on every render, so
 * the effect re-ran forever and `loading` never settled. The hook now takes a
 * scalar `sinceHours` and resolves the timestamp at request time.
 */
describe('useFailedDeliveries (infinite-refetch regression)', () => {
  beforeEach(() => {
    getFailedDeliveriesMock.mockReset()
    getFailedDeliveriesMock.mockResolvedValue({
      deliveries: [],
      meta: { page: 1, per_page: 20, total: 0, total_pages: 0 },
    })
  })

  it('fetches once and reaches a terminal loading state', async () => {
    const { result } = renderHook(() => useFailedDeliveries({ sinceHours: 24 }))

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(getFailedDeliveriesMock).toHaveBeenCalledTimes(1)
    expect(result.current.error).toBeNull()
  })

  it('resolves the lookback window to a fresh ISO timestamp at request time', async () => {
    const { result } = renderHook(() => useFailedDeliveries({ sinceHours: 1 }))

    await waitFor(() => expect(result.current.loading).toBe(false))

    const call = getFailedDeliveriesMock.mock.calls[0][0] as { since: string }
    expect(Number.isNaN(Date.parse(call.since))).toBe(false)
    expect(Date.now() - Date.parse(call.since)).toBeLessThan(2 * 60 * 60 * 1000)
  })

  it('refetches once when the lookback window changes, then settles', async () => {
    const { result, rerender } = renderHook(
      ({ hours }: { hours: number }) => useFailedDeliveries({ sinceHours: hours }),
      { initialProps: { hours: 24 } }
    )

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(getFailedDeliveriesMock).toHaveBeenCalledTimes(1)

    rerender({ hours: 6 })

    await waitFor(() => expect(getFailedDeliveriesMock).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(getFailedDeliveriesMock).toHaveBeenCalledTimes(2)
  })
})
