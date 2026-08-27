# COMMERCE — PRODUCT QUANTITY / AVAILABILITY SEMANTICS — STAGE 6A AUDIT

Read-only audit. No files modified, no migration, no rename, no cleanup, no shim.
All conclusions derive from the current filesystem (source + database schema/migrations).
Prior reports used only as leads; every claim below re-derived from code/SQL.

Locked business truth honored; anything found contradicting it is flagged with
exact evidence. This stage produced a **stop-condition finding** (§7.1 and §12)
which must be reported before any Stage 6B architecture work.

---

## 1. EXECUTIVE VERDICT

Manufactured "sold out" visibility mostly works today, but by **accident of
status convention, not by an availability predicate**:

- The system has ONE real stock authority: `fixed_price_sales.quantity_available`.
  It is a **remaining-units counter**, decremented at order creation, restored on
  cancel/expire, never on refund or completion.
- "Sold out" is *defined* as `status='sold'`, produced only by
  `ReduceQuantity` reaching 0. Both FPS buyer-discovery queries then hide it —
  but only because they filter `fps.status='active'`. **No query anywhere
  enforces `quantity_available > 0`.** One discovery code path documents that
  predicate and then fails to implement it
  (`discovery/search/search_repository_impl.go:104` vs `:160`).
- Default non-stock model holds: create defaults quantity to 1
  (`fixed_price_sale_handler.go:244-247`), `CHECK (quantity_available >= 0)`
  (migration 000009:19-20). A qty=1 listing behaves exactly as a unique item.
- Seller inventory visibility holds: `GET /api/v1/listings?seller_id=<seller>`
  returns sold rows through `GetBySellerIDPaginated`
  (`fixed_price_sale_handler.go:810-813`; repo
  `fixed_price_sale_repository_impl.go:297-311`).
- Comment/chat references resolve sold-out surfaces as LIVE projections carrying
  `status` + `quantity_available` and `CanInteract=false` — referenceable and
  labelable, never purchasable. Business truth #10 is satisfiable from current
  data.
- **Reuse contradiction CONFIRMED (§7):** a sold FPS reusing a Product creates a
  brand-new `fixed_price_sales` row whose quantity is the seller's fresh
  declaration. Old quantity is neither carried nor validated. Silent reset.
- **Two access-control/discovery defects discovered** (not fixable in 6A):
  1. `GET /api/v1/listings?seller_id=<any>` runs on the anonymous browse group
     and returns **draft + sold + active** listings of any seller with no
     ownership gate (§5, §6). Another seller's private workspace and history
     are publicly enumerable.
  2. `GET /api/v1/auctions` browse has no status restriction — anonymous
     discovery returns drafts, cancelled, and fully-settled (`ended`) auctions
     (§5, §9). Auction search also includes `ended` auctions (§5).

Verdict: **no change to the quantity model is required to satisfy the locked
truths, but Stage 6B must NOT be an inventory-model change.** Stage 6B should be
a narrow visibility-contract hardening (explicit availability predicates +
access gating on the seller-inventory branch), plus explicit re-sale quantity
governance so silent reset is at least a decided, contractual behavior.

---

## 2. BUSINESS TRUTH RE-DERIVED

Re-derived from the filesystem, each with evidence:

| # | Locked truth | Derived state | Evidence |
|---|---|---|---|
| 1 | Product = stable commerce identity | HOLDS | `products` has no quantity/lifecycle cols (000001:1322-1342); entity doc "internal physical item authority … intentionally sale-surface agnostic" (product/entity/product.go:9-10) |
| 2 | Product reusable across sequential surfaces | HOLDS | `ProductID` reuse in FPS create (fixed_price_sale_repository_impl.go:42-56) and auction create (auction_service.go:270-304); single-active-surface trigger (000010:76-104); runtime proof (tests/product_identity_reuse_integration_test.go) |
| 3 | Product does NOT own stock/quantity | HOLDS | zero quantity hits in `backend/internal/commerce/product`; no column |
| 4 | Stock is a selling-surface concern | HOLDS | sole authority `fixed_price_sales.quantity_available` (000009:15-20) |
| 5 | FPS owns `quantity_available` | HOLDS | producer/reader map §3 |
| 6 | Auction single-unit | HOLDS | no quantity column (000001:477-498); every path hardcodes 1 (order_creation_service.go:920,979; pricing_token_service.go:1204,1300-1304,1362) |
| 7 | Default NON-STOCK (unique item, one unit) | HOLDS, ACTIVE | create binding `Quantity *int … min=1`, defaulted to 1 when omitted (handler.go:66-69,244-247); entity requires `>=1` (fixed_price_sale.go:343-346); column `DEFAULT 1` (000009:16) |
| 8 | Sold-out product disappears from ALL buyer discovery | HOLDS-BY-CONVENTION, NOT-ENFORCED | all FPS discovery filters are `status='active'` only; no `quantity_available>0` predicate anywhere; "INVENTORY TRUTH" comment unimplemented (search_repository_impl.go:104 vs :160) |
| 9 | Sold-out visible to seller dashboard/history | HOLDS | `GetBySellerIDPaginated` returns sold rows (only withdrawn excluded) (fixed_price_sale_repository_impl.go:297-311); dashboard totals count all rows (seller_handler.go:812-814); **but** see §6 access-control defect |
| 10 | Sold-out referenceable in comments/chat with Sold-Out label, never purchasable | HOLDS (payload-capable) | content resolver loads FPS by any status → LIVE payload with status+qty, `CanInteract` false (content_resource_projection_resolver.go:561-646, 180-214); chat projection carries status+qty, CanBuy=false via shared evaluator (fixed_price_sale_viewer_capabilities.go:48-63); view access allows sold (view_access.go:59-66) |
| 11 | stock>1 discoverable until 0 → sold-out rule | HOLDS-BY-CONVENTION | `ReduceQuantity` flips sold at 0 (fixed_price_sale.go:258-261); discovery hides via status filter only |
| 12 | Reuse MUST NOT silently reset/manufacture stock | **VIOLATED** | new FPS row declared fresh each time (§7) |
| 13 | Product identity stable across relist | HOLDS | reuse keeps `products.id` (fixed_price_sale_repository_impl.go:42-56; integration test :122-150) |
| 14 | `order_items.product_id = products.id` | HOLDS | 000045; restore resolves surface via `source_id` (order_completion_service.go:1948-2024) |
| 15 | No Product lifecycle cols | HOLDS | removed 000044:16-19; no reintroduction |

Contradiction with locked truth detected: **#12** (and thereby a stage-
STOP condition — see §12).

---

## 3. QUANTITY AUTHORITY

### 3.1 The column
`fixed_price_sales.quantity_available integer DEFAULT 1 NOT NULL` with
`CHECK (quantity_available >= 0)` (migrations/000009_fixed_price_sale_quantity_persistence.up.sql:15-20).
It is the **only** quantity-bearing production column besides the order/order-item
snapshots (`orders.quantity`, `order_items.quantity`, `pricing_tokens.quantity`,
all 000001:1029,1093,1267).

### 3.2 Writers of `quantity_available` (complete)
| Writer | Site | When |
|---|---|---|
| Create (mint + reuse) | fixed_price_sale_repository_impl.go:86-116, `listing.QuantityAvailable` | insert with seller-declared value (default 1) |
| Seller edit | fixed_price_sale_handler.go:418-438 → `Update` (repo_impl.go:155-178) | direct set, guarded: blocked when any order exists for the Product and the value changes (handler.go:404-425) |
| Reservation decrement | order_creation_service.go:1504-1511 (`ReduceQuantity` + `UpdateStock`) | order creation |
| Restore | order_completion_service.go:2017-2022 (`RestoreQuantity` + `UpdateStock`) | cancel/cancel_overdue/expire only |
| Migration backfill | 000009:27-28 | sold/withdrawn → 0 (from-zero) |

No other writer exists. `FixedPriceSaleService.ReduceStock`
(fixed_price_sale_service.go:553-594) is **dead code** — zero callers.

