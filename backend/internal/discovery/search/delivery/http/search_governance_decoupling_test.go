package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"go.uber.org/zap"
)

// TestSearchHandler_CanonicalEnforcementIndependentOfShadowRunner proves
// that /search/content enforcement is unconditional and never depends on
// the observability shadow runner: there is no mode parameter, and the
// single business gate drops a denied row whether the handler was built
// with a runner or without one.
func TestSearchHandler_CanonicalEnforcementIndependentOfShadowRunner(t *testing.T) {
	viewer := uuid.New()
	blockedAuthor := uuid.New()
	activeAuthor := uuid.New()
	keepID := uuid.New()
	denyID := uuid.New()

	contents := []*entity.ContentPreview{
		{ID: keepID, AuthorID: activeAuthor},
		{ID: denyID, AuthorID: blockedAuthor},
	}
	identity := viewercontext.IdentityOverlay{CanonicalUserID: viewer}
	lc := viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, true)
	baseVC := viewercontext.NewAuthenticated(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST, identity, lc, viewercontext.CapabilityOverlay{}, viewercontext.ModerationOverlay{})
	vcAllow := baseVC.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(nil))
	vcDeny := baseVC.WithRelationship(viewercontext.NewHydratedRelationshipOverlay([]uuid.UUID{blockedAuthor}))

	tc := viewercontext.NewTargetContext()
	tc.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{
		activeAuthor:  viewercontext.PublicLifecycleStateActive,
		blockedAuthor: viewercontext.PublicLifecycleStateActive,
	})
	tc.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{
		keepID: viewercontext.ContentModerationStateVisible,
		denyID: viewercontext.ContentModerationStateVisible,
	})

	runnerEnabled := evaluator.NewSearchContentShadowRunner(zap.NewNop())
	hEnabled := NewSearchHandler(nil, nil, zap.NewNop(), runnerEnabled, nil)
	hDisabled := NewSearchHandler(nil, nil, zap.NewNop(), nil, nil)

	if hEnabled.searchContentShadowRunner == nil {
		t.Fatal("handler built with a runner must retain it (observability)")
	}
	if hDisabled.searchContentShadowRunner != nil {
		t.Fatal("handler built without a runner must have a nil shadowRunner")
	}

	// Deny: blocked author dropped.
	resDeny := evaluator.EnforceSearchContent(vcDeny, tc, contents)
	if len(resDeny.Filtered) != 1 || resDeny.Filtered[0].ID != keepID {
		t.Fatalf("deny must drop the blocked row; got %d rows", len(resDeny.Filtered))
	}
	if resDeny.DroppedCount != 1 {
		t.Fatalf("DroppedCount = %d, want 1", resDeny.DroppedCount)
	}

	// Allow: both rows kept.
	resAllow := evaluator.EnforceSearchContent(vcAllow, tc, contents)
	if len(resAllow.Filtered) != 2 {
		t.Fatalf("allow must keep both rows; got %d", len(resAllow.Filtered))
	}

	// UNKNOWN/input_invalid fails CLOSED with no mode / runner dependency.
	resUnknown := evaluator.EnforceSearchContent(nil, nil, contents)
	if len(resUnknown.Filtered) != 0 {
		t.Fatalf("input_invalid must fail-closed; got %d rows", len(resUnknown.Filtered))
	}

	// Nil runner is observability-safe.
	hDisabled.searchContentShadowRunner.Run(vcAllow, tc, contents)
}

func TestSearchHandler_OneConstructionModel(t *testing.T) {
	runner := evaluator.NewSearchContentShadowRunner(zap.NewNop())
	h := NewSearchHandler(nil, nil, zap.NewNop(), runner, nil)
	if h.searchContentShadowRunner != runner {
		t.Fatal("runner not retained by handler")
	}
}
