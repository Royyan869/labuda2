# VERDICT

**PASS**

## 1. Current Canonical Signature

`NewOrderFromSource` lives in
`backend/internal/commerce/order/entity/order.go` (line 1030) and is the **single
authority** for order construction. No legacy shipping arguments exist and no
compatibility shim was introduced.

```go
func NewOrderFromSource(
	buyerID, sellerID uuid.UUID,
	sourceType OrderSourceType,
	sourceID uuid.UUID,
	negotiationID *uuid.UUID,
	quantity int,
	unitPrice, subtotal, shippingTotal money.Money,
	commissionPercent int64,
	commissionAmount money.Money,
	serviceFeeAmount money.Money,
	totalPayableAmount money.Money,
	shippingSetupID *uuid.UUID,      // NULLABLE: nil when using shipping quote
	shippingSetupName string,
	shippingTransportType string,
	auctionSettlementType *AuctionSettlementType,
	preparationTimeSnapshot string,
	preparationNoteSnapshot *string,
	shippingSource *string,          // "for_sale" | "shipping_quote"
	shippingQuoteID *uuid.UUID,
	shippingQuotePrice *int64,
	pricingTokenID *uuid.UUID,
	paymentMethod string,
	paymentExpiresAt time.Time,
) *Order
```

Canonical shipping identity is `ShippingSetupID` + `ShippingSetupName` +
`ShippingTransportType`. The obsolete metadata
`shippingExpeditionName` / `shippingEstimatedDays` (and DB columns
`shipping_expedition_name` / `shipping_estimated_days`) were **not** revived.

## 2. Root Cause

The signature drift was a **two-part residue of the ShippingOption→ShippingSetup
convergence** (migrations 000014/000060, PASS_03A backend convergence), which had
been applied to the production entity/repository layer but left callers and
repositories half-migrated:

1. **Caller drift (F2/F3).** Several integration test call sites still passed the
   pre-convergence argument list — either 26 arguments (an extra displaced
   `"immediate"` preparation-time string plus a misplaced nil run) or 24
   arguments (missing `shippingQuotePrice` placeholder so `&pricingTokenID`
   landed in the wrong slot). Because those suites carry `//go:build integration`,
   plain `go build ./...` never caught them; only
   `go test -tags integration` compilation did. This is exactly the blocker
   recorded in
   `docs/audits/REPORT_AUCTION_SETTLEMENT_PHASE1_DB_VERIFICATION_FINAL.md` and
   `docs/audits/REPORT_AUCTION_SETTLEMENT_PHASE1_MIGRATION_FIX_FINAL.md`
   (F2/F3 + F2-family).

2. **Repository placeholder drift (runtime, newly exposed).** The convergence
   removed the legacy columns from the `orders` / `pricing_tokens` INSERT
   column lists and Go argument lists, but left the `VALUES ($1…$N)` placeholder
   runs inconsistent with the new column counts:
   - `order_repository.go` `CreateOrderTx`: 37 columns/37 args but `VALUES $1..$39`
     → Postgres `INSERT has more expressions than target columns (42601)`.
   - `pricing_token_repository_impl.go` `CreateTx`: 35 columns/35 args but
     `VALUES $1..$34` → Postgres `INSERT has more target columns than expressions
     (42601)`.
   These surfaced the moment the previously compile-blocked DB integration tests
   could actually run.

3. **Test semantic drift.** `TestOrderItemProductIdentity_Convergence_RuntimeProof`
   section "1b" asserted that creating a **second** for-sale on a sold product
   fails at *order creation*. Current canonical product identity
   (`products.selling_surface` permanent claim via `ClaimSellingSurface`,
   `ForSaleStatusSold` seller-terminal) rejects the second surface at *surface
   creation* — the order-creation path is unreachable. The test was updated to
   assert at the correct layer.

No production business behavior was changed: `NewOrderFromSource`, order
creation, auction settlement, and the restore dispatch were all left intact.

## 3. Production Consumers

All production callers already used the canonical 25-argument signature and were
**inspected, not changed**:

| Caller | Result |
|---|---|
| `internal/commerce/order/application/order_creation_service.go:941` (for-sale surface path) | CANONICAL — unchanged |
| `internal/commerce/order/application/order_creation_service.go:1623` (auction path) | CANONICAL — unchanged |
| `internal/commerce/order/entity/order.go:1030` (definition) | CANONICAL — unchanged |

Repository SQL that the constructor feeds was **changed** to match the current
schema (placeholder counts, not business logic):

| File | Change |
|---|---|
| `internal/commerce/order/infrastructure/repository/order_repository.go` | `CreateOrderTx` `VALUES $1..$39` → `$1..$37` (matches 37 columns/args) |
| `internal/pricing/token/infrastructure/repository/pricing_token_repository_impl.go` | `CreateTx` `VALUES $1..$34` → `$1..$35` (matches 35 columns/args) |

## 4. Test Consumers

Stale tests/helpers updated to the canonical contract:

| File | Change |
|---|---|
| `internal/commerce/order/tests/order_canonical_test.go` | Two 26-arg call sites in `TestDifferentBuyersSameIdempotencyKey` (buyer1/buyer2) restored to canonical 25-arg order (`auctionSettlementType nil` then `"immediate"` preparation-time snapshot) |
| `internal/serverboot/payment_intent_verification_integration_test.go` | `createPaymentIntentOrderWithToken` restored to canonical 25 args — added the missing `shippingQuotePrice` nil slot so `&tokenID` lands in `pricingTokenID` |
| `tests/order_item_product_identity_convergence_integration_test.go` | `createLegacyOrderWithFPSNamespace` restored to canonical 25 args (displaced `"immediate"` moved into `preparationTimeSnapshot`); section 1b re-asserted at surface-creation layer (permanent selling-surface claim) |
| `internal/commerce/order/application/order_completion_restore_source_integration_test.go` | Two stale `svc.restoreListingStock(...)` calls → canonical `svc.restoreForSaleStock(...)` (the current source-resolved restore dispatch) |
| `tests/product_public_availability_stage6b_integration_test.go` | `NewAuctionService` call aligned to current 11-arg signature (added `zap.NewNop()` logger arg) so the `tests/` integration package compiles |

## 5. Shipping Residue

Obsolete shipping references removed in this session's files:

- `order_repository.go` `CreateOrderTx`: no column/value residue for the dropped
  `shipping_expedition_name` / `shipping_estimated_days` (already removed by the
  prior convergence; this task fixed the leftover placeholder overhang).
- `pricing_token_repository_impl.go` `CreateTx`: no column/value residue for the
  dropped columns (placeholder underhang fixed).
- All four order integration test callers + the serverboot caller + the
  `tests/` helper: zero legacy shipping arguments.

Global residue classification is in §9. No obsolete shipping metadata was
reintroduced anywhere.

## 6. Files Changed

1. `backend/internal/commerce/order/infrastructure/repository/order_repository.go`
2. `backend/internal/pricing/token/infrastructure/repository/pricing_token_repository_impl.go`
3. `backend/internal/commerce/order/tests/order_canonical_test.go`
4. `backend/internal/serverboot/payment_intent_verification_integration_test.go`
5. `backend/internal/commerce/order/application/order_completion_restore_source_integration_test.go`
6. `backend/tests/order_item_product_identity_convergence_integration_test.go`
7. `backend/tests/product_public_availability_stage6b_integration_test.go`

## 7. Test Results

### Compile

