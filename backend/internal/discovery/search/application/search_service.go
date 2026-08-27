package application

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/discovery/search/repository"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/pkg/db"
)

// ErrViewerContextRequired is returned by service methods that require a
// non-nil ViewerContext per docs/03-architecture/viewer-context-contract.md
// §2.1 / §8.1. Reaching this error indicates a caller-side construction
// defect — the handler did not construct AnonymousViewer or
// AuthenticatedViewer at the HTTP boundary.
var ErrViewerContextRequired = errors.New("search: viewer context is required (viewer-context-contract.md §2.1)")

// SearchService handles search operations across domains.
// Queries existing domains without duplicating data.
type SearchService struct {
	repo repository.SearchRepository
}

// NewSearchService creates a new SearchService.
func NewSearchService(repo repository.SearchRepository) *SearchService {
	return &SearchService{
		repo: repo,
	}
}

// ============================================================================
// SEARCH HISTORY
// ============================================================================

// RecordSearch records a search query for a user.
// Maintains only the last 20 searches per user.
func (s *SearchService) RecordSearch(ctx context.Context, tx db.Tx, userID uuid.UUID, query string) error {
	if query == "" {
		return nil
	}

	// Add new search history entry
	if err := s.repo.AddSearchHistory(ctx, tx, userID, query); err != nil {
		return err
	}

	// Trim to max 20 entries (delete oldest if超过)
	return s.repo.TrimSearchHistory(ctx, tx, userID)
}

// GetSearchHistory retrieves search history for a user.
func (s *SearchService) GetSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID, limit int) ([]*entity.SearchHistory, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > entity.MaxSearchHistory {
		limit = entity.MaxSearchHistory
	}
	return s.repo.GetSearchHistory(ctx, tx, userID, limit)
}

// ClearSearchHistory deletes all search history entries for a user.
func (s *SearchService) ClearSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID) error {
	return s.repo.ClearSearchHistory(ctx, tx, userID)
}

// DeleteSearchHistory deletes a specific search history entry.
func (s *SearchService) DeleteSearchHistory(ctx context.Context, tx db.Tx, id uuid.UUID, userID uuid.UUID) error {
	return s.repo.DeleteSearchHistory(ctx, tx, id, userID)
}

// ============================================================================
// FOR SALE SEARCH
// ============================================================================

// SearchForSales performs full-text search on forSales.
// Matches: title, description, koi variety, seller name
func (s *SearchService) SearchForSales(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.ForSalePreview, int, error) {
	// Set defaults
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Limit > 50 {
		filters.Limit = 50
	}
	if filters.SortBy == "" {
		filters.SortBy = "relevance"
	}
	if filters.SortDir == "" {
		filters.SortDir = "desc"
	}

	return s.repo.SearchForSales(ctx, tx, filters)
}

// ============================================================================
// CONTENT SEARCH
// ============================================================================

// SearchContent performs full-text search on content.
// Matches: caption, text, hashtags
//
// ViewerContext propagation (Pattern A — Public Discovery):
// Per docs/03-architecture/viewer-context-contract.md §6 and
// docs/05-rollout/search-content-viewercontext-runtime-threading-task-
// design.md, this method accepts the canonical ViewerContext as input.
// The viewer is non-nil per viewer-context-contract.md §2.1; AnonymousViewer
// is an explicit, named state per §3.1 — nil viewer fallback is forbidden
// per §8.1.
//
// The service does NOT consume the ViewerContext today (no precedence path,
// no evaluator entry, no SQL gating). It threads the canonical input
// downstream so that future material tasks (the search shadow seam runtime
// landing per docs/05-rollout/search-shadow-seam-landing-task-design.md
// Sequence A second-half) can observe the propagation without per-task
// signature surgery. The repository signature remains unchanged per
// docs/05-rollout/search-content-viewercontext-runtime-threading-task-
// design.md §6.4 — repository must not hydrate.
//
// Other search service methods (SearchForSales, SearchUsers, SearchAuctions)
// retain their pre-threading signatures; their threading is each a separately
// authorized future material task per docs/05-rollout/search-content-
// viewercontext-runtime-threading-task-design.md §1.2.
func (s *SearchService) SearchContent(ctx context.Context, tx db.Tx, vc *viewercontext.ViewerContext, filters entity.SearchFilters) ([]*entity.ContentPreview, int, error) {
	if vc == nil {
		// Per viewer-context-contract.md §8.1, nil viewer fallback is
		// forbidden. The handler must construct ViewerContext at the HTTP
		// boundary (AuthenticatedViewer or AnonymousViewer). Reaching this
		// branch indicates a caller-side construction defect; we do NOT
		// silently synthesize a fallback.
		return nil, 0, ErrViewerContextRequired
	}

	// Set defaults
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Limit > 50 {
		filters.Limit = 50
	}
	if filters.SortBy == "" {
		filters.SortBy = "relevance"
	}
	if filters.SortDir == "" {
		filters.SortDir = "desc"
	}

	return s.repo.SearchContent(ctx, tx, filters)
}

// ============================================================================
// USER SEARCH
// ============================================================================

// SearchUsers performs full-text search on users.
// Matches: username
func (s *SearchService) SearchUsers(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.UserPreview, int, error) {
	// Set defaults
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Limit > 50 {
		filters.Limit = 50
	}

	return s.repo.SearchUsers(ctx, tx, filters)
}

// ============================================================================
// AUCTION SEARCH
// ============================================================================

// SearchAuctions performs full-text search on auctions.
// AUCTION SEARCH ELIGIBILITY (Phase 3.5):
// Only searches auctions with status IN ('scheduled', 'active', 'ended')
// Matches: title, description
func (s *SearchService) SearchAuctions(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.AuctionPreview, int, error) {
	// Set defaults
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Limit > 50 {
		filters.Limit = 50
	}
	if filters.SortBy == "" {
		filters.SortBy = "relevance"
	}
	if filters.SortDir == "" {
		filters.SortDir = "desc"
	}

	return s.repo.SearchAuctions(ctx, tx, filters)
}
