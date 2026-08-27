package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	capabilityctx "github.com/labuda/backend/internal/platform/capability"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
	"github.com/labuda/backend/pkg/db"
)

// F1-W3A — /feed Pattern A handler-boundary ViewerContext construction
// and overlay hydration. Mirrors the /search/content topology at
// backend/internal/discovery/search/delivery/http/search_viewercontext.go
// and the /auctions/:id/bids D14 closure at
// backend/internal/commerce/auction/delivery/http/
// auction_bids_viewercontext.go.
//
// Three exported helpers form the canonical hydration boundary:
//
//   - constructFeedViewerContext(c, tx) — viewer identity + capability +
//     viewer lifecycle (SELECT account_status, deleted_at FROM users
//     WHERE id = $1, coarsened via viewercontext.CoarsenLifecycle).
//   - hydrateFeedTargetContext(ctx, tx, items) — per-row author
//     lifecycle (SELECT id, account_status, deleted_at FROM users WHERE
//     id = ANY($1)) and per-row content moderation (SELECT id, is_hidden
//     FROM contents WHERE id = ANY($1)). Returns the canonical
//     *viewercontext.TargetContext.
//   - hydrateFeedRelationship(ctx, tx, vc, items) — bidirectional
//     `user_blocks` resolution keyed by viewer × per-row authors.
//     Returns the ViewerContext with the relationship overlay attached
//     via WithRelationship (immutability-preserving copy).
//
// All three are pure handler-layer helpers; the evaluator package
// (feed_shadow.go, feed_enforce.go) no longer touches the DB.
//
// Boundary guarantees:
//   - Raw account_status NEVER reaches the wire; coarsening goes through
//     viewercontext.CoarsenLifecycle exactly once.
//   - users.email is NEVER projected (viewer-context-contract.md §4.1).
//   - On nil tx / DB error: overlays remain hydrated=false; the
//     evaluator emits UNKNOWN; the fail-OPEN policy in the feed adapter
//     keeps the row (high-traffic Home doctrine, feed_adapter.go §41).

