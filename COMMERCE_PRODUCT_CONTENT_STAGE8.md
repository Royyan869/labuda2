# COMMERCE_PRODUCT_CONTENT_AUTHORITY_STAGE8_AUDIT

**Date:** 2026-08-25
**Scope:** READ-ONLY. Identity/content authority only.

---

## VERDICT

**Product IS canonical for FPS. Product is NOT canonical for Auction.**

FPS edits propagate to Product via buildProductFromSale -> productRepo.Update.
Auction edits write ONLY to auctions table — zero Auction edit paths call productRepo.Update.

This is deliberate asymmetry. Creates structural contradiction in Model B.

**CRITICAL:** Auction CreateDraft accepts MediaURLs/koi fields in request body. AuctionService.CreateDraft mints Product with these. BUT Auction.UpdateDraft has NO parameters for these fields. Auction detail response NEVER returns koi fields from Product.

---

## 0. ENTITY STRUCTURAL DIFFERENCE

### FPS Entity (fixedprice/entity/fixed_price_sale.go:18-72)
FPS has 14 DUPLICATE fields mirroring Product (Title, Description, MediaURLs, Variety, SizeCm, AgeMonths, Gender, Breeder, Bloodline, Certificates, FarmAddressID, PreparationTime, PreparationNote). Joined Product becomes authoritative via response projection override (fixed_price_sale_response_projection.go:37-49).

### Auction Entity (auction/entity/auction.go:290-335)
Auction has ONLY 4 surface-local fields (title, description, preparation_time, preparation_note). NO MediaURLs, NO koi attributes. These do NOT exist on Auction entity.

---

## 1. TITLE

### Storage
- products.title — writable by FPS Update
- auctions.title — writable by Auction CreateDraft + UpdateDraft + UpdateScheduled

### Producer Trace

#### FPS — Authoritative Producer
fixed_price_sale_repository_impl.go:60-76 (mint): buildProductFromSale copies listing.Title -> product.Title
fixed_price_sale_repository_impl.go:42-56 (reuse): buildProductFromSale OVERWRITES product.Title
fixed_price_sale_repository_impl.go:138-149 (update): buildProductFromSale -> productRepo.Update

FPS Update writes products.title. product_repository_impl.go:114-130 proves it.

#### Auction — Independent Producer
auction_service.go:307-319: auction := entity.NewDraft(sellerID, productID, input.Title, ...) — writes ONLY to auctions.title
auction_service.go:543-548: a.Title = title; auctionRepo.UpdateTx — NO productRepo.Update
auction_service.go:591-596: same — NO productRepo.Update
auction_repository.go:203-241: UPDATE auctions SET title =  — only auctions table

### Consumer Trace
FPS Detail: fixed_price_sale_response_projection.go:37-49 — title = product.Title
Auction Detail: auction_handler.go:1293 — title = a.Title (Auction entity, NOT Product)

### Classification: DUPLICATE AUTHORITY

Scenario: Seller reuses Product P (title="Original") -> creates Auction C (auctions.title="Original")
Seller edits Auction C title to "Premium" -> auctions.title="Premium", products.title="Original"
Result: Auction C displays "Premium". FPS surfaces using P display "Original".

---

## 2. DESCRIPTION

Identical pattern to Title. DUPLICATE AUTHORITY.

Auction UpdateDraft writes auctions.description NOT products.description.

---

## 3. MEDIA_URLS

### Storage
- products.media_urls (jsonb) — FPS authoritative
- fixed_price_sale_media — DEAD (000023 backfill, zero production writers)
- auction_media — DEAD (000023 backfill, zero production writers)

### Producer Trace

#### FPS — Authoritative
fixed_price_sale_repository_impl.go:641: product.MediaURLs = rawMediaURLs(listing.MediaURLs) -> productRepo.Update
Reuse path: OVERWRITES existing Product.MediaURLs with FPS input.

#### Auction — NEVER Producer
auction_service.go:288: product := &productEntity.Product{MediaURLs: input.MediaURLs, ...}
Auction CreateDraft mint DOES create Product with MediaURLs.

BUT:
1. UpdateAuctionRequest (auction_handler.go:288-296) has NO media_urls field
2. Auction UpdateDraft/UpdateScheduled have NO media parameter
3. auctionToResponseWithSeller: NO media_urls in response (auction_handler.go:1258 comment confirms)
4. auction_detail_response_projection.go: no media override from Product

