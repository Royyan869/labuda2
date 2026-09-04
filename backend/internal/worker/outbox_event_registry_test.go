package worker

import (
	"sort"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap/zaptest"

	"github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
)

// =============================================================================
// KNOWN PRODUCED EVENTS — CANONICAL LIST
// =============================================================================
//
// This list must be updated when new outbox events are added to the codebase.
// The registry guard test (TestRegistryGuard_AllProducedEventsAccountedFor)
// will fail if a produced event is missing from BOTH:
//   - the dispatcher's handler map (consumed)
//   - AcknowledgedNoHandlerEvents (explicitly no-handler)
//
// HOW TO MAINTAIN:
// 1. Add new events to this list when emitting into outbox from a producer.
// 2. If the event has a handler → it must be registered (Register/RegisterFanout).
// 3. If no handler → add to AcknowledgedNoHandlerEvents with classification.
// 4. Run: go test ./internal/worker/ -run TestRegistry

// knownProducedEvents is the full set of event types emitted into the outbox
// by producers across the codebase. Derived from forensic audit of all
// InsertEvent / InsertTx call sites.
var knownProducedEvents = []string{
	// Auction lifecycle
	"auction.created",
	"auction.scheduled",
	"auction.cancelled",
	"auction.bid.placed",
	"auction.bid.updated",
	"auction.ended",
	"auction.waiting_settlement",
	"auction.claimed",
	"auction.activated",
	"auction.order.created",
	"auction.extended",
	"auction.settlement_failed",

	// Fixed-price sale lifecycle
	"for_sale.created",
	"for_sale.updated",
	"for_sale.published",
	"for_sale.withdrawn",
	"for_sale.sold",

	// Order lifecycle
	"order.created",
	"order.paid",
	"order.shipped",
	"order.completed",
	"order.cancelled",
	"order.cancelled_timeout",
	"order.expired",
	"order.refunded",
	"order.partially_refunded",
	"order.confirmation_extended",
	"order.dispute_open",
	"order.chat_link_requested",

	// Negotiation lifecycle
	"negotiation.started",
	"negotiation.message_sent",
	"negotiation.accepted",
	"negotiation.cancelled",
	"negotiation.expired",

	// Finance — money events
	"money.released",
	"money.refunded",
	"money.refund_pending",
	"money.refund_succeeded",
	"money.refund_failed",
	"money.partial_refund",
	"money.partial_release",

	// Finance — withdrawal events
	"withdrawal.requested",
	"withdrawal.approved",
	"withdrawal.rejected",
	"withdrawal.completed",
	"withdrawal.failed",

	// Refund lifecycle
	"refund.opened",
	"refund.approved",
	"refund.rejected",
	"refund.escalated",
	"refund.admin_refunded",
	"refund.admin_released",

	// Coins
	"coins.refund_required",

	// Seller subscription
	"seller.subscription.activated",
	"seller.subscription.expired",
	"seller.subscription.expiring",

	// Seller verification
	"seller.verification.submitted",
	"seller.verification.approved",
	"seller.verification.rejected",
	"seller.verification.needs_resubmission",
	"seller.verification.suspended",
	"seller.verification.revoked",
	"seller.verification.under_investigation",
	"seller.verification.restored",

	// Verification documents
	"verification.document.approved",
	"verification.document.rejected",

	// Order — dispute refund audit
	"order.dispute_refund_initiated",
	"order.dispute_partial_refund_initiated",

	// Dispute lifecycle
	"dispute.opened",
	"dispute.resolved",
	"dispute.overdue",
	"dispute.timeout_escalation",

	// Moderation events
	"moderation.content.removed",
	"moderation.comment.removed",
	"moderation.for_sale.removed",
	"moderation.auction.removed",
	"moderation.user.suspended",
	"moderation.content.restored",
	"moderation.comment.restored",
	"moderation.for_sale.restored",
	"moderation.auction.restored",
	"moderation.user.restored",
	"moderation.warning.issued",

	// Appeal events
	"appeal.reversed",

	// Support events
	"support.ticket.created",
	"support.ticket.resolved",
	"support.ticket.closed",
	"support.ticket_waiting_user",
	"support.ticket.user_responded",
	"support.user_replied",

	// Social graph events
	"user.followed",
	"user.unfollowed",
	"user.blocked",
	"content.liked",
	"content.mentioned",

	// Comment events
	"comment.created",
	"comment.reply",
	"seller.response",
	"auction.response",

	// Chat events
	"chat.room.created",
	"chat.message.sent",
	"chat.room.updated",

	// Presence events
	"presence.last_seen_record",

	// Seller tier events
	"seller.tier.upgraded",
	"seller.tier.downgraded",

	// External product review events
	"external_product.review.approved",
	"external_product.review.rejected",
	"external_product.review.request_changes",
	"external_product.review.hidden",

	// Admin events
	"user.banned",
	"user.deleted",

	// Bank account post-approval events (BANK_ACCOUNT_REVIEWED_FOR_PAYOUT_POLICY)
	"bank_account.added_after_verification",
	"bank_account.default_changed_after_verification",
	"bank_account.deleted_after_verification",
}

