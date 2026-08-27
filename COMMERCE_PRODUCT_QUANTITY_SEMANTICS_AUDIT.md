# COMMERCE — PRODUCT QUANTITY / INVENTORY SEMANTICS AUDIT (STAGE 6)

Read-only architecture audit. No files modified. Claims derived exclusively from
the current filesystem; prior reports used as evidence pointers only, never as
authority. Locked prior decisions honored verbatim (Product = stable physical
identity, sequential sale-surface reuse, one active surface per Product,
`order_items.product_id` = `products.id`, source identified by
`orders.source_type` + `orders.source_id`, no production-data compatibility).

---

## 1. EXECUTIVE VERDICT

Quantity in this codebase has **exactly one owner**: `FixedPriceSale.quantity_available`
(a `fixed_price_sales` column). `products` carries **no quantity state at all**.
`auctions` carry **no quantity state at all** — auction quantity is an
un-modeled implicit `1` hard-coded at every usage site. `orders.quantity` /
`order_items.quantity` are **buyer-requested unit snapshots**, carved from the
sourcing FPS pool at order creation (reservation semantics), restored on
cancel/expire, never on refund.

The model is coherent **for a single, one-time FPS surface**. It is **contradicted
by Product-as-physical-item (Model B) once any quantity > 1 exists or any Product
is reused after a partial sale**. There is no product-level inventory ledger, no
per-unit identity, and no remaining-unit continuity across surface reuse. The
system intentionally and silently resets quantity on every relist because the
new surface is a brand-new `fixed_price_sales` row with a freshly-declared
`quantity_available`.

Concurrency for the FPS decrement path is **PROVEN** at the mechanism level
(`GetForUpdate` → `FOR UPDATE OF fps, p` row lock + entity `ReduceQuantity`
guard + integration tests proving oversell rejection and persistence). It is
**NOT PROVEN** by any parallel/race test, and the expiry path edit surface
(`PUT /fixed-price-sales/:id` quantity edit) does not take the row lock.

---

## 2. PRODUCT QUANTITY AUTHORITY

**Products has NO quantity authority. Explicitly: none.**

- `products` schema (migrations/000001_canonical_schema.up.sql:1322-1342):
  id, seller_id, title, description, media_urls, variety, size_cm, age_months,
  gender, breeder, bloodline, certificates, farm_address_id, preparation_time,
  preparation_note, created_at, updated_at. **No quantity, stock, inventory,
  reserved, or sold_quantity column.**
- `products.status` / `products.sold_at` were removed in
  migrations/000044_product_lifecycle_removal.up.sql:16-19 (no production data
  requirement). Availability is derived solely from the active selling surface.
- `Product` entity (backend/internal/commerce/product/entity/product.go:11-29):
  13 fields, all physical-item attributes, sale-surface agnostic. **No quantity field.**
- Repo-wide grep over `backend/internal/commerce/product`: zero hits for
  quantity/stock/inventory.
- No product-level ledger, no per-unit identity (no serial/sku/instance table),
  no product-level increment/decrement anywhere.

Product represents **one physical item** by design (Model B). It cannot
represent multiple fungible units, because nothing on the product row can hold
that fact. Quantity is **not** reconstructed from surfaces/orders for Product
either — nothing reads sum-of-surface-quantity to derive Product stock.

---

## 3. FPS QUANTITY AUTHORITY

Authority is `FixedPriceSale.QuantityAvailable`, persisted as
`fixed_price_sales.quantity_available integer DEFAULT 1 NOT NULL CHECK (>= 0)`
(migrations/000009_fixed_price_sale_quantity_persistence.up.sql:15-20).

### 3.1 Lifecycle trace (all source evidence)

1. **Initial quantity** — seller-supplied at create. Handler binding
   `fixed_price_sale_handler.go:66-69`: `Quantity *int json:"quantity"
   binding:"omitempty,min=1"`; defaulted to `1` (unique-item mode) at
   `:244-247`; stored via `CreateFixedPriceSaleInput.QuantityAvailable`
   (`:265`) → `entity.NewFixedPriceSale` (`fixed_price_sale.go:319-382`,
   requires `quantity >= 1` for fixed-price type).
