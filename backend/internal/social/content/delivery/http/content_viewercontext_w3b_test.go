package http

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/labuda/backend/internal/governance/viewercontext"
	contententity "github.com/labuda/backend/internal/social/content/entity"
)

// F1-W3B — handler-boundary hydrator unit tests for /contents/:id.
//
// These tests pin the nil-tx / empty-input / anonymous-viewer / self-
// authored failure-mode contract so the canonical hydrators stay safe
// when invoked outside an integration suite. DB-dependent paths are
// covered in the integration tests already.

func TestHydrateContentDetailViewerLifecycle_NilTxReturnsUnhydrated(t *testing.T) {
	lc := hydrateContentDetailViewerLifecycle(context.Background(), nil, uuid.New())
	if lc.IsHydrated() {
		t.Error("nil tx must yield hydrated=false; the evaluator then UNKNOWNs and the fail-CLOSED adapter 404s")
	}
}

func TestHydrateContentDetailViewerLifecycle_NilUUIDReturnsUnhydrated(t *testing.T) {
	lc := hydrateContentDetailViewerLifecycle(context.Background(), nil, uuid.Nil)
	if lc.IsHydrated() {
		t.Error("nil viewer UUID must yield hydrated=false")
	}
}

func TestHydrateContentDetailTargetContext_NilContentReturnsHydratedEmpty(t *testing.T) {
	tc := hydrateContentDetailTargetContext(context.Background(), nil, nil)
	if tc == nil {
		t.Fatal("expected non-nil TC")
	}
	if !tc.AuthorLifecycleHydrated() {
		t.Error("nil-content TC must report AuthorLifecycle hydrated (empty)")
	}
	if !tc.ContentModerationHydrated() {
		t.Error("nil-content TC must report ContentModeration hydrated (empty)")
	}
}

func TestHydrateContentDetailTargetContext_NilTxFallsBackToEntityHidden(t *testing.T) {
	content := &contententity.Content{
		ID:       uuid.New(),
		AuthorID: uuid.New(),
		Status:   contententity.StatusActive,
		IsHidden: true,
	}
	tc := hydrateContentDetailTargetContext(context.Background(), nil, content)
	if !tc.ContentModerationHydrated() {
		t.Fatal("ContentModeration must be hydrated even on nil tx (fallback to entity flag)")
	}
	mod, ok := tc.ContentModeration(content.ID)
	if !ok {
		t.Fatal("per-content lookup must succeed when entity-flag fallback is taken")
	}
	if mod != viewercontext.ContentModerationStateHidden {
		t.Errorf("ContentModeration = %q, want hidden (entity flag was IsHidden=true)", mod)
	}

	// Author lifecycle remains hydrated-empty (no DB to query).
	if !tc.AuthorLifecycleHydrated() {
		t.Error("AuthorLifecycle must report hydrated=true (empty map) when tx is nil")
	}
	if _, ok := tc.AuthorLifecycle(content.AuthorID); ok {
		t.Error("per-author lookup must be (_, false) when no DB was hit")
	}
}

func TestHydrateContentDetailRelationship_AnonymousReturnsVCUnchanged(t *testing.T) {
	anon := viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST)
	content := &contententity.Content{ID: uuid.New(), AuthorID: uuid.New(), Status: contententity.StatusActive}
	got := hydrateContentDetailRelationship(context.Background(), nil, anon, content)
	if got != anon {
		t.Error("anonymous VC must be returned unchanged (relationship is topology-empty)")
	}
}

func TestHydrateContentDetailRelationship_NilVCReturnsNil(t *testing.T) {
	if got := hydrateContentDetailRelationship(context.Background(), nil, nil, nil); got != nil {
		t.Errorf("nil VC must return nil; got %v", got)
	}
}

func TestHydrateContentDetailRelationship_NilContentAttachesEmptyHydratedSet(t *testing.T) {
	identity := viewercontext.IdentityOverlay{CanonicalUserID: uuid.New()}
	vc := viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST,
		identity,
		viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, true),
		viewercontext.CapabilityOverlay{}, viewercontext.ModerationOverlay{},
	)
	got := hydrateContentDetailRelationship(context.Background(), nil, vc, nil)
	if !got.Relationship().IsHydrated() {
		t.Error("nil-content hydration must yield IsHydrated()=true")
	}
}

func TestHydrateContentDetailRelationship_SelfAuthoredShortCircuits(t *testing.T) {
	uid := uuid.New()
	identity := viewercontext.IdentityOverlay{CanonicalUserID: uid}
	vc := viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST,
		identity,
		viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, true),
		viewercontext.CapabilityOverlay{}, viewercontext.ModerationOverlay{},
	)
	content := &contententity.Content{ID: uuid.New(), AuthorID: uid, Status: contententity.StatusActive}
	got := hydrateContentDetailRelationship(context.Background(), nil, vc, content)
	if !got.Relationship().IsHydrated() {
		t.Error("self-authored hydration must yield IsHydrated()=true (cannot be blocked against self)")
	}
	if got.Relationship().IsBlocked(uid) {
		t.Error("self-authored must report IsBlocked(self)=false")
	}
}

func TestConstructContentDetailViewerContext_W3B_AuthenticatedWithNilTxStillHasIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/contents/abc", nil)
	uid := uuid.New()
	c.Set("user_id", uid)
	vc := constructContentDetailViewerContext(c, nil)
	if vc.IsAnonymous() {
		t.Fatal("auth path must yield AuthenticatedViewer even when tx is nil")
	}
	if vc.Identity().CanonicalUserID != uid {
		t.Errorf("CanonicalUserID = %v, want %v", vc.Identity().CanonicalUserID, uid)
	}
	if vc.Lifecycle().IsHydrated() {
		t.Error("nil-tx hydration must leave lifecycle unhydrated (fail-CLOSED trigger)")
	}
}

func TestConstructContentDetailViewerContext_W3B_CapabilityFallbackFromGin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/contents/abc", nil)
	c.Set("user_id", uuid.New())
	c.Set("is_admin", true)
	// No actor in context — falls back to gin-set capability flags.
	vc := constructContentDetailViewerContext(c, nil)
	if !vc.Capability().IsAdmin {
		t.Error("is_admin gin flag must propagate when actor middleware did not run")
	}
	// Block override has no gin fallback by design — must be false.
	if vc.Capability().HasBlockOverrideCapability {
		t.Error("HasBlockOverrideCapability must default false without actor capability binding")
	}
}


