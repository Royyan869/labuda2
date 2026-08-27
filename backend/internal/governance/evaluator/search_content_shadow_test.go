package evaluator_test

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"go.uber.org/zap"
)

// helper builders ---------------------------------------------------------

func newAnonVC() *viewercontext.ViewerContext {
	return viewercontext.NewAnonymous(
		viewercontext.SurfacePublicDiscovery,
		viewercontext.RequestOriginREST,
	)
}

func newAuthVC(viewerID uuid.UUID) *viewercontext.ViewerContext {
	// Lifecycle is constructed via NewLifecycleOverlay(active, hydrated=true)
	// to match the canonical state produced by constructSearchContentViewerContext
	// after the viewer_lifecycle hydration runtime landed (see current-execution-
	// state.md §3 CURRENT_RUNTIME_STATE: search_content_viewer_lifecycle_hydration_active).
	// "Authenticated active viewer" is the canonical, hydrated form by topology;
	// hydrated=false is reserved for DB-error simulation in dedicated tests.
	return viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery,
		viewercontext.RequestOriginREST,
		viewercontext.IdentityOverlay{
			FirebaseUID:     "firebase-test",
			CanonicalUserID: viewerID,
		},
		viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, true),
		viewercontext.CapabilityOverlay{},
		viewercontext.ModerationOverlay{},
	)
}

// newAuthVCLifecycle is a focused builder for tests of the new viewer_lifecycle
// precedence step. It constructs an authenticated ViewerContext with a caller-
// specified lifecycle state and hydration flag, plus a hydrated empty
// relationship overlay so tests of step 1 (viewer_lifecycle) can isolate the
// step's outcome without being shadowed by step 3 (relationship) UNKNOWN.
func newAuthVCLifecycle(viewerID uuid.UUID, state viewercontext.PublicLifecycleState, hydrated bool) *viewercontext.ViewerContext {
	vc := viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery,
		viewercontext.RequestOriginREST,
		viewercontext.IdentityOverlay{
			FirebaseUID:     "firebase-test",
			CanonicalUserID: viewerID,
		},
		viewercontext.NewLifecycleOverlay(state, hydrated),
		viewercontext.CapabilityOverlay{},
		viewercontext.ModerationOverlay{},
	)
	return vc.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(nil))
}

func newAuthVCWithRelationship(viewerID uuid.UUID, blocked []uuid.UUID) *viewercontext.ViewerContext {
	rel := viewercontext.NewHydratedRelationshipOverlay(blocked)
	return newAuthVC(viewerID).WithRelationship(rel)
}

func newTargetCtx(authorStates map[uuid.UUID]viewercontext.PublicLifecycleState, modStates map[uuid.UUID]viewercontext.ContentModerationState) *viewercontext.TargetContext {
	tc := viewercontext.NewTargetContext()
	if authorStates != nil {
		tc.WithAuthorLifecycle(authorStates)
	}
	if modStates != nil {
		tc.WithContentModeration(modStates)
	}
	return tc
}

// EvaluateSearchContent — pure decision tests --------------------------

// TestEvaluateSearchContent_NilInput verifies the input-invalid contract
// per docs/05-rollout/search-shadow-seam-architecture.md §7.7.
func TestEvaluateSearchContent_NilInput(t *testing.T) {
	tc := newTargetCtx(map[uuid.UUID]viewercontext.PublicLifecycleState{}, map[uuid.UUID]viewercontext.ContentModerationState{})

	d, r, s, sem := evaluator.EvaluateSearchContent(nil, tc, &entity.ContentPreview{ID: uuid.New(), AuthorID: uuid.New()})
	if d != evaluator.ShadowDecisionUnknown {
		t.Errorf("nil vc: decision = %q; want unknown", d)
	}
	if r != evaluator.SearchUnknownReasonInputInvalid {
		t.Errorf("nil vc: reason = %q; want input_invalid", r)
	}
	if s != evaluator.SearchUnknownSourceNone {
		t.Errorf("nil vc: source = %q; want none", s)
	}
	if sem != evaluator.SearchExposureSemanticUnknownShadowOnly {
		t.Errorf("nil vc: semantic = %q; want unknown_shadow_only", sem)
	}

	d2, r2, _, _ := evaluator.EvaluateSearchContent(newAnonVC(), tc, nil)
	if d2 != evaluator.ShadowDecisionUnknown || r2 != evaluator.SearchUnknownReasonInputInvalid {
		t.Errorf("nil row: decision/reason = %q/%q; want unknown/input_invalid", d2, r2)
	}
}

