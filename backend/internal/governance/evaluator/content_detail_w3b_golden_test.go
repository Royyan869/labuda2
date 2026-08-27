package evaluator

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/labuda/backend/internal/governance/viewercontext"
	contententity "github.com/labuda/backend/internal/social/content/entity"
)

// F1-W3B — closure tests that pin the canonical purity rebuild.
// content_detail_shadow_test.go covers per-cell decision precedence
// already; this file pins the structural contracts the rebuild
// established:
//
//   - the runner constructor must NOT require a pool argument,
//   - the runner Run must accept canonical types and be DB-free,
//   - the evaluator package must NOT re-introduce pgxpool imports
//     in the rebuilt content-detail files,
//   - block-overlay precedence is canonicalized at evaluator level,
//   - EnforceContentDetail nil-safety fail-CLOSES.

// TestEnforceContentDetail_W3B_NilInputsFailClosed pins the fail-CLOSED
// nil-safety contract: nil vc / tc / content all collapse to
// Allow=false in enforce mode. This is the doctrine §8.5 inversion of
// /feed's fail-OPEN policy.
func TestEnforceContentDetail_W3B_NilInputsFailClosed(t *testing.T) {
	cases := []struct {
		name    string
		vc      *viewercontext.ViewerContext
		tc      *viewercontext.TargetContext
		content *contententity.Content
	}{
		{"all_nil", nil, nil, nil},
		{"nil_vc", nil, viewercontext.NewTargetContext(), healthyContent()},
		{"nil_tc", healthyVC(), nil, healthyContent()},
		{"nil_content", healthyVC(), viewercontext.NewTargetContext(), nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			res := EnforceContentDetail(ContentDetailEvaluatorModeEnforce, tc.vc, tc.tc, tc.content)
			if res.Allow {
				t.Errorf("Allow = true, want false (fail-CLOSED on nil input)")
			}
			if res.Reason != ContentDetailDecisionReasonUnknownFailClosed {
				t.Errorf("Reason = %q, want unknown_fail_closed", res.Reason)
			}
		})
	}
}

// TestEnforceContentDetail_W3B_ShadowModeAllowsRegardless pins the
// rollback contract: shadow mode short-circuits to Allow=true
// regardless of decision outcome.
func TestEnforceContentDetail_W3B_ShadowModeAllowsRegardless(t *testing.T) {
	// Build inputs that WOULD deny in enforce mode (blocked viewer).
	vc := makeContentDetailVC(cdViewerOpts{
		lifecycleHydrated:    true,
		relationshipHydrated: true,
		blocked:              true,
	})
	tc := makeContentDetailTC(cdTargetOpts{ownerHydrated: true, moderationHydrated: true})
	content := healthyContent()
	res := EnforceContentDetail(ContentDetailEvaluatorModeShadow, vc, tc, content)
	if !res.Allow {
		t.Errorf("shadow mode must short-circuit to Allow=true; got %+v", res)
	}
	if res.Reason != ContentDetailDecisionReasonNone {
		t.Errorf("shadow mode reason = %q, want none", res.Reason)
	}
}

// TestContentDetailShadowRunner_W3B_ConstructorTakesNoPool is a static
// contract check — the runner constructor must NOT require a pool
// argument. F4 closure (no DB pool in evaluator).
func TestContentDetailShadowRunner_W3B_ConstructorTakesNoPool(t *testing.T) {
	r := NewContentDetailShadowRunner(nil) // log=nil → Nop
	if r == nil {
		t.Fatal("NewContentDetailShadowRunner(nil log) must return a usable runner; pool is no longer required")
	}
}

// TestContentDetailShadowRunner_W3B_RunAcceptsCanonicalTypes is a
// compile-time contract pin — Run must accept (*viewercontext.ViewerContext,
// *viewercontext.TargetContext, *contententity.Content, LegacyContentDetailOutcome).
// If a future edit reverts the signature this test will refuse to compile.
func TestContentDetailShadowRunner_W3B_RunAcceptsCanonicalTypes(t *testing.T) {
	r := NewContentDetailShadowRunner(nil)
	vc := viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST)
	tc := viewercontext.NewTargetContext()
	r.Run(vc, tc, healthyContent(), LegacyContentDetailOutcome200)
	r.Run(vc, tc, nil, LegacyContentDetailOutcome404)
	// No assertion: the test passes if Run does not panic and
	// signature compiles.
}

// TestContentDetailShadowRunner_W3B_NilReceiverSafe pins the nil-safe
// contract the handler relies on when the runner is disabled via env.
func TestContentDetailShadowRunner_W3B_NilReceiverSafe(t *testing.T) {
	var r *ContentDetailShadowRunner
	r.Run(nil, nil, nil, LegacyContentDetailOutcome200) // must not panic
}

