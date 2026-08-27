# COMMERCE_PRODUCT_MODEL_B_VALIDATION_AUDIT

MODE: READ-ONLY. No file modified. All claims re-derived from current filesystem.

**Model B under test:** `Product` = stable sellable item/inventory definition; `FixedPriceSale` and `Auction` = selling mechanisms/surfaces attached to Product; Product identity stable across surface transitions and relisting.

---

## 1. EXECUTIVE VERDICT

**VERDICT: `MODEL_B_REQUIRES_OWNER_DECISION`**

Model B is structurally coherent with the current codebase — **no business contradiction that invalidates it was found**. The database layer already anticipates product-sharing: the per-table partial unique indexes (`000001:2015,2092`) and the cross-table single-active-channel trigger (`000010`) are product_id-scoped, so sequential FPS→Auction→FPS on one `products.id` is schema-legal and becomes the *enforced* behavior as soon as create flows reuse the product row. Quantity>1 is coherent as a per-selling-surface count of a product definition, matching the 000009 owner decision and auction single-unit semantics.

Model B is **NOT fully validated** because it requires five explicit decisions every other component depends on, and current behavior contradicts the proposed model in exactly the places those decisions must land:

1. **Quantity authority** — today `quantity_available` lives on `fixed_price_sales` only; Model B's word "inventory" implies product-level stock, which does not exist and would require a new ledger to survive channel transitions. (Decide: per-channel quantity = keep current; product-level inventory = new machinery.)
2. **Relisting policy** — documents forbid reusing a sold Product (`commerce-selling-doctrine.md:67`); Model B requires reuse + new surface rows + channel history. The doctrine must be overturned, not implemented against.
3. **Product lifecycle** — `products.status`/`sold_at` exist today (FPS-derived + auction-explicit writes) but have **no production read consumer** (verified); under Model B availability must derive from surfaces, so these columns become removable or read-only projections.
4. **Seller identity** — three unconstrained `seller_id` copies; Model B requires one authority (Product) with enforcement (constraint/trigger), because today auction never re-syncs product.
5. **Auction metadata** — auction's own `title/description/preparation` are defensible as an **immutable auction snapshot** under Model B, but the FPS write-through-syncs-product path is the opposite policy; one snapshot policy must be chosen.

---

## 2. Validation of each proposed claim

### 2.1 "Product = stable sellable item/inventory definition"
**Status: UNKNOWN / OWNER DECISION.** 
- All koi/catalog facts (title, description, media, variety, size, age, gender, breeder, bloodline, certificates, farm_address, preparation) are **already entirely on `products`** (`000001:1322-1342`; `product.go`), and the FPS row stores none of them (joined reads, `joinedSaleByIDQuery`/`scanJoinedSale`). So Product is already the *item-definition* table.
- But **identity is not stable**: every FPS create mints a new product (`fixed_price_sale_repository_impl.go:38-51`), every auction create mints a new product (`auction_service.go:260-278`), and **no create input accepts a `product_id`** (`CreateFixedPriceSaleInput`, `fixed_price_sale_service.go:107-135`; auction create has no product input). Stability is the exact gap; the rest of the definition fits.
- The word "inventory" is the unstable part: see 2.2.

### 2.2 "Does quantity > 1 have coherent business meaning under Model B?"
**Status: PROVEN COMPATIBLE (per-channel quantity) / UNKNOWN (product-level inventory).**
- Only `fixed_price_sales.quantity_available` exists (`000009`); no product-level stock/ledger table exists anywhere (verified: no `inventory`/`stock_ledger`/`units_remaining` construct in commerce code). Defaults `1` = unique-item mode (`fixed_price_sale_handler.go:62-65,229-232`), `min=1` (`:65`).
- `quantity=10` today = **10 fungible sellable units of one Product definition**, offered by that FPS surface until exhaustion (`ReduceQuantity`→sold at 0, `fixed_price_sale.go:235-264`); each order consumes `quantity` units (`order_items.quantity`).
- Under Model B this is coherent **as a per-selling-surface count**: Product defines WHAT, the surface defines offerable units. The 000009 owner decision (multi-quantity) and auction single-unit are both compatible with B.
- The alternative reading ("inventory on Product") has **no implementation and no transition bookkeeping**: nothing tracks units consumed by a former FPS, then by an auction, etc. Adopting product-level stock = a new inventory ledger requirement. This is the decision point; B works with per-channel quantity.