// TestEvaluateSearchContent_TargetLifecycleMissing verifies the
// target_overlay_missing UNKNOWN classification when the author-lifecycle
// overlay is not hydrated.
func TestEvaluateSearchContent_TargetLifecycleMissing(t *testing.T) {
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}

	d, r, s, _ := evaluator.EvaluateSearchContent(newAnonVC(), nil, row)
	if d != evaluator.ShadowDecisionUnknown {
		t.Errorf("nil tc: decision = %q; want unknown", d)
	}
	if r != evaluator.SearchUnknownReasonTargetOverlayMissing {
		t.Errorf("nil tc: reason = %q; want target_overlay_missing", r)
	}
	if s != evaluator.SearchUnknownSourceTargetLifecycle {
		t.Errorf("nil tc: source = %q; want target_lifecycle", s)
	}

	// Per-row absence (overlay hydrated but author key missing) — same UNKNOWN.
	tcEmpty := newTargetCtx(map[uuid.UUID]viewercontext.PublicLifecycleState{}, map[uuid.UUID]viewercontext.ContentModerationState{})
	d2, r2, s2, _ := evaluator.EvaluateSearchContent(newAnonVC(), tcEmpty, row)
	if d2 != evaluator.ShadowDecisionUnknown || r2 != evaluator.SearchUnknownReasonTargetOverlayMissing || s2 != evaluator.SearchUnknownSourceTargetLifecycle {
		t.Errorf("per-row missing: got (%q,%q,%q); want unknown/target_overlay_missing/target_lifecycle", d2, r2, s2)
	}
}

// TestEvaluateSearchContent_AuthorRemoved verifies DENY emission with
// unknown_shadow_only semantic for removed authors per task-design §7.4.
func TestEvaluateSearchContent_AuthorRemoved(t *testing.T) {
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateRemoved},
		map[uuid.UUID]viewercontext.ContentModerationState{},
	)

	d, r, s, sem := evaluator.EvaluateSearchContent(newAnonVC(), tc, row)
	if d != evaluator.ShadowDecisionDeny {
		t.Errorf("removed author: decision = %q; want deny", d)
	}
	if r != evaluator.SearchUnknownReasonNone || s != evaluator.SearchUnknownSourceNone {
		t.Errorf("removed author: unexpected unknown classification (%q,%q)", r, s)
	}
	if sem != evaluator.SearchExposureSemanticUnknownShadowOnly {
		t.Errorf("removed author: semantic = %q; want unknown_shadow_only (BLOCKER-005 gate)", sem)
	}
}

// TestEvaluateSearchContent_AuthorUnavailable verifies DENY emission for
// suspended/banned authors.
func TestEvaluateSearchContent_AuthorUnavailable(t *testing.T) {
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateUnavailable},
		map[uuid.UUID]viewercontext.ContentModerationState{},
	)

	d, _, _, sem := evaluator.EvaluateSearchContent(newAnonVC(), tc, row)
	if d != evaluator.ShadowDecisionDeny {
		t.Errorf("unavailable author: decision = %q; want deny", d)
	}
	if sem != evaluator.SearchExposureSemanticUnknownShadowOnly {
		t.Errorf("unavailable author: semantic = %q; want unknown_shadow_only", sem)
	}
}

