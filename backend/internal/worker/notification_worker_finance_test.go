package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	platformevent "github.com/labuda/backend/internal/platform/event"
	dbpkg "github.com/labuda/backend/pkg/db"
)

func TestWithdrawalRequested_SellerNotified_NoLister(t *testing.T) {
	// Without capabilityLister, only seller gets notified (backward compat).
	withdrawalID := uuid.New()
	sellerID := uuid.New()

	payload, _ := json.Marshal(WithdrawalPayload{
		WithdrawalID: withdrawalID.String(),
		SellerID:     sellerID.String(),
		Amount:       100000,
	})

	var capturedRecipient uuid.UUID
	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, nil, nil, nil, nil),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	// No SetCapabilityLister — backward compat path.

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "withdrawal.requested", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if capturedRecipient != sellerID {
		t.Errorf("recipient = %s, want seller %s", capturedRecipient, sellerID)
	}
}

func TestWithdrawalRequested_SellerAndAdminFanout(t *testing.T) {
	withdrawalID := uuid.New()
	sellerID := uuid.New()
	admin1 := uuid.New()
	admin2 := uuid.New()

	payload, _ := json.Marshal(WithdrawalPayload{
		WithdrawalID: withdrawalID.String(),
		SellerID:     sellerID.String(),
		Amount:       250000,
	})

	var recipients []uuid.UUID
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 6 {
						recipients = append(recipients, args[1].(uuid.UUID))
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.withdraw.review": {admin1, admin2},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "withdrawal.requested", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// 1 seller + 2 admins = 3 inserts
	if len(recipients) != 3 {
		t.Fatalf("insert count = %d, want 3 (1 seller + 2 admins)", len(recipients))
	}
	if recipients[0] != sellerID {
		t.Errorf("first recipient = %s, want seller %s", recipients[0], sellerID)
	}
	adminSet := map[uuid.UUID]bool{admin1: true, admin2: true}
	for _, r := range recipients[1:] {
		if !adminSet[r] {
			t.Errorf("unexpected admin recipient %s", r)
		}
	}
}

func TestWithdrawalRequested_ZeroReviewers_NoError(t *testing.T) {
	withdrawalID := uuid.New()
	sellerID := uuid.New()

	payload, _ := json.Marshal(WithdrawalPayload{
		WithdrawalID: withdrawalID.String(),
		SellerID:     sellerID.String(),
		Amount:       50000,
	})

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			dbCalls++
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{}, // no reviewers
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "withdrawal.requested", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// Only seller notification — no admin inserts.
	if dbCalls != 1 {
		t.Errorf("WithTx called %d times, want 1 (seller only)", dbCalls)
	}
}

func TestWithdrawalRequested_TotalAdminFanoutFailure_ReturnsError(t *testing.T) {
	withdrawalID := uuid.New()
	sellerID := uuid.New()
	admin1 := uuid.New()

	payload, _ := json.Marshal(WithdrawalPayload{
		WithdrawalID: withdrawalID.String(),
		SellerID:     sellerID.String(),
		Amount:       100000,
	})

	callCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			callCount++
			if callCount == 1 {
				// Seller insert succeeds
				tx := &mockTxForNotification{
					QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
						return &mockRowForNotification{scanValue: uuid.New()}
					},
				}
				return fn(tx)
			}
			// Admin insert fails
			return fmt.Errorf("db connection lost")
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.withdraw.review": {admin1},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "withdrawal.requested", Payload: payload,
	})
	if err == nil {
		t.Fatal("expected error for total admin fanout failure, got nil")
	}
	if !strings.Contains(err.Error(), "all 1 admin fanout inserts failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWithdrawalRequested_PartialAdminFanoutFailure_Succeeds(t *testing.T) {
	withdrawalID := uuid.New()
	sellerID := uuid.New()
	admin1 := uuid.New()
	admin2 := uuid.New()

	payload, _ := json.Marshal(WithdrawalPayload{
		WithdrawalID: withdrawalID.String(),
		SellerID:     sellerID.String(),
		Amount:       75000,
	})

	callCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			callCount++
			if callCount == 2 {
				// First admin insert fails
				return fmt.Errorf("transient error")
			}
			// Seller + second admin succeed
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.withdraw.review": {admin1, admin2},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "withdrawal.requested", Payload: payload,
	})
	if err != nil {
		t.Fatalf("partial failure should not return error, got: %v", err)
	}
	// 3 WithTx calls: 1 seller + 2 admin attempts (1 failed, 1 succeeded)
	if callCount != 3 {
		t.Errorf("WithTx calls = %d, want 3", callCount)
	}
}

