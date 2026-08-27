package evaluator

import (
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"go.uber.org/zap"
)

// SearchContentShadowRunner is the Stage 1 shadow seam runner for the
// /search/content endpoint per docs/05-rollout/search-shadow-seam-
// landing-task-design.md §3.1 / §4.2 / §5.
//
// PHASE C — SEARCH SHADOW SEAM STAGE 1 (TELEMETRY ONLY)
//
// Strict shadow rules enforced by this runner:
//
//   - Legacy runtime remains the sole visibility authority on
//     /search/content. The seam observes; the legacy decides.
//   - The runner is fire-and-forget per the feed-seam pattern at
//     backend/internal/governance/evaluator/feed_shadow.go:130-141 and
//     per docs/05-rollout/search-shadow-seam-architecture.md §2.1
//     (Option A — handler-post-response).
//   - The runner performs NO IO and NO DB reads. ViewerContext and
//     TargetContext are caller-hydrated at the HTTP handler boundary
//     per docs/05-rollout/search-content-viewercontext-runtime-
//     threading-task-design.md §6 — the seam is the OBSERVER, not the
//     hydrator (docs/05-rollout/search-shadow-seam-architecture.md §5.7).
//   - The runner never mutates the gin.H response, the contents slice
//     pointers, the ViewerContext, or the TargetContext.
//   - The runner never returns evaluator decisions to the caller.
//   - On UNKNOWN, the runner classifies the reason and source per the
//     bounded enum (search_shadow_types.go); it never synthesizes
//     fallback truth (viewer-context-contract.md §8.5).
//   - Per docs/05-rollout/search-shadow-seam-landing-task-design.md
//     §10.1 / §10.7, NO feature flag controls seam emission. The runner
//     is unconditionally constructed and unconditionally dispatched.
//   - Per docs/05-rollout/search-shadow-seam-architecture.md §3.2,
//     legacy_deny_* divergence cells are RESERVED-NOT-EMITTED on
//     Option A; the legacy SQL filter excluded those rows before the
//     runner saw them.
type SearchContentShadowRunner struct {
	log     *zap.Logger
	metrics *searchShadowMetrics
	// mode is the adapter operating-mode label emitted via
	// enforce_mode_total once per request. It NEVER changes shadow-runner
	// behavior — the runner is fire-and-forget telemetry-only regardless
	// of mode. Batch 3B's handler-side enforcement consumes the same
	// config-driven mode value through a separate code path.
	mode SearchContentAdapterMode
}

// NewSearchContentShadowRunner constructs the search-content shadow
// runner. Returns a non-nil runner; the runner is unconditionally
// dispatched per docs/05-rollout/search-shadow-seam-landing-task-
// design.md §10.1 (no feature flag).
//
// Mode defaults to SearchContentAdapterModeShadow. Use WithMode to
// label requests under SearchContentAdapterModeEnforce once Batch 3B
// promotes the enforcement path; the shadow runner remains
// observation-only either way.
func NewSearchContentShadowRunner(log *zap.Logger) *SearchContentShadowRunner {
	if log == nil {
		log = zap.NewNop()
	}
	return &SearchContentShadowRunner{
		log:     log,
		metrics: newSearchShadowMetrics(),
		mode:    SearchContentAdapterModeShadow,
	}
}

// WithMode returns a copy of the runner with the given adapter mode set
// for telemetry labeling. Invalid mode strings are normalized to
// SearchContentAdapterModeShadow per NormalizeSearchContentAdapterMode
// safety contract. Safe to call on a nil receiver (returns nil).
func (r *SearchContentShadowRunner) WithMode(mode SearchContentAdapterMode) *SearchContentShadowRunner {
	if r == nil {
		return nil
	}
	clone := *r
	if !mode.IsValid() {
		clone.mode = SearchContentAdapterModeShadow
	} else {
		clone.mode = mode
	}
	return &clone
}

// Mode exposes the runner's configured adapter mode. Safe on nil receiver
// (returns SearchContentAdapterModeShadow) so handler code can read the
// authoritative pilot mode without a nil-check dance. This is the
// canonical query path for the SearchContent handler enforcement seam in
// Batch 3B; bypassing it (e.g. reading the env var directly in the
// handler) would create two sources of truth.
func (r *SearchContentShadowRunner) Mode() SearchContentAdapterMode {
	if r == nil {
		return SearchContentAdapterModeShadow
	}
	if !r.mode.IsValid() {
		return SearchContentAdapterModeShadow
	}
	return r.mode
}

