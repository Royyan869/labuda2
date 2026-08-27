package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/governance/viewercontext"
	capabilityctx "github.com/labuda/backend/internal/platform/capability"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// constructSearchContentViewerContext builds the canonical Pattern A
// ViewerContext at the HTTP boundary for /search/content per
// docs/05-rollout/search-content-viewer-lifecycle-hydration-runtime-
// task-design.md §5.1 / §5.2.
//
// The function is the single canonical construction site for
// /search/content; it consumes the gin context values that the
// AuthMiddleware / UserLookupMiddleware / RolesLookupMiddleware /
// ActorContextInject chain already resolved (per
// backend/cmd/core_server/routes_core.go:99-104) and returns either
// AuthenticatedViewer (if user_id is present and resolvable) or
// AnonymousViewer (otherwise).
//
// For authenticated viewers the function issues a single canonical
// lifecycle query:
//
//	SELECT account_status, deleted_at FROM users WHERE id = $1
//
// using the existing transaction tx. The result is coarsened via
// CoarsenLifecycle and recorded in LifecycleOverlay{hydrated: true}.
// On DB error / missing row / nil tx, the overlay is constructed with
// hydrated=false and the HTTP request continues unaffected per
// docs/05-rollout/search-content-viewer-lifecycle-hydration-runtime-
// task-design.md §6.1 / §6.2. The shadow runner emits overlay_status=error
// when it observes IsHydrated()==false.
//
// Per viewer-context-contract.md §4.1, email is FORBIDDEN as identity
// overlay content. The PublicHandle is left empty here; future material
// tasks may attach it from a canonical username binding (users.username)
// without violating this rule.
func constructSearchContentViewerContext(c *gin.Context, tx db.Tx) *viewercontext.ViewerContext {
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
		// PublicHandle deliberately left empty — email-forbidden per §4.1.
	}

	// Canonical viewer lifecycle hydration per
	// docs/05-rollout/search-content-viewer-lifecycle-hydration-runtime-
	// task-design.md §5.1-§5.4.
	//
	// The query reads users.account_status + users.deleted_at for the
	// viewer's own row and coarsens via CoarsenLifecycle. On any failure
	// (nil tx, DB error, missing row) the overlay is marked hydrated=false;
	// the HTTP request continues and the shadow runner emits
	// overlay_status=error + unknown_source=viewer_lifecycle.
	//
	// c.Request may be nil in unit tests (gin.CreateTestContext without a
	// real HTTP request); context.Background() is the safe fallback —
	// lifecycle query timeouts propagate via the caller's deadline in prod.
	reqCtx := context.Background()
	if c.Request != nil {
		reqCtx = c.Request.Context()
	}
	lifecycle := hydrateViewerLifecycle(reqCtx, tx, userID)

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

// hydrateViewerLifecycle issues the canonical single-row lifecycle query
// for the viewer's own users row and returns a LifecycleOverlay.
//
// On success: State is coarsened via CoarsenLifecycle, hydrated=true.
// On nil tx / DB error / missing row: State=active, hydrated=false.
//
// Raw account_status and deleted_at values do not leave this function;
// only the coarsened LifecycleOverlay is returned per
// docs/05-rollout/search-content-viewer-lifecycle-hydration-runtime-
// task-design.md §5.4.
func hydrateViewerLifecycle(ctx context.Context, tx db.Tx, viewerID uuid.UUID) viewercontext.LifecycleOverlay {
	if tx == nil {
		// nil tx is treated identically to a DB error — hydrated=false.
		// This path is hit in unit tests that stub anonymous/early-return
		// paths; it must not default-active in production.
		return viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, false)
	}

	const q = `SELECT account_status, deleted_at FROM users WHERE id = $1`
	var accountStatus string
	var deletedAt interface{}
	if err := tx.QueryRow(ctx, q, viewerID).Scan(&accountStatus, &deletedAt); err != nil {
		// DB error or missing row — log at WARN level (user_id_hashed, no
		// raw SQL error text per docs/05-rollout/search-endpoint-telemetry-
		// enum-design.md §11.3); continue with hydrated=false.
		zap.L().Warn("viewer lifecycle hydration failed",
			zap.String("unknown_source", "viewer_lifecycle"),
			zap.String("user_id_hashed", hashViewerID(viewerID)),
		)
		return viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, false)
	}

	state := viewercontext.CoarsenLifecycle(accountStatus, deletedAt != nil)
	return viewercontext.NewLifecycleOverlay(state, true)
}

// hashViewerID returns a 16-char hex one-way hash of the viewer UUID for
// structured-log correlation per docs/05-rollout/search-endpoint-
// telemetry-enum-design.md §12.4 (raw UUIDs FORBIDDEN in logs).
func hashViewerID(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	sum := sha256.Sum256(id[:])
	return hex.EncodeToString(sum[:8])
}

