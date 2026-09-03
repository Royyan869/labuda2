package evaluator

import (
	"testing"

	"github.com/google/uuid"

	"github.com/labuda/backend/internal/governance/viewercontext"
	contententity "github.com/labuda/backend/internal/social/content/entity"
)

// F1-W3B — content_detail evaluator precedence table, rewritten to
// consume canonical *viewercontext.ViewerContext + *viewercontext.TargetContext
// + *contententity.Content. The cases mirror the pre-W3B coverage
// cell-for-cell so doctrine semantics are pinned across the rebuild.

// Test fixtures: stable IDs reused across cases.
var (
	cdViewerID  = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	cdAuthorID  = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	cdContentID = uuid.MustParse("33333333-3333-4333-8333-333333333333")
)

type cdViewerOpts struct {
	anonymous            bool
	state                viewercontext.PublicLifecycleState
	lifecycleHydrated    bool
	relationshipHydrated bool
	blocked              bool
	isAdmin              bool
	hasBlockOverride     bool
	viewerIDOverride     uuid.UUID // when non-Nil, overrides cdViewerID (used for "viewer == author" case)
}

func makeContentDetailVC(opts cdViewerOpts) *viewercontext.ViewerContext {
	if opts.anonymous {
		return viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST)
	}
	state := opts.state
	if state == "" {
		state = viewercontext.PublicLifecycleStateActive
	}
	viewerID := opts.viewerIDOverride
	if viewerID == uuid.Nil {
		viewerID = cdViewerID
	}
	identity := viewercontext.IdentityOverlay{CanonicalUserID: viewerID}
	lifecycle := viewercontext.NewLifecycleOverlay(state, opts.lifecycleHydrated)
	capability := viewercontext.CapabilityOverlay{
		IsAdmin:                    opts.isAdmin,
		HasBlockOverrideCapability: opts.hasBlockOverride,
	}
	vc := viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST,
		identity, lifecycle, capability, viewercontext.ModerationOverlay{},
	)
	if opts.relationshipHydrated {
		var blocked []uuid.UUID
		if opts.blocked {
			blocked = []uuid.UUID{cdAuthorID}
		}
		vc = vc.WithRelationship(viewercontext.NewHydratedRelationshipOverlay(blocked))
	}
	return vc
}

type cdTargetOpts struct {
	ownerState        viewercontext.PublicLifecycleState
	ownerHydrated     bool
	moderationHidden  bool
	moderationHydrated bool
}

func makeContentDetailTC(opts cdTargetOpts) *viewercontext.TargetContext {
	tc := viewercontext.NewTargetContext()
	if opts.ownerHydrated {
		state := opts.ownerState
		if state == "" {
			state = viewercontext.PublicLifecycleStateActive
		}
		tc.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{
			cdAuthorID: state,
		})
	}
	if opts.moderationHydrated {
		mod := viewercontext.ContentModerationStateVisible
		if opts.moderationHidden {
			mod = viewercontext.ContentModerationStateHidden
		}
		tc.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{
			cdContentID: mod,
		})
	}
	return tc
}

func makeContentDetailContent(status contententity.Status, isHidden bool) *contententity.Content {
	if status == "" {
		status = contententity.StatusActive
	}
	return &contententity.Content{
		ID:       cdContentID,
		AuthorID: cdAuthorID,
		Status:   status,
		IsHidden: isHidden,
	}
}

// healthyVC + healthyTC + healthyContent — baseline that yields ALLOW.
func healthyVC() *viewercontext.ViewerContext {
	return makeContentDetailVC(cdViewerOpts{
		lifecycleHydrated:    true,
		relationshipHydrated: true,
	})
}
func healthyTC() *viewercontext.TargetContext {
	return makeContentDetailTC(cdTargetOpts{
		ownerHydrated:      true,
		moderationHydrated: true,
	})
}
func healthyContent() *contententity.Content {
	return makeContentDetailContent(contententity.StatusActive, false)
}

