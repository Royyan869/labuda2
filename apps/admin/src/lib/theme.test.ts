import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { applyTheme, getStoredTheme, setStoredTheme, THEME_STORAGE_KEY, useTheme } from './theme'

beforeEach(() => {
  localStorage.clear()
  document.documentElement.className = ''
  document.documentElement.style.colorScheme = ''
})

describe('theme preference', () => {
  it('defaults invalid storage to system', () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'sepia')
    expect(getStoredTheme()).toBe('system')
  })

  it('applies light and dark modes to the document', () => {
    applyTheme('light')
    expect(document.documentElement).not.toHaveClass('dark')
    expect(document.documentElement.style.colorScheme).toBe('light')

    applyTheme('dark')
    expect(document.documentElement).toHaveClass('dark')
    expect(document.documentElement.style.colorScheme).toBe('dark')
  })

  it('persists selected mode', () => {
    const { result } = renderHook(() => useTheme())

    act(() => result.current.setMode('dark'))

    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
    expect(document.documentElement).toHaveClass('dark')
  })

  it('follows system changes in system mode', () => {
    let listener: (() => void) | undefined
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: false,
      media: '(prefers-color-scheme: dark)',
      onchange: null,
      addEventListener: (_event: string, callback: EventListenerOrEventListenerObject) => { listener = callback as () => void },
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })

    renderHook(() => useTheme())
    expect(document.documentElement).not.toHaveClass('dark')

    Object.defineProperty(window, 'matchMedia', {
      value: vi.fn().mockReturnValue({ matches: true }),
      writable: true,
    })
    act(() => listener?.())

    expect(document.documentElement).toHaveClass('dark')
  })

  it('survives storage write failures', () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('storage unavailable')
    })

    expect(() => setStoredTheme('dark')).not.toThrow()
    setItem.mockRestore()
  })
})
