# STAGE 2G — LISTING→FORSALE NAVIGATION AUTHORITY AUDIT

Mode: READ-ONLY. Production/tests/schema not modified.

## VERDICT

**AUDIT COMPLETE**

Compile blockers are three production call sites invoking **removed** `NavigationHandler` methods. Canonical ForSale navigation already exists on the same abstraction (`navigateToCreateForSale`, `navigateToForSaleDetail`) and is wired to live `RoutePaths` + `ForSaleModule` screens. Additional hardcoded `/listing/...` paths do not cause the current kernel compile errors but are the same authority residue.

---

## RESIDUE TABLE

| File | Line | Symbol | Caller | Classification | Canonical Target | Evidence |
|------|------|--------|--------|----------------|------------------|----------|
| `apps/mobile/lib/features/home/presentation/screens/main_screen.dart` | 237 | `NavigationHandler.navigateToCreateListing()` | `MainScreen._showCreateContentModal` → `CreateContentBottomSheet.onCreateListing` | **A** | `NavigationHandler.navigateToCreateForSale()` → `AppRouter.navigateToCreateForSale()` → `RoutePaths.createForSale` (`/create/for-sale`) → `CreateForSaleScreen` | Method **does not exist** on `NavigationHandler` (`navigation_handler.dart:46` has `navigateToCreateForSale` only). Same modal already uses `navigateToCreateAuction` which **does** exist. |
| `apps/mobile/lib/features/home/presentation/handlers/main_screen_navigation_handler.dart` | 103–104 | `MainScreenNavigationHandler.navigateToListingDetail` → `_appRouter.navigateToListingDetail` | Local wrapper; `_appRouter` typed as `NavigationHandler` (default `AppRouter()`) | **A** | `NavigationHandler.navigateToForSaleDetail(fixedPriceSaleId)` → `RoutePaths.forSaleDetail` → `ForSaleDetailScreen` | Parameter is already named `fixedPriceSaleId`. `AppRouter` implements `navigateToForSaleDetail` (`app_router.dart:517–522`), **not** `navigateToListingDetail`. No production caller of this wrapper method was found (only definition). Still **compiles** because the missing method is invoked on `NavigationHandler`. |
| `apps/mobile/lib/domains/system/notification/services/notification_navigation_service.dart` | 276 | `NavigationHandler.navigateToListingDetail(targetId)` | `_navigateToComment` when `targetType == 'listing'` | **A** (+ **E** for wire key) | `navigateToForSaleDetail(targetId)` → `/for-sale/:fixedPriceSaleId` | Field `_navigationHandler` is `NavigationHandler` (`notification_navigation_service.dart:25–27`). Intent: open the fixed-price product that received a comment. |
| `apps/mobile/lib/domains/system/notification/services/notification_navigation_service.dart` | 328 | `NavigationHandler.navigateToListingDetail(targetId)` | `_navigateToLikedContent` when `targetType == 'listing'` | **A** (+ **E** for wire key) | same as above | Same missing method. Intent: open the liked fixed-price product. |
| `apps/mobile/lib/core/utils/notification_navigation_handler.dart` | 360–364 | `GoRouter.push('/listing/$targetId')` | parallel notification handler, `case 'listing'` | **A** | `RoutePaths.forSaleDetail` / `navigateToForSaleDetail` | Does **not** call the missing NavigationHandler method. Path `/listing/:id` is **not** registered in `ForSaleModule`. |
| `apps/mobile/lib/features/home/presentation/widgets/commerce_preview_section.dart` | 147 | `context.push('/listing/${item.listing!.forSaleId}')` | Home commerce preview tap | **A** | `context.push('/for-sale/${id}')` or `navigateToForSaleDetail` | Payload is already `ForSale.forSaleId`. |
| `apps/mobile/lib/features/home/presentation/providers/feed_renderers.dart` | 647 | `context.push('/listing/$fixedPriceSaleId')` | `PromotedListingCard` tap | **A** | `RoutePaths.forSaleDetail` | Impression/click already use promotion API; destination path is stale. |
| `apps/mobile/lib/features/search/search/presentation/screens/search_results_screen.dart` | 315–316 | `GoRouter.go('/listing/${result.id}')` | `SearchResultType.listing` | **A** | `navigateToForSaleDetail(result.id)` | Search type enum still named `listing`; destination path is stale. |
| `apps/mobile/lib/domains/social/comment/presentation/screens/discussion_screen.dart` | 226 | `context.push('/listing/$fixedPriceSaleId')` | comment resource tap | **A** | ForSale detail path | |
| `apps/mobile/lib/domains/commerce/catalog/for_sale/presentation/screens/my_for_sales_screen.dart` | 234, 238, 331, 242 | `pushNamed('/listing/...')`, `pushNamed('/create/listing')` | seller my-for-sales | **A** | `RoutePaths.forSaleDetail`, `editForSale`, `createForSale` | Screen is canonical ForSale; paths are old Listing URLs. `/create/listing` is **not** in `RoutePaths`. |
| `apps/mobile/lib/domains/social/content/domain/entities/content_resource_projection.dart` | 586 | `canonicalPath` → `'/listing/$resourceId'` | projection path builder | **A** | `'/for-sale/$resourceId'` | Type enum is already `fixedPriceSale`. |
| `apps/mobile/lib/domains/user/preference/onboarding/presentation/screens/welcome_screen.dart` | 194 | `context.go('/listings')` | guest browse | **A** / **D** | `RoutePaths.forSales` (`/for-sale`) | Comment claims `/listings` is a public route. `RoutePaths.forSales` is `/for-sale`. No `RoutePaths` symbol `listings`. |
| `apps/mobile/lib/core/src/router/app_router.dart` | 201 | redirect allowlist `'/listing'` | unauthenticated public browse | **D** (prefix) / **A** (if kept as alias) | `'/for-sale'` | `ForSaleModule` registers `/for-sale`, not `/listing`. Allowing `/listing` does not create a route. |
| `apps/mobile/lib/domains/social/share/domain/entities/share_target.dart` | 51–52 | public URL `$base/listing/$id` | external share | **E** | in-app: `/for-sale/:id`; public web: owner must confirm public URL contract | External URL may be a web contract, not mobile GoRouter. |
| `apps/mobile/lib/shared/widgets/create_content_bottom_sheet.dart` | 26, 184–187 | callback `onCreateListing` + label “Jual Koi (Listing)” | UI copy / callback name | **C** (callback name) + **A** (wired from main_screen to missing API) | Keep callback or rename later; **must** invoke `navigateToCreateForSale` | Not a missing NavigationHandler method by itself. |
| `apps/mobile/lib/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart` | 1405–1406 | local `_navigateToCreateListing` | seller onboarding step | **B** (destination) / naming **A** | already `Navigator.pushNamed(context, RoutePaths.createForSale)` | Compiles. Method name is leftover; route is canonical. |
| `apps/mobile/lib/domains/user/preference/seller/presentation/widgets/profile_store_tab.dart` | 131–133 | `navigation.navigateToForSaleDetail` | store tab | **B** | already canonical | |
| `apps/mobile/lib/features/explore/presentation/widgets/explore_for_sale_tab.dart` | 97–98 | `context.push('/for-sale/${forSale.forSaleId}')` | explore tab | **B** | already canonical path | |
| `apps/mobile/lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart` | 978–1039 | `_navigateToCreateForSale` / `ForSaleDetailScreen` | chat attach | **B** | already canonical screens | |
| l10n / help center `articleCreateListing` | various | copy “create listing” | help articles | **C** | not navigation authority | |

