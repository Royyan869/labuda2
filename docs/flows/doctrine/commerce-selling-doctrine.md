# Doctrine - Commerce Selling Split

> **Status:** CANONICAL v1
>
> **Scope:** Product, FixedPriceListing, Auction, Draft, Buy Now, seller create UX, sale exclusivity, lifecycle, authority gating.

## Canonical Wording

> "Product is the internal physical item authority."

> "FixedPriceListing is the fixed-price sale surface."

> "Auction is the bidding sale surface."

> "Seller UX is sale-intent-first, not inventory-first."

> "Draft is first-class."

> "Auction must not require an existing fixed-price listing."

## Executive Verdict

The commerce model is a three-part split:

1. `Product` owns the internal physical item truth.
2. `FixedPriceListing` owns the fixed-price sale surface.
3. `Auction` owns the bidding sale surface.

The split is canonical. Future implementation MUST NOT collapse these nouns back into a single listing model.

## Final Target Model

```text
Seller
  -> Product (internal physical item authority)
       -> FixedPriceListing (fixed-price sale surface)
            -> Negotiation (fixed-price only)
       -> Auction (bidding sale surface)
            -> Bid
            -> optional Buy Now
```

## Commerce Rules

- `Product` MUST be the canonical internal noun for the physical item authority.
- `FixedPriceListing` MUST be the only surface that supports negotiation.
- `Auction` MUST be the only bidding surface.
- `Auction` MUST support optional Buy Now.
- `Auction` MUST NOT require a preexisting fixed-price listing.
- `Shipping` MUST attach to `Product`.
- `Koi` items MUST default to unique-item handling.
- Multi-stock support is intentionally deferred.

## Draft Rules

- Draft is first-class for `Product`, `FixedPriceListing`, and `Auction`.
- A seller MAY stop mid-flow and keep draft state.
- Draft MAY exist as a product draft plus a sale-surface draft internally.
- Publish is the explicit commitment boundary.
- Draft MUST NOT be treated as a hidden inventory prerequisite.

## Sale Exclusivity Rules

- A single `Product` MUST NOT have an active `FixedPriceListing` and an active `Auction` at the same time.
- A single `Product` MUST NOT have two active `Auction` records at the same time.
- An active `FixedPriceListing` MUST be withdrawn before the same product can enter auction.
- A sold `Product` MUST NOT be reused for a new sale.
- Draft records MAY coexist because they are not active sale surfaces.

## Lifecycle Matrix

| Entity | Canonical states | Notes |
|--------|------------------|-------|
| `product` | `draft`, `active`, `sold`, `archived` | Internal physical item authority. `draft` covers incomplete seller input. |
| `fixed_price_listing` | `draft`, `active`, `withdrawn`, `sold` | Negotiation applies only here. |
| `auction` | `draft`, `scheduled`, `active`, `waiting_settlement`, `ended`, `cancelled` | Buy Now remains available while active until terminal settlement/order. |

### Product status

- `draft` means the item exists but is not publishable as an active sale surface yet.
- `active` means the product is available to be sold through one active surface.
- `sold` means the product has been consumed by a terminal sale.
- `archived` means the product is retained but not active for commerce.

### Fixed-price listing status

- `draft` means seller intent exists but publication has not happened.
- `active` means the listing is live and negotiable.
- `withdrawn` means the listing was intentionally taken down by the seller.
- `sold` means the listing reached terminal order settlement.

### Auction status

- `draft` means seller setup is incomplete.
- `scheduled` means the auction is published but not yet open for bidding.
- `active` means bidding is open.
- `waiting_settlement` means the auction hit its terminal buyer/order path and the backend is completing canonical settlement.
- `ended` means the auction closed without a terminal sale and the product returns to available/active.
- `cancelled` means the auction was withdrawn before completion.

## Buy Now Rules

- `buy_now_price` MUST be nullable.
- If `buy_now_price` is `null`, Buy Now is disabled.
- If `buy_now_price` is present, backend validation is authoritative.
- Recommended validation is `buy_now_price >= start_price + bid_increment`.
- Buy Now MUST route through the canonical backend order/payment flow.
- Mobile MUST NOT calculate final authority for Buy Now.
- Buy Now MUST remain available while the auction is active until a terminal settlement or order exists.
- Buy Now MUST NOT create a separate settlement path that bypasses backend authority.

## UX Rules

- Seller create UX MUST be sale-intent-first.
- The visible create path SHOULD read like `Buat Jualan`.
- Seller MUST choose `Harga Tetap` or `Lelang`.
- Shared item/product detail section comes first.
- Sale-specific section comes second.
- Shipping section comes after sale-specific data.
- Actions SHOULD include `Simpan Draft` and `Publish`.
- Mobile UX MUST NOT force a visible inventory-first step.
- The UI MAY later expose `Produk Saya`, but not as a prerequisite for normal create listing or auction flow.

## Authority And Gating Rules

- Create listing and create auction MUST require active selling authority.
- Create listing and create auction MUST require active subscription selling access, not payout approval.
- KYC approval MUST NOT gate create listing or create auction.
- Payout approval MUST remain separate from selling authority.
- Backend MUST be the final authority for canonical order/payment entry, including Buy Now.

## Forbidden Behaviors

- No auction create path may depend on `listing_id`.
- No auction create path may require a fixed-price listing first.
- No mandatory visible inventory-first UX.
- No compatibility shim for old commerce signatures or old commerce shape assumptions.
- No old signature support.
- No tests-as-business-truth.
- No direct conversion from active fixed-price listing to auction.
- No backward-compatibility preservation for the old wrong model.

## Cross-Domain Impacts

- API design must split product and sale-surface concerns cleanly.
- Mobile must treat product capture and sale choice as one selling journey.
- Search and feed projections must point to the active sale surface, not to an old monolithic listing concept.
- Order and payment flows must accept canonical backend authority from `FixedPriceListing` and `Auction`.

## Glossary

- `Product` - internal physical item authority.
- `FixedPriceListing` - fixed-price sale surface.
- `Auction` - bidding sale surface.
- `Draft` - first-class pre-publish state for product or sale surface.
- `Buy Now` - optional auction-only instant purchase path.
- `Negotiation` - fixed-price-only counter-offer flow.
- `Selling authority` - the gate that allows commercial publish actions.

## Related Doctrine

- [Seller Authority Separation](./seller-authority-separation.md)
- [Capability Matrix](./capability-matrix.md)