2. **Stored** — `insertFixedPriceSaleRow`
   (fixed_price_sale_repository_impl.go:86-116) writes `quantity_available`.
3. **Available quantity** — the persisted scalar itself. Read by the entity
   field; exposed to buyers as `"quantity"` (`fixed_price_sale_handler.go:1094`,
   `fixed_price_sale_response_projection.go:120`). Migration 000009 fixed a
   pre-existing bug where quantity was derived from status on every scan
   (000009:3-8).
4. **Purchase reservation** — `order_creation_service.go:1490-1511`:
   `GetForUpdate` (row lock) → `listing.IsAvailable()` → `ReduceQuantity(input.Quantity)`
   → `UpdateStock`. `ReduceQuantity` (`fixed_price_sale.go:235-264`) decrements
   and **auto-transitions to `sold` at quantity 0**. This is a reservation at
   checkout, not fulfillment-counting; completion later does not touch stock.
5. **Cancellation / expiry restore** — `order_completion_service.go`:
   `Cancel` (`:714`, restore at `:770`), `CancelOverdue` (`:949`),
   `Expire` (`:1034`) all call `restoreListingStock` (`:1952`) which routes
   FPS orders to `restoreFixedPriceListingStock` (`:1981`): locks the sourcing
   listing via `order.source_id`, sums `order_items.quantity`, calls
   `RestoreQuantity(total)` (`fixed_price_sale.go:271-287`, revives `sold →
   active` when quantity becomes > 0), `UpdateStock`.
6. **Completion** — `Complete` (`order_completion_service.go:369`) and its
   wallet/escrow success path never touch quantity. No second decrement.
7. **Refund** — `RefundOrder` (`:1261`), `RefundFromDispute` (`:1416`),
   `PartialRefundFromDispute` (`:1697`) never call `restoreListingStock`
   (verified: the only three callers are Cancel/CancelOverdue/Expire).
   **Refunds never restore stock.**
8. **Concurrency** — see §8.
9. **At zero** — `ReduceQuantity` → empty (`fixed_price_sale.go:258-261`):
   status `sold`, `SoldAt` set. `sold` is terminal for seller edit
   (`fixed_price_sale_handler.go:396-398`).
10. **Edit while orders active** — handler guard `fixed_price_sale_handler.go:404-425`:
    product-keyed `CountAnyOrdersByProduct`; if any order exists, changing
    `price`, `title`, or `quantity` from their current values is rejected.
    Non-critical fields (description, media, variety, preparation, visibility)
    remain editable. Guard is handler-only, **not DB-enforced**, and the
    count query is not locked.
11. **Quantity on Product reuse** — a new FPS attaching to an existing Product
    (`fixed_price_sale_repository_impl.go:42-56`) writes a **brand-new**
    `fixed_price_sales` row with the seller-declared quantity. The old surface
    row and its quantity remain untouched. Nothing is copied from or merged
    with any prior surface. See §7.

`FixedPriceSaleService.ReduceStock` (`fixed_price_sale_service.go:553-594`) is
**dead code** — zero callers repo-wide. All live decrement happens inline in
`CreateFromSaleSurface`.

`RestoreQuantity` is deliberately not exposed on `FixedPriceSaleService`
(documentation `fixed_price_sale.go:226-227`); restore is reachable only through
the order-completion path. Moderation restore `MarkActiveFromModeration`
(`fixed_price_sale.go:186-206`) can revive `withdrawn → active` but **rejects
`sold`, and re-activation does not modify quantity**.

### 3.2 Ownership conclusion

Quantity belongs to **`FixedPriceSale`**, not Product. Product has no
inventory state to host it; every writer and reader named in §10 reads/writes
`fixed_price_sales.quantity_available`.

---

## 4. AUCTION QUANTITY SEMANTICS

**Auction is always quantity 1, and this is enforced by construction, not by a
quantity field.**

- `auctions` schema (000001_canonical_schema.up.sql:477-498): **no quantity
  column.** Auction entity (`auction/entity/auction.go:277-322`): no quantity.
