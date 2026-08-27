# COMMERCE — ORDER ITEM PRODUCT IDENTITY AUDIT

**Mode:** READ-ONLY. No migration, no code change, no rename, no schema change.
**Truth source:** current filesystem only (source-of-truth). All claims re-derived from working tree; the Stage-4 audit was used only as a checklist, then re-verified line-by-line below.

---

## 1. VERDICT

**CONTRADICTION CONFIRMED in full:**

| Claim | Result |
|---|---|
| A. FPS order stores FPS ID in `order_items.product_id` | **PROVEN** |
| B. Auction order stores Product ID | **PROVEN** |
| C. Negotiation order uses the FPS namespace | **PROVEN** |
| D. `CountActiveOrdersByProduct` receives Product ID → misses FPS orders | **PROVEN** |
| E. Other consumers with the same namespace mismatch | **PROVEN** (see §5, §7) |
| F. `order_items.product_id` is intended to be the physical Product under Model B | **PROVEN** (API + entity + doc contract) |
| G. Any legitimate reason for it to be a surface ID | **NOT PROVEN / DISPROVEN** — no data-model reason; surface identity already exists at `orders.source_type`+`orders.source_id` |

**Headline:** the two order-item writers disagree about what `order_items.product_id` means; the API contract, the entity, and one writer (auction) say "Product ID"; the other writer (FPS) persists the selling-surface ID. The task is a syptic normalization, not a schema redesign: **`order_items.product_id` can and should become `products.id`, and the selling-surface identity is already durably stored one level up on `orders` (`source_type`,`source_id`), so no dedicated surface column is required.**

---

## 2. CANONICAL PRODUCT IDENTITY UNDER MODEL B

Under Model B (canonical direction, confirmed in the working tree):

- `products.id` is the stable, reusable physical-item identity. Product has **no status/sold_at** (`000044_product_lifecycle_removal.up.sql:16-21`).
- Selling surfaces `fixed_price_sales.id` (`000001_canonical_schema.up.sql:868-880`) and `auctions.id` (`:477-498`) are siblings that attach to exactly one product (`FK RESTRICT`, `:2340`, `:2286`).
- One active surface per product (partial unique indexes `:2015`, `:2092` + cross-table trigger `000010:76-112`).
- **A data record that stores "the item that was bought" must store `products.id`. The surface that was used is secondary provenance and is already recorded by `orders.source_type` + `orders.source_id`.**

Evidence that the API contract already separates the two namespaces:
- Backend `CreateOrderRequest` requires **both** `product_id` and `source_id` — "Parse canonical product and source identities" (`backend/internal/commerce/order/delivery/http/order_handler.go:956-966`).
- Mobile checkout separately resolves `productId` (from listing detail / widget) and `sourceId` (`fixedPriceSaleId` for FPS, `auctionId` for auction) (`apps/mobile/lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart:74-100`).
- Entity: `NewOrderItem(orderID, productID, ...)` — parameter named `productID` (`order/entity/order_item.go:23-33`), struct doc: "It captures the product relationship and price snapshot at order time" (`order_item.go:10-20`).
- Auction writer even documents its own intent: "Create order_item with full product snapshot" (`order_creation_service.go:977-985`).

---

## 3. PRODUCER MATRIX

| Producer | File:line | Value written into `order_items.product_id` | Actual namespace | FK on column |
|---|---|---|---|---|
| Buy-now direct (FPS checkout) | `backend/internal/commerce/order/application/order_creation_service.go:1664-1670` | `listing.ID` | **fixed_price_sales.id (surface)** | none |
| Negotiation checkout (runs the same `CreateFromSaleSurface`) | `backend/internal/interaction/chat/delivery/http/chat_handler.go:2106-2118` (`SourceID: fixedPriceSale.ID`, `SourceType: OrderSourceFixedPriceSale`) → item built at `order_creation_service.go:1664-1670` | `listing.ID` | **fixed_price_sales.id (surface)** | none |
| Auction bid-win claim | `auction/application/auction_service.go:898-911` → `order_creation_service.go:979-985` | `product.ID` | **products.id** | none |
| Auction buy-now | `order/delivery/http/order_handler.go:1090-1104` (auction source) → `order_creation_service.go:979-985` | `product.ID` | **products.id** | none |
| SQL persistence (single INSERT site for all) | `order/infrastructure/repository/order_repository.go:844-869` | `orderItem.ProductID` | whatever entity held | none (`product_id` uuid nullable, no FK) |

