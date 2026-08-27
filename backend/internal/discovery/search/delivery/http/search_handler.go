package http

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	searchApp "github.com/labuda/backend/internal/discovery/search/application"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/pkg/blockcheck"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
	"github.com/labuda/backend/internal/platform/response"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// SearchHandler handles HTTP requests for search operations.
type SearchHandler struct {
	searchService *searchApp.SearchService
	db            *db.DB
	log           *zap.Logger

	// searchContentShadowRunner is the Stage 1 shadow telemetry seam for
	// /search/content per docs/05-rollout/search-shadow-seam-landing-
	// task-design.md §3.1 / §4.2. Per §10.1 / §10.7, no feature flag
	// controls seam emission; the runner is unconditionally constructed
	// in dependencies_core.go and unconditionally dispatched after the
	// legacy /search/content response is committed. A nil value disables
	// the seam (no-op via SearchContentShadowRunner.Run nil receiver).
	searchContentShadowRunner *evaluator.SearchContentShadowRunner

	// P3B — Optional promotion injector. When non-nil, appends a
	// promoted_items sidecar to forSale and auction search responses.
	// Nil disables injection entirely.
	promotionInjector *SearchPromotionInjector
}

// NewSearchHandler creates a new SearchHandler.
//
// searchContentShadowRunner: the Stage 1 shadow telemetry seam runner
// for /search/content. Per docs/05-rollout/search-shadow-seam-landing-
// task-design.md, the runner is fire-and-forget; it never affects the
// response, pagination, or legacy authority. Pass nil to disable the
// seam (the runner's Run method is nil-safe).
func NewSearchHandler(
	searchService *searchApp.SearchService,
	database *db.DB,
	log *zap.Logger,
	searchContentShadowRunner *evaluator.SearchContentShadowRunner,
	promotionInjector *SearchPromotionInjector,
) *SearchHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &SearchHandler{
		searchService:             searchService,
		db:                        database,
		log:                       log,
		searchContentShadowRunner: searchContentShadowRunner,
		promotionInjector:         promotionInjector,
	}
}

// ============================================================================
// FOR SALE SEARCH
// ============================================================================

// SearchForSalesRequest holds query parameters for forSale search.
type SearchForSalesRequest struct {
	Query   string `form:"q" binding:"required"`
	Limit   int    `form:"limit" binding:"omitempty,min=1,max=50"`
	Offset  int    `form:"offset" binding:"omitempty,min=0"`
	SortBy  string `form:"sort" binding:"omitempty,oneof=relevance created_at"`
	SortDir string `form:"sort_dir" binding:"omitempty,oneof=asc desc"`
}

// SearchForSales handles GET /api/v1/search/forSales?q=
func (h *SearchHandler) SearchForSales(c *gin.Context) {
	ctx := c.Request.Context()

	var req SearchForSalesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	filters := entity.SearchFilters{
		Query:   req.Query,
		Limit:   req.Limit,
		Offset:  req.Offset,
		SortBy:  req.SortBy,
		SortDir: req.SortDir,
	}

	// Extract viewer ID for block filtering
	var viewerID uuid.UUID
	if userIDVal, exists := c.Get("userID"); exists {
		if id, ok := userIDVal.(uuid.UUID); ok {
			viewerID = id
		}
	}

	var forSales []*entity.ForSalePreview
	var total int
	// E8.1 — Seller user-identity lifecycle batched inside the same tx;
	// rollback-safe on hydration error (empty map → nil lifecycle on wire).
	var sellerLifecycle map[uuid.UUID]string

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		forSales, total, err = h.searchService.SearchForSales(ctx, tx, filters)
		if err != nil {
			return err
		}

		// Block enforcement: remove forSales from blocked sellers
		if viewerID != uuid.Nil {
			sellerIDs := make([]uuid.UUID, 0, len(forSales))
			for _, l := range forSales {
				sellerIDs = append(sellerIDs, l.SellerID)
			}
			blockedSet, _ := blockcheck.BlockedSet(ctx, tx, viewerID, sellerIDs)
			if len(blockedSet) > 0 {
				filtered := make([]*entity.ForSalePreview, 0, len(forSales))
				for _, l := range forSales {
					if !blockedSet[l.SellerID] {
						filtered = append(filtered, l)
					}
				}
				forSales = filtered
				total = len(forSales)
			}
		}

		sellerLifecycle = hydrateSellerUserLifecycleFromForSales(ctx, tx, forSales)
		return nil
	})

	if err != nil {
		h.log.Error("Failed to search forSales", zap.Error(err))
		response.InternalServerError(c, "Failed to search forSales")
		return
	}

	// P3B — Build promoted sidecar for forSale search.
	organicIDs := make([]uuid.UUID, 0, len(forSales))
	organicSellerIDs := make([]uuid.UUID, 0, len(forSales))
	for _, l := range forSales {
		organicIDs = append(organicIDs, l.ID)
		organicSellerIDs = append(organicSellerIDs, l.SellerID)
	}
	promotedSidecar := h.promotionInjector.GetPromotedSidecar(ctx, organicIDs, organicSellerIDs)

	responseData := gin.H{
		"query":    req.Query,
		"forSales": forSalePreviewsToResponse(forSales, sellerLifecycle),
		"total":    total,
		"limit":    req.Limit,
		"offset":   req.Offset,
	}
	if len(promotedSidecar) > 0 {
		responseData["promoted_items"] = promotedSidecar
	}
	response.Success(c, responseData)
}