- Every auction price/order path hard-codes 1:
  - `order_creation_service.go:776-808`: synthetic in-memory FPS surface with
    `QuantityAvailable: 1`; `Quantity: 1` validation; `NewOrderFromSource(..., 1, ...)`
    (`:920`) and `NewOrderItem(..., 1, ...)` (`:979-985`).
  - Pricing token: `NewPricingTokenFromAuction(..., 1, ...)`
    (pricing_token_service.go:1300-1304), snapshot `Quantity: 1` (`:1362`),
    "Auction is always for 1 item" (`:1204`).
  - Claim path passes `0` for quantity (auction_handler.go:794-796) with
    "Quantity from token"; token validation uses token-stored 1.
- Buy-now and bid-win are consistent on quantity (both 1): buy-now sets the
  token via `GenerateForAuction` (`pricing_token_service.go:1124-1127`) and
  order via `CreateFromAuction` with `AuctionSettlementBuyNow`
  (order_handler.go:1090-1121); bid-win via claim (`auction_handler.go:812-838`).
- Cancel/expired/refund of an auction order (PASS_20B) **does not release any
  quantity** — there is none. `restoreListingStock` routes auction orders to
  `releaseAuctionOrderBinding` (`order_completion_service.go:2036-2054`), which
  only clears `auctions.order_id`; the auction stays `ended` (terminal), and a
  resale requires a new surface on the (reused) Product
  (`auction.go:434-452`).
- Multiple units can never be sold through one auction: `auctions` has no
  quantity knob, all order paths emit 1, `OrderID` is a single settlement slot
  (double-settlement guard `auction.go:274-276`, `GenerateForAuction` `:1163-1166`).

Do not force auction into FPS quantity semantics — source proves they are
distinct: FPS has a persisted pool; auction has an implicit singleton.

---

## 5. ORDER QUANTITY SEMANTICS

- `orders.quantity` / `order_items.quantity` are **buyer-requested unit
  snapshots**, captured at order creation (both persisted by
  `order_repository.go:57-60` and `:850-851`).
- Writer chain: pricing token mints `quantity` (`pricing_tokens.quantity`,
  000001:1267) from the preview request; order creation uses
  `validatedToken.Quantity` (`order_handler.go:1035`, `:1058`) — **request
  quantity is never trusted at order time**. `order_items.quantity` is set from
  the same value (`order_creation_service.go:1667-1673`).
- NOT copied from FPS's `quantity_available`; it is an independent buyer choice
  bounded by `QuantityAvailable` at two gates (pricing-token generation
  `pricing_token_service.go:246-247`; checkout validation
  `order_creation_service.go:327-332`).
- Represents **reserved units at checkout**, not units actually shipped.
  Restored on cancel/expire (§3.1.5), never on refund, and never decremented
  again at completion. There is no separate "fulfilled" counter.
- **Auction orders never exceed 1** (§4).
- One order = one Product structurally: `orders` has a single `source_id`; a
  single `order_items` row is inserted per order
  (`order_creation_service.go:1186-1188`, `order_repository.go:850-851`).
  `orders.quantity` x `unit_price` CHECK = subtotal (000001:2486). A single
  order cannot span multiple Products or multiple selling surfaces.
- Reconciled with stage-5 rule: restoration resolves the surface via
  `orders.source_type + source_id`, never from `order_items.product_id`
  (`order_completion_service.go:1948-2013`). `product_id` is `products.id`
  for FPS, negotiation, and auction items (000045; integration proof
  `order_completion_restore_source_integration_test.go:29-36`).

Negotiation (chat purchase): always quantity 1. `GenerateForNegotiation`
mints `NewPricingTokenFromNegotiation(..., 1, ...)` and comments "Negotiation is
always for 1 item" (`pricing_token_service.go:946, 998, 1049`); chat handler
rejects any request quantity != 1 (`chat_handler.go:589-594`). Negotiation
domain itself stores no quantity (`negotiation/entity/negotiation_session.go`).
Note: the chat order path records `source_type = fixed_price_sale`
(`chat_handler.go:2096-2118` buildNegotiationCheckoutInput); the
`OrderSourceNegotiation = "negotiation"` enum (`order_source_type.go:20`)
is unused by that path.

