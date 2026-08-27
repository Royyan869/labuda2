# COIN SETTLEMENT FUNDING / LEDGER AUTHORITY AUDIT

Read-only audit. No production code, tests, migrations, schema, ledger, refund, payment, or accounting behavior modified. No new ledger account or journal entry invented.

```
VERDICT: COIN_FUNDING_CONTRACT_CONTRADICTION_FOUND
```

The seller economic entitlement (PD + S) cannot be funded from GATEWAY_CLEARING (PD + S − K) because K has no accounting representation anywhere: no coins→ledger journal entry, no coin account, no funding mechanism. Additionally, the coin redemption itself (balance deduction + `order_spend` transaction) is **not implemented in production** — the reservation is created but never consumed, and the spend transaction is never written.

---

## 1. CANONICAL_FACTS (locked contract, verified against source)

| Fact | Source |
|---|---|
| `EscrowAmount = PD + S` | `canonical_finalization_service.go:117-131` (escrow row = `order.TotalBeforeCoinsAmount`) |
| Seller economic entitlement = `PD + S` | `order_payment_service.go:318-337` (`ReleaseGatewayEscrowToSeller`: `gross = TotalBeforeCoinsAmount`, `sellerNet = gross − commission`) |
| Buyer cash payment = `(PD+S) − K + F` | `serverboot/dependencies.go:3280-3317` (`baseAmount = TotalBeforeCoins`, `cashAmount = base − coins`, `grossMoney = cash + fee`) |
| GATEWAY_CLEARING funded by `payment.GrossAmount = (PD+S)−K+F` | `finance_service.go:323-398` (`RecordGatewayPaymentSettlement`) |
| K authoritative in coins domain | `coin_reservation.go` (Model R: reserve/consume/release), `user_coin_balance.go` (single source of truth) |
| `orders.coins_used` NOT financial authority | locked contract; `coins_refund_handler.go:130` ("can be stale / not persisted to orders") |

## 2. COIN_AUTHORITY

- **Canonical coin authority**: the coins domain — `user_coin_balance` (aggregate balance), `coin_reservations` (Model R hold), `coins_transactions` (audit trail).
- **Explicitly non-financial**: `coins_transaction.go:66-70` — "IMPORTANT: This is NOT a financial ledger entry. Loyalty points are non-financial rewards only, not money."
- **No production spend path**: `ConsumeReservation` (`coins_repository_impl.go:477`), `NewSpendTransaction` (`coins_transaction.go:136`), `AtomicDeductBalance` (`coins_repository_impl.go:651`) have **no production callers** — only tests (`coin_reservation_concurrency_test.go`, `order_creation_service_test.go`).
- **Reservation lifecycle in production**: created at payment creation (`dependencies.go:3401-3407`), released on failure (`dependencies.go:3932`), **never consumed** on settlement (no production `ConsumeReservation` call).

## 3. FUNDING_SOURCE

| Component | Funding source | Exists? |
|---|---|---|
| Buyer cash `(PD+S)−K` | gateway (Midtrans) | ✅ `dependencies.go:3306` |
| Buyer fee F | gateway (part of gross) | ✅ `dependencies.go:3317` |
| **K (coin redemption)** | **NONE** — no ledger credit to GATEWAY_CLEARING, no coin account, no journal entry | ❌ |
| Commission C | carved from seller release (GATEWAY_CLEARING → PLATFORM_REVENUE) | ✅ `finance_service.go:220-224` |
| Seller entitlement `PD+S` | GATEWAY_CLEARING (funded `(PD+S)−K`) | ⚠️ **shortfall = K** |

## 4. GATEWAY_CLEARING_FUNDING (full trace)

| Step | Entry | GATEWAY_CLEARING delta | Balance |
|---|---|---|---|
| Settlement | `RecordGatewayPaymentSettlement(gross = (PD+S)−K+F)` | +((PD+S)−K+F) | (PD+S)−K+F |
| Fee sweep | `RecordBuyerPaymentFeeRevenue(F)` | −F | (PD+S)−K |
| Release | `RecordOrderRelease(gross = PD+S)` | −(PD+S) | **−K** |

