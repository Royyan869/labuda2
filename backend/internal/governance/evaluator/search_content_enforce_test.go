package evaluator_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/governance/evaluator"
	"github.com/labuda/backend/internal/governance/viewercontext"
)

// PHASE 3B — EnforceSearchContent helper tests.
//
// These tests pin the helper's behavior across the full pilot decision
// matrix. They use the same test helpers as search_content_shadow_test.go
// (newAnonVC, newAuthVC, newAuthVCWithRelationship, newTargetCtx)
// imported from the same _test package — so the canonical
// EvaluateSearchContent decision precedence is reused verbatim.

// helper — builds a ContentPreview with the given IDs. The other fields
// are not consulted by EvaluateSearchContent.
func previewRow(authorID, contentID uuid.UUID) *entity.ContentPreview {
	return &entity.ContentPreview{ID: contentID, AuthorID: authorID}
}

// TestEnforceSearchContent_ShadowModeIsIdentity asserts the canonical
// pilot rollback contract: in shadow mode the helper is a no-op pass-
// through. Filtered = input slice (same backing array), overrides=nil,
// counts=0. This is what guarantees a clean env-flip rollback.
func TestEnforceSearchContent_ShadowModeIsIdentity(t *testing.T) {
	authorID := uuid.New()
	row1 := previewRow(authorID, uuid.New())
	row2 := previewRow(authorID, uuid.New())

	// Pass intentionally-incomplete contexts. In shadow mode the helper
	// MUST NOT call EvaluateSearchContent, so it MUST NOT care that the
	// contexts would otherwise yield UNKNOWN/input_invalid.
	res := evaluator.EnforceSearchContent(
		evaluator.SearchContentAdapterModeShadow,
		nil, nil,
		[]*entity.ContentPreview{row1, row2},
	)
	if len(res.Filtered) != 2 || res.Filtered[0] != row1 || res.Filtered[1] != row2 {
		t.Errorf("shadow mode: Filtered slice changed; pilot rollback contract is broken")
	}
	if res.LifecycleOverrides != nil {
		t.Errorf("shadow mode: LifecycleOverrides = %v; want nil", res.LifecycleOverrides)
	}
	if res.DroppedCount != 0 || res.OverriddenCount != 0 {
		t.Errorf("shadow mode: counts = (drop=%d, override=%d); want zero",
			res.DroppedCount, res.OverriddenCount)
	}
}

// TestEnforceSearchContent_EnforceDropsDenyRow asserts that DENY decisions
// produce row drops in enforce mode. A removed author triggers DENY per
// the canonical evaluator precedence.
func TestEnforceSearchContent_EnforceDropsDenyRow(t *testing.T) {
	authorRemoved := uuid.New()
	authorActive := uuid.New()
	rowRemoved := previewRow(authorRemoved, uuid.New())
	rowActive := previewRow(authorActive, uuid.New())

	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{
			authorRemoved: viewercontext.PublicLifecycleStateRemoved,
			authorActive:  viewercontext.PublicLifecycleStateActive,
		},
		map[uuid.UUID]viewercontext.ContentModerationState{
			rowRemoved.ID: viewercontext.ContentModerationStateVisible,
			rowActive.ID:  viewercontext.ContentModerationStateVisible,
		},
	)

	res := evaluator.EnforceSearchContent(
		evaluator.SearchContentAdapterModeEnforce,
		newAnonVC(), tc,
		[]*entity.ContentPreview{rowRemoved, rowActive},
	)
	if len(res.Filtered) != 1 {
		t.Fatalf("enforce: expected 1 row after DENY drop, got %d", len(res.Filtered))
	}
	if res.Filtered[0] != rowActive {
		t.Errorf("enforce: surviving row should be active author's row")
	}
	if res.DroppedCount != 1 {
		t.Errorf("enforce: DroppedCount = %d; want 1", res.DroppedCount)
	}
	if res.OverriddenCount != 0 {
		t.Errorf("enforce: OverriddenCount = %d; want 0", res.OverriddenCount)
	}
}

