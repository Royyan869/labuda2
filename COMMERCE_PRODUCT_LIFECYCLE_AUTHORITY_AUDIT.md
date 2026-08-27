# COMMERCE_PRODUCT_LIFECYCLE_AUTHORITY_AUDIT

MODE: READ-ONLY. No file modified. Re-derived from current filesystem; no prior report trusted.

> **Correction to earlier passes:** a prior semantic audit claimed `products.status`/`sold_at` had **zero production readers**. That claim is **WRONG**. Fresh tracing proves **live SQL consumers** in the buy-side catalog queries (below). This audit supersedes that statement.

---

## 1. BUSINESS TRUTH TESTED

- Does `products.status` have ONE meaning and ONE authority?
- Is `products.status`/`sold_at` still required under Model B (Product = stable item identity; availability derived from selling surfaces)?

**Answer:** `products.status` has **one meaning** ("channel-availability mirror") but **two authors** (FPS derived-write vs Auction manual-write) with a **stale-state hole** on the auction-reuse path (Stage 1). It is **not canonical**; it is a **DERIVED availability mirror** that is **redundant** for every consumer, because every consumer co-gates on the channel's own status. `sold_at` is **dead** (write-only, round-trip only). Model B makes the field **removable**; evidence is complete enough to justify drop (owner decision D3).

## 2. ALL PRODUCTION WRITERS

### FPS pipeline (single mapping `derivedProductStatus` + `SoldAt`)

| # | Writer (path) | Effect | Triggered by | Evidence |
|---|---|---|---|---|
| 1 | `FixedPriceSaleRepository.Create` (mint) | `product.Status = derivedProductStatus(draft)`, `product.SoldAt` | create listing | `fixed_price_sale_repository_impl.go:654-655` (via `buildProductFromSale` `:599-626`) |
| 2 | `FixedPriceSaleRepository.Update` (edit) | derived write-through | `FixedPriceSaleService.Update` | `fixed_price_sale_repository_impl.go:147`, service `:302-331` |
| 3 | `FixedPriceSaleRepository.UpdateStatus` | derived write-through | Publish, Withdraw, RestoreFromModeration | `fixed_price_sale_repository_impl.go:241-244`; service `:348,397,531` |
| 4 | `FixedPriceSaleRepository.UpdateStock` | derived write-through + `SoldAt` | order create reduce (→sold), order cancel/expire restore | `fixed_price_sale_repository_impl.go:199-202`; `order_creation_service.go:1520`, `order_completion_service.go:2000` |

Mapping: `derivedProductStatus`: `active→available, sold→sold, withdrawn→withdrawn, else draft` (`fixed_price_sale_repository_impl.go:674-683`). This is the SINGLE mapping for the FPS path → FPS maintains a consistent mirror by construction.

### Auction pipeline (MANUAL writes, no shared mapping, INCOMPLETE transition coverage)

| # | Writer (path) | Effect | Triggered by | Evidence |
|---|---|---|---|---|
| 5 | `AuctionService.CreateDraft` (mint only) | `product.Status = "available"` | auction create (new product) | `auction_service.go:275` |
| 6 | `AuctionService.MarkAuctionProductSold` | `product.Status="sold"`, `SoldAt=now` | bid-win claim | `auction_service.go:1051-1061` |
| 7 | `OrderCreationService.CreateFromAuction` (buy-now) | `product.Status="sold"`, `SoldAt=now` | auction buy-now order | `order_creation_service.go:845-853` |
| 8 | `OrderCompletionService.releaseAuctionOrderBinding` | `product.Status="available"`, `SoldAt=nil` (only if was "sold") | auction order cancel/expire | `order_completion_service.go:2037-2044` |

**Not written by auction path**: cancel-without-order, scheduled→active transitions, **and the Stage-1 reuse create** (reuse skips `productRepo.Create`; nothing resets product status → an auction actively selling a reused Product leaves `products.status` at its stale pre-reuse value, e.g. `"sold"`).

**Auction create ALSO hard-codes a raw string `"available"`/`"sold"`** — a second literal vocabulary that can drift from `product_status_enum` (`000001:268`).

## 3. ALL PRODUCTION CONSUMERS

