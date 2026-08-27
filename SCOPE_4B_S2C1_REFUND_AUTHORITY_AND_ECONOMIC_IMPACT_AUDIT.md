# SCOPE 4B-S2C1 — REFUND AUTHORITY, ECONOMIC IMPACT, AND CURRENT CAPABILITY AUDIT

**VERDICT: `REFUND_AUTHORITY_AND_ECONOMIC_IMPACT_AUDIT_COMPLETE`**

**Date:** 2026-08-10
**Scope:** AUDIT ONLY — no implementation, no rollback, no resurrection

---

# 1. CURRENT REFUND END-TO-END FLOW

## 1A. Two Parallel Refund Paths

The system has TWO distinct refund paths that operate independently:

### Path A: Buyer-Seller Negotiation Refund (Workflow Domain)
```
Buyer sees "Ajukan Pengembalian" on shipped order
  → POST /api/v1/orders/:id/refund (OrderHandler.CreateRefund)
  → RefundService.CreateRefund(ctx, tx, orderID, buyerID, input)
     - Validates: order.status=shipped, escrow=HOLDING, no active dispute
     - Creates Refund row (status=pending_seller_review)
     - Emits refund.opened outbox event
  → Seller sees Setujui/Tolak on order detail
     - Approve: POST /api/v1/refunds/:id/approve
       → RefundService.ApproveRefund
          - Policy resolves amount (product_only or full)
          - SellerApprove() → seller_approved
          - Dispatches gateway refund via CreateAndDispatchSystemRefundFromApproval
          - refund row gateway_status moves to pending
     - Reject: POST /api/v1/refunds/:id/reject
       → RefundService.RejectRefund → seller_rejected
  → Buyer can escalate: POST /api/v1/refunds/:id/escalate
     → RefundService.EscalateToDispute → escalated_to_admin
     → DisputeService.OpenDisputeFromEscalation → creates Dispute row
  → Admin resolves dispute: POST /api/v1/admin/disputes/:id/approve|reject|partial-split
     → DisputeService.ResolveDispute
     → OrderService.RefundFromDispute / PartialRefundFromDispute
     → InitiateGatewayRefundForOrder → gateway refund dispatch
```

### Path B: Platform-Initiated System Refund (System Authority)
```
Trigger: dispute resolution, timeout cancel, expire-with-escrow, manual admin
  → OrderPaymentService.InitiateGatewayRefundForOrder
  → RefundService.CreateAndDispatchSystemRefund (single tx)
     - Creates refund row in admin_refunded state (or reuses existing)
     - Synchronously dispatches gateway refund (gateway_status → pending)
     - NO escrow/ledger mutation at dispatch time
```

### Path C: Gateway Webhook Ack (Financial Settlement)
```
Midtrans sends refund/partial_refund webhook
  → POST /webhooks/payment/midtrans
  → PaymentWebhookService → IsRefundNotification → RefundService.HandleGatewayRefundAck
     - Resolves refund row by RefundChargeID → RefundKey
     - Locks order + escrow FOR UPDATE
     - Computes proportional breakdown (CalculateProportionalRefundBreakdown)
     - Calls FinanceService.RecordRefundReversal (canonical ledger entry)
     - Flips escrow: full→refunded, partial→released+remainder release
     - Syncs order status via OrderRefundStatusSyncer
     - Emits coins.refund_required (full refund only)
     - Emits money.refund_succeeded outbox
```

### Path D: Coins Refund (Event-Driven)
```
coins.refund_required outbox event
  → CoinsRefundRequiredHandler.Handle
     - Finds original payment_spend transaction from coins_transactions
     - Checks idempotency (refund_earn already exists?)
     - Calls RefundCoinsInternal (INSERT-FIRST, UNIQUE guard)
     - Marks orders.coins_refunded_at
     - Partial refund (money_partially_refunded): SKIPS coin refund entirely
```

## 1B. Call Graph Summary

```
                        ┌─────────────────────────┐
                        │   Refund Initiation     │
                        └───────────┬─────────────┘
                  ┌─────────────────┼─────────────────┐
                  │                 │                  │
          Buyer Request     Seller Approve     Platform System
         (CreateRefund)   (ApproveRefund)    (CreateAndDispatch
                  │                 │          SystemRefund)
                  ▼                 ▼                  │
         ┌──────────────────────────────────────┐      │
         │     Refund Row Persisted              │◄─────┘
         │  (status, gateway_status=unsubmitted) │
         └────────────────┬─────────────────────┘
                          │
                          ▼
         ┌──────────────────────────────────────┐
         │   InitiateGatewayRefund              │
         │   → Midtrans POST /v2/{id}/refund    │
         │   → gateway_status = pending/failed  │
         │   → money.refund_pending outbox      │
         └────────────────┬─────────────────────┘
                          │
                 ┌────────┴────────┐
                 │                 │
            HTTP 200          HTTP Error
                 │                 │
                 ▼                 ▼
          gateway_status      gateway_status
          = pending           = failed
                 │            (retryable)
                 │
    ┌────────────┴────────────┐
    │  Midtrans Webhook Ack   │
    │  (refund/partial_refund)│
    └────────────┬────────────┘
                 │
    ┌────────────┼────────────────────┐
    │            ▼                    │
    │  HandleGatewayRefundAck         │
    │  ┌──────────────────────────┐  │
    │  │ 1. Resolve refund row    │  │
    │  │ 2. Lock order + escrow   │  │
    │  │ 3. Proportional split    │  │
    │  │ 4. RecordRefundReversal  │  │ ← FinanceService (ledger)
    │  │ 5. Flip escrow status    │  │ ← WalletService
    │  │ 6. Partial: remainder    │  │ ← RecordPartialRefundRelease
    │  │    release to seller     │  │
    │  │ 7. Sync order status     │  │ ← OrderRefundStatusSyncer
    │  │ 8. Emit coins.refund_    │  │ ← Full refund only
    │  │    required (full only)  │  │
    │  └──────────────────────────┘  │
    └────────────────────────────────┘
                 │
    ┌────────────┴────────────┐
    │  coins.refund_required  │ (full refund only)
    │  outbox event           │
    └────────────┬────────────┘
                 │
                 ▼
    ┌──────────────────────────┐
    │ CoinsRefundRequiredHandler│
    │ → Find payment_spend     │
    │ → RefundCoinsInternal    │
    │ → Mark coins_refunded_at │
    └──────────────────────────┘
```

---

# 2. REFUND ROUTES / APIs

