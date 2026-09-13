import { describe, expect, it, vi, beforeEach } from 'vitest'

const apiGetMock = vi.hoisted(() => vi.fn())
const apiPostMock = vi.hoisted(() => vi.fn())
const apiDeleteMock = vi.hoisted(() => vi.fn())

vi.mock('./client', () => ({
  api: {
    get: apiGetMock,
    post: apiPostMock,
    delete: apiDeleteMock,
  },
}))

import {
  getCapabilities,
  getUserCapabilities,
  assignCapability,
  revokeCapability,
} from './platform'

// Regression: backend response.Success wraps every payload as
// { success, data, timestamp }. The capability read clients previously returned
// the whole envelope, so `response.capabilities` was undefined and the Admin
// detail page rendered no capability checkboxes.
describe('capability read path (canonical {data} envelope unwrap)', () => {
  beforeEach(() => {
    apiGetMock.mockReset()
    apiPostMock.mockReset()
    apiDeleteMock.mockReset()
  })

  it('getCapabilities unwraps data.capabilities from the canonical envelope', async () => {
    apiGetMock.mockResolvedValue({
      success: true,
      data: {
        capabilities: [
          {
            capability: 'finance.withdraw.read',
            category: 'Finance',
            description: 'Can view withdrawal requests',
            critical: false,
          },
        ],
      },
      timestamp: '2026-01-01T00:00:00Z',
    })

    const result = await getCapabilities()

    expect(apiGetMock).toHaveBeenCalledWith('/api/v1/admin/capabilities')
    expect(result.capabilities).toHaveLength(1)
    expect(result.capabilities[0].capability).toBe('finance.withdraw.read')
  })

  it('getCapabilities surfaces an empty catalog as an empty array', async () => {
    apiGetMock.mockResolvedValue({
      success: true,
      data: { capabilities: [] },
      timestamp: '2026-01-01T00:00:00Z',
    })

    const result = await getCapabilities()

    expect(result.capabilities).toEqual([])
  })

  it('getUserCapabilities unwraps the authority summary from data', async () => {
    apiGetMock.mockResolvedValue({
      success: true,
      data: {
        user_id: 'user-1',
        role: 'admin',
        is_admin: true,
        capabilities: [
          { capability: 'finance.withdraw.read', granted_at: '2026-01-01T00:00:00Z' },
        ],
        total: 1,
        full_access: false,
        missing_capabilities: ['order.read'],
      },
      timestamp: '2026-01-01T00:00:00Z',
    })

    const result = await getUserCapabilities('user-1')

    expect(apiGetMock).toHaveBeenCalledWith('/api/v1/admin/users/user-1/capabilities')
    expect(result.role).toBe('admin')
    expect(result.is_admin).toBe(true)
    expect(result.capabilities).toHaveLength(1)
    expect(result.total).toBe(1)
    expect(result.full_access).toBe(false)
    expect(result.missing_capabilities).toEqual(['order.read'])
  })

  it('leaves the assign/revoke mutation calls unchanged', async () => {
    apiPostMock.mockResolvedValue({ success: true, data: { message: 'ok' } })
    apiDeleteMock.mockResolvedValue({ success: true, data: { message: 'ok' } })

    await assignCapability('user-1', 'order.read')
    await revokeCapability('user-1', 'order.read')

    expect(apiPostMock).toHaveBeenCalledWith('/api/v1/admin/users/user-1/capabilities', {
      capability: 'order.read',
    })
    expect(apiDeleteMock).toHaveBeenCalledWith(
      '/api/v1/admin/users/user-1/capabilities/order.read'
    )
  })
})
