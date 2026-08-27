# COMMERCE_PRODUCT_SEMANTIC_DECISION_AUDIT

MODE: READ-ONLY. Filesystem is the only implementation truth. Nothing modified.
Every claim re-derived this pass against current files, not inherited from prior reports.

---

## 1. EXECUTIVE VERDICT

The semantics of `Product` in Labuda Commerce are **contradictory and unresolved**. There is no single definition the codebase honors:

- **Documented intent (doctrine + split-design docs):** `Product` = "internal physical item authority"; koi are unique-by-default; a sold Product MUST NOT be reused; one live sale per product (`commerce-selling-doctrine.md:9,51-52,64-68`; `commerce-db-model-split-design.md:335,420`).
- **Documented UX intent:** seller flow is "sale-intent-first, NOT inventory-first" (`commerce-selling-doctrine.md:15,121,136`) — Product is deliberately NOT a seller-facing surface.
- **Actual behavior:** every FPS create and every auction create **mints a brand-new `products` row** from inline form fields (`fixed_price_sale_repository_impl.go:38-51`; `auction_service.go:260-278`); **no create path accepts or reuses an existing `product_id`** (`CreateFixedPriceSaleInput` has no `ProductID`, `fixed_price_sale_service.go:107-135`; no `input.ProductID` anywhere in `auction_service.go`); there is **no standalone product surface** (only `PUT /products/:id/shipping`, `routes_core.go:338-345`).

So `products.id`, in the running system, identifies a **one-sale-attempt-scoped projection row**, not a stable physical koi and not a reusable catalog item. Choice of final semantic is **OWNER DECISION REQUIRED**; evidence is insufficient to pick one uncontested interpretation (Section 11). The system also casually combines **unique-item semantics** (default `quantity=1`, koi vertical) with **batch semantics** (`quantity>1` supported, `000009`; PRD stock / `RESERVED` lifecycle wording), which is the load-bearing contradiction for Product identity.

## 2. PRODUCT SEMANTIC DEFINITION

**What Product IS today (behavioral):**

- Created only as a side-effect of a sale-surface create:
  - FPS: `FixedPriceSaleRepositoryImpl.Create` builds product via `buildProductFromSale` and `productRepo.Create` in the same tx (`fixed_price_sale_repository_impl.go:33-51,599-626`).
  - Auction: `AuctionService.CreateDraft` builds product inline and `productRepo.Create` in the same tx (`auction_service.go:258-278`).
- `products.id` is then referenced by: `fixed_price_sales.product_id` (FK), `auctions.product_id` (FK), `pricing_tokens.product_id`, `shipping_quotes.product_id`, `order_items.product_id` (no FK, and FPS path writes fps.id instead), `product_shipping_options.product_id`.
- Never read or rendered as a user-facing entity anywhere (no GET/POST product route, no mobile Product model, no product card; share/search/feed/saved all key on `fixed_price_sale_id`/`auction_id`).

**What Product IS claimed to be (documentation):**
- "internal physical item authority" (`product.go:9-10`; `commerce-selling-doctrine.md:9`; `commerce-db-model-split-design.md:17,340`).
- Carries the koi facts: `variety, size_cm, age_months, gender, breeder, bloodline, certificates, media_urls` (+ title/description/farm_address/preparation) — `products` DDL `000001:1322-1342`; `product.go:10-31`.

**Contradiction:** a "physical item authority" that is re-created (new UUID) on every sale attempt and never reused across sale attempts cannot be the physical item's identity. Either the item's identity is the doc claim (and the implementation is wrong) or the item's identity is the implementation (and the docs are wrong). Not resolvable from code.

## 3. QUANTITY SEMANTICS

Every relevant fact, traced:

| Fact | Evidence |
|---|---|
| `quantity_available` lives on `fixed_price_sales` only (Product has none) | `000009`; `000001:868-880`; `fixed_price_sale.go:40` |
| Default is **1 = unique-item mode** | `fixed_price_sale_handler.go:62-65,229-232` ("unique-item mode (quantity=1) when omitted; sellers with real stock set it explicitly to enable multi-quantity sale") |
| `min=1`, no cap | `binding:"omitempty,min=1"` `fixed_price_sale_handler.go:65` |
| Quantity change blocked once orders exist | `fixed_price_sale_handler.go:385,400` (price/title/quantity frozen) |
| Reduce → sold at 0, restore on cancel/expire, only via OrderService | `fixed_price_sale.go:229-287` |
| PRD models FPS with stock + `RESERVED` lifecycle state | `PRD.md:309,313` (`DRAFT→ACTIVE→RESERVED→SOLD/INACTIVE`) — **no `RESERVED` state exists** in `fixed_price_sale_status_enum` (`000001:133-138`); stock is reduced at order create, not reserved as a state |
| Doctrine says multi-stock deferred / koi unique | `commerce-selling-doctrine.md:51-52`; `commerce-db-model-split-design.md:50` — **contradicted by `000009`** (recorded owner decision: multi-quantity is supported) and by the live handler |
| "sale-intent-first, not inventory-first"; Mobile/UX must not force inventory-first | `commerce-selling-doctrine.md:15,121,136`; `commerce-db-model-split-design.md:784` |

**Explicit answers:**

1. One Product row is intended to be **one physical koi (docs)** OR **a sellable definition with N fungible units (quantity>1 behavior)** — the two readings cannot both hold. `quantity_available` semantics force the choice: quantity=1 ⇔ unique item; quantity=10 ⇔ ten **fungible units of one product row**.
2. If FPS quantity = 10, the 10 units are **10 sellable copies sharing one title/one media/one koi description/one seller/one price** — i.e. batch copies of a catalog-style definition; no per-unit identity exists (no unit table, no SKU, no variant, no serial number).
3. Can those 10 units become 10 separate auctions? **NO.** Auctions are single-unit and carry no quantity; each auction create mints its own product row; there is no path that splits one product/qty into per-unit identities.
4. Where would their identities live? Nowhere — no per-unit row exists. Under the current model the 10 units are indistinguishable; only the single `quantity_available` counter exists.
5. Business meaning of quantity=10 today: **N fungible copies of the same FPS row** — the seller sells up to 10 units of a described koi until the counter exhausts. Nothing personalizes a unit after sale; an order buys "1x of that FPS" only (`order_items.quantity`, `Request.Quantity`).

## 4. PRODUCT VS SALE SURFACE OWNERSHIP

