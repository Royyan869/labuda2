import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { RouteErrorBoundary } from './RouteErrorBoundary'

function Crash(): never {
  throw new Error('Cannot read properties of undefined (reading \'length\')')
}

describe('RouteErrorBoundary (PASS_20E blank-page defense)', () => {
  it('renders children normally when nothing throws', () => {
    render(
      <RouteErrorBoundary>
        <div>page content</div>
      </RouteErrorBoundary>
    )

    expect(screen.getByText('page content')).toBeInTheDocument()
  })

  it('shows an error message instead of a blank page when a page component throws', () => {
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

    render(
      <RouteErrorBoundary>
        <Crash />
      </RouteErrorBoundary>
    )

    expect(screen.getByText('This page failed to load')).toBeInTheDocument()
    expect(screen.getByText(/reading 'length'/)).toBeInTheDocument()

    consoleErrorSpy.mockRestore()
  })
})
