# SCOPE 4B-S2B0 — COIN REDEMPTION RESERVATION AND PAYMENT INTENT CONCURRENCY AUTHORITY AUDIT

## VERDICT

**`COIN_REDEMPTION_RESERVATION_AND_PAYMENT_INTENT_CONCURRENCY_AUDIT_COMPLETE`**

The recommended canonical technical direction is **Model R (RESERVE → CONSUME / RELEASE)** with the **payment row** as the reservation owner. A **new `coin_reservations` table** is required; the existing `coins_transactions` schema cannot safely represent reservation state. The existing `RecordCoinsUsage` / `RecordCoinsUsageTx` methods must be **PURGED** — their immediate-SPEND semantics are incompatible with the required reservation model.

The single most critical finding: **Midtrans Snap is called OUTSIDE the database transaction**, which alone makes Model S (immediate spend → restore) unacceptable. If coins are spent inside the payment-creation transaction but Midtrans fails afterward, the coins are permanently lost with no automatic recovery path (the payment expiry worker is DISABLED by default).

---

# 1. CURRENT COIN BALANCE / TRANSACTION MODEL (Part A)

## 1.1 Balance Schema

**Table:** `user_coin_balance` (`backend/migrations/000001_canonical_schema.up.sql:1704-1710`)

| Column | Type | Default |
|--------|------|---------|
| user_id | uuid | NOT NULL (PK) |
| balance | bigint | 0 NOT NULL |
| version | bigint | 1 NOT NULL |
| created_at | timestamptz | now() |
| updated_at | timestamptz | now() |

Constraints: `PRIMARY KEY (user_id)`, `CHECK (balance >= 0)`, `FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`.

**Go entity:** `backend/internal/incentive/coins/entity/user_coin_balance.go:20-26` — single `Balance int64` field. No available/reserved/spent split. `Version` is an optimistic-locking counter used only for reconciliation, not checked in UPDATE WHERE clauses.

## 1.2 Transaction Schema

**Table:** `coins_transactions` (`000001_canonical_schema.up.sql:612-620`)

| Column | Type | Notes |
|--------|------|-------|
| id | uuid | PK, gen_random_uuid() |
| user_id | uuid | NOT NULL, FK → users |
| type | text | NOT NULL, CHECK IN ('earn','spend') |
| amount | bigint | NOT NULL, CHECK > 0 |
| reference_type | text | NOT NULL, CHECK IN ('order_reward','order_spend','refund_earn','refund_spend') |
| reference_id | uuid | NULLABLE |
| created_at | timestamptz | NOT NULL, DEFAULT now() |

**There is NO `status` column, NO `idempotency_key` column, NO `expiry` column, NO `updated_at` column.**

**Unique constraint (critical):** `CREATE UNIQUE INDEX idx_coins_transactions_unique_reference ON coins_transactions (user_id, reference_type, reference_id) WHERE (reference_id IS NOT NULL)` (line 2016). This is a **partial unique index** — rows with NULL `reference_id` are unconstrained. This is the only hard idempotency guard.

**Go entity:** `backend/internal/incentive/coins/entity/coins_transaction.go:71-79` — `CoinsTransaction` struct with ID, UserID, Type, Amount, ReferenceType, ReferenceID, CreatedAt. Constructors: `NewSpendTransaction` (hardcodes `CoinReferenceOrderSpend`), `NewEarnTransaction`.

## 1.3 Concurrency Mechanism

`AtomicDeductBalance` (`backend/internal/incentive/coins/infrastructure/repository/coins_repository_impl.go:393-408`):

```sql
UPDATE user_coin_balance
SET balance = balance - $1, updated_at = NOW(), version = version + 1
WHERE user_id = $2 AND balance >= $1
```

Returns `rowsAffected` — 1 = success, 0 = insufficient funds. **No `SELECT FOR UPDATE`** — the conditional UPDATE on the single PK row is the lock. PostgreSQL row-level locking on the PK ensures serialization of concurrent UPDATEs against the same `user_id`.

## 1.4 Answers to Part A Questions

| Q | Answer |
|---|--------|
| Is there a concept of RESERVED coins? | **NO.** No reservation table, no status column on transactions, no hold mechanism. |
| Is balance split into available/reserved/spent? | **NO.** Single `balance` field. `GetActiveBalance()` derives balance from SUM(earn) - SUM(spend). |
| Can existing schema represent reservation? | **NO.** No status column, no way to distinguish reserved from spent. |
| Is there a unique key for ONE payment intent? | PARTIALLY. `(user_id, reference_type, reference_id)` partial unique index exists, but `reference_type` is limited to 4 enum values. A coin reservation would need a new reference type or a separate table. |
| Can reservation be atomically released/consumed? | **NO.** The only primitives are `AtomicDeductBalance` (immediate spend) and `AtomicAddBalance` (immediate credit). There is no "move from available to reserved" primitive. |
| Is current schema unsuitable? | **YES.** A clean new design (`coin_reservations` table) is preferable to overloading `coins_transactions` with status/expiry columns. |

---

# 2. DEAD / ZERO-CALL COIN METHODS (Part B)

## 2.1 RecordCoinsUsage

**File:** `backend/internal/commerce/order/application/order_payment_service.go:354-388`

```
RecordCoinsUsage(ctx, tx, order, buyerID, coinsToUse, orderValueForCoins)
  → SpendCoins(ctx, buyerID, coinsToUse, order.ID, orderValueForCoins)  // IMMEDIATE SPEND
  → order.ApplyCoinsSnapshot(coinsToUse)                                  // display-only
```