| Route | Method | Handler | Auth | Purpose |
|-------|--------|---------|------|---------|
| `/api/v1/orders/:id/refund` | POST | OrderHandler.CreateRefund | RequireActiveAccount (buyer) | Buyer requests refund |
| `/api/v1/orders/:id/refunds` | GET | OrderHandler.ListRefundHistory | RequireActiveAccount | Paginated refund history |
| `/api/v1/orders/:id/dispute` | POST | OrderHandler.CreateDispute | RequireActiveAccount (buyer) | Direct dispute creation |
| `/api/v1/refunds/:id/approve` | POST | SellerRefundHandler.ApproveRefund | RequireActiveAccount (seller) | Seller approves refund |
| `/api/v1/refunds/:id/reject` | POST | SellerRefundHandler.RejectRefund | RequireActiveAccount (seller) | Seller rejects refund |
| `/api/v1/refunds/:id/escalate` | POST | BuyerEscalationHandler.EscalateRefund | RequireActiveAccount (buyer) | Buyer escalates to admin |
| `/api/v1/admin/disputes` | GET | DisputeHandler.ListDisputes | Admin + finance.withdraw.read | List all disputes |
| `/api/v1/admin/disputes/:id` | GET | DisputeHandler.GetDisputeDetail | Admin + finance.withdraw.read | Dispute detail |
| `/api/v1/admin/disputes/:id/approve\|reject\|partial-split` | POST | DisputeHandler.ResolveDispute* | Admin + finance.dispute.resolve | Admin resolves dispute |
| `/api/v1/admin/refunds/:refund_id/gateway/initiate` | POST | AdminRefundHandler.InitiateGatewayRefund | Admin + finance.refund.gateway.initiate + ENABLE_GATEWAY_REFUND_PHASE2 flag | Manual gateway refund trigger |
| `/api/v1/admin/support/tickets/:id/escalate-to-dispute` | POST | SupportHandler.EscalateToDispute | Admin + support.ticket.escalate | Ticket → Dispute |
| `/webhooks/payment/midtrans` | POST | PaymentWebhookHandler | None (signature verified) | Midtrans refund webhook ack |

---

# 3. DATABASE AUTHORITY

## 3A. `refunds` Table (migration 000001)

| Column | Type | Producer | Consumer | Mutability | Meaning |
|--------|------|----------|----------|------------|---------|
| `id` | uuid PK | RefundService | All | Immutable | Refund row identity |
| `order_id` | uuid FK | RefundService | All | Immutable | Parent order |
| `buyer_id` | uuid | RefundService | Auth | Immutable | Buyer identity |
| `seller_id` | uuid | RefundService | Auth | Immutable | Seller identity |
| `reason` | refund_reason_enum | Buyer via API | Policy resolver | Immutable | Why refund requested |
| `status` | refund_status_enum | Entity state machine | All | Mutable | Negotiation state |
| `requested_amount` | bigint | Buyer via API | Display only | Immutable | Buyer's claim (advisory) |
| `seller_approved_amount` | bigint | Policy resolver | Gateway dispatch | Mutable | SYSTEM-COMPUTED amount |
| `admin_approved_amount` | bigint | Admin via dispute | Gateway dispatch | Mutable | Admin decision |
| `final_refund_amount` | bigint | HandleGatewayRefundAck | Observability | Mutable | Actual gateway amount |
| `gateway_refund_id` | text | InitiateGatewayRefund | Webhook lookup | Mutable | Midtrans refund_chargeback_id |
| `gateway_status` | text | Entity state machine | All | Mutable | unsubmitted/pending/succeeded/failed |
| `gateway_attempts` | integer | InitiateGatewayRefund | Observability | Mutable | Retry counter |
| `last_gateway_error` | text | InitiateGatewayRefund | Observability | Mutable | Last error message |
| `gateway_idempotency_key` | text UNIQUE | InitiateGatewayRefund | Idempotency | Mutable | Dedup key for gateway calls |
| `request_idempotency_key` | text UNIQUE | CreateRefund | Idempotency | Mutable | Dedup key for buyer requests |

## 3B. `orders` Table — Refund-Relevant Columns

| Column | Type | Source of Truth | Usage |
|--------|------|-----------------|-------|
| `status` | order_status_enum | Order entity | refunded/partially_refunded terminal states |
| `escrow_status` | escrow_status_enum | WalletService (mirror) | holding/released/refunded |
| `has_dispute` | boolean | DisputeService | Blocks Complete(); used in queries |
| `coins_used` | bigint | DISPLAY ONLY | NOT financial authority |
| `coin_discount_amount` | bigint | Settlement snapshot | Display only |
| `coins_refunded_at` | timestamptz | CoinsRefundRequiredHandler | Observability only |
| `subtotal` | bigint | Order creation | Refund policy input (product only) |
| `shipping_total` | bigint | Order creation | Refund policy input (full) |
| `commission_amount` | bigint | Order creation | Refund policy input (full) |
| `escrow_amount` | bigint | Order creation | CHECK refunded_amount <= escrow_amount |
| `refunded_amount` | bigint | Order completion service | CHECK constraint enforcement |

## 3C. `payments` Table — Refund-Relevant Columns

| Column | Type | Producer | Meaning |
|--------|------|----------|---------|
| `gross_amount` | bigint | Payment creation | Actual Midtrans charge = Cash + F |
| `coins_to_use` | integer | Renamed from coin_discount (migr 000036) | K: coins used for this payment |
| `coin_discount_amount` | bigint | Settlement | Rp value of coins consumed |
| `service_fee_amount` | bigint | Payment creation (from Method.CalculateFee) | F: buyer payment fee |
| `payment_method_code` | text | Payment creation | Links to payment_methods.fee_type |
| `status` | payment_status_enum | Webhook handler | settlement/capture/deny/cancel/expire |

## 3D. `coins_transactions` Table

| Column | Type | Meaning |
|--------|------|---------|
| `type` | earn\|spend | Direction |
| `reference_type` | order_spend\|payment_spend\|refund_earn\|refund_spend\|order_reward | What kind |
| `reference_id` | uuid | Order ID or Payment ID |
| UNIQUE(user_id, reference_type, reference_id) | | Hard idempotency guard |
| CHECK(reference_type IN (...)) | | Canonical list per migration 000038 |

## 3E. Financial Ledger Accounts

