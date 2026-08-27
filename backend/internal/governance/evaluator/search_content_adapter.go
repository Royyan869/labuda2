package evaluator

// PHASE 3A — /search/content evaluator authority promotion prerequisite.
//
// This file ONLY lands the pure adapter type + mapping function used to
// translate the existing shadow-mode evaluator decision into an
// enforcement-ready outcome. NO ROUTE IS YET ENFORCED. The adapter is
// consumed in shadow mode by the existing SearchContentShadowRunner to
// emit `would_enforce_*` telemetry, which is the canonical operational
// signal Batch 3B needs to flip /search/content to authority safely.
//
// CONTRACT (mirrors docs/05-rollout/search-shadow-seam-* and
// docs/contracts/public-card-boundary.md):
//
//   - Pure: no DB reads, no IO, no logging. Caller-side telemetry only.
//   - Single source of decision truth: the canonical ShadowDecision
//     enum from shadow_types.go. No new top-level enum is introduced.
//   - Lifecycle override vocabulary is the canonical {active, unavailable,
//     removed} from viewercontext.PublicLifecycleState; the adapter
//     emits the matching string so any card.Lifecycle field can consume
//     it without re-translation.
//   - Fail-open on overlay-missing UNKNOWN (audit doctrine —
//     "incomplete overlay ≠ proof of denial"). Fail-closed on
//     input-invalid UNKNOWN (handler construction defect).
//   - Mode is a passive label. The adapter NEVER conditionally changes
//     its mapping based on mode; mode is forwarded so callers can emit
//     `enforce_mode_total` and `would_enforce_*` telemetry consistently.
//     Batch 3B (the actual enforcement wiring) reads `Include` /
//     `LifecycleOverride` and applies them to the response.

// SearchContentAdapterMode is the operating mode of the /search/content
// evaluator integration. ONLY two values are valid; any other env-string
// value MUST be normalized to ModeShadow by the config layer.
type SearchContentAdapterMode string

const (
	// SearchContentAdapterModeShadow is the default operating mode. In
	// shadow mode the legacy SQL filter remains the sole visibility
	// authority. The adapter is consumed strictly for telemetry; the
	// SearchContentDecision.Include field is observed via
	// `would_enforce_decision_total` but does NOT alter the gin.H
	// response, the contents slice, the ViewerContext, or pagination.
	SearchContentAdapterModeShadow SearchContentAdapterMode = "shadow"

	// SearchContentAdapterModeEnforce is the FUTURE Batch 3B mode where
	// SearchContentDecision.Include drives row inclusion and
	// SearchContentDecision.LifecycleOverride drives ContentCard.Lifecycle.
	// Wiring is NOT IN THIS BATCH; the constant exists so config parsing
	// and telemetry can label requests consistently across batches.
	SearchContentAdapterModeEnforce SearchContentAdapterMode = "enforce"
)

// IsValid reports whether m is one of the two canonical adapter modes.
func (m SearchContentAdapterMode) IsValid() bool {
	switch m {
	case SearchContentAdapterModeShadow, SearchContentAdapterModeEnforce:
		return true
	default:
		return false
	}
}

// SearchContentLifecycleOverride coarsens the canonical public lifecycle
// vocabulary into string constants the adapter emits when a non-ALLOW
// decision should still surface a degraded card (TOMBSTONE / REDACT).
// Values match viewercontext.PublicLifecycleState string form, so any
// card.Lifecycle field — including publiccard.ContentCard.Lifecycle —
// can consume them verbatim.
const (
	SearchContentLifecycleActive      = "active"
	SearchContentLifecycleUnavailable = "unavailable"
	SearchContentLifecycleRemoved     = "removed"
)

// SearchContentDecisionReason is the bounded telemetry-safe reason label
// emitted on a non-ALLOW adapter outcome. The set is intentionally small
// to keep Prometheus cardinality bounded.
type SearchContentDecisionReason string

const (
	SearchContentDecisionReasonNone              SearchContentDecisionReason = ""
	SearchContentDecisionReasonDeny              SearchContentDecisionReason = "deny"
	SearchContentDecisionReasonTombstone         SearchContentDecisionReason = "tombstone"
	SearchContentDecisionReasonRedact            SearchContentDecisionReason = "redact"
	SearchContentDecisionReasonUnknownFailOpen   SearchContentDecisionReason = "unknown_fail_open"
	SearchContentDecisionReasonUnknownFailClosed SearchContentDecisionReason = "unknown_fail_closed"
)