### Consumer Trace
FPS Detail: fixed_price_sale_response_projection.go:215-235 — Product.MediaURLs primary, listing fallback
Auction Detail: NO media. auction_media table is dead. Chat projections read stale snapshot only.

### Classification: DUPLICATE AUTHORITY with DEAD RESIDUE

Auction CreateDraft can store MediaURLs on Product but Auction detail never displays them.

---

## 4. KOI ATTRIBUTES

Fields: Variety, SizeCm, AgeMonths, Gender, Breeder, Bloodline, Certificates

### Producer Trace

#### FPS — Authoritative
FPS Create (mint/reuse/update): buildProductFromSale copies all koi fields -> productRepo.Update

#### Auction — Initial Snapshot Only
auction_service.go:284-303: product := &productEntity.Product{Variety: input.Variety, SizeCm: input.SizeCm, ...}
Auction CreateDraft mint creates Product with koi fields.
Auction CreateDraft reuse: does NOT overwrite Product koi fields.
Auction UpdateDraft/UpdateScheduled: NO koi parameters. Cannot update. Does NOT propagate to Product.

### Consumer Trace
FPS Detail: product overrides (fixed_price_sale_response_projection.go:37-49)
Auction Detail: auction_detail_response_projection.go:38-48 — koi fields from Product

IMPORTANT: Auction detail DOES read koi fields from Product. So if FPS edits Product koi fields AFTER Auction creation, Auction detail will show UPDATED koi fields (even though Auction was never edited).

Scenario:
1. FPS A created, reuses Product P (variety="Kohaku")
2. FPS A edited: variety="Showa" -> product.Variety="Showa"
3. Auction C created, reuses P (koi fields not overwritten on reuse)
4. Auction C detail shows variety="Showa" (from Product) — even though Auction was never edited

### Classification: SNAPSHOT on Auction

FPS is authoritative. Auction reads from Product but cannot edit. Changes to Product via FPS propagate to Auction detail.

---

## 5. PREPARATION_TIME / PREPARATION_NOTE

### Storage
- products.preparation_time, products.preparation_note
- auctions.preparation_time, auctions.preparation_note (WRITTEN but NEVER DISPLAYED)

### Producer Trace

#### FPS — Authoritative
buildProductFromSale -> productRepo.Update (FPS is authoritative)

#### Auction — Zombie Fields
Auction CreateDraft: auctions.preparation_time and auctions.preparation_note WRITTEN to auction entity
Auction UpdateDraft: auctions.preparation_time/preparation_note UPDATED

BUT auction_detail_response_projection.go:46-47:
resp["preparation_time"] = product.PreparationTime (from Product, NOT auction entity)
resp["preparation_note"] = product.PreparationNote (from Product, NOT auction entity)

### Classification: MIXED — Zombie Fields on Auction Entity

auctions.preparation_time and auctions.preparation_note are write-only zombies. Written but never read anywhere. Auction detail always displays from Product.

---

## 6. FARM_ADDRESS_ID

### Producer Trace
FPS: buildProductFromSale -> product.FarmAddressID = listing.FarmAddressID -> productRepo.Update
Auction: auction_service.go:296 — Product minted with FarmAddressID. Auction UpdateDraft has no FarmAddressID parameter.

### Consumer Trace
FPS Detail: product.FarmAddressID shown (fixed_price_sale_response_projection.go:1099)
Auction Detail: NO farm_address_id in response. Auction entity has no FarmAddressID field.

Classification: DUPLICATE with DEAD DISPLAY on Auction.

---

## 7. MEDIA TABLES — PROVEN DEAD

### fixed_price_sale_media
Production Writers: ZERO
Production Readers: ONE — chat_fixedprice_projection_resolver.go:128
Migration Writers: 000023 (one-time backfill from products.media_urls)
Status: DEAD. Chat reads stale snapshot. FPS detail reads from products.media_urls.

### auction_media
Production Writers: ZERO
Production Readers: ONE — chat_auction_projection_resolver.go:170
Migration Writers: 000023
Status: DEAD. Auction surfaces have no media at all.

---

## 8. AUTHORITY MATRIX

