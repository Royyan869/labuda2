import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { AdminErrorState } from './AdminErrorState'

describe('AdminErrorState (canonical fetch-error card)', () => {
  it('exposes the canonical error state with role="alert"', () => {
    render(<AdminErrorState message="HTTP 500" />)

    expect(screen.getByRole('alert')).toBeInTheDocument()
  })

  it('renders the provided title and message', () => {
    render(<AdminErrorState title="Failed to load users" message="HTTP 500" />)

    expect(screen.getByText('Failed to load users')).toBeInTheDocument()
    expect(screen.getByText('HTTP 500')).toBeInTheDocument()
  })

  it('renders a retry button and invokes the callback when provided', async () => {
    const onRetry = vi.fn()
    render(<AdminErrorState message="HTTP 500" onRetry={onRetry} />)

    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))

    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it('does not render a retry button when onRetry is omitted', () => {
    render(<AdminErrorState message="HTTP 500" />)

    expect(screen.queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument()
  })
})