- Opens its **own transaction** inside `SpendCoins` (ignoring the passed `tx` parameter)
- Performs immediate `AtomicDeductBalance` + `CreateTransaction` (spend row)
- 20% max validation in `SpendCoins`; no commission safety check in this variant
- Idempotency: pre-check via `FindSpendByReference` (soft), returns success if spend exists
- **Callers: ZERO** (global grep confirmed)

## 2.2 RecordCoinsUsageTx

**File:** `backend/internal/commerce/order/application/order_payment_service.go:410-442`

```
RecordCoinsUsageTx(ctx, tx, orderID, buyerID, coinsToUse, orderValueForCoins, commissionAmount)
  → SpendCoinsTx(ctx, tx, buyerID, coinsToUse, orderID, orderValueForCoins, commissionAmount)
```

- Runs in **caller's transaction** (no internal commit)
- Same immediate spend: `AtomicDeductBalance` + `CreateTransaction`
- Adds **commission safety** validation (coins cannot reduce payment below commission)
- **Callers: ZERO** — was previously called from `finalizeOrderCreationTx`, removed in Scope 4B-S2A

## 2.3 SpendCoins / SpendCoinsTx

**File:** `backend/internal/incentive/coins/application/coins_service.go`

- `SpendCoins` (line 342): Own-tx variant. Only caller is dead `RecordCoinsUsage`.
- `SpendCoinsTx` (line 467): Caller-tx variant. Only caller is dead `RecordCoinsUsageTx`.

Both perform: validation → `FindSpendByReference` idempotency → `EnsureBalanceRow` → `AtomicDeductBalance` → `CreateTransaction` (spend row).

## 2.4 Classification

| Method | Classification | Rationale |
|--------|---------------|-----------|
| `RecordCoinsUsage` | **PURGE** | Wrong semantics (immediate spend), dead code, ignores passed tx |
| `RecordCoinsUsageTx` | **PURGE** | Wrong semantics (immediate spend), dead code |
| `SpendCoins` | **PURGE** | Only caller is dead `RecordCoinsUsage`; own-tx behavior is dangerous for composition |
| `SpendCoinsTx` | **CANONICAL REUSABLE PRIMITIVE** | Correctly implements atomic deduction in caller's tx with idempotency and commission safety. Can serve as the CONSUME step in Model R, but only when called AFTER successful settlement — not at payment-intent time. |
| `ApplyCoinsSnapshotToOrder` | **PURGE** | Dead code, only reachable through dead methods |

The immediate-SPEND semantics of `RecordCoinsUsage`/`RecordCoinsUsageTx` fundamentally conflict with the reservation model. They spend coins BEFORE Midtrans is called, creating an irrecoverable gap if Midtrans fails. **Do not retain for future scope.**

---

# 3. CURRENT PAYMENT CREATION TRANSACTION SEQUENCE (Part C)

## 3.1 Handler: `CorePaymentHandler.CreatePayment`

**File:** `backend/internal/serve rboot/dependencies.go:3118-3438`

**Exact sequence:**

```
PHASE 1 — VALIDATION (separate short transactions, not atomic with payment creation):

  1. [DB TX 1] Load order by ID                  (line 3147-3151)
     - orderRepo.GetByID (no lock)
  
  2. Ownership check: order.BuyerID == userID     (line 3162-3170)
  
  3. Payability guard:                            (line 3184-3203)
     - order.Status == pending_payment?           → 409 if not
     - time.Now() < order.PaymentExpiresAt?       → 410 if expired
  
  4. [DB TX 2] Payment reuse guard                (line 3213-3228)
     - FindExistingPaymentForOrder (status IN pending/settlement/capture)
     - If found + non-expired + has payment_url   → RETURN existing (idempotent)
  
  5. [DB TX 3] Payment method resolution          (line 3257-3261)
     - paymentMethodRepo.GetByCode (method_code)
     - method.Enabled check

PHASE 2 — AMOUNT DERIVATION (in-memory, no DB):

  6. escrowAmount = Subtotal + ShippingTotal + CommissionAmount            (line 3289)
  7. buyerPaymentFee = CalculateFee(escrowAmount, method)                  (line 3290)
  8. grossMoney = escrowAmount + buyerPaymentFee                           (line 3300)
  9. coinDiscountAmount := int64(0)   ← HARDCODED TO ZERO                 (line 3302)
  10. netAmount = grossAmount - coinDiscountAmount                         (line 3303)
  11. MidtransOrderID = "LAB-" + uuid.New()                                (line 3327)
  12. ExpiredAt = order.PaymentExpiresAt                                   (line 3328)

PHASE 3 — PAYMENT PERSISTENCE [DB TX 4 — THE CRITICAL TRANSACTION]:

  13. CreatePayment(input)                                                  (line 3350)
      - ReferenceType = "order", ReferenceID = &orderID
      - CoinDiscount = req.CoinDiscount (from client, informational)
      - CoinDiscountAmount = money.New(0)  ← ZERO
      - NetAmount = grossMoney (no discount)
  
  14. UpdatePaymentSelectionTx (order: method, fee, gross)                 (line 3363)
  
  15. Create payment_attempt (best-effort, failure ignored)                (line 3372)
  
  16. COMMIT

PHASE 4 — MIDTRANS SNAP CALL [OUTSIDE ANY DB TRANSACTION]:

  17. createMidtransTransaction(payment, order, ...)                        (line 3403)
      → buildSnapRequest(payment.GrossAmount, payment.MidtransOrderID, ...)
      → midtransClient.CreateSnapTransaction(snapReq)
      → HTTP POST to Midtrans Snap API
      → Returns redirect_url

PHASE 5 — URL PERSISTENCE [DB TX 5]:

  18. updatePaymentURL(payment.ID, paymentURL)                              (line 3413-3415)

PHASE 6 — RESPONSE:

  19. Return payment_id, status, payment_url, gross_amount,
      coin_discount, coin_discount_amount, net_amount, etc.
```

