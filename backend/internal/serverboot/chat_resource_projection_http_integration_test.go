//go:build integration

package serverboot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	fpsEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	"github.com/labuda/backend/internal/identity/auth"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatHTTP "github.com/labuda/backend/internal/interaction/chat/delivery/http"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	contentEntity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/rate"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type noopChatOutbox struct{}

func (noopChatOutbox) InsertTx(context.Context, db.Tx, string, any, string) error { return nil }

type chatProjectionHTTPFixture struct {
	*aggregateQueryProofFixture
	service *chatApp.Service
	handler *chatHTTP.Handler
}

func newChatProjectionHTTPFixture(t *testing.T) *chatProjectionHTTPFixture {
	t.Helper()

	base := newAggregateQueryProofFixture(t)
	service := chatApp.NewServiceWithDefaults(
		base.traced,
		noopChatOutbox{},
		rate.NewRateLimiter(),
		nil,
		auth.NewAccountStatusCheckerDB(base.traced),
		nil,
		zap.NewNop(),
	)

	handler := chatHTTP.NewHandler(
		service,
		nil,
		nil,
		nil,
		nil,
		base.traced,
		zap.NewNop(),
	)
	handler.SetResourceProjectionResolver(base.resolver)

	return &chatProjectionHTTPFixture{
		aggregateQueryProofFixture: base,
		service:                    service,
		handler:                    handler,
	}
}

func (f *chatProjectionHTTPFixture) routerFor(userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})

	chat := router.Group("/api/v1/chat")
	rooms := chat.Group("/rooms")
	rooms.GET("/", f.handler.ListRooms)
	rooms.GET("/:room_id/messages", f.handler.ListMessages)
	rooms.GET("/:room_id", f.handler.GetRoomByOrderID)
	rooms.POST("/:room_id/messages", f.handler.SendMessage)

	return router
}

func (f *chatProjectionHTTPFixture) seedRoom(t *testing.T, participantA, participantB uuid.UUID) uuid.UUID {
	t.Helper()

	if participantA.String() > participantB.String() {
		participantA, participantB = participantB, participantA
	}
	roomID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO chat_rooms (
			id, room_type, participant_a, participant_b, created_at, updated_at, last_message_at
		)
		VALUES ($1, 'direct', $2, $3, $4, $4, $4)
	`, roomID, participantA, participantB, now)
	require.NoError(t, err)
	return roomID
}

func (f *chatProjectionHTTPFixture) seedMessage(
	t *testing.T,
	roomID, senderID uuid.UUID,
	body *string,
	attachmentJSON json.RawMessage,
	createdAt time.Time,
) uuid.UUID {
	t.Helper()

	messageID := uuid.New()
	idempotencyKey := uuid.NewString()
	var parsedAttachment map[string]interface{}
	if attachmentJSON != nil {
		_ = json.Unmarshal(attachmentJSON, &parsedAttachment)
	}
	fingerprint := chatEntity.ComputeCommandFingerprint(senderID, chatEntity.MessageTypeText, body, parsedAttachment)
	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO chat_messages (
			id, room_id, sender_id, message_type, body, attachment_json,
			idempotency_key, command_fingerprint, created_at
		)
		VALUES ($1, $2, $3, 'text', $4, $5, $6, $7, $8)
	`, messageID, roomID, senderID, body, attachmentJSON, idempotencyKey, fingerprint, createdAt)
	require.NoError(t, err)
	return messageID
}

func (f *chatProjectionHTTPFixture) seedResourceOccurrence(
	t *testing.T,
	messageID uuid.UUID,
	op chatEntity.ResourceOccurrenceOperation,
	rt chatEntity.ResourceOccurrenceResourceType,
	resourceID uuid.UUID,
) {
	t.Helper()

	var profileID, contentID, saleID, auctionID any
	switch rt {
	case chatEntity.ResourceOccurrenceResourceTypeProfile:
		profileID = resourceID
	case chatEntity.ResourceOccurrenceResourceTypeContent:
		contentID = resourceID
	case chatEntity.ResourceOccurrenceResourceTypeForSale:
		saleID = resourceID
	case chatEntity.ResourceOccurrenceResourceTypeAuction:
		auctionID = resourceID
	default:
		t.Fatalf("unsupported resource type %q", rt)
	}

	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO chat_message_resource_occurrences (
			message_id, operation, profile_source_id, content_source_id,
			for_sale_source_id, auction_source_id, fallback_snapshot, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, '{}'::jsonb, NOW())
	`, messageID, string(op), profileID, contentID, saleID, auctionID)
	require.NoError(t, err)
}

func (f *chatProjectionHTTPFixture) seedMessageWithOccurrence(
	t *testing.T,
	roomID, senderID uuid.UUID,
	body *string,
	attachmentJSON json.RawMessage,
	createdAt time.Time,
	op chatEntity.ResourceOccurrenceOperation,
	rt chatEntity.ResourceOccurrenceResourceType,
	resourceID uuid.UUID,
) uuid.UUID {
	t.Helper()

	messageID := f.seedMessage(t, roomID, senderID, body, attachmentJSON, createdAt)
	f.seedResourceOccurrence(t, messageID, op, rt, resourceID)
	return messageID
}

func (f *chatProjectionHTTPFixture) setContentShareReference(t *testing.T, contentID, targetID uuid.UUID, targetType contentEntity.ShareTargetType) {
	t.Helper()

	var actorID uuid.UUID
	require.NoError(t, f.appDB.Pool().QueryRow(context.Background(), `
		SELECT author_id
		FROM contents
		WHERE id = $1
	`, contentID).Scan(&actorID))

	var profileSourceID, contentSourceID, forSaleSourceID, auctionSourceID *uuid.UUID
	switch targetType {
	case contentEntity.ShareTargetTypeProfile:
		profileSourceID = &targetID
	case contentEntity.ShareTargetTypeContent:
		contentSourceID = &targetID
	case contentEntity.ShareTargetTypeForSale:
		forSaleSourceID = &targetID
	case contentEntity.ShareTargetTypeAuction:
		auctionSourceID = &targetID
	default:
		t.Fatalf("unsupported target type %q", targetType)
	}

	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO content_resource_occurrences (
			content_id, actor_id, operation,
			profile_source_id, content_source_id, for_sale_source_id, auction_source_id,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (content_id) DO NOTHING
	`, contentID, actorID, string(contentEntity.ContentResourceOccurrenceOperationShareToFeed), profileSourceID, contentSourceID, forSaleSourceID, auctionSourceID)
	require.NoError(t, err)
}

