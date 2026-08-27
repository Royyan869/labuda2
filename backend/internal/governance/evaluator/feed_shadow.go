package evaluator

import (
	"context"
	"time"

	"github.com/labuda/backend/internal/governance/viewercontext"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
	"go.uber.org/zap"
)

// EvaluateFeedItem is the pure shadow decision function for one feed item.
//
// F1-W3A — canonical signature. The function reads only the caller-
// hydrated `*viewercontext.ViewerContext` (viewer lifecycle, viewer×author
// relationship) and `*viewercontext.TargetContext` (per-row author
// lifecycle, per-row content moderation). It performs no IO, no DB reads,
// and no fallback synthesis. Missing overlays surface as UNKNOWN with a
// classified reason (Shadow Mode Doctrine — UNKNOWN allowed in shadow
// only).
//
// The decision sequence follows the canonical evaluator precedence model
// from docs/architecture.md (Evaluator Authority Design — Precedence
// Model): actor → target lifecycle → owner → relationship → moderation
// → visibility scope → public allow.
//
// Semantic preservation vs the pre-W3A evaluator:
//   - Owner side (banned / suspended / deleted-flag / deleted-timestamp)
//     coarsens cell-for-cell to the same DENY decision; the existing
//     TestEnforceFeed_EnforceDropsSuspendedBannedDeletedOwners table
//     passes unchanged.
//   - Viewer side: the legacy code denied {deleted, banned} for viewer
//     self but ALLOWED {suspended} as a quirk. Canonical coarsening
//     collapses banned + suspended → Unavailable; this rebuild treats
//     Unavailable viewer self as DENY, bringing /feed into alignment
//     with the constitution §8.4 precedence rule applied on /contents/
//     :id (`AccountStatus IN {deleted, banned, suspended} → DENY` for
//     viewer self). The shift is shadow-only (FEED_EVALUATOR_MODE
//     defaults to shadow); the wire is unchanged.
func EvaluateFeedItem(
	vc *viewercontext.ViewerContext,
	tc *viewercontext.TargetContext,
	item *feedentity.FeedItem,
) (ShadowDecision, UnknownReason) {
	if vc == nil || tc == nil || item == nil {
		return ShadowDecisionUnknown, UnknownReasonInputInvalid
	}

	// Precedence step 1 — actor invalid/banned/deleted (viewer side).
	// AnonymousViewer skips this branch by topology — anonymous viewers
	// cannot be in a lifecycle-degraded state.
	if !vc.IsAnonymous() {
		if !vc.Lifecycle().IsHydrated() {
			return ShadowDecisionUnknown, UnknownReasonViewerOverlayMissing
		}
		switch vc.Lifecycle().State {
		case viewercontext.PublicLifecycleStateRemoved,
			viewercontext.PublicLifecycleStateUnavailable:
			return ShadowDecisionDeny, UnknownReasonNone
		}
	}

	// Precedence step 2 — target deleted/terminal lifecycle. Reads the
	// raw entity status because content-lifecycle vocabulary (active /
	// fulfilled / deleted) is owned by the content entity, not by the
	// user-axis canonical coarsening. Active is the legacy ALLOW gate;
	// anything else denies.
	if item.Status != "active" {
		return ShadowDecisionDeny, UnknownReasonNone
	}

	// Precedence step 3 — owner banned/suspended/deleted.
	ownerLC, ownerHydrated := tc.AuthorLifecycle(item.AuthorID)
	if !ownerHydrated {
		return ShadowDecisionUnknown, UnknownReasonTargetOverlayMissing
	}
	switch ownerLC {
	case viewercontext.PublicLifecycleStateRemoved,
		viewercontext.PublicLifecycleStateUnavailable:
		return ShadowDecisionDeny, UnknownReasonNone
	}

	// Precedence step 4 — relationship deny (bidirectional block).
	// AnonymousViewer skips this branch (the anonymous relationship
	// overlay is hydrated-empty per topology, but anonymous viewers
	// have no targets to be blocked against).
	if !vc.IsAnonymous() {
		if !vc.Relationship().IsHydrated() {
			return ShadowDecisionUnknown, UnknownReasonViewerOverlayMissing
		}
		if vc.Relationship().IsBlocked(item.AuthorID) {
			return ShadowDecisionDeny, UnknownReasonNone
		}
	}

	// Precedence step 5 — moderation enforcement (hidden flag).
	// Post-F1-W1, the feed SQL filters c.is_hidden=false at the
	// repository layer, so any item reaching the evaluator is by SQL
	// invariant visible. The TC moderation overlay is hydrated for
	// shape parity with /search/content; the TOMBSTONE branch survives
	// as defense-in-depth and to keep
	// TestEnforceFeed_EnforceTombstonesHiddenItem meaningful when a
	// test bypasses the SQL gate.
	if modState, modHydrated := tc.ContentModeration(item.ID); modHydrated {
		if modState == viewercontext.ContentModerationStateHidden {
			return ShadowDecisionTombstone, UnknownReasonNone
		}
	}

	// Precedence steps 6–8 — visibility scope, lifecycle, public allow.
	// Feed legacy already restricts to follow-graph; followers-only
	// scope is implicit. Reaching here means the item is publicly
	// allowed for this viewer at this time.
	return ShadowDecisionAllow, UnknownReasonNone
}

