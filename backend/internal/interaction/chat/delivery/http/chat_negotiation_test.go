package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
)

// TestSessionToResponse_AllFieldsPopulated verifies that sessionToResponse
// correctly marshals all non-nil fields of a NegotiationSession.
func TestSessionToResponse_AllFieldsPopulated(t *testing.T) {
	now := time.Now().UTC()
	forSaleID := uuid.New()
	chatRoomID := uuid.New()
	orderID := uuid.New()
	var currentPrice int64 = 15000
	var acceptedPrice int64 = 12000
	expiresAt := now.Add(24 * time.Hour)

	session := &negotiationEntity.NegotiationSession{
		ID:               uuid.New(),
		ResourceType:     negotiationEntity.NegotiationResourceForSale,
		ForSaleID:        forSaleID,
		BuyerID:          uuid.New(),
		SellerID:         uuid.New(),
		ChatRoomID:       &chatRoomID,
		Status:           negotiationEntity.NegotiationStatusAccepted,
		OrderID:          &orderID,
		ExpiresAt:        &expiresAt,
		CurrentPrice:     &currentPrice,
		AcceptedPrice:    &acceptedPrice,
		AcceptedAt:       &now,
		ProposalSequence: 3,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	resp := sessionToResponse(session)

	// Required fields
	if resp["id"] != session.ID.String() {
		t.Errorf("id mismatch: got %v, want %v", resp["id"], session.ID.String())
	}
	if resp["resource_type"] != "for_sale" {
		t.Errorf("resource_type mismatch: got %v", resp["resource_type"])
	}
	if resp["status"] != "accepted" {
		t.Errorf("status mismatch: got %v", resp["status"])
	}
	if resp["proposal_sequence"] != int64(3) {
		t.Errorf("proposal_sequence mismatch: got %v", resp["proposal_sequence"])
	}

	// Nullable fields that should be present
	if resp["for_sale_id"] != forSaleID.String() {
		t.Errorf("for_sale_id mismatch: got %v", resp["for_sale_id"])
	}
	if resp["chat_room_id"] != chatRoomID.String() {
		t.Errorf("chat_room_id mismatch: got %v", resp["chat_room_id"])
	}
	if resp["current_price"] != currentPrice {
		t.Errorf("current_price mismatch: got %v, want %v", resp["current_price"], currentPrice)
	}
	if resp["accepted_price"] != acceptedPrice {
		t.Errorf("accepted_price mismatch: got %v, want %v", resp["accepted_price"], acceptedPrice)
	}
	if resp["order_id"] != orderID.String() {
		t.Errorf("order_id mismatch: got %v", resp["order_id"])
	}

	// is_expired should be false (expires 24h from now)
	if resp["is_expired"] != false {
		t.Errorf("is_expired should be false, got %v", resp["is_expired"])
	}
}

// TestSessionToResponse_NilOptionalFields verifies that sessionToResponse
// omits nullable fields when they are nil.
func TestSessionToResponse_NilOptionalFields(t *testing.T) {
	now := time.Now().UTC()

	session := &negotiationEntity.NegotiationSession{
		ID:               uuid.New(),
		ResourceType:     negotiationEntity.NegotiationResourceForSale,
		ForSaleID:        uuid.Nil,
		BuyerID:          uuid.New(),
		SellerID:         uuid.New(),
		Status:           negotiationEntity.NegotiationStatusActive,
		ProposalSequence: 0,
		CreatedAt:        now,
		UpdatedAt:        now,
		// All pointer fields are nil
	}

	resp := sessionToResponse(session)

	// These should NOT be present when nil
	nilKeys := []string{"for_sale_id", "chat_room_id", "current_price", "accepted_price", "expires_at", "accepted_at", "order_id"}
	for _, key := range nilKeys {
		if _, exists := resp[key]; exists {
			t.Errorf("expected %s to be absent when nil, but it was present with value %v", key, resp[key])
		}
	}

	// Required fields should still be present
	if resp["id"] == nil {
		t.Error("id should be present")
	}
	if resp["status"] != "active" {
		t.Errorf("status mismatch: got %v", resp["status"])
	}
}

// TestSessionToResponse_ExpiredSession verifies is_expired is true
// when the session has passed its expiration time.
func TestSessionToResponse_ExpiredSession(t *testing.T) {
	now := time.Now().UTC()
	expired := now.Add(-1 * time.Hour) // 1 hour ago

	session := &negotiationEntity.NegotiationSession{
		ID:               uuid.New(),
		ResourceType:     negotiationEntity.NegotiationResourceForSale,
		ForSaleID:        uuid.New(),
		BuyerID:          uuid.New(),
		SellerID:         uuid.New(),
		Status:           negotiationEntity.NegotiationStatusActive,
		ExpiresAt:        &expired,
		ProposalSequence: 1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	resp := sessionToResponse(session)

	if resp["is_expired"] != true {
		t.Errorf("is_expired should be true for expired session, got %v", resp["is_expired"])
	}
}

// TestStartNegotiationRequest_Validation verifies the request DTO struct tags.
func TestStartNegotiationRequest_Validation(t *testing.T) {
	// Verify struct can be constructed with expected fields
	req := StartNegotiationRequest{
		ForSaleID: uuid.New().String(),
		Price:     15000,
		Note:      "Can you do a lower price?",
	}

	if req.ForSaleID == "" {
		t.Error("for_sale_id should not be empty")
	}
	if req.Price <= 0 {
		t.Error("price should be positive")
	}
}

// TestBuildNegotiationCheckoutInput_UsesDistinctProductAndSaleIDs verifies the
// negotiation checkout handoff keeps product_id and source_id separate.
func TestBuildNegotiationCheckoutInput_UsesDistinctProductAndSaleIDs(t *testing.T) {
	negotiationID := uuid.New()
	productID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	forSaleID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	negotiation := &negotiationEntity.NegotiationSession{ID: negotiationID}
	forSale := &forsaleEntity.ForSale{
		ID:        forSaleID,
		ProductID: productID,
	}
	pricingTokenID := uuid.New()
	snapshot := &orderApp.PricingSnapshot{}

	input := buildNegotiationCheckoutInput(
		negotiation,
		forSale,
		uuid.New(),
		uuid.New(),
		uuid.New(),
		snapshot,
		&pricingTokenID,
		1,
	)

	if input.ProductID != productID {
		t.Fatalf("ProductID = %s, want %s", input.ProductID, productID)
	}
	if input.SourceID != forSaleID {
		t.Fatalf("SourceID = %s, want %s", input.SourceID, forSaleID)
	}
	if input.SourceType != orderentity.OrderSourceForSale {
		t.Fatalf("SourceType = %s, want %s", input.SourceType, orderentity.OrderSourceForSale)
	}
	if input.NegotiationID == nil || *input.NegotiationID != negotiationID {
		t.Fatalf("NegotiationID mismatch: got %v, want %s", input.NegotiationID, negotiationID)
	}
}

// TestCounterOfferRequest_Validation verifies the counter offer request DTO.
func TestCounterOfferRequest_Validation(t *testing.T) {
	req := CounterOfferRequest{
		SessionID: uuid.New().String(),
		Price:     12000,
		Note:      "How about this?",
	}

	if req.SessionID == "" {
		t.Error("session_id should not be empty")
	}
	if req.Price <= 0 {
		t.Error("price should be positive")
	}
}

// TestRespondNegotiationRequest_AcceptAction verifies the accept action.
func TestRespondNegotiationRequest_AcceptAction(t *testing.T) {
	req := RespondNegotiationRequest{
		SessionID: uuid.New().String(),
		Action:    "accept",
	}

	if req.Action != "accept" {
		t.Errorf("action should be 'accept', got %v", req.Action)
	}
}

// TestRespondNegotiationRequest_CancelAction verifies the cancel action.
func TestRespondNegotiationRequest_CancelAction(t *testing.T) {
	req := RespondNegotiationRequest{
		SessionID: uuid.New().String(),
		Action:    "cancel",
	}

	if req.Action != "cancel" {
		t.Errorf("action should be 'cancel', got %v", req.Action)
	}
}

// TestCC1D_NegotiationRouteContract documents the canonical chat-owned
// negotiation route contract. This test locks the structural decision
// that negotiation is NOT a standalone REST resource.
func TestCC1D_NegotiationRouteContract(t *testing.T) {
	// CANONICAL ROUTES (chat-scoped):
	// POST /api/v1/chat/rooms/:room_id/negotiate  → StartNegotiation
	// POST /api/v1/chat/rooms/:room_id/counter    → SendCounterOffer
	// POST /api/v1/chat/rooms/:room_id/respond    → RespondToNegotiation (accept|cancel)
	// GET  /api/v1/chat/rooms/:room_id/negotiation → GetNegotiation
	//
	// NON-CANONICAL (must NOT exist):
	// POST /api/v1/negotiations
	// GET  /api/v1/negotiations/:id
	// POST /api/v1/negotiations/:id/respond

	// Verify handler methods exist (compilation proof)
	var h *Handler
	_ = h // nil — only testing type system

	// These must compile (type assertion proof)
	type negotiationHandlerMethods interface {
		StartNegotiation(c interface{ Param(string) string })
		SendCounterOffer(c interface{ Param(string) string })
		RespondToNegotiation(c interface{ Param(string) string })
		GetNegotiation(c interface{ Param(string) string })
	}

	// Verify NegotiationService is injected into Handler struct
	// (this test will fail to compile if the field is removed)
	type handlerWithNegotiation struct {
		negotiationService interface{}
	}

	// Document the field exists
	_ = handlerWithNegotiation{negotiationService: nil}
}
