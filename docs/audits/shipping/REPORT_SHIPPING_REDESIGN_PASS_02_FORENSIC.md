# SHIPPING-02 PASS 02 — SHIPPING DOMAIN AUTHORITY & SCHEMA CLEANUP FORENSIC AUDIT

**Status:** CLEANUP REQUIRED  
**Date:** 2026-09-02  
**Auditor:** Buffy (Codebuff)  
**Scope:** Complete field-level authority trace, schema/code alignment, monetary invariant proof

---

## 1. DATABASE FORENSICS — FIELD-BY-FIELD AUTHORITY PROOF

### 1.1 shipping_options

| Column | Type | Nullable | Migration Status | Go Entity Field | Repository Reads | Repository Writes | Business Truth | Classification |
|--------|------|----------|-----------------|----------------|-----------------|-------------------|----------------|----------------|
| id | uuid | NOT NULL | CANONICAL | ID | All queries | Create | Required | CANONICAL |
| seller_id | uuid | NOT NULL | CANONICAL | SellerID | All queries | Create | Required | CANONICAL |
| name | text | NOT NULL | CANONICAL | Name | All queries | Create/Update | Required | CANONICAL |
| transport_type | shipping_transport_type_enum | NOT NULL | CANONICAL | TransportType | All queries | Create/Update | Required | CANONICAL |
| expedition_name | text | NULL | **DROPPED (migration 000014)** | ExpeditionName | **STILL READ in shipping_option_repository_impl.go** | **STILL WRITTEN in shipping_option_repository_impl.go** | Not required | **DEAD — MISMATCH** |
| is_active | boolean | NOT NULL | CANONICAL | IsActive | All queries | Create/Update | Required | CANONICAL |
| internal_purpose | text | NULL | CANONICAL | **NOT IN ENTITY** | Not read by Go code | Not written by Go code | Seller internal note | **CANONICAL — ENTITY MISSING** |
| created_at | timestamptz | NOT NULL | CANONICAL | CreatedAt | All queries | Create | Required | CANONICAL |
| updated_at | timestamptz | NOT NULL | CANONICAL | UpdatedAt | All queries | Create/Update | Required | CANONICAL |

**CRITICAL FINDING (P0):** `expedition_name` was DROPPED from the database by migration 000014, but `shipping_option_repository_impl.go` still references it in every SQL query (Create, Update, GetByID, GetForUpdate, GetBySeller, GetByName). This repository is used by `SellerShippingService` for all seller CRUD operations. If migration 000014 was applied, every seller shipping option creation/edit would fail with "column expedition_name does not exist".

**Evidence:**
- Migration 000014: `ALTER TABLE shipping_options DROP COLUMN IF EXISTS expedition_name;`
- `shipping_option_repository_impl.go` line 30: `INSERT INTO shipping_options (..., expedition_name, ...)`
- `shipping_option_repository_impl.go` line 60: `SET name = $2, transport_type = $3, expedition_name = $4,`
- Integration test `seller_shipping_handler_contract_test.go` line 80: `INSERT INTO shipping_options (..., expedition_name, ...)`

**Note:** `listing_shipping_option_repository_impl.go` (actually `ProductShippingOptionRepositoryImpl`) CORRECTLY omits `expedition_name` from its SELECT queries and sets `ExpeditionName: nil` in scans. This suggests the product-shipping query path was fixed but the seller CRUD path was not.

**Note 2:** `internal_purpose` exists in the DB schema but has NO corresponding Go entity field. The `ShippingOption` entity does not have an `InternalPurpose` field. The repository does not read or write it. The mobile `CreateShippingOptionRequest` does not send it. This column is CANONICAL in the DB but has no Go representation.

### 1.2 shipping_coverages

| Column | Type | Nullable | Migration Status | Go Entity Field | Repository Reads | Repository Writes | Business Truth | Classification |
|--------|------|----------|-----------------|----------------|-----------------|-------------------|----------------|----------------|
| id | uuid | NOT NULL | CANONICAL | ID | All queries | Create | Required | CANONICAL |
| shipping_option_id | uuid | NOT NULL | CANONICAL | ShippingOptionID | All queries | Create | Required | CANONICAL |
| province_code | text | NOT NULL | CANONICAL | ProvinceCode | All queries | Create | Required | CANONICAL |
| province_name | text | NULL | CANONICAL | ProvinceName | All queries | Create/Update | Required | CANONICAL |
| province_rate | bigint | NOT NULL | CANONICAL | ProvinceRate | All queries | Create/Update | Combined shipping+packing cost | CANONICAL |
| estimated_days | text | NULL | **DROPPED (migration 000014)** | EstimatedDays | **STILL READ in shipping_coverage_repository_impl.go** | **STILL WRITTEN in shipping_coverage_repository_impl.go** | Not required | **DEAD — MISMATCH** |
| is_available | boolean | NOT NULL | CANONICAL | IsAvailable | All queries | Create/Update | Required | CANONICAL |
| created_at | timestamptz | NOT NULL | CANONICAL | CreatedAt | All queries | Create | Required | CANONICAL |

**CRITICAL FINDING (P0):** `estimated_days` was DROPPED from `shipping_coverages` by migration 000014, but `shipping_coverage_repository_impl.go` still references it in Create, Update, GetByID, and GetByShippingOption queries. Only `GetByOptionAndProvince` correctly sets `EstimatedDays: nil`.

**Evidence:**
- Migration 000014: `ALTER TABLE shipping_coverages DROP COLUMN IF EXISTS estimated_days;`
- `shipping_coverage_repository_impl.go` line 32: `INSERT INTO shipping_coverages (..., estimated_days, ...)`
- `shipping_coverage_repository_impl.go` line 61: `SET province_name = $2, province_rate = $3, estimated_days = $4,`
- `shipping_coverage_repository_impl.go` line 95: `SELECT ..., estimated_days, ... FROM shipping_coverages`

### 1.3 shipping_city_overrides

