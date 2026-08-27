# COMMERCE_PRODUCT_DOMAIN_ARCHITECTURE_AUDIT

READ-ONLY audit. No code, schema, docs, tests, or config modified.
Scope: Amazon-grade domain-model sanity of the Labuda commerce core: `Product`, `FixedPriceSale`, `Auction`, and the ghost of `Listing`.

---

## 1. Executive Verdict

**VERDICT: `COMMERCE_PRODUCT_ARCHITECTURE_HAS_OWNER_DECISION`**

The old Product→Listing→Auction unified `listings` table is structurally gone (migration `000010` + `schemaguard` guard + runtime migration tests prove it). The live model IS `products` (physical koi item authority) with sibling `fixed_price_sales` and `auctions`. That structural convergence is real, not cosmetic.

But the vocabulary is not converged, and no existing source in the repo decides it. Three competing canonical vocabularies exist in-repo:
- **PRD**: "Fixed Price Sale" (`PRD.md:301-307`).
- **Commerce doctrine/contracts**: "Product / **FixedPriceListing** / Auction", with an explicit "MUST NOT collapse back into a single listing model" (`docs/flows/doctrine/commerce-selling-doctrine.md:9-13,29`; `docs/contracts/commerce-selling-contract-notes.md:15-19`).
- **Schema + code + every audit report + PRD-adjacent docs**: "Product / **FixedPriceSale** / Auction" (tables `products`, `fixed_price_sales`).

Additionally there are **two genuine modeling contradictions** that are not naming-only and are unresolved:
1. `order_items.product_id` stores **fixed_price_sales.id for FPS purchases and products.id for auction purchases** in the same column (`order_creation_service.go:1677` vs `:992`) — same column, two id-spaces.
2. `Product` is documented "intentionally sale-surface agnostic" (`product.go:9-10`) yet carries a sale-coupled `status`/`sold_at` directly mutated by order lifecycle (`order_creation_service.go:847-848`, `order_completion_service.go:2037-2040`) — doctrine vs behavior conflict.

A destructive refactor is NOT required — the Product/FPS/Auction split is the correct shape. What is required is (a) an **owner decision on ONE canonical noun**, (b) two **targeted identity/lifecycle fixes**, and (c) a **vocabulary purge** of the `listing` alias across surfaces.

---

## 2. Current Domain Model

From schema + runtime paths:

```
products  (id, seller_id, koi metadata, media_urls, farm_address_id,
           preparation_time, status['draft|available|sold|withdrawn'], sold_at)
   │  product_id FK   (one product → many sale rows allowed; ONE active channel total)
   ├── fixed_price_sales  (price_per_unit, quantity_available, negotiation_enabled,
   │        status[draft|active|sold|withdrawn], published_at/sold_at/withdrawn_at)
   │        → product_shipping_options (product-scoped shipping)
   └── auctions  (start_price, bid_increment, buy_now_price, start_at/end_at,
            current_bid, current_winner_id, status[...], own title/description)
```

- `Product` is created **inline by the sale-channel writers** (`FixedPriceSaleRepositoryImpl.Create` mints product, `fixed_price_sale_repository_impl.go:38-51`; `AuctionService` mints product, `auction_service.go:260-278`). There is **no standalone Product create/read HTTP surface** — only `PUT /products/:id/shipping` (`routes_core.go:338-345`).
- One Product may have **one active cross-table channel**: `000010` trigger `enforce_single_active_sale_channel_per_product` forbids an active/pending FPS + auction on the same product, and per-table partial unique indexes forbid two active rows inside one table.
- Historical/distinct sale rows over one product are allowed (multiple draft/withdrawn/sold rows), so hunting-history and relisting-by-new-row are schema-possible but only partially documented/enforced.

---

## 3. Product Authority Audit

| Fact | Producer | Storage | Reader(s) | Mutation authority | Lifecycle authority |
|---|---|---|---|---|---|
| product identity | FPS repo / Auction service minted row | `products.id` | FPS join, auction join, order_items, pricing_tokens, shipping_quotes, saved_items hydration, comment FPS refs, feed/search | FPS/auction create flows only | not independently mutable |
| koi metadata (variety,size,age,gender,breeder,bloodline,certificates) | seller input via FPS/auction create | `products` | FPS detail (`scanJoinedSale`), auction, search/feed cards | FPS/auction create+edit | none |
| product media | seller upload via FPS/auction create | `products.media_urls` + typed commerce media (000022-24) | listing/auction detail, cards | FPS/auction edit | none |
| farm address / preparation_time | seller input | `products` (farm_address_id, preparation_time/preparation_note) | FPS entity duplicates fields; shipping validation | FPS/auction edit | none |
| product price | — | **none on products** | pricing token/order (snapshot) | server pricing authority | pricing token, NOT product |
| stock | — | **none on products** | FPS `quantity_available` instead | FPS | FPS/OrderService |
| sale state | order completion | `products.status` + `sold_at` (values `available`/`sold`) | order completion release | order lifecycle | order completion (contradiction with "surface-agnostic" — see §19) |

