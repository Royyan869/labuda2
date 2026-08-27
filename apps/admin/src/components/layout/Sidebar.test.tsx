import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { Sidebar } from './Sidebar'

vi.mock('@/hooks/useAuth', () => ({
  useAuth: () => ({ capabilities: ['*'] }),
}))

// PASS_20F: owner reported the sidebar could not scroll — lower nav items
// (below the fold) were unreachable. Root cause: the <aside> was h-screen
// but not a flex column, and <nav> had flex-1 with no overflow handling, so
// it silently clipped instead of scrolling. Regression guard below asserts
// the DOM structure that makes scrolling actually work, since jsdom doesn't
// compute real layout/overflow.
describe('Sidebar (PASS_20F scroll regression)', () => {
  it('renders the nav as an independently scrollable flex child', () => {
    render(
      <MemoryRouter>
        <Sidebar />
      </MemoryRouter>
    )

    const nav = screen.getByRole('navigation')
    expect(nav.className).toContain('overflow-y-auto')
    expect(nav.className).toContain('min-h-0')
    expect(nav.className).toContain('flex-1')
  })

  it('renders the aside as a flex column so header/nav/footer stack correctly', () => {
    render(
      <MemoryRouter>
        <Sidebar />
      </MemoryRouter>
    )

    const nav = screen.getByRole('navigation')
    const aside = nav.parentElement
    expect(aside?.className).toContain('flex')
    expect(aside?.className).toContain('flex-col')
    expect(aside?.className).toContain('h-screen')
  })

  it('renders every nav item, including ones below the fold, so they remain reachable once scrolled to', () => {
    render(
      <MemoryRouter>
        <Sidebar />
      </MemoryRouter>
    )

    // First and last items in the nav list — both must be present in the
    // DOM (jsdom doesn't clip on overflow, but a missing item here would
    // mean the list itself was truncated, not just visually clipped).
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Payment Methods')).toBeInTheDocument()
  })
})
