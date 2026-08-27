package viewercontext

import (
	"github.com/google/uuid"
)

// RelationshipOverlay carries the canonical viewer × target relationship
// inputs per docs/03-architecture/viewer-context-contract.md §4.4.
//
// Per the contract, the overlay is INPUT to the evaluator. The evaluator
// does NOT re-query the relationship graph (§2.4 / §8.2). On
// /search/content the canonical relationship is the bidirectional
// `user_blocks` resolution (viewer × per-row author) per
// docs/05-rollout/search-overlay-ownership-matrix.md §3.4.
//
// The overlay is constructed empty at NewAuthenticated / NewAnonymous time
// and attached via ViewerContext.WithRelationship after the handler
// caller-batches the per-row resolution per docs/05-rollout/search-
// content-viewercontext-runtime-threading-task-design.md §5.4.
type RelationshipOverlay struct {
	// hydrated reports whether the overlay has been hydrated. False on
	// construction; true after WithRelationship attaches a resolved set.
	// On AnonymousViewer the overlay is always reported as hydrated-empty
	// per viewer-context-contract.md §3.1 (anonymous viewers have no
	// relationship state by topology).
	hydrated bool

	// anonymousEmpty marks an overlay that is empty by virtue of the
	// AnonymousViewer topology rather than by failed hydration. This
	// distinguishes "empty because no relationships exist" (a valid
	// Pattern A anonymous case) from "empty because hydration was not
	// performed" (a defect — UNKNOWN per viewer-context-contract.md
	// §2.4).
	anonymousEmpty bool

	// blockedAuthors is the set of author IDs that are blocked-by-viewer
	// or blocking-of-viewer (bidirectional resolution per the canonical
	// `user_blocks` semantic). The set MAY be empty if the viewer has no
	// blocks against any of the per-row authors.
	blockedAuthors map[uuid.UUID]struct{}
}

// NewEmptyRelationshipOverlay constructs an empty overlay. anonymousEmpty
// distinguishes the AnonymousViewer-empty case from the
// pre-hydration-empty case per the contract's §2.4 UNKNOWN semantics.
func NewEmptyRelationshipOverlay(anonymousEmpty bool) RelationshipOverlay {
	return RelationshipOverlay{
		hydrated:       anonymousEmpty,
		anonymousEmpty: anonymousEmpty,
		blockedAuthors: nil,
	}
}

// NewHydratedRelationshipOverlay constructs the overlay with a resolved
// blocked-author set. The set MAY be empty if no blocks were found; this
// is distinct from "not hydrated" per the contract's §2.4 / §8.5
// distinction.
//
// Per docs/05-rollout/search-overlay-ownership-matrix.md §5, this
// constructor is called by the caller (the HTTP handler) after the
// caller-batched `user_blocks` resolution. It is NOT called by
// repositories, evaluators, card builders, or middleware.
func NewHydratedRelationshipOverlay(blockedAuthorIDs []uuid.UUID) RelationshipOverlay {
	set := make(map[uuid.UUID]struct{}, len(blockedAuthorIDs))
	for _, id := range blockedAuthorIDs {
		if id == uuid.Nil {
			continue
		}
		set[id] = struct{}{}
	}
	return RelationshipOverlay{
		hydrated:       true,
		anonymousEmpty: false,
		blockedAuthors: set,
	}
}

// IsHydrated reports whether the overlay has been hydrated. False
// indicates the caller has NOT performed the canonical resolution; the
// evaluator should emit UNKNOWN (in shadow) or DENY (in production) per
// viewer-context-contract.md §2.4.
func (r RelationshipOverlay) IsHydrated() bool {
	return r.hydrated
}

// IsAnonymousEmpty reports whether the overlay is empty by virtue of
// the AnonymousViewer topology. True implies hydrated-by-topology;
// false on a hydrated overlay implies hydrated-by-resolution.
func (r RelationshipOverlay) IsAnonymousEmpty() bool {
	return r.anonymousEmpty
}

// IsBlocked reports whether the given author ID is blocked-by-viewer or
// blocking-of-viewer per the canonical `user_blocks` bidirectional
// resolution. Returns false for AnonymousViewer (anonymous viewers have
// no relationship state).
//
// This method is the canonical query path for downstream consumers
// (evaluator / future seam runner). It MUST NOT be used to drive any
// shadow-driven filtering, ranking, pagination, or DTO mutation per
// docs/05-rollout/shadow-operations-doctrine.md §7.
func (r RelationshipOverlay) IsBlocked(authorID uuid.UUID) bool {
	if !r.hydrated || r.anonymousEmpty {
		return false
	}
	if r.blockedAuthors == nil {
		return false
	}
	_, ok := r.blockedAuthors[authorID]
	return ok
}

// BlockedSetSize reports the number of resolved blocked authors. Used by
// future telemetry (per docs/05-rollout/search-endpoint-telemetry-enum-
// design.md §7) — this package does NOT register telemetry today.
func (r RelationshipOverlay) BlockedSetSize() int {
	if !r.hydrated {
		return 0
	}
	return len(r.blockedAuthors)
}