func TestWithdrawalRequested_ReplayIdempotent(t *testing.T) {
	withdrawalID := uuid.New()
	sellerID := uuid.New()
	admin1 := uuid.New()

	payload, _ := json.Marshal(WithdrawalPayload{
		WithdrawalID: withdrawalID.String(),
		SellerID:     sellerID.String(),
		Amount:       30000,
	})

	insertCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					insertCount++
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"finance.withdraw.review": {admin1},
		},
	})

	event := platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "withdrawal.requested", Payload: payload,
	}

	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("replay Handle() error = %v", err)
	}

	// 2 deliveries × (1 seller + 1 admin) = 4 inserts. DB dedup handles actual duplicates.
	if insertCount != 4 {
		t.Errorf("insertCount = %d, want 4", insertCount)
	}
}

func TestWithdrawalRequested_CapabilityLookupError_ReturnsError(t *testing.T) {
	withdrawalID := uuid.New()
	sellerID := uuid.New()

	payload, _ := json.Marshal(WithdrawalPayload{
		WithdrawalID: withdrawalID.String(),
		SellerID:     sellerID.String(),
		Amount:       100000,
	})

	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		err: fmt.Errorf("connection refused"),
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "withdrawal.requested", Payload: payload,
	})
	if err == nil {
		t.Fatal("expected error for capability lookup failure, got nil")
	}
	if !strings.Contains(err.Error(), "list admins by capability") {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// SELLER.VERIFICATION.SUBMITTED ADMIN FANOUT
// =============================================================================

func TestSellerVerificationSubmitted_SellerNotified_NoLister(t *testing.T) {
	// Without capabilityLister, only seller gets notified (backward compat).
	sellerID := uuid.New()

	payload, _ := json.Marshal(SellerVerificationPayload{
		SellerID: sellerID.String(),
		Status:   "pending_review",
	})

	var capturedRecipient uuid.UUID
	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, nil, nil, nil, nil),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	// No SetCapabilityLister — backward compat path.

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.submitted", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if capturedRecipient != sellerID {
		t.Errorf("recipient = %s, want seller %s", capturedRecipient, sellerID)
	}
}

func TestSellerVerificationSubmitted_SellerAndAdminFanout(t *testing.T) {
	sellerID := uuid.New()
	admin1 := uuid.New()
	admin2 := uuid.New()
	admin3 := uuid.New()

	payload, _ := json.Marshal(SellerVerificationPayload{
		SellerID: sellerID.String(),
		Status:   "pending_review",
	})

	var recipients []uuid.UUID
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, args ...any) pgx.Row {
					if len(args) >= 6 {
						recipients = append(recipients, args[1].(uuid.UUID))
					}
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"seller.verification.review": {admin1, admin2, admin3},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.submitted", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// 1 seller + 3 admins = 4 inserts
	if len(recipients) != 4 {
		t.Fatalf("insert count = %d, want 4 (1 seller + 3 admins)", len(recipients))
	}
	if recipients[0] != sellerID {
		t.Errorf("first recipient = %s, want seller %s", recipients[0], sellerID)
	}
	adminSet := map[uuid.UUID]bool{admin1: true, admin2: true, admin3: true}
	for _, r := range recipients[1:] {
		if !adminSet[r] {
			t.Errorf("unexpected admin recipient %s", r)
		}
	}
}

