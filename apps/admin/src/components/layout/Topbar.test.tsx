import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { Topbar } from './Topbar'
import { useAuthStore } from '@/store/authStore'

// Mock the API client to prevent actual HTTP calls
vi.mock('@/lib/api', () => ({
  logoutAdmin: vi.fn(),
}))

describe('Topbar authenticated identity presentation', () => {
  beforeEach(() => {
    useAuthStore.setState({
      user: null,
      isLoading: false,
      error: null,
      sessionToken: null,
    })
  })

  it('renders canonical @username from profile.username', () => {
    useAuthStore.setState({
      user: {
        id: 'user-1',
        email: 'admin@labuda.com',
        username: 'busiyono79',
        isAdmin: true,
      },
      sessionToken: 'token',
    })

    render(<MemoryRouter><Topbar /></MemoryRouter>)

    // Must display the canonical @username, not a UUID prefix
    expect(screen.getByText('@busiyono79')).toBeInTheDocument()
    expect(screen.getByText('admin@labuda.com')).toBeInTheDocument()
  })

  it('renders canonical avatar image when avatarUrl is present', () => {
    useAuthStore.setState({
      user: {
        id: 'user-1',
        email: 'admin@labuda.com',
        username: 'busiyono79',
        avatarUrl: 'https://cdn.labuda.com/avatars/user-1.jpg',
        isAdmin: true,
      },
      sessionToken: 'token',
    })

    render(<MemoryRouter><Topbar /></MemoryRouter>)

    const avatarImg = screen.getByRole('img', { name: 'busiyono79' })
    expect(avatarImg).toBeInTheDocument()
    expect(avatarImg).toHaveAttribute('src', 'https://cdn.labuda.com/avatars/user-1.jpg')
  })

  it('renders person-icon avatar fallback when avatarUrl is absent (no initials)', () => {
    useAuthStore.setState({
      user: {
        id: 'user-1',
        email: 'admin@labuda.com',
        username: 'busiyono79',
        isAdmin: true,
      },
      sessionToken: 'token',
    })

    render(<MemoryRouter><Topbar /></MemoryRouter>)

    // Canonical fallback: person icon in a circle, exposed as role="img"
    // with the identity label — never text initials.
    const avatarFallback = screen.getByRole('img', { name: 'busiyono79' })
    expect(avatarFallback).toBeInTheDocument()
    // Initials are banned business-wide (Owner decision 2026-09-24)
    expect(screen.queryByText('B')).not.toBeInTheDocument()
  })

  it('does NOT render DB UUID prefix as authenticated username', () => {
    useAuthStore.setState({
      user: {
        id: '40448f54-aaaa-bbbb-cccc-dddddddddddd',
        email: 'admin@labuda.com',
        username: 'mycanonicalname',
        isAdmin: true,
      },
      sessionToken: 'token',
    })

    render(<MemoryRouter><Topbar /></MemoryRouter>)

    // Must show @mycanonicalname, NOT @40448f54
    expect(screen.getByText('@mycanonicalname')).toBeInTheDocument()
    expect(screen.queryByText('@40448f54')).not.toBeInTheDocument()
  })

  it('shows "Admin" label when canonical username is empty (missing identity)', () => {
    useAuthStore.setState({
      user: {
        id: 'user-1',
        email: 'admin@labuda.com',
        username: '',
        isAdmin: true,
      },
      sessionToken: 'token',
    })

    render(<MemoryRouter><Topbar /></MemoryRouter>)

    // Empty username → shows "Admin" as the identity label, not UUID prefix
    expect(screen.getByText('Admin')).toBeInTheDocument()
    expect(screen.queryByText(/^[0-9a-f]{8}$/)).not.toBeInTheDocument()
  })

  it('dropdown also shows canonical username', () => {
    useAuthStore.setState({
      user: {
        id: 'user-1',
        email: 'admin@labuda.com',
        username: 'canonicaluser',
        avatarUrl: 'https://cdn.labuda.com/avatars/user-1.jpg',
        isAdmin: true,
      },
      sessionToken: 'token',
    })

    render(<MemoryRouter><Topbar /></MemoryRouter>)

    // Open dropdown via the account menu disclosure (not the mobile trigger)
    const menuButton = screen.getByRole('button', { name: /admin account menu/i })
    menuButton.click()

    // Dropdown should show the same canonical username
    const dropdownUsernames = screen.getAllByText('@canonicaluser')
    expect(dropdownUsernames.length).toBeGreaterThanOrEqual(1)
  })
})

// The top bar owns shell chrome only: the mobile navigation trigger and the
// account menu. It must NOT render a page title — pages own their own h1.
describe('Topbar shell chrome', () => {
  beforeEach(() => {
    useAuthStore.setState({
      user: null,
      isLoading: false,
      error: null,
      sessionToken: null,
    })
  })

  it('does not render a competing page title', () => {
    render(<MemoryRouter><Topbar /></MemoryRouter>)
    expect(screen.queryByText('Admin Dashboard')).not.toBeInTheDocument()
  })

  it('exposes the mobile navigation trigger with expanded state', async () => {
    const onMenuClick = vi.fn()
    const { rerender } = render(
      <MemoryRouter><Topbar onMenuClick={onMenuClick} isNavOpen={false} /></MemoryRouter>
    )

    const trigger = screen.getByRole('button', { name: /open navigation menu/i })
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(trigger).toHaveAttribute('aria-controls', 'admin-sidebar')

    await userEvent.click(trigger)
    expect(onMenuClick).toHaveBeenCalledTimes(1)

    rerender(
      <MemoryRouter><Topbar onMenuClick={onMenuClick} isNavOpen /></MemoryRouter>
    )
    expect(screen.getByRole('button', { name: /close navigation menu/i })).toHaveAttribute('aria-expanded', 'true')
  })

  it('closes the account disclosure on Escape', async () => {
    render(<MemoryRouter><Topbar /></MemoryRouter>)

    await userEvent.click(screen.getByRole('button', { name: /admin account menu/i }))
    expect(screen.getByText('Profile')).toBeInTheDocument()

    await userEvent.keyboard('{Escape}')
    expect(screen.queryByText('Profile')).not.toBeInTheDocument()
  })
})
