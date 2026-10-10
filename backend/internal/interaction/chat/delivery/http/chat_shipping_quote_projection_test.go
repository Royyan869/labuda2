package http

import (
	"context"
	"testing"

	"github.com/google/uuid"
	chatApp "github.com/hishumi/backend/internal/interaction/chat/application"
	chatEntity "github.com/hishumi/backend/internal/interaction/chat/entity"
)

// PROJECTION TRANSPORT CONTRACT.
//
// Layer 1B: the conversation read path must carry a viewer-scoped shipping
// quote actionability projection on shipping-quote messages, resolved in ONE
// batch call by the Shipping commerce authority. Chat must not compute quote
// lifecycle itself, and must not fetch per message.

type stubShippingQuoteProjectionResolver struct {
	called   bool
	viewer   uuid.UUID
	quoteIDs []uuid.UUID
	result   map[uuid.UUID]chatApp.ShippingQuoteProjection
	err      error
}

func (s *stubShippingQuoteProjectionResolver) ResolveShippingQuoteProjections(
	_ context.Context,
	viewerID uuid.UUID,
	quoteIDs []uuid.UUID,
) (map[uuid.UUID]chatApp.ShippingQuoteProjection, error) {
	s.called = true
	s.viewer = viewerID
	s.quoteIDs = quoteIDs
	return s.result, s.err
}

func shippingQuoteMessage(id, offerID uuid.UUID) *chatEntity.ChatMessage {
	return &chatEntity.ChatMessage{
		ID:          id,
		MessageType: chatEntity.MessageTypeShippingQuote,
		AttachmentJSON: map[string]interface{}{
			"type": "shipping_quote",
			"data": map[string]interface{}{"offer_id": offerID.String()},
		},
	}
}

func TestResolveShippingQuoteProjections_BatchAndViewerScoped(t *testing.T) {
	viewer := uuid.New()
	msgID := uuid.New()
	offerID := uuid.New()

	stub := &stubShippingQuoteProjectionResolver{
		result: map[uuid.UUID]chatApp.ShippingQuoteProjection{
			offerID: {IsCurrent: true, ViewerActionable: true},
		},
	}
	h := &Handler{shippingQuoteProjectionResolver: stub}

	got, err := h.resolveShippingQuoteProjections(
		context.Background(),
		viewer,
		[]*chatEntity.ChatMessage{shippingQuoteMessage(msgID, offerID)},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stub.called {
		t.Fatal("resolver must be called for a shipping-quote message")
	}
	if stub.viewer != viewer {
		t.Fatalf("resolver viewer = %s, want %s", stub.viewer, viewer)
	}
	if len(stub.quoteIDs) != 1 || stub.quoteIDs[0] != offerID {
		t.Fatalf("resolver quote ids = %v, want [%s]", stub.quoteIDs, offerID)
	}
	p, ok := got[msgID]
	if !ok || !p.IsCurrent || !p.ViewerActionable {
		t.Fatalf("projection not keyed on message id: %#v", got)
	}
}

func TestResolveShippingQuoteProjections_NonShippingMessageIsEmpty(t *testing.T) {
	stub := &stubShippingQuoteProjectionResolver{}
	h := &Handler{shippingQuoteProjectionResolver: stub}

	msg := &chatEntity.ChatMessage{
		ID:             uuid.New(),
		MessageType:    chatEntity.MessageTypeText,
		AttachmentJSON: map[string]interface{}{"type": "reference"},
	}
	got, err := h.resolveShippingQuoteProjections(context.Background(), uuid.New(), []*chatEntity.ChatMessage{msg})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty projection, got %#v", got)
	}
	if stub.called {
		t.Fatal("resolver must not be called when no shipping quote is present")
	}
}

func TestResolveShippingQuoteProjections_FailsClosedWhenUnwired(t *testing.T) {
	h := &Handler{} // no resolver wired
	msg := shippingQuoteMessage(uuid.New(), uuid.New())
	if _, err := h.resolveShippingQuoteProjections(context.Background(), uuid.New(), []*chatEntity.ChatMessage{msg}); err == nil {
		t.Fatal("expected fail-closed error when a shipping quote has no resolver")
	}
}

func TestShippingQuoteProjectionJSON_CanonicalShape(t *testing.T) {
	got := shippingQuoteProjectionJSON(chatApp.ShippingQuoteProjection{IsCurrent: true, ViewerActionable: false})
	if got["is_current"] != true {
		t.Fatalf("is_current = %v, want true", got["is_current"])
	}
	if got["viewer_actionable"] != false {
		t.Fatalf("viewer_actionable = %v, want false", got["viewer_actionable"])
	}
	if len(got) != 2 {
		t.Fatalf("projection must carry exactly 2 fields, got %#v", got)
	}
}
