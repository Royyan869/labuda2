# SHIPPING-01 PASS 01 — DEEP FORENSIC AUDIT

**Status:** REDESIGN REQUIRED  
**Date:** 2026-09-02  
**Auditor:** Buffy (Codebuff)  
**Scope:** Complete shipping domain architecture, naming, data flow, and business truth alignment

---

## 1. CURRENT ARCHITECTURE MAP

### 1.1 Domain Structure

```
backend/internal/commerce/shipping/
├── application/
│   ├── shipping_service.go          — Read-only delivery availability check
│   ├── seller_shipping_service.go   — Seller CRUD for shipping options + coverages
│   ├── for_sale_shipping_service.go — Product↔shipping option linking + sellable validation
│   └── errors.go                    — Sentinel errors
├── entity/
│   ├── shipping_option.go           — ShippingOption entity (DEAD: see below)
│   ├── shipping_coverage.go         — ShippingCoverage entity
│   ├── city_override.go             — CityOverride entity
│   └── transport_type.go            — TransportType enum (train/bus/travel/plane/custom)
├── infrastructure/repository/       — Repository implementations
├── delivery/http/                   — HTTP handlers
└── quote/                           — Private shipping quote subdomain
    ├── application/shipping_quote_service.go
    ├── entity/shipping_quote.go
    ├── repository/shipping_quote_repository.go
    ├── infrastructure/repository/
    └── delivery/http/shipping_quote_handler.go
```

### 1.2 Mobile Domain Structure

```
apps/mobile/lib/domains/commerce/transaction/shipping/
├── domain/entities/shipping.dart    — ShippingOption, ShippingCoverage, CityShippingRate, DeliveryOption
├── domain/repositories/shipping_repository.dart — ShippingRepository interface
├── data/repositories/shipping_repository_impl.dart
├── data/remote/shipping_remote_datasource.dart
├── data/mappers/shipping_mapper.dart
├── presentation/providers/shipping_notifier.dart
├── presentation/widgets/
│   ├── shipping_option_setup_screen.dart   — Seller creates/edits a shipping option + coverages
│   └── seller_shipping_options_selector.dart — Multi-select widget for product creation
```

---

## 2. DATABASE MAP

### 2.1 Tables (PostgreSQL)

**shipping_options** — Seller-level reusable shipping setups
| Column | Type | Notes |
|--------|------|-------|
| id | uuid PK | |
| seller_id | uuid FK→users | |
| name | text | Display name (e.g., "Bus ke Jateng") |
| transport_type | shipping_transport_type_enum | train/bus/travel/plane/custom |
| expedition_name | text | **DROPPED in migration 000014** |
| is_active | boolean | |
| internal_purpose | text | Seller-only note |
| created_at, updated_at | timestamptz | |

**shipping_coverages** — Province-level coverage per shipping option
| Column | Type | Notes |
|--------|------|-------|
| id | uuid PK | |
| shipping_option_id | uuid FK→shipping_options | CASCADE |
| province_code | text | BPS 2-digit |
| province_name | text | |
| province_rate | bigint | Shipping+packing combined |
| estimated_days | text | **DROPPED in migration 000014** |
| is_available | boolean | |
| created_at | timestamptz | |
| UNIQUE | (shipping_option_id, province_code) | |

**shipping_city_overrides** — City-level overrides per coverage
| Column | Type | Notes |
|--------|------|-------|
| id | uuid PK | |
| shipping_coverage_id | uuid FK→shipping_coverages | CASCADE |
| city_code | text | |
| city_name | text | |
| price | bigint | **DEAD COLUMN** (see below) |
| rate | bigint | Canonical rate column |
| estimated_days | text | **DROPPED in migration 000014** |
| is_available | boolean | |
| created_at, updated_at | timestamptz | |
| UNIQUE | (shipping_coverage_id, city_code) | |

**product_shipping_options** — Product↔shipping option linking
| Column | Type | Notes |
|--------|------|-------|
| product_id | uuid FK→products | |
| shipping_option_id | uuid FK→shipping_options | |
| sort_order | integer | |
| created_at | timestamptz | |
| PK | (product_id, shipping_option_id) | |