func TestSellerVerificationSubmitted_ZeroReviewers_NoError(t *testing.T) {
	sellerID := uuid.New()

	payload, _ := json.Marshal(SellerVerificationPayload{
		SellerID: sellerID.String(),
		Status:   "pending_review",
	})

	dbCalls := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			dbCalls++
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{}, // no reviewers
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.submitted", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	// Only seller notification — no admin inserts.
	if dbCalls != 1 {
		t.Errorf("WithTx called %d times, want 1 (seller only)", dbCalls)
	}
}

func TestSellerVerificationSubmitted_CapabilityLookupError_ReturnsError(t *testing.T) {
	sellerID := uuid.New()

	payload, _ := json.Marshal(SellerVerificationPayload{
		SellerID: sellerID.String(),
		Status:   "pending_review",
	})

	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		err: fmt.Errorf("connection refused"),
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.submitted", Payload: payload,
	})
	if err == nil {
		t.Fatal("expected error for capability lookup failure, got nil")
	}
	if !strings.Contains(err.Error(), "list admins by capability") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSellerVerificationSubmitted_TotalAdminFanoutFailure_ReturnsError(t *testing.T) {
	sellerID := uuid.New()
	admin1 := uuid.New()

	payload, _ := json.Marshal(SellerVerificationPayload{
		SellerID: sellerID.String(),
		Status:   "pending_review",
	})

	callCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			callCount++
			if callCount == 1 {
				// Seller insert succeeds
				tx := &mockTxForNotification{
					QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
						return &mockRowForNotification{scanValue: uuid.New()}
					},
				}
				return fn(tx)
			}
			// Admin insert fails
			return fmt.Errorf("db connection lost")
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"seller.verification.review": {admin1},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.submitted", Payload: payload,
	})
	if err == nil {
		t.Fatal("expected error for total admin fanout failure, got nil")
	}
	if !strings.Contains(err.Error(), "all 1 admin fanout inserts failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSellerVerificationSubmitted_PartialAdminFanoutFailure_Succeeds(t *testing.T) {
	sellerID := uuid.New()
	admin1 := uuid.New()
	admin2 := uuid.New()

	payload, _ := json.Marshal(SellerVerificationPayload{
		SellerID: sellerID.String(),
		Status:   "pending_review",
	})

	callCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			callCount++
			if callCount == 2 {
				// First admin insert fails
				return fmt.Errorf("transient error")
			}
			// Seller + second admin succeed
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"seller.verification.review": {admin1, admin2},
		},
	})

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.submitted", Payload: payload,
	})
	if err != nil {
		t.Fatalf("partial failure should not return error, got: %v", err)
	}
	// 3 WithTx calls: 1 seller + 2 admin attempts (1 failed, 1 succeeded)
	if callCount != 3 {
		t.Errorf("WithTx calls = %d, want 3", callCount)
	}
}

func TestSellerVerificationSubmitted_ReplayIdempotent(t *testing.T) {
	sellerID := uuid.New()
	admin1 := uuid.New()

	payload, _ := json.Marshal(SellerVerificationPayload{
		SellerID: sellerID.String(),
		Status:   "pending_review",
	})

	insertCount := 0
	mockDB := &mockDBForNotification{
		WithTxFunc: func(_ context.Context, fn func(dbpkg.Tx) error) error {
			tx := &mockTxForNotification{
				QueryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
					insertCount++
					return &mockRowForNotification{scanValue: uuid.New()}
				},
			}
			return fn(tx)
		},
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)
	h.SetCapabilityLister(&mockCapabilityLister{
		users: map[string][]uuid.UUID{
			"seller.verification.review": {admin1},
		},
	})

	event := platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.submitted", Payload: payload,
	}

	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("replay Handle() error = %v", err)
	}

	// 2 deliveries × (1 seller + 1 admin) = 4 inserts. DB dedup handles actual duplicates.
	if insertCount != 4 {
		t.Errorf("insertCount = %d, want 4", insertCount)
	}
}