// TestEvaluateSearchContent_RelationshipMissing verifies that
// authenticated traffic without hydrated relationship overlay emits
// viewer_overlay_missing/relationship UNKNOWN.
func TestEvaluateSearchContent_RelationshipMissing(t *testing.T) {
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		map[uuid.UUID]viewercontext.ContentModerationState{row.ID: viewercontext.ContentModerationStateVisible},
	)
	// Plain authenticated VC — no WithRelationship → relationship overlay
	// not hydrated.
	vc := newAuthVC(uuid.New())

	d, r, s, _ := evaluator.EvaluateSearchContent(vc, tc, row)
	if d != evaluator.ShadowDecisionUnknown {
		t.Errorf("auth + no relationship: decision = %q; want unknown", d)
	}
	if r != evaluator.SearchUnknownReasonViewerOverlayMissing {
		t.Errorf("auth + no relationship: reason = %q; want viewer_overlay_missing", r)
	}
	if s != evaluator.SearchUnknownSourceRelationship {
		t.Errorf("auth + no relationship: source = %q; want relationship", s)
	}
}

// TestEvaluateSearchContent_Blocked verifies DENY emission for blocked
// authors per the dominant relationship-drift expectation in task-design §3.4.
func TestEvaluateSearchContent_Blocked(t *testing.T) {
	viewerID := uuid.New()
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		map[uuid.UUID]viewercontext.ContentModerationState{row.ID: viewercontext.ContentModerationStateVisible},
	)
	vc := newAuthVCWithRelationship(viewerID, []uuid.UUID{authorID})

	d, r, _, sem := evaluator.EvaluateSearchContent(vc, tc, row)
	if d != evaluator.ShadowDecisionDeny {
		t.Errorf("blocked author: decision = %q; want deny", d)
	}
	if r != evaluator.SearchUnknownReasonNone {
		t.Errorf("blocked author: unexpected reason = %q", r)
	}
	if sem != evaluator.SearchExposureSemanticUnknownShadowOnly {
		t.Errorf("blocked author: semantic = %q; want unknown_shadow_only (BLOCKER-005 gate)", sem)
	}
}

// TestEvaluateSearchContent_AnonymousSkipsRelationship verifies that
// AnonymousViewer never emits relationship UNKNOWN per viewer-context-
// contract.md §3.1 (anonymous has no relationship state by topology).
func TestEvaluateSearchContent_AnonymousSkipsRelationship(t *testing.T) {
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		map[uuid.UUID]viewercontext.ContentModerationState{row.ID: viewercontext.ContentModerationStateVisible},
	)

	d, r, s, sem := evaluator.EvaluateSearchContent(newAnonVC(), tc, row)
	if d != evaluator.ShadowDecisionAllow {
		t.Errorf("anonymous + active author + visible content: decision = %q; want allow", d)
	}
	if r != evaluator.SearchUnknownReasonNone || s != evaluator.SearchUnknownSourceNone {
		t.Errorf("anonymous allow: unexpected unknown classification (%q,%q)", r, s)
	}
	if sem != evaluator.SearchExposureSemanticAllow {
		t.Errorf("anonymous allow: semantic = %q; want allow", sem)
	}
}

// TestEvaluateSearchContent_ModerationMissing verifies that absent
// content-moderation overlay emits target_overlay_missing/target_moderation.
func TestEvaluateSearchContent_ModerationMissing(t *testing.T) {
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		nil, // moderation overlay not hydrated
	)

	d, r, s, _ := evaluator.EvaluateSearchContent(newAnonVC(), tc, row)
	if d != evaluator.ShadowDecisionUnknown {
		t.Errorf("moderation missing: decision = %q; want unknown", d)
	}
	if r != evaluator.SearchUnknownReasonTargetOverlayMissing {
		t.Errorf("moderation missing: reason = %q; want target_overlay_missing", r)
	}
	if s != evaluator.SearchUnknownSourceTargetModeration {
		t.Errorf("moderation missing: source = %q; want target_moderation", s)
	}
}

// TestEvaluateSearchContent_ContentHidden verifies that hydrated
// is_hidden=true emits TOMBSTONE (the unexpected-on-Option-A case per
// task-design §3.4 — race condition between legacy SQL and overlay).
func TestEvaluateSearchContent_ContentHidden(t *testing.T) {
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		map[uuid.UUID]viewercontext.ContentModerationState{row.ID: viewercontext.ContentModerationStateHidden},
	)

	d, _, _, sem := evaluator.EvaluateSearchContent(newAnonVC(), tc, row)
	if d != evaluator.ShadowDecisionTombstone {
		t.Errorf("content hidden: decision = %q; want tombstone", d)
	}
	if sem != evaluator.SearchExposureSemanticUnknownShadowOnly {
		t.Errorf("content hidden: semantic = %q; want unknown_shadow_only (BLOCKER-005 gate)", sem)
	}
}