**shipping_quotes** — Private seller-issued shipping cost quotes
| Column | Type | Notes |
|--------|------|-------|
| id | uuid PK | |
| chat_id | uuid FK→chat_rooms | |
| product_id | uuid FK→products | NOT NULL |
| source_type | sale_surface_type_enum | for_sale/auction/negotiation |
| source_id | uuid | |
| auction_id | uuid | **LEGACY COLUMN** — source_type/source_id replaced this |
| seller_id | uuid | |
| buyer_id | uuid | |
| cost | bigint | Combined shipping+packing quote |
| note | text | |
| status | shipping_quote_status_enum | ACTIVE/USED/EXPIRED/INVALID |
| superseded_at | timestamptz | |
| superseded_by_id | uuid FK→shipping_quotes | |
| destination_city_id | text | Address lock |
| destination_province_id | text | Address lock |
| used_at | timestamptz | |
| expires_at | timestamptz NOT NULL | |
| reactivation_count | integer | |
| max_reuse | integer | |
| created_at | timestamptz | |
| UNIQUE | (chat_id, product_id, source_type, source_id, seller_id, buyer_id) WHERE status=ACTIVE AND superseded_at IS NULL | |

### 2.2 Enums

**shipping_transport_type_enum:** `train`, `bus`, `travel`, `plane`, `custom`  
**shipping_quote_status_enum:** `ACTIVE`, `USED`, `EXPIRED`, `INVALID`  
**sale_surface_type_enum:** `for_sale`, `auction`, `negotiation`  
**order_source_enum:** `for_sale`, `auction`, `negotiation` (legacy `listing` was dropped in migration 000047)

### 2.3 Shipping-Related Columns on `orders`

| Column | Type | Notes |
|--------|------|-------|
| shipping_option_id | uuid FK→shipping_options | Nullable — NULL when using shipping quote |
| shipping_option_name | text | Snapshot |
| shipping_transport_type | text | Snapshot |
| shipping_expedition_name | text | Snapshot |
| shipping_estimated_days | text | Snapshot |
| shipping_total | bigint | Combined shipping+packing amount |
| shipping_quote_id | uuid | FK reference to shipping_quotes |
| shipping_quote_price | bigint | Snapshot of quote cost |
| shipping_source | text | "for_sale" or "shipping_quote" |
| shipping_destination | jsonb | Address snapshot |
| shipping_origin_snapshot | jsonb | Farm address snapshot |

### 2.4 Pricing Token Shipping Columns

The pricing_token also carries: `shipping_option_id`, `shipping_option_name`, `shipping_transport_type`, `shipping_expedition_name`, `shipping_estimated_days`, `shipping_total`, `shipping_quote_id`, `shipping_source`.

---

## 3. SELLER SETUP FLOW

### 3.1 Current Flow

```
Mobile: SellerShippingScreen
    ↓
GET /api/v1/shipping/options  → listMyShippingOptions()
    ↓
FAB "Tambah Opsi" → ShippingOptionSetupScreen
    ↓
Form: name, transport_type, expedition_name (optional)
    ↓
POST /api/v1/shipping/options  → CreateShippingOption
    ↓
Next: Add coverages per province
    ↓
POST /api/v1/shipping/options/:id/coverages  → CreateCoverage
    ↓
Each coverage: province_code, province_name, rate, estimated_days (optional), is_available
    ↓
Optional: City overrides per coverage
    ↓
PUT /api/v1/shipping/coverages/:id  → UpdateCoverage (city override)
```

### 3.2 Assessment Against Business Truth

| Business Requirement | Status | Evidence |
|---------------------|--------|----------|
| Many reusable setups | ✅ CORRECT | `shipping_options` is seller-level, not product-level |
| Internal notes | ✅ CORRECT | `internal_purpose` column on `shipping_options` |
| One combined shipping+packing price | ⚠️ PARTIAL | `province_rate` on coverage IS the combined amount. But the UI label says "Tariff" not "Shipping + Packing". The hint text does not instruct seller to include packing cost. |
| Coverage | ✅ CORRECT | Province-level coverage with city overrides |
| Seller-defined method label | ✅ CORRECT | `name` field on shipping_option |
| Editing | ✅ CORRECT | Update endpoints exist |
| Deleting | ✅ CORRECT | Delete endpoints exist with cascade |
| Reuse | ✅ CORRECT | Product↔option linking is a separate step |

### 3.3 Naming Issues in Current Seller Setup

| Current Name | Business Truth | Classification |
|-------------|----------------|----------------|
| `ShippingOption` | Shipping Setup | MISNAMED — should be "ShippingSetup" or "ShippingMethod" |
| `ShippingCoverage` | Coverage Area | CANONICAL |
| `CityOverride` | City Rule | CANONICAL |
| `transport_type` enum | Method type | CANONICAL values (bus/travel/etc.) but "transport" is misleading — should be "method_type" |
| `expedition_name` | **DROPPED** | Correctly removed in migration 000014 — Labuda doesn't control carriers |
| `internal_purpose` | Internal Note | CANONICAL |
| `province_rate` | Combined shipping+packing cost | CANONICAL but UI doesn't instruct seller to enter combined amount |
| "Tariff" in UI | Combined shipping+packing cost | MISNAMED — should say "Biaya Pengiriman + Packing" |

