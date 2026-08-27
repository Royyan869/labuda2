# STAGE 2H — LISTING HARDCODED ROUTE RESIDUE AUDIT

Mode: READ-ONLY. No production/test/schema edits in this stage.

Source list: STAGE 2G hardcoded `/listing...` cluster (10 files only).

## VERDICT

**AUDIT COMPLETE**

Every listed occurrence is a **stale in-app Listing path** except: (a) `share_target.dart` public URL (**E / OWNER DECISION**), (b) `app_router.dart` unauthenticated **allowlist prefix** (not a registered route), (c) enum/field/copy named “listing” that is **not** a route. Canonical mobile destinations already exist: `RoutePaths.forSales`, `forSaleDetail`, `createForSale`, `editForSale`. None of the ten files currently uses those symbols for the stale paths (except auction sibling on home preview).

---

## 1. EXACT RESIDUE TABLE

| # | File | Line | Exact path | Caller / consumer | ID / entity | Intent valid? | Canonical target | Class | Action |
|---|------|------|------------|-------------------|-------------|---------------|------------------|-------|--------|
| 1 | `commerce_preview_section.dart` | 147 | `'/listing/${item.listing!.forSaleId}'` | `_navigateToDetail` via `context.push`; item is `ForSale? listing` | `ForSale.forSaleId` | Yes — open fixed-price product | `RoutePaths.forSaleDetail` → `/for-sale/:fixedPriceSaleId` | **A** | **CONVERGE** |
| 2 | `feed_renderers.dart` | 647 | `'/listing/$fixedPriceSaleId'` | `PromotedListingCard` `InkWell.onTap` `context.push` | `additionalData['fixedPriceSaleId']` | Yes — promoted FPS detail | same | **A** | **CONVERGE** |
| 3 | `search_results_screen.dart` | 316 | `'/listing/${result.id}'` | `SearchResultType.listing` branch; `GoRouter.go` | `result.id` (search listing/FPS id) | Yes — search hit → product | `navigateToForSaleDetail(result.id)` or `/for-sale/${result.id}` | **A** | **CONVERGE** |
| 4 | `discussion_screen.dart` | 226 | `'/listing/$fixedPriceSaleId'` | `onFixedPriceSaleTap` `context.push` | comment-attached FPS id | Yes | `RoutePaths.forSaleDetail` | **A** | **CONVERGE** |
| 5 | `notification_navigation_handler.dart` | 364 | `'/listing/$targetId'` | `case 'listing'` after home; `GoRouter.push` | notification `targetId` | Yes if id is FPS/ForSale | `navigateToForSaleDetail(targetId)` | **A** | **CONVERGE** (keep `case 'listing'` wire) |
| 6a | `my_for_sales_screen.dart` | 234 | `'/listing/$forSaleId'` | `_viewForSaleDetail` `Navigator.pushNamed` | `ForSale` id | Yes — seller view own FPS | `RoutePaths.forSaleDetail` | **A** | **CONVERGE** |
| 6b | `my_for_sales_screen.dart` | 238 | `'/listing/${listing.forSaleId}/edit'` | `_editForSale` `pushNamed` | `ForSale.forSaleId` | Yes — edit | `RoutePaths.editForSale` = `/for-sale/:fixedPriceSaleId/edit` — **not** currently used | **A** | **CONVERGE** |
| 6c | `my_for_sales_screen.dart` | 242 | `'/create/listing'` | `_createNewForSale` `pushNamed` | none (create) | Yes — create FPS | `RoutePaths.createForSale` = `/create/for-sale` | **A** | **CONVERGE** |
| 6d | `my_for_sales_screen.dart` | 331 | `'/listing/${listing.forSaleId}/edit'` | shipping-gate dialog “Edit Listing” `pushNamed` | same | Yes | `RoutePaths.editForSale` | **A** | **CONVERGE** |
| 7 | `content_resource_projection.dart` | 586 | `'/listing/$resourceId'` | getter `canonicalPath` for `fixedPriceSale` | `resourceId` | Yes — in-app path | `/for-sale/$resourceId` | **A** | **CONVERGE** |
| 8 | `welcome_screen.dart` | 194 | `'/listings'` | guest home `IconButton` `context.go` | catalog, no id | Yes — guest browse catalog | `RoutePaths.forSales` = `/for-sale` | **A** | **CONVERGE** |
| 9 | `app_router.dart` | 201 | `'/listing'` (prefix in `publicBrowsePrefixes`) | unauthenticated redirect: skip bounce to `/welcome` if path is `/listing` or `/listing/...` | n/a | Allowlist only — **no GoRoute** for `/listing` | If in-app paths converge to `/for-sale`, prefix should be `'/for-sale'` (or drop `/listing`) | **D** | **CONVERGE** prefix after/with in-app paths; not a registered route |
| 10 | `share_target.dart` | 52 | `'$base/listing/$id'` | `generatePublicShareUrl` / `ExternalShareType.listing` | share target id | External/web URL, not GoRouter | **Do not assume** `/for-sale/...` | **E** | **OWNER DECISION** |

Non-route in file 9 (not a path): `app_router.dart:554` log `'navigateToCheckout requires a listing context; no bare route emitted'` — **C**, **KEEP**.

---

## 2. CLASSIFICATION PER OCCURRENCE

- **A (stale Listing route authority):** #1–#8, #6a–#6d. In-app navigation to a **non-registered** `/listing...` path while ForSale module owns `/for-sale...`.
- **B:** none of the ten path strings are already canonical.
- **C:** `app_router.dart:554` wording; local enums/fields named `listing` (`_CommercePreviewType.listing`, `SearchResultType.listing`, `ExternalShareType.listing`) — **not routing**. **KEEP**.
- **D:** `app_router.dart:201` public-browse **allowlist**, not `ForSaleModule` route.
- **E:** `share_target.dart:52` public share URL.

