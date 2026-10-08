package entity

// CancelReason classifies WHY an auction was cancelled.
//
// ONE AUTHORITY for cancellation cause: the producer stamps this reason into
// the auction.cancelled outbox payload for the audit trail. The reason is
// internal outbox data — never a public wire field (buyers see only the coarse
// PublicPhase "cancelled"; why an auction died stays private).
//
// NO notification routes on this vocabulary (purged Oct 2026): every remaining
// cancel is seller-, moderation- or admin-initiated, each of which already
// knows (self-action) or communicates through its canonical channel
// (moderation.auction.removed / admin decision UX). The subscription-expired
// direction is GONE: a lapsed subscription now lapses scheduled auctions
// (StatusLapsed) instead of cancelling, so there is no silent system-cancel
// left to notify about.
type CancelReason string

const (
	// CancelReasonSeller — seller self-cancelled their own auction (Cancel()).
	// Needs no echo notification: the seller performed the action.
	CancelReasonSeller CancelReason = "seller"

	// CancelReasonModeration — governance enforcement cancel
	// (CancelForModeration). Outcome travels the canonical moderation
	// channel (moderation.auction.removed → ModerationEventHandler).
	CancelReasonModeration CancelReason = "moderation"

	// CancelReasonAdmin — admin emergency cancel (AdminCancel); the
	// free-text admin reason is carried separately and stays internal.
	CancelReasonAdmin CancelReason = "admin"

	// CancelReasonLegacy is the zero value for auction.cancelled events
	// produced without a reason (and for non-cancel lifecycle events).
	CancelReasonLegacy CancelReason = ""
)
