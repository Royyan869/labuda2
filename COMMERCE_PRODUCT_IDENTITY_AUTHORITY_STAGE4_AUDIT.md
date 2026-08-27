# COMMERCE — STAGE 4
# Product Identity & Selling-Surface Authority Audit

**Mode:** READ-ONLY audit. No production code, no migration, no rename, no refactor was performed.
**Truth source:** current filesystem + applied migration chain (`backend/migrations/000001` base … `000044`) + production Go compile check + targeted test-compile checks.
**Date:** 2026-08-24

---

## 1. EXECUTIVE VERDICT

**C. PRODUCT_MODEL_HAS_ARCHITECTURAL_CONTRADICTION**

The Product → FixedPriceSale → Auction skeleton is **coherent and DB-enforced** (single active selling surface per product, product reuse, cross-surface relist, app-level seller ownership guards). Stage 3 cleanup is confirmed complete: **Product carries no hidden lifecycle authority.**

The contradiction that forces verdict C is the **order identity namespace**:

> `order_items.product_id` stores **fixed_price_sales.id** for FPS/negotiation orders and **products.id** for auction orders — one column, two identity namespaces, with consumers that label it "product_id" and resolve it as both kinds of id.

Secondary gaps that require owner decision before closure: **quantity has no product-level ledger** (per-surface, fabricated per relist, non-survivable across surface switch), **title/description/prep-time diverge between `auctions` and `products` on edit**, and **`fixed_price_sale_media` / `auction_media` are read-but-never-written** (stale snapshot tables consumed by chat projection).

The model is NOT corrupt end-to-end; the contradiction is localized and repairable. Verdict C, not D.

---

## 2. PRODUCT SEMANTIC MEANING

**Answer (behavior-derived, not doc-derived):** Product is a **seller-owned canonical commerce identity for a sellable item, intentionally sale-surface agnostic.**

- Entity comment: `backend/internal/commerce/product/entity/product.go:9-11` — "Product is the internal physical item authority. It is intentionally sale-surface agnostic."
- Physical columns: `products` = `seller_id, title, description, media_urls, variety, size_cm, age_months, gender, breeder, bloodline, certificates, farm_address_id, preparation_time, preparation_note` + timestamps. `backend/migrations/000001_canonical_schema.up.sql:1322-1342`. **No quantity, no price, no status, no sold_at.**
- Product creation writers (both mint atomically with the surface, both in-transaction):
  - FPS path: `backend/internal/commerce/fixedprice/infrastructure/repository/fixed_price_sale_repository_impl.go:33-81` (mint at 60-74).
  - Auction path: `backend/internal/commerce/auction/application/auction_service.go:283-304`.
- Product is **not** a standalone consumer surface: `product` package has repository only, no delivery/HTTP layer, no `/products` route (`backend/internal/commerce/product/` tree; route grep proves zero product routes). PROVEN.
- Product is exposed to buyers only **through a selling surface** (FPS = `/listings`, Auction = `/auctions`; `backend/cmd/core_server/routes_core.go:145-153,265-283,313-317`).

**Classification:**
| Option | Verdict |
|---|---|
| Physical unique item | NOT PROVEN — no unit-ledger, quantity can be 10 (see §7) |
| Reusable catalog definition | PROVEN — reuse across surfaces and relists is DB-proven and integration-tested |
| Stable commerce identity | PROVEN — `products.id` survives sold FPS / ended auction / relist / surface switch (§3) |

PROVEN. A "physical item authority".

---

## 3. PRODUCT IDENTITY LIFECYCLE

**1. What creates a Product?** Only two writers:
- `FixedPriceSaleRepositoryImpl.Create` mint path (`fixed_price_sale_repository_impl.go:60-74`).
- `AuctionService.CreateDraft` mint path (`auction_service.go:283-304`).
`go build ./internal/... ./cmd/...` passes; only these two call `productRepo.Create`.

**2. When is a Product created?** Atomically with the first FPS/auction that does not supply an existing `product_id`. Both create flows accept an **optional** `ProductID`:
- FPS: `CreateFixedPriceSaleInput.ProductID *uuid.UUID` (`fixed_price_sale_service.go:106-140`, reuse wiring at 240-242).
- Auction: `CreateDraftInput.ProductID*` (`auction_service.go:192-193`, reuse at 270-282).

**3. Can a Product be reused?** PROVEN. Reuse attaches the new surface to the existing product row, no duplicate mint: `fixed_price_sale_repository_impl.go:42-56`, `auction_service.go:270-282`. Runtime-proven: `backend/tests/product_identity_reuse_integration_test.go:117-200` (count stays 1 across reuse).

**4. Can one Product have multiple historical selling surfaces?** PROVEN. `uniq_active_fixed_price_sale_per_product` (`000001:2092`) and `uniq_active_auction_per_product` (`000001:2015`) are **partial** (only block active states). Multiple `sold/withdrawn/ended` rows per product are legal; test proves 2 FPS rows (sold + relist) on one product: `product_identity_reuse_integration_test.go:144-151`.

