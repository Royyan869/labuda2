package evaluator

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"
)

// Search shadow seam bounded enum families per
// docs/05-rollout/search-endpoint-telemetry-enum-design.md.
//
// PHASE C — SEARCH SHADOW SEAM STAGE 1 (TELEMETRY ONLY)
//
// All values registered here are the BOUNDED scope for the first-seam
// landing on /search/content per docs/05-rollout/search-shadow-seam-
// landing-task-design.md §5.1 / §8. Values reserved at design level but
// not consumed by /search/content are intentionally NOT registered;
// scaffolding TODO enum stubs are forbidden per
// docs/05-rollout/search-endpoint-telemetry-enum-design.md §15.
//
// Cardinality is bounded per docs/05-rollout/search-endpoint-telemetry-
// enum-design.md §11. No raw account_status, no raw status enum, no
// deleted_at timestamp, no UUID, no email, no query string, no SQL error
// text appears as a label value.

// SearchEndpoint identifies the per-endpoint label on search-shadow
// telemetry. Per docs/05-rollout/search-endpoint-telemetry-enum-design.md
// §3.2 the design enumerates six endpoint members; the first-seam
// landing registers ONLY search_content. Per docs/05-rollout/search-
// shadow-seam-landing-task-design.md §5.1 / §13, registering reserved
// values is forbidden until each endpoint's own material task lands.
type SearchEndpoint string

const (
	// SearchEndpointContent — /api/v1/search/content per
	// backend/cmd/core_server/routes_core.go:280.
	SearchEndpointContent SearchEndpoint = "search_content"
)

// CandidateSetOption identifies the seam's candidate-set insertion-point
// option per docs/05-rollout/search-shadow-seam-architecture.md §2 and
// docs/05-rollout/search-endpoint-telemetry-enum-design.md §4.
//
// First-seam landing registers ONLY option_a_handler_post_response per
// docs/05-rollout/search-shadow-seam-landing-task-design.md §4.2 / §5.1.
// Options B and C are reserved-but-not-registered at this material task.
type CandidateSetOption string

const (
	// CandidateSetOptionAHandlerPostResponse — runner is invoked after
	// the legacy gin.H response is committed; per-page legacy-allowed
	// denominator only; legacy_deny_* cells reserved-but-unobservable
	// per docs/05-rollout/search-shadow-seam-architecture.md §3.2.
	CandidateSetOptionAHandlerPostResponse CandidateSetOption = "option_a_handler_post_response"
)

// SearchDivergenceCell is the bounded 7-cell divergence taxonomy per
// docs/05-rollout/search-endpoint-telemetry-enum-design.md §5.1.
//
// On Option A the legacy_deny_* cells are RESERVED-NOT-EMITTED per
// docs/05-rollout/search-shadow-seam-architecture.md §3.2 — the legacy
// SQL filter excluded these rows before the runner saw them. Treating
// absence-of-emission as zero is the canonical fake-parity violation
// (docs/05-rollout/search-shadow-seam-architecture.md §3.7).
type SearchDivergenceCell string

const (
	SearchDivLegacyAllowShadowAllow SearchDivergenceCell = "legacy_allow_shadow_allow"
	SearchDivLegacyAllowShadowDeny  SearchDivergenceCell = "legacy_allow_shadow_deny"
	// SearchDivLegacyDenyShadowAllow — RESERVED on Option A per
	// search-shadow-seam-architecture.md §3.2; never emitted.
	SearchDivLegacyDenyShadowAllow SearchDivergenceCell = "legacy_deny_shadow_allow"
	// SearchDivLegacyDenyShadowDeny — RESERVED on Option A; never emitted.
	SearchDivLegacyDenyShadowDeny SearchDivergenceCell = "legacy_deny_shadow_deny"
	SearchDivShadowUnknown        SearchDivergenceCell = "shadow_unknown"
	SearchDivShadowError          SearchDivergenceCell = "shadow_error"
	SearchDivShadowPanic          SearchDivergenceCell = "shadow_panic"
)

// SearchUnknownReason is the bounded UNKNOWN reason taxonomy per
// docs/05-rollout/search-endpoint-telemetry-enum-design.md §6.1 and
// docs/05-rollout/search-shadow-seam-architecture.md §7.
type SearchUnknownReason string

const (
	SearchUnknownReasonNone                     SearchUnknownReason = ""
	SearchUnknownReasonViewerOverlayMissing     SearchUnknownReason = "viewer_overlay_missing"
	SearchUnknownReasonTargetOverlayMissing     SearchUnknownReason = "target_overlay_missing"
	SearchUnknownReasonRankingContextMissing    SearchUnknownReason = "ranking_context_missing"
	SearchUnknownReasonHydrationError           SearchUnknownReason = "hydration_error"
	SearchUnknownReasonPaginationContextInvalid SearchUnknownReason = "pagination_context_invalid"
	SearchUnknownReasonCandidateSetIncomplete   SearchUnknownReason = "candidate_set_incomplete"
	SearchUnknownReasonInputInvalid             SearchUnknownReason = "input_invalid"
)

// SearchUnknownSource sub-classifies viewer_overlay_missing /
// target_overlay_missing / hydration_error per
// docs/05-rollout/search-endpoint-telemetry-enum-design.md §6.2.
//
// First-seam landing on /search/content registers ONLY the six values
// the endpoint consumes per docs/05-rollout/search-shadow-seam-landing-
// task-design.md §8.6. seller_account, linked_listing, parent_content
// are NOT consumed by /search/content and are NOT registered.
type SearchUnknownSource string

