package serverboot

import (
	"testing"

	"github.com/google/uuid"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	"github.com/stretchr/testify/require"
)

func TestBuildForSaleCommerceActions_MapsCanonicalCapabilities(t *testing.T) {
	viewerID := uuid.New()
	sellerID := uuid.New()
	productID := uuid.New()

	caps := commerceshared.EvaluateForSaleViewerCapabilities(commerceshared.ForSaleViewerCapabilitiesInput{
		ViewerID:           viewerID,
		SellerID:           sellerID,
		ProductID:          productID,
		Status:             "active",
		QuantityAvailable:  2,
		NegotiationEnabled: true,
		SellerTrustActive:  true,
	})

	got := buildForSaleCommerceActions(caps)
	require.Equal(t, chatApp.CommerceActionCapabilities{
		Role:         caps.Role,
		CanChat:      caps.CanChat,
		CanNegotiate: caps.CanNegotiate,
		CanBuy:       caps.CanBuy,
		CanBid:       caps.CanBid,
		CanManage:    caps.CanManage,
	}, got)
}

func TestBuildForSaleCommerceActions_OwnerHasNoBidOrChat(t *testing.T) {
	sellerID := uuid.New()
	caps := commerceshared.EvaluateForSaleViewerCapabilities(commerceshared.ForSaleViewerCapabilitiesInput{
		ViewerID:           sellerID,
		SellerID:           sellerID,
		ProductID:          uuid.New(),
		Status:             "draft",
		QuantityAvailable:  1,
		NegotiationEnabled: false,
		SellerTrustActive:  true,
	})

	got := buildForSaleCommerceActions(caps)
	require.False(t, got.CanChat)
	require.False(t, got.CanNegotiate)
	require.False(t, got.CanBuy)
	require.False(t, got.CanBid)
	require.True(t, got.CanManage)
}
