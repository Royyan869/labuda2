package entity

// StopReason represents why a promotion instance was stopped.
// These are canonical stop reason constants.
type StopReason string

const (
	// User-initiated stops
	StopReasonUserPaused    StopReason = "user_paused"
	StopReasonUserCancelled StopReason = "user_cancelled"

	// Admin-initiated stops
	StopReasonAdminCancelled StopReason = "admin_cancelled"

	// Duration-based stops
	StopReasonDurationExhausted StopReason = "duration_exhausted"
	StopReasonValidityExpired   StopReason = "validity_expired"

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
		StopReasonDurationExhausted, StopReasonValidityExpired,
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