---

## FULL CALLER MAP

### `navigateToCreateListing`

Production (`apps/mobile/lib`) **call sites**:

1. `features/home/presentation/screens/main_screen.dart:237`

**Definitions:** none on `NavigationHandler`, `AppNavigationHandler`, or `AppRouter`.

### `navigateToListingDetail`

Production **call sites**:

1. `features/home/presentation/handlers/main_screen_navigation_handler.dart:104` (definition + call into `NavigationHandler`)
2. `domains/system/notification/services/notification_navigation_service.dart:276`
3. `domains/system/notification/services/notification_navigation_service.dart:328`

**Definitions on NavigationHandler / AppRouter:** none.

Wrapper `MainScreenNavigationHandler.navigateToListingDetail` (`:103`) has **no other production callers** (MainScreen constructs the handler for drawer/tabs, not this method).

### `RoutePaths.createListing` / `RoutePaths.listingDetail`

**Zero** production references. Those symbols **do not exist**.

Live commerce path symbols (exact):

- `RoutePaths.forSales` = `'/for-sale'`
- `RoutePaths.forSaleDetail` = `'/for-sale/:fixedPriceSaleId'`
- `RoutePaths.createForSale` = `'/create/for-sale'`
- `RoutePaths.editForSale` = `'/for-sale/:fixedPriceSaleId/edit'`
- `RoutePaths.sellerForSales` = `'/seller/for-sale'`

