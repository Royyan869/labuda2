# COMMERCE PRODUCT IDENTITY & LIFECYCLE AUDIT

READ-ONLY. Filesystem is the only truth. No code/schema/docs modified.

Re-derived this pass, independently of earlier reports. Where earlier reports made claims, the claims were re-proven (or disproven) against current files in this pass.

---

## 1. Executive verdict

The `Product → FixedPriceSale / Auction` split is structurally real (old `listings` table gone; only `products`, `fixed_price_sales`, `auctions` exist as commerce core rows). But **Product identity and lifecycle are internally contradictory**:

1. **`products.id` is NOT a stable physical-koi identity.** Every FPS create (`fixed_price_sale_repository_impl.go:38-51`) and every auction create (`auction_service.go:260-278`) mints a brand-new `products` row; no create path accepts or reuses an existing `product_id`. Product is de-facto a **one-sale-attempt-scoped catalog projection**, while every doc calls it the "physical item authority" (`product.go:9-10`; `commerce-selling-doctrine.md:9`). Identity semantics = documentation vs behavior conflict.
2. **Product lifecycle is a mixed, asymmetric model.** FPS path derives `products.status`/`sold_at` from FPS status (`UpdateStock` → `derivedProductStatus`, `fixed_price_sale_repository_impl.go:151-170,639-650`). Auction path writes `products.status='sold'` explicitly on buy-now (`order_creation_service.go:845-853`) and bid-win claim (`auction_service.go:1006-1035`), and reverts to `'available'` on order release (`order_completion_service.go:2033-2044`). FPS writes no `withdrawn_at` on product (no column exists). Two channels, two styles, one mixed authority.
3. **`order_items.product_id` stores two namespaces in one column** — re-proven: FPS → `fixed_price_sales.id` (`order_creation_service.go:1677`), auction → `products.id` (`:992`). Column is documented as "product relationship" (`order_item.go:11-18`) and is false on the FPS path.
4. **The string `listing` is live in two same-named but different id-namespaces.** `saved_items.target_id` for 'listing' = `fixed_price_sales.id` (join proven, `saved_item_repository_impl.go:207`); `discount_targets.target_id` for 'listing' is matched at checkout against **product ids** (`pricing_token_service.go:358-360`) with no FK and no id-space validation (`discount_repository.go:572-581`).

No destructive refactor is justified by logic alone; a narrow identity/lifecycle convergence is (see §17). Naming is deliberately NOT solved here.

---

## 2. Product identity authority

| Question | Answer | Evidence |
|---|---|---|
| What does `products.id` identify? | **A sale-attempt-scoped catalog projection, not a stable physical koi.** Doc says otherwise. | Every FPS create mints a new product (`fixed_price_sale_repository_impl.go:38-51`); every auction create mints a new product (`auction_service.go:260-278`). No create input accepts `ProductID` (FPS: `CreateFixedPriceSaleInput` at `fixed_price_sale_service.go:107` has no ProductID field; auction: no `input.ProductID` anywhere in `auction_service.go`). |
| Producer | FPS repo `Create` (`:33-51`), auction `CreateDraft` (`auction_service.go:223,277`) | |
| Storage | `products` table (`000001:1322-1342`) | |
| Mutation | same sale-channel rows only (edit keeps `product.ID=listing.ProductID`) | `buildProductFromSale` `:604` |
| Consumer | FPS join (`joinedSaleByIDQuery`), auction join, `order_items`, `pricing_tokens`, `product_shipping_options`, `shipping_quotes`, discount match, content/comment FPS previews | |
| Lifecycle authority | FPS status-derived write-through (`:151-170,639-650`) + auction explicit writes (`auction_service.go:1026`, `order_creation_service.go:847`) | |

**Conclusion.** `products.id` is (documented) "internal physical item authority" but (behaved) a **per-sale-attempt catalog row**. The physical-koi reading is not honored by any path. Classification: **CONTRADICTION** between docs and behavior; owner decision required on the intended identity.

## 3. Product lifecycle authority

Every writer of `products.status` / `products.sold_at` (there is **no** `products.withdrawn_at` column):

