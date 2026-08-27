package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
	capabilityctx "github.com/labuda/backend/internal/platform/capability"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	"github.com/labuda/backend/pkg/db"
)

// F1-W3B — /contents/:id and /users/:id/contents Pattern A handler-
// boundary ViewerContext construction and overlay hydration. Mirrors
// the F1-W3A /feed rebuild at
// backend/internal/social/feed/delivery/http/feed_viewercontext.go and
// the /search/content topology at
// backend/internal/discovery/search/delivery/http/search_viewercontext.go.
//
// Three exported helpers form the canonical hydration boundary for
// /contents/:id:
//
//   - constructContentDetailViewerContext(c, tx) — viewer identity +
//     capability (admin / block-override) + viewer lifecycle
//     (SELECT account_status, deleted_at FROM users WHERE id = $1,
//     coarsened via viewercontext.CoarsenLifecycle).
//   - hydrateContentDetailTargetContext(ctx, tx, content) — per-author
//     lifecycle (SELECT id, account_status, deleted_at FROM users WHERE
//     id = $1) and per-content moderation (SELECT id, is_hidden FROM
//     contents WHERE id = $1). Returns the canonical
//     *viewercontext.TargetContext.
//   - hydrateContentDetailRelationship(ctx, tx, vc, content) —
//     bidirectional `user_blocks` resolution keyed by viewer ×
//     content.AuthorID. Returns the ViewerContext with the relationship
//     overlay attached via WithRelationship.
//
// All three are pure handler-layer helpers; the evaluator package
// (content_detail_shadow.go, content_detail_enforce.go) no longer
// touches the DB.
//
// /users/:id/contents shares constructUserContentViewerContext which
// uses the same VC builder. The /users/:id/contents repository owns
// item filtering; this file only provides the canonical VC.
//
// Boundary guarantees:
//   - Raw account_status NEVER reaches the wire; coarsening goes through
//     viewercontext.CoarsenLifecycle exactly once.
//   - users.email is NEVER projected (viewer-context-contract.md §4.1).
//   - On nil tx / DB error: overlays remain hydrated=false; the
//     evaluator emits UNKNOWN; the fail-CLOSED detail adapter then
//     emits HTTP 404 in enforce mode (doctrine §8.5).

// constructContentDetailViewerContext is the canonical Pattern A
// ViewerContext construction site for GET /api/v1/contents/:id.
//
// Returns AnonymousViewer when no userID is present (anonymous-
// permissive per content-detail-visibility-doctrine §2.6);
// AuthenticatedViewer with inline-hydrated viewer lifecycle + lifted
// capability flags (admin / block-override) otherwise.
//
// F1-W3B — viewer lifecycle is now inline-hydrated via the request tx
// (mirrors hydrateFeedViewerLifecycle at feed_viewercontext.go). Block
// override capability is lifted from the actor middleware
// (CapabilityContentViewBlockedOverride) and carried on the canonical
// CapabilityOverlay so the evaluator can read it without a side channel.
//
// Per viewer-context-contract.md §4.1, email is FORBIDDEN as identity
// overlay content. PublicHandle is left empty.
func constructContentDetailViewerContext(c *gin.Context, tx db.Tx) *viewercontext.ViewerContext {
	return buildContentSurfaceViewerContext(c, tx)
}

// constructUserContentViewerContext is the canonical construction site
// for GET /api/v1/users/:id/contents. Shares the body with
// constructContentDetailViewerContext; both surfaces are anonymous-
// permissive and share identical viewer-side construction needs.
// Kept as separate symbols so each call site is doctrinally
// attributed even though the body is shared today.
func constructUserContentViewerContext(c *gin.Context, tx db.Tx) *viewercontext.ViewerContext {
	return buildContentSurfaceViewerContext(c, tx)
}

