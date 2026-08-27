# Doctrine — Layered Identity & Trust Model

> **Status:** CANONICAL v1
> **Scope:** Foundation, Social, Commerce, Finance — every flow that asserts "what the account is allowed to do" must read its authority through these layers.

## Canonical Wording

> *"Authentication may differ. Identity completion may differ. But every account must establish a canonical public identity before entering the platform as a full participant."*

## The Four Layers

User authority on Labuda is composed of **four distinct trust layers**. Each layer is established by its **own evidence** and is evaluated independently. Crossing one layer does **not** imply crossing another.

| Layer | Name | Established by | Authority unlocked |
|-------|------|----------------|--------------------|
| **A** | **Authentication** | Credential or OAuth identity validated against External (Firebase Auth + Google OAuth). Internal session token issued. | Account exists; session active. **Not** sufficient for full app entry. |
| **B** | **Identity Completion** | Canonical public identity established: `username` chosen and stored; `profile_completed=true`. | Full app entry as a participant. Browsing authority. |
| **C** | **Email Verification** | External claims `email_verified=true`; backend syncs `email_verified_at` timestamp. | Interaction authority + transaction authority baseline. See [Email Gating Matrix](./email-gating-matrix.md) for the canonical ALLOWED / BLOCKED lists. |
| **D** | **Seller / Financial Trust** | Composed of two **independent** sub-gates: subscription `active` (selling authority) and verification `approved` (payout authority). See [Seller Authority Separation](./seller-authority-separation.md). | Commercial participation (publish, sell) and financial extraction (withdraw, payout) — gated separately. |

## Account Stages (canonical names)

Each stage corresponds to the highest layer the account has cleared.

- **Guest** — pre-Layer A. No account.
- **Authenticated Account** — Layer A only. Session exists; identity not yet complete. **Not** a full participant. Locked behind the Complete Profile gate.
- **Identity Complete Account** — Layers A + B. Full app entry; browsing authority granted. Email may still be unverified.
- **Email Verified Account** — Layers A + B + C. Interaction + transaction authority baseline.
- **Seller (sub-stages)** — Layers A + B + C + D. Sub-stages are governed by [Capability Matrix](./capability-matrix.md) Layer 4 / Layer 5 / Layer 6.

## Path Convergence (sign-up symmetry)

The two sign-up paths reach Layer B differently, but **business outcome must be identical**: no full participation without `username`.

| Path | Layer A established | Layer B established |
|------|---------------------|---------------------|
| Email/password sign-up | Sign-up form | Same sign-up form (`username` is a required field). Identity Complete on account creation — **no incomplete window**. |
| Google OAuth sign-in | OAuth callback | Mandatory Complete Profile gate after OAuth callback. The window between A and B is held behind the gate. |

UX timing **may** differ. The canonical outcome **must** match.

## Cross-Domain Impacts

- **Search / Mention / Social Graph** — Layer B identity must remain stable across rename. See [Username Lifecycle](./username-lifecycle.md).
- **Moderation traceability** — every layer transition is auditable; admin lookup must work even after rename / trust downgrade.
- **Finance authority boundaries** — Layer C unlocks transaction baseline; Layer D unlocks selling and payout via separate sub-gates. Money authority semantics (gateway-funded commerce) sit on top of Layer D.
- **Notification reachability** — financial recovery, dispute notification, fraud notification depend on Layer C being satisfied platform-wide.

## Forbidden Behaviors (doctrine-level)

- A flow MUST NOT collapse two layers into one. `profile_completed=true` is not evidence of email verification; email verified is not evidence of seller verification; seller verified is not evidence of identity completion.
- A flow MUST NOT grant full app entry to an Authenticated Account that has not cleared Layer B.
- A flow MUST NOT grant interaction or transaction authority to an Identity Complete Account whose Layer C is not satisfied. The gate enforcement lives in [Email Gating Matrix](./email-gating-matrix.md).
- A flow MUST NOT use Google OAuth's `email_verified=true` claim as a substitute for Layer B. Layer C may sync from Google; Layer B never does.
- A flow MUST NOT describe the model as "verified user" without specifying which layer is being asserted.

## Related Doctrine

- [Email Gating Matrix](./email-gating-matrix.md) — Layer C enforcement detail.
- [Username Lifecycle](./username-lifecycle.md) — Layer B identity continuity.
- [Seller Authority Separation](./seller-authority-separation.md) — Layer D sub-gate separation.
- [Capability Matrix](./capability-matrix.md) — per-capability authority lookup.
