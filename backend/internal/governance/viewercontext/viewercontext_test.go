package viewercontext_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
)

// TestNewAnonymous verifies the canonical AnonymousViewer construction
// per docs/03-architecture/viewer-context-contract.md §3.1.
func TestNewAnonymous(t *testing.T) {
	vc := viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST)

	if vc == nil {
		t.Fatal("NewAnonymous returned nil — viewer-context-contract.md §8.1 violated")
	}
	if !vc.IsAnonymous() {
		t.Error("AnonymousViewer.IsAnonymous() == false")
	}
	if vc.Surface() != viewercontext.SurfacePublicDiscovery {
		t.Errorf("Surface mismatch: got %q want %q", vc.Surface(), viewercontext.SurfacePublicDiscovery)
	}
	if vc.Origin() != viewercontext.RequestOriginREST {
		t.Errorf("Origin mismatch: got %q want %q", vc.Origin(), viewercontext.RequestOriginREST)
	}
	if vc.Identity().FirebaseUID != "" || vc.Identity().CanonicalUserID != uuid.Nil {
		t.Error("AnonymousViewer carries identity overlay — violates §3.1")
	}
	rel := vc.Relationship()
	if !rel.IsHydrated() || !rel.IsAnonymousEmpty() {
		t.Error("AnonymousViewer relationship overlay must be hydrated-as-anonymous-empty per §3.1")
	}
}

// TestNewAuthenticated verifies the canonical AuthenticatedViewer
// construction per docs/03-architecture/viewer-context-contract.md §3.2.
func TestNewAuthenticated(t *testing.T) {
	userID := uuid.New()
	vc := viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery,
		viewercontext.RequestOriginREST,
		viewercontext.IdentityOverlay{
			FirebaseUID:     "firebase-abc",
			CanonicalUserID: userID,
			PublicHandle:    "alice",
		},
		viewercontext.LifecycleOverlay{State: viewercontext.PublicLifecycleStateActive},
		viewercontext.CapabilityOverlay{IsSeller: true},
		viewercontext.ModerationOverlay{},
	)

	if vc == nil {
		t.Fatal("NewAuthenticated returned nil")
	}
	if vc.IsAnonymous() {
		t.Error("AuthenticatedViewer.IsAnonymous() == true")
	}
	if vc.Identity().CanonicalUserID != userID {
		t.Errorf("CanonicalUserID mismatch: got %v want %v", vc.Identity().CanonicalUserID, userID)
	}
	if vc.Identity().FirebaseUID != "firebase-abc" {
		t.Errorf("FirebaseUID mismatch: got %q", vc.Identity().FirebaseUID)
	}
	if vc.Identity().PublicHandle != "alice" {
		t.Errorf("PublicHandle mismatch: got %q", vc.Identity().PublicHandle)
	}
	if vc.Lifecycle().State != viewercontext.PublicLifecycleStateActive {
		t.Errorf("Lifecycle.State mismatch: got %q", vc.Lifecycle().State)
	}
	if !vc.Capability().IsSeller {
		t.Error("Capability.IsSeller lost during construction")
	}
	rel := vc.Relationship()
	if rel.IsHydrated() {
		t.Error("AuthenticatedViewer relationship overlay must be unhydrated until WithRelationship attaches a resolved set")
	}
	if rel.IsAnonymousEmpty() {
		t.Error("AuthenticatedViewer relationship overlay must NOT be marked anonymous-empty")
	}
}

// TestWithRelationship_Immutability verifies that WithRelationship
// returns a new ViewerContext rather than mutating the receiver per
// viewer-context-contract.md §8.3.
func TestWithRelationship_Immutability(t *testing.T) {
	userID := uuid.New()
	original := viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery,
		viewercontext.RequestOriginREST,
		viewercontext.IdentityOverlay{CanonicalUserID: userID},
		viewercontext.LifecycleOverlay{State: viewercontext.PublicLifecycleStateActive},
		viewercontext.CapabilityOverlay{},
		viewercontext.ModerationOverlay{},
	)

	blockedID := uuid.New()
	rel := viewercontext.NewHydratedRelationshipOverlay([]uuid.UUID{blockedID})
	with := original.WithRelationship(rel)

	if with == original {
		t.Error("WithRelationship returned the same pointer — violates §8.3 immutability")
	}
	if original.Relationship().IsHydrated() {
		t.Error("Receiver's relationship was mutated by WithRelationship — violates §8.3")
	}
	if !with.Relationship().IsHydrated() {
		t.Error("Returned ViewerContext relationship is not hydrated")
	}
	if !with.Relationship().IsBlocked(blockedID) {
		t.Error("Returned ViewerContext relationship does not record the blocked author")
	}
}