| Field | Product Col | FPS Writes | Auction Writes | FPS Read | Auction Read | Classification |
|-------|-----------|-----------|--------------|---------|-------------|---------------|
| title | products.title | YES (mint+update) | YES (mint only) | Product | auction.title | DUPLICATE AUTHORITY |
| description | products.description | YES | YES (mint only) | Product | auctions.description | DUPLICATE AUTHORITY |
| media_urls | products.media_urls | YES | YES (mint only) | Product | NONE | DUPLICATE + DEAD DISPLAY |
| variety | products.variety | YES | YES (mint only) | Product | Product (read) | SNAPSHOT on Auction |
| size_cm | products.size_cm | YES | YES (mint only) | Product | Product | SNAPSHOT on Auction |
| age_months | products.age_months | YES | YES (mint only) | Product | Product | SNAPSHOT on Auction |
| gender | products.gender | YES | YES (mint only) | Product | Product | SNAPSHOT on Auction |
| breeder | products.breeder | YES | YES (mint only) | Product | Product | SNAPSHOT on Auction |
| bloodline | products.bloodline | YES | YES (mint only) | Product | Product | SNAPSHOT on Auction |
| certificates | products.certificates | YES | YES (mint only) | Product | Product | SNAPSHOT on Auction |
| prep_time | products.prep_time | YES | YES (mint only) | Product | Product (zombie auction entity) | MIXED |
| prep_note | products.prep_note | YES | YES (mint only) | Product | Product (zombie auction entity) | MIXED |
| farm_addr | products.farm_addr | YES | YES (mint only) | Product | NONE | DUPLICATE + DEAD DISPLAY |
| fps_media | table | NO | NO | NONE | NONE | DEAD RESIDUE |
| auction_media | table | NO | NO | NONE | NONE | DEAD RESIDUE |

---

## 9. PRODUCT REUSE IDENTITY DRIFT

Scenario: Product P -> FPS A (reuse P) -> edit FPS A -> Auction C (reuse P) -> edit Auction C

After step 2 (FPS A edited): P.title="New Title"
After step 4 (Auction C edited): auctions.title="Auction Title", P.title="New Title" (UNCHANGED)

Result: Auction C shows "Auction Title" (from auctions.title). FPS surfaces using P show "New Title" (from P.title).

Same physical item: TWO DIFFERENT TITLES on two surfaces. DRIFT CONFIRMED.

---

## 10. PROVEN GOOD

1. FPS Update properly propagates all 14 content fields to Product
2. FPS Create (mint) properly creates Product with all fields
3. FPS Create (reuse) overwrites Product fields — predictable
4. Product reuse identity is stable (Product.ID constant across surfaces)
5. Seller ownership check on Product reuse works in both FPS and Auction
6. Auction CreateDraft mint creates Product with all koi/media/prep fields
7. Product entity has no lifecycle/status fields (Stage 3 cleanup complete)
8. No orphan Product creation path
9. Order items correctly use products.id as product_id
10. Shipping correctly uses products.id

---

## 11. PROVEN CONTRADICTION

1. **Title/Description divergence:** FPS edits write to Product. Auction edits write only to auctions. Same item: different titles on FPS vs Auction surfaces.

2. **Koi fields drift via FPS:** FPS edits change Product koi fields -> Auction detail shows UPDATED koi fields even though Auction was never edited.

3. **Zombie fields:** auctions.preparation_time and auctions.preparation_note are written but NEVER read in any Auction response.

4. **Media dead on Auction:** Auction surfaces have no media display. Auction CreateDraft accepts media_urls but Auction has no display path.

5. **FarmAddressID dead on Auction:** Stored on Product by Auction mint but never displayed by Auction detail.

6. **FPS reuse overwrites all Product fields:** Original values from prior creating surface are LOST.

---

## 12. DEAD / ZOMBIE RESIDUE

### DEAD (zero runtime)
- fixed_price_sale_media — zero production writers, one dead reader (chat)
- auction_media — zero production writers, one dead reader (chat)
- products.status, products.sold_at — dropped by 000044
- listing.visibility.apply/restored — parked stubs

### ZOMBIE (unreachable/unread)
- auctions.preparation_time / preparation_note — WRITTEN but NEVER READ
- fixed_price_sales.preparation_time / preparation_note — read-through to Product
- FixedPriceSaleRepositoryImpl.Delete (fixedprice/repository_impl.go:259-266) — writes products.status = withdrawn (col dropped, method never called)

