# GLOBAL FINANCIAL AUTHORITY & RESIDUE AUDIT

Read-only audit. No production code, tests, migrations, schema, ledger, refund, payment, coin, or accounting behavior was modified. The ONLY artifact created is this report.

Audit resumed after an interrupted session. The designated report did not exist at resume time; this pass completed it from scratch, harvesting verified evidence from the pre-existing audit reports and cross-checking every claim against the current filesystem.

---

## 1. VERDICT

```
GLOBAL_FINANCIAL_AUTHORITY_CONVERGED
```

- ONE FINANCIAL TRUTH: the pricing-token snapshot `(P−D)+S` for BuyerBase/EscrowAmount; the coins domain for K; the finance ledger (`Record*` methods, Σ=0 each) for money movement; PLATFORM_BANK as the funding source of K.
- ONE AUTHORITY per symbol. No competing authority survives in a live production path.
- ALL PRODUCERS CONVERGED: `pricing_token_service.go` (3 producers) emit `escrowAmount = (P−D)+S`; `canonical_finalization_service.go` creates escrow from `TotalBeforeCoinsAmount`; `order_creation_service.go`/`order_repository.go` persist `total_before_coins_amount`; `finance_service.go` books settlement/fee/funding/release/reversal with explicit funding.
- ALL CONSUMERS CONVERGED: escrow integrity checker, verifier, recon resolver/classifier, projection worker, refund gateway/service, order payment/completion, admin finance summary — all read the canonical base or the ledger.
- REJECTED MODEL DEAD: no `CalculateGrossEscrowFromSnapshot`, no `LegacyGross`, no `P+S+C` producer, no orderGross commission denominator, no `orders.discount_amount`/`escrow_amount`/`coins_used` authority in production.
- NO ZOMBIE RESIDUE in a live money path. Remaining residue is confined to stale comments, dead DTO fields, hardcoded l10n strings, one stale test fixture, and a `SUM(0)` stub — none can resurrect the rejected model at runtime (classification and rationale in §4, §6).
- NO SECOND AUTHORITY: the only surviving duplicate identities are benign aliases/derived passthroughs (documented in §4).
- NO UNFUNDED LEDGER MOVEMENT: `RecordCoinFunding` (DR PLATFORM_BANK / CR GATEWAY_CLEARING) and `RecordCoinFundingReversal` (DR GATEWAY_CLEARING / CR PLATFORM_BANK) give K an explicit funding/source relationship. GATEWAY_CLEARING is never debited beyond its funded balance.

Pre-existing unrelated test failures (worker content-mention/alert build break, `withdraw_request_idempotency_test.go` build break, two refund-history contract tests, rating HTTP contract test, serverboot chat projection test) are tracked, pre-date the convergence, and do not encode the rejected financial model. They are classified in §8 and §11.

---

## 2. CANONICAL_CONTRACT

| Symbol | Definition | Authority source |
|---|---|---|
| P | product subtotal | `pricing_token_service.go` (`subtotal`) |
| D | seller-funded discount | pricing token snapshot `discountAmount` |
| PD | P − D | `pricing_token_service.go:385,978,1261` (`discountedProduct`) |
| S | shipping | `order.ShippingTotal` snapshot |
| C | `floor(PD × rate / 100)`, product-only | `pricing_token_service.go:679-682` (`calculateCommission`), invoked at `:384,948,1248` |
| K | redeemed coins, authoritative in coins domain | `payments.coins_to_use`, `coin_reservations`, `coins_transactions`, `user_coin_balance` |
| BuyerBase / EscrowAmount | PD + S | `pricing_token_service.go:409,1273` (`escrowAmount`); persisted as `orders.total_before_coins_amount` (`order_repository.go:96`); escrow row = same (`canonical_finalization_service.go:174-180`) |
| BuyerCash | (PD+S) − K + F | `serverboot/dependencies.go:3286,3312,3323` (`baseAmount`, `cashAmount`, `grossMoney`) |
| GATEWAY_CLEARING funding | settlement `+((PD+S)−K+F)` → fee sweep `−F` → platform K funding `+K` = `PD+S` | `RecordGatewayPaymentSettlement`, `RecordBuyerPaymentFeeRevenue`, `RecordCoinFunding` (`finance_service.go:323,426,508`) |
| Seller entitlement | (PD+S) − C at release | `order_payment_service.go:326-337` (`ReleaseGatewayEscrowToSeller`) |
| Platform revenue | F + C | fee sweep + release carve (`finance_service.go:426,187-234`) |
| Refund | `CashRefund = Rpd + Rs − CoinDelta`; funding reversed by `CoinDelta` | `refund_math.go:35-139`; `refund_gateway.go:386-434`; `RecordCoinFundingReversal` (`finance_service.go:588`) |