### 2.3 "Can FPS and Auction safely attach to the same Product?"
**Status: PROVEN COMPATIBLE (constraint layer), code layer requires change.**
- Constraints are product_id-scoped and already anticipate sharing:
  - `uniq_active_fixed_price_sale_per_product` (product_id, WHERE status IN draft,active) — `000001:2092`.
  - `uniq_active_auction_per_product` (product_id, WHERE status IN draft,scheduled,active,waiting_settlement) — `000001:2015`.
  - Cross-table trigger: an FPS draft/active blocks auction draft/scheduled/active/waiting_settlement and vice versa (`000010`).
- A **sold FPS or ended/cancelled auction does NOT block** a new channel on the same product → sequential reuse is schema-legal. The trigger even says creates "currently always mint a brand-new Product per channel" — i.e., the sharing path exists but has zero callers today.
- Code change required: create flows must accept/reuse `product_id` (currently absent on both inputs).

### 2.4 "One Product → one active selling surface remains a sensible invariant?"
**Status: PROVEN COMPATIBLE.** The exclusivity invariant is the documented design (`commerce-selling-doctrine.md:64-68`; `commerce-db-model-split-design.md:417-423`) and matches the cross-table DB enforcement. Model B keeps it; sequential channels over one product are exactly the intended pattern. No contradiction.

### 2.5 "Relisting should reuse Product and create a new selling-surface record?"
**Status: PROVEN COMPATIBLE (mechanically) / requires policy overturn (doctrinally).**
- Schema permits it (2.3). Order history can link because `orders.source_type/source_id` pin the exact channel (`order_creation_service.go:1615` FPS, auction path) — a new relist row is independently traceable.
- **Blocking doc**: "A sold Product MUST NOT be reused for a new sale" (`commerce-selling-doctrine.md:67`; `commerce-db-model-split-design.md:420`) — unenforced today but in direct conflict with Model B's relisting. The doc invariant must be explicitly overturned by the owner before B is canonical.
- Code reality: correctly **no relist path exists** (withdrawn/sold are terminal for sellers; only moderation `MarkActiveFromModeration` reopens, `fixed_price_sale.go:186-206`).

### 2.6 "FPS → Auction and Auction → FPS transitions make business sense without cloning Product?"
**Status: PROVEN COMPATIBLE.** Reuse yields clean semantics: FPS sold/withdrawn → same product enters auction (active-channel list no longer includes sold/withdrawn FPS); auction ended-unsold → product re-available (existing restore path `order_completion_service.go:2037-2044`) and can re-enter FPS. The trigger correctly prevents *simultaneous* channels. No clone needed. The only nuance: **units across transitions** — if the product had qty>1 sold via FPS then relisted as auction, no product-level ledger exists; with per-channel quantity this is a non-issue (each channel defines its own quantity); with product-level quantity it becomes a Model-B feature to build.

### 2.7 "Order history should reference products.id plus the selling-surface identity?"
**Status: PROVEN COMPATIBLE (with one required bug fix).** 
- `orders.source_type/source_id` already carry the surface identity; `order_items.product_id` is supposed to be the product (doc `order_item.go:11-18`) but **currently stores `fixed_price_sales.id` on FPS orders** (`order_creation_service.go:1677`) and `products.id` on auction orders (`:992`) — two id-spaces re-verified, no FK (`000001:1024-1033`; index only `:2134`). Under Model B this column must be `products.id` + FK, with the FPS writer fixed to write `listing.ProductID`. Compatible once fixed.

### 2.8 "Shipping configuration belongs to Product?"
**Status: PROVEN COMPATIBLE.** Already product-scoped: `product_shipping_options` (`000001:1315-1320`), `products.farm_address_id`, `products.preparation_time`; buyer availability is keyed by `product_id` (`/shipping/options`, `routes_core.go:359-364`; `shipping_quote` validation compares `quote.ProductID` to order product, `order_creation_service.go:529-553`). Shipping on Product is implemented-and-correct today; Model B preserves it.