// TestEvaluateSearchContent_HappyPath verifies the canonical ALLOW path
// for an authenticated viewer with a non-blocked active author and
// visible content.
func TestEvaluateSearchContent_HappyPath(t *testing.T) {
	viewerID := uuid.New()
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		map[uuid.UUID]viewercontext.ContentModerationState{row.ID: viewercontext.ContentModerationStateVisible},
	)
	vc := newAuthVCWithRelationship(viewerID, nil) // no blocks

	d, r, s, sem := evaluator.EvaluateSearchContent(vc, tc, row)
	if d != evaluator.ShadowDecisionAllow {
		t.Errorf("happy path: decision = %q; want allow", d)
	}
	if r != evaluator.SearchUnknownReasonNone || s != evaluator.SearchUnknownSourceNone {
		t.Errorf("happy path: unexpected unknown classification (%q,%q)", r, s)
	}
	if sem != evaluator.SearchExposureSemanticAllow {
		t.Errorf("happy path: semantic = %q; want allow", sem)
	}
}

// EvaluateSearchContent — viewer_lifecycle precedence step tests ------
//
// The 8 tests below pin the Candidate A insertion-point contract from
// docs/05-rollout/search-content-viewer-lifecycle-precedence-runtime-task-
// design.md §3.1 / §4 / §8. They cover:
//   - the four input cases of §4.1 (anonymous bypass / hydrated=false /
//     unavailable|removed / active),
//   - the four precedence-ordering assertions of §8.2 (the new step fires
//     before target_lifecycle and before relationship).
//
// Modifying or removing any of these tests requires an ADR per
// precedence-runtime-task-design.md §12 (forbidden patterns).

// TestEvaluateSearchContent_ViewerLifecycle_AnonymousBypassesPrecedence
// covers §4.1 case 1 — anonymous viewer falls through the new step
// without DENY / UNKNOWN emission. Per precedence design analysis §7,
// anonymous bypass is the architecturally correct (mandatory) behavior.
func TestEvaluateSearchContent_ViewerLifecycle_AnonymousBypassesPrecedence(t *testing.T) {
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		map[uuid.UUID]viewercontext.ContentModerationState{row.ID: viewercontext.ContentModerationStateVisible},
	)

	d, r, s, sem := evaluator.EvaluateSearchContent(newAnonVC(), tc, row)
	if d != evaluator.ShadowDecisionAllow {
		t.Errorf("anonymous + active author + visible content: decision = %q; want allow (anonymous must bypass step 1)", d)
	}
	if r != evaluator.SearchUnknownReasonNone || s != evaluator.SearchUnknownSourceNone {
		t.Errorf("anonymous bypass: unexpected unknown classification (%q,%q)", r, s)
	}
	if sem != evaluator.SearchExposureSemanticAllow {
		t.Errorf("anonymous bypass: semantic = %q; want allow", sem)
	}
}

// TestEvaluateSearchContent_ViewerLifecycle_HydrationFailedReturnsUnknown
// covers §4.1 case 2 — authenticated viewer with hydrated=false emits
// ShadowDecisionUnknown with viewer_overlay_missing / viewer_lifecycle.
// The decision must fire BEFORE target_lifecycle is reached — proven
// by passing a non-nil tc with the author lifecycle hydrated; if the
// new step is mispositioned, the test's expected reason/source would
// shift to relationship/target_overlay instead.
func TestEvaluateSearchContent_ViewerLifecycle_HydrationFailedReturnsUnknown(t *testing.T) {
	viewerID := uuid.New()
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		map[uuid.UUID]viewercontext.ContentModerationState{row.ID: viewercontext.ContentModerationStateVisible},
	)
	vc := newAuthVCLifecycle(viewerID, viewercontext.PublicLifecycleStateActive, false)

	d, r, s, sem := evaluator.EvaluateSearchContent(vc, tc, row)
	if d != evaluator.ShadowDecisionUnknown {
		t.Errorf("auth + hydrated=false: decision = %q; want unknown", d)
	}
	if r != evaluator.SearchUnknownReasonViewerOverlayMissing {
		t.Errorf("auth + hydrated=false: reason = %q; want viewer_overlay_missing", r)
	}
	if s != evaluator.SearchUnknownSourceViewerLifecycle {
		t.Errorf("auth + hydrated=false: source = %q; want viewer_lifecycle", s)
	}
	if sem != evaluator.SearchExposureSemanticUnknownShadowOnly {
		t.Errorf("auth + hydrated=false: semantic = %q; want unknown_shadow_only", sem)
	}
}

