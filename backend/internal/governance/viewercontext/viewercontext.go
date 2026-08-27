package viewercontext

import (
	"github.com/google/uuid"
)

// Surface classifies the request surface per
// docs/03-architecture/viewer-context-contract.md §3 / §6 / §7. The enum is
// bounded; values are short tokens with no per-endpoint variants.
type Surface string

const (
	// SurfacePublicDiscovery is the Pattern A surface classification used by
	// /search/content per docs/05-rollout/search-content-viewercontext-
	// runtime-threading-task-design.md §4.1.
	SurfacePublicDiscovery Surface = "public_discovery"
)

// RequestOrigin classifies how the request entered the system per
// docs/03-architecture/viewer-context-contract.md §3.2. Bounded enum.
type RequestOrigin string

const (
	// RequestOriginREST is the canonical origin for HTTP REST requests on
	// the search surface.
	RequestOriginREST RequestOrigin = "rest"
)

// PublicLifecycleState is the canonical coarsening of raw lifecycle fields
// (users.account_status, users.deleted_at) into the public-facing enum per
// docs/05-rollout/search-lifecycle-overlay-topology.md §3 and
// docs/05-rollout/search-endpoint-telemetry-enum-design.md §8.2.
//
// Coarsening rules per docs/05-rollout/search-endpoint-telemetry-enum-
// design.md §8.3:
//
//   - account_status='active' AND deleted_at IS NULL                → active
//   - account_status IN {'suspended','banned'}                      → unavailable
//   - account_status='deleted' OR deleted_at IS NOT NULL            → removed
//
// Raw account_status enum values do not leave this package; consumers
// (evaluator, public card builder, future seam runner) operate on the
// coarsened state per the boundary contract.
type PublicLifecycleState string

const (
	PublicLifecycleStateActive      PublicLifecycleState = "active"
	PublicLifecycleStateUnavailable PublicLifecycleState = "unavailable"
	PublicLifecycleStateRemoved     PublicLifecycleState = "removed"
)

// CoarsenLifecycle maps raw users.account_status / users.deleted_at to the
// canonical PublicLifecycleState per the boundary contract. The function
// is the single canonical mapping site; per-call-site coarsening is
// forbidden per docs/05-rollout/search-lifecycle-overlay-topology.md §11.
//
// deletedAtPresent is true when users.deleted_at IS NOT NULL.
func CoarsenLifecycle(rawAccountStatus string, deletedAtPresent bool) PublicLifecycleState {
	if deletedAtPresent || rawAccountStatus == "deleted" || rawAccountStatus == "removed" {
		return PublicLifecycleStateRemoved
	}
	if rawAccountStatus == "suspended" || rawAccountStatus == "banned" {
		return PublicLifecycleStateUnavailable
	}
	return PublicLifecycleStateActive
}

// CoarsenSellerTrust maps raw seller_subscriptions.status (latest row by
// created_at DESC) to the canonical PublicLifecycleState for the
// seller-trust axis. This is the canonical mapping site for the
// seller-capability axis carried by SellerCard.Lifecycle (top-level).
//
// Coarsening rules:
//   - "active"                     → active (capable)
//   - "expired" | "inactive" | ""  → unavailable (subscription lapsed or absent)
//   - anything else                → unavailable (fail-closed)
//
// The "removed" state is NOT produced by this axis — seller-trust never
// tombstones; account-removal flows through CoarsenLifecycle on the
// user-identity axis instead.
func CoarsenSellerTrust(rawSubscriptionStatus string) PublicLifecycleState {
	switch rawSubscriptionStatus {
	case "active":
		return PublicLifecycleStateActive
	default:
		return PublicLifecycleStateUnavailable
	}
}

// IdentityOverlay carries the canonical viewer identity binding per
// docs/03-architecture/viewer-context-contract.md §4.1. Email is FORBIDDEN
// per §4.1 and BLOCKER-001 doctrine — public handle reference is the
// canonical username binding, never users.email.
type IdentityOverlay struct {
	// FirebaseUID is the auth identity binding (the token's subject claim).
	// Opaque outside the auth boundary; not exposed in any user-visible
	// response.
	FirebaseUID string

	// CanonicalUserID is the platform's canonical user ID (users.id UUID).
	CanonicalUserID uuid.UUID

	// PublicHandle is the canonical username binding. Never users.email
	// per viewer-context-contract.md §4.1.
	PublicHandle string
}

