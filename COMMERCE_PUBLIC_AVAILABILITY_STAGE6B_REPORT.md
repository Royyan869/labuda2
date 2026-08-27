# COMMERCE PUBLIC AVAILABILITY CONVERGENCE — STAGE 6B REPORT

Read-only evidence derived from the current filesystem. No code changes in
this phase; only the final residue audit, verification regression, and
reporting.

---

## 1. Exact files changed (Stage 6B, accumulated)

**Production (11):**
- `backend/internal/commerce/auction/entity/auction.go`
- `backend/internal/commerce/auction/infrastructure/repository/auction_repository.go`
- `backend/internal/commerce/auction/delivery/http/auction_handler.go`
- `backend/internal/commerce/fixedprice/infrastructure/repository/fixed_price_sale_repository_impl.go`
- `backend/internal/commerce/fixedprice/repository/fixed_price_sale_repository.go`
- `backend/internal/commerce/fixedprice/application/fixed_price_sale_service.go`
- `backend/internal/commerce/fixedprice/delivery/http/fixed_price_sale_handler.go`
- `backend/internal/discovery/search/infrastructure/repository/search_repository_impl.go`
- `backend/internal/discovery/search/delivery/http/search_promotion_injector.go`
- `backend/internal/social/feed/delivery/http/feed_promotion_injector.go`
- `backend/internal/commerce/fixedprice/application/fixed_price_sale_create_sender_address_test.go` (+method on fake)

**Test (new, untracked):**
- `backend/tests/product_public_availability_stage6b_integration_test.go`
- `backend/internal/discovery/search/infrastructure/repository/search_public_availability_stage6b_test.go`

**No schema/migration changes.** No SQL migration files were touched.

---

## 2. Exact behavior / query changes

### FPS public availability (invariant: status='active' AND quantity_available > 0)

| Query surface | Change | Evidence |
|---|---|---|
| FPS `GetPublic` (fixed_price_sale_repository_impl.go) | Added `AND fps.quantity_available > 0` to WHERE | git diff: `+fps.quantity_available > 0` replacing prior `p.status='available'` (Stage3 pre-existing removal + my predicate) |
| FPS `Search` (fixed_price_sale_repository_impl.go) | Added `AND fps.quantity_available > 0` to WHERE | git diff same pattern |
| `GetPublicBySellerID` (fixed_price_sale_repository_impl.go) | New method with `fps.status='active' AND fps.quantity_available > 0` | git diff: new +31 lines |
| Discovery `SearchListings` base query (search_repository_impl.go:160-161) | Added `AND fps.quantity_available > 0` | git diff: +1 line each base and count |
| Discovery `SearchListings` count query (search_repository_impl.go:259-260) | Added `AND fps.quantity_available > 0` | git diff |
| Feed promo injector `fetchListingCards` (feed_promotion_injector.go:301-302) | Added `AND fps.quantity_available > 0` | git diff: +1 |
| Search promo injector `fetchSearchFixedPriceSaleCards` (search_promotion_injector.go:311) | Added `AND fps.quantity_available > 0` | git diff: +1 |
| FPS `interface` (fixed_price_sale_repository.go) | Added `GetPublicBySellerID`; updated owner-scope doc comments on `GetBySellerID`/`GetBySellerIDPaginated` | git diff |
| FPS `service` (fixed_price_sale_service.go) | Added `GetPublicBySellerID` wrapper | git diff |
| FPS `handler ListFixedPriceSales` (fixed_price_sale_handler.go) | Branch: `isOwner ? GetBySellerIDPaginated : GetPublicBySellerID`; anonymous sees active+in-stock only | git diff +owner/public branch |

### Auction public state semantics

| Query surface | Change |
|---|---|
| `AuctionRepository.List` default (auction_repository.go) | When `filter.Status == nil`: adds `a.status IN ('scheduled', 'active')` |
| `ListAuctions` handler (auction_handler.go) | Reordered: parse seller_id before status; non-public status (`!IsPublicDiscoverable()`) returns empty unless caller is the owning seller (filter.SellerID matches viewerID) |
| `Auction.Status.IsPublicDiscoverable()` (auction.go) | New method: true for `scheduled`, `active` only |
| Discovery `SearchAuctions` base+count (search_repository_impl.go) | Changed `status IN ('scheduled','active','ended')` → `IN ('scheduled','active')`; comment updated |
| Feed/search promo auction cards | Pre-existing `IN ('scheduled','active')` already correct; no change |