// Run dispatches a fire-and-forget shadow evaluation for the given
// /search/content candidate slice. Safe to call when r is nil (no-op).
//
// MUST be called by the HTTP handler with the canonical ViewerContext
// and TargetContext that were caller-hydrated per docs/05-rollout/
// search-content-viewercontext-runtime-threading-task-design.md §6.
//
// Per docs/05-rollout/search-shadow-seam-architecture.md §2.1 the call
// is timed to the post-response location of the /search/content
// handler. The dispatch returns immediately; the goroutine survives
// caller return and operates on a defensive snapshot of the contents
// slice header so the caller cannot observe runner-induced mutation.
func (r *SearchContentShadowRunner) Run(
	vc *viewercontext.ViewerContext,
	targetCtx *viewercontext.TargetContext,
	contents []*entity.ContentPreview,
) {
	if r == nil {
		return
	}
	// Snapshot the slice header so the goroutine cannot observe later
	// caller-side mutation. The underlying ContentPreview pointers are
	// read-only consumed by the runner (it reads ID, AuthorID only).
	snapshot := make([]*entity.ContentPreview, len(contents))
	copy(snapshot, contents)

	go r.runShadow(vc, targetCtx, snapshot)
}

func (r *SearchContentShadowRunner) runShadow(
	vc *viewercontext.ViewerContext,
	targetCtx *viewercontext.TargetContext,
	contents []*entity.ContentPreview,
) {
	defer func() {
		// Defensive: shadow path must never crash the process. Any panic
		// here is observability infrastructure, not authority. Per
		// docs/05-rollout/search-shadow-seam-architecture.md §9.10 the
		// recovered panic is logged at Error level and emitted as the
		// shadow_panic divergence cell.
		if rec := recover(); rec != nil {
			r.log.Error("search content shadow goroutine panic recovered",
				zap.String("surface", string(SurfaceSearch)),
				zap.String("endpoint", string(SearchEndpointContent)),
				zap.String("candidate_set_option", string(CandidateSetOptionAHandlerPostResponse)),
				zap.Any("panic", rec),
			)
			r.metrics.recordDivergence(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, SearchDivShadowPanic)
		}
	}()

	start := time.Now()
	defer func() {
		r.metrics.observeLatency(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, time.Since(start).Seconds())
	}()

	r.metrics.recordRequest(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse)

	// BATCH 3A — Per-request operating-mode telemetry. Default
	// SearchContentAdapterModeShadow when WithMode has not been called,
	// preserving observe-only semantics.
	r.metrics.recordEnforceMode(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, r.mode)

	if vc == nil {
		// The handler MUST construct ViewerContext per viewer-context-
		// contract.md §2.1 / §8.1. Reaching this branch indicates a
		// caller-side construction defect; emit shadow_error and abort
		// the run cleanly.
		r.log.Error("search content shadow: nil ViewerContext from caller (viewer-context-contract.md §8.1 violation)")
		r.metrics.recordDivergence(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, SearchDivShadowError)
		return
	}

	// Per-overlay completeness telemetry per docs/05-rollout/search-
	// endpoint-telemetry-enum-design.md §13.4.
	r.emitOverlayStatus(vc, targetCtx)

	// Per-row anonymous-rate baseline per docs/05-rollout/search-shadow-
	// seam-architecture.md §9.1.
	if vc.IsAnonymous() {
		r.metrics.recordAnonymous(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse)
	}

	// Viewer-self lifecycle state — emitted once per request (not per
	// row). For authenticated traffic the canonical viewer lifecycle is
	// the Public Lifecycle State coarsening of users.account_status /
	// deleted_at; for anonymous, viewer lifecycle is N/A.
	if !vc.IsAnonymous() {
		r.metrics.recordLifecycleState(
			SearchEndpointContent,
			CandidateSetOptionAHandlerPostResponse,
			SearchLifecycleCategoryViewerAccount,
			coarsenViewercontextLifecycleState(vc.Lifecycle().State),
		)
	}

	// Per-row evaluation. Each row's decision is classified into the
	// canonical 7-cell divergence taxonomy; UNKNOWN events are
	// decomposed into reason / source.
	for _, c := range contents {
		if c == nil {
			continue
		}
		r.evaluateRow(vc, targetCtx, c)
	}
}