// =============================================================================
// ORDER.DISPUTE_OPEN ADMIN FANOUT
// =============================================================================

func makeDisputeOpenPayload(orderID, buyerID, sellerID uuid.UUID) []byte {
	b, _ := json.Marshal(OrderPayload{
		OrderID:  orderID.String(),
		BuyerID:  buyerID.String(),
		SellerID: sellerID.String(),
		Status:   "dispute_open",
	})
	return b
}

func TestSellerSubscriptionExpiringLegacy_UserNotified(t *testing.T) {
	userID := uuid.New()
	subscriptionID := uuid.New()

	payload, _ := json.Marshal(SellerSubscriptionExpiringPayload{
		SubscriptionID:  subscriptionID.String(),
		UserID:          userID.String(),
		ExpiresAt:       "2026-05-18T00:00:00Z",
		DaysUntilExpiry: 7,
	})

	var capturedRecipient, capturedActor, capturedEntity uuid.UUID
	var capturedType string
	var capturedData map[string]interface{}

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, &capturedActor, &capturedType, &capturedEntity, &capturedData),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.subscription.expiring", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if capturedRecipient != userID {
		t.Errorf("recipient = %s, want %s", capturedRecipient, userID)
	}
	if capturedActor != uuid.Nil {
		t.Errorf("actor = %s, want uuid.Nil (system-initiated)", capturedActor)
	}
	if capturedType != "seller.subscription.expiring" {
		t.Errorf("type = %s, want seller.subscription.expiring", capturedType)
	}
	if capturedEntity != subscriptionID {
		t.Errorf("entityID = %s, want %s", capturedEntity, subscriptionID)
	}
	if capturedData["subscriptionId"] != subscriptionID.String() {
		t.Errorf("data.subscriptionId = %v, want %s", capturedData["subscriptionId"], subscriptionID.String())
	}
	if capturedData["daysUntilExpiry"] != 7 {
		t.Errorf("data.daysUntilExpiry = %v, want 7", capturedData["daysUntilExpiry"])
	}
}

func TestSellerSubscriptionExpiringLegacy_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.subscription.expiring", Payload: []byte("invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

func TestSellerSubscriptionExpiringLegacy_InvalidUserID(t *testing.T) {
	payload, _ := json.Marshal(SellerSubscriptionExpiringPayload{
		SubscriptionID:  uuid.New().String(),
		UserID:          "not-a-uuid",
		ExpiresAt:       "2026-05-18T00:00:00Z",
		DaysUntilExpiry: 7,
	})

	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.subscription.expiring", Payload: payload,
	})
	if err == nil {
		t.Fatal("expected error for invalid user_id, got nil")
	}
}

// =============================================================================
// SV1D: seller.verification.suspended / revoked / under_investigation / restored
// =============================================================================

func TestSellerVerificationSuspended_UserNotified(t *testing.T) {
	sellerID := uuid.New()

	payload, _ := json.Marshal(SellerVerificationPayload{
		SellerID: sellerID.String(),
		Status:   "suspended",
		Reason:   "Suspicious activity detected",
	})

	var capturedRecipient, capturedActor, capturedEntity uuid.UUID
	var capturedType string
	var capturedData map[string]interface{}

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, &capturedActor, &capturedType, &capturedEntity, &capturedData),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.suspended", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if capturedRecipient != sellerID {
		t.Errorf("recipient = %s, want %s", capturedRecipient, sellerID)
	}
	if capturedActor != uuid.Nil {
		t.Errorf("actor = %s, want uuid.Nil (system-initiated)", capturedActor)
	}
	if capturedType != "seller.verification.suspended" {
		t.Errorf("type = %s, want seller.verification.suspended", capturedType)
	}
	if capturedData["reason"] != "Suspicious activity detected" {
		t.Errorf("data.reason = %v, want 'Suspicious activity detected'", capturedData["reason"])
	}
}

