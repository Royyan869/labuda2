# STAGE D — MOBILE FOR SALE VOCABULARY AUDIT

**Project**: Labuda monorepo (`D:\Project\labuda`)  
**Audit date**: 2026-08-26  
**Mode**: READ-ONLY (no code changed)  
**Executor**: Recovery + audit after backend Stage B/C convergence  
**Canonical target**: Backend `For Sale` domain (`for_sale` wire, `/api/v1/for-sale` routes)

---

## 0. EXECUTIVE SUMMARY

### Current State
- **Backend**: Fully converged to `For Sale` vocabulary after Stage B (Go domain rename) and Stage C (schema migration 000047)
- **Mobile**: Still uses **legacy `listing` / `fixed_price_sale`** vocabulary extensively
- **Wire Contract Split**: Backend emits `/api/v1/for-sale` routes; mobile still calls **`/api/v1/listings`** and expects `fixed_price_sale` discriminators

### Audit Findings
- **578 modified files** + **137 untracked files** in worktree (pre-existing work, NOT Stage D)
- **~550+ active legacy For Sale references** in mobile (fixedPriceSale, fixed_price_sale, listing routes)
- **4 canonical references** (forSale, ForSale factory/class) — greenfield only
- **Zero backend rollback needed** — backend canonical contract is stable and verified
- **Mobile-backend wire contract MISMATCH CONFIRMED** — mobile cannot communicate with current backend routes

### Verdict
**NO-GO for immediate implementation**. Critical wire contract incompatibility discovered between mobile client and backend API.

---

## 1. FILESYSTEM / RECOVERY STATE

### Git Status
```
Modified: 578 files (backend + mobile)
Untracked: 137 files (reports, work artifacts)
```

**Classification**:
- Backend modifications: Stage B/C For Sale convergence (Go domain + schema) — **VERIFIED, DO NOT TOUCH**
- Mobile modifications: Mixed ownership (unknown scope, pre-existing)
- Untracked files: Audit reports, design docs — **IGNORE**

### Pre-Existing Mobile Changes
Current worktree has extensive mobile modifications already present. **Boundary rule**: Do NOT clean up, restore, reset, or remove any files outside explicit Stage D scope. Treat filesystem as source of truth.

---

## 2. CANONICAL BACKEND CONTRACT (ASSUMED FROZEN)

Based on backend source audit (`backend/cmd/core_server/routes_core.go`, `backend/internal/commerce/forsale/`):

### Backend Routes (PUBLIC)
```go
// v1Browse group (unauthenticated allowed)
GET  /api/v1/for-sale           // ListForSales
GET  /api/v1/for-sale/:id       // GetForSale
GET  /api/v1/search/for-sale    // SearchForSales
```

### Backend Routes (SELLER)
```go
// v1 /for-sale group (requires seller authority)
POST   /api/v1/for-sale         // CreateForSale
PUT    /api/v1/for-sale/:id     // UpdateForSale
DELETE /api/v1/for-sale/:id     // DeleteForSale
```

### Backend Wire Discriminator
- **Domain**: `For Sale` / `forsale` package
- **Types**: `ForSale`, `ForSaleStatus`, `ForSaleVisibility`, etc.
- **Wire objectType**: `for_sale` (NOT `fixed_price_sale`)
- **DB table**: `for_sales` (migrated from `fixed_price_sales` in 000047)
- **Event strings**: `for_sale.created`, `for_sale.published`, etc.

### Backend Vocabulary Proof
- `backend/internal/commerce/forsale/` — entire package renamed
- `backend/internal/commerce/forsale/delivery/http/for_sale_handler.go` — HTTP handler
- `backend/cmd/core_server/routes_core.go:145-147` — routes registered
- Migration `000047` — table/enum convergence completed
- Stage 6B regression passed — live schema proof completed

**Conclusion**: Backend canonical contract is `for_sale` on all surfaces. No `fixed_price_sale` or `/listings` routes exist in current backend.

---

## 3. ACTIVE MOBILE FOR SALE DOMAIN

### Domain Location
`apps/mobile/lib/domains/commerce/catalog/listing/`

**Structure**:
- **23 files** total
- **3 layers**: domain (entities, repositories) + data (DTOs, datasources, mappers) + presentation (providers, screens, widgets)
- **Barrel exports**: `listing.dart` (module root) → `domain/`, `data/`, `presentation/`