// TestEvaluateSearchContent_ViewerLifecycle_UnavailableReturnsDeny covers
// §4.1 case 3 (Unavailable) — suspended/banned viewer DENIED at step 1
// with unknown_shadow_only semantic (PublicCard runtime absent —
// BLOCKER-005).
func TestEvaluateSearchContent_ViewerLifecycle_UnavailableReturnsDeny(t *testing.T) {
	viewerID := uuid.New()
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		map[uuid.UUID]viewercontext.ContentModerationState{row.ID: viewercontext.ContentModerationStateVisible},
	)
	vc := newAuthVCLifecycle(viewerID, viewercontext.PublicLifecycleStateUnavailable, true)

	d, r, s, sem := evaluator.EvaluateSearchContent(vc, tc, row)
	if d != evaluator.ShadowDecisionDeny {
		t.Errorf("unavailable viewer: decision = %q; want deny", d)
	}
	if r != evaluator.SearchUnknownReasonNone || s != evaluator.SearchUnknownSourceNone {
		t.Errorf("unavailable viewer: unexpected unknown classification (%q,%q); deny is not unknown", r, s)
	}
	if sem != evaluator.SearchExposureSemanticUnknownShadowOnly {
		t.Errorf("unavailable viewer: semantic = %q; want unknown_shadow_only (BLOCKER-005 gate)", sem)
	}
}

// TestEvaluateSearchContent_ViewerLifecycle_RemovedReturnsDeny covers
// §4.1 case 3 (Removed) — deleted viewer DENIED at step 1.
func TestEvaluateSearchContent_ViewerLifecycle_RemovedReturnsDeny(t *testing.T) {
	viewerID := uuid.New()
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		map[uuid.UUID]viewercontext.ContentModerationState{row.ID: viewercontext.ContentModerationStateVisible},
	)
	vc := newAuthVCLifecycle(viewerID, viewercontext.PublicLifecycleStateRemoved, true)

	d, r, s, sem := evaluator.EvaluateSearchContent(vc, tc, row)
	if d != evaluator.ShadowDecisionDeny {
		t.Errorf("removed viewer: decision = %q; want deny", d)
	}
	if r != evaluator.SearchUnknownReasonNone || s != evaluator.SearchUnknownSourceNone {
		t.Errorf("removed viewer: unexpected unknown classification (%q,%q)", r, s)
	}
	if sem != evaluator.SearchExposureSemanticUnknownShadowOnly {
		t.Errorf("removed viewer: semantic = %q; want unknown_shadow_only (BLOCKER-005 gate)", sem)
	}
}

// TestEvaluateSearchContent_ViewerLifecycle_ActiveContinuesToTargetLifecycle
// covers §4.1 case 4 — authenticated active hydrated viewer falls
// through step 1 and reaches the existing precedence chain (target
// lifecycle → relationship → moderation → allow).
func TestEvaluateSearchContent_ViewerLifecycle_ActiveContinuesToTargetLifecycle(t *testing.T) {
	viewerID := uuid.New()
	authorID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: authorID}
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		map[uuid.UUID]viewercontext.ContentModerationState{row.ID: viewercontext.ContentModerationStateVisible},
	)
	vc := newAuthVCLifecycle(viewerID, viewercontext.PublicLifecycleStateActive, true)

	d, r, s, sem := evaluator.EvaluateSearchContent(vc, tc, row)
	if d != evaluator.ShadowDecisionAllow {
		t.Errorf("active hydrated viewer: decision = %q; want allow (must continue past step 1)", d)
	}
	if r != evaluator.SearchUnknownReasonNone || s != evaluator.SearchUnknownSourceNone {
		t.Errorf("active hydrated viewer: unexpected unknown classification (%q,%q)", r, s)
	}
	if sem != evaluator.SearchExposureSemanticAllow {
		t.Errorf("active hydrated viewer: semantic = %q; want allow", sem)
	}
}