func TestSellerVerificationSuspended_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.suspended", Payload: []byte("invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

func TestSellerVerificationRevoked_UserNotified(t *testing.T) {
	sellerID := uuid.New()

	payload, _ := json.Marshal(SellerVerificationPayload{
		SellerID: sellerID.String(),
		Status:   "revoked",
		Reason:   "Fraud confirmed",
	})

	var capturedRecipient, capturedActor, capturedEntity uuid.UUID
	var capturedType string
	var capturedData map[string]interface{}

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, &capturedActor, &capturedType, &capturedEntity, &capturedData),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.revoked", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if capturedRecipient != sellerID {
		t.Errorf("recipient = %s, want %s", capturedRecipient, sellerID)
	}
	if capturedType != "seller.verification.revoked" {
		t.Errorf("type = %s, want seller.verification.revoked", capturedType)
	}
	if capturedData["reason"] != "Fraud confirmed" {
		t.Errorf("data.reason = %v, want 'Fraud confirmed'", capturedData["reason"])
	}
}

func TestSellerVerificationRevoked_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.revoked", Payload: []byte("invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

func TestSellerVerificationUnderInvestigation_UserNotified(t *testing.T) {
	sellerID := uuid.New()

	payload, _ := json.Marshal(SellerVerificationPayload{
		SellerID: sellerID.String(),
		Status:   "under_investigation",
		Reason:   "Multiple complaints received",
	})

	var capturedRecipient, capturedActor, capturedEntity uuid.UUID
	var capturedType string
	var capturedData map[string]interface{}

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, &capturedActor, &capturedType, &capturedEntity, &capturedData),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.under_investigation", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if capturedRecipient != sellerID {
		t.Errorf("recipient = %s, want %s", capturedRecipient, sellerID)
	}
	if capturedType != "seller.verification.under_investigation" {
		t.Errorf("type = %s, want seller.verification.under_investigation", capturedType)
	}
	if capturedData["reason"] != "Multiple complaints received" {
		t.Errorf("data.reason = %v, want 'Multiple complaints received'", capturedData["reason"])
	}
}

func TestSellerVerificationUnderInvestigation_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.under_investigation", Payload: []byte("invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

func TestSellerVerificationRestored_UserNotified(t *testing.T) {
	sellerID := uuid.New()

	payload, _ := json.Marshal(SellerVerificationPayload{
		SellerID: sellerID.String(),
		Status:   "approved",
	})

	var capturedRecipient, capturedActor, capturedEntity uuid.UUID
	var capturedType string
	var capturedData map[string]interface{}

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, &capturedActor, &capturedType, &capturedEntity, &capturedData),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.restored", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if capturedRecipient != sellerID {
		t.Errorf("recipient = %s, want %s", capturedRecipient, sellerID)
	}
	if capturedType != "seller.verification.restored" {
		t.Errorf("type = %s, want seller.verification.restored", capturedType)
	}
	if capturedData["sellerId"] != sellerID.String() {
		t.Errorf("data.sellerId = %v, want %s", capturedData["sellerId"], sellerID.String())
	}
	// Restored should NOT have reason in data
	if _, hasReason := capturedData["reason"]; hasReason {
		t.Errorf("data.reason should not be present for restored event, got %v", capturedData["reason"])
	}
}

func TestSellerVerificationRestored_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.verification.restored", Payload: []byte("invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

// =============================================================================
// SV1B: seller.subscription.expiring NOTIFICATION TESTS
// =============================================================================

