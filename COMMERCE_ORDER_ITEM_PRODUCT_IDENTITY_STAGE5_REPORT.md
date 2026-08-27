# COMMERCE — STAGE 5
# ORDER ITEM PRODUCT IDENTITY CONVERGENCE — IMPLEMENTATION REPORT

**Verdict: `ORDER_ITEM_PRODUCT_IDENTITY_CONVERGED_RUNTIME_PROVEN`**

`order_items.product_id` now means — and is enforced to be — `products.id` on every order path. FPS, negotiation, and auction writers converge on the Product ID; the selling-surface identity lives in `orders.source_type` + `orders.source_id`; a DB FK (`000045`) makes the contract explicit and non-negotiable.

No product lifecycle was reintroduced. No quantity/inventory architecture was solved. No listing rename was performed. Scope confined to the Product↔OrderItem identity contract; finance/coins/ledger/refund/commission untouched (verified not to depend on this identity).

---

## 1. EXACT WRITERS TRACED (re-derived from current source)

The only production `order_items` writers (both call the single INSERT at `backend/internal/commerce/order/infrastructure/repository/order_repository.go:844-869`):

| Writer | Before | After | File:line |
|---|---|---|---|
| FPS buy-now (`CreateFromSaleSurface`) | `listing.ID` (`fixed_price_sales.id`) | **`listing.ProductID`** (`products.id`) | `order/application/order_creation_service.go:1664-1670` |
| Negotiation (runs the same `CreateFromSaleSurface`; `SourceType = OrderSourceFixedPriceSale`, `SourceID = fixedPriceSale.ID`) | `listing.ID` | `listing.ProductID` (same line, shared path) | `interaction/chat/delivery/http/chat_handler.go:2106-2118` → `order_creation_service.go:1664-1670` |
| Auction bid-win / buy-now (`CreateFromAuction`) | `product.ID` (`products.id`) | unchanged (`product.ID`) | `order_creation_service.go:979-985` |

`listing.ProductID` is the canonical FPS→Product relationship (already present on the joined entity, `fixedprice/infrastructure/repository/fixed_price_sale_repository_impl.go:123-126,584`); no new helper was introduced. Verified: `grep NewOrderItem(` across repo shows exactly these two call sites (plus the Stage-5 tests). No other writer exists (`cmd/*`, workers, seed all clean). PROVEN.

## 2. EXACT CONSUMERS TRACED (current source)

| Consumer | Semantics | File:line | Action taken |
|---|---|---|---|
| `GetOrderItems` | opaque passthrough → `OrderItem.ProductID` | `order/infrastructure/repository/order_repository_extensions.go:300-341` | unchanged (opaque transport) |
| Stock restore `restoreListingStock` → `restoreFixedPriceListingStock` | previously surface-keyed via `item.ProductID` (FPS namespace); now **surface-keyed via `orders.source_id`**; `item.Quantity` summed; early no-items shortcut preserved | `order/application/order_completion_service.go:1949-2009` | **FIXED** |
| `CountActiveOrdersByProduct` | equality `oi.product_id = $1`, no source filter; called with `products.id` | `order_repository_extensions.go:393-411`; caller `shipping/application/listing_shipping_service.go:83` | **Fix is upstream (writer)** — the SQL and caller were product-keyed already; now they actually see FPS orders (proven) |
| `CountAnyOrdersByProduct` | equality `oi.product_id = $1`; caller passed FPS id | `order_repository_extensions.go:413-419`; caller `fixedprice/delivery/http/fixed_price_sale_handler.go:402` | **FIXED** — caller now passes `listing.ProductID` (product-wide lock) |
| Buyer order detail DTO | surfaces `item.ProductID` as `product_id` | `order/delivery/http/dto/decision.go:668-677` | unchanged (opaque; value now correct for FPS orders) |
| Admin order detail | surfaces `item.ProductID` | `order/delivery/http/admin_order_handler.go:475-483` | unchanged (opaque) |
| Finance / refund / coins / commission | never read `order_items` or its product column | (whole `internal/finance` grep clean) | n/a |

No consumer remains that interprets `order_items.product_id` as a selling-surface ID. PROVEN.

## 3. BEFORE / AFTER IDENTITY SEMANTICS

