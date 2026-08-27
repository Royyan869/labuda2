package http

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
)

// F1-W2 — Content surface ViewerContext constructor behavior pin.
// Both /contents/:id and /users/:id/contents share buildContentSurfaceViewerContext;
// the per-symbol tests exercise the public entry points so future
// divergence between the two constructors fails loudly.

func newContentTestGinContext(t *testing.T, path string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", path, nil)
	return c
}

// ---- /contents/:id ----

func TestConstructContentDetailViewerContext_AnonymousOnMissingUserID(t *testing.T) {
	c := newContentTestGinContext(t, "/api/v1/contents/abc")
	vc := constructContentDetailViewerContext(c, nil)
	if vc == nil {
		t.Fatal("expected non-nil ViewerContext")
	}
	if !vc.IsAnonymous() {
		t.Fatal("/contents/:id is anonymous-permissive; missing userID must yield AnonymousViewer")
	}
	if vc.Identity().CanonicalUserID != uuid.Nil {
		t.Fatalf("anonymous derived userID = %v, want uuid.Nil (preserves handler semantics)", vc.Identity().CanonicalUserID)
	}
}

func TestConstructContentDetailViewerContext_AnonymousOnNilUUID(t *testing.T) {
	c := newContentTestGinContext(t, "/api/v1/contents/abc")
	c.Set("user_id", uuid.Nil)
	vc := constructContentDetailViewerContext(c, nil)
	if !vc.IsAnonymous() {
		t.Fatal("nil UUID must yield AnonymousViewer per F6")
	}
}

func TestConstructContentDetailViewerContext_AuthenticatedOnValidUUID(t *testing.T) {
	c := newContentTestGinContext(t, "/api/v1/contents/abc")
	uid := uuid.New()
	c.Set("user_id", uid)
	c.Set("is_admin", true)
	vc := constructContentDetailViewerContext(c, nil)
	if vc.IsAnonymous() {
		t.Fatal("expected AuthenticatedViewer when valid UUID in gin context")
	}
	if vc.Identity().CanonicalUserID != uid {
		t.Fatalf("CanonicalUserID = %v, want %v", vc.Identity().CanonicalUserID, uid)
	}
	if !vc.Capability().IsAdmin {
		t.Fatal("is_admin capability did not propagate")
	}
}

// ---- /users/:id/contents ----

func TestConstructUserContentViewerContext_AnonymousOnMissingUserID(t *testing.T) {
	c := newContentTestGinContext(t, "/api/v1/users/abc/contents")
	vc := constructUserContentViewerContext(c, nil)
	if !vc.IsAnonymous() {
		t.Fatal("/users/:id/contents is anonymous-permissive; missing userID must yield AnonymousViewer")
	}
	if vc.Identity().CanonicalUserID != uuid.Nil {
		t.Fatalf("anonymous derived userID = %v, want uuid.Nil (preserves block-skip semantics)", vc.Identity().CanonicalUserID)
	}
}

func TestConstructUserContentViewerContext_AuthenticatedOnValidUUID(t *testing.T) {
	c := newContentTestGinContext(t, "/api/v1/users/abc/contents")
	uid := uuid.New()
	c.Set("user_id", uid)
	vc := constructUserContentViewerContext(c, nil)
	if vc.IsAnonymous() {
		t.Fatal("expected AuthenticatedViewer when valid UUID in gin context")
	}
	if vc.Identity().CanonicalUserID != uid {
		t.Fatalf("CanonicalUserID = %v, want %v", vc.Identity().CanonicalUserID, uid)
	}
}

// ---- Shared invariants ----

func TestContentConstructors_LifecycleHydratedFalseOnNilTx(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctor func(*gin.Context, interface{}) *viewercontext.ViewerContext
	}{
		{"contents/:id", func(c *gin.Context, _ interface{}) *viewercontext.ViewerContext { return constructContentDetailViewerContext(c, nil) }},
		{"users/:id/contents", func(c *gin.Context, _ interface{}) *viewercontext.ViewerContext { return constructUserContentViewerContext(c, nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newContentTestGinContext(t, "/api/v1/test")
			c.Set("user_id", uuid.New())
			vc := tc.ctor(c, nil)
			// F1-W3B contract: viewer lifecycle IS hydrated inline, but
			// hydrateContentDetailViewerLifecycle returns hydrated=false
			// when tx is nil (the pre-WithTx anonymous-permissive
			// construction path and unit tests with no DB). The
			// evaluator + fail-CLOSED detail adapter then 404 in enforce
			// mode per doctrine §8.5 — semantically equivalent to the
			// pre-W3B state at this construction site.
			if vc.Lifecycle().IsHydrated() {
				t.Fatal("nil tx must yield hydrated=false lifecycle overlay")
			}
		})
	}
}

func TestContentConstructors_SurfaceIsPublicDiscovery(t *testing.T) {
	c := newContentTestGinContext(t, "/api/v1/test")
	c.Set("user_id", uuid.New())
	for name, vc := range map[string]*viewercontext.ViewerContext{
		"contents/:id":       constructContentDetailViewerContext(c, nil),
		"users/:id/contents": constructUserContentViewerContext(c, nil),
	} {
		if vc.Surface() != viewercontext.SurfacePublicDiscovery {
			t.Fatalf("%s surface = %v, want %v", name, vc.Surface(), viewercontext.SurfacePublicDiscovery)
		}
	}
}

func TestContentConstructors_FallsBackToLegacyUserIDKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctor func(*gin.Context) *viewercontext.ViewerContext
	}{
		{"contents/:id", func(c *gin.Context) *viewercontext.ViewerContext { return constructContentDetailViewerContext(c, nil) }},
		{"users/:id/contents", func(c *gin.Context) *viewercontext.ViewerContext { return constructUserContentViewerContext(c, nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newContentTestGinContext(t, "/api/v1/test")
			uid := uuid.New()
			c.Set("userID", uid) // legacy key, not snake_case
			vc := tc.ctor(c)
			if vc.IsAnonymous() {
				t.Fatal("constructor must accept legacy 'userID' key in addition to 'user_id'")
			}
			if vc.Identity().CanonicalUserID != uid {
				t.Fatalf("legacy userID key did not propagate: got %v, want %v", vc.Identity().CanonicalUserID, uid)
			}
		})
	}
}


