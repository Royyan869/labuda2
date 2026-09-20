//go:build integration

package serverboot

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	contentEntity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/stretchr/testify/require"
)

// occurrenceRow is the persisted communication reference for a message.
type occurrenceRow struct {
	Operation string
	Sources   map[string]string // resource_type -> source id
	Fallback  map[string]any
}

func (f *chatProjectionHTTPFixture) occurrenceForMessage(t *testing.T, messageID uuid.UUID) occurrenceRow {
	t.Helper()

	var (
		op                                      string
		profileID, contentID, saleID, auctionID *string
		fallbackJSON                            []byte
	)
	err := f.appDB.Pool().QueryRow(context.Background(), `
		SELECT operation::text,
		       profile_source_id::text, content_source_id::text,
		       for_sale_source_id::text, auction_source_id::text,
		       fallback_snapshot
		FROM chat_message_resource_occurrences
		WHERE message_id = $1
	`, messageID).Scan(&op, &profileID, &contentID, &saleID, &auctionID, &fallbackJSON)
	require.NoError(t, err, "occurrence row must be persisted for message %s", messageID)

	sources := map[string]string{}
	if profileID != nil {
		sources["profile"] = *profileID
	}
	if contentID != nil {
		sources["content"] = *contentID
	}
	if saleID != nil {
		sources["for_sale"] = *saleID
	}
	if auctionID != nil {
		sources["auction"] = *auctionID
	}

	var fallback map[string]any
	require.NoError(t, json.Unmarshal(fallbackJSON, &fallback))

	return occurrenceRow{Operation: op, Sources: sources, Fallback: fallback}
}

func (f *chatProjectionHTTPFixture) countMessages(t *testing.T, messageID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, f.appDB.Pool().QueryRow(context.Background(),
		`SELECT COUNT(*) FROM chat_messages WHERE id = $1`, messageID).Scan(&count))
	return count
}

// sendWithOccurrence performs the canonical Chat send flow carrying a resource
// reference and returns the message data map from the response.
func sendWithOccurrence(
	t *testing.T,
	f *chatProjectionHTTPFixture,
	senderID, roomID uuid.UUID,
	operation, resourceType string,
	resourceID string,
) map[string]any {
	t.Helper()

	status, resp := f.doJSON(t, f.routerFor(senderID), http.MethodPost,
		"/api/v1/chat/rooms/"+roomID.String()+"/messages",
		map[string]any{
			"message_type":    "text",
			"body":            "referencing a resource",
			"idempotency_key": uuid.NewString(),
			"resource_occurrence": map[string]any{
				"operation":     operation,
				"resource_type": resourceType,
				"resource_id":   resourceID,
			},
		},
	)
	require.Equal(t, http.StatusOK, status)
	msg, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	return msg
}

