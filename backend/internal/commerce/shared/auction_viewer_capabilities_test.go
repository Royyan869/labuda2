package shared

import (
	"testing"

	"github.com/google/uuid"
)

func TestEvaluateAuctionViewerCapabilities_StateMatrix(t *testing.T) {
	sellerID := uuid.New()
	otherID := uuid.New()
	buyNow := int64(1500)

	cases := []struct {
		name string
		in   AuctionViewerCapabilitiesInput
		want ViewerCapabilities
	}{
		{
			name: "A1 scheduled non-owner",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusScheduled,
				SellerTrustActive: true,
				BuyNowPrice:       &buyNow,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanChat:      true,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanBuyNow:    false,
			},
		},
		{
			name: "A2 active non-owner",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusActive,
				SellerTrustActive: true,
				BuyNowPrice:       &buyNow,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanChat:      true,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       true,
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanBuyNow:    true,
			},
		},
		{
			name: "A3 waiting settlement non-owner",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusWaitingSettlement,
				SellerTrustActive: true,
				BuyNowPrice:       &buyNow,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanChat:      true,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanBuyNow:    false,
			},
		},
		{
			name: "A4 ended non-owner",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusEnded,
				SellerTrustActive: true,
				BuyNowPrice:       &buyNow,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanChat:      true,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanBuyNow:    false,
			},
		},
		{
			name: "A5 cancelled non-owner",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusCancelled,
				SellerTrustActive: true,
				BuyNowPrice:       &buyNow,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanChat:      true,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanBuyNow:    false,
			},
		},
		{
			name: "A6 waiting_settlement non-owner",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusWaitingSettlement,
				SellerTrustActive: true,
				BuyNowPrice:       &buyNow,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanChat:      true,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanBuyNow:    false,
			},
		},
		{
			name: "A7 owner scheduled",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          sellerID,
				SellerID:          sellerID,
				Status:            auctionStatusScheduled,
				SellerTrustActive: false,
				BuyNowPrice:       &buyNow,
			},
			want: ViewerCapabilities{
				Role:         "owner",
				CanChat:      false,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanManage:    true,
				CanEdit:      true,
				CanPromote:   false,
				CanBuyNow:    false,
			},
		},
		{
			name: "A8 owner active",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          sellerID,
				SellerID:          sellerID,
				Status:            auctionStatusActive,
				SellerTrustActive: true,
				BuyNowPrice:       &buyNow,
			},
			want: ViewerCapabilities{
				Role:         "owner",
				CanChat:      false,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanManage:    true,
				CanEdit:      false,
				CanPromote:   false,
				CanBuyNow:    false,
			},
		},
		{
			name: "A9 current winner not special",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusActive,
				SellerTrustActive: true,
				BuyNowPrice:       &buyNow,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanChat:      true,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       true,
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanBuyNow:    true,
			},
		},
		{
			name: "A10 seller trust unavailable",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusActive,
				SellerTrustActive: false,
				BuyNowPrice:       &buyNow,
			},
			want: ViewerCapabilities{
				Role:         "buyer",
				CanChat:      false,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanBuyNow:    false,
			},
		},
		{
			name: "A11 guest anonymous",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          uuid.Nil,
				SellerID:          sellerID,
				Status:            auctionStatusActive,
				SellerTrustActive: true,
				BuyNowPrice:       &buyNow,
			},
			want: ViewerCapabilities{
				Role:         "guest",
				CanChat:      false,
				CanNegotiate: false,
				CanBuy:       false,
				CanBid:       false,
				CanManage:    false,
				CanEdit:      false,
				CanPromote:   false,
				CanBuyNow:    false,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EvaluateAuctionViewerCapabilities(tc.in); got != tc.want {
				t.Fatalf("EvaluateAuctionViewerCapabilities() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestEvaluateAuctionViewerCapabilities_StateMatrix_A12ToA30(t *testing.T) {
	sellerID := uuid.New()
	otherID := uuid.New()
	activeBuyNow := int64(1500)
	zeroBuyNow := int64(0)

	buyerChat := ViewerCapabilities{
		Role:         "buyer",
		CanChat:      true,
		CanNegotiate: false,
		CanBuy:       false,
		CanBid:       false,
		CanManage:    false,
		CanEdit:      false,
		CanPromote:   false,
		CanBuyNow:    false,
	}
	buyerBid := ViewerCapabilities{
		Role:         "buyer",
		CanChat:      true,
		CanNegotiate: false,
		CanBuy:       false,
		CanBid:       true,
		CanManage:    false,
		CanEdit:      false,
		CanPromote:   false,
		CanBuyNow:    true,
	}
	buyerBidNoBuyNow := ViewerCapabilities{
		Role:         "buyer",
		CanChat:      true,
		CanNegotiate: false,
		CanBuy:       false,
		CanBid:       true,
		CanManage:    false,
		CanEdit:      false,
		CanPromote:   false,
		CanBuyNow:    false,
	}
	buyerQuiet := ViewerCapabilities{
		Role:         "buyer",
		CanChat:      false,
		CanNegotiate: false,
		CanBuy:       false,
		CanBid:       false,
		CanManage:    false,
		CanEdit:      false,
		CanPromote:   false,
		CanBuyNow:    false,
	}
	ownerDraft := ViewerCapabilities{
		Role:         "owner",
		CanChat:      false,
		CanNegotiate: false,
		CanBuy:       false,
		CanBid:       false,
		CanManage:    true,
		CanEdit:      true,
		CanPromote:   false,
		CanBuyNow:    false,
	}
	ownerLocked := ViewerCapabilities{
		Role:         "owner",
		CanChat:      false,
		CanNegotiate: false,
		CanBuy:       false,
		CanBid:       false,
		CanManage:    true,
		CanEdit:      false,
		CanPromote:   false,
		CanBuyNow:    false,
	}
	guest := ViewerCapabilities{
		Role:         "guest",
		CanChat:      false,
		CanNegotiate: false,
		CanBuy:       false,
		CanBid:       false,
		CanManage:    false,
		CanEdit:      false,
		CanPromote:   false,
		CanBuyNow:    false,
	}

	cases := []struct {
		name string
		in   AuctionViewerCapabilitiesInput
		want ViewerCapabilities
	}{
		{
			name: "A12 guest scheduled",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          uuid.Nil,
				SellerID:          sellerID,
				Status:            auctionStatusScheduled,
				SellerTrustActive: true,
				BuyNowPrice:       &activeBuyNow,
			},
			want: guest,
		},
		{
			name: "A13 guest draft",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          uuid.Nil,
				SellerID:          sellerID,
				Status:            auctionStatusDraft,
				SellerTrustActive: true,
				BuyNowPrice:       &activeBuyNow,
			},
			want: guest,
		},
		{
			name: "A14 non-owner trust inactive active",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusActive,
				SellerTrustActive: false,
				BuyNowPrice:       &activeBuyNow,
			},
			want: buyerQuiet,
		},
		{
			name: "A15 non-owner trust inactive scheduled",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusScheduled,
				SellerTrustActive: false,
				BuyNowPrice:       &activeBuyNow,
			},
			want: buyerQuiet,
		},
		{
			name: "A16 non-owner trust inactive waiting settlement",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusWaitingSettlement,
				SellerTrustActive: false,
				BuyNowPrice:       &activeBuyNow,
			},
			want: buyerQuiet,
		},
		{
			name: "A17 non-owner trust inactive ended",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusEnded,
				SellerTrustActive: false,
				BuyNowPrice:       &activeBuyNow,
			},
			want: buyerQuiet,
		},
		{
			name: "A18 non-owner trust inactive cancelled",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusCancelled,
				SellerTrustActive: false,
				BuyNowPrice:       &activeBuyNow,
			},
			want: buyerQuiet,
		},
		{
			name: "A19 non-owner trust inactive waiting_settlement",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusWaitingSettlement,
				SellerTrustActive: false,
				BuyNowPrice:       &activeBuyNow,
			},
			want: buyerQuiet,
		},
		{
			name: "A20 active non-owner trust active without buy now",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusActive,
				SellerTrustActive: true,
				BuyNowPrice:       nil,
			},
			want: buyerBidNoBuyNow,
		},
		{
			name: "A21 active non-owner trust active with zero buy now",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusActive,
				SellerTrustActive: true,
				BuyNowPrice:       &zeroBuyNow,
			},
			want: buyerBid,
		},
		{
			name: "A22 draft non-owner trust active",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusDraft,
				SellerTrustActive: true,
				BuyNowPrice:       &activeBuyNow,
			},
			want: buyerChat,
		},
		{
			name: "A23 scheduled non-owner trust active",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusScheduled,
				SellerTrustActive: true,
				BuyNowPrice:       &activeBuyNow,
			},
			want: buyerChat,
		},
		{
			name: "A24 waiting settlement non-owner trust active",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusWaitingSettlement,
				SellerTrustActive: true,
				BuyNowPrice:       &activeBuyNow,
			},
			want: buyerChat,
		},
		{
			name: "A25 ended non-owner trust active",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusEnded,
				SellerTrustActive: true,
				BuyNowPrice:       &activeBuyNow,
			},
			want: buyerChat,
		},
		{
			name: "A26 cancelled non-owner trust active",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusCancelled,
				SellerTrustActive: true,
				BuyNowPrice:       &activeBuyNow,
			},
			want: buyerChat,
		},
		{
			name: "A27 waiting_settlement non-owner trust active",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          otherID,
				SellerID:          sellerID,
				Status:            auctionStatusWaitingSettlement,
				SellerTrustActive: true,
				BuyNowPrice:       &activeBuyNow,
			},
			want: buyerChat,
		},
		{
			name: "A28 owner active",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          sellerID,
				SellerID:          sellerID,
				Status:            auctionStatusActive,
				SellerTrustActive: false,
				BuyNowPrice:       &activeBuyNow,
			},
			want: ownerLocked,
		},
		{
			name: "A29 owner draft",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          sellerID,
				SellerID:          sellerID,
				Status:            auctionStatusDraft,
				SellerTrustActive: true,
				BuyNowPrice:       &activeBuyNow,
			},
			want: ownerDraft,
		},
		{
			name: "A30 owner scheduled",
			in: AuctionViewerCapabilitiesInput{
				ViewerID:          sellerID,
				SellerID:          sellerID,
				Status:            auctionStatusScheduled,
				SellerTrustActive: false,
				BuyNowPrice:       &activeBuyNow,
			},
			want: ownerDraft,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EvaluateAuctionViewerCapabilities(tc.in); got != tc.want {
				t.Fatalf("EvaluateAuctionViewerCapabilities() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