## 3.2 Critical Architectural Gap

**Midtrans is called OUTSIDE the database transaction (Phase 4).** The payment row is committed (Phase 3) BEFORE the Snap API call. This means:

- If Midtrans fails: a payment row exists with `status=pending` and no `payment_url`. The reuse guard won't return it (requires non-empty URL). A second attempt hits the partial unique index and fails.
- If Midtrans succeeds but URL update fails: the Midtrans transaction exists but the user can't access it (no URL stored). The payment row is in `pending` with no URL.
- **There is no atomicity between payment creation and Midtrans transaction creation.**

## 3.3 Coin Involvement in Current Flow

**NONE.** `coinDiscountAmount` is hardcoded to `int64(0)` at line 3302. No coin balance check, no coin deduction, no coin reservation. The client's `coin_discount` field is accepted and stored but has zero monetary effect. `netAmount` always equals `grossAmount`.

---

# 4. PAYMENT PERSISTENCE / UNIQUENESS (Part D)

## 4.1 Payments Table

**File:** `backend/migrations/000001_canonical_schema.up.sql:1201-1222`

Key columns: id, user_id, payment_number, midtrans_order_id, gross_amount, net_amount, status (enum: pending/settlement/capture/deny/cancel/expire/challenge), reference_type, reference_id, payment_url, transaction_id, payment_type, paid_at, expired_at, coin_discount (int, default 0), coin_discount_amount (bigint, default 0), price_snapshot_id, service_fee_amount, payment_method_code (added in migration 000006).

## 4.2 Uniqueness Constraints

**Partial unique index (THE critical constraint):**

```sql
CREATE UNIQUE INDEX idx_active_payment_per_order
ON payments (reference_type, reference_id)
WHERE status IN ('pending', 'capture', 'challenge')
AND reference_type = 'order';
```

This enforces: **exactly ONE active payment per order at any time.** Terminal payments (settlement, deny, cancel, expire) are excluded from the index — a new payment can be created after the previous one reaches a terminal state.

## 4.3 Payment Attempts

**Table:** `payment_attempts` (`000001_canonical_schema.up.sql:1167-1185`) — separate table with `order_id` (FK), user_id, attempt_at, checkout_started, payment_method_selected, gateway_reached, status, failure_reason, etc. Created inside the payment-creation transaction (best-effort, failure ignored). Not suitable as reservation owner — it's an analytics/observability table.

## 4.4 Order-Payment Relationship

- **No `payment_id` column on orders.** The link is logical: `payments.reference_type = 'order'` + `payments.reference_id = orders.id`.
- **One active payment per order** (enforced by partial unique index).
- **Multiple terminal payments per order possible** (after expiry/cancel of prior payment).
- Order carries `payment_method`, `service_fee_amount`, `total_payable_amount` — updated by `UpdatePaymentSelectionTx` when payment method is chosen.

## 4.5 Answer: What DB Object Should Own a Coin Reservation?

**The `payments` row** is the correct reservation owner.

Reasoning:
- **Lifecycle alignment**: A coin reservation lives exactly as long as a payment intent is active. Payment pending → coins reserved. Payment settled → coins consumed. Payment failed/expired → coins released.
- **Uniqueness**: The partial unique index guarantees at most ONE active payment per order. No two active payment intents can exist simultaneously, so no two coin reservations can contend for the same order.
- **The order is wrong**: An order can exist without a payment, and an order without coins is valid. Reservation lifetime should match payment lifetime, not order lifetime.
- **The payment_attempt is wrong**: It's best-effort only (creation failure is ignored), has no monetary fields, and is designed for analytics.
- **A dedicated coin_reservations table is needed**: The payments table has `coin_discount`/`coin_discount_amount` columns but no coin balance/reservation state. A dedicated table provides clean separation of concerns and allows the reservation to have its own status machine independent of payment status.

---

# 5. CONCURRENCY FINDINGS (Part E)

## Scenario 1 — Same User, Two Different Orders

**Setup:** Balance = 20,000 coins. Order A requests 15,000. Order B requests 15,000. Both orders created simultaneously, then both attempt payment creation with coin redemption.

**Current behavior:** No coins are spent or reserved during either order creation or payment creation. Both orders succeed. Both payments are created with `coin_discount_amount=0`. Total authorized: 0 (no coins are used). **Current primitives are safe because they do nothing.**

**With Model R (reservation at payment time):**
- Payment A tx: `AtomicDeductBalance(user, 15000)` → balance becomes 5,000 → reservation created.
- Payment B tx (concurrent): `AtomicDeductBalance(user, 15000)` on same user_id row. PostgreSQL row-level lock serializes: if A commits first, balance = 5,000 < 15,000 → 0 rows affected → `ErrInsufficientBalance`.
- **Result: SAFE.** Only one payment can reserve 15,000. The second gets insufficient balance.

**With Model S (immediate spend at payment time):** Same PostgreSQL serialization. One succeeds, one fails with insufficient balance. **Also safe at DB level, but unsafe at business level** — the "successful" payment may have Midtrans fail, leaving coins spent with no recovery.