| Writer | Status/SoldAt mutation | Path | Evidence |
|---|---|---|---|
| FPS order create | `UpdateStock` → `derivedProductStatus` (active→available, sold→sold, withdrawn→withdrawn, else draft) | `CreateFromSaleSurface` → `listingRepo.UpdateStock` | `order_creation_service.go:1520`; `fixed_price_sale_repository_impl.go:162-170,639-650` |
| FPS cancel/expire | `RestoreQuantity` (sold→active) → `UpdateStock` derives back to available | `OrderCompletionService.Restore` | `order_completion_service.go:1995-2000` |
| Auction create | `Status="available"` | `CreateDraft` | `auction_service.go:275` |
| Auction buy-now order | `Status="sold"`, `SoldAt=now` | `CreateFromAuction`, buy-now branch | `order_creation_service.go:845-853` |
| Auction bid-win claim | `Status="sold"`, `SoldAt=now` | `MarkAuctionProductSold` | `auction_service.go:1006-1035` |
| Auction order cancel/expire release | back to `"available"`, `SoldAt=nil` | `releaseAuctionOrderBinding` | `order_completion_service.go:2033-2044` |

**Classification: MIXED model.**
- FPS: product state is a **derived projection** of FPS state (write-through).
- Auction: product state is an **explicit order/claim-derived write**, asymmetric with FPS; auction *edits* never sync the product (no `productRepo.Update` on auction Update — only Create and MarkSold touch product).
- Product is never mutated by a standalone "item" flow; there is no independent item lifecycle.

Documentation claims Product is sale-surface agnostic (`product.go:9-10`, `commerce-selling-doctrine.md:9`) while four runtime writers couple it to sale/order outcomes. **CONTRADICTION.**

## 4. FixedPriceSale relationship

| Fact | Determined | Evidence |
|---|---|---|
| FPS is a sale mechanism over Product | YES — FPS row stores no title/desc/media/koi columns; those live in `products`, joined (`joinedSaleByIDQuery` → `scanJoinedSale`) | `fixed_price_sales` DDL `000001:868-880`; entity has `Product *productEntity.Product` `fixed_price_sale.go:71` |
| FPS owns price | YES — `price_per_unit` | `000001:872`; `fixed_price_sale.go:39` |
| FPS owns quantity | YES — `quantity_available` (000009) | `000009`; `fixed_price_sale.go:40`; non-neg CHECK |
| FPS owns sale state | YES — `status` + `published_at/sold_at/withdrawn_at` | `000001:874-877` |
| FPS duplicates Product identity | Partial — `product_id` FK + full in-memory `Product` projection `fixed_price_sale.go:71`; entity also repeats koi fields (`:29-35`) | |
| FPS exists historically after sale | YES — sold rows persist (status=terminal, not deleted) | `fixed_price_sale_status.go:60-66` |
| Multiple FPS rows for one Product | Schema allows (no unique `product_id`; only partial unique active index); runtime unreachable because each create mints a fresh product | `000010` partial unique index; create mints product `:38-51` |
| Relisting | **Accidentally possible as a NEW product**; never reuses an existing product row; no relist-specific gate | no create path accepts product id; `derivedProductStatus` has no sold-product guard |

## 5. Auction relationship

| Fact | Determined | Evidence |
|---|---|---|
| Auction is a sibling mechanism over Product | YES — `product_id NOT NULL` FK mandatory | `000001:477-498` |
| Auction owns price/bid state | YES — `start_price`, `bid_increment`, `buy_now_price`, `current_bid`, `current_winner_id` | `000001:487-493` |
| Auction owns quantity | NO quantity defined; single-unit buy-now only | `docs/adr/009:111`; no quantity column |
| Auction duplicates Product metadata | YES — stores own `title`, `description`, `preparation_time`, `preparation_note` (columns `000001:483-486`) AND `product_id`. FPS does not duplicate. | |
| Auction title/desc/prep: independent authority or duplicate? | **Duplicate authority at rest; also diverges on edit**: auction edits persist to auction row only (no product sync), so product snapshot goes stale | no `productRepo.Update` on auction update path; only Create/MarkSold (grep of `auction_service.go` productRepo uses → `:277`, `:1010-1032`) |
| Auction can exist without Product | NO — `product_id NOT NULL`, created inline same-tx | `000001:497`; `auction_service.go:260-278` |
| Auction + FPS coexist for one Product | Forbidden while active/pending (cross-table trigger), enforced in DB | `000010` trigger |
| DB enforcement matches documented rule | YES — trigger mirrors doctrine exclusivity (proven at schema level) | `000010` |