---

## 4. PRODUCT SHIPPING SELECTION FLOW

### 4.1 For Sale Creation

```
Mobile: CreateForSaleScreen
    ↓
SellerShippingOptionsSelector (multi-select chips)
    ↓
Loads seller's ACTIVE shipping options via listMyActiveShippingOptions()
    ↓
Seller ticks subset → _selectedShippingOptionIds
    ↓
After product creation:
    PUT /api/v1/products/:id/shipping  → SetProductShippingOptions
    ↓
Backend: ProductShippingService.SetProductShippingOptions
    ↓
Validates: product exists, belongs to seller, no active orders
    ↓
Overwrite model: deletes existing rows, inserts new rows
```

### 4.2 Auction Creation

```
Mobile: CreateAuctionScreen
    ↓
SellerShippingOptionsSelector (same widget)
    ↓
After auction creation:
    POST /api/v1/auctions  (includes shipping_option_ids in body)
    ↓
Backend: AuctionService.CreateAuction
    ↓
ValidateSellableCreateShippingSelection: each option must have ≥1 active coverage
    ↓
LinkSellableCreateShippingSelection: creates product_shipping_options rows
```

### 4.3 Assessment Against Business Truth

| Business Requirement | Status | Evidence |
|---------------------|--------|----------|
| Seller MUST configure shipping | ✅ CORRECT | For Sale: `EnsureShippingConfigured` at publish time. Auction: validated at creation. |
| Multiple setups can be selected | ✅ CORRECT | `SellerShippingOptionsSelector` is multi-select |
| Seller can select subset | ✅ CORRECT | Only selected options are linked |
| Selected options persisted correctly | ✅ CORRECT | `product_shipping_options` table, overwrite model |
| Same architecture reused | ✅ CORRECT | Both For Sale and Auction use `product_shipping_options` + `ValidateSellableCreateShippingSelection` |

### 4.4 Dead Code: `listing_shipping_options`

Migration 000016 drops `listing_shipping_options` table. The old file `listing_shipping_option_repository_impl.go` still exists in `shipping/infrastructure/repository/` — this is **DEAD CODE** and should be deleted.

---

## 5. BUYER SHIPPING FLOW

### 5.1 Coverage Matching

```
Buyer selects address (province + city)
    ↓
CheckoutScreen._loadDeliveryOptions()
    ↓
GET /api/v1/shipping/check  (product_id + province_code + city_code)
    ↓
ShippingService.CheckDeliveryAvailability:
    1. Load product_shipping_options for product
    2. For each option:
       a. Skip if is_active = false
       b. Find coverage by province_code
       c. Skip if no coverage or is_available = false
       d. Check city_override if city_code provided
       e. Build DeliveryOption with final rate
    3. Return []DeliveryOption
    ↓
Also returns: product_configured (bool) — whether ANY options are linked
```

### 5.2 Buyer Experience

| Scenario | Backend Response | Mobile UX |
|----------|-----------------|-----------|
| Options exist, address covered | `options: [...]`, `product_configured: true` | Radio list of shipping options with rates |
| Options exist, address NOT covered | `options: []`, `product_configured: true` | "Alamat di luar coverage. Hubungi penjual." + CTA |
| No options linked | `options: []`, `product_configured: false` | "Penjual belum mengatur pengiriman. Hubungi penjual." + CTA |

### 5.3 Checkout Blocking

- Buyer MUST select a shipping option before checkout (mobile validates: `if shippingQuoteId == null && shippingOptionId == null → error`)
- Backend `OrderCreationService.CreateFromSaleSurface` Step 4: checks delivery availability, returns `ErrShippingOptionUnavailable` if selected option not in available list
- Backend Step 2.5: returns `ErrNoShippingOptions` if zero linked options

---

## 6. PRIVATE SHIPPING QUOTE FLOW

### 6.1 End-to-End Flow

```
Seller in chat → taps "Buat Ongkir" (onSendQuote)
    ↓
ShippingQuoteCreationModal: cost + optional note
    ↓
POST /api/v1/chat/:chat_id/shipping-quote
    ↓
ShippingQuoteService.CreateShippingQuote:
    1. Validate chat exists, seller is participant
    2. Validate for_sale/auction exists, belongs to seller
    3. Determine buyer (other participant)
    4. Supersede any prior unsuperseded quotes for same context
    5. Create new quote with ACTIVE status + expiry
    6. Send chat message with shipping_quote attachment
    ↓
Buyer receives shipping_quote message in chat
    ↓
Buyer taps "Beli" on quote
    ↓
Navigate to CheckoutScreen with shippingQuoteId
    ↓
Pricing preview: pricing_token_service uses quote.Cost as shipping_total
    ↓
Order creation: validateShippingQuoteForOrder (9 anti-tamper checks)
    ↓
MarkQuoteUsed (atomic in same transaction)
```

