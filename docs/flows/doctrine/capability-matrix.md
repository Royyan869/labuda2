# Doctrine — Capability Matrix

> **Status:** CANONICAL v1
> **Authority sources:** [Layered Identity & Trust Model](./layered-identity-trust-model.md), [Email Gating Matrix](./email-gating-matrix.md), [Seller Authority Separation](./seller-authority-separation.md), [Revocable Trust Model](./revocable-trust-model.md).
> **Scope:** every flow that asks "is the user allowed to do X?" reads this matrix as the canonical authority lookup. Flow docs MUST reference this table rather than redescribing capability gates inline.

## Purpose

A single canonical table for platform capabilities. Every entry maps a capability to:

- **Requires** — the trust layer or sub-gate that must be satisfied.
- **Revocable?** — whether the granted authority can be revoked (typically by trust state change or subscription expiry).
- **Survives downgrade?** — when the user is downgraded to Layer 6 (suspended / revoked / under_investigation), does the capability still function (typically only obligation-handling capabilities survive)?
- **Authority domain** — the doctrine doc or domain that owns the gating rule.

## Layer Reference (canonical)

These layer names are used throughout the matrix. Definitions live in [Layered Identity & Trust Model](./layered-identity-trust-model.md) and [Seller Authority Separation](./seller-authority-separation.md).

| Layer | Stage |
|-------|-------|
| **Layer 0** | Guest (no account). |
| **Layer 1** | Authenticated Account (Layer A only — no full participation). |
| **Layer 2** | Identity Complete Account (Layers A + B; email may be unverified). |
| **Layer 3** | Email Verified Account (Layers A + B + C). |
| **Layer 4** | Subscribed Seller — Unverified (Layer 3 + subscription `active`; payout NOT yet open). |
| **Layer 5** | Verified Seller (Layer 4 + verification `approved`; full participation). |
| **Layer 6** | Suspended / Revoked Trust (downgraded from Layer 5 via admin action; existence + obligations survive). |

## Capability Matrix

| Capability | Requires | Revocable? | Survives downgrade (Layer 6)? | Authority domain |
|------------|----------|------------|-------------------------------|------------------|
| Browse (home, explore, listing read, auction read) | Layer 0 | No | Yes | [Email Gating Matrix](./email-gating-matrix.md) — ALLOWED list |
| View profile of other users | Layer 0 (public profile fields) | No | Yes | [Email Gating Matrix](./email-gating-matrix.md) — ALLOWED list |
| Sign up / Sign in | Layer 0 | No (account-bound) | N/A | [Layered Identity & Trust Model](./layered-identity-trust-model.md) — Layer A |
| Full app entry as participant | Layer 2 | Yes (via Layer B regression — e.g. forced re-completion) | N/A | [Layered Identity & Trust Model](./layered-identity-trust-model.md) — Layer B |
| Follow user | Layer 2 | No | Yes | [Email Gating Matrix](./email-gating-matrix.md) — ALLOWED list |
| Like / react | Layer 2 | No | Yes | [Email Gating Matrix](./email-gating-matrix.md) — ALLOWED list |
| Shortlist / favorite | Layer 2 | No | Yes | [Email Gating Matrix](./email-gating-matrix.md) — ALLOWED list |
| Edit basic profile (display name, avatar, bio) | Layer 2 | No (subject to T&S review on flagged fields) | Yes | [Email Gating Matrix](./email-gating-matrix.md) — ALLOWED list |
| Comment | Layer 3 | Yes (via Layer C regression — e.g. email change) | N/A | [Email Gating Matrix](./email-gating-matrix.md) — BLOCKED list |
| Create post | Layer 3 | Yes | N/A | [Email Gating Matrix](./email-gating-matrix.md) — BLOCKED list |
| Chat / DM | Layer 3 | Yes | N/A | [Email Gating Matrix](./email-gating-matrix.md) — BLOCKED list |
| Negotiation | Layer 3 | Yes | N/A | [Email Gating Matrix](./email-gating-matrix.md) — BLOCKED list |
| Checkout (purchase) | Layer 3 | Yes | N/A | [Email Gating Matrix](./email-gating-matrix.md) — BLOCKED list |
| Bid auction | Layer 3 | Yes | N/A | [Email Gating Matrix](./email-gating-matrix.md) — BLOCKED list |
| Become seller (start onboarding) | Layer 3 | Yes (cannot start while Layer C regressed) | N/A | [Email Gating Matrix](./email-gating-matrix.md) — BLOCKED list |
| Submit seller verification | Layer 3 + Seller capability | Yes | N/A | [Verification Review Governance](./verification-review-governance.md) |
| Subscription payment (seller) | Layer 3 | Yes | N/A | [Email Gating Matrix](./email-gating-matrix.md) — BLOCKED list |
| Publish listing (public) | Layer 4 (subscription `active`) | Yes (subscription expiry → public listings revert to private) | Conditional (existing listings MAY be restricted; new publish blocked) | [Seller Authority Separation](./seller-authority-separation.md) — selling sub-gate |
| Create auction | Layer 4 | Yes | Conditional (new creation MAY be restricted) | [Seller Authority Separation](./seller-authority-separation.md) — selling sub-gate |
| Receive order | Layer 4 | Yes (selling sub-gate state) | Yes (active orders survive — obligation handling) | [Seller Authority Separation](./seller-authority-separation.md) — selling sub-gate |
| Promotion / boost listing (growth) | Layer 4 | Yes | No (growth restricted on downgrade) | [Revocable Trust Model](./revocable-trust-model.md) |
| Withdraw seller balance | Layer 5 (verification `approved` AND not Layer 6) | Yes (trust downgrade closes the gate) | No (blocked) | [Seller Authority Separation](./seller-authority-separation.md) — payout sub-gate |
| Payout extraction / settlement | Layer 5 | Yes | No (blocked) | [Seller Authority Separation](./seller-authority-separation.md) — payout sub-gate |
| Trust escalation actions (request re-review, etc.) | Layer 5 | Yes | No (blocked) | [Trust Escalation Safety](./trust-escalation-safety.md) |
| Active dispute participation (already opened) | Pre-existing dispute | No (lifecycle-bound) | Yes | [Revocable Trust Model](./revocable-trust-model.md) — *"Active obligations survive trust downgrade"* |
| Open new dispute (as buyer or seller party) | Layer 3 (buyer) / Layer 4+ (seller) | Yes | Yes (obligation handling) | [Revocable Trust Model](./revocable-trust-model.md) |
| Support participation (talk to support) | Layer 1+ | No | Yes | Support domain |
| Moderation restriction reception (being subject of moderation) | Any layer | N/A | N/A — applies regardless of layer | Governance domain |
| Audit traceability (admin-side lookup of user history) | Admin authority | N/A | N/A — survives every state change | [Trust Escalation Safety](./trust-escalation-safety.md) + Governance |
| Address book — purpose `shipping` | Layer 2 | No | Yes | Address Book flow |
| Address book — purpose `sender` (farm address) | Layer 4 (Seller capability) | Yes | Conditional | Address Book flow + [Seller Authority Separation](./seller-authority-separation.md) |
| Notification preferences (toggle categories) | Layer 2 | No (commerce-critical categories non-toggleable) | Yes | Preferences flow |

