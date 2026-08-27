# Doctrine — Revocable Trust Model

> **Status:** CANONICAL v1
> **Scope:** Verification, Seller Capability, Wallet, Payout, Dispute, Moderation, Audit.

## Canonical Wording

> *"Verification is not permanent moral approval. It is a revocable financial trust decision."*

> *"Trust restriction should reduce authority, not erase history."*

> *"Active obligations survive trust downgrade."*

## Core Principle

`approved` is **not terminal forever**. Verification is a **revocable** financial-trust decision: admin MAY transition an approved seller into a downgraded state when fraud, falsified documents, or serious violations are discovered. The reverse path also exists — investigation may clear an account back to `approved`.

## What Survives a Trust Downgrade

When trust is downgraded (Layer 5 → Layer 6, see [Capability Matrix](./capability-matrix.md)), the following MUST continue:

- **Seller existence** — the account is not deleted, not hidden.
- **Historical visibility** — past listings, orders, reviews remain visible per their own domain rules.
- **Audit traceability** — every state transition remains queryable by admin / moderator.
- **Active order lifecycle** — delivery, dispute, refund flows already in motion proceed normally.
- **Active dispute lifecycle** — disputes already opened resolve under their own rules; trust downgrade does not freeze them.
- **Support interaction** — the seller can still talk to support and respond to obligations.
- **Historical finance visibility** — balance is visible; past transactions queryable.
- **Balance** — *"Blocked withdrawal does not erase seller ownership."* The platform does not confiscate. Funds remain seller liability.

## What Is Cut During a Trust Downgrade

The following authorities are revoked (or MAY be restricted):

- **Withdraw / payout** — blocked.
- **New listing publish** — MAY be restricted (severity-dependent; exact matrix not locked).
- **New auction creation** — MAY be restricted.
- **Growth capability** (promotion, boost listing) — MAY be restricted.
- **Trust escalation actions** — blocked.

## Trust State Transitions (canonical target)

`approved` is the trust state from which downgrade transitions can originate. Downgraded states share the property that seller existence persists.

- `approved → suspended` — temporary admin pause; reversible.
- `approved → under_investigation` — payout held while admin investigates actively.
- `approved → revoked` — trust authority fully withdrawn after investigation.
- `under_investigation → approved` — investigation cleared.
- `under_investigation → suspended / revoked / needs_resubmission` — investigation outcome.
- `suspended → approved` — admin lifts suspension.

For the full state machine (forward path included), see [Verification Review Governance](./verification-review-governance.md).

## Liability Invariant (cross-domain)

Trust downgrade is **financial-authority gating**, not **financial-state mutation**.

- Ledger MUST NOT be mutated by a trust-state change.
- Escrow release on order completion follows finance rules normally; released funds enter the seller's balance even if withdrawal is blocked.
- Refund / payout semantics remain gateway-driven; trust downgrade closes the extraction door without touching the gateway.

## Cross-Domain Impacts

- **Wallet / Escrow** — balance untouched by downgrade; only the withdraw entry-point closes.
- **Payout** — entry-point reads trust state; Layer 6 blocks. See [Seller Authority Separation](./seller-authority-separation.md).
- **Dispute** — active dispute lifecycle survives. Dispute-domain freezes (e.g. post-release dispute freeze, operate as a separate financial tool — they coexist with trust state, not substitute.
- **Listing / Auction** — publish capability MAY be restricted on downgrade; existing listings MAY be frozen depending on severity (matrix not locked).
- **Moderation / Governance** — downgrade is admin authority. RBAC sub-role detailing is a Governance-domain concern.
- **Audit** — every transition (forward or reverse) MUST be attributable per [Trust Escalation Safety](./trust-escalation-safety.md).
- **Notification** — trust downgrade should produce a formal notification with recourse path.

## Forbidden Behaviors (doctrine-level)

- The system MUST NOT delete the seller record as a consequence of trust downgrade.
- The system MUST NOT mutate seller balance as a consequence of trust downgrade.
- The system MUST NOT terminate active obligations (active orders, active disputes, support interactions) as a consequence of trust downgrade.
- The system MUST NOT treat `approved` as a permanent moral endorsement of the seller.
- The system MUST NOT downgrade trust without an attributable admin action (operator + reason + timestamp + audit log entry). See [Trust Escalation Safety](./trust-escalation-safety.md).
- The system MUST NOT remove historical visibility (past listings, orders, reviews) as a consequence of trust downgrade.

## Intentionally Deferred Parameters

- publish restriction severity on downgrade,
- listing freeze behavior on downgrade,
- growth restriction matrix on downgrade,
- dispute escalation matrix on downgrade,
- admin RBAC for downgrade authority.

## Related Doctrine

- [Verification Review Governance](./verification-review-governance.md) — full state machine and forward path.
- [Seller Authority Separation](./seller-authority-separation.md) — sub-gate context.
- [Trust Escalation Safety](./trust-escalation-safety.md) — accountability rules for any trust-state transition.
- [Capability Matrix](./capability-matrix.md) — Layer 4 / 5 / 6 capability impact.