// SearchContentDecision is the adapter's per-row enforcement-ready
// output, derived purely from the canonical ShadowDecision + UNKNOWN
// classification produced by EvaluateSearchContent. It carries no
// pointers into any DB row, ViewerContext, or TargetContext; it is safe
// to log fields directly into bounded metrics labels.
type SearchContentDecision struct {
	// Include reports whether the row should appear in the response when
	// the route is operating in SearchContentAdapterModeEnforce. In
	// SearchContentAdapterModeShadow the caller MUST IGNORE this for
	// response composition (legacy SQL remains authority) but SHOULD
	// emit would-enforce telemetry from it.
	Include bool

	// LifecycleOverride, when non-nil, is the coarsened public lifecycle
	// string the card should adopt instead of the lifecycle the surface
	// would normally emit. The vocabulary is the canonical
	// {active, unavailable, removed} set. Nil means "do not override."
	//
	// In SearchContentAdapterModeShadow the override is observation
	// only (telemetry); the actual response card is unchanged.
	LifecycleOverride *string

	// Reason is the bounded telemetry-safe label that explains why the
	// decision is not a plain ALLOW. Empty when ShadowDecision is Allow
	// and no override is emitted.
	Reason SearchContentDecisionReason

	// ShadowDecision passes through the raw evaluator decision for tests
	// and downstream callers that already log against the shadow enum.
	// Useful for "shadow says X, adapter says Y" cross-checks.
	ShadowDecision ShadowDecision
}

// AdaptSearchContentDecision converts the pure shadow evaluator output
// for a single /search/content row into an enforcement-ready decision.
//
// Mapping (input → output):
//
//	ShadowDecisionAllow      → Include=true, no override, Reason=none
//	ShadowDecisionDeny       → Include=false, no override, Reason=deny
//	ShadowDecisionTombstone  → Include=true,  override="removed",     Reason=tombstone
//	ShadowDecisionRedact     → Include=true,  override="unavailable", Reason=redact
//	ShadowDecisionUnknown    +
//	    reason=InputInvalid  → Include=false, no override, Reason=unknown_fail_closed
//	  (handler construction defect; treated as fail-closed per
//	   docs/contracts/viewer-context.md §8.1 caller responsibility).
//	ShadowDecisionUnknown    +
//	    any other reason     → Include=true,  no override, Reason=unknown_fail_open
//	  (overlay-missing or hydration-error; legacy authority is preserved
//	   per Batch 3 audit doctrine — "incomplete overlay ≠ proof of denial").
//
// The mode parameter is a passive label carried by the caller so its
// telemetry can correlate with the request's operating mode. The adapter
// itself NEVER conditionalizes its mapping on mode — the mapping above
// is the canonical truth in both shadow and enforce modes; what differs
// between modes is whether the caller acts on Include/Override.
func AdaptSearchContentDecision(
	decision ShadowDecision,
	reason SearchUnknownReason,
	_ SearchExposureSemantic, // accepted for forward-compat with the evaluator return shape; unused today
	_ SearchContentAdapterMode, // passive label; see docstring
) SearchContentDecision {
	switch decision {
	case ShadowDecisionAllow:
		return SearchContentDecision{
			Include:        true,
			Reason:         SearchContentDecisionReasonNone,
			ShadowDecision: decision,
		}
	case ShadowDecisionDeny:
		return SearchContentDecision{
			Include:        false,
			Reason:         SearchContentDecisionReasonDeny,
			ShadowDecision: decision,
		}
	case ShadowDecisionTombstone:
		removed := SearchContentLifecycleRemoved
		return SearchContentDecision{
			Include:           true,
			LifecycleOverride: &removed,
			Reason:            SearchContentDecisionReasonTombstone,
			ShadowDecision:    decision,
		}
	case ShadowDecisionRedact:
		unavailable := SearchContentLifecycleUnavailable
		return SearchContentDecision{
			Include:           true,
			LifecycleOverride: &unavailable,
			Reason:            SearchContentDecisionReasonRedact,
			ShadowDecision:    decision,
		}
	case ShadowDecisionUnknown:
		if reason == SearchUnknownReasonInputInvalid {
			return SearchContentDecision{
				Include:        false,
				Reason:         SearchContentDecisionReasonUnknownFailClosed,
				ShadowDecision: decision,
			}
		}
		return SearchContentDecision{
			Include:        true,
			Reason:         SearchContentDecisionReasonUnknownFailOpen,
			ShadowDecision: decision,
		}
	default:
		// Future-proofing: any unrecognized ShadowDecision value coarsens
		// to fail-open with a bounded reason. Promoting authority on a
		// surface MUST add an explicit mapping above before the new
		// decision can ship.
		return SearchContentDecision{
			Include:        true,
			Reason:         SearchContentDecisionReasonUnknownFailOpen,
			ShadowDecision: decision,
		}
	}
}

// NormalizeSearchContentAdapterMode parses an environment / config string
// into a canonical SearchContentAdapterMode. Any unrecognized or empty
// input is normalized to SearchContentAdapterModeShadow — the safe
// default. The function is intentionally NOT error-returning: shadow is
// always a correct answer, and a misconfigured env value MUST NOT take
// the route into enforce mode by accident.
func NormalizeSearchContentAdapterMode(raw string) SearchContentAdapterMode {
	m := SearchContentAdapterMode(raw)
	if m.IsValid() {
		return m
	}
	return SearchContentAdapterModeShadow
}


