# AUCTION WINNER SHIPPING & SETTLEMENT — REPORT CORRECTION / BUSINESS-TRUTH ADDENDUM

**Applies to:** `REPORT_AUCTION_WINNER_SHIPPING_DESIGN.md`  
**Date:** 2026-09-02  
**Purpose:** Supersede stale statements in the forensic design report after Owner decisions made during report review.

> This document is a correction layer for the agent report. It does not change the forensic findings that were correctly established. It corrects only business-truth/status statements that became stale or were already incorrect.

---

## 1. CORRECTION: TRANSACTION RESTRICTION SCOPE

The original report incorrectly leaves restriction scope as an Owner Decision.

**This is now LOCKED.**

Transaction restriction is **cross-commerce**, covering both Labuda commerce products:

### Buyer restriction

A restricted buyer cannot:
- purchase through **For Sale**;
- purchase/bid through **Auction** when the action can lead to a transaction.

### Seller restriction

A restricted seller cannot:
- sell through **For Sale**;
- sell through **Auction**.

Restriction is **not** an auction-only restriction and is **not** a full account ban. Non-commerce capabilities remain available unless another independent policy says otherwise.

The implementation must therefore have **one commerce-level restriction authority**, not separate auction-only restriction systems.

---

## 2. CORRECTION: RESTRICTION LADDER

The locked ladder is:

| Cumulative violation | Restriction |
|---:|---:|
| 1st | 7 days |
| 2nd | 15 days |
| 3rd and subsequent | 30 days |

This applies equally to:
- Buyer BNR;
- Buyer failure to complete required settlement shipping selection;
- Seller Shipping Default.

The count is:
- cumulative;
- never automatically reset;
- no trust score;
- no permanent-ban escalation.

The existing 14-day / 90-day / permanent ladder is obsolete and must be removed during implementation.

The existing automatic 180-day BNR decay is also obsolete and must be removed.

---

## 3. CORRECTION: NORMAL SHIPPING SELECTION DEADLINE

The original report incorrectly leaves the normal-shipping selection window as an Owner Decision.

**This is now LOCKED.**

When normal Shipping Setup is available:

> The auction winner has **24 hours from auction end** to select a valid Shipping Setup and resolve shipping.

If the buyer fails to complete the required normal shipping selection within that 24-hour window:

> **Buyer Transaction Violation**

The buyer receives the same cumulative restriction ladder.

This is distinct from BNR terminology: BNR specifically describes failure to pay after the buyer is able to pay. The normal-shipping-selection failure is a buyer settlement violation, but it uses the same violation/restriction authority and ladder.

---

## 4. CORRECTION: BUYER PAYMENT WINDOW

The buyer payment window is a **separate 24-hour window**.

It starts only after:

> **Shipping is RESOLVED and the buyer has a final payable amount.**

Therefore:

```text
Auction ends
    ↓
Winner confirmed
    ↓
Shipping resolution
    │
    ├── Normal Shipping Setup selected
    │
    └── Private Quote accepted
    ↓
SHIPPING RESOLVED
    ↓
Buyer Payment Window = 24 hours
    ↓
Paid → continue
Timeout → BNR
```

The old 30-minute payment-expiry behavior is divergent and must not remain as the auction-winner payment policy.

The exact technical timestamp should be the canonical `shipping_resolved_at` event/timestamp once the implementation design confirms the appropriate authority.

---

## 5. CORRECTION: SELLER SHIPPING DEADLINE

Seller responsibility is conditional.

The seller has:

> **24 hours from auction end**

to resolve shipping **when shipping cannot be resolved through normal Shipping Setup and a private quote is required**.

The seller is **not required to create a private quote when normal shipping is already available**.

Therefore the seller deadline must not punish a seller merely for not creating a quote when no quote is needed.

If private shipping is required and the seller fails to resolve shipping within the 24-hour deadline:

> **Seller Shipping Default**

Consequences:
- auction transaction fails/terminates according to the final technical state design;
- winner receives **no violation**;
- seller receives a commerce transaction violation;
- seller receives the applicable 7/15/30-day cross-commerce restriction.

---

## 6. CORRECTION: TWO DIFFERENT 24-HOUR RESPONSIBILITIES

Do not collapse these into one generic settlement deadline.

### Seller

```text
Auction End
→ Seller Shipping Resolution Deadline
→ 24 hours
```

Only relevant when private shipping resolution is required.

### Buyer — normal shipping

```text
Auction End
→ Buyer Normal Shipping Selection Deadline
→ 24 hours
```

If buyer does not select normal shipping within this window, buyer receives a transaction violation.

### Buyer — payment

```text
Shipping Resolved
→ Buyer Payment Deadline
→ 24 hours
```

If buyer does not pay within this window, buyer receives BNR.

These are different responsibilities and different points in the lifecycle.

---

## 7. CORRECTION: SELLER DEFAULT IS NOT BNR