func (r *SearchContentShadowRunner) emitOverlayStatus(vc *viewercontext.ViewerContext, tc *viewercontext.TargetContext) {
	// identity — present iff authenticated viewer with non-nil canonical
	// user id; missing for AnonymousViewer per viewer-context-contract.md
	// §3.1 (anonymous has no identity overlay by topology).
	identityStatus := SearchOverlayStatusMissing
	if !vc.IsAnonymous() && vc.Identity().CanonicalUserID != uuid.Nil {
		identityStatus = SearchOverlayStatusPresent
	}
	r.metrics.recordOverlayStatus(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, SearchOverlayIdentity, identityStatus)

	// viewer_lifecycle — canonical lifecycle hydration via
	// constructSearchContentViewerContext (hydrateViewerLifecycle).
	// For authenticated viewers: hydrated=true means the DB query succeeded
	// → overlay_status=present (the signal is now honest per
	// docs/05-rollout/search-content-viewer-lifecycle-hydration-runtime-
	// task-design.md §9.1). hydrated=false means the DB query failed or
	// returned no row → overlay_status=error; emit UNKNOWN telemetry so
	// the hydration-error pathway is observable.
	// Anonymous viewer: lifecycle is active-by-topology; always missing
	// (anonymous has no account to query).
	viewerLifecycleStatus := SearchOverlayStatusMissing
	if !vc.IsAnonymous() {
		if vc.Lifecycle().IsHydrated() {
			viewerLifecycleStatus = SearchOverlayStatusPresent
		} else {
			viewerLifecycleStatus = SearchOverlayStatusError
			r.metrics.recordUnknown(
				SearchEndpointContent, CandidateSetOptionAHandlerPostResponse,
				SearchUnknownReasonViewerOverlayMissing, SearchUnknownSourceViewerLifecycle,
			)
			r.metrics.recordHydrationError(
				SearchEndpointContent, CandidateSetOptionAHandlerPostResponse,
				SearchUnknownSourceViewerLifecycle,
			)
		}
	}
	r.metrics.recordOverlayStatus(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, SearchOverlayViewerLifecycle, viewerLifecycleStatus)

	// target_lifecycle — present iff the handler caller-batched the
	// users-lifecycle hydration successfully. On hydration failure the
	// status is "missing" (not "error" because hydrateSearchContent
	// TargetContext returns a hydrated-empty overlay on error per task-
	// design §10.9; per-row absence is the dominant signal).
	targetLifecycleStatus := SearchOverlayStatusMissing
	if tc != nil && tc.AuthorLifecycleHydrated() {
		targetLifecycleStatus = SearchOverlayStatusPresent
	}
	r.metrics.recordOverlayStatus(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, SearchOverlayTargetLifecycle, targetLifecycleStatus)

	// relationship — for AuthenticatedViewer present iff the handler
	// caller-batched the user_blocks resolution. AnonymousViewer is
	// hydrated-as-anonymous-empty per relationship.go §3.1 — we emit
	// "missing" for anonymous because the overlay is not derived from
	// per-row hydration (anonymous has no relationship by topology).
	relationshipStatus := SearchOverlayStatusMissing
	if !vc.IsAnonymous() {
		rel := vc.Relationship()
		if rel.IsHydrated() && !rel.IsAnonymousEmpty() {
			relationshipStatus = SearchOverlayStatusPresent
		}
	}
	r.metrics.recordOverlayStatus(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, SearchOverlayRelationship, relationshipStatus)

	// target_moderation — present iff the handler caller-batched the
	// content-moderation hydration successfully.
	targetModerationStatus := SearchOverlayStatusMissing
	if tc != nil && tc.ContentModerationHydrated() {
		targetModerationStatus = SearchOverlayStatusPresent
	}
	r.metrics.recordOverlayStatus(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, SearchOverlayTargetModeration, targetModerationStatus)

	// capability — emitted as "missing" per docs/05-rollout/search-
	// overlay-ownership-matrix.md §3.7: no current visibility doctrine
	// on /search/content consumes capability. The metric is emitted for
	// completeness; future precedence steps will hydrate it.
	r.metrics.recordOverlayStatus(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, SearchOverlayCapability, SearchOverlayStatusMissing)
}