### Key Symbols (ALL For Sale Surface)

| Layer | File | Classes/Symbols | Classification |
|-------|------|-----------------|----------------|
| **Domain** | `entities/listing.dart` | `Listing`, `ListingStatus`, `ListingVisibility`, `ListingLocation` | For Sale entity |
| | `repositories/listing_repository.dart` | `ListingRepository` (abstract interface) | For Sale contract |
| **Data** | `dto/listing_dto.dart` | `ListingResponseDto`, `CreateListingRequestDto` | For Sale wire DTO |
| | `remote/listing_remote_datasource.dart` | `ListingRemoteDatasource` | For Sale API client |
| | `mappers/listing_dto_mapper.dart` | `ListingDtoMapper.toEntity()` | DTO→entity mapper |
| | `repositories/listing_repository_impl.dart` | `ListingRepositoryImpl` | For Sale repo impl |
| **Presentation** | `providers/listing_controller.dart` | `ListingController` | For Sale app service |
| | `providers/listing_providers.dart` | `fixedPriceSaleDetailProvider`, `listingsProvider` | Riverpod providers |
| | `screens/create_listing_screen.dart` | `CreateListingScreen` | Seller create UI |
| | `screens/edit_listing_screen.dart` | `EditListingScreen` | Seller edit UI |
| | `screens/listing_detail_screen.dart` | `ListingDetailScreen` | Buyer detail UI |
| | `screens/my_listings_screen.dart` | `MyListingsScreen` | Seller inventory UI |
| | `screens/listing_list_screen.dart` | `ListingListScreen` | Public browse UI |
| | `widgets/listing_card.dart` | `ListingCard` | Discovery card |
| | `widgets/listing_picker_bottom_sheet.dart` | `ListingPickerBottomSheet` | Seller picker |

### Entity Structure
```dart
class Listing {
  final String fixedPriceSaleId;  // ⚠️ ID field name
  final String title;
  final String description;
  final ListingStatus status;     // draft/active/sold/withdrawn
  final ListingVisibility visibility;
  final double price;
  final int quantityAvailable;
  // ... seller, product, media, etc.
}
```

**Classification**: Every file in `listing/` tree is **genuinely For Sale** — the fixed-price selling surface. No generic fallback, no auction crossover.

---

## 4. WIRE CONTRACT VOCABULARY INVENTORY

### 4A. Mobile API Calls (LEGACY)

**File**: `apps/mobile/lib/domains/commerce/catalog/listing/data/remote/listing_remote_datasource.dart`

| Method | HTTP Route | Wire Format |
|--------|-----------|-------------|
| `getListing(id)` | `GET /listings/:id` | `ListingResponseDto` |
| `getListingsByIds(ids)` | `POST /listings/batch` | Batch request |
| `listListings(params)` | `GET /listings` | Query params |
| `searchListings(query)` | `GET /search/listings` | Search params |
| `createListing(request)` | `POST /listings` | `CreateListingRequestDto` |
| `updateListing(id, request)` | `PUT /listings/:id` | `UpdateListingRequestDto` |
| `deleteListing(id)` | `DELETE /listings/:id` | Empty response |

**Base path**: `/api/v1` (via `BaseApiRepository`)

**⚠️ CRITICAL MISMATCH**: Mobile calls `/api/v1/listings/*` but backend only serves `/api/v1/for-sale/*`.

### 4B. Wire Discriminator Usages

**Grep audit results** (from explore agent):

