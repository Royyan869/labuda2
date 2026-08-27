package evaluator

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/governance/viewercontext"
	contententity "github.com/labuda/backend/internal/social/content/entity"
)

// F1-W3B — /contents/:id evaluator canonical purity rebuild.
//
// EvaluateContentDetail is the pure shadow decision function for one
// /contents/:id row. It performs no IO and no DB reads; all inputs come
// from the caller-hydrated *viewercontext.ViewerContext (viewer lifecycle,
// viewer×author block relationship, block-override capability) and
// *viewercontext.TargetContext (per-author lifecycle, per-content
// moderation), plus the raw *contententity.Content row.
//
// The evaluator package no longer holds a DB pool, no longer executes
// SQL, no longer owns hydration. The handler boundary
// (backend/internal/social/content/delivery/http/content_viewercontext.go)
// is the sole hydration site.
//
// Strict shadow rules (mirroring FeedShadowRunner at feed_shadow.go):
//
//   - Never mutates the response. The runner is fire-and-forget,
//     dispatched by the handler post-response-write.
//   - Never returns a decision to the caller. Decisions are emitted to
//     bounded Prometheus telemetry only.
//   - Reads only pre-hydrated overlays. Missing overlays surface as
//     UNKNOWN with a classified reason; the detail-surface fail-CLOSED
//     adapter then converts UNKNOWN to a 404 in enforce mode.
//
// Precedence (content-detail-visibility-doctrine §8.4):
//
//  1. Input validity (vc / tc / content non-nil).
//  2. Block check FIRST (capability-gated). Doctrine §5.2: admin role
//     alone does NOT bypass blocks — the capability must be explicitly
//     granted. Running the block check before the admin bypass enforces
//     that constraint without an explicit retraction step.
//  3. Admin / moderator bypass for non-block deny cases (deleted,
//     hidden, viewer-lifecycle, owner-lifecycle).
//  4. Viewer lifecycle (legacy handler does not enforce; expected to be
//     the dominant new-DENY divergence signal under shadow observation).
//  5. Target lifecycle (deleted). Doctrine §3.3: "fulfilled" remains
//     ALLOW (coarsens to public lifecycle "unavailable" but stays visible).
//  6. Target moderation (hidden). DENY for non-admin callers per §2.3.
//  7. Owner lifecycle.
//  8. ALLOW.
//
// Returns (decision, reason). UNKNOWN is only valid in shadow mode and
// represents missing overlays or invalid input; the fail-CLOSED adapter
// (content_detail_adapter.go) converts UNKNOWN to 404 in enforce mode
// per doctrine §8.5.

// LegacyContentDetailOutcome captures the response status code the legacy
// handler actually emitted for the request the shadow is observing. The
// runner uses it to classify each row into the canonical 4-cell divergence
// taxonomy without re-running the handler's gates.
type LegacyContentDetailOutcome string

const (
	// LegacyContentDetailOutcome200 means the legacy handler wrote a
	// successful response (ALLOW path — admin override, or active +
	// non-hidden content for non-admin).
	LegacyContentDetailOutcome200 LegacyContentDetailOutcome = "200"

	// LegacyContentDetailOutcome404 means the legacy handler wrote a
	// not-found response on the gate path (IsHidden || StatusDeleted
	// for non-admin caller). The runner emits legacy_deny_* divergence
	// cells against this outcome.
	LegacyContentDetailOutcome404 LegacyContentDetailOutcome = "404"
)

// CapabilityContentViewBlockedOverride is the canonical capability name
// that authorizes a moderator/admin caller to override viewer↔author
// block relations on the public content-detail endpoint. Per doctrine
// §5.2 the role alone is NOT sufficient — the capability must be
// explicitly granted. Absence of this capability collapses the moderator
// caller back to normal-viewer block visibility.
const CapabilityContentViewBlockedOverride = "content.view_blocked_override"

