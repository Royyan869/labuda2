package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
	"go.uber.org/zap"
)

// TestFeedHandler_CanonicalEnforcementIndependentOfShadowRunner proves that
// /feed enforcement is unconditional and never depends on the observability
// shadow runner: there is no mode parameter, and the single business gate
// drops a denied row whether the handler was built with a runner or without
// one.
func TestFeedHandler_CanonicalEnforcementIndependentOfShadowRunner(t *testing.T) {
	viewer := uuid.New()
	suspendedAuthor := uuid.New()
	activeAuthor := uuid.New()
	keepID := uuid.New()
	denyID := uuid.New()
	in := []*feedentity.FeedItem{
		{ID: keepID, AuthorID: activeAuthor, Status: "active", IsHidden: false},
		{ID: denyID, AuthorID: suspendedAuthor, Status: "active", IsHidden: false},
	}
	identity := viewercontext.IdentityOverlay{CanonicalUserID: viewer}
	lifecycle := viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, true)
	vc := viewercontext.NewAuthenticated(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST, identity, lifecycle, viewercontext.CapabilityOverlay{}, viewercontext.ModerationOverlay{})
	vc = vc.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(nil))
	tc := viewercontext.NewTargetContext()
	tc.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{
		activeAuthor:    viewercontext.PublicLifecycleStateActive,
		suspendedAuthor: viewercontext.PublicLifecycleStateUnavailable,
	})
	tc.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{
		keepID: viewercontext.ContentModerationStateVisible,
		denyID: viewercontext.ContentModerationStateVisible,
	})

	runnerEnabled := evaluator.NewFeedShadowRunner(zap.NewNop())
	hEnabled := NewFeedHandler(nil, nil, zap.NewNop(), runnerEnabled, nil)
	hDisabled := NewFeedHandler(nil, nil, zap.NewNop(), nil, nil)

	if hEnabled.shadowRunner == nil {
		t.Fatal("handler built with a runner must retain it (observability)")
	}
	if hDisabled.shadowRunner != nil {
		t.Fatal("handler built without a runner must have a nil shadowRunner")
	}

	// The canonical business gate has exactly one answer — no mode, no
	// shadow branch, independent of runner existence.
	res := evaluator.EnforceFeed(vc, tc, in)
	if len(res.Filtered) != 1 || res.Filtered[0].ID != keepID {
		t.Fatalf("enforcement must drop the denied row; got %d rows", len(res.Filtered))
	}
	if res.DroppedCount != 1 {
		t.Fatalf("DroppedCount = %d, want 1", res.DroppedCount)
	}

	// Nil runner is observability-safe.
	hDisabled.shadowRunner.Run(vc, tc, in)
}

func TestFeedHandler_OneConstructionModel(t *testing.T) {
	runner := evaluator.NewFeedShadowRunner(zap.NewNop())
	h := NewFeedHandler(nil, nil, zap.NewNop(), runner, nil)
	if h.shadowRunner != runner {
		t.Fatal("runner not retained by handler")
	}
}
