# COMMERCE — FOR SALE VOCABULARY / AUTHORITY AUDIT (STAGE A)

Project: Labuda monorepo (`D:/Project/labuda`)
Audit date: 2026-08-25 · Mode: READ-ONLY (no code changed)
Canonical target vocabulary: **FOR SALE** (`for_sale` / `ForSale`)

---

## 0. SCOPE & METHOD

Read-only sweep of `backend/`, `apps/mobile/`, migrations, and docs for every `listing` / `fixed_price_sale` / `fixedprice` token and its semantic role. Token-frequency (case-insensitive) totals:

| Surface | `listing|listings|fixed_price_sale|FixedPriceSale|fixedprice` matches |
|---|---:|
| backend (Go + SQL) | ~5100 across ~350 files |
| apps/mobile (lib + test) | ~3980 across ~330 files |

**Bottom line up front:** the canonical wire/DB vocabulary is ALREADY `fixed_price_sale` in most reference planes (share/comment/chat objectType, moderation, negotiation, promotion search sidecar, chat attachment validator), while **saved-item, discount, and the feed-promotion injector still emit `listing`** on the wire, and the **HTTP routes + response envelope + the whole `fixedprice` Go package + the entire mobile `catalog/listing/` domain** still use `listing`. This is a **genuine production vocabulary split** — the feed emit `target_type: "listing"` while search emits `target_type: "fixed_price_sale"` for the same surface.

---

## 1. CURRENT VOCABULARY MAP

| Symbol / string | Location | Role / classification |
|---|---|---|
| `fixed_price_sales` (table) | migrations 000001..live | **canonical** surface table |
| `fixed_price_sale_status_enum` | 000001 | status enum (`draft/active/sold/withdrawn`) |
| `FixedPriceSale*` (types) | `backend/internal/commerce/fixedprice/**` | Go domain types |
| `fixedprice` (package dir) | `backend/internal/commerce/fixedprice/` | Go package name |
| `FixedPriceSaleHandler/Service/Repository` | fixedprice package | app/http/repository |
| `create_listing` / `listing.create` (idempotency) | fixedprice handler | internal key |
| `/api/v1/listings`, `/api/v1/search/listings` | `cmd/core_server/routes_core.go` | HTTP routes |
| `"listings"` (response key), `"listing"` (envelope key) | fixedprice handler | JSON envelope |
| `"fixed_price_sale"` (wire objectType) | share/comment/chat refs, moderation, negotiation, search promo, chat validator | wire type value |
| `TargetTypeListing = "listing"` | `saved_item/entity/saved_item.go` | wire targetType |
| `DiscountAppliesToListing = "listing"` / `applicable_listing_ids` | `discount/entity/discount.go` | applies_to + JSON field |
| `ShippingSourceListing = "listing"` | `order/entity/order.go` | shipping_source value |
| `target_type: "listing"`, `type: "promoted_listing"` | `feed/.../feed_promotion_injector.go` | feed promo wire |
| `target_type: "fixed_price_sale"`, `type: "promoted_fixed_price_sale"` | `search/.../search_promotion_injector.go` | search promo wire |
| `ListingPreview`, `json:"listing,omitempty"`, `GetListingPreviewFromListing` | `content/application/comment_response.go` | comment preview (response key) |
| `/listing/{id}` deep-link | `content/.../share_reference.go`, `chat/.../chat_resource_projection.go` | OG/deep-link path |
| mobile `Listing`, `catalog/listing/`, `ListingStatus`, `ListingVisibility`, `listing_*` files | `apps/mobile/lib/domains/commerce/catalog/listing/**` | mobile surface domain |
| `fps` / `FPS` shorthands | serverboot resolvers, authorizer adapter | Go identifier aliases |

**Canonical decision**
- wire objectType for the fixed-price surface → **`for_sale`**
- Go type/package → **`ForSale` / `forsale`** (package dir `fixedprice` → `forsale`)
- DB table → new migration renames `fixed_price_sales` → `for_sales`
- HTTP route → `/api/v1/for-sale` (route shape is an owner decision, §12)
- mobile domain → `catalog/listing/` → `catalog/for_sale/`, `Listing` → `ForSale`
- **Product stays `product`; Auction stays `auction`** — only the fixed-price surface terms change.

---

## 2. CANONICAL TARGET VOCABULARY (proposed mapping)