---

## 13. CONTRADICTIONS WITH PRIOR DOCUMENTATION

Stage 7 report claimed FPS edits propagate to Product (CORRECT) and Auction edits do NOT (PROVEN here). The claim that "koi fields read from Product in Auction detail" is CORRECT but the implication (that this is clean) is WRONG — it means FPS edits to koi fields silently update Auction detail display.

---

## 14. DESIGN OPTIONS

**Option A: Product is sole canonical authority (Symmetric)**
- Auction.UpdateDraft calls productRepo.Update for title/description/koi
- All surfaces share same content
- Con: Editing Auction changes Product -> all surfaces sharing Product see change

**Option B: Surfaces are independent (Current Asymmetric)**
- FPS edits Product. Auction edits are surface-local.
- Same item can have different titles on different surfaces
- Accept as architectural decision

**Option C: Product is immutable after creation**
- Neither surface can edit Product fields after creation
- Surface-level title/description become truly surface-local
- Con: Cannot correct typos, update photos after creation

**Option D: Two-phase model**
- Title/description: surface-local
- Koi attributes: Product-local (FPS authoritative, Auction read-only)
- Media: Product-local
- Con: Per-field authority rules add complexity

---

## 15. OWNER DECISIONS REQUIRED

1. **Title/Description authority:** Product canonical (both propagate) OR surface-local (both independent) OR hybrid?

2. **Koi attribute mutability:** FPS is authoritative. Auction cannot edit. FPS edits propagate to Auction detail. Accept or change?

3. **Auction media:** Enable Auction detail to display media from Product, or accept no thumbnails on Auction surfaces?

4. **Zombie fields:** Remove auctions.preparation_time/preparation_note columns, or enable Auction detail to display them instead of Product values?

5. **Dead media tables:** Delete fixed_price_sale_media and auction_media (update chat projections to read products.media_urls), or keep as frozen snapshots?

6. **FPS reuse overwrite scope:** Should FPS reuse overwrite ALL Product fields (current behavior) or only specific fields (preserve koi attributes from creating surface)?

---

## 16. EXACT FILE REFERENCES

### FPS Content Authority
- fixedprice/entity/fixed_price_sale.go:18-72 — 14 duplicate fields
- fixedprice/repository_impl.go:138-149 — buildProductFromSale -> productRepo.Update
- fixedprice/repository_impl.go:632-656 — buildProductFromSale copies all 14 fields
- fixed_price_sale_response_projection.go:37-49 — Product overrides listing fields
- fixed_price_sale_response_projection.go:215-235 — media: Product first, listing fallback
- product_repository_impl.go:114-130 — UPDATE products SET all 14 fields

### Auction Content Authority
- auction/entity/auction.go:290-335 — only 4 surface fields (NO media, NO koi)
- auction/service.go:284-303 — Product minted with ALL fields (media+koi+prep included)
- auction/service.go:543-548 — UpdateDraft: a.Title=a.Description only; auctionRepo.UpdateTx
- auction/service.go:591-596 — UpdateScheduled: same; NO productRepo.Update
- auction/repository.go:203-241 — UPDATE auctions SET title=, description=; NO products update
- auction/handler.go:288-296 — UpdateAuctionRequest: title+description ONLY; no media, no koi
- auction/handler.go:1258 — comment: "Thumbnail not hydrated on this surface"
- auction_detail_response_projection.go:38-48 — koi+prep from Product (NOT from auction entity)

### Media Dead Tables
- migrations/000023_typed_commerce_media_authority.up.sql:39 — backfill fixed_price_sale_media
- migrations/000023_typed_commerce_media_authority.up.sql:62 — backfill auction_media
- chat_fixedprice_projection_resolver.go:128 — ONLY production reader of fixed_price_sale_media
- chat_auction_projection_resolver.go:170 — ONLY production reader of auction_media

### Zombie/Latent Code
- fixedprice/repository_impl.go:259-266 — FixedPriceSaleRepositoryImpl.Delete writes products.status='withdrawn' (latent: col dropped, method never called)
- auction/service.go — auction authority test asserts nonexistent DELETE FROM auction_media (test broken)
