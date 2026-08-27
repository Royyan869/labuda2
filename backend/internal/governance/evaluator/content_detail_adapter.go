package evaluator

import "strings"

// D1 — /contents/:id (content detail) governance convergence adapter.
//
// This file lands the pure adapter type + mapping function used to translate
// the canonical EvaluateContentDetail shadow decision into an enforcement-
// ready outcome. It mirrors search_content_adapter.go / feed_adapter.go BUT
// honors detail-surface doctrine differences:
//
//   * Feed / /search/content are DISCOVERY surfaces — further-restrict-only;
//     UNKNOWN fail-OPEN (feed) or fail-CLOSED-on-input-invalid only
//     (/search/content); TOMBSTONE / REDACT emit lifecycle override on the
//     wire so the row remains visible with a degraded card.
//
//   * /contents/:id is a DETAIL surface — the viewer explicitly navigated
//     to a single object. Doctrine
//     (docs/contracts/content-detail-visibility-doctrine.md §8.5):
//       - fail-CLOSED on UNKNOWN (the seam must NEVER silently pass an
//         unhydrated decision through to the wire);
//       - tombstone semantic is HTTP 404 on this surface (existing
//         architectural truth — see content_handler.go GetContent gate),
//         no in-wire tombstone JSON body, no LifecycleOverrides map on the
//         enforcement result.
//
// CONTRACT:
//
//   - Pure: no DB reads, no IO, no logging. Caller-side telemetry only.
//   - Single source of decision truth: the canonical ShadowDecision enum
//     from shadow_types.go. No new top-level enum is introduced.
//   - Mode is a passive label. The adapter NEVER conditionally changes its
//     mapping based on mode; mode is forwarded so callers can emit
//     content_detail_evaluator_enforce_mode_total / would_enforce_decision_total
//     telemetry consistently.

// ContentDetailEvaluatorMode is the operating mode of the /contents/:id
// evaluator integration. shadow keeps current behaviour; enforce activates
// the synchronous fail-CLOSED-on-non-allow path. Invalid / empty values
// normalise to shadow via NormalizeContentDetailEvaluatorMode.
type ContentDetailEvaluatorMode string

const (
	// ContentDetailEvaluatorModeShadow is the default operating mode. The
	// enforcement helper short-circuits to allow=true; the legacy handler's
	// own gate remains the sole visibility authority.
	ContentDetailEvaluatorModeShadow ContentDetailEvaluatorMode = "shadow"

	// ContentDetailEvaluatorModeEnforce activates the synchronous
	// fail-CLOSED enforcement. Any non-ALLOW decision (DENY / TOMBSTONE /
	// REDACT / UNKNOWN) causes the handler to convert its successful
	// response to a 404. Wire shape is byte-identical to a legacy-gate 404
	// (no in-wire tombstone payload — preserves existing architectural
	// truth).
	ContentDetailEvaluatorModeEnforce ContentDetailEvaluatorMode = "enforce"
)

// IsValid reports whether m is a recognised mode.
func (m ContentDetailEvaluatorMode) IsValid() bool {
	switch m {
	case ContentDetailEvaluatorModeShadow, ContentDetailEvaluatorModeEnforce:
		return true
	}
	return false
}

// NormalizeContentDetailEvaluatorMode parses an env / config string into a
// canonical ContentDetailEvaluatorMode. Any unrecognised or empty value
// falls safely to shadow — enforce is opt-in only. Mirror of the feed /
// search normalize helpers.
func NormalizeContentDetailEvaluatorMode(raw string) ContentDetailEvaluatorMode {
	switch ContentDetailEvaluatorMode(strings.ToLower(strings.TrimSpace(raw))) {
	case ContentDetailEvaluatorModeEnforce:
		return ContentDetailEvaluatorModeEnforce
	default:
		return ContentDetailEvaluatorModeShadow
	}
}

