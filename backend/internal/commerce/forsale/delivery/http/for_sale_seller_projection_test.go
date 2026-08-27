package http

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/require"
)

func TestBuildForSaleSellerProjection(t *testing.T) {
	t.Setenv("ENABLE_PUBLIC_SELLER_TIER_PROFILE", "1")

	sellerID := uuid.New()
	pro, ok := buildForSaleSellerProjection(
		sellerID,
		"  user_deadbeef  ",
		"  https://example.com/avatar.jpg  ",
		"  Acme Farm  ",
		"active",
		false,
		"active",
		"pro",
	)
	require.True(t, ok)
	require.Equal(t, sellerID, pro.Author.ID)
	require.Equal(t, "user_deadbeef", pro.Author.Username)
	require.NotNil(t, pro.Author.AvatarURL)
	require.Equal(t, "https://example.com/avatar.jpg", *pro.Author.AvatarURL)
	require.NotNil(t, pro.Author.Lifecycle)
	require.Equal(t, "active", *pro.Author.Lifecycle)
	require.Equal(t, "Acme Farm", pro.FarmName)
	require.NotNil(t, pro.Seller.FarmName)
	require.Equal(t, "Acme Farm", *pro.Seller.FarmName)
	require.NotNil(t, pro.Seller.AvatarURL)
	require.Equal(t, "https://example.com/avatar.jpg", *pro.Seller.AvatarURL)
	require.NotNil(t, pro.Seller.Lifecycle)
	require.Equal(t, "active", *pro.Seller.Lifecycle)
	require.NotNil(t, pro.Seller.Tier)
	require.Equal(t, "pro", *pro.Seller.Tier)

	cases := []struct {
		name                string
		sellerID            uuid.UUID
		username            string
		avatarURL           string
		farmName            string
		accountStatus       string
		isDeleted           bool
		subscriptionStatus  string
		wantOK              bool
		wantUsername        string
		wantUserLifecycle   string
		wantSellerLifecycle string
		wantFarmName        string
		wantAvatar          string
		wantTier            bool
	}{
		{
			name:                "blank username redacts without synthetic fallback",
			sellerID:            uuid.New(),
			username:            "",
			avatarURL:           "https://example.com/avatar.jpg",
			farmName:            "Acme Farm",
			accountStatus:       "active",
			isDeleted:           false,
			subscriptionStatus:  "active",
			wantOK:              true,
			wantUsername:        "",
			wantUserLifecycle:   "unavailable",
			wantSellerLifecycle: "active",
			wantFarmName:        "Acme Farm",
			wantAvatar:          "",
			wantTier:            false,
		},
		{
			name:                "whitespace username redacts without synthetic fallback",
			sellerID:            uuid.New(),
			username:            "   ",
			avatarURL:           "https://example.com/avatar.jpg",
			farmName:            "Acme Farm",
			accountStatus:       "active",
			isDeleted:           false,
			subscriptionStatus:  "active",
			wantOK:              true,
			wantUsername:        "",
			wantUserLifecycle:   "unavailable",
			wantSellerLifecycle: "active",
			wantFarmName:        "Acme Farm",
			wantAvatar:          "",
			wantTier:            false,
		},
		{
			name:                "suspended user redacts identity",
			sellerID:            uuid.New(),
			username:            "seller_user",
			avatarURL:           "https://example.com/avatar.jpg",
			farmName:            "Acme Farm",
			accountStatus:       "suspended",
			isDeleted:           false,
			subscriptionStatus:  "active",
			wantOK:              true,
			wantUsername:        "",
			wantUserLifecycle:   "unavailable",
			wantSellerLifecycle: "active",
			wantFarmName:        "Acme Farm",
			wantAvatar:          "",
			wantTier:            false,
		},
		{
			name:                "banned user redacts identity",
			sellerID:            uuid.New(),
			username:            "seller_user",
			avatarURL:           "https://example.com/avatar.jpg",
			farmName:            "Acme Farm",
			accountStatus:       "banned",
			isDeleted:           false,
			subscriptionStatus:  "active",
			wantOK:              true,
			wantUsername:        "",
			wantUserLifecycle:   "unavailable",
			wantSellerLifecycle: "active",
			wantFarmName:        "Acme Farm",
			wantAvatar:          "",
			wantTier:            false,
		},
		{
			name:                "removed user redacts identity",
			sellerID:            uuid.New(),
			username:            "seller_user",
			avatarURL:           "https://example.com/avatar.jpg",
			farmName:            "Acme Farm",
			accountStatus:       "active",
			isDeleted:           true,
			subscriptionStatus:  "active",
			wantOK:              true,
			wantUsername:        "",
			wantUserLifecycle:   "unavailable",
			wantSellerLifecycle: "active",
			wantFarmName:        "Acme Farm",
			wantAvatar:          "",
			wantTier:            false,
		},
		{
			name:                "missing profile row stays blank and fails closed",
			sellerID:            uuid.New(),
			username:            "",
			avatarURL:           "",
			farmName:            "",
			accountStatus:       "active",
			isDeleted:           false,
			subscriptionStatus:  "active",
			wantOK:              true,
			wantUsername:        "",
			wantUserLifecycle:   "unavailable",
			wantSellerLifecycle: "unavailable",
			wantFarmName:        "",
			wantAvatar:          "",
			wantTier:            false,
		},
		{
			name:       "nil seller id fails closed",
			sellerID:   uuid.Nil,
			username:   "seller_user",
			avatarURL:  "https://example.com/avatar.jpg",
			farmName:   "Acme Farm",
			wantOK:     false,
			wantTier:   false,
			wantAvatar: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proj, ok := buildForSaleSellerProjection(
				tc.sellerID,
				tc.username,
				tc.avatarURL,
				tc.farmName,
				tc.accountStatus,
				tc.isDeleted,
				tc.subscriptionStatus,
				"pro",
			)
			require.Equal(t, tc.wantOK, ok)
			if !ok {
				return
			}

			require.Equal(t, tc.wantUsername, proj.Author.Username)
			if tc.wantAvatar == "" {
				require.Nil(t, proj.Author.AvatarURL)
				require.Nil(t, proj.Seller.AvatarURL)
			} else {
				require.NotNil(t, proj.Author.AvatarURL)
				require.Equal(t, tc.wantAvatar, *proj.Author.AvatarURL)
				require.NotNil(t, proj.Seller.AvatarURL)
				require.Equal(t, tc.wantAvatar, *proj.Seller.AvatarURL)
			}

			require.NotNil(t, proj.Author.Lifecycle)
			require.Equal(t, tc.wantUserLifecycle, *proj.Author.Lifecycle)
			if tc.wantFarmName == "" {
				require.Nil(t, proj.Seller.FarmName)
			} else {
				require.NotNil(t, proj.Seller.FarmName)
				require.Equal(t, tc.wantFarmName, *proj.Seller.FarmName)
			}
			require.NotNil(t, proj.Seller.Lifecycle)
			require.Equal(t, tc.wantSellerLifecycle, *proj.Seller.Lifecycle)
			if tc.wantUserLifecycle != "active" {
				require.Nil(t, proj.Seller.Tier)
				return
			}
			require.NotNil(t, proj.Seller.Tier)
		})
	}
}