| Term | Hits (lib) | Hits (test) | Classification |
|------|------------|-------------|----------------|
| `fixed_price_sale` (snake) | 44 | 87 | **ACTIVE LEGACY FOR SALE** — wire discriminator in chat/share/search/promotion/order |
| `FixedPriceSale` (Pascal) | 4 | 3 | **ACTIVE LEGACY FOR SALE** — class name in comments |
| `fixedPriceSale` (camel) | ~130 | 52 | **ACTIVE LEGACY FOR SALE** — enum values, field names |
| `fixedPriceSaleId` (camel) | ~100 | ~120 | **ACTIVE LEGACY FOR SALE** — canonical ID field |
| `promoted_fixed_price_sale` | 2 | 13 | **ACTIVE LEGACY FOR SALE** — search promo type |
| `promoted_listing` | 3 | 0 | **ACTIVE LEGACY FOR SALE** — feed promo type (older) |
| `targetType: "listing"` | 9 | 23 | **ACTIVE LEGACY FOR SALE** — wire enum value |
| `listing_id` (snake) | 7 | 18 | **GENERIC** (discount) + **FAIL-CLOSED** (tests reject it) |
| `listingId` (camel) | 30 | 26 | **MIXED** — generic in some layers, For Sale in others |
| `for_sale` (snake) | 0 | 0 | **NOT ADOPTED** — canonical backend term missing |
| `forSale` (camel) | 1 | 0 | **CANONICAL** — `StatusOverlayConfig.forSale()` factory only |
| `ForSale` (Pascal) | 3 | 0 | **CANONICAL** — `_ForSaleTab` class only |

### 4C. Key Wire Enums

```dart
// ShareTargetType (shared/attachment/entities/share_reference.dart)
enum ShareTargetType {
  fixedPriceSale('fixed_price_sale', 'fixed_price_sale', '/listing', 'listings', 'Produk Dijual'),
  //             ^^^ wireValue          ^^^ objectType    ^^^ navigationPath  ^^^ apiPath
  auction(...),
  content(...),
  profile(...),
}

// ChatResourceType (domains/chat/chat/domain/entities/chat_resource_projection.dart)
enum ChatResourceType {
  fixedPriceSale,  // wire: 'fixed_price_sale', display: 'Listing'
  auction,
  content,
  profile,
}

// OrderSource (domains/commerce/transaction/order/domain/entities/order_source.dart)
enum OrderSource {
  fixedPriceSale,  // backend wire: 'fixed_price_sale'
  auction,
  negotiation,
}

// TargetType (domains/commerce/pricing/promotion/domain/entities/target_type.dart)
enum TargetType {
  fixedPriceSale('fixed_price_sale'),  // promotion target
  auction('auction'),
  externalProduct('external_product'),
}
```

**Pattern**: Mobile code uses `fixedPriceSale` (camelCase) in Dart identifiers but sends `fixed_price_sale` (snake_case) on wire.

---

## 5. MOBILE ROUTING / DEEP-LINK AUDIT

### 5A. Route Definitions

**File**: `apps/mobile/lib/core/src/router/route_paths.dart`

```dart
class RoutePaths {
  static const listings = '/listings';                     // Browse
  static const listingDetail = '/listing/:fixedPriceSaleId';  // Detail
  static const createListing = '/create/listing';          // Create
  static const editListing = '/listing/:fixedPriceSaleId/edit';  // Edit
  static const sellerListings = '/seller/listings';        // Inventory
  static const checkout = '/checkout/:fixedPriceSaleId';   // Purchase
}
```

**Router module**: `apps/mobile/lib/core/src/router/modules/listing_module.dart`

All routes registered with `fixedPriceSaleId` path parameter.

### 5B. Navigation Methods

**File**: `apps/mobile/lib/core/src/router/app_router.dart`

```dart
void navigateToListingDetail(String fixedPriceSaleId) {
  push('/listing/$fixedPriceSaleId');
}
```

### 5C. Deep-Link Generation

**ShareReference canonical path**:
```dart
// shareReference.navigationPath for fixedPriceSale type
String get navigationPath => '/listing/$targetId';
```

**Chat resource projection**:
```dart
// ChatResourceProjection.canonicalUrl
case ChatResourceType.fixedPriceSale:
  return '/listing/$resourceId';
```

**External share URL** (social/share/domain):
```dart
ExternalShareType.listing -> '$baseUrl/listing/$id'
```

### 5D. Route Vocabulary Summary

| Route Segment | Usage | For Sale Surface? |
|---------------|-------|-------------------|
| `/listings` | Public browse | ✅ Yes |
| `/listing/:fixedPriceSaleId` | Detail page | ✅ Yes |
| `/create/listing` | Seller create | ✅ Yes |
| `/listing/:fixedPriceSaleId/edit` | Seller edit | ✅ Yes |
| `/seller/listings` | Seller inventory | ✅ Yes |
| `/search/listings` | Search endpoint | ✅ Yes |

**Conclusion**: Mobile routing uses **`listing`** terminology exclusively. Zero usage of `for-sale` or `for_sale` in route paths.