// ============================================================================
// CONTENT SEARCH
// ============================================================================

// SearchContentRequest holds query parameters for content search.
type SearchContentRequest struct {
	Query   string `form:"q" binding:"required"`
	Limit   int    `form:"limit" binding:"omitempty,min=1,max=50"`
	Offset  int    `form:"offset" binding:"omitempty,min=0"`
	SortBy  string `form:"sort" binding:"omitempty,oneof=relevance created_at"`
	SortDir string `form:"sort_dir" binding:"omitempty,oneof=asc desc"`
}

// SearchContent handles GET /api/v1/search/content?q=
//
// Pattern A — Public Discovery ViewerContext propagation (per
// docs/03-architecture/viewer-context-contract.md §6 and
// docs/05-rollout/search-content-viewercontext-runtime-threading-task-
// design.md):
//
//   - ViewerContext is constructed at the HTTP boundary
//     (constructSearchContentViewerContext): AuthenticatedViewer if
//     user_id is present and resolvable, AnonymousViewer otherwise.
//   - Viewer-side overlays (identity, viewer_lifecycle, capability,
//     moderation) are hydrated at construction time from middleware-
//     resolved values per task-design §6.1.
//   - Target-side overlays (target_lifecycle for per-row author,
//     target_moderation for per-row content, relationship for viewer ×
//     per-row author) are hydrated caller-batched-after-candidate-set
//     per task-design §6.2 — caller hydrates truth, repository does NOT
//     hydrate, evaluator does NOT fetch.
//   - The ViewerContext + TargetContext are held for the request lifetime
//     and discarded after response. The future search shadow seam runtime
//     landing material task (Sequence A second-half per docs/05-rollout/
//     search-shadow-seam-landing-task-design.md §6.4) will consume them
//     to emit per-endpoint divergence / overlay completeness telemetry;
//     this threading task does NOT register telemetry per task-design
//     §7.2.
//   - The legacy gin.H response shape and SQL filter behavior are
//     byte-for-byte preserved per task-design §7.6 / §10.2 / §10.5,
//     EXCEPT for the additive `has_more` field landed in Batch 3E (see
//     searchContentHasMore docstring + the inline contract comment near
//     the response.Success call). The legacy fields (query / contents /
//     total / limit / offset) are unchanged in name, type, and meaning.
func (h *SearchHandler) SearchContent(c *gin.Context) {
	ctx := c.Request.Context()

	var req SearchContentRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	filters := entity.SearchFilters{
		Query:   req.Query,
		Limit:   req.Limit,
		Offset:  req.Offset,
		SortBy:  req.SortBy,
		SortDir: req.SortDir,
	}

	// vc is declared before WithTx so it remains accessible after the
	// transaction closes (shadow runner dispatch + response write).
	var vc *viewercontext.ViewerContext
	var contents []*entity.ContentPreview
	var total int
	var targetCtx *viewercontext.TargetContext
	// E9.1 — author lifecycle map built inside WithTx; used after tx closes.
	var authorLifecycleByID map[uuid.UUID]string
	var resourceProjections map[uuid.UUID]*contentApp.ContentResourceProjection

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		// Construct canonical Pattern A ViewerContext inside the transaction
		// scope so the lifecycle DB query reuses the existing tx per
		// docs/05-rollout/search-content-viewer-lifecycle-hydration-runtime-
		// task-design.md §5.2 / §5.3.
		vc = constructSearchContentViewerContext(c, tx)

		var err error
		contents, total, err = h.searchService.SearchContent(ctx, tx, vc, filters)
		if err != nil {
			return err
		}

		// Caller-batched target-side overlay hydration per task-design
		// §6.2. Hydration runs at the handler boundary; the repository
		// signature remains unchanged per §6.4.
		targetCtx = hydrateSearchContentTargetContext(ctx, tx, contents)

		// Caller-batched viewer × per-row author relationship hydration
		// per task-design §5.4. The hydrated relationship overlay is
		// attached to the ViewerContext via WithRelationship (immutability-
		// preserving copy per viewer-context-contract.md §8.3).
		vc = hydrateSearchContentRelationship(ctx, tx, vc, contents)
		// E9.1 — batch-hydrate content author lifecycle inside tx scope.
		authorLifecycleByID = hydrateContentAuthorLifecycle(ctx, tx, contents)
		resourceProjections = hydrateSearchContentResourceProjections(ctx, tx, vc, contents)

		return nil
	})

	if err != nil {
		h.log.Error("Failed to search content", zap.Error(err))
		response.InternalServerError(c, "Failed to search content")
		return
	}

	// BATCH 3B — Synchronous enforcement pass.
	//
	// In SearchContentAdapterModeShadow this is a no-op pass-through:
	// `enforcement` carries the input slice unchanged and a nil
	// LifecycleOverrides map. The response wire shape is byte-for-byte
	// identical to the pre-Batch-3B behavior. Rolling back to shadow is
	// a single env-var flip (SEARCH_CONTENT_EVALUATOR_MODE=shadow).
	//
	// In SearchContentAdapterModeEnforce the helper runs the SAME pure
	// EvaluateSearchContent decision the shadow runner observes; rows
	// adapter.Include=false are dropped, rows with a LifecycleOverride
	// have their card.Lifecycle coarsened. The legacy SQL filter at
	// search_repository_impl.go preserves projection-coupling
	// invariants (hidden/deleted physically absent), so enforcement is
	// strictly further-restrict-only — it cannot recover undershare.
	enforcement := evaluator.EnforceSearchContent(
		h.searchContentShadowRunner.Mode(),
		vc, targetCtx, contents,
	)

	// PHASE C — Search shadow seam Stage 1 dispatch (telemetry only) per
	// docs/05-rollout/search-shadow-seam-landing-task-design.md §3.1 /
	// §4.2 / §5. Fire-and-forget telemetry runs against the ORIGINAL
	// contents slice (pre-enforcement) so divergence metrics continue
	// to reflect what the legacy SQL allowed; the synchronous helper
	// above is what shapes the response.
	h.searchContentShadowRunner.Run(vc, targetCtx, contents)

	// BATCH 3E — `has_more` pagination hint.
	//
	// Client-facing semantic per /search/content response contract:
	//
	//   - `total`     : SQL pre-enforcement candidate count. Stable across
	//                   pages for a given query; can be larger than the sum
	//                   of `contents.length` across all pages once enforce
	//                   mode drops rows. NOT a "results available to viewer"
	//                   count.
	//   - `contents`  : post-enforcement visible rows for THIS page.
	//   - `has_more`  : pagination hint derived from the SQL pre-enforcement
	//                   total. True iff the next offset (`offset + limit`)
	//                   would still fall inside `total`. NOT a guarantee
	//                   that the next page contains any visible rows after
	//                   enforcement — clients MUST treat an empty `contents`
	//                   on a subsequent fetch as terminal even when this
	//                   field is true.
	//
	// Rationale for "Option 1" (pre-enforcement-only) over the alternative
	// `has_more = ... AND len(filtered) > 0`: a fully-dropped middle page
	// (every row evaluator-denied) does not imply later pages are also
	// dropped. The pre-enforcement-only form preserves the client's option
	// to keep paginating past such pages; the empty-page-stop rule is
	// applied client-side per the contract above.
	response.Success(c, gin.H{
		"query":    req.Query,
		"contents": contentPreviewsToResponseWithProjections(enforcement.Filtered, enforcement.LifecycleOverrides, authorLifecycleByID, resourceProjections),
		// `total` deliberately remains the SQL pre-enforcement count.
		// See Batch 3B "TOTAL COUNT DECISION" — shrinking total to the
		// post-filter page count would break paging semantics for
		// clients that compute next-page offsets from it.
		"total":    total,
		"limit":    req.Limit,
		"offset":   req.Offset,
		"has_more": searchContentHasMore(req.Offset, req.Limit, total),
	})
}

