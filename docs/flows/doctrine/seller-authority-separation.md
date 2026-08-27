# Doctrine — Seller Authority Separation (Selling ≠ Payout)

> **Status:** CANONICAL v1
> **Scope:** Seller Capability, Verification, Subscription, Wallet, Payout, Listing, Auction.

## Canonical Wording

> *"Selling authority and payout authority are separate trust layers."*

> *"Platform may allow commercial participation before allowing financial extraction."*

> *"Blocked withdrawal does not erase seller ownership."*

## The Two Sub-Gates

Layer D (Seller / Financial Trust) is **not** a single gate. It is composed of two **independent** sub-gates that MUST NOT be coupled.

| Sub-gate | Authority | Opened by | Closed by |
|----------|-----------|-----------|-----------|
| **Selling authority** | Publish listing, create auction, receive order, commercial participation, reputation building. | Subscription `active`. | Subscription expired / cancelled. |
| **Payout authority** | Withdraw, payout extraction, financial settlement out of platform. | Verification `approved`. | Trust downgrade — see [Revocable Trust Model](./revocable-trust-model.md). |

## Independence Rules

- Subscription `active` **DOES NOT** automatically grant withdraw / payout / financial settlement authority.
- Verification `approved` **DOES NOT** automatically grant publish listing / commercial participation.
- A seller MAY hold one without the other (Layer 4: subscribed but unverified — sells, accumulates balance, cannot withdraw).
- A seller MAY hold both (Layer 5: full participation).
- A seller MAY temporarily lose one (Layer 6: trust downgraded — selling MAY restrict, payout blocks; existence and obligations survive).

## Pre-Verification Seller Model

> *"Platform may allow commercial participation before allowing financial extraction."*

- Seller MAY sell before verification is approved.
- Seller MAY receive orders.
- Seller MAY build reputation.
- Money MAY accumulate **internally** as seller liability.
- Money MAY NOT leave the platform until verification approved.

## Liability & Balance Philosophy

> *"Blocked withdrawal does not erase seller ownership."*

- Balance remains seller liability. Platform does **not** confiscate.
- Withdrawal-blocked balance stays visible to the seller.
- Payout authority remains restricted until trust requirements are satisfied.
- Consistent with gateway-funded commerce: trust gating closes the **extraction** door; it does **not** mutate the ledger.

## Cross-Domain Impacts

- **Listing / Auction** — publish and create gating reads selling authority sub-gate (subscription), **not** verification.
- **Wallet / Escrow** — escrow release on order completion follows finance rules normally; the seller's claim on released funds is preserved even when withdrawal is blocked.
- **Payout** — withdraw entry-point reads payout authority sub-gate (verification + not in Layer 6).
- **Dispute** — active dispute lifecycle uses dispute-domain rules (refund / release) regardless of trust state. Trust gating does not freeze obligations.
- **Subscription** — subscription lifecycle (expiry) controls only selling authority. Renewal-window semantics relative to withdrawal are intentionally not locked.
- **Moderation** — admin authority over the two sub-gates is itself separable; RBAC sub-roles are a Governance-domain concern.

## Forbidden Behaviors (doctrine-level)

- The system MUST NOT couple the two sub-gates. Subscription `active` MUST NOT auto-grant withdraw. Verification `approved` MUST NOT auto-grant publish.
- The system MUST NOT grant payout authority just because Become Seller completed.
- The system MUST NOT grant selling authority just because verification approved.
- The system MUST NOT mutate seller balance as a consequence of trust state changes. Ledger is the financial authority; trust gating closes the extraction door, not the ledger.
- The system MUST NOT delete or hide seller existence as a consequence of trust state changes — see [Revocable Trust Model](./revocable-trust-model.md).
- Documentation MUST NOT describe "verified seller" as a single-axis status; the two sub-gates must be named explicitly.

## Related Doctrine

- [Layered Identity & Trust Model](./layered-identity-trust-model.md) — Layer D context.
- [Revocable Trust Model](./revocable-trust-model.md) — payout sub-gate revocation.
- [Verification Review Governance](./verification-review-governance.md) — payout sub-gate forward path.
- [Capability Matrix](./capability-matrix.md) — per-capability lookup including Layer 4 / 5 / 6 transitions.
