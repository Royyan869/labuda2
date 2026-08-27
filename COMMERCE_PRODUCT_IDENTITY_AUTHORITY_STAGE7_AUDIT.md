# COMMERCE_PRODUCT_IDENTITY_AUTHORITY_STAGE7_AUDIT

**Date:** 2026-08-25
**Scope:** READ-ONLY — No code, schema, migration, test, or architecture changes.
**Derivation:** Re-derived from current filesystem. Prior stage reports treated as unverified.

---

## 1. PRODUCT IDENTITY

### 1.1 Entity Definition

`product/entity/product.go:11-29`

```go
type Product struct {
    ID              uuid.UUID
    SellerID        uuid.UUID
    Title           string
    Description     string
    MediaURLs       []string
    Variety         string
    SizeCm          *int
    AgeMonths       *int
    Gender          *string
    Breeder         *string
    Bloodline       *string
    Certificates    []string
    FarmAddressID   *uuid.UUID
    PreparationTime string
    PreparationNote *string
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

**Status:** PROVEN GOOD. No `status`, `sold_at`, `quantity`, `derivedProductStatus` fields exist in the entity. Stage 3 cleanup is complete at the entity level.

### 1.2 When Product Is Created

**Two mint paths exist (both create atomically with their surface):**

**Path A — FPS Mint:**
`fixedprice/infrastructure/repository/fixed_price_sale_repository_impl.go:58-76`
```
Create (FPS repo) → buildProductFromSale(listing) → productRepo.Create → insertFixedPriceSaleRow
```
- Triggered when `listing.ProductID == uuid.Nil` on `repo.Create`
- Product fields sourced from FPS entity fields (`listing.Title`, `listing.Description`, etc.)
- Atomic: both rows created in same transaction

**Path B — Auction Mint:**
`auction/application/auction_service.go:283-303`
```
CreateDraft → productEntity.Product{...Title, Description, ...} → productRepo.Create
```
- Triggered when `input.ProductID == nil` on `CreateDraft`
- Product fields sourced from `CreateDraftInput` (auction fields)
- Atomic: both rows created in same transaction

**Both paths:** `product.ID` is generated as a new UUID. `product.SellerID` = surface seller.

### 1.3 When Product Is Reused

**FPS Reuse:** `fixedprice/infrastructure/repository/fixed_price_sale_repository_impl.go:42-56`
```go
if listing.ProductID != uuid.Nil {
    product, err := r.productRepo.GetByID(ctx, tx, listing.ProductID)
    if err != nil { return err }
    if product.SellerID != listing.SellerID {
        return fmt.Errorf("cannot attach fixed-price sale to product owned by another seller")
    }
    listing.Product = product
    // NO product row written
}
```
**Seller ownership check:** `product.SellerID != listing.SellerID` → fail-closed.

**Auction Reuse:** `auction/application/auction_service.go:270-282`
```go
if input.ProductID != nil {
    lookup, ok := s.productRepo.(productReusableGetter)
    existing, err := lookup.GetByID(ctx, tx, *input.ProductID)
    if existing.SellerID != input.SellerID {
        return fmt.Errorf("cannot create auction on product owned by another seller")
    }
    productID = existing.ID
}
```
**Seller ownership check:** same fail-closed pattern.

**Status:** PROVEN GOOD. Both reuse paths fail-closed on seller mismatch.

### 1.4 Orphan Product Paths

**Path A — FPS mint, then FPS insert fails:**
`fixedprice/repository_impl.go:68-76`:
```go
if err := r.productRepo.Create(ctx, tx, product); err != nil {
    return fmt.Errorf("create product failed: %w", err)  // transaction rolls back
}
listing.ProductID = product.ID
if err := r.insertFixedPriceSaleRow(...); err != nil {
    return err  // transaction rolls back → product also rolled back
}
```
**Atomic:** if `insertFixedPriceSaleRow` fails, the product insert is rolled back by the same transaction.

**Path B — Auction mint, then auction insert fails:**
`auction/application/auction_service.go:300-323`:
```go
if err := s.productRepo.Create(ctx, tx, product); err != nil { return err }
productID = product.ID
if err := s.auctionRepo.CreateTx(...); err != nil { return err }
```
**Atomic:** same transaction rollback pattern.

**Path C — Product minted separately (no surface):**
- `productRepo.Create` is NOT exposed through any HTTP handler directly.
- The only callers are `FixedPriceSaleRepositoryImpl.Create` (mint path) and `AuctionService.CreateDraft` (mint path).
- No code path creates a bare Product without a surface.

**Status:** PROVEN GOOD. No orphan Product creation path exists.

### 1.5 Product Reuse + Surface Transition (Scenario: P → FPS A → sold → FPS B → ended → Auction C)

Trace:

| Step | Product.ID | FPS Seller | FPS Title | Auction Seller | Auction Title |
|------|-----------|-----------|-----------|---------------|--------------|
| FPS A created | P (new) | S | T1 | N/A | N/A |
| FPS A sold | P | S | T1 | N/A | N/A |
| FPS B created (reuse P) | P | S | T1 | N/A | N/A |
| FPS B ended | P | S | T1 | N/A | N/A |
| Auction C created (reuse P) | P | N/A | N/A | S | (A1 = auction's own title) |

**Key findings:**

- **Product identity P is stable** throughout all transitions.
- **Seller identity is stable** (same seller S owns P, FPS A, FPS B, Auction C).
- **Title consistency:** After FPS A is created, if FPS B reuses P, FPS B gets P's current title (T1). FPS B could edit P's title (via `buildProductFromSale` → `productRepo.Update`). Auction C's title is its own, independently editable.
- **Critical inconsistency (see Section 3):** FPS edit writes to `products.title/description`. Auction edit only writes to `auctions.title/description` — it does NOT write to `products`. This means after reuse:
  - FPS B edit changes P's title → FPS surfaces see new title
  - Auction C edit does NOT change P's title → Auction surface keeps A1
  - If same Product is reused for another FPS D after Auction C → FPS D sees P's (possibly FPS-B-edited) title, NOT Auction C's title

### 1.6 Product Identity Stability Throughout Lifecycle

**Product is mutable** (no immutability enforcement). `productRepo.Update` allows updating:
- Title, description, media_urls, variety, size, age, gender, breeder, bloodline, certificates, farm_address_id, preparation_time, preparation_note, seller_id, updated_at

**Surfaces referencing same Product:**

| Surface | Edit effect on Product |
|---------|------------------------|
| FPS Update | `buildProductFromSale` → `productRepo.Update` — **writes Product** |
| Auction UpdateDraft/UpdateScheduled | `auction.UpdateDraft/UpdateScheduled` → `auctionRepo.UpdateTx` — only writes `auctions` table — **DOES NOT write Product** |
| FPS Withdraw | `UpdateStatus` only — no Product write |
| Auction Cancel | `auctionRepo.UpdateTx` — no Product write |

**Status:** PROVEN CONTRADICTION. FPS edit propagates to Product. Auction edit is surface-local. Product identity is stable, but content authority is asymmetric.

---

## 2. SELLER AUTHORITY

### 2.1 Product Owner

**Storage:** `products.seller_id` (NOT NULL, FK → users)
**Set at:** `productRepo.Create` (mint path) or `productRepo.Update` (FPS Update path)

`product_repository_impl.go:56` — Create writes `product.SellerID`
`product_repository_impl.go:133` — Update writes `product.SellerID`

**Source of truth:** The surface-level `SellerID` (FPS or Auction) flows into Product via `buildProductFromSale` (FPS) or `CreateDraftInput` (Auction).

### 2.2 FPS Seller vs Product Seller Consistency

**FPS Create:** `entity.NewFixedPriceSale(sellerID, ...)` sets `listing.SellerID = sellerID`. If mint path: `buildProductFromSale` sets `product.SellerID = listing.SellerID`. If reuse path: ownership check `product.SellerID != listing.SellerID` → reject.

**FPS Update:** `buildProductFromSale(listing)` sets `product.SellerID = listing.SellerID`. Seller can only update their own listing (enforced at handler layer at `fixed_price_sale_handler.go:695-698`).

**Status:** PROVEN GOOD. FPS seller and Product seller are consistent. Fail-closed on mismatch.

### 2.3 Auction Seller vs Product Seller Consistency

**Auction CreateDraft:** `productEntity.Product{SellerID: input.SellerID, ...}` → product created with same seller as auction. Reuse path checks `existing.SellerID != input.SellerID` → reject.

**Auction UpdateDraft/UpdateScheduled:** Does NOT update Product. Seller authority is checked at handler level (`auction_handler.go:537-540`):
```go
if !s.ownership.IsSeller(input.CallerID, auction.SellerID) {
    return auth.ErrSellerRequired
}
```

**Status:** PROVEN GOOD. Initial consistency maintained. Auction edits are surface-local (do not touch Product), which is a different design decision from FPS (FPS edits touch Product).

### 2.4 Reuse by Different Seller

Both FPS and Auction reject reuse when seller doesn't match:
- FPS: `fixed_price_sale_repository_impl.go:47-48`
- Auction: `auction_service.go:279-280`

**Status:** PROVEN GOOD. Fail-closed.

### 2.5 Seller Identity Authority Location

| Operation | Authority Location | Behavior |
|-----------|-------------------|----------|
| Product creation (mint) | Surface (FPS repo / Auction svc) | `product.SellerID = surface.SellerID` |
| FPS update | FPS surface | `product.SellerID` updated via `buildProductFromSale` |
| Auction update | Auction surface | **No Product update** |
| Product reuse | Both surfaces | Seller mismatch → fail-closed |

**Conclusion:** FPS has authority over Product seller. Auction does NOT have authority over Product seller (Auction edits don't touch Product). This is asymmetric but consistent with the FPS-Product coupling vs. Auction's surface-local editing.

### 2.6 Surface Transition Seller Consistency

Scenario: Product P (seller S) → FPS A (seller S) → FPS B (reuse P, seller S) → Auction C (reuse P, seller S).

All surfaces share the same seller S. Product.seller_id = S throughout. No surface can change Product.seller_id to a different seller without owning the surface.

**Status:** PROVEN GOOD for same-seller transitions.

---

## 3. TITLE / DESCRIPTION / PRODUCT CONTENT

### 3.1 FPS Title/Description Authority

**Write path:** `fixedprice/repository_impl.go:138-149`
```go
product := buildProductFromSale(listing)  // copies listing.Title → product.Title
product.ID = listing.ProductID
if err := r.productRepo.Update(ctx, tx, product); err != nil { ... }
```
`buildProductFromSale` at line 632-656:
```go
product.Title = listing.Title
product.Description = listing.Description
product.MediaURLs = rawMediaURLs(listing.MediaURLs)
```

**Producer:** `FixedPriceSaleService.Update` (service layer) → `FixedPriceSaleRepositoryImpl.Update` (repository layer) → `productRepo.Update`.

**Storage:** `products.title`, `products.description`.

**Read path:** `fixedprice/repository_impl.go:scanJoinedSaleFromRow`:
```go
sale.Title = product.Title        // read from p.title
sale.Description = product.Description  // read from p.description
```
`fixed_price_sale_response_projection.go:37-49`:
```go
if product != nil {
    title = product.Title        // Product wins
    description = product.Description
}
```

**Consumer:** FPS detail, FPS search, FPS saved items, FPS chat projection, FPS feed.

**Classification:** FPS is the **canonical author** of `products.title` and `products.description`.

### 3.2 Auction Title/Description Authority

**Write path:** Auction CreateDraft at `auction_service.go:284-303` mints Product from input fields:
```go
product := &productEntity.Product{
    Title:       input.Title,
    Description: input.Description,
    ...
}
s.productRepo.Create(ctx, tx, product)
```

Auction UpdateDraft/UpdateScheduled at `auction_service.go:543-548` and `591-596`:
```go
a.Title = title        // Only writes to auction entity
a.Description = description
// Then: s.auctionRepo.UpdateTx(ctx, tx, auction)
// NO productRepo.Update call
```

`auction_repository.go:203-241` — `UpdateTx` only writes `auctions` table. No `products` UPDATE.

**Producer:** Auction CreateDraft (mint path only — initial write) creates Product. Auction UpdateDraft/UpdateScheduled does NOT propagate to Product.

**Storage:** `auctions.title`, `auctions.description` (owned by Auction). `products.title`, `products.description` (created at mint time, then orphaned from Auction edits).

**Read path:** `auction_handler.go:1293-1294`:
```go
"title":      a.Title,      // from auction entity
"description":a.Description,
```
`auction_detail_response_projection.go:38-48` — koi fields from Product, but title/description from Auction.

**Consumer:** Auction detail, auction search (partial: thumbnail from Product, text on Auction fields).

**Classification:** Auction is the **canonical author** of `auctions.title/description`. `products.title/description` are **initial snapshots** (created at auction creation), not updated on edit.

### 3.3 Critical Contradiction: FPS vs Auction Title/Description Divergence

This is the most significant architectural finding.

**Scenario:** Seller creates Product P → FPS A (title="Koi A") → edit FPS A title to "Premium Koi A" → create Auction C reusing P → edit Auction C title to "Rare Koi A" (Auction edit writes only to `auctions.title`).

Result:
- `products.title` = "Premium Koi A" (from FPS edit)
- `auctions.title` (for Auction C) = "Rare Koi A" (from Auction edit)
- FPS surfaces display "Premium Koi A" (from Product)
- Auction C surface displays "Rare Koi A" (from Auction)

**Same physical item has two different titles on two different selling surfaces.**

The same divergence applies to description.

**Additional divergence vector:** If FPS is created AFTER the auction ended, FPS reads the Product's title (possibly modified by a prior FPS). Auction title remains at its creation value.

**Status:** PROVEN CONTRADICTION. No single canonical author for title/description across surfaces. FPS claims authority; Auction claims surface-local authority and does not propagate edits to Product.

### 3.4 Preparation/Prep-Related Fields

FPS: `PreparationTime` and `PreparationNote` are in both FPS entity (`listing.PreparationTime`) and Product (`product.PreparationTime`). `buildProductFromSale` copies from FPS to Product (`product/preparation_time = listing.PreparationTime`). FPS detail reads from Product (line 47 of projection). FPS Withdraw does NOT update Product (uses `UpdateStatus`).

Auction: `PreparationTime` and `PreparationNote` are in both Auction entity and Product. Auction CreateDraft mints Product with these fields. Auction UpdateDraft/UpdateScheduled does NOT update Product.

**Status:** Same contradiction pattern as title/description.

### 3.5 Other Product Content Fields

`variety`, `size_cm`, `age_months`, `gender`, `breeder`, `bloodline`, `certificates`, `media_urls`:
- FPS Update: `buildProductFromSale` copies all from FPS entity to Product → **FPS is authority**
- Auction CreateDraft: minted from auction input into Product
- Auction UpdateDraft/UpdateScheduled: **DOES NOT update Product**

---

## 4. MEDIA AUTHORITY

### 4.1 `products.media_urls`

**Writer 1 — Product Create:** `product_repository_impl.go:46-72` — `INSERT INTO products (media_urls ...) VALUES (mustMarshalJSON(mediaURLs))`

**Writer 2 — Product Update:** `product_repository_impl.go:113-148` — `UPDATE products SET media_urls = mustMarshalJSON(mediaURLs)`

**Producer:** `buildProductFromSale` (FPS Update path only) at `fixed_price_sale_repository_impl.go:641`:
```go
product.MediaURLs = rawMediaURLs(listing.MediaURLs)
```

Called from `fixedprice/repository_impl.go:138-149` (`repo.Update`).

Auction does NOT write `products.media_urls` on edit. Auction CreateDraft does NOT write `products.media_urls` (the Product is minted without media; `input.MediaURLs` is discarded — `auction_service.go:284-303` creates Product without copying MediaURLs).

**Readers:** 12 files, 17+ SQL references (search, feed, saved items, chat, OG, etc.).

**Classification:** PROVEN GOOD — `products.media_urls` is canonical for FPS media.

### 4.2 `fixed_price_sale_media` — PROVEN DEAD

**Writer:** NONE. Zero production Go writers.
**Reader:** ONE — `chat_fixedprice_projection_resolver.go:128` (SELECT subquery for chat commerce references).
**Migration-only writer:** `000023_typed_commerce_media_authority.up.sql:39` — one-time backfill from `products.media_urls`.

The table was created and backfilled but never written to again. Chat projections read frozen snapshots that will never update.

**Classification:** PROVEN DEAD. Architecture problem — chat projections read stale data from an effectively unmaintained snapshot table.

### 4.3 `auction_media` — PROVEN DEAD

**Writer:** NONE. Zero production Go writers.
**Reader:** ONE — `chat_auction_projection_resolver.go:170` (SELECT subquery for chat commerce references).
**Migration-only writer:** `000023_typed_commerce_media_authority.up.sql:62` — one-time backfill.

**Classification:** PROVEN DEAD. Same as fixed_price_sale_media.

### 4.4 Media in Auction Detail

`auction_handler.go:1291-1295` — `auctionToResponseWithSeller` emits:
```go
"id": a.ID.String(),
"product_id": a.ProductID.String(),
"title": a.Title,
// NO media_urls field
```

The comment at line 1258 explicitly states: `"Thumbnail not hydrated on this surface because the auction entity does not carry listing media."`

Auction surfaces do not display media at all in the detail response.

**Classification:** PROVEN DEAD. No production path reads `auction_media`. Auction detail intentionally omits media.

---

## 5. PRODUCT REUSE + SURFACE TRANSITION

### 5.1 Scenario Trace: P → FPS A → sold → FPS B → ended → Auction C

| State | products table | fixed_price_sales table | auctions table |
|-------|---------------|-------------------------|----------------|
| FPS A created (mint P) | P{id, S, T1, D1, M1} | FPS_A{id=P.id, S, status=sold} | — |
| FPS B created (reuse P) | P unchanged | FPS_B{id=P.id, S, status=ended} | — |
| Auction C created (reuse P) | P unchanged | — | C{id, P.id, S, TA, DA, status=scheduled} |

- **identity P** remains stable ✓
- **seller S** remains consistent ✓
- **title/description:** FPS B reads P's current title (T1). If FPS B was edited after creation, P's title may differ from FPS A's original. Auction C's title = TA (created at Auction C creation time, independent).
- **media:** All surfaces share P's media ✓
- **selling-surface data:** Price/quantity/status on FPS surfaces; bid state on Auction ✓
- **surface state leakage:** None. FPS B and Auction C have their own surface rows. FPS A's state is ended/sold, FPS B's is ended (or whatever). No leakage.
- **surface becomes authority:** FPS surfaces become de-facto authority for Product content. Auction surface does not.

**Key divergence:** After FPS B edit, P's title changes → subsequent FPS surfaces (FPS D, FPS E) will see the new title. But Auction C's title stays at TA regardless of Product changes.

---

## 6. QUANTITY BOUNDARY

### 6.1 Schema Facts

- `products` table: NO `quantity` column. PROVEN DEAD (never existed).
- `products` table: NO `status` column (dropped by `000044`). PROVEN DEAD.
- `products` table: NO `sold_at` column (dropped by `000044`). PROVEN DEAD.
- `products` table: NO `derived_product_status` column. PROVEN DEAD.

### 6.2 Surface-Level Quantity

FPS: `fixed_price_sales.quantity_available` — surface-level stock. `ReduceQuantity` / `RestoreQuantity` only write to `fixed_price_sales` (via `UpdateStock` at line 186-216 of `repository_impl.go`).

Auction: implicit single-unit. No quantity concept.

### 6.3 Consumers Treating Product as Stock Authority

**`CountActiveOrdersByProduct` / `CountAnyOrdersByProduct`** at `order_repository_extensions.go:390-431`:
```go
SELECT COUNT(*) FROM orders o INNER JOIN order_items oi ON o.id = oi.order_id
WHERE oi.product_id = $1 AND o.status IN (...)
```
These correctly treat `order_items.product_id` as Product identity and count all orders (FPS + auction) for a given Product. Used as edit guards on FPS.

**Status:** PROVEN GOOD. These consumers correctly use Product as the identity key for cross-surface order counting. Product is NOT treated as a stock authority — it has no quantity fields.

---

## 7. ORDER / HISTORY IDENTITY

### 7.1 Producer — `order_items.product_id`

`order/entity/order_item.go:10-33`:
```go
type OrderItem struct {
    ID                uuid.UUID
    OrderID           uuid.UUID
    ProductID         uuid.UUID  // products.id — always
    UnitPriceSnapshot money.Money
    Quantity          int
    Name              string
    CreatedAt         time.Time
}
```

**FPS checkout:** `order_creation_service.go:1667-1673`:
```go
orderItem := orderentity.NewOrderItem(
    order.ID,
    listing.ProductID,   // products.id
    listing.PricePerUnit,
    input.Quantity,
    listing.Title,
)
```

**Auction checkout:** `order_creation_service.go:979-985`:
```go
orderItem := orderentity.NewOrderItem(
    order.ID,
    product.ID,        // products.id
    winningBidAmount,
    1,
    product.Title,
)
```

**Both paths:** `order_items.product_id` stores `products.id`. `listing_id` column was dropped by migration `000010`. No surface ID is stored in `product_id`.

**FK constraint:** `000045_order_item_product_identity_convergence.up.sql` adds `REFERENCES products(id) ON DELETE RESTRICT` + `NOT NULL`.

### 7.2 Consumers of `order_items.product_id`

| Consumer | Purpose | Correct? |
|----------|---------|---------|
| `GetOrderItems` (order_repository_extensions.go:306) | Display order item details | ✓ — reads as product identity |
| `CountActiveOrdersByProduct` (order_repository_extensions.go:399) | Block FPS edit when active orders exist | ✓ — product identity for cross-surface guard |
| `CountAnyOrdersByProduct` (order_repository_extensions.go:421) | Block critical FPS field edit | ✓ — product identity |
| Admin order detail (admin_order_handler.go:473) | Display product info | ✓ — reads product_id + name |
| FPS edit guard (fixed_price_sale_handler.go:404) | Block edit with active orders | ✓ |

### 7.3 Selling-Surface Identity in Orders

`order/entity/order.go:28-31`:
```go
SourceType OrderSourceType  // "fixed_price_sale" | "negotiation" | "auction"
SourceID   uuid.UUID       // surface UUID (listing_id / auction_id)
```

**Producer:** `order_repository.go:56-78` — INSERT into `orders` with `source_type` and `source_id`.

**Consumer:** All order history consumers should use `(source_type, source_id)` for surface-level identity and `product_id` for item-level identity.

**Status:** PROVEN GOOD. Product identity and selling-surface identity are properly separated. `order_items.product_id` = canonical Product. `orders.source_type + source_id` = surface identity.

---

## 8. LISTING VOCABULARY

### 8.1 `/listings` Route Prefix

13 production references. All canonical — public API URL. **Classification: A (canonical)**

### 8.2 `fixed_price_sale`

~420 matches across ~80 files. All canonical. Table name, type name, const value, event name, resource type. **Classification: A (canonical)**

### 8.3 `listing` (concept/variable)

~1,353 matches across 134 files. Mixed:
- **A:** In-domain shorthand for `FixedPriceSale` (package aliases, internal variables)
- **B:** API contract for saved items (`target_type = "listing"`)
- **C:** Legacy response fields (discount domain `ListingIDs`, saved item `listingType`, audit/moderation payloads)
- **D:** Dead stubs (`listing.visibility.apply/restored` outbox events never wired)
- **E:** DB column name (`comment_commerce_references.listing_id`)

### 8.4 `fixed_price`

- **A:** `FixedPriceSaleTypeFixedPrice` enum value
- **C:** `listingType = "fixed_price"` in saved item response
- **D:** `for_sale` — zero production matches (dead)

### 8.5 `listing_id` (variable/field)

- **A:** Internal variable holding `FixedPriceSaleID` in commerce domain
- **B:** API field in saved items and discount request/response
- **C:** Legacy field name in audit/moderation/appeal payloads
- Explicit rejection guards at `fixed_price_sale_handler.go:137` and `auction_handler.go:141` prevent old clients from sending `listing_id` for auctions

### 8.6 `target_type` with value `"listing"`

- **B:** `saved_items` path (DB value + API contract) — backward compat needed
- **C:** Feed promotion injector uses `"listing"` but promotion domain uses `"fixed_price_sale"` — inconsistency

### 8.7 `catalog` / `catalog/listing`

Zero matches. Dead.

### 8.8 Highest-Impact Legacy Items for Rename

1. `discount` domain: `ListingIDs` / `applicable_listing_ids` / `listing_id` API fields
2. `feed_promotion_injector.go:491`: `target_type: "listing"` → should be `"fixed_price_sale"`
3. `saved_item` response: `listingType = "fixed_price"` legacy field
4. Moderation/appeal payloads: `listing_id` field names

---

## 9. PRODUCT LIFECYCLE RESIDUE

### 9.1 Schema: products.status — PROVEN DEAD

`000044_product_lifecycle_removal.up.sql`:
```sql
DROP INDEX IF EXISTS idx_products_status;
ALTER TABLE products DROP COLUMN IF EXISTS status;
ALTER TABLE products DROP COLUMN IF EXISTS sold_at;
DROP TYPE IF EXISTS product_status_enum;
```

### 9.2 Code: `products.status` write — LATENT DEAD CODE

`fixed_price_sale_repository_impl.go:259-266`:
```go
if _, err := tx.Exec(ctx, `
    UPDATE products
    SET status = 'withdrawn',
        updated_at = $2
    WHERE id = $1
`, productID, now); err != nil {
    return fmt.Errorf("withdraw product failed: %w", err)
}
```

**Context:** This is inside `FixedPriceSaleRepositoryImpl.Delete` (line 252). This method is **never called in production**.

**Proof of non-use:**
- `DeleteFixedPriceSale` handler (`fixed_price_sale_handler.go:666-719`) calls `listingService.Withdraw()`, NOT `repo.Delete()`
- `Withdraw` → `repo.UpdateStatus()` → only writes `fixed_price_sales` table
- `FixedPriceSaleRepositoryImpl.Delete` is dead code with a latent SQL write to a dropped column

**Severity:** Low (never executed), but exists as a latent bug that would surface if anyone wires up a hard delete path.

**Status:** PROVEN DEAD at runtime. LATENT code in `Delete` method.

### 9.3 Code: `derivedProductStatus`, `MarkAuctionProductSold`, `productUpdater`

- `derivedProductStatus`: NOT found anywhere in production code.
- `MarkAuctionProductSold`: NOT found anywhere in production code.
- `productUpdater`: NOT found anywhere in production code.

**Status:** PROVEN DEAD.

### 9.4 `product_status_enum` Type

Dropped by `000044`. No code references it.

**Status:** PROVEN DEAD.

---

## 10. CONSUMER AUDIT

### 10.1 FPS Detail

**Identity used:** Both — `id` (fixed_price_sale UUID) and `product_id` (product UUID) in response.
**Title source:** `product.Title` (via `scanJoinedSaleFromRow` mapping `p.title` → `sale.Title`, then projection override to `product.Title`)
**Media source:** `product.MediaURLs`
**Seller authority:** Product's seller_id (joined from `products`)

### 10.2 Auction Detail

**Identity used:** Both — `id` (auction UUID) and `product_id` (product UUID) in response.
**Title source:** `auctions.title` (auction entity, NOT product)
**Description source:** `auctions.description` (auction entity, NOT product)
**Media source:** NONE — no `media_urls` in response
**Koi fields:** `product.Variety`, `product.SizeCm`, etc. (from Product)
**Seller authority:** `auctions.seller_id`

### 10.3 Discovery/Search

**Listing search:** `products.title`, `products.description`, `products.variety` — Product is source. Text search on Product fields.
**Auction search:** `auctions.title`, `auctions.description` — Auction is source. Text search on Auction fields.

### 10.4 Feed/Social

**Fixed price repost:** `fps.id` (selling-surface ID). Title/media from Product via JOIN.
**Auction repost:** `auction.id` (selling-surface ID). Title from Auction.

### 10.5 Saved Items

**Saved listings:** `target_id` = fixed_price_sale ID. Title from Product. Media from Product.
**Saved auctions:** `target_id` = auction ID. Title from Auction. Media: NONE (no products JOIN).

### 10.6 Order

**Product identity:** `order_items.product_id` = `products.id` ✓
**Surface identity:** `orders.source_type + source_id` ✓

### 10.7 Shipping

**Product identity:** `product_id` throughout (shipping options, coverage, quotes) ✓

### 10.8 Chat/Comments

**Surface identity:** `(target_type, target_id)` = selling-surface ID ✓
**Display:** FPS → Product via JOIN. Auction → Auction entity.
**Snapshot:** frozen display cache.

### 10.9 Admin

**Primary key:** selling-surface ID (`auction_id`, `source_id`) ✓
**Product:** not directly queried in admin surface ops ✓

### 10.10 Summary: Identity Mixing

| Consumer | Product Identity | Surface Identity | Mixed? |
|----------|-----------------|-------------------|--------|
| FPS detail | ✓ (display fields) | ✓ (`id` field) | Yes — both in response |
| Auction detail | ✓ (`product_id` field) | ✓ (`id` field, display) | Yes — both in response |
| FPS search | ✓ (title/desc) | ✓ (surface ID in results) | Yes |
| Auction search | Partial (thumbnail) | ✓ (title/desc, surface ID) | Yes |
| Saved items | Partial (listings) | ✓ (target_id) | Yes |
| Order | ✓ (`product_id`) | ✓ (`source_type/source_id`) | Yes — but semantically correct |
| Shipping | ✓ (product_id) | ✗ | No mixing |
| Chat | Partial (FPS display) | ✓ (target_id) | Yes |

---

## 11. ORPHAN / DUPLICATE PRODUCT

### 11.1 Create Product Without FPS/Auction

**Not possible.** `productRepo.Create` is not exposed through any HTTP handler. Only called from:
- `FixedPriceSaleRepositoryImpl.Create` (mint path) — atomic with FPS row insert
- `AuctionService.CreateDraft` — atomic with auction row insert

**Status:** PROVEN GOOD.

### 11.2 Create FPS/Auction Without Product

**Not possible.** The create flow always creates or reuses a Product atomically. If product creation fails, the transaction rolls back (both rows rolled back).

**Status:** PROVEN GOOD.

### 11.3 Duplicate Product for Same Semantic Item

**Risk exists through mint path.** If seller creates Product P1 → FPS A → withdraws → creates Product P2 (mint, no reuse) → FPS B. Semantically P1 and P2 represent the same physical item but have different IDs. The system cannot detect this.

**Mitigations:**
- Reuse path is available and has ownership check
- Nothing in the code prevents minting a new Product when an existing one could be reused

**Status:** NEEDS OWNER DECISION. The mint path creates semantic duplicates. Owner must decide if this is acceptable or if reuse should be enforced.

### 11.4 Delete Product While Surface Active

**Not possible through production paths.** No code path deletes a Product. The only delete-adjacent path is `FixedPriceSaleRepositoryImpl.Delete` which is dead code and only writes `products.status = 'withdrawn'` (which would be a no-op since the column was dropped).

### 11.5 Delete Surface Then Create Product Unnecessarily

**Not prevented by code.** If a seller withdraws FPS A (Product P), then creates FPS B without reusing P (mint path creates new Product P2), P becomes an orphan. No background cleanup exists.

**Mitigation:** Reuse path is available. No enforcement.

**Status:** NEEDS OWNER DECISION. Orphan Products can exist after surface withdrawal if seller creates a new surface without reuse.

---

## 12. FINAL AUTHORITY MAP

| Concept | Canonical Authority | Producer | Consumers | Status |
|---------|--------------------|----------|-----------|--------|
| Product identity | `products.id` | FPS mint / Auction mint / reuse | All | PROVEN GOOD |
| Product seller | `products.seller_id` | Surface (via buildProductFromSale / CreateDraft) | All | PROVEN GOOD |
| Product title | `products.title` | **FPS Update only** | FPS surfaces, listing search, saved items | PROVEN CONTRADICTION — Auction edits do NOT update Product |
| Product description | `products.description` | **FPS Update only** | FPS surfaces, listing search, saved items | PROVEN CONTRADICTION — Auction edits do NOT update Product |
| Product media | `products.media_urls` | **FPS Update only** | FPS surfaces, listing search, saved items, chat | PROVEN CONTRADICTION — Auction never writes |
| Product koi fields | `products.{variety,size_cm,...}` | **FPS Update only** | FPS surfaces, auction detail (read-only) | PROVEN CONTRADICTION |
| FPS price | `fixed_price_sales.price_per_unit` | FPS entity | FPS surfaces | PROVEN GOOD |
| FPS quantity | `fixed_price_sales.quantity_available` | FPS entity | FPS surfaces, order creation | PROVEN GOOD |
| FPS status | `fixed_price_sales.status` | FPS entity | FPS surfaces | PROVEN GOOD |
| FPS title | (surface-local alias of Product) | FPS entity | — | PROVEN GOOD — mirrors Product |
| Auction bid state | `auctions.{current_bid,current_winner_id}` | Auction entity | Auction surfaces | PROVEN GOOD |
| Auction title | `auctions.title` | Auction entity (CreateDraft + UpdateDraft/UpdateScheduled) | Auction surfaces | PROVEN GOOD — surface-local |
| Auction description | `auctions.description` | Auction entity | Auction surfaces | PROVEN GOOD — surface-local |
| Auction media | None | None (no media on auction detail) | None | PROVEN GOOD — intentional omission |
| Selling-surface identity | `fixed_price_sales.id` / `auctions.id` | Surface entity | Feed, saved items, chat, moderation | PROVEN GOOD |
| Order product identity | `order_items.product_id` = `products.id` | Order creation | Order surfaces, admin | PROVEN GOOD |
| Order surface identity | `orders.source_type + source_id` | Order creation | Order surfaces, admin | PROVEN GOOD |

---

## 13. FINAL CLASSIFICATION

### Product Identity Authority
- **Product entity has no lifecycle/status fields** → PROVEN GOOD
- **Product created atomically with surface** → PROVEN GOOD
- **Product reuse paths exist and are fail-closed** → PROVEN GOOD
- **No orphan Product creation** → PROVEN GOOD
- **Semantic duplicate Product possible through mint path** → NEEDS OWNER DECISION
- **Orphan Product possible after surface withdrawal** → NEEDS OWNER DECISION

### Seller Authority
- **FPS seller = Product seller at creation** → PROVEN GOOD
- **Auction seller = Product seller at creation** → PROVEN GOOD
- **Reuse blocked for different seller** → PROVEN GOOD
- **FPS update propagates seller to Product** → PROVEN GOOD
- **Auction update does NOT propagate seller to Product** → PROVEN GOOD (by design, but asymmetric)

### Title/Description Content Authority
- **FPS is canonical author for Product.title/description** → PROVEN GOOD
- **Auction is canonical author for auction.title/description** → PROVEN GOOD
- **FPS and Auction edits can produce divergent content for same Product** → PROVEN CONTRADICTION
- **No media on auction detail surface** → PROVEN GOOD (by design, intentional)

### Media Authority
- **`products.media_urls` is canonical** → PROVEN GOOD
- **FPS writes `products.media_urls`** → PROVEN GOOD
- **Auction does NOT write `products.media_urls`** → PROVEN GOOD (by design, consistent with surface-local model)
- **`fixed_price_sale_media` has no production writers** → PROVEN DEAD
- **`auction_media` has no production writers** → PROVEN DEAD
- **Chat projections read stale snapshots from dead tables** → PROVEN DEAD (architectural problem)

### Order Identity
- **`order_items.product_id` stores `products.id`** → PROVEN GOOD
- **`orders.source_type + source_id` stores surface identity** → PROVEN GOOD
- **All consumers use correct identity** → PROVEN GOOD

### Schema Cleanup
- **`products.status` dropped** → PROVEN DEAD
- **`products.sold_at` dropped** → PROVEN DEAD
- **`products.quantity` never existed** → PROVEN DEAD
- **`derivedProductStatus` removed** → PROVEN DEAD
- **`FixedPriceSaleRepositoryImpl.Delete` writes to `products.status`** → PROVEN DEAD (dead code path, never executed)
- **`product_status_enum` type dropped** → PROVEN DEAD

### Listing Vocabulary
- **`/listings` routes** → PROVEN GOOD (canonical)
- **`fixed_price_sale` type/table** → PROVEN GOOD (canonical)
- **`listing` variable/concept** → MIXED (A/B/C/D)
- **`listing_id` API field** → NEEDS OWNER DECISION (B vs C split)
- **`listing.visibility.*` events** → PROVEN DEAD (parked stubs)
- **`for_sale`** → PROVEN DEAD (zero matches)

---

## 14. OWNER DECISIONS REQUIRED

These findings require explicit owner decisions and MUST NOT be resolved by the auditor:

1. **Asymmetric title/description authority (FPS vs Auction):** FPS edits propagate to Product; Auction edits are surface-local. This creates divergent content for reused Products. Owner must decide: should Auction edits propagate to Product (making Auction a co-author), or should FPS edits also be surface-local (making Product strictly immutable)?

2. **Semantic duplicate Products (mint path reuse):** The system allows creating a new Product when an existing one could be reused. Owner must decide: should reuse be enforced (reject mint when seller has existing Product for same item), or is mint-path-dup acceptable?

3. **Orphan Products after withdrawal:** Products can become orphans when their surface is withdrawn and no new surface reuses them. Owner must decide: should orphan Products be cleaned up (soft-deleted or hard-deleted)?

4. **`listing_id` vocabulary:** Still used as API field in saved items and discount domains. Owner must decide: maintain as backward-compat alias (B), or begin deprecation path (C)?

5. **`fixed_price_sale_media` / `auction_media`:** Chat projections read stale snapshots from tables that have no writers. Owner must decide: delete the tables and update chat projections to read from `products.media_urls`, or maintain the frozen snapshot behavior?

---

## 15. TEST / RUNTIME EVIDENCE

### 15.1 Build Verification

Build was not run (build system not available in audit environment). Recommend:
```
cd backend && go build ./...
```

### 15.2 Key Code Evidence (Source-Only Verification)

| Finding | File:Line | Evidence |
|---------|-----------|---------|
| Product entity clean | `product/entity/product.go:11-29` | No status, sold_at, quantity fields |
| FPS repo impl: Product.Update on FPS edit | `fixed_price_sale_repository_impl.go:138-149` | `buildProductFromSale` → `productRepo.Update` |
| FPS repo impl: Auction UpdateTx does NOT update Product | `auction_repository.go:203-241` | Only `UPDATE auctions` — no `UPDATE products` |
| FPS Delete latent write | `fixed_price_sale_repository_impl.go:259-266` | `UPDATE products SET status = 'withdrawn'` |
| FPS Delete never called | `fixed_price_sale_handler.go:666-719` | Calls `Withdraw` not `Delete` |
| Auction mint Product without media | `auction_service.go:284-303` | Creates Product without copying MediaURLs |
| Auction UpdateDraft does NOT update Product | `auction_service.go:543-548` | Only `a.Title = ...` then `auctionRepo.UpdateTx` |
| Auction UpdateScheduled does NOT update Product | `auction_service.go:591-596` | Same pattern |
| Products table: no status/sold_at/quantity | `000044_product_lifecycle_removal.up.sql:18-20` | DROP COLUMN statements |
| Order items product_id = products.id | `order/entity/order_item.go:17` | `ProductID uuid.UUID` |
| Order creation: FPS path | `order_creation_service.go:1667-1673` | `listing.ProductID` |
| Order creation: Auction path | `order_creation_service.go:979-985` | `product.ID` |
| Fixed price sale media: no writers | grep results | Zero production INSERT/UPDATE/DELETE |
| Auction media: no writers | grep results | Zero production INSERT/UPDATE/DELETE |
| Chat projection reads typed media tables | `chat_fixedprice_projection_resolver.go:128` | SELECT subquery |
| Order items: FK to products | `000045_order_item_product_identity_convergence.up.sql` | REFERENCES products(id) |

---

## SUMMARY

Product identity (Model B) is **mostly well-designed but has one critical structural contradiction**: FPS and Auction have asymmetric authority over Product content. FPS edits are canonical and propagate to Product. Auction edits are surface-local and do NOT propagate to Product. This means the same physical item can display different titles/descriptions on different selling surfaces — not because of a bug, but because the design intentionally gives Auction its own content authority.

The media architecture has a dead table problem: `fixed_price_sale_media` and `auction_media` exist as frozen snapshot tables with no production writers and only one production reader each (chat projections reading stale data). `products.media_urls` is the sole canonical media authority.

The product lifecycle cleanup is complete: no `status`, `sold_at`, or `quantity` fields remain in the Product entity or the database schema. The one latent residue (`FixedPriceSaleRepositoryImpl.Delete` writing `products.status`) is dead code that is never executed.

Order identity is clean: `order_items.product_id` correctly stores `products.id` with a FK constraint. Selling-surface identity is correctly stored in `orders.source_type + source_id`.

The most important architectural question for the owner: should Auction surface-local editing be treated as a deliberate design choice (with the divergence it creates), or should it be changed to make Product the canonical author for all content fields, requiring Auction edits to propagate to Product?
