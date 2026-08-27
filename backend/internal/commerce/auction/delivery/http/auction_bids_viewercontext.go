package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/pkg/publiccard"
	capabilityctx "github.com/labuda/backend/internal/platform/capability"
	"github.com/labuda/backend/pkg/db"
)

// D14 — Auction bid discovery governance convergence.
//
// This file is the canonical Pattern A construction + hydration boundary
// for GET /api/v1/auctions/:id/bids. It mirrors the /search/content
// topology at backend/internal/discovery/search/delivery/http/
// search_viewercontext.go: ViewerContext is built at the HTTP boundary,
// per-row overlays are caller-batched against the request tx, and the
// evaluator is NOT introduced — this is bounded visibility convergence,
// not an evaluator rollout.
//
// Boundary guarantees:
//   - Raw account_status NEVER reaches the wire; coarsening goes through
//     viewercontext.CoarsenLifecycle exactly once.
//   - Bidder identity surfaces only through publiccard.UserCard; no flat
//     bidder_username scalar is re-introduced on the wire.
//   - Bidirectional user_blocks is enforced caller-side; the handler drops
//     rows whose bidder is in the blocked set before any response writing.
//   - Hydration errors fail-OPEN per the D14 spec (Part 3): a DB failure
//     on the blocks query does NOT silently downgrade to allow-all; it
//     returns an empty blocked set (no rows hidden) but never bypasses
//     authorization — authorization is the auth middleware's job, not this
//     overlay's.

// constructAuctionBidsViewerContext is the single canonical ViewerContext
// construction site for /auctions/:id/bids. It consumes the gin context
// values already resolved by AuthMiddleware → UserLookupMiddleware →
// RolesLookupMiddleware → ActorContextInject (per
// backend/cmd/core_server/routes_core.go:119-125) and returns either an
// AuthenticatedViewer (when user_id is present and resolvable) or an
// explicit AnonymousViewer (otherwise).
//
// The route is currently gated by AuthMiddleware so anonymous callers do
// not reach this handler in production; the AnonymousViewer branch is
// defensive — it makes the construction site safe under future route
// re-mounting without re-introducing forbidden pattern F6 (nil userID
// treated as anonymous without explicit NewAnonymous construction).
//
// Per viewer-context-contract.md §4.1, email is FORBIDDEN as identity
// overlay content. PublicHandle is left empty here.
func constructAuctionBidsViewerContext(c *gin.Context, tx db.Tx) *viewercontext.ViewerContext {
	userIDVal, userIDPresent := c.Get("user_id")
	if !userIDPresent {
		userIDVal, userIDPresent = c.Get("userID")
	}
	if !userIDPresent {
		return viewercontext.NewAnonymous(
			viewercontext.SurfacePublicDiscovery,
			viewercontext.RequestOriginREST,
		)
	}

	userID, ok := userIDVal.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return viewercontext.NewAnonymous(
			viewercontext.SurfacePublicDiscovery,
			viewercontext.RequestOriginREST,
		)
	}

	firebaseUID := ""
	if v, exists := c.Get("firebase_uid"); exists {
		if s, ok := v.(string); ok {
			firebaseUID = s
		}
	}

	identity := viewercontext.IdentityOverlay{
		FirebaseUID:     firebaseUID,
		CanonicalUserID: userID,
	}

	reqCtx := context.Background()
	if c.Request != nil {
		reqCtx = c.Request.Context()
	}

	lifecycle := hydrateAuctionBidsViewerLifecycle(reqCtx, tx, userID)

	capability := viewercontext.CapabilityOverlay{}
	if v, exists := c.Get("is_admin"); exists {
		if b, ok := v.(bool); ok {
			capability.IsAdmin = b
		}
	}
	if actor := capabilityctx.GetActor(reqCtx); actor != nil {
		capability.IsSeller = actor.IsSellerReady()
	}

	moderation := viewercontext.ModerationOverlay{}

	return viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery,
		viewercontext.RequestOriginREST,
		identity,
		lifecycle,
		capability,
		moderation,
	)
}

// hydrateAuctionBidsViewerLifecycle issues the canonical single-row
// lifecycle query for the viewer's own users row and returns a
// LifecycleOverlay. On success: State is coarsened via CoarsenLifecycle,
// hydrated=true. On nil tx / DB error / missing row: State=active,
// hydrated=false. Raw account_status / deleted_at do not leave this
// function.
func hydrateAuctionBidsViewerLifecycle(ctx context.Context, tx db.Tx, viewerID uuid.UUID) viewercontext.LifecycleOverlay {
	if tx == nil {
		return viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, false)
	}
	const q = `SELECT account_status, deleted_at FROM users WHERE id = $1`
	var accountStatus string
	var deletedAt interface{}
	if err := tx.QueryRow(ctx, q, viewerID).Scan(&accountStatus, &deletedAt); err != nil {
		return viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, false)
	}
	state := viewercontext.CoarsenLifecycle(accountStatus, deletedAt != nil)
	return viewercontext.NewLifecycleOverlay(state, true)
}