`GATEWAY_CLEARING` is debited `PD+S` against a funded `(PD+S)−K`. The K difference has no funding source. This violates the locked invariant "GATEWAY_CLEARING must never be debited for K unless an actual funding mechanism exists."

## 5. EXISTING_COIN_ACCOUNTING

- `coins_transactions` (order_reward / order_spend / refund_earn / refund_spend): **not a ledger**, no double-entry, no account balance. `coins_transaction.go:66-70`.
- `coin_reservations`: reservation hold only (reserve→consume/release). `coin_reservation.go`.
- `user_coin_balance`: single aggregate balance row per user. `user_coin_balance.go`.
- **No coins↔ledger bridge**: no finance-domain method references coins; no coins-domain method writes to `financial_accounts` or `ledger_entries`.

## 6. EXISTING_LEDGER_ENTRIES (complete set)

| Ledger method | Accounts touched | Coin involvement |
|---|---|---|
| `RecordGatewayPaymentSettlement` | GATEWAY_CLEARING +gross, BANK_SETTLEMENT −gross | none (gross excludes nothing coin-specific; it IS `(PD+S)−K+F`) |
| `RecordBuyerPaymentFeeRevenue` | GATEWAY_CLEARING −F, PLATFORM_REVENUE +F | none |
| `RecordOrderRelease` | GATEWAY_CLEARING −gross, SELLER_PAYABLE +sellerNet, PLATFORM_REVENUE +commission | none |
| `RecordRefundReversal` | BUYER_REFUNDABLE +refund, GATEWAY_CLEARING/SELLER_PAYABLE/PLATFORM_REVENUE −components | none (CoinDelta is NOT a ledger entry; it is a coins-domain earn) |
| `RecordPartialRefundRelease` | GATEWAY_CLEARING −remainder, SELLER_PAYABLE +net, PLATFORM_REVENUE +commission | none |

**No ledger entry represents K.** The coins refund (`coin_delta`) is delivered via `coins.refund_required` outbox → `RefundCoinsWithDelta` (`coins_refund_handler.go:224`), which writes a `coins_transactions` earn row — **never a ledger entry**.

## 7. LEDGER_INVARIANT

- Per-transaction Σ=0: holds (each `Record*` method enforces balanced entries; `finance_service.go` invariants).
- **GATEWAY_CLEARING ≥ 0: VIOLATED when K > 0** — release debits `PD+S` from a pool of `(PD+S)−K` → balance `−K`.
- The `financial_accounts.balance >= 0` CHECK constraint would **reject the release** at the DB level for coin-redemption orders, causing the release to fail (a hard failure, not a silent overdraw).

## 8. CONTRADICTIONS

1. **Escrow = PD+S vs funded = (PD+S)−K**: the escrow row and release debit assume `PD+S` was funded; gateway cash was only `(PD+S)−K`. No K funding entry exists.
2. **Coin redemption not implemented**: the reservation is created but never consumed; `order_spend` transaction and balance deduction have no production caller. Coin refunds therefore no-op (the refund handler finds no spend → "No spend transaction found, skipping" at `coins_refund_handler.go:302-309`).
3. **`payment_coin_settlement_integration_test.go` asserts a rejected model**: it expects escrow = 124500 (P+S+C) and `orders.coins_used`/`coin_discount_amount` populated — both rejected by the locked contract. This tracked test cannot pass against the converged production.

## 9. OWNER_DECISION_REQUIRED