| # | Reader | What it reads | Effect if wrong/removed | Evidence |
|---|---|---|---|---|
| R1 | `FixedPriceSaleRepository.GetPublic` | `WHERE fps.status='active' AND p.status='available'` | buyer FPS catalog visibility | `fixed_price_sale_repository_impl.go:339-342` |
| R2 | `FixedPriceSaleRepository.Search` | `WHERE fps.status='active' AND p.status='available' AND u…` | buyer FPS search | `fixed_price_sale_repository_impl.go:362` |
| R3 | `OrderCompletionService.releaseAuctionOrderBinding` | `if product.Status == "sold"` → revert to available | auction-order release semantics | `order_completion_service.go:2037-2042` |
| R4 | persistence round-trip | `p.status, p.sold_at` read into `listing.Product` then re-derived on next FPS write | none (self-referential) | `joinedSaleSelectColumns` `:486-487`, `scanJoinedSale` |
| R5 | `product_repository_impl.go` | insert/update/scan of the columns | none (storage) | `:70,151,188` |

**Non-consumers (verified, do NOT read `products.status`)**: feed promotion cards (`feed_promotion_injector.go:297-302` filters `fps.status='active'` only), comment/share preview projection (`content_resource_projection_resolver.go:583-598` uses fps status + product media), auction marketplace queries, saved/saved-item hydration, seller dashboard (uses FPS status), mobile DTOs, admin.

## 4. AUTHORITY MAP

| Thing | Authority | Writer(s) | Consumer(s) | Classification |
|---|---|---|---|---|
| `products.id` | Product identity | FPS/Auction create (mint or reuse) | FPS/Auction FK, pricing_tokens, shipping_quotes, order_items | CANONICAL |
| `products.status` | channel-availability mirror | FPS (derived, single map) + Auction (manual, partial incl. stale-on-reuse) | R1, R2 (redundant gates), R3 (release gate) | **DERIVED / DUPLICATE-AUTHOR** → Model-B redundant |
| `products.sold_at` | channel sold timestamp mirror | FPS(UpdateStock), Auction(claim/buy-now) | none (R4 round-trip only) | **DEAD** |
| `fixed_price_sales.status` (+ published/sold/withdrawn_at) | FPS lifecycle | FPS service/repo + OrderService stock | catalog, seller dashboard, feed promo, comment previews | CANONICAL |
| `auctions.status` | auction lifecycle | auction service + order flow | auction marketplace, feed | CANONICAL |
| active-surface exclusivity | cross-channel | DB trigger/indexes (000010/000001) | create/settle paths | CANONICAL |

## 5. LIFECYCLE STATE MATRIX

| Product state | FPS state (canonical) | Auction state (canonical) | Order state driver | Who writes product |
|---|---|---|---|---|
| draft | draft | — | — | FPS #1 |
| available | active | draft/scheduled/active | — | FPS #4 (publish), Auction #5 (create only); Auction release #8 |
| sold | sold | ended + claim/buy-now | order created/paid | FPS #4 (reduce→0), Auction #6/#7 |
| withdrawn | withdrawn | — | — | FPS #3 |

Product has NO independent transition; every `products.status` value is a copy of a channel state, and the ENUM of product states `{draft,available,sold,withdrawn}` (`000001:268`) is just FPS states re-labeled.

## 6. CONTRADICTIONS FOUND

1. **Two authors, one field.** `products.status` is derived by one single mapping on the FPS path but hand-written by the auction path with raw strings and **incomplete transition coverage** (`auction_service.go:275,1051-1061`; `order_creation_service.go:847`; `order_completion_service.go:2038`).
2. **Stale-state hole introduced by Stage-1 reuse.** Auction reuse skips all product writes → an actively selling auction on a reused Product leaves `products.status` e.g. `"sold"` (`auction_service.go` reuse branch has no product-status reset). No live catalog bug today (auction browse ignores product status; exclusivity prevents a simultaneous active FPS), but the field would lie to any future product-status consumer.
3. **Redundant gate (self-contradiction with the FPS filter).** R1/R2 require BOTH `fps.status='active'` AND `p.status='available'`, but `active ⇒ available` is guaranteed by FPS write-through #3/#4. The product gate adds zero filtering power; it just duplicates the channel gate while introducing a second failure mode (a desynced value hides a legitimately active FPS).
4. **`products.sold_at` is write-only.** Only mirror writes; no reader (R4 is storage round-trip). Under Model B it carries no information that `fixed_price_sales.sold_at`/`auctions` settlement timestamps don't already carry.
5. **Documented vs implemented**: `product.go:9-10` says Product is "sale-surface agnostic"; runtime writes tie Product to sale outcomes in 8 places. (Re-stated; now with the reader evidence.)
6. **Auction literal vocabulary drift**: raw `"available"`/`"sold"` strings vs `product_status_enum` type — bypasses the enum's protection.

## 7. FIELD CLASSIFICATION

