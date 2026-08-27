// Package viewercontext is the canonical runtime materialization of the
// ViewerContext Contract per docs/03-architecture/viewer-context-contract.md.
//
// PHASE C — VIEWERCONTEXT RUNTIME (PROGRESSIVE PER-ENDPOINT THREADING)
//
// Per docs/05-rollout/search-content-viewercontext-runtime-threading-task-design.md,
// this package is the canonical runtime ViewerContext type used by the first
// per-endpoint threading material task on the search discovery surface
// (`/search/content`). It is NOT an evaluator, NOT a shadow seam, NOT a card
// builder, NOT an authority surface — it is the input contract that those
// downstream surfaces consume.
//
// Canonical doctrine references:
//
//   - docs/03-architecture/viewer-context-contract.md
//     §2 Core Rules (viewer never nil; AnonymousViewer explicit; caller
//     hydrates truth; evaluator never fetches internally; overlays are
//     inputs not authority lookups),
//     §3 Topology (AnonymousViewer / AuthenticatedViewer; no third "system"
//     shape),
//     §4 Overlay Model (identity / lifecycle / capability / relationship /
//     moderation),
//     §5 Lifecycle (created at exactly one boundary per request; immutable
//     after construction-boundary handoff),
//     §6 Pattern A — Public Discovery,
//     §8 Forbidden Patterns (nil viewer fallback; internal evaluator DB
//     reads; ViewerContext mutation; outbox snapshot trust; ad-hoc
//     ViewerContext creation).
//
//   - docs/05-rollout/search-overlay-ownership-matrix.md §3 / §5 — caller
//     hydrates target overlays; evaluator / repository / card builder
//     forbidden as hydrators.
//
//   - docs/05-rollout/search-lifecycle-overlay-topology.md §3 / §8 — Public
//     Lifecycle State coarsening (active / unavailable / removed).
//
//   - docs/05-rollout/search-endpoint-telemetry-enum-design.md §7 / §8 —
//     overlay enum and lifecycle enum design (this package's bounded
//     enums are aligned with that design but NOT registered as metric
//     labels by this package; metric registration is the future seam-
//     landing material task per docs/05-rollout/search-shadow-seam-
//     landing-task-design.md).
//
// Strict rules enforced by this package:
//
//   - The type is constructed exactly once per request, at the canonical
//     construction boundary (HTTP boundary for Pattern A surfaces, per
//     viewer-context-contract.md §5.1).
//
//   - After construction the value is read-only. Downstream layers do not
//     mutate it (viewer-context-contract.md §8.3).
//
//   - AnonymousViewer is an explicit, named state. Nil values, empty
//     strings, and synthesized "system" actors are forbidden as anonymous
//     fallbacks (viewer-context-contract.md §8.1).
//
//   - Email is forbidden in identity overlay per viewer-context-contract.md
//     §4.1 and BLOCKER-001 doctrine.
//
//   - Public Lifecycle State coarsening at construction. Raw account_status
//     enum values do not leave this package as public-facing values; the
//     coarsened state (active / unavailable / removed) is the canonical
//     consumed shape per docs/05-rollout/search-lifecycle-overlay-
//     topology.md §3 and docs/05-rollout/search-endpoint-telemetry-enum-
//     design.md §8.3.
//
// Convergence Constitution citation:
//
//   This package is one half of Sequence A on `/search/content` per
//   docs/05-rollout/search-shadow-seam-landing-task-design.md §6.4.
//   The seam-landing material task (Sequence A second half) is a
//   separately authorized future task; this package prepares but does
//   not authorize it.
//
//   This package does not close BLOCKER-002 or BLOCKER-008. It is partial
//   preparation only per docs/05-rollout/search-content-viewercontext-
//   runtime-threading-task-design.md §11.
package viewercontext