// EvaluateContentDetail is the canonical pure decision function for
// one /contents/:id request. See package docstring for the contract.
//
// F1-W3B — canonical signature consuming *viewercontext.ViewerContext +
// *viewercontext.TargetContext + *contententity.Content. The evaluator
// reads coarsened lifecycle / relationship / moderation state from the
// caller-hydrated overlays; raw enum strings are not consumed.
func EvaluateContentDetail(
	vc *viewercontext.ViewerContext,
	tc *viewercontext.TargetContext,
	content *contententity.Content,
) (ShadowDecision, UnknownReason) {
	if vc == nil || tc == nil || content == nil {
		return ShadowDecisionUnknown, UnknownReasonInputInvalid
	}

	// Step 2 — relationship. Capability-gated; admin role alone does NOT
	// bypass per doctrine §5.2. The relationship overlay must be hydrated
	// even for admin callers because block decisions are evaluated for
	// them. AnonymousViewer skips this branch by topology — anonymous
	// viewers cannot have block relations.
	if !vc.IsAnonymous() {
		if !vc.Relationship().IsHydrated() {
			return ShadowDecisionUnknown, UnknownReasonViewerOverlayMissing
		}
		if vc.Relationship().IsBlocked(content.AuthorID) &&
			!vc.Capability().HasBlockOverrideCapability {
			return ShadowDecisionDeny, UnknownReasonNone
		}
	}

	// Step 3 — admin bypass for deleted, hidden, viewer-lifecycle,
	// and owner-lifecycle deny classes. Reached only when the block
	// check did not deny.
	if vc.Capability().IsAdmin {
		return ShadowDecisionAllow, UnknownReasonNone
	}

	// Step 4 — viewer lifecycle (legacy handler does not enforce this on
	// this endpoint; expected to be the dominant new-DENY divergence
	// signal during shadow observation). AnonymousViewer skips by
	// topology — anonymous viewers cannot be lifecycle-degraded.
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

	// Step 5 — target lifecycle. Only "deleted" is a deny trigger.
	// "fulfilled" remains ALLOW (coarsens to public lifecycle
	// "unavailable" at the boundary but is still visible per doctrine
	// §3.3). Reads content.Status directly because content-lifecycle
	// vocabulary (active / fulfilled / deleted) is owned by the
	// content entity, not by the user-axis canonical coarsening.
	if content.Status == contententity.StatusDeleted {
		return ShadowDecisionDeny, UnknownReasonNone
	}

	// Step 6 — target moderation. Prefer the canonical TargetContext
	// overlay; fall back to content.IsHidden when the overlay is not
	// hydrated (e.g. unit tests that build a content directly without
	// going through the handler). Pre-W3B behavior preserved: DENY
	// (not TOMBSTONE) — the detail-surface adapter collapses both to
	// HTTP 404 (content_detail_adapter.go).
	if modState, modHydrated := tc.ContentModeration(content.ID); modHydrated {
		if modState == viewercontext.ContentModerationStateHidden {
			return ShadowDecisionDeny, UnknownReasonNone
		}
	} else if content.IsHidden {
		return ShadowDecisionDeny, UnknownReasonNone
	}

	// Step 7 — owner lifecycle.
	ownerLC, ownerHydrated := tc.AuthorLifecycle(content.AuthorID)
	if !ownerHydrated {
		return ShadowDecisionUnknown, UnknownReasonTargetOverlayMissing
	}
	switch ownerLC {
	case viewercontext.PublicLifecycleStateRemoved,
		viewercontext.PublicLifecycleStateUnavailable:
		return ShadowDecisionDeny, UnknownReasonNone
	}

	// Step 8 — ALLOW.
	return ShadowDecisionAllow, UnknownReasonNone
}

// ContentDetailShadowRunner executes the content-detail evaluator shadow
// asynchronously. Construction is gated by env var
// EVALUATOR_SHADOW_CONTENT_DETAIL_ENABLED at boot — a nil runner is a
// documented no-op at every call site.
//
// F1-W3B — the runner no longer holds a DB pool. Overlay hydration
// happens at the handler boundary (content_viewercontext.go) and is
// passed in pre-hydrated as the canonical (vc, tc, content) tuple. The
// runner is a pure observer: it dispatches a goroutine, runs
// EvaluateContentDetail over the snapshot, and emits telemetry. No DB
// calls inside the goroutine.
type ContentDetailShadowRunner struct {
	log     *zap.Logger
	metrics *shadowMetrics
	timeout time.Duration
	// mode is the configured enforce operating mode (D1 convergence).
	// Defaults to shadow at construction; set explicitly via WithMode at
	// boot. The async Run path ignores this field for visibility-decision
	// purposes — it is consumed only by the synchronous EnforceContentDetail
	// handler path and by the per-request enforce_mode_total telemetry
	// emission below.
	mode ContentDetailEvaluatorMode
}

// NewContentDetailShadowRunner constructs a runner. F1-W3B: the pool
// argument is gone — the runner no longer touches the DB. Pass nil log
// to silence (Nop logger applied internally).
func NewContentDetailShadowRunner(log *zap.Logger) *ContentDetailShadowRunner {
	if log == nil {
		log = zap.NewNop()
	}
	return &ContentDetailShadowRunner{
		log:     log,
		metrics: newShadowMetrics(),
		timeout: 2 * time.Second,
		mode:    ContentDetailEvaluatorModeShadow, // safe default; flip via WithMode at boot
	}
}

// Run dispatches a fire-and-forget shadow evaluation for one content-
// detail request. Safe to call when r is nil (no-op). The goroutine
// snapshot is independent of the caller's request context; it survives
// client disconnect without affecting handler latency.
//
// F1-W3B — accepts pre-hydrated (vc, tc, content) plus the
// legacy-outcome divergence label. The runner does not perform any DB
// access; the timeout below caps wallclock on the evaluator + telemetry
// loop only.
func (r *ContentDetailShadowRunner) Run(
	vc *viewercontext.ViewerContext,
	tc *viewercontext.TargetContext,
	content *contententity.Content,
	legacyOutcome LegacyContentDetailOutcome,
) {
	if r == nil {
		return
	}
	go r.runShadow(vc, tc, content, legacyOutcome)
}

