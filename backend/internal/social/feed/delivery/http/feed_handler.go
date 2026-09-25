package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/platform/response"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	feedApp "github.com/labuda/backend/internal/social/feed/application"
	"github.com/labuda/backend/internal/social/feed/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// derefOrEmpty safely dereferences a string pointer, returning empty string if nil.
// Used to prevent empty string leaks in location responses.
func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// hasNonEmptyValue returns true if the pointer is non-nil and points to a non-empty string.
// Used to guard against empty string leaks in conditional logic.
func hasNonEmptyValue(s *string) bool {
	return s != nil && *s != ""
}

// FeedHandler handles HTTP requests for feed operations.
type FeedHandler struct {
	feedService       *feedApp.FeedService
	db                *db.DB
	log               *zap.Logger
	shadowRunner      *evaluator.FeedShadowRunner // optional; nil disables shadow observability
	promotionInjector *FeedPromotionInjector      // optional; nil disables promotion injection
}

// NewFeedHandler creates a new FeedHandler. /feed enforcement is
// unconditional. shadowRunner is observability-only and never gates
// business enforcement.
func NewFeedHandler(
	feedService *feedApp.FeedService,
	database *db.DB,
	log *zap.Logger,
	shadowRunner *evaluator.FeedShadowRunner,
	promotionInjector *FeedPromotionInjector,
) *FeedHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &FeedHandler{
		feedService:       feedService,
		db:                database,
		log:               log,
		shadowRunner:      shadowRunner,
		promotionInjector: promotionInjector,
	}
}

// GetFeedRequest holds query parameters for getting feed.
type GetFeedRequest struct {
	Cursor *string `form:"cursor" binding:"omitempty"` // opaque continuation cursor from prior page
	Limit  int     `form:"limit" binding:"omitempty,min=1,max=50"`
}