// searchContentHasMore is the pure derivation of the /search/content
// response `has_more` pagination hint. Pre-enforcement-only form: true
// iff a next offset would still fall inside the SQL candidate `total`.
//
// Returns false when `limit <= 0` (caller did not request pagination)
// regardless of total — emitting `has_more: true` on a no-pagination
// request would invite confused clients to fetch a non-existent page.
func searchContentHasMore(offset, limit, total int) bool {
	if limit <= 0 {
		return false
	}
	if offset < 0 {
		offset = 0
	}
	return offset+limit < total
}

// ============================================================================
// USER SEARCH
// ============================================================================

// SearchUsersRequest holds query parameters for user search.
type SearchUsersRequest struct {
	Query  string `form:"q" binding:"required"`
	Limit  int    `form:"limit" binding:"omitempty,min=1,max=50"`
	Offset int    `form:"offset" binding:"omitempty,min=0"`
}

// SearchUsers handles GET /api/v1/search/users?q=
func (h *SearchHandler) SearchUsers(c *gin.Context) {
	ctx := c.Request.Context()

	var req SearchUsersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	filters := entity.SearchFilters{
		Query:  req.Query,
		Limit:  req.Limit,
		Offset: req.Offset,
	}

	// Extract viewer ID for block filtering
	var viewerID uuid.UUID
	if userIDVal, exists := c.Get("userID"); exists {
		if id, ok := userIDVal.(uuid.UUID); ok {
			viewerID = id
		}
	}

	var users []*entity.UserPreview
	var total int

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		users, total, err = h.searchService.SearchUsers(ctx, tx, filters)
		if err != nil {
			return err
		}

		// Block enforcement: remove blocked users from results
		if viewerID != uuid.Nil {
			userIDs := make([]uuid.UUID, 0, len(users))
			for _, u := range users {
				userIDs = append(userIDs, u.ID)
			}
			blockedSet, _ := blockcheck.BlockedSet(ctx, tx, viewerID, userIDs)
			if len(blockedSet) > 0 {
				filtered := make([]*entity.UserPreview, 0, len(users))
				for _, u := range users {
					if !blockedSet[u.ID] {
						filtered = append(filtered, u)
					}
				}
				users = filtered
				total = len(users)
			}

			// Follow state: batch lookup — one query for all remaining results.
			// Fail-open: if the query fails, IsFollowedByCurrentUser remains false.
			followedSet, _ := followedByViewerSet(ctx, tx, viewerID, users)
			for _, u := range users {
				u.IsFollowedByCurrentUser = followedSet[u.ID]
			}
		}

		return nil
	})

	if err != nil {
		h.log.Error("Failed to search users", zap.Error(err))
		response.InternalServerError(c, "Failed to search users")
		return
	}

	response.Success(c, gin.H{
		"query":  req.Query,
		"users":  userPreviewsToResponse(users),
		"total":  total,
		"limit":  req.Limit,
		"offset": req.Offset,
	})
}