## Scenario 2 — Same Order, Duplicate CreatePayment

**Current behavior:** First payment created. Second request: `FindExistingPaymentForOrder` returns existing pending payment. If it has a URL and is not expired, it's returned immediately (idempotent). If no URL (Midtrans failed), the code tries to create another payment → hits `idx_active_payment_per_order` partial unique index → unique violation → error.

**With coins:** If coins were reserved/spent on the first payment, the duplicate would find the existing payment and return it — coins already reserved. If the first payment has no URL (Midtrans failed), a second payment row can't be created. **The partial unique index prevents double-reservation at the DB level.**

## Scenario 3 — Retry After HTTP Timeout

**Setup:** Midtrans Snap call succeeds but mobile gets timeout. User retries.

**Current behavior:** First payment row exists with `status=pending`, no `payment_url` (URL update is a separate tx that may also fail). On retry, `FindExistingPaymentForOrder` finds the payment but it has no URL → not returned. Handler tries to create another payment → partial unique index violation → error returned to mobile. **The first payment is orphaned.**

**With coins:** Same problem. If coins were reserved on the first payment, they're stuck in reserved state. The reuse guard must be enhanced to handle the "payment exists but has no URL" case — either by calling Midtrans again for the same payment, or by atomically releasing the reservation and creating a new one.

**This is a pre-existing architectural issue independent of coins**, but coins make it a financial problem rather than just a UX problem.

## Scenario 4 — Payment Method Change

**Current behavior:** No explicit method-change flow. To change method, user would need the first payment to reach a terminal state, then create a new payment. The partial unique index blocks a second active payment.

**With coins:** If coins are reserved on the first payment, changing method requires:
1. Cancel/expire the first payment → release coin reservation
2. Create new payment with new method → reserve coins again

This is clean if release is atomic and idempotent. **The reservation belongs to the payment, not the attempt.** Method change is semantically a new payment.

## Scenario 5 — Payment Expiry

**Current behavior:** Payment expiry worker is **DISABLED by default** (requires `DISABLE_PAYMENT_EXPIRY_WORKER=false` AND `ACK_DANGEROUS_PAYMENT_EXPIRY_WORKER=true`). When enabled: `pending → expire` on payment, then `orderService.Expire` on order. Coin refund: emits `coins.refund_required` outbox event if `order.CoinsUsed > 0` (but `CoinsUsed` is always 0).

**With coins:** The expiry worker must atomically release the coin reservation when expiring a payment. The release must be idempotent (expiry worker may retry). The existing `coins.refund_required` outbox pattern can be adapted — emit `coins.reservation_release_required` on payment expiry.

**Critical note:** The expiry worker being disabled by default is itself a risk. If coins can be reserved but never auto-released, a disabled expiry worker means coins are permanently locked. **The expiry worker MUST be enabled before coin reservation goes live.**

## Scenario 6 — Payment Deny/Cancel/Error

**Current behavior:** Webhook receives deny/cancel/expire → `FailPayment` (payment `pending → deny|cancel|expire`). No coin handling. No order transition (only the expiry worker or buyer action closes the order).

**With coins:** The webhook failure path must release the coin reservation. Since the webhook runs in a single DB transaction with `FOR UPDATE` locks, the release can happen atomically within the same tx.

## Scenario 7 — Successful Settlement

**Current behavior:** Webhook receives settlement/capture → `FinalizeOrderPayment` (settle payment, create ledger entries, create escrow, mark order paid). No coin handling. `orders.coins_used` stays 0.

**With coins:** The settlement path must **consume** the reservation (reserved → spent). This must happen inside the same webhook transaction, after the bulletproof guard confirms the payment is still pending, but before the response. The existing `SpendCoinsTx` primitive can be reused here — it provides atomic deduction, idempotency via `FindSpendByReference`, and commission safety. However, it would need to work against the reservation rather than the balance directly, or the reservation would need to have already reduced available balance.

**Idempotency under webhook replay:** The `payment_webhook_events.event_id` UNIQUE constraint prevents the entire webhook transaction from running twice. Even without it, the conditional `WHERE status='pending'` on settlement and the `FindSpendByReference` idempotency check on spend would prevent double-consumption.

---

# 6. REQUIRED TRANSACTIONAL INVARIANTS (Part F)

| # | Invariant | Current Architecture Can Satisfy? |
|---|-----------|-----------------------------------|
| 1 | available = balance - reservations | **NO.** No reservation concept exists. Requires new `coin_reservations` table or status-enabled transaction model. The `user_coin_balance.balance` would need to be reduced at reservation time (making it "available balance after reservations"), OR a derived available balance would need to subtract active reservations. |
| 2 | One coin cannot back two active payment intents | **YES**, if reservation uses `AtomicDeductBalance` at payment creation time (same user_id PK serialization). The partial unique index on payments reinforces this at the payment level. |
| 3 | K = reservedCoins × Rp1 | **YES**, by design. Store `coins_reserved` on the reservation row; use it when building the monetary snapshot. |
| 4 | Midtrans gross must never use K unless K is atomically protected | **NOT CURRENTLY.** Midtrans is called outside the DB transaction. Model R must reserve coins inside the payment-creation DB tx (Phase 3), BEFORE the Midtrans call (Phase 4). The critical invariant is: reservation committed → Midtrans call can proceed with reduced gross. If Midtrans fails, reservation must be released. |
| 5 | Webhook replay cannot consume coins twice | **YES.** `payment_webhook_events.event_id` UNIQUE constraint + `FindSpendByReference` idempotency on spend + conditional `WHERE status='pending'` updates. |
| 6 | Payment failure/expiry release cannot credit coins twice | **YES**, with idempotent release design. Use unique reference on release transaction, or conditional release (`WHERE status='reserved'`). |
| 7 | Changing payment method cannot create additional subsidy | **YES.** The partial unique index enforces one active payment per order. Old payment must reach terminal state (releasing reservation) before new payment can be created. |
| 8 | Seller entitlement independent of K | **ALREADY TRUE.** Commission is calculated on (P-D), not on (P-D-K). The `escrowAmount = Subtotal + ShippingTotal + CommissionAmount` in CreatePayment does not include K. Coins are a Labuda subsidy. |

