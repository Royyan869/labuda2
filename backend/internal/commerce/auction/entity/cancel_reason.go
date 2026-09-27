package entity

// CancelReason classifies WHY an auction was cancelled.
//
// ONE AUTHORITY for cancellation cause: the producer stamps this reason into
// the auction.cancelled outbox payload; the notification worker routes the
// seller-facing auto-cancel notification on it. The reason is internal
// outbox data — never a public wire field (buyers see only the coarse
// PublicPhase "cancelled"; why an auction died stays private).
//
// Buyer notifications on cancellation are intentionally NOT built (owner
// decision): this vocabulary exists for producer→worker routing only.
type CancelReason string

const (
	// CancelReasonSeller — seller self-cancelled their own auction (Cancel()).
	// Needs no echo notification: the seller performed the action.
	CancelReasonSeller CancelReason = "seller"

	// CancelReasonSubscriptionExpired — auction auto-cancelled by the
	// activation worker because the seller's subscription (market authority)
	// expired. The ONLY reason that triggers a seller notification today:
	// the seller took no action and must learn their listing died with the
	// subscription (Scope B).
	CancelReasonSubscriptionExpired CancelReason = "subscription_expired"

	// CancelReasonModeration — governance enforcement cancel
	// (CancelForModeration). Outcome travels the canonical moderation
	// channel (moderation.auction.removed → ModerationEventHandler).
	CancelReasonModeration CancelReason = "moderation"

	// CancelReasonAdmin — admin emergency cancel (AdminCancel); the
	// free-text admin reason is carried separately and stays internal.
	CancelReasonAdmin CancelReason = "admin"

	// CancelReasonLegacy is the zero value for historical auction.cancelled
	// events produced before reasons existed (and for non-cancel lifecycle
	// events). The worker parses it as a no-op — fail-closed (no
	// notification, no error) without replay churn.
	CancelReasonLegacy CancelReason = ""
)

// NotifiesSeller reports whether this cancellation reason warrants a
// seller-facing notification. Single routing authority for the notification
// worker: only system-initiated auto-cancels notify (the seller took no
// action); moderation/admin outcomes communicate through their own canonical
// channels, and self-cancels need no echo.
func (r CancelReason) NotifiesSeller() bool {
	return r == CancelReasonSubscriptionExpired
}