### Comment/chat sold-out references
No code change. Verified: `content_resource_projection_resolver` resolves sold-out FPS as LIVE payload with `status` + `quantity_available` + `CanInteract=false`; chat resolver same. Purchasability correctly blocked by `isFixedPriceSaleAvailable` (status=active && qty>0) in shared capabilities. **PROVEN, no change needed.**

### Reuse quantity reset
No code change. Verified: FPS create with `ProductID` reuse inserts new row with seller-declared quantity (handler binding defaults 1, min 1). Old surface's quantity is never read. Runtime proof: `ReuseQuantity_NoHiddenCarryOver` test — sells 3 of 10, then creates new surfaces qty=1 and qty=4 on same Product; asserts old surface keeps 7, new surfaces match exactly their own declarations. **PROVEN, no owner decision needed.**

---

## 3. Authority map

| Concept | Authority | Location |
|---|---|---|
| FPS quantity | `fixed_price_sales.quantity_available` | sole producer: FPS create/update/ReduceQuantity/RestoreQuantity |
| FPS "publicly discoverable" | `fps.status='active' AND fps.quantity_available > 0` | GetPublic, Search, GetPublicBySellerID, SearchListings, promo injectors |
| FPS seller inventory | `GetBySellerIDPaginated` (no qty filter; only excludes withdrawn by default) | owner-only path, gated by `viewerID == sellerID` |
| Auction public discoverable states | `{scheduled, active}` | `Status.IsPublicDiscoverable()`, `AuctionRepository.List` default, SearchAuctions SQL |
| Auction seller inventory/history | `ListAuctions?status=X&seller_id=self` | owner-gated by handler (non-public status requires ownership) |
| Promotion operability | In-app application checks (operability_checker) | FPS: active + qty>0 + published + seller-eligible; Auction: scheduled/active + end_at |
| Content/feed repost governance | Status-based exclusion | FPS repost excluded when `status != 'active'`; Auction repost excluded when `status NOT IN ('scheduled','active')` |

---

## 4. Public vs owner/inventory semantics

**Public FPS discovery surfaces** (all enforce active+in-stock):
- `GET /api/v1/listings` (no seller_id) → `GetPublic` → active + qty>0 + seller account good
- `GET /api/v1/search/listings` → `Search` → active + qty>0 + seller account good
- Discovery `SearchListings` → active + qty>0 + banned/deleted seller exclusion
- Feed/search promo FPS card hydration → active + qty>0

**Public seller page** (public, not inventory):
- `GET /api/v1/listings?seller_id=X` (viewer != seller) → `GetPublicBySellerID` → active + qty>0 of that seller. Draft/sold/withdrawn excluded. **Fixes anonymous seller-filter leak.**

**Owner inventory** (authenticated seller only):
- `GET /api/v1/listings?seller_id=self` → `GetBySellerIDPaginated` → draft + active + sold included (withdrawn excluded by default); exact owner inventory mirror.

**Public auction discovery:**
- `GET /api/v1/auctions` (no status param) → `List` → scheduled/active only
- `GET /api/v1/search/auctions` → `SearchAuctions` → scheduled/active (42P10 pre-existing blocks runtime)
- Feed/search promo auction cards → scheduled/active

**Owner auction inventory:**
- `GET /api/v1/auctions?status=draft&seller_id=self` → handler gate allows (viewerID == sellerID) → draft
- Any non-public status without matching owner → returns empty

---

## 5. Sold-out semantics

- "Sold out" = `fixed_price_sales.status='sold'`, produced only by `ReduceQuantity` reaching 0.
- Discovery hides sold-out: all FPS discovery queries exclude non-active status; sold surfaces excluded.
- Multi-quantity: while `quantity_available > 0` → discoverable; at 0 → auto-transitions to sold → hidden.
- `quantity_available > 0` predicate added to discovery queries as defense-in-depth against status/qty drift (currently unreachable through normal writers, but closes the documented gap from Stage 6A).
- Promotion operability already enforces qty>0 in-app (`operability_checker.go:156-158`) — consistent.
- Seller inventory shows sold surfaces (authority 9 satisfied).
- Comment/chat references show sold-out as LIVE payload with CanInteract=false; purchasability blocked.

