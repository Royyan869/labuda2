package evaluator

import (
	"testing"

	"github.com/google/uuid"

	"github.com/labuda/backend/internal/governance/viewercontext"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
)

// makeViewerContext builds a fully-hydrated canonical ViewerContext
// (active viewer, hydrated relationship overlay with empty BlockedSet).
// F1-W3A canonical shape — evaluator reads only pre-hydrated overlays.
func makeViewerContext(viewerID uuid.UUID) *viewercontext.ViewerContext {
	identity := viewercontext.IdentityOverlay{CanonicalUserID: viewerID}
	lifecycle := viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, true)
	vc := viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery,
		viewercontext.RequestOriginREST,
		identity,
		lifecycle,
		viewercontext.CapabilityOverlay{},
		viewercontext.ModerationOverlay{},
	)
	return vc.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(nil))
}

// makeTargetContext builds the canonical TargetContext a fully-hydrated
// handler-side batch would produce — every author "active", every
// content row "visible". The TC carries both per-author lifecycle AND
// per-content moderation; tests that exercise the hidden / non-active
// branches override the relevant cells via WithAuthorLifecycle /
// WithContentModeration after construction.
func makeTargetContext(items ...*feedentity.FeedItem) *viewercontext.TargetContext {
	tc := viewercontext.NewTargetContext()
	authorLC := make(map[uuid.UUID]viewercontext.PublicLifecycleState, len(items))
	contentMod := make(map[uuid.UUID]viewercontext.ContentModerationState, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.AuthorID != uuid.Nil {
			authorLC[item.AuthorID] = viewercontext.PublicLifecycleStateActive
		}
		if item.ID != uuid.Nil {
			if item.IsHidden {
				contentMod[item.ID] = viewercontext.ContentModerationStateHidden
			} else {
				contentMod[item.ID] = viewercontext.ContentModerationStateVisible
			}
		}
	}
	tc.WithAuthorLifecycle(authorLC)
	tc.WithContentModeration(contentMod)
	return tc
}

// makeTargetContextWithAuthors constructs a TC with per-author
// lifecycle states explicitly specified. Used by tests that exercise
// degraded-owner branches (banned / suspended / removed). Every
// content row is marked visible by default.
func makeTargetContextWithAuthors(
	authorStates map[uuid.UUID]viewercontext.PublicLifecycleState,
	items ...*feedentity.FeedItem,
) *viewercontext.TargetContext {
	tc := viewercontext.NewTargetContext()
	tc.WithAuthorLifecycle(authorStates)
	contentMod := make(map[uuid.UUID]viewercontext.ContentModerationState, len(items))
	for _, item := range items {
		if item == nil || item.ID == uuid.Nil {
			continue
		}
		if item.IsHidden {
			contentMod[item.ID] = viewercontext.ContentModerationStateHidden
		} else {
			contentMod[item.ID] = viewercontext.ContentModerationStateVisible
		}
	}
	tc.WithContentModeration(contentMod)
	return tc
}

// makeFeedItem builds a minimal FeedItem with the fields the evaluator
// actually consumes (ID, AuthorID, Status, IsHidden). Other fields are
// left zero — the evaluator does not read them.
func makeFeedItem(id, authorID uuid.UUID, status string, isHidden bool) *feedentity.FeedItem {
	return &feedentity.FeedItem{
		ID:       id,
		AuthorID: authorID,
		Status:   status,
		IsHidden: isHidden,
	}
}