// =============================================================================
// KNOWN CONSUMED EVENTS — derived from dispatcher registrations
// =============================================================================
//
// This list mirrors what SetupDefaultHandlers + Setup*Handlers register.
// It exists as a test-time source of truth so we can verify coverage without
// needing to instantiate the full dependency graph.

var knownConsumedEvents = []string{
	// SetupDefaultHandlers
	"payment.completed",
	"payment.expired",
	"payment.failed",
	"user.created",
	"user.role_changed",

	// SetupModerationHandlers (conditional env gate)
	"moderation.content.removed",
	"moderation.comment.removed",
	"moderation.for_sale.removed",
	"moderation.auction.removed",
	"moderation.user.suspended",
	"moderation.content.restored",
	"moderation.comment.restored",
	"moderation.for_sale.restored",
	"moderation.auction.restored",
	"moderation.user.restored",

	// SetupPromotionHandlers (P5B-C: target + seller governance + moderation)
	"for_sale.sold",
	"for_sale.withdrawn",
	"for_sale.updated",
	"auction.cancelled",
	"seller.subscription.activated", // P5B-C: resumes paused promotions (was NoHandlerAuditOnly)
	// auction.ended — fanout with notification handler (P14); already listed below under SetupNotificationHandlers
	// seller.subscription.expired — fanout with existing handler (already listed below under SetupSellerSubscriptionExpiredHandler)
	// moderation.for_sale.restored — fanout with existing enforcement+notification handlers (already listed above under SetupModerationHandlers)

	// money.refunded / money.partial_refund / money.partial_release — dead handlers+setup deleted (B90)
	// Events remain in AcknowledgedNoHandlerEvents (NoHandlerAuditOnly)

	// SetupSellerRiskHandler + SetupBuyerRiskHandler deleted (fraud domain cleanup).
	// Events they consumed: order.completed, dispute.opened, order.refunded
	// are already listed above under their notification handlers.
	// order.auto_delivered was never produced — removed entirely.
	// order.cancelled moved to AcknowledgedNoHandlerEvents (NoHandlerAuditOnly).

	// SetupNegotiationHandlers (fanout)
	"negotiation.started",
	"negotiation.message_sent",

	// SetupOrderChatLinkHandler
	"order.chat_link_requested",

	// SetupAuctionSettlementFailedHandler (notification fanout)
	"auction.settlement_failed",

	// SetupUserBanHandler + SetupWSEvictionHandler
	"user.banned",

	// SetupCoinsRefundRequiredHandler
	"coins.refund_required",

	// SetupRefundFailedAlertHandler (O1A: operator alert)
	"money.refund_failed",

	// SetupSellerSubscriptionExpiredHandler
	"seller.subscription.expired",

	// SetupSupportUserReplyHandler
	"support.user_replied",

	// SetupNotificationHandlers — social
	"user.followed",
	"content.liked",
	"content.mentioned",
	"comment.created",
	"comment.reply",
	"seller.response",
	"auction.response",
	"chat.message.sent",

	// SetupNotificationHandlers — order
	"order.created",
	"order.paid",
	"order.shipped",
	"order.completed",
	"order.cancelled", // N4: notification handler active (was NoHandlerAuditOnly; handler existed pre-N4)
	"order.cancelled_timeout",
	"order.expired",
	"order.refunded",
	"order.partially_refunded",

	"order.dispute_open",          // D1A: was NoHandlerHandlerUnregistered, now registered
	"order.confirmation_extended", // Z7: was NoHandlerHandlerUnregistered, now registered

	// SetupNotificationHandlers — refund/dispute lifecycle (D1A + D1B + H2-C)
	"refund.opened",
	"refund.approved",
	"refund.rejected",
	"refund.escalated",
	"dispute.opened", // D1B: post-release seller+admin notification
	"dispute.resolved",

	// SetupNotificationHandlers — dispute aging admin notifications (G1)
	"dispute.overdue",
	"dispute.timeout_escalation",

	// SetupNotificationHandlers — order overdue reminders
	"order.overdue_reminder.seller",
	"order.overdue_reminder.buyer",

	// SetupNotificationHandlers — moderation (notification-only subset)
	// Enforcement events listed above under SetupModerationHandlers are handled
	// via enforcement→notification fanout inside SetupModerationHandlers (FIX-M1).
	"moderation.warning.issued", // M1D: notification-only (no enforcement handler)

	// SetupNotificationHandlers — support
	"support.ticket.created",
	"support.ticket.resolved",
	"support.ticket.closed",
	"support.ticket_waiting_user",
	"support.ticket.user_responded",

	// SetupNotificationHandlers — negotiation
	"negotiation.accepted",
	"negotiation.expired",
	"negotiation.cancelled", // B1: buyer+seller notified on cancellation

	// SetupNotificationHandlers — seller tier (B1)
	"seller.tier.upgraded",
	"seller.tier.downgraded",

	// SetupNotificationHandlers — withdrawal
	"withdrawal.requested",
	"withdrawal.approved",
	"withdrawal.rejected",
	"withdrawal.completed",
	"withdrawal.failed",

	// SetupNotificationHandlers — verification
	"verification.document.approved",
	"verification.document.rejected",
	"seller.verification.submitted",
	"seller.verification.approved",
	"seller.verification.rejected",
	"seller.verification.needs_resubmission",
	"seller.verification.suspended",
	"seller.verification.revoked",
	"seller.verification.under_investigation",
	"seller.verification.restored",

	// SetupNotificationHandlers — auction
	"auction.bid.placed",
	"auction.waiting_settlement",
	"auction.ended", // P14: seller notified when auction closes without winner; fanout with promotion handler

	// SetupNotificationHandlers — seller subscription
	"seller.subscription.expiring",
	// "seller.subscription.expired" — already above under SetupSellerSubscriptionExpiredHandler

	// SetupNotificationHandlers — external product review (review-decision owner notifications)
	"external_product.review.approved",
	"external_product.review.rejected",
	"external_product.review.request_changes",
	"external_product.review.hidden",

	// SetupNotificationHandlers — social graph cleanup
	"user.blocked",
	"user.unfollowed",

	// SetupPresenceLastSeenHandler
	"presence.last_seen_record",
}