---

## 6. STOCK LIFECYCLE (single-authority trace)

```
FPS.create           quantity := payload (default 1)                -> fixed_price_sales.quantity_available
FPS.update(PUT)      quantity reassignable only when 0 orders exist (handler guard)
                                                        else direct Set (no lock)
checkout token       gate: token.quantity <= QuantityAvailable      (read-only)
order create         GetForUpdate(FOR UPDATE) -> ReduceQuantity(n)  -> UpdateStock
                     auto sold at 0
order cancel/expire  GetForUpdate -> RestoreQuantity(sum items)     -> UpdateStock
                     sold -> active revival when qty > 0
order complete       no touch
order refund         no touch
auction order        no touch (no FPS row), only auction.order_id binding
moderation restore   withdrawn -> active, quantity untouched; sold rejected
sold-out             status=sold via ReduceQuantity at 0
relist/reuse         NEW fixed_price_sales row, fresh quantity, product untouched
```

Proven end-to-end by integration tests:
`quantity_persistence_test.go` (persist 5, 5→2 active, 2→0 sold, 0→1 active,
oversell rejected with row untouched, direct-edit persists),
`listing_stock_roundtrip_test.go` (qty1 and qty5 round-trips against real PG),
`order_completion_restore_source_integration_test.go` (restore resolves via
source_id).

---

## 7. REUSE / RELIST SEMANTICS

Reuse is implemented by attaching a **new sale-surface row** to an existing
`products.id` (`fixed_price_sale_repository_impl.go:42-56`; auction:
`auction_service.go:270-304`). Enforced by trigger
`enforce_single_active_sale_channel_per_product`
(000010_product_sale_channel_canonicalization.up.sql:76-104): only one active
surface per product; sold/ended/withdrawn surfaces are outside the guard.
Proven at runtime: `product_identity_reuse_integration_test.go` (sold FPS →
reused by FPS `:144-150`, reused by ended auction `:177`, active-vs-active
rejected `:186`, active auction after sale allowed `:189-196`).

Scenario results (what the code actually does):

- **Scenario 1**: P → FPS qty10, sells 3 (old row becomes qty 7, or 0 if sold
  out) → new FPS reusing P. New `fixed_price_sales` row with the **seller's
  freshly-declared quantity** (e.g. 10 again). Old row's numeric residue (7) is
  irrelevant to the new surface. **The 7 remaining units are silently reset.**
- **Scenario 2**: P → FPS qty10, sells 3 → reusing P for an Auction. Auction
  sells **one** unit (implicit 1). If P is one physical item, this is fine; but
  the prior FPS sold 3 units of that same "physical item", which Model B cannot
  reconcile. The auction's unit count is un-modeled.
- **Scenario 3**: P → Auction (ended/sold) → FPS qty10. Valid mechanically
  (trigger allows it); the FPS row declares 10 units of a Product that a prior
  auction sold as one physical item. Contradictory only if the seller intends
  the auction's unit and the FPS pool to be tracked as one inventory.
- **Scenario 4**: P → FPS qty10, 3 sold (7 left), surface withdrawn → reused.
  Remaining qty (7) sits on a **withdrawn, un-sellable** old row. The new
  surface carries its own quantity. Cancel of an order on the old surface
  calls `restoreListingStock` by `source_id` and pushes the released units
  **back into the withdrawn old surface** — units are restored into a surface
  that can never sell them.

Net effect: **quantity is a per-surface scalar, deliberately independent of the
Product, and relisting resets it.** There is no inventory authority spanning
surfaces. This is the structural contradiction with Model B for anything
quantity > 1.

---

## 8. CONCURRENCY / ATOMICITY EVIDENCE

FPS stock mutation is serialized by **row lock**, not by conditional UPDATE:

- `FixedPriceSaleRepository.GetForUpdate`
  (`fixed_price_sale_repository_impl.go:128-131`) issues
  `joinedSaleByIDQuery() + " FOR UPDATE OF fps, p"` — locks the FPS row AND the
  joined product row inside the caller's transaction.
