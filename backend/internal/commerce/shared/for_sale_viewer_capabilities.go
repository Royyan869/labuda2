package shared

import (
	"strings"

	"github.com/google/uuid"
)

// ForSaleViewerCapabilitiesInput captures the canonical facts used to
// evaluate fixed-price sale detail and chat action capabilities.
type ForSaleViewerCapabilitiesInput struct {
	ViewerID           uuid.UUID
	SellerID           uuid.UUID
	ProductID          uuid.UUID
	Status             string
	QuantityAvailable  int
	NegotiationEnabled bool
	SellerTrustActive  bool
}

// EvaluateForSaleViewerCapabilities applies the canonical fixed-price
// sale viewer/action rule set.
func EvaluateForSaleViewerCapabilities(input ForSaleViewerCapabilitiesInput) ViewerCapabilities {
	role := "guest"
	if input.ViewerID != uuid.Nil {
		if input.ViewerID == input.SellerID {
			role = "owner"
		} else {
			role = "buyer"
		}
	}

	if role == "owner" {
		isEditable := isForSaleEditableStatus(input.Status)
		return ViewerCapabilities{
			Role:         role,
			CanManage:    true,
			CanEdit:      isEditable,
			CanPromote:   strings.TrimSpace(input.Status) == forSaleStatusActive,
			CanChat:      false,
			CanNegotiate: false,
			CanBuy:       false,
			CanBid:       false,
			CanBuyNow:    false,
		}
	}

	canChat := input.ViewerID != uuid.Nil && input.SellerTrustActive
	isActiveAndAvailable := isForSaleAvailable(input.Status, input.QuantityAvailable)
	canNegotiate := canChat && input.NegotiationEnabled && isActiveAndAvailable
	canBuy := canChat && isActiveAndAvailable && input.ProductID != uuid.Nil

	return ViewerCapabilities{
		Role:         role,
		CanManage:    false,
		CanEdit:      false,
		CanPromote:   false,
		CanChat:      canChat,
		CanNegotiate: canNegotiate,
		CanBuy:       canBuy,
		CanBid:       false,
		CanBuyNow:    false,
	}
}

func isForSaleEditableStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case forSaleStatusDraft, forSaleStatusActive:
		return true
	default:
		return false
	}
}

func isForSaleAvailable(status string, quantityAvailable int) bool {
	return strings.TrimSpace(status) == forSaleStatusActive && quantityAvailable > 0
}