| Property | Before (Stage 4) | After (Stage 5) |
|---|---|---|
| FPS order `order_items.product_id` | `fixed_price_sales.id` (surface) | `products.id` (Product) |
| negotiation order `order_items.product_id` | `fixed_price_sales.id` (surface) | `products.id` (Product) |
| auction order `order_items.product_id` | `products.id` (Product) | `products.id` (Product, unchanged) |
| selling-surface identity | (only accidentally derivable via `orders.source_type`+`source_id`) | authoritative, unchanged: `orders.source_type`+`source_id` |
| stock restore lookup | `item.ProductID` (= FPS id) | `orders.source_id` (single surface per order) |
| FPS edit guard | product-keyed count called with FPS id | product-keyed count called with `products.id` — spans FPS + auction orders |
| shipping-change guard | product-keyed count; silently missed FPS orders | product-keyed count; **now includes FPS orders** (writer fix) |
| FK / nullability | no FK, nullable, dual namespace | FK `order_items_product_id_fkey → products(id)`, `NOT NULL` |

## 4. FILES CHANGED (production)

1. `backend/internal/commerce/order/application/order_creation_service.go` — FPS/negotiation order-item writer: `listing.ID` → `listing.ProductID`; comment updated (line ~1663-1669).
2. `backend/internal/commerce/order/application/order_completion_service.go` — `restoreFixedPriceListingStock` now takes the `*entity.Order` and resolves the surface via `order.SourceType`/`order.SourceID`; sums `item.Quantity`; empty-item shortcut retained; doc comments updated (lines 1938-2009).
3. `backend/internal/commerce/fixedprice/delivery/http/fixed_price_sale_handler.go` — FPS edit guard `CountAnyOrdersByProduct(listingID)` → `CountAnyOrdersByProduct(listing.ProductID)`; comment updated (lines 400-403).

**Blocking-dependency fixes (pre-existing production defects on the order-creation shipping path, 000014 column residue):**
4. `backend/internal/commerce/shipping/infrastructure/repository/listing_shipping_option_repository_impl.go` — `GetByProduct`/`GetAvailableByProduct` no longer SELECT/scan the dropped `so.expedition_name` (migration 000014); `ExpeditionName` scanned as nil.
5. `backend/internal/commerce/shipping/infrastructure/repository/shipping_coverage_repository_impl.go` — `GetByOptionAndProvince` no longer SELECT/scan the dropped `estimated_days` (migration 000014); `EstimatedDays` nil.

## 5. MIGRATION / SCHEMA CHANGES

`backend/migrations/000045_order_item_product_identity_convergence.{up,down}.sql`

Up:
1. Backfill legacy rows: `UPDATE order_items oi SET product_id = fps.product_id FROM fixed_price_sales fps WHERE fps.id = oi.product_id` — converges any historical FPS-namespace row to `products.id` (auction rows already correct; unfixable rows abort loudly on step 2).
2. `ALTER COLUMN product_id SET NOT NULL`.
3. `ADD CONSTRAINT order_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE RESTRICT` (matches the FPS/Auction FK posture; products are never hard-deleted while a surface references them).

Down: drop FK, drop NOT NULL.

No nullable/dual-ID compatibility field was added. No `selling_surface_id` column was added (unneeded — `orders.source_type`+`source_id` already carry it). Fresh-DB and legacy-DB migration both prove out (see §6).

## 6. RUNTIME DB PROOF (real Postgres, `labuda_test`)

All three integration proofs PASS (disposable schema rebuilt from scratch by `testdb.SetupDB`, canonical `migration.Run`):

| Test | Proves | Result |
|---|---|---|
| `backend/tests/order_item_product_identity_convergence_integration_test.go` — `TestOrderItemProductIdentity_Convergence_RuntimeProof` | 1 FPS order `product_id == fixed_price_sale.product_id`; 2 negotiation order `product_id == fixed_price_sale.product_id` (+ `source_type`/`source_id`/`negotiation_id` assertions); 3 auction order `product_id == auction.product_id`; 4 same Product reused across sold-FPS→FPS→auction with the same `products.id` per order; 5/6 `CountActiveOrdersByProduct`/`CountAnyOrdersByProduct(productID)` see FPS orders and span FPS+auction (final count 3); 8 auction quantity stays 1. | **PASS** |
| `..._integration_test.go` — `TestMigration000045_OrderItemProductIdentity_UpDownReplay` | Fresh up state (FK + NOT NULL present); down removes both; a crafted legacy order whose item stored `fixed_price_sales.id` **backfills to `products.id`** on re-up; FK+NOT NULL reinstated. | **PASS** |
| `backend/internal/commerce/order/application/order_completion_restore_source_integration_test.go` — `TestStage5_RestoreListingStock_ResolvesSurfaceFromOrderSource` | FPS order restore resolves the listing through `orders.source_id` (item `product_id` is `products.id`); quantity restored 1→2; auction order binding released through `orders.source_id` (PASS_20B preserved). | **PASS** |

Wiring note: identity-irrelevant gates (account-status, seller-capability, actor-resolution) are stubbed; every persistence touch uses the real repositories against the real test database.

## 7. REGRESSION RESULTS