// TestCoarsenLifecycle verifies the canonical coarsening rules per
// docs/05-rollout/search-endpoint-telemetry-enum-design.md §8.3.
func TestCoarsenLifecycle(t *testing.T) {
	cases := []struct {
		name             string
		rawAccountStatus string
		deletedAtPresent bool
		want             viewercontext.PublicLifecycleState
	}{
		{"active no-deleted", "active", false, viewercontext.PublicLifecycleStateActive},
		{"active deleted-set", "active", true, viewercontext.PublicLifecycleStateRemoved},
		{"suspended", "suspended", false, viewercontext.PublicLifecycleStateUnavailable},
		{"banned", "banned", false, viewercontext.PublicLifecycleStateUnavailable},
		{"deleted enum", "deleted", false, viewercontext.PublicLifecycleStateRemoved},
		{"removed via GetStatus", "removed", false, viewercontext.PublicLifecycleStateRemoved},
		{"unknown enum default to active", "weird-state-enum", false, viewercontext.PublicLifecycleStateActive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := viewercontext.CoarsenLifecycle(tc.rawAccountStatus, tc.deletedAtPresent)
			if got != tc.want {
				t.Errorf("CoarsenLifecycle(%q, %v) = %q; want %q", tc.rawAccountStatus, tc.deletedAtPresent, got, tc.want)
			}
		})
	}
}

// TestCoarsenSellerTrust verifies the canonical seller-trust coarsening rules.
func TestCoarsenSellerTrust(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want viewercontext.PublicLifecycleState
	}{
		{"active", "active", viewercontext.PublicLifecycleStateActive},
		{"expired", "expired", viewercontext.PublicLifecycleStateUnavailable},
		{"inactive", "inactive", viewercontext.PublicLifecycleStateUnavailable},
		{"empty", "", viewercontext.PublicLifecycleStateUnavailable},
		{"unknown", "legacy-status", viewercontext.PublicLifecycleStateUnavailable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := viewercontext.CoarsenSellerTrust(tc.raw)
			if got != tc.want {
				t.Errorf("CoarsenSellerTrust(%q) = %q; want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// TestRelationshipOverlay_Empty verifies the empty overlay distinguishes
// pre-hydration from anonymous-empty per viewer-context-contract.md §2.4.
func TestRelationshipOverlay_Empty(t *testing.T) {
	pre := viewercontext.NewEmptyRelationshipOverlay(false)
	if pre.IsHydrated() {
		t.Error("Pre-hydration empty overlay reports hydrated=true")
	}
	if pre.IsAnonymousEmpty() {
		t.Error("Pre-hydration empty overlay reports anonymous-empty=true")
	}

	anon := viewercontext.NewEmptyRelationshipOverlay(true)
	if !anon.IsHydrated() {
		t.Error("Anonymous-empty overlay reports hydrated=false — should be hydrated-by-topology")
	}
	if !anon.IsAnonymousEmpty() {
		t.Error("Anonymous-empty overlay reports anonymous-empty=false")
	}
	if anon.IsBlocked(uuid.New()) {
		t.Error("Anonymous-empty overlay reports IsBlocked=true")
	}
}

// TestRelationshipOverlay_Hydrated verifies the canonical hydrated
// relationship overlay set semantics.
func TestRelationshipOverlay_Hydrated(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	c := uuid.New()
	rel := viewercontext.NewHydratedRelationshipOverlay([]uuid.UUID{a, b})

	if !rel.IsHydrated() {
		t.Error("Hydrated overlay reports IsHydrated=false")
	}
	if rel.IsAnonymousEmpty() {
		t.Error("Hydrated overlay reports IsAnonymousEmpty=true")
	}
	if !rel.IsBlocked(a) || !rel.IsBlocked(b) {
		t.Error("Hydrated overlay missing expected blocked authors")
	}
	if rel.IsBlocked(c) {
		t.Error("Hydrated overlay reports unexpected author as blocked")
	}
	if rel.BlockedSetSize() != 2 {
		t.Errorf("BlockedSetSize = %d; want 2", rel.BlockedSetSize())
	}
}

// TestTargetContext_AuthorLifecycle verifies caller-batched author
// lifecycle attachment per docs/05-rollout/search-content-viewercontext-
// runtime-threading-task-design.md §5.3.
func TestTargetContext_AuthorLifecycle(t *testing.T) {
	tc := viewercontext.NewTargetContext()
	if tc.AuthorLifecycleHydrated() {
		t.Error("Empty TargetContext reports AuthorLifecycleHydrated=true")
	}

	authorA := uuid.New()
	authorB := uuid.New()
	tc.WithAuthorLifecycle(map[uuid.UUID]viewercontext.PublicLifecycleState{
		authorA: viewercontext.PublicLifecycleStateActive,
		authorB: viewercontext.PublicLifecycleStateUnavailable,
	})

	if !tc.AuthorLifecycleHydrated() {
		t.Error("After WithAuthorLifecycle, AuthorLifecycleHydrated=false")
	}
	if state, hydrated := tc.AuthorLifecycle(authorA); !hydrated || state != viewercontext.PublicLifecycleStateActive {
		t.Errorf("AuthorLifecycle(authorA) = (%q, %v); want (active, true)", state, hydrated)
	}
	if state, hydrated := tc.AuthorLifecycle(authorB); !hydrated || state != viewercontext.PublicLifecycleStateUnavailable {
		t.Errorf("AuthorLifecycle(authorB) = (%q, %v); want (unavailable, true)", state, hydrated)
	}
	if _, hydrated := tc.AuthorLifecycle(uuid.New()); hydrated {
		t.Error("AuthorLifecycle returned hydrated=true for absent author")
	}
}

// TestLifecycleOverlay_IsHydrated verifies the canonical hydration flag
// semantics per docs/05-rollout/search-content-viewer-lifecycle-hydration-
// runtime-task-design.md §6.
func TestLifecycleOverlay_IsHydrated(t *testing.T) {
	hydrated := viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, true)
	if !hydrated.IsHydrated() {
		t.Error("NewLifecycleOverlay(active, true).IsHydrated() = false; want true")
	}
	if hydrated.State != viewercontext.PublicLifecycleStateActive {
		t.Errorf("State = %q; want active", hydrated.State)
	}

	unhydrated := viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, false)
	if unhydrated.IsHydrated() {
		t.Error("NewLifecycleOverlay(active, false).IsHydrated() = true; want false")
	}

	unavailable := viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateUnavailable, true)
	if !unavailable.IsHydrated() {
		t.Error("NewLifecycleOverlay(unavailable, true).IsHydrated() = false; want true")
	}
	if unavailable.State != viewercontext.PublicLifecycleStateUnavailable {
		t.Errorf("State = %q; want unavailable", unavailable.State)
	}

	removed := viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateRemoved, true)
	if removed.State != viewercontext.PublicLifecycleStateRemoved {
		t.Errorf("State = %q; want removed", removed.State)
	}
}