const (
	SearchUnknownSourceNone             SearchUnknownSource = ""
	SearchUnknownSourceIdentity         SearchUnknownSource = "identity"
	SearchUnknownSourceRelationship     SearchUnknownSource = "relationship"
	SearchUnknownSourceViewerLifecycle  SearchUnknownSource = "viewer_lifecycle"
	SearchUnknownSourceTargetLifecycle  SearchUnknownSource = "target_lifecycle"
	SearchUnknownSourceTargetModeration SearchUnknownSource = "target_moderation"
	SearchUnknownSourceCapability       SearchUnknownSource = "capability"
)

// SearchOverlay identifies which overlay an overlay_status emission
// applies to per docs/05-rollout/search-endpoint-telemetry-enum-design.md
// §7.1. First-seam landing on /search/content registers the six
// consumed overlays per docs/05-rollout/search-shadow-seam-landing-task-
// design.md §8.7; viewer_moderation / seller_account / parent_content /
// linked_listing are NOT consumed and NOT registered.
type SearchOverlay string

const (
	SearchOverlayIdentity         SearchOverlay = "identity"
	SearchOverlayViewerLifecycle  SearchOverlay = "viewer_lifecycle"
	SearchOverlayTargetLifecycle  SearchOverlay = "target_lifecycle"
	SearchOverlayRelationship     SearchOverlay = "relationship"
	SearchOverlayTargetModeration SearchOverlay = "target_moderation"
	SearchOverlayCapability       SearchOverlay = "capability"
)

// SearchOverlayStatus per docs/05-rollout/search-endpoint-telemetry-enum-
// design.md §7.2.
type SearchOverlayStatus string

const (
	SearchOverlayStatusPresent SearchOverlayStatus = "present"
	SearchOverlayStatusMissing SearchOverlayStatus = "missing"
	SearchOverlayStatusError   SearchOverlayStatus = "error"
)

// SearchLifecycleCategory identifies which lifecycle category a
// lifecycle-state emission applies to per docs/05-rollout/search-
// endpoint-telemetry-enum-design.md §8.1.
//
// First-seam landing on /search/content registers only the three
// categories the endpoint consumes per docs/05-rollout/search-shadow-
// seam-landing-task-design.md §8.9; seller_account / listing / auction /
// linked_listing / parent_content are NOT registered.
type SearchLifecycleCategory string

const (
	SearchLifecycleCategoryViewerAccount SearchLifecycleCategory = "viewer_account"
	SearchLifecycleCategoryTargetAccount SearchLifecycleCategory = "target_account"
	SearchLifecycleCategoryContent       SearchLifecycleCategory = "content"
)

// SearchPublicLifecycleState is the canonical Public Lifecycle State
// coarsening per docs/05-rollout/search-endpoint-telemetry-enum-design.md
// §8.2. Raw account_status / *.status enum values are FORBIDDEN as
// labels per §8.5; coarsening to this enum is the canonical emission.
type SearchPublicLifecycleState string

const (
	SearchPublicLifecycleStateActive      SearchPublicLifecycleState = "active"
	SearchPublicLifecycleStateUnavailable SearchPublicLifecycleState = "unavailable"
	SearchPublicLifecycleStateRemoved     SearchPublicLifecycleState = "removed"
)

// SearchExposureSemantic is the canonical evaluator-output / boundary-
// rendering semantic per docs/05-rollout/search-endpoint-telemetry-enum-
// design.md §9.1. The seven values are bounded; extension requires an
// ADR amending docs/03-architecture/public-card-boundary-contract.md.
//
// On /search/content at first-seam landing, PublicCard runtime is absent
// (BLOCKER-005); REDACT / TOMBSTONE / SUSPENDED / DELETED rendering is
// not interpretable until PublicCard adoption per docs/05-rollout/search-
// publiccard-adoption-design.md §10.3. Per docs/05-rollout/search-
// shadow-seam-landing-task-design.md §7.4, all such cells emit as
// unknown_shadow_only with a structured-log annotation
// blocker="blocker_005".
type SearchExposureSemantic string

const (
	SearchExposureSemanticAllow             SearchExposureSemantic = "allow"
	SearchExposureSemanticRedact            SearchExposureSemantic = "redact"
	SearchExposureSemanticTombstone         SearchExposureSemantic = "tombstone"
	SearchExposureSemanticDeleted           SearchExposureSemantic = "deleted"
	SearchExposureSemanticSuspended         SearchExposureSemantic = "suspended"
	SearchExposureSemanticAnonymousFallback SearchExposureSemantic = "anonymous_fallback"
	SearchExposureSemanticUnknownShadowOnly SearchExposureSemantic = "unknown_shadow_only"
)

// hashUUID returns a 16-char hex prefix of a SHA-256 over the UUID. Used
// for structured-log forensic correlation per docs/05-rollout/search-
// endpoint-telemetry-enum-design.md §12.4 (IDs only in logs if hashed/
// redacted; raw UUIDs FORBIDDEN as metric labels per §11.3).
//
// The prefix is short enough that cardinality stays bounded (~2^64 max),
// long enough that per-row collisions are operationally negligible for
// forensic correlation. The hash is one-way; the original UUID cannot be
// recovered from the label.
func hashUUID(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	h := sha256.Sum256(id[:])
	return hex.EncodeToString(h[:8])
}