### 2.9 "Seller identity belongs to Product or surface; identify every duplicate?"
**Status: UNKNOWN / OWNER DECISION (placement) — duplicates enumerated.**
- Three copies: `products.seller_id`, `fixed_price_sales.seller_id`, `auctions.seller_id` (all `NOT NULL`, `000001:1324,871,479`).
- FPS keeps product synced (`buildProductFromSale:605` `product.SellerID = listing.SellerID` on create+update).
- Auction never writes product after create → an auction-edit ownership divergence is at least structurally possible; no FK/check ties the three columns.
- Readers: surface rows carry seller for checks/handlers; `products.seller_id` read via joins. Model B implies Product is the seller authority and channels inherit — requires a constraint (or dropping channel seller columns and deriving). Decision needed; current is "single producer, unconstrained copies".

### 2.10 "Auction metadata duplicated from Product — canonicalize onto Product or keep auction snapshot?"
**Status: PROVEN COMPATIBLE as SNAPSHOT (decision: snapshot policy).**
- Auction stores its own `title, description, preparation_time, preparation_note` (`000001:483-486`) AND `product_id`. This is *defensible* as an **immutable snapshot** of the auctioned item (mirrors order_items snapshot philosophy `000001:1024-1033`; `order_canonical_test`).
- **Asymmetry problem**: FPS edits write through to Product (`buildProductFromSale`), auction edits do NOT sync Product. So the snapshot policy is inconsistent between channels. Model B should pick one: (a) auction-owned auction facts = frozen snapshot, FPS edits also freeze/append, or (b) both write through to Product. Decision required, not a blocked contradiction.

### 2.11 "Any current Product field becomes invalid or ambiguous under Model B?"
**Status: PARTIAL — `status`/`sold_at` become redundant; `withdrawn_at` already absent.**
- `products.status` (enum draft/available/sold/withdrawn, `000001:1338`) + `products.sold_at`. Under Model B, availability derives from surfaces. Verified: **no production reader consumes `products.status`** for availability decisions (write path only: `product_repository_impl.go:70,151,188` persist/scan; `derivedProductStatus` `fixed_price_sale_repository_impl.go:639-650`; auction explicit writes `auction_service.go:1026`, `order_creation_service.go:847`; restore `order_completion_service.go:2037-2044`). FPS/auction/saved/search/feed all filter on the ±surface status, not product status.
- `products.withdrawn_at` does not exist while `derivedProductStatus` emits `'withdrawn'` — the derived value has no timestamp authority. Under B this column set should be removed or converted to a read-only projection.
- All other Product columns (koi facts, media, farm/prep) remain canonical definition data — valid under B.

### 2.12 "Should Product lifecycle exist independently, or availability derive from surfaces?"
**Status: PROVEN COMPATIBLE with derive-from-surfaces (the current consumers demand it).**
- No workflow consults a standalone Product lifecycle; every availability check is surface-driven (`FixedPriceSale.IsAvailable()` = channel status+qty, `fixed_price_sale.go:300-302`; auction status machine). The `products.status` writes are projections with zero readers. Under Model B: remove columns (or make them generated/projection-only) and let surface states drive availability. This is a decision to delete/modify columns, but the *behavior* is already "derive from surfaces".

### 2.13 "Do current DB constraints/triggers support Model B or need change?"
**Status: PROVEN COMPATIBLE — constraints support B; additions required.**
- Keep: partial unique indexes + cross-table single-active-channel trigger. They already scope by `product_id` and permit sequential reuse (`2.3`).
- Add/change for B:
  1. Enforce single seller authority (FK/trigger so `fixed_price_sales.seller_id = auctions.seller_id = products.seller_id`), or drop channel seller columns.
  2. `order_items.product_id` → FK to `products(id)`.
  3. Decide `products.status/sold_at`: drop (B-clean) or keep as maintained projection (needs a trigger to update on any channel-status change, including auctions from moderation/restore — today only order flows touch it).
  4. Optional: unique index allowing multiple **active**? No — reuse is sequential (one active channel), so existing partial indexes already suffice.