**Finding:** Product = the correct domain root as the *physical item authority*, but its lifecycle is NOT independent: `status`/`sold_at` is written by order lifecycle, contradicting its own doctrine (§19).

---

## 4. Fixed Price Sale Authority Audit

- **Storage**: `fixed_price_sales(id, product_id, seller_id, price_per_unit, negotiation_enabled, status, published_at, sold_at, withdrawn_at, quantity_available)` — `000001:868-880` + `000009` adds `quantity_available`.
- **Price**: `price_per_unit` on FPS — server-authoritative; checkout authority is the Pricing Token, not the display price (`docs/adr/008-listing-card-family.md:31`; `PRD.md:359-368`).
- **Stock**: `quantity_available` on FPS. Persisted by `000009`; guarded `>= 0`; `ReduceQuantity/RestoreQuantity` pair forced through OrderService.Cancel/Expire (`fixed_price_sale.go:220-287`). Owner decision (in-000009): multi-quantity FPS is a supported feature.
- **State machine**: `draft → active → sold|withdrawn`; `active = public only`; sold/withdrawn terminal (`fixed_price_sale_status.go`).
- **Name**: entity is `FixedPriceSale`; wire value `fixed_price_sale`; UI/API/paths call it `/listings`, `Listing`, `Produk Dijual` (see §17).
- **Entity duplication**: `FixedPriceSale` struct carries koi fields inline (`fixed_price_sale.go:29-35`); rows do NOT store them (joined from product by `scanJoinedSale`). Pure read-side duplication, not storage duplication.

**Finding:** FPS is the correct selling-mechanism for price/stock/negotiation state. Its NAME is the contested noun (§17, §26).

---

## 5. Auction Authority Audit

- **Storage**: `auctions(id, seller_id, product_id, title, description, preparation_time, start_price, bid_increment, buy_now_price, start_at, end_at, current_bid, current_winner_id, status, settlement_deadline, order_id)` — `000001:477-498` (listing_id dropped by 000010).
- **Price/bid**: `start_price`, `bid_increment`, `buy_now_price`, `current_bid` — auction-scoped. Post-win checkout authority = pricing token / claim pipeline (`docs/adr/009-auction-card-family.md:29`).
- **Stock**: single-item by canonical rule (`buy_now` qty 1); no stock counters (`docs/adr/009:111`).
- **State**: `auction_status_enum` (draft/scheduled/active/ended/cancelled/waiting_settlement + `expired_bnr` present in actual DB per `GLOBAL_DOMAIN_SURFACE_AUDIT.md:270`, which doctrine deletes — docs conflict, §21).
- **Identity**: auctions reference `products.id`; orders reference `auctions.id` as `source_id`; auction order_items reference `products.id` (§13).

**Findings:** (a) Auction **duplicates** title/description/preparation_time on its own row while also having `product_id` — FPS reads these from Product, Auction writes its own copy ⇒ duplicate authority for item title/description. (b) `expired_bnr` state: doctrine says collapsed (`commerce-db-model-split-design.md:128`), actual DB/audit says present — schema/doc contradiction.

---

## 6. Product ↔ Fixed Price Sale Relationship

- One product → many FPS rows allowed; **one active FPS row** per product (partial unique index + cross-table trigger). Sold/withdrawn/draft rows may accumulate historically.
- FPS is the **only** sale surface owning price + quantity. Product never carries price/quantity.
- FPS create mints the product atomically in one tx (no orphan product possible on that path).
- Sold FPS drives `products.status='sold'` via order completion (`order_creation_service.go:847-848`); FPS itself goes sold by `ReduceQuantity`→0 or completion.
- **Doc-vs-schema gap**: doctrine "A sold Product MUST NOT be reused for a new sale" (`commerce-selling-doctrine.md:67`) is NOT enforced anywhere (no DB constraint, no service gate found); schema permits a new draft/active channel row after a sold FPS if no active row exists. Enforced-in-code status: **NOT PROVEN** (no guard). Risk item, not a live contradiction since from-zero and flows may never trigger it.

## 7. Product ↔ Auction Relationship

- One product → many auction rows allowed; **one active/pending auction** per product; cross-table trigger prevents an FPS `draft/active` from coexisting with an auction `draft/scheduled/active/waiting_settlement` on the same product (`000010`).
- Unsold auction → `ended` returns product to `available` (`commerce-db-model-split-design.md:382-383`; `auction_service.go` release path via `order_completion_service.go:2037-2040`).
- Item identity = `product_id`; auction's own title/description/preparation duplicate the product (§5a).
- Buying flow: bid → claim → order (`order_creation_service.go:985-1014`, auction source).

## 8. Ownership Authority

- `seller_id` is **triplicated**: `products.seller_id`, `fixed_price_sales.seller_id`, `auctions.seller_id`. Same value by construction, but three authorities; nothing prevents divergence at schema level (no computed/FK-enforced identity).
- Cross-checked reads mostly use the surface's own seller (e.g., promotion seller resolution goes through `fixed_price_sales.seller_id`; comment FPS previews use the sale row).
- **Duplicate authority** classification: `H` (shortlist: three identical columns), not semantically wrong.