### 6.2 Assessment Against Business Truth

| Business Requirement | Status | Evidence |
|---------------------|--------|----------|
| Created through commerce chat | ✅ CORRECT | `POST /chat/:chat_id/shipping-quote` |
| Transaction-specific | ✅ CORRECT | Scoped to chat_id + product_id + source_type + source_id + seller_id + buyer_id |
| NOT a public Shipping Setup | ✅ CORRECT | Separate table, separate service, no mutation of shipping_options |
| Does NOT mutate seller's reusable setup | ✅ CORRECT | shipping_quotes table is independent |
| Belongs to buyer + commerce context | ✅ CORRECT | buyer_id, product_id, source_type, source_id stored |
| Only ONE active per context | ✅ CORRECT | UNIQUE index + supersession model |
| New quote supersedes previous | ✅ CORRECT | `SupersedeCurrentQuotes` marks old as superseded |
| Expiry | ✅ CORRECT | Default 24h, max 7 days |
| Price authority | ✅ CORRECT | `cost` field is the authoritative shipping amount |

### 6.3 Gaps Against Business Truth

| Gap | Severity | Description |
|-----|----------|-------------|
| **No address lock by default** | P2 | `DestinationCityID` and `DestinationProvinceID` are optional in the quote. The business truth says a quote should be tied to a specific destination. Currently seller can create a quote without locking destination. |
| **No "special shipping price" use case** | P2 | Business truth says quote can exist when "seller wants to offer a special shipping price" even within normal coverage. Current mobile UI only triggers quote flow from "Hubungi Penjual" CTA (out-of-area). No entry point for seller to proactively offer a special price within coverage. |
| **Quote cost label says "Biaya Ongkir"** | P3 | Should say "Biaya Pengiriman + Packing" to match business truth |

---

## 7. AUCTION WINNER FLOW

### 7.1 Current Winner Path

```
Auction ends → status = waiting_settlement
    ↓
Winner taps "Klaim Sekarang"
    ↓
AuctionClaimShippingModal:
    1. Winner selects address
    2. Fetches delivery options via checkDeliveryAvailability
    3. Winner selects shipping option
    4. Claim callback → POST /api/v1/auctions/:id/claim
    ↓
Backend: AuctionService.ClaimWinner
    ↓
Creates order via OrderCreationService.CreateFromAuction
    ↓
Winner navigates to payment
```

### 7.2 Assessment Against Business Truth

| Business Requirement | Status | Evidence |
|---------------------|--------|----------|
| Winner gets shipping via normal options | ✅ CORRECT | `AuctionClaimShippingModal` uses same delivery check |
| Seller can provide Private Quote to winner | ⚠️ PARTIAL | Chat-based quote works IF seller-winner chat exists. But no automatic chat creation. |
| System opens/creates seller↔winner chat | ❌ NOT IMPLEMENTED | Seller must manually find winner or winner must initiate chat |
| Seller can initiate quote without existing chat | ❌ NOT IMPLEMENTED | `POST /chat/:chat_id/shipping-quote` requires existing chat_id |
| Winner can pay before shipping resolved | ⚠️ UNCLEAR | Winner must select shipping option in claim modal. If no options exist, claim is blocked. But no path to use a shipping quote during claim. |

### 7.3 Critical Gap: Auction Winner + Private Quote

The business truth requires:
> "Auction ends → Winner determined → Seller can access 'Give Shipping Quote' → Labuda opens/creates seller ↔ winner commerce chat → Auction/product card is available → Seller creates Private Shipping Quote"

**Current implementation:**
- Winner claim flow ONLY supports standard shipping options (AuctionClaimShippingModal)
- No UI entry point for seller to create a shipping quote for an auction winner
- No automatic chat creation between seller and winner
- If winner is outside all coverage areas, claim is blocked with no fallback

This is a **P1 gap** for the auction winner use case.

---

## 8. CHAT INTEGRATION

### 8.1 Chat Commerce Architecture

```
ChatRoom
    ├── ContextJSON: optional commerce context (forSale/auction preview)
    ├── ContextSetBy: who set it
    └── LinkedOrderID: order linked for commerce continuity
```

### 8.2 Shipping Quote in Chat

- **Creation:** Seller taps "Buat Ongkir" in chat → `ShippingQuoteCreationModal` → `POST /chat/:chat_id/shipping-quote`
- **Display:** `shipping_quote` message type with attachment containing offer_id, rate, status, linked_item info
- **Purchase:** Buyer taps "Beli" on quote → navigates to CheckoutScreen with `shippingQuoteId`