// ============================================================================
// AUCTION SEARCH
// ============================================================================

// SearchAuctionsRequest holds query parameters for auction search.
type SearchAuctionsRequest struct {
	Query   string `form:"q" binding:"required"`
	Limit   int    `form:"limit" binding:"omitempty,min=1,max=50"`
	Offset  int    `form:"offset" binding:"omitempty,min=0"`
	SortBy  string `form:"sort" binding:"omitempty,oneof=relevance created_at end_at"`
	SortDir string `form:"sort_dir" binding:"omitempty,oneof=asc desc"`
}

// SearchAuctions handles GET /api/v1/search/auctions?q=
//
// AUCTION SEARCH ELIGIBILITY (Phase 3.5):
// Only searches auctions with status IN ('scheduled', 'active', 'ended')
// Draft and cancelled auctions are NOT discoverable via search
//
// Search fields: title, description
// Sort options: relevance (active first, then bid count), created_at, end_at
func (h *SearchHandler) SearchAuctions(c *gin.Context) {
	ctx := c.Request.Context()

	var req SearchAuctionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	filters := entity.SearchFilters{
		Query:   req.Query,
		Limit:   req.Limit,
		Offset:  req.Offset,
		SortBy:  req.SortBy,
		SortDir: req.SortDir,
	}

	// Extract viewer ID for block filtering
	var viewerID uuid.UUID
	if userIDVal, exists := c.Get("userID"); exists {
		if id, ok := userIDVal.(uuid.UUID); ok {
			viewerID = id
		}
	}

	var auctions []*entity.AuctionPreview
	var total int
	// E8.1 — Seller user-identity lifecycle batched inside the same tx.
	var sellerLifecycle map[uuid.UUID]string

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		auctions, total, err = h.searchService.SearchAuctions(ctx, tx, filters)
		if err != nil {
			return err
		}

		// Block enforcement: remove auctions from blocked sellers
		if viewerID != uuid.Nil {
			sellerIDs := make([]uuid.UUID, 0, len(auctions))
			for _, a := range auctions {
				sellerIDs = append(sellerIDs, a.SellerID)
			}
			blockedSet, _ := blockcheck.BlockedSet(ctx, tx, viewerID, sellerIDs)
			if len(blockedSet) > 0 {
				filtered := make([]*entity.AuctionPreview, 0, len(auctions))
				for _, a := range auctions {
					if !blockedSet[a.SellerID] {
						filtered = append(filtered, a)
					}
				}
				auctions = filtered
				total = len(auctions)
			}
		}

		sellerLifecycle = hydrateSellerUserLifecycleFromAuctions(ctx, tx, auctions)
		return nil
	})

	if err != nil {
		h.log.Error("Failed to search auctions", zap.Error(err))
		response.InternalServerError(c, "Failed to search auctions")
		return
	}

	// P3B — Build promoted sidecar for auction search.
	organicAuctionIDs := make([]uuid.UUID, 0, len(auctions))
	organicAuctionSellerIDs := make([]uuid.UUID, 0, len(auctions))
	for _, a := range auctions {
		organicAuctionIDs = append(organicAuctionIDs, a.ID)
		organicAuctionSellerIDs = append(organicAuctionSellerIDs, a.SellerID)
	}
	auctionPromotedSidecar := h.promotionInjector.GetPromotedSidecar(ctx, organicAuctionIDs, organicAuctionSellerIDs)

	auctionResponseData := gin.H{
		"query":    req.Query,
		"auctions": auctionPreviewsToResponse(auctions, sellerLifecycle),
		"total":    total,
		"limit":    req.Limit,
		"offset":   req.Offset,
	}
	if len(auctionPromotedSidecar) > 0 {
		auctionResponseData["promoted_items"] = auctionPromotedSidecar
	}
	response.Success(c, auctionResponseData)
}