- The only stock-decrement writer path (`CreateFromSaleSurface`,
  order_creation_service.go:1351-1511) runs in the handler's `WithTx`
  (order_handler.go:1024-1132) and does get-lock → entity-guard →
  update-commit. A second concurrent checkout blocks on the lock, then reads
  the fresh quantity/status and is rejected if exhausted (entity
  `ReduceQuantity` `InsufficientQuantityError`, fixed_price_sale.go:246-252).
- Restoration (`Cancel/Expire/CancelOverdue`) uses the same
  GetForUpdate + RestoreQuantity + UpdateStock pattern in one tx.
- Oversell rejection and row-unchanged-on-reject proven
  (`quantity_persistence_test.go:244-311`); persistence across the full cycle
  proven against real Postgres.
- DB guard is only `quantity_available >= 0` (000009:19-20); oversell
  prevention is **app-layer** (row lock + entity guard), not a DB CHECK.

Classification:
- **PROVEN** — decrement/restore atomicity and oversell rejection mechanism
  (row locks, entity guards, real-DB integration tests).
- **NOT PROVEN** — (a) no explicit parallel/racing test exists; (b) the
  seller edit path `UpdateFixedPriceSale` loads via `GetByID` (no lock) and its
  `CountAnyOrdersByProduct` guard is lock-free, so a concurrent quantity edit
  vs. checkout can theoretically race; (c) pricing-token stock gate
  (`pricing_token_service.go:246-247`) is a pre-check with no lock — it can
  produce a token for stock that is gone by order time (order create then fails
  safely at the locked gate).

No CONTRADICTED evidence found.

---

## 9. CONSUMER/PRODUCER MAP

| Role | Site | Field | Semantics |
|---|---|---|---|
| Producer | fixed_price_sale_handler.go:69,244-247 | create `quantity` | seller input, default 1 |
| Producer | fixed_price_sale_handler.go:418-438 | PUT quantity set | direct set, guarded (0 orders) |
| Producer | order_creation_service.go:1504-1511 | ReduceQuantity→UpdateStock | reservation decrement |
| Producer | order_completion_service.go:2017-2022 | RestoreQuantity→UpdateStock | cancel/expire restore |
| Producer | migrations/000009 | column + CHECK | schema authority |
| Producer (dead) | fixed_price_sale_service.go:553-594 | ReduceStock | zero callers |
| Reader buy-gate | order_creation_service.go:327-332 | QuantityAvailable compare | oversell gate |
| Reader buy-gate | pricing_token_service.go:246-247, 911-913 | quantity gate | token preview gate |
| Reader gate | commerce/shared/fixed_price_sale_viewer_capabilities.go:75-76 | qty>0 | availability capability |
| Reader gate | social/content ... content_resource_projection_resolver.go:205-206 | qty>0 | CanInteract |
| Reader display | fixed_price_sale_handler.go:1094; response_projection.go:120 | `"quantity"` wire | list/search/detail |
| Reader display | saved_item_repository_impl.go:202,233 | quantity_available | saved items |
| Reader display | chat/social live projections (chat_resource_projection.go:125; content_resource_projection.go:57, serverboot/chat_fixedprice_projection_resolver.go:252,269) | quantity_available | feed/chat payload |
| Reader aggregator | seller_monthly_metrics_worker.go:375-400 | SUM(orders.quantity) | completed==items sold |
| Reader (stale) | discovery/search/search_repository_impl.go:104 | comment claims qty>0 filter; SQL has none | latent gap |
| Mobile input | apps/mobile .../create_listing_screen.dart:48,118,417-418; edit_listing_screen.dart:73,132,181,381-382 | `_quantity` / stock | seller UI |
| Mobile wire | apps/mobile .../dto/listing_dto.dart:45,112,161 (read); :296,346,380,419 (write) | quantity | DTO |
| Mobile gate | listing.dart:222 `isAvailable => stock>0`; :236-239 `isSoldOut`; live_status_provider.dart:414; attachment_truth_resolver.dart:159-162 | stock | badges/gates |
| Mobile gate | get_listing_share_reference_usecase.dart:43 | stock<=0 rejects share | chat share |
| Mobile checkout | checkout_request.dart:24,55; checkout_screen_logic.dart:90,287; checkout_repository_impl.dart:94 | quantity hardcoded 1 | buyer always buys 1 |
| Mobile display | order_dto.dart, order_api_response_dtos.dart, order_items_card.dart:143 | item quantity | display |
| Mobile snapshot display | pricing_preview_dto.dart:52; pricing_breakdown.dart:83 | snapshot.quantity | display |
| Admin | apps/admin .../OrderDetailModal.tsx:273; types/orders.ts:90 | item quantity | display only |
| Auction | everywhere | none | implicit 1, no field |
| Order DTO | order/.../dto/decision.go:284,387; admin_order_handler.go:224,474-483 | order/item quantity | read/display |