// TestEnforceFeed_ShadowModeIdentityPassthrough — spec §E.1.
// shadow mode returns the input slice unchanged, with zero counts,
// regardless of what the items look like.
func TestEnforceFeed_ShadowModeIdentityPassthrough(t *testing.T) {
	viewer := uuid.New()
	authorA := uuid.New()
	authorB := uuid.New()
	in := []*feedentity.FeedItem{
		makeFeedItem(uuid.New(), authorA, "active", true),         // would-DENY/TOMBSTONE in enforce
		makeFeedItem(uuid.New(), authorB, "deleted", false),       // would-DENY in enforce
		makeFeedItem(uuid.New(), authorA, "active", false),
	}

	result := EnforceFeed(
		FeedEvaluatorModeShadow,
		makeViewerContext(viewer),
		makeTargetContext(in...),
		in,
	)

	if len(result.Filtered) != len(in) {
		t.Fatalf("shadow mode must return all items unchanged: got %d, want %d", len(result.Filtered), len(in))
	}
	for i := range in {
		if result.Filtered[i] != in[i] {
			t.Errorf("shadow mode must preserve slice identity at index %d", i)
		}
	}
	if result.DroppedCount != 0 || result.UnknownFailOpenCount != 0 {
		t.Errorf("shadow mode must record zero counts; got dropped=%d unknown=%d",
			result.DroppedCount, result.UnknownFailOpenCount)
	}
}

// TestEnforceFeed_EnforceTombstonesHiddenItem — spec §E.2 (C1 convergence).
// In enforce mode an item with IsHidden=true is TOMBSTONED, not dropped:
// the row remains in the response with a "removed" lifecycle override so
// the mobile lifecycle parser can render it accordingly. This is the C1
// transport-parity behavior — collapse-to-drop was the pre-convergence
// limitation explicitly retired in this batch (mirror of
// /search/content's enforcement.LifecycleOverrides path).
func TestEnforceFeed_EnforceTombstonesHiddenItem(t *testing.T) {
	viewer := uuid.New()
	author := uuid.New()
	keepID := uuid.New()
	hiddenID := uuid.New()
	in := []*feedentity.FeedItem{
		makeFeedItem(keepID, author, "active", false),
		makeFeedItem(hiddenID, author, "active", true),
	}

	result := EnforceFeed(
		FeedEvaluatorModeEnforce,
		makeViewerContext(viewer),
		makeTargetContext(in...),
		in,
	)

	if len(result.Filtered) != 2 {
		t.Fatalf("expected 2 items (TOMBSTONE now keeps + overrides), got %d", len(result.Filtered))
	}
	if result.DroppedCount != 0 {
		t.Errorf("DroppedCount = %d, want 0 (TOMBSTONE no longer drops)", result.DroppedCount)
	}
	if result.OverriddenCount != 1 {
		t.Errorf("OverriddenCount = %d, want 1", result.OverriddenCount)
	}
	if got, ok := result.LifecycleOverrides[hiddenID]; !ok || got != FeedLifecycleRemoved {
		t.Errorf("LifecycleOverrides[hidden] = %q ok=%v, want %q true", got, ok, FeedLifecycleRemoved)
	}
	if _, ok := result.LifecycleOverrides[keepID]; ok {
		t.Errorf("LifecycleOverrides must not contain the active row")
	}
	if result.UnknownFailOpenCount != 0 {
		t.Errorf("UnknownFailOpenCount = %d, want 0", result.UnknownFailOpenCount)
	}
}

