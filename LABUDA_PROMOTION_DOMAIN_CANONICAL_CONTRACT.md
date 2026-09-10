# LABUDA PROMOTION DOMAIN — CANONICAL CONTRACT

**Status:** LOCKED FOR IMPLEMENTATION DESIGN  
**Authority:** Product/Business Truth → this document  
**Scope:** Promotion domain only  
**Version:** Canonical revision — Budget + Duration + CPM Pacing  
**Date:** 2026-09-06

---

## 0. DOCUMENT AUTHORITY AND REPLACEMENT RULE

This document is the canonical Promotion domain contract for Labuda.

It supersedes previous Promotion design authority where there is any contradiction regarding promotion duration, promotion billing, CPM, budget consumption, pacing, planned finish, unused budget, and estimated impressions.

Older Promotion documents may remain as historical records only. They MUST NOT be used as implementation authority when they conflict with this document.

Implementation MUST follow this document. Existing code, schema, migrations, comments, tests, DTOs, UI, and prior implementation are not business authority.

After canonical convergence is proven, obsolete authority—including dead code, stale comments, legacy tests, duplicate DTOs, compatibility branches, and unused schema—must be removed.

---

# 1. DOMAIN PURPOSE

Labuda Promotion is a paid distribution system.

Promotion allows an eligible seller to allocate a chosen budget to increase distribution opportunities for eligible:

1. For Sale products,
2. Auctions,
3. External Promotion subjects.

Promotion is NOT a guaranteed number of impressions, guaranteed sale, guaranteed click, prepaid wall-clock entitlement, or fixed-duration advertising package.

A Promotion contract has both:

- **Budget**, and
- **Target Duration**.

Their authorities are different:

> **Qualified Impression is the billing unit.**  
> **Duration is the delivery pacing and planned completion constraint.**

A seller does not pay merely because time passes.

---

# 2. CORE MODEL

```text
Seller Funding
      ↓
Promote Balance
      ↓
Promotion Contract
      ↓
Allocation
      ↓
Paced Delivery Opportunities
      ↓
Delivery Ticket
      ↓
Qualified Impression
      ↓
Atomic Financial Consumption
      ↓
Promotion Ledger
```

The contract also contains:

```text
Budget + Target Start + Target Finish / Duration
```

The system attempts to distribute qualified delivery so that usable budget is consumed approximately near the target finish.

---

# 3. HARD FINANCIAL INVARIANTS

## 3.1 Billing unit

Only a server-validated **Qualified Impression** may create billable promotion consumption.

Elapsed time, raw page load, client callback alone, duplicate callback, expired ticket, self-delivery, or stale target delivery are not independently billable.

## 3.2 Duration is not billing

Duration MUST NOT directly deduct balance, consume allocation, or create billable cost merely because time elapsed.

The legacy model below is prohibited:

```text
purchase hours
→ activate
→ elapsed wall-clock time
→ consume entitlement
```

## 3.3 Budget authority

Promotion financial authority is based on immutable financial facts and ledger entries.

Mutable counters alone MUST NOT become the sole financial source of truth.

## 3.4 No negative balance

Consumption MUST be atomic and MUST NOT allow consumed funds to exceed allocated funds.

## 3.5 One qualified impression

One Delivery Ticket may produce at most one billable Qualified Impression.

---

# 4. BUDGET, DURATION, AND PACING

## 4.1 Seller inputs

A Promotion contract is created with:

- target type,
- target(s),
- budget,
- duration,
- optional targeting configuration where applicable.

The server derives:

- start time,
- planned finish time,
- pacing parameters,
- pricing snapshot,
- seller-facing estimated impression range.

## 4.2 Duration authority

Duration answers:

> "Over approximately what period should this promotion attempt to distribute its budget?"

It does NOT mean:

> "How long may promotion remain entitled to delivery regardless of budget or delivery?"

## 4.3 Primary pacing objective

The system SHOULD attempt to consume usable budget approximately near the planned finish.

The system MUST NOT simply consume the entire budget immediately because early traffic is unusually high.

Example:

```text
Budget: Rp30.000
Duration: 3 days
Traffic during first 10 hours: extremely high
```

The promotion must be paced so that delivery does not consume the entire budget during those first 10 hours merely because many opportunities exist.

## 4.4 Approximation, not guarantee

The platform does not guarantee exact budget exhaustion, exact impression count, or exact final-second exhaustion.