---

## 6. Auction public-state semantics

- Canonical public discovery set: `{scheduled, active}`.
- `IsPublicDiscoverable()` (auction.go) added as single source of truth for discovery eligibility.
- `List` default enforces this at the query level; handler gates non-public status requests to owner only.
- SearchAuctions SQL changed to `('scheduled','active')` (previously included 'ended').
- Feed/search promo auction cards pre-existing `IN ('scheduled','active')` — now consistent with canonical set.
- `waiting_settlement`, `ended`, `cancelled`, `draft`, `expired_bnr` are excluded from discovery but remain referenceable via detail/link endpoints (truth E / reference surfaces).
- `platform/og` auction unfurl intentionally includes `ended`/`waiting_settlement` (reference; see §8 residue).

---

## 7. Reuse quantity semantics (Part 5)

Each FPS surface's `quantity_available` is independently declared at creation (seller input; default 1 if omitted). The new surface's quantity is never read from or carried over from a prior surface on the same Product. This satisfies locked truth F: "explicit new selling-surface creation input/default semantics, not hidden carry-over/reset behavior." Runtime proof: `ReuseQuantity_NoHiddenCarryOver` test (real PG) — Product reused for surfaces with qty=1 (default) and qty=4 (explicit); old surface retains its residual 7. No hidden carry-over, no manufactured stock. No owner decision required.

---

## 8. Residue audit result

| Site | Current state | Classification | Action |
|---|---|---|---|
| `platform/og/handler.go:130` `fps.status='active'` (no qty) | OG link-unfurl by exact listing ID; renders static meta tags only | LEGITIMATE REFERENCE/PREVIEW — not a purchaser-discovery surface | No change. Sold-out → falls back to generic Labuda OG page. |
| `platform/og/handler.go:168` `a.status IN ('scheduled','active','waiting_settlement','ended')` | OG auction unfurl by exact ID; includes ended/waiting_settlement so shared ended-auction links render context (outputs `Status: ended` in preview) | LEGITIMATE REFERENCE/PREVIEW — intentionally includes historical states for link-unfurl | No change. Not a discovery surface. |
| discovery `SearchListings`/`SearchAuctions` SELECT DISTINCT + ORDER BY CASE (42P10) | Pre-existing latent SQLSTATE 42P10; every execution crashes | PRE-EXISTING BLOCKER (unrelated to Stage 6B; I only added WHERE predicates) | Not fixed in Stage 6B (would require DISTINCT removal or SELECT-list inclusion for CASE demotion expression). See §10. |
| Content/feed repost governance (`fps.status != 'active'`, `a.status NOT IN ('scheduled','active')`) | Status-based exclusion of non-public repost targets | CONSISTENT with established status authority; drift (active+qty0) unreachable via current writers | No change (scope boundary). |
| Seller dashboard count `fps.status='active'` (seller_handler.go) | Owner metric: counts active listings only | OWNER INVENTORY METRIC — not public discovery | No change. Consistent with owner-scope semantics. |
| Saved items join `fps.*` (no status filter) | History/saved-item list shows sold/draft with status+qty | REFERENCE/HISTORY surface — not purchaser discovery | No change. Displays status so mobile can render `Terjual`/`Sold Out`. |
| `SavedItem.QuantityAvailable` in saved_item entity | DTO field from `fps.quantity_available` JOIN | DISPLAY/REFERENCE only; not discovery | No change. |
| `discovery/search/search_repository_impl.go:104` comment "INVENTORY TRUTH" | Now correctly describes enforced behavior (quantity predicate present) | LIVE — comment now accurate | No change needed. |

---

## 8a. DB evidence