1. **Is K an intentional non-ledger platform cost?** The contract says "K is NOT part of GATEWAY_CLEARING funding." If the platform intentionally absorbs K as a marketing/loyalty expense (coins are issued as non-monetary loyalty points per `coins_transaction.go:66-70`), then the ledger correctly reflects only real cash, and the escrow/release of `PD+S` is economically backed by the platform's coin liability — but this is **not documented anywhere as an accounting contract**, and the escrow row (`escrows.amount = PD+S`) would exceed the actual gateway cash held. An explicit decision is required: (a) confirm K is a platform-borne non-ledger cost with `escrows.amount = PD+S` being an economic (not cash-backed) obligation, OR (b) authorize a specific funding mechanism.
2. **The missing coin spend implementation**: no production code creates the `order_spend` transaction or deducts the coin balance at settlement. This is a functional gap in the coins domain (not a ledger issue) — the reservation lifecycle (RESERVE→CONSUME) is incomplete. Owner must decide whether to implement the consume+spend in the coins domain (canonical coins authority) or accept reservations never being consumed.
3. **The tracked `payment_coin_settlement_integration_test.go`**: its rejected assertions (P+S+C escrow, dead-column coins) block the integration suite. Owner must authorize updating/removing those assertions.

## 10. KILL_CANDIDATES

| Artifact | Classification | Reason |
|---|---|---|
| `payment_coin_settlement_integration_test.go` rejected assertions (escrow=124500, `orders.coins_used`/`coin_discount_amount` populated) | LEGACY/ZOMBIE (rejected model) | Encodes P+S+C + dead-column authority |
| `CalculateGrossEscrowFromSnapshot` (P+S+C) | CONFLICTING AUTHORITY | Only remaining production reference is the tracked test; production escrow/release no longer uses it |
| `ConsumeReservation`/`NewSpendTransaction`/`AtomicDeductBalance` unused production surface | LEGACY/ZOMBIE | No production caller; the consume+spend is unimplemented |

## 11. PROTECTED

- `user_coin_balance`, `coin_reservations`, `coins_transactions` — canonical coin authority (non-ledger).
- `RecordGatewayPaymentSettlement` / `RecordBuyerPaymentFeeRevenue` / `RecordOrderRelease` / `RecordRefundReversal` / `RecordPartialRefundRelease` — canonical ledger movements (Σ=0 each).
- `escrows.amount` + escrow state machine — canonical escrow row (currently set to PD+S).
- `coins.refund_required` outbox → `CoinsRefundRequiredHandler` → `RefundCoinsWithDelta`/`RefundCoinsInternal` — canonical coin restoration path (coins domain only).
- `payment.GrossAmount = (PD+S)−K+F` — canonical buyer cash.

## 12. TESTS

| Command | Exit | Result |
|---|---|---|
| `go build ./...` | 0 | Production builds |
| `go vet` (affected packages) | 0 | Clean |
| `go test ./internal/finance/refund/application/ ./internal/finance/verifier/ ./internal/core/wallet/application/ ./internal/commerce/order/application/ ./internal/integration/payment/...` | 0 | All pass |
| `go test -tags integration -run TestCreatePayment_BasicFlowAndPreviewAuthority ./internal/serverboot/` | 0 | Canonical `(PD+S)` buyer base verified |
| `go test -tags integration ./internal/serverboot/` | compiles | Harness intact |

(No DB was modified; the audit ran `go build`/`go vet`/`go test -run` compile checks only.)

## 13. FILES_CHANGED

```
FILES_CHANGED:
  NONE
  (this audit is read-only; only this report COIN_SETTLEMENT_FUNDING_LEDGER_AUTHORITY_AUDIT.md was created)
```

```
DATABASE_CHANGED:
  NONE

MIGRATIONS_CHANGED:
  NONE
```

## 14. FINAL_STOP

The existing code does **not** establish a funding contract for K. The locked requirement "seller entitlement = PD + S while gateway cash funding = PD + S − K, without overdrawing GATEWAY_CLEARING" is unsatisfiable in the current source: K has no accounting representation, no coin account exists, no ledger entry funds K, and the coin redemption (spend) itself is unimplemented in production. Per the audit's STOP rule, this audit stops here — no accounting model was chosen, created, or modified. The owner must decide whether K is an intentional non-ledger platform cost (documenting the escrow as economic-not-cash-backed) or authorize a specific funding mechanism, and must resolve the unimplemented coin-spend path and the tracked test's rejected assertions.