**5. Can one Product have both FPS and Auction over its lifetime?** PROVEN. FPS→sold→active auction on same product: `product_identity_reuse_integration_test.go:162-196`.

**6. Product identity survives:**
| Scenario | Verdict | Evidence |
|---|---|---|
| sold FPS | PROVEN | `product_identity_reuse_integration_test.go:143-150` |
| ended Auction | PROVEN | `insertAuction("ended")` allowed: `...:177` |
| relisting (new FPS after sold) | PROVEN | `...:144-151` |
| switching FPS→Auction | PROVEN | `...:188-196` |
| switching Auction→FPS | PROVEN by schema (ended auction is outside the trigger's active set; `000010:79-101`) — NOT integration-proven |

**7. What Product represents:** a stable seller-owned commerce identity ("physical item authority", `product.go:9-11`) reuseable across selling surfaces. PROVEN.

---

## 4. FPS AUTHORITY MAP (FixedPriceSale)

Authoritative storage: `fixed_price_sales` row = **only** `id, product_id, seller_id, price_per_unit, negotiation_enabled, status, published_at, sold_at, withdrawn_at, quantity_available, created_at, updated_at` (`000001:868-880` + `000009:15-16`). The FPS **entity** carries title/description/media/variety/… but those fields are materialized from the joined Product on every read (`fixed_price_sale_repository_impl.go:437-604`) and written back into Product on update (`...:133-184`).

| Fact | Product | FixedPriceSale | Auction | Other | Canonical? |
|---|---|---|---|---|---|
| seller | products.seller_id (FK `000001:2378`) | fps.seller_id (FK `:2341`) | auctions.seller_id (FK `:2285`) | — | **TWO+ authorities in DB**; cross-row equality enforced **only in app code** (§8) |
| product identity | products.id | fps.product_id FK RESTRICT (`:2340`) | auctions.product_id FK RESTRICT (`:2286`) | — | YES |
| title | products.title | none (entity field from join) | auctions.title | — | FPS→Product; **Auction→own column (CONTRADICTION class D, §7/§10)** |
| description | products.description | none | auctions.description | — | same as above |
| media | products.media_urls | fixed_price_sale_media (no writer, §10) | auction_media (no writer, §10) | — | Product; typed tables = DEAD residue |
| variety/size/age/gender/breeder/bloodline/certificates | products | none | none (auction detail drops them) | — | Product |
| shipping linkage | product_shipping_options (FK `:2375`) | — | — | — | Product |
| farm_address / prep_time / prep_note | products | entity field (product storage) | **auctions row (own columns `:485-486`)** | — | FPS→Product; **Auction→own column (diverge on edit)** |
| quantity | none | fps.quantity_available (`000009:15-16`) | none (implicit 1) | — | surface-scoped (§7) |
| price | none | fps.price_per_unit (check `:2469`) | auctions.start_price/bid_increment/buy_now_price | — | per-surface |
| availability/status | none post-000044 | fps.status (enum `:133-138`) | auctions.status (enum `:36-45`) | — | surface status authority (PROVEN: no product-status gate in `GetPublic`, `fixed_price_sale_repository_impl.go:313-330`) |
| selling config | — | fps.negotiation_enabled | — | — | per-surface |
| auction-specific fields | — | — | auctions start/bid/buy-now/timing/current_bid/winner/anti-snipe | — | Auction |
| lifecycle | none | fps.status state machine (`fixed_price_sale.go:128-173`) | auction status machine (`auction.go:56-98`) | — | per-surface |
| ownership/capability checks | app-level reuse guards (§8) | seller==caller in service (`fixed_price_sale_service.go:501`, handler `:390`) | seller==caller (`auction_service.go:458,538`) | — | app-level |

PROVEN (traced writers+readers, not field-name heuristics).

---

## 5. AUCTION AUTHORITY MAP

Auction row is the **self-sufficient sellable-entry record** (`000001:477-498`): it owns `title, description, preparation_time, preparation_note, start_price, bid_increment, buy_now_price, start_at, end_at, current_bid, current_winner_id, status, anti_snipe_extension_seconds` (added `000003`/`000004`), `order_id`, `settlement_deadline`, plus `seller_id, product_id`.

- Auction **does not** inherit product title/desc on read: `auction_handler.go:1283-1284` emits `a.Title/a.Description`; Product fields (variety, size…) are surfaced by a dedicated detail projection that **production never calls** — `auctionToDetailResponseWithSeller` is referenced only from `_test.go` (`auction_detail_response_projection.go:15`, tests at `..._projection_test.go:47`). PROVEN: the only invocation is tests.
- Auction update (`auction.go:480-536`) mutates only the auction row; **no product write**. So auction title edits do not reach `products.title`. Divergence authority (§10-D).
- Auction settlement/order: `CreateOrderFromAuction` (`auction_service.go:881-924`) → order item carries `product.ID` (`order_creation_service.go:979-985`).
- Media: auction entity has **no media field**; `auction_media` exists but has no production INSERT path (§10). Auction detail emits no media (`auctionToResponseWithSeller`, `auction_handler.go:1240-1299`).

| Fact | Authority | Canonical? |
|---|---|---|
| seller | auctions.seller_id | YES (FK) |
| product identity | auctions.product_id | YES (FK RESTRICT) |
| title/description | auctions.title/description | own-authority, **divergent** from products |
| prep time/note | auctions.preparation_* | own-authority, divergent |
| media | (none written; product.media_urls is the only populated store) | GAP |
| price fields | auctions.* | YES |
| lifecycle | auctions.status | YES |
| order binding | auctions.order_id (unique `auction_order_consistency` check `:2436`; `orders` OUTER) | YES |
| quantity | none; implicit 1 in checkout (`order_creation_service.go:806`, `NewOrderItem(..., 1, ...)` `:982-983`) | YES |

---

## 6. PRODUCT ↔ SELLING-SURFACE CARDINALITY

| Relationship | Actual | Enforcement |
|---|---|---|
| Product → 0/1 active FPS | YES | `uniq_active_fixed_price_sale_per_product` on `(product_id)` WHERE status IN (draft,active) — `000001:2092` |
| Product → 0/1 active Auction | YES | `uniq_active_auction_per_product` WHERE status IN (draft,scheduled,active,waiting_settlement) — `000001:2015` |
| Product → many historical FPS | YES | partial index only; test `product_identity_reuse_integration_test.go:150` |
| Product → many historical Auctions | YES | same partial-index logic (multiple ended rows allowed, no index blocks them) |
| FPS → exactly 1 Product | YES | `fps.product_id` FK RESTRICT `000001:2340` |
| Auction → exactly 1 Product | YES | `auctions.product_id` FK RESTRICT `000001:2286` |

**Two active surfaces on one Product — PROHIBITED at BOTH levels:**
1. **DB (same-table):** the two partial unique indexes above.
2. **DB (cross-table trigger):** `enforce_single_active_sale_channel_per_product` + triggers `trg_fixed_price_sales_single_active_channel` / `trg_auctions_single_active_channel` (`000010:76-112`) — checks the *other* table's active set on INSERT/UPDATE and raises `check_violation`.
3. **Runtime-proven:** `product_identity_reuse_integration_test.go:126-131,179-186` (duplicate active FPS rejected; FPS-active + auction-active rejected).

DB and application agree. No disagreement found. PROVEN.

---

## 7. QUANTITY SEMANTICS

**Storage locations (traced writers/readers):**
- Product: **no quantity column, no quantity field, no inventory/ledger anywhere in schema** (products table `000001:1322-1342`; schema-wide table list has no inventory/stock ledger).
- FPS: `quantity_available` added `000009:15-16`, `CHECK >= 0` (`000009:18-20`), backfilled sold/withdrawn→0 (`000009:27-29`). Default 1; handler defaults omitted `quantity` to 1 (`fixed_price_sale_handler.go:244`, request comment `:99-108`). Multi-quantity is a supported, persisted feature (`000009:1-14`; `quantity_persistence_test.go`).
- Auction: **no quantity field anywhere**; checkout hardcodes 1 (`order_creation_service.go:806`, order item `:979-983`).
- Order: `orders.quantity>0` (`000001:2490`), `subtotal = quantity * unit_price` (`:2486`).
- OrderItem: `order_items.quantity>0` (`:2479`).

**Answers:**
1. Quantity is stored **only on the FPS surface** (and as order/order-item snapshots). PROVEN.
2. Meaning: unit price × quantity on a sale; there is no durable unit registry. PROVEN.
3. Per **selling surface** (FPS), not per Product. PROVEN.
4. Quantity > 1: PROVEN possible on FPS (`000009` + entity `NewFixedPriceSale` requires `>=1`, `fixed_price_sale.go:344-346`; multi-qty orders flow through `ReduceQuantity`, `fixed_price_sale.go:235-264`).
5. Product with quantity 10 sold through multiple surfaces at once: **IMPOSSIBLE while active** — one-active-surface rule (§6) forbids simultaneous channels. **Sequentially**: possible, but each new surface starts a **fresh, caller-supplied quantity** — the 10 is not carried, decremented, or restored at Product level. NOT PROVEN as a coherent model.
6. Partial consumption: PROVEN per-surface (`ReduceQuantity`/`RestoreQuantity`; `order_creation_service.go:1504-1511`, restore at `order_completion_service.go:1972-2003`). Partial consumption is **not** visible at Product level.
7. True inventory/unit ledger: **does not exist**. PROVEN (schema has no such table; no Go type).
8. Product ⇒ unique physical unit: **FALSE** as an invariant — `quantity_available` may be 10, i.e. Product is a *multi-unit sellable definition*, yet the one-active-surface rule and auction-quantity-of-1 make that multi-unit state non-survivable across a switch to auction (a 10-unit FPS that withdraws with 7 left cannot carry "7" into an auction — an auction has no quantity). **This is an architectural gap (OWNER DECISION REQUIRED).**

CONTRADICTION/GAP: the model answers questions 1–6 inconsistently across surfaces; there is no single quantity truth. Owner must decide whether Product owns stock (ledger) or selling surfaces remain the sole quantity authority (documenting that relist = fresh stock).

---

## 8. SELLER AUTHORITY

- Product seller: `products.seller_id` (FK users, CASCADE `000001:2378`). No quantity/status fields that could act as implicit ownership.
- FPS seller: `fps.seller_id` FK users (`:2341`); `fps.product_id` FK products RESTRICT (`:2340`).
- Auction seller: `auctions.seller_id` FK users (`:2285`); `auctions.product_id` FK products RESTRICT (`:2286`).
- **No DB constraint, trigger, or CHECK requires surface.seller_id == products.seller_id** (full FK/CHECK dump of `000001`: none; `000010` trigger checks status only). PROVEN.
- **App-level reuse guards prevent mismatch on all write paths:**
  - FPS reuse: `if product.SellerID != listing.SellerID { return error }` — `fixed_price_sale_repository_impl.go:47-49`; runtime-proven rejection `product_identity_reuse_integration_test.go:198-206`.
  - Auction reuse: `if existing.SellerID != input.SellerID { return error }` — `auction_service.go:279-281`.
  - Mint path: product always minted with the caller's seller id (`fixed_price_sale_repository_impl.go:60-74`; `auction_service.go:284-304`).
- Edit paths: handler requires `listing.SellerID == callerID` (`fixed_price_sale_handler.go:390`, `auction_service.go:458,538`). Product Update writes `seller_id = listing.SellerID`, so ownership cannot drift through the guarded path.
- Order: FPS order seller = `listing.SellerID` (`order_creation_service.go:1602`); auction order seller = `product.SellerID` ("IMPORTANT: use canonical auction product seller" `order_creation_service.go:916`). Since app guards force surface.seller==product.seller at creation, both orders agree in practice.

**Conclusion:** seller identity is attached to **both** Product and each surface with identical values in practice; mismatch is prevented by **application code only**, not by the database. Not a live bug; DB-level laxity is architectural debt. PROVEN.

---

## 9. ORDER ITEM product_id IDENTITY AUDIT  (AUDIT 5)

### Writers (the only INSERT into order_items — `order_repository.go:850`, invoked via `finalizeOrderCreationTx`):

| Path | Column written | Evidence |
|---|---|---|
| FPS direct purchase | `order_item.product_id = listing.ID` (**fixed_price_sales.id**) | `order_creation_service.go:1664-1670` |
| Negotiation (FPS price override) | same path → `listing.ID` | negotiation routes through `CreateFromSaleSurface` (`chat_handler.go:700`), item built at `:1664-1670` |
| Auction (bid win / buy-now) | `order_item.product_id = product.ID` (**products.id**) | `order_creation_service.go:979-985`, product loaded at `:764` |

### Readers of `order_items.product_id` (Go):
| Consumer | Interpretation | Line |
|---|---|---|
| `GetOrderItems` | opaque passthrough → `OrderItem.ProductID` | `order_repository_extensions.go:300-341` |
| `restoreFixedPriceListingStock` | treats it as **FPS id**, `listingRepo.GetForUpdate(item.ProductID)` | `order_completion_service.go:1986` |
| `CountActiveOrdersByProduct` | untyped equality `oi.product_id = $1`, no source filter | `order_repository_extensions.go:399-405` |
| `CountAnyOrdersByProduct` | same | `order_repository_extensions.go:415-419` |
| FPS update guard | **calls** `CountAnyOrdersByProduct(ctx, tx, listingID)` (FPS id — matches FPS orders) | `fixed_price_sale_handler.go:402` |
| Shipping-change guard | **calls** `CountActiveOrdersByProduct(ctx, tx, input.ProductID)` (products.id) | `listing_shipping_service.go:83` |
| Order detail DTO | surfaces `item.ProductID` as `product_id` | `dto/decision.go:671` |
| Admin order detail | surfaces `item.ProductID` | `admin_order_handler.go:483` |

### Answer:
> **No. `order_items.product_id` does not represent one stable identity namespace.** It is `fixed_price_sales.id` for FPS & negotiation orders and `products.id` for auction orders.

**Affected consumers (deterministic breakage):**
1. **`CountActiveOrdersByProduct`/shipping guard** (`listing_shipping_service.go:83`): queried with `products.id`, but FPS-sourced rows store `fixed_price_sales.id` → **silently under-counts / cannot see FPS orders**; only auction-sourced orders would ever match. The guard is therefore non-functional for its primary (FPS) case.
2. **`CountAnyOrdersByProduct`/FPS-update guard** (`fixed_price_sale_handler.go:402`): queried with FPS id → sees only FPS orders (intended); auction orders on the same product are invisible to it (semantic mismatch if the guard is meant to be product-wide).
3. **`order_completion_service.go:1986`** is branch-saved from the dual namespace by the `SourceType` switch (`order_completion_service.go:1954-1958`), but the code comment admits the old path looked up `products.id` and failed auction orders (PASS_20B fix). The two namespaces are an acknowledged minefield; branches live only where someone patched them.
4. **Order detail JSON `product_id`** — mobile consumers read `json['product_id']` per order item (`apps/mobile/.../order_api_response_dtos.dart:264`) and interpret it as the item id for navigation/display; for FPS orders this value is the FPS id (mobile's `/listing/:id` convention matches `SearchResultType.listing`, `search_results_screen.dart:316`, so it "works" by accident of naming).

**Historical order identity determinism:** NOT POSSIBLE today from `order_items.product_id` alone. It is only resolvable by JOINing through `orders.source_type` (`order_source_enum`), i.e. `product_id` meaning is a *function of the sibling `source_type` row*. Any query treating the column as `products.id` is wrong for FPS rows and vice-versa.

**Why the column has no FK:** no single FK can target two tables — `order_items` has no FK on `product_id` (`000001:2357-2358` only order/listing FKs) — which is the structural symptom confirming the dual namespace.

**NOT FIXED. Finding only.** CONTRADICTION (see §13).

---

## 10. DUPLICATED-FIELD CLASSIFICATION

| Field | products | fixed_price_sales | auctions | Classification |
|---|---|---|---|---|
| `title` | canonical | **entity-only** field (reads join, writes back); no column | **own column** (`:483`) | FPS: A (Product authority). Auction: **D — competing authority that diverges on edit** |
| `description` | canonical | entity-only (same) | own column (`:484`) | same as title |
| `media` | `media_urls` (jsonb, canonical) | `fixed_price_sale_media` table (`000023:1-12`) | `auction_media` (`000023:14-25`) | fixed_price_sale_media/auction_media: **E/DEAD — created + one-time backfilled (`000023:27-71`), no production writer (grep: zero `INSERT INTO fixed_price_sale_media` / `INSERT INTO auction_media` in Go), yet READ by chat projection (`serverboot/chat_fixedprice_projection_resolver.go:126-130`, `chat_auction_projection_resolver.go:170`). Frozen snapshot authority = divergence (media edits land on products, chat shows stale or empty typed media)** |
| `seller_id` | yes | yes | yes | C — intentional duplicate (ownership denormalization), equality guarded only in app (§8) |
| `preparation_time/note` | yes | **entity-only** (product storage) | own columns (`:485-486`) | FPS: A. Auction: **D — divergent on edit** |
| `farm_address_id` | yes | entity-only (product storage) | none (read at checkout from product) | A |
| `variety/size/age/gender/breeder/bloodline/certificates` | yes | entity-only | none | A |
| `price_per_unit/start_price` | none | fps.price_per_unit | auctions.* | per-surface (C) — correct, not duplicate |
| `status/sold/withdrawn/timestamps` | none (removed 000044) | own | own | per-surface (C) |

Key nuance honored: FPS's entity-level `Title/Description/PricePerUnit/QuantityAvailable` are **not separate columns** — do not call them duplicates. They either read from Product (title/desc) or are surface-owned (price/quantity).

---

## 11. PRODUCT LIFECYCLE RESIDUE AUDIT (AFTER STAGE 3)

Prod-code semantic sweep for `product sold/available/withdrawn/ended/reserved/status/lifecycle`:
- **Clean.** No production logic implies a hidden Product lifecycle.
- Physical removal applied by `000044_product_lifecycle_removal.up.sql:16-21` (drop `idx_products_status`, `products.status`, `products.sold_at`, drop `product_status_enum`). Only remaining references are the base schema + down-migration.
- Catalog predicates are surface-status-only: FPS browse `WHERE fps.status='active'` (`fixed_price_sale_repository_impl.go:313-330`), search same (`:341`). Auction browse surface-status-only (`auction_repository.go:366-404`).
- `order_creation_service.go:841-843` explicitly documents the removed Product "sold" mirror.
- Product lifecycle-residue test present and green-oriented: `backend/tests/product_lifecycle_removal_integration_test.go` (asserts `product_status_enum`/`idx_products_status` gone, `:45-51`).

**But one DEAD code path survives Stage 3:**
- `FixedPriceSaleRepositoryImpl.Delete` still executes `UPDATE products SET status = 'withdrawn'` (`fixed_price_sale_repository_impl.go:259-266`) — **references the dropped `products.status` column → would fail at runtime with column-not-found**. It is unimplemented by the HTTP path (handler Delete delegates to `service.Withdraw` → `UpdateStatus`, `fixed_price_sale_handler.go:686-700`); no production caller of `repo.Delete` exists (interface `fixed_price_sale_repository.go:53-54`; grep confirms only test stubs + impl). **DEAD / live-latent-failure.** Should be excised or rewritten — noting, not fixing.

**Stage 3 verdict:** lifecycle removal is behaviorally complete. The `Delete` residue is a latent, unreachable SQL break. PROVEN (for clean lifecycle) + DEAD (for `Delete`).

---

## 12. NAMING / VOCABULARY AUDIT (no renaming performed)

**Term inventory — "listing"/"Listing":**

| Occurrence | Location | Class |
|---|---|---|
| HTTP routes `/api/v1/listings`, `/search/listings` | `routes_core.go:145-147`, `:313-317`; handler comments `fixed_price_sale_handler.go:95,347,585,...` | **ACTIVE API CONTRACT (external) — listing == FPS surface** |
| FPS service/repo/entity errors & comments ("create a new listing", "listing not available") | `fixed_price_sale.go:97-113`; `fixed_price_sale_service.go` throughout | canonical-in-English comments / active internal alias |
| `listingRepo` package alias for `fixedprice/repository` | `order_creation_service.go:13`; comment_service `listingApp` = `fixedprice/application` (`comment_service.go:12-13`) | LEGACY ALIAS (code alias only) |
| saved_items `target_type='listing'` | schema CHECK `000001:2515`; `saved_item.go:14`; hydration via FPS join (`saved_item_repository_impl.go:196-216`) | ACTIVE API CONTRACT; **target id namespace = fixed_price_sales.id** (PROVEN) |
| discount_targets / discount `applies_to='listing'` | schema CHECK `000001:2452`; `discount.go:50`; handler `:280-325` | ACTIVE API CONTRACT; resolves against FPS |
| negotiation resource type | schema enum `negotiation_resource_enum` has `'listing'/'auction'` (`000001:192-195`), extended with canonical `'fixed_price_sale'` (`000002:20`). Go now writes only `fixed_price_sale` (`negotiation_resource_type.go:8`). | LEGACY enum values still in DB (dead values); negotiation code canonical |
| `order_source_enum` values `'listing','seller_quote'` | `000001:204-210` | SCHEMA RESIDUE (Go never writes them; Go enum `order_source_type.go:11-25` writes `fixed_price_sale|negotiation|auction`) |
| orphan DB enum **types** `listing_origin_enum, listing_status_enum, listing_type_enum, listing_visibility_enum` | created `000001:145-168`, only referenced by dropped `listings` table (`000001:947-953`); **never dropped** | DEAD schema residue |
| dropped tables `listings`, `listing_shipping_options`, `listing_views`, `order_items.listing_id`, `auctions.listing_id`, `pricing_tokens.listing_id`, `shipping_quotes.listing_id` | removed `000010:26-59`; `000016` | LEGACY (removed) |
| comment commerce references | `comment_commerce_references(fixed_price_sale_id, auction_id)` — canonical (`000031:13-27`); `comments` legacy `fixed_price_sale_id/share_reference/type` dropped (`000031:98-107`) | comment layer: CANONICAL; legacy comment fields DEAD/removed. Legacy doc-naming in `comment_response.go` `ListingPreview` (`:13-31`) | STALE DOCUMENTATION |
| mobile `ShareTargetType.fixedPriceSale` → path `'/listing'` | `apps/mobile/lib/shared/attachment/entities/share_reference.dart:32-36` | ACTIVE alias (runtime path `/listing/{id}`), canonical Contact `fixedPriceSale` |
| mobile `SearchResultType.listing` + tab "Listings", route `/listing/:id` | `search_results_screen.dart:60,316`; `search_result_type_helper.dart:44` | ACTIVE API/concept alias |
| mobile L10n "Listing"/"Jual Koi (Listing)" | `create_content_bottom_sheet.dart:181-209`; localizations | consumer-facing canonical customer term |
| `fixed_price_sale` (canonical backend term) appears across entities, DTOs, `sale_surface_type_enum` (`000001:296-300`), route response block `"fixed_price_sale":` (`fixed_price_sale_response_projection.go:135`) | — | CANONICAL |
| "listing" = shipping **source** value `"listing"` vs `"shipping_quote"` | `order.go:329`, `decision.go:351`, `auction_handler.go:883` | separate domain (shipping source), canonical there — do not rename with the FPS rename |

**Is `listing` an independent domain concept?**
> **NO.** Every production resolver maps `listing` onto `fixed_price_sales`: saved-item hydration (`saved_item_repository_impl.go:207`), comments (`comment_service.go:414` uses `FixedPriceSaleService.GetByID`), discounts (`discount_repository.go:499,558,579`), negotiation (moved to `fixed_price_sale`, `000002:20`), routes (mount FPS handlers). `listing` is the **customer-facing alias for the FixedPriceSale selling surface**; `FixedPriceSale` is the canonical internal name. PROVEN.

Final naming decision is deferred (owner), per scope.

---

## 13. CONTRADICTIONS FOUND

1. **[CONTRADICTION] `order_items.product_id` — dual identity namespace.** FPS/negotiation orders store `fixed_price_sales.id`; auction orders store `products.id`. No FK. Readers either branch by `source_type` (`order_completion_service.go:1954-1958`) or silently assume one namespace (`listing_shipping_service.go:83` misses FPS orders). Historical order identity is not deterministically interpretable from the column alone. §9.
2. **[CONTRADICTION / architecture gap] Quantity has no product-level truth.** Product has no quantity; FPS owns quantity per-surface (fresh on every relist); auction is implicit 1; no inventory ledger; multi-unit product cannot carry remaining stock across a surface switch. §7.
3. **[D-authority] Auction title/description/preparation_* diverge from Product on edit.** `UpdateDraft/UpdateScheduled` write only `auctions` (`auction.go:480-536`); product keeps stale values, so an FPS relist shows old text while the auction showed new text. §4/§5/§10.
4. **[E/DEAD] `fixed_price_sale_media` & `auction_media` are read but never written** in production (only migration backfill `000023` + tests); chat projection consumes stale/frozen data. There is no matching writer. §10.

---

## 14. PROVEN-GOOD INVARIANTS

- Product identity survives sold-FPS / ended-auction / relist / FPS↔auction switch (runtime test `tests/product_identity_reuse_integration_test.go`).— PROVEN
- One active selling surface per product is DB-enforced (2 partial unique indexes + cross-table trigger `000010:76-112`), match the app's intent; DB and app agree. — PROVEN
- FPS → exactly 1 product, Auction → exactly 1 product (FK RESTRICT). — PROVEN
- Product reuse is non-minting and ownership-guarded in both create paths. — PROVEN
- Product no longer carries a selling lifecycle (post-000044); browse/search/release gates use surface status only. — PROVEN
- Quantity persistence for FPS (>1) works (000009, `quantity_persistence_test.go`). — PROVEN
- FPS seller reuse of another seller's product is rejected, data untouched. — PROVEN (test `:198-206`)
- Production code compiles: `go build ./internal/... ./cmd/...` clean. — PROVEN
- Order source type is reliably recorded separately (`orders.source_type`), which is the only deterministic disambiguator for the dual-namespace product_id. — PROVEN

---

## 15. UNKNOWN / NOT PROVEN

- **Auction→FPS relist on same product** (reverse switch) is legal by schema but not integration-proven. Leader: add coverage when testing is sanctioned.
- **Non-FPS/waiting_settlement interplay for relist of a partially-purchased item** — no owner policy exists (quantity metering across surfaces). NOT DEFINED.
- **`fixed_price_sale_media`/`auction_media` intended role** (snapshot for chat? dead?) — unknown; needs owner decision (§16.4).
- **Whether `CountAnyOrdersByProduct` should be product-wide** or listing-wide — semantic intent unstated; behavior currently namespace-dependent. UNKNOWN (owner).
- `go test` over the following packages **does not compile** (stale tests referencing removed APIs):
  - `internal/commerce/auction/application` — `Auction.Media` field, `addressRepo` field, `ErrAuctionFarmAddressNotConfigured` (`auction_sender_address_test.go:39,106`; `auction_service_authority_test.go:53-123`).
  - `internal/commerce/fixedprice/application` — `CreateFixedPriceSaleInput.PublishNow` (`fixed_price_sale_create_sender_address_test.go:188,229`).
  - `internal/commerce/fixedprice/infrastructure/repository` — `normalizeSaleMedia` undefined, `FixedPriceSale.Media` removed (`fixed_price_sale_repository_media_test.go:21,48,55,74`).
  - `internal/interaction/chat/application` — `Service.resourceAuthorizer` (`chat_room_event_resource_projection_test.go:207,308`).
  These are **test residue** (not production breakage; `go build` is clean) and the "existing tests just-gone-green" baseline is therefore false for these suites. Recorded, not fixed. — PROVEN (compile errors reproduced with `go test -run XXXNoSuch`).

---

## 16. OWNER DECISIONS REQUIRED

1. **Order identity authority (must-fix before further order work).** Choose the single meaning of `order_items.product_id`:
   - (a) always `products.id` + add an explicit surface-reference column (`fixed_price_sale_id`/`auction_id`) — restores a true Product-keyed ledger; or
   - (b) rename/repurpose to `surface_id` FPS-sourced + keep product for auction (formalize current accidental behavior).
   Decision gates migration + every §9 consumer.
2. **Quantity semantics.** Does Product own a stock ledger (units that survive relist/surface-switch) or is stock fabricated per surface (relist = new stock, unsold quantity forgone)? This decides whether a multi-unit Product can ever enter an auction and how partial consumption recurs on relist.
3. **Title/description/prep-time authority.** Single author (edit synced down to `products`, auction becomes read-only copy of product) vs. per-surface snapshot (auction keeps own text; document that product stays stale). Also fixes the FPS-vs-auction asymmetric behavior.
4. **Typed media tables.** Delete `fixed_price_sale_media`/`auction_media` + point chat projection at `products.media_urls`, or restore real writers (rebuild typed media on every create/update). Current read-but-never-write state is a trap.
5. **Vocabulary.** Decide whether "listing" stays as the public/API term for the FPS surface (rename routes/DTOs/target types is the alternative). Also purge-able residue flagged for a future cleanup stage: orphan enums `listing_origin/status/type/visibility_enum`, dead `order_source_enum` values `listing/seller_quote`, dead `negotiation_resource_enum` value `listing`, and the dead `FixedPriceSaleRepositoryImpl.Delete` / `GetByProductID` methods.

---

## 17. RECOMMENDED NEXT STAGE

**Stage 5: order-identity closure** (after your decision on §16.1): migration for `order_items` identity normalization, rewrite of `CountActiveOrdersByProduct`/`CountAnyOrdersByProduct` callers and `restoreFixedPriceListingStock`, DTO fix, plus consumer tests. Secondarily, a **media-authority** closing pass (§16.4) and a **vocabulary residue purge** (§16.5) as separate, owner-optional stages.

---

## 18. KEY FILE:LINE EVIDENCE INDEX

- `backend/migrations/000001_canonical_schema.up.sql:477-498` auctions; `:868-880` fixed_price_sales; `:1024-1033` order_items; `:1322-1342` products (old status/sold_at); `:2007-2015` auction indexes incl. `uniq_active_auction_per_product`; `:2089-2092` FPS indexes incl. `uniq_active_fixed_price_sale_per_product`; `:2340/2341/2378` FKs; `:2452/2515` check enums; `:2469-2498` FPS/products checks.
- `backend/migrations/000009_fixed_price_sale_quantity_persistence.up.sql:15-29`
- `backend/migrations/000010_product_sale_channel_canonicalization.up.sql:26-59,76-112`
- `backend/migrations/000016_purge_legacy_listing_shipping_options.up.sql`
- `backend/migrations/000023_typed_commerce_media_authority.up.sql:1-71`
- `backend/migrations/000031_comment_commerce_reference_canonical.up.sql:13-107`
- `backend/migrations/000044_product_lifecycle_removal.up.sql:16-21`
- `backend/internal/commerce/product/entity/product.go:9-29`
- `backend/internal/commerce/product/infrastructure/repository/product_repository_impl.go:23-79,93-155`
- `backend/internal/commerce/fixedprice/infrastructure/repository/fixed_price_sale_repository_impl.go:33-81,133-184,252-280,313-330,606-631`
- `backend/internal/commerce/fixedprice/application/fixed_price_sale_service.go:106-140,240-262`
- `backend/internal/commerce/fixedprice/entity/fixed_price_sale.go:128-302`
- `backend/internal/commerce/auction/application/auction_service.go:230-391,881-924,526-606`
- `backend/internal/commerce/auction/infrastructure/repository/auction_repository.go:30-67,469-492`
- `backend/internal/commerce/auction/entity/auction.go:56-98,480-536`
- `backend/internal/commerce/order/application/order_creation_service.go:655-770,979-985,1140-1222,1253-1700 (order item `listing.ID` at 1664-1670)`
- `backend/internal/commerce/order/application/order_completion_service.go:1938-2003`
- `backend/internal/commerce/order/infrastructure/repository/order_repository_extensions.go:300-411`
- `backend/internal/commerce/shipping/application/listing_shipping_service.go:83`
- `backend/internal/commerce/fixedprice/delivery/http/fixed_price_sale_handler.go:390-423,402,659-717`
- `backend/internal/commerce/order/delivery/http/dto/decision.go:668-677`
- `backend/internal/commerce/fixedprice/delivery/http/fixed_price_sale_response_projection.go:25-49,215-235`
- `backend/internal/commerce/auction/delivery/http/auction_handler.go:1282-1299`
- `backend/cmd/core_server/routes_core.go:145-153,265-317`
- `backend/internal/interaction/saved_item/infrastructure/repository/saved_item_repository_impl.go:196-216`
- `backend/internal/social/content/application/comment_service.go:53,411-439`
- `backend/internal/ser verboot/chat_fixedprice_projection_resolver.go:126-130`; `.../chat_auction_projection_resolver.go:170`
- `backend/tests/product_identity_reuse_integration_test.go:105-206`
- `backend/tests/product_lifecycle_removal_integration_test.go:45-51`
- Mobile: `apps/mobile/lib/domains/commerce/transaction/order/data/models/api/order_api_response_dtos.dart:264`; `.../order/data/mappers/order_mapper.dart:98-111`; `share_reference.dart:32-36`

---

*Report finished. STOP condition honored: no implementation, no rename, no schema change, no refactor performed.*