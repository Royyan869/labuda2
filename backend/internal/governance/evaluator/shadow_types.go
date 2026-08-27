package evaluator

// SurfaceLabel identifies the canonical surface a shadow run is observing.
// Held to a small enum so Prometheus label cardinality stays bounded.
type SurfaceLabel string

const (
	// SurfaceFeed is the feed-first shadow surface
	// (sequencing-addendum §3.1 / §4 — feed-first SHADOW only).
	SurfaceFeed SurfaceLabel = "feed"

	// SurfaceSearch is the discovery search surface. Per
	// docs/05-rollout/search-endpoint-telemetry-enum-design.md §3.1, all
	// search-shadow telemetry carries surface="search". The label is
	// emitted only by the search shadow seam — feed metrics retain
	// surface="feed". Cross-surface aggregation is denominator-dishonest
	// per docs/05-rollout/search-shadow-seam-architecture.md §6.6.
	SurfaceSearch SurfaceLabel = "search"

	// SurfaceContentDetail is the public content-detail read endpoint
	// surface (`GET /api/v1/contents/:id`). Telemetry shape mirrors
	// SurfaceFeed but the divergence taxonomy emits all four legacy_×_
	// shadow_× cells — the legacy handler has explicit deny logic
	// (IsHidden / StatusDeleted gated on IsAdmin) that the shadow can
	// compare against, so denominators are well-defined on both sides.
	// See docs/contracts/content-detail-visibility-doctrine.md §8.
	SurfaceContentDetail SurfaceLabel = "content_detail"
)

// ShadowDecision is the per-item decision the shadow evaluator emits.
//
// Mirrors the canonical evaluator output set defined in
// docs/ARCHITECTURE.md (Evaluator Authority Design — Output Contract):
// ALLOW / DENY / TOMBSTONE / REDACT, with UNKNOWN allowed only in shadow
// per Shadow Mode Doctrine.
type ShadowDecision string

const (
	ShadowDecisionAllow     ShadowDecision = "allow"
	ShadowDecisionDeny      ShadowDecision = "deny"
	ShadowDecisionTombstone ShadowDecision = "tombstone"
	ShadowDecisionRedact    ShadowDecision = "redact"
	ShadowDecisionUnknown   ShadowDecision = "unknown"
)

// DivergenceCategory classifies legacy-vs-shadow agreement per item.
//
// Per Shadow Mode Doctrine — Undefined Denominator Rule, the categories
// LegacyDenyShadowAllow and LegacyDenyShadowDeny require observation of
// items the legacy filter did not return. The feed shadow seam consumes
// only legacy-allowed items and therefore CANNOT emit those two
// categories. They are listed for completeness and reserved for broader
// shadow stages that operate on a wider candidate set.
type DivergenceCategory string

const (
	// DivLegacyAllowShadowAllow is the agreement cell — legacy and shadow
	// both ALLOW the item.
	DivLegacyAllowShadowAllow DivergenceCategory = "legacy_allow_shadow_allow"

	// DivLegacyAllowShadowDeny is the primary divergence signal for this
	// surface stage — legacy ALLOWED an item that shadow would DENY (or
	// TOMBSTONE). Each occurrence is a candidate governance leak for
	// post-rollout authority activation.
	DivLegacyAllowShadowDeny DivergenceCategory = "legacy_allow_shadow_deny"

	// DivLegacyDenyShadowAllow — undefined denominator on this surface.
	// Reserved enum value; never emitted by the feed seam.
	DivLegacyDenyShadowAllow DivergenceCategory = "legacy_deny_shadow_allow"

	// DivLegacyDenyShadowDeny — undefined denominator on this surface.
	// Reserved enum value; never emitted by the feed seam.
	DivLegacyDenyShadowDeny DivergenceCategory = "legacy_deny_shadow_deny"

	// DivShadowUnknown — shadow evaluator returned UNKNOWN; divergence is
	// not classifiable until overlays are hydrated.
	DivShadowUnknown DivergenceCategory = "shadow_unknown"
)

// OverlayKind enumerates the canonical overlays from the ViewerContext
// Contract §4. Used as the "overlay" Prometheus label.
type OverlayKind string

const (
	OverlayIdentity     OverlayKind = "identity"
	OverlayLifecycle    OverlayKind = "lifecycle"
	OverlayCapability   OverlayKind = "capability"
	OverlayRelationship OverlayKind = "relationship"
	OverlayModeration   OverlayKind = "moderation"
)

// OverlayStatus is the per-overlay hydration status emitted to telemetry.
type OverlayStatus string

const (
	OverlayStatusPresent OverlayStatus = "present"
	OverlayStatusMissing OverlayStatus = "missing"
	OverlayStatusError   OverlayStatus = "error"
)

// UnknownReason classifies why the shadow evaluator returned UNKNOWN.
type UnknownReason string

const (
	UnknownReasonNone                 UnknownReason = ""
	UnknownReasonViewerOverlayMissing UnknownReason = "viewer_overlay_missing"
	UnknownReasonTargetOverlayMissing UnknownReason = "target_overlay_missing"
	UnknownReasonHydrationError       UnknownReason = "hydration_error"
	UnknownReasonInputInvalid         UnknownReason = "input_invalid"
)

// HydrationSource labels an overlay-hydration error by source.
type HydrationSource string

const (
	HydrationSourceViewer     HydrationSource = "viewer"
	HydrationSourceOwner      HydrationSource = "owner"
	HydrationSourceBlockedSet HydrationSource = "blocked_set"
)



