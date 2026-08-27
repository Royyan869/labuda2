# COMMERCE PRODUCT IDENTITY & REUSE — STAGE 1 REPORT

## VERDICT

**PRODUCT_IDENTITY_REUSE_PROVEN**

Object scope achieved: Product reuse by FixedPriceSale and by Auction is implemented on the backend; the DB selling-surface authority (the single active-surface invariant, sequential reuse after sold/ended, coexistence rejection) is **runtime-proven against real Postgres**. The FPS reuse path is runtime-proven end-to-end (repo → DB). The Auction `CreateDraft` reuse branch is code-verified + the identical DB constraint behavior runtime-proven; its service-branch unit proof is pending a pre-existing test-package build fix (see §9). Claim #6 ("Product itself does not become sold…") is **NOT PROVEN — intentionally deferred** to the lifecycle/order-settlement stage (fixing it requires touching order/auction product-status writers).

---

## 1. READ-ONLY FINDINGS

Independent re-derivation (file:line):

- **Product is minted per sale attempt today**: `FixedPriceSaleRepositoryImpl.Create` builds+inserts a Product (`fixed_price_sale_repository_impl.go:33-51` → `buildProductFromSale` `:599-626`); `AuctionService.CreateDraft` builds+inserts a Product (`auction_service.go:260-278`). Neither create input carried a `product_id` (`CreateFixedPriceSaleInput` `fixed_price_sale_service.go:107-135`; auction `CreateDraftInput`).
- **Constraints already support reuse**: partial unique indexes `uniq_active_auction_per_product` (`000001:2015`) and `uniq_active_fixed_price_sale_per_product` (`000001:2092`) + cross-table single-active-channel trigger (`000010`). Sold/ended statuses are outside the active lists → sequential reuse is schema-legal today; only the code minted fresh rows.
- **Handlers actively omitted product_id**: FPS create `CreateFixedPriceSaleRequest` (`fixed_price_sale_handler.go:58-89`); auction create `CreateAuctionRequest` (`auction_handler.go:60-88`). Auction docs explicitly said product_id must not be sent (`auction_handler.go` CreateAuction doc) — now updated.
- **Guard still bans legacy `listing_id`/`listingId`** (`fixed_price_sale_handler.go`, `auction_handler.go` listing_id guard) — untouched, correct.
- **Product status writers (claim #6 input)**: FPS-derived `derivedProductStatus` write-through (`fixed_price_sale_repository_impl.go:151-170,639-650`) via `UpdateStock` in order create (`order_creation_service.go:1520`); auction explicit `product.Status="sold"` (`order_creation_service.go:845-853`, `auction_service.go:1026-1035`) and revert on release (`order_completion_service.go:2037-2044`). No production reader consumes `products.status` for availability (verified in prior semantic audit; unchanged).
- **`order_items.product_id` two id-spaces** confirmed again (`order_creation_service.go:1677` FPS→fps.id; `:992` auction→products.id). **NOT touched** — order-domain, Stage 2.
- API/mobile: mobile create DTOs do not carry `product_id`; backend now accepts it → architecturally safe (order/pricing contracts key on `product_id`/`fixed_price_sale_id` where appropriate; adding an optional request key breaks nothing).

## 2. BUSINESS TRUTH VERIFIED

| # | Claim | Result |
|---|---|---|
| 1 | Product is stable identity | **PROVEN (with reuse), qualified**: reuse preserves `products.id` across surfaces; mint-on-create only when no `product_id` supplied (backward-compatible default). |
| 2 | FPS and Auction are selling surfaces over Product | **PROVEN** |
| 3 | Sold/ended surface may be followed by new surface on same Product | **PROVEN (runtime)** — FPS sold → new FPS; FPS sold → ended auction; FPS sold → active auction |
| 4 | Only one active surface per Product | **PROVEN (runtime)** — DB trigger rejects second active/draft surface |
| 5 | Quantity belongs to FPS, not Product | **PROVEN** — untouched; `quantity_available` still FPS-only |
| 6 | Product itself does not become sold merely because a surface sold | **NOT PROVEN — CONTRADICTED by current writers** (`products.status='sold'` derived/explicit); repair deferred to lifecycle/order-settlement stage |

## 3. AUTHORITY MAP

- **Product identity**: created only by FPS/Auction create (mint) OR reused via explicit `product_id`; stored `products.id`; consumers: FPS/Auction FK, pricing_tokens, shipping_quotes, order_items (Stage-2 namespace issues remain). Lifecycle: reuse does NOT write the product row (no clobber on reuse).
- **FPS**: price (`price_per_unit`), quantity (`quantity_available`), status — canonical on the surface. ProductID linkage FK.
- **Auction**: bid/price/timing state — canonical on the surface. ProductID linkage FK; own title/desc snapshot retained (unchanged this stage).
- **Active-surface authority**: single authority = DB cross-table trigger + partial unique indexes. **No duplicate guard added.**

## 4. CHANGES

Production (backend only; mobile untouched):
1. **`backend/internal/commerce/fixedprice/application/fixed_price_sale_service.go`** — `CreateFixedPriceSaleInput.ProductID *uuid.UUID`; `Create()` maps `input.ProductID → listing.ProductID` before persist.
2. **`backend/internal/commerce/fixedprice/infrastructure/repository/fixed_price_sale_repository_impl.go`** — `Create()`: reuse branch (resolve product via `productRepo.GetByID`, seller-ownership check, NO product mint, no product write) vs mint branch (previous behavior); shared `insertFixedPriceSaleRow`.
3. **`backend/internal/commerce/fixedprice/delivery/http/fixed_price_sale_handler.go`** — optional `product_id` on create request; parse + forward; legacy `listing_id` guard message updated (still banned).
4. **`backend/internal/commerce/auction/application/auction_service.go`** — `CreateDraftInput.ProductID *uuid.UUID`; `productReusableGetter` interface; `CreateDraft` reuse-or-mint branch (reuse: GetByID + ownership + skip mint; mint: previous behavior).
5. **`backend/internal/commerce/auction/delivery/http/auction_handler.go`** — optional `product_id` on create request; parse + forward; CreateAuction doc/guard comments updated (listing_id still banned).

Test:
6. **`backend/tests/product_identity_reuse_integration_test.go`** — new real-Postgres runtime proof (package `tests`, `//go:build integration`).

No schema/migration change. No rename. No Listing alias introduced.

## 5. DATABASE / MIGRATION

None required this stage. Existing constraints (`000001` partial unique indexes, `000010` cross-table trigger) already enforce the Model-B invariants; runtime test proves they fire on reuse paths.

## 6. RUNTIME PROOF

`go test -tags integration -count=1 -run 'TestProductIdentityReuse_Stage1_Runtime' ./tests/` → **PASS** (real Postgres, ~120s). Proven scenarios:
1. FPS created with explicit ProductID keeps the exact `products.id` (no new product).
2. Product row count unchanged after reuse (no duplication).
3. Second FPS on a product whose FPS is draft → rejected by DB (trigger).
4. After FPS → sold (SQL transition), the SAME product is reusable by a new FPS.
5. Old sold surface remains historically intact (`product_id` + status preserved).
6. An Ended auction row can attach to the same Product after FPS sold.
7. FPS active + auction active on the same Product → rejected (trigger).
8. After the second FPS sells, an ACTIVE auction on the same Product is allowed.
9. Reuse of another seller's Product → rejected with no row written.

## 7. REGRESSION

- `go build ./...` — clean.
- `gofmt` — clean on all changed files.
- `go vet` (compile-clean packages): `fixedprice/delivery/http`, `auction/delivery/http`, `fixedprice/entity`, `product/...`, `tests` — clean.
- Unit: `go test ./internal/commerce/fixedprice/delivery/http/ ./internal/commerce/auction/delivery/http/ ./internal/commerce/fixedprice/entity/` — ok.
- Runtime: Stage-1 reuse test above — ok.
- Packages that could not be compiled due to **pre-existing baseline test-file drift** (not this stage): see §9.

## 8. RESIDUE

- **Mint-on-create remains default** when `product_id` omitted — intentional (opt-in reuse; backward compatible).
- **`products.status='sold'` writers remain live** (FPS-derived + auction-explicit) — claim #6 contradiction persists; deleting/deriving them requires order_creation/order_completion/auction-settlement coordination → next stage.
- **`order_items.product_id` two id-spaces** — untouched (order-domain, next stage).
- **000010 migration comment** ("creates always mint a brand-new Product per channel") is now descriptive-only and stale for real — left in place (no migration edits this stage).
- **`auction_listing_id_guard_test.go:3`** comment "omits … product_id" — descriptively stale for product_id (guard itself still correctly rejects `listing_id`).
- **No duplicate active-surface guard introduced** — single authority = DB trigger.
- **No Product-as-inventory changes** — quantity remains FPS-scoped.
- **Mobile create DTOs do not transport `product_id` yet** — required only when a "reuse product" seller flow is designed (future stage; backend contract already accepts it).

## 9. UNRELATED PRE-EXISTING FAILURES

Baseline test-package build drifts (none caused by this stage; all reproduce without it):
- `fixedprice/application` — `PublishNow` undefined (`fixed_price_sale_create_sender_address_test.go:188,229`).
- `fixedprice/infrastructure/repository` — `normalizeSaleMedia` undefined (`fixed_price_sale_repository_media_test.go:21`).
- `fixedprice/delivery/http` (integration tag) — `listing.Media`, `shippingRepo.ShippingOptionSummary` undefined (`fixed_price_sale_media_integration_test.go`).
- `auction/application` — `addressRepo`/`Media`/`ErrAuctionFarmAddressNotConfigured` undefined (`auction_sender_address_test.go`, `auction_service_authority_test.go`).
- `backend/tests` selective-migration tests — assume a pre-000031 `comments.fixed_price_sale_id` column (`migration_000031_selective_*.go`); full suite also hit a 600s hang after those failures.
- These blocks prevented running unit/HTTP proof of the auction `CreateDraft` reuse branch and the fixedprice HTTP integration create path — both compiled cleanly (`go build ./...`, handler unit tests pass).

## 10. NEXT STAGE

1. **Product lifecycle columns** (D-3/O-3): decide drop vs single-authority projection for `products.status`/`sold_at`; coordinate removal of the FPS-derived write (`UpdateStock`) and the auction buy-now/claim/settlement writes (`order_creation_service.go`, `auction_service.go`, `order_completion_service.go`) — **order/auction-settlement adjacency, separate decision**.
2. Fix the baseline test-package drifts (§9) so `auction/application` unit + `fixedprice` HTTP integration tests can run; then add a runtime test for the auction `CreateDraft` reuse branch.
3. Optionally add mobile `product_id` transport on FPS/Auction create DTOs when a "reuse product" seller UX is designed.

DO NOT start it in this pass.