func TestEvaluateContentDetail_PrecedenceTable(t *testing.T) {
	tests := []struct {
		name         string
		viewer       cdViewerOpts
		target       cdTargetOpts
		contentStatus contententity.Status
		contentHidden bool
		nilViewer    bool
		nilTarget    bool
		nilContent   bool
		wantDecision ShadowDecision
		wantReason   UnknownReason
	}{
		// === 1. Happy path ===
		{
			name:               "active visible healthy author => ALLOW",
			viewer:             cdViewerOpts{lifecycleHydrated: true, relationshipHydrated: true},
			target:             cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			wantDecision:       ShadowDecisionAllow,
			wantReason:         UnknownReasonNone,
		},

		// === 2. Deleted content / normal viewer ===
		{
			name:          "deleted content normal viewer => DENY",
			viewer:        cdViewerOpts{lifecycleHydrated: true, relationshipHydrated: true},
			target:        cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			contentStatus: contententity.StatusDeleted,
			wantDecision:  ShadowDecisionDeny,
			wantReason:    UnknownReasonNone,
		},

		// === 3. Hidden content / normal viewer ===
		{
			name:          "hidden content normal viewer => DENY",
			viewer:        cdViewerOpts{lifecycleHydrated: true, relationshipHydrated: true},
			target:        cdTargetOpts{ownerHydrated: true, moderationHydrated: true, moderationHidden: true},
			contentHidden: true,
			wantDecision:  ShadowDecisionDeny,
			wantReason:    UnknownReasonNone,
		},

		// === 4. Owner-self hidden — no self bypass ===
		// Doctrine §4: there is no self-author override. Self-viewer
		// gets DENY on own hidden content.
		{
			name: "owner self hidden content => DENY (no self bypass)",
			viewer: cdViewerOpts{
				lifecycleHydrated:    true,
				relationshipHydrated: true,
				viewerIDOverride:     cdAuthorID,
			},
			target:        cdTargetOpts{ownerHydrated: true, moderationHydrated: true, moderationHidden: true},
			contentHidden: true,
			wantDecision:  ShadowDecisionDeny,
			wantReason:    UnknownReasonNone,
		},

		// === 5. Admin bypass — hidden / deleted ===
		{
			name: "admin hidden content => ALLOW",
			viewer: cdViewerOpts{
				lifecycleHydrated:    true,
				relationshipHydrated: true,
				isAdmin:              true,
			},
			target:        cdTargetOpts{ownerHydrated: true, moderationHydrated: true, moderationHidden: true},
			contentHidden: true,
			wantDecision:  ShadowDecisionAllow,
			wantReason:    UnknownReasonNone,
		},
		{
			name: "admin deleted content => ALLOW",
			viewer: cdViewerOpts{
				lifecycleHydrated:    true,
				relationshipHydrated: true,
				isAdmin:              true,
			},
			target:        cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			contentStatus: contententity.StatusDeleted,
			wantDecision:  ShadowDecisionAllow,
			wantReason:    UnknownReasonNone,
		},

		// === 6. Admin bypass ===
		{
			name: "admin hidden content => ALLOW",
			viewer: cdViewerOpts{
				lifecycleHydrated:    true,
				relationshipHydrated: true,
				isAdmin:              true,
			},
			target:        cdTargetOpts{ownerHydrated: true, moderationHydrated: true, moderationHidden: true},
			contentHidden: true,
			wantDecision:  ShadowDecisionAllow,
			wantReason:    UnknownReasonNone,
		},
		{
			name: "admin deleted content => ALLOW",
			viewer: cdViewerOpts{
				lifecycleHydrated:    true,
				relationshipHydrated: true,
				isAdmin:              true,
			},
			target:        cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			contentStatus: contententity.StatusDeleted,
			wantDecision:  ShadowDecisionAllow,
			wantReason:    UnknownReasonNone,
		},

		// === 7. Blocked relation / normal viewer ===
		{
			name: "blocked relation normal viewer => DENY",
			viewer: cdViewerOpts{
				lifecycleHydrated:    true,
				relationshipHydrated: true,
				blocked:              true,
			},
			target:       cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			wantDecision: ShadowDecisionDeny,
			wantReason:   UnknownReasonNone,
		},

		// === 8. Admin without block override / blocked ===
		// Doctrine §5.2: admin role alone does NOT bypass blocks.
		{
			name: "admin without block override capability + blocked => DENY",
			viewer: cdViewerOpts{
				lifecycleHydrated:    true,
				relationshipHydrated: true,
				blocked:              true,
				isAdmin:              true,
			},
			target:       cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			wantDecision: ShadowDecisionDeny,
			wantReason:   UnknownReasonNone,
		},
		// === 9. Block override capability ===
		{
			name: "block override capability + blocked => ALLOW",
			viewer: cdViewerOpts{
				lifecycleHydrated:    true,
				relationshipHydrated: true,
				blocked:              true,
				isAdmin:              true,
				hasBlockOverride:     true,
			},
			target:       cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			wantDecision: ShadowDecisionAllow,
			wantReason:   UnknownReasonNone,
		},
		{
			// Capability presence (not role) is the gate. The capability
			// is normally granted to admins/moderators, but the evaluator
			// does not double-check role — that's the capability system's
			// responsibility. The evaluator treats capability as the
			// authoritative grant.
			name: "block override capability alone + blocked => ALLOW (capability is the authoritative gate)",
			viewer: cdViewerOpts{
				lifecycleHydrated:    true,
				relationshipHydrated: true,
				blocked:              true,
				hasBlockOverride:     true,
			},
			target:       cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			wantDecision: ShadowDecisionAllow,
			wantReason:   UnknownReasonNone,
		},

		// === 10. Owner-side lifecycle: banned / deleted ===
		// Note: canonical coarsening collapses {suspended, banned} → Unavailable.
		// Pre-W3B raw-enum reads denied all three; the canonical reads
		// preserve that DENY decision via the coarsened state.
		{
			name: "banned author normal viewer => DENY (coarsens to Unavailable)",
			viewer: cdViewerOpts{lifecycleHydrated: true, relationshipHydrated: true},
			target: cdTargetOpts{
				ownerHydrated: true, moderationHydrated: true,
				ownerState: viewercontext.PublicLifecycleStateUnavailable,
			},
			wantDecision: ShadowDecisionDeny,
			wantReason:   UnknownReasonNone,
		},
		{
			name: "deleted author normal viewer => DENY (coarsens to Removed)",
			viewer: cdViewerOpts{lifecycleHydrated: true, relationshipHydrated: true},
			target: cdTargetOpts{
				ownerHydrated: true, moderationHydrated: true,
				ownerState: viewercontext.PublicLifecycleStateRemoved,
			},
			wantDecision: ShadowDecisionDeny,
			wantReason:   UnknownReasonNone,
		},

		// === 11. (removed) Fulfilled content ===
		// The "fulfilled" content status was deliberately removed from the
		// canonical content model (CONTRACT ALIGNMENT V1 — content is a
		// social object with active/deleted only; see
		// social/content/entity/content_status.go). The behavior that case
		// pinned — only "deleted" is a status deny-trigger, every other
		// status stays ALLOW — is already covered by the happy-path case
		// (active → ALLOW) and the deleted case (deleted → DENY) above.

		// === 12. Missing overlays ===
		{
			name:   "missing viewer relationship overlay => UNKNOWN viewer_overlay_missing",
			viewer: cdViewerOpts{lifecycleHydrated: true /* relationshipHydrated:false */},
			target: cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			wantDecision: ShadowDecisionUnknown,
			wantReason:   UnknownReasonViewerOverlayMissing,
		},
		{
			name:   "missing viewer lifecycle overlay (non-admin) => UNKNOWN viewer_overlay_missing",
			viewer: cdViewerOpts{relationshipHydrated: true /* lifecycleHydrated:false */},
			target: cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			wantDecision: ShadowDecisionUnknown,
			wantReason:   UnknownReasonViewerOverlayMissing,
		},
		{
			name:         "missing owner lifecycle overlay (non-admin, healthy content) => UNKNOWN target_overlay_missing",
			viewer:       cdViewerOpts{lifecycleHydrated: true, relationshipHydrated: true},
			target:       cdTargetOpts{moderationHydrated: true /* ownerHydrated:false */},
			wantDecision: ShadowDecisionUnknown,
			wantReason:   UnknownReasonTargetOverlayMissing,
		},
		// Admin path skips lifecycle/owner overlays since bypass
		// short-circuits after block check.
		{
			name: "missing viewer lifecycle + admin => ALLOW (admin bypass after block)",
			viewer: cdViewerOpts{
				relationshipHydrated: true,
				isAdmin:              true,
			},
			target:       cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			wantDecision: ShadowDecisionAllow,
			wantReason:   UnknownReasonNone,
		},
		{
			name: "missing owner lifecycle + admin => ALLOW (admin bypass after block)",
			viewer: cdViewerOpts{
				lifecycleHydrated:    true,
				relationshipHydrated: true,
				isAdmin:              true,
			},
			target:       cdTargetOpts{moderationHydrated: true},
			wantDecision: ShadowDecisionAllow,
			wantReason:   UnknownReasonNone,
		},

		// === 13. Invalid input ===
		{
			name:         "nil viewer => UNKNOWN input_invalid",
			nilViewer:    true,
			wantDecision: ShadowDecisionUnknown,
			wantReason:   UnknownReasonInputInvalid,
		},
		{
			name:         "nil target => UNKNOWN input_invalid",
			nilTarget:    true,
			wantDecision: ShadowDecisionUnknown,
			wantReason:   UnknownReasonInputInvalid,
		},
		{
			name:         "nil content => UNKNOWN input_invalid",
			nilContent:   true,
			wantDecision: ShadowDecisionUnknown,
			wantReason:   UnknownReasonInputInvalid,
		},

		// === Viewer-lifecycle deny cases ===
		// Canonical coarsening: {suspended, banned} → Unavailable; deleted_at
		// → Removed. Pre-W3B raw-enum reads denied {deleted, banned,
		// suspended}; canonical reads DENY {Unavailable, Removed} which
		// preserves the same decisions cell-for-cell.
		{
			name: "viewer Unavailable (banned/suspended) normal request => DENY",
			viewer: cdViewerOpts{
				lifecycleHydrated:    true,
				relationshipHydrated: true,
				state:                viewercontext.PublicLifecycleStateUnavailable,
			},
			target:       cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			wantDecision: ShadowDecisionDeny,
			wantReason:   UnknownReasonNone,
		},
		{
			name: "viewer Removed (deleted) normal request => DENY",
			viewer: cdViewerOpts{
				lifecycleHydrated:    true,
				relationshipHydrated: true,
				state:                viewercontext.PublicLifecycleStateRemoved,
			},
			target:       cdTargetOpts{ownerHydrated: true, moderationHydrated: true},
			wantDecision: ShadowDecisionDeny,
			wantReason:   UnknownReasonNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var vc *viewercontext.ViewerContext
			if !tt.nilViewer {
				vc = makeContentDetailVC(tt.viewer)
			}
			var tc *viewercontext.TargetContext
			if !tt.nilTarget {
				tc = makeContentDetailTC(tt.target)
			}
			var content *contententity.Content
			if !tt.nilContent {
				content = makeContentDetailContent(tt.contentStatus, tt.contentHidden)
			}

			gotDecision, gotReason := EvaluateContentDetail(vc, tc, content)
			if gotDecision != tt.wantDecision {
				t.Errorf("decision: got %q, want %q", gotDecision, tt.wantDecision)
			}
			if gotReason != tt.wantReason {
				t.Errorf("reason: got %q, want %q", gotReason, tt.wantReason)
			}
		})
	}
}