func TestSellerSubscriptionExpiring_UserNotified(t *testing.T) {
	userID := uuid.New()
	subscriptionID := uuid.New()

	payload, _ := json.Marshal(SellerSubscriptionExpiringPayload{
		SubscriptionID:  subscriptionID.String(),
		UserID:          userID.String(),
		ExpiresAt:       "2026-06-18T00:00:00Z",
		DaysUntilExpiry: 30,
	})

	var capturedRecipient, capturedActor, capturedEntity uuid.UUID
	var capturedType string
	var capturedData map[string]interface{}

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, &capturedActor, &capturedType, &capturedEntity, &capturedData),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.subscription.expiring", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if capturedRecipient != userID {
		t.Errorf("recipient = %s, want %s", capturedRecipient, userID)
	}
	if capturedActor != uuid.Nil {
		t.Errorf("actor = %s, want uuid.Nil (system-initiated)", capturedActor)
	}
	if capturedType != "seller.subscription.expiring" {
		t.Errorf("type = %s, want seller.subscription.expiring", capturedType)
	}
	if capturedEntity != subscriptionID {
		t.Errorf("entityID = %s, want %s", capturedEntity, subscriptionID)
	}
	if capturedData["subscriptionId"] != subscriptionID.String() {
		t.Errorf("data.subscriptionId = %v, want %s", capturedData["subscriptionId"], subscriptionID.String())
	}
	if capturedData["expiresAt"] != "2026-06-18T00:00:00Z" {
		t.Errorf("data.expiresAt = %v, want 2026-06-18T00:00:00Z", capturedData["expiresAt"])
	}
	// daysUntilExpiry is int in payload but gets deserialized as float64 by JSON
	if capturedData["daysUntilExpiry"] != 30 {
		t.Errorf("data.daysUntilExpiry = %v, want 30", capturedData["daysUntilExpiry"])
	}
}

func TestSellerSubscriptionExpiring_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.subscription.expiring", Payload: []byte("invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

// =============================================================================
// SV1B: seller.subscription.expired NOTIFICATION TESTS
// =============================================================================

func TestSellerSubscriptionExpired_UserNotified(t *testing.T) {
	userID := uuid.New()
	subscriptionID := uuid.New()

	payload, _ := json.Marshal(SellerSubscriptionExpiredPayload{
		SubscriptionID:  subscriptionID.String(),
		UserID:          userID.String(),
	})

	var capturedRecipient, capturedActor, capturedEntity uuid.UUID
	var capturedType string
	var capturedData map[string]interface{}

	mockDB := &mockDBForNotification{
		WithTxFunc: insertCaptureTx(&capturedRecipient, &capturedActor, &capturedType, &capturedEntity, &capturedData),
	}

	h := buildSocialGovernanceHandler(t, mockDB, &mockAccountStatusControlled{}, &mockBlockCheckerControlled{}, nil)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.subscription.expired", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if capturedRecipient != userID {
		t.Errorf("recipient = %s, want %s", capturedRecipient, userID)
	}
	if capturedActor != uuid.Nil {
		t.Errorf("actor = %s, want uuid.Nil (system-initiated)", capturedActor)
	}
	if capturedType != "seller.subscription.expired" {
		t.Errorf("type = %s, want seller.subscription.expired", capturedType)
	}
	if capturedEntity != subscriptionID {
		t.Errorf("entityID = %s, want %s", capturedEntity, subscriptionID)
	}
	if capturedData["subscriptionId"] != subscriptionID.String() {
		t.Errorf("data.subscriptionId = %v, want %s", capturedData["subscriptionId"], subscriptionID.String())
	}
}

func TestSellerSubscriptionExpired_InvalidPayload(t *testing.T) {
	h := buildSocialGovernanceHandler(t,
		&mockDBForNotification{},
		&mockAccountStatusControlled{},
		&mockBlockCheckerControlled{},
		nil,
	)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID: uuid.New(), EventType: "seller.subscription.expired", Payload: []byte("invalid json"),
	})
	if err == nil {
		t.Fatal("expected error for invalid payload, got nil")
	}
}

// =============================================================================
// H2-C: REFUND DECISION NOTIFICATION TESTS
// =============================================================================

