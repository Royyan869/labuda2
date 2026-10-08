import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ADMIN_DESKTOP_QUERY, useMediaQuery } from './useMediaQuery'

function installMatchMedia(initial: boolean) {
  const listeners = new Set<() => void>()
  const mql = {
    matches: initial,
    media: '',
    onchange: null,
    addEventListener: (_type: string, cb: () => void) => listeners.add(cb),
    removeEventListener: (_type: string, cb: () => void) => listeners.delete(cb),
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  }
  const matchMedia = vi.fn().mockReturnValue(mql)
  Object.defineProperty(window, 'matchMedia', { writable: true, value: matchMedia })
  return {
    matchMedia,
    setMatches(value: boolean) {
      mql.matches = value
      listeners.forEach((listener) => listener())
    },
    listenerCount: () => listeners.size,
  }
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('useMediaQuery — single JS breakpoint authority', () => {
  it('reads the initial match, reacts to changes, and unsubscribes on unmount', () => {
    const media = installMatchMedia(false)

    const { result, unmount } = renderHook(() => useMediaQuery(ADMIN_DESKTOP_QUERY))
    expect(result.current).toBe(false)
    expect(media.matchMedia).toHaveBeenCalledWith('(min-width: 1024px)')
    expect(media.listenerCount()).toBe(1)

    act(() => media.setMatches(true))
    expect(result.current).toBe(true)

    unmount()
    expect(media.listenerCount()).toBe(0)
  })
})