### 3.3 Readers of `quantity_available` (complete)
| Reader | Site | Purpose |
|---|---|---|
| Entity field | fixed_price_sale.go:40 | domain state |
| Availability gates | entity `IsAvailable` (fixed_price_sale.go:300-302); shared evaluator (fixed_price_sale_viewer_capabilities.go:75-76); checkout (order_creation_service.go:327-332); pricing token (pricing_token_service.go:246-247, 911-913) | buy/negotiate gating |
| Discovery wire | fixed_price_sale_handler.go:1094; response_projection.go:120 | `"quantity"` field on listings browse/detail/search |
| Saved items | saved_item_repository_impl.go:202,233 | `quantity_available` display |
| Feed/comment/chat projections | content_resource_projection_resolver.go:576,622; content_resource_projection.go:57; chat_resource_projection.go:125; serverboot/chat_fixedprice_projection_resolver.go:252,269 | live payload display + CanInteract |
| Search | discovery search claims it but does NOT select/compare it (search_repository_impl.go:104, projection has no quantity) | dead promise |

### 3.4 Decrement / reservation paths
Exactly one live reduction writer: order creation. Presence of **pending, unpaid
orders therefore reduces `quantity_available` immediately** (reservation, not
fulfillment). Only OrderService.cancel / cancel_overdue / expire restore
(counted from `SUM(order_items.quantity)`, restoreFixedPriceListingStock
order_completion_service.go:2011-2024). If the buyer of the last unit never
pays and no worker expires it, the product stays hidden until expiry — correct,
but worth knowing the hide-trigger is reservation, not completion.

### 3.5 Sold-out trigger (exact)
`ReduceQuantity` when `QuantityAvailable` becomes 0 → `Status = sold`,
`SoldAt = now` (fixed_price_sale.go:258-261). `sold` terminal (no outgoing
transition, fixed_price_sale_status.go:78); moderation restore refuses sold
(fixed_price_sale.go:199-201); only direct SQL or an order-cancel/expire on a
qualifying order can ever revive it (`RestoreQuantity` flips sold→active only
when qty>0, fixed_price_sale.go:280-284).

### 3.6 Default non-stock behavior (truth #7)
Quantity omitted → 1 (handler.go:244-247). qty=1 + active is the identical
profile to any unique item: sold when the single unit's order is placed.
Model-consistent. **No stock thinking required of the seller on the happy path.**

---

## 4. STOCK MUTATION LIFECYCLE (authoritative order)

```
seller creates FPS  -> quantity_available = new value (default 1)   [Create]
seller edits FPS    -> quantity_available = new value               [Update; blocked if orders exist]
buyer order created -> ReduceQuantity(order.quantity)               [reservation]
                       if qty==0: status=sold, sold_at=now
order cancel/expire -> RestoreQuantity(SUM order_items.quantity)    [only restore path]
                       if sold && qty>0: status=active, sold_at stays (MarkSold time not cleared)
order complete      -> (no stock effect; metrics only)
order refund        -> (no stock effect)
withdrawn           -> status=withdrawn; qty value retained on row   [terminal]
moderation restore  -> withdrawn->active only; qty unchanged; sold rejected
sold-out            -> defined by status='sold' (= qty reached 0 via ReduceQuantity)
```

- Completion does **not** decrement; refund does **not** restore. Under default
  qty=1 the model is exact: one order reserves the unit; cancel/expire returns
  it; completion locks it sold forever.
- Withdrawing a qty>0 listing leaves `quantity_available` untouched on a
  withdrawn row — a number that can never sell. Source: `Withdraw`
  (fixed_price_sale_service.go:333-372 → `UpdateStatus`, repo_impl.go:218-250,
  does not touch quantity).

---

## 5. BUYER DISCOVERY VISIBILITY MATRIX

Routes (cmd/core_server/routes_core.go): `v1Browse` group is
`StrictBrowseAuthMiddleware` = **anonymous allowed** (middleware/auth.go:222-230).