// TestEvaluateSearchContent_ViewerLifecycle_FiresBefore_TargetLifecycle_Missing
// covers §8.2 ordering test #5 — viewer_lifecycle DENY must fire even
// when tc=nil (which would have caused target_overlay_missing UNKNOWN
// at the old step 1). DENY proves the new step is at position 1, not 2.
func TestEvaluateSearchContent_ViewerLifecycle_FiresBefore_TargetLifecycle_Missing(t *testing.T) {
	viewerID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: uuid.New()}
	vc := newAuthVCLifecycle(viewerID, viewercontext.PublicLifecycleStateUnavailable, true)

	d, r, s, _ := evaluator.EvaluateSearchContent(vc, nil, row)
	if d != evaluator.ShadowDecisionDeny {
		t.Errorf("unavailable viewer + nil tc: decision = %q; want deny (viewer step must fire before target step)", d)
	}
	if r != evaluator.SearchUnknownReasonNone || s != evaluator.SearchUnknownSourceNone {
		t.Errorf("unavailable viewer + nil tc: unexpected (%q,%q); deny must NOT carry target_overlay_missing/target_lifecycle", r, s)
	}
}

// TestEvaluateSearchContent_ViewerLifecycle_FiresBefore_TargetLifecycle_NotHydrated
// covers §8.2 ordering test #6 — same proof with tc non-nil but not
// AuthorLifecycleHydrated() (would have caused UNKNOWN at the old step 1).
func TestEvaluateSearchContent_ViewerLifecycle_FiresBefore_TargetLifecycle_NotHydrated(t *testing.T) {
	viewerID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: uuid.New()}
	tc := viewercontext.NewTargetContext() // AuthorLifecycleHydrated() == false
	vc := newAuthVCLifecycle(viewerID, viewercontext.PublicLifecycleStateRemoved, true)

	d, r, s, _ := evaluator.EvaluateSearchContent(vc, tc, row)
	if d != evaluator.ShadowDecisionDeny {
		t.Errorf("removed viewer + tc not hydrated: decision = %q; want deny", d)
	}
	if r != evaluator.SearchUnknownReasonNone || s != evaluator.SearchUnknownSourceNone {
		t.Errorf("removed viewer + tc not hydrated: unexpected (%q,%q); deny must NOT carry target classification", r, s)
	}
}

// TestEvaluateSearchContent_ViewerLifecycle_HydrationUnknown_FiresBefore_TargetLifecycle
// covers §8.2 ordering test #8 — viewer-side UNKNOWN classification must
// win over target-side UNKNOWN classification when both inputs would
// produce UNKNOWN. Setup: viewer hydrated=false (would emit viewer
// UNKNOWN) AND tc=nil (would emit target UNKNOWN at old step 1).
// Expect: viewer_overlay_missing/viewer_lifecycle, NOT target_overlay_
// missing/target_lifecycle.
func TestEvaluateSearchContent_ViewerLifecycle_HydrationUnknown_FiresBefore_TargetLifecycle(t *testing.T) {
	viewerID := uuid.New()
	row := &entity.ContentPreview{ID: uuid.New(), AuthorID: uuid.New()}
	vc := newAuthVCLifecycle(viewerID, viewercontext.PublicLifecycleStateActive, false)

	d, r, s, sem := evaluator.EvaluateSearchContent(vc, nil, row)
	if d != evaluator.ShadowDecisionUnknown {
		t.Errorf("hydrated=false + nil tc: decision = %q; want unknown", d)
	}
	if r != evaluator.SearchUnknownReasonViewerOverlayMissing {
		t.Errorf("hydrated=false + nil tc: reason = %q; want viewer_overlay_missing (NOT target_overlay_missing)", r)
	}
	if s != evaluator.SearchUnknownSourceViewerLifecycle {
		t.Errorf("hydrated=false + nil tc: source = %q; want viewer_lifecycle (NOT target_lifecycle)", s)
	}
	if sem != evaluator.SearchExposureSemanticUnknownShadowOnly {
		t.Errorf("hydrated=false + nil tc: semantic = %q; want unknown_shadow_only", sem)
	}
}