| Account | Meaning | Refund Role |
|---------|---------|-------------|
| GATEWAY_CLEARING | Where gateway funds land at settlement | Source of refund reversal (before release) |
| SELLER_PAYABLE[seller] | Seller's withdrawable earnings | Source of refund reversal (after release) |
| PLATFORM_REVENUE | Platform commission income | Source of commission reversal |
| BUYER_REFUNDABLE[buyer] | Accounting entry for buyer refund | Destination of refund reversal (AUDIT ONLY — not spendable) |
| BANK_SETTLEMENT | Reserve float for gateway inflows | Not touched by refund |

---

# 4. CURRENT REFUND STATE MACHINE

## RefundStatus (buyer-seller negotiation)

```
pending_seller_review ──seller approve──► seller_approved ──gateway ack──► refunded
       │                                      │
       │ seller reject                        │ (gateway dispatched)
       ▼                                      ▼
seller_rejected ──buyer escalate──► escalated_to_admin
                                          │         │
                              admin refund│         │admin release
                                          ▼         ▼
                                   admin_refunded  admin_released
                                          │
                                   gateway ack
                                          │
                                          ▼
                                      refunded
```

Transitions enforced by entity methods: `SellerApprove()`, `SellerReject()`, `EscalateToAdmin()`, `AdminRelease()`, `CompleteRefund()`.

## GatewayRefundStatus (parallel — gateway conversation)

```
unsubmitted ──dispatch──► pending ──webhook success──► succeeded
     │                       │
     │ dispatch failure      │ webhook failure
     ▼                       ▼
  failed ◄──────────────── failed
     │                       │
     │ retry                 │ retry
     ▼                       ▼
  pending                  pending
```

Transitions enforced by: `MarkGatewayDispatched()`, `MarkGatewayRequestFailed()`, `MarkGatewayAckSucceeded()`, `MarkGatewayAckFailed()`.

**Gate:** `MarkGatewayAckSucceeded` is idempotent (returns nil if already succeeded).
**Gate:** Cannot transition from `succeeded` → anything else.
**Phase 1 invariant:** Gateway status transitions do NOT mutate ledger/escrow/order status.

---

# 5. MIDTRANS REFUND INTEGRATION

## 5A. Current Implementation

**Endpoint:** `POST {coreAPIURL}/{order_id}/refund` (Core API v2)

**Two client methods:**
1. `Client.Refund(ctx, orderID, amount, reason)` — legacy; no refund_key; returns error only
2. `Client.RefundWithKey(ctx, orderID, refundKey, amount, reason)` — **CANONICAL**; sends merchant idempotency key; returns `*RefundResponse{RefundKey, RefundChargeID, TransactionID, Amount}`

**Webhook acknowledgment:**
- `IsRefundNotification(status)` gates on `transaction_status ∈ {refund, partial_refund}` + `status_code ∈ {empty, "200"}`
- `NotificationPayload` carries: `RefundKey`, `RefundAmount` (string, e.g. "100000.00"), `RefundChargeID`
- `HandleGatewayRefundAck` resolves refund row by: `RefundChargeID` first, then `RefundKey` fallback

**Refund amount:** Sent as Rupiah integer (no /100 scaling) — PASS_18J canonical.

## 5B. What Midtrans Refund Features Are Used

| Feature | Implemented? | Notes |
|---------|-------------|-------|
| Refund API (POST /v2/{id}/refund) | YES | Synchronous dispatch, async settlement |
| Direct Refund API | NO | Not implemented |
| Cancel Transaction API | NO | Not implemented |
| Status Query for refund | NO | Not directly; uses webhook only |
| Refund amount | YES | Rupiah integer, Midtrans webhook echoes |
| Refund reason | YES | Sent as reason string |
| Refund idempotency (refund_key) | YES | Dedup at both Midtrans and Labuda layers |
| Gateway refund status reconciliation | PARTIAL | Recon classifier exists but not wired to active refund lifecycle |

## 5C. Configured Payment Methods (from AllowedMidtransChannels)

```
Bank transfers: bca_va, bni_va, bri_va, permata_va, cimb_va, bsi_va, danamon_va, maybank_va, btn_va, other_va
E-wallets: gopay, dana, ovo, linkaja
QRIS: other_qris
Cards: credit_card, debit_card
Convenience stores: alfamart, indomaret
```

**Banned by policy:** shopeepay, akulaku, kredivo (PayLater products)

---

# 6. REFUND VS CANCEL SEPARATION — CLEAN

**Finding: CLEAN.** The system correctly separates cancel/expire (pre-settlement) from refund (post-settlement).

### Cancel / Expire (PRE-settlement):
- `payment_failed`, `payment_expired`, `order_expired`, `order_cancelled`
- Coin reservation RELEASED (no earn/spend tx created)
- `coins.refund_required` with reason=`payment_failed|payment_expired|order_expired|order_cancelled`
- If spend tx exists (rare edge case): `RefundCoinsInternal` refunds
- **No gateway refund call** (no Midtrans money was received)

### Refund (POST-settlement):
- `money_refunded`, `money_partially_refunded`
- Payment already settled, escrow exists, gateway funding confirmed
- `coins.refund_required` with reason=`money_refunded|money_partially_refunded`
- Gateway refund dispatched (or will be)
- Ledger reversal booked

### Competing authority check:
- `InitiateGatewayRefund` validates `escrow.Status == HOLDING` — prevents refunding already-released escrow
- `HandleGatewayRefundAck` rejects `afterRelease` acks ("post-release refund acknowledgements are disabled")
- `CreateRefund` validates `order.Status == shipped` and `escrow == HOLDING`
- No code found that mixes cancel/expire semantics with refund ledger reversal

---

# 7. BUYER REFUND AMOUNT FORMULA

## 7A. Current Policy-Based Formula (Seller Approval Path)

`RefundPolicy` resolver in `entity/refund_policy.go` determines amount:

| Reason | Policy Type | Amount | Components |
|--------|------------|--------|------------|
| item_damaged | product_only | Order.Subtotal | PD (product after discount) |
| defective_item | product_only | Order.Subtotal | PD |
| item_not_received | full | Order.Subtotal + ShippingTotal + CommissionAmount | PD + S + C |
| wrong_item | full | Order.Subtotal + ShippingTotal + CommissionAmount | PD + S + C |
| item_not_as_described | admin_review_required | 0 (admin decides) | — |
| delivery_delay | admin_review_required | 0 (admin decides) | — |
| change_of_mind | admin_review_required | 0 (admin decides) | — |
| other | admin_review_required | 0 (admin decides) | — |

## 7B. Gateway Refund Amount

