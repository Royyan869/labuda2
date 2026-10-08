//go:build integration

package serverboot

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	shippingQuoteApp "github.com/labuda/backend/internal/commerce/shipping/quote/application"
	shippingQuoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	shippingQuoteRepo "github.com/labuda/backend/internal/commerce/shipping/quote/infrastructure/repository"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// LAYER 1B.1 — F-02 END-TO-END PROOF.
//
// DB → persisted shipping quote → shipping_quote chat message → Chat HTTP
// ListMessages → shipping_quote_projection → viewer-scoped JSON.
//
// One persisted quote + one persisted shipping_quote message, read by the
// BUYER and the SELLER through the REAL HTTP path with the REAL Shipping
// commerce authority (ShippingQuoteService.ProjectForViewer + GetByIDs). No
// stub resolver.
//
// Proves:
//   - buyer sees is_current=true, viewer_actionable=true;
//   - seller sees is_current=true, viewer_actionable=false;
//   - same quote / same message / same room for both reads;
//   - the actionability values are read-time projection, NOT persisted in
//     attachment_json.
func TestShippingQuoteProjection_ViewerScoped_Integration(t *testing.T) {
	fixture := newChatProjectionHTTPFixture(t)

	// REAL Shipping commerce authority (service + serverboot resolver adapter).
	realRepo := shippingQuoteRepo.NewShippingQuoteRepository()
	svc := shippingQuoteApp.NewService(
		fixture.traced,
		realRepo,
		nil, // roomGetter — not used by ProjectForViewer
		nil, // forSaleRepo
		nil, // auctionRepo
		nil, // chatService
		nil, // orderRepo
		zap.NewNop(),
	)
	fixture.handler.SetShippingQuoteProjectionResolver(newShippingQuoteProjectionResolver(svc))

	buyer := fixture.seedUser(t, "active", nil, uniqueUsername("sq-buyer"), nil, nil, nil)
	seller := fixture.seedActiveSeller(t, uniqueUsername("sq-seller"), "Toko SQ Projection")
	room := fixture.seedRoom(t, buyer, seller)

	// Real persisted quote: ACTIVE, unexpired, current, unsuperseded.
	quote := shippingQuoteEntity.NewShippingQuote(
		room,
		uuid.New(),
		"for_sale",
		uuid.New(),
		seller,
		buyer,
		money.New(15000),
		nil,
		nil,
		nil,
		time.Now().Add(24*time.Hour),
	)
	err := fixture.traced.WithTx(context.Background(), func(tx db.Tx) error {
		return realRepo.Create(context.Background(), tx, quote)
	})
	require.NoError(t, err)

	// Persist a chat message of type shipping_quote carrying the real offer_id.
	attachment := map[string]interface{}{
		"type": "shipping_quote",
		"data": map[string]interface{}{"offer_id": quote.ID.String()},
	}
	body := "Penawaran ongkir"
	messageID := insertShippingQuoteMessage(t, fixture, room, seller, &body, attachment)

	path := "/api/v1/chat/rooms/" + room.String() + "/messages"

	// ---- BUYER read ----
	status, buyerResp := fixture.doJSON(t, fixture.routerFor(buyer), http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, status)
	buyerMsg := requireMessageInResponse(t, buyerResp, messageID)
	buyerProj := requireShippingProjection(t, buyerMsg)
	require.Equal(t, true, buyerProj["is_current"])
	require.Equal(t, true, buyerProj["viewer_actionable"])

	// ---- SELLER read (same quote, same message, same room) ----
	status, sellerResp := fixture.doJSON(t, fixture.routerFor(seller), http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, status)
	sellerMsg := requireMessageInResponse(t, sellerResp, messageID)
	sellerProj := requireShippingProjection(t, sellerMsg)
	require.Equal(t, true, sellerProj["is_current"])
	require.Equal(t, false, sellerProj["viewer_actionable"])

	// ---- SAME identity across both reads ----
	require.Equal(t, buyerMsg["id"], sellerMsg["id"])
	require.Equal(t, quote.ID.String(), attachmentOfferID(t, buyerMsg))
	require.Equal(t, quote.ID.String(), attachmentOfferID(t, sellerMsg))

	// ---- Read-time projection, not persisted data ----
	for _, m := range []map[string]any{buyerMsg, sellerMsg} {
		att, ok := m["attachment_json"].(map[string]any)
		require.True(t, ok)
		_, hasIsCurrent := att["is_current"]
		_, hasViewerActionable := att["viewer_actionable"]
		require.False(t, hasIsCurrent, "is_current must not be persisted in attachment_json")
		require.False(t, hasViewerActionable, "viewer_actionable must not be persisted in attachment_json")
	}
}

// insertShippingQuoteMessage persists a chat message of type `shipping_quote`
// with the given attachment (the production seedMessage helper hardcodes
// message_type='text', so we insert directly to keep the type faithful).
func insertShippingQuoteMessage(
	t *testing.T,
	f *chatProjectionHTTPFixture,
	roomID, senderID uuid.UUID,
	body *string,
	attachment map[string]interface{},
) uuid.UUID {
	t.Helper()

	messageID := uuid.New()
	idempotencyKey := uuid.NewString()
	fingerprint := chatEntity.ComputeCommandFingerprint(
		senderID,
		chatEntity.MessageTypeShippingQuote,
		body,
		attachment,
		nil,
	)
	rawAttachment, err := json.Marshal(attachment)
	require.NoError(t, err)

	_, err = f.appDB.Pool().Exec(context.Background(), `
		INSERT INTO chat_messages (
			id, room_id, sender_id, message_type, body, attachment_json,
			idempotency_key, command_fingerprint, created_at
		)
		VALUES ($1, $2, $3, 'shipping_quote', $4, $5, $6, $7, $8)
	`, messageID, roomID, senderID, body, rawAttachment, idempotencyKey, fingerprint, time.Now().UTC())
	require.NoError(t, err)
	return messageID
}

func requireMessageInResponse(
	t *testing.T,
	resp map[string]any,
	messageID uuid.UUID,
) map[string]any {
	t.Helper()

	msg, ok := messageByID(messageDataFromHTTPResponse(t, resp))[messageID.String()]
	require.True(t, ok, "message %s must be present in the ListMessages response", messageID)
	return msg
}

func requireShippingProjection(t *testing.T, msg map[string]any) map[string]any {
	t.Helper()

	raw, ok := msg["shipping_quote_projection"]
	require.True(t, ok, "shipping_quote_projection must be present on the shipping quote message")
	proj, ok := raw.(map[string]any)
	require.True(t, ok)
	return proj
}

func attachmentOfferID(t *testing.T, msg map[string]any) string {
	t.Helper()

	att, ok := msg["attachment_json"].(map[string]any)
	require.True(t, ok)
	data, ok := att["data"].(map[string]any)
	require.True(t, ok)
	offer, _ := data["offer_id"].(string)
	return offer
}
