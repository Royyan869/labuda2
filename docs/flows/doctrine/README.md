# Doctrine Index

Folder ini berisi **canonical doctrine** untuk Foundation dan Commerce domain. Doctrine docs adalah **sumber kebenaran resmi** untuk invariant, authority semantics, layered trust, revocation semantics, capability matrix, lifecycle invariants, commerce split, dan governance-sensitive rules.

## Authority Hierarchy

1. **Doctrine docs (this folder)** - canonical truth. Wins over flow docs in case of conflict.
2. **Flow docs** - operational journey docs. Reference doctrine; do not redefine it.

If a flow doc says X and a doctrine doc says Y, Y wins. The flow doc is wrong and must be aligned. If documentation says X and runtime implementation says Y, doctrine still wins - runtime is the convergence target.

## Doctrine Index

| Doctrine | What it owns |
|----------|--------------|
| [Layered Identity & Trust Model](./layered-identity-trust-model.md) | Four layers (Authentication / Identity Completion / Email Verification / Seller-Financial Trust). Account stages. Path convergence rule. |
| [Email Gating Matrix](./email-gating-matrix.md) | Canonical ALLOWED / BLOCKED lists. Enforcement pattern. Unverified account lifecycle. |
| [Username Lifecycle](./username-lifecycle.md) | Username is canonical identity, immutable after establishment. Trust continuity invariants. |
| [Seller Authority Separation](./seller-authority-separation.md) | Selling sub-gate != payout sub-gate. Pre-verification seller model. Liability invariant. |
| [Commerce Selling Doctrine](./commerce-selling-doctrine.md) | Product / FixedPriceListing / Auction split, sale-intent-first UX, lifecycle matrix, exclusivity rules, Buy Now rules. |
| [Revocable Trust Model](./revocable-trust-model.md) | `approved` is not terminal. Survive / cut lists for trust downgrade. Liability invariant. |
| [Verification Review Governance](./verification-review-governance.md) | Mandatory review properties. Canonical lifecycle states. State machine diagram. |
| [Trust Escalation Safety](./trust-escalation-safety.md) | Hidden production auto-approve forbidden. Triple guard for sandbox shortcuts. Attribution requirements. |
| [Capability Matrix](./capability-matrix.md) | Per-capability authority lookup synthesized from the doctrines above. |

## How Flow Docs Reference Doctrine

A flow doc should **summarize** doctrine in a couple of sentences and **link** to the authoritative doc, rather than copy-pasting bullets.

**Good:**

> Email verification is the gate for interaction and transaction authority. See [Email Gating Matrix](./email-gating-matrix.md) for the canonical ALLOWED / BLOCKED lists.

**Bad:**

> [Fifteen bullets repeated verbatim from the doctrine doc.]

## Adding a New Doctrine

A new doctrine doc should be created when an invariant is asserted in three or more flow docs and starts to drift in wording across them. A doctrine doc must include:

- Status header.
- Canonical wording (the quote(s) that anchor the doctrine).
- The rules in invariant form (MUST / MUST NOT, not "we should").
- Cross-Domain Impacts section.
- Forbidden Behaviors (doctrine-level - flow-level forbiddens stay in flow docs).
- Intentionally Deferred Parameters (if any).
- Related Doctrine cross-links.

## Anti-Drift Rules

- A doctrine doc MUST NOT be edited to reflect runtime drift. Runtime is the convergence target; doctrine documents the canonical target.
- A doctrine doc MUST NOT be deleted to "simplify" the system. Doctrine docs persist past their convergence; their job is invariant authority, not project management.
- A flow doc MUST NOT silently change a capability gate documented in [Capability Matrix](./capability-matrix.md). The matrix entry is updated first, then the flow.
