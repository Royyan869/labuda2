# Doctrine — Email Gating Matrix (HYBRID TRUST-FIRST)

> **Status:** CANONICAL v1
> **Scope:** Layer C of the [Layered Identity & Trust Model](./layered-identity-trust-model.md). Every flow that gates an action behind email verification must read this matrix.

## Canonical Wording

> *"Read freely. Interact only after trust exists. Transact only after identity is trusted."*

Email verification is the **minimum trust gate for interaction authority**. Not a cosmetic formality. Not full identity proof.

## Three Authority Tiers

| Authority | Definition | Requires email verified? |
|-----------|------------|--------------------------|
| **Browsing** | Read, view, follow, like. | No. |
| **Interaction** | Comment, post, chat, negotiate. | Yes. |
| **Transaction** | Checkout, bid, become seller, publish, withdraw, subscription payment. | Yes (baseline; further gates may apply per [Seller Authority Separation](./seller-authority-separation.md)). |

## Canonical Gating Matrix

> **Backend is the single source of truth.** Mobile MUST conform UX to backend's enforcement. Backend MUST NOT extend or shrink either list without a formal amendment to the canonical decision.

### Unverified user — ALLOWED

The following actions MUST NOT be rejected solely because email is unverified:

- Browse app (home, explore, categories).
- View listing.
- View auction (read-only — bidding is BLOCKED below).
- View other users' profiles.
- View content / posts.
- Follow user.
- Like / react.
- Shortlist / favorite (save listing for later).
- Edit basic profile (display name, avatar, bio — provided the field is not under Trust & Safety review; the precise sub-list will be locked when the username cluster propagation finishes).

### Unverified user — BLOCKED

The following actions MUST be rejected by backend when `email_verified=false`:

- Comment.
- Create post.
- Chat / DM.
- Negotiation (every bargaining path).
- Checkout (purchase).
- Bid auction.
- Become seller (seller onboarding).
- Publish listing.
- Withdraw seller balance.
- Seller subscription payment.
- All other **interaction-sensitive** actions (those producing public side effects or touching another user).
- All other **transaction-sensitive** actions (those moving funds or opening financial liability).

> **Stability rule:** the BLOCKED list is the canonical baseline. Backend MUST NOT add to it without a formal amendment. Mobile MUST NOT enforce gating for actions absent from the BLOCKED list.

## Enforcement Pattern

- **Banner persistent (mobile)** — surfaces verification reminder while letting the user roam ALLOWED surfaces. Does not block global navigation.
- **Inline gate (mobile)** — surfaces verification prompt at the point a BLOCKED action is attempted; backend has already rejected with a stable reason code (`EMAIL_VERIFICATION_REQUIRED`).
- **No full-screen blocker as default behavior.**

## Unverified Account Lifecycle (canonical)

- An unverified account MAY persist indefinitely.
- No auto-delete.
- No hard expiry.
- BLOCKED restrictions remain active for the entire lifetime of the unverified state.
- Reminder / escalation cadence MAY exist later — not yet canonicalized.

## Cross-Domain Impacts

- **Social** (post, comment, follow, chat) — gating consumers must read the BLOCKED list, not invent local rules.
- **Commerce** (listing publish, checkout, bid) — gating consumers must read the BLOCKED list.
- **Finance** (withdraw, subscription payment) — gating consumers must read the BLOCKED list. Memori `Money authority model` (gateway-funded commerce) is preserved: the matrix gates **access** to financial actions; it does not change refund / payout / escrow semantics.
- **Notification reachability** — financial-recovery, dispute, and fraud notifications depend on email verification platform-wide.

## Forbidden Behaviors (doctrine-level)

- Mobile MUST NOT block global navigation for an unverified Identity Complete Account. The full-screen blocker pattern as default is forbidden.
- Backend MUST NOT accept a BLOCKED action from an unverified account; it MUST reject with a stable reason code.
- Backend MUST NOT reject an ALLOWED action solely because the account is unverified.
- The system MUST NOT mark `email_verified_at` without a verified claim from External (Firebase Auth).
- Mobile MUST NOT cache `emailVerified=true` longer than the periodic session validation guarantees.
- The system MUST NOT extend or shrink either list without a formal amendment of.
- The system MUST NOT skip verification for an email previously used by another account (e.g. delete-recreate); verification must be performed again.

## Related Doctrine

- [Layered Identity & Trust Model](./layered-identity-trust-model.md) — Layer C context.
- [Capability Matrix](./capability-matrix.md) — per-capability lookup including Email Verification gate.
- [Seller Authority Separation](./seller-authority-separation.md) — additional gates layered on top of Layer C for seller / financial actions.