// ============================================================================
// SEARCH HISTORY
// ============================================================================

// GetSearchHistory handles GET /api/v1/search/history
func (h *SearchHandler) GetSearchHistory(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsedLimit := parseIntOrDefault(l, 20); parsedLimit > 0 && parsedLimit <= 50 {
			limit = parsedLimit
		}
	}

	var history []*entity.SearchHistory
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		history, err = h.searchService.GetSearchHistory(ctx, tx, userID, limit)
		return err
	})

	if err != nil {
		h.log.Error("Failed to get search history", zap.Error(err))
		response.InternalServerError(c, "Failed to get search history")
		return
	}

	response.Success(c, gin.H{
		"history": searchHistoryToResponse(history),
	})
}

// AddSearchHistoryRequest holds request body for adding search history.
type AddSearchHistoryRequest struct {
	Query string `json:"query" binding:"required"`
}

// AddSearchHistory handles POST /api/v1/search/history
func (h *SearchHandler) AddSearchHistory(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	var req AddSearchHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.searchService.RecordSearch(ctx, tx, userID, req.Query)
	})

	if err != nil {
		h.log.Error("Failed to add search history", zap.Error(err))
		response.InternalServerError(c, "Failed to add search history")
		return
	}

	response.Success(c, gin.H{
		"message": "Search history added",
	})
}

