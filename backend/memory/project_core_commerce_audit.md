---
name: PRE_OWNER_TEST_CORE_COMMERCE_REHEARSAL_AUDIT
description: Journey-level audit of core commerce flows before owner test, including identity trace and test results
type: project
---

AUDIT_DATE: 2026-06-22
VERDICT: NOT_SAFE_P0_P1_FOUND (1 P1 blocker — negotiation checkout preview shows wrong price)

## P1 BLOCKER

### NB-1: Negotiation Checkout Preview Shows Listed Price (Not Accepted Price)

**File:** `backend/internal/pricing/token/delivery/http/pricing_token_handler.go` (GeneratePreview handler)

**Root cause:** `GeneratePreviewRequest` struct has no `NegotiationID` field. Handler switch has `case "auction"` and `default` — no `case "negotiation"`. When mobile sends `source_type:'fixed_price_sale'` + `negotiation_id` in preview body, backend ignores `negotiation_id` and generates token at FPS **listed price** (not accepted negotiation price).

**What mobile sends:** `source_type:'fixed_price_sale'`, `source_id:<fps_uuid>`, `negotiation_id:<neg_uuid>`
**What backend uses:** `GenerateForFixedPriceSale(fpsId)` → FPS listed price, negotiation_id silently ignored.
**Order creation:** `order_creation_service.go:1226–1281` DOES override with `negotiation.accepted_price` when `negotiation_id` is provided. Order IS created at correct price.
**Owner test impact:** Checkout review screen shows FPS listed price (incorrect). Order is created at negotiation price (correct). Owner tester sees mismatch → reports broken flow.

**Backend has `GenerateForNegotiation` service (line 834) but it is NOT wired into the HTTP handler.**

**Fix:** Add `NegotiationID *uuid.UUID` to `GeneratePreviewRequest`. In handler `default` branch: if `NegotiationID != nil`, route to `GenerateForNegotiation`. Also add `"negotiation"` to `CreateOrderRequest.SourceType` oneof (or keep FPS source_type but with negotiation override).

Why: **Not a new Phase 2E regression** — pre-existing gap. Phase 2E renamed `listingId→fixedPriceSaleId` but did not change checkout flow.

## BUILD / TEST RESULTS

| Check | Result |
|-------|--------|
| `go build ./...` | CLEAN (0 errors) |
| Backend unit tests | ALL PASS (100+ packages) |
| `pkg/testdb` | FAIL (pre-existing: needs DB_NAME env var, integration only) |
| `flutter analyze lib/` | EXIT 0 (0 errors, 0 warnings) |
| `npx tsc --noEmit` | EXIT 0 |
| Mobile unit tests | 747 PASS, 8 flaky-in-parallel (pass in isolation — pre-existing runner timing issue) |

## COMPILE STUB FIXES (this session)

Three files had stub method names from pre-Phase-2E that didn't implement the renamed interface:
- `cmd/seed/main.go` — `GetByListing/CountByListing/...` → `GetByProduct/CountByProduct/...`
- `internal/integration/payment/application/orchestrator/order_handler.go` — same
- `cmd/corpus_driver/main.go:391` — `d.ListingHandler` → `d.FixedPriceSaleHandler`

## SEARCH CONTENT COUNT QUERY FIX (this session)

`search_repository_impl.go` line ~467 (count query): FIX-3 repost governance still queried `FROM listings l` — fixed to `FROM fixed_price_sales fps WHERE fps.id = ... AND fps.status != 'active'`. (Base query was already fixed in the prior session; only count query was missed.)

## JOURNEY MATRIX