// TestNotificationEventHandler_HandleRefundApproved tests handling refund.approved events.
// Seller approves refund → buyer gets notified.
func TestN4A2_WithdrawalRequested_WrapperAllowPushLog(t *testing.T) {
	withdrawalID, sellerID := uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1) // single push via Handle() dispatch goroutine
	logger := &mockDeliveryCapture{}
	logger.wg.Add(2) // in_app "sent" + push "sent" (both logged via async goroutines)

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "withdrawal.requested",
		Payload:   makeWithdrawalPayloadN4(withdrawalID, sellerID),
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
	if rec.notifType != "withdrawal.requested" {
		t.Errorf("notifType = %q, want %q", rec.notifType, "withdrawal.requested")
	}
	if push.pushCount() != 1 {
		t.Errorf("push count = %d, want 1", push.pushCount())
	}
	if status := logger.inAppStatus(); status != "sent" {
		t.Errorf("in_app delivery status = %q, want %q", status, "sent")
	}
}

// TestN4A2_WithdrawalCompleted_WrapperAllowPushLog proves:
//   - withdrawal.completed uses insertNotificationWithPolicy
//   - allowPush=true
//   - delivery log written
func TestN4A2_WithdrawalCompleted_WrapperAllowPushLog(t *testing.T) {
	withdrawalID, sellerID := uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1)
	logger := &mockDeliveryCapture{}
	logger.wg.Add(2) // in_app "sent" + push "sent"

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "withdrawal.completed",
		Payload:   makeWithdrawalPayloadN4(withdrawalID, sellerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()
	logger.wg.Wait()

	if db.count() != 1 {
		t.Errorf("DB inserts = %d, want 1", db.count())
	}
	if db.at(0).recipient != sellerID {
		t.Errorf("recipient = %v, want sellerID %v", db.at(0).recipient, sellerID)
	}
	if push.pushCount() != 1 {
		t.Errorf("push count = %d, want 1", push.pushCount())
	}
	if status := logger.inAppStatus(); status != "sent" {
		t.Errorf("in_app delivery status = %q, want %q", status, "sent")
	}
}

// TestN4A2_WithdrawalFailed_WrapperAllowPushLog proves:
//   - withdrawal.failed uses insertNotificationWithPolicy
//   - allowPush=true
//   - delivery log written
//   - push copy matches the failed payout wording
func TestN4A2_WithdrawalFailed_WrapperAllowPushLog(t *testing.T) {
	withdrawalID, sellerID := uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &capturePushSender{}
	push.wg.Add(1)
	logger := &mockDeliveryCapture{}
	logger.wg.Add(2) // in_app "sent" + push "sent"

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "withdrawal.failed",
		Payload:   makeWithdrawalPayloadN4(withdrawalID, sellerID),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()
	logger.wg.Wait()

	if db.count() != 1 {
		t.Errorf("DB inserts = %d, want 1", db.count())
	}
	if db.at(0).recipient != sellerID {
		t.Errorf("recipient = %v, want sellerID %v", db.at(0).recipient, sellerID)
	}
	if push.pushCount() != 1 {
		t.Errorf("push count = %d, want 1", push.pushCount())
	}
	if got := push.lastTitle(); got != "Penarikan Gagal" {
		t.Errorf("push title = %q, want %q", got, "Penarikan Gagal")
	}
	if got := push.lastBody(); got != "Penarikan dana Anda gagal diproses. Dana telah dikembalikan ke saldo Anda" {
		t.Errorf("push body = %q, want failed payout copy", got)
	}
	if status := logger.inAppStatus(); status != "sent" {
		t.Errorf("in_app delivery status = %q, want %q", status, "sent")
	}
}

