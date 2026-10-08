import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { AdminLoadingState } from './AdminLoadingState'

describe('AdminLoadingState (canonical loading region)', () => {
  it('announces a busy status once to assistive technology', () => {
    render(<AdminLoadingState />)

    const status = screen.getByRole('status')
    expect(status).toHaveAttribute('aria-busy', 'true')
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('accepts a custom accessible label', () => {
    render(<AdminLoadingState label="Loading orders" />)

    expect(screen.getByText('Loading orders…')).toBeInTheDocument()
  })

  it('keeps the decorative skeleton out of the accessibility tree', () => {
    const { container } = render(<AdminLoadingState rows={3} />)

    expect(container.querySelector('[aria-hidden="true"]')).not.toBeNull()
  })

  it('renders without a Card wrapper when embedded in an existing container', () => {
    const { container } = render(<AdminLoadingState embedded label="Loading timeline" />)

    expect(screen.getByRole('status')).toBeInTheDocument()
    expect(container.querySelector('.bg-surface')).toBeNull()
  })
})