// TestEnforceSearchContent_EnforceTombstoneOverridesLifecycle asserts that
// a hidden content (ContentModerationStateHidden) produces a TOMBSTONE
// adapter decision, which keeps the row in the response but coarsens
// its lifecycle to "removed".
func TestEnforceSearchContent_EnforceTombstoneOverridesLifecycle(t *testing.T) {
	authorID := uuid.New()
	rowHidden := previewRow(authorID, uuid.New())

	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		map[uuid.UUID]viewercontext.ContentModerationState{rowHidden.ID: viewercontext.ContentModerationStateHidden},
	)

	res := evaluator.EnforceSearchContent(
		evaluator.SearchContentAdapterModeEnforce,
		newAnonVC(), tc,
		[]*entity.ContentPreview{rowHidden},
	)
	if len(res.Filtered) != 1 {
		t.Fatalf("enforce/tombstone: expected 1 row (kept), got %d", len(res.Filtered))
	}
	if res.LifecycleOverrides == nil {
		t.Fatalf("enforce/tombstone: LifecycleOverrides = nil; want map with row entry")
	}
	got, ok := res.LifecycleOverrides[rowHidden.ID]
	if !ok {
		t.Fatalf("enforce/tombstone: row missing from LifecycleOverrides map")
	}
	if got != evaluator.SearchContentLifecycleRemoved {
		t.Errorf("enforce/tombstone: lifecycle = %q; want %q", got, evaluator.SearchContentLifecycleRemoved)
	}
	if res.OverriddenCount != 1 || res.DroppedCount != 0 {
		t.Errorf("enforce/tombstone: counts = (drop=%d, override=%d); want (0, 1)",
			res.DroppedCount, res.OverriddenCount)
	}
}

// TestEnforceSearchContent_EnforceUnknownOverlayFailsOpen asserts the
// audit doctrine fail-open path: when overlays are missing (a transient
// hydration race), the row is included and no lifecycle override is
// emitted. This is the canonical "incomplete overlay ≠ proof of denial"
// behavior — without it, enforce mode would silently censor on every
// transient hydration glitch.
func TestEnforceSearchContent_EnforceUnknownOverlayFailsOpen(t *testing.T) {
	authorID := uuid.New()
	row := previewRow(authorID, uuid.New())

	// Target context with NO author lifecycle hydrated → evaluator emits
	// UNKNOWN/target_overlay_missing → adapter classifies as
	// unknown_fail_open with Include=true.
	tcEmpty := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{},
		map[uuid.UUID]viewercontext.ContentModerationState{},
	)

	res := evaluator.EnforceSearchContent(
		evaluator.SearchContentAdapterModeEnforce,
		newAnonVC(), tcEmpty,
		[]*entity.ContentPreview{row},
	)
	if len(res.Filtered) != 1 {
		t.Errorf("enforce/unknown overlay: expected fail-open (1 row), got %d", len(res.Filtered))
	}
	if res.LifecycleOverrides != nil {
		t.Errorf("enforce/unknown overlay: LifecycleOverrides = %v; want nil", res.LifecycleOverrides)
	}
	if res.DroppedCount != 0 {
		t.Errorf("enforce/unknown overlay: DroppedCount = %d; want 0", res.DroppedCount)
	}
}

// TestEnforceSearchContent_EnforceInputInvalidFailsClosed asserts the
// other UNKNOWN branch: an evaluator construction defect (nil
// ViewerContext) causes EVERY row to be dropped. This is the safe-
// default — a handler-side bug must not silently expose unmoderated
// content if enforce mode is on.
func TestEnforceSearchContent_EnforceInputInvalidFailsClosed(t *testing.T) {
	authorID := uuid.New()
	row1 := previewRow(authorID, uuid.New())
	row2 := previewRow(authorID, uuid.New())

	res := evaluator.EnforceSearchContent(
		evaluator.SearchContentAdapterModeEnforce,
		nil, nil, // construction defect — nil VC
		[]*entity.ContentPreview{row1, row2},
	)
	if len(res.Filtered) != 0 {
		t.Errorf("enforce/input_invalid: expected fail-closed (0 rows), got %d", len(res.Filtered))
	}
	if res.DroppedCount != 2 {
		t.Errorf("enforce/input_invalid: DroppedCount = %d; want 2 (both rows fail-closed)", res.DroppedCount)
	}
}