// TestNewAnonymous_LifecycleHydrated verifies that the AnonymousViewer
// always carries IsHydrated()=true with State=active per
// docs/05-rollout/search-content-viewer-lifecycle-hydration-runtime-task-design.md §4.5.
func TestNewAnonymous_LifecycleHydrated(t *testing.T) {
	vc := viewercontext.NewAnonymous(viewercontext.SurfacePublicDiscovery, viewercontext.RequestOriginREST)

	lc := vc.Lifecycle()
	if !lc.IsHydrated() {
		t.Error("AnonymousViewer lifecycle IsHydrated()=false; want true — active by topology definition")
	}
	if lc.State != viewercontext.PublicLifecycleStateActive {
		t.Errorf("AnonymousViewer lifecycle.State = %q; want active", lc.State)
	}
}

// TestNewAuthenticated_LifecyclePassthrough verifies that NewAuthenticated
// passes the provided LifecycleOverlay (including its hydration flag) through
// unchanged per viewer-context-contract.md §5.2 (enrichment at boundary).
func TestNewAuthenticated_LifecyclePassthrough(t *testing.T) {
	userID := uuid.New()

	// Hydrated=true, unavailable (suspended viewer).
	lc := viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateUnavailable, true)
	vc := viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery,
		viewercontext.RequestOriginREST,
		viewercontext.IdentityOverlay{CanonicalUserID: userID},
		lc,
		viewercontext.CapabilityOverlay{},
		viewercontext.ModerationOverlay{},
	)

	if !vc.Lifecycle().IsHydrated() {
		t.Error("AuthenticatedViewer lifecycle IsHydrated() lost through NewAuthenticated")
	}
	if vc.Lifecycle().State != viewercontext.PublicLifecycleStateUnavailable {
		t.Errorf("lifecycle.State = %q; want unavailable", vc.Lifecycle().State)
	}

	// Hydrated=false (DB error case).
	lcErr := viewercontext.NewLifecycleOverlay(viewercontext.PublicLifecycleStateActive, false)
	vcErr := viewercontext.NewAuthenticated(
		viewercontext.SurfacePublicDiscovery,
		viewercontext.RequestOriginREST,
		viewercontext.IdentityOverlay{CanonicalUserID: userID},
		lcErr,
		viewercontext.CapabilityOverlay{},
		viewercontext.ModerationOverlay{},
	)
	if vcErr.Lifecycle().IsHydrated() {
		t.Error("hydrated=false LifecycleOverlay became IsHydrated()=true through NewAuthenticated")
	}
}

// TestTargetContext_ContentModeration verifies caller-batched moderation
// attachment per docs/05-rollout/search-content-viewercontext-runtime-
// threading-task-design.md §5.5.
func TestTargetContext_ContentModeration(t *testing.T) {
	tc := viewercontext.NewTargetContext()
	if tc.ContentModerationHydrated() {
		t.Error("Empty TargetContext reports ContentModerationHydrated=true")
	}

	contentA := uuid.New()
	tc.WithContentModeration(map[uuid.UUID]viewercontext.ContentModerationState{
		contentA: viewercontext.ContentModerationStateVisible,
	})

	if !tc.ContentModerationHydrated() {
		t.Error("After WithContentModeration, ContentModerationHydrated=false")
	}
	if state, hydrated := tc.ContentModeration(contentA); !hydrated || state != viewercontext.ContentModerationStateVisible {
		t.Errorf("ContentModeration(contentA) = (%q, %v); want (visible, true)", state, hydrated)
	}
}
