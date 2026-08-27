package shared

import (
	"testing"

	"github.com/google/uuid"
)

func TestEvaluateForSaleViewAccess(t *testing.T) {
	sellerID := uuid.New()
	otherID := uuid.New()

	cases := []struct {
		name string
		in   ForSaleViewAccessInput
		want bool
	}{
		{
			name: "public active visible",
			in: ForSaleViewAccessInput{
				ViewerID:   otherID,
				SellerID:   sellerID,
				Status:     "active",
				Visibility: "public",
				Seller:     SellerAccessSnapshot{AccountStatus: "active", SubscriptionStatus: "active"},
			},
			want: true,
		},
		{
			name: "public sold non-owner visible",
			in: ForSaleViewAccessInput{
				ViewerID:   otherID,
				SellerID:   sellerID,
				Status:     "sold",
				Visibility: "public",
				Seller:     SellerAccessSnapshot{AccountStatus: "active", SubscriptionStatus: "active"},
			},
			want: true,
		},
		{
			name: "public withdrawn non-owner visible",
			in: ForSaleViewAccessInput{
				ViewerID:   otherID,
				SellerID:   sellerID,
				Status:     "withdrawn",
				Visibility: "public",
				Seller:     SellerAccessSnapshot{AccountStatus: "active", SubscriptionStatus: "active"},
			},
			want: true,
		},
		{
			name: "draft owner visible",
			in: ForSaleViewAccessInput{
				ViewerID:   sellerID,
				SellerID:   sellerID,
				Status:     "draft",
				Visibility: "private",
				Seller:     SellerAccessSnapshot{AccountStatus: "active", SubscriptionStatus: "active"},
			},
			want: true,
		},
		{
			name: "draft non-owner hidden",
			in: ForSaleViewAccessInput{
				ViewerID:   otherID,
				SellerID:   sellerID,
				Status:     "draft",
				Visibility: "private",
				Seller:     SellerAccessSnapshot{AccountStatus: "active", SubscriptionStatus: "active"},
			},
			want: false,
		},
		{
			name: "seller removed hidden",
			in: ForSaleViewAccessInput{
				ViewerID:   sellerID,
				SellerID:   sellerID,
				Status:     "active",
				Visibility: "public",
				Seller:     SellerAccessSnapshot{AccountStatus: "removed", SubscriptionStatus: "active"},
			},
			want: false,
		},
		{
			name: "subscription expired ignored for view",
			in: ForSaleViewAccessInput{
				ViewerID:   sellerID,
				SellerID:   sellerID,
				Status:     "active",
				Visibility: "public",
				Seller:     SellerAccessSnapshot{AccountStatus: "active", SubscriptionStatus: "expired"},
			},
			want: true,
		},
		{
			name: "blocked hidden",
			in: ForSaleViewAccessInput{
				ViewerID:   otherID,
				SellerID:   sellerID,
				Status:     "active",
				Visibility: "public",
				Blocked:    true,
				Seller:     SellerAccessSnapshot{AccountStatus: "active", SubscriptionStatus: "active"},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EvaluateForSaleViewAccess(tc.in); got != tc.want {
				t.Fatalf("EvaluateForSaleViewAccess() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEvaluateAuctionViewAccess(t *testing.T) {
	sellerID := uuid.New()
	otherID := uuid.New()

	cases := []struct {
		name string
		in   AuctionViewAccessInput
		want bool
	}{
		{
			name: "scheduled visible",
			in: AuctionViewAccessInput{
				ViewerID: sellerID,
				SellerID: sellerID,
				Status:   "scheduled",
				Seller:   SellerAccessSnapshot{AccountStatus: "active", SubscriptionStatus: "active"},
			},
			want: true,
		},
		{
			name: "draft owner visible",
			in: AuctionViewAccessInput{
				ViewerID: sellerID,
				SellerID: sellerID,
				Status:   "draft",
				Seller:   SellerAccessSnapshot{AccountStatus: "active", SubscriptionStatus: "active"},
			},
			want: true,
		},
		{
			name: "draft non-owner hidden",
			in: AuctionViewAccessInput{
				ViewerID: otherID,
				SellerID: sellerID,
				Status:   "draft",
				Seller:   SellerAccessSnapshot{AccountStatus: "active", SubscriptionStatus: "active"},
			},
			want: false,
		},
		{
			name: "seller removed hidden",
			in: AuctionViewAccessInput{
				ViewerID: sellerID,
				SellerID: sellerID,
				Status:   "active",
				Seller:   SellerAccessSnapshot{AccountStatus: "removed", SubscriptionStatus: "active"},
			},
			want: false,
		},
		{
			name: "subscription expired ignored for view",
			in: AuctionViewAccessInput{
				ViewerID: sellerID,
				SellerID: sellerID,
				Status:   "active",
				Seller:   SellerAccessSnapshot{AccountStatus: "active", SubscriptionStatus: "expired"},
			},
			want: true,
		},
		{
			name: "A12 blocked relationship hidden",
			in: AuctionViewAccessInput{
				ViewerID: sellerID,
				SellerID: sellerID,
				Status:   "active",
				Blocked:  true,
				Seller:   SellerAccessSnapshot{AccountStatus: "active", SubscriptionStatus: "active"},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EvaluateAuctionViewAccess(tc.in); got != tc.want {
				t.Fatalf("EvaluateAuctionViewAccess() = %v, want %v", got, tc.want)
			}
		})
	}
}
