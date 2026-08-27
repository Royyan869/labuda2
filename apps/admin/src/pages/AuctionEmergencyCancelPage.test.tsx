import { MemoryRouter } from 'react-router-dom'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AuctionEmergencyCancelPage } from './AuctionEmergencyCancelPage'
import { RequireCapability } from '@/components/auth/RequireCapability'
import { ApiError } from '@/lib/api/client'

const adminCancelAuctionMock = vi.hoisted(() => vi.fn())

vi.mock('@/lib/api/auctions', () => ({
  adminCancelAuction: adminCancelAuctionMock,
}))

const useAuthMock = vi.fn()
vi.mock('@/hooks/useAuth', () => ({
  useAuth: () => useAuthMock(),
}))

const VALID_AUCTION_ID = '11111111-1111-1111-1111-111111111111'

async function fillForm(auctionId: string, reason: string) {
  const user = userEvent.setup()
  if (auctionId) {
    await user.type(screen.getByLabelText('Auction ID'), auctionId)
  }
  if (reason) {
    await user.type(screen.getByPlaceholderText('e.g. seller unreachable, trust & safety stop'), reason)
  }
  return user
}

describe('AuctionEmergencyCancelPage', () => {
  beforeEach(() => {
    adminCancelAuctionMock.mockReset()
    useAuthMock.mockReturnValue({
      user: { id: 'admin-1' },
      capabilities: ['governance.auction.cancel'],
    })
  })

  it('renders the control for an authorized admin', () => {
    render(<AuctionEmergencyCancelPage />)

    expect(screen.getByLabelText('Auction ID')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('e.g. seller unreachable, trust & safety stop')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Cancel Auction' })).toBeInTheDocument()
  })

  it('does not expose the control to an admin without the capability', () => {
    useAuthMock.mockReturnValue({
      user: { id: 'admin-2' },
      capabilities: [],
    })

    render(
      <MemoryRouter>
        <RequireCapability cap="governance.auction.cancel">
          <AuctionEmergencyCancelPage />
        </RequireCapability>
      </MemoryRouter>
    )

    expect(screen.getByText('Access Denied')).toBeInTheDocument()
    expect(screen.queryByLabelText('Auction ID')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Cancel Auction' })).not.toBeInTheDocument()
  })

  it('blocks submit when reason is empty', async () => {
    render(<AuctionEmergencyCancelPage />)
    await fillForm(VALID_AUCTION_ID, '')

    expect(screen.getByRole('button', { name: 'Cancel Auction' })).toBeDisabled()
  })

  it('blocks submit when auction id is empty', async () => {
    render(<AuctionEmergencyCancelPage />)
    await fillForm('', 'seller unreachable')

    expect(screen.getByRole('button', { name: 'Cancel Auction' })).toBeDisabled()
  })

  it('shows a confirmation step before submitting', async () => {
    render(<AuctionEmergencyCancelPage />)
    const user = await fillForm(VALID_AUCTION_ID, 'seller unreachable')

    await user.click(screen.getByRole('button', { name: 'Cancel Auction' }))

    expect(screen.getByText('Confirm Emergency Cancel')).toBeInTheDocument()
    expect(adminCancelAuctionMock).not.toHaveBeenCalled()
  })

  it('calls the API with the exact auctionId and reason only after confirmation', async () => {
    adminCancelAuctionMock.mockResolvedValue({
      auction_id: VALID_AUCTION_ID,
      status_before: 'active',
      status_after: 'cancelled',
      reason: 'seller unreachable',
    })

    render(<AuctionEmergencyCancelPage />)
    const user = await fillForm(VALID_AUCTION_ID, 'seller unreachable')
    await user.click(screen.getByRole('button', { name: 'Cancel Auction' }))
    await user.click(screen.getByRole('button', { name: 'Confirm Cancel' }))

    await waitFor(() => {
      expect(adminCancelAuctionMock).toHaveBeenCalledWith(VALID_AUCTION_ID, 'seller unreachable')
    })
    expect(adminCancelAuctionMock).toHaveBeenCalledTimes(1)
  })

  it('shows a success state with the status transition', async () => {
    adminCancelAuctionMock.mockResolvedValue({
      auction_id: VALID_AUCTION_ID,
      status_before: 'active',
      status_after: 'cancelled',
      reason: 'seller unreachable',
    })

    render(<AuctionEmergencyCancelPage />)
    const user = await fillForm(VALID_AUCTION_ID, 'seller unreachable')
    await user.click(screen.getByRole('button', { name: 'Cancel Auction' }))
    await user.click(screen.getByRole('button', { name: 'Confirm Cancel' }))

    expect(await screen.findByText('Auction cancelled')).toBeInTheDocument()
    expect(screen.getByText('active')).toBeInTheDocument()
    expect(screen.getByText('cancelled')).toBeInTheDocument()
  })

  it('shows a clear message and does not swallow a 409 conflict', async () => {
    adminCancelAuctionMock.mockRejectedValue(
      new ApiError(
        409,
        { success: false, error: { code: 'AUCTION_CANCEL_CONFLICT', message: 'auction already has an order' } },
        'HTTP 409: An error occurred'
      )
    )

    render(<AuctionEmergencyCancelPage />)
    const user = await fillForm(VALID_AUCTION_ID, 'seller unreachable')
    await user.click(screen.getByRole('button', { name: 'Cancel Auction' }))
    await user.click(screen.getByRole('button', { name: 'Confirm Cancel' }))

    expect(await screen.findByText('auction already has an order')).toBeInTheDocument()
    expect(screen.getByText('AUCTION_CANCEL_CONFLICT')).toBeInTheDocument()
    expect(screen.getByText(/canonical order\/dispute\/refund flow/)).toBeInTheDocument()
  })

  it('shows a clear message for a 404 not found response', async () => {
    adminCancelAuctionMock.mockRejectedValue(
      new ApiError(404, { success: false, error: { code: 'NOT_FOUND', message: 'auction not found' } }, 'The requested resource was not found.', 'NOT_FOUND')
    )

    render(<AuctionEmergencyCancelPage />)
    const user = await fillForm(VALID_AUCTION_ID, 'seller unreachable')
    await user.click(screen.getByRole('button', { name: 'Cancel Auction' }))
    await user.click(screen.getByRole('button', { name: 'Confirm Cancel' }))

    expect(await screen.findByText('The requested resource was not found.')).toBeInTheDocument()
  })

  it('shows a clear message for a 403 forbidden response', async () => {
    adminCancelAuctionMock.mockRejectedValue(
      new ApiError(403, { success: false }, 'Access denied. You do not have sufficient permissions for this action.', 'FORBIDDEN')
    )

    render(<AuctionEmergencyCancelPage />)
    const user = await fillForm(VALID_AUCTION_ID, 'seller unreachable')
    await user.click(screen.getByRole('button', { name: 'Cancel Auction' }))
    await user.click(screen.getByRole('button', { name: 'Confirm Cancel' }))

    expect(await screen.findByText('Access denied. You do not have sufficient permissions for this action.')).toBeInTheDocument()
  })

  it('shows a clear message for a 400 validation error', async () => {
    adminCancelAuctionMock.mockRejectedValue(
      new ApiError(400, { success: false, error: { code: 'BAD_REQUEST', message: 'reason is required' } }, 'HTTP 400: An error occurred')
    )

    render(<AuctionEmergencyCancelPage />)
    const user = await fillForm(VALID_AUCTION_ID, 'seller unreachable')
    await user.click(screen.getByRole('button', { name: 'Cancel Auction' }))
    await user.click(screen.getByRole('button', { name: 'Confirm Cancel' }))

    expect(await screen.findByText('reason is required')).toBeInTheDocument()
  })

  it('disables the confirm button while the request is in flight and does not double-submit', async () => {
    let resolveFn: (value: unknown) => void
    adminCancelAuctionMock.mockReturnValue(
      new Promise((resolve) => {
        resolveFn = resolve
      })
    )

    render(<AuctionEmergencyCancelPage />)
    const user = await fillForm(VALID_AUCTION_ID, 'seller unreachable')
    await user.click(screen.getByRole('button', { name: 'Cancel Auction' }))
    await user.click(screen.getByRole('button', { name: 'Confirm Cancel' }))

    for (const btn of screen.getAllByRole('button', { name: 'Cancelling...' })) {
      expect(btn).toBeDisabled()
    }

    resolveFn!({
      auction_id: VALID_AUCTION_ID,
      status_before: 'active',
      status_after: 'cancelled',
      reason: 'seller unreachable',
    })

    await screen.findByText('Auction cancelled')
    expect(adminCancelAuctionMock).toHaveBeenCalledTimes(1)
  })
})
