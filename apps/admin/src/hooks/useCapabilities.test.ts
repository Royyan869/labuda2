import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'

const getCapabilitiesMock = vi.hoisted(() => vi.fn())
const getUserCapabilitiesMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api', () => ({
  getCapabilities: getCapabilitiesMock,
  getUserCapabilities: getUserCapabilitiesMock,
  assignCapability: vi.fn(),
  revokeCapability: vi.fn(),
}))

import { useCapabilities, useUserCapabilities } from './useCapabilities'

// Regression for the capability read-path disconnect: the client unwraps the
// canonical {data} envelope, so the hooks must receive the payload directly.
describe('useCapabilities (capability read-path regression)', () => {
  beforeEach(() => {
    getCapabilitiesMock.mockReset()
    getUserCapabilitiesMock.mockReset()
  })

  it('exposes the backend capability catalog', async () => {
    getCapabilitiesMock.mockResolvedValue({
      capabilities: [
        {
          capability: 'finance.withdraw.read',
          category: 'Finance',
          description: 'Can view withdrawal requests',
          critical: false,
        },
      ],
    })

    const { result } = renderHook(() => useCapabilities())

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.capabilities).toHaveLength(1)
    expect(result.current.capabilities[0].capability).toBe('finance.withdraw.read')
    expect(result.current.error).toBeNull()
  })

  it('exposes an empty catalog without error', async () => {
    getCapabilitiesMock.mockResolvedValue({ capabilities: [] })

    const { result } = renderHook(() => useCapabilities())

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.capabilities).toEqual([])
    expect(result.current.error).toBeNull()
  })

  it('exposes the user authority summary fields', async () => {
    getUserCapabilitiesMock.mockResolvedValue({
      user_id: 'user-1',
      role: 'admin',
      is_admin: true,
      capabilities: [{ capability: 'finance.withdraw.read', granted_at: '2026-01-01T00:00:00Z' }],
      total: 1,
      full_access: false,
      missing_capabilities: ['order.read'],
    })

    const { result } = renderHook(() => useUserCapabilities('user-1'))

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.role).toBe('admin')
    expect(result.current.isAdmin).toBe(true)
    expect(result.current.total).toBe(1)
    expect(result.current.userCapabilities).toHaveLength(1)
    expect(result.current.fullAccess).toBe(false)
    expect(result.current.missingCapabilities).toEqual(['order.read'])
  })

  it('surfaces a failed catalog fetch as an error (unchanged)', async () => {
    getCapabilitiesMock.mockRejectedValue(new Error('boom'))

    const { result } = renderHook(() => useCapabilities())

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.error?.message).toBe('boom')
    expect(result.current.capabilities).toEqual([])
  })
})
