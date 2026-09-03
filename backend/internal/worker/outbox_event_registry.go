package worker

// =============================================================================
// OUTBOX EVENT REGISTRY — NO-HANDLER ALLOWLIST
// =============================================================================
//
// This file declares all event types that are PRODUCED into the outbox but
// intentionally have NO handler registered. Any produced event not in this
// list AND not consumed by a registered handler will fail the registry guard
// test, preventing silent swallow of typo'd or forgotten event types.
//
// RUNTIME BEHAVIOUR: This file does NOT change runtime behaviour. The outbox
// dispatcher still marks no-handler events as succeeded. This is a test-time
// regression guard only.
//
// HOW TO USE:
// 1. When adding a new outbox event that intentionally has no consumer,
//    add it to AcknowledgedNoHandlerEvents with the correct classification.
// 2. When wiring a handler for a previously no-handler event, REMOVE it
//    from this list — the registry guard test will verify it's now consumed.

// NoHandlerClass classifies why an event intentionally has no handler.
type NoHandlerClass string

const (
	// NoHandlerAuditOnly — event exists for outbox audit trail / observability.
	// No domain side-effect required.
	NoHandlerAuditOnly NoHandlerClass = "audit_only"

	// NoHandlerFutureHook — handler will be wired in a future iteration.
	// Event is produced but consumer is not yet built.
	NoHandlerFutureHook NoHandlerClass = "future_hook"

	// NoHandlerHandlerUnregistered — handler CODE exists in the notification
	// worker (switch cases + dedicated functions) but the event is not included
	// in any RegisterMultiple() call. Likely oversight; low risk because the
	// handler code is ready.
	NoHandlerHandlerUnregistered NoHandlerClass = "handler_unregistered"

	// NoHandlerDisabledGate — handler exists and is wired but behind an env
	// gate that is currently off. Events still produced; handler not active.
	// These are NOT in this list because they ARE registered when the gate is on.
	// Only add here if the handler registration itself is disabled.
	NoHandlerDisabledGate NoHandlerClass = "disabled_gate"
)

// NoHandlerEntry documents a single no-handler event.
type NoHandlerEntry struct {
	Class NoHandlerClass
	Note  string // short human explanation
}