**Current architecture cannot satisfy Invariant 4 without changes** — the Midtrans-outside-tx gap means any coin protection must be committed BEFORE the Snap call, creating a risk window if Snap fails.

---

# 7. MODEL R vs MODEL S COMPARISON (Part G)

## Model R — RESERVE → CONSUME / RELEASE

**Payment creation:**
1. Inside DB tx: validate balance ≥ CoinsToUse, validate 20% cap
2. `AtomicDeductBalance(user, K)` → balance reduced immediately
3. INSERT into `coin_reservations` (payment_id, user_id, amount, status='reserved')
4. Set `payment.coin_discount_amount = K`, `payment.net_amount = gross - K`
5. COMMIT (reservation is durable)
6. Call Midtrans with `gross_amount = escrow + fee - K`
7. If Midtrans succeeds → update payment_url (no reservation change)
8. If Midtrans fails → release reservation (compensating action)

**Settlement webhook:**
- Inside webhook tx: UPDATE coin_reservations SET status='consumed'
- Create `coins_transactions` spend row (type='spend', reference_type='order_spend', reference_id=orderID)
- Set `order.coins_used = K`, `order.coin_discount_amount = K`

**Failure/expiry:**
- Inside failure tx: UPDATE coin_reservations SET status='released'
- `AtomicAddBalance(user, K)` → balance restored
- Create `coins_transactions` earn row (type='earn', reference_type='refund_earn', reference_id=paymentID)

**Schema impact:**
- New table: `coin_reservations` (id, user_id, payment_id, order_id, amount, status [reserved/consumed/released], created_at, updated_at)
- UNIQUE index on (payment_id) WHERE status = 'reserved' (one active reservation per payment)
- UNIQUE index on (user_id, payment_id) — idempotency guard
- Modify `payments.coin_discount_amount` to accept non-zero values

**Locking:**
- Reservation creation: row lock on `user_coin_balance` via `AtomicDeductBalance` (already implemented)
- Reservation consumption: row lock on `coin_reservations` via `FOR UPDATE` + conditional status update
- Same lock ordering as existing payment settlement (ORDER → PAYMENT), add RESERVATION after PAYMENT

**Idempotency:**
- Reservation creation: UNIQUE (user_id, payment_id) on coin_reservations
- Reservation consumption: conditional `WHERE status='reserved'`
- Reservation release: conditional `WHERE status='reserved'` + `AtomicAddBalance` (idempotent — adding twice would double-credit, so release must be guarded by reservation status change)

**Crash recovery:**
- If crash after reservation commit but before Midtrans call: reservation exists, payment has no URL. Needs reconciliation — either retry Midtrans for same payment or release reservation.
- If crash after Midtrans but before URL update: reservation exists, Midtrans transaction exists, no URL stored. Payment reuse guard must be enhanced.
- If crash after URL update: everything consistent.

**Reconciliation:**
- Periodic scan for `coin_reservations.status='reserved'` WHERE `payments.status IN ('deny','cancel','expire')` → auto-release
- Periodic scan for `coin_reservations.status='reserved'` WHERE `payments.expired_at < NOW()` → auto-release (defense in depth)

## Model S — IMMEDIATE SPEND → RESTORE ON FAILURE

**Payment creation:**
1. Inside DB tx: validate balance, 20% cap
2. `SpendCoinsTx(user, K, orderID)` → immediate spend (AtomicDeductBalance + spend transaction)
3. Set `payment.coin_discount_amount = K`, `payment.net_amount = gross - K`
4. `order.ApplyCoinsSnapshot(K)` → `order.coins_used = K`
5. COMMIT
6. Call Midtrans with reduced gross

**Settlement webhook:**
- No coin action needed (already spent)

**Failure/expiry:**
- `RefundCoinsInternal(user, orderID)` → AtomicAddBalance + refund_earn transaction
- Set `order.coins_used = 0`

**Problems with Model S:**

1. **Abandoned payment gap**: If Midtrans fails after coins are spent, the payment row sits in `pending` with no URL. Coins are spent. The only recovery path is the expiry worker, which is **DISABLED by default**. Even if enabled, coins are unavailable for the entire expiry window (up to 6 hours for retail payments).

2. **Misleading transaction history**: User sees a SPEND transaction for a payment that never completed. If the user retries and succeeds, they see SPEND → REFUND → SPEND. If they give up and the payment expires, they see SPEND → REFUND with a long gap.

3. **Restore races**: The refund on failure competes with any concurrent operation on the same balance. While `AtomicAddBalance` is safe (no WHERE clause), the refund must be idempotent to avoid double-credit. The existing `RefundCoinsInternal` handles this via INSERT-FIRST idempotency.