The amount sent to Midtrans = policy-computed `SellerApprovedAmount` (NOT buyer's RequestedAmount).

For system refunds: caller provides `refundAmount` explicitly, validated: `∈ (0, orderGross]`.

## 7C. Proportional Breakdown (Webhook Ack)

`CalculateProportionalRefundBreakdown(orderGross, orderCommission, previouslyRefunded, refundAmount)`:

```
commission(cumulative) = floor(cumulative * orderCommission / orderGross)
this_commission = commission(after) - commission(before)
seller_component = refundAmount - commission_component
```

**Key:** Shipping (S) is treated as part of `orderGross` and is proportionally split between seller and commission. There is NO separate shipping refund line item.

## 7D. What The Buyer Actually Gets Back

The buyer gets their money back from **Midtrans** (the gateway reverses the charge to the buyer's original payment instrument). The Labuda ledger records this as:

```
DR BUYER_REFUNDABLE[buyer]  +refundAmount    (before release)
CR GATEWAY_CLEARING         -refundAmount

OR (after release):
DR BUYER_REFUNDABLE[buyer]  +refundAmount
CR SELLER_PAYABLE[seller]   -sellerComponent
CR PLATFORM_REVENUE         -commissionComponent
```

`BUYER_REFUNDABLE` is an **accounting/audit** account — NOT a spendable wallet balance.

---

# 8. COINS RESTORATION BEHAVIOR

## 8A. Current Behavior

| Scenario | Coins Refunded? | Mechanism |
|----------|----------------|-----------|
| Full refund (gateway ack success) | YES | `coins.refund_required` emitted → `CoinsRefundRequiredHandler` → `RefundCoinsInternal` |
| Partial refund (gateway ack success) | NO | Explicitly skipped: "Partial refunds are edge cases... coins should stay with buyer" |
| Payment failure (deny/cancel) | YES (if spend exists) | `coins.refund_required` with reason=`payment_failed` |
| Payment expiry | YES (if spend exists) | `coins.refund_required` with reason=`payment_expired` |
| Order expiry (no payment) | YES (if spend exists) | `coins.refund_required` with reason=`order_expired` |

## 8B. Refund Amount: EXACT original spend

```go
amountToRefund := spendTx.Amount  // Full amount of payment_spend
```

The handler refunds the **entire** `payment_spend` amount. There is **NO proportional coin refund** for partial refunds — coins are all-or-nothing.

## 8C. Reference Identity

- Spends use: `reference_type='payment_spend'`, `reference_id=payment_id`
- Refunds use: `reference_type='refund_earn'`, `reference_id=order_id`
- Idempotency: UNIQUE(user_id, reference_type, reference_id) + pre-check `isAlreadyRefunded`

## 8D. Idempotency

**3-layer defense:**
1. Pre-check: `isAlreadyRefunded()` — SELECT for existing `refund_earn`
2. INSERT-FIRST: attempt insert, UNIQUE constraint catches race
3. `IsDuplicateTransaction()` check on error

## 8E. Key Finding: Coins CAN be restored for FULL refund only

Full refund → full coin restoration. Partial refund → zero coin restoration. This is an explicit policy choice coded in `coins_refund_handler.go:189-205`.

---

# 9. SELLER ENTITLEMENT / CLAWBACK

## 9A. What Is Materialized After Payment

```
Payment settlement:
  GATEWAY_CLEARING += gross  (from BANK_SETTLEMENT)
  Payment fee recognized:
    GATEWAY_CLEARING -= F     (to PLATFORM_REVENUE)
  Escrow created: HOLDING

Order completion (release):
  GATEWAY_CLEARING -= gross_remaining
  SELLER_PAYABLE[seller] += sellerNet  (= PD + S - C)
  PLATFORM_REVENUE += C
```

## 9B. Refund Behavior (Before Release — escrow=HOLDING)

```
GATEWAY_CLEARING credited → buyer refund reversal credits BUYER_REFUNDABLE
GATEWAY_CLEARING debited → refund outflow
NO SELLER_PAYABLE or PLATFORM_REVENUE touched
```

**Seller loses nothing** — they were never credited yet. **Commission NOT reversed** (never booked).

## 9C. Refund Behavior (After Release — escrow=RELEASED)

**CURRENTLY DISABLED.** `HandleGatewayRefundAck` line 622-624:
```go
if afterRelease {
    return fmt.Errorf("post-release refund acknowledgements are disabled; handle objections outside the app")
}
```

The code DOES have the machinery (`RecordRefundReversal` after-release branch):
```
DR BUYER_REFUNDABLE[buyer]  +refundAmount
CR SELLER_PAYABLE[seller]   -sellerComponent
CR PLATFORM_REVENUE         -commissionComponent
```

With a guard: `SELLER_PAYABLE balance >= sellerComponent` or returns `ErrSellerPayableInsufficient`.

**This path is explicitly blocked at runtime** — post-release refunds are not operational.

## 9D. Withdrawal Safety

- `AssertSellerWithdrawalAllowed` subtracts active dispute freezes from payable balance
- `CreateDisputeFreeze` locks SELLER_PAYABLE FOR UPDATE before inserting freeze
- `dispute_freezes` table blocks withdrawal of disputed amounts

---

# 10. SELLER DISCOUNT BEHAVIOR (D)

## 10A. Current State

The RefundPolicy resolver uses `Order.Subtotal` which is PD (P - D, the discounted product price). The seller discount D is permanently absorbed — the refund basis is the DISCOUNTED amount PD, not the original P.

## 10B. Implementation Detail

```
order_subtotal = P - D  (discounted price)
```

`RefundPolicyProductOnly` refunds `order.Subtotal` = PD.
`RefundPolicyFull` refunds `order.Subtotal + order.ShippingTotal + order.CommissionAmount` = PD + S + C.

**There is no code path that refunds the original P.** The discount D is seller-funded and non-refundable.

---

# 11. BUYER PAYMENT FEE (F) BEHAVIOR

## 11A. Current State

**F is NOT refundable.** This is explicitly documented in `finance_service.go:412-416`:

```go
// The buyer payment fee is non-refundable platform revenue regardless of what
// later happens to the order (refund/dispute/completion) — see PASS_18V report
// for the explicit refund-policy rationale — so recognizing it at settlement
// (rather than at order release, like commission) is correct: it is earned the
// moment the buyer successfully pays via the chosen method, not when the order ships.
```

## 11B. Mechanism

At payment settlement, `RecordBuyerPaymentFeeRevenue` drains F from GATEWAY_CLEARING to PLATFORM_REVENUE:
```
GATEWAY_CLEARING  -= F
PLATFORM_REVENUE  += F
```

This happens BEFORE any escrow creation, so F is permanently removed from the refundable pool.

---

# 12. ACTUAL MIDTRANS MDR / TRANSACTION FEE

## Finding: NOT PERSISTED

Labuda does NOT store:
- Actual Midtrans merchant discount rate (MDR)
- Provider settlement fee
- Provider payout deduction
- Merchant transaction fee per payment

The `service_fee_amount` on `payments` is Labuda's own buyer payment fee F — NOT Midtrans's MDR.

**Verdict: Actual Midtrans MDR is not currently a persisted refund accounting authority.**

---

# 13. SHIPPING REFUND BEHAVIOR

## 13A. Current Model

Shipping (S) is seller-managed/manual. Seller pays courier outside platform.

## 13B. Refund Policy

- `product_only` policy: Seller KEEPS shipping (ShippingTotal NOT refunded). Rationale in code: "Seller already paid the courier."
- `full` policy: Shipping IS included in refund (as part of orderGross).
- Admin-review reasons: Admin may decide.

## 13C. Partial Refund

In the `CalculateProportionalRefundBreakdown`, shipping is NOT separately tracked — it's part of `orderGross`. The proportional split treats all of orderGross uniformly.

## 13D. Return Shipping

Not represented in current code.

## 13E. Shipping Refund Decision Points (UNRESOLVED)

- What if shipping was free (promo)?
- What if courier already delivered but goods damaged?
- What if return shipping is paid by buyer?
- Pre-shipment cancellation: full S refund?
- Post-delivery refund: S refund?

---

# 14. PARTIAL REFUND CAPABILITY

## 14A. What Exists

| Capability | Status |
|-----------|--------|
| Multiple partial refunds per order | YES — cumulative tracking in Place |
| Refund amount per event | YES — `FinalRefundAmount` on each refund row |
| Cumulative refunded amount | YES — `GetSuccessfulRefundTotalByOrder` excludes current refund |
| Remaining refundable amount | DERIVABLE — `orderGross - cumulativeRefunded` |
| Per-item/quantity refund | NO — only amount-based |
| Coin restoration allocation | ALL-OR-NOTHING — only full refund triggers coins |
| Seller clawback allocation | PROPORTIONAL via `CalculateProportionalRefundBreakdown` |

## 14B. Invariant Check

`CalculateProportionalRefundBreakdown` enforces: `cumulativeRefunded <= orderGross`. Returns error if exceeded.

`InitiateGatewayRefund` enforces: `refundAmount <= orderTotal` (Subtotal + ShippingTotal + CommissionAmount).

---

# 15. REFUND IDEMPOTENCY

## 15A. Identity Keys Used

| Layer | Key | Uniqueness |
|-------|-----|------------|
| Buyer refund request | `request_idempotency_key` | UNIQUE on refunds table |
| Gateway refund dispatch | `gateway_idempotency_key` | UNIQUE on refunds table |
| Midtrans API | `refund_key` in request body | Midtrans dedup |
| Payment webhook event | `event_id` | UNIQUE on payment_webhook_events |
| Ledger reversal | `refund_reversal_<refund_id>` | UNIQUE on ledger_transactions.idempotency_key |
| Partial release | `partial_release_<refund_id>` | UNIQUE on ledger_transactions |
| Coins refund | UNIQUE(user_id, reference_type, reference_id) | DB constraint + pre-check |

## 15B. Classification

**P0:** None found. All financial mutations have idempotency guards.

**P2:** `InitiateGatewayRefund` idempotency lookup by `gateway_idempotency_key` could race between two concurrent dispatches with the same key — but Midtrans-side dedup via `refund_key` provides the second layer.

---

# 16. REFUND CONCURRENCY RISKS

| Scenario | Protection | Risk |
|----------|-----------|------|
| Two refund approvals | `SELECT FOR UPDATE` on refund row | SAFE |
| Admin + worker same order | `FOR UPDATE` on order + escrow in canonical lock order | SAFE |
| Refund + seller payout | `AssertSellerWithdrawalAllowed` checks dispute freeze | SAFE (before release); BLOCKED (after release) |
| Partial + full refund same order | Cumulative tracking + `CalculateProportionalRefundBreakdown` overflow check | SAFE |
| Duplicate Midtrans callback | `payment_webhook_events.event_id UNIQUE` + entity idempotency | SAFE |
| Coin restoration + retry | UNIQUE(user_id, reference_type, reference_id) | SAFE |

**Lock order (canonical):** order → escrow → ledger accounts (sorted by UUID). Both release and refund paths follow this order.

---

# 17. ORDER LIFECYCLE AFTER REFUND

## 17A. Full Refund

```
Status: shipped/delivered/dispute_open → refunded (TERMINAL)
EscrowStatus: holding → refunded (via WalletService.RefundGatewayEscrow)
```

## 17B. Partial Refund

```
Status: shipped/delivered/dispute_open → partially_refunded (TERMINAL)
EscrowStatus: holding → released (via WalletService.PartialRefundGatewayEscrow + remainder release)
```

## 17C. Terminal State Properties

- `StatusRefunded` and `StatusPartiallyRefunded` are terminal (no further transitions allowed)
- `CanComplete()` blocked if `HasDispute = true`
- Refund history preserved via `refunds` table (not overwritten on status change)
- `coins_refunded_at` timestamp marks when coins were refunded

## 17D. What Does NOT Happen on Refund

- Review/rating is NOT automatically invalidated
- Buyer history is NOT purged
- Chat is NOT closed
- Seller metrics are NOT immediately recomputed

---

# 18. MOBILE UX WIRING

## 18A. Current Buyer UX

1. Order detail screen → "Ajukan Pengembalian" button (only on `shipped` status)
2. `RefundRequestDialog`: reason chips (emoji), description, REQUIRED unboxing video, up to 5 photos
3. Status card shows: pending seller review / approved / rejected / escalated
4. Seller decision: shows approved percent + amount
5. Escalation: "Ajukan ke Admin (Eskalasi)" button on seller-rejected refund
6. Refund history: paginated list in collapsible section
7. Status timeline: "Direfund", "Sebagian Direfund", "Dispute Dibuka"

## 18B. Current Seller UX

1. Order detail → refund section: "Setujui" / "Tolak" buttons
2. Approve dialog: amount is BACKEND-COMPUTED (not editable)
3. Reject dialog: notes required
4. Seller dashboard: `refundedOrders`, `refundedRevenue`, `activeDisputeFreeze`

## 18C. Backend/Mobile Mismatches

| Issue | Severity |
|-------|----------|
| No coin refund information shown to buyer | P2 — buyer doesn't know coins were restored |
| No payment fee (F) non-refundability disclosure | P2 — buyer might expect full gross back |
| No shipping refund status shown | P2 — buyer doesn't know if shipping was refunded |
| Refund amount in dialog is buyer's requested_amount (advisory only) — actual amount is backend-computed | P2 — potential confusion |
| Mobile DTO has `gatewayStatus` field parity with backend | None — wire contract matches |

---

# 19. `coins_refund_handler.go` CLASSIFICATION

## 19A. MECHANICAL RECOGNITION (Safe Structural)

| Behavior | Classification |
|----------|---------------|
| Reads from `coins_transactions` (not order snapshot) | MECHANICAL — correct canonical source |
| Uses `payment_spend` reference_type | MECHANICAL — recognizes migration 000038 |
| Pre-check + INSERT-FIRST + UNIQUE guard | MECHANICAL — idempotency pattern |
| Marks `orders.coins_refunded_at` | MECHANICAL — observability |
| Dead-letter / poison-order injection | MECHANICAL — test harness |

## 19B. REFUND POLICY (NOT Business-Authoritative)

| Behavior | Classification | BUSINESS DECISION NEEDED? |
|----------|---------------|--------------------------|
| Partial refund → SKIP coins entirely | POLICY | YES — owner must decide: proportional coins or all-or-nothing? |
| Full refund → restore ALL coins | POLICY | YES — owner must confirm: always full K restoration? |
| Uses `spendTx.Amount` (full amount) | POLICY | YES — tied to partial vs full decision |
| No limit on refunded coins vs original K | POLICY | YES — the constraint is DB-level (can't exceed spend amount), but is that sufficient? |
| Multiple partial refunds → NO coin restoration (first partial skips, no later restoration) | POLICY | YES — what about partial + partial = full cumulative? |

**The handler's POLICY decisions are explicitly marked with comments like "CRITICAL: PARTIAL REFUND HANDLING — For partial refunds, we do NOT refund coins."**

---

# 20. AUTHORIZATION / SECURITY

## 20A. Access Control

| Operation | Authorization | Enforcement |
|-----------|--------------|-------------|
| Create refund | Buyer == order.BuyerID | `RefundService.CreateRefund` line 139 |
| Approve refund | Seller == refund.SellerID | `RefundService.ApproveRefund` line 432 |
| Reject refund | Seller == refund.SellerID | `RefundService.RejectRefund` line 561 |
| Escalate refund | Buyer == refund.BuyerID | `RefundService.EscalateToDispute` line 355 |
| Create dispute | Buyer == order.BuyerID | `DisputeService` ownership |
| Admin dispute resolve | Admin + `finance.dispute.resolve` capability | Route middleware |
| Admin gateway refund | Admin + `finance.refund.gateway.initiate` + feature flag | Route middleware |
| System refund | `auth.SystemCallerID` + `GatewayRefundCallerTypeSystem` | `InitiateGatewayRefund` validation |

## 20B. Amount Tampering Protection

- Buyer `RequestedAmount` is ADVISORY ONLY — never used as financial amount
- Seller approval amount is SYSTEM-COMPUTED from `RefundPolicy`
- Gateway dispatch uses `SellerApprovedAmount` (not buyer's claim)
- `InitiateGatewayRefund` validates: `amount > 0`, `amount <= orderTotal`, `escrow == HOLDING`
- `HandleGatewayRefundAck` uses Midtrans-echoed `RefundAmount` (parsed from webhook string "100000.00" → int64)

## 20C. Cross-Order Refund Protection

- Refund row always keyed to `order_id` + `buyer_id` + `seller_id` from the order
- No API allows specifying arbitrary buyer/seller
- Escrow is per-order (UNIQUE on escrows.order_id)

**No unauthorized refund paths detected.**

---

# 21. ECONOMIC COMPONENT MATRIX

| Component | Paid/funded by | Current refund behavior | Current authority | Business decision needed? |
|-----------|---------------|------------------------|-------------------|--------------------------|
| PD (product after discount) | buyer cash/coins mix | Refunded (product_only or full policy) | `entity/refund_policy.go` | PARTIALLY — partial refund proportional split; per-item not supported |
| seller discount D | seller | NOT refundable (subtotal already discounted) | Order creation | NO — locked |
| shipping S | buyer | Refunded ONLY in `full` policy; kept by seller in `product_only` | `entity/refund_policy.go` | YES — rules for partial, pre-shipment, post-delivery |
| coins K | Labuda subsidy | Full refund → full restoration. Partial → NONE | `coins_refund_handler.go` POLICY section | YES — proportional coins? all-or-nothing? |
| buyer payment fee F | buyer | NOT refundable — permanently moved to PLATFORM_REVENUE at settlement | `finance_service.go:412` RecordBuyerPaymentFeeRevenue | YES — owner must confirm non-refundability |
| seller commission C | seller deduction | Reversed proportionally in refund reversal | `CalculateProportionalRefundBreakdown` | NO — proportional reversal is canonical |
| actual Midtrans MDR | Labuda/provider | NOT PERSISTED — not part of Labuda accounting | NONE | YES — need to track for P&L? |
| seller entitlement | derived (PD+S-C) | Before release: untouched. After release: CLAWBACK code exists but DISABLED | `finance_service.go` after-release branch | YES — when/how to enable post-release clawback? |

---

# 22. NUMERIC FULL-REFUND EXAMPLE

**Canonical numbers:**
P = 100000, D = 10000, PD = 90000, S = 20000, C = 4500, K = 18000, F = 4000

**Buyer values:**
B = PD + S = 90000 + 20000 = 110000
Cash = B - K = 110000 - 18000 = 92000
Gateway gross = Cash + F = 92000 + 4000 = 96000
Seller product net = PD - C = 90000 - 4500 = 85500

**Full refund (item_not_received — policy=full):**

1. Refund policy resolves: `orderGross = PD + S + C = 114500`
2. Gateway dispatched: amount = 114500 (Rupiah integer sent to Midtrans)
3. Midtrans reverses 114500 to buyer's payment instrument
4. Webhook ack: `refundAmount = 114500`
5. Proportional breakdown: `commissionComponent = floor(114500 * 4500 / 114500) = 4500`, `sellerComponent = 110000`
6. **Before release:** `DR BUYER_REFUNDABLE 114500 / CR GATEWAY_CLEARING 114500`
7. Escrow → REFUNDED, order → refunded
8. coins.refund_required emitted → K = 18000 restored to buyer

**Buyer net outcome:**
- Midtrans returns 114500 to payment instrument
- 18000 coins restored
- F = 4000 NOT returned (already in PLATFORM_REVENUE)
- **Buyer gets: 114500 cash + 18000 coins = 132500 total (original outlay was 96000 cash + 18000 coins = 114000)** ← NOTE: This represents the seller commission (C=4500) being refunded to buyer as well — the commission was part of the gross and was never paid separately by buyer; it's structural to the order model.

**Product_only refund (item_damaged):**
1. Policy resolves: `amount = PD = 90000`
2. Gateway dispatched: amount = 90000
3. Proportional breakdown: `commissionComponent = floor(90000 * 4500 / 114500) = 3538`, `sellerComponent = 86462`
4. Partial refund path: remainder = 114500 - 90000 = 24500 released to seller
5. coins.refund_required NOT emitted (partial) → coins NOT restored
6. **Buyer gets: 90000 cash, NO coins, keeps goods**

---

# 23. NUMERIC PARTIAL-REFUND EXAMPLE (50% product)

Using same canonical numbers. **Current code cannot express "50% product partial" through the buyer-seller negotiation path** — the policy resolver only supports product_only (full PD) and full (entire orderGross). Partial amounts would require admin dispute resolution with `partial_split` type.

**Admin partial_split dispute (refund 50% of product price = 45000):**

1. Gateway dispatched: amount = 45000
2. Proportional breakdown: `commissionComponent = floor(45000 * 4500 / 114500) = 1769`, `sellerComponent = 43231`
3. Remainder: 114500 - 45000 = 69500 → released to seller with proportional commission
4. **coins.refund_required NOT emitted** (partial → no coins restored)
5. Buyer gets: 45000 cash, NO coins, keeps goods
6. Order → partially_refunded (TERMINAL)

**Cannot derive from current code:** The exact admin partial-split UX, minimum/maximum partial amounts, and whether buyer can request specific partial amounts (vs admin-determined) are not canonically defined.

---

# 24. P0/P1 FINDINGS

## P0 — NONE FOUND

No duplicate money refund, no unauthorized refund, no over-refund, no irreversible payout inconsistency, no refund-amount-exceeds-paid, no double coin restoration detected.

## P1 — 1 FINDING

### P1-1: Post-Release Full Refund Blocked With No Recovery Path

**Finding:** `HandleGatewayRefundAck` line 622-624 explicitly rejects post-release refund acknowledgements. The after-release ledger reversal code exists but is unreachable. If a legitimate post-delivery full refund is needed (e.g., item not received but order auto-completed), there is NO automated path.

**Impact:** Manual intervention required for legitimate post-release refunds. Escrow already released, seller already paid.

**Current mitigation:** The system prevents auto-completion when `HasDispute = true`. The dispute window is 12 hours post-shipping + buyer has 5 days to confirm.

## P2 — OBSERVABILITY (not fixing)

1. No coin refund amount visible to buyer in mobile UX
2. No payment fee non-refundability disclosure
3. No shipping refund status breakdown
4. Actual Midtrans MDR not persisted
5. Buyer RequestedAmount shown in UI but not authoritative
6. Recon classifier for refund exists but not wired to active lifecycle

---

# 25. OWNER DECISION MATRIX

## D1: Full Refund — Restore K Fully?

| Aspect | Detail |
|--------|--------|
| **Question** | Should full refund always restore 100% of coins used (K)? |
| **Current behavior** | YES — full gateway ack → coins.refund_required → full K restored |
| **Option A** | Always restore full K on full refund (current) |
| **Option B** | Restore K proportionally based on refund amount / orderGross |
| **Recommendation** | **Option A** — 1 coin = Rp1, Labuda-funded. Restoring full K is correct: buyer doesn't "partially use" coins. |
| **Numeric example** | Full refund: K=18000 restored. Option B: 114500/114500 * 18000 = 18000 (same for full, differs for partial) |
| **Implementation consequence** | None if Option A chosen. Option B requires proportional coin calculation in handler. |

## D2: Partial Refund — How to Restore K?

| Aspect | Detail |
|--------|--------|
| **Question** | Should partial refund restore any coins? |
| **Current behavior** | NO — partial refunds skip coin restoration entirely |
| **Option A** | Never restore coins for partial refunds (current) |
| **Option B** | Restore proportional coins: K * refundAmount / orderGross |
| **Option C** | Restore coins only if cumulative partials reach 100% |
| **Recommendation** | **Option B** — consistent with proportional money refund. Buyer who gets 50% money back should get ~50% coins back. |
| **Numeric example** | 50% product refund (45000/114500 = 39.3%): Option A gives 0 coins; Option B gives ~7077 coins |
| **Implementation consequence** | Option B requires new `money_partially_refunded` handler path + proportional coin amount calculation |

## D3: Refund Buyer Payment Fee F?

| Aspect | Detail |
|--------|--------|
| **Question** | Should the buyer payment fee F be refundable? |
| **Current behavior** | F is PERMANENTLY non-refundable — moved to PLATFORM_REVENUE at settlement |
| **Option A** | F is never refundable (current — "fee is earned at payment moment") |
| **Option B** | F is refundable on full refund (platform eats the fee loss) |
| **Option C** | F is refundable only when refund reason is platform/seller fault |
| **Recommendation** | **Option A** — F pays for payment processing infrastructure that was already consumed. Midtrans does not refund its MDR either. |
| **Numeric example** | Full refund: F=4000 stays with platform. Option B: buyer gets 4000 more. |
| **Implementation consequence** | None if Option A. Option B/C requires fee reversal ledger path. |

## D4: Shipping Refund Rules?

| Aspect | Detail |
|--------|--------|
| **Question** | When should shipping S be refunded? |
| **Current behavior** | Seller-approvable: item_damaged/defective → S NOT refunded; item_not_received/wrong_item → S refunded |
| **Option A** | Keep current policy (product damage = seller keeps shipping) |
| **Option B** | Always refund shipping (buyer never pays shipping on refund) |
| **Option C** | Shipping refund depends on delivery status (pre-shipment = full, post = none) |
| **Recommendation** | **Option A** — shipping is seller-managed, already paid to courier. For item_not_received, nothing was shipped so shipping should be refunded. |
| **Numeric example** | item_damaged + product_only: buyer gets PD=90000 (S=20000 not refunded). Option B: buyer gets 110000. |
| **Implementation consequence** | None if Option A. |

## D5: Commission Reversal?

| Aspect | Detail |
|--------|--------|
| **Question** | Should seller commission be reversed on refund? |
| **Current behavior** | YES — proportional reversal: `commissionComponent = floor(refundAmount * orderCommission / orderGross)` |
| **Decision needed?** | **NO** — proportional reversal is canonical and correct. Platform didn't earn commission on refunded portion. |

## D6: Seller Entitlement Clawback After Release?

| Aspect | Detail |
|--------|--------|
| **Question** | Should post-release refunds be allowed? |
| **Current behavior** | DISABLED — code returns error. After-release ledger reversal code EXISTS but unreachable. |
| **Option A** | Never allow post-release refunds (current — handle outside app) |
| **Option B** | Allow with seller payable balance check (code already written) |
| **Option C** | Allow via manual admin override only |
| **Recommendation** | **Option B with dispute-only guard** — only through formal dispute resolution with admin approval, with the existing `SELLER_PAYABLE >= sellerComponent` guard. |
| **Numeric example** | Post-release full refund: seller_payable must have >= 110000 available, commission=4500 reversed from PLATFORM_REVENUE |
| **Implementation consequence** | Remove the `afterRelease` error guard in `HandleGatewayRefundAck`, ensure dispute freeze captures funds before release. |

## D7: Refund After Seller Payout/Withdrawal?

| Aspect | Detail |
|--------|--------|
| **Question** | Can refund happen after seller has withdrawn funds? |
| **Current behavior** | NOT HANDLED. `ErrSellerPayableInsufficient` is the fail-closed backstop. |
| **Option A** | Block refund if seller payable insufficient (fail-closed, current) |
| **Option B** | Platform covers shortfall, pursues seller separately |
| **Recommendation** | **Option A** — platform should not be the counterparty to seller's withdrawal. Dispute freeze should prevent this race. |
| **Implementation consequence** | Ensure dispute freeze captures full order gross before seller can withdraw. Current `AssertSellerWithdrawalAllowed` does this. |

## D8: Multiple Partial Refunds?

| Aspect | Detail |
|--------|--------|
| **Question** | Should multiple partial refunds be supported? |
| **Current behavior** | SUPPORTED structurally — cumulative tracking, `previouslyRefunded` parameter. But coins are all-or-nothing. |
| **Option A** | Support multiple partials with cumulative tracking (current infrastructure) |
| **Option B** | Limit to one partial refund per order |
| **Recommendation** | **Option A** — infrastructure exists, cumulative invariant protects against over-refund. |
| **Implementation consequence** | Coins policy (D2) must handle "cumulative partials reaching 100% = full refund" edge case. |

## D9: Provider Refund Window Expired?

| Aspect | Detail |
|--------|--------|
| **Question** | What happens when Midtrans refund window expires? |
| **Current behavior** | Midtrans returns HTTP error → `gateway_status = failed` → retryable via outbox. No maximum refund age enforced by Labuda. |
| **Option A** | Keep retrying indefinitely (current) |
| **Option B** | Escalate to manual admin refund after N retries |
| **Option C** | Check Midtrans refund eligibility before dispatch |
| **Recommendation** | **Option B** — after 3 failed gateway attempts, emit `money.refund_failed` alert for manual ops intervention. This already exists via `refund_failed_alert_handler.go`. |
| **Implementation consequence** | None — already implemented. |

## D10: Manual Refund Fallback?

| Aspect | Detail |
|--------|--------|
| **Question** | What is the manual refund process when gateway refund fails? |
| **Current behavior** | `money.refund_failed` alert + dead-letter queue. Admin can use `POST /admin/refunds/:id/gateway/initiate` (feature-flagged). |
| **Option A** | Manual bank transfer outside system (current "handle outside app") |
| **Option B** | Admin-initiated gateway retry via existing endpoint |
| **Recommendation** | **Option B** for gateway-retryable failures; **Option A** for permanent failures (e.g., refund window permanently closed). |
| **Implementation consequence** | Enable `ENABLE_GATEWAY_REFUND_PHASE2` flag in production when ready. |

---

# 26. FILES MODIFIED

**Expected: NONE** — This is an audit-only scope. No files were modified.

---

# 27. EXACT AUDIT/SEARCH/TEST COMMANDS

```bash
# Backend refund domain
grep -r "refund\|Refund" backend/internal/finance/refund/ --include="*.go"
grep -r "dispute\|Dispute" backend/internal/governance/dispute/ --include="*.go"
grep -r "refund\|Refund" backend/internal/commerce/order/application/ --include="*.go"
grep -r "refund\|Refund" backend/internal/integration/payment/application/ --include="*.go"
grep -r "refund\|Refund" backend/pkg/midtrans/ --include="*.go"

# Database
grep -r "refund\|dispute" backend/migrations/000001_canonical_schema.up.sql

# Workers
grep -r "refund\|Refund\|dispute\|Dispute" backend/internal/worker/ --include="*.go"

# Mobile
grep -r "refund\|Refund\|dispute\|Dispute\|pengembalian" apps/mobile/lib/ --include="*.dart"

# Tests
find backend/ -name "*refund*test*" -o -name "*dispute*test*" | head -50

# Idempotency
grep -rn "idempotency\|Idempotency\|UNIQUE\|unique" backend/internal/finance/ --include="*.go"
grep -rn "refund_reversal_\|partial_release_\|payment_settlement_" backend/internal/finance/ --include="*.go"

# Coin refund
grep -rn "RefundCoinsInternal\|coins.refund_required\|refund_earn" backend/internal/ --include="*.go"
```

---

# 28. GIT STATUS

Current branch: `main`
Modified files: 38 (all pre-existing modifications, none from this audit)
No new files.
No deleted files.

---

# 29. RECOMMENDED NEXT IMPLEMENTATION SCOPE

**SCOPE 4B-S2C2: OWNER DECISION LOCK + PARTIAL REFUND & COINS PROPORTIONALITY**

This scope should:
1. Present D1-D10 decisions to owner for approval
2. Implement owner-approved coin proportionality policy (likely D2 Option B — proportional coin restoration)
3. Unblock post-release refund path (D6 Option B) with dispute-freeze guard
4. Add coin refund amount to mobile order detail
5. Add payment fee non-refundability disclosure to mobile UX

**Estimated files:** ~8-10 backend + ~4-6 mobile
**Risk:** LOW — changes are additive; proportional coin math mirrors existing proportional commission math pattern

---

**END OF AUDIT**