| Current | Target | Style |
|---|---|---|
| `fixed_price_sales` (table) | `for_sales` | snake |
| `FixedPriceSale`, `FixedPriceSaleStatus`, ... | `ForSale`, `ForSaleStatus`, ... | Pascal |
| `fixedprice` (package) | `forsale` | lower |
| `fixed_price_sale` (wire/DB enum value) | `for_sale` | snake |
| `listing` / `listings` (surface synonym + route + envelope) | `for_sale` / `for_sales` | snake |
| `TargetTypeListing` | `TargetTypeForSale` | Pascal |
| `DiscountAppliesToListing` / `DiscountContextListing` | `DiscountAppliesToForSale` / `DiscountContextForSale` | Pascal |
| `/listing/{id}` deep-link | `/for-sale/{id}` | kebab path |
| `promoted_listing` / `promoted_fixed_price_sale` | `promoted_for_sale` | snake (unify) |
| `FixedPriceSaleHandler/Service/Repository` | `ForSaleHandler/Service/Repository` | Pascal |

Rationale: the term `for_sale` currently appears **zero times** in the codebase, so the rename is clean and unambiguous. It reads as the natural singular business term the owner selected.

---

## 3. LEGACY CLUSTERS (exhaustive classification)

### 3A. True business concept (fixed-price selling surface) → MUST rename to For Sale
- **`backend/internal/commerce/fixedprice/**`** — the entire package: `FixedPriceSale` struct + status/origin/type/visibility enums, `FixedPriceSaleService`, `FixedPriceSaleRepository`, `FixedPriceSaleRepositoryImpl`, `FixedPriceSaleHandler`, `CreateFixedPriceSaleInput`, response projections, SQL strings (`FROM fixed_price_sales`, etc.). 100% surface concept.
- Wiring: `cmd/seed`, `cmd/core_server/routes_core.go`, `cmd/migrate`, `internal/serboot`.
- `serverboot/` resolvers: `chat_fixedprice_projection_resolver*.go`, `chat_resource_projection_aggregate_resolver.go`, `chat_resource_projection_wiring.go`, `chat_resource_authorizer_adapter.go` – `FixedPriceSaleSourceRow`, `ResolveFixedPriceSales`, `fps*` aliases, SQL `fixed_price_sales`.
- `platform/events/events.go` — event strings `fixed_price_sale.created/.updated/.published/.withdrawn/.sold` → `for_sale.*`.
- `social/content/` — `ShareTargetTypeFixedPriceSale = "fixed_price_sale"`, `ContentResourceOccurrenceResourceTypeFixedPriceSale`, `NewShareReferenceFromFixedPriceSale`, `loadFixedPriceSales` SQL, `fixed_price_sale_source_id` column.
- `interaction/chat/` — `CommerceTargetTypeFixedPriceSale = "fixed_price_sale"`, `ResourceOccurrenceResourceTypeFixedPriceSale`, `chat_service.go` target_type switch, `chat_occurrence_fallback_builder.go`, `attachmentvalidator/validator.go` `validTargetTypes["fixed_price_sale"]`.
- `governance/moderation/` — `moderation_resource_enum 'fixed_price_sale'`.
- `commerce/negotiation/` — `NegotiationResourceFixedPriceSale = "fixed_price_sale"`, `negotiation_sessions.fixed_price_sale_id`.
- `pricing/promotion/` — search sidecar `promoted_fixed_price_sale`, `target_type: "fixed_price_sale"`.
- `commerce/order/` — `OrderSourceFixedPriceSale = "fixed_price_sale"` + persisted `order_source_enum`.
- `pricing/token` — `sale_surface_type_enum 'fixed_price_sale'` on `pricing_tokens.source_type` / `shipping_quotes.source_type`.
- `shipping/quote/application/shipping_quote_service.go` and `listing_shipping_service.go` — surface references (rename to for_sale where they name the surface).

### 3B. Product concept → LEAVE as `product` (do NOT touch)
- `fixed_price_sales.product_id → products` joins (fps repo, content resolver, chat fallback builder, comment_service, promotion injectors, search repo). Product is content+identity authority.
- `order_items.product_id = products.id` (converged in 000045) — unchanged.
- Product title/media/koi-attr/preparation/farm-address = **single authority** from `products`; the FPS entity still mirrors some fields for display (bounded refactor noted as out of scope).

### 3C. Generic word "listing" (bystander, not the surface) → KEEP
- Generic enum/list words unrelated to the fixed-price surface.
- **Cleanest rule:** any `listing` that means "a fixed-price selling surface" → `for_sale`; any `listing` that is purely a generic enumeration word → leave.