## 9. Price Authority

- **Canonical chain**: display price (FPS `price_per_unit` / auction start/buy-now) → server-computed Pricing Token (`pricing_tokens` snapshot: unit_price/subtotal/shipping/discount/coins/escrow/service/total) → `orders` snapshot. Checkout amount = pricing token + order (`ORDER_FINANCIAL_CONTRACT_CONVERGENCE_AUDIT.md:251`). No product-level price exists — correct.
- Severity: clean; single server authority, `PRD.md:1420` invariant 12.

## 10. Stock/Inventory Authority

- **FPS**: `fixed_price_sales.quantity_available` (000009) — the only persisted stock counter. Reduction/restore only via Order lifecycle.
- **Auction**: none (single-unit).
- **Product**: none.
- **Doc contradiction**: doctrine says koi unique-by-default and "multi-stock deliberately deferred" (`commerce-selling-doctrine.md:51-52`, `commerce-db-model-split-design.md:50`) while schema + PRD + 000009 owner-decision support multi-quantity FPS. The 000009 header records the decision; the doctrine text is stale. **Resolvable from canonical truth** (000009 = later, recorded owner decision), but docs still contradict.

## 11. Media Authority

- Schema canonical: `products.media_urls` (+ typed commerce media tables 000022-24, asset-specific). FPS rows store no media; FPS entity/media resolution reads from product (`joinedSaleByIDQuery`). Auction row has no media column either.
- **Correct placement** (media on Product, not on selling mechanism) per `docs/contracts/commerce-db-model-split-design.md:32-49`. Mobile lists media inline on the `Listing` entity = read-side convenience.

## 12. Shipping Authority

- **Configuration**: product-scoped — `product_shipping_options` (product→shipping_option), `products.farm_address_id`, `products.preparation_time`. FPS entity re-exposes farm_address_id/preparation_time (duplicated read fields).
- **Execution**: chat-based manual `shipping_quotes` now keyed to `chat_id` + `product_id` + `source_type/source_id` (+ `auction_id`), `listing_id` dropped (000010). Buyer availability = `GET /shipping/options` by `product_id`.
- **Cleaner**: legacy `listing_shipping_options` table dropped (000010); the "listing owns shipping" model is gone. Shipping correctly hangs off Product.

## 13. Order/Transaction Identity

- `orders.source_type/source_id`: FPS → `fixed_price_sale` + fps.id; auction → `auction` + auction.id (`order_creation_service.go:1542-1615`). Canonical surface identity on the ORDER is correct.
- **CONTRADICTION — `order_items.product_id` two id-spaces**:
  - FPS: `NewOrderItem(order.ID, listing.ID, ...)` — `listing.ID` = **fixed_price_sales.id** (`order_creation_service.go:1677`).
  - Auction: `NewOrderItem(order.ID, product.ID, ...)` — `products.id` (`order_creation_service.go:992`).
  - Cancel/expire restore re-resolves via the FPS repo: `listingRepo.GetForUpdate(ctx, tx, item.ProductID)` (`order_completion_service.go:1989`) — i.e. provenance expects fps.id for FPS items and is only correct because auction items go through a different release path (`:2021`). One column, two meanings. Mobile order DTOs read `product_id` in the order path — so a buy-side consumer sees fps.id or product.id depending on channel. **Requires identity fix.**

## 14. Pricing/Payment Identity Boundary

- `pricing_tokens`: `product_id` + `source_type/source_id` + `auction_id` column (itself unmapped in the entity, `pricing_token.go:31-34`), `listing_id` dropped. Snapshot columns carry the whole transaction math. `payments.reference_type/reference_id` generic; `price_snapshot_id`.
- **Latent ambiguity**: discount matching expects **product ids** (`pricing_token_service.go:360` passes `req.ProductID`) while the DISCOUNT WRITE path names them `applicable_listing_ids`/`target_type='listing'` with **no FK on `discount_targets.target_id`** (`000001:694-700,2452`). A seller-supplied "listing" id could silently be a product id or an fps id with nothing to stop it. Class `H`/misleading.
- Boundary overall: clean — pricing/coins/escrow all reference order/pricing-token, not the sale surface.

## 15. Search/Discovery Identity

- Federated search emits **`fixed_price_sale`** card key + `fixed_price_sale_id` (`search_projection_adapter.go:24-79`; test `search_commerce_seller_projection_test.go`). Canonical.
- Promoted/feed: discovery service queries by `target_type='fixed_price_sale'` (`promotion_discovery_service.go:68`); **BUT** the feed injector emits `"target_type":"listing"` (`feed_promotion_injector.go:490`), which mobile ignores (`feed_renderers.dart` reads `fixedPriceSaleId`, never `targetType`). Class: C/E (wire residue, inert).
- Mobile search navigates via `/listing/${result.id}` (`search_results_screen.dart:316`).