// FeedShadowRunner executes the feed evaluator shadow asynchronously.
//
// F1-W3A — the runner no longer holds a DB pool. Overlay hydration
// happens at the handler boundary (feed_viewercontext.go) and is
// passed in pre-hydrated as the canonical (vc, tc) pair. The runner
// is a pure observer: it dispatches a goroutine, runs EvaluateFeedItem
// over the snapshot, and emits telemetry. No DB calls inside the
// goroutine, no per-request overlay queries.
//
// Lifecycle (ViewerContext Contract §5.1 — created at HTTP boundary):
//
//  1. Caller (feed handler) constructs vc + tc inside the request tx.
//  2. Caller invokes Run(vc, tc, items) AFTER the legacy response has
//     been written.
//  3. Run snapshots the items slice header and dispatches a goroutine
//     with a fresh, bounded context. The caller's request context is
//     intentionally NOT propagated, so that shadow work survives client
//     disconnect without affecting response latency.
//  4. The goroutine reads pre-hydrated overlays only; it does not
//     touch the DB.
//
// Strict shadow rules:
//   - The runner never mutates the response, never filters items,
//     never returns evaluator decisions to the caller.
//   - The runner never performs writes.
//   - When overlays were not hydrated (handler error / anonymous
//     viewer), per-item decisions degrade to UNKNOWN with a classified
//     reason; nothing is fabricated.
type FeedShadowRunner struct {
	log     *zap.Logger
	metrics *shadowMetrics
	timeout time.Duration
	// mode is the configured enforce operating mode (Batch 3M).
	// Defaults to shadow at construction; set explicitly via WithMode
	// at boot. The async Run path ignores this field — it is consumed
	// only by the synchronous EnforceFeed handler path.
	mode FeedEvaluatorMode
}

// NewFeedShadowRunner constructs a runner. F1-W3A: the pool argument
// is gone — the runner no longer touches the DB. Pass `nil` log to
// silence (Nop logger applied internally).
func NewFeedShadowRunner(log *zap.Logger) *FeedShadowRunner {
	if log == nil {
		log = zap.NewNop()
	}
	return &FeedShadowRunner{
		log:     log,
		metrics: newShadowMetrics(),
		timeout: 2 * time.Second,
		mode:    FeedEvaluatorModeShadow, // safe default; flip via WithMode at boot
	}
}

// Run dispatches a fire-and-forget shadow evaluation for the given
// feed items. Safe to call when r is nil (no-op).
//
// F1-W3A — accepts pre-hydrated (vc, tc, items). The caller (feed
// handler) builds these at the handler boundary inside the request tx
// and passes them in immutably. The runner does not perform any DB
// access; the timeout below caps wallclock on the evaluator + telemetry
// loop only.
//
// vc may be the AnonymousViewer per Pattern A. Legacy /feed requires
// authenticated identity so AnonymousViewer reaching this runner is a
// caller-side construction defect; the anonymous branch records via
// the anonymous_total counter and the per-item evaluator returns
// UNKNOWN for the viewer-overlay missing case.
func (r *FeedShadowRunner) Run(
	vc *viewercontext.ViewerContext,
	tc *viewercontext.TargetContext,
	items []*feedentity.FeedItem,
) {
	if r == nil {
		return
	}
	// Snapshot the items slice header so the goroutine cannot observe
	// later mutation by the caller. The underlying FeedItem pointers
	// are read-only consumed (we read three fields per item). vc and
	// tc are immutable by contract (viewercontext §8.3).
	snapshot := make([]*feedentity.FeedItem, len(items))
	copy(snapshot, items)

	go r.runShadow(vc, tc, snapshot)
}