Only two `NewOrderItem` call sites exist in production (`order_creation_service.go:979` and `:1664`); only one INSERT into `order_items` (`order_repository.go:850`). No other writer exists. PROVEN.

---

## 4. CONSUMER MATRIX

| Consumer | File:line | Reads as | Namespace it assumes | Deterministic today? | Assumes-one-namespace-mismatch? |
|---|---|---|---|---|---|
| `GetOrderItems` | `order/infrastructure/repository/order_repository_extensions.go:300-341` | opaque passthrough → `OrderItem.ProductID` | none (opaque) | n/a | no |
| Stock restore (Cancel/Overdue/Expire) | `order/application/order_completion_service.go:1978-2002` (item at `:1986`) | `listingRepo.GetForUpdate(item.ProductID)` | **fixed_price_sales.id** | no (relies on SourceType branch `:1954-1957`) | no on FPS rows; breaks for product-id rows |
| `CountActiveOrdersByProduct` | `order_repository_extensions.go:393-411` | equality `oi.product_id = $1` no source filter | caller decides | no | **YES** — FPS rows unreachable when called with Product ID |
| `CountAnyOrdersByProduct` | `order_repository_extensions.go:413-419` | equality `oi.product_id = $1` no source filter | caller decides | no | **YES** — currently "works" only because caller passes FPS ID |
| Buyer order detail DTO | `order/delivery/http/dto/decision.go:668-677` | surface `item.ProductID` as `product_id` | none (passthrough) | n/a | no (value exposes wrong namespace) |
| Admin order detail | `order/delivery/http/admin_order_handler.go:475-483` | surface `item.ProductID` | none (passthrough) | n/a | no (value exposes wrong namespace) |
| Mobile order detail | `apps/mobile/lib/domains/commerce/transaction/order/data/mappers/order_mapper.dart:96-105` (fallback branch; backend order detail has no `product` block) | stores item `id`/`productId` = `dto.productId` | display only; no navigation found | n/a | no (value changes semantics on display) |
| finance/refund/commission/escrow/coins | whole `internal/finance` tree | **never reads `order_items` or its product column** (grep: zero `product_id` hits in `internal/finance`) | n/a | n/a | no |

Destination of the restore branch: `releaseAuctionOrderBinding` uses `order.SourceID` (auction id), not `item.ProductID` (`order_completion_service.go:2012-2025`) — that path is already source-keyed and immune.

---

## 5. NAMESPACE MISMATCH PROOF

### A. FPS order stores FPS ID (PROVEN)
`order_creation_service.go:1664-1670`:
```
orderItem := orderentity.NewOrderItem(order.ID, listing.ID, listing.PricePerUnit, input.Quantity, listing.Title)
```
`listing` is the `FixedPriceSale` entity loaded by `listingRepo.GetForUpdate(input.SourceID)` (`order_creation_service.go:1351`), so `listing.ID` is `fixed_price_sales.id`. The request-supplied `input.ProductID` (products.id) is validated (`:1437` negotiation match check) but **never persisted** in this path.

### B. Auction order stores Product ID (PROVEN)
`order_creation_service.go:979-985`:
```
orderItem := orderentity.NewOrderItem(order.ID, product.ID, winningBidAmount, 1, product.Title)
```
`product` is loaded via `s.productRepo.GetByID(ctx, tx, input.ProductID)` (`order_creation_service.go:764`). `product.ID` is `products.id`.

### C. Negotiation uses the FPS namespace (PROVEN)
Negotiation checkout is not a third writer. It funnels into `CreateFromSaleSurface` with `SourceType: OrderSourceFixedPriceSale`, `SourceID: fixedPriceSale.ID`, `ProductID: fixedPriceSale.ProductID` (`chat_handler.go:2106-2118`), and the item is built at `order_creation_service.go:1664-1670` → **`listing.ID`**. Note: `CreateFromSaleSurface` itself rejects any `SourceType != OrderSourceFixedPriceSale` (`order_creation_service.go:1531-1533`), so the enum value `OrderSourceNegotiation` (`order/entity/order_source_type.go:20-21`) is **never written by production code** — negotiated orders are persisted with `source_type='fixed_price_sale'` and `negotiation_id` set. PROVEN.