// TestN4A2_VerificationDocumentApproved_WrapperAllowPushLog proves:
//   - verification.document.approved uses insertNotificationWithPolicy
//   - userID is the recipient, uuid.Nil is actor
//   - allowPush=true and delivery log written
func TestN4A2_VerificationDocumentApproved_WrapperAllowPushLog(t *testing.T) {
	documentID, userID := uuid.New(), uuid.New()
	db := &multiInsertDB{}
	push := &pushCountSender{}
	push.wg.Add(1)
	logger := &mockDeliveryCapture{}
	logger.wg.Add(2) // in_app "sent" + push "sent"

	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "verification.document.approved",
		Payload:   makeVerificationDocPayloadN4(documentID, userID, "ktp"),
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
	if rec.recipient != userID {
		t.Errorf("recipient = %v, want userID %v", rec.recipient, userID)
	}
	if rec.actor != uuid.Nil {
		t.Errorf("actor = %v, want uuid.Nil (admin-initiated)", rec.actor)
	}
	if rec.notifType != "verification.document.approved" {
		t.Errorf("notifType = %q, want %q", rec.notifType, "verification.document.approved")
	}
	if push.pushCount() != 1 {
		t.Errorf("push count = %d, want 1", push.pushCount())
	}
	if status := logger.inAppStatus(); status != "sent" {
		t.Errorf("in_app delivery status = %q, want %q", status, "sent")
	}
}

// TestN4A2_SellerVerificationApproved_WrapperAllowPushLog proves:
//   - seller.verification.approved uses insertNotificationWithPolicy
//   - caller-provided title/body override getTitleAndBody default ("Notification")
//   - allowPush=true and delivery log written
func TestN4A2_SellerVerificationApproved_WrapperAllowPushLog(t *testing.T) {
	sellerID := uuid.New()
	db := &multiInsertDB{}
	push := &capturePushSender{}
	push.wg.Add(1)
	logger := &mockDeliveryCapture{}
	logger.wg.Add(2) // in_app "sent" + push "sent"

	// capturePushSender also satisfies PushSender
	h := buildN4Handler(t, db, &mockBlockCheckerForNotification{}, push, logger)

	err := h.Handle(context.Background(), platformevent.OutboxEvent{
		ID:        uuid.New(),
		EventType: "seller.verification.approved",
		Payload:   makeSellerVerificationPayloadN4(sellerID, "approved"),
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	push.wg.Wait()
	logger.wg.Wait()

	if db.count() != 1 {
		t.Errorf("DB inserts = %d, want 1", db.count())
	}
	if db.at(0).recipient != sellerID {
		t.Errorf("recipient = %v, want sellerID %v", db.at(0).recipient, sellerID)
	}
	// Verify the caller-provided override, not the generic getTitleAndBody fallback.
	const wantTitle = "Verifikasi Disetujui"
	if got := push.lastTitle(); got != wantTitle {
		t.Errorf("push title = %q, want %q (override applied)", got, wantTitle)
	}
	if status := logger.inAppStatus(); status != "sent" {
		t.Errorf("in_app delivery status = %q, want %q", status, "sent")
	}
}

// =============================================================================
// N4-A3: NEGOTIATION + SUPPORT + MODERATION HANDLER MIGRATION TESTS
// =============================================================================

func makeNegotiationPayloadN4(sessionID, sellerID, buyerID uuid.UUID) []byte {
	b, _ := json.Marshal(NegotiationPayload{
		SessionID: sessionID.String(),
		SellerID:  sellerID.String(),
		BuyerID:   buyerID.String(),
	})
	return b
}

func makeSupportTicketPayloadN4(ticketID, userID uuid.UUID) []byte {
	b, _ := json.Marshal(SupportTicketPayload{
		TicketID: ticketID.String(),
		UserID:   userID.String(),
		Status:   "closed",
	})
	return b
}

func makeModerationRemovedPayloadN4(resourceID uuid.UUID) []byte {
	b, _ := json.Marshal(ModerationRemovedPayload{
		ResourceID:   resourceID.String(),
		ResourceType: "post",
	})
	return b
}