func (f *chatProjectionHTTPFixture) setForSaleStatus(t *testing.T, saleID uuid.UUID, status fpsEntity.ForSaleStatus) {
	t.Helper()

	_, err := f.appDB.Pool().Exec(context.Background(), `
		UPDATE for_sales
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`, string(status), saleID)
	require.NoError(t, err)
}

func (f *chatProjectionHTTPFixture) setAuctionStatus(t *testing.T, auctionID uuid.UUID, status auctionEntity.Status) {
	t.Helper()

	_, err := f.appDB.Pool().Exec(context.Background(), `
		UPDATE auctions
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`, string(status), auctionID)
	require.NoError(t, err)
}

func (f *chatProjectionHTTPFixture) addBlock(t *testing.T, blockerID, blockedID uuid.UUID) {
	t.Helper()

	_, err := f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO user_blocks (blocker_id, blocked_id, created_at)
		VALUES ($1, $2, NOW())
	`, blockerID, blockedID)
	require.NoError(t, err)
}

func (f *chatProjectionHTTPFixture) doJSON(t *testing.T, router *gin.Engine, method, path string, body any) (int, map[string]any) {
	t.Helper()

	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		require.NoError(t, err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(reqBody))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Body.Len() == 0 {
		return w.Code, nil
	}

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &decoded))
	return w.Code, decoded
}

func messageDataFromHTTPResponse(t *testing.T, resp map[string]any) []map[string]any {
	t.Helper()

	outer, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	raw, ok := outer["data"].([]any)
	require.True(t, ok)

	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		out = append(out, item.(map[string]any))
	}
	return out
}

func roomDataFromHTTPResponse(t *testing.T, resp map[string]any) []map[string]any {
	t.Helper()

	outer, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	raw, ok := outer["data"].([]any)
	require.True(t, ok)

	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		out = append(out, item.(map[string]any))
	}
	return out
}

func messageByID(messages []map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any, len(messages))
	for _, msg := range messages {
		out[msg["id"].(string)] = msg
	}
	return out
}

func mustProjectionMap(t *testing.T, msg map[string]any) map[string]any {
	t.Helper()

	raw, ok := msg["resource_projection"]
	require.True(t, ok)
	proj, ok := raw.(map[string]any)
	require.True(t, ok)
	return proj
}

func requireNoProjection(t *testing.T, msg map[string]any) {
	t.Helper()

	_, ok := msg["resource_projection"]
	require.False(t, ok)
}

func requireMessageProjectionState(t *testing.T, msg map[string]any, wantState, wantType string) map[string]any {
	t.Helper()

	proj := mustProjectionMap(t, msg)
	require.Equal(t, wantState, proj["state"])
	require.Equal(t, wantType, proj["resource_type"])
	return proj
}

func requireProjectionResourceID(t *testing.T, proj map[string]any, wantID uuid.UUID) {
	t.Helper()

	require.Equal(t, wantID.String(), proj["resource_id"])
}

func requireContentNestedResource(t *testing.T, proj map[string]any, wantType string, wantID uuid.UUID) {
	t.Helper()

	content, ok := proj["content"].(map[string]any)
	require.True(t, ok)
	nested, ok := content["nested_resource"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, wantType, nested["resource_type"])
	require.Equal(t, wantID.String(), nested["resource_id"])
}

func requireNoContentNestedResource(t *testing.T, proj map[string]any) {
	t.Helper()

	content, ok := proj["content"].(map[string]any)
	require.True(t, ok)
	_, ok = content["nested_resource"]
	require.False(t, ok)
}

func requireFPSPayload(t *testing.T, proj map[string]any) map[string]any {
	t.Helper()

	fps, ok := proj["for_sale"].(map[string]any)
	require.True(t, ok)
	return fps
}

func requireAuctionPayload(t *testing.T, proj map[string]any) map[string]any {
	t.Helper()

	auction, ok := proj["auction"].(map[string]any)
	require.True(t, ok)
	return auction
}

func requireProjectionIsTombstone(t *testing.T, msg map[string]any, wantType string) {
	t.Helper()

	proj := mustProjectionMap(t, msg)
	require.Equal(t, "TOMBSTONE", proj["state"])
	require.Equal(t, wantType, proj["resource_type"])
}

func requireProjectionIsLive(t *testing.T, msg map[string]any, wantType string, wantID uuid.UUID) map[string]any {
	t.Helper()

	proj := mustProjectionMap(t, msg)
	require.Equal(t, "LIVE", proj["state"])
	require.Equal(t, wantType, proj["resource_type"])
	require.Equal(t, wantID.String(), proj["resource_id"])
	return proj
}

func TestChatResourceProjectionHTTPMatrix(t *testing.T) {
	fixture := newChatProjectionHTTPFixture(t)

	t.Run("H1 normal text message", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h1-viewer"), nil, nil, nil)
		otherID := fixture.seedUser(t, "active", nil, uniqueUsername("h1-other"), nil, nil, nil)
		roomID := fixture.seedRoom(t, viewerID, otherID)
		messageID := fixture.seedMessage(t, roomID, otherID, strPtr("hello"), nil, time.Now().UTC())

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		messages := messageDataFromHTTPResponse(t, resp)
		require.Len(t, messages, 1)
		msg := messageByID(messages)[messageID.String()]
		requireNoProjection(t, msg)
		require.Equal(t, "hello", msg["body"])
	})

	t.Run("H2 normal media attachment message", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h2-viewer"), nil, nil, nil)
		otherID := fixture.seedUser(t, "active", nil, uniqueUsername("h2-other"), nil, nil, nil)
		roomID := fixture.seedRoom(t, viewerID, otherID)
		attachment := json.RawMessage(`{"type":"image","data":{"url":"https://cdn.example.test/media.jpg","filename":"media.jpg"}}`)
		messageID := fixture.seedMessage(t, roomID, otherID, nil, attachment, time.Now().UTC())

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		messages := messageDataFromHTTPResponse(t, resp)
		require.Len(t, messages, 1)
		msg := messageByID(messages)[messageID.String()]
		requireNoProjection(t, msg)
		require.NotNil(t, msg["attachment_json"])
	})

	t.Run("H3 Profile LIVE occurrence", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h3-viewer"), nil, nil, nil)
		targetID := fixture.seedUser(t, "active", nil, uniqueUsername("h3-profile"), nil, nil, nil)
		roomID := fixture.seedRoom(t, viewerID, targetID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, targetID, strPtr("profile"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, targetID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		proj := requireProjectionIsLive(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeProfile), targetID)
		profile, ok := proj["profile"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "active", profile["lifecycle"])
	})

	t.Run("H4 Profile TOMBSTONE", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h4-viewer"), nil, nil, nil)
		targetID := fixture.seedUser(t, "suspended", nil, uniqueUsername("h4-profile"), nil, nil, nil)
		roomID := fixture.seedRoom(t, viewerID, targetID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, targetID, strPtr("profile"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, targetID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		requireProjectionIsTombstone(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeProfile))
	})

	t.Run("H5 Content LIVE", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h5-viewer"), nil, nil, nil)
		authorID := fixture.seedUser(t, "active", nil, uniqueUsername("h5-author"), nil, nil, nil)
		contentID := fixture.seedContent(t, authorID, "h5-content", contentEntity.VisibilityPublic)
		roomID := fixture.seedRoom(t, viewerID, authorID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, authorID, strPtr("content"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		proj := requireProjectionIsLive(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeContent), contentID)
		require.NotNil(t, proj["content"])
	})

	t.Run("H6 Content with permitted depth-1 nested indicator", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h6-viewer"), nil, nil, nil)
		authorID := fixture.seedUser(t, "active", nil, uniqueUsername("h6-author"), nil, nil, nil)
		targetID := fixture.seedContent(t, authorID, "h6-target", contentEntity.VisibilityPublic)
		contentID := fixture.seedContent(t, authorID, "h6-primary", contentEntity.VisibilityPublic)
		fixture.setContentShareReference(t, contentID, targetID, contentEntity.ShareTargetTypeContent)
		roomID := fixture.seedRoom(t, viewerID, authorID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, authorID, strPtr("content"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		proj := requireProjectionIsLive(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeContent), contentID)
		requireContentNestedResource(t, proj, string(chatEntity.ResourceOccurrenceResourceTypeContent), targetID)
	})

	t.Run("H7 Content nested target inaccessible", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h7-viewer"), nil, nil, nil)
		authorID := fixture.seedUser(t, "active", nil, uniqueUsername("h7-author"), nil, nil, nil)
		nestedAuthorID := fixture.seedUser(t, "active", nil, uniqueUsername("h7-nested-author"), nil, nil, nil)
		targetID := fixture.seedContent(t, nestedAuthorID, "h7-target", contentEntity.VisibilityPrivate)
		contentID := fixture.seedContent(t, authorID, "h7-primary", contentEntity.VisibilityPublic)
		fixture.setContentShareReference(t, contentID, targetID, contentEntity.ShareTargetTypeContent)
		roomID := fixture.seedRoom(t, viewerID, authorID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, authorID, strPtr("content"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		proj := requireProjectionIsLive(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeContent), contentID)
		requireNoContentNestedResource(t, proj)
	})

	t.Run("H8 FPS LIVE active", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h8-viewer"), nil, nil, nil)
		sellerID := fixture.seedActiveSeller(t, uniqueUsername("h8-seller"), "H8 Store")
		saleID := fixture.seedSale(t, sellerID, "h8-sale")
		roomID := fixture.seedRoom(t, viewerID, sellerID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, sellerID, strPtr("sale"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, saleID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		proj := requireProjectionIsLive(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeForSale), saleID)
		fps := requireFPSPayload(t, proj)
		require.NotNil(t, fps["price"])
	})

	t.Run("H9 public terminal canonically-viewable FPS", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h9-viewer"), nil, nil, nil)
		sellerID := fixture.seedActiveSeller(t, uniqueUsername("h9-seller"), "H9 Store")
		saleID := fixture.seedSale(t, sellerID, "h9-sale")
		fixture.setForSaleStatus(t, saleID, fpsEntity.ForSaleStatusSold)
		roomID := fixture.seedRoom(t, viewerID, sellerID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, sellerID, strPtr("sale"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, saleID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		proj := requireProjectionIsLive(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeForSale), saleID)
		fps := requireFPSPayload(t, proj)
		require.NotNil(t, fps["price"])
	})

	t.Run("H10 FPS inaccessible", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h10-viewer"), nil, nil, nil)
		sellerID := fixture.seedActiveSeller(t, uniqueUsername("h10-seller"), "H10 Store")
		saleID := fixture.seedSale(t, sellerID, "h10-sale")
		fixture.setForSaleStatus(t, saleID, fpsEntity.ForSaleStatusDraft)
		roomID := fixture.seedRoom(t, viewerID, sellerID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, sellerID, strPtr("sale"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, saleID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		requireProjectionIsTombstone(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeForSale))
	})

	t.Run("H11 Auction LIVE active", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h11-viewer"), nil, nil, nil)
		sellerID := fixture.seedActiveSeller(t, uniqueUsername("h11-seller"), "H11 Store")
		auctionID := fixture.seedAuction(t, sellerID, "h11-auction")
		roomID := fixture.seedRoom(t, viewerID, sellerID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, sellerID, strPtr("auction"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		proj := requireProjectionIsLive(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeAuction), auctionID)
		require.NotNil(t, requireAuctionPayload(t, proj)["title"])
	})

	t.Run("H12 Auction terminal but viewable", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h12-viewer"), nil, nil, nil)
		sellerID := fixture.seedActiveSeller(t, uniqueUsername("h12-seller"), "H12 Store")
		auctionID := fixture.seedAuction(t, sellerID, "h12-auction")
		fixture.setAuctionStatus(t, auctionID, auctionEntity.StatusEnded)
		roomID := fixture.seedRoom(t, viewerID, sellerID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, sellerID, strPtr("auction"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		requireProjectionIsLive(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeAuction), auctionID)
	})

	t.Run("H13 Auction inaccessible", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h13-viewer"), nil, nil, nil)
		sellerID := fixture.seedActiveSeller(t, uniqueUsername("h13-seller"), "H13 Store")
		auctionID := fixture.seedAuction(t, sellerID, "h13-auction")
		fixture.setAuctionStatus(t, auctionID, auctionEntity.StatusDraft)
		roomID := fixture.seedRoom(t, viewerID, sellerID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, sellerID, strPtr("auction"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		requireProjectionIsTombstone(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeAuction))
	})

	t.Run("H14 mixed page normal + all four resource types", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h14-viewer"), nil, nil, nil)
		profileTargetID := fixture.seedUser(t, "active", nil, uniqueUsername("h14-profile"), nil, nil, nil)
		contentAuthorID := fixture.seedUser(t, "active", nil, uniqueUsername("h14-content-author"), nil, nil, nil)
		saleSellerID := fixture.seedActiveSeller(t, uniqueUsername("h14-sale-seller"), "H14 Store")
		auctionSellerID := fixture.seedActiveSeller(t, uniqueUsername("h14-auction-seller"), "H14 Auction Store")
		roomID := fixture.seedRoom(t, viewerID, profileTargetID)
		normalID := fixture.seedMessage(t, roomID, profileTargetID, strPtr("normal"), nil, time.Now().UTC())
		profileID := fixture.seedMessageWithOccurrence(t, roomID, profileTargetID, strPtr("profile"), nil, time.Now().UTC().Add(1*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, profileTargetID)
		contentID := fixture.seedContent(t, contentAuthorID, "h14-content", contentEntity.VisibilityPublic)
		contentMsgID := fixture.seedMessageWithOccurrence(t, roomID, contentAuthorID, strPtr("content"), nil, time.Now().UTC().Add(2*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)
		saleID := fixture.seedSale(t, saleSellerID, "h14-sale")
		saleMsgID := fixture.seedMessageWithOccurrence(t, roomID, saleSellerID, strPtr("sale"), nil, time.Now().UTC().Add(3*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, saleID)
		auctionID := fixture.seedAuction(t, auctionSellerID, "h14-auction")
		auctionMsgID := fixture.seedMessageWithOccurrence(t, roomID, auctionSellerID, strPtr("auction"), nil, time.Now().UTC().Add(4*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		got := messageByID(messageDataFromHTTPResponse(t, resp))
		requireNoProjection(t, got[normalID.String()])
		requireProjectionIsLive(t, got[profileID.String()], string(chatEntity.ResourceOccurrenceResourceTypeProfile), profileTargetID)
		requireProjectionIsLive(t, got[contentMsgID.String()], string(chatEntity.ResourceOccurrenceResourceTypeContent), contentID)
		requireProjectionIsLive(t, got[saleMsgID.String()], string(chatEntity.ResourceOccurrenceResourceTypeForSale), saleID)
		requireProjectionIsLive(t, got[auctionMsgID.String()], string(chatEntity.ResourceOccurrenceResourceTypeAuction), auctionID)
	})

	t.Run("H15 same source shared into multiple messages", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h15-viewer"), nil, nil, nil)
		authorID := fixture.seedUser(t, "active", nil, uniqueUsername("h15-author"), nil, nil, nil)
		contentID := fixture.seedContent(t, authorID, "h15-content", contentEntity.VisibilityPublic)
		roomID := fixture.seedRoom(t, viewerID, authorID)
		firstID := fixture.seedMessageWithOccurrence(t, roomID, authorID, strPtr("content"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)
		secondID := fixture.seedMessageWithOccurrence(t, roomID, authorID, strPtr("content"), nil, time.Now().UTC().Add(1*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)
		thirdID := fixture.seedMessageWithOccurrence(t, roomID, authorID, strPtr("content"), nil, time.Now().UTC().Add(2*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		got := messageByID(messageDataFromHTTPResponse(t, resp))
		requireProjectionIsLive(t, got[firstID.String()], string(chatEntity.ResourceOccurrenceResourceTypeContent), contentID)
		requireProjectionIsLive(t, got[secondID.String()], string(chatEntity.ResourceOccurrenceResourceTypeContent), contentID)
		requireProjectionIsLive(t, got[thirdID.String()], string(chatEntity.ResourceOccurrenceResourceTypeContent), contentID)
	})

	t.Run("H16 viewer A vs viewer B on same stored page", func(t *testing.T) {
		viewerA := fixture.seedUser(t, "active", nil, uniqueUsername("h16-viewer-a"), nil, nil, nil)
		viewerB := fixture.seedUser(t, "active", nil, uniqueUsername("h16-viewer-b"), nil, nil, nil)
		peerA := fixture.seedUser(t, "active", nil, uniqueUsername("h16-peer-a"), nil, nil, nil)
		peerB := fixture.seedUser(t, "active", nil, uniqueUsername("h16-peer-b"), nil, nil, nil)
		contentID := fixture.seedContent(t, viewerA, "h16-content", contentEntity.VisibilityPrivate)
		roomA := fixture.seedRoom(t, viewerA, peerA)
		roomB := fixture.seedRoom(t, viewerB, peerB)
		messageA := fixture.seedMessageWithOccurrence(t, roomA, peerA, strPtr("content"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)
		messageB := fixture.seedMessageWithOccurrence(t, roomB, peerB, strPtr("content"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)

		statusA, respA := fixture.doJSON(t, fixture.routerFor(viewerA), http.MethodGet, "/api/v1/chat/rooms/"+roomA.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, statusA)
		statusB, respB := fixture.doJSON(t, fixture.routerFor(viewerB), http.MethodGet, "/api/v1/chat/rooms/"+roomB.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, statusB)

		msgA := messageByID(messageDataFromHTTPResponse(t, respA))[messageA.String()]
		msgB := messageByID(messageDataFromHTTPResponse(t, respB))[messageB.String()]
		requireProjectionIsLive(t, msgA, string(chatEntity.ResourceOccurrenceResourceTypeContent), contentID)
		requireProjectionIsTombstone(t, msgB, string(chatEntity.ResourceOccurrenceResourceTypeContent))
	})

	t.Run("H17 aggregate child infrastructure failure", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h17-viewer"), nil, nil, nil)
		otherID := fixture.seedUser(t, "active", nil, uniqueUsername("h17-other"), nil, nil, nil)
		roomID := fixture.seedRoom(t, viewerID, otherID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, otherID, strPtr("profile"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, otherID)
		failingHandler := chatHTTP.NewHandler(
			fixture.service,
			nil,
			nil,
			nil,
			nil,
			fixture.traced,
			zap.NewNop(),
		)

		status, _ := fixture.doJSON(t, (&chatProjectionHTTPFixture{aggregateQueryProofFixture: fixture.aggregateQueryProofFixture, service: fixture.service, handler: failingHandler}).routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusInternalServerError, status)

		_ = messageID
	})

	t.Run("H18 malformed or integrity occurrence failure", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h18-viewer"), nil, nil, nil)
		otherID := fixture.seedUser(t, "active", nil, uniqueUsername("h18-other"), nil, nil, nil)
		roomID := fixture.seedRoom(t, viewerID, otherID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, otherID, strPtr("profile"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, otherID)
		malformedHandler := chatHTTP.NewHandler(
			fixture.service,
			nil,
			nil,
			nil,
			nil,
			fixture.traced,
			zap.NewNop(),
		)

		status, _ := fixture.doJSON(t, (&chatProjectionHTTPFixture{aggregateQueryProofFixture: fixture.aggregateQueryProofFixture, service: fixture.service, handler: malformedHandler}).routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusInternalServerError, status)
		_ = messageID
	})

	t.Run("H19 legacy unrelated attachment_json semantics unchanged", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h19-viewer"), nil, nil, nil)
		otherID := fixture.seedUser(t, "active", nil, uniqueUsername("h19-other"), nil, nil, nil)
		roomID := fixture.seedRoom(t, viewerID, otherID)
		attachment := json.RawMessage(`{"type":"shipping_quote","data":{"courier":"JNE","service":"REG"}}`)
		messageID := fixture.seedMessage(t, roomID, otherID, strPtr("shipping"), attachment, time.Now().UTC())

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		requireNoProjection(t, msg)
		require.NotNil(t, msg["attachment_json"])
	})

	t.Run("H20 HTTP JSON does not expose occurrence internals", func(t *testing.T) {
		viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("h20-viewer"), nil, nil, nil)
		targetID := fixture.seedUser(t, "active", nil, uniqueUsername("h20-target"), nil, nil, nil)
		roomID := fixture.seedRoom(t, viewerID, targetID)
		messageID := fixture.seedMessageWithOccurrence(t, roomID, targetID, strPtr("profile"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, targetID)

		status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		require.Equal(t, http.StatusOK, status)

		msg := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
		requireProjectionIsLive(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeProfile), targetID)
		_, hasResourceOccurrence := msg["resource_occurrence"]
		_, hasOperation := msg["operation"]
		_, hasProfileSourceID := msg["profile_source_id"]
		_, hasContentSourceID := msg["content_source_id"]
		_, hasFPSSourceID := msg["for_sale_source_id"]
		_, hasAuctionSourceID := msg["auction_source_id"]
		_, hasFallbackSnapshot := msg["fallback_snapshot"]
		require.False(t, hasResourceOccurrence)
		require.False(t, hasOperation)
		require.False(t, hasProfileSourceID)
		require.False(t, hasContentSourceID)
		require.False(t, hasFPSSourceID)
		require.False(t, hasAuctionSourceID)
		require.False(t, hasFallbackSnapshot)
	})
}

func TestChatResourceProjectionHTTPSendMessageIncludesProjection(t *testing.T) {
	fixture := newChatProjectionHTTPFixture(t)

	senderID := fixture.seedUser(t, "active", nil, uniqueUsername("send-viewer"), nil, nil, nil)
	targetID := fixture.seedUser(t, "active", nil, uniqueUsername("send-target"), nil, nil, nil)
	roomID := fixture.seedRoom(t, senderID, targetID)
	body := "profile share"

	status, resp := fixture.doJSON(
		t,
		fixture.routerFor(senderID),
		http.MethodPost,
		"/api/v1/chat/rooms/"+roomID.String()+"/messages",
		map[string]any{
			"message_type":    "text",
			"body":            body,
			"idempotency_key": uuid.NewString(),
			"resource_occurrence": map[string]any{
				"operation":     string(chatEntity.ResourceOccurrenceOperationShareToChat),
				"resource_type": string(chatEntity.ResourceOccurrenceResourceTypeProfile),
				"resource_id":   targetID.String(),
			},
		},
	)
	require.Equal(t, http.StatusOK, status)

	msg := resp["data"].(map[string]any)
	proj := requireProjectionIsLive(t, msg, string(chatEntity.ResourceOccurrenceResourceTypeProfile), targetID)
	profile := proj["profile"].(map[string]any)
	require.Equal(t, "active", profile["lifecycle"])
}

func TestChatResourceProjectionHTTPRoomListIncludesLastMessageProjection(t *testing.T) {
	fixture := newChatProjectionHTTPFixture(t)

	viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("room-viewer"), nil, nil, nil)
	otherID := fixture.seedUser(t, "active", nil, uniqueUsername("room-other"), nil, nil, nil)
	roomID := fixture.seedRoom(t, viewerID, otherID)
	profileID := fixture.seedUser(t, "active", nil, uniqueUsername("room-profile"), nil, nil, nil)
	_ = fixture.seedMessageWithOccurrence(t, roomID, otherID, strPtr("profile"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, profileID)

	status, resp := fixture.doJSON(t, fixture.routerFor(viewerID), http.MethodGet, "/api/v1/chat/rooms/", nil)
	require.Equal(t, http.StatusOK, status)

	rooms := roomDataFromHTTPResponse(t, resp)
	require.Len(t, rooms, 1)
	lastMessage := rooms[0]["last_message"].(map[string]any)
	requireProjectionIsLive(t, lastMessage, string(chatEntity.ResourceOccurrenceResourceTypeProfile), profileID)
}

func TestChatResourceProjectionHTTPQueryCounts(t *testing.T) {
	fixture := newChatProjectionHTTPFixture(t)
	viewerID := fixture.seedUser(t, "active", nil, uniqueUsername("qh-viewer"), nil, nil, nil)
	senderProfileID := fixture.seedUser(t, "active", nil, uniqueUsername("qh-profile"), nil, nil, nil)
	senderContentAuthorID := fixture.seedUser(t, "active", nil, uniqueUsername("qh-content-author"), nil, nil, nil)
	saleSellerID := fixture.seedActiveSeller(t, uniqueUsername("qh-sale-seller"), "QH Sale")
	auctionSellerID := fixture.seedActiveSeller(t, uniqueUsername("qh-auction-seller"), "QH Auction")
	roomID := fixture.seedRoom(t, viewerID, senderProfileID)
	router := fixture.routerFor(viewerID)

	q1 := func() map[string]any {
		return map[string]any{}
	}
	_ = q1

	buildNormalMessages := func(count int) {
		for i := 0; i < count; i++ {
			fixture.seedMessage(t, roomID, senderProfileID, strPtr(fmt.Sprintf("normal-%d", i)), nil, time.Now().UTC().Add(time.Duration(i)*time.Microsecond))
		}
	}

	buildProfileMessages := func(count int) uuid.UUID {
		profileTarget := fixture.seedUser(t, "active", nil, uniqueUsername("qh-profile-target"), nil, nil, nil)
		for i := 0; i < count; i++ {
			fixture.seedMessageWithOccurrence(t, roomID, senderProfileID, strPtr(fmt.Sprintf("profile-%d", i)), nil, time.Now().UTC().Add(time.Duration(i)*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, profileTarget)
		}
		return profileTarget
	}

	buildContentMessages := func(count int) uuid.UUID {
		contentID := fixture.seedContent(t, senderContentAuthorID, uniqueUsername("qh-content"), contentEntity.VisibilityPublic)
		for i := 0; i < count; i++ {
			fixture.seedMessageWithOccurrence(t, roomID, senderContentAuthorID, strPtr(fmt.Sprintf("content-%d", i)), nil, time.Now().UTC().Add(time.Duration(i)*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)
		}
		return contentID
	}

	buildSaleMessages := func(count int, status fpsEntity.ForSaleStatus) uuid.UUID {
		saleID := fixture.seedSale(t, saleSellerID, uniqueUsername("qh-sale"))
		fixture.setForSaleStatus(t, saleID, status)
		for i := 0; i < count; i++ {
			fixture.seedMessageWithOccurrence(t, roomID, saleSellerID, strPtr(fmt.Sprintf("sale-%d", i)), nil, time.Now().UTC().Add(time.Duration(i)*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, saleID)
		}
		return saleID
	}

	buildAuctionMessages := func(count int, status auctionEntity.Status) uuid.UUID {
		auctionID := fixture.seedAuction(t, auctionSellerID, uniqueUsername("qh-auction"))
		fixture.setAuctionStatus(t, auctionID, status)
		for i := 0; i < count; i++ {
			fixture.seedMessageWithOccurrence(t, roomID, auctionSellerID, strPtr(fmt.Sprintf("auction-%d", i)), nil, time.Now().UTC().Add(time.Duration(i)*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionID)
		}
		return auctionID
	}

	measure := func(name string) int64 {
		t.Helper()
		fixture.tracer.reset()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/rooms/"+roomID.String()+"/messages", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, name)
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &decoded))
		require.NotNil(t, decoded)
		count := fixture.tracer.value()
		t.Logf("%s=%d", name, count)
		return count
	}

	qh1 := measure("QH1")
	buildNormalMessages(1)
	qh1 = measure("QH1")
	fixture.appDB = fixture.appDB
	qh2 := measure("QH2")
	_ = qh1
	_ = qh2

	// Reset room state by seeding additional messages into the same room.
	profileTarget := buildProfileMessages(1)
	_ = profileTarget
	qh3 := measure("QH3")

	contentID := buildContentMessages(1)
	_ = contentID
	qh4 := measure("QH4")

	saleID := buildSaleMessages(1, fpsEntity.ForSaleStatusActive)
	_ = saleID
	qh5 := measure("QH5")

	buildSaleMessages(1, fpsEntity.ForSaleStatusSold)
	qh6 := measure("QH6")

	auctionID := buildAuctionMessages(1, auctionEntity.StatusActive)
	_ = auctionID
	qh7 := measure("QH7")

	buildAuctionMessages(1, auctionEntity.StatusEnded)
	qh8 := measure("QH8")

	_ = qh3
	_ = qh4
	_ = qh5
	_ = qh6
	_ = qh7
	_ = qh8

	// The counts below are intentionally measured on fresh rooms to avoid page
	// pollution. Each case uses a new room with the relevant mixture.
	freshMeasure := func(name string, seed func(roomID uuid.UUID), viewer uuid.UUID) int64 {
		t.Helper()
		room := fixture.seedRoom(t, viewer, fixture.seedUser(t, "active", nil, uniqueUsername(name+"-peer"), nil, nil, nil))
		seed(room)
		fixture.tracer.reset()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/rooms/"+room.String()+"/messages", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, name)
		count := fixture.tracer.value()
		t.Logf("%s=%d", name, count)
		return count
	}

	q1 = func() map[string]any { return map[string]any{} }
	_ = q1

	build1Normal := func(room uuid.UUID) {
		fixture.seedMessage(t, room, viewerID, strPtr("normal"), nil, time.Now().UTC())
	}
	build1Profile := func(room uuid.UUID) {
		target := fixture.seedUser(t, "active", nil, uniqueUsername("qh-profile-1"), nil, nil, nil)
		fixture.seedMessageWithOccurrence(t, room, viewerID, strPtr("profile"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, target)
	}
	build20Profile := func(room uuid.UUID) {
		target := fixture.seedUser(t, "active", nil, uniqueUsername("qh-profile-20"), nil, nil, nil)
		for i := 0; i < 20; i++ {
			fixture.seedMessageWithOccurrence(t, room, viewerID, strPtr(fmt.Sprintf("profile-%d", i)), nil, time.Now().UTC().Add(time.Duration(i)*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, target)
		}
	}
	build1Content := func(room uuid.UUID) {
		author := fixture.seedUser(t, "active", nil, uniqueUsername("qh-content-1"), nil, nil, nil)
		contentID := fixture.seedContent(t, author, uniqueUsername("qh-content-1"), contentEntity.VisibilityPublic)
		fixture.seedMessageWithOccurrence(t, room, viewerID, strPtr("content"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)
	}
	build20Content := func(room uuid.UUID) {
		author := fixture.seedUser(t, "active", nil, uniqueUsername("qh-content-20"), nil, nil, nil)
		contentID := fixture.seedContent(t, author, uniqueUsername("qh-content-20"), contentEntity.VisibilityPublic)
		for i := 0; i < 20; i++ {
			fixture.seedMessageWithOccurrence(t, room, viewerID, strPtr(fmt.Sprintf("content-%d", i)), nil, time.Now().UTC().Add(time.Duration(i)*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)
		}
	}
	build1Sale := func(room uuid.UUID) {
		seller := fixture.seedActiveSeller(t, uniqueUsername("qh-sale-1"), "QH Sale 1")
		saleID := fixture.seedSale(t, seller, uniqueUsername("qh-sale-1"))
		fixture.seedMessageWithOccurrence(t, room, viewerID, strPtr("sale"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, saleID)
	}
	build20Sale := func(room uuid.UUID) {
		seller := fixture.seedActiveSeller(t, uniqueUsername("qh-sale-20"), "QH Sale 20")
		saleID := fixture.seedSale(t, seller, uniqueUsername("qh-sale-20"))
		for i := 0; i < 20; i++ {
			fixture.seedMessageWithOccurrence(t, room, viewerID, strPtr(fmt.Sprintf("sale-%d", i)), nil, time.Now().UTC().Add(time.Duration(i)*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, saleID)
		}
	}
	build1Auction := func(room uuid.UUID) {
		seller := fixture.seedActiveSeller(t, uniqueUsername("qh-auction-1"), "QH Auction 1")
		auctionID := fixture.seedAuction(t, seller, uniqueUsername("qh-auction-1"))
		fixture.seedMessageWithOccurrence(t, room, viewerID, strPtr("auction"), nil, time.Now().UTC(), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionID)
	}
	build20Auction := func(room uuid.UUID) {
		seller := fixture.seedActiveSeller(t, uniqueUsername("qh-auction-20"), "QH Auction 20")
		auctionID := fixture.seedAuction(t, seller, uniqueUsername("qh-auction-20"))
		for i := 0; i < 20; i++ {
			fixture.seedMessageWithOccurrence(t, room, viewerID, strPtr(fmt.Sprintf("auction-%d", i)), nil, time.Now().UTC().Add(time.Duration(i)*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionID)
		}
	}
	buildMixedOneEach := func(room uuid.UUID) {
		build1Profile(room)
		build1Content(room)
		build1Sale(room)
		build1Auction(room)
	}
	buildMixedLarge := func(room uuid.UUID) {
		build20Profile(room)
		build20Content(room)
		build20Sale(room)
		build20Auction(room)
	}
	buildRepeatedFour := func(room uuid.UUID) {
		profileTarget := fixture.seedUser(t, "active", nil, uniqueUsername("qh-repeat-profile"), nil, nil, nil)
		contentAuthor := fixture.seedUser(t, "active", nil, uniqueUsername("qh-repeat-content"), nil, nil, nil)
		contentID := fixture.seedContent(t, contentAuthor, uniqueUsername("qh-repeat-content"), contentEntity.VisibilityPublic)
		saleSeller := fixture.seedActiveSeller(t, uniqueUsername("qh-repeat-sale"), "QH Repeat Sale")
		saleID := fixture.seedSale(t, saleSeller, uniqueUsername("qh-repeat-sale"))
		auctionSeller := fixture.seedActiveSeller(t, uniqueUsername("qh-repeat-auction"), "QH Repeat Auction")
		auctionID := fixture.seedAuction(t, auctionSeller, uniqueUsername("qh-repeat-auction"))
		for i := 0; i < 20; i++ {
			fixture.seedMessageWithOccurrence(t, room, viewerID, strPtr(fmt.Sprintf("repeat-profile-%d", i)), nil, time.Now().UTC().Add(time.Duration(i)*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeProfile, profileTarget)
			fixture.seedMessageWithOccurrence(t, room, viewerID, strPtr(fmt.Sprintf("repeat-content-%d", i)), nil, time.Now().UTC().Add(time.Duration(100+i)*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeContent, contentID)
			fixture.seedMessageWithOccurrence(t, room, viewerID, strPtr(fmt.Sprintf("repeat-sale-%d", i)), nil, time.Now().UTC().Add(time.Duration(200+i)*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeForSale, saleID)
			fixture.seedMessageWithOccurrence(t, room, viewerID, strPtr(fmt.Sprintf("repeat-auction-%d", i)), nil, time.Now().UTC().Add(time.Duration(300+i)*time.Microsecond), chatEntity.ResourceOccurrenceOperationShareToChat, chatEntity.ResourceOccurrenceResourceTypeAuction, auctionID)
		}
	}

	qh1 = freshMeasure("QH1", build1Normal, viewerID)
	qh2 = freshMeasure("QH2", build1Profile, viewerID)
	qh3 = freshMeasure("QH3", build20Profile, viewerID)
	qh4 = freshMeasure("QH4", build1Content, viewerID)
	qh5 = freshMeasure("QH5", build20Content, viewerID)
	qh6 = freshMeasure("QH6", build1Sale, viewerID)
	qh7 = freshMeasure("QH7", build20Sale, viewerID)
	qh8 = freshMeasure("QH8", build1Auction, viewerID)
	qh9 := freshMeasure("QH9", build20Auction, viewerID)
	qh10 := freshMeasure("QH10", buildMixedOneEach, viewerID)
	qh11 := freshMeasure("QH11", buildMixedLarge, viewerID)
	qh12 := freshMeasure("QH12", buildRepeatedFour, viewerID)

	require.Equal(t, qh2, qh3)
	require.Equal(t, qh4, qh5)
	require.Equal(t, qh6, qh7)
	require.Equal(t, qh8, qh9)
	require.LessOrEqual(t, qh11, qh10)
	require.Equal(t, qh10, qh12)
}

type failingResourceProjectionResolver struct {
	err error
}

func (r failingResourceProjectionResolver) ResolveResourceProjections(context.Context, uuid.UUID, map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence) (map[uuid.UUID]*chatApp.ResourceProjection, error) {
	return nil, r.err
}
