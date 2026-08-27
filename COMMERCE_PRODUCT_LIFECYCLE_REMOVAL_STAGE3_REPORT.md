# COMMERCE_PRODUCT_LIFECYCLE_REMOVAL_STAGE3_REPORT

## VERDICT SUMMARY

Owner ratified **Option A**: remove `products.status` and `products.sold_at` (with `idx_products_status` and `product_status_enum`). Product carries no selling lifecycle; availability is derived from the active selling surface (`fixed_price_sales.status` / `auctions.status`). Stage opened by fully re-verifying every audit reference against the current source (post Stage-1 line drift).

---

## 1. EXACT FILES CHANGED

### Production code
1. `backend/internal/commerce/product/entity/product.go` — removed fields `Status string`, `SoldAt *time.Time`.
2. `backend/internal/commerce/product/infrastructure/repository/product_repository_impl.go` — removed `status`/`sold_at` from INSERT, UPDATE, SELECT, and `scanProductRow`.
3. `backend/internal/commerce/fixedprice/infrastructure/repository/fixed_price_sale_repository_impl.go` — removed:
   - `derivedProductStatus` function;
   - the Product mirror write-through in `UpdateStock` and `UpdateStatus` (product `Status`/`SoldAt` projection updates);
   - `product.Status`/`product.SoldAt` writes in `buildProductFromSale`;
   - `p.status, p.sold_at` from `joinedSaleSelectColumns` + the two scan destinations in `scanJoinedSaleFromRow`;
   - `AND p.status = 'available'` from `GetPublic` and `Search` (retained `fps.status = 'active'` + seller governance).
4. `backend/internal/commerce/auction/application/auction_service.go` — removed:
   - `Status: "available"` from the minted Product;
   - `MarkAuctionProductSold` method and the `productUpdater` interface.
5. `backend/internal/commerce/auction/delivery/http/auction_handler.go` — removed the `MarkAuctionProductSold` call in the bid-win claim step.
6. `backend/internal/commerce/order/application/order_creation_service.go` — removed the auction buy-now Product `sold` mirror block (order pricing/auction semantics untouched).
7. `backend/internal/commerce/order/application/order_completion_service.go` — removed `releaseAuctionOrderBinding`'s Product revert block, the now-unused `productRepo` struct field, its constructor assignment, and its import (`productRepoImpl`); order/auction release semantics otherwise untouched.

### Migration
8. `backend/migrations/000044_product_lifecycle_removal.up.sql`
9. `backend/migrations/000044_product_lifecycle_removal.down.sql`
10. `backend/migrations/README.md` — one-line entry for 000044.

### Tests / fixtures encoding the removed model
11. `backend/tests/product_identity_reuse_integration_test.go` — seed no longer inserts `products.status`.
12. `backend/tests/product_lifecycle_removal_integration_test.go` — NEW: migration up/down/replay + buyer catalog/search + auction marketplace runtime proofs.
13. `backend/internal/commerce/order/tests/auction_settlement_test.go` — removed Product mirror seed/write/assertions; two tests renamed (`TestAuctionOrderCancel_ReleasesBinding`, `TestAuctionOrderExpire_ReleasesBinding`).
14. `backend/internal/interaction/chat/application/chat_read_plumbing_3d1_proof_test.go` — 4 product seeds dropped the `status` column.
15. `backend/internal/social/feed/infrastructure/repository/feed_repository_test.go` — product seed dropped `status`.
16. `backend/internal/commerce/negotiation/infrastructure/repository/negotiation_repository_integration_test.go` — product seed dropped `status`.
17. `backend/internal/social/content/delivery/http/comment_query_count_test.go` — product seed dropped `status`.
18. `backend/internal/serverboot/` projections: `chat_auction_projection_resolver_integration_test.go`, `chat_content_projection_resolver_integration_test.go`, `chat_fixedprice_projection_resolver_integration_test.go`, `chat_resource_projection_aggregate_resolver_integration_test.go` — product seeds dropped `status`.
19. `backend/internal/commerce/shipping/quote/infrastructure/repository/shipping_quote_race_condition_test.go` — removed the raw `UPDATE products SET status='available'` mirror write.
20. DELETED `backend/internal/commerce/auction/application/auction_product_sold_update_test.go` — tested only the removed `MarkAuctionProductSold` mirror.

## 2. EXACT SYMBOLS REMOVED