| Discovery path | Route/entry | Filter | qty>0 enforced? | Sold-out leak? | Violation vs truth #8 |
|---|---|---|---|---|---|
| FPS marketplace browse | GET /api/v1/listings (no seller_id) → `GetPublic` (fixed_price_sale_handler.go:817-823; repo_impl.go:313-330) | `fps.status='active'` + seller account active/not deleted | NO | no live leak (sold has status='sold') | OK-by-convention |
| FPS search | GET /api/v1/search/listings → `Search` (repo_impl.go:332-410) | `fps.status='active'` + account filters | NO | no live leak under convention | OK-by-convention |
| Federated/search discovery SQL | discovery/search/search_repository_impl.go:138-162,253-260 | `fps.status='active'` + banned/deleted filter | **documented but NOT implemented** (:104) | no live leak under convention | OK-by-convention / documented gap |
| Feed commerce projections | content resolver live payloads (feed/content detail) | FPS row exists (any status) → LIVE | n/a (reference, not discovery) | referenceable on purpose (truth #10) | n/a |
| Content search reposts FPS | search_repository_impl.go:344-352 | excludes reposts whose `fps.status != 'active'` | NO (status only) | no live leak under convention | OK-by-convention |
| Saved items | GET /api/v1/saved-items → saved_item_repository_impl.go:196-250 | `LEFT JOIN fps … target_type='listing'`, no status filter | NO | sold rows shown (history surface) | reference surface; truth #10-compatible if labeled |
| Auction browse | GET /api/v1/auctions → List (auction_repository.go:350-412) | seller account active only; **status optional** | n/a | **drafts, cancelled, settled `ended` all returned when no ?status= param** (handler ListAuctions default filter nil, auction_handler.go:948-958) | LEAK (not product stock, but sold-out-rule adjacent) |
| Auction search | GET /api/v1/search/auctions → search_repository_impl.go:624-772 | `status IN ('scheduled','active','ended')` | n/a | `ended` = frequently already-settled (order created) | LEAK of settled auctions |

**The recurring pattern:** every FPS discovery query uses `status='active'` as the
availability proxy. That proxy is correct today only because the single
decrement writer also flips to `sold` at 0. Any future writer that modifies
quantity without flipping status, or any data drift, silently resurrects an
out-of-stock (or zero-qty) surface into discovery. Stage 6B should add an
explicit `quantity_available > 0` predicate (or a derived `is_available`
predicate) at these query sites.

Secondary finding (not fixed in this stage): `GET /api/v1/listings?seller_id=X`
uses `GetBySellerIDPaginated` (repo_impl.go:297-311) which returns
**draft, sold, active** rows (only `withdrawn` excluded by default). Because the
route is anonymous-access and the branch only block-checks the seller
(handler.go:788-801, 810-813), **any anonymous visitor can enumerate another
seller's draft and sold listings.** See §6.

---

## 6. SELLER / INTERNAL VISIBILITY MATRIX

| Surface | Route/entry | Sold-out visible? | Evidence |
|---|---|---|---|
| Seller dashboard | GET /api/v1/seller/dashboard → seller_handler.go:792-868 | TotalListings counts **all** rows (`COUNT(*) FROM fixed_price_sales WHERE seller_id=$1`, :812-814); ActiveListings = status='active' (:820-822); SoldItems = completed orders (:827-833) | HOLDS truth #9 (count) |
| Seller listing inventory | GET /api/v1/listings?seller_id=<self> → GetBySellerIDPaginated (repo_impl.go:297-311) | sold rows returned (only withdrawn excluded) | HOLDS truth #9 |
| Order history | GET /api/v1/orders role=seller (order_handler.go:80-...) | orders persist regardless of product state | HOLDS |
| Analytics/performance | GET /api/v1/seller/analytics (:881-937), /performance (:954-1021) | completed-order counts, not listing visibility | n/a |
| Admin | admin_order_handler.go OrderItemDetail.Quantity (read-only display) | n/a (orders) | n/a |

**Access-control defect (must be decided in 6B):** the seller-inventory branch
of the public browse route has **no ownership or visibility gate**. Evidence:
- Route: `v1Browse.GET("/listings", …)` anonymous-allowed (routes_core.go:145;
  middleware/auth.go:226-229).
- Branch: `if sellerID != nil { listings, err = GetBySellerIDPaginated(...) }`
  (fixed_price_sale_handler.go:810-813) — no `listing.SellerID == callerID`
  check, no draft-visibility check.
- Query returns draft + active + sold rows (repo_impl.go:297-311).
- Contrast: the private-detail endpoint DOES gate drafts
  (GetFixedPriceSale, handler.go:650-656: private → owner-only) — so the
  inventory branch is inconsistent with the detail branch on the same table.

---

## 7. COMMENT / CHAT REFERENCE VISIBILITY

Objectionable to truth #10, verified:

- **Resolution:** an existing commerce reference resolves as long as the FPS row
  exists. Content resolver loads by any status (`loadFixedPriceSales`
  content_resource_projection_resolver.go:570-640, no status predicate;
  `isLive:true` at :639). Missing row only → tombstone. Sold-out is therefore
  live-referenceable. Chat resolver likewise loads and evaluates view access
  which permits `sold`/`withdrawn` public surfaces (view_access.go:59-66) and
  projects `Status` + `QuantityAvailable`
  (serverboot/chat_fixedprice_projection_resolver.go:263-270).
- **Differentiation data:** payloads carry both `status` (raw enum) and
  `quantity_available`; `CanInteract` computed as
  `status=="active" && quantityAvailable>0` (content resolver
  content_resource_projection_resolver.go:206) or via shared buyer-capability
  evaluator (fixed_price_sale_viewer_capabilities.go:48-63), yielding
  `CanBuy=false` for sold surfaces. Enough information exists today to render
  `Sold Out` / `Terjual` and to prevent purchase.
- **Canonical availability source:** the FPS row itself —
  `fps.status` + `fps.quantity_available` (Product has neither, per truth #3).
  Any consumer rendering a sold-out label must read these two columns;
  a single `is_sold_out` derivation would be redundant to (status) but safe for
  qty=0 divergence cases.

No comment/chat redesign performed. Only the above evidence captured.

---

## 8. PRODUCT REUSE VS STOCK — DEFECT CONFIRMATION

### 8.1 The behavior (proven, not assumed)
FPS create with `ProductID` set:
- resolves + ownership-checks the existing product
  (fixed_price_sale_repository_impl.go:42-51);
- inserts a NEW `fixed_price_sales` row with `listing.QuantityAvailable`
  (the fresh seller-declared value) (insertFixedPriceSaleRow :86-116);
- never reads or merges any prior surface's `quantity_available`.

Auction create with `ProductID` set: same pattern, no quantity involved
(auction_service.go:270-304).

### 8.2 Decision scenario
```
FPS_1 (product P): quantity_available=10
  → 3 units sold (orders on FPS_1; quantity_available=7; maybe status=sold at 0)
  → surface sold/withdrawn (terminal)
  → seller reuses P for FPS_2: quantity_available = new declared value (e.g. 10)
  → OLD FPS_1 row retains 7 (or 0); NEW FPS_2 row holds its own value.
```
- If the 7 remaining units were real inventory, **they are silently reset**:
  no carryover, no carry-in validation (e.g. cannot declare 5 after 7 already
  sold — nothing checks 5+3<=10), no warning.
- Cancel/expire of an order on FPS_1 (if it still has qty>0 rows or a canceled
  order) restores into **FPS_1's row** (restore resolves by `order.source_id`,
  order_completion_service.go:2006-2024) — units return to a row that is no
  longer the active selling surface.
- Product identity remains stable (truth #13 holds), which is precisely why the
  discontinuity is invisible and dangerous: the same `product_id` suddenly
  shows a different "stock" number with no ledger explaining it.

### 8.3 Classification
**CONFIRMED LIFECYCLE DEFECT** against locked truth #12
("must NOT silently reset or manufacture stock semantics"). Current behavior is
silent reset that can manufacture stock (a fully-sold product relisted with
`quantity=10` implies 10 new units appear from nothing on the same identity).

### 8.4 Not resolved here
No architecture change performed. Stage 6B must obtain an owner decision
(§13: DECISION B) and then implement one of: (a) explicit re-sale quantity
declaration contract with verification against prior-surface orders; (b)
block relist while a prior surface holds nonzero remaining qty or live orders;
(c) product-level ledger (out of 6B scope — requires truth #3 reconsideration).

---

## 9. AUCTION QUANTITY SEMANTICS

- **Single-unit:** enforced by construction, no field. Order/order-item/token
  quantity hardcoded 1 (order_creation_service.go:920,979; auction.go entity
  model: no quantity; pricing_token_service.go:1204,1300-1304).
- **Independent of FPS stock:** auction settlement never touches
  `fixed_price_sales.quantity_available`; order restore for auction clears only
  `auctions.order_id` (order_completion_service.go:1957-1958, 2036-2054).
- **Compatible with Product reuse:** `CreateDraft` attaches `ProductID` to an
  existing product (auction_service.go:270-304); trigger allows it once the FPS
  is sold/ended (000010:76-104; runtime proof
  tests/product_identity_reuse_integration_test.go:189-196).
- **Not accidentally dependent on FPS quantity:** auction token generation
  validates the auction entity, not an FPS row (pricing_token_service.go:1104-
  1158); the synthetic FPS view used for auction checkout hardcodes qty 1 and is
  never persisted (order_creation_service.go:775-788).

**Discovery leak (auction-specific, flagged for 6B):** public browse
`GET /api/v1/auctions` returns all statuses when `?status=` is omitted
(auction_repository.go:382-404; handler default nil filter), including
`draft`, `cancelled`, and settled `ended`. Auction search includes `ended`
(search_repository_impl.go:662). A settled (sold) auction remains findable in
buyer discovery.

---

## 10. PROVEN GOOD

1. Single transition table enforces `sold`/`withdrawn` terminal
   (fixed_price_sale_status.go:75-80); moderation restore refuses sold
   (fixed_price_sale.go:199-201) — reviving a sold surface requires an order
   event, not a seller edit.
2. Reservation decrement is row-locked and guarded; oversell rejected before
   write (fixed_price_sale_repository_impl.go:128-131; fixed_price_sale.go:
   246-252; integration tests quantity_persistence_test.go:244-311,
   listing_stock_roundtrip_test.go).
3. Restore path resolves the correct surface via `orders.source_id` and sums
   `order_items.quantity` exactly (order_completion_service.go:2006-2024;
   order_completion_restore_source_integration_test.go).
4. Default non-stock: quantity defaults 1, existing koi-unique flow unaffected.
5. Comment/chat reference payloads carry enough state to label sold-out and
   cannot leave those surfaces purchasable (content resolver :204-207; shared
   evaluator :48-63).
6. Seller history visibility correct in count and inventory-list surfaces
   (§6), ignoring the access-control defect.
7. Order create takes quantity from the validated pricing token, never client
   input (order_handler.go:1035,1058).
8. Edit guard freezes price/title/quantity once any order exists on the
   product (fixed_price_sale_handler.go:404-425).

## 11. PROVEN BAD

1. **Silent stock reset on reuse (truth #12 violation)** — §8. CONFIRMED.
2. **Availability is inferred from `status='active'`; `quantity_available>0`
   is never enforced by any discovery query** — documented-but-absent in
   discovery search (search_repository_impl.go:104 vs :160); absent in
   `GetPublic` (repo_impl.go:313-330) and FPS `Search` (:341).
3. **`GET /api/v1/listings?seller_id=<any>` leaks draft + sold listings of any
   seller anonymously** (anonymous browse + ungated GetBySellerIDPaginated) —
   §6.
4. **Auction browse/search expose non-market states (draft, cancelled, settled
   `ended`)** to anonymous discovery — §9.
5. Withdrawing a qty>0 surface leaves a dead `quantity_available` value on the
   withdrawn row with no semantics (a number that can never be sold nor
   inspected).
6. `FixedPriceSaleService.ReduceStock` is dead code (zero callers) while the
   live decrement is an inline repository pattern — two "stock APIs", one real.

## 12. DEAD / DUPLICATE / AMBIGUOUS AUTHORITY

- Dead: `FixedPriceSaleService.ReduceStock` (fixed_price_sale_service.go:553-594).
- Dead promise: "INVENTORY TRUTH: Filters out out-of-stock items" comment on
  discovery search (search_repository_impl.go:104) — SQL has no quantity
  predicate and `ListingPreview` has no quantity field.
- Ambiguous: **"sold out" has two candidate authorities** — `status='sold'`
  (effective rule) vs `quantity_available=0` (definitional rule). They coincide
  under current writers but can diverge via: direct status edit setting `sold`
  with qty>0 (handler.go:458-495), withdrawal with qty>0, or any future
  quantity writer. A single derived `is_sold_out`/availability predicate is
  needed; today consumers each re-interpret status+qty independently.
- Ambiguous: `GET /api/v1/listings` is simultaneously buyer marketplace
  (GetPublic) and seller inventory (GetBySellerIDPaginated) selected by a
  query parameter, with different authority scopes and no gate.

**STOP-CONDITION (per mandate):** locked truth #12 is contradicted by current
implementation (§8). Architecture changes are NOT designed in this stage;
reporting is the required action. Stage 6B must first obtain the owner decision
below before code.

## 13. REQUIRED OWNER DECISIONS

### DECISION A (needed for any 6B work): sold-out enforcement point
- Current: status-transition convention only; no `quantity_available>0`
  predicate in discovery.
- Options: (1) add explicit `quantity_available>0` (or derived availability) to
  every buyer discovery query; (2) keep status-only and additionally hard-enforce
  the qty=0→sold transition at DB level (cannot be, without a trigger/constraint
  change); (3) decide `status='sold'` IS the sold-out definition and forbid any
  writer from creating the decomposed states.
- Recommendation (not fact): Option 1 for discovery + codify sold-out as exactly
  "status=sold OR quantity_available=0" in one shared predicate, because the
  quantity counter is the physical truth and status is a projection of it.

### DECISION B (REQUIRED by the confirmed defect, truth #12): reuse/resale quantity
- Current: silent reset of quantity on relist; can manufacture stock.
- Options: (a) mandate an explicit quantity declaration per resale with a check
  `declared <= max(0, prior_declared − prior_reserved)`; (b) block resale while
  any prior surface has remaining qty or live (non-terminal) orders; (c)
  introduce a product-level quantity ledger (CHANGES truth #3 — must not be
  decided lightly).
- Recommendation (not fact): (a) — smallest change that preserves Model B and
  makes reset explicit and verifiable; reserve (c) only if the owner chooses
  Product-level inventory.

### DECISION C: buyer/seller browse separation on `GET /api/v1/listings`
- Current: single anonymous route serves both marketplace and a seller's
  inventory with no ownership gate.
- Options: (1) gate the `seller_id` branch to authenticated owner; (2) move
  seller inventory to an authenticated `/seller/listings` endpoint; (3) keep
  status-visible public seller history but exclude drafts from anonymous view.
- Recommendation (not fact): (2) — cleanest; seller inventory is an internal
  surface (truth #9) and does not belong on an anonymous browse route.

### DECISION D: auction browse status eligibility
- Current: browse exposes all statuses; search exposes `ended`.
- Options: (1) browse defaults to `scheduled,active`;(2) also include
  `waiting_settlement` (winner pending) — a win-claimable state buyers may
  legitimately inspect; (3) exclude `ended` from search unless supporting
  "sold for reference" UX.
- Recommendation (not fact): (2) + exclude settled `ended` from discovery.

## 14. RECOMMENDED STAGE 6B SCOPE (narrow, implementable)

Stop architecture work. Execute in order:

1. **Owner decisions A–D** recorded (ADR-style) — required gate; nothing below
   proceeds without B.
2. **Shared availability predicate + discovery enforcement**: add
   `quantity_available > 0` (or derived equivalent) to GetPublic, FPS Search,
   discovery SearchListings (and remove/realize the INVENTORY TRUTH comment);
   implement auction browse/search eligibility per DECISION D.
3. **Browse separation**: introduce authenticated seller inventory surface and
   gate leaks identified in §6/§9 (DECISION C).
4. **Reuse governance (truth #12)**: per DECISION B — explicit resale-quantity
   declaration validation on the FPS-create-with-ProductID path (not a ledger;
   no Product schema change).
5. **Dead-code/comment cleanup judged in-scope only if it does not widen the
   diff**: drop `ReduceStock` or repoint callers; align status/qty divergence
   handling per DECISION A option 3 if chosen.
6. **Evidence**: add regression tests: (i) sold-out not returned by
   marketplace/search even if `quantity_available` is 0 while status is
   `active` (drift case); (ii) anonymous buyer cannot read another seller's
   draft inventory; (iii) relist with over-reserved quantity is rejected.

Explicitly OUT of 6B: payment, coins, finance, ledger, refund, commission,
reconciliation, social/content/chat redesign, Product schema, listing rename,
any product-level inventory model.

STOP — Stage 6A complete. No Stage 6B work started.