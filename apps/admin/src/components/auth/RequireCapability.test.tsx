import { MemoryRouter } from 'react-router-dom'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { RequireCapability } from './RequireCapability'

const useAuthMock = vi.fn()

vi.mock('@/hooks/useAuth', () => ({
  useAuth: () => useAuthMock(),
}))

describe('RequireCapability', () => {
  beforeEach(() => {
    useAuthMock.mockReset()
  })

  it('renders the child when the capability is present', () => {
    useAuthMock.mockReturnValue({
      user: { id: '1' },
      capabilities: ['finance.withdraw.review'],
    })

    render(
      <MemoryRouter>
        <RequireCapability cap="finance.withdraw.review">
          <div>allowed</div>
        </RequireCapability>
      </MemoryRouter>
    )

    expect(screen.getByText('allowed')).toBeInTheDocument()
  })

  it('shows the access denied state when capability is missing', () => {
    useAuthMock.mockReturnValue({
      user: { id: '1' },
      capabilities: ['governance.audit.read'],
    })

    render(
      <MemoryRouter>
        <RequireCapability cap="finance.withdraw.review">
          <div>allowed</div>
        </RequireCapability>
      </MemoryRouter>
    )

    expect(screen.getByText('Access Denied')).toBeInTheDocument()
    expect(screen.getByText('finance.withdraw.review')).toBeInTheDocument()
  })
})