---

## 10. CONTRADICTIONS

Every item with exact source evidence:

- **C1 — Product = one physical item vs FPS quantity > 1.**
  Product is documented as "the internal physical item authority"
  (product/entity/product.go:9); migration 000009 documents multi-quantity FPS
  as "sellers with multiple units of the same product" (000009:10-12). Same
  object, two opposing identities. The reuse runtime proof literally attaches a
  qty=2 FPS ("Koi-Third") to a Product previously sold as a unit
  (product_identity_reuse_integration_test.go:144-150, :137).
- **C2 — Reuse after partial sale has no remaining-unit semantics.**
  Scenario results §7: remaining pool (e.g. 7 of 10) is stranded on the old
  surface while the new surface silently resets to a fresh declared quantity.
  No carry-over, no ledger, no warning.
- **C3 — quantity lives on FPS, but docs/UI treat Product as the inventory
  subject.** Mobile labels `stock` and 'Habis' at Product/listing detail
  (`listing_detail_screen.dart:835`, `isSoldOut` listing.dart:236-239),
  while the value is `fixed_price_sales.quantity_available`. On reuse, the
  "stock" shown for the same product_id jumps to the new surface's declared
  number with no historical continuity.
- **C4 — Auction implicitly 1, FPS N — no unified quantity model.**
  No auction quantity field exists (auctions schema 000001:477-498); every
  auction path hardcodes 1 (§4). There is no shared notion of "units" between
  surfaces.
- **C5 — order quantity is a reservation, not fulfillment.** Decrement happens
  at order creation (order_creation_service.go:1504-1511); completion never
  touches stock; refunds never restore. A buyer-requested quantity (5) is
  reserved at checkout and only returned on cancel/expire. "sold quantity"
  accounting (seller_monthly_metrics_worker.go:375-400) counts completed
  `orders.quantity` — the same number that is already carved out of the pool —
  while the pool itself has no "sold" latch tally. Two different meanings of
  quantity (pool `quantity_available` vs summed `orders.quantity`) are not
  reconciled by any check.
- **C6 — cancellation/refund restores a different quantity than reserved in the
  withdrawn-relist case.** Scenario 4: cancel after withdrawal restores into a
  withdrawn, unwillable surface (order_completion_service.go:2006-2024).
- **C7 — quantity silently reset on relisting.** FPS Create reuse mints a fresh
  row (fixed_price_sale_repository_impl.go:42-56); no copy from prior surface.
- **C8 — no inventory authority exists while business rules depend on one.**
  Migration 000009's owner decision names a real "multi-quantity" feature
  (000009:10-12); catalog UI renders it as stock; but no Product or ledger row
  can answer "how many units remain for this product across surfaces".
- **C9 — mobile UI doubles down on quantity-1 even for multi-unit FPS.** The
  backend supports FPS orders of quantity > 1 end-to-end (token quantity,
  orders.quantity, order_items.quantity), but mobile checkout hardcodes
  `quantity: 1` (checkout_screen_logic.dart:90,287;
  checkout_repository_impl.dart:94) while the listing UI shows the multi-unit
  `stock` and sold-out states — a writer/reader mismatch that the "stock" number
  masks.