// TestEnforceSearchContent_EnforceAllowsHappyPath asserts the dominant
// case is preserved end-to-end: hydrated overlays + active author +
// visible content + non-blocked relationship → row passes through with
// no override.
func TestEnforceSearchContent_EnforceAllowsHappyPath(t *testing.T) {
	authorID := uuid.New()
	row := previewRow(authorID, uuid.New())

	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateActive},
		map[uuid.UUID]viewercontext.ContentModerationState{row.ID: viewercontext.ContentModerationStateVisible},
	)

	res := evaluator.EnforceSearchContent(
		evaluator.SearchContentAdapterModeEnforce,
		newAnonVC(), tc,
		[]*entity.ContentPreview{row},
	)
	if len(res.Filtered) != 1 || res.Filtered[0] != row {
		t.Errorf("enforce/allow: expected pass-through; got len=%d", len(res.Filtered))
	}
	if res.LifecycleOverrides != nil {
		t.Errorf("enforce/allow: LifecycleOverrides = %v; want nil", res.LifecycleOverrides)
	}
	if res.DroppedCount != 0 || res.OverriddenCount != 0 {
		t.Errorf("enforce/allow: counts = (drop=%d, override=%d); want zero", res.DroppedCount, res.OverriddenCount)
	}
}

// TestEnforceSearchContent_EnforceBlockedAuthorDropsRow asserts the
// authenticated-viewer relationship gate: a row whose author is in the
// viewer's bidirectional block set yields DENY and is dropped.
func TestEnforceSearchContent_EnforceBlockedAuthorDropsRow(t *testing.T) {
	viewerID := uuid.New()
	blockedAuthor := uuid.New()
	allowedAuthor := uuid.New()
	rowBlocked := previewRow(blockedAuthor, uuid.New())
	rowAllowed := previewRow(allowedAuthor, uuid.New())

	vc := newAuthVCWithRelationship(viewerID, []uuid.UUID{blockedAuthor})
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{
			blockedAuthor: viewercontext.PublicLifecycleStateActive,
			allowedAuthor: viewercontext.PublicLifecycleStateActive,
		},
		map[uuid.UUID]viewercontext.ContentModerationState{
			rowBlocked.ID: viewercontext.ContentModerationStateVisible,
			rowAllowed.ID: viewercontext.ContentModerationStateVisible,
		},
	)

	res := evaluator.EnforceSearchContent(
		evaluator.SearchContentAdapterModeEnforce,
		vc, tc,
		[]*entity.ContentPreview{rowBlocked, rowAllowed},
	)
	if len(res.Filtered) != 1 {
		t.Fatalf("enforce/blocked: expected 1 row after block drop, got %d", len(res.Filtered))
	}
	if res.Filtered[0] != rowAllowed {
		t.Errorf("enforce/blocked: wrong row survived — expected non-blocked author's row")
	}
}

// TestEnforceSearchContent_NilContents asserts the helper handles a nil
// or empty contents slice without panic. Both modes should return a
// zero-value-ish result.
func TestEnforceSearchContent_NilContents(t *testing.T) {
	shadow := evaluator.EnforceSearchContent(
		evaluator.SearchContentAdapterModeShadow,
		newAnonVC(), nil, nil,
	)
	if len(shadow.Filtered) != 0 {
		t.Errorf("shadow/nil contents: Filtered len = %d; want 0", len(shadow.Filtered))
	}

	enforce := evaluator.EnforceSearchContent(
		evaluator.SearchContentAdapterModeEnforce,
		newAnonVC(), nil, nil,
	)
	if len(enforce.Filtered) != 0 {
		t.Errorf("enforce/nil contents: Filtered len = %d; want 0", len(enforce.Filtered))
	}
	if enforce.DroppedCount != 0 || enforce.OverriddenCount != 0 {
		t.Errorf("enforce/nil contents: counts = (%d, %d); want zero", enforce.DroppedCount, enforce.OverriddenCount)
	}
}

// TestEnforceSearchContent_InvalidModeFallsToShadow asserts the
// safety-default: any non-canonical mode string is treated as shadow.
// This belt-and-suspenders the config-layer NormalizeSearchContentAdapterMode
// — even if a caller bypasses the normalizer and passes a custom mode
// string, the helper still defaults to shadow (identity pass-through).
func TestEnforceSearchContent_InvalidModeFallsToShadow(t *testing.T) {
	authorID := uuid.New()
	rowRemoved := previewRow(authorID, uuid.New())
	tc := newTargetCtx(
		map[uuid.UUID]viewercontext.PublicLifecycleState{authorID: viewercontext.PublicLifecycleStateRemoved},
		map[uuid.UUID]viewercontext.ContentModerationState{rowRemoved.ID: viewercontext.ContentModerationStateVisible},
	)

	res := evaluator.EnforceSearchContent(
		evaluator.SearchContentAdapterMode("garbage"),
		newAnonVC(), tc,
		[]*entity.ContentPreview{rowRemoved},
	)
	if len(res.Filtered) != 1 {
		t.Errorf("invalid mode: expected pass-through (1 row), got %d", len(res.Filtered))
	}
}