// TestEnforceFeed_EnforceDropsSuspendedBannedDeletedOwners — spec §E.3.
// In enforce mode items whose author's lifecycle is in {suspended,
// banned, deleted-via-flag, deleted-via-deleted_at} are dropped.
func TestEnforceFeed_EnforceDropsSuspendedBannedDeletedOwners(t *testing.T) {
	viewer := uuid.New()
	activeAuthor := uuid.New()
	suspendedAuthor := uuid.New()
	bannedAuthor := uuid.New()
	deletedFlagAuthor := uuid.New()
	deletedTimestampAuthor := uuid.New()

	keepID := uuid.New()
	in := []*feedentity.FeedItem{
		makeFeedItem(keepID, activeAuthor, "active", false),
		makeFeedItem(uuid.New(), suspendedAuthor, "active", false),
		makeFeedItem(uuid.New(), bannedAuthor, "active", false),
		makeFeedItem(uuid.New(), deletedFlagAuthor, "active", false),
		makeFeedItem(uuid.New(), deletedTimestampAuthor, "active", false),
	}

	// F1-W3A — the canonical TargetContext carries the COARSENED
	// PublicLifecycleState (no raw "banned"/"suspended" strings).
	// All four degraded states coarsen to either Unavailable
	// (banned/suspended) or Removed (deleted enum / deleted_at).
	// EvaluateFeedItem treats both as DENY — same outcome as the
	// pre-W3A raw-enum branch.
	authorStates := map[uuid.UUID]viewercontext.PublicLifecycleState{
		activeAuthor:           viewercontext.PublicLifecycleStateActive,
		suspendedAuthor:        viewercontext.PublicLifecycleStateUnavailable, // coarsen("suspended", false)
		bannedAuthor:           viewercontext.PublicLifecycleStateUnavailable, // coarsen("banned", false)
		deletedFlagAuthor:      viewercontext.PublicLifecycleStateRemoved,     // coarsen("deleted", false)
		deletedTimestampAuthor: viewercontext.PublicLifecycleStateRemoved,     // coarsen("active", true)
	}

	result := EnforceFeed(
		FeedEvaluatorModeEnforce,
		makeViewerContext(viewer),
		makeTargetContextWithAuthors(authorStates, in...),
		in,
	)

	if len(result.Filtered) != 1 {
		t.Fatalf("expected 1 kept item; got %d", len(result.Filtered))
	}
	if result.Filtered[0].ID != keepID {
		t.Errorf("expected only the active-author item to remain; got %s", result.Filtered[0].ID)
	}
	if result.DroppedCount != 4 {
		t.Errorf("DroppedCount = %d, want 4", result.DroppedCount)
	}
}

// TestEnforceFeed_EnforceKeepsAllowedItem — spec §E.4.
// Plain ALLOW item passes through unchanged.
func TestEnforceFeed_EnforceKeepsAllowedItem(t *testing.T) {
	viewer := uuid.New()
	author := uuid.New()
	in := []*feedentity.FeedItem{
		makeFeedItem(uuid.New(), author, "active", false),
	}

	result := EnforceFeed(
		FeedEvaluatorModeEnforce,
		makeViewerContext(viewer),
		makeTargetContext(in...),
		in,
	)

	if len(result.Filtered) != 1 {
		t.Fatalf("expected 1 kept item; got %d", len(result.Filtered))
	}
	if result.Filtered[0] != in[0] {
		t.Errorf("kept item must be the same pointer as the input")
	}
	if result.DroppedCount != 0 || result.UnknownFailOpenCount != 0 {
		t.Errorf("ALLOW must record zero drops / unknowns; got dropped=%d unknown=%d",
			result.DroppedCount, result.UnknownFailOpenCount)
	}
}

// TestEnforceFeed_EnforceUnknownFailsOpen — spec §E.5.
// In enforce mode an UNKNOWN classification (here: missing owner
// lifecycle overlay → target_overlay_missing) keeps the item and
// records action="unknown_fail_open".
func TestEnforceFeed_EnforceUnknownFailsOpen(t *testing.T) {
	viewer := uuid.New()
	authorMissing := uuid.New()
	authorKnown := uuid.New()

	in := []*feedentity.FeedItem{
		makeFeedItem(uuid.New(), authorMissing, "active", false), // owner overlay absent → UNKNOWN
		makeFeedItem(uuid.New(), authorKnown, "active", false),   // owner overlay present → ALLOW
	}

	// F1-W3A — TC carries an explicit author-lifecycle map that
	// OMITS authorMissing. The evaluator sees the absent entry and
	// emits UNKNOWN/target_overlay_missing; the feed adapter
	// fail-OPENs and the row is kept.
	authorStates := map[uuid.UUID]viewercontext.PublicLifecycleState{
		// authorMissing intentionally absent
		authorKnown: viewercontext.PublicLifecycleStateActive,
	}

	result := EnforceFeed(
		FeedEvaluatorModeEnforce,
		makeViewerContext(viewer),
		makeTargetContextWithAuthors(authorStates, in...),
		in,
	)

	if len(result.Filtered) != 2 {
		t.Fatalf("UNKNOWN must be kept (fail-open); got %d items, want 2", len(result.Filtered))
	}
	if result.DroppedCount != 0 {
		t.Errorf("DroppedCount = %d, want 0 (UNKNOWN must NOT drop)", result.DroppedCount)
	}
	if result.UnknownFailOpenCount != 1 {
		t.Errorf("UnknownFailOpenCount = %d, want 1", result.UnknownFailOpenCount)
	}
}