4. **Semantic mismatch**: Coins are a "loyalty usage right," not money. Spending them before the payment exists creates a conceptual problem — the user "spent" loyalty points but received nothing in return. The reservation model better matches the "right to use" semantics.

5. **Order snapshot contamination**: `order.coins_used` would be set to K at payment creation time, but the order isn't paid yet. If the order expires, `coins_used` must be cleared. This creates ambiguity about what `coins_used` means — is it "coins reserved for this order" or "coins actually consumed by this order"?

**Verdict: Model S is UNACCEPTABLE.** The Midtrans-outside-tx gap alone disqualifies it. Adding the semantic mismatch, the disabled expiry worker risk, and the misleading transaction history makes Model S clearly inferior.

## Recommended Model: Model R

**Canonical direction:** RESERVE at payment intent creation → CONSUME on settlement webhook → RELEASE on terminal failure/expiry.

**Key design decisions:**
- Reservation happens inside the payment-creation DB transaction (Phase 3) — coins are atomically protected BEFORE Midtrans is called
- The `coin_reservations` table is the reservation authority
- `AtomicDeductBalance` provides the concurrency primitive (balance reduced at reservation time, so `available = balance` after all reservations are deducted)
- `SpendCoinsTx` is reused for the CONSUME step (called from webhook, not from payment creation)
- Existing `coins.refund_required` outbox pattern is adapted for RELEASE
- `payments.coin_discount_amount` becomes the canonical monetary record of K
- `orders.coins_used` is set ONLY at settlement (when reservation is consumed), maintaining its semantics as "coins actually used on this completed order"

---

# 8. CLIENT INTENT CONTRACT (Part H)

## Current State

**Backend `CreatePaymentRequest`:**
```go
CoinDiscount int `json:"coin_discount"`  // line 3107
```

This is accepted from the client, stored on the payment row, but `coinDiscountAmount` is hardcoded to 0. The field exists but has no monetary effect.

**Mobile `CreatePaymentRequestDto`:**
```dart
final int coinDiscount;  // default 0
```

Both call sites pass `coinDiscount: null` (effectively 0). The coin toggle in checkout sends `use_coins` on POST /orders (not on POST /payments), but the backend ignores it (order creation never spends coins).

**Mobile checkout flow:**
- Coin toggle exists with balance display
- `use_coins: true/false` sent on POST /orders → backend ignores (coins_used = 0)
- Preview endpoint receives `use_coins` but the implementation drops it; `coinDiscount` always 0.0
- "Diskon Koin" UI row can never render
- The entire client-side coin flow is wired up but non-functional

## Required API Change

**Replace `coin_discount` with explicit `coins_to_use`:**

| Aspect | Current | Future |
|--------|---------|--------|
| Field name | `coin_discount` (int) | `coins_to_use` (int) |
| Semantics | Ambiguous — is it Rupiah? Count? | Explicit: count of coins user requests to redeem |
| Client authority | Client submits the number | Client submits the count; backend derives K, validates eligibility, computes fee/gross |
| Monetary derivation | None (hardcoded to 0) | Backend: K = coins_to_use (1 coin = Rp1), MaxCoinsAllowed = floor(20% × PD), validate K ≤ min(balance, MaxCoinsAllowed) |
| Response fields | `coin_discount`, `coin_discount_amount`, `net_amount` | `coins_to_use`, `coin_discount_amount`, `net_amount` (same structure, clearer naming) |

**Client MUST NOT submit:**
- Coin Rupiah amount (K is derived, 1 coin = Rp1)
- Gross amount
- Fee amount
- Commission amount
- Discount monetary value

## Decision: `coin_discount` → RENAME to `coins_to_use`

The old name `coin_discount` is ambiguous (Rupiah? count? percentage?). The new name `coins_to_use` is semantically exact: "the count of coins the user explicitly requests to redeem." Backend derives everything else. **No compatibility shim needed** — the current field has zero monetary effect.

---

# 9. MOBILE FLOW IMPACT (Part I)

## Current Flow

```
Checkout Screen
  ├── Coin balance loaded (GET /coins/balance)
  ├── Coin toggle "Gunakan Koin Labuda" with balance display
  ├── Pricing preview (POST /pricing/preview) — use_coins dropped, coinDiscount=0
  ├── "Buat Pesanan" → POST /orders (use_coins sent, backend ignores)
  ├── Payment method picker (GET /payments/methods) ← AFTER order creation
  ├── Create payment (POST /payments) — coin_discount always 0
  └── Launch Midtrans URL
```

## Target Future Flow

```
Checkout Screen
  ├── Coin balance loaded (GET /coins/balance)
  ├── Pricing preview (POST /pricing/preview) — with coins_to_use, backend calculates K
  │   └── Response: coin_discount_amount, net_amount, fee with coins applied
  ├── Coin selector (user picks exact amount, 0 to min(balance, max_allowed))
  │   └── Display: "Anda akan menggunakan N koin (potongan Rp N)"
  ├── "Buat Pesanan" → POST /orders (no coin fields — coins applied at payment, not order)
  ├── Payment method picker (GET /payments/methods) — preview includes coin reduction
  │   └── Each method shows: fee with coins, total with coins
  ├── Create payment (POST /payments) — coins_to_use sent, backend reserves coins
  │   └── Response: coins_to_use, coin_discount_amount, net_amount (authoritative)
  └── Launch Midtrans URL (with reduced gross_amount)
```

## Key Changes

