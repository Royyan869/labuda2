# Doctrine — Verification Review Governance

> **Status:** CANONICAL v1
> **Scope:** Verification (KYC) submit + review, Seller-facing UX of verification status, Admin tooling for review queue.

## Canonical Wording

> *"Verification review is a governed trust process, not a black box queue."*

## Core Principle

The verification review is a **governed trust process**. It is not a silent queue. The seller MUST never be left in an unbounded limbo without horizon. The reviewer is accountable; the lifecycle is visible; the retry path exists.

## Mandatory Properties

A canonical verification review process MUST satisfy every property below:

- **Lifecycle visibility** — the seller can see the current canonical state of their submission at any time.
- **Status semantics explicit** — each state has a defined meaning communicated to the seller in plain wording.
- **Retry path exists** — after `rejected` or `needs_resubmission`, the seller MAY resubmit through the canonical retry path.
- **Moderation accountability** — every reviewer decision records operator id, timestamp, and reason; recorded for audit.
- **Reason mandatory on negative outcome** — `rejected`, `needs_resubmission`, and any [Revocable Trust Model](./revocable-trust-model.md) downgrade MUST include a reason. The reason is shown to the seller as a recourse path and stored in the audit log.

## Canonical Lifecycle States

- `not_submitted` — initial seller record; documents not yet submitted.
- `pending_review` — submission queued for admin review.
- `needs_resubmission` — admin requested corrections / additional documents; seller MAY retry.
- `approved` — review approved; payout authority opened. **Not** terminal forever (see [Revocable Trust Model](./revocable-trust-model.md)).
- `rejected` — review rejected; seller MAY retry.
- `suspended` — admin paused trust authority temporarily; reversible.
- `revoked` — trust authority withdrawn after investigation; seller existence persists.
- `under_investigation` — admin actively investigating; payout held; seller existence persists.

## Canonical State Machine

```
not_submitted ──(submit)──┐
 ▼
 pending_review ──(approve)──▶ approved
 ▲ │
 │ ├──(admin: suspend)─────────▶ suspended
 │ ├──(admin: under_invest.)──▶ under_investigation
 │ └──(admin: revoke)─────────▶ revoked
 │
 ├──(reject)──▶ rejected ──(seller resubmit)──┐
 │ │
 └──(needs_resubmission)──▶ needs_resubmission ──(seller resubmit)──┘

under_investigation ──(admin clear)──▶ approved
under_investigation ──(admin escalate)─▶ suspended / revoked / needs_resubmission
suspended ──(admin lift)─────────────▶ approved
```

The downgrade transitions (from `approved` to `suspended` / `under_investigation` / `revoked`) are governed by [Revocable Trust Model](./revocable-trust-model.md). The accountability rules for every transition are governed by [Trust Escalation Safety](./trust-escalation-safety.md).

## Cross-Domain Impacts

- **Notification** — formal notification at every state change. Outcome-negative transitions MUST include reason as recourse path.
- **Audit / Governance** — every transition (forward, retry, downgrade, lift) is mandatorily auditable.
- **Payout** — `approved` opens the payout sub-gate; downgraded states close it.
- **Moderation / Governance** — admin RBAC for review vs trust downgrade vs investigation is a Governance-domain concern.

## Forbidden Behaviors (doctrine-level)

- The system MUST NOT leave a submission in an unbounded silent state without a visible horizon.
- The system MUST NOT accept a `rejected` / `needs_resubmission` / downgrade transition without a reason.
- The system MUST NOT bypass the state machine — every transition must be valid per the diagram above.
- The system MUST NOT show the seller a status that is not backed by a real backend transition (no client-side optimism).
- The system MUST NOT block a retry from `rejected` or `needs_resubmission`. *(Exact retry cooldown is intentionally not locked.)*
- The system MUST NOT accept a new submission while status is `pending_review` or `approved`. New submissions originate only from `not_submitted` / `rejected` / `needs_resubmission`.
- Documentation MUST NOT describe the review as an SLA-bound queue. SLA numbers are intentionally not canonical.

## Intentionally Deferred Parameters

- SLA duration for `pending_review`,
- retry cooldown duration after `rejected` / `needs_resubmission`,
- escalation cadence.

## Related Doctrine

- [Revocable Trust Model](./revocable-trust-model.md) — downgrade transitions and survival rules.
- [Trust Escalation Safety](./trust-escalation-safety.md) — accountability for every transition.
- [Seller Authority Separation](./seller-authority-separation.md) — `approved` opens payout sub-gate, not selling sub-gate.
- [Capability Matrix](./capability-matrix.md) — Layer 4 / 5 / 6 capability map.
