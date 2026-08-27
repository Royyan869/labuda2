package shared

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEvaluateForSaleViewerCapabilities(t *testing.T) {
	sellerID := uuid.New()
	productID := uuid.New()
	viewerID := uuid.New()

	cases := []struct {
		name  string
		input ForSaleViewerCapabilitiesInput
		want  ViewerCapabilities
	}{
		{
			name: "guest active available",
			input: ForSaleViewerCapabilitiesInput{
				ViewerID:           uuid.Nil,
				SellerID:           sellerID,
				ProductID:          productID,
				Status:             forSaleStatusActive,
				QuantityAvailable:  1,
				NegotiationEnabled: false,
				SellerTrustActive:  true,
			},
			want: ViewerCapabilities{
				Role:         "guest",
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanChat:      false,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanBuyNow:    false,
			},
		},
		{
			name: "buyer active available negotiation disabled",
			input: ForSaleViewerCapabilitiesInput{
				ViewerID:           viewerID,
				SellerID:           sellerID,
				ProductID:          productID,
				Status:             forSaleStatusActive,
				QuantityAvailable:  3,
				NegotiationEnabled: false,
				SellerTrustActive:  true,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanChat:      true,
				CanNegotiate: false,
				CanBuy:       true,
				CanBid:       false,
				CanBuyNow:    false,
			},
		},
		{
			name: "buyer active available negotiation enabled",
			input: ForSaleViewerCapabilitiesInput{
				ViewerID:           viewerID,
				SellerID:           sellerID,
				ProductID:          productID,
				Status:             forSaleStatusActive,
				QuantityAvailable:  2,
				NegotiationEnabled: true,
				SellerTrustActive:  true,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanChat:      true,
				CanNegotiate: true,
				CanBuy:       true,
				CanBid:       false,
				CanBuyNow:    false,
			},
		},
		{
			name: "buyer sold unavailable",
			input: ForSaleViewerCapabilitiesInput{
				ViewerID:           viewerID,
				SellerID:           sellerID,
				ProductID:          productID,
				Status:             forSaleStatusSold,
				QuantityAvailable:  0,
				NegotiationEnabled: true,
				SellerTrustActive:  true,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanChat:      true,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanBuyNow:    false,
			},
		},
		{
			name: "owner draft management",
			input: ForSaleViewerCapabilitiesInput{
				ViewerID:           sellerID,
				SellerID:           sellerID,
				ProductID:          productID,
				Status:             forSaleStatusDraft,
				QuantityAvailable:  1,
				NegotiationEnabled: false,
				SellerTrustActive:  false,
			},
			want: ViewerCapabilities{
				Role:         "owner",
				CanManage:    true,
				CanEdit:      true,
				CanPromote:   false,
				CanChat:      false,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanBuyNow:    false,
			},
		},
		{
			name: "owner active promotion",
			input: ForSaleViewerCapabilitiesInput{
				ViewerID:           sellerID,
				SellerID:           sellerID,
				ProductID:          productID,
				Status:             forSaleStatusActive,
				QuantityAvailable:  4,
				NegotiationEnabled: true,
				SellerTrustActive:  true,
			},
			want: ViewerCapabilities{
				Role:         "owner",
				CanManage:    true,
				CanEdit:      true,
				CanPromote:   true,
				CanChat:      false,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanBuyNow:    false,
			},
		},
		{
			name: "seller trust inactive",
			input: ForSaleViewerCapabilitiesInput{
				ViewerID:           viewerID,
				SellerID:           sellerID,
				ProductID:          productID,
				Status:             forSaleStatusActive,
				QuantityAvailable:  1,
				NegotiationEnabled: true,
				SellerTrustActive:  false,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanChat:      false,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanBuyNow:    false,
			},
		},
		{
			name: "zero quantity unavailable",
			input: ForSaleViewerCapabilitiesInput{
				ViewerID:           viewerID,
				SellerID:           sellerID,
				ProductID:          productID,
				Status:             forSaleStatusActive,
				QuantityAvailable:  0,
				NegotiationEnabled: true,
				SellerTrustActive:  true,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanChat:      true,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanBuyNow:    false,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateForSaleViewerCapabilities(tc.input)
			require.Equal(t, tc.want, got)
		})
	}
}