| Column | Type | Nullable | Migration Status | Go Entity Field | Repository Reads | Repository Writes | Business Truth | Classification |
|--------|------|----------|-----------------|----------------|-----------------|-------------------|----------------|----------------|
| id | uuid | NOT NULL | CANONICAL | ID | All queries | Create | Required | CANONICAL |
| shipping_coverage_id | uuid | NOT NULL | CANONICAL | ShippingCoverageID | All queries | Create | Required | CANONICAL |
| city_code | text | NOT NULL | CANONICAL | CityCode | All queries | Create | Required | CANONICAL |
| city_name | text | NULL | CANONICAL | CityName | All queries | Create/Update | Required | CANONICAL |
| price | bigint | NULL | CANONICAL (but dead) | **NOT IN ENTITY** | Not read by Go code | Not written by Go code | **DEAD — rate is canonical** | **DEAD COLUMN** |
| rate | bigint | NULL | CANONICAL | Rate | All queries | Create/Update | Combined shipping+packing cost | CANONICAL |
| estimated_days | text | NULL | **DROPPED (migration 000014)** | EstimatedDays | **STILL READ in city_override_repository_impl.go** | **STILL WRITTEN in city_override_repository_impl.go** | Not required | **DEAD — MISMATCH** |
| is_available | boolean | NOT NULL | CANONICAL | IsAvailable | All queries | Create/Update | Required | CANONICAL |
| created_at | timestamptz | NOT NULL | CANONICAL | CreatedAt | All queries | Create | Required | CANONICAL |
| updated_at | timestamptz | NOT NULL | CANONICAL | UpdatedAt | All queries | Create/Update | Required | CANONICAL |

**CRITICAL FINDING (P0):** `estimated_days` was DROPPED from `shipping_city_overrides` by migration 000014, but `city_override_repository_impl.go` still references it in Create, Update, GetByID, GetByCoverage, and GetByCoverageAndCity queries.

**Evidence:**
- Migration 000014: `ALTER TABLE shipping_city_overrides DROP COLUMN IF EXISTS estimated_days;`
- `city_override_repository_impl.go` line 22: `INSERT INTO shipping_city_overrides (..., estimated_days, ...)`
- `city_override_repository_impl.go` line 40: `SET city_name = $2, rate = $3, estimated_days = $4,`
- `city_override_repository_impl.go` line 57: `SELECT ..., estimated_days, ... FROM shipping_city_overrides`

**FINDING (P2):** `price` is a DEAD COLUMN on `shipping_city_overrides`. The Go code uses `rate` exclusively. `price` has no Go entity field, no reader, no writer.

### 1.4 product_shipping_options

| Column | Type | Nullable | Go Entity Field | Repository Reads | Repository Writes | Business Truth | Classification |
|--------|------|----------|----------------|-----------------|-------------------|----------------|----------------|
| product_id | uuid | NOT NULL | N/A (link table) | All queries | Create/CreateBulk/Delete | Required | CANONICAL |
| shipping_option_id | uuid | NOT NULL | N/A (link table) | All queries | Create/CreateBulk/Delete | Required | CANONICAL |
| sort_order | integer | NOT NULL | N/A (link table) | Not read | Create/CreateBulk | Optional display order | CANONICAL |
| created_at | timestamptz | NOT NULL | N/A (link table) | Not read | Create/CreateBulk | Required | CANONICAL |

**Status:** CLEAN — no dead fields, no mismatches.

### 1.5 shipping_quotes

| Column | Type | Nullable | Migration Status | Go Entity Field | Repository Reads | Repository Writes | Business Truth | Classification |
|--------|------|----------|-----------------|----------------|-----------------|-------------------|----------------|----------------|
| id | uuid | NOT NULL | CANONICAL | ID | All queries | Create | Required | CANONICAL |
| chat_id | uuid | NOT NULL | CANONICAL | ChatID | All queries | Create | Required | CANONICAL |
| product_id | uuid | NOT NULL | CANONICAL | ProductID | All queries | Create | Required | CANONICAL |
| source_type | sale_surface_type_enum | NOT NULL | CANONICAL | SourceType | All queries | Create | Required | CANONICAL |
| source_id | uuid | NOT NULL | CANONICAL | SourceID | All queries | Create | Required | CANONICAL |
| auction_id | uuid | NULL | **LEGACY** | AuctionID | **STILL IN ENTITY** | **STILL WRITTEN by NewAuctionShippingQuote** | **NOT REQUIRED — source_type/source_id replaced** | **LEGACY — MISMATCH** |
| seller_id | uuid | NOT NULL | CANONICAL | SellerID | All queries | Create | Required | CANONICAL |
| buyer_id | uuid | NOT NULL | CANONICAL | BuyerID | All queries | Create | Required | CANONICAL |
| cost | bigint | NOT NULL | CANONICAL | Cost | All queries | Create | Combined shipping+packing quote | CANONICAL |
| note | text | NULL | CANONICAL | Note | All queries | Create | Optional seller note | CANONICAL |
| status | shipping_quote_status_enum | NOT NULL | CANONICAL | Status | All queries | Create/Update | Required | CANONICAL |
| superseded_at | timestamptz | NULL | CANONICAL | SupersededAt | All queries | Update | Required | CANONICAL |
| superseded_by_id | uuid | NULL | CANONICAL | SupersededByID | All queries | Update | Required | CANONICAL |
| destination_city_id | text | NULL | CANONICAL | DestinationCityID | All queries | Create | Optional address lock | CANONICAL |
| destination_province_id | text | NULL | CANONICAL | DestinationProvinceID | All queries | Create | Optional address lock | CANONICAL |
| used_at | timestamptz | NULL | CANONICAL | UsedAt | All queries | Update | Required | CANONICAL |
| expires_at | timestamptz | NOT NULL | CANONICAL | ExpiresAt | All queries | Create | Required | CANONICAL |
| reactivation_count | integer | NOT NULL | CANONICAL | ReactivationCount | All queries | Update | Required | CANONICAL |
| max_reuse | integer | NOT NULL | CANONICAL | MaxReuse | All queries | Create | Required | CANONICAL |
| created_at | timestamptz | NOT NULL | CANONICAL | CreatedAt | All queries | Create | Required | CANONICAL |

**FINDING (P2):** `auction_id` is a LEGACY column. The `NewAuctionShippingQuote` function in `shipping_quote_service.go` STILL writes to it. The entity still has the field. But `source_type` + `source_id` replaced it. The column is nullable and no production code reads it for business logic.

