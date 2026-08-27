package application

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// =============================================================================
// MOCKS
// =============================================================================

// capturedOutboxEvent records what InsertEvent was called with.
type capturedOutboxEvent struct {
	eventType string
	entityID  uuid.UUID
	payload   []byte
}

type mockOutboxRepo struct {
	calls []capturedOutboxEvent
}

func (m *mockOutboxRepo) InsertEvent(
	_ context.Context,
	_ db.Tx,
	eventType string,
	entityID uuid.UUID,
	payload []byte,
) error {
	m.calls = append(m.calls, capturedOutboxEvent{
		eventType: eventType,
		entityID:  entityID,
		payload:   payload,
	})
	return nil
}

// =============================================================================
// TESTS: emitActivationEvent — STATIC EVENT TYPE
// =============================================================================

func TestEmitActivationEvent_StaticEventType(t *testing.T) {
	outbox := &mockOutboxRepo{}
	svc := &SellerSubscriptionPaymentService{outboxRepo: outbox}

	subID := uuid.New()
	userID := uuid.New()
	paymentID := uuid.New()
	now := time.Now()

	subscription := &subscriptionEntity.SellerSubscription{
		ID:         subID,
		UserID:     userID,
		PaymentID:  paymentID,
		StartedAt:  now,
		ExpiresAt:  now.Add(365 * 24 * time.Hour),
		AmountPaid: money.New(500_000),
		Currency:   "IDR",
	}

	err := svc.emitActivationEvent(context.Background(), nil, subscription)
	if err != nil {
		t.Fatalf("emitActivationEvent error = %v", err)
	}

	if len(outbox.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(outbox.calls))
	}

	call := outbox.calls[0]

	// CRITICAL: event_type must be STATIC — no UUID suffix
	if call.eventType != "seller.subscription.activated" {
		t.Errorf("eventType = %q, want %q", call.eventType, "seller.subscription.activated")
	}

	// entityID must be the subscription ID (drives the idempotency key)
	if call.entityID != subID {
		t.Errorf("entityID = %s, want %s", call.entityID, subID)
	}
}

func TestEmitActivationEvent_PayloadContainsSubscriptionID(t *testing.T) {
	outbox := &mockOutboxRepo{}
	svc := &SellerSubscriptionPaymentService{outboxRepo: outbox}

	subID := uuid.New()
	userID := uuid.New()
	paymentID := uuid.New()
	now := time.Now()

	subscription := &subscriptionEntity.SellerSubscription{
		ID:         subID,
		UserID:     userID,
		PaymentID:  paymentID,
		StartedAt:  now,
		ExpiresAt:  now.Add(365 * 24 * time.Hour),
		AmountPaid: money.New(500_000),
		Currency:   "IDR",
	}

	err := svc.emitActivationEvent(context.Background(), nil, subscription)
	if err != nil {
		t.Fatalf("emitActivationEvent error = %v", err)
	}

	call := outbox.calls[0]

	// Parse payload and assert subscription_id is present and correct
	var payloadMap map[string]interface{}
	if err := json.Unmarshal(call.payload, &payloadMap); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}

	gotSubID, ok := payloadMap["subscription_id"]
	if !ok {
		t.Fatal("payload missing subscription_id")
	}
	if gotSubID != subID.String() {
		t.Errorf("subscription_id = %v, want %s", gotSubID, subID)
	}

	// Also verify other key fields
	gotUserID, ok := payloadMap["user_id"]
	if !ok {
		t.Fatal("payload missing user_id")
	}
	if gotUserID != userID.String() {
		t.Errorf("user_id = %v, want %s", gotUserID, userID)
	}

	gotPaymentID, ok := payloadMap["payment_id"]
	if !ok {
		t.Fatal("payload missing payment_id")
	}
	if gotPaymentID != paymentID.String() {
		t.Errorf("payment_id = %v, want %s", gotPaymentID, paymentID)
	}
}

func TestEmitActivationEvent_IdempotencyKeyDeterministic(t *testing.T) {
	// The idempotency key is built by InsertEvent as "eventType.entityID".
	// With static event type "seller.subscription.activated" and entityID = subscriptionID,
	// the key is "seller.subscription.activated.<subscription_id>" — unique per subscription,
	// deterministic across retries.

	outbox := &mockOutboxRepo{}
	svc := &SellerSubscriptionPaymentService{outboxRepo: outbox}

	subID := uuid.New()
	now := time.Now()

	subscription := &subscriptionEntity.SellerSubscription{
		ID:         subID,
		UserID:     uuid.New(),
		PaymentID:  uuid.New(),
		StartedAt:  now,
		ExpiresAt:  now.Add(365 * 24 * time.Hour),
		AmountPaid: money.New(500_000),
		Currency:   "IDR",
	}

	// Call twice (simulating retry)
	_ = svc.emitActivationEvent(context.Background(), nil, subscription)
	_ = svc.emitActivationEvent(context.Background(), nil, subscription)

	if len(outbox.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(outbox.calls))
	}

	// Both calls must produce the same eventType + entityID pair,
	// which means InsertEvent will derive the same idempotency key.
	expectedKey := fmt.Sprintf("seller.subscription.activated.%s", subID)

	for i, call := range outbox.calls {
		derivedKey := fmt.Sprintf("%s.%s", call.eventType, call.entityID)
		if derivedKey != expectedKey {
			t.Errorf("call[%d] derived key = %q, want %q", i, derivedKey, expectedKey)
		}
	}
}

func TestEmitActivationEvent_EventTypeNeverContainsUUID(t *testing.T) {
	outbox := &mockOutboxRepo{}
	svc := &SellerSubscriptionPaymentService{outboxRepo: outbox}

	// Run with multiple subscription IDs to ensure none leak into event_type
	for i := 0; i < 5; i++ {
		subID := uuid.New()
		now := time.Now()

		subscription := &subscriptionEntity.SellerSubscription{
			ID:         subID,
			UserID:     uuid.New(),
			PaymentID:  uuid.New(),
			StartedAt:  now,
			ExpiresAt:  now.Add(365 * 24 * time.Hour),
			AmountPaid: money.New(500_000),
			Currency:   "IDR",
		}

		_ = svc.emitActivationEvent(context.Background(), nil, subscription)
	}

	for i, call := range outbox.calls {
		if call.eventType != "seller.subscription.activated" {
			t.Errorf("call[%d] eventType = %q, want static %q", i, call.eventType, "seller.subscription.activated")
		}
	}
}
