package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/labuda/backend/internal/interaction/notification/policy"
	platformevent "github.com/labuda/backend/internal/platform/event"
	dbpkg "github.com/labuda/backend/pkg/db"
)

func TestAuctionBidPlaced_SellerNotified(t *testing.T) {
	bidderID := uuid.New()
	sellerID := uuid.New()
	auctionID := uuid.New()
	bidID := uuid.New()

	payload, _ := json.Marshal(AuctionBidPayload{
		BidID:     bidID.String(),
		AuctionID: auctionID.String(),
		BidderID:  bidderID.String(),
		Amount:    150000,
	})

	callCount := 0
	var capturedRecipient, capturedActor uuid.UUID
	var capturedType string

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			callCount++
			if callCount == 1 {
				// First call: SELECT seller_id FROM auctions WHERE id = $1
				return fn(&mockTxForNotification{
					QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
						return &mockRowForNotification{scanValue: sellerID}
					},
				})
			}
			// Second call: INSERT notification
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 4 {
						capturedRecipient = args[1].(uuid.UUID)
						capturedActor = args[2].(uuid.UUID)
						capturedType = args[3].(string)
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "auction.bid.placed", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if callCount != 2 {
		t.Errorf("WithTx called %d times, want 2 (SELECT seller + INSERT notif)", callCount)
	}
	if capturedRecipient != sellerID {
		t.Errorf("recipient = %s, want seller %s", capturedRecipient, sellerID)
	}
	if capturedActor != bidderID {
		t.Errorf("actor = %s, want bidder %s", capturedActor, bidderID)
	}
	if capturedType != "auction.bid.placed" {
		t.Errorf("type = %s, want auction.bid.placed", capturedType)
	}
}

func TestAuctionBidPlaced_SelfBid_NoNotification(t *testing.T) {
	sellerID := uuid.New() // seller bids on own auction
	auctionID := uuid.New()

	payload, _ := json.Marshal(AuctionBidPayload{
		BidID:     uuid.New().String(),
		AuctionID: auctionID.String(),
		BidderID:  sellerID.String(),
		Amount:    100000,
	})

	callCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			callCount++
			// SELECT seller_id FROM auctions — return same ID as bidder
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: sellerID}
				},
			})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "auction.bid.placed", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// Only SELECT call, no INSERT.
	if callCount != 1 {
		t.Errorf("WithTx called %d times, want 1 (SELECT only, self-bid skips INSERT)", callCount)
	}
}

func TestAuctionBidPlaced_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "auction.bid.placed", Payload: []byte("invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

func TestAuctionBidPlaced_AuctionNotFound(t *testing.T) {
	payload, _ := json.Marshal(AuctionBidPayload{
		BidID:     uuid.New().String(),
		AuctionID: uuid.New().String(),
		BidderID:  uuid.New().String(),
		Amount:    50000,
	})

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{err: pgx.ErrNoRows}
				},
			})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "auction.bid.placed", Payload: payload,
	})
	if err == nil {
		t.Fatal("expected error when auction not found, got nil")
	}
}

func TestAuctionSettlementFailed_BuyerViolation_ReturnsErrorOnBuyerInsertFail(t *testing.T) {
	auctionID := uuid.New()
	winnerID := uuid.New()
	sellerID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"auction_id":       auctionID.String(),
		"violated_user_id": winnerID.String(),
		"violation_type":   "buyer_shipping_timeout",
		"seller_id":        sellerID.String(),
		"winner_id":        winnerID.String(),
		"violation_id":     uuid.New().String(),
		"restricted_until": time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
	})

	call := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(tx dbpkg.Tx) error) error {
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					call++
					if call == 1 {
						return &mockRowForNotification{err: errors.New("buyer insert failed")}
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "auction.settlement_failed", Payload: payload,
	})
	if err == nil {
		t.Fatal("expected error for buyer insert failure, got nil")
	}
	if !strings.Contains(err.Error(), "buyer insert failed") {
		t.Errorf("error = %v, want buyer insert failure", err)
	}
}