// TestNormalizeFeedEvaluatorMode — spec §E.6.
// Invalid / empty / case-varied inputs all fall safe to shadow.
// Only the literal "enforce" (any case, with surrounding whitespace)
// activates enforce.
func TestNormalizeFeedEvaluatorMode(t *testing.T) {
	cases := []struct {
		in   string
		want FeedEvaluatorMode
	}{
		{"", FeedEvaluatorModeShadow},
		{"shadow", FeedEvaluatorModeShadow},
		{"SHADOW", FeedEvaluatorModeShadow},
		{" shadow ", FeedEvaluatorModeShadow},
		{"enforce", FeedEvaluatorModeEnforce},
		{"ENFORCE", FeedEvaluatorModeEnforce},
		{" enforce ", FeedEvaluatorModeEnforce},
		{"weird", FeedEvaluatorModeShadow},
		{"shadowmode", FeedEvaluatorModeShadow},
		{"true", FeedEvaluatorModeShadow},
		{"on", FeedEvaluatorModeShadow},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			if got := NormalizeFeedEvaluatorMode(tc.in); got != tc.want {
				t.Errorf("NormalizeFeedEvaluatorMode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	// Belt-and-suspenders: a directly-passed invalid mode value into
	// EnforceFeed must short-circuit to shadow behavior (identity
	// passthrough). This guards against future drift between the
	// normalizer and the helper.
	t.Run("invalid_mode_passed_directly", func(t *testing.T) {
		in := []*feedentity.FeedItem{
			makeFeedItem(uuid.New(), uuid.New(), "active", true), // would-DENY in enforce
		}
		result := EnforceFeed(FeedEvaluatorMode("garbage"), nil, nil, in)
		if len(result.Filtered) != 1 || result.Filtered[0] != in[0] {
			t.Errorf("invalid mode must behave as shadow (identity passthrough)")
		}
	})
}

// TestEnforceFeed_NilEmptyInputSafe — spec §E.7.
// nil items, empty items, and nil item entries inside the slice are
// all handled without panic; the result is well-formed.
func TestEnforceFeed_NilEmptyInputSafe(t *testing.T) {
	viewer := uuid.New()
	vc := makeViewerContext(viewer)

	t.Run("nil_items_shadow", func(t *testing.T) {
		result := EnforceFeed(FeedEvaluatorModeShadow, vc, nil, nil)
		if len(result.Filtered) != 0 {
			t.Errorf("expected empty filtered; got %d", len(result.Filtered))
		}
	})

	t.Run("nil_items_enforce", func(t *testing.T) {
		result := EnforceFeed(FeedEvaluatorModeEnforce, vc, nil, nil)
		if len(result.Filtered) != 0 {
			t.Errorf("expected empty filtered; got %d", len(result.Filtered))
		}
	})

	t.Run("empty_items_enforce", func(t *testing.T) {
		result := EnforceFeed(FeedEvaluatorModeEnforce, vc, viewercontext.NewTargetContext(), []*feedentity.FeedItem{})
		if len(result.Filtered) != 0 {
			t.Errorf("expected empty filtered; got %d", len(result.Filtered))
		}
	})

	t.Run("nil_entry_inside_slice_enforce", func(t *testing.T) {
		author := uuid.New()
		keepID := uuid.New()
		in := []*feedentity.FeedItem{
			nil,
			makeFeedItem(keepID, author, "active", false),
			nil,
		}
		result := EnforceFeed(
			FeedEvaluatorModeEnforce,
			vc,
			makeTargetContext(in...),
			in,
		)
		if len(result.Filtered) != 1 || result.Filtered[0].ID != keepID {
			t.Errorf("nil entries must be skipped; got len=%d", len(result.Filtered))
		}
	})

	t.Run("nil_vc_enforce_fails_open", func(t *testing.T) {
		// Without a viewer context every row classifies as
		// UNKNOWN/input_invalid. Per the fail-open contract for /feed
		// the rows must remain in the output and count under
		// unknown_fail_open — opposite of /search/content which
		// fails-closed in this scenario.
		author := uuid.New()
		in := []*feedentity.FeedItem{
			makeFeedItem(uuid.New(), author, "active", false),
			makeFeedItem(uuid.New(), author, "active", false),
		}
		result := EnforceFeed(FeedEvaluatorModeEnforce, nil, nil, in)
		if len(result.Filtered) != 2 {
			t.Errorf("nil vc must fail-open and keep all items; got %d", len(result.Filtered))
		}
		if result.UnknownFailOpenCount != 2 {
			t.Errorf("UnknownFailOpenCount = %d, want 2", result.UnknownFailOpenCount)
		}
		if result.DroppedCount != 0 {
			t.Errorf("DroppedCount = %d, want 0 (fail-open must not drop)", result.DroppedCount)
		}
	})
}

// TestFeedShadowRunner_ModeNilSafe verifies that the mode accessor is
// safe to call on a nil receiver (the handler relies on this when the
// runner is disabled via env: a nil runner is treated as shadow mode
// and the enforce branch is therefore skipped entirely).
func TestFeedShadowRunner_ModeNilSafe(t *testing.T) {
	var r *FeedShadowRunner
	if got := r.Mode(); got != FeedEvaluatorModeShadow {
		t.Errorf("nil runner Mode() = %q, want %q", got, FeedEvaluatorModeShadow)
	}
	// WithMode on a nil receiver must also be nil-safe — used by
	// dependency wiring before checking the env-gated enable flag.
	if got := r.WithMode(FeedEvaluatorModeEnforce); got != nil {
		t.Errorf("nil runner WithMode must return nil, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// VIEWER STATUS ALIGNMENT TESTS
// ---------------------------------------------------------------------------
//
// Matrix under test (enforce mode, viewer-side lifecycle gate):
//
//   viewer_lifecycle     | expected
//   ────────────────────-┼──────────────────
//   active               | ALLOW (items visible)
//   unavailable (banned) | DENY (items dropped)
//   unavailable (susp.)  | DENY (items dropped)
//   removed (deleted)    | DENY (items dropped)
//   shadow rollback      | items preserved (no enforcement)

// makeViewerContextWithLifecycle builds a hydrated ViewerContext with
// the specified lifecycle state. Used by viewer-status alignment tests.
func makeViewerContextWithLifecycle(viewerID uuid.UUID, state viewercontext.PublicLifecycleState) *viewercontext.ViewerContext {
	identity := viewercontext.IdentityOverlay{CanonicalUserID: viewerID}
	lifecycle := viewercontext.NewLifecycleOverlay(state, true)
	vc := viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery,
		viewercontext.RequestOriginREST,
		identity,
		lifecycle,
		viewercontext.CapabilityOverlay{},
		viewercontext.ModerationOverlay{},
	)
	return vc.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(nil))
}

func TestEnforceFeed_ActiveViewer_SeesFeed(t *testing.T) {
	viewer := uuid.New()
	author := uuid.New()
	in := []*feedentity.FeedItem{
		makeFeedItem(uuid.New(), author, "active", false),
		makeFeedItem(uuid.New(), author, "active", false),
	}

	vc := makeViewerContextWithLifecycle(viewer, viewercontext.PublicLifecycleStateActive)
	result := EnforceFeed(FeedEvaluatorModeEnforce, vc, makeTargetContext(in...), in)

	if len(result.Filtered) != 2 {
		t.Fatalf("active viewer should see all items; got %d, want 2", len(result.Filtered))
	}
	if result.DroppedCount != 0 {
		t.Errorf("DroppedCount = %d, want 0", result.DroppedCount)
	}
}

func TestEnforceFeed_SuspendedViewer_Denied(t *testing.T) {
	viewer := uuid.New()
	author := uuid.New()
	in := []*feedentity.FeedItem{
		makeFeedItem(uuid.New(), author, "active", false),
		makeFeedItem(uuid.New(), author, "active", false),
		makeFeedItem(uuid.New(), author, "active", false),
	}

	// Suspended coarsens to PublicLifecycleStateUnavailable
	vc := makeViewerContextWithLifecycle(viewer, viewercontext.PublicLifecycleStateUnavailable)
	result := EnforceFeed(FeedEvaluatorModeEnforce, vc, makeTargetContext(in...), in)

	if len(result.Filtered) != 0 {
		t.Fatalf("suspended viewer should see zero items; got %d", len(result.Filtered))
	}
	if result.DroppedCount != 3 {
		t.Errorf("DroppedCount = %d, want 3", result.DroppedCount)
	}
}

func TestEnforceFeed_BannedViewer_Denied(t *testing.T) {
	viewer := uuid.New()
	author := uuid.New()
	in := []*feedentity.FeedItem{
		makeFeedItem(uuid.New(), author, "active", false),
		makeFeedItem(uuid.New(), author, "active", false),
	}

	// Banned coarsens to PublicLifecycleStateUnavailable
	vc := makeViewerContextWithLifecycle(viewer, viewercontext.PublicLifecycleStateUnavailable)
	result := EnforceFeed(FeedEvaluatorModeEnforce, vc, makeTargetContext(in...), in)

	if len(result.Filtered) != 0 {
		t.Fatalf("banned viewer should see zero items; got %d", len(result.Filtered))
	}
	if result.DroppedCount != 2 {
		t.Errorf("DroppedCount = %d, want 2", result.DroppedCount)
	}
}

func TestEnforceFeed_RemovedViewer_Denied(t *testing.T) {
	viewer := uuid.New()
	author := uuid.New()
	in := []*feedentity.FeedItem{
		makeFeedItem(uuid.New(), author, "active", false),
		makeFeedItem(uuid.New(), author, "active", false),
	}

	// Removed (soft-deleted) maps to PublicLifecycleStateRemoved
	vc := makeViewerContextWithLifecycle(viewer, viewercontext.PublicLifecycleStateRemoved)
	result := EnforceFeed(FeedEvaluatorModeEnforce, vc, makeTargetContext(in...), in)

	if len(result.Filtered) != 0 {
		t.Fatalf("removed viewer should see zero items; got %d", len(result.Filtered))
	}
	if result.DroppedCount != 2 {
		t.Errorf("DroppedCount = %d, want 2", result.DroppedCount)
	}
}

func TestEnforceFeed_ShadowRollback_PreservesItems(t *testing.T) {
	viewer := uuid.New()
	author := uuid.New()
	in := []*feedentity.FeedItem{
		makeFeedItem(uuid.New(), author, "active", false),
		makeFeedItem(uuid.New(), author, "active", false),
	}

	// Viewer is suspended (would be DENY in enforce)
	vc := makeViewerContextWithLifecycle(viewer, viewercontext.PublicLifecycleStateUnavailable)

	// But mode is shadow — enforcement is disabled (env rollback)
	result := EnforceFeed(FeedEvaluatorModeShadow, vc, makeTargetContext(in...), in)

	if len(result.Filtered) != 2 {
		t.Fatalf("shadow mode must preserve all items regardless of viewer status; got %d, want 2", len(result.Filtered))
	}
	if result.DroppedCount != 0 {
		t.Errorf("shadow mode must record zero drops; got %d", result.DroppedCount)
	}
}