### D. `CountActiveOrdersByProduct` receives Product ID and misses FPS orders (PROVEN)
- SQL: `INNER JOIN order_items oi ON o.id = oi.order_id WHERE oi.product_id = $1 AND o.status IN ('pending_payment','paid','shipped','delivered')` (`order_repository_extensions.go:399-405`).
- Caller 1 (shipping option set): `s.orderRepo.CountActiveOrdersByProduct(ctx, tx, input.ProductID)` — `input.ProductID` is a **product** id (`backend/internal/commerce/shipping/application/listing_shipping_service.go:70-83`, product resolved at `:71`).
- FPS-sourced rows store `fixed_price_sales.id` → the join can never match the passed products.id → **FPS active orders are invisible to the shipping change guard**. Only auction-sourced rows (which store products.id) match. PROVEN.

### E. Other consumers with the same mismatch (PROVEN)
1. **`CountAnyOrdersByProduct`** (`:413-419`) — no source-type filter. Called from the FPS **edit** guard with `listingID` (an FPS id, `fixed_price_sale_handler.go:402`). Under the dual namespace it happens to match FPS orders and misses auction orders on the same product. The interface doc says "counts any orders for a product" (`order/repository/order_repository.go:99-101`) — the doc intent is product-keyed; current behavior is listing-keyed for FPS and product-keyed for auction, i.e. the SAME function means different things depending on which caller keys it. PROVEN mismatch.
2. **Stock restore** (`order_completion_service.go:1986`) — the only resolver-style consumer. It is currently *internally consistent* (FPS namespace for FPS orders) only because of the `SourceType` branch (`:1953-1958`). The PASS_20B comment confirms the prior failure mode when the column did not match the lookup (`:1940-1948`).
3. **DTO passthrough** (`decision.go:668-677`, `admin_order_handler.go:483`) — the `product_id` JSON value for FPS orders is actually the FPS id. No backend consumer dereferences it (name is a persisted snapshot), so today this is a *data-contract* lie, not a live join error.

---

## 6. DATABASE CONSTRAINT / FK ANALYSIS

Current `order_items` DDL state (re-derived from `000001` + later migrations):

| Item | DDL | Status |
|---|---|---|
| Columns | `id` PK uuid, `order_id` uuid NOT NULL, `name` text NOT NULL, `unit_price_snapshot` bigint NOT NULL, `quantity` int NOT NULL, `product_id` uuid (**nullable**), `created_at` | `000001:1024-1033` |
| PK | `order_items_pkey (id)` | `000001:1901` |
| FK | `order_id → orders(id) ON DELETE CASCADE` | `000001:2358` |
| FK | `listing_id → listings` | **removed** `000010:33-35` (column + FK + index dropped) |
| FK | **on `product_id`: NONE** | confirmed — no FK, no unique |
| CHECK | `quantity > 0` (`:2479`), `unit_price_snapshot >= 0` (`:2480`) | present |
| Index | `idx_order_items_order_id` (`:2133`), `idx_order_items_product_id` (`:2134`) | present |

Migration history relevant to `order_items`: only `000010` touched it (dropped legacy `listing_id`). No migration ever wrote or validated the namespace of `product_id`.

**Adding `product_id → products(id)` FK:** not yet possible — existing FPS-sourced rows contain `fixed_price_sales.id` values; any `REFERENCES products` constraint would fail on them. Order of operations (future implementation): backfill rows to products.id → `SET NOT NULL` → add `REFERENCES products(id)` + `ON DELETE RESTRICT` (product deletion is already blocked by `fixed_price_sales.product_id`/`auctions.product_id` RESTRICT FKs, so an additional RESTRICT is consistent and non-breaking). Structural safety: **PROVEN safe with backfill**; products referenced by every FPS/auction row must exist (mint or reuse), and products can never be hard-deleted while a surface exists.

**No existing column already stores the selling-surface identity at item level** — but it does not need to: `orders.source_type` (`order_source_enum`, `000001:204-210`) and `orders.source_id` are persisted for every order (`order_repository.go:56-84`) and are exact/namespaced (`fixed_price_sale`→FPS id, `auction`→auction id). An order is single-source and single-item in practice (single `NewOrderItem` per order). Category: **surface provenance already exists; do not build a duplicate.**