// LifecycleOverlay carries the viewer's own lifecycle state per
// docs/03-architecture/viewer-context-contract.md §4.2. The raw column
// values are NOT exposed; only the canonical PublicLifecycleState is
// carried.
//
// hydrated distinguishes a canonically-sourced overlay (from the DB
// query in constructSearchContentViewerContext) from a DB-error case or
// the pre-canonical default. When hydrated=false the State field must
// NOT be treated as authoritative; consumers emit UNKNOWN with
// unknown_source=viewer_lifecycle per
// docs/05-rollout/search-content-viewer-lifecycle-hydration-runtime-
// task-design.md §6.
type LifecycleOverlay struct {
	State    PublicLifecycleState
	hydrated bool
}

// IsHydrated reports whether this overlay was sourced from the canonical
// DB query. Returns false when hydration was not attempted (pre-canonical
// threading task state) or when the DB query returned an error / no row.
//
// The anonymous viewer always returns IsHydrated()=true — the active
// state is correct by semantic definition, not a DB failure.
func (l LifecycleOverlay) IsHydrated() bool {
	return l.hydrated
}

// NewLifecycleOverlay constructs a LifecycleOverlay with an explicit
// hydration flag. The handler boundary (constructSearchContentViewerContext)
// calls this after the canonical lifecycle DB query:
//   - hydrated=true on successful SELECT account_status, deleted_at,
//   - hydrated=false on DB error / missing row / nil tx.
//
// Anonymous viewers use NewAnonymous which sets hydrated=true; they do
// not go through this path.
func NewLifecycleOverlay(state PublicLifecycleState, hydrated bool) LifecycleOverlay {
	return LifecycleOverlay{State: state, hydrated: hydrated}
}

// CapabilityOverlay carries declared viewer capabilities per
// docs/03-architecture/viewer-context-contract.md §4.3. For /search/content
// today the overlay is constructed bounded-as-resolved per
// docs/05-rollout/search-content-viewercontext-runtime-threading-task-
// design.md §5.6 — registered for canonical completeness; not consumed
// by current precedence.
//
// F1-W3B — HasBlockOverrideCapability added. The /contents/:id evaluator
// reads this flag per content-detail-visibility-doctrine §5.2: admin role
// alone does NOT bypass viewer↔author blocks; the capability must be
// explicitly granted (CapabilityContentViewBlockedOverride in the actor
// middleware). The field is additive and consumed only by the content-detail
// evaluator today; /feed, /search/content, and other surfaces leave it at
// the zero value.
type CapabilityOverlay struct {
	IsAdmin                    bool
	IsSeller                   bool
	HasBlockOverrideCapability bool
}

// ModerationOverlay carries moderation-relevant state pertaining to the
// viewer (NOT target moderation — target moderation lives on TargetContext
// per viewer-context-contract.md §4.5 corollary). For /search/content today
// the overlay is constructed empty pending future precedence consumption.
type ModerationOverlay struct {
	// HasActiveWarnings is reserved for future precedence consumption.
	HasActiveWarnings bool
}

// ViewerContext is the canonical viewer truth carrier per
// docs/03-architecture/viewer-context-contract.md §3.
//
// The type is constructed at exactly one boundary per request (the HTTP
// handler boundary for Pattern A surfaces) via NewAnonymous or
// NewAuthenticated. Once constructed it is immutable — downstream layers
// must not mutate fields per viewer-context-contract.md §8.3.
type ViewerContext struct {
	// anonymous is true for AnonymousViewer per viewer-context-contract.md
	// §3.1. Always set explicitly; nil-as-anonymous is forbidden per
	// §8.1.
	anonymous bool

	identity     IdentityOverlay
	lifecycle    LifecycleOverlay
	capability   CapabilityOverlay
	moderation   ModerationOverlay
	relationship RelationshipOverlay

	surface Surface
	origin  RequestOrigin
}

