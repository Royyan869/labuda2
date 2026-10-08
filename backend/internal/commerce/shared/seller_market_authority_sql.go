package shared

import "fmt"

// ActiveSellerMarketAuthoritySQL is the CANONICAL read-side market-authority
// predicate: the seller's LATEST seller_subscriptions row is currently
// active and inside its entitlement interval (status = 'active' AND
// started_at <= now < expires_at).
//
// It mirrors the write-side authority (RoleCheckerDB.HasActiveSellerCapability
// subscription gate, which reads the same latest row) so a listing can never
// be browsable while its seller cannot sell. The account-status axis
// (banned/deleted) is enforced separately by each query's own governance
// filter.
//
// Owner decision (Oct 2026): sellers whose market authority has lapsed are
// EXCLUDED from viewer surfaces outright. This replaces the earlier
// demote-only doctrine ("do not exclude, only demote") recorded in the
// search repository — the owner reversed it: hidden from every list, CTAs
// disabled via the sellerTrust lifecycle axis, purchase blocked by Guard 6.
//
// Seller-OWNED inventory views deliberately do not use this predicate:
// sellers always see their own listings (management surface).
func ActiveSellerMarketAuthoritySQL(sellerColumn string) string {
	return fmt.Sprintf(
		`COALESCE((SELECT ms.status = 'active' AND ms.started_at <= NOW() AND NOW() < ms.expires_at FROM seller_subscriptions ms WHERE ms.user_id = %s ORDER BY ms.created_at DESC LIMIT 1), FALSE)`,
		sellerColumn,
	)
}