Matching names: `RouteNames.forSales`, `forSaleDetail`, `createForSale`, `editForSale`, `sellerForSales`.

### Hardcoded Listing path strings (live in `lib`, not the compile trio)

| Path | File:line |
|------|-----------|
| `/listing/$id` | `commerce_preview_section.dart:147` |
| `/listing/$fixedPriceSaleId` | `feed_renderers.dart:647` |
| `/listing/${result.id}` | `search_results_screen.dart:316` |
| `/listing/$fixedPriceSaleId` | `discussion_screen.dart:226` |
| `/listing/$targetId` | `notification_navigation_handler.dart:364` |
| `/listing/$forSaleId` | `my_for_sales_screen.dart:234` |
| `/listing/${listing.forSaleId}/edit` | `my_for_sales_screen.dart:238`, `:331` |
| `/create/listing` | `my_for_sales_screen.dart:242` |
| `/listing/$resourceId` | `content_resource_projection.dart:586` |
| `/listings` | `welcome_screen.dart:194` |
| `$base/listing/$id` (public share) | `share_target.dart:52` |
| redirect prefix `'/listing'` | `app_router.dart:201` |

### Canonical ForSale navigation (already live)

| Symbol | File |
|--------|------|
| `NavigationHandler.navigateToCreateForSale` | `navigation_handler.dart:46` |
| `NavigationHandler.navigateToForSaleDetail` | `navigation_handler.dart:33–35` |
| `NavigationHandler.navigateToSellerForSales` | `navigation_handler.dart:92` |
| `AppNavigationHandler.navigateToCreateForSale` | `app_navigation_handler.dart:89` |
| `AppNavigationHandler.navigateToForSaleDetail` | `app_navigation_handler.dart:79–80` |
| `AppRouter.navigateToCreateForSale` | `app_router.dart:525–526` |
| `AppRouter.navigateToForSaleDetail` | `app_router.dart:517–522` |
| `ForSaleModule` routes | `for_sale_module.dart:47–93` |
| `CreateForSaleScreen` | `create_for_sale_screen.dart` |
| `ForSaleDetailScreen` | `for_sale_detail_screen.dart` |
| `ForSaleListScreen` | catalog `/for-sale` |
| `EditForSaleScreen` | edit route |
| `MyForSalesScreen` | `RoutePaths.sellerForSales` |

---

## ROUTE AUTHORITY

**Alive (exact, from `route_paths.dart` + `for_sale_module.dart`):**

```
/for-sale                          → ForSaleListScreen
/for-sale/:fixedPriceSaleId        → ForSaleDetailScreen(forSaleId: …)
/create/for-sale                   → CreateForSaleScreen
/for-sale/:fixedPriceSaleId/edit   → EditForSaleScreen
/seller/for-sale                   → MyForSalesScreen
```

**Navigation abstraction (exact):**

```
navigateToCreateForSale()
  → AppRouter.push(RoutePaths.createForSale)
  → CreateForSaleScreen

navigateToForSaleDetail(fixedPriceSaleId)
  → AppRouter.push(RoutePaths.forSaleDetail with :fixedPriceSaleId replaced)
  → ForSaleDetailScreen
```

**Dead (no RoutePaths, no ForSaleModule GoRoute):**

- `/listing`
- `/listing/:id`
- `/listings`
- `/create/listing`
- `navigateToCreateListing`
- `navigateToListingDetail`
- `RoutePaths.createListing`
- `RoutePaths.listingDetail`

