// Package evaluator hosts the canonical visibility / governance evaluator and
// its shadow observability seams.
//
// PHASE C — FEED EVALUATOR SHADOW OBSERVABILITY
//
// This package currently implements only the shadow seam for the feed
// surface (Pattern A in docs/03-architecture/viewer-context-contract.md).
// It is observability-only and never affects runtime authority.
//
// Doctrine references:
//   - docs/FOUNDATION.md (Canonical Authorities — Visibility, Public Exposure)
//   - docs/DOCTRINE.md (Shadow Mode Doctrine, Observability Before Authority)
//   - docs/ADR.md (ADR-003 Governance Evaluator)
//   - docs/03-architecture/viewer-context-contract.md (Pattern A; partial-
//     ViewerContext §7.2; UNKNOWN semantics §6 / §8.2)
//   - docs/03-architecture/public-card-boundary-contract.md (separation of
//     evaluator decision from exposure rendering)
//   - docs/05-rollout/convergence-sequencing-addendum-viewercontext-evaluator.md
//     (feed-first SHADOW; not feed-first authority)
//   - docs/05-rollout/blocker-registry.md (BLOCKER-002, BLOCKER-004 —
//     observability-only, no closure, no severity reduction)
//
// Strict shadow rules enforced by this package:
//
//   - Legacy runtime remains the sole visibility authority on every surface.
//   - The shadow evaluator is pure: it performs no IO and no DB reads.
//     All inputs are hydrated by the caller (ViewerContext Contract §2.4).
//   - Missing inputs surface as UNKNOWN with a classified reason; the
//     evaluator never synthesizes fallback truth (ViewerContext Contract
//     §8.5).
//   - Shadow execution is asynchronous and fire-and-forget. It must not
//     change the runtime response bytes, latency envelope, or pagination.
//   - Per Shadow Mode Doctrine — Undefined Denominator Rule, divergence
//     categories that require observation of legacy-denied items
//     (LegacyDenyShadow*) are unobservable on this surface stage and are
//     never emitted from this package.
//
// Implementation gate citation (Convergence Constitution §22):
//
//   This module is the materialized observability seam for BLOCKER-002 and
//   BLOCKER-004. It does not close those blockers, does not lower their
//   severity, and does not enable evaluator authority.
package evaluator