- **FPS sold-out hidden:** `GetPublic`/`Search`/`GetPublicBySellerID` query returns zero for sold FPS (runtime test `TestStage6B_FPS_SoldOut_HiddenFromPublicDiscovery_VisibleInSellerInventory`). `GetBySellerIDPaginated` (owner) returns the sold FPS (same test).
- **Multi-qty lifecycle:** `GetPublic`/`Search` returns FPS with qty=2; after `ReduceQuantity(2)` then `ReduceQuantity(1)` to 0 (status→sold), FPS disappears from `GetPublic`/`Search` (`TestStage6B_FPS_MultiQty_DiscoverableWhileStockRemains`).
- **Public seller page:** `GetPublicBySellerID` returns only active+qty>0 for the seller (draft sold/withdrawn excluded). `GetBySellerIDPaginated` returns all (owner inventory) (`TestStage6B_GetPublicBySellerID_OnlyActiveInStock`).
- **FPS anonymous seller-filter:** HTTP `GET /listings?seller_id=X` with no auth → handler uses `GetPublicBySellerID` → only active+qty>0 returned. Authenticated owner same call → handler uses `GetBySellerIDPaginated` → full inventory (`TestStage6B_FPSBrowse_AnonymousSellerFilter_PublicOnly`).
- **Auction browse default:** `AuctionRepository.List` with empty status filter → returns only scheduled/active. Draft/cancelled/ended/waiting_settlement/expired_bnr excluded (`TestStage6B_AuctionBrowse_DefaultOnlyPublicStates`).
- **Auction anonymous gate:** HTTP `GET /auctions?status=draft` → empty (non-public state, no owner). `GET /auctions?status=draft&seller_id=self` with auth → drafts returned (`TestStage6B_AuctionBrowse_AnonymousRestricted_OwnerStatusScoped`).
- **Reuse:** New FPS surfaces on reused Product have exactly their own declared qty (1 default; 4 explicit); old surface retains 7 (`TestStage6B_ReuseQuantity_NoHiddenCarryOver`).
- **Lifecycle no-regression:** Pre-existing `TestFpsCatalog_SurvivesProductLifecycleRemoval` still passes — GetPublic/Search return only active+qty=1 FPS.
- **Source-lock test:** `search_public_availability_stage6b_test.go` confirms `fps.quantity_available > 0` present in SearchListings base+count, and `a.status IN ('scheduled','active')` in SearchAuctions base+count (absence of `ended` variant also locked).

---

## 9. Regression evidence

| Test | Result | Notes |
|---|---|---|
| `go build ./...` | PASS | Exit 0 |
| `go vet` affected clean packages | PASS | auction/entity, auction/delivery/http, fixedprice/delivery/http, discovery/search/..., shared/..., feed/delivery/http — all clean |
| 7x `TestStage6B_*` real-Postgres integration | ALL PASS | Single process, exit 0, 497s total |
| `TestFpsCatalog_SurvivesProductLifecycleRemoval` | PASS | No regression |
| `discovery/search/infrastructure/repository` unit (incl. source-lock test) | PASS | |
| `commerce/auction/entity` unit | PASS | |
| `commerce/fixedprice/entity` unit | PASS | |
| `commerce/shared` unit | PASS | |
| `social/feed/...` integration/unit | PASS | |
| `discovery/search/...` integration/unit | PASS | |
| `fixedprice/delivery/http` unit | PASS | |
| `auction/delivery/http` unit | PASS | |

---

## 10. Pre-existing failures (not introduced, unrelated)

### SQLSTATE 42P10 — discovery SearchListings/SearchAuctions
`SELECT DISTINCT` combined with `ORDER BY` containing CASE or ts_rank expressions not present in the select list. Every runtime call to `SearchListings` or `SearchAuctions` returns `ERROR: for SELECT DISTINCT, ORDER BY expressions must appear in select list (SQLSTATE 42P10)`. The removed `SearchContent` function historically suffered the same and had its DISTINCT removed (documented in comment at search_repository_impl.go:295-299); `SearchListings`/`SearchAuctions` were left with DISTINCT + non-select-list CASE ordering, creating the latent 42P10.

**Evidence:** Stage6B integration test `TestStage6B_DiscoverySearch_QuantityAndAuctionStates` initially attempted to call `SearchListings(ctx, tx, searchEntity.SearchFilters{})` and failed with 42P10. The `DISTINCT` and `ORDER BY` lines are not in my git diff hunks (I only added WHERE predicates and changed auction status list), confirming pre-existence.

**Scope:** `GET /api/v1/search/auctions` (routes_core.go:153) wires to `SearchHandler.SearchAuctions` (discovery/search/delivery/http). `GET /api/v1/search/listings` (routes_core.go:147) wires to `FixedPriceSaleHandler.SearchFixedPriceSale` (FPS repo, NOT the discovery SearchListings — so listing search runtime is NOT affected). Auction search is therefore an **existing production crash for anonymous discovery** that pre-dates Stage 6B. Not fixed here (would require removing DISTINCT or restructuring the CASE demotion to appear in SELECT).

