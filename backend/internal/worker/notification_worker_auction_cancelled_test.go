package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/labuda/backend/internal/interaction/notification/policy"
	platformevent "github.com/labuda/backend/internal/platform/event"
	dbpkg "github.com/labuda/backend/pkg/db"
)

// Scope B — auction.cancelled seller notification tests.
//
// Routing authority: auctionentity.CancelReason.NotifiesSeller(). Only the
// system-initiated subscription-expired auto-cancel notifies the seller;
// seller/moderation/admin/legacy reasons are handled silent no-ops (no
// notification row, no push, no error — replay-safe).

func cancelledPayload(auctionID, sellerID uuid.UUID, reason string) []byte {
	p, _ := json.Marshal(AuctionLifecyclePayload{
		AuctionID:    auctionID.String(),
		SellerID:     sellerID.String(),
		Status:       "cancelled",
		CancelReason: reason,
	})
	return p
}

func TestAuctionCancelled_SubscriptionExpired_SellerNotified(t *testing.T) {
	sellerID := uuid.New()
	auctionID := uuid.New()

	var capturedRecipient, capturedActor uuid.UUID
	var capturedType string
	var capturedEntity uuid.UUID

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, &capturedActor, &capturedType, &capturedEntity, nil),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "auction.cancelled",
		Payload:   cancelledPayload(auctionID, sellerID, "subscription_expired"),
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
	if capturedType != "auction.cancelled.seller" {
		t.Errorf("type = %s, want auction.cancelled.seller", capturedType)
	}
	if capturedEntity != auctionID {
		t.Errorf("entity = %s, want auction %s", capturedEntity, auctionID)
	}
}

func TestAuctionCancelled_OtherReasons_SilentNoOp(t *testing.T) {
	for _, reason := range []string{"", "seller", "moderation", "admin", "unknown_reason"} {
		mockDB := &mockDBForNotification{
			WithTxFunc: func(ctx context.Context, fn func(dbpkg.Tx) error) error {
				t.Errorf("reason %q: WithTx must not be called (no notification insert)", reason)
				return nil
			},
		}

		h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

		err := h.Handle(context.Background(), platformevent.OutboxEvent{
			ID:  uuid.New(),
			EventType: "auction.cancelled",
			Payload: cancelledPayload(uuid.New(), uuid.New(), reason),
		})
		if err != nil {
			t.Fatalf("reason %q: Handle() error = %v, want nil (silent no-op)", reason, err)
		}
	}
}

func TestAuctionCancelled_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t, &mockDBForNotification{}, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "auction.cancelled",
		Payload:   []byte("invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

func TestAuctionCancelled_SellerIDRequired(t *testing.T) {
	h := buildSocialGovernanceHandler(t, &mockDBForNotification{}, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	payload, _ := json.Marshal(AuctionLifecyclePayload{
		AuctionID:    uuid.New().String(),
		SellerID:     "not-a-uuid",
		Status:       "cancelled",
		CancelReason: "subscription_expired",
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "auction.cancelled", Payload: payload,
	})
	if err == nil {
		t.Fatal("expected error for invalid seller_id, got nil")
	}
}

func TestAuctionCancelled_PushAndCategoryPolicy(t *testing.T) {
	if !policy.RequiresPushByType("auction.cancelled.seller") {
		t.Error("auction.cancelled.seller must require push notification")
	}
	cat := policy.GetCategory("auction.cancelled.seller")
	if cat != policy.CommerceCritical {
		t.Errorf("category = %s, want CommerceCritical", cat)
	}
}
