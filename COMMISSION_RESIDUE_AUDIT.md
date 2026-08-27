# COMMISSION RESIDUE AUDIT

READ-ONLY AUDIT — DISCOVERY AND CLASSIFICATION ONLY.
No production code, tests, migrations, schema, ledger, refund, accounting, payment, escrow, or verifier behavior was modified.
The ONLY artifact created is this report.

Reference standard (LOCKED, not reopened):
- `order.CommissionAmount` is the single canonical commission identity (immutable order snapshot).
- `CommissionAmount = floor(PD × rate / 100)`, `PD = Subtotal − Discount`.
- Commission is product-only; shipping has no commission.
- `CommissionDelta` is a derived per-event allocation from the canonical CommissionAmount.
- Refund math uses PD as the denominator.
- The verifier has been converged to the same PD denominator (see COMMISSION_VERIFIER_CONVERGENCE_REPORT.md; working-tree `verifier.go` is the converged state).
- EscrowAmount / EscrowGross is NOT a commission authority and must NOT be used as the commission denominator.
- Escrow semantics remain separate from commission identity.

---

# VERDICT

**COMMISSION_RESIDUE_FOUND**

The previously-identified verifier contradiction is gone (converged to PD). However, this audit found **two live runtime divergences that can still produce commission values inconsistent with the canonical PD authority**, plus a set of stale comments, dead/zombie code, stale test fixtures, and documentation residue.

**LIVE RUNTIME FINDINGS (highest priority):**

1. **`refund_gateway.go:326` — `pd := order.Subtotal.Int64()` (no discount subtraction).** The ack-time refund pipeline computes PD without subtracting Discount. Combined with finding #2, this makes the canonical refund denominator silently equal Subtotal whenever a discount exists.
2. **Production never persists `orders.discount_amount`.** `OrderRepository.CreateOrderTx` (`order_repository.go:56-96`) omits `discount_amount` from the INSERT, and `NewOrderFromSource` (`order.go:1086-1127`) has no discount parameter. The column stays `0` for production orders. The verifier (`verifier.go:1029`) reads `discount_amount` to compute PD — so in production the verifier's PD is always `Subtotal`, i.e. the exact old-model behavior the convergence was meant to eliminate. (This gap was previously documented in `SCOPE_4A_AUDIT_REPORT.md:63-66,600`.)
3. **`OrderRepository.CreateOrderTx` writes `total_before_coins_amount = order.TotalPayableAmount`** (`order_repository.go:96`), and `NewOrderFromSource` sets `TotalBeforeCoinsAmount = totalPayableAmount` (`order.go:1103`). The canonical definition is `TotalBeforeCoinsAmount = Subtotal + Shipping − Discount + Commission` (`pricing_token_service.go:404-415`; canonical test `canonical_pricing_snapshot_persistence_test.go:108-120` asserts `(P−D)+S`). This diverges when Discount > 0.
4. **`orders.escrow_amount` is never written by production.** `CreateOrderTx` omits it (DB default 0), yet it is read by: the verifier escrow-cap checks (`verifier.go:640-666`), the escrow integrity checker (`escrow_integrity_checker.go:234,266`), the recon D15 classifier (`classifier.go:444`), the recon resolver (`recon/audit/resolver.go:197`), the projection (`projection_worker.go:785`), and admin/mobile DTOs. The escrow row (`escrows.amount`) is the real authority; `orders.escrow_amount` is a shadow column that is 0 in production.
5. **`partial_refund_release_test.go` encodes a commission-on-shipping denominator model.** It computes remainder commission as `floor(31250 × 6250 / 131250)` = 1488 where the denominator is the full gross `131250 = Subtotal + Shipping + Commission` (`partial_refund_release_test.go:65-71,152-154`). This is the rejected EscrowAmount/Gross-denominator model, expressed in a test fixture. The production partial-release path (`refund_gateway.go:426-434`) uses the canonical `proportionalFloor(remProduct, cVal, pd)` (PD denominator) — so this test does not match production math.
6. **`seller_handler.go:837` — `SUM(subtotal - commission_amount)`** computes seller "total_revenue" directly from the order row without subtracting Discount. This is a live SQL aggregate that can diverge from the canonical seller economics (which is ledger-backed SELLER_PAYABLE) whenever Discount > 0.

All other findings are derived consumers, benign duplicates, stale comments/docs, or test-only residue (classified below).

---

# CANONICAL_AUTHORITY