### 8.3 Chat Context Requirements

| Requirement | Status |
|------------|--------|
| Product/auction card in chat | ✅ ContextJSON stores target type + ID |
| Seller entry point for quote creation | ✅ "Buat Ongkir" button in ChatInputArea |
| Quote creation requires chat context | ✅ Handler checks `chat.context.targetType == ShareTargetType.forSale` |
| Auction winner chat creation | ❌ NOT AUTOMATIC — seller must manually find winner |

---

## 9. PRICING/ORDER/PAYMENT FLOW

### 9.1 Shipping Amount Authority Chain

```
Shipping Option Coverage (province_rate / city_override rate)
    OR
Shipping Quote (cost)
    ↓
Pricing Token Generation (shipping_total = rate or quote cost)
    ↓
Order Creation (shipping_total from token snapshot)
    ↓
Payment (total_payable = escrow + service_fee; escrow = subtotal + shipping - discount)
    ↓
Seller Release (shipping_total transferred to seller wallet on completion)
```

### 9.2 Financial Integrity

| Aspect | Status |
|--------|--------|
| Unit consistency | ✅ All amounts in smallest currency unit (bigint = cents/rupiah) |
| Amount authority | ✅ Pricing token is single source of truth; order uses token values |
| Seller receives shipping fee | ✅ `order_completion_service.go` transfers shipping_total to seller |
| Buyer pays shipping | ✅ Included in total_payable_amount |
| Discount interaction | ✅ Discount applies to subtotal only, not shipping |
| Coin interaction | ✅ Coins deducted from escrow_amount (subtotal + shipping - discount) |
| No separate packing line | ✅ Shipping is one combined amount |

### 9.3 Naming Issues in Financial Flow

| Current Name | Business Truth | Classification |
|-------------|----------------|----------------|
| `shipping_total` | Combined shipping + packing cost | CANONICAL — the column IS the combined amount |
| `shipping_option_name` | Shipping method name | CANONICAL |
| `shipping_transport_type` | Method type | CANONICAL |
| `shipping_expedition_name` | **DROPPED** | Column still exists on orders but is legacy |
| `shipping_estimated_days` | ETA | CANONICAL but misleading — Labuda doesn't control logistics |
| `shipping_quote_price` | Quote amount snapshot | CANONICAL |
| `shipping_source` | "for_sale" or "shipping_quote" | CANONICAL |

---

## 10. MOBILE UI MAP

### 10.1 Seller Shipping Setup

| Screen | Purpose | Assessment |
|--------|---------|------------|
| `SellerShippingScreen` | List all shipping options | ✅ Correct — shows options with coverage count |
| `SellerShippingOptionDetailScreen` | View/edit option + coverages | ✅ Correct |
| `ShippingOptionSetupScreen` | Create/edit option + coverages | ⚠️ UI says "Tariff" not "Shipping + Packing" |
| `SellerShippingOptionsSelector` | Multi-select for product creation | ✅ Correct |

### 10.2 Product Creation

| Screen | Purpose | Assessment |
|--------|---------|------------|
| `CreateForSaleScreen` | Create For Sale with shipping selection | ✅ Correct — embeds SellerShippingOptionsSelector |
| `EditForSaleScreen` | Edit For Sale shipping selection | ✅ Correct |
| `CreateAuctionScreen` | Create Auction with shipping selection | ✅ Correct — same selector |

### 10.3 Buyer Checkout

| Screen | Purpose | Assessment |
|--------|---------|------------|
| `CheckoutScreen` | Full checkout flow | ✅ Correct — address → delivery options → pricing → order |
| `_ShippingOptionPickerSection` | Radio list of options | ✅ Correct |
| Out-of-area state | Shows message + Hubungi Penjual CTA | ✅ Correct |
| Seller-not-configured state | Shows different message | ✅ Correct |

### 10.4 Chat

| Screen | Purpose | Assessment |
|--------|---------|------------|
| `ChatDetailScreen` | Chat with "Buat Ongkir" button for sellers | ✅ Correct |
| `ShippingQuoteCreationModal` | Seller inputs cost + note | ⚠️ Label says "Biaya Ongkir" not "Biaya Pengiriman + Packing" |
| Quote purchase flow | Buyer taps "Beli" → checkout | ✅ Correct |

### 10.5 Auction Winner

| Screen | Purpose | Assessment |
|--------|---------|------------|
| `AuctionClaimShippingModal` | Winner selects address + shipping option | ⚠️ No path for private quote |
| `AuctionSellerSettlementMonitor` | Seller sees winner info | ❌ No "Give Shipping Quote" entry point |