The objective is:

> **Spend usable budget as close as reasonably possible to the planned finish without violating delivery integrity or placement policy.**

---

# 5. MINIMUM DAILY BUDGET

Promotion requires a configured minimum planned daily budget.

```text
minimum_required_budget =
    configured_minimum_daily_budget × duration_days
```

Initial intended policy:

```text
Minimum planned daily budget: Rp10.000
```

| Duration | Minimum Budget |
|---|---:|
| 1 day | Rp10.000 |
| 2 days | Rp20.000 |
| 3 days | Rp30.000 |
| 7 days | Rp70.000 |

Therefore `Rp10.000 / 3 days` must be rejected under a Rp10.000/day policy.

The minimum daily budget is admin-configured.

---

# 6. PACING MODEL — V1

Labuda V1 does not need a machine-learning advertising engine.

Canonical V1:

```text
Time-based pacing envelope
+
bounded catch-up
+
traffic-aware throttling
```

## 6.1 Expected spend curve

Conceptually:

```text
expected_spend_by_now
≈
budget × elapsed_fraction
```

Implementation may use a tolerance envelope rather than exact linear equality.

## 6.2 Under-pace

When materially behind pace, valid delivery opportunity may become more active to catch up.

## 6.3 On-pace

Normal delivery selection applies.

## 6.4 Over-pace

When materially ahead of pace, ordinary delivery issuance should be throttled.

Early high traffic must not prematurely exhaust the contract.

## 6.5 Final approach

As planned finish approaches, remaining usable budget may be delivered more aggressively through valid opportunities.

The system MUST NOT bypass target eligibility, seller governance, viewer rules, geography, placement limits, organic dominance, Delivery Ticket integrity, or Qualified Impression validation.

---

# 7. UNUSED BUDGET

Unused promotion funds MUST NOT automatically become platform revenue merely because planned duration ends.

Example:

```text
Budget: Rp10.000
Actual qualified delivery stops below the estimated capacity
```

If remaining budget cannot validly be consumed before completion, it remains seller-owned Promote Balance and must be released during finalization.

> **Unconsumed budget is not forfeited merely because time elapsed.**

---

# 8. ESTIMATED IMPRESSIONS

Seller-facing UI should not expose raw CPM arithmetic as the primary UX.

Preferred presentation:

```text
Budget
Rp30.000

Duration
3 days

Estimated views
Approximately 2.700–3.300 views
```

The range is server-derived and informational.

It is NOT guaranteed inventory, guaranteed billable capacity, guaranteed minimum views, or a contractual promise.

---

# 9. PRICING SNAPSHOT

Each Promotion contract must contain an immutable pricing snapshot sufficient to explain future qualified consumption.

Admin pricing changes affect future contracts only.

Existing contracts and historical ledger facts MUST NOT be retroactively repriced.

---

# 10. PROMOTE BALANCE AND ALLOCATION

## 10.1 Promote Balance

Promote Balance is seller-owned platform usage balance.

It is:

- platform-scoped,
- non-withdrawable,
- not cash,
- used to fund promotion allocation.

## 10.2 Allocation

A Promotion Allocation reserves funds for one Promotion contract.

Allocation is NOT a second wallet.

## 10.3 Release

Unused allocation must be releasable when a Promotion is finalized, including seller stop, planned duration completion with remaining funds, or permanent finalization.

---

# 11. PROMOTION CONTRACT LIFECYCLE

```text
draft / prepared
      ↓
active
      ↔
paused
      ↓
finalizing
      ↓
finalized
```

Legacy terminal `expired` semantics representing wall-clock entitlement exhaustion are prohibited.

## 11.1 Pause

Pause stops ordinary delivery, retains allocation, does not release the seller slot, and may later resume.

## 11.2 Resume

Resume restores eligible delivery and resumes pacing without recreating the contract or resetting financial history.

## 11.3 Stop

Seller Stop is terminal:

```text
freeze ordinary ticket issuance
→ settle/invalidate outstanding delivery tickets
→ finalize qualified consumption
→ release unused allocation
→ finalize contract
```

## 11.4 Planned duration completion

At planned finish:

```text
stop ordinary delivery issuance
→ settle/invalidate outstanding tickets
→ finalize qualified consumption
→ release unused budget
→ finalize
```

Remaining budget MUST NOT be burned merely because finish time arrived.

---

# 12. INTERNAL PROMOTION

