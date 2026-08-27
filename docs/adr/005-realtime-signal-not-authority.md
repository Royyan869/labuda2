# ADR-005 — Realtime Signal, Not Authority

## Status

Accepted

## Context

Realtime / WebSocket delivery is operationally convenient (immediate, user-visible, low-latency) and easy to implicitly trust as authoritative. Treating WebSocket frames as final state silently couples replay safety, governance, payment confirmation, and moderation to a transport layer that is not designed for those guarantees.

## Decision

Realtime / WebSocket is **signal-only**.

- Canonical truth remains in the domain DB / REST / canonical write model.
- Realtime exists to accelerate UX, reduce polling, improve responsiveness.
- Realtime does NOT define: truth, payment state, final notification state, moderation truth.
- Outbox events are durable facts of what happened — not current truth.

Delivery-time governance is mandatory:

- evaluator runs at WebSocket subscribe,
- evaluator runs at WebSocket broadcast (per-recipient, fresh ViewerContext),
- evaluator runs at notification delivery and at every retry.

Enqueue-time evaluator decisions are observability / correlation only — never authority.

## Consequences

### Positive

- clearer authority boundaries,
- safer replay semantics,
- governance-at-delivery,
- rollback survivability,
- reconnect / reconciliation clarity.

### Negative

- more reconciliation flows,
- optimistic UI harder,
- client complexity,
- replay complexity.

## Rejected Alternatives

- **WebSocket as authority** — replay ambiguity; stale state risk; impossible auditability.
- **Trust enqueue-time governance forever** — stale retry leakage; block propagation failure; moderation inconsistency.
- **Activate dormant realtime directly** — unknown blast radius; governance uncertainty; no rollback confidence.

## Operational Warnings

Realtime rollout is unsafe unless:

- the evaluator is integrated at delivery time,
- replay is audited,
- WebSocket frames are versioned (additive evolution; unknown frames are ignored, never trusted),
- rollback is measurable,
- divergence is observable.

Financial replay never auto-applies without explicit reconciliation authority.