## 16. Share/Reference Identity

- `ShareTargetType`: content / fixed_price_sale / auction / profile — **no listing** (`share_reference.go:29-34`). Comment FPS refs emit `reference.targetType="fixed_price_sale"` (proven live in earlier stage's wiring test). Chat attachments reject `'listing'` as legacy (`attachmentvalidator/validator.go:109`; `attachment_validator_test.go:138-148`).
- `canonical_url` for FPS = `/listing/{id}` (server-built, `UNIFIED_SHARE_SCOPE_3D2C:232`) — URL path keeps the legacy noun.
- Comment `ListingPreview` Go type name (`comment_response.go:31`) — `D`.

## 17. Complete Terminology Map

Classification key: A=canonical business concept, B=valid UI term, C=valid API term, D=valid internal impl term, E=stale, F=misleading, G=dead residue, H=duplicate authority.

### `Product` / `product`
- Table `products`; entity `internal/commerce/product/entity/product.go` — **A**.
- Route `PUT /products/:id/shipping` — **C**.
- Wire keys `product_id` (orders, pricing tokens, shipping quotes, comment previews) — **C**.
- `products.status` sales coupling — **F** (conflicts with surface-agnostic claim).
- No mobile `Product` class; product fields inlined into mobile `Listing` — **D** (read-side), risk.

### `FixedPriceSale` / `fixed_price_sale`
- Table/entity/enum `fixed_price_sale_status_enum` — **A**.
- Wire `fixed_price_sale` (source_type, fixed_price_sale_id, targetType, chat attachment, promotion target, search card) — **C**.
- Import alias `listingApp/listingRepo` and local vars `listing` = FPS in handlers/services — **D**.
- No `/fixed-price-sales` API path exists — gap (C nonexistent; API speaks `/listings`).

### `Listing` / `listing`
- Mobile domain `catalog/listing`, class `Listing` (= FPS; header explicitly says so at `listing.dart:6-11`) — **B/D**.
- Screens `listing_detail_screen.dart`, `listing_list_screen.dart`, `my_listings_screen.dart`, etc. — **B**.
- Routes `/listings`, `/listing/:fixedPriceSaleId`, `/search/listings`; backend `routes_core.go:145-147,314` dispatch `/listings` → `FixedPriceSaleHandler` — **C** (but wrong-noun API).
- `listingToResponse`/`listingToResponseWithSeller` (`fixed_price_sale_handler.go:1003-1010`) — **D**.
- OG `/og/listing/:id`, deep-link `/listing/{id}` — **C** (URL).
- `saved_items.target_type='listing'` (CHECK `000001:2515`; `saved_item.go:14`) with target_id = fps.id — **C** (real wire value, else branch identical meaning).
- `discount_targets.target_type='listing'` (CHECK `000001:2452`) — **C** but id-space ambiguous vs product (§14) — **F/H**.
- `feed_promotion_injector.go:490` `target_type:"listing"` — **E** (inert; mobile reads fixed_price_sale_id).
- `orders.shipping_source` value `'listing'` (`order.go:329`) — **E**/(edge, legacy).
- `order_source_enum` member `'listing'` (`000001:205`) — **E** (never written; only `fixed_price_sale` written, `order_source_type.go:16`).
- `negotiation_resource_enum` member `'listing'` (`000001:193`) — **E** (entity only writes `fixed_price_sale`, `negotiation_resource_type.go:8`; FK is `negotiation_sessions.fixed_price_sale_id`).
- `listing_type_enum` ('fixed_price','auction'), `listing_status_enum`, `listing_origin_enum`, `listing_visibility_enum` (`000001:145-168`) — **G** (orphan types; table dropped, **no up-migration drops the types** — verified: only `000010.down.sql` references them).
- Import aliases `listingApp/listingHTTP/listingRepo` in `serverboot/dependencies.go`, `order_creation_service.go`, `pricing_token_service.go`, `saved_item_service.go`, `moderation_event_handler.go`, `comment_service.go` — **D**.
- `listing_commission_percent` config key (`config_service.go:30`) — **D**.
- `ActionTypeHideListing/RemoveListing/SuspendListings`, moderation payload key `"listing_id"` (`domain_action.go`, `domain_action_worker.go:421`), `listing.visibility.restore...` outbox (`appeal_reversal_service.go`) — **D** (act on FPS) + **E** (payload label).
- `AuditService.NegotiationStarted(..., listingID)` payload `"listing_id"` — **G** (no prod caller).
- Stale scripts `validation/query_db.go`, `scripts/api_flow_validation.go` (marked `// STALE`) still SELECT from `listings`/POST `/api/v1/listings` — **G**.
- `comment_type_enum 'listing_reference'` — dropped by 000031 (was **E**).
- schemaguard `TestNoProductionCodeReadsLegacyListingsTable` — enforcement lock, **A**-adjacent (guard).

### `For Sale` / `ForSale` / `Sale` / `fixed_price` / `sale`
- `Explore` tab "For Sale" (`explore_screen.dart:109`), profile store "Dijual (For Sale)" (`profile_store_tab.dart:7`) — **B**.
- `FixedPriceSaleType` value `'fixed_price'` (`fixed_price_sale_type.go:13-18`; header calls it hygiene debt after PASS_21C removed `auction`) — **D**.
- `listing_type_enum 'fixed_price'` — **G** (orphan type).

### `Auction` / `auction` / `Lelang`
- Table/entity/enum — **A**; wire `auction` (`auction_id`, targetType, promotion/saved-items value) — **C**.
- UI `Lelang` — **B**.
- `auction.title/description` duplication of product content — **F/H**.

### `Catalog item` / `Sellable item` / `Offer` / `Inventory` / `Stock`
- "Catalog — Listing" domain in `docs/flows/domain-map.md:78-83` — **E** (doc treats catalog the old way).
- "sellable item" in FPS entity header comment (`fixed_price_sale.go:2`) — **F** (sellable concept now belongs to Product; comment is legacy phrasing).
- `Offer`: domain disabled/removed (`routes_core.go` collections/offers disabled) — **G**.
- `Inventory/Stock`: mobile `Listing.stock` ← `quantity_available` — **D** (correct FPS placement).

---

## 18. Schema Contradictions

1. **`order_items.product_id` double id-space** — FPS writes fps.id, auction writes product.id, same column (`000001:1024-1033`; writers `order_creation_service.go:1677` / `:992`). Misleading column; cannot be resolved statically. **FIX REQUIRED** (see §29).
2. **Auction duplicates item content** — `auctions.title/description/preparation_time` columns while `product_id` is the item authority (`000001:477-498`). Duplicate authority for title/description. Not collapsed to product like FPS.
3. **`saved_items.target_type` and `discount_targets.target_type` still CHECK-constrained to `'listing'`** — `000001:2452,2515`. The word `listing` is a live, enforced DB value for FPS in two tables.
4. **`order_source_enum` and `negotiation_resource_enum` still contain `'listing'` members** — `000001:204-210,192-195` — never dropped, never written. Dead enum members.
5. **`listing_*_enum` types orphaned** — created `000001:145-168`, table dropped `000010`, types never dropped. Pure residue.
6. **`discount_targets.target_id` has no FK** and namespace is ambiguous (product vs fps id) — nullable-everything relationship that should not exist.
7. **No schema guard blocks a new active sale channel on a SOLD product** — the 000010 trigger only blocks cross-table active/pending coexistence; it says nothing about `products.status='sold'`. The doctrine invariant "sold product MUST NOT be reused" is not schema-enforced. Residue risk for relisting.
8. **Schema permits old design to "return"?** — No `listings` table exists; guard test bans `FROM listings`/`JOIN listings`; the four orphan enums are the only detectable fossils. The old FK graph is gone. **Hidden resurrection risk is LOW structurally but HIGH terminologically** (CHECK values + enum members keep `listing` alive as a first-class string).

## 19. Code Contradictions

1. **Product lifecycle violates "surface-agnostic"** — `product.go:9-10` claims agnostic; `order_creation_service.go:847-848` sets `product.Status="sold"`+`SoldAt`; `order_completion_service.go:2037-2040` flips back `available`. Product sale-state is written by the order lifecycle — a second authority for "sold" alongside FPS/auction status.
2. **Negotiation `listing`**: enum still has `'listing'`, entity only writes `fixed_price_sale` — code/enum drift (`negotiation_resource_type.go:8` vs `000001:193`).
3. **Discount id-space mismatch** — matching uses product id, write path names `listing` ids; unconstrained. (`pricing_token_service.go:360` vs `discount_repository.go:578-581`, `discount.go:50`.)
4. **Feed injector emits `listing` while promotion domain is canonical `fixed_price_sale`** — two vocabularies on the same promoted card.
5. **`sellerCapabilityChecker`/market-authority** now properly wired (previous stages) — not a contradiction here, listed for completeness.
6. **HTTP status mapping**: non-capable commerce-ref create returns 500 (sentinels not mapped) — pre-existing, out of scope.

## 20. Mobile/UI Contradictions

1. Mobile names the entire FPS domain `listing` (dir `catalog/listing`, class `Listing`, screens `*_listing*`, datasource `/listings`) while the backend domain noun is `FixedPriceSale`. Internally consistent but vocabulary-drifted (`listing.dart:6-11` itself states "this is the fixed-price sale mechanism").
2. Mobile `Listing` entity inlines product koi fields (variety,size,age,gender,breeder,bloodline, media, preparation) — no `Product` model exists client-side. Every future product-level feature (relist across channels, product page) hits the wall immediately.
3. Saved-items UI presents category `Listing` (`saved_item_screen.dart:69`) — matches backend value, synchronized but deprecated naming.
4. Feed `PromotedListingCard` reads `fixedPriceSaleId` and navigates `/listing/:id`; the inert backend `target_type:"listing"` field is never used — a live wire value with no consumer.
5. `CommerceSavedItemActionButton` is dead code (never instantiated) — G.
6. Route param is already `fixedPriceSaleId` (`route_paths.dart:26`) while the path segment says `listing` — naming debt visible to any developer.

## 21. Test/Fixture Contradictions

1. **Pre-existing build failure**: `fixed_price_sale_create_sender_address_test.go:188,229` reference `PublishNow` field which no longer exists in `CreateFixedPriceSaleInput` — the fixedprice application test package does not compile with `-tags integration`. Pre-existing (README says do-not-touch), but it is a fixture/source drift. Confirmed by `go test -tags integration` (build failed), while `go build ./...` (non-test) passes.
2. `negotiation_event_handler_test.go:13` fixture `ResourceType:"listing"`; sibling fixtures use `fixed_price_sale` — mixed vocab in one domain's tests.
3. `promotion_safety_sweep_test.go:313-314` lists legacy reason strings `listing_not_found/listing_unavailable` as fixture data of removed states.
4. Docs-index residue: `docs/README.md` lists guide/foundation/architecture/RUNTIME-INVARIANTS/glossary files that do not exist; omits the commerce doctrine — stale index.
5. `backend/internal/interaction/chat/delivery/http/ATTACHMENT_SCHEMA_V2.md` still documents `listing|auction|post|request` with `listing_id`, self-marked "Legacy ... not canonical".

## 22. Dead/Zombie/Legacy Residue

- Orphan enum types `listing_type_enum`, `listing_status_enum`, `listing_origin_enum`, `listing_visibility_enum`.
- enum members `order_source_enum 'listing'/'seller_quote'`, `negotiation_resource_enum 'listing'`.
- `pricing_tokens.listing_id`, `shipping_quotes.listing_id`, `auctions.listing_id`, `order_items.listing_id` — dropped (000010); code mentions only in tests/scripts.
- `search_handler.go` `SearchListings` (unrouted; superseded by `FixedPriceSaleHandler.SearchFixedPriceSales`).
- `AuditService.NegotiationStarted` `listingID` parameter (no caller).
- Stale scripts `validation/query_db.go`, `scripts/api_flow_validation.go` (marked STALE), `cmd/dev-reset-data/main.go` drop lists.
- `CommerceSavedItemActionButton` (mobile, never instantiated).
- Disabled Offer/Collection domains (`routes_core.go`).
- `external_products` table + `search_results` table (000011 dropped `search_results`; `external_products` still present, used by promotion).
- `saved_item_entity.go` `SavedItemWithListing*` hydration-rename fields.

## 23. Hidden Resurrection Risks

1. **Terminological resurrection**: `listing` remains a live CHECK-constrained value in `saved_items` and `discount_targets`, live enum members in `order_source_enum`/`negotiation_resource_enum`, and an emitted wire label in `feed_promotion_injector`. Any "renamed column back to listing_id" refactor could silently reconnect.
2. **Doc resurrection**: `ATTACHMENT_SCHEMA_V2.md`, `docs/flows/domain-map.md`/`actor-map.md`/`cross-domain-relations.md`, ADR-008/010 still teach "Listing" as first-class. A developer following docs rebuilds the alias.
3. **Id-space resurrection**: `order_items.product_id` and `discount_targets` mix product/fps ids — the exact kind of ambiguity that made old `listing_id` dangerous; it can breed a new polymorphic-identity bug.
4. **Sold-product reuse**: no schema/service gate prevents a new channel on a sold product (doctrine only). If relisting ships, it ships without the doc's own invariant.
5. `products.status` dual-write is the same defect class that created FPS status/quantity drift before 000009.

## 24. Runtime Proof

| Claim | Proof | Type |
|---|---|---|
| `listings` table gone | migration `000010` applied; `go test ./internal/platform/schemaguard/` PASS (guard bans `FROM listings`); `go test -tags integration -run TestMigration00031 ./tests/` PASS (303s, real Postgres, current schema state) | RUNTIME + migration |
| comment commerce-ref identity `fixed_price_sale` | earlier stage: `TestCommentCommerceReferenceWire_CreateListShape_*` PASS on real DB | RUNTIME |
| FPS quantity persistent (not derived) | migration `000009` (owner decision recorded); entity `fixed_price_sale.go` Reduce/Restore. NOTE: quantity integration test build currently blocked by pre-existing `PublishNow` fixture failure — quantity round-trip runtime re-run **NOT PROVEN this pass** | STATIC + migration |
| sale-surface create mints product atomically | `fixed_price_sale_repository_impl.go:38-51`; `auction_service.go:260-278` (same-tx product create) | STATIC |
| order FPS source identity | `order_creation_service.go:1542-1615` writes `orders.source_type='fixed_price_sale'` + source_id=fps.id | STATIC (code) — witness in `escrow_ledger_atomicity_real_db_proof_integration_test.go:29` seed `'fixed_price_sale'` | RUNTIME |
| order_items double id-space | `order_creation_service.go:992` (auction → product.ID), `:1677` (FPS → listing.ID); restore at `order_completion_service.go:1989` | STATIC |
| marketplace surface works end-to-end | prior stages: comment FPS wire + capability gate + list handler wiring all PASS on real Postgres | RUNTIME |

## 25. Static Proof (selected)

- Schema root identity: `products` (item), `fixed_price_sales`/`auctions` (channels), `product_shipping_options` (product-scoped shipping) — `000001`.
- No standalone Product HTTP create/read — `routes_core.go:338-345` (only `PUT /products/:id/shipping`).
- FPS name collision evidence: `routes_core.go:145-147,314` path `/listings` → `FixedPriceSaleHandler`.
- Feed wire label: `feed_promotion_injector.go:490` `target_type:"listing"` + `:491` `fixed_price_sale_id`.
- Docs doctrine: `commerce-selling-doctrine.md:9-13,29,64-68`; PRD: `PRD.md:301-315`.

## 26. Owner Decisions Required

| # | Decision | Evidence A | Evidence B | Resolvable from existing truth? |
|---|---|---|---|---|
| D1 | **Canonical noun for the fixed-price sale channel**: `FixedPriceSale` vs `FixedPriceListing` vs UI `Listing`/`Produk Dijual` | Doctrine: "noun MUST be `FixedPriceListing`; MUST NOT collapse into a single listing model" (`commerce-selling-doctrine.md:9-13,29`, `commerce-db-model-split-design.md:17-19,114`) | Schema+code+PRD+audits: `fixed_price_sales`, `FixedPriceSale`, "Fixed Price Sale" (`PRD.md:301-307`) | **NO — two canonically-worded docs inside the repo disagree. OWNER DECISION REQUIRED.** |
| D2 | **Product lifecycle**: keep `products.status`/`sold_at` as an order-lifecycle-written coarse sale state, or strip Product back to purely item metadata (surface-agnostic as documented)? | Doctrine: "internal physical item authority" (`commerce-selling-doctrine.md:9`) | Code writes `product.Status="sold"`/revert in order flow (`order_creation_service.go:847-848`, `order_completion_service.go:2037-2040`) | **NO — doctrine text contradicts implemented behavior. OWNER DECISION REQUIRED.** |
| D3 | **Quantity doctrine**: keeper text says "unique-item, multi-stock deferred" (`commerce-selling-doctrine.md:51-52`) vs 000009 owner decision + schema + PRD multi-quantity FPS | Doctrine/DB-design §2 (`commerce-db-model-split-design.md:50`) | `000009` migration header (recorded owner decision) + `PRD.md:309` + live `fixed_price_sales.quantity_available` | YES — settle on 000009 (multi-quantity on FPS); update doctrine text. (Not a true fork; doctrine is stale.) |
| D4 | **Sold-product reuse / relisting policy** — allow relist (new channel row on sold product) or enforce doctrine "sold MUST NOT be reused" | Doctrinal invariant `commerce-selling-doctrine.md:67` (unenforced in schema/code) | Relisting vacancy + trigger only handles active/pending coexistence | **NO — policy absent from canonical code. OWNER DECISION REQUIRED.** |
| D5 | **Auction title/description** — delete auction-owned copies and always read from product (FPS pattern) or keep auction snapshot semantics | `auctions` has own title/description cols (`000001:483-484`) | FPS pattern joins product; mobile auction entity carries its own snapshot | Requires D-vocab + item-authority stance; fold into D2 decision. |

## 27. Recommended Canonical Vocabulary

Given the schema+PRD gravity, and ONLY IF the owner confirms D1, the least-cost canonical set is:

- **`Product`** — the physical koi item authority (metadata, media, farm address, preparation, koi attributes). Not price, not stock, not sale state (pending D2).
- **`FixedPriceSale`** — the fixed-price selling mechanism (price, quantity, negotiation, status). API keeps `fixed_price_sale`; **create `/fixed-price-sales` aliases or rename the user-facing brand to "Produk Dijual"** in UI, and retire `/listings` routes over time.
- **`Auction`** — sibling selling mechanism over Product.
- **`Sale channel` / `selling mechanism`** — the shared category noun.
- **Retire**: `Listing` as any non-UI term; `listing_*` enum types; `'listing'` CHECK values (saved_items, discount_targets → `fixed_price_sale`); `'listing'/'seller_quote'` order_source members (keep `'fixed_price_sale'`,`'auction'`,`'negotiation'`); `negotiation_resource_enum 'listing'`; `feed_promotion_injector` target_type → `fixed_price_sale`; `orders.shipping_source 'listing'` if still referenced.

## 28. Recommended Domain Model

```
Product (id, seller, koi metadata, media, farm_address, preparation)
   ├── 0..* FixedPriceSale (one active; price, quantity, negotiation, FPS status)
   │        └── persists stock on FPS; product only marked per D2 policy
   └── 0..* Auction (one active/pending; start/buy-now/bid state, auction status)

Order (source_type+source_id = the winning channel; product_id = products.id on order_items ALWAYS)
Pricing Token (product_id + source_type/source_id; snapshots; sole money authority)
Shipping config → product_shipping_options + farm_address (product-scoped)
Discount targets → target_type fixed_price_sale/auction with target_id = channel id (single id-space)
Saved items → target_type fixed_price_sale/auction (single id-space)
Share/reference → fixed_price_sale / auction (already canonical; keep)
```

Key invariants to lock: one active channel per product (done, 000010 trigger); quantity on FPS (done, 000009); order_items.product_id = products.id ALWAYS (fix); product.status decided by D2.

## 29. Implementation Plan IF refactor is required

Refactor is NOT required for the split (it's correct). A **narrow convergence** is required, sequenced:

1. **(Bug fix, owner-approvable independently)** Unify `order_items.product_id` to always store `products.id`; keep channel identity on `orders.source_type/source_id`; make `order_completion_service.go:1989` resolve FPS via `orders.source_id` or via product→active FPS lookup. Cover with a runtime order test asserting both channels write the same id-space.
2. **(D1+D3)** Decide noun; update doctrine text (reconcile `FixedPriceListing` vs `FixedPriceSale`), then rename surface UI + docs + `saved_items`/`discount_targets` CHECK values (`'listing'→'fixed_price_sale'`), feed injector wire label, and finally legacy enum members.
3. **(D2/D5)** Decide Product sale-state; if surface-agnostic: remove product.status/sold_at writes, derive sellability from channel states; remove auction.title/description columns, read from product (FPS pattern). If product-sold kept: codify the coupling in product.go doc + enforce sold→not-reusable gate (D4).
4. **(D4)** Add explicit relist policy + enforcement (service gate + optional DB trigger on `products.status='sold'`).
5. Drop orphan `listing_*_enum` types and dead enum members via a migration once 1-4 land.
6. Rename mobile `catalog/listing` → `catalog/fixed_price_sale` (or add `Product` model) only after D1; keep `Listing`/`Produk Dijual` solely as UI strings.
7. Fix pre-existing `PublishNow` test compile drift.

## 30. Cleanup Plan IF convergence is achieved

If the owner picks the recommended vocabulary and the order_items fix ships:
- Remove orphan types: `DROP TYPE listing_type_enum, listing_status_enum, listing_origin_enum, listing_visibility_enum` (+ `000010.down` no longer needs them).
- Re-map `saved_items.target_type`, `discount_targets.target_type` CHECK and rows (`listing`→`fixed_price_sale`).
- Trim `order_source_enum` and `negotiation_resource_enum` members.
- Emit `fixed_price_sale` from `feed_promotion_injector`.
- Retire `/listings` route naming (alias during transition), `listingToResponse`, OG `/listing` path (keep `/listing/{id}` URL only for back-compat share links).
- Purge stale scripts (`validation/query_db.go`, `scripts/api_flow_validation.go`), header comments, import aliases (`listingApp`→`fixedPriceApp`), dead `AuditService.NegotiationStarted listingID`, mobile dead widget.
- Update `docs/README.md` index, `ATTACHMENT_SCHEMA_V2.md`, `domain-map/actor-map/cross-domain-relations.md`, ADR-008/010, and PRD phrasing to the single vocabulary.

## 31. Remaining Risks

- **Identity drift in order_items/discounts** is live risk today (two id-spaces, no FK). Highest-risk item; could corrupt financial provenance if it ever mixes.
- **Naming debt** hides the domain truth from new developers: without audit docs a new dev cannot distinguish Product vs FixedPriceSale vs Listing (they share fields inline).
- `products.status` dual-write could desync from channel state (e.g., FPS restored from moderation, auction cancelled post-sale).
- Untested discount `applicable_listing_ids` semantics can silently scope discounts to wrong ids.
- Feature-completeness of `order_source_enum`/`negotiation_resource_enum` pruning depends on D1.
- Pre-existing `PublishNow` fixture build failure blocks the fixedprice integration test package until fixed.

## 32. Final Verdict

**`COMMERCE_PRODUCT_ARCHITECTURE_HAS_OWNER_DECISION`**

Structural verdict: the Product / FixedPriceSale / Auction sibling model is the correct industry shape for a 10M-product marketplace — item identity, price/stock, and auction state are on the correct authorities, order money flows through a server pricing token, and the old listings table is fully removed with a guard. Multi-product/historical-sales/relisting-by-new-row and one-active-channel are all structurally representable (one active channel enforced by DB).

However: (1) the repo documents two incompatible canonical vocabularies (FixedPriceListing vs FixedPriceSale) with no authority to choose; (2) `OrderItem.product_id` and Discount id-spaces are ambiguous; (3) Product's documented "surface-agnostic" lifecycle is contradicted by order-lifecycle writes; (4) relisting-on-sold-product policy is unenforced. These require the Five Owner Decisions (D1-D5) before convergence — most critically D1 (one canonical noun) and D2 (product sale-state). The model is converged enough to IMPLEMENT ON; it is not yet converged enough to CALL converged.