Internal Promotion subjects are:

- For Sale,
- Auction.

An Internal Promotion may maintain a rolling target queue of up to 10 configured targets.

Delivery resolves one currently effective target.

Unavailable targets are skipped without duplicating product lifecycle authority inside Promotion.

---

# 13. FOR SALE AUTHORITY

For Sale promotion eligibility must defer to canonical For Sale authority.

For Sale must stop receiving promotion delivery immediately when it loses purchase availability.

Promotion must not wait for payment completion, order completion, or delivery completion.

No new billable Qualified Impression may be created after authoritative ineligibility is established.

---

# 14. AUCTION AUTHORITY

Auction promotion eligibility must defer to canonical Auction authority.

Auction end authority is server-controlled `end_at`.

Promotion must not independently invent auction completion state.

---

# 15. EXTERNAL PROMOTION

External Promotion is a separate promotion kind.

Initial subjects are intentionally minimal:

1. Event
2. Business

Examples may include competition events, hobby events, relevant businesses, products, and services.

External Promotion infrastructure may be prepared before production without requiring immediate activation.

---

# 16. SELLER CONTRACT LIMITS

Canonical concurrency:

```text
Maximum one non-finalized Internal Promotion
+
Maximum one non-finalized External Promotion
```

Paused contracts still count.

A seller cannot bypass limits by pausing a contract.

The invariant should be enforced at storage/concurrency level, not only through application pre-checks.

---

# 17. EFFECTIVE DELIVERY ELIGIBILITY

A delivery opportunity is valid only when required effective conditions hold, including:

- contract is not finalized,
- allocation remains fundable,
- seller governance is eligible,
- target is canonically eligible,
- viewer is eligible,
- geographic targeting matches where configured,
- placement policy permits delivery,
- pacing permits issuance,
- integrity gates pass.

Promotion must not duplicate Commerce lifecycle authority.

---

# 18. DELIVERY TICKET

A Delivery Ticket is server authority for one potential promotion delivery.

It binds relevant delivery context including:

- Promotion contract,
- selected target,
- viewer,
- placement,
- issue time,
- expiry,
- consumption/finalization state.

One ticket may result in at most one billable Qualified Impression.

Tickets are not financial consumption by themselves.

---

# 19. QUALIFIED IMPRESSION

Before billing, the server validates:

- ticket exists,
- ticket belongs to the intended contract and target,
- ticket is not expired,
- ticket is not consumed,
- target remains eligible,
- viewer is not prohibited,
- viewer is not prohibited self-delivery,
- qualification conditions are satisfied,
- allocation remains sufficient.

Only then may financial consumption occur.

---

# 20. ATOMIC CONSUMPTION AND IDEMPOTENCY

The system must guarantee:

```text
1 ticket
→ at most 1 billable impression
```

It must prevent double billing, negative allocation, duplicate callback consumption, and concurrent duplicate qualification.

Database and transaction authority are mandatory.

---

# 21. VIEWER SELF-DELIVERY

A seller must not receive billable promotion delivery for their own promotion where canonical ownership identity matches the viewer.

Server-side enforcement is mandatory.

---

# 22. GEOGRAPHIC TARGETING

Minimum targeting granularity:

```text
City / Regency
```

A Promotion may target an arbitrary allowed set.

Promotion does not own canonical viewer location.

---

# 23. DISCOVERY AND SELECTION

Promotion selection should be randomized, fair, observable, and resistant to obvious starvation.

V1 random selection is acceptable when extended for effective eligibility, pacing, geography, viewer restrictions, and contract type.

---

# 24. PLACEMENT

## 24.1 Content surfaces

Promoted For Sale and Auction cards reuse canonical commerce cards with promotion disclosure such as `Dipromosikan`.

A content promotion row may display up to two promoted commerce cards because canonical commerce presentation is a two-column/grid pattern.

The two items may come from different sellers and different target types.

If only one eligible promotion exists, one may be shown.

Organic content remains dominant.

## 24.2 External Promotion

External Promotion uses a dedicated simple presentation.

One External Promotion item is displayed per external promotion insertion.

## 24.3 Commerce surfaces

Promotion remains subordinate to organic inventory.

A promoted item may occupy one commerce grid position, but must not dominate the catalog.

---

# 25. ANALYTICS

Analytics is projection, not financial authority.

Possible aggregates include:

- impressions,
- clicks,
- CTR,
- surface distribution,
- pacing observations,
- delivery anomalies.

Financial authority remains ledger-backed qualified delivery facts.

No causal sales attribution is required for V1.

---

# 26. ADMIN AUTHORITY

Admin-configured policy includes at minimum:

- CPM/pricing configuration,
- minimum daily budget,
- future delivery policy parameters,
- platform-level promotion enablement.

Pricing changes apply to future contracts only.

Platform disable/enforcement should gate delivery without destroying historical contract or financial facts.

---

# 27. SAFETY AND OPERABILITY

Promotion delegates target truth to canonical domain authority.

Reversible conditions may pause effective delivery.

Permanent conditions may remove targets or finalize contracts according to canonical handling.

Promotion must not duplicate product or auction lifecycle state.

---

# 28. EXPLICITLY PROHIBITED LEGACY MODEL

The following is prohibited:

```text
Promotion Package
→ purchased duration hours
→ validity window
→ activated_at
→ paused duration arithmetic
→ consumed_duration_hours
→ wall-clock expiry
→ duration exhausted
```

Legacy concepts that must not remain as canonical financial authority include:

- `total_duration_hours`,
- `validity_window_hours`,
- `consumed_duration_hours`,
- wall-clock promotion entitlement,
- duration purchase packages,
- duration exhaustion billing,
- time-based mutable consumption counters,
- terminal `expired` meaning purchased time ran out.

Duration may exist only as planned delivery/pacing boundary.

---

# 29. IMPLEMENTATION CONSEQUENCES

Preserve:

- canonical target operability delegation,
- seller governance checks,
- reversible/permanent reason classification,
- discovery filtering,
- organic-first injectors,
- promoted disclosure,
- external subject review foundation,
- analytics as projection,
- safety sweep concepts,
- server DB-time pattern.

Rewrite:

- financial core,
- package/ownership duration accounting,
- allocation authority,
- Delivery Ticket authority,
- Qualified Impression authority,
- pacing authority,
- target queue contract model,
- geographic matching,
- seller contract concurrency enforcement.

Purge:

- duration purchase authority,
- duration consumption arithmetic,
- duration expiration worker,
- wall-clock entitlement state,
- legacy duration DTOs,
- duration-based admin UI,
- tests encoding obsolete duration accounting,
- stale comments and dead code.

---

# 30. PRODUCTION ACTIVATION POLICY

Promotion infrastructure may be implemented before Labuda production launch.

Promotion delivery does not need to be activated immediately when the marketplace initially has a small user base.

The domain should be implemented cleanly, tested, disabled-safe, and operationally prepared.

Initial production may keep Promotion delivery disabled.

---

# 31. CANONICAL IMPLEMENTATION ORDER

1. Promote Balance and ledger foundation.
2. Promotion contract and allocation model.
3. Pricing snapshot.
4. Target queue model.
5. Seller concurrency constraints.
6. Delivery Ticket issuance.
7. Qualified Impression validation and atomic consumption.
8. Budget + Duration pacing authority.
9. Minimum daily budget enforcement.
10. Seller-facing estimated impression range.
11. Geographic gating.
12. Discovery/injector convergence.
13. Pause/resume/stop/finalization settlement.
14. Analytics projection convergence.
15. Safety worker convergence.
16. Disabled-by-default production gate.
17. Full legacy duration authority purge.

No dual authority may remain after convergence.

---

# 32. FINAL CANONICAL SUMMARY

```text
Seller chooses:
    Budget
    +
    Duration

System creates:
    Promotion Contract
    +
    Allocation
    +
    Pacing Plan

Delivery creates:
    Delivery Ticket

Server validates:
    Qualified Impression

Qualified Impression:
    consumes financial allocation atomically

Duration:
    controls pacing and planned completion

Budget:
    should be consumed as close as reasonably possible
    to planned finish

High early traffic:
    must not prematurely exhaust the budget

Low traffic:
    may produce under-delivery

Remaining valid funds:
    return to seller-owned Promote Balance

Time alone:
    never creates a billable charge
```

> **Promotion is billed by qualified delivery, paced by seller-selected duration, and finalized without forfeiting unused funds.**

---

# 33. LOCK

This contract is LOCKED for implementation design.

Future changes to billing unit, Promote Balance authority, duration semantics, pacing objective, unused fund handling, seller contract limits, or target eligibility authority require an explicit business decision before implementation changes are made.