---

## 7. RUNTIME IMPACT ANALYSIS

Bugs live today (silent, not exploding):

- **Shipping change guard is void for FPS orders** (`CountActiveOrdersByProduct` + product-id caller): sellers can alter shipping options on a product while a paid/active fixed-price order still references it, because FPS rows are unreachable by products.id. Auction orders are (partially) protected; FPS are not. Real correctness gap under the stated invariant ("cannot update shipping: active orders exist", `listing_shipping_service.go:81-89`).
- **FPS edit lock is product-incomplete** (`CountAnyOrdersByProduct(listingID)`): prevents price/title/quantity edits only when an FPS-sourced order exists; an auction-sold order on the same product does not lock the FPS edit. Whether that is desired is an owner ratifiable nuance (§12).
- **DTO mislabeling:** buyer/admin order detail shows `product_id` = FPS id for FPS orders. Harmless today (no dereference) but contractually wrong.

Behavior that must be preserved carefully when fixing:

- **Stock restore** (`Cancel`/`CancelOverdue`/`Expire` → `restoreListingStock` → `restoreFixedPriceListingStock`) currently *depends on* the FPS namespace (`order_completion_service.go:1986`). After `product_id` → `products.id`, this lookup breaks unless rewritten to resolve the listing from the order's `source_id` (single source per order, already the branch pattern used for auctions). This is the single most sensitive consumer (§8 #1).
- **CountActive/CountAny** callers must be keyed consistently (§8 #2, #3), or the guards silently invert on the other namespace.

---

## 8. EXACT FILES/FUNCTIONS THAT WOULD NEED MODIFICATION (implementation surface — NOT changed now)

Writers:

1. `backend/internal/commerce/order/application/order_creation_service.go` — `CreateFromSaleSurface`, order-item construction at `:1664-1670`: pass `listing.ProductID` (products.id) instead of `listing.ID`. (`listing.ProductID` already exists on the joined entity, `fixed_price_sale_repository_impl.go:123-126,584`.)

Consumers (must be adapted in the same stage):

2. `backend/internal/commerce/order/application/order_completion_service.go` — `restoreFixedPriceListingStock` (`:1972-2003`): stop resolving the surface via `item.ProductID`; resolve via `orders.source_id` (already `FOR UPDATE`-locked order is in scope; source is single per order). Retain the `SourceType` branch (`:1953-1958`) as-is for auctions.
3. `backend/internal/commerce/order/delivery/http/order_handler.go` / `admin_order_handler.go` / `dto/decision.go` — no code change required (passthrough), but the emitted `product_id` value for FPS/negotiation orders changes from FPS id to products.id → verify client-side contract (§9 #3).
4. `backend/internal/commerce/fixedprice/delivery/http/fixed_price_sale_handler.go` — FPS edit guard at `:402`: key `CountAnyOrdersByProduct` by `listing.ProductID` (products.id) so the guard becomes product-wide (matching the interface doc), after the writer fix makes that query see FPS rows again.
5. `backend/internal/commerce/shipping/application/listing_shipping_service.go` — `:83` needs **no change**; after the writer fix it starts correctly counting FPS orders too.
6. `backend/internal/commerce/order/infrastructure/repository/order_repository_extensions.go` — `CountActiveOrdersByProduct`/`CountAnyOrdersByProduct` SQL (`:399-405`, `:419-423`) needs no change; their callers §#4/#5 determine semantics.

Schema (future, separate stage): backfill `UPDATE order_items SET product_id = fps.product_id FROM fixed_price_sales fps WHERE fps.id = order_items.product_id` (FPS rows only; auction rows already correct), then `NOT NULL`, then optional `REFERENCES products(id)`.

Tests to update/extend (future): order-item identity integration assertions (FPS order item product_id == products.id), restore-path test, Count* guard tests — none currently assert the stored namespace (verified: no test INSERTs into `order_items`; only full-flow integration tests exist), so the assertion surface is additive, not corrective.

---

## 9. RISKS OF CHANGING THE FIELD

| # | Risk | Severity | Mitigation (when implementing) |
|---|---|---|---|
| 1 | Stock restore breaks for FPS/negotiation orders if rewritten carelessly (GetForUpdate with products.id → no row → whole Cancel/Expire tx errors) | **HIGH** | Resolve surface from `orders.source_id`; single-source guarantee makes it deterministic |
| 2 | FPS edit guard loses matching if left passing listingID (rows now products.id) → edit-lock silently void | **HIGH/MEDIUM** | Rewrite guard key to `listing.ProductID` in the same stage |
| 3 | API `product_id` value change for FPS order detail (buyer/admin/mobile display). Mobile stores it as item id/productId (`order_mapper.dart:96-105`) with **no navigation** attached (no GoRouter/push uses found) | MEDIUM | Verify mobile displays before/after; flag to mobile owner during rollout |
| 4 | Mixed namespace across the deploy window: historical rows (FPS-id) vs new rows (product-id) coexist until backfill | MEDIUM | Sequence: backfill first (or same deploy), then code switch; count-guards tolerate either until both sides move |
| 5 | Backfill of corrupted/legacy rows (e.g. product_id already NULL or unusable) | LOW | Fail-closed backfill (transaction + row-count assert) |
| 6 | No current test asserts the namespace → change could regress silently | LOW | Add explicit integration assertion in same stage |
| 7 | Finance/coins/refund/commission: **no exposure** (never read `order_items`) | NONE | n/a |

---

## 10. IS A DEDICATED `selling_surface_id` REQUIRED?

**NO.** `orders.source_type` + `orders.source_id` already store the exact selling-surface identity (`order_repository.go:56-84`), are set by every writer (§3), and every order is single-source/single-item. Adding `order_items.fixed_price_sale_id`/etc. would duplicate provenance already one level up and re-create the legacy duplication that was deliberately removed. PROVEN — do not build it.

---

## 11. CAN `order_items.product_id` SAFELY BECOME `products.id`?

**YES — structurally proven safe** after, and only together with:
1. Writer change (§8 #1) so new FPS/negotiation rows store `products.id`.
2. Restore-path rewrite (§8 #2) to source-keyed resolution.
3. Guard-key rewrite (§8 #4).
4. Backfill of historical rows, then optional `NOT NULL` + `REFERENCES products(id)` (§6, §8).

Schema is currently permissive (no FK, nullable, indexed) and contains no uniqueness constraint that would conflict. Products are never hard-deleted while referenced. The two namespaces are distinguishable row-wise today only by joining `orders.source_type`, which is exactly why the column must be normalized.

---

## 12. BUSINESS CONTRADICTION REQUIRING OWNER DECISION

No irreconcilable business contradiction was found. Two intent ratifications are recommended (recorded, not decided):

1. **Edit-lock and shipping-lock scope.** Should the FPS-edit guard and the shipping-option guard be **product-wide** (an active order for the same Product, in either surface, blocks edits/shipping changes — the natural Model B reading of "product"), or **surface-scoped** (only the surfaced used by the order blocks edits)? Current DB counts are surfaced-mis-keyed and cannot answer. Recommended (Model B): product-wide. **OWNER DECISION REQUIRED** to lock the semantic, since it changes which orders block a seller action.
2. **`order_items.product_id` display contract.** Whether buyer/admin order-detail should continue to expose a value named `product_id` that for auction orders was already products.id and (after fix) will be products.id for all — i.e., no owner decision needed here beyond confirming mobile display is identifier-only (it is: no navigation consumer found). **OWNER NOTIFICATION only.**

No other business-level ambiguity: Model B + the existing API contract resolve the canonical meaning (`products.id`); negotiation is surface-checkout (not a third namespace); finance is untouched.

---

## 13. SUMMARY OF PROOF INTEGRITY

- `go build ./internal/... ./cmd/...` — production code compiles (audit baseline).
- Every producer/consumer above was re-read from the current working tree; no claim relies on Git history or the Stage-4 report text.
- Only inference flagged as such: the *historical origin* of the drift (legacy `order_items.listing_id` column dropped at `000010:33-35` while the writer kept passing `listing.ID` into the remaining identity column) — inferred from DDL + current code; treated as context, not as load-bearing authority.
- No file was modified. No migration was created. No rename was performed. STOP condition honored.