- **C10 — search claims out-of-stock filtering; SQL has none.**
  discovery/search/search_repository_impl.go:104 comment "INVENTORY TRUTH:
  Filters out out-of-stock items (quantity_available > 0)"; actual queries at
  :138-162/:253-260 filter only `status='active'`. Latent: a status/qty
  divergence (reachable via direct status edit, handler.go:458-495 allows
  setting `sold` while quantity stays > 0) would surface unwillable rows.
- **C11 — status and quantity can diverge via the edit path.** Update allows
  `req.Status` to force `sold` while retaining nonzero `quantity_available`
  (handler.go:458-495); moderation restore refuses `sold` (fixed_price_sale.go:199-201),
  so such a row is terminal with phantom stock.

---

## 11. PROVEN-GOOD AREAS

- Quantity persistence is real and versioned (000009), with real-DB proofs.
- Reservation/decrement and cancel/expire restore are row-lock serialized and
  entity-guarded; oversell is rejected before any write and leaves the row
  untouched (`quantity_persistence_test.go:244-311`).
- Round-trip qty=1 and qty=5 cycles persist through `UpdateStock`
  (`listing_stock_roundtrip_test.go`).
- Restoration resolves the correct surface exclusively through
  `order.source_type + source_id` (stage-5 rule) and sums `order_items.quantity`
  exactly (`order_creation_service` vs `order_completion_service:1948-2024`;
  integration proof `order_completion_restore_source_integration_test.go`).
- Auction settlement is single-unit and double-settlement-safe wholesale
  (OrderID guard + pricing-token uniqueness).
- Product has no duplicated lifecycle mirror; availability derives purely from
  the active surface (000044).
- Order create takes quantity from the validated pricing token, never from
  client input (order_handler.go:1035,1058).
- FPS edit freezes price/title/quantity once any order exists on the Product
  (handler.go:404-425), protecting order snapshots.

## 12. NOT-PROVEN AREAS (with concurrency §8)

- No parallel/racing oversell test (both decrement and restore concurrency
  claims rest on mechanism, not on a race test).
- Seller quantity-edit path is not FOR UPDATE-locked and its order-count guard
  is un-locked (handler.go:384-425); a concurrent edit-vs-checkout race is
  unverified.
- "No oversell" has no DB CHECK enforcing `quantity_available >= requested`;
  only `>= 0` (000009:19-20) and the app-lock hold it together.
- Search/discovery out-of-stock exclusion: documented, not implemented (C10).
- Whether multi-unit FPS orders are actually exercised — mobile hardcodes
  quantity 1 (C9); no test exercises a multi-unit order end-to-end through the
  handler with quantity > 1.
- Post-refund stock reconciliation — refunds never restore, and nothing
  verifies pool vs orders-remaining consistency.

---

## 13. OWNER DECISIONS REQUIRED

### DECISION 1 (the central question). Is quantity a property of the **Product**
### or of each **selling surface**?

- **Current behavior:** Surface-local scalar on `fixed_price_sales`
  (`quantity_available`). Product has none. Reuse resets it. (evidence: §2, §3,
  §7)
- **Possible models:**
  - A) Keep surface-local (today). Product stays "physical item" (or becomes a
    catalog concept later); each surface is self-contained.
  - B) Product-level ledger: `products.quantity_available` (or an inventory
    table) as the single authority; surfaces read/write against it. Requires a
    per-unit or per-claim ledger record and reuse/restore routing by product
    not surface.
  - C) Full per-unit identity (serialized instances) — heaviest; matches the
    koi uniqueness narrative (each fish distinct).
- **Consequence A:** minimal change, but C1/C2/C8 persist (relist
  resets stock; "physical item" remains contradicted by qty > 1).
- **Consequence B:** makes Product the inventory authority (contradicts current
  "sale-surface-agnostic physical item" doc, product/entity/product.go:9-10);
  requires the Surface-Quantity Authority decision D3 to define how surfaces
  draw from the pool.
- **Consequence C:** aligns with physical-item identity but is a large schema
  + flow change with no production-data constraint.
