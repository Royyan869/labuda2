import { render, screen } from '@testing-library/react'
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

    render(<Topbar />)

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

    render(<Topbar />)

    const avatarImg = screen.getByRole('img', { name: 'busiyono79' })
    expect(avatarImg).toBeInTheDocument()
    expect(avatarImg).toHaveAttribute('src', 'https://cdn.labuda.com/avatars/user-1.jpg')
  })

  it('renders initial-based avatar fallback when avatarUrl is absent', () => {
    useAuthStore.setState({
      user: {
        id: 'user-1',
        email: 'admin@labuda.com',
        username: 'busiyono79',
        isAdmin: true,
      },
      sessionToken: 'token',
    })

    render(<Topbar />)

    // No img element should exist when avatarUrl is absent
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    // The initial 'B' from username should be rendered
    expect(screen.getByText('B')).toBeInTheDocument()
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

    render(<Topbar />)

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

    render(<Topbar />)

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

    render(<Topbar />)

    // Open dropdown
    const menuButton = screen.getByRole('button')
    menuButton.click()

    // Dropdown should show the same canonical username
    const dropdownUsernames = screen.getAllByText('@canonicaluser')
    expect(dropdownUsernames.length).toBeGreaterThanOrEqual(1)
  })
})
