import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useNavigate } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MainLayout } from './MainLayout'

const auth = vi.hoisted(() => ({
  state: { isLoading: false, isAuthenticated: true, isAdmin: true },
}))

vi.mock('@/hooks/useAuth', () => ({
  useAuth: () => ({
    user: { id: 'u1', email: 'a@b.c', username: 'admin', isAdmin: true, capabilities: ['*'] },
    capabilities: ['*'],
    error: null,
    ...auth.state,
  }),
}))

const media = vi.hoisted(() => ({ desktop: false }))

vi.mock('@/hooks/useMediaQuery', () => ({
  ADMIN_DESKTOP_QUERY: '(min-width: 1024px)',
  useMediaQuery: () => media.desktop,
  useIsDesktop: () => media.desktop,
}))

function NavigateButton() {
  const navigate = useNavigate()
  return (
    <button data-testid="go-orders" onClick={() => navigate('/orders')}>
      go
    </button>
  )
}

function renderShell(initialEntry = '/') {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route element={<MainLayout />}>
          <Route path="/" element={<div>home-page<NavigateButton /></div>} />
          <Route path="/orders" element={<div>orders-page</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('MainLayout shell', () => {
  beforeEach(() => {
    auth.state = { isLoading: false, isAuthenticated: true, isAdmin: true }
    media.desktop = false
  })

  it('renders the single navigation surface, top bar and routed page', () => {
    renderShell()
    expect(screen.getByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /open navigation menu/i })).toBeInTheDocument()
    expect(screen.getByText('home-page')).toBeInTheDocument()
    // Exactly one sidebar in the DOM — no duplicate mobile/desktop copies.
    expect(screen.getAllByRole('complementary', { name: 'Admin navigation' })).toHaveLength(1)
  })

  it('shows the layout loading state and no shell while auth is resolving', () => {
    auth.state = { isLoading: true, isAuthenticated: false, isAdmin: false }
    renderShell()
    expect(screen.getByText('Loading...')).toBeInTheDocument()
    expect(screen.queryByRole('complementary', { name: 'Admin navigation' })).not.toBeInTheDocument()
  })

  it('opens the mobile drawer, focuses navigation, and Escape closes it returning focus to the trigger', async () => {
    const user = userEvent.setup()
    renderShell()

    const trigger = screen.getByRole('button', { name: /open navigation menu/i })
    await user.click(trigger)

    const aside = screen.getByRole('complementary', { name: 'Admin navigation' })
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(aside).not.toHaveAttribute('inert')
    expect(document.querySelector('[aria-hidden="true"].fixed.inset-0')).not.toBeNull()
    expect(document.activeElement?.textContent).toContain('Dashboard')
    // Background content is inert while the drawer is open (focus containment).
    expect(screen.getByText('home-page').closest('[inert]')).not.toBeNull()

    await user.keyboard('{Escape}')

    expect(screen.getByRole('button', { name: /open navigation menu/i })).toHaveAttribute('aria-expanded', 'false')
    expect(document.querySelector('[aria-hidden="true"].fixed.inset-0')).toBeNull()
    expect(screen.getByText('home-page').closest('[inert]')).toBeNull()
    expect(document.activeElement).toBe(trigger)
  })

  it('closes the drawer when the route changes from programmatic navigation', async () => {
    const user = userEvent.setup()
    renderShell()

    await user.click(screen.getByRole('button', { name: /open navigation menu/i }))
    expect(screen.getByRole('button', { name: /close navigation menu/i })).toHaveAttribute('aria-expanded', 'true')

    fireEvent.click(screen.getByTestId('go-orders'))

    expect(screen.getByText('orders-page')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /open navigation menu/i })).toHaveAttribute('aria-expanded', 'false')
  })

  it('docks the sidebar on desktop and never renders a scrim', () => {
    media.desktop = true
    renderShell()

    const aside = screen.getByRole('complementary', { name: 'Admin navigation' })
    expect(aside).not.toHaveAttribute('inert')
    expect(document.querySelector('[aria-hidden="true"].fixed.inset-0')).toBeNull()
  })
})