func (r *ContentDetailShadowRunner) runShadow(
	vc *viewercontext.ViewerContext,
	tc *viewercontext.TargetContext,
	content *contententity.Content,
	legacyOutcome LegacyContentDetailOutcome,
) {
	defer func() {
		// Defensive: shadow path must never panic the process.
		if rec := recover(); rec != nil {
			r.log.Error("content_detail shadow goroutine panic recovered",
				zap.Any("panic", rec),
			)
		}
	}()

	start := time.Now()
	defer func() {
		r.metrics.observeLatency(SurfaceContentDetail, float64(time.Since(start).Milliseconds()))
	}()

	// Bounded fresh context. F1-W3B note: no DB calls inside this
	// goroutine, so the timeout is effectively a wallclock cap on
	// EvaluateContentDetail + telemetry emission rather than on SELECTs.
	_, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	r.metrics.recordRequest(SurfaceContentDetail)

	// D1 — per-request operating-mode telemetry. Mirror of the feed
	// recordFeedEnforceMode emission (feed_shadow.go).
	recordContentDetailEnforceMode(r.mode)

	// Overlay completeness telemetry — emitted once per request.
	// Cardinality preserved across the W3B rebuild: same overlay kinds,
	// same status enum, same surface label.
	if vc != nil && vc.IsAnonymous() {
		r.metrics.recordAnonymous(SurfaceContentDetail)
		r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayIdentity, OverlayStatusMissing)
		r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayLifecycle, OverlayStatusMissing)
		r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayRelationship, OverlayStatusMissing)
	} else if vc != nil {
		r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayIdentity, OverlayStatusPresent)
		if vc.Lifecycle().IsHydrated() {
			r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayLifecycle, OverlayStatusPresent)
		} else {
			r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayLifecycle, OverlayStatusError)
		}
		if vc.Relationship().IsHydrated() {
			r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayRelationship, OverlayStatusPresent)
		} else {
			r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayRelationship, OverlayStatusError)
		}
	} else {
		// nil VC is a caller-side defect; flag everything missing.
		r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayIdentity, OverlayStatusMissing)
		r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayLifecycle, OverlayStatusMissing)
		r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayRelationship, OverlayStatusMissing)
	}

	// Capability and moderation overlay status. Capability is always
	// present (lifted from the actor at the handler boundary); moderation
	// is per-row, carried on TC (recorded missing for completeness as
	// pre-W3B did).
	r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayCapability, OverlayStatusPresent)
	r.metrics.recordOverlayStatus(SurfaceContentDetail, OverlayModeration, OverlayStatusMissing)

	decision, reason := EvaluateContentDetail(vc, tc, content)
	r.metrics.recordDecision(SurfaceContentDetail, decision)

	// D1 — adapter classification emit. Per-request
	// would_enforce_decision_total fires regardless of mode so
	// promotion-safety telemetry exists alongside the existing
	// decision_total / divergence_total signals. The adapter mapping is
	// unconditional; only the caller's reaction to Include changes
	// between shadow and enforce.
	adapted := AdaptContentDetailDecision(decision, reason, r.mode)
	recordContentDetailWouldEnforceDecision(adapted.Reason)

	// Divergence classification. Unlike the feed seam, the content-
	// detail legacy handler emits explicit denies (404) for hidden /
	// deleted non-admin requests — all four legacy_×_shadow_× cells
	// are observable on this surface.
	var category DivergenceCategory
	switch decision {
	case ShadowDecisionAllow, ShadowDecisionRedact:
		if legacyOutcome == LegacyContentDetailOutcome200 {
			category = DivLegacyAllowShadowAllow
		} else {
			category = DivLegacyDenyShadowAllow
		}
	case ShadowDecisionDeny, ShadowDecisionTombstone:
		if legacyOutcome == LegacyContentDetailOutcome200 {
			category = DivLegacyAllowShadowDeny
		} else {
			category = DivLegacyDenyShadowDeny
		}
	case ShadowDecisionUnknown:
		category = DivShadowUnknown
	}
	r.metrics.recordDivergence(SurfaceContentDetail, category)

	if decision == ShadowDecisionUnknown {
		r.metrics.recordUnknown(SurfaceContentDetail, reason)
	}

	// Structured log for high-signal divergence cells only. IDs are
	// emitted (not hashed) to match the existing feed-seam log shape;
	// raw user_id/content_id appear in logs but never as Prometheus
	// labels.
	if category == DivLegacyAllowShadowDeny || category == DivLegacyDenyShadowAllow {
		var contentID, authorID uuid.UUID
		var contentStatus string
		var contentIsHidden bool
		if content != nil {
			contentID = content.ID
			authorID = content.AuthorID
			contentStatus = string(content.Status)
			contentIsHidden = content.IsHidden
		}
		r.log.Info("content_detail shadow divergence",
			zap.String("surface", string(SurfaceContentDetail)),
			zap.String("decision", string(decision)),
			zap.String("category", string(category)),
			zap.String("legacy_outcome", string(legacyOutcome)),
			zap.String("content_id", contentID.String()),
			zap.String("author_id", authorID.String()),
			zap.Bool("is_hidden", contentIsHidden),
			zap.String("content_status", contentStatus),
		)
	}
}


