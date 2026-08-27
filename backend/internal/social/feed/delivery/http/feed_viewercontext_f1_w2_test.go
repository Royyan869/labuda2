package http

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	capabilityctx "github.com/labuda/backend/internal/platform/capability"
	capabilityentity "github.com/labuda/backend/internal/platform/capability/entity"
)

// F1-W2 — constructFeedViewerContext behavior pin.

func newFeedTestGinContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/feed", nil)
	return c
}

func feedSellerActor() *capabilityentity.Actor {
	status := string(capabilityentity.SellerStatusActive)
	return &capabilityentity.Actor{
		Role:          "user",
		AccountStatus: "active",
		EmailVerified: true,
		SellerStatus:  &status,
	}
}

func TestConstructFeedViewerContext_AnonymousOnMissingUserID(t *testing.T) {
	c := newFeedTestGinContext(t)
	vc := constructFeedViewerContext(c, nil)
	if vc == nil {
		t.Fatal("expected non-nil ViewerContext")
	}
	if !vc.IsAnonymous() {
		t.Fatal("expected AnonymousViewer when no userID is in gin context")
	}
	if vc.Identity().CanonicalUserID != uuid.Nil {
		t.Fatalf("anonymous identity CanonicalUserID = %v, want uuid.Nil", vc.Identity().CanonicalUserID)
	}
}

func TestConstructFeedViewerContext_AnonymousOnNilUUID(t *testing.T) {
	c := newFeedTestGinContext(t)
	c.Set("user_id", uuid.Nil)
	vc := constructFeedViewerContext(c, nil)
	if !vc.IsAnonymous() {
		t.Fatal("nil UUID must yield AnonymousViewer per F6")
	}
}

func TestConstructFeedViewerContext_AnonymousOnWrongType(t *testing.T) {
	c := newFeedTestGinContext(t)
	c.Set("user_id", "not-a-uuid-just-a-string")
	vc := constructFeedViewerContext(c, nil)
	if !vc.IsAnonymous() {
		t.Fatal("wrong-typed userID must yield AnonymousViewer (defensive)")
	}
}

func TestConstructFeedViewerContext_AuthenticatedOnValidUUID(t *testing.T) {
	c := newFeedTestGinContext(t)
	uid := uuid.New()
	c.Set("user_id", uid)
	c.Set("firebase_uid", "fb-test-uid")
	c.Set("is_admin", false)
	c.Set("is_moderator", false)
	c.Request = c.Request.WithContext(capabilityctx.WithActor(c.Request.Context(), feedSellerActor()))
	vc := constructFeedViewerContext(c, nil)
	if vc.IsAnonymous() {
		t.Fatal("expected AuthenticatedViewer when valid UUID in gin context")
	}
	if vc.Identity().CanonicalUserID != uid {
		t.Fatalf("CanonicalUserID = %v, want %v", vc.Identity().CanonicalUserID, uid)
	}
	if vc.Identity().FirebaseUID != "fb-test-uid" {
		t.Fatal("firebase_uid did not propagate")
	}
	if !vc.Capability().IsSeller {
		t.Fatal("seller actor capability did not propagate")
	}
}

func TestConstructFeedViewerContext_LifecycleHydratedFalseOnNilTx(t *testing.T) {
	c := newFeedTestGinContext(t)
	uid := uuid.New()
	c.Set("user_id", uid)
	vc := constructFeedViewerContext(c, nil)
	// F1-W3A contract: viewer lifecycle IS hydrated inline, but
	// hydrateFeedViewerLifecycle returns hydrated=false when tx is
	// nil (the pre-WithTx anonymous-reject construction path and
	// unit tests with no DB). The evaluator + feed adapter then
	// fail-OPEN on the missing overlay per the high-traffic /feed
	// doctrine — semantically equivalent to the pre-W3A state.
	if vc.Lifecycle().IsHydrated() {
		t.Fatal("nil tx must yield hydrated=false lifecycle overlay")
	}
}

func TestConstructFeedViewerContext_FallsBackToUserIDKey(t *testing.T) {
	c := newFeedTestGinContext(t)
	uid := uuid.New()
	// Some middleware paths set "userID" (camel), not "user_id".
	c.Set("userID", uid)
	vc := constructFeedViewerContext(c, nil)
	if vc.IsAnonymous() {
		t.Fatal("constructor must accept legacy 'userID' key in addition to 'user_id'")
	}
	if vc.Identity().CanonicalUserID != uid {
		t.Fatalf("legacy userID key did not propagate: got %v, want %v", vc.Identity().CanonicalUserID, uid)
	}
}

// TestConstructFeedViewerContext_SurfaceIsPublicDiscovery pins the surface
// classification to /search/content's canonical value so the eventual
// F1-W3 rebuild can drop in evaluator-side overlay hydration without
// surface-class drift.
func TestConstructFeedViewerContext_SurfaceIsPublicDiscovery(t *testing.T) {
	c := newFeedTestGinContext(t)
	c.Set("user_id", uuid.New())
	vc := constructFeedViewerContext(c, nil)
	if vc.Surface() != viewercontext.SurfacePublicDiscovery {
		t.Fatalf("surface = %v, want %v", vc.Surface(), viewercontext.SurfacePublicDiscovery)
	}
}