- `go build ./...` — clean.
- `go vet` (non-integration) `./internal/commerce/order/...`, `./internal/commerce/fixedprice/delivery/http/...`, `./internal/commerce/shipping/infrastructure/...` — clean.
- `go vet -tags integration ./internal/commerce/order/application/ ./tests/` — clean (Stage-5 test files compile under the integration tag).
- Unit tests (no tag) — all `ok`: `order/application`, `order/entity`, `order/delivery/http`, `order/delivery/http/dto`, `order/infrastructure/repository`, `order/rating/*`; `fixedprice/entity`, `fixedprice/delivery/http`; `tests` (compile).
- Integration proofs in §6 — PASS (real Postgres). Note: `tests/` and `order/application` must be run in **separate** `go test` invocations — `testdb.SetupDB` resets the shared `public` schema per test binary, and two binaries racing the same `labuda_test` database corrupt each other's schema mid-run. Each package's binary is green in isolation.

## 8. RESIDUE CLASSIFICATION

Sweep for the old assumption `order_items.product_id == fixed_price_sales.id` (Go + SQL + docs):

| Hit | Classification |
|---|---|
| `order_creation_service.go:1664-1669` (new comment) | canonical (stage-5 contract) |
| `order_completion_service.go:1949-2009` (new comments) | canonical |
| `fixed_price_sale_handler.go:400-403` (new comment) | canonical |
| `decision.go:671`, `admin_order_handler.go:483`, `order_repository.go:857`, `order_repository_extensions.go:324` | intentionally opaque transport (no namespace assumption) |
| `order_item_identity_test.go:18-19` (entity test: `NewOrderItem(productID…)`) | test fixture — product-keyed, correct |
| `auction_settlement_test.go:287` (comment about PASS_20B history) | documentation — accurate history, not made false |
| `monitoring_service_test.go:88` (commented `oi.listing_id` line) | dead/stale pre-Stage-5 schema comment; pre-existing, not made false by this stage — left |
| `000045_order_item_product_identity_convergence.up.sql` | canonical (migration) |
| Stage-5 proof files (crafted legacy row at `..._integration_test.go:667`) | test fixture — intentional legacy fixture for backfill proof |

No production code, doc, or fixture claims `order_items.product_id` is a sale/surface ID. Clean.

## 9. UNRELATED PRE-EXISTING FAILURES (reproduced, NOT caused by this stage, NOT fixed)

1. **Shipping domain 000014 column residue — production.** `shipping_option_repository_impl.go` (multiple SELECT/UPDATE of `expedition_name`), `shipping_coverage_repository_impl.go` (`estimated_days` in Create/Update/GetByID/GetByShippingOption), `city_override_repository_impl.go` (`estimated_days` CRUD), and handlers dereferencing `*opt.ExpeditionName` (`shipping_handler.go:129`, `seller_shipping_handler.go:742`). Migration `000014_shipping_authority_hard_purge` dropped the columns without aligning the code — seller-side shipping CRUD is live-broken. **Out of scope for order identity; only the two checkout-path queries (`GetByProduct`, `GetByOptionAndProvince`) were fixed (§4 items 4-5) because the Stage-5 runtime proof drives order creation's shipping check.**
2. **Test build breaks (reproduced):** `shipping/application` (`seller_shipping_service_test.go` references removed `DisplayName`/`InternalNote`/`Coverages`/`CreateShippingOptionCoverageInput`/`ShippingOptionSummary`); `fixedprice/delivery/http` under `-tags integration` (`fixed_price_sale_media_integration_test.go` references undefined `shippingRepo.ShippingOptionSummary`); plus the previously recorded `auction/application`, `fixedprice/application`, `fixedprice/infrastructure/repository`, `chat/application`. `go build ./...` is clean — test-only residue.

These are recorded as separate workstreams, not repaired here.

## 10. FINAL VERDICT

**`ORDER_ITEM_PRODUCT_IDENTITY_CONVERGED_RUNTIME_PROVEN`**

- Every `order_items` writer emits `products.id`; proven against real Postgres for FPS buy-now, negotiation, auction claim, and product reuse.
- Product-based consumers are now product-keyed and correct (`CountActiveOrdersByProduct`/`CountAnyOrdersByProduct` see FPS orders; FPS edit guard is product-wide).
- Stock restoration resolves the surface exclusively through `orders.source_type`+`source_id`, never through `order_items.product_id`.
- The DB contract is explicit: `product_id` `NOT NULL` + FK → `products(id)`, with a backfill migration that converges legacy rows and replays cleanly up/down.
- No selling-surface duplicate identity introduced; no product lifecycle reintroduced; finance/coins/ledger/refund/commission untouched (grep-verified).

STOP condition honored — Stage 6 not started.