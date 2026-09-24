package entity

// StopReason represents why a promotion target became ineligible for
// delivery (target/seller governance vocabulary shared with the canonical
// operability authority).
//
// NOTE: duration-purchase stop reasons ("duration_exhausted",
// "validity_expired") are FORBIDDEN LEGACY (duration-purchase model purged,
// canonical contract §28) and are deliberately absent here. Duration in the
// canonical model is a pacing/planned-finish boundary only — it never
// terminates a promotion as an entitlement.
type StopReason string

const (
	// User-initiated stops
	StopReasonUserPaused    StopReason = "user_paused"
	StopReasonUserCancelled StopReason = "user_cancelled"

	// Admin-initiated stops
	StopReasonAdminCancelled StopReason = "admin_cancelled"

	// Fixed-price sale-specific stops
	StopReasonForSaleSold      StopReason = "for_sale_sold"
	StopReasonForSaleHidden    StopReason = "for_sale_hidden"
	StopReasonForSaleDeleted   StopReason = "for_sale_deleted"
	StopReasonForSaleModerated StopReason = "for_sale_moderated"
	StopReasonForSaleExpired   StopReason = "for_sale_expired"

	// Auction-specific stops
	StopReasonAuctionEnded     StopReason = "auction_ended"
	StopReasonAuctionCancelled StopReason = "auction_cancelled"
	StopReasonAuctionDeleted   StopReason = "auction_deleted"
	StopReasonAuctionModerated StopReason = "auction_moderated"

	// Seller governance stops (account-level or verification-level)
	StopReasonSellerGovernance StopReason = "seller_governance"

	// External product stops
	StopReasonExternalInvalid StopReason = "external_invalid"
)

// IsValid returns true if the stop reason is a canonical constant.
func (s StopReason) IsValid() bool {
	switch s {
	case StopReasonUserPaused, StopReasonUserCancelled, StopReasonAdminCancelled,
		StopReasonForSaleSold, StopReasonForSaleHidden, StopReasonForSaleDeleted,
		StopReasonForSaleModerated, StopReasonForSaleExpired,
		StopReasonAuctionEnded, StopReasonAuctionCancelled, StopReasonAuctionDeleted,
		StopReasonAuctionModerated, StopReasonSellerGovernance, StopReasonExternalInvalid:
		return true
	default:
		return false
	}
}

// String returns the string representation of the stop reason.
func (s StopReason) String() string {
	return string(s)
}