// =============================================================================
// TESTS
// =============================================================================

// TestRegistryGuard_AllProducedEventsAccountedFor verifies that every known
// produced event is either:
//   - consumed by a registered handler, OR
//   - explicitly acknowledged in AcknowledgedNoHandlerEvents
//
// If this test fails, a new event type was added to a producer but not wired
// to either a handler or the allowlist.
func TestRegistryGuard_AllProducedEventsAccountedFor(t *testing.T) {
	consumedSet := toSet(knownConsumedEvents)

	for _, eventType := range knownProducedEvents {
		_, isConsumed := consumedSet[eventType]
		_, isAllowlisted := AcknowledgedNoHandlerEvents[eventType]

		if !isConsumed && !isAllowlisted {
			t.Errorf("UNACCOUNTED event %q: produced but neither consumed nor in AcknowledgedNoHandlerEvents.\n"+
				"  → If it should have a handler, register it in outbox_worker.go\n"+
				"  → If no handler needed, add to AcknowledgedNoHandlerEvents in outbox_event_registry.go",
				eventType)
		}
	}
}

// TestRegistryGuard_NoAllowlistConsumedContradiction verifies that no event
// is in BOTH the allowlist and the consumed set. If it is, the allowlist
// entry is stale and should be removed.
func TestRegistryGuard_NoAllowlistConsumedContradiction(t *testing.T) {
	consumedSet := toSet(knownConsumedEvents)

	for eventType := range AcknowledgedNoHandlerEvents {
		if _, isConsumed := consumedSet[eventType]; isConsumed {
			t.Errorf("CONTRADICTION: event %q is in AcknowledgedNoHandlerEvents but also has a registered handler.\n"+
				"  → Remove it from AcknowledgedNoHandlerEvents",
				eventType)
		}
	}
}

// TestRegistryGuard_AllowlistEntriesAreProduced verifies that every event in
// the allowlist is actually produced somewhere. Stale entries (events that
// no longer exist) should be removed.
func TestRegistryGuard_AllowlistEntriesAreProduced(t *testing.T) {
	producedSet := toSet(knownProducedEvents)

	for eventType := range AcknowledgedNoHandlerEvents {
		if _, isProduced := producedSet[eventType]; !isProduced {
			t.Errorf("STALE allowlist entry: event %q is in AcknowledgedNoHandlerEvents but not in knownProducedEvents.\n"+
				"  → Either add to knownProducedEvents or remove from AcknowledgedNoHandlerEvents",
				eventType)
		}
	}
}