### 3D. Historical / documentation reference → KEEP, refresh comments/docs where they assert current-vocabulary claims
- Malformed/stale comments asserting `listing` is the current canonical term (e.g. `fixed_price_sale_type.go` PASS_21C, `ATTACHMENT_SCHEMA_V2.md` body which conflicts with its own legacy-note).
- Repo-root audit/report `.md` files (UNIFIED_SHARE_*, COMMERCE_*, etc.) — **do not rewrite** (historical evidence chain).

### 3E. Dead / zombie → REMOVE (Stage F)
- Orphaned DB enums `listing_type_enum`, `listing_origin_enum`, `listing_visibility_enum` (unreferenced post-000010).
- Dead `order_source_enum` value `'listing'`.
- Already-dropped tables (`listings`, `listing_shipping_options`, `listing_views`, `fixed_price_sale_media`, `auction_media`) — history only, do not re-touch.
- `fixed_price_sale_create_sender_address_test.go` — references nonexistent `PublishNow` field on `CreateFixedPriceSaleInput` → stale test, prune.
- `serverboot/chat_commerce_reference.go` — confirmed **does not exist** (red herring).
- Old routes `/api/v1/listings*` + `/api/v1/search/listings` — replaced by For Sale routes (no backward-compat).

### 3F. Schema / DB identifiers — §5.
### 3G. API / wire contract — §6.
### 3H. Mobile model/UI — §7.
### 3I. Test / fixture terminology — §8.
### 3J. Event / outbox / resource terminology — §5/§6.

---

## 4. DEPENDENCY GRAPH

```
products (canonical content + identity authority)
   ▲
   │ product_id (FK RESTRICT)
for_sales  (was fixed_price_sales)  ← one-active-surface unique index + trigger
   │         source surface identity
   ├─► orders.source_type/source_id      (order_source_enum)
   ├─► order_items.product_id → products (unchanged)
   ├─► saved_items.target_type           (was 'listing')
   ├─► discount_targets.target_type      (was 'listing'); discount.applies_to/context
   ├─► negotiation_sessions resource_type + for_sale_id
   ├─► comment_commerce_references.for_sale_id
   ├─► content_resource_occurrences.for_sale_source_id
   ├─► chat_message_resource_occurrences.for_sale_source_id
   ├─► chat_commerce_references.target_type
   ├─► moderation_cases.resource_type
   ├─► pricing_tokens.source_type / shipping_quotes.source_type (sale_surface_type_enum)
   └─► platform/events outbox (for_sale.*)
```

**Rename order must be schema-first, then entity types, then every consumer switch-case/literal, or the app breaks.** See §10.

---

## 5. SCHEMA IMPACT (the plan)

**Live table:** `fixed_price_sales` (stable since squashed baseline 000001). Columns `id, product_id, seller_id, price_per_unit, negotiation_enabled, status(fixed_price_sale_status_enum), published_at, sold_at, withdrawn_at, quantity_available, created_at, updated_at`. One-active-surface partial unique index `uniq_active_fixed_price_sale_per_product` on `product_id WHERE status IN ('draft','active')`. Trigger `trg_fixed_price_sales_single_active_channel` + function `enforce_single_active_sale_channel_per_product`.

**Do NOT modify migrations 000001–000046** (canonical history). **Add ONE new migration `000047`** that:
1. `ALTER TABLE fixed_price_sales RENAME TO for_sales;` — rename `fixed_price_sale_status_enum` → `for_sale_status_enum`; rename index/constraint/trigger/function (`uniq_active_for_sale_per_product`, `trg_for_sales_single_active_channel`, `enforce_single_active_sale_channel_per_product`).
2. `order_source_enum`: drop dead `'listing'`, rename `'fixed_price_sale'`→`'for_sale'` (PG<12 cannot `RENAME VALUE` — destructive rewrite strategy, §12).
3. `sale_surface_type_enum` value `'fixed_price_sale'`→`'for_sale'` (pricing_tokens / shipping_quotes).
4. `negotiation_resource_enum`: drop `'listing'`, rename `'fixed_price_sale'`→`'for_sale'`.
5. `moderation_resource_enum` value `'fixed_price_sale'`→`'for_sale'`.
6. `chat_commerce_reference_target_type_enum` value `'fixed_price_sale'`→`'for_sale'`.
7. `discount_scope_enum` value `'listing'`→`'for_sale'`; `saved_items.target_type` CHECK + `discount_targets.target_type` CHECK `('listing','auction')` → `('for_sale','auction')` + **backfill rows** `'listing'→'for_sale'`.
8. FK columns: `comment_commerce_references.fixed_price_sale_id`→`for_sale_id`, `content_resource_occurrences.fixed_price_sale_source_id`→`for_sale_source_id`, `chat_message_resource_occurrences.fixed_price_sale_source_id`→`for_sale_source_id`, `negotiation_sessions.fixed_price_sale_id`→`for_sale_id`.
9. Drop orphaned `listing_type_enum`, `listing_origin_enum`, `listing_visibility_enum`.
10. `promotion_instances/events.allowed_target_types` (text) — backfill `'fixed_price_sale'`→`'for_sale'`.