// TestEvaluateContentDetail_W3B_BlockOverlayPrecedence pins the
// canonical block overlay's two-direction semantics: viewer-blocked-
// owner and owner-blocked-viewer both DENY (the relationship overlay
// resolution is already bidirectional at the handler boundary).
//
// This is the F1-W3B closure of the C-class block-overlay gap on
// /contents/:id (the design audit's required closure item).
func TestEvaluateContentDetail_W3B_BlockOverlayPrecedence(t *testing.T) {
	// Direction 1: viewer-blocked-owner (the canonical blocked set is
	// the bidirectional resolution; the evaluator only reads "is
	// author in viewer's blocked set?", so this is one cell).
	vc := makeContentDetailVC(cdViewerOpts{
		lifecycleHydrated:    true,
		relationshipHydrated: true,
		blocked:              true, // → cdAuthorID in blocked set
	})
	tc := healthyTC()
	content := healthyContent()
	if got, _ := EvaluateContentDetail(vc, tc, content); got != ShadowDecisionDeny {
		t.Errorf("viewer-blocked-owner: got %q, want DENY", got)
	}

	// Direction 2: capability override neutralizes the block.
	vcOverride := makeContentDetailVC(cdViewerOpts{
		lifecycleHydrated:    true,
		relationshipHydrated: true,
		blocked:              true,
		hasBlockOverride:     true,
	})
	if got, _ := EvaluateContentDetail(vcOverride, tc, content); got != ShadowDecisionAllow {
		t.Errorf("block-override capability: got %q, want ALLOW", got)
	}

	// Direction 3: anonymous viewer is never blocked (topology — no
	// relationship state).
	anon := viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST)
	if got, _ := EvaluateContentDetail(anon, tc, content); got != ShadowDecisionAllow {
		t.Errorf("anonymous viewer + healthy state: got %q, want ALLOW", got)
	}
}

// TestEvaluateContentDetail_W3B_BlockOverlayUnhydratedFailsClosed
// pins the UNKNOWN-on-unhydrated-relationship contract: the
// evaluator MUST surface UNKNOWN when the relationship overlay was
// not hydrated, so the fail-CLOSED detail adapter converts it to 404.
func TestEvaluateContentDetail_W3B_BlockOverlayUnhydratedFailsClosed(t *testing.T) {
	vc := makeContentDetailVC(cdViewerOpts{
		lifecycleHydrated: true,
		/* relationshipHydrated: false */
	})
	tc := healthyTC()
	content := healthyContent()
	got, reason := EvaluateContentDetail(vc, tc, content)
	if got != ShadowDecisionUnknown {
		t.Errorf("unhydrated relationship: got %q, want UNKNOWN", got)
	}
	if reason != UnknownReasonViewerOverlayMissing {
		t.Errorf("unhydrated relationship reason: got %q, want viewer_overlay_missing", reason)
	}

	// And the fail-CLOSED adapter converts that UNKNOWN to a 404.
	res := EnforceContentDetail(ContentDetailEvaluatorModeEnforce, vc, tc, content)
	if res.Allow {
		t.Errorf("fail-CLOSED on UNKNOWN: Allow=true, want false")
	}
	if res.Reason != ContentDetailDecisionReasonUnknownFailClosed {
		t.Errorf("fail-CLOSED reason: got %q, want unknown_fail_closed", res.Reason)
	}
}

// TestContentDetailEvaluator_NoPgxpoolImport pins F4 closure for the
// W3B rebuild — neither content_detail_shadow.go nor
// content_detail_enforce.go may import pgxpool after the rebuild. The
// check reads the source bytes directly; a future regression that
// re-introduces the import will fail this test loudly.
//
// Mirrors TestFeedEvaluator_NoPgxpoolImport in feed_viewercontext_w3a_test.go.
func TestContentDetailEvaluator_NoPgxpoolImport(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("cannot resolve test file path")
	}
	// thisFile lives in backend/internal/governance/evaluator/.
	evaluatorDir := filepath.Dir(thisFile)

	guarded := []string{"content_detail_shadow.go", "content_detail_enforce.go"}
	const forbidden = "github.com/jackc/pgx/v5/pgxpool"

	for _, name := range guarded {
		path := filepath.Join(evaluatorDir, name)
		bytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(bytes), forbidden) {
			t.Errorf("F1-W3B regression: %s imports %q; evaluator package must hold no DB pool", name, forbidden)
		}
	}
}

// TestContentDetailShadowRunner_W3B_NoDBAccessOnRun pins the
// observability contract: invoking Run with anonymous viewer + empty
// TC + minimal content must not panic and must not require a pool. A
// goroutine is dispatched; we don't synchronise on it (the runner is
// fire-and-forget by design), but the absence of panic + the constructor
// taking no pool is the structural evidence.
func TestContentDetailShadowRunner_W3B_NoDBAccessOnRun(t *testing.T) {
	r := NewContentDetailShadowRunner(nil)
	r.Run(
		viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST),
		viewercontext.NewTargetContext(),
		&contententity.Content{
			ID:       uuid.New(),
			AuthorID: uuid.New(),
			Status:   contententity.StatusActive,
		},
		LegacyContentDetailOutcome200,
	)
}


