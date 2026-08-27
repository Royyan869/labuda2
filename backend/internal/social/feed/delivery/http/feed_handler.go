package http

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/response"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	contententity "github.com/labuda/backend/internal/social/content/entity"
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

// NewFeedHandler creates a new FeedHandler.
//
// shadowRunner is optional. When non-nil, the handler dispatches a
// fire-and-forget feed evaluator shadow run after each request — strictly
// observability-only per docs/03-architecture/viewer-context-contract.md
// (Pattern A) and docs/05-rollout/convergence-sequencing-addendum-viewercontext-evaluator.md
// (feed-first SHADOW; not feed-first authority). The shadow path never
// modifies the response.
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

	// F1-W3A — Pattern A ViewerContext construction at the HTTP
	// boundary. The pre-tx invocation passes nil tx for the cheap
	// anonymous-reject path; the post-tx re-invocation (inside the
	// transaction below) wires inline viewer-lifecycle hydration. The
	// shape mirrors /search/content's two-phase derivation
	// (constructSearchContentViewerContext inside WithTx) — see
	// search_viewercontext.go:46. F8 closed: raw c.Get("userID") is
	// no longer the visibility authority; the explicit AnonymousViewer
	// check at the boundary is.
	vc := constructFeedViewerContext(c, nil)
	if vc.IsAnonymous() {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	callerID := vc.Identity().CanonicalUserID
	if callerID == uuid.Nil {
		// Defensive: constructFeedViewerContext returns AnonymousViewer
		// when the UUID is nil, so reaching this branch indicates a
		// constructor bug rather than a missing-auth condition.
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

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
		var err error
		result, err = h.feedService.GetFeed(ctx, tx, callerID, cursor, limit)
		if err != nil {
			return err
		}
		tc = hydrateFeedTargetContext(ctx, tx, result.Items)
		vc = hydrateFeedRelationship(ctx, tx, vc, result.Items)
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
	// In FeedEvaluatorModeEnforce the handler runs EvaluateFeedItem +
	// AdaptFeedDecision over the legacy SQL result. C1 convergence: the
	// adapter coarsens TOMBSTONE → "removed" and REDACT → "unavailable"
	// into a LifecycleOverrides map (mirror of /search/content's
	// enforcement.LifecycleOverrides). DENY rows drop; UNKNOWN rows fail
	// OPEN (kept). The mode is read off the runner (nil receiver →
	// shadow), so a disabled runner skips enforce entirely.
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
	var lifecycleOverrides map[uuid.UUID]string
	if h.shadowRunner.Mode() == evaluator.FeedEvaluatorModeEnforce {
		enf := evaluator.EnforceFeed(
			evaluator.FeedEvaluatorModeEnforce,
			vc,
			tc,
			originalItems,
		)
		result.Items = enf.Filtered
		lifecycleOverrides = enf.LifecycleOverrides
	}

	// Convert feed items to response format. lifecycleOverrides is nil in
	// shadow mode and when no row took the override path; the renderer
	// short-circuits cleanly in both cases.
	projections := make(map[uuid.UUID]*contentApp.ContentResourceProjection)
	if len(result.Items) > 0 {
		if loaded, projErr := loadFeedContentResourceProjections(ctx, h.db, vc.Identity().CanonicalUserID, result.Items); projErr == nil {
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
		items[i] = resp
	}

	// PHASE C / F1-W3A — Feed evaluator shadow observability. Fire-
	// and-forget. The runner is the caller (ViewerContext Contract §5.1)
	// and never affects the response, pagination, or legacy authority.
	// nil-safe: when shadow is disabled, this is a no-op.
	//
	// IMPORTANT (Batch 3M): passes the ORIGINAL pre-filter slice so
	// shadow divergence cells stay denominator-consistent under both
	// FEED_EVALUATOR_MODE=shadow and =enforce. F1-W3A: also passes the
	// pre-hydrated (vc, tc) so the goroutine no longer touches the DB.
	h.shadowRunner.Run(vc, tc, originalItems)

	// P3A — Promotion injection. Fetch active promoted items, hydrate
	// card data, and interleave into the organic feed at slot positions.
	// FAIL-OPEN: if anything errors, items stays unchanged.
	items = h.promotionInjector.InjectPromotions(ctx, items)

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

// feedItemToResponse converts a FeedItem entity to API response format.
// MEDIA INTEGRATION: Includes media array from FeedItem.
// POST LOCATION: Includes location for posts only.
//
// C1 — lifecycleOverrides is the evaluator-enforcement override map (keyed
// by FeedItem.ID). When non-nil and the row has an entry, the emitted
// `lifecycle` top-level key AND the nested `card.lifecycle` are coarsened
// to that value (canonical vocabulary: "active" / "unavailable" / "removed").
// Pass nil for shadow-mode callers; the renderer short-circuits cleanly.
//
// FIX-3 — origAuthorLifecycles maps original_author_id → coarsened lifecycle
// string for reposts. Emitted as `original_author_lifecycle` when non-empty.
// Pass nil to skip (non-repost rows are unaffected regardless).
func feedItemToResponse(item *entity.FeedItem, lifecycleOverrides map[uuid.UUID]string, origAuthorLifecycles map[uuid.UUID]string) map[string]interface{} {
	// PUBLIC BOUNDARY:
	//   - `status` is COARSENED into the public lifecycle vocabulary
	//     (active / unavailable / removed). The raw internal enum
	//     (active / fulfilled / deleted) is never emitted.
	//   - `is_hidden` is intentionally NOT emitted. Hidden items must be
	//     filtered upstream of this handler; the moderation flag does not
	//     cross the boundary.
	//
	// Lifecycle precedence (C1):
	//   1. evaluator override (lifecycleOverrides[item.ID]) — when present,
	//      reflects the governance decision (TOMBSTONE → "removed",
	//      REDACT → "unavailable"). Wins.
	//   2. status-derived public lifecycle — the default coarsening of
	//      item.Status (active / fulfilled / deleted → active / unavailable
	//      / removed). Used when no override is present.
	cardLifecycle := contententity.PublicLifecycleFromString(item.Status)
	if lifecycleOverrides != nil {
		if v, ok := lifecycleOverrides[item.ID]; ok && v != "" {
			cardLifecycle = v
		}
	}

	resp := map[string]interface{}{
		"id":        item.ID.String(),
		"author_id": item.AuthorID.String(),
		"type":      item.Type,
		"status":    contententity.PublicLifecycleFromString(item.Status),
		// Top-level canonical lifecycle key — mirror of card.lifecycle so
		// mobile DTOs that already read `json['lifecycle']` (see
		// apps/mobile/lib/features/home/data/dto/feed_dto.dart) receive the
		// governance decision without a card-nested traversal. The vocabulary
		// is identical to ContentCard.Lifecycle.
		"lifecycle":  cardLifecycle,
		"body":       item.Body,
		"created_at": item.CreatedAt.Format(time.RFC3339),
		"updated_at": item.UpdatedAt.Format(time.RFC3339),
		// MEDIA INTEGRATION: Always include media array (may be empty)
		"media": item.Media,
	}

	// Add optional fields.
	// SCHEMA ALIGNMENT (Batch 3J): `title` is no longer emitted — the
	// canonical contents table has no title column and the FeedItem
	// entity no longer carries one. Mobile DTO marks `title` nullable,
	// so the missing key remains contract-compatible.
	if item.Caption != nil {
		resp["caption"] = *item.Caption
	}
	if item.AuthorUsername != nil {
		resp["author_username"] = *item.AuthorUsername
	}
	if item.AuthorAvatar != nil {
		resp["author_avatar"] = *item.AuthorAvatar
	}

	// Canonical PublicCard author (Batch 2B — feed converged onto publiccard.UserCard).
	// AuthorUsername / AuthorAvatar are sourced from user_profiles via the
	// feed JOIN — the card reuses those already-hydrated pointers without a
	// second DB hit. DisplayName stays nil; feed does not hydrate full_name
	// (NEVER per public-card-boundary doctrine).
	//
	// E2 — Lifecycle activation (feed-only). item.AuthorLifecycle carries
	// the canonical public coarsening of users.account_status +
	// users.deleted_at, materialized in the repository layer via
	// viewercontext.CoarsenLifecycle. Empty string degrades to a nil
	// Lifecycle (rollback-safe legacy emission); non-empty values flow
	// into the wire as the canonical {active, unavailable, removed}
	// vocabulary. Feed remains the SOLE surface wired through this seam
	// per E2 doctrine — other publiccard.UserCard call sites continue to
	// use publiccard.New and emit nil Lifecycle.
	authorUsername := ""
	if item.AuthorUsername != nil {
		authorUsername = *item.AuthorUsername
	}
	authorCard := publiccard.NewWithLifecycle(
		item.AuthorID,
		authorUsername,
		item.AuthorAvatar,
		item.AuthorLifecycle,
	)
	resp["author"] = authorCard

	// Canonical PublicCard ContentCard (Batch 2D — additive).
	// Built from already-hydrated feed-item fields; no extra DB hit.
	// Caption / media / lifecycle / author identity all reuse the surface's
	// existing truth. Commerce-card hydration is out of scope this batch.
	var captionPtr *string
	if item.Caption != nil && *item.Caption != "" {
		c := *item.Caption
		captionPtr = &c
	}
	var feedMediaURLs []string
	for _, m := range item.Media {
		if m.URL != "" {
			feedMediaURLs = append(feedMediaURLs, m.URL)
		}
	}
	contentCard := publiccard.NewContentCard(
		item.ID,
		item.Type,
		captionPtr,
		feedMediaURLs,
		cardLifecycle,
		item.CreatedAt,
		&authorCard,
	)
	resp["card"] = contentCard

	// AUTHOR LOCATION: Include author city/province if present
	if item.AuthorCity != nil || item.AuthorProvince != nil {
		city := ""
		province := ""
		if item.AuthorCity != nil {
			city = *item.AuthorCity
		}
		if item.AuthorProvince != nil {
			province = *item.AuthorProvince
		}
		resp["author_city"] = city
		resp["author_province"] = province
	}

	// CONTENT LOCATION HARDENED: Include location only when non-empty values exist.
	hasCity := hasNonEmptyValue(item.City)
	hasProvince := hasNonEmptyValue(item.Province)

	if hasCity || hasProvince {
		resp["location"] = map[string]interface{}{
			"city":     derefOrEmpty(item.City),
			"province": derefOrEmpty(item.Province),
		}
	}

	// SHARE CONTRACT V1: Include share fields if this is a repost
	if item.OriginalAuthorID != nil {
		resp["original_author_id"] = item.OriginalAuthorID.String()
		// FIX-3 — emit original author lifecycle so mobile can degrade
		// attribution display when the original author is unavailable/removed.
		if origAuthorLifecycles != nil {
			if lc, ok := origAuthorLifecycles[*item.OriginalAuthorID]; ok && lc != "" {
				resp["original_author_lifecycle"] = lc
			}
		}
	}
	return resp
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