**Out of scope (do NOT touch):** coins, ledger, refund, commission, escrow, settlement, addresses, payments, external_products.

---

## 6. API / WIRE CONTRACT IMPACT (the plan)

### Input objectType / targetType → canonical `for_sale`
- `dto/share_reference_request.go` `oneof=content fixed_price_sale auction profile` → `for_sale`.
- `attachmentvalidator/validator.go` `validTargetTypes["fixed_price_sale"]` → `"for_sale"`; `"listing"` already rejected (fail-closed — keep rejecting).
- `comment_service.go`, `chat_service.go` target_type switch cases `"fixed_price_sale"` → `"for_sale"`.

### Emit wire
- **saved_item** `target_type: "listing"` → `"for_sale"`.
- **discount** `applies_to: "listing"` / `DiscountContextListing` → `"for_sale"`; JSON `applicable_listing_ids` → `applicable_for_sale_ids`.
- **feed promotion** `target_type: "listing"`, `type: "promoted_listing"` → `"for_sale"` / `promoted_for_sale` — **unify** with search's already-canonical output.
- **search promotion** `promoted_fixed_price_sale` → `promoted_for_sale`; `fixed_price_sale_id` field → `for_sale_id`.
- **comment preview** `ListingPreview`→`ForSalePreview`, `json:"listing"`→`json:"for_sale"`, `GetListingPreviewFromListing`→`GetForSalePreviewFromForSale`.
- **HTTP routes** `/api/v1/listings*` → `/api/v1/for-sale*`; `/api/v1/search/listings` → `/api/v1/search/for-sale`; envelope keys `"listings"/"listing"` → `"for_sales"/"for_sale"`.
- **deep-link** `/listing/{id}` → `/for-sale/{id}` (share/comment/chat refs).
- **moderation resource** `'fixed_price_sale'` → `'for_sale'`.
- **events/outbox** `fixed_price_sale.*` → `for_sale.*` (worker, promotion event handler, `outbox_event_registry`).

### Fail-closed
- `listing` on wire rejected post-rename (attachment validator already locks this; extend to saved-item/discount input parsing if they accept targetType).

---

## 7. MOBILE IMPACT (the plan)

- **`lib/domains/commerce/catalog/listing/`** → rename to `catalog/for_sale/`:
  - `Listing`→`ForSale` (entity already stores `fixedPriceSaleId` — good precedent)
  - `ListingStatus`, `ListingVisibility`, `ListingLocation`, `GetListingsParams`, `CreateListingRequest`, `UpdateListingRequest` → `ForSale*`
  - files: `listing.dart`, `listing_dto.dart`, `listing_repository*`, `listing_remote_datasource.dart`, `listing_controller.dart`, `listing_providers.dart`, `listing_picker_bottom_sheet.dart`, `create_listing_route_contract.dart`, `listing_media_handler.dart`, `presentation.dart`, `listing_module.dart`, screens.
- **wire parsing**: DTOs parsing `"listing"/"fixed_price_sale"` targetType/objectType → `"for_sale"`; fail-closed on stale `"listing"`.
- **routes/deep-links**: `route_paths.dart`, `listing_module.dart`, `app_router.dart` — `/listing`, `/create-listing` → `/for-sale`, `/create-for-sale`.
- **saved item / discount / promotion / negotiation / report / search / feed / commerce_preview / chat / comment** — every surface `targetType`/`resourceType` literal `"listing"` or `"fixed_price_sale"` → `"for_sale"` + mapping switches (`search_result_type_helper.dart`, `feed_mapper.dart`, `attachment.dart`, `share_reference.dart`, `object_reference.dart`, promotion DTOs, etc.).
- Product stays product; Auction stays auction; keep `productId` reference.
- l10n `app_en.arb` / `app_id.arb`: surface labels "Listing" → "For Sale" where they name the surface.

---

## 8. TEST IMPACT (the plan)