func TestAuctionSettlementFailed_BuyerViolation_NotifiesWinnerAndSeller(t *testing.T) {
	auctionID := uuid.New()
	winnerID := uuid.New()
	sellerID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{
		"auction_id":       auctionID.String(),
		"violated_user_id": winnerID.String(),
		"violation_type":   "buyer_shipping_timeout",
		"seller_id":        sellerID.String(),
		"winner_id":        winnerID.String(),
		"violation_id":     uuid.New().String(),
		"restricted_until": time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
	})

	type captured struct {
		recipient uuid.UUID
		nType     string
	}
	var inserts []captured

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 4 {
						inserts = append(inserts, captured{
							recipient: args[1].(uuid.UUID),
							nType:     args[3].(string),
						})
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "auction.settlement_failed", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(inserts) != 2 {
		t.Fatalf("expected 2 notifications (winner + seller), got %d", len(inserts))
	}
	if inserts[0].recipient != winnerID || inserts[0].nType != "auction.settlement_failed.buyer" {
		t.Errorf("insert[0] = %+v, want buyer notification to winner", inserts[0])
	}
	if inserts[1].recipient != sellerID || inserts[1].nType != "auction.settlement_failed.relistable" {
		t.Errorf("insert[1] = %+v, want relistable notification to seller", inserts[1])
	}
}

// =============================================================================
// AUCTION WAITING SETTLEMENT — Winner notification
// =============================================================================

