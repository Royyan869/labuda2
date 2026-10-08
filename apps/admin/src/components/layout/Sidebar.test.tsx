import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
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

// ONE navigation surface: the same <aside> is docked on desktop and an
// overlay drawer below lg. These tests pin the single-instance contract and
// the drawer's open/close behaviour.
describe('Sidebar responsive drawer contract', () => {
  it('is off-canvas and inert while closed on narrow viewports', () => {
    render(
      <MemoryRouter>
        <Sidebar isDesktop={false} isOpen={false} />
      </MemoryRouter>
    )

    const aside = screen.getByRole('complementary', { name: 'Admin navigation' })
    expect(aside).toHaveAttribute('inert')
    expect(aside.className).toContain('-translate-x-full')
  })

  it('slides in and becomes interactive when open, with a dismissible scrim', async () => {
    const onNavigate = vi.fn()
    render(
      <MemoryRouter>
        <Sidebar isDesktop={false} isOpen onNavigate={onNavigate} />
      </MemoryRouter>
    )

    const aside = screen.getByRole('complementary', { name: 'Admin navigation' })
    expect(aside).not.toHaveAttribute('inert')
    expect(aside.className).toContain('translate-x-0')

    const scrim = document.querySelector('[aria-hidden="true"].fixed.inset-0')
    expect(scrim).not.toBeNull()
    await userEvent.click(scrim as Element)
    expect(onNavigate).toHaveBeenCalled()
  })

  it('is always interactive when docked on desktop, even if not "open"', () => {
    render(
      <MemoryRouter>
        <Sidebar isDesktop isOpen={false} />
      </MemoryRouter>
    )

    const aside = screen.getByRole('complementary', { name: 'Admin navigation' })
    expect(aside).not.toHaveAttribute('inert')
    expect(aside.className).toContain('translate-x-0')
  })

  it('reports navigation intent so the shell can close the drawer', async () => {
    const onNavigate = vi.fn()
    render(
      <MemoryRouter>
        <Sidebar isDesktop={false} isOpen onNavigate={onNavigate} />
      </MemoryRouter>
    )

    await userEvent.click(screen.getByText('Orders'))
    expect(onNavigate).toHaveBeenCalled()
  })

  it('marks only the most specific nested route active (/users vs /users/admins)', () => {
    render(
      <MemoryRouter initialEntries={['/users/admins/user-1']}>
        <Sidebar isDesktop />
      </MemoryRouter>
    )

    expect(screen.getByRole('link', { name: /Admins/ })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Users' })).not.toHaveAttribute('aria-current', 'page')
  })
})