func TestForSaleToResponseWithSellerProjection_SerializesCanonicalSellerIdentity(t *testing.T) {
	t.Setenv("ENABLE_PUBLIC_SELLER_TIER_PROFILE", "1")

	sellerID := uuid.New()
	for_sale := testForSale(sellerID)
	sellerInfo := sellerdisplay.Info{
		Username:           "  user_deadbeef  ",
		FarmName:           "  Acme Farm  ",
		AvatarURL:          "  https://example.com/avatar.jpg  ",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
		Tier:               "pro",
	}

	resp := for_saleToResponseWithSellerProjection(for_sale, sellerInfo)
	raw, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &decoded))

	require.Equal(t, "user_deadbeef", decoded["seller_username"])
	require.Equal(t, "Acme Farm", decoded["seller_farm_name"])
	require.Equal(t, "https://example.com/avatar.jpg", decoded["seller_avatar_url"])

	forSale, ok := decoded["for_sale"].(map[string]interface{})
	require.True(t, ok)
	seller, ok := forSale["seller"].(map[string]interface{})
	require.True(t, ok)
	user, ok := seller["user"].(map[string]interface{})
	require.True(t, ok)

	require.Equal(t, "user_deadbeef", user["username"])
	require.Equal(t, "active", user["lifecycle"])
	require.Equal(t, "Acme Farm", seller["farm_name"])
	require.Equal(t, "https://example.com/avatar.jpg", seller["avatar_url"])
	require.Equal(t, "active", seller["lifecycle"])
	require.Equal(t, "pro", seller["tier"])
}