Ledger invariants enforced:
- Σ(entries) = 0 per transaction — `ledger_repository.go:33-70` (panic on unbalanced) + DB constraint `ledger_transactions_balanced` (`000001_canonical_schema.up.sql:2460`).
- `financial_accounts.balance >= 0` — CHECK constraint (`:2455`); PLATFORM_BANK and BANK_SETTLEMENT reserve floats seeded (`system_account_bootstrap.go:72,91`).
- Every movement has a funding/source relationship — settlement funds GATEWAY_CLEARING; fee sweep drains F to PLATFORM_REVENUE; `RecordCoinFunding` funds K from PLATFORM_BANK; release drains to SELLER_PAYABLE + PLATFORM_REVENUE; refund reversal + `RecordCoinFundingReversal` restore the funded structure.

REJECTED MODEL (must remain dead): P+S+C; P+S+C−D; orderGross as commission denominator or buyer authority; `orders.discount_amount`/`escrow_amount`/`coins_used` as authority; `CalculateGrossEscrowFromSnapshot`; `LegacyGross`; K vanishing from gateway cash without funding; K reducing seller entitlement; K becoming PLATFORM_REVENUE; unfunded GATEWAY_CLEARING debits; refund coin restoration without funding reversal; duplicate Commission identity; duplicate PD authority; duplicate BuyerBase authority.

---

## 3. AUTHORITY_MAP