- `Product.Status`, `Product.SoldAt` (entity + repo persistence + scan).
- `derivedProductStatus`.
- FPS repo mirror writes inside `Update`, `UpdateStock`, `UpdateStatus`, `buildProductFromSale`.
- Auction `MarkAuctionProductSold`, `productUpdater`, mint `Status: "available"`, claim-step mirror call.
- Order buy-now product `sold` write; order-release product revert block + `productRepo` dep on `OrderCompletionService`.
- DB: `products.status`, `products.sold_at`, `idx_products_status`, `product_status_enum`.
- `AND p.status = 'available'` predicates (X2: `GetPublic`, `Search`).

## 3. MIGRATION EVIDENCE (real Postgres)

`TestMigration000044_UpDownReplay` — **PASS**:
- After up: `products.status`, `products.sold_at`, `idx_products_status`, `product_status_enum` all absent (information_schema / pg_type / pg_indexes).
- down: all four restored.
- Replay up: all four removed again.

Migration files use `IF EXISTS` / `IF NOT EXISTS` guards so up/down are re-runnable.

## 4. RUNTIME PROOF (real Postgres)

`go test -tags integration -count=1 -run 'TestMigration000044_UpDownReplay|TestFpsCatalog_SurvivesProductLifecycleRemoval|TestProductIdentityReuse_Stage1_Runtime' ./tests/` → **PASS (ok)**:
- `TestFpsCatalog_SurvivesProductLifecycleRemoval` — buyer FPS catalog (`GetPublic`) and search (`Search`) return the ACTIVE FPS and exclude the withdrawn FPS, driven solely by `fps.status = 'active'`; an active Auction on its own product remains visible in `AuctionRepository.List` (auction marketplace never depended on product status).
- `TestProductIdentityReuse_Stage1_Runtime` — the Stage‑1 reuse scenario (sold FPS → reuse SAME product → new FPS; → active Auction) still passes **without any Product status reset**: Product reuse no longer depends on resetting a non-existent field (requirement #“sold Product → reuse → new active Auction” verified).
- `TestMigration000044_UpDownReplay` — see §3.

## 5. REGRESSION

- `go build ./...` — clean.
- `gofmt` — all changed files formatted; `gofmt -l` on the touched dirs shows only pre-existing unformatted files (unchanged).
- `go vet ./tests ./internal/commerce/product/... ./internal/commerce/fixedprice/delivery/http/ ./internal/commerce/auction/delivery/http/ ./internal/commerce/order/application/` — clean.
- Unit: `go test ./internal/commerce/fixedprice/delivery/http/ ./internal/commerce/auction/delivery/http/ ./internal/commerce/fixedprice/entity/ ./internal/commerce/product/...` — ok.
- Integration (order domain): `go test -tags integration -run 'TestAuction' ./internal/commerce/order/tests/` — ok (auction settlement/release suite passes after mirror removal).
- Stage tests above — ok.

## 6. REPOSITORY-WIDE RESIDUE SEARCH

Post-implementation grep across backend:
- `derivedProductStatus`, `MarkAuctionProductSold`, `productUpdater`, `Product.Status`, `product.Status`, `product.SoldAt`, `Status: "available"` — **ZERO production/test references** except the deliberate assertion strings inside `product_lifecycle_removal_integration_test.go` and migration SQL/README.
- `insert into products (` — all 24 remaining seeds now exclude `status`/`sold_at` (verified column lists).
- `UPDATE products SET status` — zero.
- `products.status` readers — zero (the two catalog predicates removed).
- `product_status_enum` / `idx_products_status` — only the new migration files + README entry + the migration test.

No Product lifecycle field is read or written anywhere in production code.

## 7. PRE-EXISTING FAILURES (NOT from this stage, NOT fixed)

- `internal/commerce/auction/application` test package build drift (baseline): `addressRepo` / `Media` / `ErrAuctionFarmAddressNotConfigured` in `auction_sender_address_test.go`, `auction_service_authority_test.go`.
- `internal/commerce/auction/infrastructure/repository` test build drift (baseline): `auction_repository_media_test.go` references `Auction.Media`.
- `internal/commerce/fixedprice/application` `PublishNow` drift; `internal/commerce/fixedprice/infrastructure/repository` `normalizeSaleMedia` drift; fixedprice http integration `listing.Media`/`ShippingOptionSummary` drift (all pre-existing).
- `backend/tests` selective-migration tests assume a pre-000031 schema state; full `./tests/` suite also hangs after those failures (pre-existing; the focused stage tests run clean in isolation).
- (Intentional) deletion of `auction_product_sold_update_test.go` — a test of the removed mirror, not a failure.

## 8. HARD BOUNDARIES RESPECTED

Payment, coins, escrow, ledger, refund, commission, reconciliation, pricing formulas, FPS lifecycle, Auction lifecycle, naming, and mobile were NOT modified. Order files changed only where they wrote/reverted the removed Product mirror (no order semantics altered).

STOP after Stage 3. No next Commerce stage started.