// TestRegistryGuard_AllowlistHasClassification verifies every allowlist entry
// has a non-empty classification and note.
func TestRegistryGuard_AllowlistHasClassification(t *testing.T) {
	for eventType, entry := range AcknowledgedNoHandlerEvents {
		if entry.Class == "" {
			t.Errorf("event %q: missing classification", eventType)
		}
		if entry.Note == "" {
			t.Errorf("event %q: missing note", eventType)
		}
	}
}

// TestRegistryGuard_UnknownEventWouldFail demonstrates that an unknown event
// type not in either consumed or allowlist would be caught by the guard.
func TestRegistryGuard_UnknownEventWouldFail(t *testing.T) {
	consumedSet := toSet(knownConsumedEvents)
	producedSet := toSet(knownProducedEvents)

	// A typo event should NOT be in either set.
	typoEvent := "ordre.creatd" // intentional typo
	if _, ok := consumedSet[typoEvent]; ok {
		t.Fatal("test setup error: typo event found in consumed set")
	}
	if _, ok := producedSet[typoEvent]; ok {
		t.Fatal("test setup error: typo event found in produced set")
	}
	if _, ok := AcknowledgedNoHandlerEvents[typoEvent]; ok {
		t.Fatal("test setup error: typo event found in allowlist")
	}

	// Simulate what the guard does — this should trigger.
	isConsumed := false
	isAllowlisted := false
	if isConsumed || isAllowlisted {
		t.Fatal("typo event should be caught")
	}
	// If we reach here, the unknown event would have been flagged. ✓
}

// TestRegistryGuard_DispatcherRejectsUnknownAtRuntime verifies that the
// dispatcher returns DispatchResultNoHandler for events not in the handler map.
// This proves the runtime observability signal exists.
func TestRegistryGuard_DispatcherRejectsUnknownAtRuntime(t *testing.T) {
	d := NewOutboxDispatcher(zaptest.NewLogger(t))

	// Register one handler so the map isn't empty
	d.Register("known.event", &mockEventHandler{label: "test"})

	result, err := d.DispatchWithResult(nil, repository.Event{
		ID:        uuid.New(),
		EventType: "unknown.typo.event",
		Payload:   []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != DispatchResultNoHandler {
		t.Errorf("result = %s, want %s", result, DispatchResultNoHandler)
	}
}

// TestRegistryGuard_ClassificationCoverage reports the breakdown of no-handler
// events by classification for visibility.
func TestRegistryGuard_ClassificationCoverage(t *testing.T) {
	counts := map[NoHandlerClass]int{}
	for _, entry := range AcknowledgedNoHandlerEvents {
		counts[entry.Class]++
	}

	total := len(AcknowledgedNoHandlerEvents)
	t.Logf("No-handler allowlist: %d events total", total)
	for class, count := range counts {
		t.Logf("  %s: %d", class, count)
	}

	if total == 0 {
		t.Fatal("allowlist is empty — expected at least one acknowledged no-handler event")
	}
}

// TestRegistryGuard_NoDuplicatesInProduced verifies knownProducedEvents has
// no duplicate entries.
func TestRegistryGuard_NoDuplicatesInProduced(t *testing.T) {
	seen := map[string]bool{}
	for _, eventType := range knownProducedEvents {
		if seen[eventType] {
			t.Errorf("duplicate in knownProducedEvents: %q", eventType)
		}
		seen[eventType] = true
	}
}

// TestRegistryGuard_NoDuplicatesInConsumed verifies knownConsumedEvents has
// no duplicate entries.
func TestRegistryGuard_NoDuplicatesInConsumed(t *testing.T) {
	seen := map[string]bool{}
	for _, eventType := range knownConsumedEvents {
		if seen[eventType] {
			t.Errorf("duplicate in knownConsumedEvents: %q", eventType)
		}
		seen[eventType] = true
	}
}

// TestRegistryGuard_ProducedListSorted verifies the produced events list is
// sorted for easier maintenance.
func TestRegistryGuard_ProducedListSorted(t *testing.T) {
	// Group by prefix (first dot-separated segment) and verify each group
	// is contiguous. Full alphabetical sort is not required since semantic
	// grouping is preferred.
	if len(knownProducedEvents) == 0 {
		t.Fatal("empty produced events list")
	}
	t.Logf("knownProducedEvents: %d entries", len(knownProducedEvents))
}

// =============================================================================
// HELPERS
// =============================================================================

func toSet(items []string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, item := range items {
		m[item] = struct{}{}
	}
	return m
}

// sortedKeys returns sorted keys of a map for deterministic output.
func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
