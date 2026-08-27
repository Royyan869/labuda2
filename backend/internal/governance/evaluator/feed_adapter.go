package evaluator

// C1 — /feed governance convergence onto the /search/content topology.
//
// This file lands the pure adapter type + mapping function used to translate
// the canonical EvaluateFeedItem shadow decision into an enforcement-ready
// outcome. It mirrors search_content_adapter.go one-for-one; the only
// behavioral delta is the UNKNOWN policy direction (feed FAIL-OPEN vs
// /search/content FAIL-CLOSED on input_invalid). That delta is doctrine,
// not topology drift — feed is Home and a hydration outage must not blank
// the surface.
//
// CONTRACT (mirrors docs/05-rollout/search-shadow-seam-* and
// docs/contracts/public-card-boundary.md):
//
//   - Pure: no DB reads, no IO, no logging. Caller-side telemetry only.
//   - Single source of decision truth: the canonical ShadowDecision enum
//     from shadow_types.go. No new top-level enum is introduced.
//   - Lifecycle override vocabulary is the canonical {active, unavailable,
//     removed} from viewercontext.PublicLifecycleState; the adapter emits
//     the matching string so any card.Lifecycle field can consume it
//     without re-translation (ContentCard.Lifecycle today).
//   - Fail-OPEN on UNKNOWN regardless of reason — high-traffic feed
//     doctrine declared in feed_enforce.go §41-43 ("hydration outage must
//     not blank Home"). This is the documented inversion of
//     /search/content's fail-closed-on-input-invalid.
//   - Mode is a passive label. The adapter NEVER conditionally changes its
//     mapping based on mode; mode is forwarded so callers can emit
//     feed_evaluator_enforce_mode_total / would_enforce_decision_total
//     telemetry consistently. The handler reads Include / LifecycleOverride
//     and applies them to the response in enforce mode; shadow mode
//     observes only.

// FeedLifecycleActive / FeedLifecycleUnavailable / FeedLifecycleRemoved
// are the canonical Public Lifecycle State strings the adapter emits when
// a non-ALLOW decision should still surface a degraded card. Values match
// viewercontext.PublicLifecycleState string form (and the canonical
// publiccard.ContentCard.Lifecycle vocabulary) so the handler can pass
// them straight into NewContentCard without re-translation.
const (
	FeedLifecycleActive      = "active"
	FeedLifecycleUnavailable = "unavailable"
	FeedLifecycleRemoved     = "removed"
)

// FeedDecisionReason is the bounded telemetry-safe reason label emitted on
// a non-ALLOW adapter outcome. The set is intentionally small to keep
// Prometheus cardinality bounded. Mirrors SearchContentDecisionReason —
// the only delta is FeedDecisionReasonUnknownFailOpen covers ALL UNKNOWN
// reasons on feed (no fail-closed branch for input_invalid).
type FeedDecisionReason string

const (
	FeedDecisionReasonNone            FeedDecisionReason = ""
	FeedDecisionReasonDeny            FeedDecisionReason = "deny"
	FeedDecisionReasonTombstone       FeedDecisionReason = "tombstone"
	FeedDecisionReasonRedact          FeedDecisionReason = "redact"
	FeedDecisionReasonUnknownFailOpen FeedDecisionReason = "unknown_fail_open"
)

// FeedDecision is the adapter's per-row enforcement-ready output, derived
// purely from the canonical ShadowDecision + UnknownReason produced by
// EvaluateFeedItem. It carries no pointers into any DB row or overlay
// struct; it is safe to log fields directly into bounded metrics labels.
type FeedDecision struct {
	// Include reports whether the row should appear in the response when
	// the route is operating in FeedEvaluatorModeEnforce. In
	// FeedEvaluatorModeShadow the caller MUST IGNORE this for response
	// composition (legacy SQL remains authority) but SHOULD emit
	// would-enforce telemetry from it.
	Include bool

	// LifecycleOverride, when non-nil, is the coarsened public lifecycle
	// string the card should adopt instead of the lifecycle the surface
	// would normally emit. Vocabulary: {active, unavailable, removed}.
	// Nil means "do not override."
	//
	// In FeedEvaluatorModeShadow the override is observation only
	// (telemetry); the actual response card is unchanged.
	LifecycleOverride *string

	// Reason is the bounded telemetry-safe label that explains why the
	// decision is not a plain ALLOW. Empty when ShadowDecision is Allow
	// and no override is emitted.
	Reason FeedDecisionReason

	// ShadowDecision passes through the raw evaluator decision for tests
	// and downstream callers that already log against the shadow enum.
	// Useful for "shadow says X, adapter says Y" cross-checks.
	ShadowDecision ShadowDecision
}

// AdaptFeedDecision converts the pure shadow evaluator output for a single
// /feed row into an enforcement-ready decision.
//
// Mapping (input → output):
//
//	ShadowDecisionAllow      → Include=true,  no override,            Reason=none
//	ShadowDecisionDeny       → Include=false, no override,            Reason=deny
//	ShadowDecisionTombstone  → Include=true,  override="removed",     Reason=tombstone
//	ShadowDecisionRedact     → Include=true,  override="unavailable", Reason=redact
//	ShadowDecisionUnknown    → Include=true,  no override,            Reason=unknown_fail_open
//	  (feed fail-OPEN regardless of reason — high-traffic Home doctrine
//	   per feed_enforce.go §41-43. /search/content's fail-closed branch
//	   on input_invalid is intentionally not replicated here.)
//
// The mode parameter is a passive label carried by the caller so its
// telemetry can correlate with the request's operating mode. The adapter
// itself NEVER conditionalizes its mapping on mode.
func AdaptFeedDecision(
	decision ShadowDecision,
	_ UnknownReason, // accepted for forward-compat + parity with search/content; unused under fail-open
	_ FeedEvaluatorMode, // passive label; see docstring
) FeedDecision {
	switch decision {
	case ShadowDecisionAllow:
		return FeedDecision{
			Include:        true,
			Reason:         FeedDecisionReasonNone,
			ShadowDecision: decision,
		}
	case ShadowDecisionDeny:
		return FeedDecision{
			Include:        false,
			Reason:         FeedDecisionReasonDeny,
			ShadowDecision: decision,
		}
	case ShadowDecisionTombstone:
		removed := FeedLifecycleRemoved
		return FeedDecision{
			Include:           true,
			LifecycleOverride: &removed,
			Reason:            FeedDecisionReasonTombstone,
			ShadowDecision:    decision,
		}
	case ShadowDecisionRedact:
		unavailable := FeedLifecycleUnavailable
		return FeedDecision{
			Include:           true,
			LifecycleOverride: &unavailable,
			Reason:            FeedDecisionReasonRedact,
			ShadowDecision:    decision,
		}
	case ShadowDecisionUnknown:
		// Feed fail-OPEN on every UNKNOWN reason. The handler keeps the
		// row; the telemetry counter unknown_fail_open lets ops observe
		// hydration-degradation rate without losing visibility.
		return FeedDecision{
			Include:        true,
			Reason:         FeedDecisionReasonUnknownFailOpen,
			ShadowDecision: decision,
		}
	default:
		// Future-proofing: any unrecognized ShadowDecision value coarsens
		// to fail-open with a bounded reason. Promoting authority on this
		// surface MUST add an explicit mapping above before the new
		// decision can ship.
		return FeedDecision{
			Include:        true,
			Reason:         FeedDecisionReasonUnknownFailOpen,
			ShadowDecision: decision,
		}
	}
}