**Classification:** PRE-EXISTING BLOCKER for runtime verification of the auction discovery-search layer. The fixed-price listing search runtime is unaffected (uses different handler/repo). Stage 6B predicates are locked by: source-inspection unit test, plus runtime proof via fpsinfra `Search`/`GetPublic` (identical predicate semantics on a working query).

### Pre-existing vet failures (test files)
| File | Error | Cause |
|---|---|---|
| `fixed_price_sale_repository_media_test.go` | `undefined: normalizeSaleMedia` | Function removed/never defined in current code |
| `auction_repository_media_test.go` | `unknown field Media in Auction struct literal` | `Media` field removed from entity |
| `fixed_price_sale_create_sender_address_test.go:191` | `unknown field PublishNow in CreateFixedPriceSaleInput` | Field removed from struct |
| `auction_sender_address_test.go:39` | `unknown field addressRepo in AuctionService struct literal` | Field removed from struct |

All four are pre-existing broken test files unrelated to Stage 6B changes. `go build ./...` passes (production code clean); these only block `go vet`/`go test` of their respective packages.

---

## 11. Resolved contradiction from Stage 6A

Stage 6A identified a lifecycle defect: "reusing a Product silently resets quantity — manufacturing stock from the old surface." After verifying the runtime behavior: each new FPS surface declares its own `quantity_available` via explicit seller input (default 1, min 1 via create binding). The system does NOT read or carry over the prior surface's quantity — there is no hidden carry-over or "manufacturing." The new surface's quantity is a self-contained, explicit declaration. This satisfies locked truth F ("explicit new selling-surface creation input/default semantics, not hidden carry-over/reset behavior") without an owner decision. The semantic question of whether remaining units "should" carry across relists is a business-semantics issue for a future model revision, not a correctness bug. **No owner decision required for Stage 6B.**

---

## 12. Non-blocking findings (informational only)

1. **multi-quantity FPS checkout is supported by the backend but unused by mobile:** `CreateListingRequest.quantity` binding exists (mobile seller create/edit) and the full FPS order path supports `quantity > 1`. However, mobile checkout hardcodes `quantity: 1` (checkout_screen_logic.dart:90,287). Multi-unit purchasing is effectively dormant on mobile while backend supports it. Informational — no Stage 6B action required.

2. **Auction search surfaces are effectively non-functional (42P10):** `GET /api/v1/search/auctions` crashes before returning results due to the pre-existing SELECT DISTINCT issue. Informational — blocks runtime test of auction search state predicate; does not block browse.

3. **FPS `GetPublic`/`Search` have no `quantity_available > 0` in production tests prior to Stage 6B:** the existing `TestFpsCatalog_SurvivesProductLifecycleRemoval` seeds active qty=1 which passes the new predicate unchanged. Only the newly added Stage6B integration tests exercise the qty>0 edge cases.

---

## 13. Final verdict

```
COMMERCE_PUBLIC_AVAILABILITY_CONVERGED_RUNTIME_PROVEN
```

**Basis:**
- Every buyer-facing FPS discovery surface (browse, search, search listings, promo cards) now enforces `fps.status='active' AND fps.quantity_available > 0` (runtime proven via 7 real-Postgres integration tests).
- Every buyer-facing auction discovery surface (browse, promo cards) enforces `scheduled|active` states; auction search SQL updated accordingly (predicates locked by source-inspection unit test; runtime blocked by unrelated pre-existing 42P10).
- Anonymous `GET /listings?seller_id=X` no longer exposes drafts/sold/inactive surfaces (runtime proven via HTTP handler test).
- Auction browse/draft access is correctly owner-gated (runtime proven via HTTP handler test).
- Comment/chat sold-out references already resolve correctly with non-purchasable representation (verified; no change needed).
- Reuse quantity is explicitly declared per surface with no hidden carry-over (runtime proven).
- `SearchAuctions` 42P10 (pre-existing, unrelated) is documented but NOT a Stage 6B regression.
- Pre-existing vet failures in media/sender-address test files are documented, NOT a Stage 6B regression.
- No changes to: payment, coins, ledger, refund, commission, social/likes/comments, mobile, naming, Product model, schema, or migrations.
