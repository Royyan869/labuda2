package serverboot

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/stretchr/testify/require"
)

func TestBuildAuctionCommerceActions_MapsCanonicalCapabilities(t *testing.T) {
	caps := commerceshared.ViewerCapabilities{
		Role:         "buyer",
		CanManage:    false,
		CanEdit:      false,
		CanPromote:   false,
		CanChat:      true,
		CanNegotiate: true,
		CanBuy:       false,
		CanBid:       true,
		CanBuyNow:    true,
	}

	got := buildAuctionCommerceActions(caps)
	require.Equal(t, chatApp.CommerceActionCapabilities{
		Role:         caps.Role,
		CanChat:      caps.CanChat,
		CanNegotiate: false,
		CanBuy:       true,
		CanBid:       caps.CanBid,
		CanManage:    caps.CanManage,
	}, got)
}

func TestBuildAuctionCommerceActions_OwnerKeepsManageOnly(t *testing.T) {
	caps := commerceshared.ViewerCapabilities{
		Role:         "owner",
		CanManage:    true,
		CanEdit:      true,
		CanPromote:   false,
		CanChat:      false,
		CanNegotiate: false,
		CanBuy:       false,
		CanBid:       false,
		CanBuyNow:    false,
	}

	got := buildAuctionCommerceActions(caps)
	require.Equal(t, chatApp.CommerceActionCapabilities{
		Role:         "owner",
		CanChat:      false,
		CanNegotiate: false,
		CanBuy:       false,
		CanBid:       false,
		CanManage:    true,
	}, got)
}

func TestNewAuctionOccurrenceWithOperation_SeedsAuctionType(t *testing.T) {
	msgID := uuid.New()
	auctionID := uuid.New()

	occ := chatEntity.NewChatMessageResourceOccurrence(
		msgID,
		chatEntity.ResourceOccurrenceOperationShareToChat,
		chatEntity.ResourceOccurrenceResourceTypeAuction,
		auctionID,
		json.RawMessage(`{}`),
	)
	require.NotNil(t, occ)
	require.Equal(t, msgID, occ.MessageID)
	require.Equal(t, chatEntity.ResourceOccurrenceOperationShareToChat, occ.Operation)
	require.Equal(t, chatEntity.ResourceOccurrenceResourceTypeAuction, occ.ResourceType())
	require.Equal(t, auctionID, occ.SourceID())
	require.Equal(t, json.RawMessage(`{}`), occ.FallbackSnapshot)
}