**ForSale vs fixed-price sale:** not two products. `ForSaleModule` header states it is the **fixed-price sale** channel (`/api/v1/for-sale`). Path param is named `fixedPriceSaleId`; entity field is `ForSale.forSaleId`. Auction is a **sibling** (`RoutePaths.createAuction`, `/auction/:auctionId`), not a Listing subtype. Old “Listing” name = this ForSale/fixed-price channel.

---

## BEHAVIOR CLASSIFICATION

### Compile trio (this stage’s blockers)

1. **`main_screen.dart:237` `navigateToCreateListing`**  
   **CONVERGE** → `navigateToCreateForSale()`.  
   Intent still valid: seller FAB/sheet “Jual Koi” creates a fixed-price ForSale. Not obsolete. Not auction.

2. **`main_screen_navigation_handler.dart:103–104` `navigateToListingDetail`**  
   **CONVERGE** method body to `navigateToForSaleDetail(fixedPriceSaleId)` (or delete the wrapper if unused).  
   Intent: public product detail. Param already `fixedPriceSaleId`.  
   **KEEP** behavior; **CONVERGE** symbol.

3. **`notification_navigation_service.dart:276` and `:328`**  
   **CONVERGE** call to `navigateToForSaleDetail(targetId)`.  
   Intent still valid if `targetId` is a ForSale id.  
   **OWNER DECISION** on wire `targetType`: production chat/order/report use `'for_sale'`; these two switches only handle `'listing'`. Next implementer should not invent `'for_sale'` handling without backend payload evidence—but the **method** must become `navigateToForSaleDetail` to compile.

### Same-authority path residue (not this compile, not in the 1–3 file next step)

- Hardcoded `/listing/...` taps: **CONVERGE** to `/for-sale/:id` (or handler).  
- `welcome_screen` `/listings`: **CONVERGE** to `RoutePaths.forSales`.  
- `my_for_sales_screen` `/create/listing`: **CONVERGE** to `RoutePaths.createForSale`.  
- `app_router` public prefix `'/listing'`: **DELETE** or replace with `'/for-sale'` (no live `/listing` route).  
- Public share `$base/listing/$id`: **OWNER DECISION** (web URL vs in-app).  
- `CreateContentBottomSheet.onCreateListing` name / “Listing” label: **KEEP** for this compile step; rename is copy, not routing.  
- Seller dashboard `_navigateToCreateListing`: **KEEP** destination; optional rename later.

---

## BASELINE BLOCKERS

Only the compile errors in this navigation scope:

```
lib/features/home/presentation/screens/main_screen.dart:237
  NavigationHandler.navigateToCreateListing isn't defined

lib/features/home/presentation/handlers/main_screen_navigation_handler.dart:104
  NavigationHandler.navigateToListingDetail isn't defined

lib/domains/system/notification/services/notification_navigation_service.dart:276
  NavigationHandler.navigateToListingDetail isn't defined

lib/domains/system/notification/services/notification_navigation_service.dart:328
  NavigationHandler.navigateToListingDetail isn't defined
```

No missing ForSale route/screen/handler for the intended destinations.

---

## EXACT NEXT BOUNDED STEP

Touch **only these 3 production files** (call-site symbol swap; no new Listing APIs, no RoutePaths rename, no ForSale domain rewrite):

1. `apps/mobile/lib/features/home/presentation/screens/main_screen.dart`  
   `navigateToCreateListing()` → `navigateToCreateForSale()`

2. `apps/mobile/lib/features/home/presentation/handlers/main_screen_navigation_handler.dart`  
   `_appRouter.navigateToListingDetail(...)` → `_appRouter.navigateToForSaleDetail(...)`  
   (rename wrapper if desired in the same file)

3. `apps/mobile/lib/domains/system/notification/services/notification_navigation_service.dart`  
   both `navigateToListingDetail(targetId)` → `navigateToForSaleDetail(targetId)`  
   Keep `case 'listing'` until wire evidence for `'for_sale'` exists.

Do **not** in that step: hardcoded `/listing/` cluster, help copy, share URLs, `CreateContentBottomSheet` rename.

---

## FILES CHANGED

- Production: **0**
- Tests: **0**
- Schema: **0**
- Docs: **1** (`STAGE_2G_LISTING_FORSALE_NAVIGATION_AUTHORITY_AUDIT.md`)