| # | Symbol / function | File:lines | Value/formula | Denominator | Agrees with PD authority? | Runtime influence |
|---|---|---|---|---|---|---|
| CA-1 | `platform_configs` `listing_commission_percent` / `auction_commission_percent` | `config_service.go:30-59` (`KeyListingCommissionPercent`, `KeyAuctionCommissionPercent`, `GetListingCommission`, `GetAuctionCommission`, aliases `GetOrderListingCommission`/`GetOrderAuctionCommission`); seed `000001_canonical_schema.up.sql:2531-2532` (default 4) | the rate | — | — | YES (input authority) |
| CA-2 | `calculateCommission(subtotal, percent)` | `pricing_token_service.go:679-682` | `subtotal.Int64() * percent.IntPart() / 100` (floor) | — | YES (primitive) | YES |
| CA-3 | fixed-price commission computation | `pricing_token_service.go:379-384` | `netSubtotal = subtotal − discountAmount`; `commissionAmount = calculateCommission(netSubtotal, commissionPercent)` | PD | YES | YES |
| CA-4 | negotiation commission computation | `pricing_token_service.go:948-949` | `calculateCommission(subtotal, percent)` (no discounts by design) | Subtotal (=PD) | YES | YES |
| CA-5 | auction commission computation | `pricing_token_service.go:1243-1248` | `netSubtotal = subtotal − discountAmount`; `calculateCommission(netSubtotal, auctionPercent)` | PD | YES | YES |
| CA-6 | pricing-token `CommissionPercent` / `CommissionAmount` snapshot | `pricing_token.go:50-51`; repo `pricing_token_repository_impl.go:83,206-207,323-324`; handler `pricing_token_handler.go:381-382` | stored snapshot from CA-3/4/5 | — | YES | YES (snapshot source) |
| CA-7 | `order.CommissionAmount` (immutable identity) | `order.go:58-59`; copied at creation `order_creation_service.go:936,1622`; persisted `order_repository.go:91`; read `order_repository.go:184+` | verbatim copy of token value | — | YES | YES |
| CA-8 | `CalculateProportionalRefundBreakdown` (canonical refund allocation) | `refund_math.go:35-139` | `CommissionDelta = floor(C×cumProductAfter/PD) − floor(C×cumProductBefore/PD)`; `SellerComponent = Rpd + Rs − CommissionDelta` | PD (validated > 0, `:39-40`) | YES | YES (refund ack) |
| CA-9 | `proportionalFloor` primitive | `refund_math.go:141-146` | `(amount × numerator) / denominator` floor | PD (caller) | YES | YES |
| CA-10 | refund-gateway PD binding (partial-release remainder commission) | `refund_gateway.go:367,432` | `cumCommissionBefore = proportionalFloor(cumProductBefore, cVal, pd)`; `remCommission = proportionalFloor(remProduct, cVal, pd)` | pd | YES (formula) / **NO (binding, see LIVE-1)** | YES |
| CA-11 | verifier converged commission expectation | `verifier.go:669-683,735-746` (`verifierProportionalCommissionPD`) | `(amount × orderCommission) / pd`, `pd = Subtotal − Discount` | PD | YES (formula) / **NO (data, see LIVE-2)** | YES (offline forensic check) |
| CA-12 | ledger postings: `RecordOrderRelease`, `RecordPartialRefundRelease`, `RecordRefundReversal` | `finance_service.go:187-234, 570-759, 794-866` | DR GATEWAY_CLEARING −gross; CR SELLER_PAYABLE +sellerNet; CR PLATFORM_REVENUE +commission; reversal shape with CommissionComponent | commission value passed in (derived from canonical snapshot) | YES | YES (money movement) |

---

# DERIVED_VALUES