// TestEvaluateContentDetail_AnonymousViewer pins the anonymous-topology
// path: AnonymousViewer skips viewer-lifecycle + relationship checks by
// definition, so only target lifecycle / moderation / owner lifecycle
// can deny.
func TestEvaluateContentDetail_AnonymousViewer(t *testing.T) {
	anon := viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST)
	tc := healthyTC()
	content := healthyContent()
	if got, _ := EvaluateContentDetail(anon, tc, content); got != ShadowDecisionAllow {
		t.Errorf("anonymous + healthy state: got %q, want %q", got, ShadowDecisionAllow)
	}

	// Anonymous + suspended author still DENIES.
	tcSuspended := makeContentDetailTC(cdTargetOpts{
		ownerHydrated: true, moderationHydrated: true,
		ownerState: viewercontext.PublicLifecycleStateUnavailable,
	})
	if got, _ := EvaluateContentDetail(anon, tcSuspended, content); got != ShadowDecisionDeny {
		t.Errorf("anonymous + suspended author: got %q, want %q", got, ShadowDecisionDeny)
	}
}

// TestEvaluateContentDetail_ContentModerationFallback verifies the
// in-memory IsHidden fallback when TC ContentModeration is not hydrated.
// This path covers unit tests that build a content directly without going
// through the handler hydrators.
func TestEvaluateContentDetail_ContentModerationFallback(t *testing.T) {
	vc := healthyVC()
	tc := makeContentDetailTC(cdTargetOpts{ownerHydrated: true /* moderationHydrated:false */})
	content := makeContentDetailContent(contententity.StatusActive, true /* IsHidden */)
	if got, _ := EvaluateContentDetail(vc, tc, content); got != ShadowDecisionDeny {
		t.Errorf("hidden content with unhydrated moderation overlay: got %q, want DENY (fallback to content.IsHidden)", got)
	}
}