// ClearSearchHistory handles DELETE /api/v1/search/history
func (h *SearchHandler) ClearSearchHistory(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.searchService.ClearSearchHistory(ctx, tx, userID)
	})

	if err != nil {
		h.log.Error("Failed to clear search history", zap.Error(err))
		response.InternalServerError(c, "Failed to clear search history")
		return
	}

	response.Success(c, gin.H{
		"message": "Search history cleared",
	})
}

// DeleteSearchHistory handles DELETE /api/v1/search/history/:id
func (h *SearchHandler) DeleteSearchHistory(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid history ID")
		return
	}

	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.searchService.DeleteSearchHistory(ctx, tx, id, userID)
	})

	if err != nil {
		h.log.Error("Failed to delete search history", zap.Error(err))
		response.InternalServerError(c, "Failed to delete search history")
		return
	}

	response.Success(c, gin.H{
		"message": "Search history deleted",
	})
}

// ============================================================================
// RESPONSE CONVERTERS
// ============================================================================

// hydrateSellerUserLifecycleFromForSales batch-hydrates the coarsened user-
// identity lifecycle for every distinct seller across the forSale page via
// sellerdisplay.FetchMany. Returns a map keyed by seller_id; missing keys
// (or empty map on hydration error) → caller falls back to nil lifecycle.
//
// E8.1 — no N+1, no raw account_status leakage. Coarsening happens at
// viewercontext.CoarsenLifecycle (single canonical site). Top-level
// seller.lifecycle is NOT populated by this helper.
func hydrateSellerUserLifecycleFromForSales(ctx context.Context, tx db.Tx, forSales []*entity.ForSalePreview) map[uuid.UUID]string {
	out := make(map[uuid.UUID]string, len(forSales))
	if len(forSales) == 0 {
		return out
	}
	ids := make([]uuid.UUID, 0, len(forSales))
	seen := make(map[uuid.UUID]struct{}, len(forSales))
	for _, l := range forSales {
		if l.SellerID == uuid.Nil {
			continue
		}
		if _, ok := seen[l.SellerID]; ok {
			continue
		}
		seen[l.SellerID] = struct{}{}
		ids = append(ids, l.SellerID)
	}
	infos, err := sellerdisplay.FetchMany(ctx, tx, ids)
	if err != nil {
		return out
	}
	for id, info := range infos {
		out[id] = string(viewercontext.CoarsenLifecycle(info.AccountStatus, info.IsDeleted))
	}
	return out
}

// hydrateSellerUserLifecycleFromAuctions mirrors the forSales helper for
// AuctionPreview rows. Same semantics, same boundary guarantees.
func hydrateSellerUserLifecycleFromAuctions(ctx context.Context, tx db.Tx, auctions []*entity.AuctionPreview) map[uuid.UUID]string {
	out := make(map[uuid.UUID]string, len(auctions))
	if len(auctions) == 0 {
		return out
	}
	ids := make([]uuid.UUID, 0, len(auctions))
	seen := make(map[uuid.UUID]struct{}, len(auctions))
	for _, a := range auctions {
		if a.SellerID == uuid.Nil {
			continue
		}
		if _, ok := seen[a.SellerID]; ok {
			continue
		}
		seen[a.SellerID] = struct{}{}
		ids = append(ids, a.SellerID)
	}
	infos, err := sellerdisplay.FetchMany(ctx, tx, ids)
	if err != nil {
		return out
	}
	for id, info := range infos {
		out[id] = string(viewercontext.CoarsenLifecycle(info.AccountStatus, info.IsDeleted))
	}
	return out
}