| Journey | Verdict | Identity Trace | Notes |
|---------|---------|----------------|-------|
| Seller FPS create | SAFE | FPS.ID=new UUID, FPS.ProductID=new UUID | Shipping against ProductID ✓ |
| Buyer checkout (FPS direct) | SAFE | source_type=fixed_price_sale, source_id=fpsId | Token at correct price ✓ |
| Pricing preview (FPS) | SAFE | Fixed: handler switch + gin.H lowercase | Session prior-pass fix ✓ |
| Pricing preview (auction) | SAFE | Fixed: routes to GenerateForAuction | Session prior-pass fix ✓ |
| Pricing preview (negotiation) | **P1** | source_type=fixed_price_sale, negotiation_id ignored | Shows listed price, order uses accepted price |
| Shipping quote (FPS) | SAFE | source_type=fixed_price_sale, source_id=fpsId, linked_item_id=fpsId | ✓ |
| Shipping quote checkout (FPS) | SAFE | fixedPriceSaleId=linked_item_id=fpsId → checkout correct | ✓ |
| Shipping quote checkout (auction) | P2 | resolveAuctionListingId returns productId → used as source_id | ProductID≠FPS UUID, order creation would fail |
| Chat attachment (FPS share) | SAFE | fixedPriceSaleId in attachment ✓ | |
| Negotiation proposal messages | SAFE | resource_type/resource_id in backend payload, negotiation_proposal type | |
| Negotiation checkout | **P1** | navigates to /checkout/$fpsId?negotiation_id — shows wrong price | Order created at correct price |
| Auction create | SAFE | auction.ProductID = FK to products.id | |
| Auction bid | SAFE | POST /auctions/:id/bid | |
| Auction claim (bid-win) | SAFE | POST /auctions/:id/claim — generates own token | separate from checkout screen |
| Auction buy-now | P2 | checkout screen: preview OK (source_type=auction), order creation FAILS (source_type hardcoded fixed_price_sale, source_id=auctionId) | checkout flow never fully wired for auction buy-now |
| Promotion purchase | SAFE | target_type=fixed_price_sale|auction|external_product | oneof validation correct |
| Promotion activate | SAFE | TargetTypeFixedPriceSale branch + ExternalProduct branch | |
| Promotion reassign | SAFE | same target_type handling | |
| Promotion moderation restore | SAFE | moderation.fixed_price_sale.restored event wired | |
| Feed governance | SAFE | FIX-3 repost: fixed_price_sales fps, author lifecycle F1-B1 | |
| Search (listings) | SAFE | FROM fixed_price_sales fps JOIN products prod, inline tsvector | Session prior-pass fix |
| Search (content) | SAFE | FIX-3 both base+count fixed | Count fix done this session |
| Search (auctions) | SAFE | LEFT JOIN products prod ON prod.id = a.product_id | Session prior-pass fix |
| Admin moderation | SAFE | entity_type oneof includes fixed_price_sale | |
| Admin seller verify | SAFE | view_url (not document_url), KYC audit complete | |
| Admin promotion campaigns | SAFE | TARGET_TYPE_OPTIONS: fixed_price_sale/auction/external_product | |
| Admin dispute | SAFE | audit logging wired (Pass 23) | |

## IDENTITY TRACE TABLE

| Concept | Go field | DB column | Mobile field | API key | Notes |
|---------|----------|-----------|--------------|---------|-------|
| Product | `Product.ID` | `products.id` | `product.id` | `product_id` | New UUID in products table |
| FixedPriceSale | `FixedPriceSale.ID` | `fixed_price_sales.id` | `fixedPriceSaleId` | `fixed_price_sale_id` / `source_id` when source_type=fps | Separate UUID ≠ ProductID |
| FixedPriceSale.Product FK | `FixedPriceSale.ProductID` | `fixed_price_sales.product_id` | `listing.productId` | `product_id` in FPS response | FK to products.id |
| Auction | `Auction.ID` | `auctions.id` | `auction.id` | `auction_id` / `source_id` when source_type=auction | |
| Auction.Product FK | `Auction.ProductID` | `auctions.product_id` | `auction.productId` | n/a in most responses | FK to products.id |
| Negotiation | `NegotiationSession.ID` | `negotiation_sessions.id` | `negotiation.id` | `negotiation_id` | |
| Negotiation.FPS FK | `NegotiationSession.FixedPriceSaleID` | `negotiation_sessions.fixed_price_sale_id` | `negotiation.fixedPriceSaleId` | `fixed_price_sale_id` | NOT listing_id (Phase 2E-C2 fix) |
| Pricing preview source | `GeneratePreviewRequest.SourceType/SourceID` | n/a | `PreviewOrderParams.sourceType/sourceId` | `source_type` / `source_id` | REQUIRED by backend |
| Order source | `CreateOrderRequest.SourceType/SourceID` | `orders.source_type/source_id` | checkout_repository_impl | `source_type` / `source_id` | fixed_price_sale only via checkout screen |

## P2/P3 DEFERRED RESIDUE

| ID | Description | Impact | Fix path |
|----|-------------|--------|----------|
| P2-1 | Auction buy-now order creation fails (checkout screen hardcodes source_type=fixed_price_sale, source_id=auctionId) | auction buy-now buyers cannot place orders via checkout | Either wire mobile to POST /auctions/:id/claim OR allow auction source_type in checkout_repository_impl |
| P2-2 | Auction shipping quote checkout: resolveAuctionListingId returns productId (used as source_id) | auction shipping quote checkout order would fail | Fix: fetch actual FPS linked to auction's product |
| P2-3 | `AuctionPreview.ListingID` field name stale (carries ProductID UUID, not ListingID) | Semantic confusion, API response field name `listing_id` carries product UUID | Rename to ProductID/product_id (coordinated) |
| P3-1 | Promotion deeplink TODO for force-stop notification | notification tap goes nowhere | Intentional P3 |
| P3-2 | `isFollowingCurrentUser` always false | reverse follow not in search endpoint | P3 |
