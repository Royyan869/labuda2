# ADR-004 — Discovery / Projection Boundary

## Status

Accepted

## Context

Discovery surfaces (feed, search, storefront) need read-optimization and ranking, but they must not be allowed to invent visibility or exposure semantics. Without an explicit layer separation, projections become hidden visibility authorities and ranking becomes a visibility filter.

## Decision

Discovery converges into an explicit layered topology:

```
Write Model
 → Projection / Read Model
 → Evaluator
 → Public Card Boundary
 → Ranking
```

Each layer's authority is bounded:

- **Write Model** — canonical mutation truth.
- **Projection / Read Model** — read optimization only. Never authority.
- **Evaluator** (ADR-003) — final visibility authority.
- **Public Card Boundary** — final exposure authority (which fields cross the wire).
- **Ranking** — ordering only. Visibility-agnostic; never overrides evaluator deny.

Projection prefiltering is allowed as a **subset** of evaluator deny — never as final visibility authority.

## Consequences

### Positive

- governance-aware discovery becomes possible,
- projection rollout is measurable,
- visibility layering is explicit,
- exposure rules are centralized,
- ranking separation is explicit.

### Negative

- projection freshness burden,
- invalidation complexity,
- dual-read rollout complexity,
- evaluator integration burden.

## Rejected Alternatives

- **Direct-table discovery** — visibility fragmentation; impossible convergence; exposure inconsistency.
- **Projection as truth** — stale authority risk; governance inconsistency.
- **Ranking-driven visibility** — governance bypass risk; blocked-user discoverability.

## Operational Warnings

Projection freshness and governance freshness have different operational requirements. Governance staleness tolerance is **stricter** than ranking freshness tolerance:

- a banned seller still visible after several seconds is an incident,
- ranking stale by a few minutes is acceptable.

Projection rollout MUST be shadow-first — dual-read, measure drift, measure stale rows, measure ranking divergence — before any cutover to projection-as-read-source.