---

## 11. NAMING AUDIT

### 11.1 Backend Naming

| Current Name | Business Truth | Classification | Recommendation |
|-------------|----------------|----------------|----------------|
| `ShippingOption` | Shipping Setup / Shipping Method | MISNAMED | Rename to `ShippingSetup` |
| `ShippingCoverage` | Coverage Area | CANONICAL | Keep |
| `CityOverride` | City Rule | CANONICAL | Keep |
| `ShippingService` | Delivery Availability Service | CANONICAL | Keep |
| `SellerShippingService` | Seller Shipping Setup Service | CANONICAL | Keep |
| `ProductShippingService` | Product Shipping Selection Service | CANONICAL | Keep |
| `ShippingQuoteService` | Private Shipping Quote Service | CANONICAL | Keep |
| `transport_type` | Method Type | CANONICAL | Keep values (bus/travel/etc.) |
| `expedition_name` | **DROPPED** | DEAD | Migration 000014 removed from DB, entity still has field |
| `internal_purpose` | Internal Note | CANONICAL | Keep |
| `province_rate` | Combined Shipping+Packing Cost | CANONICAL | Keep |

### 11.2 Mobile Naming

| Current Name | Business Truth | Classification | Recommendation |
|-------------|----------------|----------------|----------------|
| `ShippingOption` (entity) | Shipping Setup | MISNAMED | Rename to `ShippingSetup` |
| `ShippingType` (enum) | Method Type | CANONICAL | Keep values |
| `DeliveryOption` | Resolved Shipping Option | CANONICAL | Keep |
| `ShippingCoverage` | Coverage Area | CANONICAL | Keep |
| `CityShippingRate` | City Rule | CANONICAL | Keep |
| `ShippingRateResult` | Resolved Rate | CANONICAL | Keep |
| `ShippingOptionSetupScreen` | Shipping Setup Screen | CANONICAL | Keep |
| `SellerShippingOptionsSelector` | Seller Shipping Setup Selector | CANONICAL | Keep |
| `ShippingQuoteCreationModal` | Private Quote Creation Modal | CANONICAL | Keep |

### 11.3 Database Naming

| Current Name | Business Truth | Classification |
|-------------|----------------|----------------|
| `shipping_options` | Shipping Setups | MISNAMED |
| `shipping_coverages` | Coverage Areas | CANONICAL |
| `shipping_city_overrides` | City Rules | CANONICAL |
| `product_shipping_options` | Product Shipping Selections | CANONICAL |
| `shipping_quotes` | Private Shipping Quotes | CANONICAL |
| `shipping_total` | Combined Shipping+Packing Cost | CANONICAL |

---

## 12. DUPLICATE AUTHORITY / RESIDUE

### 12.1 Dead Code

| File/Table | Status | Action |
|-----------|--------|--------|
| `listing_shipping_option_repository_impl.go` | DEAD — `listing_shipping_options` table dropped in migration 000016 | DELETE |
| `ShippingOption.ExpeditionName` field | DEAD — column dropped in migration 000014 | DELETE from entity |
| `CityOverride.EstimatedDays` field | DEAD — column dropped in migration 000014 | DELETE from entity |
| `ShippingCoverage.EstimatedDays` field | DEAD — column dropped in migration 000014 | DELETE from entity |
| `shipping_city_overrides.price` column | DEAD — `rate` is the canonical column | DROP in future migration |
| `shipping_quotes.auction_id` column | LEGACY — `source_type`/`source_id` replaced this | DROP in future migration |
| `orders.shipping_expedition_name` column | LEGACY — expedition_name dropped | DROP in future migration |

### 12.2 Compatibility Layers

| Layer | Status |
|-------|--------|
| `deliveryOptionToResponse` in shipping_handler.go | ACTIVE — converts DeliveryOption to API response |
| `shippingQuoteToResponse` in shipping_quote_handler.go | ACTIVE — converts quote to API response |
| Legacy substring matching in mobile error handler | ACTIVE — fallback for older clients |

### 12.3 Stale Documentation

| File | Status |
|------|--------|
| `SHIPPING_DOMAIN_SCHEMA.md` | STALE — still references `expedition_name` and `estimated_days` |
| `shipping_honesty_messages.dart` | STALE — uses "listing" terminology |

---

## 13. BUSINESS MISMATCHES

### 13.1 P0 — None

### 13.2 P1 — Auction Winner + Private Quote Gap

**Business Truth:** After auction ends, seller must be able to give a Private Shipping Quote to the winner, with automatic chat creation.

**Current Reality:** No automatic chat creation. No seller entry point for auction winner quotes. Winner can only use standard shipping options via claim modal. If winner is outside all coverage, claim is blocked with no fallback.