// hydrateSearchContentTargetContext performs the canonical caller-batched
// hydration of target-side overlays for /search/content per
// docs/05-rollout/search-content-viewercontext-runtime-threading-task-
// design.md §6.2.
//
// The function:
//   - extracts the per-row author_id and content_id sets from the candidate
//     slice,
//   - issues a single batched users-lifecycle query against the per-row
//     author set,
//   - issues a single batched contents-moderation query against the per-row
//     content set,
//   - constructs a TargetContext with the hydrated overlays,
//   - returns the TargetContext to the caller (the HTTP handler) for
//     downstream consumption by the future seam runner.
//
// Per viewer-context-contract.md §2.4 and docs/05-rollout/search-overlay-
// ownership-matrix.md §3 / §5, this hydration runs at the handler boundary
// — it is NOT in the repository, NOT in the service, NOT in the evaluator.
// Per docs/05-rollout/search-content-viewercontext-runtime-threading-task-
// design.md §7.5, this is additive batched hydration; the legacy content-
// search SQL filter is unchanged.
//
// Hydration failures are NOT escalated — the function returns a
// TargetContext whose per-row entries reflect what was successfully
// hydrated. Consumers (future seam runner) emit UNKNOWN with classified
// reason per viewer-context-contract.md §2.4 / docs/05-rollout/search-
// endpoint-telemetry-enum-design.md §6 for unhydrated rows. This task
// does NOT register telemetry per task-design §7.2.
func hydrateSearchContentTargetContext(
	ctx context.Context,
	tx db.Tx,
	contents []*entity.ContentPreview,
) *viewercontext.TargetContext {
	tc := viewercontext.NewTargetContext()
	if len(contents) == 0 {
		// No candidates → empty hydration. Mark overlays as hydrated-empty
		// so downstream consumers do not treat them as never-hydrated
		// (UNKNOWN classification).
		tc.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{})
		tc.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{})
		return tc
	}

	authorIDSet := make(map[uuid.UUID]struct{}, len(contents))
	contentIDSet := make(map[uuid.UUID]struct{}, len(contents))
	for _, c := range contents {
		if c == nil {
			continue
		}
		if c.AuthorID != uuid.Nil {
			authorIDSet[c.AuthorID] = struct{}{}
		}
		if c.ID != uuid.Nil {
			contentIDSet[c.ID] = struct{}{}
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

	authorLifecycle := batchHydrateAuthorLifecycle(ctx, tx, authorIDs)
	tc.WithAuthorLifecycle(authorLifecycle)

	contentModeration := batchHydrateContentModeration(ctx, tx, contentIDs)
	tc.WithContentModeration(contentModeration)

	return tc
}

// batchHydrateAuthorLifecycle issues the canonical batched lifecycle
// resolution against `users` for the given author IDs.
//
// The query is read-only and additive — it does NOT modify the legacy
// content-search SQL per docs/05-rollout/search-content-viewercontext-
// runtime-threading-task-design.md §7.5.
func batchHydrateAuthorLifecycle(
	ctx context.Context,
	tx db.Tx,
	authorIDs []uuid.UUID,
) map[uuid.UUID]viewercontext.PublicLifecycleState {
	out := make(map[uuid.UUID]viewercontext.PublicLifecycleState, len(authorIDs))
	if len(authorIDs) == 0 {
		return out
	}

	const q = `SELECT id, account_status, deleted_at FROM users WHERE id = ANY($1)`
	rows, err := tx.Query(ctx, q, authorIDs)
	if err != nil {
		// Hydration error — return what we have (empty map). Consumers
		// (future seam runner) emit UNKNOWN per viewer-context-contract.md
		// §2.4. The handler does NOT escalate hydration failure into the
		// user-visible response per docs/05-rollout/search-content-viewer
		// context-runtime-threading-task-design.md §10.9.
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

// batchHydrateContentModeration issues the canonical batched moderation
// resolution against `contents` for the given content IDs.
//
// The query is read-only and additive. The legacy content-search SQL
// filter `c.is_hidden = false` at search_repository_impl.go:216 means
// rows reaching this hydration step are by construction is_hidden=false;
// the canonical resolution issued here is the boundary doctrine (per
// docs/05-rollout/search-overlay-ownership-matrix.md §5 — caller hydrates
// truth) rather than a per-row novelty. The shadow runner (future seam-
// landing material task) will compare the hydrated value against the
// legacy SQL filter behavior to detect drift.
func batchHydrateContentModeration(
	ctx context.Context,
	tx db.Tx,
	contentIDs []uuid.UUID,
) map[uuid.UUID]viewercontext.ContentModerationState {
	out := make(map[uuid.UUID]viewercontext.ContentModerationState, len(contentIDs))
	if len(contentIDs) == 0 {
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

// hydrateSearchContentRelationship attaches the canonical viewer × per-row
// author relationship overlay to the ViewerContext per docs/05-rollout/
// search-content-viewercontext-runtime-threading-task-design.md §5.4.
//
// Per docs/05-rollout/search-shadow-seam-architecture.md §5.4, the
// canonical relationship resolution is the bidirectional `user_blocks`
// query (`viewerID × {per-row authorIDs}`).
//
// AnonymousViewer skips the resolution per viewer-context-contract.md
// §3.1 — the existing AnonymousViewer relationship overlay is already
// hydrated-as-anonymous-empty.
//
// Hydration failures return the original ViewerContext unchanged — the
// relationship overlay remains unhydrated; consumers (future seam runner)
// emit UNKNOWN with reason=hydration_error and source=relationship per
// docs/05-rollout/search-endpoint-telemetry-enum-design.md §6.
func hydrateSearchContentRelationship(
	ctx context.Context,
	tx db.Tx,
	vc *viewercontext.ViewerContext,
	contents []*entity.ContentPreview,
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

	authorIDSet := make(map[uuid.UUID]struct{}, len(contents))
	for _, c := range contents {
		if c == nil || c.AuthorID == uuid.Nil {
			continue
		}
		authorIDSet[c.AuthorID] = struct{}{}
	}
	if len(authorIDSet) == 0 {
		return vc.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(nil))
	}

	authorIDs := make([]uuid.UUID, 0, len(authorIDSet))
	for id := range authorIDSet {
		authorIDs = append(authorIDs, id)
	}

	// Bidirectional `user_blocks` resolution: viewer blocks any author OR
	// any author blocks viewer.
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
		// Hydration error — leave overlay unhydrated; future seam runner
		// emits UNKNOWN. We do NOT escalate; per task-design §10.9 user-
		// visible response is unaffected by hydration failures.
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