**Consumers of #7 `canonicalPath` (evidence, not extra files to change in this audit):**

- `content_detail_screen.dart:837` `context.push(resourceProjection.canonicalPath)`
- `content_resource_projection_card.dart:21` `context.push(...)`
- `feed_renderers.dart:211` `context.push(...)`

Fixing the getter updates those pushes without editing those files.

**`my_for_sales_screen` extra:** uses `Navigator.pushNamed` with path-shaped strings. GoRouter app routes are `RoutePaths` / `RouteNames` (`forSaleDetail`, `editForSale`, `createForSale`). Wrong path **and** likely wrong navigator API. Destination still ForSale.

---

## 3. CANONICAL TARGETS (exact symbols)

From `route_paths.dart` (verified):

| Symbol | Value |
|--------|--------|
| `RoutePaths.forSales` | `'/for-sale'` |
| `RoutePaths.forSaleDetail` | `'/for-sale/:fixedPriceSaleId'` |
| `RoutePaths.createForSale` | `'/create/for-sale'` |
| `RoutePaths.editForSale` | `'/for-sale/:fixedPriceSaleId/edit'` |

**Absent:** `RoutePaths.listing`, `RoutePaths.createListing`, `RoutePaths.listingDetail`.

**Dead vs live:**

| Stale | Live |
|-------|------|
| `/listing/:id` | `/for-sale/:fixedPriceSaleId` |
| `/listings` | `/for-sale` |
| `/create/listing` | `/create/for-sale` |
| `/listing/:id/edit` | `/for-sale/:fixedPriceSaleId/edit` |

`ForSale.forSaleId` is the same id the path param calls `fixedPriceSaleId`.

---

## 4. OWNER-DECISION ITEMS

1. **`share_target.dart:52`** — `$base/listing/$id` may be a **public web** contract (`kPublicShareBaseUrl`). ForSale detail screen uses `ExternalShareType.listing`. Do **not** rewrite to `/for-sale/` without web/SEO owner confirmation.
2. **`notification_navigation_handler.dart` `case 'listing'`** — keep wire key; only change **path**. Same as Stage 2G for `notification_navigation_service.dart`.
3. **`app_router.dart` allowlist** — after in-app `/listing` is gone, unauthenticated `/listing/...` deep links would 404 unless a redirect exists. Owner: add `/for-sale` to allowlist (required for guest catalog) and **remove** `/listing` **or** keep `/listing` only if a compatibility redirect is added later (out of this cluster).

---

## 5. EXACT NEXT BOUNDED IMPLEMENTATION CLUSTER

**Cluster 2H-impl-1 (recommended first, one substitution shape):** in-app GoRouter **detail** `/listing/$id` → `/for-sale/$id` (or `navigateToForSaleDetail` / `RoutePaths.forSaleDetail.replaceFirst`).

Files (6):

1. `commerce_preview_section.dart`
2. `feed_renderers.dart`
3. `search_results_screen.dart`
4. `discussion_screen.dart`
5. `notification_navigation_handler.dart`
6. `content_resource_projection.dart` (`canonicalPath` only)

Do **not** in that cluster: `my_for_sales_screen` (create+edit+`pushNamed`), `welcome_screen` (`/listings`), `app_router` allowlist, `share_target`.

**Follow-on clusters (not this next step):**

- 2H-impl-2: `my_for_sales_screen.dart` (detail + edit + create)
- 2H-impl-3: `welcome_screen.dart` `/listings` → `RoutePaths.forSales`
- 2H-impl-4: `app_router.dart` prefix `/listing` → `/for-sale` (after or with impl-1)
- 2H-impl-5: share URL — owner only

---

## 6. FILES THAT MUST NOT BE TOUCHED (next impl cluster)

- `share_target.dart` until owner decision
- `CreateContentBottomSheet` names/labels
- seller dashboard `_navigateToCreateListing`
- `RoutePaths` / `ForSaleModule` / ForSale domain
- tests, backend, schema, generated l10n
- `case 'listing'` / `SearchResultType.listing` / enum names
- files outside the 2H-impl-1 list when executing that cluster

---

## 7. PROOF COMMAND RESULTS

Searches **bounded to the 10 target files**:

| Pattern | Hits |
|---------|------|
| `/listing/` | #1, #2, #3, #4, #5, #6a/b/d, #7, #10 |
| `/listings` | `welcome_screen.dart:193` (comment), `:194` (`context.go`) |
| `/create/listing` | `my_for_sales_screen.dart:242` |
| `RoutePaths.listing` | **0** |
| `RoutePaths.createListing` | **0** |
| `navigateToListingDetail` | **0** (cleared in 2G) |
| `navigateToCreateListing` | **0** in these 10 files |

`app_router.dart`: `'/listing'` at **201** (no `/listing/` substring); log at **554** is not a path.

Canonical `route_paths.dart`: `forSales`, `forSaleDetail`, `createForSale`, `editForSale` as table in §3.

---

## 8. PRODUCTION / TEST NOT MODIFIED BY THIS AUDIT

This stage only **read** the ten files and wrote this report.

`git status --short` still lists **pre-existing** `M` production/test files from earlier stages (including 2G navigation and 2F tests). **This audit did not edit them.**

Only **new** artifact for Stage 2H: this document.

---

## FILES CHANGED (this stage)

- Production: **0**
- Tests: **0**
- Schema: **0**
- Docs: **1** (`STAGE_2H_LISTING_HARDCODED_ROUTE_RESIDUE_AUDIT.md`)