## Reading the Matrix — Operational Notes

- **"Requires" is the minimum layer.** A capability with "Requires Layer 3" is also available at Layers 4 and 5 unless the matrix says otherwise.
- **"Revocable" means the granted authority can be lost** without deleting the account — typically through Layer C regression (email change), subscription expiry, or trust downgrade.
- **"Survives downgrade"** specifically asks: when a Verified Seller is moved to Layer 6, does this capability still work? "Yes" is reserved for capabilities tied to obligation handling, audit, or basic existence. "Conditional" indicates the matrix is intentionally not locked (severity-dependent — see [Revocable Trust Model](./revocable-trust-model.md)).
- **"Authority domain"** points to the doctrine doc that owns the canonical gating rule. If a flow doc and the authority domain diverge, the authority domain wins.

## Forbidden Behaviors (doctrine-level)

- A flow MUST NOT redefine a capability's required layer locally. The matrix is the canonical authority lookup.
- A flow MUST NOT silently extend or shrink the BLOCKED list. See [Email Gating Matrix](./email-gating-matrix.md).
- A flow MUST NOT couple selling-sub-gate capabilities (publish, create auction, receive order) with payout-sub-gate capabilities (withdraw, payout). See [Seller Authority Separation](./seller-authority-separation.md).
- A flow MUST NOT mark a capability "survives downgrade" if it would re-open trust extraction (e.g. withdraw cannot survive Layer 6).
- The matrix MUST NOT be edited as a side effect of an operational change. Updating an entry requires citing the doctrine doc that authorizes the change, or amending that doctrine doc first.

## Intentionally Deferred Entries

The following capabilities are intentionally not in the matrix and require their own canonical decisions before being added:

- Phone-verified gating capabilities (no canonical phone verification policy).
- Two-factor authentication–gated capabilities.
- Logout-all-devices, session revocation, multi-device management.

## Related Doctrine

- [Layered Identity & Trust Model](./layered-identity-trust-model.md)
- [Email Gating Matrix](./email-gating-matrix.md)
- [Seller Authority Separation](./seller-authority-separation.md)
- [Revocable Trust Model](./revocable-trust-model.md)
- [Verification Review Governance](./verification-review-governance.md)
- [Trust Escalation Safety](./trust-escalation-safety.md)
- [Username Lifecycle](./username-lifecycle.md)