### 2.14 "Every place current code assumes Product is per-sale-attempt"
Enumerated (all create-time minting, no reuse):
1. `FixedPriceSaleRepositoryImpl.Create` builds product from sale and persists both — `fixed_price_sale_repository_impl.go:33-51`.
2. `AuctionService.CreateDraft` builds product and persists — `auction_service.go:258-278`.
3. `CreateFixedPriceSaleInput` has no `ProductID` — `fixed_price_sale_service.go:107-135`.
4. Auction create has no product input (product minted from auction fields) — `auction_service.go:260-278`.
5. `buildProductFromSale` always overwrites the whole product row from the listing (single-row ownership assumption) — `fixed_price_sale_repository_impl.go:599-626`.
6. Migration 000010 comment: "Both create flows currently always mint a brand-new Product per channel" — explicit owner acknowledgement.
7. Order write casts `product_id` to the channel id for FPS (`order_creation_service.go:1677`) — the most harmful per-sale-attempt coupling.
8. No seller/product inventory list exists anywhere (marketplace surfaces are channel-first), consistent with per-attempt products.

### 2.15 "Migrations/schema changes required to canonicalize Model B"
Structural list (no data changes needed — from-zero):
1. (code) Create inputs accept an optional existing `product_id`; find-or-create by definition key or explicit selection; mint-on-create only when no Product exists.
2. (schema) New FK `order_items.product_id → products(id)` (+ fix the FPS writer to store `products.id`).
3. (schema) Seller equality enforcement between the three seller columns (FK from fixed_price_sales/auctions seller to products.seller or a trigger/CHECK).
4. (schema) `products.status`/`sold_at`: choose drop vs derived; if dropped, remove `derivedProductStatus` write-through (FPS path) and auction explicit writes; availability comes from surface status. If kept, add a trigger covering all channel transitions.
5. (schema) Optional: drop `auctions.title/description/preparation*/...` if the owner picks write-through instead of snapshot (2.10).
6. (schema) Optional (only if product-level inventory is chosen): add a product stock/ledger concept and channel-consumption bookkeeping — this is new machinery, not a rename.
7. Doc reconciliation: overturn "sold Product MUST NOT be reused" (`commerce-selling-doctrine.md:67`) to "relist = reuse Product + new surface row with channel history"; reconcile multi-stock doc wording with 000009; drop PRD `RESERVED` FPS-state wording or implement it.

---

## 3. Aggressive challenge of Model B

Challenges raised, tested against current evidence:

1. **"Inventory" naming collides with the documented "sale-intent-first, not inventory-first" UX doctrine** (`commerce-selling-doctrine.md:15,121,136`). Resolved if Product stays **internal-only** (no seller-facing inventory UI), which matches the current zero-product-surface reality. Not a blocker; wording decision.
2. **Unit identity across transitions.** "10 units" under B have no per-unit identity and no path to 10 auctions (each auction is single-unit and mint-per-product until §2.15.1 lands). Under per-channel quantity this is coherent (the 10 units are offerable copies, not individuals); under product-level inventory it requires a ledger. Decision, not contradiction.
3. **The koi uniqueness business fact.** B's "definition" reading must not erase that each koi is unique. Compatible: qty=1 (default, handler `:62-65`) means the definition IS that one koi; qty>1 means identical stock. Nothing in B forces a misleading "one definition = many physical koi".
4. **Historical leakage on reused products.** A sold FPS row stays as a channel row; an auction later on the same product creates a visible "previously sold at $X" history. This is a feature (trust/history), not a bug — but the share/search surfaces currently key to channel ids, so product-page history is a future surface decision.
5. **The strongest challenge**: Model B is the only model that makes `order_items.product_id` meaningful as `products.id`, and today that column is broken for FPS. Model B therefore *requires* the order-item fix, and the relist/quantity/product-status decisions, before it can be called coherent. That is why the verdict is REQUIRES_OWNER_DECISION, not VALIDATED.

**No challenge produced a hard business contradiction that invalidates Model B.**

---

## 4. Classification summary