func buildContentSurfaceViewerContext(c *gin.Context, tx db.Tx) *viewercontext.ViewerContext {
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

	// F1-W3B — canonical inline viewer lifecycle hydration. c.Request
	// may be nil in unit tests (gin.CreateTestContext without a real
	// HTTP request); context.Background() is the safe fallback.
	reqCtx := context.Background()
	if c.Request != nil {
		reqCtx = c.Request.Context()
	}
	lifecycle := hydrateContentDetailViewerLifecycle(reqCtx, tx, userID)

	// F1-W3B — capability lift. The actor context carries admin role flag
	// plus per-capability bindings once per request; the constructor lifts
	// the bits the evaluator needs (admin / block-override) into the
	// canonical CapabilityOverlay. Block-override is required by the
	// content-detail evaluator per doctrine §5.2 — admin role alone does
	// NOT bypass blocks.
	capability := viewercontext.CapabilityOverlay{}
	if actor := capabilityctx.GetActor(reqCtx); actor != nil {
		capability.IsAdmin = actor.IsAdmin()
		capability.HasBlockOverrideCapability = actor.HasCapability(evaluator.CapabilityContentViewBlockedOverride)
	}
	// Fallback to gin-set capability flags when the actor middleware did
	// not run (e.g. unit tests / unauthenticated optional-auth path).
	// The actor path wins when both are present.
	if !capability.IsAdmin {
		if v, exists := c.Get("is_admin"); exists {
			if b, ok := v.(bool); ok {
				capability.IsAdmin = b
			}
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

// hydrateContentDetailViewerLifecycle issues the canonical single-row
// lifecycle query for the viewer's own users row and returns a
// LifecycleOverlay.
//
// On success: State coarsened via viewercontext.CoarsenLifecycle,
// hydrated=true. On nil tx / DB error / missing row: State=active,
// hydrated=false (the evaluator emits UNKNOWN; the detail-surface
// adapter then emits 404 in enforce mode per doctrine §8.5).
//
// Raw account_status and deleted_at do not leave this function.
func hydrateContentDetailViewerLifecycle(ctx context.Context, tx db.Tx, viewerID uuid.UUID) viewercontext.LifecycleOverlay {
	if tx == nil || viewerID == uuid.Nil {
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

// hydrateContentDetailTargetContext performs the canonical caller-batched
// hydration of per-row target overlays for /contents/:id. Issues two
// additive queries against the existing tx for the single requested
// content:
//
//   - SELECT id, account_status, deleted_at FROM users WHERE id = $1
//     for the per-author lifecycle (coarsened).
//   - SELECT id, is_hidden FROM contents WHERE id = $1 for the
//     per-content moderation state.
//
// Returns the canonical *viewercontext.TargetContext (reused verbatim
// from the /search/content topology). Hydration failures yield empty
// maps; the evaluator surfaces those as UNKNOWN and the fail-CLOSED
// detail adapter then emits 404 in enforce mode.
//
// Nil content / nil tx fast-paths to an empty hydrated TC; the
// evaluator distinguishes "hydrated=true map miss" from "hydrated=false"
// to keep the UNKNOWN classification accurate.
func hydrateContentDetailTargetContext(
	ctx context.Context,
	tx db.Tx,
	content *contententity.Content,
) *viewercontext.TargetContext {
	tc := viewercontext.NewTargetContext()
	if content == nil {
		tc.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{})
		tc.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{})
		return tc
	}

	tc.WithAuthorLifecycle(hydrateContentDetailAuthorLifecycle(ctx, tx, content.AuthorID))
	tc.WithContentModeration(hydrateContentDetailContentModeration(ctx, tx, content))
	return tc
}

// hydrateContentDetailAuthorLifecycle issues the canonical single-author
// lifecycle resolution against `users`. Single-row variant of the feed
// batched query — content detail has exactly one author per request.
//
// Raw enum values do NOT leave this function — coarsening goes through
// viewercontext.CoarsenLifecycle.
func hydrateContentDetailAuthorLifecycle(
	ctx context.Context,
	tx db.Tx,
	authorID uuid.UUID,
) map[uuid.UUID]viewercontext.PublicLifecycleState {
	out := make(map[uuid.UUID]viewercontext.PublicLifecycleState, 1)
	if tx == nil || authorID == uuid.Nil {
		return out
	}
	const q = `SELECT account_status, deleted_at FROM users WHERE id = $1`
	var accountStatus string
	var deletedAt interface{}
	if err := tx.QueryRow(ctx, q, authorID).Scan(&accountStatus, &deletedAt); err != nil {
		return out
	}
	out[authorID] = viewercontext.CoarsenLifecycle(accountStatus, deletedAt != nil)
	return out
}

// hydrateContentDetailContentModeration resolves the canonical per-row
// moderation state for the single requested content. The content row
// itself already carries IsHidden; the canonical query is retained for
// boundary parity with /search/content and as defense-in-depth against
// a stale in-memory entity.
func hydrateContentDetailContentModeration(
	ctx context.Context,
	tx db.Tx,
	content *contententity.Content,
) map[uuid.UUID]viewercontext.ContentModerationState {
	out := make(map[uuid.UUID]viewercontext.ContentModerationState, 1)
	if content == nil || content.ID == uuid.Nil {
		return out
	}
	if tx == nil {
		// Fall back to the in-memory entity flag — the caller has
		// already loaded the row; the canonical truth is the same
		// boolean we'd have re-queried for.
		if content.IsHidden {
			out[content.ID] = viewercontext.ContentModerationStateHidden
		} else {
			out[content.ID] = viewercontext.ContentModerationStateVisible
		}
		return out
	}
	const q = `SELECT is_hidden FROM contents WHERE id = $1`
	var isHidden bool
	if err := tx.QueryRow(ctx, q, content.ID).Scan(&isHidden); err != nil {
		// DB error → fall back to the entity flag rather than empty
		// (defense-in-depth: we already trusted the load that produced
		// `content`).
		if content.IsHidden {
			out[content.ID] = viewercontext.ContentModerationStateHidden
		} else {
			out[content.ID] = viewercontext.ContentModerationStateVisible
		}
		return out
	}
	if isHidden {
		out[content.ID] = viewercontext.ContentModerationStateHidden
	} else {
		out[content.ID] = viewercontext.ContentModerationStateVisible
	}
	return out
}

// hydrateContentDetailRelationship attaches the canonical viewer ×
// content-author relationship overlay (bidirectional `user_blocks`) to
// the ViewerContext. This is the F1-W3B closure of the C-class block-
// overlay gap on /contents/:id — pre-W3B the evaluator's block check
// relied on the legacy ContentDetailViewerContext.BlockedSet hydrated
// inside the evaluator package; W3B moves the resolution to the
// canonical handler boundary.
//
// Returns:
//   - vc unchanged for AnonymousViewer (anonymous has no relationship
//     state by topology) and for nil-UUID viewer.
//   - vc.WithRelationship(NewHydratedRelationshipOverlay(nil)) when the
//     content has no author (defensive — should not happen in practice).
//   - vc.WithRelationship(NewHydratedRelationshipOverlay(nil)) when the
//     viewer is the content author (self-authored — no block check
//     applies).
//   - vc.WithRelationship(NewHydratedRelationshipOverlay(blocked)) after
//     the bidirectional resolution.
//   - vc unchanged on DB error — the relationship overlay remains
//     unhydrated; the evaluator emits UNKNOWN and the fail-CLOSED
//     detail adapter then emits 404 in enforce mode.
func hydrateContentDetailRelationship(
	ctx context.Context,
	tx db.Tx,
	vc *viewercontext.ViewerContext,
	content *contententity.Content,
) *viewercontext.ViewerContext {
	if vc == nil {
		return vc
	}
	if vc.IsAnonymous() {
		return vc
	}

	viewerID := vc.Identity().CanonicalUserID
	if viewerID == uuid.Nil {
		return vc
	}

	if content == nil || content.AuthorID == uuid.Nil {
		return vc.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(nil))
	}

	// Self-authored content cannot be blocked against oneself.
	if content.AuthorID == viewerID {
		return vc.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(nil))
	}

	if tx == nil {
		return vc
	}

	const q = `
		SELECT CASE WHEN blocker_id = $1 THEN blocked_id ELSE blocker_id END
		FROM user_blocks
		WHERE (blocker_id = $1 AND blocked_id = $2)
		   OR (blocked_id = $1 AND blocker_id = $2)
	`
	rows, err := tx.Query(ctx, q, viewerID, content.AuthorID)
	if err != nil {
		return vc
	}
	defer rows.Close()

	blocked := make([]uuid.UUID, 0, 1)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			continue
		}
		if id == uuid.Nil {
			continue
		}
		blocked = append(blocked, id)
	}

	return vc.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(blocked))
}