| # | Symbol / function | File:lines | Value/formula | Denominator | Agrees with PD authority? | Runtime influence |
|---|---|---|---|---|---|---|
| DV-1 | `CalculateGrossEscrowFromSnapshot` | `finance/application/pricing_helper.go:30-31` | `Subtotal + Shipping + Commission` | — | Not a commission calc (escrow display helper, explicitly NON-AUTHORITATIVE `:8-27`) | YES (escrow creation `canonical_finalization_service.go:117-126`; dispute validation `order_completion_service.go:1739`; tests) |
| DV-2 | `ReleaseSummary` gross/commission/sellerNet | `order_payment_service.go:257-294` | `sellerNet = Subtotal + Shipping`; `gross = sellerNet + commission`; commission = `order.CommissionAmount` | — | YES (reads canonical snapshot) | YES (release) |
| DV-3 | `CommissionDelta` field | `refund_math.go:30,135`; `refund_gateway.go:394-397` | derived per-event from CA-8 | PD | YES | YES (ack) |
| DV-4 | `SellerComponent` | `refund_math.go:31,118` | `Rpd + Rs − CommissionDelta` | PD | YES | YES (ack) |
| DV-5 | `OrderSnapshot.ProductGross()` | `refund_policy.go:35` | `Subtotal + Shipping` | — | Not commission | YES (refund policy `refund_service.go:451`) |
| DV-6 | `RefundPolicyResult.CashRefund` | `refund_policy.go:26,43-58` | `Rpd + Rs` (C excluded) | — | YES (C never in buyer refund) | YES |
| DV-7 | admin finance `commission_revenue_rupiah` classification | `admin_finance_handler.go:502-506,522-562` | sums ledger entries by reference_type (`order_release`, `partial_refund_release`, `refund_reversal`) | — | YES (derived from ledger, not order row) | YES (admin display) |
| DV-8 | `order.EscrowAmount` read paths (verifier caps, escrow checker, recon, projection, DTOs) | `verifier.go:640-666`; `escrow_integrity_checker.go:234,266`; `classifier.go:444`; `recon/audit/resolver.go:197`; `projection_worker.go:785`; `admin_order_handler.go`; `decision.go`; `notification_worker_shared.go:77`; mobile `order_dto`/`checkout_response` | reads `orders.escrow_amount` column | — | Not a commission authority (escrow semantics) | **YES but reads a column production never writes (LIVE-4)** |
| DV-9 | `money.released` outbox payload | `events.go:30-33`; `outbox_event_registry.go:69-72`; `order_completion_release_from_dispute_test.go:28,144` | carries gross/commission/sellerNet | — | YES (audit-only) | NO side-effect (audit only) |

---

# CANONICAL_CONSUMERS

| # | Consumer | File:lines | What it does | Agrees? |
|---|---|---|---|---|
| CC-1 | `order_creation_service.go:936,1622` | order creation (auction + sale) | copies `snapshot.CommissionAmount` → order | YES |
| CC-2 | `order_repository.go:91` / `:203,255` | persistence + hydration | writes/reads `orders.commission_amount` | YES |
| CC-3 | `pricing_token_repository_impl.go:83,207,324` | token persistence | writes/reads token commission | YES |
| CC-4 | `serverboot/dependencies.go:3312-3319` | CreatePayment boundary guard | `baseAmount < order.CommissionAmount` → error | YES (defensive guard) |
| CC-5 | `order_payment_service.go:268,270,281` | release | commission = `order.CommissionAmount`; posts to ledger | YES |
| CC-6 | `refund_gateway.go:328,367,432` | refund ack | `cVal := order.CommissionAmount.Int64()` as the C in canonical math | YES (formula) |
| CC-7 | `refund_service.go:420` | seller-approve policy | `CommissionAmount: order.CommissionAmount.Int64()` into OrderSnapshot | YES |
| CC-8 | `verifier.go:65,681-683,741-746,1029,1044` | offline verifier | loads `commission_amount` + `discount_amount`; computes expected via PD | YES (formula; data caveat LIVE-2) |
| CC-9 | `decision.go:288,726`; `admin_order_handler.go:174,407`; `order_query_service.go:92,325,531,636`; `notification_worker_shared.go:76`; `order_handler.go:1612`; `auction_handler.go:885`; `chat_handler.go:2012` | DTO/projection/notification serialization | passthrough of `order.CommissionAmount`/`token.CommissionAmount` | YES |
| CC-10 | `projection_worker.go:452,530`; `projection/repository.go:77,152,189,402,481,709` | projection read model | passthrough | YES |
| CC-11 | `order_completion_service.go:1736,1739-1744` | partial-dispute validation | `commissionFee = order.CommissionAmount`; validates `P+S+C == escrow` | YES (formula; relies on escrow column LIVE-4) |
| CC-12 | `verify_partial_refund_semantics/main.go` (cmd tool) | proof tool | uses canonical `CalculateProportionalRefundBreakdown` | YES |
| CC-13 | admin `OrderDetailModal.tsx:304-306`; `types/orders.ts:183` | admin display | shows `commission_amount` | YES |
| CC-14 | mobile `checkout_repository_impl.dart:235-236`; `checkout_response.dart:28,44`; `pricing_breakdown.dart:126-131`; `pricing_snapshot.dart:17-18,46-47`; `pricing_preview_dto.dart:55-57` | mobile display | parses/displays commission_amount/percent from backend | YES |
| CC-15 | mobile `order_pricing_cards.dart:104,149`; `order_pricing.dart:17`; `order_mapper.dart:171` | mobile order pricing | sellerCommission/sellerEarnings REMOVED (display-only doc) | YES (no formula) |
| CC-16 | `canonical_discount_snapshot_real_db_integration_test.go:196`; `order_domain_test.go:388-393`; `shipping_proof_test.go:280-281,318-319`; `refund_gateway_webhook_spy_test.go:334-344,438,608` | tests | assert canonical snapshot/escrow math | YES |