func (r *FeedShadowRunner) runShadow(
	vc *viewercontext.ViewerContext,
	tc *viewercontext.TargetContext,
	items []*feedentity.FeedItem,
) {
	defer func() {
		// Defensive: shadow path must never panic the process. Any
		// panic here is observability infrastructure, not authority.
		if rec := recover(); rec != nil {
			r.log.Error("feed shadow goroutine panic recovered",
				zap.Any("panic", rec),
			)
		}
	}()

	start := time.Now()
	defer func() {
		r.metrics.observeLatency(SurfaceFeed, float64(time.Since(start).Milliseconds()))
	}()

	// Bounded fresh context. Client disconnect does not abort shadow
	// telemetry. F1-W3A note: no DB calls inside this goroutine, so
	// the timeout is effectively a wallclock cap on EvaluateFeedItem
	// + telemetry emission rather than on SELECTs.
	_, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	r.metrics.recordRequest(SurfaceFeed)

	// C1 — Per-request operating-mode telemetry. Default
	// FeedEvaluatorModeShadow when WithMode has not been called,
	// preserving observe-only semantics. Mirrors the /search/content
	// seam's recordEnforceMode emission.
	recordFeedEnforceMode(r.mode)

	// Overlay completeness telemetry — emitted once per request.
	// Cardinality preserved across the W3A rebuild: same overlay
	// kinds, same status enum, same surface label.
	if vc != nil && vc.IsAnonymous() {
		r.metrics.recordAnonymous(SurfaceFeed)
		r.metrics.recordOverlayStatus(SurfaceFeed, OverlayIdentity, OverlayStatusMissing)
		r.metrics.recordOverlayStatus(SurfaceFeed, OverlayLifecycle, OverlayStatusMissing)
		r.metrics.recordOverlayStatus(SurfaceFeed, OverlayRelationship, OverlayStatusMissing)
	} else if vc != nil {
		r.metrics.recordOverlayStatus(SurfaceFeed, OverlayIdentity, OverlayStatusPresent)
		if vc.Lifecycle().IsHydrated() {
			r.metrics.recordOverlayStatus(SurfaceFeed, OverlayLifecycle, OverlayStatusPresent)
		} else {
			r.metrics.recordOverlayStatus(SurfaceFeed, OverlayLifecycle, OverlayStatusError)
		}
		if vc.Relationship().IsHydrated() {
			r.metrics.recordOverlayStatus(SurfaceFeed, OverlayRelationship, OverlayStatusPresent)
		} else {
			r.metrics.recordOverlayStatus(SurfaceFeed, OverlayRelationship, OverlayStatusError)
		}
	} else {
		// nil VC is a caller-side defect; flag everything missing.
		r.metrics.recordOverlayStatus(SurfaceFeed, OverlayIdentity, OverlayStatusMissing)
		r.metrics.recordOverlayStatus(SurfaceFeed, OverlayLifecycle, OverlayStatusMissing)
		r.metrics.recordOverlayStatus(SurfaceFeed, OverlayRelationship, OverlayStatusMissing)
	}

	// Capability and moderation overlays are not consumed by feed
	// precedence and are recorded as missing for completeness — same
	// emission pattern as pre-W3A.
	r.metrics.recordOverlayStatus(SurfaceFeed, OverlayCapability, OverlayStatusMissing)
	r.metrics.recordOverlayStatus(SurfaceFeed, OverlayModeration, OverlayStatusMissing)

	for _, item := range items {
		if item == nil {
			continue
		}
		decision, reason := EvaluateFeedItem(vc, tc, item)
		r.metrics.recordDecision(SurfaceFeed, decision)

		// C1 — Adapter classification emit. Per-row
		// would_enforce_decision_total fires regardless of mode so
		// promotion-safety telemetry exists alongside the existing
		// decision_total / divergence_total signals. The adapter
		// mapping is unconditional; only the caller's reaction to
		// Include / LifecycleOverride changes between shadow and
		// enforce.
		adapted := AdaptFeedDecision(decision, reason, r.mode)
		recordFeedWouldEnforceDecision(adapted.Reason)

		// Divergence classification: this seam consumes only legacy-
		// allowed items. Per Shadow Mode Doctrine — Undefined
		// Denominator Rule, no legacy_deny_* category is emitted.
		var category DivergenceCategory
		switch decision {
		case ShadowDecisionAllow, ShadowDecisionRedact:
			category = DivLegacyAllowShadowAllow
		case ShadowDecisionDeny, ShadowDecisionTombstone:
			category = DivLegacyAllowShadowDeny
		case ShadowDecisionUnknown:
			category = DivShadowUnknown
		}
		r.metrics.recordDivergence(SurfaceFeed, category)

		if decision == ShadowDecisionUnknown {
			r.metrics.recordUnknown(SurfaceFeed, reason)
		}

		// Structured log for high-signal divergence cells only. Logs
		// are low-cardinality (no request body, no user PII), and
		// avoid log volume by skipping the dominant ALLOW/ALLOW
		// agreement cell.
		if category == DivLegacyAllowShadowDeny {
			r.log.Info("feed shadow divergence: legacy_allow_shadow_deny",
				zap.String("surface", string(SurfaceFeed)),
				zap.String("decision", string(decision)),
				zap.String("content_id", item.ID.String()),
				zap.String("author_id", item.AuthorID.String()),
				zap.Bool("is_hidden", item.IsHidden),
			)
		}
	}
}