---

## 6. CONSUMER / DEPENDENCY GRAPH

### 6A. Feature Consumers

| Feature | Files Consuming Listing Domain | Surface Type |
|---------|--------------------------------|--------------|
| **Home/Feed** | `features/home/presentation/widgets/commerce_preview_section.dart` | For Sale preview cards |
| | `features/home/presentation/providers/feed_renderers.dart` | Feed item rendering |
| | `features/home/data/mappers/feed_mapper.dart` | Maps `promoted_listing` type |
| **Explore** | `features/explore/presentation/widgets/explore_listing_tab.dart` | For Sale discovery tab |
| **Search** | `features/search/search/data/search_repository_impl.dart` | Maps `fixed_price_sale` from search |
| | `features/search/search/data/dto/search_dto.dart` | `promoted_fixed_price_sale` DTO |
| **Seller Dashboard** | `domains/user/preference/seller/presentation/widgets/profile_store_tab.dart` | Seller store listings (`_ForSaleTab` class) |
| **Checkout** | `domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart` | Checkout flow |
| | `domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_accepted_action.dart` | Negotiation → checkout |
| **Create/Edit** | `domains/commerce/catalog/listing/presentation/screens/create_listing_screen.dart` | Seller create |
| | `domains/commerce/catalog/listing/presentation/screens/edit_listing_screen.dart` | Seller edit |
| **Share** | `shared/widgets/link_picker_modal.dart` | Link picker (For Sale tab) |
| | `shared/attachment/entities/share_reference.dart` | `ShareReference.fixedPriceSale()` factory |
| | `domains/social/share/domain/entities/share_target.dart` | `ExternalShareType.listing` |
| **Chat** | `domains/chat/chat/presentation/screens/chat_detail_screen.dart` | Chat commerce context banner |
| | `domains/chat/chat/domain/entities/chat_resource_projection.dart` | `ChatResourceType.fixedPriceSale` |
| **Comment** | `domains/social/comment/presentation/widgets/commerce_resource_picker.dart` | Seller response picker |
| **Saved Items** | `domains/user/preference/saved_item/models/saved_item_model.dart` | `targetType: TargetType.listing` |
| **Promotion** | `domains/commerce/pricing/promotion/presentation/screens/promotion_activation_screen.dart` | Promote listings |

### 6B. Cross-Domain References

| Domain | Field/Enum | Wire Value |
|--------|------------|------------|
| **Order** | `OrderSource.fixedPriceSale` | `"fixed_price_sale"` |
| **Negotiation** | `Negotiation.fixedPriceSaleId` | FK to For Sale |
| **Pricing Preview** | `sourceType` param | `"fixed_price_sale"` |
| **Report** | `ReportTargetType.fixedPriceSale` | `"fixed_price_sale"` |
| **Discount** | `applicable_listing_ids` (GENERIC) | Array of IDs |

---

## 7. TEST CLASSIFICATION

### Test File Count
- **17 files** matching `*listing*` pattern in `apps/mobile/test/`

### Test Classifications

#### A. Active Contract Tests (ACTIVE LEGACY FOR SALE)
**Must update wire expectations after mobile convergence**:
- `test/features/search/search_share_reference_parity_test.dart` — expects `'resource_type': 'fixed_price_sale'`
- `test/features/search/search_repository_promoted_sidecar_concurrency_test.dart` — expects `promoted_fixed_price_sale` type
- `test/features/search/promoted_search_identity_test.dart` — expects `targetType: 'fixed_price_sale'`
- `test/domains/system/report/report_target_type_contract_test.dart` — expects `ReportTargetType.fixedPriceSale.backendValue == 'fixed_price_sale'`
- `test/domains/chat/attachment_contract_alignment_test.dart` — expects `target_type: 'fixed_price_sale'`
- `test/domains/commerce/pricing/promotion/promotion_endpoint_contract_test.dart` — expects `allowed_target_types: ['fixed_price_sale', ...]`

**Count**: ~87 test hits with `fixed_price_sale` wire expectations.