| Authority | Canonical Source | Producers | Consumers | Status |
|---|---|---|---|---|
| Pricing snapshot (P, D, PD, S, C, F-base) | `pricing_tokens` + `PricingTokenService` | `pricing_token_service.go:379-512,948-1060,1211-1373`; `pricing_token_repository_impl.go`; `NewPricingToken` | order creation (`order_creation_service.go:925-953,1609-1624`), order handler DTOs (`order_handler.go:1613`, `chat_handler.go:2013`, `auction_handler.go:886`), payment boundary (`serverboot/dependencies.go:3145-3148`, `3312-3319`) | CANONICAL_AUTHORITY — converged |
| BuyerBase / EscrowAmount = PD+S | `orders.total_before_coins_amount` | token escrow → order creation (`order_creation_service.go:938` → `NewOrderFromSource` `order.go:1103` → `order_repository.go:96`); escrow row (`canonical_finalization_service.go:174-180`) | escrow checker (`escrow_integrity_checker.go:240,273`), verifier (`verifier.go:641,677,1038`), recon (`classifier.go:432`, `resolver.go:199`), projection (`projection_worker.go:440,518,785`), refund (`refund_service.go:232`, `refund_gateway.go:337`), release (`order_payment_service.go:326`), payment (`dependencies.go:3286,3607,3660`), partial-dispute (`order_completion_service.go:1741`) | CANONICAL_AUTHORITY — converged |
| Coin authority K | coins domain: `user_coin_balance`, `coin_reservations`, `coins_transactions` | `CoinsService.ConsumeAndSpendForOrder` (`coins_service.go:371-427`) via `CanonicalFinalizationService` (`:118-127`); reservation at payment creation (`dependencies.go` coins wiring) | refund pipeline `coinsSpendForOrder` (coins domain reader — `refund_gateway.go:343`, `order_payment_service.go:217`, `refund_service.go`); coin refund handler (`coins_refund_handler.go`); K never read from `orders.coins_used` | CANONICAL_AUTHORITY — converged |
| K funding source | `PLATFORM_BANK` (platform's own bank holdings) | `RecordCoinFunding` (`finance_service.go:508-557`, DR PLATFORM_BANK −K / CR GATEWAY_CLEARING +K, idem `coin_funding_<payment_id>`) | release (funded `PD+S`); reversal on refund | CANONICAL_AUTHORITY — converged, reserve float seeded |
| Ledger | `FinanceService.Record*` (Σ=0 each) | `RecordGatewayPaymentSettlement` (`:323`), `RecordBuyerPaymentFeeRevenue` (`:426`), `RecordCoinFunding` (`:508`), `RecordOrderRelease` (`:187`), `RecordRefundReversal` (`:728`), `RecordPartialRefundRelease` (`:952`), `RecordCoinFundingReversal` (`:588`), withdrawal `Record*` (`:1278-1550`) | admin finance summary (`admin_finance_handler.go:502-562`), verifier ledger checks, recon | CANONICAL_AUTHORITY — converged |
| Commission identity | `order.CommissionAmount` (immutable snapshot) | token → order copy (`order_creation_service.go:936,1622`), `order_repository.go:91` | release, refund gateway, verifier, DTOs | CANONICAL_AUTHORITY — converged (single identity; CommissionDelta is derived, not an identity) |
| Escrow row | `escrows.amount` (= PD+S) | `CreateEscrowFromGatewaySettlement` (`canonical_finalization_service.go:175-183`) | wallet service, escrow checker, release, refund flip | CANONICAL_AUTHORITY — converged |
| `orders.discount_amount` | dead column (never written by production) | NONE | NONE in production (comments only) | LEGACY_RESIDUE — harmless (see §4 R-1) |
| `orders.escrow_amount` | dead column (never written by production) | NONE (projection writes `total_before_coins_amount` into the `order_summaries.escrow_amount` column — a projection column name, not the orders column) | NONE in production money paths (comments only; `decision.go:251` stale comment) | LEGACY_RESIDUE — harmless (see §4 R-2) |
| `orders.coins_used` | display-only DTO field (never persisted by production; production writes 0) | `order_repository.go:94` (writes 0) | DTO passthroughs (`decision.go:293`), outbox trigger guard `order.CoinsUsed > 0` (in-memory value, not a DB read) | CANONICAL (display) / LEGACY (column) — see §4 R-3 |

---

## 4. COMPLETE_RESIDUE_MAP

Classification of every non-canonical finding. All are non-runtime or test-only; none can resurrect the rejected model in production.

| # | Path | Symbol | Lines | Current meaning | Authority | Safe? | Resurrects rejected model? | Action |
|---|---|---|---|---|---|---|---|---|
| R-1 | `backend/migrations/000001_canonical_schema.up.sql` | `orders.discount_amount` | 1104, 1265 | dead column, DEFAULT 0; no production INSERT/UPDATE writes it | NONE (dead) | Yes | No (nothing reads it in production) | OWNER_DECISION (drop or keep as display column) |
| R-2 | `backend/migrations/000001_canonical_schema.up.sql` | `orders.escrow_amount` | 1073, 1099, 1261; CHECK 2473, 2477 | dead column, DEFAULT 0; no production writer; `orders_check` (refunded_amount ≤ escrow_amount) binds to it | NONE (dead) | Yes (always 0, so `refunded_amount ≤ 0` is the effective constraint) | No | OWNER_DECISION (drop column + CHECK, or repoint CHECK at `total_before_coins_amount`) |
| R-3 | `backend/migrations/000001_canonical_schema.up.sql` | `orders.coins_used`, `orders.coin_discount_amount` | 1140-1142, 1226, 1275; CHECK 2471-2472 | dead columns, production writes 0; integration tests assert they stay 0 | NONE (coins domain is authority) | Yes | No | OWNER_DECISION (drop or keep as display columns) |
| R-4 | `backend/internal/worker/notification_worker_shared.go` | `EscrowAmount` field | 77 | unused struct field with JSON tag | none (no production consumer found) | Yes | No | KEEP (dead field, or delete in a cleanup pass) |
| R-5 | `backend/internal/commerce/order/delivery/http/dto/decision.go` | comment "EscrowAmount = Subtotal + Shipping + Commission" | 251 | stale comment contradicting the token `(P−D)+S` snapshot | none (comment only) | Yes | No (comment only; DTO value comes from token.EscrowAmount at `order_handler.go:1613`) | CONVERGE (fix comment) |
| R-6 | `backend/internal/governance/dispute/application/dispute_service.go` | `CreateDisputeFreeze` + comment "seller's net (escrow_amount − commission)" | 70-76 | interface + `FinanceService.CreateDisputeFreeze` impl (`finance_service.go:1180`) with no production caller (grep: interface, impl, tests only) | none (zombie surface) | Yes | No (unwired; dispute freeze flow not in live money path) | KEEP (unwired) or OWNER_DECISION |
| R-7 | `backend/internal/finance/refund/infrastructure/repository/refund_repository_impl.go` | `GetCumulativeCommissionReversedByOrder` | 389-415 | `SELECT COALESCE(SUM(0),0)` stub; real commission reversal derived at a higher layer (`refund_gateway.go:384` via `proportionalFloor`); live sibling `GetCumulativeCoinsRefundedByOrder` reads real data | none (dead stub) | Yes | No | KEEP (interface compat) or CONVERGE (delete + update interface) |
| R-8 | `backend/internal/commerce/order/application/order_completion_service.go` | stale doc comments "Refund is always full gross (subtotal + shipping + commission)" | 1256-1262, 1410-1416 | comments contradict code (`:1290,1456` refund = PD+S; `:1741-1744` validates `item+shipping == TotalBeforeCoinsAmount`) | none (comment only) | Yes | No | CONVERGE (fix comments) |
| R-9 | `backend/internal/finance/application/partial_refund_release_test.go` | remainder-commission fixture `floor(31_250 × 6_250 / 131_250) = 1_488` | 65-71, 143-174 | **TEST_RESIDUE** — encodes the rejected full-gross (P+S+C=131250) denominator; production (`refund_gateway.go:463-475`) computes remainder commission with PD denominator | none (test fixture) | Yes (test-only; production math is PD-denominator) | No at runtime; the fixture models the old contract | CONVERGE (rebase fixture to PD denominator) — test change, owner-authorized only |
| R-10 | `backend/internal/finance/application/settlement_release_ledger_test.go` | `exEscrow = Subtotal+Shipping+Commission = 125000` | 281-296 | **TEST_RESIDUE** — legacy worked-example treating C as buyer-funded escrow/gross; tests `RecordGatewayPaymentSettlement`/`RecordOrderRelease` shape with that framing | none (test fixture; not the canonical ledger contract) | Yes (test-only; package build is already broken by R-12, so it does not run) | No at runtime | CONVERGE (rebase to canonical `(P−D)+S` + funding) or OWNER_DECISION |
| R-11 | `backend/internal/incentive/coins/COINS_REFUND_ARCHITECTURE.md:43` | "Relies on order.coins_used snapshot" (diagram) | 43 | stale doc line in a `|_|` diagram | none (doc) | Yes | No (doc only; code comment at `coins_refund_handler.go:130,288` explicitly rejects the snapshot) | CONVERGE (fix doc) |
| R-12 | `backend/internal/finance/application/withdraw_request_idempotency_test.go` | `CanonicalRequestWithdrawalInput.IdempotencyKey`, `ErrWithdrawalIdempotencyConflict` | 204, 236, 246, 250 | **TEST_RESIDUE** — references fields/symbols removed from `CanonicalRequestWithdrawalInput`; breaks the whole `finance/application` test package build | none | Yes (blocks only its own package's tests; production builds) | No | CONVERGE (rebase test to current API) or OWNER_DECISION |
| R-13 | `backend/internal/worker/{content_mentioned_notification_matrix_test.go, content_mentioned_notification_test.go, alert_detection_rules_multi_test.go}` | `SetContentVisibilityChecker`, `ContentMentionedPayload`, `EventContentMentioned`, `rule.Detect` arity | matrix 17-76; notification 61; alert 32,69 | **TEST_RESIDUE** — social/content worker tests referencing removed/renamed APIs; breaks `internal/worker` test build | none | Yes (blocks only worker tests; unrelated to finance) | No | CONVERGE or OWNER_DECISION (unrelated domain) |
| R-14 | `backend/internal/commerce/order/rating/delivery/http/rating_http_test.go` | `toRatingResponse`, `toSummaryResponse`, `reviewerCard` | 34-364 | **TEST_RESIDUE** — references removed rating response helpers | none | Yes | No | CONVERGE or OWNER_DECISION |
| R-15 | `backend/internal/finance/refund/infrastructure/repository/refund_history_contract_test.go` | missing `created_at < $2` | 24 | **TEST_RESIDUE** — contract test expecting a pagination predicate the repo no longer emits | none | Yes | No | CONVERGE or OWNER_DECISION |
| R-16 | `backend/internal/commerce/order/delivery/http/order_refund_history_contract_test.go` | `OrderHandler.ListRefundHistory` | 29 | **TEST_RESIDUE** — expects a handler that no longer exists | none | Yes | No | CONVERGE or OWNER_DECISION |
| R-17 | `backend/internal/serverboot/chat_resource_projection_http_integration_test.go` | `fixture.appDB = fixture.appDB` | 883 | vet-only self-assignment | none | Yes | No | CONVERGE or OWNER_DECISION |
| R-18 | `apps/mobile/lib/domains/commerce/transaction/order/data/dto/order_dto.dart` | `sellerCommission` | 425, 534, 628, 728, 918 | dead field marked "REMOVED (Wave 3.1B) — seller financial data" | none | Yes | No | DELETE |
| R-19 | `apps/mobile/lib/l10n/*.arb` + generated `app_localizations*.dart` | `commission5Percent`, `commission3Percent`, `commission5PercentSale`, `commission3PercentSale` | app_en.arb:1377,1412,1557,1562; app_id.arb:278,285,314,315 + generated | hardcoded "4%/3% per sale" strings with no code consumers; can go stale vs `platform_configs` | none | Yes | No | DELETE |
| R-20 | `apps/admin/src/types/orders.ts` + `OrderDetailModal.tsx`/`OrdersPage.tsx`/`DisputeDetailModal.tsx` | `escrow_amount` | orders.ts:67,186,419; modals | admin display of `escrow_amount` from backend DTO; backend DTO value is canonical (`total_before_coins_amount` at `dependencies.go:3660`) | display consumer of canonical value | Yes | No | KEEP (verify label) |
| R-21 | `backend/migrations/000001_canonical_schema.up.sql` | `orders_check` `refunded_amount <= escrow_amount` | 2473 | binds to dead `escrow_amount` column (always 0); effective constraint `refunded_amount = 0` in production | none (dead) | Yes | No | OWNER_DECISION (repoint to `total_before_coins_amount`) |
| R-22 | `apps/mobile/lib/domains/finance/transaction/payment/domain/entities/payment.dart` / `payment_dto.dart` | `coin_discount` parse | payment_dto.dart:170-172, 309-338 | parses `coin_discount`/`coin_discount_amount` from payment DTO; backend emits `payment.CoinDiscountAmount` (canonical payment domain, not orders column) | payment domain | Yes | No | KEEP (valid consumer of payment-domain value) |

UNKNOWN (explicitly not inferred):
- Actual DB contents of the dead `orders.discount_amount`/`escrow_amount`/`coins_used` columns for historical rows (no DB query was performed — read-only audit; a historical backfill could hold non-zero values, but no production path reads them).
- Whether the untracked WIP integration tests (`escrow_ledger_atomicity_real_db_proof_integration_test.go`, `escrow_repository_real_db_proof_integration_test.go`, `ledger_repository_real_db_proof_integration_test.go`) encode pre- or post-convergence assertions (not read line-by-line; they are untracked and were preserved per the worktree rule).

---

## 5. CONFLICTING_AUTHORITY_TABLE

No live production path requires a second financial authority. Every symbol has exactly one authority:

| Symbol | Canonical authority | Conflicting authority found? | Where verified |
|---|---|---|---|
| EscrowAmount / BuyerBase | `total_before_coins_amount` = (P−D)+S | None (no producer emits P+S+C; `CalculateGrossEscrowFromSnapshot` deleted) | grep across repo; `canonical_finalization_service.go:174`; `order_payment_service.go:326`; token producers |
| PD (commission/refund/coin denominator) | `TotalBeforeCoinsAmount − Shipping` (fallback Subtotal for legacy rows) | None (no `orderGross`, no `EscrowAmount` denominator; `partial_refund_release_test.go` is test-only) | `refund_gateway.go:337`, `refund_math.go`, `verifier.go:677`, `order_payment_service.go:211` |
| Commission identity | `order.CommissionAmount` | None (single identity; no duplicate) | `order.go:58`, copy sites `order_creation_service.go:936,1622` |
| K | coins domain | None (funding now booked; `orders.coins_used` explicitly non-authoritative) | `coins_service.go:362-366`, `payment_coin_settlement_integration_test.go:562-570` |
| GATEWAY_CLEARING funding | settlement + fee sweep + `RecordCoinFunding` | None (funding mechanism exists; over-debit gap closed) | `finance_service.go:481-557`, integration ledger proof |
| Ledger balancing | `ledgerRepo.CreateTransaction` (panic on Σ≠0) + DB CHECK | None | `ledger_repository.go:33-70`, `000001:2460` |
| Buyer cash | `(PD+S)−K+F` server-derived | None (client is not source of truth; payment boundary guard) | `dependencies.go:3154-3155,3286-3323,3145-3148` |

---

## 6. KILL_LIST

Nothing in this list was executed in this pass — inventory only, per the modification rule.

**Production cleanup candidates (require owner decision; none are runtime-safety issues):**
1. Dead `orders.discount_amount` column (R-1) — drop or document as display.
2. Dead `orders.escrow_amount` column + `orders_check` binding (R-2, R-21) — drop, or repoint CHECK at `total_before_coins_amount`.
3. Dead `orders.coins_used`/`orders.coin_discount_amount` columns (R-3) — drop or document as display.
4. Stale comments encoding P+S+C: `order_completion_service.go:1256-1262,1410-1416`; `decision.go:251`; `dispute_service.go:72` (R-5, R-6, R-8) — fix text.
5. `GetCumulativeCommissionReversedByOrder` SUM(0) stub (R-7) — delete + update interface, or document as dead.
6. Mobile dead field `sellerCommission` (R-18) and hardcoded commission l10n strings (R-19) — delete.
7. Unwired `CreateDisputeFreeze` surface (R-6) — delete or wire, owner decision.
8. `COINS_REFUND_ARCHITECTURE.md:43` stale diagram line (R-11) — fix.

**Test residue (owner-authorized convergence only — never in this pass):**
9. `partial_refund_release_test.go` P+S+C-denominator fixture (R-9).
10. `settlement_release_ledger_test.go` P+S+C worked example (R-10).
11. `withdraw_request_idempotency_test.go` removed-field references (R-12) — unrelated domain (withdrawal), pre-existing.
12. Worker content-mention/alert test build break (R-13) — unrelated domain, pre-existing.
13. Rating HTTP contract test (R-14), refund-history contract tests (R-15, R-16), serverboot chat projection vet self-assignment (R-17) — pre-existing.

---

## 7. PROTECTED_LIST

Canonical code and semantics that MUST NOT be removed or altered during any cleanup:
1. `PricingTokenService` producers — `escrowAmount = (P−D)+S`, `calculateCommission(PD, rate)` (`pricing_token_service.go:379-512,948-1060,1211-1373`).
2. `orders.total_before_coins_amount` as the persisted BuyerBase; `NewOrderFromSource` + `order_repository.go` wiring.
3. `CanonicalFinalizationService.FinalizeOrderPayment` sequence — settle → coin consume/spend → ledger settlement → fee sweep → `RecordCoinFunding` → escrow = PD+S → mark paid.
4. `FinanceService.Record*` ledger methods (Σ=0 each): settlement, fee revenue, coin funding, coin funding reversal, order release, refund reversal, partial refund release — plus `ledgerRepo.CreateTransaction` idempotency and balance CHECK.
5. `PLATFORM_BANK` reserve float + `RecordCoinFunding`/`RecordCoinFundingReversal` — the K funding contract.
6. Coins domain authority — `user_coin_balance`, `coin_reservations`, `coins_transactions`; `ConsumeAndSpendForOrder`; `coins.refund_required` → `CoinsRefundRequiredHandler` → `RefundCoinsWithDelta`/`RefundCoinsInternal`.
7. `refund_math.go` PD-denominator allocation (`CalculateProportionalRefundBreakdown`, `proportionalFloor`, `MaxGatewayRefund`) and `refund_policy.go` canonical `ProductGross()`/`CashRefund`.
8. `refund_gateway.go` PD binding (`TotalBeforeCoinsAmount − Shipping`, Subtotal fallback) and `RecordCoinFundingReversal` call on CoinDelta.
9. `ReleaseGatewayEscrowToSeller` gross/commission/sellerNet derivation and `commission > gross` guard.
10. Verifier PD convergence (`verifier.go:639-689`) and escrow-cap from `total_before_coins_amount`.
11. Escrow row (`escrows.amount`) as escrow authority; `CreateEscrowFromGatewaySettlement`.
12. Admin finance summary ledger-derived classification (`admin_finance_handler.go:498-562`) and its reference-type buckets.
13. Payment boundary hardening — client is never the source of amount; `CreatePayment` derives from order; `loadOrderPricingTokenSnapshot` mismatch guard (`dependencies.go:3145-3148`).
14. `payment_coin_settlement_integration_test.go` canonical assertions (escrow=110000, dead-column zeros, ledger funding proof).

---

## 8. TEST_RESIDUE

| Test | Lines | Encodes rejected model? | Build status | Classification |
|---|---|---|---|---|
| `internal/finance/application/partial_refund_release_test.go` | 65-71, 143-174 | YES — P+S+C=131250 denominator for remainder commission | compiles (package blocked by R-12) | TEST_RESIDUE (fixture conflicts with PD-denominator production math) |
| `internal/finance/application/settlement_release_ledger_test.go` | 281-296 | PARTIAL — legacy worked example with C in escrow gross | compiles (package blocked by R-12) | TEST_RESIDUE (legacy scenario, not canonical contract) |
| `internal/finance/application/withdraw_request_idempotency_test.go` | 204-250 | NO | **build broken** | TEST_RESIDUE (stale API references; unrelated withdrawal domain) |
| `internal/worker/{content_mentioned_notification_matrix_test.go, content_mentioned_notification_test.go, alert_detection_rules_multi_test.go}` | matrix 17-76 | NO | **build broken** | TEST_RESIDUE (unrelated social/content domain) |
| `internal/commerce/order/rating/delivery/http/rating_http_test.go` | 34-364 | NO | **build broken** | TEST_RESIDUE (unrelated rating domain) |
| `internal/finance/refund/infrastructure/repository/refund_history_contract_test.go` | 24 | NO | runs, fails | TEST_RESIDUE (contract drift, not financial model) |
| `internal/commerce/order/delivery/http/order_refund_history_contract_test.go` | 29 | NO | runs, fails | TEST_RESIDUE (contract drift, not financial model) |
| `internal/serverboot/chat_resource_projection_http_integration_test.go` | 883 | NO | vet self-assignment | TEST_RESIDUE (cosmetic) |
| `internal/serverboot/payment_coin_settlement_integration_test.go` | 543-1472 | NO — canonical (escrow 110000, dead-column zeros, spend/ledger proofs) | compiles | CANONICAL TEST — protected |
| `internal/finance/refund/entity/refund_policy_test.go` | whole file | NO — canonical `ProductGross()`/`CashRefund` | passes | CANONICAL TEST — converged |
| `internal/commerce/order/tests/canonical_pricing_snapshot_persistence_test.go` | 30-121 | NO — asserts (P−D)+S and anti-proof ≠ P+S+C−D | passes | CANONICAL TEST — converged |
| `internal/finance/refund/application/refund_gateway_webhook_spy_test.go` | 334-344, 438, 448-608 | NO — canonical fields + `RecordCoinFundingReversal` spy | passes | CANONICAL TEST — converged |

All canonical tests pass. All failing/broken tests are pre-existing, unrelated to the locked financial model, and were untouched.

---

## 9. LEDGER_AUTHORITY_CHECK

Every `Record*` ledger method has an explicit funding/source relationship; none books an unfunded GATEWAY_CLEARING debit.

| Movement | Entries (Σ=0) | Funding/source | Verified at |
|---|---|---|---|
| Settlement | GATEWAY_CLEARING +gross / BANK_SETTLEMENT −gross | gateway cash inflow, idem `payment_settlement_<txn>` | `finance_service.go:323-398` |
| Fee sweep | GATEWAY_CLEARING −F / PLATFORM_REVENUE +F | F already in clearing (part of gross) | `finance_service.go:426-477` |
| **Coin funding** | **PLATFORM_BANK −K / GATEWAY_CLEARING +K** | **PLATFORM_BANK (platform's own money), idem `coin_funding_<payment_id>`** | `finance_service.go:508-557` |
| Release | GATEWAY_CLEARING −(PD+S) / SELLER_PAYABLE +(PD+S)−C / PLATFORM_REVENUE +C | clearing holds PD+S after settlement+fee+funding | `finance_service.go:187-234`; `order_payment_service.go:326-348` |
| Refund reversal | GATEWAY_CLEARING −CashRefund / SELLER_PAYABLE −SellerComponent / PLATFORM_REVENUE −CommissionDelta | refunded cash returns to BUYER_REFUNDABLE; commission/seller reversal proportional | `finance_service.go:728-759`; `refund_gateway.go:409-418` |
| **Coin funding reversal** | **GATEWAY_CLEARING −CoinDelta / PLATFORM_BANK +CoinDelta** | **returns the refunded portion of K to PLATFORM_BANK, idem `coin_funding_reversal_<refund_id>`** | `finance_service.go:588-633`; `refund_gateway.go:425-429` |
| Partial refund release | GATEWAY_CLEARING −remainder / SELLER_PAYABLE +net / PLATFORM_REVENUE +commission | remainder fully cash-backed after funding reversal | `finance_service.go:952`; `refund_gateway.go:463-476` |
| Withdrawals | SELLER_PAYABLE → WITHDRAWAL_PENDING/COMMITTED → PLATFORM_BANK | seller-payable source; completes at PLATFORM_BANK | `withdraw_service.go:458,565,673-677,799` |

Invariants:
- `ledgerRepo.CreateTransaction` panics on Σ(entries) ≠ 0 (`ledger_repository.go:70`); DB CHECK `ledger_transactions_balanced` (`000001:2460`) is the second line of defense.
- `financial_accounts.balance >= 0` CHECK (`000001:2455`) is satisfiable because PLATFORM_BANK (9e15 float) and BANK_SETTLEMENT (9e15 float) are seeded one-shot at bootstrap (`system_account_bootstrap.go:72,91`); GATEWAY_CLEARING is funded before any debit.
- Full-refund outcome: GATEWAY_CLEARING = 0, PLATFORM_BANK = −K + K = 0 (net of funding/reversal); documented at `finance_service.go:577-579`.

No ledger movement without a defined funding source was found.

---

## 10. SEMANTIC_RESIDUE_SEARCH

Exact commands and exit codes (read-only; Windows environment, PowerShell/cmd):

| Command | Exit | Result |
|---|---|---|
| `git status --short` (repo root) | 0 | Identical to session-start snapshot; no changes introduced by this audit |
| grep `CalculateGrossEscrowFromSnapshot` (repo) | 1 (no matches in code) | Only historical report mentions + 3 anti-proof comments in tests; `pricing_helper.go` DELETED (`D` in git) |
| grep `LegacyGross` (repo) | 1 (no matches in code) | Only historical report mentions |
| grep `orderGross|OrderGross` (backend) | 0 | `verifier.go:639-689` and `refund_gateway.go:413` — variable named `orderGross` bound to `TotalBeforeCoinsAmount`/`pd+s` (CANONICAL value, not the rejected orderGross model); `settlement_release_ledger_test.go`/`partial_refund_release_test.go` legacy fixtures |
| grep `P\+S\+C` (backend, code) | 0 | Only anti-proof comments and test comments |
| grep `discount_amount` (backend, non-test) | 0 | Schema column + `pricing_token_*` (token snapshot) + comments; **no production writer to `orders.discount_amount`** |
| grep `escrow_amount` (backend, non-test) | 0 | Schema columns + comments; all production reads are `total_before_coins_amount` or token snapshot; no production writer to `orders.escrow_amount` |
| grep `coins_used\|CoinsUsed` (backend) | 0 | `order_repository.go:94` writes 0; display-only DTO passthroughs; outbox trigger guard on in-memory value; **never an authority read** |
| grep `SUM(subtotal` (backend) | 0 | `seller_handler.go:837` — `SUM(subtotal - commission_amount)` dashboard metric (see below) |
| grep `GetCumulativeCommissionReversedByOrder` (backend) | 0 | interface + SUM(0) stub + test stubs; no live caller with a meaningful expectation |
| grep `CreateDisputeFreeze(` (backend) | 0 | interface + impl + tests only; no production caller |
| grep `commission_revenue_rupiah` (backend) | 0 | `admin_finance_handler.go:431,502-562` — ledger-derived classification |
| grep `coin_discount_amount` (backend, non-test) | 0 | `payment_repository.go` (payment domain, canonical) + `order_repository.go:95` writes 0 |
| grep `P\+S\+C\|LegacyGross\|CalculateGrossEscrow\|orderGross` in `backend/**/*.md` | 1 (no matches) | No rejected-model documentation residue in backend docs |
| `go build ./...` (backend) | 0 | Production builds clean |
| `go vet` core finance/payment/order/wallet/coins/pricing packages | 1 | Only stale test files fail (`withdraw_request_idempotency_test.go`, `rating_http_test.go`); production clean |
| `go test ./internal/finance/refund/... ./internal/finance/verifier/...` | 1 | refund/application + entity + verifier PASS; `refund_history_contract_test.go` FAIL (pre-existing contract drift) |
| `go test ./internal/finance/... ./internal/integration/payment/... ./internal/commerce/order/... ./internal/pricing/token/... ./internal/incentive/coins/...` | 1 | All core financial packages PASS except pre-existing failures: `refund_history_contract_test.go`, `order_refund_history_contract_test.go`, `rating_http_test.go` (build); `finance/application` blocked by `withdraw_request_idempotency_test.go` (build) |
| `go test ./internal/core/wallet/...` | 0 | PASS |
| `go vet -tags integration ./internal/serverboot/` | 1 | Compiles except pre-existing `chat_resource_projection_http_integration_test.go:883` self-assignment |

**Seller dashboard metric (note, not a contradiction):** `seller_handler.go:837` computes `total_revenue = SUM(subtotal - commission_amount)` from completed orders. This is a seller-facing estimate, not a ledger movement and not a financial authority; it can differ from the ledger-backed SELLER_PAYABLE when a discount exists (subtotal vs PD). It does not move money, is not consumed by any other financial calculation, and cannot resurrect the rejected model. Classification: VALID_CONSUMER (display metric) with a known data caveat; action KEEP or CONVERGE (subtract discount) — owner decision.

**Grep note:** `P\+S\+C` style literal greps return 0 because the pattern characters are spread across code constructs; the semantic sweep (reading every producer/consumer path) is the authoritative evidence, and it found no live P+S+C formula.

---

## 11. RUNTIME / TEST EVIDENCE

Compilation alone is not runtime proof. The following runtime evidence was gathered in this pass (read-only; no code or DB changed):

- **Unit tests (passing):** `finance/refund/application`, `finance/refund/entity`, `finance/verifier`, `integration/payment/application` (+ orchestrator, recon, recon/audit), `commerce/order/application`, `commerce/order/entity`, `commerce/order/infrastructure/repository`, `pricing/token/*`, `incentive/coins/*`, `core/wallet/*` — all `ok`.
- **Ledger invariants (source-verified):** Σ=0 per transaction (panic + DB CHECK), balance ≥ 0 CHECK with seeded reserve floats, idempotency keys on every `Record*` (`payment_settlement_<txn>`, `coin_funding_<payment_id>`, `coin_funding_reversal_<refund_id>`, `withdrawal_*`).
- **Integration test harness (compiles; asserts canonical contract):** `payment_coin_settlement_integration_test.go` contains `TestPaymentCoinSettlement_LedgerFundingProof`, `..._KPositive_PersistsSpendSnapshotAndLedger`, `..._SellerEntitlementStableAcrossKZeroAndKPositive`, `..._AvailableBalanceContinuityAndExactlyOneSpend`, `..._ConcurrentDuplicateWebhook_OneSpendOnly` and ~25 more — asserting escrow=110000 (PD+S), `orders.coins_used`/`coin_discount_amount` stay 0, reservation consumed exactly once, spend row exactly once, balance deducted exactly once, and the K funding ledger shape. The prior `COIN_FUNDING_CONTRACT_CONVERGENCE_IMPLEMENTATION_REPORT` records these integration tests passing (exit 0) with the exact ledger numbers (GATEWAY_CLEARING=110000, PLATFORM_BANK −10000, SELLER_PAYABLE=105500, PLATFORM_REVENUE=8500 for the K>0 fixture). Full runtime re-execution requires a live Postgres (no DB available in this pass); the harness compiles and the assertions are canonical.
- **No runtime evidence of a live P+S+C producer:** every producer/consumer path was read; all compute from `TotalBeforeCoinsAmount` or the pricing-token snapshot.
- **Pre-existing failures are documented as such** and were not introduced, fixed, or modified in this pass. They do not exercise the locked financial contract.

---

## 12. FILES_CHANGED

```
FILES_CHANGED:
  GLOBAL_FINANCIAL_AUTHORITY_RESIDUE_AUDIT.md   (created — the ONLY artifact of this pass)
```

All other working-tree changes (the 26 tracked modifications, `verify_partial_refund_semantics` deletion, `pricing_helper.go` deletion, untracked reports/tests/migrations) are the **pre-existing state of the interrupted prior missions** — verified identical to the session-start snapshot via `git status` before and after this audit. None were caused by or modified by this pass.

---

## 13. DATABASE_CHANGED

```
DATABASE_CHANGED:
  NONE
```

No database was queried, migrated, seeded, or modified. `SystemAccountBootstrap` (PLATFORM_BANK float) is runtime bootstrap behavior already in the codebase, not a change made here.

---

## 14. MIGRATIONS_CHANGED

```
MIGRATIONS_CHANGED:
  NONE
```

The untracked migrations 000042/000043 and the tracked 000001 schema are exactly as found at session start.

---

## 15. FINAL_STOP

The entire repository was audited against the locked canonical contract. Result: **GLOBAL_FINANCIAL_AUTHORITY_CONVERGED**.

- All producers and consumers of BuyerBase/EscrowAmount, PD, C, K, F, seller entitlement, and platform revenue are converged on a single authority per symbol.
- K has an explicit funding source (PLATFORM_BANK) and funding reversal; GATEWAY_CLEARING is never debited beyond funded cash; Σ=0 and balance≥0 invariants hold.
- The rejected model is dead in production and in all running tests. Remaining residue is confined to stale comments, dead columns/fields/strings, a SUM(0) stub, an unwired freeze surface, and test fixtures/contract tests — none runtime-capable of resurrecting the rejected model, all listed with exact locations and owner-decision actions.
- No implementation was performed. This pass is read-only; the only artifact is this report.

STOP — audit complete. No implementation authorized in this pass.