func (r *SearchContentShadowRunner) evaluateRow(
	vc *viewercontext.ViewerContext,
	tc *viewercontext.TargetContext,
	c *entity.ContentPreview,
) {
	decision, reason, source, semantic := EvaluateSearchContent(vc, tc, c)

	r.metrics.recordDecision(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, semantic)

	// BATCH 3A — Adapter classification. Emit `would_enforce_decision_total`
	// from the bounded SearchContentDecisionReason set so promotion safety
	// telemetry exists alongside the existing decision_total / divergence_total
	// signals. The adapter mapping itself is unconditional; only the caller's
	// reaction to Include/LifecycleOverride changes between shadow and enforce
	// modes. See AdaptSearchContentDecision.
	adapted := AdaptSearchContentDecision(decision, reason, semantic, r.mode)
	r.metrics.recordWouldEnforceDecision(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, adapted.Reason)

	// Divergence classification per Option A: this seam consumes only
	// legacy-allowed rows per docs/05-rollout/search-shadow-seam-
	// architecture.md §3.1; legacy_deny_* cells are RESERVED-NOT-EMITTED.
	var cell SearchDivergenceCell
	switch decision {
	case ShadowDecisionAllow, ShadowDecisionRedact:
		cell = SearchDivLegacyAllowShadowAllow
	case ShadowDecisionDeny, ShadowDecisionTombstone:
		cell = SearchDivLegacyAllowShadowDeny
	case ShadowDecisionUnknown:
		cell = SearchDivShadowUnknown
	default:
		cell = SearchDivShadowError
	}
	r.metrics.recordDivergence(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, cell)

	if decision == ShadowDecisionUnknown {
		r.metrics.recordUnknown(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, reason, source)
		if reason == SearchUnknownReasonHydrationError {
			r.metrics.recordHydrationError(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse, source)
		}
		if reason == SearchUnknownReasonCandidateSetIncomplete {
			r.metrics.recordDenominatorHealth(SearchEndpointContent, CandidateSetOptionAHandlerPostResponse)
		}
	}

	// Per-row target_account lifecycle state observation, when hydrated.
	if tc != nil && tc.AuthorLifecycleHydrated() {
		if state, ok := tc.AuthorLifecycle(c.AuthorID); ok {
			r.metrics.recordLifecycleState(
				SearchEndpointContent,
				CandidateSetOptionAHandlerPostResponse,
				SearchLifecycleCategoryTargetAccount,
				coarsenViewercontextLifecycleState(state),
			)
		}
	}

	// Per-row content lifecycle state. The legacy SQL filter at
	// search_repository_impl.go:216 already restricts to
	// c.status='active' AND c.deleted_at IS NULL — every row reaching
	// the runner is content="active" by construction.
	r.metrics.recordLifecycleState(
		SearchEndpointContent,
		CandidateSetOptionAHandlerPostResponse,
		SearchLifecycleCategoryContent,
		SearchPublicLifecycleStateActive,
	)

	// High-signal structured log per docs/05-rollout/search-endpoint-
	// telemetry-enum-design.md §12.2 — scoped to legacy_allow_shadow_deny
	// (the canonical operational triage cell). Other cells are observed
	// via metrics only to avoid log-volume risk per §12.2.
	//
	// IDs are hashed per §12.4 (raw UUIDs FORBIDDEN); is_hidden appears
	// in the structured log only (FORBIDDEN as metric label per §11.3).
	if cell == SearchDivLegacyAllowShadowDeny {
		isHidden := false
		if tc != nil && tc.ContentModerationHydrated() {
			if state, ok := tc.ContentModeration(c.ID); ok {
				isHidden = state == viewercontext.ContentModerationStateHidden
			}
		}
		r.log.Info("search content shadow divergence: legacy_allow_shadow_deny",
			zap.String("surface", string(SurfaceSearch)),
			zap.String("endpoint", string(SearchEndpointContent)),
			zap.String("candidate_set_option", string(CandidateSetOptionAHandlerPostResponse)),
			zap.String("divergence_cell", string(cell)),
			zap.String("exposure_semantic", string(semantic)),
			zap.String("content_id_hashed", hashUUID(c.ID)),
			zap.String("author_id_hashed", hashUUID(c.AuthorID)),
			zap.Bool("is_hidden", isHidden),
		)
	}
}

