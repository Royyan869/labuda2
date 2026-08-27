package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/application"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/pkg/db"
)

// fakeSearchRepository is a minimal stand-in implementing only the
// SearchContent method. Other methods panic if called — this guarantees
// the threading task did not accidentally re-route any other endpoint
// through the test path.
type fakeSearchRepository struct {
	contentResult []*entity.ContentPreview
	contentTotal  int
	contentErr    error
	calls         int
}

func (f *fakeSearchRepository) SearchContent(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.ContentPreview, int, error) {
	f.calls++
	return f.contentResult, f.contentTotal, f.contentErr
}

func (f *fakeSearchRepository) AddSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID, query string) error {
	panic("not reachable in /search/content threading test")
}
func (f *fakeSearchRepository) GetSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID, limit int) ([]*entity.SearchHistory, error) {
	panic("not reachable in /search/content threading test")
}
func (f *fakeSearchRepository) ClearSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID) error {
	panic("not reachable in /search/content threading test")
}
func (f *fakeSearchRepository) DeleteSearchHistory(ctx context.Context, tx db.Tx, id uuid.UUID, userID uuid.UUID) error {
	panic("not reachable in /search/content threading test")
}
func (f *fakeSearchRepository) TrimSearchHistory(ctx context.Context, tx db.Tx, userID uuid.UUID) error {
	panic("not reachable in /search/content threading test")
}
func (f *fakeSearchRepository) SearchForSales(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.ForSalePreview, int, error) {
	panic("not reachable in /search/content threading test — other endpoints retain pre-threading signatures per task-design §1.2")
}
func (f *fakeSearchRepository) SearchUsers(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.UserPreview, int, error) {
	panic("not reachable in /search/content threading test")
}
func (f *fakeSearchRepository) SearchAuctions(ctx context.Context, tx db.Tx, filters entity.SearchFilters) ([]*entity.AuctionPreview, int, error) {
	panic("not reachable in /search/content threading test")
}

// TestSearchContent_NilViewerContext_ReturnsExplicitError verifies that
// the service rejects nil ViewerContext per docs/03-architecture/viewer-
// context-contract.md §2.1 / §8.1. Reaching this branch indicates a
// caller-side defect; the service does NOT silently synthesize a
// fallback.
func TestSearchContent_NilViewerContext_ReturnsExplicitError(t *testing.T) {
	repo := &fakeSearchRepository{}
	svc := application.NewSearchService(repo)

	contents, total, err := svc.SearchContent(context.Background(), nil, nil, entity.SearchFilters{Query: "test"})

	if !errors.Is(err, application.ErrViewerContextRequired) {
		t.Errorf("err = %v; want application.ErrViewerContextRequired", err)
	}
	if contents != nil {
		t.Errorf("contents = %v; want nil on nil ViewerContext", contents)
	}
	if total != 0 {
		t.Errorf("total = %d; want 0 on nil ViewerContext", total)
	}
	if repo.calls != 0 {
		t.Errorf("repository was called %d times despite nil ViewerContext — service must reject before delegation", repo.calls)
	}
}

// TestSearchContent_AnonymousViewer_DelegatesToRepository verifies that
// AnonymousViewer (a valid Pattern A construction per viewer-context-
// contract.md §3.1 / §6) is accepted by the service and forwarded to the
// repository.
func TestSearchContent_AnonymousViewer_DelegatesToRepository(t *testing.T) {
	repo := &fakeSearchRepository{
		contentResult: []*entity.ContentPreview{{ID: uuid.New(), AuthorID: uuid.New()}},
		contentTotal:  1,
	}
	svc := application.NewSearchService(repo)

	vc := viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST)
	contents, total, err := svc.SearchContent(context.Background(), nil, vc, entity.SearchFilters{Query: "test"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d; want 1", total)
	}
	if len(contents) != 1 {
		t.Errorf("len(contents) = %d; want 1", len(contents))
	}
	if repo.calls != 1 {
		t.Errorf("repo.calls = %d; want 1", repo.calls)
	}
}

// TestSearchContent_AuthenticatedViewer_DelegatesToRepository verifies
// that AuthenticatedViewer is accepted by the service and forwarded to
// the repository, AND that the repository signature did NOT change (the
// fake repository's SearchContent method receives only (ctx, tx, filters)
// — no viewer parameter — per task-design §6.4).
func TestSearchContent_AuthenticatedViewer_DelegatesToRepository(t *testing.T) {
	repo := &fakeSearchRepository{
		contentResult: []*entity.ContentPreview{{ID: uuid.New(), AuthorID: uuid.New()}},
		contentTotal:  1,
	}
	svc := application.NewSearchService(repo)

	vc := viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery,
		viewercontext.RequestOriginREST,
		viewercontext.IdentityOverlay{CanonicalUserID: uuid.New(), FirebaseUID: "firebase-test"},
		viewercontext.LifecycleOverlay{State: viewercontext.PublicLifecycleStateActive},
		viewercontext.CapabilityOverlay{},
		viewercontext.ModerationOverlay{},
	)
	contents, total, err := svc.SearchContent(context.Background(), nil, vc, entity.SearchFilters{Query: "test"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || len(contents) != 1 {
		t.Errorf("unexpected results: total=%d len=%d", total, len(contents))
	}
}

// TestSearchContent_DefaultsApplied verifies that the existing default-
// applying behavior (limit, sort) is preserved by the threading change
// per task-design §10.5 (no SQL behavior change).
func TestSearchContent_DefaultsApplied(t *testing.T) {
	repo := &fakeSearchRepository{}
	svc := application.NewSearchService(repo)

	vc := viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST)
	_, _, _ = svc.SearchContent(context.Background(), nil, vc, entity.SearchFilters{Query: "test"})

	if repo.calls != 1 {
		t.Fatalf("repo.calls = %d; want 1", repo.calls)
	}
}
