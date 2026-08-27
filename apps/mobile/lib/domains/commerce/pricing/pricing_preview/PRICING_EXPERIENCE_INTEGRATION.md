# Pricing Experience Implementation - Integration Guide

> **PASS_21C STALENESS NOTICE:** this document is a pre-implementation
> planning doc (note the unchecked `- [ ]` testing checklist and "routes to
> add" section below) — it does not reflect the current wire contract or
> navigation flow. In particular:
> - The real order-create/pricing-preview contract uses `product_id` +
>   `source_type` (`fixed_price_sale`|`auction`) + `source_id`, never a
>   shared `listing_id` — `listing_id` is explicitly REJECTED by the
>   backend (see `order_create_contract_test.go`
>   `TestCreateOrder_RejectsLegacyListingID`).
> - The real auction buy-now flow (`auction_detail_screen.dart:_handleBuyNow`)
>   navigates to `/checkout/:id?product_id=...&auction_id=...`, not
>   `/order-preview` with `listingId`+`auctionId` extras as shown below.
> Treat the `listingId`-based examples below as historical design intent,
> not current truth. This doc needs a full rewrite against the real
> checkout/auction-detail screens — tracked as remaining debt, not done in
> this pass to avoid guessing at a rewrite this audit didn't independently
> verify end-to-end.

## Overview

This document describes the implementation of transparent pricing UI and flow for the Labuda marketplace. The implementation follows **STRICT MODE** principles:

❌ **NO frontend calculations**
❌ **NO business logic in UI**
✅ **All pricing from backend**
✅ **Display only - read once, show twice**

---

## Architecture

### Backend Flow

```
1. POST /api/v1/pricing/preview
   → Request: product_id, source_type, source_id, quantity, shipping_option_id, address_id, discount_code
   → Response: { token, expires_at, pricing_snapshot }

2. POST /api/v1/orders
   → Request: product_id, source_type (fixed_price_sale|auction), source_id, pricing_token, shipping_address, negotiation_id?
   → Response: { order_id, payment_url, total_amount }
```

Note: `listing_id` is REJECTED by the order-create endpoint (see
`order_create_contract_test.go` `TestCreateOrder_RejectsLegacyListingID`) —
Listing and Auction are canonical siblings identified by `product_id` +
`source_type`/`source_id`, never by a shared `listing_id`.

### Frontend Flow

```
1. User Action (Buy Now / Negotiation Accepted)
   ↓
2. OrderPreviewScreen
   → Calls pricing preview API
   → Displays PricingBreakdown
   → Shows trust labels
   ↓
3. User Confirms
   ↓
4. CheckoutScreen
   → Creates order with pricing_token
   → Opens payment URL
```

---

## Components Created

### 1. Data Models

#### `PricingSnapshot` Entity
**Location:** `apps/mobile/lib/domains/commerce/pricing/pricing_preview/domain/entities/pricing_snapshot.dart`

**Purpose:** Complete pricing breakdown from backend

**Key Fields:**
- `token` - Required for order creation
- `expiresAt` - Token validity period
- `unitPrice` - Base price per unit
- `quantity` - Number of items
- `subtotal` - Unit price × quantity
- `shippingTotal` - Shipping cost
- `discountAmount` - Applied discount
- `escrowAmount` - Total to pay

**Rules:**
- All fields are read-only
- No calculations in UI
- Token required for order creation

#### `PricingPreviewResponseDto`
**Location:** `apps/mobile/lib/domains/commerce/pricing/pricing_preview/data/dto/pricing_preview_dto.dart`

**Purpose:** API response deserialization

**Methods:**
- `fromJson()` - Parse backend response
- `toEntity()` - Convert to PricingSnapshot

---

### 2. UI Components

#### `PricingBreakdown` Widget
**Location:** `apps/mobile/lib/domains/commerce/pricing/pricing_preview/presentation/widgets/pricing_breakdown.dart`

**Purpose:** Display complete pricing breakdown

**Features:**
- Item price (unit × quantity)
- Subtotal
- Shipping cost
- Discount (if applied)
- Commission (optional)
- Total amount

**Usage:**
```dart
PricingBreakdown(
  snapshot: pricingSnapshot,
  isNegotiatedPrice: true, // For negotiation checkout
  showCommission: false, // For buyer view
)
```

#### `PricingTrustLabels` Widget
**Location:** Same as PricingBreakdown

**Purpose:** Display trust badges

**Badges:**
- "Harga Nego" (Negotiated price)
- "Max koin 20%" (Max coins)
- "Diskon max 50%" (Max discount)

**Usage:**
```dart
PricingTrustLabels(
  isNegotiatedPrice: true, // Show negotiated price badge
  showCoinsInfo: true, // Show coins limit
  showDiscountInfo: false, // Hide discount for negotiation
)
```

#### `TokenExpiryWidget`
**Location:** Same as PricingBreakdown

**Purpose:** Display token expiry countdown

**Features:**
- Shows remaining time
- Warning when < 3 minutes
- Refresh button
- Error state when expired

---

### 3. Screens

#### `OrderPreviewScreen`
**Location:** `apps/mobile/lib/domains/commerce/pricing/pricing_preview/presentation/screens/order_preview_screen.dart`

**Purpose:** Complete order preview before confirmation

**Flow:**
1. Load pricing preview from backend
2. Display pricing breakdown
3. Show trust labels
4. Show token expiry
5. User confirms → Navigate to checkout

**Parameters:**
- `listingId` - Required
- `negotiationId` - Optional (for negotiation checkout)
- `auctionId` - Optional (for auction checkout)
- `shippingQuoteId` - Optional (for seller quote)

**Usage:**
```dart
context.push('/order-preview', extra: {
  'listingId': 'xxx',
  'negotiationId': 'yyy', // Optional
  'returnToChat': 'zzz', // Optional
});
```

---

### 4. Chat Integration

#### `NegotiationAcceptedAction` Widget
**Location:** `apps/mobile/lib/domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_accepted_action.dart`

**Purpose:** Show "Buy Now" button when negotiation is accepted

**Features:**
- Displays agreed price
- Shows savings from original price
- Trust badges (Harga Terkunci, Valid 10 Menit)
- "Beli Sekarang" button → OrderPreviewScreen

**Usage in Chat:**
```dart
if (negotiation.status == NegotiationStatus.accepted) {
  NegotiationAcceptedAction(
    negotiation: negotiation,
    chatId: chatId,
  )
}
```

---

### 5. Error Handling

#### `PricingErrorStateWidget`
**Location:** `apps/mobile/lib/domains/commerce/pricing/pricing_preview/presentation/widgets/pricing_error_states.dart`

**Purpose:** User-friendly error messages

**Error Types:**
- `listingSold` - Barang sudah terjual
- `listingNotFound` - Barang tidak ditemukan
- `negotiationExpired` - Negosiasi habis waktu
- `tokenExpired` - Waktu harga habis
- `priceMismatch` - Harga berubah
- `networkError` - Koneksi bermasalah

**Usage:**
```dart
PricingErrorStateWidget(
  errorType: PricingErrorType.tokenExpired,
  onRetry: () => loadPricingPreview(),
  onBack: () => context.pop(),
)
```

**Dialog Variant:**
```dart
PricingErrorDialog.show(
  context,
  errorType: PricingErrorType.listingSold,
  onBack: () => context.pop(),
);
```

**Banner Variant:**
```dart
PricingErrorBanner(
  errorType: PricingErrorType.priceMismatch,
  onRetry: () => refreshPricing(),
  onDismiss: () => dismissError(),
)
```

---

## Integration Points

### 1. From Listing Detail

**When user clicks "Beli Sekarang":**

```dart
onTap: () {
  context.push('/order-preview', extra: {
    'listingId': listing.id,
  });
}
```

### 2. From Chat (Negotiation Accepted)

**When seller accepts negotiation:**

```dart
// In ChatDetailScreen or ChatInputArea
if (negotiation.status == NegotiationStatus.accepted) {
  NegotiationAcceptedAction(
    negotiation: negotiation,
    chatId: chatId,
  )
}
```

### 3. From Auction (Winner Claim)

**When auction winner claims victory:**

```dart
context.push('/order-preview', extra: {
  'listingId': listing.id,
  'auctionId': auction.id,
});
```

---

## Backend Integration Required

### 1. Pricing Preview API

**Endpoint:** `POST /api/v1/pricing/preview`

**Request:**
```json
{
  "listing_id": "uuid",
  "quantity": 1,
  "shipping_option_id": "uuid",
  "address_id": "uuid",
  "discount_code": "CODE123"
}
```

**Response:**
```json
{
  "token": "uuid",
  "expires_at": "2024-01-01T10:00:00Z",
  "pricing_snapshot": {
    "unit_price": 100000,
    "quantity": 1,
    "subtotal": 100000,
    "shipping_total": 15000,
    "commission_percent": 5.0,
    "commission_amount": 5000,
    "discount_amount": 0,
    "escrow_amount": 120000,
    "listing_id": "uuid",
    "address_id": "uuid"
  }
}
```

### 2. Order Creation API

**Endpoint:** `POST /api/v1/orders`

**Request:**
```json
{
  "listing_id": "uuid",
  "pricing_token": "uuid",
  "quantity": 1,
  "shipping_address": { ... },
  "negotiation_id": "uuid"
}
```

**Response:**
```json
{
  "id": "uuid",
  "order_number": "ORD123",
  "payment_url": "https://payment.gateway.com/...",
  "total_amount": 120000,
  "status": "pending",
  "created_at": "2024-01-01T10:00:00Z"
}
```

---

## Navigation Routes to Add

Add these routes to your router configuration:

```dart
// Order Preview Screen
GoRoute(
  path: '/order-preview',
  builder: (context, state) {
    final extra = state.extra as Map<String, dynamic>;
    return OrderPreviewScreen(
      listingId: extra['listingId'],
      negotiationId: extra['negotiationId'],
      auctionId: extra['auctionId'],
      shippingQuoteId: extra['shippingQuoteId'],
      returnToChat: extra['returnToChat'],
    );
  },
),
```

---

## Testing Checklist

### Unit Tests
- [ ] `PricingSnapshot` entity validation
- [ ] `PricingPreviewResponseDto` JSON parsing
- [ ] Token expiry calculation
- [ ] Currency formatting

### Widget Tests
- [ ] `PricingBreakdown` renders correctly
- [ ] `PricingTrustLabels` displays badges
- [ ] `TokenExpiryWidget` shows countdown
- [ ] `PricingErrorStateWidget` shows error messages
- [ ] `NegotiationAcceptedAction` displays agreed price

### Integration Tests
- [ ] Navigation from listing to order preview
- [ ] Navigation from chat to order preview
- [ ] Pricing preview API call
- [ ] Order creation with pricing token
- [ ] Error states (sold, expired, mismatch)

### E2E Tests
- [ ] Complete flow: Listing → Preview → Order → Payment
- [ ] Negotiation flow: Chat → Accepted → Buy Now → Preview → Order
- [ ] Auction flow: Win → Claim → Preview → Order
- [ ] Error recovery: Token expiry → Refresh → Order

---

## Key Principles

### ✅ DO
- Display pricing from backend only
- Show complete breakdown
- Display trust labels
- Handle errors gracefully
- Use pricing tokens for order creation

### ❌ DON'T
- Calculate prices in UI
- Store prices locally
- Reuse expired tokens
- Hide pricing details
- Make business decisions in UI

---

## Future Enhancements

### Phase 2
- [ ] Pricing preview for multiple quantities
- [ ] Compare prices (original vs negotiated)
- [ ] Price history graph
- [ ] Promotional pricing badges

### Phase 3
- [ ] Dynamic pricing (surge pricing)
- [ ] Bundle pricing
- [ ] Tiered discounts
- [ ] Loyalty pricing

---

## Support

For questions or issues:
1. Check backend API documentation
2. Verify pricing token validity
3. Review error messages
4. Check network connectivity

---

**Last Updated:** 2026-04-24
**Version:** 1.0.0
**Status:** ✅ Implementation Complete
