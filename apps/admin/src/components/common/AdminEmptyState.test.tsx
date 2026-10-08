import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { AdminEmptyState } from './AdminEmptyState'

describe('AdminEmptyState (canonical empty / no-result state)', () => {
  it('renders title and description as a status region', () => {
    render(<AdminEmptyState title="No Users Found" description="No users in the system." />)

    expect(screen.getByRole('status')).toBeInTheDocument()
    expect(screen.getByText('No Users Found')).toBeInTheDocument()
    expect(screen.getByText('No users in the system.')).toBeInTheDocument()
  })

  it('does not offer a reset action for a genuinely empty dataset', () => {
    render(<AdminEmptyState title="No Users Found" description="No users in the system." />)

    expect(screen.queryByRole('button', { name: 'Clear filters' })).not.toBeInTheDocument()
  })

  it('offers a reset action for a filtered no-result and invokes it', async () => {
    const onClearFilters = vi.fn()
    render(
      <AdminEmptyState
        title="No Users Found"
        description="No users match the current filters."
        filtered
        onClearFilters={onClearFilters}
      />
    )

    await userEvent.click(screen.getByRole('button', { name: 'Clear filters' }))

    expect(onClearFilters).toHaveBeenCalledTimes(1)
  })

  it('is content-only (does not own a Card container)', () => {
    const { container } = render(<AdminEmptyState title="Empty" />)

    expect(container.querySelector('.bg-surface')).toBeNull()
  })
})