| Field | Class |
|---|---|
| `products.status` | **DERIVED** (meaning: channel mirror) → **redundant**; drop-candidate in Model B (its one external consumer is itself redundant with `fps.status='active'`) |
| `products.sold_at` | **DEAD** (write-only) |
| `fixed_price_sales.status`, `.published_at`, `.sold_at`, `.withdrawn_at`, `quantity_available` | CANONICAL (channel authority) |
| `auctions.status`, `.current_bid`, `.current_winner_id`, `.settlement_deadline` | CANONICAL |
| active-surface exclusivity (indexes + trigger) | CANONICAL |

## 8. MODEL-B IMPACT (impact proof, not a decision)

Removal of `products.status`/`sold_at` — full impact surface:

- **Schema**: `product_status_enum` type (`000001:268`), `products.status` (`:1338`), `products.sold_at` (`:1339`), index `idx_products_status` (`:2183`). Only migration 000001 touches them (no later migration references) → a single new up/down migration suffices.
- **Queries**: delete `AND p.status='available'` from `GetPublic` (`fixed_price_sale_repository_impl.go:340`) and `Search` (`:362`); drop `p.status, p.sold_at` from `joinedSaleSelectColumns` (`:486-487`) + adjust `scanJoinedSale`; drop the `product.Status=="sold"` gate in `order_completion_service.go:2037` and the product-revert block `:2038-2044` (its purpose — undoing a now-nonexistent field — disappears); remove status/sold_at from `product_repository_impl.go` insert/update/scan (`:70,151,188`) and the entity fields (`product.go`).
- **Code removals**: `derivedProductStatus` (`fixed_price_sale_repository_impl.go:674`), product write-through in FPS `Update/UpdateStock/UpdateStatus` + `buildProductFromSale` status/soldAt (`:147,199-202,241-244,654-655`); auction create `"available"` (`auction_service.go:275`), `MarkAuctionProductSold` (`:1051-1061`), order buy-now write (`order_creation_service.go:845-853`).
- **Tests affected** (enumerated for Stage 3): `auction_settlement_test.go` (product sold/available asserts `:346-348,373,449-473`), `quantity_persistence_test.go` (UpdateStock projection), `shipping_quote_race_condition_test.go` (raw `UPDATE products SET status` `:236`), product repo/entity tests if any.
- **DTO / mobile / admin**: **no impact** — no mobile or admin surface reads `products.status` (verified §3).
- **Runtime verification for Stage 3**: re-run `TestProductIdentityReuse_Stage1_Runtime`, buyer FPS catalog + search integration tests, auction marketplace tests; confirm feed/comment previews unaffected.

## 9. OWNER DECISIONS REQUIRED

**D3 (the only real decision):** 
- Option A (recommended): **Drop `products.status` and `products.sold_at`** (with enum + index). Availability is computed from the active channel — the catalog gates already do this (`fps.status='active'`; auction statuses). Matches Model B exactly; removes the two-author field and the Stage-1 stale-state hole.
- Option B (keep as projection): keep the columns but make **one** author — add full auction write-through (incl. create-reuse, cancel, schedule, claim) so every transition updates product the way FPS does. Larger, permanent coupling to "Product carries sale state", contradicts Model B wording, and reintroduces the redundant gate — not recommended.
- Option C (keep uncontrolled): status quo — silently risks desync (Stages-1 hole) and misleads future consumers. Not recommended.

If Option A is ratified, the redundant catalog gates and the release-gate code become removable (their sole purpose dies with the field).

## 10. RECOMMENDED STAGE 3 (small, not a mega-task)

Ordered, independently shippable:
1. Migration (up+down): drop `products.status`, `products.sold_at`, `idx_products_status`, `product_status_enum`.
2. Remove write-through code: `derivedProductStatus`, FPS repo product projections (Update/UpdateStock/UpdateStatus/`buildProductFromSale`), auction `"available"` create + `MarkAuctionProductSold`, order buy-now write, order-release revert block.
3. Remove the redundant READ gates (`AND p.status='available'`) from `GetPublic`/`Search`; drop `p.status`/`p.sold_at` from joined sale columns + scan.
4. Update the enumerated affected tests; add one runtime test asserting buyer catalog/search visibility is driven solely by channel status.
5. Re-run Stage-1 reuse + full affected integration suites.

Keep: FPS/Auction channel lifecycles, order semantics, pricing/coins/ledger/refund untouched. No rename.

## 11. HARD BOUNDARY RESPECTED

Payment, coins, escrow, ledger, refund, commission, reconciliation, mobile, and naming were NOT touched (read-only pass). Order lifecycle read for writer enumeration only; no change proposed to order behavior this stage — the two order-located product-status writers are removed ONLY inside Stage 3's step 2 together with the field they mirror.

---

**END OF AUDIT. No files modified. STOP.**