| Command | Result |
|---|---|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test -tags integration -count=1 -run '^$' ./internal/commerce/order/...` | PASS |
| `go test -tags integration -count=1 -run '^$' ./internal/serverboot/...` | PASS |
| `go test -tags integration -count=1 -run '^$' ./tests/...` | PASS |
| `go vet -tags integration ./internal/... ./tests/...` | FAIL (pre-existing, unrelated — see §8) |

### Unit

| Command | Result |
|---|---|
| `go test -count=1 ./internal/commerce/order/...` | PASS |

### Integration + DB runtime (real PostgreSQL `labuda_test`, fresh migration per test)

| Command | Result |
|---|---|
| `go test -tags integration -run '^TestDoubleCheckoutProtection$' ./internal/commerce/order/tests/` | PASS |
| `go test -tags integration -run '^TestStockRaceCondition$' ./internal/commerce/order/tests/` | PASS |
| `go test -tags integration -run '^TestOrderCreationIdempotency$' ./internal/commerce/order/tests/` | PASS |
| `go test -tags integration -run '^TestDifferentBuyersSameIdempotencyKey$' ./internal/commerce/order/tests/` | PASS |
| `go test -tags integration -run '^TestStage5_RestoreListingStock_ResolvesSurfaceFromOrderSource$' ./internal/commerce/order/application/` | PASS |
| `go test -tags integration -run '^TestAuctionBuyNowSettlement_ClosesAuctionAndBlocksDoubleSale$' ./internal/commerce/order/tests/` | PASS |
| `go test -tags integration -run '^TestAuctionOrderCancel_ReleasesBinding$' ./internal/commerce/order/tests/` | PASS |
| `go test -tags integration -run '^TestFPS002' ./internal/commerce/order/tests/` | PASS |
| `go test -tags integration -run '^TestCreatePayment_BasicFlowAndPreviewAuthority$' ./internal/serverboot/` | PASS |
| `go test -tags integration -run '^TestCreatePayment_ShippingPositivePricingTokenCoinCap$' ./internal/serverboot/` | PASS |
| `go test -tags integration -run '^TestOrderItemProductIdentity_Convergence_RuntimeProof$' ./tests/` | PASS |
| `go test -tags integration -run '^TestStage6B_ReuseQuantity_RejectsSecondForSale$' ./tests/` | PASS |
| `go test -tags integration -run '^TestProductSellingSurfaceExclusivity$' ./tests/` | PASS |
| `go test -tags integration -run '^TestStage6B_AuctionBrowse_AnonymousRestricted_OwnerStatusScoped$' ./tests/` | PASS |

All DB runtime proof obtained against the existing local PostgreSQL — no DB
infrastructure blocker.

## 8. Remaining Blockers

Outside this task's scope (present before this task, untouched, recorded for the
next phase):

1. **`vet -tags integration` failures in unrelated packages**
   (not exercised by the task's `go vet ./...` gate; none touch
   `NewOrderFromSource`):
   - `internal/commerce/seller/delivery/http/seller_dashboard_for_sale_integration_test.go:88` —
     `SellerDashboardResponse.TotalListings` undefined.
   - `internal/finance/bankaccount/application/bank_account_authority_integration_test.go:288` —
     `BankAccountService.UpdateBankAccount` undefined.
   - `internal/governance/support/delivery/http/support_handler_capability_test.go:286` —
     `supportApp.NewService` arity drift (missing `DisputeService`).
   - `internal/governance/verification/application/verification_service_test.go:29` —
     `mockTx` does not satisfy `db.Tx` (`Query` signature drift).
   - `internal/serverboot/chat_resource_projection_http_integration_test.go:881` —
     `fixture.appDB` self-assignment.
   - `internal/social/feed/infrastructure/repository/feed_follow_first_bootstrap_test.go:27` —
     `testdb.TestDB.TruncateSubset` undefined.
2. `chat_rooms.context_json / context_set_by` drift (recorded in prior reports).
3. Presence DB test harness race (recorded in prior reports).
4. Missing migration-governance docs (recorded in prior reports).

None of these block the previously-blocked order/serverboot/tests integration
suites from compiling or running.

## 9. Residue Audit

Global searches run over the whole repository after implementation:

| Term | Finding |
|---|---|
| `NewOrderFromSource(` | 1 definition + 2 production callers + 18 test callers — all CANONICAL (25 args, `ShippingSetup` contract) |
| `ShippingOptionID` (Go identifier) | 0 matches in `**/*.go` — CLEAN |
| `ShippingOptionName` (Go field) | 0 matches in `**/*.go` — CLEAN |
| `ShippingExpeditionName` / `ShippingEstimatedDays` | 0 matches in `**/*.go`; only audit-doc history — CLEAN |
| `shippingExpeditionName` / `shippingEstimatedDays` | 0 matches outside migrations/docs — CLEAN |
| `expedition_name` / `estimated_days` (non-migration `.go`) | 4 NOTE comments documenting the migration-000014 DROP (`shipping_setup.go`, `shipping_coverage.go`, `city_override.go`, `listing_shipping_option_repository_impl.go`) — LEGITIMATE HISTORICAL |
| `shipping_option_id` / `shipping_option_ids` / `shipping_options` | Live DB column/table + JSON wire names — **CANONICAL** (schema/API contract deliberately retained `shipping_option*` naming per the shipping redesign reports; Go struct fields are `ShippingSetup*`) |
| `shipping_expedition_name` / `shipping_estimated_days` | Migrations `000060` (up/down), `000001` base schema, plus audit-doc history — LEGITIMATE HISTORICAL MIGRATION REFERENCE |

Classification summary: every remaining non-canonical textual reference is either
a **legitimate historical migration reference** (migration SQL, DROP-NOTE
comments, audit docs) or the **canonical wire/DB `shipping_option*` naming** that
the convergence deliberately preserved. No STALE code reference remains.

## 10. Final Assessment

1. **Is `NewOrderFromSource` now single-authority?**
   YES. One definition in `order/entity/order.go`; no `Legacy`/`Compat`/`V2`
   variants, no variadic/optional-argument shim.

2. **Are all callers aligned?**
   YES. All production and test callers (entity unit tests, order integration
   tests, serverboot integration tests, `tests/` helpers) use the canonical
   25-argument `ShippingSetup`-based signature. Compile proof:
   `go test -tags integration -run '^$'` over `./internal/commerce/order/...`,
   `./internal/serverboot/...`, and `./tests/...` all PASS.

3. **Has obsolete shipping terminology been removed from affected callers?**
   YES. Zero `ShippingOptionID`/`ShippingOptionName`/`ShippingExpeditionName`/
   `ShippingEstimatedDays` Go identifiers remain anywhere in the backend; the
   affected callers carry only `ShippingSetup*` fields, and the only remaining
   `expedition_name`/`estimated_days` strings in non-migration Go are
   DROP-documenting comments.

4. **Did this task avoid changing business behavior?**
   YES. The constructor body, order creation paths, auction settlement
   implementation, and restore dispatch were untouched. The only production
   changes were SQL placeholder-count corrections (37/39 and 35/34) that make the
   INSERT statements match the columns/args already present — restoring the
   intended behavior rather than altering it. Migration `000062` was not touched.

5. **Can the previously blocked integration suites now proceed?**
   YES. All three previously compile-blocked suites
   (`./internal/commerce/order/...`, `./internal/serverboot/...`, `./tests/...`)
   compile with `-tags integration`, and the DB runtime tests that were blocked
   (order concurrency/idempotency, restore-source, auction settlement, FPS002,
   payment-intent verification, product-identity convergence) now pass against
   real PostgreSQL.