func TestAuctionWaitingSettlement_WinnerAndSellerNotified(t *testing.T) {
	// P14: auction.waiting_settlement now notifies BOTH winner and seller.
	winnerID := uuid.New()
	sellerID := uuid.New()
	auctionID := uuid.New()

	winnerStr := winnerID.String()
	payload, _ := json.Marshal(AuctionLifecyclePayload{
		AuctionID:     auctionID.String(),
		SellerID:      sellerID.String(),
		Status:        "waiting_settlement",
		CurrentWinner: &winnerStr,
	})

	type captured struct {
		recipient uuid.UUID
		actor     uuid.UUID
		nType     string
	}
	var inserts []captured

	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			return fn(&mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 4 {
						inserts = append(inserts, captured{
							recipient: args[1].(uuid.UUID),
							actor:     args[2].(uuid.UUID),
							nType:     args[3].(string),
						})
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "auction.waiting_settlement", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(inserts) != 2 {
		t.Fatalf("expected 2 notifications (winner + seller), got %d", len(inserts))
	}

	// First insert: winner notification
	if inserts[0].recipient != winnerID {
		t.Errorf("insert[0] recipient = %s, want winner %s", inserts[0].recipient, winnerID)
	}
	if inserts[0].actor != sellerID {
		t.Errorf("insert[0] actor = %s, want seller %s", inserts[0].actor, sellerID)
	}
	if inserts[0].nType != "auction.waiting_settlement" {
		t.Errorf("insert[0] type = %s, want auction.waiting_settlement", inserts[0].nType)
	}

	// Second insert: seller notification
	if inserts[1].recipient != sellerID {
		t.Errorf("insert[1] recipient = %s, want seller %s", inserts[1].recipient, sellerID)
	}
	if inserts[1].actor != winnerID {
		t.Errorf("insert[1] actor = %s, want winner %s", inserts[1].actor, winnerID)
	}
	if inserts[1].nType != "auction.seller_has_winner" {
		t.Errorf("insert[1] type = %s, want auction.seller_has_winner", inserts[1].nType)
	}
}

func TestAuctionWaitingSettlement_NoWinner_Skipped(t *testing.T) {
	payload, _ := json.Marshal(AuctionLifecyclePayload{
		AuctionID: uuid.New().String(),
		SellerID:  uuid.New().String(),
		Status:    "waiting_settlement",
		// CurrentWinner is nil — no winner to notify
	})

	callCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
			callCount++
			return fn(&mockTxForNotification{})
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "auction.waiting_settlement", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// No DB calls should happen — skip silently
	if callCount != 0 {
		t.Errorf("WithTx called %d times, want 0 (no winner to notify)", callCount)
	}
}

func TestAuctionWaitingSettlement_Push(t *testing.T) {
	// Verify that auction.waiting_settlement requires push
	if !policy.RequiresPushByType("auction.waiting_settlement") {
		t.Error("auction.waiting_settlement must require push notification")
	}
	// Verify CommerceCritical category
	cat := policy.GetCategory("auction.waiting_settlement")
	if cat != policy.CommerceCritical {
		t.Errorf("category = %s, want CommerceCritical", cat)
	}
}

// =============================================================================
// P14: AUCTION SELLER HAS WINNER — Seller notification (waiting_settlement)
// =============================================================================

func TestAuctionSellerHasWinner_Push(t *testing.T) {
	if !policy.RequiresPushByType("auction.seller_has_winner") {
		t.Error("auction.seller_has_winner must require push notification")
	}
	cat := policy.GetCategory("auction.seller_has_winner")
	if cat != policy.CommerceCritical {
		t.Errorf("category = %s, want CommerceCritical", cat)
	}
}

// =============================================================================
// P14: AUCTION ENDED NO WINNER — Seller notification (ended with no bids)
// =============================================================================

func TestAuctionEndedNoWinner_SellerNotified(t *testing.T) {
	sellerID := uuid.New()
	auctionID := uuid.New()

	payload, _ := json.Marshal(AuctionLifecyclePayload{
		AuctionID: auctionID.String(),
		SellerID:  sellerID.String(),
		Status:    "ended",
		// CurrentWinner is nil — no winner
	})

	var capturedRecipient, capturedActor uuid.UUID
	var capturedType string
	var capturedEntity uuid.UUID

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, &capturedActor, &capturedType, &capturedEntity, nil),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "auction.ended", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if capturedRecipient != sellerID {
		t.Errorf("recipient = %s, want seller %s", capturedRecipient, sellerID)
	}
	if capturedActor != uuid.Nil {
		t.Errorf("actor = %s, want uuid.Nil (system-initiated)", capturedActor)
	}
	if capturedType != "auction.ended_no_winner" {
		t.Errorf("type = %s, want auction.ended_no_winner", capturedType)
	}
	if capturedEntity != auctionID {
		t.Errorf("entity = %s, want auction %s", capturedEntity, auctionID)
	}
}

func TestAuctionEndedNoWinner_Push(t *testing.T) {
	if !policy.RequiresPushByType("auction.ended_no_winner") {
		t.Error("auction.ended_no_winner must require push notification")
	}
	cat := policy.GetCategory("auction.ended_no_winner")
	if cat != policy.CommerceCritical {
		t.Errorf("category = %s, want CommerceCritical", cat)
	}
}

func TestAuctionEndedNoWinner_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t, &mockDBForNotification{}, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "auction.ended", Payload: []byte("invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

// =============================================================================
// MISSING CRITICAL NOTIFICATIONS: seller.subscription.expiring
// =============================================================================

func TestN4A3_NegotiationStarted_WrapperPushLog(t *testing.T) {
	sessionID, sellerID, buyerID := uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1) // single push via Handle() dispatch goroutine
	logger := &mockDeliveryCapture{}
	logger.wg.Add(2) // in_app "sent" + push "sent"

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "negotiation.started",
		Payload:   makeNegotiationPayloadN4(sessionID, sellerID, buyerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()
	logger.wg.Wait()

	if db.count() != 1 {
		t.Errorf("DB inserts = %d, want 1", db.count())
	}
	rec := db.at(0)
	if rec.recipient != sellerID {
		t.Errorf("recipient = %v, want sellerID %v", rec.recipient, sellerID)
	}
	if rec.actor != uuid.Nil {
		t.Errorf("actor = %v, want uuid.Nil (buyer-initiated, system delivers)", rec.actor)
	}
	if rec.notifType != "negotiation.started" {
		t.Errorf("notifType = %q, want %q", rec.notifType, "negotiation.started")
	}
	if push.pushCount() != 1 {
		t.Errorf("push count = %d, want 1", push.pushCount())
	}
	if status := logger.inAppStatus(); status != "sent" {
		t.Errorf("in_app delivery status = %q, want %q", status, "sent")
	}
}

func TestN5_NegotiationMessageSent_BuyerSends_SellerReceives(t *testing.T) {
	sessionID, buyerID, sellerID := uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1)
	logger := &mockDeliveryCapture{}
	logger.wg.Add(2) // in_app "sent" + push "sent"

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "negotiation.message_sent",
		Payload:   makeNegotiationMessageSentPayloadN5(sessionID, buyerID, sellerID, buyerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()
	logger.wg.Wait()

	if db.count() != 1 {
		t.Errorf("DB inserts = %d, want 1", db.count())
	}
	rec := db.at(0)
	if rec.recipient != sellerID {
		t.Errorf("recipient = %v, want sellerID %v (buyer sent → seller notified)", rec.recipient, sellerID)
	}
	if rec.recipient == buyerID {
		t.Error("sender (buyerID) must not receive own notification")
	}
	if push.pushCount() != 1 {
		t.Errorf("push count = %d, want 1", push.pushCount())
	}
	if status := logger.inAppStatus(); status != "sent" {
		t.Errorf("in_app delivery status = %q, want %q", status, "sent")
	}
}

// TestN5_NegotiationMessageSent_SellerSends_BuyerReceives proves:
//   - When seller is the sender, buyer receives the notification
//   - This was the P0 bug: before the fix, seller always received own notification
//     because the handler did SELECT seller_id and used that as recipient unconditionally
func TestN5_NegotiationMessageSent_SellerSends_BuyerReceives(t *testing.T) {
	sessionID, buyerID, sellerID := uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1)
	logger := &mockDeliveryCapture{}
	logger.wg.Add(2) // in_app "sent" + push "sent"

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "negotiation.message_sent",
		Payload:   makeNegotiationMessageSentPayloadN5(sessionID, buyerID, sellerID, sellerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()
	logger.wg.Wait()

	if db.count() != 1 {
		t.Errorf("DB inserts = %d, want 1", db.count())
	}
	rec := db.at(0)
	if rec.recipient != buyerID {
		t.Errorf("recipient = %v, want buyerID %v (seller sent → buyer notified)", rec.recipient, buyerID)
	}
	if rec.recipient == sellerID {
		t.Error("sender (sellerID) must not receive own notification — was P0 bug before N5 fix")
	}
	if push.pushCount() != 1 {
		t.Errorf("push count = %d, want 1", push.pushCount())
	}
	if status := logger.inAppStatus(); status != "sent" {
		t.Errorf("in_app delivery status = %q, want %q", status, "sent")
	}
}

// TestN5_NegotiationMessageSent_InvalidSender_Skips proves malformed sender_id
// does not default to notifying seller.
func TestN5_NegotiationMessageSent_InvalidSender_Skips(t *testing.T) {
	sessionID, buyerID, sellerID, badSenderID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "negotiation.message_sent",
		Payload:   makeNegotiationMessageSentPayloadN5(sessionID, buyerID, sellerID, badSenderID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if db.count() != 0 {
		t.Errorf("DB inserts = %d, want 0 (invalid sender must be skipped)", db.count())
	}
	if push.pushCount() != 0 {
		t.Errorf("push count = %d, want 0", push.pushCount())
	}
}

// =============================================================================
// PASS_8A: negotiation.started/accepted/expired must carry chatRoomId so
// mobile's notification tap can deep-link to the buyer/seller's direct
// chat room (F1/F2 fix regression tests).
// =============================================================================

func makeNegotiationStartedPayloadWithRoom(sessionID, sellerID, buyerID, chatRoomID uuid.UUID) []byte {
	b, _ := json.Marshal(NegotiationPayload{
		SessionID:  sessionID.String(),
		ChatRoomID: chatRoomID.String(),
		SellerID:   sellerID.String(),
		BuyerID:    buyerID.String(),
	})
	return b
}

func makeNegotiationAcceptedPayload(sessionID, buyerID, chatRoomID uuid.UUID) []byte {
	b, _ := json.Marshal(NegotiationPayload{
		SessionID:  sessionID.String(),
		ChatRoomID: chatRoomID.String(),
		BuyerID:    buyerID.String(),
	})
	return b
}

func makeNegotiationExpiredPayload(sessionID, buyerID, chatRoomID uuid.UUID) []byte {
	b, _ := json.Marshal(NegotiationPayload{
		SessionID:  sessionID.String(),
		ChatRoomID: chatRoomID.String(),
		BuyerID:    buyerID.String(),
	})
	return b
}

// TestNegotiationStarted_CarriesChatRoomIdFromPayload proves that once the
// negotiation.started outbox payload includes chat_room_id (Pass 8A — the
// direct room is now known synchronously at StartNegotiation time, per Pass
// 7B), the notification worker forwards it directly without needing the
// DB-fallback lookup.
func TestNegotiationStarted_CarriesChatRoomIdFromPayload(t *testing.T) {
	sessionID, sellerID, buyerID, chatRoomID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1)

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "negotiation.started",
		Payload:   makeNegotiationStartedPayloadWithRoom(sessionID, sellerID, buyerID, chatRoomID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	push.wg.Wait()

	if db.count() != 1 {
		t.Fatalf("DB inserts = %d, want 1", db.count())
	}
	rec := db.at(0)
	if rec.recipient != sellerID {
		t.Errorf("recipient = %v, want sellerID %v", rec.recipient, sellerID)
	}
	if got := rec.data["chatRoomId"]; got != chatRoomID.String() {
		t.Fatalf("chatRoomId = %v, want %q (mobile cannot deep-link without this)", got, chatRoomID.String())
	}
}

// TestNegotiationAccepted_CarriesChatRoomId is the F2 regression test: the
// negotiation.accepted outbox payload already carried chat_room_id, but the
// notification handler's data map silently dropped it, leaving the buyer's
// "offer accepted" notification with no deep-link target at all.
func TestNegotiationAccepted_CarriesChatRoomId(t *testing.T) {
	sessionID, buyerID, chatRoomID := uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1)

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "negotiation.accepted",
		Payload:   makeNegotiationAcceptedPayload(sessionID, buyerID, chatRoomID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	push.wg.Wait()

	if db.count() != 1 {
		t.Fatalf("DB inserts = %d, want 1", db.count())
	}
	rec := db.at(0)
	if rec.recipient != buyerID {
		t.Errorf("recipient = %v, want buyerID %v", rec.recipient, buyerID)
	}
	if got := rec.data["chatRoomId"]; got != chatRoomID.String() {
		t.Fatalf("chatRoomId = %v, want %q — F2 regression: buyer's accepted notification had no deep-link target", got, chatRoomID.String())
	}
}

// TestNegotiationExpired_CarriesChatRoomId confirms the expired handler
// (unchanged by Pass 8A, already correct) still forwards chatRoomId.
func TestNegotiationExpired_CarriesChatRoomId(t *testing.T) {
	sessionID, buyerID, chatRoomID := uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1)

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "negotiation.expired",
		Payload:   makeNegotiationExpiredPayload(sessionID, buyerID, chatRoomID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	push.wg.Wait()

	if db.count() != 1 {
		t.Fatalf("DB inserts = %d, want 1", db.count())
	}
	rec := db.at(0)
	if got := rec.data["chatRoomId"]; got != chatRoomID.String() {
		t.Fatalf("chatRoomId = %v, want %q", got, chatRoomID.String())
	}
}

func TestB1_SellerTierUpgraded_SellerNotified(t *testing.T) {
	sellerID := uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1)
	logger := &mockDeliveryCapture{}
	logger.wg.Add(2)

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "seller.tier.upgraded",
		Payload:   makeSellerTierPayload(sellerID, "none", "pro"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()
	logger.wg.Wait()

	if db.count() != 1 {
		t.Errorf("DB inserts = %d, want 1", db.count())
	}
	rec := db.at(0)
	if rec.recipient != sellerID {
		t.Errorf("recipient = %v, want sellerID %v", rec.recipient, sellerID)
	}
	if rec.actor != uuid.Nil {
		t.Errorf("actor = %v, want uuid.Nil (system-initiated)", rec.actor)
	}
	if rec.notifType != "seller.tier.upgraded" {
		t.Errorf("notifType = %q, want %q", rec.notifType, "seller.tier.upgraded")
	}
	if push.pushCount() != 1 {
		t.Errorf("push count = %d, want 1", push.pushCount())
	}
}

// TestB1_SellerTierDowngraded_SellerNotified proves:
//   - seller.tier.downgraded notifies the seller
//   - CommerceCritical category → allowPush=true
func TestB1_SellerTierDowngraded_SellerNotified(t *testing.T) {
	sellerID := uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1)
	logger := &mockDeliveryCapture{}
	logger.wg.Add(2)

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "seller.tier.downgraded",
		Payload:   makeSellerTierPayload(sellerID, "elite", "pro"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()
	logger.wg.Wait()

	if db.count() != 1 {
		t.Errorf("DB inserts = %d, want 1", db.count())
	}
	rec := db.at(0)
	if rec.recipient != sellerID {
		t.Errorf("recipient = %v, want sellerID %v", rec.recipient, sellerID)
	}
	if rec.notifType != "seller.tier.downgraded" {
		t.Errorf("notifType = %q, want %q", rec.notifType, "seller.tier.downgraded")
	}
	if push.pushCount() != 1 {
		t.Errorf("push count = %d, want 1", push.pushCount())
	}
}

// TestB1_NegotiationCancelled_BothPartiesNotified proves:
//   - negotiation.cancelled notifies buyer (primary) AND seller (secondary goroutine)
//   - Both inserts captured
//   - negotiation.* → CommerceCritical → allowPush=true for both (2 push goroutines)
func TestB1_NegotiationCancelled_BothPartiesNotified(t *testing.T) {
	sessionID, buyerID, sellerID := uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(2) // buyer push goroutine (Handle dispatcher) + seller push goroutine (inline)

	// nil logger: dual-notify triggers 4 logDelivery calls (buyer in_app + push + seller in_app + push)
	// Verifying DB inserts + push count is sufficient here.
	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "negotiation.cancelled",
		Payload:   makeNegotiationCancelledPayload(sessionID, buyerID, sellerID, uuid.Nil),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()

	if db.count() != 2 {
		t.Errorf("DB inserts = %d, want 2 (buyer + seller)", db.count())
	}

	recipients := map[uuid.UUID]bool{}
	for i := 0; i < db.count(); i++ {
		recipients[db.at(i).recipient] = true
	}
	if !recipients[buyerID] {
		t.Errorf("buyerID not in recipients; got %v", recipients)
	}
	if !recipients[sellerID] {
		t.Errorf("sellerID not in recipients; got %v", recipients)
	}
	if push.pushCount() != 2 {
		t.Errorf("push count = %d, want 2 (one per party)", push.pushCount())
	}
}

// TestB1_NegotiationCancelled_SelfSendPrevented proves:
//   - When buyerID == sellerID, only one notification is inserted (self-send guard)
func TestB1_NegotiationCancelled_SelfSendPrevented(t *testing.T) {
	sessionID, singlePartyID := uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1)
	logger := &mockDeliveryCapture{}
	logger.wg.Add(2)

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "negotiation.cancelled",
		Payload:   makeNegotiationCancelledPayload(sessionID, singlePartyID, singlePartyID, uuid.Nil),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()
	logger.wg.Wait()

	if db.count() != 1 {
		t.Errorf("DB inserts = %d, want 1 (self-send guard prevents duplicate)", db.count())
	}
}

// TestB1_NegotiationCancelled_CarriesChatRoomID proves:
//   - notification.data includes chatRoomId so list + open-path navigation can deep-link to chat
func TestB1_NegotiationCancelled_CarriesChatRoomID(t *testing.T) {
	sessionID, buyerID, sellerID, chatRoomID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(2)

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "negotiation.cancelled",
		Payload:   makeNegotiationCancelledPayload(sessionID, buyerID, sellerID, chatRoomID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()

	if db.count() != 2 {
		t.Fatalf("DB inserts = %d, want 2", db.count())
	}

	for i := 0; i < db.count(); i++ {
		rec := db.at(i)
		if got := rec.data["chatRoomId"]; got != chatRoomID.String() {
			t.Fatalf("insert %d chatRoomId = %v, want %q", i, got, chatRoomID.String())
		}
		if got := rec.data["sessionId"]; got != sessionID.String() {
			t.Fatalf("insert %d sessionId = %v, want %q", i, got, sessionID.String())
		}
	}
}

// TestB1_SellerTierPushPolicy proves that seller.tier.* types require push
// per RequiresPushByType — this is the policy regression lock.
func TestB1_SellerTierPushPolicy(t *testing.T) {
	for _, notifType := range []string{"seller.tier.upgraded", "seller.tier.downgraded"} {
		if !policy.RequiresPushByType(notifType) {
			t.Errorf("policy.RequiresPushByType(%q) = false, want true", notifType)
		}
	}
}

// =============================================================================
// O3: PANIC RECOVERY TESTS
// =============================================================================

// panickingInserter is a NotificationInserter that panics on InsertNotification.
type panickingInserter struct{}

func (p *panickingInserter) InsertNotification(ctx context.Context, tx dbpkg.Tx, recipientID, actorID uuid.UUID, notificationType string, entityID uuid.UUID, data map[string]interface{}) (uuid.UUID, error) {
	panic("simulated inserter crash")
}

// panickingPushSender is a PushSender that panics on SendNotification.
type panickingPushSender struct{}

func (p *panickingPushSender) SendNotification(ctx context.Context, tx interface{}, notification interface{}, title, body string) error {
	panic("simulated push crash")
}