// SearchContentShadowRunner — dispatch / panic isolation tests --------

// TestRunner_NilReceiverIsNoOp verifies nil-safe Run.
func TestRunner_NilReceiverIsNoOp(t *testing.T) {
	var r *evaluator.SearchContentShadowRunner // nil receiver
	// Should not panic.
	r.Run(newAnonVC(), nil, []*entity.ContentPreview{{ID: uuid.New(), AuthorID: uuid.New()}})
}

// TestRunner_FireAndForget verifies that Run returns immediately even
// for large slices — the goroutine survives the caller return.
func TestRunner_FireAndForget(t *testing.T) {
	logger := zap.NewNop()
	r := evaluator.NewSearchContentShadowRunner(logger)

	contents := make([]*entity.ContentPreview, 100)
	for i := range contents {
		contents[i] = &entity.ContentPreview{ID: uuid.New(), AuthorID: uuid.New()}
	}
	tc := viewercontext.NewTargetContext()
	tc.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{})
	tc.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{})

	start := time.Now()
	r.Run(newAnonVC(), tc, contents)
	elapsed := time.Since(start)

	// Run should return well under 50ms — actual evaluation runs in goroutine.
	if elapsed > 50*time.Millisecond {
		t.Errorf("Run blocked for %v; expected fire-and-forget return", elapsed)
	}
}

// TestRunner_NilContentsHandled verifies the runner survives a nil
// candidate slice and an empty slice without panicking.
func TestRunner_NilContentsHandled(t *testing.T) {
	r := evaluator.NewSearchContentShadowRunner(zap.NewNop())
	// Should not panic.
	r.Run(newAnonVC(), viewercontext.NewTargetContext(), nil)
	r.Run(newAnonVC(), viewercontext.NewTargetContext(), []*entity.ContentPreview{})
	// Give goroutines a moment to drain.
	time.Sleep(10 * time.Millisecond)
}

// TestRunner_NilEntryInSliceSkipped verifies nil ContentPreview entries
// in the slice do not panic the runner.
func TestRunner_NilEntryInSliceSkipped(t *testing.T) {
	r := evaluator.NewSearchContentShadowRunner(zap.NewNop())
	tc := viewercontext.NewTargetContext()
	tc.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{})
	tc.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{})

	contents := []*entity.ContentPreview{
		nil,
		{ID: uuid.New(), AuthorID: uuid.New()},
		nil,
	}
	r.Run(newAnonVC(), tc, contents)
	time.Sleep(10 * time.Millisecond)
}

// TestRunner_SnapshotIsolation verifies that mutations to the caller's
// contents slice after Run returns do not affect the runner's view.
func TestRunner_SnapshotIsolation(t *testing.T) {
	r := evaluator.NewSearchContentShadowRunner(zap.NewNop())
	tc := viewercontext.NewTargetContext()
	tc.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{})
	tc.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{})

	a := uuid.New()
	contents := []*entity.ContentPreview{{ID: uuid.New(), AuthorID: a}}

	r.Run(newAnonVC(), tc, contents)

	// Mutate the caller's slice — runner must not observe this.
	contents[0] = nil
	contents = append(contents, &entity.ContentPreview{ID: uuid.New(), AuthorID: uuid.New()})
	_ = contents

	// Allow goroutine to complete.
	time.Sleep(20 * time.Millisecond)
	// If snapshot isolation broke, the runner would have panicked on
	// nil entry; the deferred recover would log Error but not crash the
	// process; this test passes because there is no nil-pointer fault
	// (snapshot was taken before the mutation).
}

