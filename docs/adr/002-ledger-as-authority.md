# ADR-002 — Ledger as Authority

## Status

Accepted

## Context

Money on this platform crosses several semantic layers (gateway settlement, in-app escrow, seller payable, payout dispatch, refund reversal). Without an explicit canonical authority for balance truth, the platform risks split-brain accounting between wallet display, business balance, payout state, and gateway settlement.

## Decision

Double-entry ledger is the **canonical financial authority**.

- Ledger entries are the source of truth for: seller payable, gateway clearing, bank settlement, withdrawal pending / committed, seller debt, and platform revenue.
- Wallet is **display + derived state**. Wallet is not authority.
- Direct wallet mutation outside `WalletService` is forbidden.

## Consequences

### Positive

- replayability,
- reconciliation,
- auditability,
- deterministic settlement tracking.

### Negative

- operational complexity,
- reconciliation infrastructure burden,
- stricter mutation discipline,
- rollback complexity.

## Rejected Alternatives

- **Wallet-only balance truth** — no auditability; mutation ambiguity; impossible reconciliation.
- **Hybrid "best effort" accounting** — unclear authority; inconsistent rollback semantics; hidden money path risk.

## Operational Warnings

Ledger authority is meaningful only when:

- direct wallet mutation is removed or quarantined,
- refund / reversal semantics flow through canonical gateway pathways (refund is gateway-driven, never local-balance-driven),
- payout lifecycle (request → review → dispatch → settle / fail / rollback) is ledger-backed at every transition.