// hydrateContentAuthorLifecycle batch-hydrates the coarsened user-identity
// lifecycle for every distinct content author across the content page via a
// direct users table query. Returns a map keyed by author_id; missing keys
// (or empty map on hydration error) → caller falls back to empty string,
// which NewWithLifecycle treats as nil lifecycle (rollback-safe).
//
// E9.1 — no N+1, no raw account_status leakage. Coarsening happens at
// viewercontext.CoarsenLifecycle (single canonical site).
// Slot-persistence: NO `WHERE u.deleted_at IS NULL` filter so tombstoned
// authors surface as lifecycle="removed" rather than falling through to the
// anonymous nil-lifecycle path. Mirrors E4.2 / E6 / E8.1 slot-persistence
// doctrine.
func hydrateContentAuthorLifecycle(ctx context.Context, tx db.Tx, contents []*entity.ContentPreview) map[uuid.UUID]string {
	out := make(map[uuid.UUID]string, len(contents))
	if len(contents) == 0 {
		return out
	}
	ids := make([]uuid.UUID, 0, len(contents))
	seen := make(map[uuid.UUID]struct{}, len(contents))
	for _, c := range contents {
		if c.AuthorID == uuid.Nil {
			continue
		}
		if _, ok := seen[c.AuthorID]; ok {
			continue
		}
		seen[c.AuthorID] = struct{}{}
		ids = append(ids, c.AuthorID)
	}
	if len(ids) == 0 {
		return out
	}
	rows, err := tx.Query(ctx, `
		SELECT u.id,
		       u.account_status,
		       (u.deleted_at IS NOT NULL) AS is_deleted
		FROM users u
		WHERE u.id = ANY($1)
	`, ids)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id            uuid.UUID
			accountStatus string
			isDeleted     bool
		)
		if err := rows.Scan(&id, &accountStatus, &isDeleted); err != nil {
			return out
		}
		out[id] = string(viewercontext.CoarsenLifecycle(accountStatus, isDeleted))
	}
	return out
}

// forSalePreviewsToResponse renders the /search/forSales rows.
//
// E8.1 — accepts an optional `sellerUserLifecycleByID` map keyed by
// ForSalePreview.SellerID. When the map is non-nil and contains an entry,
// the emitted ForSaleCard's nested seller.user.lifecycle is set to that
// coarsened value (canonical vocabulary: "active" / "unavailable" /
// "removed"). When the map is nil or lacks the row's key, the nested user
// lifecycle is emitted as nil (legacy / rollback-safe shape).
//
// The top-level seller.lifecycle stays nil on this surface; E8 doctrine
// reserves it for a future seller-trust/capability coarsening batch.
func forSalePreviewsToResponse(forSales []*entity.ForSalePreview, sellerUserLifecycleByID map[uuid.UUID]string) []map[string]interface{} {
	return newSearchProjectionAdapter().forSalePreviewsToResponse(forSales, sellerUserLifecycleByID)
}

// contentPreviewsToResponse renders the /search/content rows.
//
// BATCH 3B — accepts an optional `lifecycleOverrides` map keyed by
// ContentPreview.ID. When the map is non-nil and contains an entry for
// the current row, the emitted ContentCard's Lifecycle field is
// coarsened to that value (canonical vocabulary: "active" / "unavailable"
// / "removed"). When the map is nil or lacks the row's key, the card's
// Lifecycle is emitted with the surface's existing semantics (nil today
// because ContentPreview does not carry status — see line ~835).
//
// Pass nil for shadow-mode callers; pass enforcement.LifecycleOverrides
// for enforce-mode callers. The map is read-only and not mutated.
func contentPreviewsToResponse(contents []*entity.ContentPreview, lifecycleOverrides map[uuid.UUID]string, authorLifecycleByID map[uuid.UUID]string) []map[string]interface{} {
	return newSearchProjectionAdapter().contentPreviewsToResponse(contents, lifecycleOverrides, authorLifecycleByID)
}