// hydrateBidderLifecycleCards batch-hydrates publiccard.UserCard for every
// distinct bidder across the page, including the coarsened public
// lifecycle. Returns a map keyed by user_id. Missing rows (or empty map
// on hydration error) yield no entry; the caller falls back to an
// anonymous-safe card so the row remains renderable.
//
// PUBLIC BOUNDARY:
//   - users.email is NEVER projected here per viewer-context-contract.md
//     §4.1.
//   - Raw account_status is coarsened via viewercontext.CoarsenLifecycle
//     before the value leaves this function. The map values carry only
//     the publiccard.UserCard public-safe shape.
//   - No N+1; a single ANY($1) query covers the full page.
//
// Slot-persistence: there is no `WHERE deleted_at IS NULL` filter — a
// soft-deleted bidder surfaces as lifecycle="removed" rather than
// silently disappearing. Mirrors E4.2 / E6 / E8.1 doctrine.
func hydrateBidderLifecycleCards(
	ctx context.Context,
	tx db.Tx,
	bids []*entity.AuctionBid,
) map[uuid.UUID]publiccard.UserCard {
	out := make(map[uuid.UUID]publiccard.UserCard, len(bids))
	if len(bids) == 0 {
		return out
	}

	ids := make([]uuid.UUID, 0, len(bids))
	seen := make(map[uuid.UUID]struct{}, len(bids))
	for _, b := range bids {
		if b == nil || b.BidderID == uuid.Nil {
			continue
		}
		if _, ok := seen[b.BidderID]; ok {
			continue
		}
		seen[b.BidderID] = struct{}{}
		ids = append(ids, b.BidderID)
	}
	if len(ids) == 0 {
		return out
	}

	const q = `
		SELECT u.id,
		       COALESCE(up.username, '')   AS username,
		       COALESCE(up.avatar_url, '') AS avatar_url,
		       u.account_status,
		       (u.deleted_at IS NOT NULL)  AS is_deleted
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		WHERE u.id = ANY($1)
	`
	rows, err := tx.Query(ctx, q, ids)
	if err != nil {
		return out
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id            uuid.UUID
			username      string
			avatarURL     string
			accountStatus string
			isDeleted     bool
		)
		if err := rows.Scan(&id, &username, &avatarURL, &accountStatus, &isDeleted); err != nil {
			continue
		}
		lifecycle := string(viewercontext.CoarsenLifecycle(accountStatus, isDeleted))
		var avatarPtr *string
		if avatarURL != "" {
			a := avatarURL
			avatarPtr = &a
		}
		out[id] = publiccard.NewWithLifecycle(id, username, avatarPtr, lifecycle)
	}
	return out
}

// hydrateBidsBlockedSet returns the set of bidder IDs that are
// bidirectionally blocked relative to the viewer: viewer→bidder OR
// bidder→viewer. The set is consumed by the caller to drop blocked
// bidder rows entirely from the response.
//
// Returns:
//   - Empty set for AnonymousViewer (block semantics are viewer-relative;
//     there is no viewer identity to compare against).
//   - Empty set for nil ViewerContext.
//   - Empty set on hydration failure (fail-OPEN on hydration per D14 spec
//     Part 3; this never bypasses authorization, only relationship
//     filtering — authorization is the auth middleware's job).
//
// The bidirectional resolution mirrors hydrateSearchContentRelationship
// in /search/content (search_viewercontext.go:355-422).
func hydrateBidsBlockedSet(
	ctx context.Context,
	tx db.Tx,
	vc *viewercontext.ViewerContext,
	bids []*entity.AuctionBid,
) map[uuid.UUID]struct{} {
	out := make(map[uuid.UUID]struct{})
	if vc == nil || vc.IsAnonymous() {
		return out
	}
	viewerID := vc.Identity().CanonicalUserID
	if viewerID == uuid.Nil {
		return out
	}

	idSet := make(map[uuid.UUID]struct{}, len(bids))
	for _, b := range bids {
		if b == nil || b.BidderID == uuid.Nil {
			continue
		}
		if b.BidderID == viewerID {
			continue
		}
		idSet[b.BidderID] = struct{}{}
	}
	if len(idSet) == 0 {
		return out
	}

	ids := make([]uuid.UUID, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	const q = `
		SELECT DISTINCT
			CASE
				WHEN blocker_id = $1 THEN blocked_id
				ELSE blocker_id
			END AS other_id
		FROM user_blocks
		WHERE (blocker_id = $1 AND blocked_id = ANY($2))
		   OR (blocked_id = $1 AND blocker_id = ANY($2))
	`
	rows, err := tx.Query(ctx, q, viewerID, ids)
	if err != nil {
		return out
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			continue
		}
		if id == uuid.Nil {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}