// NewAnonymous constructs the canonical AnonymousViewer per
// viewer-context-contract.md §3.1. surface and origin are required —
// implicit / nil semantics are forbidden per §8.1.
//
// AnonymousViewer carries:
//   - explicit anonymity flag (true),
//   - no identity overlay (zero value),
//   - no relationship overlay (anonymous viewers have no targets),
//   - empty capability overlay,
//   - empty moderation overlay,
//   - explicit surface and origin.
func NewAnonymous(surface Surface, origin RequestOrigin) *ViewerContext {
	return &ViewerContext{
		anonymous:    true,
		identity:     IdentityOverlay{},
		lifecycle:    NewLifecycleOverlay(PublicLifecycleStateActive, true), // anonymous-as-active, hydrated by topology definition
		capability:   CapabilityOverlay{},
		moderation:   ModerationOverlay{},
		relationship: NewEmptyRelationshipOverlay(true),
		surface:      surface,
		origin:       origin,
	}
}

// NewAuthenticated constructs the canonical AuthenticatedViewer per
// viewer-context-contract.md §3.2. All overlays are inputs from the caller
// boundary; this constructor does not perform IO. Per §2.4 the evaluator
// never fetches internally — and neither does the constructor.
//
// The relationship overlay is constructed empty; the handler caller-batches
// the per-row resolution after candidate-set determination and attaches it
// via WithRelationship.
func NewAuthenticated(
	surface Surface,
	origin RequestOrigin,
	identity IdentityOverlay,
	lifecycle LifecycleOverlay,
	capability CapabilityOverlay,
	moderation ModerationOverlay,
) *ViewerContext {
	return &ViewerContext{
		anonymous:    false,
		identity:     identity,
		lifecycle:    lifecycle,
		capability:   capability,
		moderation:   moderation,
		relationship: NewEmptyRelationshipOverlay(false),
		surface:      surface,
		origin:       origin,
	}
}

// WithRelationship returns a ViewerContext with the relationship overlay
// attached. This is the single permitted post-construction overlay
// attachment per docs/05-rollout/search-content-viewercontext-runtime-
// threading-task-design.md §5.4 — relationship is hydrated caller-batched
// after candidate-set determination, not at the initial construction.
//
// The returned value is a new ViewerContext; the receiver is not mutated.
// This preserves the immutability invariant of viewer-context-contract.md
// §8.3 while accommodating the Pattern A caller-batched hydration topology.
func (vc *ViewerContext) WithRelationship(rel RelationshipOverlay) *ViewerContext {
	if vc == nil {
		// Nil viewer fallback is forbidden per viewer-context-contract.md
		// §8.1; constructing a synthetic fallback here would itself be a
		// doctrine violation. Callers MUST construct via NewAnonymous /
		// NewAuthenticated before attaching relationship.
		panic("viewercontext: WithRelationship called on nil ViewerContext (violates viewer-context-contract.md §8.1)")
	}
	out := *vc
	out.relationship = rel
	return &out
}

// IsAnonymous reports whether this is an AnonymousViewer per
// viewer-context-contract.md §3.1.
func (vc *ViewerContext) IsAnonymous() bool {
	return vc.anonymous
}

// Identity returns the identity overlay.
func (vc *ViewerContext) Identity() IdentityOverlay {
	return vc.identity
}

// Lifecycle returns the viewer-self lifecycle overlay.
func (vc *ViewerContext) Lifecycle() LifecycleOverlay {
	return vc.lifecycle
}

// Capability returns the capability overlay.
func (vc *ViewerContext) Capability() CapabilityOverlay {
	return vc.capability
}

// Moderation returns the viewer-side moderation overlay.
func (vc *ViewerContext) Moderation() ModerationOverlay {
	return vc.moderation
}

// Relationship returns the viewer × target relationship overlay.
func (vc *ViewerContext) Relationship() RelationshipOverlay {
	return vc.relationship
}

// Surface returns the surface classification.
func (vc *ViewerContext) Surface() Surface {
	return vc.surface
}

// Origin returns the request origin classification.
func (vc *ViewerContext) Origin() RequestOrigin {
	return vc.origin
}
