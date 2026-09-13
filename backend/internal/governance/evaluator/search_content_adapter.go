package evaluator

// PHASE 3A — /search/content evaluator authority promotion prerequisite.
//
// This file ONLY lands the pure adapter type + mapping function used to
// translate the existing shadow-mode evaluator decision into an
// enforcement-ready outcome. The adapter mapping is unconditional and is
// consumed both by the synchronous enforcement pass and by the
// observability runner (`would_enforce_*` telemetry).
//
// CONTRACT:
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
//   - Enforcement is unconditional. The adapter mapping IS the canonical
//     business answer; there is no shadow/enforce branch. Batch 3B (the
//     enforcement wiring) reads `Include` / `LifecycleOverride` and
//     applies them to the response.

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
	// Include reports whether the row should appear in the enforced
	// response. Rows with Include=false are dropped; rows with a
	// LifecycleOverride are kept with the coarsened lifecycle.
	//
	// The same value is emitted by the observability runner for
	// would-enforce telemetry.
	Include bool

	// LifecycleOverride, when non-nil, is the coarsened public lifecycle
	// string the card should adopt instead of the lifecycle the surface
	// would normally emit. The vocabulary is the canonical
	// {active, unavailable, removed} set. Nil means "do not override."
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
//	  (handler construction defect; the adapter fails closed on it).
//	ShadowDecisionUnknown    +
//	    any other reason     → Include=true,  no override, Reason=unknown_fail_open
//	  (overlay-missing or hydration-error; legacy authority is preserved
//	   per Batch 3 audit doctrine — "incomplete overlay ≠ proof of denial").
//
// The mapping above is unconditional; the caller always acts on
// Include/Override.
func AdaptSearchContentDecision(
	decision ShadowDecision,
	reason SearchUnknownReason,
	_ SearchExposureSemantic, // accepted for forward-compat with the evaluator return shape; unused today
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