### 13.3 P2 — Mismatches

| ID | Description | Severity |
|----|-------------|----------|
| P2-1 | Shipping quote destination address lock is optional — business truth says quote should be tied to destination | P2 |
| P2-2 | No UI entry point for seller to proactively offer special shipping price within normal coverage | P2 |
| P2-3 | `ShippingOption` entity name is misleading — should be `ShippingSetup` | P2 |
| P2-4 | UI labels say "Tariff" / "Biaya Ongkir" instead of "Biaya Pengiriman + Packing" | P2 |
| P2-5 | `shipping_city_overrides.price` is a dead column alongside canonical `rate` | P2 |
| P2-6 | `shipping_quotes.auction_id` is a legacy column superseded by `source_type`/`source_id` | P2 |

### 13.4 P3 — Hygiene

| ID | Description | Severity |
|----|-------------|----------|
| P3-1 | `SHIPPING_DOMAIN_SCHEMA.md` references dropped columns | P3 |
| P3-2 | `listing_shipping_option_repository_impl.go` is dead code | P3 |
| P3-3 | Mobile `shipping_honesty_messages.dart` uses "listing" terminology | P3 |
| P3-4 | Backend comments reference "listing" in shipping context | P3 |
| P3-5 | `orders.shipping_expedition_name` is a legacy column | P3 |

---

## 14. PROPOSED CANONICAL ARCHITECTURE

### 14.1 Naming Changes (Recommended)

| Current | Proposed | Scope |
|---------|----------|-------|
| `ShippingOption` | `ShippingSetup` | Entity, DB table, all references |
| `shipping_options` table | `shipping_setups` table | DB migration |
| `transport_type` | `method_type` | Enum, DB column |
| `province_rate` | `combined_cost` | DB column (clarifies it's shipping+packing) |
| `Biaya Ongkir` (UI) | `Biaya Pengiriman + Packing` | Mobile UI |
| `Tariff` (UI) | `Biaya` | Mobile UI |

### 14.2 Schema Cleanup (Recommended)

| Action | Target | Reason |
|--------|--------|--------|
| DROP | `shipping_city_overrides.price` | Dead column, `rate` is canonical |
| DROP | `shipping_quotes.auction_id` | Legacy, `source_type`/`source_id` replaced |
| DROP | `orders.shipping_expedition_name` | Legacy, expedition_name dropped |
| DROP | `ShippingOption.ExpeditionName` Go field | Column dropped in migration |
| DROP | `CityOverride.EstimatedDays` Go field | Column dropped in migration |
| DROP | `ShippingCoverage.EstimatedDays` Go field | Column dropped in migration |

### 14.3 New Features Required

| Feature | Description | Priority |
|---------|-------------|----------|
| Auction winner → seller chat auto-creation | When auction ends, system creates seller↔winner chat with auction context | P1 |
| Seller "Give Shipping Quote" for auction winner | Entry point on auction settlement monitor | P1 |
| Address lock on shipping quote (default) | Quote should require destination address | P2 |
| Seller proactive special price entry point | Within coverage, seller can offer special price via quote | P2 |
| "Shipping + Packing" UI guidance | Instruct sellers to enter combined amount | P2 |

---

## 15. ITEMS TO PRESERVE

1. **Product-scoped shipping linking** — `product_shipping_options` overwrite model is correct
2. **Province + city override coverage model** — Clean, simple, sufficient
3. **Shipping quote supersession model** — One active quote per context, new supersedes old
4. **Shipping quote anti-tamper protections** — 9 validation checks in `validateShippingQuoteForOrder`
5. **FOR UPDATE locking** — Race condition prevention on quotes and sale surfaces
6. **Pricing token as single source of truth** — All pricing flows through token, no live recalculation
7. **Shipping quote reactivation lifecycle** — Quote reuse after order failure with limits
8. **Product shipping configuration gate** — `EnsureShippingConfigured` at publish time
9. **Delivery availability check** — Province→City cascade with product-scoped options
10. **Out-of-area UX** — Correct distinction between "seller not configured" and "outside coverage"

---

## 16. ITEMS TO DELETE

1. `listing_shipping_option_repository_impl.go` — Dead code
2. `ShippingOption.ExpeditionName` Go field — Column dropped
3. `CityOverride.EstimatedDays` Go field — Column dropped
4. `ShippingCoverage.EstimatedDays` Go field — Column dropped
5. `shipping_city_overrides.price` DB column — Dead, `rate` is canonical
6. `shipping_quotes.auction_id` DB column — Legacy
7. `orders.shipping_expedition_name` DB column — Legacy
8. Stale `SHIPPING_DOMAIN_SCHEMA.md` — References dropped columns

---

## 17. ITEMS REQUIRING REDESIGN

1. **Auction winner → seller chat auto-creation** — New backend service needed
2. **Seller "Give Shipping Quote" for auction winner** — New UI entry point + backend flow
3. **Naming: `ShippingOption` → `ShippingSetup`** — Cross-cutting rename (DB, backend, mobile)
4. **Naming: `transport_type` → `method_type`** — Cross-cutting rename
5. **UI guidance for combined shipping+packing cost** — Instruction text updates
6. **Address lock on shipping quote** — Make destination required or strongly encouraged

---

## 18. OWNER DECISIONS REQUIRED

### Decision 1: Naming Convention

**Question:** Should `ShippingOption` be renamed to `ShippingSetup` across the entire codebase?

**Options:**
- A) Full rename (DB table, Go entities, Dart entities, API fields) — clean but large scope
- B) Keep current name, add documentation that "ShippingOption" means "ShippingSetup" — smaller scope
- C) Rename only in new code, leave existing code — inconsistent

