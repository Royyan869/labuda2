# ADR-003 — Governance Evaluator

## Status

Accepted

## Context

Visibility, block semantics, follower-only semantics, moderation overlays, and lifecycle propagation appear on many surfaces (feed, search, realtime, notifications, profile, saved items). Without a single authority, these semantics drift across surfaces and admin / moderation effects fail to propagate consistently.

## Decision

The Canonical Visibility / Governance Evaluator is the **single operational authority** for visibility decisions across discovery, realtime, notifications, saved items, profile, and detail surfaces.

- Evaluator output: `ALLOW` / `DENY` / `TOMBSTONE` / `REDACT` (and `UNKNOWN` only in shadow mode).
- Evaluator consumes structured input: ViewerContext + target context + surface context. It does NOT perform DB reads internally.
- Evaluator is the only legal place that decides whether an entity may be exposed.

## Consequences

### Positive

- converged visibility semantics,
- measurable divergence,
- delivery-time governance for realtime / notifications,
- replay governance consistency.

### Negative

- evaluator hydration complexity,
- viewer-context propagation burden,
- performance pressure,
- rollout complexity.

## Rejected Alternatives

- **Inline predicates everywhere** — fragmented semantics; impossible convergence; invisible drift.
- **Projection as final authority** — stale visibility risk; governance drift.
- **Realtime trust-once delivery** — stale retry governance leak; block propagation failure.

## Operational Warnings

The evaluator MUST NOT:

- become a hidden DB authority (no internal DB reads — caller hydrates truth),
- become ranking authority (visibility ≠ ordering),
- become payment authority,
- bypass the public-card boundary (deciding what bytes are emitted is the boundary's job; the evaluator decides whether emission is allowed at all).

`UNKNOWN` decisions are allowed only in shadow mode. Production fails closed on unknown.
