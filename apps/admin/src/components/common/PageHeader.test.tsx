import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { PageHeader } from './PageHeader'

describe('PageHeader (canonical page composition)', () => {
  it('renders exactly one page title heading', () => {
    render(<PageHeader title="Orders" />)

    const headings = screen.getAllByRole('heading', { level: 1 })
    expect(headings).toHaveLength(1)
    expect(headings[0]).toHaveTextContent('Orders')
    expect(headings[0]).toHaveClass('type-page-title')
  })

  it('renders an optional description and actions', () => {
    render(
      <PageHeader
        title="Users"
        description="Manage user accounts"
        actions={<button type="button">Refresh</button>}
      />,
    )

    expect(screen.getByText('Manage user accounts')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeInTheDocument()
  })

  it('renders a leading control before the title block', () => {
    render(
      <PageHeader
        title="Admin Management"
        leading={<button type="button">Back</button>}
      />,
    )

    expect(screen.getByRole('button', { name: 'Back' })).toBeInTheDocument()
  })

  it('renders a decorative icon without exposing it to assistive technology', () => {
    const { container } = render(<PageHeader title="Payment Methods" icon={<svg data-testid="glyph" />} />)

    expect(container.querySelector('[aria-hidden="true"] [data-testid="glyph"]')).not.toBeNull()
  })
})