---

# DUPLICATE_AUTHORITIES

| # | Item | File:lines | Detail | Residue risk |
|---|---|---|---|---|
| DU-1 | `OrderSnapshot.LegacyGross()` | `refund_policy.go:36` | `Subtotal + Shipping + Commission` — duplicate of `CalculateGrossEscrowFromSnapshot` under a legacy name. **No production consumer.** Only `refund_policy_test.go` references `Gross()` (stale name) — the actual `LegacyGross()` has NO callers at all (grep: only definition + docs). | Test-only zombie; KILL candidate |
| DU-2 | Two release posting functions with identical shape | `finance_service.go:187-234` (`RecordOrderRelease`) and `:794-866` (`RecordPartialRefundRelease`) | same DR GC / CR SP / CR PR shape | benign duplicate, both canonical consumers |
| DU-3 | `GetOrderListingCommission` / `GetOrderAuctionCommission` aliases | `config_service.go:50-59` | pure aliases of `GetListingCommission`/`GetAuctionCommission` | benign |
| DU-4 | Duplicate canonical formula doc comments | `refund_math.go:5-22` and `refund_gateway.go:1-10` | same formula restated twice | benign, consistent |
| DU-5 | `GetCumulativeCommissionReversedByOrder` | `refund_repository_impl.go:389-415` | returns `SUM(0)` = always 0 with a doc explaining the value must be derived from cumulative product + C/PD at a higher layer | dead-ish stub; harmless but misleading |

---

# CONFLICTING_AUTHORITIES

| # | Item | File:lines | Formula / value | Denominator | Agrees with canonical PD? | Runtime influence |
|---|---|---|---|---|---|---|
| CF-1 | `refund_gateway.go:326` `pd := order.Subtotal.Int64()` | refund ack path | PD binding for the ENTIRE ack-time breakdown (CA-8 call at `:369-372`) | Subtotal (NO discount subtraction) | **NO when Discount > 0** | **YES — live refund pipeline** |
| CF-2 | `order_repository.go:96` `total_before_coins_amount = order.TotalPayableAmount` + `order.go:1103` `TotalBeforeCoinsAmount = totalPayableAmount` | order creation/persistence | canonical is `Subtotal + Shipping − Discount + Commission` (i.e. `(P−D)+S` after fee carve); written value is `P+S+C+F` shape | — | **NO when Discount > 0** | YES (persisted snapshot, display) |
| CF-3 | `seller_handler.go:837` `SUM(subtotal - commission_amount)` | seller dashboard "total_revenue" | revenue computed from order row, no Discount subtraction | Subtotal (undiscounted) | **NO when Discount > 0** (also ignores ledger authority) | YES (dashboard metric) |
| CF-4 | verifier PD data dependency | `verifier.go:1029` reads `discount_amount`; production never writes it (LIVE-2) | `pd = Subtotal − 0 = Subtotal` in production | Subtotal in production data | **NO in production data** | YES (offline forensic check flags the wrong thing) |
| CF-5 | `partial_refund_release_test.go:65-71,152-154` | test fixture | remainder commission `floor(31250 × 6250 / 131250)` = 1488 | **full gross 131250 = P+S+C** | **NO** — encodes the rejected EscrowAmount/gross-denominator model | Test-only, but encodes old model |
| CF-6 | stale doc comments on `RefundOrder`/`RefundFromDispute` | `order_completion_service.go:1243-1246,1403-1406` | "Refund is always full gross (subtotal + shipping + commission)" | — | **NO (comment contradicts code at `:1290,1456` which uses `Subtotal + Shipping`)** | Comments only; code is canonical |

---

# LEGACY_ZOMBIE_INCORRECT