Across backend `*_test.go` and mobile `test/`:
- Rename expected wire strings (`"listing"`→`"for_sale"`, `"fixed_price_sale"`→`"for_sale"`) in contract tests.
- Update enum/type references to `ForSale*`.
- Update route expectations (`/api/v1/listings` → `/api/v1/for-sale`).
- **Keep** negative tests that lock legacy `listing` rejection (fail-closed).
- **Prune** stale tests (e.g. `fixed_price_sale_create_sender_address_test.go`).
- Real-Postgres integration tests must cover: availability (active+quantity>0), one-active-surface via renamed trigger/index, saved-item/discount/promotion enum backfill, order source rename.

---

## 9. DEAD / ZOMBIE CANDIDATES (Stage F removal)

- Orphaned DB enums `listing_type_enum`, `listing_origin_enum`, `listing_visibility_enum`.
- DB `order_source_enum` dead value `'listing'`.
- Stale `fixed_price_sale_create_sender_address_test.go` (`PublishNow`).
- `serverboot/chat_commerce_reference.go` (never existed — do not chase).
- Dead mobile screens/providers under `catalog/listing/` no longer routed (verify against router registry).
- Old `/api/v1/listings*` + `/api/v1/search/listings` routes (replaced, no compat).
- Stale docs contradicting code: `ATTACHMENT_SCHEMA_V2.md`; PASS_21C comments styling `listing` as current.

---

## 10. PROPOSED STAGED RENAME ORDER

1. **Stage B — Domain rename (pure Go, no DB):** `fixedprice`→`forsale`, `FixedPriceSale*`→`ForSale*`, event string consts, all `.go` identifiers/comments/local `listing` aliases in-package + consumers (serverboot, seed, routes, social, chat, negotiation, moderation, promotion, discount, saved-item, order/cmd). Compile green.
2. **Stage C — Schema:** new `000047` migration (table/enum/column/constraint/index/trigger renames + backfill + drop orphan enums + dead `'listing'`). Apply + integration green.
3. **Stage D — API/wire:** routes, JSON fields, objectType/targetType `"for_sale"`, fail-closed on `"listing"`, feed/search promo unification, events/outbox. Runtime proof over real PG.
4. **Stage E — Mobile:** domain + wire + routes + DTO/entity/providers/l10n.
5. **Stage F — Cleanup:** grep `listing|fixed_price_sale|fixedprice` to zero in production surface code; classify and prune residue.
6. **Stage G — Final proof:** 15-point regression over real Postgres.

---

## 11. RISKS

- **Order / event data** store `"fixed_price_sale"`; enum value can't `RENAME VALUE` on older PG — needs destructive value rewrite + backfill (§12).
- **Feed vs search promo already diverge** (`listing` vs `fixed_price_sale`); renaming both to `for_sale` is a wire change consumed by mobile — must ship mobile DTO change in lockstep.
- **Cross-domain enum coupling** (`order_source_enum`, `sale_surface_type_enum`) ripples into payment/worker/outbox code — mechanical rename only, no behavior change (per scope boundary).
- **Large diff** (~9,000+ token hits): risk of over-renaming generic "listing". Mitigated by per-cluster §3 classification.
- **One-active-surface invariant** must survive trigger/index rename (runtime proof).
- **Product reuse** correctness must survive (reuse guards reference surface status).

---

## 12. BLOCKERS / OWNER DECISIONS

1. **HTTP route shape** — `/api/v1/for-sale` (singular, matches feature name; recommend) **vs** `/api/v1/for-sales` (plural, symmetric with `/auctions` and current `/listings`). Envelope key `for_sale` / `for_sales`.
2. **DB enum value migration** on `order_source_enum` / `sale_surface_type_enum` — since no prod data, recommend **full destructive rewrite migration** (new type with `'for_sale'`, drop `'listing'`, `ALTER COLUMN TYPE`, backfill) rather than PG12 partial value-add. Confirm allowed.
3. **Outbox/event persistence** — confirm events (`fixed_price_sale.*`) may be rewritten wholesale (no external consumers).

---

## 13. STOP / GO CONCLUSION

**Verdict: GO for the staged rename to `for_sale`**, provided the owner confirms the §12 decisions.

- Product/Auction boundaries are unaffected; Product stays the canonical content+identity authority.
- The `fixedprice` package is 100% fixed-price surface, so **no unresolved semantic ambiguity blocks Stage B**.
- The only owner gates are: (1) HTTP route shape, (2) destructive enum migration, (3) event wipe — all bounded to this scope.

**STAGE A COMPLETE — READ-ONLY COMPLETE, NO FILES CHANGED.** STOP — awaiting instruction to begin Stage B.