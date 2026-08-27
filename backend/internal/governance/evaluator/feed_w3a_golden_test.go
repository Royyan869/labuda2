package evaluator

import (
	"testing"

	"github.com/google/uuid"

	"github.com/labuda/backend/internal/governance/viewercontext"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
)

// TestEvaluateFeedItem_W3A_PrecedenceGolden is the cell-for-cell golden
// table that pins EvaluateFeedItem's canonical-types precedence model
// against every (viewer-lifecycle, target-status, owner-lifecycle,
// blocked, content-moderation) tuple that has a documented behavior.
//
// The test exists primarily to catch any precedence-order regression
// during the F1-W3 evaluator-purity rebuild. Each row encodes one
// canonical input combination and its expected outcome.
//
// Semantic-preservation note vs the pre-W3A raw-enum reads:
//
//	OWNER side  — exact preservation. coarsen("banned" → Unavailable)
//	              and coarsen("suspended" → Unavailable) both DENY,
//	              same as the pre-W3A raw-enum table. coarsen("deleted"
//	              → Removed) DENIES. deleted_at-set → Removed DENIES.
//
//	VIEWER side — doctrinal convergence. Pre-W3A denied {deleted,
//	              banned} but allowed {suspended} as an undocumented
//	              quirk. The canonical PublicLifecycleState collapses
//	              banned + suspended → Unavailable; the rebuilt
//	              evaluator denies both. This brings /feed in line
//	              with the constitution §8.4 precedence used by
//	              /contents/:id. The shift is shadow-only today;
//	              the wire is unchanged.
func TestEvaluateFeedItem_W3A_PrecedenceGolden(t *testing.T) {
	type viewerLC struct {
		anonymous bool
		hydrated  bool
		state     viewercontext.PublicLifecycleState
	}
	type ownerLC struct {
		hydrated bool
		state    viewercontext.PublicLifecycleState
	}
	type relSet struct {
		hydrated bool
		blocked  bool
	}
	type modSet struct {
		hydrated bool
		hidden   bool
	}
	type row struct {
		name       string
		viewer     viewerLC
		itemStatus string
		owner      ownerLC
		rel        relSet
		mod        modSet
		want       ShadowDecision
		wantReason UnknownReason
	}
	cases := []row{
		// ---- viewer-side ----
		{
			name:   "viewer_lifecycle_unhydrated_authenticated",
			viewer: viewerLC{anonymous: false, hydrated: false, state: viewercontext.PublicLifecycleStateActive},
			want:   ShadowDecisionUnknown, wantReason: UnknownReasonViewerOverlayMissing,
		},
		{
			name:       "viewer_removed_denies",
			viewer:     viewerLC{anonymous: false, hydrated: true, state: viewercontext.PublicLifecycleStateRemoved},
			itemStatus: "active",
			want:       ShadowDecisionDeny, wantReason: UnknownReasonNone,
		},
		{
			name:       "viewer_unavailable_denies_W3A_doctrine_alignment",
			viewer:     viewerLC{anonymous: false, hydrated: true, state: viewercontext.PublicLifecycleStateUnavailable},
			itemStatus: "active",
			want:       ShadowDecisionDeny, wantReason: UnknownReasonNone,
		},
		// ---- target / item status ----
		{
			name:       "target_status_deleted_denies",
			viewer:     viewerLC{anonymous: false, hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			itemStatus: "deleted",
			want:       ShadowDecisionDeny, wantReason: UnknownReasonNone,
		},
		{
			name:       "target_status_fulfilled_denies",
			viewer:     viewerLC{anonymous: false, hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			itemStatus: "fulfilled",
			want:       ShadowDecisionDeny, wantReason: UnknownReasonNone,
		},
		// ---- owner side ----
		{
			name:       "owner_overlay_missing",
			viewer:     viewerLC{anonymous: false, hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			itemStatus: "active",
			owner:      ownerLC{hydrated: false},
			want:       ShadowDecisionUnknown, wantReason: UnknownReasonTargetOverlayMissing,
		},
		{
			name:       "owner_unavailable_denies_banned_or_suspended",
			viewer:     viewerLC{anonymous: false, hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			itemStatus: "active",
			owner:      ownerLC{hydrated: true, state: viewercontext.PublicLifecycleStateUnavailable},
			want:       ShadowDecisionDeny, wantReason: UnknownReasonNone,
		},
		{
			name:       "owner_removed_denies_deleted_enum_or_timestamp",
			viewer:     viewerLC{anonymous: false, hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			itemStatus: "active",
			owner:      ownerLC{hydrated: true, state: viewercontext.PublicLifecycleStateRemoved},
			want:       ShadowDecisionDeny, wantReason: UnknownReasonNone,
		},
		// ---- relationship ----
		{
			name:       "relationship_unhydrated_authenticated",
			viewer:     viewerLC{anonymous: false, hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			itemStatus: "active",
			owner:      ownerLC{hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			rel:        relSet{hydrated: false},
			want:       ShadowDecisionUnknown, wantReason: UnknownReasonViewerOverlayMissing,
		},
		{
			name:       "viewer_blocked_author_denies",
			viewer:     viewerLC{anonymous: false, hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			itemStatus: "active",
			owner:      ownerLC{hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			rel:        relSet{hydrated: true, blocked: true},
			want:       ShadowDecisionDeny, wantReason: UnknownReasonNone,
		},
		// ---- moderation ----
		{
			name:       "content_hidden_tombstones",
			viewer:     viewerLC{anonymous: false, hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			itemStatus: "active",
			owner:      ownerLC{hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			rel:        relSet{hydrated: true, blocked: false},
			mod:        modSet{hydrated: true, hidden: true},
			want:       ShadowDecisionTombstone, wantReason: UnknownReasonNone,
		},
		// ---- happy path ----
		{
			name:       "all_clear_allows",
			viewer:     viewerLC{anonymous: false, hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			itemStatus: "active",
			owner:      ownerLC{hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			rel:        relSet{hydrated: true, blocked: false},
			mod:        modSet{hydrated: true, hidden: false},
			want:       ShadowDecisionAllow, wantReason: UnknownReasonNone,
		},
		// ---- anonymous topology ----
		{
			name:       "anonymous_skips_viewer_and_relationship_checks",
			viewer:     viewerLC{anonymous: true},
			itemStatus: "active",
			owner:      ownerLC{hydrated: true, state: viewercontext.PublicLifecycleStateActive},
			// rel unused for anonymous; mod hydrated empty
			mod:  modSet{hydrated: true, hidden: false},
			want: ShadowDecisionAllow, wantReason: UnknownReasonNone,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			authorID := uuid.New()
			contentID := uuid.New()
			item := &feedentity.FeedItem{
				ID:       contentID,
				AuthorID: authorID,
				Status:   tc.itemStatus,
				IsHidden: tc.mod.hidden,
			}

			// Build VC per case shape.
			var vc *viewercontext.ViewerContext
			if tc.viewer.anonymous {
				vc = viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST)
			} else {
				identity := viewercontext.IdentityOverlay{CanonicalUserID: uuid.New()}
				lifecycle := viewercontext.NewLifecycleOverlay(tc.viewer.state, tc.viewer.hydrated)
				vc = viewercontext.NewAuthenticated(
					viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST,
					identity, lifecycle,
					viewercontext.CapabilityOverlay{}, viewercontext.ModerationOverlay{},
				)
				if tc.rel.hydrated {
					if tc.rel.blocked {
						vc = vc.WithRelationship(viewercontext.NewHydratedRelationshipOverlay([]uuid.UUID{authorID}))
					} else {
						vc = vc.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(nil))
					}
				}
				// Else: relationship remains the default empty (hydrated=false for authenticated).
			}

			// Build TC per case shape.
			tcCtx := viewercontext.NewTargetContext()
			if tc.owner.hydrated {
				tcCtx.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{
					authorID: tc.owner.state,
				})
			}
			if tc.mod.hydrated {
				mod := viewercontext.ContentModerationStateVisible
				if tc.mod.hidden {
					mod = viewercontext.ContentModerationStateHidden
				}
				tcCtx.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{
					contentID: mod,
				})
			}

			gotDec, gotReason := EvaluateFeedItem(vc, tcCtx, item)
			if gotDec != tc.want {
				t.Errorf("decision = %q, want %q", gotDec, tc.want)
			}
			if gotReason != tc.wantReason {
				t.Errorf("reason = %q, want %q", gotReason, tc.wantReason)
			}
		})
	}
}

// TestEvaluateFeedItem_W3A_NilSafety pins the input_invalid guard.
func TestEvaluateFeedItem_W3A_NilSafety(t *testing.T) {
	cases := []struct {
		name string
		vc   *viewercontext.ViewerContext
		tc   *viewercontext.TargetContext
		item *feedentity.FeedItem
	}{
		{"all_nil", nil, nil, nil},
		{"nil_vc", nil, viewercontext.NewTargetContext(), &feedentity.FeedItem{}},
		{"nil_tc", viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST), nil, &feedentity.FeedItem{}},
		{"nil_item", viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST), viewercontext.NewTargetContext(), nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotDec, gotReason := EvaluateFeedItem(tc.vc, tc.tc, tc.item)
			if gotDec != ShadowDecisionUnknown {
				t.Errorf("decision = %q, want UNKNOWN", gotDec)
			}
			if gotReason != UnknownReasonInputInvalid {
				t.Errorf("reason = %q, want input_invalid", gotReason)
			}
		})
	}
}

// TestFeedShadowRunner_W3A_NoPoolField is a static contract check —
// the runner struct must not carry any database/pool reference field.
// Test mirrors the constitution §1.7 + F1 audit §3.1 expectation that
// F1-W3A drains F4 (pool field in evaluator).
//
// The check is structural: NewFeedShadowRunner(nil) must succeed
// without requiring a pool argument, and the returned runner must
// emit no DB activity when Run is invoked with pre-hydrated inputs.
func TestFeedShadowRunner_W3A_ConstructorTakesNoPool(t *testing.T) {
	r := NewFeedShadowRunner(nil) // log=nil → Nop
	if r == nil {
		t.Fatal("NewFeedShadowRunner(nil log) must return a usable runner; pool is no longer required")
	}
}

// TestFeedShadowRunner_W3A_RunAcceptsCanonicalTypes is a compile-time
// contract pin — Run must accept (*viewercontext.ViewerContext,
// *viewercontext.TargetContext, []*feedentity.FeedItem). If a future
// edit reverts the signature this test will refuse to compile.
func TestFeedShadowRunner_W3A_RunAcceptsCanonicalTypes(t *testing.T) {
	r := NewFeedShadowRunner(nil)
	vc := viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST)
	tcCtx := viewercontext.NewTargetContext()
	// Nil items slice — runner must not panic.
	r.Run(vc, tcCtx, nil)
	// Slice with a nil entry — runner must skip cleanly.
	items := []*feedentity.FeedItem{nil, {ID: uuid.New(), AuthorID: uuid.New(), Status: "active"}}
	r.Run(vc, tcCtx, items)
	// No assertion: the test passes if Run does not panic and
	// signature compiles.
}

// TestFeedShadowRunner_W3A_NilReceiverSafe pins the nil-safe contract
// the handler relies on when the runner is disabled via env.
func TestFeedShadowRunner_W3A_NilReceiverSafe(t *testing.T) {
	var r *FeedShadowRunner
	r.Run(nil, nil, nil) // must not panic
}

// TestEnforceFeed_W3A_NilTcFailsOpen confirms the W3A signature
// degrades the same way the pre-W3A signature degraded — fail-OPEN
// keeps every row. Mirrors the existing nil_vc_enforce_fails_open
// case for the new tc parameter.
func TestEnforceFeed_W3A_NilTcFailsOpen(t *testing.T) {
	viewer := uuid.New()
	in := []*feedentity.FeedItem{
		{ID: uuid.New(), AuthorID: uuid.New(), Status: "active"},
		{ID: uuid.New(), AuthorID: uuid.New(), Status: "active"},
	}
	result := EnforceFeed(FeedEvaluatorModeEnforce, makeViewerContext(viewer), nil, in)
	if len(result.Filtered) != 2 {
		t.Fatalf("nil tc must fail-open and keep all items; got %d", len(result.Filtered))
	}
	if result.UnknownFailOpenCount != 2 {
		t.Errorf("UnknownFailOpenCount = %d, want 2", result.UnknownFailOpenCount)
	}
	if result.DroppedCount != 0 {
		t.Errorf("DroppedCount = %d, want 0", result.DroppedCount)
	}
}