// constructFeedViewerContext is the single canonical Pattern A
// ViewerContext construction site for GET /api/v1/feed. Returns
// AnonymousViewer when no userID is present; AuthenticatedViewer with
// inline-hydrated viewer lifecycle otherwise.
//
// F1-W3A — viewer lifecycle is now inline-hydrated via the request tx
// (mirrors search_viewercontext.go:96 / auction_bids_viewercontext.go).
// Anonymous-by-defect (missing userID or wrong type) yields
// AnonymousViewer per F6 closure.
//
// Per viewer-context-contract.md §4.1, email is FORBIDDEN as identity
// overlay content. PublicHandle is left empty.
func constructFeedViewerContext(c *gin.Context, tx db.Tx) *viewercontext.ViewerContext {
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

	// F1-W3A — Canonical inline viewer lifecycle hydration. c.Request
	// may be nil in unit tests (gin.CreateTestContext without a real
	// HTTP request); context.Background() is the safe fallback.
	reqCtx := context.Background()
	if c.Request != nil {
		reqCtx = c.Request.Context()
	}
	lifecycle := hydrateFeedViewerLifecycle(reqCtx, tx, userID)

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

// hydrateFeedViewerLifecycle issues the canonical single-row lifecycle
// query for the viewer's own users row and returns a LifecycleOverlay.
//
// On success: State coarsened via viewercontext.CoarsenLifecycle,
// hydrated=true. On nil tx / DB error / missing row: State=active,
// hydrated=false (consumers emit UNKNOWN; feed adapter fail-OPENs).
//
// Raw account_status and deleted_at do not leave this function.
func hydrateFeedViewerLifecycle(ctx context.Context, tx db.Tx, viewerID uuid.UUID) viewercontext.LifecycleOverlay {
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

// hydrateFeedTargetContext performs the canonical caller-batched
// hydration of per-row target overlays for /feed. Issues two
// additive queries against the existing tx:
//
//   - SELECT id, account_status, deleted_at FROM users WHERE id = ANY($1)
//     for per-row author lifecycle (coarsened).
//   - SELECT id, is_hidden FROM contents WHERE id = ANY($1) for per-row
//     content moderation. Post-F1-W1 the feed SQL already filters
//     c.is_hidden=false at the repository, so this overlay is
//     functionally a tautology today; it is hydrated for shape parity
//     with /search/content and as defense-in-depth.
//
// Returns the canonical *viewercontext.TargetContext (reused verbatim
// from the /search/content topology). Hydration failures yield empty
// maps; the evaluator surfaces those as UNKNOWN and the feed adapter
// fail-OPENs (kept rows).
func hydrateFeedTargetContext(
	ctx context.Context,
	tx db.Tx,
	items []*feedentity.FeedItem,
) *viewercontext.TargetContext {
	tc := viewercontext.NewTargetContext()
	if len(items) == 0 {
		tc.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{})
		tc.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{})
		return tc
	}

	authorIDSet := make(map[uuid.UUID]struct{}, len(items))
	contentIDSet := make(map[uuid.UUID]struct{}, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.AuthorID != uuid.Nil {
			authorIDSet[item.AuthorID] = struct{}{}
		}
		if item.ID != uuid.Nil {
			contentIDSet[item.ID] = struct{}{}
		}
	}

	authorIDs := make([]uuid.UUID, 0, len(authorIDSet))
	for id := range authorIDSet {
		authorIDs = append(authorIDs, id)
	}
	contentIDs := make([]uuid.UUID, 0, len(contentIDSet))
	for id := range contentIDSet {
		contentIDs = append(contentIDs, id)
	}

	tc.WithAuthorLifecycle(batchHydrateFeedAuthorLifecycle(ctx, tx, authorIDs))
	tc.WithContentModeration(batchHydrateFeedContentModeration(ctx, tx, contentIDs))
	return tc
}

// batchHydrateFeedAuthorLifecycle issues the canonical batched
// lifecycle resolution against `users` for the given author IDs.
//
// The query is read-only and additive; raw enum values do NOT leave
// this function — coarsening goes through viewercontext.CoarsenLifecycle.
func batchHydrateFeedAuthorLifecycle(
	ctx context.Context,
	tx db.Tx,
	authorIDs []uuid.UUID,
) map[uuid.UUID]viewercontext.PublicLifecycleState {
	out := make(map[uuid.UUID]viewercontext.PublicLifecycleState, len(authorIDs))
	if tx == nil || len(authorIDs) == 0 {
		return out
	}
	const q = `SELECT id, account_status, deleted_at FROM users WHERE id = ANY($1)`
	rows, err := tx.Query(ctx, q, authorIDs)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id            uuid.UUID
			accountStatus string
			deletedAt     interface{}
		)
		if err := rows.Scan(&id, &accountStatus, &deletedAt); err != nil {
			continue
		}
		out[id] = viewercontext.CoarsenLifecycle(accountStatus, deletedAt != nil)
	}
	return out
}

// batchHydrateFeedContentModeration issues the canonical batched
// moderation resolution against `contents` for the given content IDs.
//
// Post-F1-W1 the feed SQL filters c.is_hidden=false at the repository
// layer, so every row reaching here is by SQL invariant visible. The
// canonical resolution is retained for boundary parity with /search/
// content; a future hidden row that bypasses the SQL filter would
// still be detected at the evaluator and coarsened to TOMBSTONE.
func batchHydrateFeedContentModeration(
	ctx context.Context,
	tx db.Tx,
	contentIDs []uuid.UUID,
) map[uuid.UUID]viewercontext.ContentModerationState {
	out := make(map[uuid.UUID]viewercontext.ContentModerationState, len(contentIDs))
	if tx == nil || len(contentIDs) == 0 {
		return out
	}
	const q = `SELECT id, is_hidden FROM contents WHERE id = ANY($1)`
	rows, err := tx.Query(ctx, q, contentIDs)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id       uuid.UUID
			isHidden bool
		)
		if err := rows.Scan(&id, &isHidden); err != nil {
			continue
		}
		if isHidden {
			out[id] = viewercontext.ContentModerationStateHidden
		} else {
			out[id] = viewercontext.ContentModerationStateVisible
		}
	}
	return out
}

