import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useModerationCases } from './useModeration'
import type { ResourceType } from '@/types'

const getModerationCasesMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api/moderation', () => ({
  getModerationCases: getModerationCasesMock,
}))

describe('useModerationCases', () => {
  beforeEach(() => {
    getModerationCasesMock.mockReset()
    getModerationCasesMock.mockResolvedValue({ cases: [], count: 0 })
  })

  it('forwards resource_type to the moderation cases API and refetches when it changes', async () => {
    const { rerender } = renderHook(
      ({ resourceType }: { resourceType?: ResourceType }) =>
        useModerationCases({
          resource_type: resourceType,
        }),
      {
        initialProps: { resourceType: 'fixed_price_sale' },
      }
    )

    await waitFor(() => {
      expect(getModerationCasesMock).toHaveBeenCalledTimes(1)
    })

    expect(getModerationCasesMock).toHaveBeenLastCalledWith({
      status: undefined,
      resource_type: 'fixed_price_sale',
      page: 1,
      limit: 20,
    })

    rerender({ resourceType: 'auction' })

    await waitFor(() => {
      expect(getModerationCasesMock).toHaveBeenCalledTimes(2)
    })

    expect(getModerationCasesMock).toHaveBeenLastCalledWith({
      status: undefined,
      resource_type: 'auction',
      page: 1,
      limit: 20,
    })
  })
})