1. **Coin input becomes a numeric selector**, not a toggle. User chooses exact coin count.
2. **Max and balance are server-authoritative.** GET /pricing/preview returns `max_coins_allowed` and `current_balance`. Client can display estimates but backend response is final.
3. **Payment method picker must account for coins.** Each method's fee is calculated on the post-coins cash amount. GET /payments/methods must accept `coins_to_use` parameter.
4. **POST /payments sends `coins_to_use`** — backend validates, reserves, derives amounts.
5. **Double-tap guards already exist** (isInitiating lock + 5s cooldown + screen-level isSubmitting). These remain sufficient.
6. **Result screen polling already exists** — polls GET /orders/{id} + GET /payments/{id} until terminal state.

---

# 10. FUTURE TEST REQUIREMENT MAP (Part J)

| # | Test | Classification | Current Infrastructure |
|---|------|---------------|----------------------|
| 1 | `coins_to_use=0` → no reservation, no deduction, gross=escrow+fee | Integration | Payment payability guard tests (`payment_payability_guard_test.go`) — can extend |
| 2 | Exact positive `coins_to_use` → K reserved, gross reduced by K | Integration | New |
| 3 | `coins_to_use > balance` → rejected with identifiable error | Integration | New |
| 4 | `coins_to_use > 20% PD` → rejected with max_allowed in error | Integration | New |
| 5 | Shipping does not increase max (max = floor(20% × PD), not PD+S) | Integration | New |
| 6 | 1 coin = Rp1 (K = coins_to_use exactly) | Unit | New |
| 7 | Two concurrent payments, same user, total K > balance → one rejected | Concurrency | Requires pgx test harness with concurrent transactions |
| 8 | Same order duplicate CreatePayment → idempotent return of first reservation | Integration | Payment reuse guard already tested |
| 9 | Retry after HTTP timeout → Midtrans called again for same payment, reservation intact | Integration | New (requires Midtrans mock) |
| 10 | Payment method change → old reservation released, new reservation created | Integration | New |
| 11 | Expiry releases reservation → balance restored, reservation status=released | Integration | Can extend expiry worker tests |
| 12 | Deny/cancel releases reservation → balance restored | Integration | Can extend webhook tests |
| 13 | Settlement consumes reservation → spend transaction created, order.coins_used = K | Integration | Can extend webhook settlement tests |
| 14 | Duplicate webhook → coins consumed exactly once | Integration | Webhook idempotency already tested |
| 15 | Duplicate release event → balance credited exactly once | Integration | New |
| 16 | Midtrans gross generated only after reservation committed | Integration | New (requires Midtrans mock with transaction boundary verification) |
| 17 | Real PostgreSQL concurrency: 10 concurrent reservations, balance ends correct | Concurrency | Requires pgx test harness |
| 18 | Seller entitlement unaffected by K (commission calculated on PD, not PD-K) | Integration | Can extend existing pricing tests |