| # | Item | File:lines | Evidence | KILL? |
|---|---|---|---|---|
| LZ-1 | `OrderSnapshot.LegacyGross()` | `refund_policy.go:36` | named "Legacy"; zero production consumers; duplicate of DV-1 | YES |
| LZ-2 | `refund_policy_test.go` (whole file) | `refund_policy_test.go:1-268` | stale — references `OrderSnapshot.Gross()`, `RefundPolicyResult.Amount`, `NewRefund`, `r.SellerApprove(policy.Amount…)` that no longer exist in `refund_policy.go` (fields renamed to `ProductGross`/`CashRefund` in S2C2 rebase). Does NOT compile against current code (documented in COMMISSION_VERIFIER_CONVERGENCE_REPORT.md §REGRESSION_RESULTS #2). Encodes the old gross model. | YES |
| LZ-3 | `GetCumulativeCommissionReversedByOrder` | `refund_repository_impl.go:389-415` | returns `SUM(0)`; self-documents as non-functional; no live caller found that depends on its value | YES (or document as dead) |
| LZ-4 | `DisputeFreezeAuthority.CreateDisputeFreeze` interface + `FinanceService.CreateDisputeFreeze` | `dispute_service.go:69-76`; `finance_service.go:1009-1060` | **no production caller** (grep: only interface, impl, and tests). Comment at `dispute_service.go:72` says "frozenAmount is the seller's net (escrow_amount − commission)" — an escrow-derived commission subtraction with no live wiring. Zombie surface. | YES |
| LZ-5 | `money.refunded` / `money.partial_refund` / `money.partial_release` outbox events | `outbox_event_registry.go:83-94` | registered as NoHandlerAuditOnly; "dead handler+setup deleted (B90); event consumed without side-effect" | Not commission-relevant (audit-only) |
| LZ-6 | `order.dispute_refund_initiated` / `order.dispute_partial_refund_initiated` | `outbox_event_registry.go:171-178` | "legacy/parked dispute refund audit trail; not emitted by current runtime" | Not commission-relevant |
| LZ-7 | mobile l10n `commission5Percent`/`commission3Percent`/`commission5PercentSale`/`commission3PercentSale` | `app_en.arb:1377,1412,1557,1562`; `app_id.arb:278,285,314,315` + generated files | "Commission: 4%/3% per sale" strings with **no code consumers** (grep `l10n.commission|commission5Percent|commission3Percent` in lib/ → 0 hits). Hardcoded rates that can go stale vs platform_configs. | YES |
| LZ-8 | mobile analytics docs `platformCommission` | `apps/mobile/lib/domains/system/analytics/docs/README.md:156` | doc-only field in a design sketch | YES (doc) |
| LZ-9 | mobile `order_dto.dart` `sellerCommission` field | `order_dto.dart:425,534,628,728,918` | parsed + serialized but commented "REMOVED (Wave 3.1B) - seller financial data"; `order_mapper.dart:171` confirms removal. Residual dead field. | YES |
| LZ-10 | `config.go:30,399` "CommissionPercent removed" comments | `config.go:30,399` | accurate historical note | keep (informative) |
| LZ-11 | `SCOPE_4A_AUDIT_REPORT.md` / `SCOPE_4B_*` reports | repo root | historical reports; `SCOPE_4A_AUDIT_REPORT.md:63-66,600` correctly documents the discount-persistence gap | informational |

---

# UNKNOWN

| # | Item | File:lines | Why unknown |
|---|---|---|---|
| UN-1 | Whether any production path writes `orders.escrow_amount` or `orders.discount_amount` outside the greps performed (e.g., seed scripts, dev tools, admin SQL tooling) | `cmd/seed`, `cmd/dev-reset-data`, ops scripts | Not exhaustively verified; grep of `UPDATE orders`/`INSERT INTO orders` shows no production writer, but seed/dev tools were not read line-by-line. Marked UNKNOWN per instruction "do not infer". |
| UN-2 | Actual DB state of `orders.escrow_amount` / `orders.discount_amount` for live orders | (database, not code) | Read-only audit of code; no DB query performed. If a migration/backfill wrote these historically, the columns may hold non-zero values for old orders. |
| UN-3 | Whether `refund_repository_impl.go` `GetCumulativeCommissionReversedByOrder` (SUM(0) stub) is called by any live path with a meaningful expectation | grep found no callers outside the file | Not proven either way; classified zombie based on absence of callers, but no exhaustive call-graph run. |
| UN-4 | The `money.partial_release` / `money.partial_refund` emitters still present somewhere | registry only | Emitters not located; registry says dead. Not commission-critical. |

---

# KILL_LIST

Everything below is proven obsolete, conflicting, legacy, or capable of resurrecting the rejected commission model. **Nothing here is to be executed in this pass — list only.**

**Runtime-affecting (must be resolved before any cleanup):**
1. `refund_gateway.go:326` — replace `pd := order.Subtotal.Int64()` with canonical `pd = Subtotal − Discount` (requires the order to carry discount, see #2). Currently makes ack-time PD = Subtotal.
2. Discount persistence gap — `NewOrderFromSource` (`order.go:1032-1127`) has no discount parameter; `CreateOrderTx` (`order_repository.go:56-96`) omits `discount_amount`. Production orders have `discount_amount = 0`, so the verifier's PD (`verifier.go:676`) and any PD consumer silently use Subtotal. This is the single most dangerous residue: it re-creates the old-model denominator in production data.
3. `TotalBeforeCoinsAmount` wiring — `order.go:1103` + `order_repository.go:96` write `total_payable` instead of canonical `(P−D)+S`; diverges when Discount > 0.
4. `orders.escrow_amount` shadow column — never written by production (`order_repository.go` INSERT omits it) but read by verifier (`:640-666`), escrow checker (`:234,266`), recon (`classifier.go:444`, `resolver.go:197`), projection (`projection_worker.go:785`), and DTOs. Either write it at creation from the pricing snapshot, or stop reading it (escrow row is the authority). This is escrow semantics — NOT a commission identity — but it feeds commission-adjacent checks.
5. `seller_handler.go:837` — `SUM(subtotal - commission_amount)` seller revenue; must subtract Discount and/or be ledger-backed.

**Formula/comment conflicts:**
6. `partial_refund_release_test.go:65-71,152-154` — remainder-commission fixture uses the rejected full-gross (P+S+C) denominator; rewrite to the canonical PD denominator (this is a test fix — listed only, not executed).
7. `order_completion_service.go:1243-1246,1403-1406` — stale "full gross (subtotal + shipping + commission)" doc comments on `RefundOrder`/`RefundFromDispute`; code at `:1290,1456` already uses PD+S.

**Zombie/dead:**
8. `refund_policy.go:36` `OrderSnapshot.LegacyGross()` — no consumers; duplicate of DV-1.
9. `refund_policy_test.go` — stale, non-compiling test encoding the old `Gross()`/`Amount` model.
10. `refund_repository_impl.go:389-415` `GetCumulativeCommissionReversedByOrder` — SUM(0) stub.
11. `dispute_service.go:69-76` + `finance_service.go:1009-1060` `CreateDisputeFreeze` — no production caller; comment encodes "escrow_amount − commission" seller-net semantics.
12. Mobile `order_dto.dart:425,534,628,728,918` `sellerCommission` — dead field (marked REMOVED).
13. Mobile l10n `commission5Percent` / `commission3Percent` / `commission5PercentSale` / `commission3PercentSale` + generated files — no consumers; hardcoded 4%/3% rates.
14. Mobile analytics doc `platformCommission` (`analytics/docs/README.md:156`).

---

# PROTECTED_LIST

Canonical code and semantics that MUST NOT be removed or altered during cleanup:

1. `order.CommissionAmount` immutable snapshot — `order.go:58-59`; copy sites `order_creation_service.go:936,1622`.
2. `calculateCommission` + PD basis — `pricing_token_service.go:379-384,679-682,948-949,1243-1248`.
3. `platform_configs` rate keys + `ConfigService.GetListingCommission`/`GetAuctionCommission` — `config_service.go:30-59`.
4. `CalculateProportionalRefundBreakdown` + `proportionalFloor` — `refund_math.go:35-146` (PD denominator).
5. `refund_gateway.go` remainder-commission formula (PD denominator) — `:367,432` (formula itself; the PD *binding* at `:326` is on the KILL_LIST, not the formula).
6. Verifier convergence — `verifier.go:669-683,735-746` `verifierProportionalCommissionPD` (PD denominator); do NOT revert to `verifierProportionalCommission`/EscrowAmount denominator.
7. Ledger postings — `RecordOrderRelease` (`finance_service.go:187-234`), `RecordPartialRefundRelease` (`:794-866`), `RecordRefundReversal` (`:570-759`) — the accounting contract.
8. `CalculateGrossEscrowFromSnapshot` — `pricing_helper.go:30-31` (escrow gross display/validation helper; keep as escrow semantics, never as commission denominator).
9. `ReleaseGatewayEscrowToSeller` gross/commission/sellerNet derivation — `order_payment_service.go:257-294`.
10. Refund policy canonical facts — `refund_policy.go:3-9,22-33` (C seller-side, never in buyer CashRefund; shipping has NO commission) and `RefundPolicyResult.CashRefund = Rpd + Rs` (`:26,43-58`).
11. DTO/projection/notification passthroughs of `order.CommissionAmount` (CC-9, CC-10, CC-13, CC-14) — consumers of the canonical value.
12. Escrow row (`escrows.amount`) as escrow authority — `wallet_service.go`, `canonical_finalization_service.go:117-126` — escrow semantics must remain separate from commission identity.
13. Admin finance `commission_revenue_rupiah` ledger-derived classification — `admin_finance_handler.go:502-562` (derived from ledger reference types, not from order rows).
14. `verify_partial_refund_semantics` cmd tool — canonical-math proof tool (Tier 2 per `cmd/README.md`).

---

# TEST_FIXTURE_RESIDUE

| # | Fixture / test | File:lines | Encodes old model? | Active runtime consumer? | Classification |
|---|---|---|---|---|---|
| TF-1 | `partial_refund_release_test.go` remainder commission 1488 | `:65-71,152-154` | **YES — full-gross P+S+C denominator** | No (test-only) | CONFLICTING fixture (must be rebased to PD) |
| TF-2 | `refund_policy_test.go` (`Gross()`, `.Amount`, `NewRefund`, 131250) | whole file | YES — old gross model; stale names | No (does not compile) | LEGACY/ZOMBIE |
| TF-3 | `order_domain_test.go:388-393` escrow assertion | `:381-393` | NO — asserts canonical `P+S+C` escrow + commission copy | No | Consistent, keep |
| TF-4 | `shipping_proof_test.go:280-281,318-319` (C=5500, pct=5) | `:280-281,318-319` | NO | No | Consistent fixture |
| TF-5 | `order_creation_service_test.go:338-343` (Escrow 120000 = P+S+C) | `:338-343` | NO — canonical | No | Consistent fixture |
| TF-6 | `refund_gateway_webhook_spy_test.go:334-344,438,608` | `:334-344,438,608` | NO (canonical fields; comment "gross=131250" is display-only) | No | Consistent |
| TF-7 | `refund_math_test.go` (PD 90000, S 20000, C 4500, K 18000) | `:8-13,57-231` | NO — canonical PD math | No | Consistent, keep |
| TF-8 | `settlement_release_ledger_test.go` (escrow 125000 = P+S+C; sellerNet 120000) | `:29-31,281-295,328-341` | NO — canonical release/refund ledger shape (gross includes commission; commission posted at release) | No | Consistent, keep |
| TF-9 | `admin_refund_real_db_proof_integration_test.go:47` (escrow_amount 129500 = P+S+C) | `:47` | NO — canonical escrow snapshot | No | Consistent fixture |
| TF-10 | raw-SQL order INSERTs with `escrow_amount` and commission 0 | `worker_integration_test.go:85-95`; `order_alert_rules_integration_test.go:45-48`; `escrow_repository_real_db_proof_integration_test.go:27,45`; `refund_real_db_proof_integration_test.go:92`; `escrow_ledger_atomicity_real_db_proof_integration_test.go:29`; `refund_service_real_db_proof_integration_test.go:107`; `order_rating_repository_integration_test.go:43`; `rating_handler_integration_test.go:77,94`; `negotiation_repository_integration_test.go:93-95` | no | No | Consistent (commission 0; they insert `escrow_amount` manually because production never writes it — this is a symptom of LIVE-4, not an old-model encoding) |
| TF-11 | `canonical_pricing_snapshot_persistence_test.go` (P=100000, D=10000, S=20000, C=4500, buyer base 110000 = (P−D)+S) | `:30-121` | NO — asserts the canonical discounted buyer base AND the anti-proof `≠ P+S+C−D` | No | **Canonical reference test** — directly contradicts the production `TotalBeforeCoinsAmount` wiring (CF-2/LIVE-3) |
| TF-12 | `canonical_discount_snapshot_real_db_integration_test.go:196,215,219,227-228,288-293` | untracked WIP | NO — canonical PD (`Subtotal − Discount`), reads `order.DiscountAmount` (field does not exist on entity → non-compiling WIP) | No | Canonical intent; currently non-compiling parallel work |
| TF-13 | verifier in-file fixtures `DiscountAmount: 0` | `verifier.go:1209,1230` | no — explicitly honest with the new field | Yes (verifier tests) | Consistent |
| TF-14 | mobile `order_contract_p1_test.dart:84-98` (`commission_amount: 500, escrow_amount: 11500 = P+S+C`) | `:84-98` | NO — canonical | No | Consistent |
| TF-15 | `admin_finance_summary_test.go:104-142` | `:104-142` | NO — ledger-derived classification | No | Consistent |

**Distinguish active runtime consumers from test-only residue:** All `_test.go` / `_test.dart` entries are test-only by construction. The ONLY fixture residue capable of influencing runtime is TF-1's *model* — but it is confined to a test. The genuinely runtime-affecting residue is CF-1/CF-2/CF-3/CF-4/LIVE-4 (production code paths), listed separately.

---

# DOCUMENTATION_RESIDUE

| # | Doc / comment | File:lines | Content | Old model? |
|---|---|---|---|---|
| DR-1 | `order_completion_service.go` "Refund is always full gross (subtotal + shipping + commission)" | `:1243-1246,1403-1406` | contradicts code (`:1290,1456` = P+S) | YES |
| DR-2 | `dispute_service.go:72` "frozenAmount is the seller's net (escrow_amount − commission)" | `:72` | escrow-derived commission subtraction on a dead surface | YES (zombie doc) |
| DR-3 | `decision.go:251` "EscrowAmount is calculated from snapshots (Subtotal + Shipping + Commission)" | `:251` | accurate for the DTO | NO |
| DR-4 | mobile l10n "Commission: 4%/3% per sale" | app_*.arb + generated | hardcoded rates, no consumers | Partial (rate copy) |
| DR-5 | `analytics/docs/README.md:156` `platformCommission` | doc sketch | doc-only | Partial |
| DR-6 | `cross-domain-relations.md:168` "jangan turunkan di bawah commission" | docs | commission-safety note | NO (accurate) |
| DR-7 | `admin-bootstrap.md:243` "Changes to commission rates … take effect immediately" | docs | rate governance note | NO |
| DR-8 | `config.go:30,399` "CommissionPercent removed" | config | accurate migration note | NO |
| DR-9 | `auction_settlement_type.go:10`, `method.go:85`, `auction_service.go:43,71`, `coins/README.md:78`, `refund.go:132`, `pricing_helper.go:8-27`, `refund_math.go:5-22`, `refund_gateway.go:1-10` | assorted | canonical doctrine comments | NO (consistent) |
| DR-10 | `SCOPE_4A_AUDIT_REPORT.md:63-66,600` | repo root | already documents the discount-persistence gap | Informational |

---

# COMMANDS_AND_RESULTS

Read-only inspection commands (no mutations). Full command list:

| Command | Result |
|---|---|
| `git status --short` (D:\Project\labuda) | working tree: `verifier.go` modified (converged); many untracked WIP files incl. `canonical_discount_snapshot_real_db_integration_test.go` |
| grep `(?i)commission` in `backend/` | 77 files |
| grep `(?i)commission` in `apps/` | mobile + admin files (DTOs, l10n, display) |
| grep `CommissionAmount` across repo | canonical chain + consumers (this report §CANONICAL_AUTHORITY, §CANONICAL_CONSUMERS) |
| grep `LegacyGross` | `refund_policy.go:36` definition only; no production consumers |
| grep `proportionalFloor|ProportionalCommission|verifierProportional` | `refund_math.go`, `refund_gateway.go`, `verifier.go` (converged PD), cmd tool |
| grep `EscrowAmount` in backend | verifier caps, escrow checker, recon, projection, DTOs, tests |
| grep `discount_amount` | schema (000001:1104,1265), pricing token repo, verifier load, canonical tests; **no production writer to orders** |
| grep `UPDATE orders SET|INSERT INTO orders` | production writers: `UpdateStatusTx`, `UpdatePaymentSelectionTx`, `CreateOrderTx` (no escrow/discount columns); rest are tests |
| grep `CreateDisputeFreeze(` | interface + impl + tests only; **no production caller** |
| grep `\.RefundOrder\(|\.RefundFromDispute\(` | live: `user_ban_handler.go:432`, `orchestrator/order_handler.go:305`, `order_service.go:322,334`, `dispute_service.go:392`, `order_completion_service.go:1569` — all via the canonical PD+S refund amount |
| grep `commission.*escrow|escrow.*commission` | cmd tool, verifier, tests, `settlement_release_ledger_test.go`, `partial_refund_release_test.go`, stale doc comments |

No build/test commands were executed (read-only pass; several tracked tests are already known non-compiling/stale per prior reports — `refund_policy_test.go`, plus untracked WIP tests).

---

# FILES_CHANGED

- production: **NONE**
- tests: **NONE**
- schema/migrations: **NONE**
- only `COMMISSION_RESIDUE_AUDIT.md` created at repo root.

---

# FINAL_STOP

No implementation authorized. Audit only.

This pass performed discovery and classification only. The KILL_LIST and PROTECTED_LIST above are evidence-based inventories; no item was modified, deleted, or migrated. The locked canonical truth (order.CommissionAmount, PD denominator, product-only commission, CommissionDelta as derived allocation, EscrowAmount not a commission authority) was used as the reference standard and was not reopened or redesigned.