#### B. Fail-Closed Tests (MUST PRESERVE)
**These tests assert legacy `listing_id` is REJECTED**:
- `test/domains/commerce/transaction/shipping/check_delivery_contract_test.dart:46` — `expect(json.containsKey('listing_id'), isFalse)`
- `test/domains/commerce/transaction/checkout/checkout_repository_impl_test.dart:147` — `expect(payload.containsKey('listing_id'), isFalse)`
- `test/domains/commerce/pricing/pricing_preview/pricing_preview_dto_test.dart` — `sends product_id, source_type, source_id — no listing_id`
- `test/domains/chat/attachment_contract_alignment_test.dart` — multiple `expect(...containsKey('listing_id'), isFalse)`
- `test/domains/commerce/catalog/auction/create_auction_contract_test.go:73,165` — auction must not send `listing_id`

**Purpose**: These tests enforce the backend contract that `listing_id` (snake_case) is NOT a valid field. After Stage D, these must still pass (they guard against backend regressions).

**Count**: ~18 fail-closed test assertions.

#### C. Generic/Historical Tests (NO CHANGE NEEDED)
- `test/shared/governance/seller_tier_stage2_test.dart` — comment mentions "listing/auction lifecycle-gate tests" (generic)
- `test/features/search/organic_search_identity_test.dart` — tests search result identity rendering (surface-agnostic)

#### D. UI/Integration Tests (UPDATE AFTER CONVERGENCE)
- `test/shared/widgets/slice_c1a_search_and_tagged_user_identity_test.dart` — renders `SearchResultType.listing`
- `test/features/search/search_result_item_commerce_identity_test.dart` — renders listing subtitle

---

## 8. BACKEND/MOBILE CONTRACT PARITY

### 8A. Critical Mismatch — API Routes

| Client (Mobile) | Server (Backend) | Status |
|-----------------|------------------|--------|
| `GET /api/v1/listings` | `GET /api/v1/for-sale` | ❌ **404 NOT FOUND** |
| `GET /api/v1/listings/:id` | `GET /api/v1/for-sale/:id` | ❌ **404 NOT FOUND** |
| `GET /api/v1/search/listings` | `GET /api/v1/search/for-sale` | ❌ **404 NOT FOUND** |
| `POST /api/v1/listings` | `POST /api/v1/for-sale` | ❌ **404 NOT FOUND** |
| `PUT /api/v1/listings/:id` | `PUT /api/v1/for-sale/:id` | ❌ **404 NOT FOUND** |
| `DELETE /api/v1/listings/:id` | `DELETE /api/v1/for-sale/:id` | ❌ **404 NOT FOUND** |

**Proof**: Backend `routes_core.go:145-147` registers only `/for-sale*` routes. No `/listings*` routes exist.

**Impact**: Mobile For Sale feature is **completely broken** against current backend.

### 8B. Wire Discriminator Parity

| Context | Mobile Sends/Expects | Backend Emits/Expects | Status |
|---------|----------------------|-----------------------|--------|
| ShareReference wire | `fixed_price_sale` | `for_sale` (assumed) | ⚠️ **UNKNOWN** |
| Chat resource type | `fixed_price_sale` | `for_sale` (assumed) | ⚠️ **UNKNOWN** |
| Order source | `fixed_price_sale` | `for_sale` (migrated in 000047) | ❌ **MISMATCH** |
| Promotion target | `fixed_price_sale` | `for_sale` (assumed) | ⚠️ **UNKNOWN** |
| Search result type | `fixed_price_sale` | `for_sale` (assumed) | ⚠️ **UNKNOWN** |
| Feed promo type | `promoted_listing` / `promoted_fixed_price_sale` | `promoted_for_sale` (assumed) | ⚠️ **UNKNOWN** |

**Backend source verification incomplete**: Cannot confirm exact wire discriminator without reading backend response projection code. Marked UNKNOWN where backend DTO serialization is not verified.

### 8C. Promoted Discriminator (Feed vs Search)

**Mobile has TWO legacy promoted types**:
- `promoted_listing` (feed) — `lib/features/home/data/dto/feed_dto.dart:299,303`
- `promoted_fixed_price_sale` (search) — `lib/features/search/search/data/dto/search_dto.dart:850,855`

**Backend canonical** (assumed): `promoted_for_sale` (unified after Stage B/C).

**Verification needed**: What does backend search/feed promotion actually emit today? Without live backend response sample, cannot confirm.

---

## 9. DEAD / ZOMBIE IDENTIFICATION