func contentPreviewsToResponseWithProjections(
	contents []*entity.ContentPreview,
	lifecycleOverrides map[uuid.UUID]string,
	authorLifecycleByID map[uuid.UUID]string,
	projections map[uuid.UUID]*contentApp.ContentResourceProjection,
) []map[string]interface{} {
	return newSearchProjectionAdapter().contentPreviewsToResponseWithProjections(contents, lifecycleOverrides, authorLifecycleByID, projections)
}

func userPreviewsToResponse(users []*entity.UserPreview) []map[string]interface{} {
	return newSearchProjectionAdapter().userPreviewsToResponse(users)
}

func searchHistoryToResponse(history []*entity.SearchHistory) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(history))
	for _, h := range history {
		result = append(result, map[string]interface{}{
			"id":         h.ID.String(),
			"query":      h.Query,
			"created_at": h.CreatedAt.Format(time.RFC3339),
		})
	}
	return result
}

func hydrateSearchContentResourceProjections(
	ctx context.Context,
	tx db.Tx,
	vc *viewercontext.ViewerContext,
	contents []*entity.ContentPreview,
) map[uuid.UUID]*contentApp.ContentResourceProjection {
	if tx == nil || len(contents) == 0 {
		return map[uuid.UUID]*contentApp.ContentResourceProjection{}
	}

	contentIDs := make([]uuid.UUID, 0, len(contents))
	for _, content := range contents {
		if content != nil && content.ID != uuid.Nil {
			contentIDs = append(contentIDs, content.ID)
		}
	}
	if len(contentIDs) == 0 {
		return map[uuid.UUID]*contentApp.ContentResourceProjection{}
	}

	viewerID := uuid.Nil
	if vc != nil && !vc.IsAnonymous() {
		viewerID = vc.Identity().CanonicalUserID
	}

	resolver := contentApp.NewContentResourceProjectionResolver()
	projections, err := resolver.ResolveContentResourceProjections(ctx, tx, viewerID, contentIDs)
	if err != nil {
		return map[uuid.UUID]*contentApp.ContentResourceProjection{}
	}
	return projections
}

// auctionPreviewsToResponse converts auction previews to API response format.
//
// E8.1 — accepts an optional `sellerUserLifecycleByID` map keyed by
// AuctionPreview.SellerID. When the map is non-nil and contains an entry,
// the emitted AuctionCard's nested seller.user.lifecycle is set to that
// coarsened value (canonical vocabulary: "active" / "unavailable" /
// "removed"). Top-level seller.lifecycle stays nil per E8 axis-boundary
// doctrine.
func auctionPreviewsToResponse(auctions []*entity.AuctionPreview, sellerUserLifecycleByID map[uuid.UUID]string) []map[string]interface{} {
	return newSearchProjectionAdapter().auctionPreviewsToResponse(auctions, sellerUserLifecycleByID)
}

func parseIntOrDefault(s string, defaultValue int) int {
	var result int
	if _, err := fmt.Sscanf(s, "%d", &result); err != nil {
		return defaultValue
	}
	return result
}

// followedByViewerSet returns the set of user IDs (from candidates) that the
// viewer is already following. Used for batch enrichment of user search results.
//
// Pattern mirrors blockcheck.BlockedSet: one query, fail-open (empty set on error).
// Anonymous viewers (uuid.Nil) always receive an empty set.
func followedByViewerSet(ctx context.Context, tx db.Tx, viewerID uuid.UUID, candidates []*entity.UserPreview) (map[uuid.UUID]bool, error) {
	result := make(map[uuid.UUID]bool)
	if viewerID == uuid.Nil || len(candidates) == 0 {
		return result, nil
	}

	ids := make([]uuid.UUID, 0, len(candidates))
	for _, u := range candidates {
		ids = append(ids, u.ID)
	}

	rows, err := tx.Query(ctx, `
		SELECT following_id
		FROM user_follows
		WHERE follower_id = $1 AND following_id = ANY($2)
	`, viewerID, ids)
	if err != nil {
		return result, fmt.Errorf("followedByViewerSet query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return result, fmt.Errorf("followedByViewerSet scan failed: %w", err)
		}
		result[id] = true
	}
	return result, rows.Err()
}
