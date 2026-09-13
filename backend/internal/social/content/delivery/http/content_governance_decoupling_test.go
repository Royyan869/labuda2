package http

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/social/content/entity"
	"go.uber.org/zap"
)

// TestContentHandler_CanonicalEnforcementIndependentOfShadowRunner proves
// that /contents/:id enforcement is unconditional and never depends on the
// observability shadow runner: there is no mode parameter, and the single
// business gate fail-CLOSES on a denied row whether the handler was built
// with a runner or without one.
func TestContentHandler_CanonicalEnforcementIndependentOfShadowRunner(t *testing.T) {
	viewer := uuid.New()
	author := uuid.New()
	blockedAuthor := uuid.New()
	contentID := uuid.New()
	blockedContentID := uuid.New()

	identity := viewercontext.IdentityOverlay{CanonicalUserID: viewer}
	lifecycle := viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, true)
	baseVC := viewercontext.NewAuthenticated(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST, identity, lifecycle, viewercontext.CapabilityOverlay{}, viewercontext.ModerationOverlay{})
	vcAllow := baseVC.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(nil))
	vcDeny := baseVC.WithRelationship(viewercontext.NewHydratedRelationshipOverlay([]uuid.UUID{blockedAuthor}))

	tc := viewercontext.NewTargetContext()
	tc.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{
		author:        viewercontext.PublicLifecycleStateActive,
		blockedAuthor: viewercontext.PublicLifecycleStateActive,
	})
	tc.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{
		contentID:        viewercontext.ContentModerationStateVisible,
		blockedContentID: viewercontext.ContentModerationStateVisible,
	})

	contentAllow := &entity.Content{ID: contentID, AuthorID: author, Status: entity.StatusActive, IsHidden: false}
	contentDeny := &entity.Content{ID: blockedContentID, AuthorID: blockedAuthor, Status: entity.StatusActive, IsHidden: false}

	runnerEnabled := evaluator.NewContentDetailShadowRunner(zap.NewNop())
	hEnabled := NewContentHandler(nil, nil, nil, zap.NewNop(), runnerEnabled)
	hDisabled := NewContentHandler(nil, nil, nil, zap.NewNop(), nil)

	if hEnabled.contentDetailShadowRunner == nil {
		t.Fatal("handler built with a runner must retain it (observability)")
	}
	if hDisabled.contentDetailShadowRunner != nil {
		t.Fatal("handler built without a runner must have a nil shadowRunner")
	}

	resAllow := evaluator.EnforceContentDetail(vcAllow, tc, contentAllow)
	if !resAllow.Allow {
		t.Fatalf("ALLOW must pass; got %+v", resAllow)
	}

	resDeny := evaluator.EnforceContentDetail(vcDeny, tc, contentDeny)
	if resDeny.Allow {
		t.Fatalf("block DENY must 404; got Allow=%v", resDeny.Allow)
	}

	// UNKNOWN fails CLOSED with no mode / runner dependency.
	resUnknown := evaluator.EnforceContentDetail(nil, nil, contentAllow)
	if resUnknown.Allow {
		t.Fatalf("UNKNOWN must fail-CLOSED; got Allow=%v", resUnknown.Allow)
	}

	// Nil runner is observability-safe.
	hDisabled.contentDetailShadowRunner.Run(nil, nil, nil, evaluator.LegacyContentDetailOutcome200)
}

func TestContentHandler_OneConstructionModel(t *testing.T) {
	runner := evaluator.NewContentDetailShadowRunner(zap.NewNop())
	h := NewContentHandler(nil, nil, nil, zap.NewNop(), runner)
	if h.contentDetailShadowRunner != runner {
		t.Fatal("runner not retained by handler")
	}
}