// GetFeed handles GET /api/v1/feed
//
// Retrieves the authenticated user's feed with content from followed users.
//
// Query parameters:
//   - cursor: opaque continuation cursor returned in the previous
//     response (treat as opaque on the client; format is internal and
//     may change without notice). Omit for the first page.
//   - limit: Number of items to return (default 20, max 50)
//
// Response:
//   - data: Array of feed items
//   - next_cursor: Opaque cursor for next page; null on the terminal page
//   - has_more: Boolean indicating if more results exist (derived from
//     a LIMIT+1 probe, not from a boundary-equality heuristic)
func (h *FeedHandler) GetFeed(c *gin.Context) {
	ctx := c.Request.Context()

	// GUEST HOME (Owner canonical): /feed serves BOTH anonymous and
	// authenticated viewers through ONE authority — the ViewerContext
	// decides the policy:
	//   - AnonymousViewer  → global public content discovery only
	//     (repository visibility clause: public content, no follow graph,
	//     no blocks/mutes; Priority Group 1).
	//   - Authenticated    → follow/own + public discovery (Priority 0 + 1).
	// F1-W3A Pattern A construction stays at the HTTP boundary; the post-tx
	// re-invocation (inside WithTx below) wires inline viewer-lifecycle
	// hydration. F8 closed: raw c.Get("userID") is never the visibility
	// authority — the ViewerContext is.
	vc := constructFeedViewerContext(c, nil)
	isAnonymous := vc.IsAnonymous()
	callerID := vc.Identity().CanonicalUserID // uuid.Nil for anonymous

	// Parse query parameters
	var req GetFeedRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	// Set default limit if not provided
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}

	// Decode opaque cursor. Empty / nil → first page; malformed → 400.
	// The decoder enforces (ts, id) presence; intermediate clients that
	// fabricate cursors will reliably fail closed here.
	var cursor *entity.FeedCursor
	if req.Cursor != nil && *req.Cursor != "" {
		decoded, err := entity.DecodeFeedCursor(*req.Cursor)
		if err != nil {
			response.BadRequest(c, "Invalid cursor")
			return
		}
		cursor = decoded
	}

	// Get feed from service. F1-W3A — overlay hydration is now
	// caller-batched at the handler boundary inside the same tx so
	// the evaluator package no longer touches the DB. Order:
	//   1. Re-construct the ViewerContext WITH tx — this triggers
	//      inline viewer-lifecycle hydration via
	//      hydrateFeedViewerLifecycle (mirror of
	//      constructSearchContentViewerContext at
	//      search_viewercontext.go:96).
	//   2. Run the existing repository query (SQL authority unchanged).
	//   3. Build the canonical TargetContext from the page (per-row
	//      author lifecycle + content moderation).
	//   4. Attach the bidirectional block overlay to the VC.
	var result *entity.FeedResult
	var tc *viewercontext.TargetContext
	var origAuthorLifecycles map[uuid.UUID]string // FIX-3: original-author lifecycle map for reposts
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		vc = constructFeedViewerContext(c, tx)
		isAnonymous = vc.IsAnonymous()
		// Geography is an authenticated-viewer overlay; anonymous viewers
		// have no primary address.
		if !isAnonymous {
			vc = vc.WithGeography(viewercontext.ResolveViewerGeography(ctx, tx, callerID))
		}
		var err error
		result, err = h.feedService.GetFeed(ctx, tx, callerID, cursor, limit)
		if err != nil {
			return err
		}
		tc = hydrateFeedTargetContext(ctx, tx, result.Items)
		// Relationship/block overlay is authenticated-only — an anonymous
		// viewer has no social graph (the repository already excludes
		// blocks/mutes through the $1 = nil clause).
		if !isAnonymous {
			vc = hydrateFeedRelationship(ctx, tx, vc, result.Items)
		}
		// FIX-3 — batch-hydrate original-author lifecycle for reposts.
		origAuthorLifecycles = hydrateOriginalAuthorLifecycles(ctx, tx, result.Items)
		return nil
	})

	if err != nil {
		h.log.Error("Failed to get feed",
			zap.String("caller_id", callerID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve feed")
		return
	}

	// Snapshot the original SQL result BEFORE enforce filtering so the
	// fire-and-forget shadow runner downstream can observe divergence
	// on the same row set in both shadow and enforce modes — the
	// telemetry denominator stays comparable across the flip
	// (Batch 3M, ack #4 in batch prompt).
	originalItems := result.Items

	// BATCH 3M / C1 / F1-W3A — synchronous further-restrict enforcement.
	//
	// Enforcement is unconditional and independent of shadowRunner
	// existence. h.shadowRunner is observability-only and never gates
	// business enforcement.
	//
	// The handler runs EvaluateFeedItem + AdaptFeedDecision over the
	// legacy SQL result. C1 convergence: the adapter coarsens TOMBSTONE →
	// "removed" and REDACT → "unavailable" into a LifecycleOverrides map
	// (mirror of /search/content's enforcement.LifecycleOverrides). DENY
	// rows drop; UNKNOWN rows fail OPEN (kept).
	//
	// F1-W3A — the enforcement helper consumes the same pre-hydrated
	// (vc, tc) the handler built inside WithTx. The evaluator package
	// owns NO SQL, NO pool, NO hydration helpers. Telemetry counter
	// names, label sets, and emission cardinality are unchanged.
	//
	// Pagination authority unchanged: result.HasMore / result.NextCursor
	// stay pre-filter (repository values). After enforce, the response
	// may carry fewer than `limit` rows with has_more=true; the mobile
	// client's cursor-stall + has_more handling (Batch 3G) tolerates
	// that without infinite-loop risk.
	enf := evaluator.EnforceFeed(vc, tc, originalItems)
	result.Items = enf.Filtered
	lifecycleOverrides := enf.LifecycleOverrides

	// Convert feed items to response format. lifecycleOverrides is nil
	// when no row took the override path; the renderer short-circuits
	// cleanly in that case.
	projections := make(map[uuid.UUID]*contentApp.ContentResourceProjection)
	// Anonymous viewers skip commerce-resource projection hydration — the
	// Guest Home feed is a public content discovery feed.
	if !isAnonymous && len(result.Items) > 0 {
		if loaded, projErr := loadFeedContentResourceProjections(ctx, h.db, callerID, result.Items); projErr == nil {
			projections = loaded
		} else {
			h.log.Warn("failed to load feed content resource projections", zap.Error(projErr))
		}
	}

	items := make([]map[string]interface{}, len(result.Items))
	for i, item := range result.Items {
		resp, err := feedItemToResponseCanonicalStrict(item, lifecycleOverrides, origAuthorLifecycles, projections[item.ID])
		if err != nil {
			h.log.Warn("failed to render feed item with canonical projection", zap.String("item_id", item.ID.String()), zap.Error(err))
			resp = feedItemToResponseCanonical(item, lifecycleOverrides, origAuthorLifecycles)
		}
		// C7C — best-effort engagement hydration (same authority as detail: CountTopLevelCommentsByContent + CountLikes).
		// Fail-open: 0 on error, never blocks feed. Single canonical source for card & detail.
		var cc, lc int
		_ = h.db.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM comments WHERE target_id = $1 AND target_type = 'content' AND deleted_at IS NULL AND parent_id IS NULL`, item.ID).Scan(&cc)
		_ = h.db.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM content_likes WHERE content_id = $1`, item.ID).Scan(&lc)
		resp["commentCount"] = cc
		resp["likeCount"] = lc
		resp["engagement"] = map[string]interface{}{"commentCount": cc, "likeCount": lc}
		items[i] = resp
	}

	// PHASE C / F1-W3A — Feed evaluator shadow observability. Fire-
	// and-forget. The runner is the caller (ViewerContext Contract §5.1)
	// and never affects the response, pagination, or legacy authority.
	// nil-safe: when shadow is disabled, this is a no-op.
	//
	// IMPORTANT (Batch 3M): passes the ORIGINAL pre-filter slice so
	// shadow divergence cells stay denominator-consistent with what the
	// legacy SQL allowed. F1-W3A: also passes the pre-hydrated (vc, tc)
	// so the goroutine no longer touches the DB.
	h.shadowRunner.Run(vc, tc, originalItems)

	// P3A — Promotion injection. Fetch active promoted items, hydrate
	// card data, and interleave into the organic feed at slot positions.
	// FAIL-OPEN: if anything errors, items stays unchanged. viewerID is the
	// audience fact carried into canonical delivery measurement; geography
	// is the canonical viewer primary address. Anonymous Guest Home stays a
	// pure public content discovery feed (no viewer-audience measurement) —
	// commerce discovery lives on the Explore/For Sale surfaces.
	if !isAnonymous {
		geo := vc.Geography()
		items = h.promotionInjector.InjectPromotionsWithGeography(ctx, callerID, geo.CityID, geo.HasPrimary, items)
	}

	// Re-encode the next cursor at the HTTP boundary. nil cursor →
	// JSON null (json.Marshal renders the typed *string nil as null).
	var nextCursorOut *string
	if encoded := entity.EncodeFeedCursor(result.NextCursor); encoded != "" {
		nextCursorOut = &encoded
	}

	response.Success(c, gin.H{
		"data":        items,
		"next_cursor": nextCursorOut,
		"has_more":    result.HasMore,
	})
}

func loadFeedContentResourceProjections(
	ctx context.Context,
	database *db.DB,
	viewerID uuid.UUID,
	items []*entity.FeedItem,
) (map[uuid.UUID]*contentApp.ContentResourceProjection, error) {
	if database == nil || len(items) == 0 {
		return map[uuid.UUID]*contentApp.ContentResourceProjection{}, nil
	}

	contentIDs := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		if item != nil && item.ID != uuid.Nil {
			contentIDs = append(contentIDs, item.ID)
		}
	}
	if len(contentIDs) == 0 {
		return map[uuid.UUID]*contentApp.ContentResourceProjection{}, nil
	}

	resolver := contentApp.NewContentResourceProjectionResolver()
	var projections map[uuid.UUID]*contentApp.ContentResourceProjection
	err := database.WithTx(ctx, func(tx db.Tx) error {
		var err error
		projections, err = resolver.ResolveContentResourceProjections(ctx, tx, viewerID, contentIDs)
		return err
	})
	if err != nil {
		return nil, err
	}
	return projections, nil
}
