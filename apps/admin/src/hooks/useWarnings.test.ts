import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'

const apiGetMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  api: {
    get: apiGetMock,
  },
}))

import { useWarnings } from './useWarnings'

describe('useWarnings (PASS_20E envelope unwrap regression)', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
  })

  it('unwraps the {data} envelope so warnings.length never runs on undefined', async () => {
    apiGetMock.mockResolvedValue({
      success: true,
      data: { warnings: [{ id: 'w1', user_id: 'u1' }], page: 1, limit: 20, count: 1 },
      timestamp: '2026-01-01T00:00:00Z',
    })

    const { result } = renderHook(() => useWarnings())

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.warnings).toHaveLength(1)
    expect(result.current.count).toBe(1)
  })

  it('falls back to an empty array (not undefined) on an unexpected response shape', async () => {
    // Regression guard: before the PASS_20E fix, this assigned `undefined`
    // to state with no fallback, which crashed WarningsPage's render at
    // `warnings.length` — a blank white page with no error boundary.
    apiGetMock.mockResolvedValue({
      success: true,
      data: undefined,
      timestamp: '2026-01-01T00:00:00Z',
    })

    const { result } = renderHook(() => useWarnings())

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.warnings).toEqual([])
    expect(() => result.current.warnings.length).not.toThrow()
  })
})