**Evidence:**
- `shipping_quote_service.go` line ~200: `quote = shippingQuoteEntity.NewAuctionShippingQuote(...)` which sets `AuctionID`
- `shipping_quote.go` entity: `AuctionID *uuid.UUID` field still exists
- No production reader uses `AuctionID` for validation or business logic

### 1.6 orders shipping columns

| Column | Type | Nullable | Go Entity Field | Writer | Reader | Business Truth | Classification |
|--------|------|----------|----------------|--------|--------|----------------|----------------|
| shipping_option_id | uuid | NULL | ShippingOptionID | OrderCreationService | OrderQueryService | Snapshot — NULL when using quote | CANONICAL |
| shipping_option_name | text | NULL | ShippingOptionName | OrderCreationService | OrderQueryService | Snapshot | CANONICAL |
| shipping_transport_type | text | NULL | ShippingTransportType | OrderCreationService | OrderQueryService | Snapshot | CANONICAL |
| shipping_expedition_name | text | NULL | ShippingExpeditionName | OrderCreationService | OrderQueryService | **LEGACY — expedition_name dropped** | **LEGACY — ALWAYS NULL** |
| shipping_estimated_days | text | NULL | ShippingEstimatedDays | OrderCreationService | OrderQueryService | **SNAPSHOT — always nil since source dropped** | **LEGACY — ALWAYS NULL** |
| shipping_total | bigint | NOT NULL | ShippingTotal | OrderCreationService | OrderQueryService, RefundService, CompletionService | Combined shipping+packing amount | CANONICAL |
| shipping_quote_id | uuid | NULL | ShippingQuoteID | OrderCreationService | OrderQueryService | Snapshot — NULL when using standard shipping | CANONICAL |
| shipping_quote_price | bigint | NULL | ShippingQuotePrice | OrderCreationService | OrderQueryService | Snapshot of quote cost | CANONICAL |
| shipping_source | text | NULL | ShippingSource | OrderCreationService | OrderQueryService | "for_sale" or "shipping_quote" | CANONICAL |
| shipping_destination | jsonb | NULL | ShippingDestination | OrderCreationService | OrderQueryService | Address snapshot | CANONICAL |
| shipping_origin_snapshot | jsonb | NULL | ShippingOriginSnapshot | OrderCreationService | OrderQueryService | Farm address snapshot | CANONICAL |

**FINDING (P2):** `orders.shipping_expedition_name` and `orders.shipping_estimated_days` are LEGACY columns. They are still written by `OrderCreationService` (copying from pricing token snapshot), but the source data is always nil because the shipping source columns were dropped. These columns exist on every order but are always NULL.

**Evidence:**
- `order_creation_service.go` line 928: `snapshot.ShippingExpeditionName` — always nil
- `order_creation_service.go` line 929: `snapshot.ShippingEstimatedDays` — always nil
- Pricing token: `ShippingExpeditionName` and `ShippingEstimatedDays` are set from `shippingOption.ExpeditionName` and coverage `estimatedDays`, both of which are nil after migration 000014

### 1.7 pricing_token shipping fields

| Column | Type | Nullable | Go Entity Field | Writer | Reader | Business Truth | Classification |
|--------|------|----------|----------------|--------|--------|----------------|----------------|
| shipping_option_id | uuid | NULL | ShippingOptionID | PricingTokenService | OrderCreationService | Snapshot | CANONICAL |
| shipping_option_name | text | NULL | ShippingOptionName | PricingTokenService | OrderCreationService | Snapshot | CANONICAL |
| shipping_transport_type | text | NULL | ShippingTransportType | PricingTokenService | OrderCreationService | Snapshot | CANONICAL |
| shipping_expedition_name | text | NULL | ShippingExpeditionName | PricingTokenService | OrderCreationService | **LEGACY — always nil** | **LEGACY** |
| shipping_estimated_days | text | NULL | ShippingEstimatedDays | PricingTokenService | OrderCreationService | **LEGACY — always nil** | **LEGACY** |
| shipping_total | bigint | NOT NULL | ShippingTotal | PricingTokenService | OrderCreationService | Combined shipping+packing amount | CANONICAL |
| shipping_quote_id | uuid | NULL | ShippingQuoteID | PricingTokenService | OrderCreationService | Snapshot | CANONICAL |
| shipping_source | text | NULL | ShippingSource | PricingTokenService | OrderCreationService | "for_sale" or "shipping_quote" | CANONICAL |

---

## 2. BACKEND AUTHORITY TRACE

### 2.1 Single Authority per Stage

| Stage | Canonical Service | Authority |
|-------|------------------|-----------|
| Seller CRUD | `SellerShippingService` | Creates/updates `shipping_options` + `shipping_coverages` |
| Product linking | `ProductShippingService` | Creates/deletes `product_shipping_options` |
| Availability check | `ShippingService.CheckDeliveryAvailability` | Reads `product_shipping_options` + `shipping_coverages` + `shipping_city_overrides` |
| Pricing token | `PricingTokenService` | Reads shipping option + coverage to calculate `shipping_total` |
| Order creation | `OrderCreationService` | Validates shipping, copies snapshot to order |
| Quote creation | `ShippingQuoteService` | Creates `shipping_quotes`, sends chat message |
| Quote validation | `OrderCreationService.validateShippingQuoteForOrder` | Validates quote anti-tamper, marks USED |

### 2.2 Duplicate Authority Check

| Question | Answer | Evidence |
|----------|--------|----------|
| Is shipping availability authoritative in one service? | **YES** | `ShippingService.CheckDeliveryAvailability` is the single read-only authority |
| Is quote validation authoritative in one service? | **YES** | `OrderCreationService.validateShippingQuoteForOrder` is the single validation authority |
| Does checkout independently reconstruct shipping rules? | **NO** | Checkout calls `ShippingService.CheckDeliveryAvailabilityForProduct` which delegates to the canonical service |
| Does order creation independently reconstruct them? | **NO** | Order creation uses the same `ShippingService` for validation |
| Does pricing token independently reconstruct them? | **YES — but correctly** | `PricingTokenService` reads shipping option + coverage to calculate cost. This is the SINGLE point where shipping cost is calculated. Order creation uses the token's pre-calculated value. |
| Are there any client-provided shipping prices accepted as authority? | **NO** | Pricing token is server-authoritative. Shipping quote cost comes from DB, not client input. |