// hydrateOriginalAuthorLifecycles returns a map from original_author_id →
// coarsened lifecycle string for all reposts in the feed page. Used to emit
// `original_author_lifecycle` on the wire so mobile can degrade the
// attribution display when the original author is unavailable/removed.
//
// FIX-3 — lifecycle-aware original attribution. The original author is a
// separate user from the repost creator (item.AuthorID). Their lifecycle is
// not captured by hydrateFeedTargetContext which only covers item.AuthorID.
//
// On nil tx / DB error: returns empty map (fail-open, best-effort only).
func hydrateOriginalAuthorLifecycles(
	ctx context.Context,
	tx db.Tx,
	items []*feedentity.FeedItem,
) map[uuid.UUID]string {
	out := make(map[uuid.UUID]string)
	if tx == nil || len(items) == 0 {
		return out
	}

	origIDSet := make(map[uuid.UUID]struct{})
	for _, item := range items {
		if item == nil || item.OriginalAuthorID == nil || *item.OriginalAuthorID == uuid.Nil {
			continue
		}
		origIDSet[*item.OriginalAuthorID] = struct{}{}
	}
	if len(origIDSet) == 0 {
		return out
	}

	origIDs := make([]uuid.UUID, 0, len(origIDSet))
	for id := range origIDSet {
		origIDs = append(origIDs, id)
	}

	const q = `SELECT id, account_status, deleted_at FROM users WHERE id = ANY($1)`
	rows, err := tx.Query(ctx, q, origIDs)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id            uuid.UUID
			accountStatus string
			deletedAt     interface{}
		)
		if err := rows.Scan(&id, &accountStatus, &deletedAt); err != nil {
			continue
		}
		out[id] = string(viewercontext.CoarsenLifecycle(accountStatus, deletedAt != nil))
	}
	return out
}

// hydrateFeedRelationship attaches the canonical viewer × per-row
// author relationship overlay (bidirectional `user_blocks`) to the
// ViewerContext.
//
// Returns:
//   - vc unchanged for AnonymousViewer (anonymous has no
//     relationship state by topology) and for nil-UUID viewer.
//   - vc.WithRelationship(NewHydratedRelationshipOverlay(empty)) when
//     there are no per-row authors to resolve against.
//   - vc.WithRelationship(NewHydratedRelationshipOverlay(blockedIDs))
//     after the bidirectional resolution.
//   - vc unchanged on DB error — the relationship overlay remains
//     unhydrated; the evaluator emits UNKNOWN and the feed adapter
//     fail-OPENs.
func hydrateFeedRelationship(
	ctx context.Context,
	tx db.Tx,
	vc *viewercontext.ViewerContext,
	items []*feedentity.FeedItem,
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

	authorIDSet := make(map[uuid.UUID]struct{}, len(items))
	for _, item := range items {
		if item == nil || item.AuthorID == uuid.Nil {
			continue
		}
		if item.AuthorID == viewerID {
			// Self-authored item; cannot be blocked against oneself.
			continue
		}
		authorIDSet[item.AuthorID] = struct{}{}
	}
	if len(authorIDSet) == 0 {
		return vc.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(nil))
	}

	authorIDs := make([]uuid.UUID, 0, len(authorIDSet))
	for id := range authorIDSet {
		authorIDs = append(authorIDs, id)
	}

	if tx == nil {
		return vc
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
	rows, err := tx.Query(ctx, q, viewerID, authorIDs)
	if err != nil {
		return vc
	}
	defer rows.Close()

	blocked := make([]uuid.UUID, 0)
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