// ContentDetailDecisionReason is the bounded telemetry-safe reason label
// emitted on a non-ALLOW adapter outcome. Cardinality is intentionally
// narrow — the detail surface coarsens all non-ALLOW decisions to a single
// "deny" or "unknown_fail_closed" reason because the wire response (HTTP
// 404) does not differentiate sub-reasons. Sub-reasons are still observable
// via the existing labuda_evaluator_shadow_unknown_total /
// labuda_evaluator_shadow_divergence_total{surface="content_detail"}
// counters.
type ContentDetailDecisionReason string

const (
	ContentDetailDecisionReasonNone              ContentDetailDecisionReason = ""
	ContentDetailDecisionReasonDeny              ContentDetailDecisionReason = "deny"
	ContentDetailDecisionReasonUnknownFailClosed ContentDetailDecisionReason = "unknown_fail_closed"
)

// ContentDetailDecision is the adapter's per-request enforcement-ready
// output. Note there is no LifecycleOverride field — detail surface
// tombstones via HTTP 404, not via in-wire lifecycle vocabulary. This is a
// deliberate doctrine divergence from feed/search and is documented in the
// package docstring above.
type ContentDetailDecision struct {
	// Include reports whether the handler should emit its successful
	// response. In ContentDetailEvaluatorModeEnforce, Include=false causes
	// the handler to write HTTP 404 instead of the 200 payload the legacy
	// gate already approved.
	//
	// In ContentDetailEvaluatorModeShadow the caller MUST IGNORE this for
	// response composition (legacy handler remains authority) but SHOULD
	// emit would-enforce telemetry from it.
	Include bool

	// Reason is the bounded telemetry-safe label. Empty when the decision
	// is plain ALLOW.
	Reason ContentDetailDecisionReason

	// ShadowDecision passes through the raw evaluator decision for tests
	// and downstream callers that already log against the shadow enum.
	ShadowDecision ShadowDecision
}

// AdaptContentDetailDecision converts the pure shadow evaluator output for
// one content-detail request into an enforcement-ready decision.
//
// Mapping (input → output):
//
//	ShadowDecisionAllow      → Include=true,  Reason=none
//	ShadowDecisionDeny       → Include=false, Reason=deny
//	ShadowDecisionTombstone  → Include=false, Reason=deny  (detail collapses to 404)
//	ShadowDecisionRedact     → Include=false, Reason=deny  (detail collapses to 404)
//	ShadowDecisionUnknown    → Include=false, Reason=unknown_fail_closed
//	  (DETAIL fail-CLOSED on every UNKNOWN reason — doctrine §8.5. This is
//	   the documented inversion of feed's fail-OPEN policy.)
//
// The mode parameter is a passive label. The mapping above is unconditional;
// only the caller's reaction to Include changes between shadow and enforce.
func AdaptContentDetailDecision(
	decision ShadowDecision,
	_ UnknownReason, // accepted for forward-compat + parity with sibling adapters
	_ ContentDetailEvaluatorMode, // passive label
) ContentDetailDecision {
	switch decision {
	case ShadowDecisionAllow:
		return ContentDetailDecision{
			Include:        true,
			Reason:         ContentDetailDecisionReasonNone,
			ShadowDecision: decision,
		}
	case ShadowDecisionDeny, ShadowDecisionTombstone, ShadowDecisionRedact:
		return ContentDetailDecision{
			Include:        false,
			Reason:         ContentDetailDecisionReasonDeny,
			ShadowDecision: decision,
		}
	case ShadowDecisionUnknown:
		// Detail fail-CLOSED on every UNKNOWN reason. The handler emits
		// 404; telemetry counter unknown_fail_closed lets ops observe
		// hydration-degradation rate without leaking visibility.
		return ContentDetailDecision{
			Include:        false,
			Reason:         ContentDetailDecisionReasonUnknownFailClosed,
			ShadowDecision: decision,
		}
	default:
		// Future-proofing: any unrecognised ShadowDecision coarsens to
		// fail-CLOSED with the unknown_fail_closed label. Promoting a new
		// decision value on this surface MUST add an explicit mapping
		// above before it can ship.
		return ContentDetailDecision{
			Include:        false,
			Reason:         ContentDetailDecisionReasonUnknownFailClosed,
			ShadowDecision: decision,
		}
	}
}


