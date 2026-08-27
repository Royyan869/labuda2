package http

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/labuda/backend/internal/governance/viewercontext"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
)

// F1-W3A — handler-boundary hydrator unit tests.
//
// These tests exercise the new caller-batched hydration helpers added
// in F1-W3A:
//
//   - constructFeedViewerContext (now hydrates viewer lifecycle inline
//     via the tx parameter)
//   - hydrateFeedTargetContext
//   - hydrateFeedRelationship
//
// DB-dependent paths are exercised in the integration suite
// (feed_repository_test.go); these unit tests pin the nil-tx /
// empty-input / anonymous-viewer behavior so the failure-mode
// contract is locked.

func TestHydrateFeedTargetContext_EmptyInputReturnsHydratedEmpty(t *testing.T) {
	tc := hydrateFeedTargetContext(context.Background(), nil, nil)
	if tc == nil {
		t.Fatal("expected non-nil TC")
	}
	if !tc.AuthorLifecycleHydrated() {
		t.Error("empty-input TC must report AuthorLifecycle hydrated (empty)")
	}
	if !tc.ContentModerationHydrated() {
		t.Error("empty-input TC must report ContentModeration hydrated (empty)")
	}
}

func TestHydrateFeedTargetContext_NilTxReturnsHydratedEmpty(t *testing.T) {
	items := []*feedentity.FeedItem{
		{ID: uuid.New(), AuthorID: uuid.New(), Status: "active"},
	}
	tc := hydrateFeedTargetContext(context.Background(), nil, items)
	// Nil tx → batchHydrate* return empty maps; TC.With* still flags
	// the overlays as hydrated (empty maps, not unhydrated). Per-row
	// lookups return (zero, false) which the evaluator surfaces as
	// UNKNOWN/target_overlay_missing — the canonical fail-OPEN path.
	if !tc.AuthorLifecycleHydrated() {
		t.Error("AuthorLifecycleHydrated should be true (empty map) even when tx is nil")
	}
	if _, ok := tc.AuthorLifecycle(items[0].AuthorID); ok {
		t.Error("per-author lookup must be (_, false) when no DB was hit")
	}
}

func TestHydrateFeedRelationship_AnonymousReturnsVCUnchanged(t *testing.T) {
	anon := viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST)
	items := []*feedentity.FeedItem{{ID: uuid.New(), AuthorID: uuid.New(), Status: "active"}}
	got := hydrateFeedRelationship(context.Background(), nil, anon, items)
	if got != anon {
		t.Error("anonymous VC must be returned unchanged (relationship is topology-empty)")
	}
}

func TestHydrateFeedRelationship_NilVCReturnsNil(t *testing.T) {
	if got := hydrateFeedRelationship(context.Background(), nil, nil, nil); got != nil {
		t.Errorf("nil VC must return nil; got %v", got)
	}
}

func TestHydrateFeedRelationship_EmptyAuthorsAttachesEmptyHydratedSet(t *testing.T) {
	identity := viewercontext.IdentityOverlay{CanonicalUserID: uuid.New()}
	vc := viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST,
		identity,
		viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, true),
		viewercontext.CapabilityOverlay{}, viewercontext.ModerationOverlay{},
	)
	got := hydrateFeedRelationship(context.Background(), nil, vc, nil)
	if !got.Relationship().IsHydrated() {
		t.Error("empty-author hydration must yield IsHydrated()=true")
	}
}

func TestHydrateFeedViewerLifecycle_NilTxReturnsUnhydrated(t *testing.T) {
	lc := hydrateFeedViewerLifecycle(context.Background(), nil, uuid.New())
	if lc.IsHydrated() {
		t.Error("nil tx must yield hydrated=false; the evaluator + feed adapter then fail-OPEN")
	}
}

func TestConstructFeedViewerContext_W3A_AuthenticatedWithNilTxStillHasIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/feed", nil)
	uid := uuid.New()
	c.Set("user_id", uid)
	vc := constructFeedViewerContext(c, nil)
	if vc.IsAnonymous() {
		t.Fatal("auth path must yield AuthenticatedViewer even when tx is nil")
	}
	if vc.Identity().CanonicalUserID != uid {
		t.Errorf("CanonicalUserID = %v, want %v", vc.Identity().CanonicalUserID, uid)
	}
	if vc.Lifecycle().IsHydrated() {
		t.Error("nil-tx hydration must leave lifecycle unhydrated (fail-OPEN trigger)")
	}
}

// TestFeedEvaluator_NoPgxpoolImport pins F4 closure — neither
// feed_shadow.go nor feed_enforce.go may import pgxpool after the
// F1-W3A rebuild. The check reads the source bytes directly; a
// future regression that re-introduces the import will fail this
// test loudly.
func TestFeedEvaluator_NoPgxpoolImport(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("cannot resolve test file path")
	}
	// thisFile = backend/internal/social/feed/delivery/http/feed_viewercontext_w3a_test.go
	// Go up four levels (http → delivery → feed → social) to reach
	// backend/internal/, then descend into governance/evaluator.
	internalDir := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))
	evaluatorDir := filepath.Join(internalDir, "governance", "evaluator")

	guarded := []string{"feed_shadow.go", "feed_enforce.go"}
	const forbidden = "github.com/jackc/pgx/v5/pgxpool"

	for _, name := range guarded {
		path := filepath.Join(evaluatorDir, name)
		bytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(bytes), forbidden) {
			t.Errorf("F1-W3A regression: %s imports %q; evaluator package must hold no DB pool", name, forbidden)
		}
	}
}