// The canonical Chat send flow persists the resource occurrence inside the
// message transaction, and returns the viewer-aware projection.
func TestChatResourceProjectionHTTPWritePathPersistsOccurrence(t *testing.T) {
	fixture := newChatProjectionHTTPFixture(t)

	t.Run("profile share_to_chat", func(t *testing.T) {
		sender := fixture.seedUser(t, "active", nil, uniqueUsername("wp-profile-sender"), nil, nil, nil)
		target := fixture.seedUser(t, "active", nil, uniqueUsername("wp-profile-target"), nil, nil, nil)
		room := fixture.seedRoom(t, sender, target)

		msg := sendWithOccurrence(t, fixture, sender, room, "share_to_chat", "profile", target.String())
		requireProjectionIsLive(t, msg, "profile", target)

		messageID := uuid.MustParse(msg["id"].(string))
		require.Equal(t, 1, fixture.countMessages(t, messageID), "message must be persisted")

		occ := fixture.occurrenceForMessage(t, messageID)
		require.Equal(t, "share_to_chat", occ.Operation)
		require.Equal(t, target.String(), occ.Sources["profile"])
		require.Len(t, occ.Sources, 1)
		require.NotEmpty(t, occ.Fallback)
	})

	t.Run("content share_to_chat", func(t *testing.T) {
		sender := fixture.seedUser(t, "active", nil, uniqueUsername("wp-content-sender"), nil, nil, nil)
		author := fixture.seedUser(t, "active", nil, uniqueUsername("wp-content-author"), nil, nil, nil)
		room := fixture.seedRoom(t, sender, author)
		contentID := fixture.seedContent(t, author, uniqueUsername("wp-content"), contentEntity.VisibilityPublic)

		msg := sendWithOccurrence(t, fixture, sender, room, "share_to_chat", "content", contentID.String())
		requireProjectionIsLive(t, msg, "content", contentID)

		occ := fixture.occurrenceForMessage(t, uuid.MustParse(msg["id"].(string)))
		require.Equal(t, contentID.String(), occ.Sources["content"])
		require.Len(t, occ.Sources, 1)
	})

	t.Run("for_sale share_to_chat", func(t *testing.T) {
		sender := fixture.seedUser(t, "active", nil, uniqueUsername("wp-sale-sender"), nil, nil, nil)
		seller := fixture.seedActiveSeller(t, uniqueUsername("wp-sale-seller"), "WP Sale Farm")
		room := fixture.seedRoom(t, sender, seller)
		saleID := fixture.seedSale(t, seller, uniqueUsername("wp-sale"))

		msg := sendWithOccurrence(t, fixture, sender, room, "share_to_chat", "for_sale", saleID.String())
		requireProjectionIsLive(t, msg, "for_sale", saleID)

		occ := fixture.occurrenceForMessage(t, uuid.MustParse(msg["id"].(string)))
		require.Equal(t, saleID.String(), occ.Sources["for_sale"])
	})

	t.Run("for_sale direct_commerce_insert_chat", func(t *testing.T) {
		sender := fixture.seedUser(t, "active", nil, uniqueUsername("wp-dc-sale-sender"), nil, nil, nil)
		seller := fixture.seedActiveSeller(t, uniqueUsername("wp-dc-sale-seller"), "WP DC Sale Farm")
		room := fixture.seedRoom(t, sender, seller)
		saleID := fixture.seedSale(t, seller, uniqueUsername("wp-dc-sale"))

		msg := sendWithOccurrence(t, fixture, sender, room, "direct_commerce_insert_chat", "for_sale", saleID.String())
		requireProjectionIsLive(t, msg, "for_sale", saleID)

		occ := fixture.occurrenceForMessage(t, uuid.MustParse(msg["id"].(string)))
		require.Equal(t, "direct_commerce_insert_chat", occ.Operation)
	})

	t.Run("auction direct_commerce_insert_chat", func(t *testing.T) {
		sender := fixture.seedUser(t, "active", nil, uniqueUsername("wp-dc-auction-sender"), nil, nil, nil)
		seller := fixture.seedActiveSeller(t, uniqueUsername("wp-dc-auction-seller"), "WP DC Auction Farm")
		room := fixture.seedRoom(t, sender, seller)
		auctionID := fixture.seedAuction(t, seller, uniqueUsername("wp-dc-auction"))

		msg := sendWithOccurrence(t, fixture, sender, room, "direct_commerce_insert_chat", "auction", auctionID.String())
		requireProjectionIsLive(t, msg, "auction", auctionID)

		occ := fixture.occurrenceForMessage(t, uuid.MustParse(msg["id"].(string)))
		require.Equal(t, auctionID.String(), occ.Sources["auction"])
	})
}

// Negative contract cases: Chat validates the reference contract only, and
// rejects malformed client input without persisting anything.
func TestChatResourceProjectionHTTPSendRejectsMalformedOccurrence(t *testing.T) {
	fixture := newChatProjectionHTTPFixture(t)

	sender := fixture.seedUser(t, "active", nil, uniqueUsername("wp-neg-sender"), nil, nil, nil)
	target := fixture.seedUser(t, "active", nil, uniqueUsername("wp-neg-target"), nil, nil, nil)
	room := fixture.seedRoom(t, sender, target)
	router := fixture.routerFor(sender)

	post := func(body map[string]any) int {
		status, _ := fixture.doJSON(t, router, http.MethodPost,
			"/api/v1/chat/rooms/"+room.String()+"/messages", body)
		return status
	}

	base := func(occ map[string]any) map[string]any {
		return map[string]any{
			"message_type":        "text",
			"body":                "hello",
			"idempotency_key":     uuid.NewString(),
			"resource_occurrence": occ,
		}
	}

	require.Equal(t, http.StatusBadRequest, post(base(map[string]any{
		"operation": "share_to_chat", "resource_type": "profile",
		"resource_id": target.String(), "preview": map[string]any{"title": "x"},
	})), "preview must be rejected")

	require.Equal(t, http.StatusBadRequest, post(base(map[string]any{
		"operation": "not_an_operation", "resource_type": "profile", "resource_id": target.String(),
	})), "invalid operation must be rejected")

	require.Equal(t, http.StatusBadRequest, post(base(map[string]any{
		"operation": "share_to_chat", "resource_type": "not_a_type", "resource_id": target.String(),
	})), "invalid resource type must be rejected")

	require.Equal(t, http.StatusBadRequest, post(base(map[string]any{
		"operation": "share_to_chat", "resource_type": "profile",
		"resource_id": uuid.Nil.String(),
	})), "nil resource id must be rejected")

	require.Equal(t, http.StatusBadRequest, post(base(map[string]any{
		"operation": "direct_commerce_insert_chat", "resource_type": "profile", "resource_id": target.String(),
	})), "direct commerce insert on a non-commerce type must be rejected")

	// Nothing persisted for any rejected request.
	var count int
	require.NoError(t, fixture.appDB.Pool().QueryRow(context.Background(),
		`SELECT COUNT(*) FROM chat_message_resource_occurrences occ
		 JOIN chat_messages m ON m.id = occ.message_id
		 WHERE m.room_id = $1`, room).Scan(&count))
	require.Equal(t, 0, count)
}