- **Recommendation (not fact):** Model **B with a product-level ledger** if
  multi-unit fungible selling is intended (koi batches), because it is the only
  model where reuse of a Product after partial sale yields correct remaining
  stock. If every listing with quantity > 1 is actually N distinct fish, choose
  **C** and treat quantity > 1 as a seller shorthand to be exploded into per-unit
  inventory.

### DECISION 2. What does `quantity > 1` on an FPS **mean**?

- **Current:** a fungible pool of units of the same Product row — `ReduceQuantity`
  carves n units, `orders.quantity` can be n, restoration returns n.
- **Candidate meanings:** (A) batch/lot of fungible identical units;
  (B) reusable catalog-like physical product (restockable); (C) multiple
  identical physical items tracked as one sku; (D) other (e.g. service
  quantity).
- **Consequence:** A→keep FPS pool + product ledger per DECISION 1-B. C→explode
  to per-unit (DECISION 1-C). B→needs explicit restock semantics (today the
  `PUT` quantity edit is the only restock lever and it is blocked once orders
  exist, handler.go:418-424).
- **Recommendation (not fact):** Adopt **A (fungible batch/lot)** to reconcile
  quantity with Model B reuse: the Product is the batch identity; a surface is
  a sale offer over the batch pool; the ledger owns remaining units.

### DECISION 3. Reuse after partial sale: carry-over, reset, or block?

- **Current:** reset (new surface, fresh quantity; §7 all scenarios).
- **Possible:** (a) block reuse while prior quantity > 0; (b) carry remaining
  units into the new surface; (c) treat surfaces as fully decoupled (today).
- **Consequence:** (a) safest for physical-item identity but blocks legitimate
  relists; (b) requires a product-level ledger (DECISION 1-B) and a definition
  of whether cancelled orders on the old surface return to the old or new
  surface; (c) keeps C2 latent.
- **Recommendation (not fact):** (b) via a product-level ledger, with restore
  routed by product and surfaced visibility of remaining units at relist time.

### DECISION 4. Should mobile checkout allow ordering **quantity > 1** on FPS?

- **Current:** backend supports it; mobile hardcodes 1 (C9).
- **Possible:** enable multi-unit cart/checkout, or keep 1 (and then the
  multi-unit FPS is effectively single-sale; quantity acts as a
  "sell N times one-at-a-time" pool).
- **Consequence:** enabling requires checkout quantity wiring + UI; keeping 1
  means the pool only ever drains one unit per order, which changes the
  meaning of "sold out" and the metrics worker's SUM(quantity).
- **Recommendation (not fact):** Keep checkout quantity 1 for now (smallest
  change); revisit with DECISION 1/2, since multi-unit ordering is moot unless
  the ledger model lands.

Each decision must be made jointly: DECISION 1's answer determines the 
plumbing for 2, 3, and 4.

---

## 14. RECOMMENDED NEXT-STAGE SEQUENCE

If approved, order of execution (each a distinct stage, no overlap):

1. **Stage 7 — Decision lock / semantic contract doc.** Record owner answers to
   DECISION 1-4 as ADR; freeze the quantity vocabulary (units, pool, ledger) and
   the handling of reuse-carry-over.
2. **Stage 8 — Schema + migration for the chosen model.** If ledger model
   chosen: `inventory`/`product_inventory_ledger` table + product-level
   quantity authority + DB CHECK on pool bounds; migration shims
   `fixed_price_sales.quantity_available` as projection or removes it.
3. **Stage 9 — Writer/handler convergence.** Route all decrement/restore
   through the new ledger; retire `FixedPriceSaleService.ReduceStock` (dead);
   lock the seller quantity-edit path; remove the search/documentation
   contradiction (C10).
4. **Stage 10 — Concurrency proof.** Add a true parallel race test for
   decrement and restore against real Postgres (two txns, one pool).
5. **Stage 11 — Read-DTO / UI reconciliation.** Align mobile `stock`/`Habis`
   with the ledger projection; resolve C9; update seller monthly metrics to the
   tally authority.

STOP — stage complete; no further implementation performed in this stage.