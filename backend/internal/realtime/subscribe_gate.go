package realtime

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/identity/auth"
	"go.uber.org/zap"
)

// SubscribeGate is the WS subscribe governance gate.
//
// Enforces the mandatory subscribe authorization sequence from
// governance-constitution.md §2.2 and ADR-005:
//  1. Fresh room membership check (DatabaseRoomAuthorizer)
//  2. Fresh lifecycle check (AccountStatusChecker)
//  3. Pure evaluator decision (EvaluateWSSubscribe)
//
// No SQL lives in this type; both IO calls delegate to injected dependencies.
// This type MUST NOT be placed inside Hub — Hub is transport only.
type SubscribeGate struct {
	roomAuth      RoomAuthorizer
	statusChecker auth.AccountStatusChecker
	log           *zap.Logger
}

// NewSubscribeGate creates a governance gate for WS subscribe requests.
func NewSubscribeGate(roomAuth RoomAuthorizer, statusChecker auth.AccountStatusChecker, log *zap.Logger) *SubscribeGate {
	if log == nil {
		log = zap.NewNop()
	}
	return &SubscribeGate{
		roomAuth:      roomAuth,
		statusChecker: statusChecker,
		log:           log,
	}
}

// SubscribeResult carries the governance decision for a subscribe request.
type SubscribeResult struct {
	Allowed bool
	// DenyReason is non-empty when Allowed == false.
	// Used to send a descriptive WS error frame to the client.
	DenyReason evaluator.WSSubscribeDenyReason
}

// Evaluate runs the full subscribe governance sequence.
// Returns SubscribeResult.Allowed=true only when:
//   - user lifecycle is active (fresh DB read)
//   - user is a member of the room (fresh DB read)
//
// Fails closed: any IO error, unknown lifecycle, or missing membership → deny.
// Must be called BEFORE hub.Subscribe(); never called inside Hub.
func (g *SubscribeGate) Evaluate(ctx context.Context, userID, roomID uuid.UUID, roomType RoomType) SubscribeResult {
	// 1. Membership check (DatabaseRoomAuthorizer does the DB query)
	membershipGranted := g.roomAuth.CanSubscribeToRoom(ctx, userID, roomID, roomType)

	// 2. Fresh lifecycle check — never reuse session-time state
	status, err := g.statusChecker.GetStatus(ctx, userID)
	if err != nil {
		g.log.Warn("Lifecycle check failed for WS subscribe",
			zap.String("user_id", userID.String()),
			zap.String("room_id", roomID.String()),
			zap.Error(err),
		)
		return SubscribeResult{Allowed: false, DenyReason: evaluator.WSSubscribeDenyReasonUnknown}
	}

	// CoarsenLifecycle is the single canonical coarsening site per viewercontext package.
	lifecycle := viewercontext.CoarsenLifecycle(status, false)

	// 3. Pure evaluator decision — no IO
	decision, reason := evaluator.EvaluateWSSubscribe(lifecycle, membershipGranted)

	if decision != evaluator.WSSubscribeAllow {
		g.log.Warn("WS subscribe denied by governance gate",
			zap.String("user_id", userID.String()),
			zap.String("room_id", roomID.String()),
			zap.String("room_type", string(roomType)),
			zap.String("lifecycle", string(lifecycle)),
			zap.Bool("membership_granted", membershipGranted),
			zap.String("deny_reason", string(reason)),
		)
		return SubscribeResult{Allowed: false, DenyReason: reason}
	}

	return SubscribeResult{Allowed: true}
}

// IsAlive checks whether a user's account lifecycle is still active.
// Used by WritePump to periodically evict stale connections for users
// whose status changed after WS connect (removed, suspended, banned).
//
// Fail-closed: returns false on any IO error.
func (g *SubscribeGate) IsAlive(ctx context.Context, userID uuid.UUID) bool {
	status, err := g.statusChecker.GetStatus(ctx, userID)
	if err != nil {
		g.log.Debug("Lifecycle check failed for IsAlive",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		return false // fail-closed
	}
	lifecycle := viewercontext.CoarsenLifecycle(status, false)
	return lifecycle == viewercontext.PublicLifecycleStateActive
}


