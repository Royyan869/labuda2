package serverboot

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/stretchr/testify/require"
)

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