// EvaluateSearchContent is the pure shadow decision function for one
// /search/content candidate row. It performs NO IO and NO DB reads;
// all inputs come from the caller-hydrated ViewerContext and
// TargetContext per viewer-context-contract.md §2.4 / §2.5.
//
// Returns:
//   - decision : the canonical evaluator output (allow/deny/tombstone/unknown).
//   - reason   : the UNKNOWN reason classification (none if decision is not unknown).
//   - source   : the UNKNOWN source sub-classification (none if not unknown).
//   - semantic : the canonical exposure semantic for telemetry emission.
//
// Precedence per docs/ARCHITECTURE.md (Evaluator Authority Design —
// Precedence Model) and docs/05-rollout/search-content-viewer-lifecycle-
// precedence-runtime-task-design.md §3.1 (Candidate A pinned):
//
//   1. input validity check (vc / target row non-nil),
//   2. viewer_lifecycle (viewer-self lifecycle gate; anonymous bypass),
//   3. target_lifecycle (per-row author account_status / deleted_at),
//   4. relationship (viewer × per-row author bidirectional user_blocks),
//   5. moderation (per-row content is_hidden),
//   6. public allow.
//
// PublicCard runtime is absent on /search/content (BLOCKER-005); per
// docs/05-rollout/search-shadow-seam-landing-task-design.md §7.4, all
// non-ALLOW outcomes (REDACT / TOMBSTONE / SUSPENDED / DELETED) emit as
// exposure_semantic="unknown_shadow_only" until PublicCard adoption
// lands as a separate material task.
func EvaluateSearchContent(
	vc *viewercontext.ViewerContext,
	tc *viewercontext.TargetContext,
	c *entity.ContentPreview,
) (ShadowDecision, SearchUnknownReason, SearchUnknownSource, SearchExposureSemantic) {
	if vc == nil || c == nil {
		return ShadowDecisionUnknown,
			SearchUnknownReasonInputInvalid,
			SearchUnknownSourceNone,
			SearchExposureSemanticUnknownShadowOnly
	}

	// Step 1 — viewer_lifecycle (viewer-self lifecycle gate).
	//
	// Per docs/05-rollout/search-content-viewer-lifecycle-precedence-
	// runtime-task-design.md §3 (Candidate A pinned) and §4 (shadow-mode
	// behavior table):
	//
	//   - vc.IsAnonymous()                        → bypass (anonymous has no
	//                                                account; never DENY here)
	//   - !vc.Lifecycle().IsHydrated()            → ShadowDecisionUnknown,
	//                                                viewer_overlay_missing,
	//                                                viewer_lifecycle
	//   - State == Unavailable | Removed          → ShadowDecisionDeny
	//   - State == Active (hydrated=true)         → continue to step 2
	//
	// IsHydrated() is checked BEFORE State because, when hydrated=false, the
	// State field is the zero value (Active) but is not authoritative (see
	// viewer-context-contract.md §4.2 hydration semantics; precedence design
	// analysis §5.1).
	//
	// Both Unavailable and Removed collapse to DENY at the decision boundary
	// here; the public-lifecycle distinction is preserved in the per-request
	// search_shadow_lifecycle_state_total emission (runShadow §145-151) and
	// is not duplicated at the evaluator decision boundary (matches the
	// existing collapse on target_lifecycle below).
	//
	// Authority-mode behavior (FUTURE Stage 5; FORBIDDEN today per
	// CURRENT_FORBIDDEN evaluator_authority_activation): hydrated=false
	// becomes fail-closed DENY per shadow-operations-doctrine.md §4.6;
	// shadow-mode keeps it as UNKNOWN.
	if !vc.IsAnonymous() {
		if !vc.Lifecycle().IsHydrated() {
			return ShadowDecisionUnknown,
				SearchUnknownReasonViewerOverlayMissing,
				SearchUnknownSourceViewerLifecycle,
				SearchExposureSemanticUnknownShadowOnly
		}
		switch vc.Lifecycle().State {
		case viewercontext.PublicLifecycleStateUnavailable,
			viewercontext.PublicLifecycleStateRemoved:
			return ShadowDecisionDeny,
				SearchUnknownReasonNone,
				SearchUnknownSourceNone,
				SearchExposureSemanticUnknownShadowOnly
		}
		// active → fall through to step 2
	}

	// Step 2 — target_lifecycle (per-row author).
	if tc == nil || !tc.AuthorLifecycleHydrated() {
		return ShadowDecisionUnknown,
			SearchUnknownReasonTargetOverlayMissing,
			SearchUnknownSourceTargetLifecycle,
			SearchExposureSemanticUnknownShadowOnly
	}
	state, hydrated := tc.AuthorLifecycle(c.AuthorID)
	if !hydrated {
		return ShadowDecisionUnknown,
			SearchUnknownReasonTargetOverlayMissing,
			SearchUnknownSourceTargetLifecycle,
			SearchExposureSemanticUnknownShadowOnly
	}
	switch state {
	case viewercontext.PublicLifecycleStateRemoved:
		// Author deleted — content should not be visible. Canonical
		// rendering would be DELETED per public-card-boundary-contract.md
		// §5.3; without PublicCard runtime we emit unknown_shadow_only
		// per task-design §7.4.
		return ShadowDecisionDeny,
			SearchUnknownReasonNone,
			SearchUnknownSourceNone,
			SearchExposureSemanticUnknownShadowOnly
	case viewercontext.PublicLifecycleStateUnavailable:
		// Author suspended/banned — canonical rendering would be SUSPENDED
		// per docs/05-rollout/search-publiccard-adoption-design.md §5.5;
		// emit unknown_shadow_only.
		return ShadowDecisionDeny,
			SearchUnknownReasonNone,
			SearchUnknownSourceNone,
			SearchExposureSemanticUnknownShadowOnly
	}

	// Step 3 — relationship (only on authenticated traffic;
	// AnonymousViewer has no relationship state by topology per
	// viewer-context-contract.md §3.1).
	if !vc.IsAnonymous() {
		rel := vc.Relationship()
		if !rel.IsHydrated() {
			return ShadowDecisionUnknown,
				SearchUnknownReasonViewerOverlayMissing,
				SearchUnknownSourceRelationship,
				SearchExposureSemanticUnknownShadowOnly
		}
		if rel.IsBlocked(c.AuthorID) {
			// Bidirectional block — content should not be visible.
			// Canonical rendering would be REDACT or DELETED depending on
			// boundary-contract policy; without PublicCard runtime we
			// emit unknown_shadow_only.
			return ShadowDecisionDeny,
				SearchUnknownReasonNone,
				SearchUnknownSourceNone,
				SearchExposureSemanticUnknownShadowOnly
		}
	}

	// Step 4 — moderation (per-row content is_hidden).
	if !tc.ContentModerationHydrated() {
		return ShadowDecisionUnknown,
			SearchUnknownReasonTargetOverlayMissing,
			SearchUnknownSourceTargetModeration,
			SearchExposureSemanticUnknownShadowOnly
	}
	if modState, ok := tc.ContentModeration(c.ID); ok {
		if modState == viewercontext.ContentModerationStateHidden {
			// Content hidden — canonical rendering would be TOMBSTONE per
			// public-card-boundary-contract.md §5.2; emit unknown_shadow_only.
			// On Option A this cell is not expected because the legacy SQL
			// already filtered c.is_hidden=false at search_repository_impl.go:216
			// (per task-design §3.4). Emission here indicates a race condition
			// (row hidden between legacy SQL and our hydration query).
			return ShadowDecisionTombstone,
				SearchUnknownReasonNone,
				SearchUnknownSourceNone,
				SearchExposureSemanticUnknownShadowOnly
		}
	}

	// Reached here = ALLOW. Canonical exposure semantic is "allow"; the
	// legacy and shadow agree on this row (legacy_allow_shadow_allow).
	return ShadowDecisionAllow,
		SearchUnknownReasonNone,
		SearchUnknownSourceNone,
		SearchExposureSemanticAllow
}

// coarsenViewercontextLifecycleState maps the canonical viewercontext
// PublicLifecycleState (from the threading task's overlay package) to
// the metric-label PublicLifecycleState enum. Both enums share the
// same value space; the function exists to pin the boundary so a
// future enum extension on either side does not silently leak across
// the metric layer.
func coarsenViewercontextLifecycleState(s viewercontext.PublicLifecycleState) SearchPublicLifecycleState {
	switch s {
	case viewercontext.PublicLifecycleStateActive:
		return SearchPublicLifecycleStateActive
	case viewercontext.PublicLifecycleStateUnavailable:
		return SearchPublicLifecycleStateUnavailable
	case viewercontext.PublicLifecycleStateRemoved:
		return SearchPublicLifecycleStateRemoved
	default:
		// Unknown / new state values from a future viewercontext extension
		// coarsen to "active" rather than emitting an unbounded label.
		// Future extensions require an ADR amending both packages.
		return SearchPublicLifecycleStateActive
	}
}