// NEGATIVE PROOF: the Chat-persisted fallback snapshot is a display
// representation — it must never carry Commerce business truth.
func TestChatResourceProjectionOccurrenceFallbackCarriesNoCommerceAuthority(t *testing.T) {
	fixture := newChatProjectionHTTPFixture(t)

	forbidden := []string{
		"price", "price_per_unit", "amount", "currency", "quantity", "quantity_available",
		"stock", "inventory", "availability", "order_id", "payment", "payment_id",
		"current_bid", "buy_now_price", "settlement", "wallet",
	}

	cases := []struct {
		name       string
		operation  string
		resourceFn func(t *testing.T, room uuid.UUID, sender uuid.UUID) (string, uuid.UUID)
	}{
		{
			name:      "profile",
			operation: "share_to_chat",
			resourceFn: func(t *testing.T, _ uuid.UUID, _ uuid.UUID) (string, uuid.UUID) {
				id := fixture.seedUser(t, "active", nil, uniqueUsername("wp-fb-profile"), nil, nil, nil)
				return "profile", id
			},
		},
		{
			name:      "content",
			operation: "share_to_chat",
			resourceFn: func(t *testing.T, _ uuid.UUID, _ uuid.UUID) (string, uuid.UUID) {
				author := fixture.seedUser(t, "active", nil, uniqueUsername("wp-fb-content-author"), nil, nil, nil)
				return "content", fixture.seedContent(t, author, uniqueUsername("wp-fb-content"), contentEntity.VisibilityPublic)
			},
		},
		{
			name:      "for_sale",
			operation: "share_to_chat",
			resourceFn: func(t *testing.T, _ uuid.UUID, _ uuid.UUID) (string, uuid.UUID) {
				seller := fixture.seedActiveSeller(t, uniqueUsername("wp-fb-sale-seller"), "WP FB Sale")
				return "for_sale", fixture.seedSale(t, seller, uniqueUsername("wp-fb-sale"))
			},
		},
		{
			name:      "auction",
			operation: "share_to_chat",
			resourceFn: func(t *testing.T, _ uuid.UUID, _ uuid.UUID) (string, uuid.UUID) {
				seller := fixture.seedActiveSeller(t, uniqueUsername("wp-fb-auction-seller"), "WP FB Auction")
				return "auction", fixture.seedAuction(t, seller, uniqueUsername("wp-fb-auction"))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sender := fixture.seedUser(t, "active", nil, uniqueUsername("wp-fb-sender-"+tc.name), nil, nil, nil)
			peer := fixture.seedUser(t, "active", nil, uniqueUsername("wp-fb-peer-"+tc.name), nil, nil, nil)
			room := fixture.seedRoom(t, sender, peer)
			resourceType, resourceID := tc.resourceFn(t, room, sender)

			msg := sendWithOccurrence(t, fixture, sender, room, tc.operation, resourceType, resourceID.String())
			occ := fixture.occurrenceForMessage(t, uuid.MustParse(msg["id"].(string)))

			for _, key := range forbidden {
				require.NotContains(t, occ.Fallback, key,
					"fallback snapshot must not carry commerce business truth key %q", key)
			}
		})
	}
}