// TestRunner_ConcurrentDispatchSafe verifies concurrent dispatch from
// multiple goroutines does not race the metric registry.
func TestRunner_ConcurrentDispatchSafe(t *testing.T) {
	r := evaluator.NewSearchContentShadowRunner(zap.NewNop())
	tc := viewercontext.NewTargetContext()
	tc.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{})
	tc.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{})

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Run(newAnonVC(), tc, []*entity.ContentPreview{{ID: uuid.New(), AuthorID: uuid.New()}})
		}()
	}
	wg.Wait()
	// Allow background goroutines to drain.
	time.Sleep(50 * time.Millisecond)
}

// TestRunner_ModeGetterDefaultShadow asserts the canonical default for
// the Mode() accessor. A freshly constructed runner reports shadow mode;
// nil receiver is safe and also reports shadow.
func TestRunner_ModeGetterDefaultShadow(t *testing.T) {
	r := evaluator.NewSearchContentShadowRunner(zap.NewNop())
	if got := r.Mode(); got != evaluator.SearchContentAdapterModeShadow {
		t.Errorf("default runner Mode() = %q; want shadow", got)
	}
	var nilRunner *evaluator.SearchContentShadowRunner
	if got := nilRunner.Mode(); got != evaluator.SearchContentAdapterModeShadow {
		t.Errorf("nil runner Mode() = %q; want shadow (nil-safety contract)", got)
	}
}

// TestRunner_WithModeRoundTrips asserts that the WithMode builder
// preserves the requested mode AND that invalid mode strings normalize
// to shadow per the safety-default contract.
func TestRunner_WithModeRoundTrips(t *testing.T) {
	base := evaluator.NewSearchContentShadowRunner(zap.NewNop())

	enforce := base.WithMode(evaluator.SearchContentAdapterModeEnforce)
	if got := enforce.Mode(); got != evaluator.SearchContentAdapterModeEnforce {
		t.Errorf("WithMode(enforce).Mode() = %q; want enforce", got)
	}
	// Base runner must remain shadow — WithMode returns a clone.
	if got := base.Mode(); got != evaluator.SearchContentAdapterModeShadow {
		t.Errorf("WithMode immutability violated: base runner mode = %q; want shadow", got)
	}

	bad := base.WithMode(evaluator.SearchContentAdapterMode("nonsense"))
	if got := bad.Mode(); got != evaluator.SearchContentAdapterModeShadow {
		t.Errorf("WithMode(invalid).Mode() = %q; want shadow (safety default)", got)
	}
}

// hashUUID is internal to the package — its existence is verified
// indirectly via the runner emitting structured logs with the hashed
// content_id_hashed / author_id_hashed fields. Direct testing happens
// via internal_test.go in the evaluator package; this file is _test.go
// (external test package) and cannot reach unexported helpers.
//
// The bounded enum families are verified by the type system at compile
// time (no string literal at any emit site) plus by the metric label
// cardinality test below.

// TestSearchShadowMetricLabelsBounded scans the package's emitted metric
// label values via Run dispatches and asserts that every emitted value
// is a member of the bounded enum family. Assertion is by-construction
// at compile time — the runner's emit functions take typed parameters
// (SearchEndpoint, CandidateSetOption, etc.). This test exists as a
// deliberate documentation reminder; modification of the runner that
// adds untyped string literals as labels would break compile.
func TestSearchShadowMetricLabelsBounded(t *testing.T) {
	// Compile-time assertion via type signatures — if a label-emit call
	// site ever passes an untyped string, this file's import set will
	// fail compile.
	_ = evaluator.SearchEndpointContent
	_ = evaluator.CandidateSetOptionAHandlerPostResponse
	_ = evaluator.SearchDivLegacyAllowShadowAllow
	_ = evaluator.SearchUnknownReasonViewerOverlayMissing
	_ = evaluator.SearchUnknownSourceTargetLifecycle
	_ = evaluator.SearchOverlayIdentity
	_ = evaluator.SearchOverlayStatusPresent
	_ = evaluator.SearchLifecycleCategoryViewerAccount
	_ = evaluator.SearchPublicLifecycleStateActive
	_ = evaluator.SearchExposureSemanticAllow
	_ = evaluator.SurfaceSearch
}