| Concern | Product | FixedPriceSale | Auction |
|---|---|---|---|
| identity | CANONICAL (`products.id`) | DERIVED (FK `product_id`; minted per create) | DERIVED (FK `product_id`; minted per create) |
| title | CANONICAL (column) | DERIVED (joined read; in-memory dup) | DUPLICATE (own column + product copy) |
| description | CANONICAL | DERIVED (joined) | DUPLICATE |
| media | CANONICAL (`media_urls`) | DERIVED (joined) | DERIVED (no auction column) |
| seller | DUPLICATE (`products.seller_id`, synced by FPS write-through, never synced by auction) | DUPLICATE (`fixed_price_sales.seller_id`) | DUPLICATE (`auctions.seller_id`) |
| price | — | CANONICAL (`price_per_unit`) | CANONICAL (`start_price/bid_increment/buy_now_price`) |
| quantity / stock | — | CANONICAL (`quantity_available`) | N/A (single-unit) |
| preparation time/note | CANONICAL (`products.preparation_time/note`) | DERIVED (entity dup) | DUPLICATE + **diverges on auction edit** (no product sync) |
| shipping config | CANONICAL (`product_shipping_options`, `farm_address_id`) | DERIVED (read product) | DERIVED |
| sale status | DERIVED for FPS (via `derivedProductStatus`); explicit-write for auction; **no own state machine** | CANONICAL (`status` enum) | CANONICAL (`auctions.status`) |
| sold state | DUPLICATE (`products.sold_at`; FPS-derived, auction-explicit) | CANONICAL (`fixed_price_sales.sold_at`) | CANONICAL (`auctions`) |
| withdrawal | UNKNOWN (`products.withdrawn_at` **does not exist**; status derived) | CANONICAL (`withdrawn_at`) | CANONICAL (auction states) |
| bid state | — | — | CANONICAL (`current_bid/current_winner_id`) |
| historical sale state | UNKNOWN (`order_items.product_id` references fps.id for FPS, products.id for auction — can't reliably link history) | SNAPSHOT (`order_items` name/price/qty) | SNAPSHOT (orders) |

## 5. LIFECYCLE AUTHORITY

All writers (re-derived):

| Writer | Mutation | Path | Evidence |
|---|---|---|---|
| FPS order create | `products.status`/`sold_at` derived from FPS status | `CreateFromSaleSurface` → `UpdateStock` → `derivedProductStatus` | `order_creation_service.go:1520`; `fixed_price_sale_repository_impl.go:151-170,639-650` |
| FPS cancel/expire | status derived back (sold→active→available) | `RestoreQuantity` + `UpdateStock` | `order_completion_service.go:1995-2000` |
| Auction create | `products.status = "available"` | `CreateDraft` | `auction_service.go:275` |
| Auction buy-now order | `products.status = "sold"`, `sold_at` | `CreateFromAuction` | `order_creation_service.go:845-853` |
| Auction bid-win claim | `products.status = "sold"`, `sold_at` | `MarkAuctionProductSold` | `auction_service.go:1026-1035` |
| Auction order cancel/expire | product back to `"available"`, `sold_at=nil` | `releaseAuctionOrderBinding` | `order_completion_service.go:2037-2044` |

**Classification: MIXED / CONTRADICTORY.**
- FPS → Product status is a **derived projection** (consistent).
- Auction → Product status is an **explicit write** from order/claim flows (different mechanism, different timing).
- Auction edits never propagate to Product (§4 divergence), while FPS edits do (write-through).
- `products.withdrawn_at` referenced in requirement but absent from schema.
- No standalone Product lifecycle exists; no item-status writer outside sale/order flows.

## 6. IDENTITY TRANSITIONS

| Transition | Possible today? | Evidence | Identity continuity? |
|---|---|---|---|
| FPS → Auction (same koi) | Only by creating a new auction (new product row). No path attaches an auction to an existing product | auction create mints product, `auction_service.go:260-278` | NO — new `products.id` |
| Auction → FPS (same koi) | Only by creating a new FPS (new product row) | FPS create mints product, `fixed_price_sale_repository_impl.go:38-51` | NO |
| sold → relisted | Only as a new product + new surface; no reactivation of a sold product (`sold` terminal in `fixed_price_sale_status.go:62,78`; withdrawn terminal too; seller reopen path absent — only moderation `MarkActiveFromModeration`) | `fixed_price_sale_status.go:60-66` | NO |
| withdrawn → reopened | Not for sellers; moderation-only | `fixed_price_sale.go:186-206` | N/A |
| unsold-ended auction → reuse product | Product status flips to `available` (`order_completion_service.go:2037-2044`) but **no code ever reuses that product row** | | NO (dormant) |

**Does any business workflow actually require identity continuity?** No current workflow keys history by product cross-sale. Share/repost/saved/comments/orders all key on the sale surface (`fixed_price_sale_id`/`auction_id`) or order. The ONLY product-keyed history is `order_items.product_id`, which for FPS orders stores fps.id anyway (§8 prior). So continuity is required **only if** the owner decides Product should be stable catalog/physical identity; nothing in the running flows enforces a need today.

## 7. MODEL A / B / C COMPARISON

| Concern | MODEL A: Product = stable physical koi | MODEL B: Product = sellable catalog/inventory definition | MODEL C: Product = per-sale-attempt internal projection |
|---|---|---|---|
| identity behavior | product row reused across sale attempts for the same koi | product row reused as the definition; sale rows attach | fresh product row per create (matches current) |
| quantity behavior | qty=1 always (unique koi) | qty = number of fungible units of the definition | qty on the sale row (matches current) |
| relisting | resell same product row; requires status re-arm | new sale row on same definition | new product (current) — continuity impossible |
| FPS→Auction transition | attach auction to same product row | attach auction to same definition | new product (current) |
| historical sales | all orders trace to one product | orders trace to definition + sale row | orders trace to fps/auction surface (current) |
| shipping | product-scoped ✓ | product-scoped ✓ | product-scoped ✓ (but product is mint-per-attempt) |
| order items | `products.id` always | `products.id` always | `products.id` WOULD be stable per attempt; currently FPS writes fps.id (bug) |
| future product/catalog surface | one page per koi — natural | one page per catalog item — natural | **no stable id — requires rewrite** |
| matches actual code? | NO (never reused) | NO (definition never reused) | YES (behavior fits exactly) |
| matches written doctrine/db-design? | YES (physical authority, unique default, sold-not-reused) | PARTIAL (defines sellable set; conflicts with "sale-intent-first/not inventory-first"; doctrine calls koi unique) | NO (contradicts both docs) |

**THE EVIDENCE IS INSUFFICIENT TO CHOOSE A vs B.** The codebase simultaneously behaves like C (fresh-row projection, which is what you get before a decision) while its docs claim A (physical authority) with B-flavored quantity (`quantity>1`) and B-flavored UX-rejection (sale-intent-first). A and B cannot both be true for what one `products.id` row IS. Choose A or B; C is the undecided default and must be eliminated by that choice.

## 8. CONTRADICTIONS

1. **`products.id` meaning**: doc "physical item authority" vs behavior "minted per sale attempt, never reused" (§2).
2. **Quantity**: docs "unique, multi-stock deferred" (`commerce-selling-doctrine.md:51-52`; `commerce-db-model-split-design.md:50`) vs `000009` + handler (multi-quantity live, default 1) + PRD stock/`RESERVED` wording (§3). A and B mutually exclusive.
3. **PRD `RESERVED` FPS lifecycle** (`PRD.md:313`) vs actual status enum `draft/active/sold/withdrawn` (no RESERVED; stock reduces at order create) — stale doc.
4. **Product lifecycle**: "sale-surface agnostic" (`product.go:9-10`) vs four sale/order writers (§5) — mixed model.
5. **Auction metadata**: own `title/description/preparation` columns AND `product_id`; diverges on auction edit (no product sync) — duplicate authority (§4) while FPS has none.
6. **`order_items.product_id`** two id-spaces: FPS→fps.id (`order_creation_service.go:1677`), auction→products.id (`:992`); no FK (`000001:1024-1033,2134`; only order_id FK) — verified.
7. **`listing` wire word** means fps.id in `saved_items` (`saved_item_repository_impl.go:207`) but products.id in discount matching (`pricing_token_service.go:358-360`) — namespace collision.
8. **`products.withdrawn_at`** required/implied by derived mapping (`derivedProductStatus` maps FPS withdrawn→product 'withdrawn') but column absent from schema.
9. **Seller identity**: three unconstrained `seller_id` copies; FPS synced, auction not (§4, §6).

## 9. PROVEN GOOD

- Price on the sale channel (FPS `price_per_unit`; auction bid/start/buy-now), never on Product — correct.
- Quantity on FPS (`quantity_available`, guarded ≥0, reduce/restore only via OrderService) — correct placement once model chosen.
- Bid state on Auction, not Product — correct.
- Old `listings` table gone + banned (`schemaguard` PASS this pass) — no structural resurrection.
- `pricing_tokens.product_id` consistent `products.id` for both channels (`pricing_token_service.go:996` FPS→`listing.ProductID`, `:1302` auction→`auction.ProductID`).
- Shipping configuration product-scoped and correct.
- One-active-channel-per-Product cross-table trigger exists and matches documented exclusivity (000010).
- Sale-surface-first UX matches code reality (no product surface).
- Share/comment/chat/search/feed reference sale-surface ids (`fixed_price_sale_id`/`auction_id`) — stable, canonical.

## 10. PROVEN BAD

- `order_items.product_id` id-space mixing (bug; no FK).
- Auction duplicating + diverging product metadata.
- Product identity discontinuity by construction (fresh UUID per create) — makes any future product/catalog/page impossible without identity migration.
- Product lifecycle mixed (FPS derived vs auction explicit) — asymmetric.
- Discount `'listing'` id-namespace ambiguity.
- PRD/doctrine quantity and `RESERVED` documentation contradict implementation.
- `products.withdrawn_at` missing column but derived-status maps to `'withdrawn'`.

## 11. OWNER DECISION REQUIRED

1. **D-1 (the semantic): choose A or B.** "Product = one physical koi (unique)" vs "Product = sellable definition with N fungible units". Everything else depends on it.
2. **D-2 consequence of D-1 — reuse:** if A: one product per koi, reuse the row across sale attempts and across channels, enforce `sold`-not-reused or relist policy explicitly. If B: reuse the row as the catalog definition; attach sale rows; per-unit (auction) identity must be a new model. Current code does neither.
3. **D-3 product lifecycle authority:** keep `products.status/sold_at` as a **unified** projection written by a single mechanism, or remove it and derive availability from channel states.
4. **D-4 quantity policy final:** multi-quantity live (000009) vs doctrine text; requires doc reconciliation, not code.
5. **D-5 auction metadata:** single authority (product) vs auction-owned snapshot; must match D-1.

## 12. RECOMMENDED ARCHITECTURE

Conditional on D-1; no implementation implied:

- If **MODEL A** (physical koi): rework creates to (a) find-or-create product by a stable koi key and REUSE `product_id` across FPS/Auction sale rows; add per-sale `closed/archived` channel history; order_items always `products.id`; quantity effectively 1 (single koi) — multi-quantity would mean batch of identical koi, which contradicts "physical item"; if batch is real, D-1 is actually B.
- If **MODEL B** (catalog definition): product row = the item; FPS = one active sellable price/qty channel over it; auction = one active single-unit channel over it; optional per-unit identity only if units must be individually traceable (auctions). One active channel per product (000010) fits B cleanly; relisting = new channel rows on the same definition; order history links to product + channel.
- ALWAYS (either model): repair `order_items.product_id` to `products.id`; collapse auction duplicate metadata onto product; unify product-status writes through one mechanism; resolve `withdrawn_at`; reconcile PRD/doctrine.

## 13. VOCABULARY RECOMMENDATION

Assessment only (no rename):

| Term | Actual referent today | Classification |
|---|---|---|
| Product | sale-attempt minted row + claimed "physical item authority" | **concept EXISTS but semantics undefined (D-1)**; currently misleading |
| FixedPriceSale | the fixed-price mechanism, owns price/qty/status | **accurate** canonical name for the concept |
| Auction | the bid mechanism, owns bid state | **accurate** |
| Listing | FPS alias everywhere (routes `/listings`, saved/discount `target_type='listing'`, feed wire `'listing'`, mobile domain), plus orphan enum members | **alias/residue** — not a separate concept |
| For Sale (ForSale) | UI label for FPS tab | **UI alias only** |
| FixedPriceListing | doctrine's preferred noun for FPS | **alias of FixedPriceSale** (name drifts between docs) |
| Sale Item / Sellable Item | "sellable item" appears in FPS entity heredocs (`fixed_price_sale.go:2,16`) and Product docs; describes the hybrid | **alias/ambiguous** — collapses Product+FPS distinction |

No canonical-name decision can be finalized before D-1; the vocabulary layer must be resolved after identity semantics.

## 14. IMPLEMENTATION DEPENDENCIES

Ordered dependencies that the (future) implementation must respect:
1. D-1 (choose A or B) → gates D-2 (reuse) and everything that touches product identity.
2. `order_items.product_id` fix depends only on D-1's *id-space* (always `products.id`), safe under both A and B — but for A it additionally requires product reuse so the id is meaningful.
3. Auction metadata collapse depends on D-5 (snapshot vs derive), itself dependant on D-1.
4. Product lifecycle unification depends on D-3.
5. Quantity policy (multi-qty vs unique) is D-4, dependant on D-1 (B supports it; A implies qty=1).
6. Vocabulary rename is LAST (after D-1/D-4).
7. `quantity`/`stock` in PRD + `RESERVED` state text must be reconciled once D-4 lands.

## 15. NOT PROVEN

- Whether a single physical koi is ever intended to be sold more than once (relisting a sold fish) — docs forbid, code allows-via-new-row, no runtime test drives the path.
- Whether the "10 units" of a multi-quantity FPS are meant to be identical koi (batch) or the same koi repeated — no doc resolves it; D-1 open.
- Whether Product reuse would ever be wired (no code hint of reuse; all inputs exclude `product_id`).
- `pricing_tokens.product_id` FK existence not re-verified at schema; `shipping_quotes.product_id` FK existence likewise.
- Auction edit → product divergence is asserted from absence of sync code, not from a runtime test.

---

**READ-ONLY. STOP. Decisions D-1…D-5 remain open for the owner before implementation begins.**