Do not reuse `expired_bnr` merely because the current implementation uses that state for settlement timeout.

Business semantics are now explicitly different:

### Buyer

`Payment deadline exceeded → BNR`

### Seller

`Required shipping resolution deadline exceeded → Seller Shipping Default`

They may share the same violation/restriction authority, but they must not be represented as if they are the same business event.

Whether the auction terminal state should be:
- existing `cancelled`;
- existing terminal state with a separate reason;
- or a new explicit terminal state;

remains a **technical design decision** until the implementation pass proves which representation is cleanest. Do not treat `seller_default` as an already-approved schema/state decision.

---

## 8. CORRECTION: ITEM AFTER SELLER DEFAULT

The forensic report proposed that the item should be freely relistable after seller default.

That is **not yet an Owner-locked business truth**.

Do not implement this assumption.

Likewise, the report's recommendation that the previous winner receives no special benefit is a recommendation, not an Owner decision.

These remain explicitly unresolved until the Owner locks them.

---

## 9. CORRECTION: `commerce_violations` IS NOT YET LOCKED AS SCHEMA

The report recommends a unified `commerce_violations` table.

The **business truth** is locked:

> Buyer and Seller use one common transaction-violation/restriction authority and one 7/15/30-day ladder.

However:

> The exact database schema/table name (`commerce_violations`) is a technical design decision, not yet a business truth.

Before implementation, the agent must verify whether consolidating the existing `buyer_bnr_strikes` into one authority is the cleanest solution.

Do not create `commerce_violations` merely because the report recommended that name.

---

## 10. CORRECTION: CHAT

The locked chat behavior remains:

- no automatic chat creation for every auction;
- seller gets a **Give Shipping Quote** entry point only when private shipping resolution is required;
- direct seller↔winner commerce chat is created/found lazily when the seller initiates that action;
- shipping quote is delivered through the existing commerce chat mechanism.

The existence of generic lazy direct-room creation is evidence of supporting infrastructure; it is not itself the completed auction-winner feature.

---

## 11. CORRECTED OWNER-DECISION LIST

After this correction, the actual unresolved Owner decisions are:

1. **Terminal business outcome/state after Seller Shipping Default.**
2. **Whether the product may be relisted after Seller Shipping Default.**
3. **Whether the previous auction winner receives any benefit after Seller Shipping Default.**

The following are **NOT Owner Decisions anymore**:

- transaction restriction scope;
- restriction ladder;
- cumulative/no-reset behavior;
- no trust score;
- normal shipping selection deadline;
- buyer payment deadline;
- seller shipping deadline.

Those are locked.

---

## 12. CORRECTED CANONICAL FLOW

### Normal Shipping

```text
AUCTION ENDS
    ↓
WINNER CONFIRMED
    ↓
Normal Shipping Available
    ↓
Buyer selects Shipping Setup
    ↓
SHIPPING RESOLVED
    ↓
24h PAYMENT WINDOW
    ↓
    ├── PAID → transaction continues
    └── TIMEOUT → BNR → transaction restriction
```

### Private Shipping Quote

```text
AUCTION ENDS
    ↓
WINNER CONFIRMED
    ↓
Private Quote Required
    ↓
SELLER HAS 24h TO RESOLVE SHIPPING
    ↓
    ├── Seller creates quote
    │       ↓
    │   Buyer accepts
    │       ↓
    │   SHIPPING RESOLVED
    │       ↓
    │   24h PAYMENT WINDOW
    │
    └── Seller timeout
            ↓
        SELLER SHIPPING DEFAULT
            ↓
        Winner not penalized
            ↓
        Seller transaction restriction
```

---

## 13. CORRECTED IMPLEMENTATION PRINCIPLES

The implementation pass must preserve these distinctions:

1. **Shipping resolution is a prerequisite to payment.**
2. **Order/payment must not bypass unresolved shipping.**
3. **Normal shipping and private quote are two shipping-resolution paths.**
4. **Seller's 24h deadline and buyer's 24h payment deadline are not the same timer.**
5. **Buyer normal-shipping selection has its own 24h-from-auction-end requirement.**
6. **BNR means buyer failed to pay after becoming able to pay.**
7. **Seller Shipping Default means seller failed to resolve required shipping.**
8. **Both violation types share one commerce restriction authority and ladder.**
9. **Restriction affects For Sale + Auction commerce, not the whole account.**
10. **No trust score and no automatic violation decay.**
11. **No legacy BNR permanent ban.**
12. **No auction-only restriction implementation.**
13. **No compatibility layer for the old BNR model.**

---

## 14. Final Corrected Status

The forensic findings in the original report remain useful.

However, after Owner review, the report must be interpreted as:

> **DESIGN READY WITH LIMITED OWNER DECISIONS**

The remaining Owner decisions are only the three explicitly listed in Section 11.

The previously listed questions about restriction scope, normal shipping selection window, and payment deadline are closed and must not be reopened.

**This correction supersedes conflicting statements in the original report.**
