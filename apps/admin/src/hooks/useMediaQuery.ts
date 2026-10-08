import { useEffect, useState } from 'react'

/**
 * The single source of truth for the Admin shell's JavaScript breakpoint.
 *
 * `(min-width: 1024px)` is Tailwind's `lg` breakpoint — the same value the
 * shell's responsive utility classes use. When this changes, the `lg:` classes
 * in the shell must change with it. It exists so that the few behaviours that
 * cannot be expressed in CSS (keep the off-canvas sidebar out of the tab order,
 * reset the mobile drawer when the viewport grows) agree with the CSS.
 */
export const ADMIN_DESKTOP_QUERY = '(min-width: 1024px)'

/**
 * Subscribes to a CSS media query. This is the only viewport hook in the
 * Admin app — do not add per-component `matchMedia`/resize listeners.
 */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState<boolean>(() =>
    typeof window !== 'undefined' && typeof window.matchMedia === 'function'
      ? window.matchMedia(query).matches
      : false,
  )

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return
    }

    const media = window.matchMedia(query)
    const handleChange = () => setMatches(media.matches)
    media.addEventListener('change', handleChange)
    return () => media.removeEventListener('change', handleChange)
  }, [query])

  return matches
}

/** True on desktop widths (`lg` and up) where the sidebar is permanently docked. */
export function useIsDesktop(): boolean {
  return useMediaQuery(ADMIN_DESKTOP_QUERY)
}