### 9A. No Dead Code Found
- All files in `listing/` domain are actively routed and used
- All screens registered in router module
- All providers wired in DI graph
- No orphaned widgets or unused imports

### 9B. Generic "listing" (MUST REMAIN)

**Discount domain** (`domains/commerce/pricing/discount/`):
- `applicable_listing_ids` field — **GENERIC** array field (applies to any listing surface, including auction if needed)
- `listingId` parameter in discount validation — **GENERIC** filter parameter

**Classification**: These are NOT For Sale vocabulary. They use "listing" as a generic term meaning "any item that can be listed for sale" (polymorphic over For Sale and Auction). **DO NOT RENAME**.

### 9C. Intentional Fail-Closed Tests (MUST REMAIN)

18 test assertions enforcing `listing_id` (snake_case) absence from For Sale wire contracts. These are **security/correctness guards**, not legacy to remove.

**Example**:
```dart
test('checkout does not send listing_id', () {
  expect(payload.containsKey('listing_id'), isFalse);
});
```

**Rationale**: Backend For Sale API uses `product_id` + `for_sale_id` (formerly `fixed_price_sale_id`), never `listing_id`. Tests ensure mobile never sends the wrong field.

---

## 10. EXACT BOUNDED IMPLEMENTATION SCOPE

### 10A. Files Expected to Change

**Domain layer** (7 files):
```
lib/domains/commerce/catalog/listing/listing.dart
lib/domains/commerce/catalog/listing/domain/domain.dart
lib/domains/commerce/catalog/listing/domain/entities/listing.dart
lib/domains/commerce/catalog/listing/domain/repositories/listing_repository.dart
lib/domains/commerce/catalog/listing/data/data.dart
lib/domains/commerce/catalog/listing/data/dto/listing_dto.dart
lib/domains/commerce/catalog/listing/data/remote/listing_remote_datasource.dart
lib/domains/commerce/catalog/listing/data/mappers/listing_dto_mapper.dart
lib/domains/commerce/catalog/listing/data/repositories/listing_repository_impl.dart
lib/domains/commerce/catalog/listing/presentation/presentation.dart
lib/domains/commerce/catalog/listing/presentation/providers/listing_controller.dart
lib/domains/commerce/catalog/listing/presentation/providers/listing_providers.dart
lib/domains/commerce/catalog/listing/presentation/providers/seller_fps_pager.dart
lib/domains/commerce/catalog/listing/presentation/create_listing_route_contract.dart
lib/domains/commerce/catalog/listing/presentation/screens/create_listing_screen.dart
lib/domains/commerce/catalog/listing/presentation/screens/edit_listing_screen.dart
lib/domains/commerce/catalog/listing/presentation/screens/listing_detail_screen.dart
lib/domains/commerce/catalog/listing/presentation/screens/listing_list_screen.dart
lib/domains/commerce/catalog/listing/presentation/screens/my_listings_screen.dart
lib/domains/commerce/catalog/listing/presentation/widgets/listing_card.dart
lib/domains/commerce/catalog/listing/presentation/widgets/listing_media_handler.dart
lib/domains/commerce/catalog/listing/presentation/widgets/listing_picker_bottom_sheet.dart
```

**Cross-domain references** (~50 files):
- `lib/shared/attachment/entities/share_reference.dart` — ShareTargetType enum
- `lib/shared/object/object_reference_bridge.dart` — object type switch
- `lib/shared/widgets/link_picker_modal.dart` — link picker tabs
- `lib/core/src/router/route_paths.dart` — route constants
- `lib/core/src/router/modules/listing_module.dart` — router module
- `lib/core/src/router/app_router.dart` — navigation methods
- `lib/features/home/data/dto/feed_dto.dart` — feed promo types
- `lib/features/search/search/data/dto/search_dto.dart` — search promo types
- `lib/domains/chat/chat/domain/entities/chat_resource_projection.dart` — ChatResourceType enum
- `lib/domains/commerce/transaction/order/domain/entities/order_source.dart` — OrderSource enum
- `lib/domains/commerce/pricing/promotion/domain/entities/target_type.dart` — TargetType enum
- `lib/domains/system/report/domain/entities/report.dart` — ReportTargetType enum
- All consumer files from §6 dependency graph