// AcknowledgedNoHandlerEvents is the canonical allowlist of event types that
// are produced into the outbox but intentionally have no registered handler.
//
// This map is consumed by outbox_event_registry_test.go. If a produced event
// is missing from BOTH this map and the dispatcher's handler map, the test
// fails — forcing explicit acknowledgement of every unhandled event.
var AcknowledgedNoHandlerEvents = map[string]NoHandlerEntry{
	// =========================================================================
	// SELLER SUBSCRIPTION
	// =========================================================================
	// P5B-C: seller.subscription.activated now consumed by SetupPromotionHandlers
	// (resumes paused promotions on subscription re-activation).

	// =========================================================================
	// FINANCE — MONEY EVENTS
	// =========================================================================
	"money.released": {
		Class: NoHandlerAuditOnly,
		Note:  "escrow release audit trail; carries gross/commission/sellerNet for reconciliation",
	},
	"money.refund_pending": {
		Class: NoHandlerAuditOnly,
		Note:  "gateway refund initiated audit trail; refund dispatched to payment gateway",
	},
	"money.refund_succeeded": {
		Class: NoHandlerAuditOnly,
		Note:  "gateway refund ack success audit trail; ledger reversal already applied in HandleGatewayRefundAck",
	},
	// O1A: money.refund_failed now has a handler (RefundFailedAlertHandler).
	// Removed from allowlist — see SetupRefundFailedAlertHandler.
	"money.refunded": {
		Class: NoHandlerAuditOnly,
		Note:  "dead handler+setup deleted (B90); coins refund at ack-time; event consumed without side-effect",
	},
	"money.partial_refund": {
		Class: NoHandlerAuditOnly,
		Note:  "dead handler+setup deleted (B90); coins refund at ack-time; event consumed without side-effect",
	},
	"money.partial_release": {
		Class: NoHandlerAuditOnly,
		Note:  "dead handler+setup deleted (B90); log-only handler was never wired; event consumed without side-effect",
	},

	// =========================================================================
	// FIXED-PRICE SALE LIFECYCLE
	// =========================================================================
	"for_sale.created": {
		Class: NoHandlerAuditOnly,
		Note:  "fixed-price sale creation audit trail",
	},
	"for_sale.published": {
		Class: NoHandlerAuditOnly,
		Note:  "fixed-price sale publication audit trail",
	},

	// =========================================================================
	// AUCTION LIFECYCLE
	// =========================================================================
	"auction.created": {
		Class: NoHandlerAuditOnly,
		Note:  "auction creation audit trail",
	},
	"auction.scheduled": {
		Class: NoHandlerAuditOnly,
		Note:  "auction scheduling audit trail",
	},
	"auction.activated": {
		Class: NoHandlerAuditOnly,
		Note:  "auction activation audit trail",
	},
	// P4C1: auction.cancelled + auction.ended moved to knownConsumedEvents —
	// promotion event handlers now ENABLED (SetupPromotionHandlers).
	"auction.claimed": {
		Class: NoHandlerAuditOnly,
		Note:  "auction claim (winner acceptance) audit trail",
	},
	"auction.bid.updated": {
		Class: NoHandlerAuditOnly,
		Note:  "bid update audit trail; auction.bid.placed drives notifications",
	},
	"auction.extended": {
		Class: NoHandlerFutureHook,
		Note:  "PASS_18C soft-close (anti-sniping) extension; notification fanout to bidders/watchers not yet built",
	},
	"auction.order.created": {
		Class: NoHandlerAuditOnly,
		Note:  "auction-sourced order creation audit trail; order.created drives side-effects",
	},
	// auction.settlement_failed — consumed by SetupAuctionSettlementFailedHandler
	// (notification fanout). Removed from allowlist.

	// =========================================================================
	// ORDER LIFECYCLE — AUDIT ONLY
	// =========================================================================
	// order.cancelled — notification handler active (registered in SetupNotificationHandlers).
	// Removed from allowlist.
	// D1A: order.dispute_open now registered in SetupNotificationHandlers
	// Z7: order.confirmation_extended now registered in SetupNotificationHandlers

	// =========================================================================
	// REFUND LIFECYCLE
	// =========================================================================
	// D1A: refund.opened now registered in SetupNotificationHandlers
	// H2-C: refund.approved now registered in SetupNotificationHandlers
	// H2-C: refund.rejected now registered in SetupNotificationHandlers
	// D1A: refund.escalated now registered in SetupNotificationHandlers
	"refund.admin_refunded": {
		Class: NoHandlerAuditOnly,
		Note:  "admin-initiated refund audit trail",
	},
	"refund.admin_released": {
		Class: NoHandlerAuditOnly,
		Note:  "admin-initiated release audit trail",
	},

	// =========================================================================
	// ORDER — DISPUTE REFUND AUDIT
	// =========================================================================
	"order.dispute_refund_initiated": {
		Class: NoHandlerAuditOnly,
		Note:  "legacy/parked dispute refund audit trail; not emitted by current runtime",
	},
	"order.dispute_partial_refund_initiated": {
		Class: NoHandlerAuditOnly,
		Note:  "legacy/parked dispute refund audit trail; not emitted by current runtime",
	},

	// =========================================================================
	// DISPUTE LIFECYCLE
	// =========================================================================
	// D1A: dispute.resolved now registered in SetupNotificationHandlers
	// D1B: dispute.opened is retained only for legacy/parked payload compatibility
	// G1: dispute.overdue + dispute.timeout_escalation now registered in SetupNotificationHandlers

	// =========================================================================
	// MODERATION / APPEAL
	// =========================================================================
	"appeal.reversed": {
		Class: NoHandlerAuditOnly,
		Note:  "appeal reversal audit trail",
	},

	// =========================================================================
	// FOR SALE / AUCTION GOVERNANCE
	//
	// Canonical enforcement paths: ModerationEventHandler (via
	// moderation.for_sale.removed / moderation.auction.removed outbox events
	// emitted by DecisionService.CreateDecision). DomainAction worker removed
	// in Slice 9 cleanup — was parked/dead parallel enforcement mechanism.
	// =========================================================================

	// =========================================================================
	// DOMAIN ACTION WORKER (REMOVED)
	// =========================================================================
	// DomainActionWorker, DomainAction entity, and DomainActionRepository were
	// removed in Slice 9 cleanup. Canonical enforcement is ModerationEventHandler
	// via outbox events. These event types were never produced by any active code.


	// seller.tier.upgraded — consumed by SetupNotificationHandlers (B1)
	// seller.tier.downgraded — consumed by SetupNotificationHandlers (B1)
	// negotiation.cancelled — consumed by SetupNotificationHandlers (B1)

	// =========================================================================
	// CHAT ROOM EVENTS
	// =========================================================================
	"chat.room.created": {
		Class: NoHandlerFutureHook,
		Note:  "room-list realtime producer is wired first; consumer will land in the next pass",
	},
	"chat.room.updated": {
		Class: NoHandlerFutureHook,
		Note:  "room-list realtime producer is wired first; consumer will land in the next pass",
	},

	// =========================================================================
	// SOCIAL GRAPH
	// =========================================================================
	"user.unblocked": {
		Class: NoHandlerAuditOnly,
		Note:  "unblock audit trail; user.blocked triggers notification cleanup, unblock has no side-effect",
	},

	// =========================================================================
	// ACCOUNT LIFECYCLE
	// =========================================================================
	"user.deleted": {
		Class: NoHandlerFutureHook,
		Note:  "account self-deletion; in-flight orders handled by existing auto-complete/expiry workers; UserBanEventHandler re-enable deferred until gateway-funded semantics are wired",
	},

	// =========================================================================
	// BANK ACCOUNT POST-APPROVAL EVENTS (BANK_ACCOUNT_REVIEWED_FOR_PAYOUT_POLICY)
	// Emitted by BankAccountService when the seller is currently KYC-approved
	// and their bank account set changes. Signals that reviewed_bank_account_ids
	// snapshot captured at approval time may be stale. No automatic side-effect;
	// an admin must re-approve to refresh the snapshot and re-enable withdrawal.
	// Future hook: trigger an admin alert / re-review workflow.
	// =========================================================================
	"bank_account.added_after_verification": {
		Class: NoHandlerFutureHook,
		Note:  "Patch E: seller added a new bank account while KYC-approved; new account blocked at GUARD 5 until admin calls POST /admin/seller-verifications/:id/bank-accounts/:ba_id/mark-reviewed",
	},
	"bank_account.default_changed_after_verification": {
		Class: NoHandlerFutureHook,
		Note:  "Patch E: seller changed default payout account while KYC-approved; new default blocked at GUARD 5 until admin calls mark-reviewed for the new account",
	},
	"bank_account.deleted_after_verification": {
		Class: NoHandlerFutureHook,
		Note:  "Patch E: seller deleted a bank account while KYC-approved; reviewed_bank_account_ids may still reference the deleted ID (harmless — GUARD 5 validates active accounts only)",
	},
}
