package viewercontext

import (
	"github.com/google/uuid"
)

// TargetContext carries the per-row target-side overlay set that travels
// alongside ViewerContext to downstream consumers (evaluator, public-card
// builder, future shadow seam runner) per
// docs/03-architecture/viewer-context-contract.md §4.5 corollary —
// target moderation / lifecycle is Target Context, not ViewerContext.
//
// On /search/content the per-row target is the content's author; per
// docs/05-rollout/search-overlay-ownership-matrix.md §4.2 and
// docs/05-rollout/search-content-viewercontext-runtime-threading-task-
// design.md §5, the overlays the threading task hydrates are:
//
//   - target_lifecycle (per-row author lifecycle, coarsened to
//     PublicLifecycleState),
//   - target_moderation (per-row content moderation — the `is_hidden`
//     boolean as a bounded value).
//
// The per-row relationship overlay (viewer × per-row author) lives on
// ViewerContext.Relationship per the contract's §4.4 viewer-side
// classification, not on TargetContext.
//
// Per docs/05-rollout/search-overlay-ownership-matrix.md §5 the caller
// (the HTTP handler) hydrates this structure. Repositories, evaluators,
// card builders, and middleware MUST NOT hydrate it. The evaluator does
// NOT fetch internally per viewer-context-contract.md §2.4.
type TargetContext struct {
	// authorLifecycle maps content.author_id → coarsened PublicLifecycleState
	// per docs/05-rollout/search-lifecycle-overlay-topology.md §3.2.
	// A missing entry indicates hydration failure for that row; consumers
	// emit UNKNOWN (shadow) / DENY (production) per viewer-context-
	// contract.md §2.4.
	authorLifecycle map[uuid.UUID]PublicLifecycleState

	// authorLifecycleHydrated reports whether the author-lifecycle batch
	// query was performed. False indicates the overlay was never
	// hydrated; consumers emit UNKNOWN with reason=hydration_error and
	// source=target_lifecycle per docs/05-rollout/search-endpoint-
	// telemetry-enum-design.md §6.
	authorLifecycleHydrated bool

	// contentModeration maps content.id → bounded moderation state. The
	// state is "hidden" (true) or "visible" (false), per the canonical
	// `contents.is_hidden` boolean coarsened to a bounded enum at the
	// boundary per docs/05-rollout/search-lifecycle-overlay-topology.md
	// §5.3 (is_hidden is moderation, NOT lifecycle).
	contentModeration map[uuid.UUID]ContentModerationState

	// contentModerationHydrated reports whether the content-moderation
	// batch query was performed.
	contentModerationHydrated bool
}

// ContentModerationState is the bounded coarsening of contents.is_hidden
// per docs/05-rollout/search-lifecycle-overlay-topology.md §5.3 and
// docs/05-rollout/search-endpoint-telemetry-enum-design.md §9 (exposure
// semantic: tombstone / allow). The enum is two-valued today; future
// canonical content-moderation overlay materializations may extend it via
// ADR.
type ContentModerationState string

const (
	// ContentModerationStateVisible corresponds to contents.is_hidden=false.
	ContentModerationStateVisible ContentModerationState = "visible"
	// ContentModerationStateHidden corresponds to contents.is_hidden=true.
	ContentModerationStateHidden ContentModerationState = "hidden"
)

// NewTargetContext constructs an empty TargetContext. Consumers attach
// per-overlay hydration via WithAuthorLifecycle / WithContentModeration.
//
// Per docs/05-rollout/search-content-viewercontext-runtime-threading-task-
// design.md §6.2, the caller (the HTTP handler) is the canonical
// hydration boundary; this constructor is invoked at the handler.
func NewTargetContext() *TargetContext {
	return &TargetContext{
		authorLifecycle:           nil,
		authorLifecycleHydrated:   false,
		contentModeration:         nil,
		contentModerationHydrated: false,
	}
}

// WithAuthorLifecycle attaches the canonical author-lifecycle resolution.
// The map keys are content author IDs; the values are coarsened per
// CoarsenLifecycle. Missing keys indicate per-row hydration failure.
//
// The receiver is mutated in place — TargetContext is built progressively
// at the HTTP handler boundary as caller-batched queries return; once the
// handler hands the TargetContext to downstream consumers, the structure
// becomes read-only by convention (consumer code must not call this
// method). The convention mirrors the ViewerContext §8.3 immutability
// rule applied to a build-then-freeze TargetContext.
func (tc *TargetContext) WithAuthorLifecycle(states map[uuid.UUID]PublicLifecycleState) {
	if tc == nil {
		panic("viewercontext: WithAuthorLifecycle called on nil TargetContext")
	}
	tc.authorLifecycle = states
	tc.authorLifecycleHydrated = true
}

// WithContentModeration attaches the canonical content-moderation
// resolution. The map keys are content row IDs.
func (tc *TargetContext) WithContentModeration(states map[uuid.UUID]ContentModerationState) {
	if tc == nil {
		panic("viewercontext: WithContentModeration called on nil TargetContext")
	}
	tc.contentModeration = states
	tc.contentModerationHydrated = true
}

// AuthorLifecycle returns the coarsened lifecycle for the given author.
// The second return value reports whether the overlay was hydrated AND
// the author was found.
//
// hydrated=false indicates the caller did not perform the canonical
// resolution; consumers emit UNKNOWN per viewer-context-contract.md §2.4.
func (tc *TargetContext) AuthorLifecycle(authorID uuid.UUID) (state PublicLifecycleState, hydrated bool) {
	if tc == nil || !tc.authorLifecycleHydrated {
		return "", false
	}
	state, ok := tc.authorLifecycle[authorID]
	return state, ok
}

// ContentModeration returns the coarsened moderation state for the given
// content row.
func (tc *TargetContext) ContentModeration(contentID uuid.UUID) (state ContentModerationState, hydrated bool) {
	if tc == nil || !tc.contentModerationHydrated {
		return "", false
	}
	state, ok := tc.contentModeration[contentID]
	return state, ok
}

// AuthorLifecycleHydrated reports whether the author-lifecycle overlay
// was hydrated (regardless of per-row presence).
func (tc *TargetContext) AuthorLifecycleHydrated() bool {
	return tc != nil && tc.authorLifecycleHydrated
}

// ContentModerationHydrated reports whether the content-moderation
// overlay was hydrated.
func (tc *TargetContext) ContentModerationHydrated() bool {
	return tc != nil && tc.contentModerationHydrated
}