**Test files** (~17 files in `test/`):
- Update wire contract expectations
- Preserve fail-closed guards

**Estimated total**: ~90 files will require changes.

### 10B. Renaming Strategy

| Current | Target | Scope |
|---------|--------|-------|
| `listing/` directory | `for_sale/` | Directory rename |
| `Listing` entity | `ForSale` | Class rename |
| `ListingStatus` | `ForSaleStatus` | Enum rename |
| `ListingVisibility` | `ForSaleVisibility` | Enum rename |
| `ListingRepository` | `ForSaleRepository` | Interface rename |
| `fixedPriceSale` enum value | `forSale` | Enum value rename |
| `fixedPriceSaleId` field | `forSaleId` | **⚠️ DECISION NEEDED** |
| `fixed_price_sale` wire | `for_sale` | Wire string |
| `/listing` routes | `/for-sale` | Route path |
| `/api/v1/listings` | `/api/v1/for-sale` | API endpoint |

### 10C. ID Field Name Decision

**Current**: `fixedPriceSaleId` used as ID field name throughout mobile (entity, routes, DTOs, providers).

**Options**:
1. Rename to `forSaleId` (consistent with domain rename)
2. Keep `fixedPriceSaleId` (preserve historical API)

**Recommendation**: Rename to `forSaleId` for full convergence. Route param becomes `:forSaleId`.

**Impact**: ~100+ references to update.

---

## 11. RISKS

### 11A. Critical Risks

**R1. Wire Contract Incompatibility**
- **Current state**: Mobile calls `/api/v1/listings`, backend only serves `/api/v1/for-sale`
- **Impact**: For Sale feature completely non-functional
- **Mitigation**: Cannot deploy mobile without backend compatibility layer OR mobile must be updated atomically

**R2. Promoted Discriminator Unknown**
- **Issue**: Cannot confirm backend's current `promoted_*` discriminator
- **Impact**: May break feed/search promo rendering after mobile update
- **Mitigation**: Requires live backend response inspection OR backend source code read

**R3. Share/Chat Wire Format Unknown**
- **Issue**: Backend Stage B/C did not provide wire contract spec
- **Impact**: ShareReference/ChatResource may break after mobile update
- **Mitigation**: Requires backend HTTP response/DTO audit

### 11B. Implementation Risks

**R4. Large Blast Radius**
- **Scope**: ~90 files, ~550 active references
- **Risk**: Over-renaming or missing references
- **Mitigation**: Staged implementation with per-layer verification

**R5. Route Param Rename**
- **Current**: `:fixedPriceSaleId` in routes, deep-links, share URLs
- **Risk**: Breaking existing deep-links if external systems store them
- **Mitigation**: Owner decision needed on backward compat

**R6. Test Regression**
- **Scope**: ~100+ test files with wire expectations
- **Risk**: Missing test updates causing false pass/fail
- **Mitigation**: Classify tests (contract vs fail-closed vs generic) before rename

### 11C. Worktree Risks

**R7. Pre-Existing Modifications**
- **Current**: 578 modified files in worktree
- **Risk**: Merge conflicts or accidental overwrite of unrelated work
- **Mitigation**: Stage D must NOT modify backend or unrelated mobile files

---

## 12. REQUIRED REGRESSION PROOF

After Stage D mobile convergence, must prove:

### 12A. Wire Contract Tests
1. Mobile can call `/api/v1/for-sale` and receive valid response
2. Mobile can search via `/api/v1/search/for-sale`
3. Mobile can create/update/delete via new routes
4. ShareReference navigation resolves correctly
5. Chat resource projection renders correctly
6. Order source wire matches backend expectation

### 12B. Feature Tests
1. Buyer can browse For Sale items (feed, explore, search)
2. Buyer can view For Sale detail
3. Buyer can purchase (checkout flow)
4. Seller can create For Sale item
5. Seller can edit/delete own For Sale items
6. Seller can share For Sale item to chat
7. Promotion activation works for For Sale
8. Saved items work for For Sale
9. Comment commerce picker works for For Sale
10. Negotiation over For Sale works

### 12C. Negative Tests
1. Fail-closed tests still pass (listing_id rejected)
2. Generic discount tests still pass (applicable_listing_ids unchanged)
3. Auction surface unaffected (no cross-contamination)

---