**Recommendation:** Option A (full rename) — this is a from-zero project with no production compatibility requirement.

### Decision 2: Auction Winner Chat Auto-Creation

**Question:** Should the system automatically create a seller↔winner chat when an auction ends?

**Options:**
- A) Auto-create chat on auction end — seller sees winner in chat list immediately
- B) Auto-create chat on seller "Give Shipping Quote" action — lazy creation
- C) Require manual chat creation — current behavior

**Recommendation:** Option A — matches business truth: "Labuda opens/creates the seller ↔ winner commerce chat"

### Decision 3: Shipping Quote Address Lock

**Question:** Should the destination address be required when creating a shipping quote?

**Options:**
- A) Required — seller must lock destination at quote creation
- B) Optional but encouraged — current behavior
- C) Required for For Sale, optional for Auction

**Recommendation:** Option A — matches business truth: quote is transaction-specific with locked destination

### Decision 4: Special Shipping Price Within Coverage

**Question:** Should sellers be able to offer a special shipping price via quote even when normal coverage exists?

**Options:**
- A) Yes — seller can create quote from chat at any time
- B) Only when buyer is outside coverage — current behavior
- C) Yes, but only from order detail screen

**Recommendation:** Option A — matches business truth: "seller wants to offer a special shipping price"

---

## 19. RECOMMENDED IMPLEMENTATION SEQUENCE

### Phase 1: Schema Cleanup (No Behavior Change)
1. Drop dead columns: `shipping_city_overrides.price`, `shipping_quotes.auction_id`, `orders.shipping_expedition_name`
2. Drop dead Go fields: `ExpeditionName`, `EstimatedDays` from shipping entities
3. Delete dead file: `listing_shipping_option_repository_impl.go`
4. Update `SHIPPING_DOMAIN_SCHEMA.md`

### Phase 2: Naming Alignment
1. Rename `ShippingOption` → `ShippingSetup` (DB migration + Go + Dart)
2. Rename `transport_type` → `method_type` (DB migration + Go + Dart)
3. Update UI labels: "Tariff" → "Biaya", "Biaya Ongkir" → "Biaya Pengiriman + Packing"

### Phase 3: Auction Winner Chat
1. Backend: Auto-create seller↔winner chat on auction end
2. Backend: Add `POST /auctions/:id/give-shipping-quote` endpoint
3. Mobile: Add "Buat Ongkir" button on `AuctionSellerSettlementMonitor`
4. Mobile: Allow shipping quote creation from auction context

### Phase 4: Shipping Quote Hardening
1. Make destination address required on shipping quote
2. Add seller proactive special price entry point in chat
3. Update UI guidance for combined shipping+packing cost

---

## 20. FINAL VERDICT

**SHIPPING-01 — REDESIGN REQUIRED**

The core shipping architecture is sound. The product-scoped linking model, province+city coverage, and shipping quote supersession are well-designed. However:

1. **P1 Gap:** Auction winner has no path to receive a private shipping quote from seller. The system does not auto-create seller↔winner chat, and there is no seller entry point for auction winner quotes.

2. **P2 Naming Debt:** `ShippingOption` should be `ShippingSetup` to match business truth. UI labels mislead sellers about what to enter.

3. **P2 Dead Columns:** Multiple dead columns and fields exist from prior migrations that should be cleaned up.

4. **P2 Missing Feature:** No entry point for sellers to offer special shipping prices within normal coverage.

The redesign should focus on:
- Auction winner chat auto-creation (P1)
- Naming alignment (P2)
- Schema cleanup (P2)
- Shipping quote address lock (P2)
- Special price entry point (P2)

No P0 findings. The financial flow is correct and well-protected.
