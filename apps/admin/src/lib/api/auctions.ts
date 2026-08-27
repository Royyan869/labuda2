import { api } from './client'

// ============================================================================
// AUCTION ADMIN EMERGENCY CANCEL/OVERRIDE (PASS_5B)
// ============================================================================
//
// No dedicated auction admin page exists yet (auction only surfaces today as
// an order-source-type filter label and, separately, as a moderation-case
// resource type whose "enforce" action already routes through the automated
// moderation cancellation pipeline). This client function is real and
// API-backed against the governance-authority endpoint added in PASS_5B;
// wiring it into a concrete page/button is deferred to a follow-up pass so
// it isn't bolted onto a surface where it would be ambiguous alongside the
// existing moderation "enforce" action for auction-targeted cases.

export interface AdminCancelAuctionResponse {
  auction_id: string
  status_before: string
  status_after: string
  reason: string
}

/**
 * Emergency-cancel any seller's auction under governance authority.
 * Requires the governance.auction.cancel capability.
 * POST /api/v1/admin/auctions/:id/cancel
 *
 * Backend wraps successful responses as `{ success, data, timestamp }`
 * (see internal/platform/response.Success) — unwrap `data` here so callers
 * receive the flat AdminCancelAuctionResponse shape.
 */
export async function adminCancelAuction(
  auctionId: string,
  reason: string
): Promise<AdminCancelAuctionResponse> {
  const resp = await api.post<{ data: AdminCancelAuctionResponse }>(
    `/api/v1/admin/auctions/${auctionId}/cancel`,
    { reason }
  )
  return resp.data
}
