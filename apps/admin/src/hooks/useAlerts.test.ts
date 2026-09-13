import { renderHook, waitFor, act } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'

const getAlertsMock = vi.hoisted(() => vi.fn())
const getAlertStatsMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  getAlerts: getAlertsMock,
  getAlertStats: getAlertStatsMock,
  acknowledgeAlert: vi.fn(),
  resolveAlert: vi.fn(),
  markAlertAsFalsePositive: vi.fn(),
  cleanupAlerts: vi.fn(),
}))

import { useAlerts, useAlertStats } from './useAlerts'

/**
 * Regression guard for the Alerts Refresh button that never returned to a
 * terminal state. AlertsPage passes an inline filters object literal; using
 * that object as the useCallback dependency made fetchAlerts a new function
 * on every render, so the effect refetched forever. The hook now depends on
 * the individual filter values.
 */
describe('useAlerts (infinite-refetch regression)', () => {
  beforeEach(() => {
    getAlertsMock.mockReset()
    getAlertsMock.mockResolvedValue({
      alerts: [],
      _meta: { page: 1, per_page: 20, total: 0, total_pages: 0 },
    })
    getAlertStatsMock.mockReset()
    getAlertStatsMock.mockResolvedValue({
      total: 0,
      active: 0,
      acknowledged: 0,
      resolved: 0,
      false_positive: 0,
      by_severity: {},
      by_type: {},
    })
  })

  it('does not refetch on every render when filters are passed inline', async () => {
    const { result } = renderHook(() =>
      // Inline object literal — the exact caller pattern from AlertsPage.
      useAlerts({ status: 'active', page: 1, page_size: 20 })
    )

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(getAlertsMock).toHaveBeenCalledTimes(1)
    expect(result.current.error).toBeNull()
  })

  it('refresh starts loading and settles back to a terminal state', async () => {
    const { result } = renderHook(() => useAlerts({ page: 1, page_size: 20 }))

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(getAlertsMock).toHaveBeenCalledTimes(1)

    await act(async () => {
      await result.current.refetch()
    })

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(getAlertsMock).toHaveBeenCalledTimes(2)
  })

  it('surfaces an honest error and still clears loading when the request fails', async () => {
    getAlertsMock.mockRejectedValueOnce(new Error('Failed to fetch alerts'))

    const { result } = renderHook(() => useAlerts({ page: 1, page_size: 20 }))

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error?.message).toBe('Failed to fetch alerts')
  })
})

describe('useAlertStats', () => {
  beforeEach(() => {
    getAlertStatsMock.mockReset()
    getAlertStatsMock.mockResolvedValue({
      total: 3,
      active: 1,
      acknowledged: 1,
      resolved: 1,
      false_positive: 0,
      by_severity: {},
      by_type: {},
    })
  })

  it('fetches once and settles', async () => {
    const { result } = renderHook(() => useAlertStats())

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(getAlertStatsMock).toHaveBeenCalledTimes(1)
    expect(result.current.stats?.total).toBe(3)
  })
})