func TestForSaleToDetailResponseWithSellerProjection_EmitsCanonicalSellerIdentity(t *testing.T) {
	for_sale := testForSale(uuid.New())
	sellerInfo := sellerdisplay.Info{
		Username:         "  user_deadbeef  ",
		FarmName:         "  Acme Farm  ",
		StoreImageURL:    "  https://example.com/store.jpg  ",
		AvatarURL:        "  https://example.com/avatar.jpg  ",
		PublicOriginLine: "  Magelang, Jawa Tengah  ",
	}

	viewerID := for_sale.SellerID
	resp := for_saleToDetailResponseWithSellerProjection(for_sale, sellerInfo, &viewerID)
	raw, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &decoded))

	sellerIdentity, ok := decoded["seller_identity"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "Acme Farm", sellerIdentity["store_name"])
	require.Equal(t, "https://example.com/store.jpg", sellerIdentity["store_image_url"])
	require.Equal(t, "user_deadbeef", sellerIdentity["username"])
	require.Equal(t, "https://example.com/avatar.jpg", sellerIdentity["avatar_url"])
	require.Equal(t, "Magelang, Jawa Tengah", sellerIdentity["public_origin_line"])

	capabilities, ok := decoded["viewer_capabilities"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "owner", capabilities["role"])
	require.Equal(t, true, capabilities["can_manage"])
	require.Equal(t, true, capabilities["can_edit"])
	require.Equal(t, true, capabilities["can_promote"])
}

func TestForSaleToDetailResponseWithSellerProjection_EmitsCanonicalProductFields(t *testing.T) {
	for_sale := testForSale(uuid.New())
	for_sale.Product = &productEntity.Product{
		ID:              uuid.New(),
		SellerID:        for_sale.SellerID,
		Title:           "Showa Koi 30cm",
		Description:     "Premium showa",
		MediaURLs:       []string{"https://example.com/thumb.jpg"},
		Variety:         "Showa",
		SizeCm:          ptrInt(30),
		AgeMonths:       ptrInt(8),
		Gender:          ptrString("female"),
		Breeder:         ptrString("Acme Farm"),
		Bloodline:       ptrString("Ogata"),
		Certificates:    []string{"cert-a"},
		PreparationTime: "short",
		PreparationNote: ptrString("Pack carefully"),
	}
	sellerInfo := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
		Tier:               "pro",
	}

	resp := for_saleToDetailResponseWithSellerProjection(for_sale, sellerInfo, nil)
	raw, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &decoded))

	require.NotContains(t, decoded, "origin")
	require.NotContains(t, decoded, "shipping_options")
	require.Equal(t, "Showa Koi 30cm", decoded["title"])
	require.Equal(t, "Premium showa", decoded["description"])
	require.Equal(t, "Showa", decoded["variety"])
	require.Equal(t, "short", decoded["preparation_time"])
	require.Equal(t, "Pack carefully", decoded["preparation_note"])
}

