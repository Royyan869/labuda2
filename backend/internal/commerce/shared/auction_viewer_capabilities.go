package shared

import (
	"strings"

	"github.com/google/uuid"
)

// AuctionViewerCapabilitiesInput captures the canonical facts used to
// evaluate auction detail actions.
type AuctionViewerCapabilitiesInput struct {
	ViewerID          uuid.UUID
	SellerID          uuid.UUID
	Status            string
	SellerTrustActive bool
	BuyNowPrice       *int64
}

// EvaluateAuctionViewerCapabilities applies the canonical auction
// viewer/action rule set.
func EvaluateAuctionViewerCapabilities(input AuctionViewerCapabilitiesInput) ViewerCapabilities {
	role := "guest"
	if input.ViewerID != uuid.Nil {
		if input.ViewerID == input.SellerID {
			role = "owner"
		} else {
			role = "buyer"
		}
	}

	if role == "owner" {
		isEditable := isAuctionEditableStatus(input.Status)
		return ViewerCapabilities{
			Role:         role,
			CanManage:    true,
			CanEdit:      isEditable,
			CanPromote:   false,
			CanChat:      false,
			CanNegotiate: false,
			CanBuy:       false,
			CanBid:       false,
			CanBuyNow:    false,
		}
	}

	canChat := input.ViewerID != uuid.Nil && input.SellerTrustActive
	isActive := strings.TrimSpace(input.Status) == auctionStatusActive
	canBid := canChat && isActive
	canBuyNow := canChat && isActive && input.BuyNowPrice != nil

	return ViewerCapabilities{
		Role:         role,
		CanManage:    false,
		CanEdit:      false,
		CanPromote:   false,
		CanChat:      canChat,
		CanNegotiate: false,
		CanBuy:       false,
		CanBid:       canBid,
		CanBuyNow:    canBuyNow,
	}
}

func isAuctionEditableStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case auctionStatusDraft, auctionStatusScheduled:
		return true
	default:
		return false
	}
}