**VERDICT: No duplicate authority issues.** The financial flow is correct.

### 2.3 Client Price Manipulation Check

**Can a buyer manipulate shipping price?**

- Pricing token generation: `PricingTokenService` reads `shippingOption.TransportType` and `coverage.ProvinceRate` from DB. Client provides `shipping_option_id` and `address_id` only. The `shipping_total` is calculated server-side.
- Shipping quote: Cost comes from `shipping_quotes.cost` which is set by the seller via `ShippingQuoteService.CreateShippingQuote`. The buyer cannot set the cost.
- Order creation: Uses `PricingSnapshot.ShippingTotal` from the validated token. No recalculation.

**VERDICT: Client cannot manipulate shipping price.** All monetary values are server-authoritative.

---

## 3. SHIPPING SETUP SEMANTICS

### 3.1 What ShippingOption Currently Really Is

`ShippingOption` is a **seller-level reusable shipping configuration**. It represents:
- A named shipping method (e.g., "Bus ke Jateng")
- A transport type category (train/bus/travel/plane/custom)
- Province-level coverage with rates
- City-level rate overrides
- Active/inactive toggle
- Internal note for seller reference

It is **NOT**:
- A product-specific option (that's `product_shipping_options`)
- A delivery method (Labuda doesn't control logistics)
- A shipping tariff (it's a setup that INCLUDES a tariff)

### 3.2 Name Accuracy Assessment

| Current Name | Real Meaning | Risk of Mistake | Rename Justified? |
|-------------|--------------|-----------------|-------------------|
| `ShippingOption` | Shipping Setup/Method | Medium — "option" implies buyer-facing choice, but this is seller configuration | **YES** — "option" suggests it's a buyer selection, but it's a seller setup |
| `transport_type` | Method Type | Low — values are clear (bus/travel/etc.) | NO — values are self-documenting |
| `expedition_name` | **DROPPED** | N/A | N/A — already dropped from DB |
| `internal_purpose` | Internal Note | Low — clear | NO |
| `province_rate` | Combined Shipping+Packing Cost | Medium — "rate" doesn't communicate "combined" | **YES** — should be `combined_cost` or `shipping_packing_cost` |

### 3.3 Rename Justification Criteria

A rename is justified if the current name **materially risks future implementation mistakes** or **contradicts the canonical domain model**.

- `ShippingOption` → `ShippingSetup`: **JUSTIFIED** — "option" implies buyer-facing choice. Future agents may incorrectly assume ShippingOption is what buyers select, when it's actually seller-level configuration. The buyer selects from `product_shipping_options` linked setups.
- `province_rate` → `combined_cost`: **JUSTIFIED** — "rate" doesn't communicate that this includes packing. Future agents may create separate packing cost fields.
- `transport_type` → `method_type`: **NOT JUSTIFIED** — "transport_type" is clear enough. The enum values (bus/travel/etc.) are self-documenting.

---

## 4. COMBINED COST INVARIANT

### 4.1 Monetary Meaning Proof

| Field | Canonical Meaning | Evidence |
|-------|-------------------|----------|
| `shipping_coverages.province_rate` | Combined shipping + packing cost | Business truth: "The cost is ONE number. It already includes shipping + packing." |
| `shipping_city_overrides.rate` | Combined shipping + packing cost (city override) | Same meaning as province_rate, overridden per city |
| `shipping_quotes.cost` | Combined shipping + packing quote | Business truth: "Quote amount is ONE combined Shipping + Packing amount." |
| `orders.shipping_total` | Combined shipping + packing snapshot | Copied from pricing token |
| `pricing_tokens.shipping_total` | Combined shipping + packing (authoritative) | Calculated from coverage rate or quote cost |

### 4.2 Where Amount Is Entered

| Entry Point | Field | Authority |
|------------|-------|-----------|
| Seller creates coverage | `shipping_coverages.province_rate` | Seller-entered, stored in DB |
| Seller creates city override | `shipping_city_overrides.rate` | Seller-entered, stored in DB |
| Seller creates shipping quote | `shipping_quotes.cost` | Seller-entered, stored in DB |

### 4.3 Where Amount Is Displayed

| Display Point | Source | Label |
|--------------|--------|-------|
| Seller coverage creation | `province_rate` | "Tariff" / "Biaya" (mobile UI) |
| Buyer delivery options | `DeliveryOption.rate` | Rate displayed as currency |
| Checkout order summary | `pricing_token.shipping_total` | "Shipping + Packing" (mobile) |
| Order detail | `order.shipping_total` | "Shipping" |

### 4.4 Where Amount Is Calculated

| Calculation Point | Formula | Authority |
|------------------|---------|-----------|
| Pricing token generation | `shippingTotal = coverage.ProvinceRate` (or city override rate, or quote cost) | Server-side from DB |
| Order creation | `shippingTotal = token.ShippingTotal` | Copy from token (no recalculation) |
| Payment amount | `totalPayable = escrowAmount + serviceFeeAmount` where `escrowAmount = subtotal + shipping - discount` | Server-side from order |

### 4.5 Where Amount Is Refunded

| Refund Point | Formula | Evidence |
|-------------|---------|----------|
| Full refund | `refundAmount = order.Subtotal + order.ShippingTotal` | `order_completion_service.go` |
| Partial refund | Proportional to quantity | Same file |

### 4.6 Separate Packing Assumption Check

**Is there any assumption that shipping and packing are separate?**

- No `packing_cost` field exists anywhere in the schema
- No UI exposes packing as a separate line item
- The checkout summary shows "Shipping + Packing" as one line
- The pricing token calculates one `shipping_total`
- The order stores one `shipping_total`

**VERDICT: No separate packing assumption exists.** The combined cost invariant is correctly maintained.

### 4.7 UI Label Assessment

| UI Label | Location | Semantic Accuracy |
|----------|----------|-------------------|
| "Tariff" | `ShippingOptionSetupScreen` | **MISLEADING** — implies government/official rate, not combined cost |
| "Biaya Ongkir" | `ShippingQuoteCreationModal` | **PARTIALLY ACCURATE** — "ongkir" means shipping cost, but doesn't mention packing |
| "Shipping + Packing" | Checkout order summary | **CORRECT** |
| "Shipping" | Order detail | **ACCEPTABLE** — context makes it clear |

**VERDICT:** "Tariff" is the most misleading label. "Biaya Ongkir" is acceptable in Indonesian context where "ongkir" colloquially includes packing.

---

## 5. PRIVATE QUOTE IDENTITY

### 5.1 Uniqueness Constraint

The DB has a UNIQUE index:
```sql
CREATE UNIQUE INDEX uq_shipping_quotes_current_active_context
    ON shipping_quotes USING btree (chat_id, product_id, source_type, source_id, seller_id, buyer_id)
    WHERE (status = 'ACTIVE' AND superseded_at IS NULL);
```

This enforces: **ONE ACTIVE QUOTE PER (chat_id, product_id, source_type, source_id, seller_id, buyer_id)**

### 5.2 Identity Components

| Component | Required | Purpose |
|-----------|----------|---------|
| chat_id | YES | Scopes quote to specific conversation |
| product_id | YES | Scopes quote to specific product |
| source_type | YES | for_sale / auction / negotiation |
| source_id | YES | Specific sale surface ID |
| seller_id | YES | Seller identity |
| buyer_id | YES | Buyer identity |

### 5.3 Concurrency Behavior

**Race prevention:**
1. `SupersedeCurrentQuotes` marks all prior unsuperseded quotes as superseded before inserting new one
2. `GetByIDForUpdate` locks the quote row with FOR UPDATE during order creation
3. UNIQUE index prevents duplicate active quotes at DB level

**Can two different buyers have different quotes for the same product?**
**YES** — buyer_id is part of the uniqueness constraint.

**Can two different commerce contexts have different quotes?**
**YES** — chat_id, source_type, source_id are part of the constraint.

### 5.4 Destination Lock Enforcement

| Layer | Enforced? | Evidence |
|-------|-----------|----------|
| Quote creation | **OPTIONAL** | `DestinationCityID` and `DestinationProvinceID` are optional in `CreateShippingQuoteInput` |
| Quote entity | **OPTIONAL** | `ValidateDestinationAddress` returns nil if both are nil |
| Order creation | **OPTIONAL** | `validateShippingQuoteForOrder` calls `quote.ValidateDestinationAddress` which passes if both are nil |
| Pricing token | **NOT CHECKED** | Token generation doesn't check destination lock |

**FINDING (P2):** Destination lock is structurally supported but OPTIONAL. A quote can be created without locking destination. This means a quote created for Jakarta could theoretically be used for a Bandung address.

---

## 6. QUOTE LIFECYCLE

### 6.1 State Transitions

```
CREATE → ACTIVE (default)
ACTIVE → USED (MarkQuoteUsed, during order creation)
ACTIVE → EXPIRED (MarkQuoteExpired, timeout/background)
ACTIVE → INVALID (InvalidateQuotesByProduct, when product sold)
ACTIVE → SUPERSEDED (SupersedeCurrentQuotes, when new quote created for same context)
USED → ACTIVE (ReactivateQuoteIfEligible, when order fails/expires)
```

### 6.2 State Assessment

| State | Required by Business? | Evidence |
|-------|----------------------|----------|
| ACTIVE | YES | Quote is available for use |
| USED | YES | Quote consumed by order, prevents reuse |
| EXPIRED | YES | Quote has time limit (24h default, 7d max) |
| INVALID | YES | Product no longer available (sold/withdrawn) |
| SUPERSEDED | YES | Newer quote replaced this one |

### 6.3 Reactivation Assessment

| Field | Required? | Evidence |
|-------|-----------|----------|
| `reactivation_count` | YES | Prevents infinite reuse after order failures |
| `max_reuse` | YES | Hard limit (default: 2) |
| `expires_at` | YES | Time-bounded quote validity |
| `used_at` | YES | Timestamp of when quote was consumed |

**VERDICT: All lifecycle fields are currently required.** The reactivation mechanism handles order failure/expiry gracefully.

---

## 7. ORDER/PRICING SNAPSHOT AUTHORITY

### 7.1 Authority Chain

```
DB: shipping_options + shipping_coverages + shipping_city_overrides
    ↓
PricingTokenService: calculates shipping_total
    ↓
pricing_tokens: stores snapshot (authoritative for this transaction)
    ↓
OrderCreationService: copies token values to order (immutable snapshot)
    ↓
orders: stores historical snapshot (never recalculated)
```

### 7.2 Snapshot Fields on Order

| Field | Source | Immutable After Creation? | Can Influence Money? |
|-------|--------|--------------------------|---------------------|
| shipping_option_id | PricingToken | YES | NO — display only |
| shipping_option_name | PricingToken | YES | NO — display only |
| shipping_transport_type | PricingToken | YES | NO — display only |
| shipping_expedition_name | PricingToken (always nil) | YES | NO — always nil |
| shipping_estimated_days | PricingToken (always nil) | YES | NO — always nil |
| shipping_total | PricingToken | YES | **YES — used in refund calculation** |
| shipping_quote_id | PricingToken | YES | NO — audit trail |
| shipping_quote_price | PricingToken | YES | NO — audit trail |
| shipping_source | PricingToken | YES | NO — display only |

### 7.3 Dead Snapshot Fields

`shipping_expedition_name` and `shipping_estimated_days` on orders are ALWAYS NULL because:
1. Migration 000014 dropped the source columns from shipping_options/shipping_coverages
2. Pricing token copies nil values
3. Order creation copies nil values from token

These fields exist on every order row but contain no data.

---

## 8. AUCTION GAP — STRUCTURAL AUDIT

### 8.1 Current Auction Winner Path

```
Auction ends → status = waiting_settlement
    ↓
Winner taps "Klaim Sekarang"
    ↓
AuctionClaimShippingModal:
    1. Winner selects address
    2. Fetches delivery options via checkDeliveryAvailability
    3. Winner selects shipping option
    4. Claim callback → POST /auctions/:id/claim
    ↓
Backend: AuctionService.ClaimWinner
    ↓
Creates order via OrderCreationService.CreateFromAuction
    ↓
Winner navigates to payment
```

### 8.2 What Prevents Auction Winner → Private Quote Flow

| Requirement | Current Status | Blocker |
|-------------|---------------|---------|
| Seller opens winner commerce chat | **NOT IMPLEMENTED** | No automatic chat creation on auction end |
| Seller creates private shipping quote | **STRUCTURALLY POSSIBLE** | `ShippingQuoteService.CreateShippingQuote` accepts `AuctionID` parameter |
| Winner accepts quote | **STRUCTURALLY POSSIBLE** | `_handleShippingQuotePurchase` handles auction quotes |
| Checkout with quote | **STRUCTURALLY POSSIBLE** | `CreateFromAuction` supports `shippingQuoteId` |

### 8.3 Exact Blockers

1. **No automatic chat creation:** When auction ends, no chat room is created between seller and winner. The seller must manually find the winner or the winner must initiate chat.

2. **No seller entry point:** The `AuctionSellerSettlementMonitor` widget shows winner info but has no "Buat Ongkir" button. The seller has no UI to initiate a quote for the winner.

3. **Quote requires existing chat_id:** `POST /chat/:chat_id/shipping-quote` requires a `chat_id` parameter. Without an existing chat, the seller cannot create a quote.

### 8.4 Structural Capability

**Is the current shipping_quote infrastructure structurally capable of supporting auction winner quotes WITHOUT redesigning the quote domain?**

**YES.** The quote domain is already designed to handle auction quotes:
- `source_type = "auction"` is a valid value
- `AuctionID` field exists (though legacy)
- `NewAuctionShippingQuote` exists
- `validateAuctionForQuote` validates auction state
- Checkout supports `shippingQuoteId` for auction orders

**The only missing piece is the chat creation and seller entry point.** The quote domain itself is ready.

---

## 9. LISTING RESIDUE / ANTI-RESURRECTION

### 9.1 Active Runtime Risk

| Finding | Severity | Evidence |
|---------|----------|----------|
| None identified | — | All listing references in shipping context are dead code or comments |

### 9.2 Dead Code

| File | Status | Action |
|------|--------|--------|
| `listing_shipping_option_repository_impl.go` | DEAD — filename misleading, content is `ProductShippingOptionRepositoryImpl` | RENAME file |
| `listing_shipping_coverage_validation_test.go` | DEAD — filename misleading, content tests `ValidateSellableCreateShippingSelection` | RENAME file |

### 9.3 Documentation-Only Residue

| Location | Content | Classification |
|----------|---------|----------------|
| Backend comments referencing "listing" in shipping context | Multiple files | COSMETIC |
| Mobile `shipping_honesty_messages.dart` | Uses "listing" in constant names | COSMETIC |
| `checkout_screen_impl.dart` line 807 | User-facing string "listing ini" | **P2 — USER-FACING** |

### 9.4 Old Listing-Owns-Auction Architecture

**No active code references the old listing-owns-auction architecture in the shipping domain.** The migration 000016 dropped `listing_shipping_options` table. Migration 000047 renamed `fixed_price_sale` to `for_sale` in enums.

---

## 10. MOBILE CONTRACT TRACE

### 10.1 API Field Inventory

| API Endpoint | Request Fields | Response Fields | Status |
|-------------|---------------|-----------------|--------|
| POST /shipping/options | name, transport_type, expedition_name | shipping_option (full) | **expedition_name sent but DB column dropped** |
| GET /shipping/options | — | shipping_options[] | **expedition_name returned but DB column dropped** |
| GET /shipping/options/:id | — | shipping_option + coverages[] | **expedition_name returned but DB column dropped** |
| PUT /shipping/options/:id | name, transport_type, expedition_name, is_active | shipping_option | **expedition_name sent but DB column dropped** |
| POST /shipping/options/:id/coverages | province_code, province_name, rate, estimated_days, is_available | coverage | **estimated_days sent but DB column dropped** |
| PUT /shipping/coverages/:id | province_name, rate, estimated_days, is_available | coverage | **estimated_days sent but DB column dropped** |
| POST /shipping/check | product_id, province_code, city_code | options[], product_configured | CLEAN |
| POST /chat/:chat_id/shipping-quote | product_id, source_type, source_id, cost, note, destination_* | quote | CLEAN |

### 10.2 Stale DTO Fields

| Mobile Field | API Field | DB Column | Status |
|-------------|-----------|-----------|--------|
| `ShippingOption.expeditionName` | `expedition_name` | DROPPED | **STALE — will be nil from server** |
| `ShippingCoverage.estimatedDays` | `estimated_days` | DROPPED | **STALE — will be nil from server** |
| `CityShippingRate.estimatedDays` | `estimated_days` | DROPPED | **STALE — will be nil from server** |
| `CreateCoverageRequest.estimatedDays` | `estimated_days` | DROPPED | **STALE — server ignores** |
| `UpdateCoverageRequest.estimatedDays` | `estimated_days` | DROPPED | **STALE — server ignores** |

### 10.3 Mobile ShippingOption vs Business Truth

The mobile `ShippingOption` entity has:
- `type: ShippingType` (enum: train/bus/travel/plane/custom)
- `expeditionName` (nullable)
- `coverageAreas: List<ShippingCoverage>`
- `internalNote` (nullable)

The mobile entity does NOT have an `internalNote` field in the API response (the DB has `internal_purpose` but it's not exposed via API). This is correct — internal notes are seller-only.

**Does mobile still think ShippingOption means "courier/expedition"?**

Partially. The `ShippingType` enum values (train/bus/travel/plane/custom) are method types, not courier names. But `expeditionName` (stale) and the `transportType` naming suggest courier/expedition thinking. The business truth is that these are seller-defined shipping methods, not carrier integrations.

---

## 11. TEST / NEGATIVE CONTRACT AUDIT

### 11.1 Existing Tests

| Test | Proves | Status |
|------|--------|--------|
| `shipping_service_test.go` | CheckDeliveryAvailability uses product_id | ✅ PASS |
| `listing_shipping_coverage_validation_test.go` | ValidateSellableCreateShippingSelection: empty IDs, option not found, wrong seller, zero coverages, all inactive, has active coverage, multiple options | ✅ PASS |
| `shipping_quote_service_test.go` | Quote creation, validation, lifecycle | ✅ PASS |
| `shipping_quote_auction_validation_test.go` | Auction-specific quote validation | ✅ PASS |
| `shipping_quote_expiry_test.go` | Quote expiry logic | ✅ PASS |
| `shipping_quote_reactivation_test.go` | Quote reactivation after order failure | ✅ PASS |
| `shipping_quote_handler_auth_test.go` | HTTP handler authorization | ✅ PASS |
| `shipping_quote_race_condition_test.go` | FOR UPDATE lock behavior | ✅ PASS |
| `order_creation_service_shipping_quote_expiry_test.go` | Order creation with expired quotes | ✅ PASS |
| `order_creation_service_shipping_quote_idempotency_test.go` | Double-order prevention | ✅ PASS |
| `seller_shipping_handler_contract_test.go` | Seller handler contract | ⚠️ **USES DROPPED COLUMN** |
| `for_sale_shipping_coverage_test.go` | For sale shipping coverage validation | ✅ PASS |
| `checkout_shipping_out_of_coverage_widget_test.dart` | Out-of-coverage UI rendering | ✅ PASS |
| `checkout_shipping_fallback_contract_test.dart` | Checkout preserves IDs | ✅ PASS |

### 11.2 Missing Tests

| Missing Test | Priority |
|-------------|----------|
| Backend: ShippingQuoteService unit test for CreateShippingQuote happy path | P2 |
| Backend: Full shipping quote lifecycle integration test | P2 |
| Backend: For Sale checkout with shipping quote (end-to-end) | P2 |
| Backend: Shipping quote supersession behavior test | P3 |
| Mobile: Checkout screen test for shipping quote checkout flow | P3 |
| Mobile: Chat detail screen test for shipping quote creation | P3 |

### 11.3 Broken Test

**`seller_shipping_handler_contract_test.go`** line 80 inserts `expedition_name` into `shipping_options`:
```sql
INSERT INTO shipping_options (id, seller_id, name, transport_type, expedition_name, is_active, created_at, updated_at)
```
This test would FAIL if migration 000014 was applied to the test database.

---

## 12. FINAL CLASSIFICATION TABLE

| # | Finding | Layer | Evidence | Severity | Canonical Truth | Required Action |
|---|---------|-------|----------|----------|-----------------|-----------------|
| 1 | `shipping_option_repository_impl.go` references dropped `expedition_name` column in all SQL queries | Backend/DB | Lines 30, 60, 93, 136, 174, 239 | **P0** | Column dropped by migration 000014 | Remove `expedition_name` from all queries and entity |
| 2 | `shipping_coverage_repository_impl.go` references dropped `estimated_days` column in Create/Update/GetByID/GetByShippingOption | Backend/DB | Lines 32, 61, 95, 130 | **P0** | Column dropped by migration 000014 | Remove `estimated_days` from all queries and entity |
| 3 | `city_override_repository_impl.go` references dropped `estimated_days` column in Create/Update/GetByID/GetByCoverage/GetByCoverageAndCity | Backend/DB | Lines 22, 40, 57, 85, 115 | **P0** | Column dropped by migration 000014 | Remove `estimated_days` from all queries and entity |
| 4 | `seller_shipping_handler_contract_test.go` inserts `expedition_name` into `shipping_options` | Test/DB | Line 80 | **P1** | Column dropped by migration 000014 | Fix test to not reference dropped column |
| 5 | `ShippingOption.ExpeditionName` Go entity field has no DB column | Entity | shipping_option.go line 14 | **P1** | Column dropped by migration 000014 | Remove field from entity |
| 6 | `ShippingCoverage.EstimatedDays` Go entity field has no DB column | Entity | shipping_coverage.go line 16 | **P1** | Column dropped by migration 000014 | Remove field from entity |
| 7 | `CityOverride.EstimatedDays` Go entity field has no DB column | Entity | city_override.go line 14 | **P1** | Column dropped by migration 000014 | Remove field from entity |
| 8 | `shipping_quotes.auction_id` is legacy, still written by `NewAuctionShippingQuote` | Backend | shipping_quote_service.go | **P2** | `source_type`/`source_id` replaced it | Stop writing, plan DROP |
| 9 | `shipping_city_overrides.price` is dead column | DB | Canonical schema | **P2** | `rate` is canonical | Plan DROP |
| 10 | `orders.shipping_expedition_name` always NULL | DB/Backend | Order creation copies nil | **P2** | Source dropped | Plan DROP |
| 11 | `orders.shipping_estimated_days` always NULL | DB/Backend | Order creation copies nil | **P2** | Source dropped | Plan DROP |
| 12 | `pricing_tokens.shipping_expedition_name` always NULL | DB/Backend | Token copies nil | **P2** | Source dropped | Plan DROP |
| 13 | `pricing_tokens.shipping_estimated_days` always NULL | DB/Backend | Token copies nil | **P2** | Source dropped | Plan DROP |
| 14 | Destination lock on shipping quote is optional | Backend | CreateShippingQuoteInput | **P2** | Business truth says quote should lock destination | Make required or strongly encouraged |
| 15 | `ShippingOption` name misleads as buyer-facing option | Naming | Entity/service naming | **P2** | It's seller-level configuration | Rename to `ShippingSetup` |
| 16 | `province_rate` doesn't communicate combined cost | Naming | DB column name | **P2** | It's shipping+packing combined | Rename to `combined_cost` |
| 17 | UI says "Tariff" instead of combined cost | Mobile UI | ShippingOptionSetupScreen | **P2** | Should say "Biaya Pengiriman + Packing" | Update label |
| 18 | `listing_shipping_option_repository_impl.go` filename misleading | File | Filename | **P3** | Content is ProductShippingOptionRepositoryImpl | Rename file |
| 19 | `listing_shipping_coverage_validation_test.go` filename misleading | File | Filename | **P3** | Content tests ValidateSellableCreateShippingSelection | Rename file |
| 20 | User-facing string "listing ini" in checkout | Mobile | checkout_screen_impl.dart:807 | **P2** | Should say "produk ini" | Update string |
| 21 | `shipping_honesty_messages.dart` uses "listing" | Mobile | Constant names | **P3** | Should use "product" | Update constants |
| 22 | `SHIPPING_DOMAIN_SCHEMA.md` references dropped columns | Docs | Schema doc | **P3** | Columns dropped | Update doc |
| 23 | Mobile `estimated_days` fields stale | Mobile | DTOs, entities | **P3** | Source dropped | Clean up |
| 24 | Mobile `expeditionName` field stale | Mobile | DTOs, entities | **P3** | Source dropped | Clean up |
| 25 | No automatic seller↔winner chat for auction | Backend/Mobile | Auction settlement | **P1** | Business truth requires auto-creation | Implement |
| 26 | No seller "Give Shipping Quote" for auction winner | Mobile | AuctionSellerSettlementMonitor | **P1** | Business truth requires entry point | Implement |

---

## 13. DESIGN READINESS

### Q1: Is current shipping domain structurally sound enough to implement the new Auction winner quote flow?

**YES.** The quote domain already supports auction quotes (`source_type = "auction"`, `NewAuctionShippingQuote`, `validateAuctionForQuote`). The only missing pieces are:
1. Automatic chat creation on auction end
2. Seller "Give Shipping Quote" entry point

These are **integration gaps**, not domain redesigns.

### Q2: Which fields MUST be removed?

**MUST REMOVE (P0 — code is broken against schema):**
- `ShippingOption.ExpeditionName` Go field + all SQL references
- `ShippingCoverage.EstimatedDays` Go field + all SQL references
- `CityOverride.EstimatedDays` Go field + all SQL references

**MUST REMOVE (P2 — dead columns):**
- `shipping_city_overrides.price` DB column
- `shipping_quotes.auction_id` DB column (stop writing first)
- `orders.shipping_expedition_name` DB column
- `orders.shipping_estimated_days` DB column
- `pricing_tokens.shipping_expedition_name` DB column
- `pricing_tokens.shipping_estimated_days` DB column

### Q3: Which fields MUST stay?

- `shipping_options.id, seller_id, name, transport_type, is_active, internal_purpose, created_at, updated_at`
- `shipping_coverages.id, shipping_option_id, province_code, province_name, province_rate, is_available, created_at`
- `shipping_city_overrides.id, shipping_coverage_id, city_code, city_name, rate, is_available, created_at, updated_at`
- `product_shipping_options.product_id, shipping_option_id, sort_order, created_at`
- All `shipping_quotes` columns except `auction_id`
- All `orders` shipping columns except `shipping_expedition_name` and `shipping_estimated_days`

### Q4: Which names genuinely need renaming?

| Current | Proposed | Justified? |
|---------|----------|-----------|
| `ShippingOption` | `ShippingSetup` | **YES** — "option" implies buyer-facing choice |
| `province_rate` | `combined_cost` | **YES** — "rate" doesn't communicate combined shipping+packing |
| `transport_type` | `method_type` | **NO** — "transport_type" is clear enough |

### Q5: Is ShippingOption → ShippingSetup justified?

**YES.** The name "ShippingOption" materially risks future implementation mistakes because:
- It implies a buyer-facing choice (like "JNE Reguler vs SiCepat")
- But it's actually a seller-level reusable configuration
- Future agents may incorrectly create buyer selection logic on ShippingOption directly
- The actual buyer selection happens through `product_shipping_options` linking

### Q6: Is transport_type → method_type justified?

**NO.** The enum values (train/bus/travel/plane/custom) are self-documenting. "transport_type" is clear enough. The rename would be cosmetic.

### Q7: Is province_rate → combined_cost justified?

**YES.** "rate" doesn't communicate that this includes packing. Future agents may create separate packing cost fields. "combined_cost" or "shipping_packing_cost" makes the invariant explicit.

### Q8: Is shipping_quotes.auction_id definitely dead?

**YES.** `source_type = "auction"` + `source_id = auction_id` replaced it. The `AuctionID` field is still written by `NewAuctionShippingQuote` but no production code reads it for business logic. It should be stopped from being written and eventually dropped.

### Q9: Are order/pricing-token shipping snapshots correct?

**YES.** The snapshot fields are correctly copied from the pricing token to the order. The only issue is that `shipping_expedition_name` and `shipping_estimated_days` are always NULL (dead snapshots from dropped source columns).

### Q10: Is quote identity/uniqueness correct?

**YES.** The UNIQUE index on `(chat_id, product_id, source_type, source_id, seller_id, buyer_id) WHERE status=ACTIVE AND superseded_at IS NULL` correctly enforces one active quote per commerce context.

### Q11: Is destination locking strong enough?

**STRUCTURALLY YES, ENFORCEMENT NO.** The `ValidateDestinationAddress` method exists and is called during order creation. But the destination fields are optional on quote creation. A quote can be created without locking destination, which means it could be used for any address.

### Q12: What exact changes are P1 vs P2/P3?

**P0 (3 findings):** Repository code references dropped DB columns — code is structurally broken
**P1 (4 findings):** Broken test, missing entity fields, auction winner chat gap, auction seller entry point
**P2 (11 findings):** Dead columns, legacy fields, naming issues, optional destination lock, user-facing string
**P3 (6 findings):** File renaming, doc updates, mobile stale fields, cosmetic naming

### Q13: What should be implemented next?

**Phase 1 (P0 fix):** Remove `expedition_name`/`estimated_days` from Go entities, repositories, handlers, and tests. This is a BLOCKING fix — the code is broken against the current schema.

**Phase 2 (P1 fix):** Fix `seller_shipping_handler_contract_test.go` to not reference dropped columns. Implement automatic seller↔winner chat creation. Add seller "Give Shipping Quote" entry point.

**Phase 3 (P2 cleanup):** Drop dead DB columns. Rename `ShippingOption` → `ShippingSetup`. Update UI labels. Make destination lock required on quotes.

**Phase 4 (P3 cleanup):** Rename files. Update docs. Clean mobile stale fields.

---

## VERDICT

**SHIPPING-02 — CLEANUP REQUIRED**

The business authority is correct. The schema/code alignment is broken (P0: 3 findings). The domain is structurally sound for auction winner quotes. Cleanup must happen before implementation proceeds.
