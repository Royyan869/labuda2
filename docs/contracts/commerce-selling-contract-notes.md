# Commerce Selling Contract Notes

> **Status:** CANONICAL v1
>
> **Scope:** implementation contract notes for product, fixed-price listing, auction, draft, and Buy Now.

Related Documents:
- [Commerce Selling Doctrine](../flows/doctrine/commerce-selling-doctrine.md)
- [Commerce DB / Model Split Design](./commerce-db-model-split-design.md)

## Purpose

This document records the implementation-facing contract boundaries for the locked commerce doctrine. It does not define runtime logic. It defines what future implementations MUST preserve.

## Contract Boundary

- `Product` is the internal physical item authority.
- `FixedPriceListing` is the fixed-price sale surface.
- `Auction` is the bidding sale surface.
- Create listing and create auction MUST require active selling authority.
- KYC or payout approval MUST NOT gate create listing or create auction.
- Backend remains the final authority for canonical order and payment flow.

## Canonical Create Shapes

- `POST /api/v1/products`
- `PATCH /api/v1/products/:id`
- `POST /api/v1/fixed-price-listings`
- `POST /api/v1/auctions`
- `POST /api/v1/mobile/fixed-price-listings`
- `POST /api/v1/mobile/auctions`

The mobile convenience endpoints MAY combine product creation with the selected sale surface, but they MUST still preserve the canonical split internally.

## Auction Create Contract

Auction create requests MUST accept:

- `product_id`
- `title`
- `description`
- `start_price`
- `bid_increment`
- `buy_now_price` nullable
- `start_at`
- `end_at`

Auction create requests MUST NOT require:

- `listing_id`
- a preexisting fixed-price listing

`buy_now_price` MUST be optional. If present, backend validation MUST enforce the canonical floor rule. Recommended validation is:

`buy_now_price >= start_price + bid_increment`

## Draft Contract

- Draft is first-class for both product and sale surfaces.
- Draft MAY be persisted as a product draft plus a sale-surface draft.
- Seller MAY stop mid-flow and save draft.
- Publish is the explicit handoff from draft to active commerce state.

## UX Contract

- Visible create flow SHOULD be sale-intent-first.
- The user-facing labels SHOULD stay aligned to:
  - `Buat Jualan`
  - `Harga Tetap`
  - `Lelang`
  - `Simpan Draft`
  - `Publish`
- The visible flow MUST NOT require an inventory-first prerequisite.
- The visible flow MUST NOT imply that auction depends on a fixed-price listing.

## Implementation Non-Goals

- No compatibility shim.
- No old signature support.
- No auction `listing_id` acceptance.
- No tests-as-business-truth.
- No visible mandatory inventory-first flow.
- No fallback to the old monolithic listing model.