| Proposed claim | Classification |
|---|---|
| Product = stable sellable item/inventory definition | UNKNOWN / OWNER DECISION (identity stability; "inventory" semantics) |
| quantity>1 coherent under B | PROVEN COMPATIBLE (per-channel quantity) / UNKNOWN (product-level inventory) |
| FPS and Auction attach to same Product safely | PROVEN COMPATIBLE (constraints) / code requires reuse support |
| one active selling surface per Product | PROVEN COMPATIBLE |
| relisting = reuse Product + new surface row | PROVEN COMPATIBLE (mechanically) / PROVEN CONTRADICTORY (with written doctrine "sold MUST NOT be reused") |
| FPS↔Auction transitions without cloning Product | PROVEN COMPATIBLE |
| order history = products.id + surface identity | PROVEN COMPATIBLE (requires order_items id-space fix) |
| shipping config belongs to Product | PROVEN COMPATIBLE (already true) |
| seller identity on Product vs surface | UNKNOWN / OWNER DECISION (duplicates enumerated; enforcement missing) |
| auction metadata canonicalized vs snapshot | PROVEN COMPATIBLE as snapshot; policy asymmetry to decide |
| product lifecycle independent vs derived | PROVEN COMPATIBLE with derive-from-surfaces (zero status consumers) |
| DB constraints support B | PROVEN COMPATIBLE (keep indexes/trigger; add FK/equality; decide status columns) |

## 5. Producer → storage → consumer weave (B focus)

- Product identity: produced only by channel creates (mint); stored `products.id`; consumed by FPS/Auction FK, pricing_tokens, shipping_quotes, saved_item hydration, order_items (broken namespace for FPS). Lifecycle: none independent; status projection written by FPS (derived) + auction (explicit) + order flows.
- Quantity: produced by seller (create/update FPS, frozen after orders); stored `fixed_price_sales.quantity_available`; consumed by `IsAvailable`, `ReduceQuantity`, order create, saved-item/seller dashboards. Only FPS has it.
- Price: produced by seller; stored on `fixed_price_sales.price_per_unit` and auction `start/bid/buy_now`; consumed via pricing token (server authority). None on Product.
- Seller: produced at channel create (input) into 3 rows; stored triplicated; consumers read surface seller (promotion, comment commerce-ref, order, saved-item, search).
- Auctions: own content snapshot columns (title/desc/prep) + product_id; edits mutate only auction row.

## 6. NOT PROVEN / runtime gaps

- No runtime test drives "FPS sold → new channel on the SAME product" through real DB (reuse is untested; only schema permits it).
- Whether product-level inventory would ever be needed has no evidence either way — pure owner decision.
- `products.status` having zero consumers is asserted from a code read of commerce/discovery/interaction/social; no runtime log proves it.
- The seller-equality divergence (auction path) is proven by absence of sync code, not by a failing test.
- Auction snapshot-vs-write-through asymmetry (FPS write-through vs auction no-sync) is proven structurally, not behaviorally.

---

## 7. FINAL VERDICT

**`MODEL_B_REQUIRES_OWNER_DECISION`**

Model B is not contradicted by any current business fact and is the most viable architecture for Labuda's quantity-per-surface, auction-exclusivity, and future product/history surfaces. It can become canonical only after the owner resolves:

1. **O-1 Quantity authority**: per-selling-surface quantity (keep `quantity_available` on FPS; recommend) vs product-level inventory ledger (new machinery).
2. **O-2 Relisting policy**: explicitly reverse the written "sold Product MUST NOT be reused" doctrine; relist = reuse Product + new surface row.
3. **O-3 Product lifecycle columns**: drop `products.status`/`sold_at` (availability derives from surfaces; recommended) vs keep as a maintained projection.
4. **O-4 Seller authority**: Product owns seller; enforce equality (FK/trigger) or drop channel seller columns.
5. **O-5 Auction metadata policy**: keep auction-owned facts as frozen snapshots (recommended; matches order snapshots) and make FPS stop write-through-syncing product content, OR unify both to write-through.

STOP after report. Nothing modified. No implementation advisory beyond the migration/code list in §2.15.