package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageToResponse_AttachmentMetadata_SeparateFromAttachmentJSON(t *testing.T) {
	msg := &chatEntity.ChatMessage{
		ID:          uuid.New(),
		RoomID:      uuid.New(),
		SenderID:    uuid.New(),
		MessageType: chatEntity.MessageTypeShippingQuote,
		CreatedAt:   time.Now(),
		AttachmentJSON: map[string]interface{}{
			"type": "shipping_quote",
			"data": map[string]interface{}{
				"offer_id":         uuid.New().String(),
				"linked_item_id":   uuid.New().String(),
				"linked_item_type": "for_sale",
			},
		},
	}
	sellerLifecycles := map[string]attachmentSellerLifecycle{
		msg.AttachmentJSON["data"].(map[string]interface{})["linked_item_id"].(string): {
			userLifecycle:        "unavailable",
			sellerTrustLifecycle: "active",
		},
	}

	resp := messageToResponse(msg, nil, sellerLifecycles)
	att, ok := resp["attachment_json"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "shipping_quote", att["type"])
	_, hasOldTopLevelUser := att["seller_user_lifecycle"]
	_, hasOldTopLevelTrust := att["seller_trust_lifecycle"]
	assert.False(t, hasOldTopLevelUser)
	assert.False(t, hasOldTopLevelTrust)

	meta, ok := resp["attachment_metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "unavailable", meta["seller_user_lifecycle"])
	assert.Equal(t, "active", meta["seller_trust_lifecycle"])
}

func TestExtractItemIDFromAttachment_ReferenceForSaleAndAuction(t *testing.T) {
	forSaleID := uuid.New().String()
	auctionID := uuid.New().String()

	forSaleAtt := map[string]interface{}{
		"type": "reference",
		"data": map[string]interface{}{
			"target_type": "for_sale",
			"target_id":   forSaleID,
		},
	}
	auctionAtt := map[string]interface{}{
		"type": "reference",
		"data": map[string]interface{}{
			"target_type": "auction",
			"target_id":   auctionID,
		},
	}
	postAtt := map[string]interface{}{
		"type": "reference",
		"data": map[string]interface{}{
			"target_type": "post",
			"target_id":   uuid.New().String(),
		},
	}

	assert.Equal(t, forSaleID, extractReferencedItemIDFromAttachment(forSaleAtt))
	assert.Equal(t, auctionID, extractReferencedItemIDFromAttachment(auctionAtt))
	assert.Equal(t, "", extractReferencedItemIDFromAttachment(postAtt))
}