**Current test infrastructure:** The project has a mature test harness with:
- `payment_payability_guard_test.go` — tests CreatePayment validation directly against handler source code
- `payment_method_default_killed_test.go` — tests payment method resolution
- `order_creation_service_test.go` — fake-repos unit tests for order creation
- `order_canonical_test.go` — integration tests asserting coins_used=0
- `coins_refund_handler_test.go` (implied by the refund handler's presence)

Missing: concurrent transaction test harness for PostgreSQL-level race conditions (items 7, 17).

---

# 11. RECOMMENDED FIRST IMPLEMENTATION SLICE

**Slice 0 (this audit):** Confirm the technical direction. **DONE.**

**Slice 1 — Schema + Reservation Primitive:**
1. Create `coin_reservations` table migration
2. Implement `CreateReservation`, `ConsumeReservation`, `ReleaseReservation` repository methods
3. Add UNIQUE constraints for idempotency
4. Unit tests for reservation state machine

**Slice 2 — Payment Creation Integration:**
5. Add `coins_to_use` to `CreatePaymentRequest`
6. Integrate reservation into `CreatePayment` handler (Phase 3, inside DB tx)
7. Derive `coin_discount_amount = K`, `net_amount = gross - K`
8. Pass reduced `gross_amount` to Midtrans
9. Add release-on-Midtrans-failure compensating action
10. Integration tests for items 1-6, 8, 16

**Slice 3 — Settlement Consumption:**
11. Integrate consumption into webhook settlement path
12. Set `order.coins_used = K` at settlement time
13. Create spend transaction via `SpendCoinsTx`
14. Tests for items 13, 14

**Slice 4 — Release Paths:**
15. Integrate release into payment failure webhook (deny/cancel/expire)
16. Integrate release into payment expiry worker
17. Add reconciliation job for orphaned reservations
18. Tests for items 11, 12, 15

**Slice 5 — Mobile + Pricing Preview:**
19. Replace `coin_discount` with `coins_to_use` in mobile API client
20. Add numeric coin selector UI
21. Wire pricing preview to include coin reduction
22. Wire payment method picker to include coin impact
23. Remove dead `RecordCoinsUsage` / `RecordCoinsUsageTx` / `SpendCoins` / `ApplyCoinsSnapshotToOrder`
24. End-to-end tests

---

# 12. NEW BUSINESS AMBIGUITIES

| # | Ambiguity | Resolution Needed |
|---|-----------|-------------------|
| 1 | What happens if Midtrans fails AFTER reservation? How long before reservation is auto-released? | Define reservation TTL. Recommend: same as `payment.expired_at`. If payment URL is not set within TTL, reconciliation job releases reservation. |
| 2 | Can user change `coins_to_use` on retry? | Recommend: NO. Retry reuses same payment and same reservation. To change coins, user must cancel first payment and create new one. |
| 3 | What if Midtrans succeeds but the user's coin balance was corrupted (e.g., reconciliation found drift)? | Settlement should still succeed — the monetary transaction with Midtrans is independent of coin balance integrity. Log the discrepancy. |
| 4 | Should `coin_discount_amount` be stored on `orders` or only on `payments`? | Both. `payments.coin_discount_amount` is the monetary authority. `orders.coin_discount_amount` is the display snapshot, set at settlement time. |
| 5 | What if settlement arrives but the reservation was already released by expiry worker? | Race condition: webhook vs expiry worker. Current `SettlePaymentByID` hard-blocks settlement if order is expired. Same guard protects reservation consumption — if reservation status ≠ 'reserved', skip consumption. |

---

# 13. FILES INSPECTED

| File | Relevance |
|------|-----------|
| `backend/migrations/000001_canonical_schema.up.sql` | All schema: user_coin_balance, coins_transactions, payments, payment_attempts, orders, enums, constraints, indexes |
| `backend/internal/incentive/coins/entity/user_coin_balance.go` | Balance entity |
| `backend/internal/incentive/coins/entity/coins_transaction.go` | Transaction entity, enums, constructors |
| `backend/internal/incentive/coins/repository/coins_repository.go` | Repository interface |
| `backend/internal/incentive/coins/infrastructure/repository/coins_repository_impl.go` | Repository implementation (AtomicDeductBalance, CreateTransaction, etc.) |
| `backend/internal/incentive/coins/application/coins_service.go` | CoinsService: SpendCoins, SpendCoinsTx, RefundCoinsInternal, EarnPointsForOrderCompletion |
| `backend/internal/commerce/order/application/order_payment_service.go` | RecordCoinsUsage, RecordCoinsUsageTx, ApplyCoinsSnapshotToOrder |
| `backend/internal/commerce/order/application/order_creation_service.go` | finalizeOrderCreationTx (coins NOT spent at order creation) |
| `backend/internal/commerce/order/entity/order.go` | Order entity, CoinsUsed field, ApplyCoinsSnapshot |
| `backend/internal/commerce/order/infrastructure/repository/order_repository.go` | CreateOrderTx (coin fields hardcoded to 0) |
| `backend/internal/serve rboot/dependencies.go` | CorePaymentHandler.CreatePayment (full flow), createMidtransTransaction, Snap builder |
| `backend/internal/serve rboot/midtrans_snap_builder.go` | Snap request builder |
| `backend/internal/integration/payment/infrastructure/repository/payment_repository.go` | CreatePayment, FindExistingPaymentForOrder, GetOrCreateForOrder |
| `backend/internal/integration/payment/infrastructure/repository/entity.go` | Payment entity, status constants, ReferenceTypeOrder |
| `backend/internal/integration/payment/infrastructure/repository/payment_settlement_service.go` | SettlePaymentByID, FailPayment |
| `backend/internal/integration/payment/application/payment_webhook.go` | Webhook handler, HandleWebhook, handleWebhookInTransaction |
| `backend/internal/integration/payment/application/canonical_finalization_service.go` | FinalizeOrderPayment |
| `backend/internal/integration/payment/delivery/http/payment_webhook_handler.go` | Webhook HTTP handler |
| `backend/internal/worker/payment_expiry_worker.go` | Payment expiry worker |
| `backend/internal/worker/coins_refund_handler.go` | Coin refund handler |
| `backend/pkg/midtrans/midtrans.go` | Midtrans client, CreateSnapTransaction |
| `apps/mobile/lib/domains/finance/transaction/payment/data/dto/payment_dto.dart` | CreatePaymentRequestDto (coinDiscount field) |
| `apps/mobile/lib/domains/finance/transaction/payment/domain/entities/payment.dart` | CreatePaymentRequest entity |
| `apps/mobile/lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart` | Checkout flow (coin_discount not sent) |
| `apps/mobile/lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_coin_section.dart` | Coin toggle UI |
| `SCOPE_4B_S2A_V_REPORT.md` | Prior verification that order creation does zero coin spend |
| `SCOPE_4A_AUDIT_REPORT.md` | Prior audit referencing RecordCoinsUsage call chain |

---

# 14. FILES CHANGED

**NONE.** This is an audit-only scope.

---

# 15. FINAL VERDICT

**`COIN_REDEMPTION_RESERVATION_AND_PAYMENT_INTENT_CONCURRENCY_AUDIT_COMPLETE`**

The technical direction is unambiguous:

1. **Model R** (RESERVE → CONSUME / RELEASE) is the only safe approach.
2. **The payment row** owns the reservation. The `coin_reservations` table is the new reservation authority.
3. **`RecordCoinsUsage` / `RecordCoinsUsageTx` / `SpendCoins` must be PURGED** — their immediate-SPEND semantics are incompatible.
4. **`SpendCoinsTx` is reusable** as the CONSUME primitive called from the settlement webhook.
5. **`coin_discount` → `coins_to_use`** rename required on both backend and mobile.
6. **Midtrans must receive reduced gross_amount** only AFTER reservation is committed. The current architecture already separates the DB tx (Phase 3) from the Snap call (Phase 4), so this ordering is achievable without refactoring the Midtrans integration.
7. **The payment expiry worker MUST be enabled** before coin reservation goes live to prevent permanent coin lockup.
8. **No business policy ambiguity remains** — all invariants are derivable from the locked business truth. The only open question is reservation TTL, which should mirror `payment.expired_at`.
