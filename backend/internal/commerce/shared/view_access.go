package shared

import (
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
)

const (
	forSaleStatusDraft     = "draft"
	forSaleStatusActive    = "active"
	forSaleStatusSold      = "sold"
	forSaleStatusWithdrawn = "withdrawn"

	forSaleVisibilityPublic  = "public"
	forSaleVisibilityPrivate = "private"

	auctionStatusDraft             = "draft"
	auctionStatusScheduled         = "scheduled"
	auctionStatusActive            = "active"
	auctionStatusWaitingSettlement = "waiting_settlement"
	auctionStatusEnded             = "ended"
	auctionStatusCancelled         = "cancelled"
	auctionStatusExpiredBNR        = "expired_bnr"
)

// SellerAccessSnapshot carries the raw seller identity/trust inputs used by
// commerce view-access evaluators. The evaluator itself coarsens them.
type SellerAccessSnapshot struct {
	AccountStatus      string
	IsDeleted          bool
	SubscriptionStatus string
}

func sellerAccessAllowed(snapshot SellerAccessSnapshot) bool {
	return viewercontext.CoarsenLifecycle(snapshot.AccountStatus, snapshot.IsDeleted) == viewercontext.PublicLifecycleStateActive
}

// ForSaleViewAccessInput is the narrow input for evaluating whether a
// viewer may see a fixed-price sale in detail or nested-chat contexts.
type ForSaleViewAccessInput struct {
	ViewerID   uuid.UUID
	SellerID   uuid.UUID
	Status     string
	Visibility string
	Blocked    bool
	Seller     SellerAccessSnapshot
}

// EvaluateForSaleViewAccess applies the canonical fixed-price sale
// view-access rule set.
func EvaluateForSaleViewAccess(input ForSaleViewAccessInput) bool {
	if input.Blocked {
		return false
	}
	if !sellerAccessAllowed(input.Seller) {
		return false
	}

	switch input.Status {
	case forSaleStatusDraft:
		return input.ViewerID == input.SellerID
	case forSaleStatusActive, forSaleStatusSold, forSaleStatusWithdrawn:
		if input.Visibility == forSaleVisibilityPrivate {
			return input.ViewerID == input.SellerID
		}
		return true
	default:
		return false
	}
}

// AuctionViewAccessInput is the narrow input for evaluating whether a viewer
// may see an auction in detail or nested-chat contexts.
type AuctionViewAccessInput struct {
	ViewerID uuid.UUID
	SellerID uuid.UUID
	Status   string
	Blocked  bool
	Seller   SellerAccessSnapshot
}

// EvaluateAuctionViewAccess applies the canonical auction view-access rule set.
func EvaluateAuctionViewAccess(input AuctionViewAccessInput) bool {
	if input.Blocked {
		return false
	}
	if !sellerAccessAllowed(input.Seller) {
		return false
	}

	switch input.Status {
	case auctionStatusDraft:
		return input.ViewerID == input.SellerID
	case auctionStatusScheduled, auctionStatusActive, auctionStatusWaitingSettlement, auctionStatusEnded, auctionStatusCancelled, auctionStatusExpiredBNR:
		return true
	default:
		return false
	}
}