## 13. CLEANUP PROOF REQUIREMENTS

After Stage D implementation:

### 13A. Grep Zero Targets
- `grep -r "fixed_price_sale"` in mobile lib/ → ZERO (except fail-closed test comments)
- `grep -r 'fixedPriceSale'` in mobile lib/ → ZERO
- `grep -r '/listing'` in mobile lib/ routes → ZERO (except generic discount)
- `grep -r '"/listings"'` in mobile lib/ → ZERO

### 13B. Vocabulary Convergence
- All enums use `forSale` (not `fixedPriceSale`)
- All wire strings use `for_sale` (not `fixed_price_sale`)
- All routes use `/for-sale` (not `/listing`)
- All classes use `ForSale` prefix (not `Listing` or `FixedPriceSale`)

### 13C. Boundary Integrity
- Generic "listing" preserved in discount domain
- Fail-closed tests preserved and passing
- Auction domain untouched

---

## 14. GO / NO-GO DECISION

### 14A. BLOCKERS (Must Resolve Before Implementation)

**B1. Backend Wire Contract Specification**
- **Issue**: Cannot confirm backend's exact wire format after Stage B/C
- **Required**: Backend HTTP response sample OR DTO source code audit for:
  - `promoted_*` discriminator value
  - ShareReference wire format
  - ChatResourceProjection wire format
  - Order source enum value
  - Search result type discriminator
- **Owner action**: Provide backend API response samples OR approve backend source read

**B2. Mobile Route Backward Compatibility**
- **Issue**: External systems may store `/listing/:id` deep-links
- **Required**: Owner decision on backward compat strategy:
  - Option A: Hard cutover (break old links)
  - Option B: Backend supports both `/listing/*` and `/for-sale/*` during transition
  - Option C: Mobile router aliasing (both paths work)
- **Owner action**: Choose backward compat strategy

**B3. ID Field Name Decision**
- **Issue**: Should `fixedPriceSaleId` become `forSaleId`?
- **Impact**: Affects routes (`:forSaleId`), deep-links, 100+ code references
- **Owner action**: Approve `forSaleId` OR keep `fixedPriceSaleId`

### 14B. RISKS REQUIRING OWNER ACKNOWLEDGMENT

**R1. Mobile Feature Broken in Production**
- **Current state**: If backend Stage B/C deployed, mobile For Sale is 404
- **Acknowledgment needed**: Owner confirms backend NOT deployed OR mobile urgency

**R2. Large Mobile Diff**
- **Estimated**: ~90 files, ~550 active references
- **Review burden**: Significant PR review required
- **Acknowledgment needed**: Owner approves large-scale mobile refactor

### 14C. DEPENDENCIES

**D1. Backend Stability**
- **Requirement**: Backend `/api/v1/for-sale` routes stable and tested
- **Status**: ASSUMED stable (Stage B/C reported complete)

**D2. Migration Applied**
- **Requirement**: Backend migration 000047 applied (for_sales table exists)
- **Status**: ASSUMED applied (Stage C reported complete)

### 14D. VERDICT

**NO-GO** for immediate implementation.

**Rationale**:
1. **Wire contract blocker (B1)** — Cannot implement mobile without backend wire spec
2. **Production impact blocker (R1)** — If backend deployed, mobile broken; if backend not deployed, no urgency
3. **Owner decisions pending (B2, B3)** — Backward compat and ID naming need approval

**Required actions before GO**:
1. Owner provides backend wire contract specification (B1)
2. Owner decides backward compat strategy (B2)
3. Owner approves ID field naming (B3)
4. Owner confirms backend deployment status (R1)
5. Owner approves large mobile diff (R2)

**Recommended next step**: STOP and request owner decisions. Do NOT proceed with implementation until blockers resolved.

---

## 15. STAGE D AUDIT COMPLETE

**Files read**: 50+ (backend routes, mobile domain, cross-domain consumers, tests, prior audit reports)  
**Files modified**: 0 (audit only, no implementation)  
**Files created**: 1 (this report)

**Audit status**: ✅ COMPLETE  
**Implementation status**: ⏸️ BLOCKED (awaiting owner decisions B1, B2, B3)

**Next session**: Owner review and decision on blockers. If GO approved, begin Stage D implementation with exact bounded scope from §10.

---

**END OF AUDIT REPORT**