**Note:** because every create mints a fresh product, the cross-table trigger is currently **dormant** (no path shares a product row). It is future-proofing, not live enforcement.

## 6. Seller ownership authority

- Three columns: `products.seller_id`, `fixed_price_sales.seller_id`, `auctions.seller_id`.
- Producer: single create input → same seller written to both rows (FPS repo `Create`; auction `CreateDraft` → product + auction same `SellerID`).
- Mutation sync: FPS path keeps product synced (`buildProductFromSale:605` `product.SellerID = listing.SellerID` on every create/update). Auction path never writes product after create.
- Constraint: **no** DB equality/FK between the three seller columns → divergence is schema-legal.
- Readers: ownership checks read the surface row's seller (`listing.SellerID`, `auction.SellerID`); promotion/saved-item/comment/order/search resolve seller from the surface or `seller_profiles`. `products.seller_id` is read via joins.
- Assessment: **intentional denormalized copies with a single producer, weakly enforced** (FPS syncs, auction doesn't; no constraint). Not a competing authority today, but no guarantee against divergence. Classification: `H` (duplicate authority, low risk, unconstrained).

## 7. Cross-domain identity matrix

| COLUMN | EXPECTED NAMESPACE | ACTUAL WRITER | ACTUAL READER | FK/constraint | AMBIGUITY |
|---|---|---|---|---|---|
| `orders.source_id` | fps.id for FPS; auction.id for auction | `CreateFromSaleSurface`/`CreateFromAuction` (`order_creation_service.go:1615`, `:990-1014`) | order read/query/completion | no FK | LOW — namespace governed by `source_type` |
| `order_items.product_id` | `products.id` (per doc `order_item.go:11-18`) | FPS → **fps.id** `:1677`; auction → **products.id** `:992` | completion restore treats it as fps.id (`order_completion_service.go:1989`) | nullable uuid, **no FK** (`000001:1032`) | **HIGH — same column, two tables' ids** (re-proven) |
| `pricing_tokens.product_id` | products.id | `pricing_token_service.go:996` `listing.ProductID`, `:1302` `auction.ProductID` | token validation `pricing_token.go:473` | FK? not verified; index exists | LOW — consistent products.id |
| `pricing_tokens.source_type/source_id` | fps.id / auction.id | same writer | shipping-quote/order validation (`order_creation_service.go:547`) | none | LOW |
| `discount_targets.target_id` | 'listing' rows → **products.id** (matched by token `req.ProductID`) | `discount_repository.go:579` stores arbitrary `listingID` parsed from `applicable_listing_ids` (`discount_handler.go:352-359`) | token match `pricing_token_service.go:358-360`; read `discount_repository.go:558` | **no FK** | **HIGH — 'listing' namespace collides with saved_items (below); no id-space validation** |
| `saved_items.target_id` | 'listing' rows → **fps.id** | saved-item create `target_type:'listing'` + fps id | `LEFT JOIN fixed_price_sales fps ON si.target_id=fps.id` (`saved_item_repository_impl.go:207`) | CHECK target_type('listing','auction') `000001:2515`; no FK | MEDIUM — same string 'listing' but fps namespace (vs discount's product namespace) |
| `shipping_quotes.product_id` | products.id | chat quote create | order validation `order_creation_service.go:529-553` | no FK verified | LOW |
| promotion `target_id` (+target_type) | fps.id / auction.id | `promotion_repository_impl.go:667-679` | feed injector/discovery | yes (ownership join) | LOW |
| comment/share/chat `fixed_price_sale_id` / targetType | fps.id | comment commerce-ref, chat attachment | preview resolvers | FK on comment_commerce_references | LOW |

**Two real ambiguities, both re-proven:** (1) `order_items.product_id`; (2) `discount_targets` 'listing' = product ids while `saved_items` 'listing' = fps ids, sharing one wire word.

## 8. OrderItem identity proof

- `OrderItem` (entity, `order_item.go:10-33`): `ProductID uuid.UUID`, doc: "captures the product relationship… (copied from listing…)".
- **FPS writer**: `NewOrderItem(order.ID, listing.ID, …)` (`order_creation_service.go:1675-1681`) — `listing.ID` is `fixed_price_sales.id` (the `listing` is the FPS entity fetched via `GetForUpdate`, `:1362`).
- **Auction writer**: `NewOrderItem(order.ID, product.ID, …)` (`:990-996`) — `product.ID` is `products.id`.
- **Reader that betrays the namespace**: `OrderCompletionService.RestoreOrderItems` does `listingRepo.GetForUpdate(ctx, tx, item.ProductID)` (`order_completion_service.go:1989`) — the FPS repository lookup only works because for FPS orders `ProductID` was loaded as fps.id. Auction orders never reach this path (they go through `releaseAuctionOrderBinding`, `:2021`).
- **Implication**: any code that treats `order_items.product_id` as `products.id` will break for FPS orders, and vice versa. The column is a polymorphic footgun.

**PROVEN (independent re-derivation).** This is an implementation bug, not a design choice.

## 9. Discount target identity proof

- Create: `applicable_listing_ids` parsed as UUIDs (`discount_handler.go:352-359,473-480`) → `insertDiscountTargets` writes `target_type='listing'` (`discount_repository.go:572-581`), CHECK allows it (`000001:2452`), **no FK on target_id**.
- Match: token generation calls discount resolution with `discountentity.DiscountContextListing` and `input.ListingID = &req.ProductID` (`pricing_token_service.go:358-360`) → the discount only ever applies if the stored 'listing' id IS a **product id**.
- Saved items uses the same wire word 'listing' but means an **fps id** (§7). Nothing validates which namespace a discount target id belongs to; a wrong-namespace id silently never matches.

**PROVEN.** Ambig-id namespace under a misleading name.

## 10. Product reuse / relisting behavior

Two combinations, and the reverse:

| Scenario | Documented policy | Actual behavior |
|---|---|---|
| Product A → FPS sold → new FPS | Sold product MUST NOT be reused (`commerce-selling-doctrine.md:67`; `commerce-db-model-split-design.md:420`) | New create mints a **new product** row; old product remains `'sold'` (derived). No gate blocks it. Behavior = "allowed as a new item row" |
| Product A → auction sold → new FPS | same doctrine | New FPS create mints a new product; no guard examines old auction/product state |
| Product A → FPS withdrawn → auction | Doctrine: withdraw before auction (`commerce-selling-doctrine.md:64-68`); trigger stops live coexistence | New auction create mints a new product too — never touches the old product |
| Product A → sold (any) → returned to available? | Doctrine: unsold auction returns product to `available` (`commerce-db-model-split-design.md:382-383`) | `releaseAuctionOrderBinding` sets `'available'` (`order_completion_service.go:2037-2044`) — but **no code ever reuses that product row** (creates always mint fresh), so the state is reachable but dormant |

**Conclusion:** relisting is **accidentally possible only by minting a new Product**; the documented "physical item" is never the same row across attempts. Documented policy (doctrine invariants) and actual enforcement are **disconnected** (doctrine unenforced; fresh-product behavior undocumented).

## 11. Product surface audit

- HTTP: **only `PUT /products/:id/shipping`** exists (`routes_core.go:338-345`). No GET /products, no POST /products, no seller product list, no admin product surface.
- Mobile: **no Product model or screen** (mobile maps FPS detail/`Listing` with inline koi fields; auction detail). Search/feed/saved/promotion all key to fps.id or auction.id.
- Share/reference: fps.id/auction.id (never products.id).
- Search/filter surfaces: `GET /search/listings` → FPS handler.
- Assessment: Product is intentionally **internal-only today** (evidence supports internal root entity: zero user-facing product surfaces). No evidence that a product page was designed; the single `PUT /products/:id/shipping` proves Product is used as the **shipping-config anchor**.

## 12. Industry-design sanity assessment

| # | Question | Answer | Classification |
|---|---|---|---|
| 1 | Product → FPS/Auction coherent for Labuda? | YES conceptually | PROVEN GOOD |
| 2 | Product a stable item identity across mechanisms? | **NO** — new product per sale attempt | IMPLEMENTATION BUG vs DOC (owner decision on intended semantics) |
| 3 | Price on Product? | Correct to exclude — channels own price | PROVEN GOOD |
| 4 | Quantity on Product or FPS? | FPS (`quantity_available`) is correct; auction single-unit | PROVEN GOOD (documentation stale — see §13) |
| 5 | Bid state on Product or Auction? | Auction | PROVEN GOOD |
| 6 | Sale state on Product or channel? | Mixed today (FPS-derived + auction-explicit) → channel should own; product projection only | AMBIGUOUS / OWNER DECISION |
| 7 | Historical sales share one Product? | Not today (fresh product per attempt) | AMBIGUOUS / OWNER DECISION |
| 8 | Relisting same physical koi coherent? | Economically coherent; currently implemented as new identity | AMBIGUOUS / OWNER DECISION |
| 9 | Auction duplicated title/desc/prep defensible? | Undefensible at rest; diverges on edit | PROVEN BAD |
| 10 | seller_id duplication defensible? | Defensible as denormalized copy; risky unconstrained | AMBIGUOUS (weak enforcement) |
| 11 | order_items identity acceptable? | **No** — two id-spaces in one column | IMPLEMENTATION BUG |
| 12 | discount target identity acceptable? | **No** — nameless 'listing' namespace collides with saved-items fps namespace | IMPLEMENTATION BUG |
| 13 | Supports future product pages/catalog/search without rewrite? | Only if Product identity is stabilized first; today new Product-per-sale collides with "catalog product" | AMBIGUOUS / OWNER DECISION |

## 13. Contradictions

1. **Product identity**: docs "physical item authority" vs behavior "new row per sale attempt" (§2). → OWNER DECISION.
2. **Product lifecycle**: doc "sale-surface agnostic" vs four sale/order writers (§3). → OWNER DECISION.
3. **`order_items.product_id`**: column doc/meaning vs two writers (§8). → IMPLEMENTATION BUG.
4. **`listing` id-namespace**: discount (product ids) vs saved_items (fps ids) shared wire word (§7/§9). → IMPLEMENTATION BUG.
5. **Auction metadata**: auction row duplicates product content and diverges on auction edit (§5). → IMPLEMENTATION BUG / OWNER DECISION.
6. **Seller authority**: three unconstrained copies, FPS syncs, auction doesn't (§6). → AMBIGUOUS.
7. **Quantity doctrine**: `commerce-selling-doctrine.md:51-52` "unique, multi-stock deferred" vs `000009` owner decision (multi-quantity supported) + PRD `309`. → STALE DOCUMENTATION (resolved by 000009; doctrine text lags).
8. **Relisting doctrine**: "sold MUST NOT be reused" unenforced; fresh-product behavior undocumented (§10). → OWNER DECISION.
9. **Auction `expired_bnr` state**: listed in real DB (`GLOBAL_DOMAIN_SURFACE_AUDIT.md:270`), doctrine deletes it (`commerce-db-model-split-design.md:128`) — schema/doc conflict. → STALE DOCUMENTATION (schema side winning).
10. **Fixture drift**: `fixed_price_sale_create_sender_address_test.go:188,229` reference nonexistent `PublishNow` field — fixedprice test package fails to compile under `-tags integration`. → pre-existing IMPLEMENTATION BUG (tests).

## 14. Proven-good areas

- Sale surface owns price, quantity (FPS), bid state (auction): correct.
- Old `listings` table structurally gone + guarded (`schemaguard` PASS this pass; migration 000010 applied).
- Share/comment/reference identity = fps.id/auction.id, canonical `fixed_price_sale` (no listing) — matches mobile and chat validators.
- `pricing_tokens.product_id` is consistent `products.id` for both channels.
- Shipping config correctly product-scoped (`product_shipping_options`, `farm_address_id`, `preparation_time`).
- One-active-channel-per-product cross-table trigger matches documented exclusivity.
- Order money via server Pricing Token (single snapshot authority).
- Quantity on FPS persisted and guarded (000009, CHECK ≥0, reduced/restored only via Order lifecycle).

## 15. Proven-bad areas

- `order_items.product_id` two id-spaces (`order_creation_service.go:1677` vs `:992`).
- Auction duplicates product title/description/preparation and diverges on edit (no auction-edit product sync).
- Discount 'listing' targets have no id-space validation and collide semantically with saved-items 'listing'.
- Product identity not stable across sale attempts (each create mints a product).
- Product lifecycle mixed/asymmetric (FPS-derived vs auction-explicit) with no withdrawn_at column.
- `products.withdrawn_at` referenced in requirements but absent from schema.

## 16. Owner decisions required

1. **D-I Identity**: Is `products.id` the stable identity of the physical koi across sale attempts (→ relist/auction-after-sale must REUSE the product row), or a per-sale-attempt catalog row (→ rename/re-document Product to "sale item" and stop calling it item authority)?
2. **D-L Lifecycle**: Should `products.status`/`sold_at` exist at all? Options: (a) keep as order/sale-derived projection (both channels must use ONE mechanism), (b) remove from Product and derive availability from channel rows, (c) define an explicit independent item lifecycle.
3. **D-R Relisting/reuse**: Allow reuse of a sold product row for a new sale (with `product.status` re-arm + history), or keep fresh-product-per-attempt as the documented model. Decide enforcement mechanism (DB trigger vs service gate).
4. **D-M Media/metadata divergence**: For auction, either drop its own title/desc/prep columns and read from product (FPS pattern), or accept auction-owned snapshots as the authoritative sale copy and stop syncing product content.

## 17. Recommended next smallest implementation stage

Do not rename anything yet. Smallest converging stage, in this order:

1. **OrderItem id-space fix** (independent, no-naming): always store `products.id` in `order_items.product_id` (FPS writer resolves `listing.ProductID` instead of `listing.ID`), and change FPS stock restore to resolve via `orders.source_id` (or product→active FPS). Add runtime test writing both channels' orders and asserting single id-space. *(No identity-policy dependency.)*
2. **Auction metadata convergence**: stop writing auction-own title/desc/prep at create; read via `product_id` join (mirror `scanJoinedSale`); migrate auction edit to edit product. *(Collapses duplicate authority; no product-identity decision needed.)*
3. **Discount id-space hardening**: validate `applicable_listing_ids` resolve to existing `products.id` (or rename the wire word per D-vocab later); add FK/check or explicit namespace. *(Decouples discount from saved-items collision.)*
4. **Product identity decision → lock** : implement D-I (either reuse or re-document). Every other stage above is safe regardless of D-I outcome except relist policy (D-R) which is gated on D-I.

## 18. NOT PROVEN

- **RUNTIME relisting behavior** (both combinations): no integration test drives "sold FPS → new FPS auction" through real DB; behavior proven only statically.
- **Product reuse reaches any live state**: `products.status='available'` after auction release is written but never consumed (no consumer of that state exists); "returns to availability" is dead in practice — NOT PROVEN either as feature or dead code.
- **`pricing_tokens.product_id` FK** and `shipping_quotes.product_id` FK existence not re-verified at schema (no FK verified this pass; low risk).
- **Seller divergence impossibility**: no test proves product.seller == fps/auction.seller stays true under all edit paths (only FPS syncs).
- **discount 'listing'** ids actually being product ids: only the token-matching direction proves the expectation; no test seeds a discount target and proves it applies.

---

**End of audit. No files were modified. STOP.**