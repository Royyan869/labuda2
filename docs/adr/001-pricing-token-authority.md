# ADR-001 — Pricing Token Authority

## Status

Accepted

## Context

Pricing on a multi-entry commerce platform (listing checkout, auction claim, negotiation checkout, chat-initiated checkout) faces a structural risk: if any caller can compute or submit pricing fields directly, calculation drifts across paths and the platform loses a single auditable source for what the buyer actually owes.

Pricing influences subtotal, fee, discount, shipping total, and coins usage. Each input is a potential drift surface. Each entry path is a potential duplicate calculation surface.

## Decision

Pricing token / PricingSnapshot is the **canonical pricing authority**.

- Order creation MUST consume a validated pricing token. The token itself encodes subtotal, fee, discount, coins, shipping breakdown, and the buyer / listing / auction binding.
- Frontend / request payload submits **intent only** (chosen address, chosen shipping option, coins opt-in). It is never pricing authority.
- All commerce entry paths converge into:

  ```
  Pricing Token → Order → Payment → Escrow
  ```

## Consequences

### Positive

- single pricing authority,
- deterministic replay,
- negotiation and chat-commerce convergence,
- cleaner auditability.

### Negative

- token lifecycle / invalidation complexity,
- stricter checkout coupling,
- retry semantics harder.

## Rejected Alternatives

- **Frontend-authoritative pricing** — client untrusted; pricing drift unavoidable; promotion / coins logic duplicated; replay non-deterministic.
- **Recalculate pricing everywhere** — duplicate logic; negotiation divergence; hidden rounding drift.
- **Chat-owned pricing** — chat is not commerce owner; shadow commerce path; impossible auditability.

## Operational Warnings

The pricing token itself MUST NOT become a hidden business-logic dump. Token authority must remain deterministic, auditable, replayable, and canonicalized. Any new pricing rule (new fee class, new discount class) is added at the canonical pricing engine, not at the token-issuance edge.