func TestBuildForSaleViewerCapabilities_CanonicalCases(t *testing.T) {
	sellerInfoActive := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
		Tier:               "pro",
	}
	sellerInfoInactive := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "expired",
		Tier:               "pro",
	}

	cases := []struct {
		name     string
		for_sale func() (*entity.ForSale, *uuid.UUID)
		seller   sellerdisplay.Info
		want     commerceshared.ViewerCapabilities
	}{
		{
			name: "guest active available",
			for_sale: func() (*entity.ForSale, *uuid.UUID) {
				l := testForSale(uuid.New())
				return l, nil
			},
			seller: sellerInfoActive,
			want: commerceshared.ViewerCapabilities{
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
			for_sale: func() (*entity.ForSale, *uuid.UUID) {
				l := testForSale(uuid.New())
				l.NegotiationEnabled = false
				viewerID := uuid.New()
				return l, &viewerID
			},
			seller: sellerInfoActive,
			want: commerceshared.ViewerCapabilities{
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
			for_sale: func() (*entity.ForSale, *uuid.UUID) {
				l := testForSale(uuid.New())
				l.NegotiationEnabled = true
				viewerID := uuid.New()
				return l, &viewerID
			},
			seller: sellerInfoActive,
			want: commerceshared.ViewerCapabilities{
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
			for_sale: func() (*entity.ForSale, *uuid.UUID) {
				l := testForSale(uuid.New())
				l.Status = entity.ForSaleStatusSold
				l.QuantityAvailable = 0
				l.NegotiationEnabled = true
				viewerID := uuid.New()
				return l, &viewerID
			},
			seller: sellerInfoActive,
			want: commerceshared.ViewerCapabilities{
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
			for_sale: func() (*entity.ForSale, *uuid.UUID) {
				ownerID := uuid.New()
				l := testForSale(ownerID)
				l.Status = entity.ForSaleStatusDraft
				return l, &ownerID
			},
			seller: sellerInfoActive,
			want: commerceshared.ViewerCapabilities{
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
			for_sale: func() (*entity.ForSale, *uuid.UUID) {
				ownerID := uuid.New()
				l := testForSale(ownerID)
				return l, &ownerID
			},
			seller: sellerInfoActive,
			want: commerceshared.ViewerCapabilities{
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
			for_sale: func() (*entity.ForSale, *uuid.UUID) {
				l := testForSale(uuid.New())
				viewerID := uuid.New()
				return l, &viewerID
			},
			seller: sellerInfoInactive,
			want: commerceshared.ViewerCapabilities{
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
			for_sale: func() (*entity.ForSale, *uuid.UUID) {
				l := testForSale(uuid.New())
				l.QuantityAvailable = 0
				viewerID := uuid.New()
				return l, &viewerID
			},
			seller: sellerInfoActive,
			want: commerceshared.ViewerCapabilities{
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
			for_sale, viewerID := tc.for_sale()
			got := buildForSaleViewerCapabilities(for_sale, tc.seller, viewerID)
			require.Equal(t, tc.want, got)
		})
	}
}

func testForSale(sellerID uuid.UUID) *entity.ForSale {
	now := time.Date(2026, time.July, 24, 10, 0, 0, 0, time.UTC)
	return &entity.ForSale{
		ID:                 uuid.New(),
		ProductID:          uuid.New(),
		SellerID:           sellerID,
		Title:              "Showa Koi 30cm",
		Description:        "Premium showa",
		PricePerUnit:       money.New(1500000),
		QuantityAvailable:  1,
		NegotiationEnabled: false,
		Visibility:         entity.ForSaleVisibilityPublic,
		Status:             entity.ForSaleStatusActive,
		CreatedAt:          now,
		UpdatedAt:          now,
		Product: &productEntity.Product{
			ID:           uuid.New(),
			SellerID:     sellerID,
			Title:        "Showa Koi 30cm",
			Description:  "Premium showa",
			MediaURLs:    []string{"https://example.com/thumb.jpg"},
			Variety:      "Showa",
			Certificates: []string{"cert-1"},
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
}

func ptrInt(v int) *int {
	return &v
}

func ptrString(v string) *string {
	return &v
}
