# Commerce DB / Model Split Design

> **Status:** DESIGN TARGET ONLY
>
> **Scope:** Product, FixedPriceListing, Auction, Bid, product_shipping_options, references, lifecycle, migration order, and implementation boundary.

Related Documents:
- [Commerce Selling Doctrine](../flows/doctrine/commerce-selling-doctrine.md)
- [Commerce Selling Contract Notes](./commerce-selling-contract-notes.md)

---

## 1. Executive Verdict

The target remains a **full split staged replacement**.

- `Product` is the internal physical item authority.
- `FixedPriceListing` is the fixed-price sale surface.
- `Auction` is the bidding sale surface.
- `Auction` supports optional `buy_now_price`.
- `Auction` must not depend on `listing_id`.
- No runtime changes are made in this phase.

Because Labuda is from-zero / pre-production, the clean target is not a compatibility bridge. It is a clean split with destructive cleanup where needed.

With the reference graph now closed in this document, Phase 2 is **unblocked from a design perspective**. The remaining work is implementation only.

---

## 2. Existing Schema Field Inventory

### 2.1 Current `listings` fields

| Current field | Current meaning | Target location | Action | Notes |
|---|---|---|---|---|
| `id` | Monolithic listing identity | `products.id` plus sale-surface IDs | Rename / split | Current item authority is split into `products` and one sale surface row. |
| `seller_id` | Listing owner | `products.seller_id`, mirrored on sale surface | Keep | Product remains seller-owned; sale surfaces also carry seller_id for fast auth. |
| `title` | Item title | `products.title` | Move | Shared item detail, not sale-only data. |
| `description` | Item description | `products.description` | Move | Shared item detail. |
| `media_urls` | Product media | `products.media_urls` | Move | Shared item detail. |
| `variety` | Koi variety | `products.variety` | Move | Physical item authority. |
| `size_cm` | Item size | `products.size_cm` | Move | Nullable draft-safe field. |
| `age_months` | Item age | `products.age_months` | Move | Nullable draft-safe field. |
| `gender` | Item gender | `products.gender` | Move | Nullable draft-safe field. |
| `breeder` | Breeder metadata | `products.breeder` | Move | Shared item detail. |
| `bloodline` | Bloodline metadata | `products.bloodline` | Move | Shared item detail. |
| `certificates` | Item certificates | `products.certificates` | Move | Shared item detail. |
| `listing_type` | Fixed-price vs auction | Deleted as a field | Delete / rename | Table split replaces this discriminator. |
| `price_per_unit` | Sale price | `fixed_price_listings.price_per_unit` | Move | Fixed-price only. |
| `quantity_available` | Stock count | Deleted from target schema | Delete | Koi is unique-by-default. Multi-stock is deferred. If a temporary compatibility column is needed during transition, it must default to `1` and must not drive checkout or stock logic. |
| `negotiation_enabled` | Negotiation flag | `fixed_price_listings.negotiation_enabled` | Move | Negotiation applies only to fixed price. |
| `visibility` | Public/private listing visibility | Deleted or derived | Delete / rename | Public exposure should come from lifecycle and publication state, not a manual visibility flag. |
| `status` | Draft/active/sold/withdrawn | Split into product and sale-surface statuses | Split | Current status is doing two jobs and must be separated. |
| `origin` | Manual/import provenance | Deleted from target schema | Delete | If provenance is needed, preserve it only in audit logs or migration notes. It must never drive sale type, visibility, or lifecycle. |
| `farm_address_id` | Shipping origin / farm address | `products.shipping_origin_address_id` | Move | Shipping attaches to Product. |
| `preparation_time` | Readiness window | `products.preparation_time` | Move | Shipping/readiness belongs to Product. |
| `preparation_note` | Readiness note | `products.preparation_note` | Move | Shipping/readiness belongs to Product. |
| `view_count` | Listing analytics | Derived projection / analytics table | Delete / rename | Do not keep as core authority. |
| `created_at` | Creation timestamp | All target tables | Keep | Split tables each need timestamps. |
| `updated_at` | Update timestamp | All target tables | Keep | Split tables each need timestamps. |
| `search_vector` | Listing search index | Derived search projection | Delete / rename | Search should be projection-based and keyed to product + sale surface, not a monolith. |

### 2.2 Current `auctions` fields

| Current field | Current meaning | Target location | Action | Notes |
|---|---|---|---|---|
| `id` | Auction identity | `auctions.id` | Keep | New auction row, but no `listing_id`. |
| `seller_id` | Auction owner | `auctions.seller_id` and `products.seller_id` | Keep | Useful for auth and fast joins. |
| `listing_id` | Item link | `auctions.product_id` | Delete / rename | This must disappear from the target model. |
| `order_id` | Terminal order link | `auctions.order_id` | Keep / clarify | Represents the canonical terminal order for the auction. |
| `settlement_deadline` | Winner claim deadline | `auctions.settlement_deadline` | Keep | Used for waiting_settlement timeout handling. |
| `title` | Auction title | `products.title` | Move | Shared item detail. |
| `description` | Auction description | `products.description` | Move | Shared item detail. |
| `preparation_time` | Readiness window | `products.preparation_time` | Move | Shipping attaches to Product. |
| `preparation_note` | Readiness note | `products.preparation_note` | Move | Shipping attaches to Product. |
| `start_price` | Auction start price | `auctions.start_price` | Keep | Auction-specific price authority. |
| `bid_increment` | Minimum increment | `auctions.bid_increment` | Keep | Auction-specific invariant. |
| `buy_now_price` | Instant buy price | `auctions.buy_now_price` | Keep | Nullable, optional Buy Now. |
| `start_at` | Start time | `auctions.start_at` | Keep | Lifecycle timing. |
| `end_at` | End time | `auctions.end_at` | Keep | Lifecycle timing. |
| `current_bid` | Current highest bid | `auctions.current_bid_amount` or `current_bid` | Keep / rename | Target name should clearly indicate this is the current high bid. |
| `current_winner_id` | Current winner | `auctions.current_winner_id` | Keep | Null until a valid winner exists. |
| `status` | Auction lifecycle | `auctions.status` | Keep / narrow | Target lifecycle removes old `expired_bnr`. |
| `created_at` | Creation timestamp | `auctions.created_at` | Keep | Target table timestamp. |
| `updated_at` | Update timestamp | `auctions.updated_at` | Keep | Target table timestamp. |

### 2.3 Current relation and reference tables

| Current reference | Current meaning | Target mapping | Action |
|---|---|---|---|
| `listing_shipping_options.listing_id` | Shipping options attached to a listing | `product_shipping_options.product_id` | Rename to product authority. |
| `shipping_quotes.listing_id` / `shipping_quotes.auction_id` | Quote linked to a sale surface | `shipping_quotes.product_id` plus `source_type` / `source_id` snapshot context | Replace listing anchor with product authority and sale-surface context. |
| `orders.source_type = 'listing'` | Order came from listing flow | `orders.source_type = 'fixed_price_listing'` | Rename source discriminator. |
| `orders.source_id` | Source record ID | `fixed_price_listing_id` or `auction_id` (or `negotiation_id` only if negotiation checkout remains) | Keep shape, update semantics. |
| `order_items.listing_id` | Item snapshot reference | `order_items.product_id` | Replace with product authority. Sale-surface identity belongs on the order header. |
| `pricing_tokens.listing_id` | Pricing authority bound to listing | `pricing_tokens.source_type` + `pricing_tokens.source_id` | Replace listing binding with sale-surface binding. |
| `negotiation_sessions.resource_type = 'listing'` | Negotiation on a listing | `negotiation_sessions.resource_type = 'fixed_price_listing'` | Negotiation stays fixed-price only. |
| `negotiation_sessions.resource_type = 'auction'` | Negotiation on an auction | Forbidden in target model | Delete / block. |
| `promotion_instances.target_type = 'listing'` | Promotion on listing | `promotion_instances.target_type = 'fixed_price_listing'` | Rename. |
| `promotion_instances.target_type = 'auction'` | Promotion on auction | Keep | Auction remains a valid sale surface target. |
| `listings.search_vector` | Search projection rooted in listing | Search projection rooted in product + sale surface | Replace with projection design. |
| `search_results` | Query history | Unchanged | Not a commerce authority table. |

### 2.4 Reference Graph Closure

The reference graph is closed with the following concrete decisions:

- `order_items` uses `product_id` as the canonical line-item authority.
- `orders.source_type` / `orders.source_id` remain the canonical sale-surface header reference.
- `shipping_quotes` uses `product_id` as the authority link and may carry `source_type` / `source_id` as contextual snapshots.
- `pricing_tokens` binds to the sale surface, not to `listing_id`.
- `negotiation_sessions` keep a fixed-price-listing target only; auction negotiation is forbidden.
- `promotion_instances` rename listing targets to fixed-price listing targets.
- Search / feed / moderation / notification labels must render `Product`, `FixedPriceListing`, and `Auction` nouns in active paths, not the old monolithic listing noun.

---

## 3. Target Schema Proposal

### 3.1 Shared enum set

Create new enums, or narrow existing names if the migration plan prefers reuse:

- `product_status_enum`: `draft`, `active`, `sold`, `archived`
- `fixed_price_listing_status_enum`: `draft`, `active`, `withdrawn`, `sold`
- `auction_status_enum`: `draft`, `scheduled`, `active`, `waiting_settlement`, `ended`, `cancelled`

If the implementation reuses the current `auction_status_enum`, the target values must still be narrowed to the six-state set above. `expired_bnr` is not part of the target model.

### 3.2 `products`

Purpose: internal physical item authority.

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | UUID | NO | `gen_random_uuid()` | Primary key. |
| `seller_id` | UUID | NO | - | FK `users(id)` `ON DELETE CASCADE`. |
| `title` | TEXT | NO | - | Shared item title. |
| `description` | TEXT | NO | - | Shared item description. |
| `media_urls` | JSONB | NO | `'[]'::jsonb` | Must be an array. |
| `variety` | TEXT | NO | - | Koi variety or equivalent item taxonomy. |
| `size_cm` | INTEGER | YES | `NULL` | Draft-safe. |
| `age_months` | INTEGER | YES | `NULL` | Draft-safe. |
| `gender` | TEXT | YES | `NULL` | Draft-safe. |
| `breeder` | TEXT | YES | `NULL` | Draft-safe. |
| `bloodline` | TEXT | YES | `NULL` | Draft-safe. |
| `certificates` | TEXT[] | NO | `ARRAY[]::TEXT[]` | Shared item proof references. |
| `shipping_origin_address_id` | UUID | YES | `NULL` | FK `addresses(id)` `ON DELETE SET NULL`. |
| `preparation_time` | preparation_time_enum | YES | `NULL` | Required before publish, not required for draft. |
| `preparation_note` | TEXT | YES | `NULL` | Optional readiness note. |
| `status` | product_status_enum | NO | `draft` | Lifecycle authority. |
| `published_at` | TIMESTAMPTZ | YES | `NULL` | Optional audit timestamp for first activation. |
| `sold_at` | TIMESTAMPTZ | YES | `NULL` | Terminal sale timestamp. |
| `archived_at` | TIMESTAMPTZ | YES | `NULL` | Terminal archive timestamp. |
| `deleted_at` | TIMESTAMPTZ | YES | `NULL` | Optional soft-delete tombstone. |
| `created_at` | TIMESTAMPTZ | NO | `NOW()` | Creation timestamp. |
| `updated_at` | TIMESTAMPTZ | NO | `NOW()` | Update timestamp. |

Recommended constraints:

- `CHECK (status IN (...))` via enum.
- `CHECK (preparation_time IS NOT NULL)` should be enforced by service only when publishing, not on draft insert.
- `CHECK (title <> '')` and `CHECK (description <> '')` are optional but recommended if the codebase uses empty-string guards elsewhere.

Recommended indexes:

- `idx_products_seller_id` on `(seller_id)`
- `idx_products_status` on `(status)`
- `idx_products_created_at` on `(created_at DESC)`

### 3.3 `fixed_price_listings`

Purpose: fixed-price sale surface, negotiation only here.

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | UUID | NO | `gen_random_uuid()` | Primary key. |
| `product_id` | UUID | NO | - | FK `products(id)` `ON DELETE CASCADE`. |
| `seller_id` | UUID | NO | - | FK `users(id)` `ON DELETE CASCADE`. |
| `price_per_unit` | BIGINT | NO | - | Must be `>= 0`. |
| `negotiation_enabled` | BOOLEAN | NO | `false` | Fixed-price only. |
| `status` | fixed_price_listing_status_enum | NO | `draft` | Lifecycle authority. |
| `published_at` | TIMESTAMPTZ | YES | `NULL` | First activation timestamp. |
| `withdrawn_at` | TIMESTAMPTZ | YES | `NULL` | Seller withdrawal timestamp. |
| `sold_at` | TIMESTAMPTZ | YES | `NULL` | Terminal sale timestamp. |
| `deleted_at` | TIMESTAMPTZ | YES | `NULL` | Optional soft-delete tombstone. |
| `created_at` | TIMESTAMPTZ | NO | `NOW()` | Creation timestamp. |
| `updated_at` | TIMESTAMPTZ | NO | `NOW()` | Update timestamp. |

Recommended constraints:

- `CHECK (price_per_unit >= 0)`
- `CHECK (status IN (...))` via enum
- Negotiation must only be meaningful when `status = 'active'`; this is a service invariant, not a DB cross-table rule.

Recommended indexes:

- `idx_fixed_price_listings_product_id` on `(product_id)`
- `idx_fixed_price_listings_seller_id` on `(seller_id)`
- `idx_fixed_price_listings_status` on `(status)`
- `idx_fixed_price_listings_created_at` on `(created_at DESC)`
- Partial unique index `uniq_fixed_price_listing_draft_per_product` on `(product_id)` where `status = 'draft'`
- Partial unique index `uniq_fixed_price_listing_active_per_product` on `(product_id)` where `status = 'active'`

The partial unique indexes above ensure there is never more than one live fixed-price listing row per product. Historical terminal rows may remain.

### 3.4 `auctions`

Purpose: bidding sale surface, optionally Buy Now.

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | UUID | NO | `gen_random_uuid()` | Primary key. |
| `product_id` | UUID | NO | - | FK `products(id)` `ON DELETE CASCADE`. |
| `seller_id` | UUID | NO | - | FK `users(id)` `ON DELETE CASCADE`. |
| `order_id` | UUID | YES | `NULL` | Unique FK `orders(id)` `ON DELETE SET NULL`. Canonical terminal order. |
| `settlement_deadline` | TIMESTAMPTZ | YES | `NULL` | Deadline for winner claim / settlement. |
| `start_price` | BIGINT | NO | - | Must be `>= 0`. |
| `bid_increment` | BIGINT | NO | - | Must be `> 0`. |
| `buy_now_price` | BIGINT | YES | `NULL` | Optional Buy Now. |
| `start_at` | TIMESTAMPTZ | NO | - | Auction start time. |
| `end_at` | TIMESTAMPTZ | NO | - | Must be greater than `start_at`. |
| `current_bid_amount` | BIGINT | YES | `NULL` | High bid snapshot. |
| `current_winner_id` | UUID | YES | `NULL` | FK `users(id)` `ON DELETE SET NULL`. |
| `winning_bid_id` | UUID | YES | `NULL` | FK `bids(id)` `ON DELETE SET NULL`. |
| `status` | auction_status_enum | NO | `draft` | Lifecycle authority. |
| `scheduled_at` | TIMESTAMPTZ | YES | `NULL` | Optional publish/schedule timestamp. |
| `activated_at` | TIMESTAMPTZ | YES | `NULL` | Auction live timestamp. |
| `waiting_settlement_at` | TIMESTAMPTZ | YES | `NULL` | Winner claim timestamp. |
| `ended_at` | TIMESTAMPTZ | YES | `NULL` | Terminal end timestamp. |
| `cancelled_at` | TIMESTAMPTZ | YES | `NULL` | Seller/system cancellation timestamp. |
| `deleted_at` | TIMESTAMPTZ | YES | `NULL` | Optional soft-delete tombstone. |
| `created_at` | TIMESTAMPTZ | NO | `NOW()` | Creation timestamp. |
| `updated_at` | TIMESTAMPTZ | NO | `NOW()` | Update timestamp. |

Recommended constraints:

- `CHECK (start_price >= 0)`
- `CHECK (bid_increment > 0)`
- `CHECK (buy_now_price IS NULL OR buy_now_price >= start_price + bid_increment)`
- `CHECK (end_at > start_at)`
- `CHECK (current_bid_amount IS NULL OR current_bid_amount >= start_price)`
- `CHECK (status IN (...))` via enum

Recommended indexes:

- `idx_auctions_product_id` on `(product_id)`
- `idx_auctions_seller_id` on `(seller_id)`
- `idx_auctions_status` on `(status)`
- `idx_auctions_start_at` on `(start_at)`
- `idx_auctions_end_at` on `(end_at)`
- `idx_auctions_order_id` unique on `(order_id)` where `order_id IS NOT NULL`
- Partial unique index `uniq_auction_draft_per_product` on `(product_id)` where `status = 'draft'`
- Partial unique index `uniq_auction_live_per_product` on `(product_id)` where `status IN ('scheduled', 'active', 'waiting_settlement')`

### 3.5 `bids`

Purpose: immutable bid history for a single auction.

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | UUID | NO | `gen_random_uuid()` | Primary key. |
| `auction_id` | UUID | NO | - | FK `auctions(id)` `ON DELETE CASCADE`. |
| `bidder_id` | UUID | NO | - | FK `users(id)` `ON DELETE CASCADE`. |
| `amount` | BIGINT | NO | - | Must be `> 0`. |
| `idempotency_key` | TEXT | NO | - | Retry safety. |
| `created_at` | TIMESTAMPTZ | NO | `NOW()` | Creation timestamp. |

Recommended constraints:

- `CHECK (amount > 0)`
- `UNIQUE (auction_id, idempotency_key)`

Recommended indexes:

- `idx_bids_auction_id` on `(auction_id, created_at DESC)`
- `idx_bids_bidder_id` on `(bidder_id, created_at DESC)`

### 3.6 `product_shipping_options`

Purpose: Product-level shipping option attachment.

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| `id` | UUID | NO | `gen_random_uuid()` | Primary key. |
| `product_id` | UUID | NO | - | FK `products(id)` `ON DELETE CASCADE`. |
| `shipping_option_id` | UUID | NO | - | FK `shipping_options(id)` `ON DELETE CASCADE`. |
| `sort_order` | INTEGER | NO | `0` | Display order. |
| `created_at` | TIMESTAMPTZ | NO | `NOW()` | Creation timestamp. |
| `updated_at` | TIMESTAMPTZ | NO | `NOW()` | Update timestamp. |
| `deleted_at` | TIMESTAMPTZ | YES | `NULL` | Soft-delete tombstone. |

Recommended constraints:

- `UNIQUE (product_id, shipping_option_id)` where `deleted_at IS NULL`

Recommended indexes:

- `idx_product_shipping_options_product_id` on `(product_id)`
- `idx_product_shipping_options_shipping_option_id` on `(shipping_option_id)`

### 3.7 Shipping origin and checkout snapshots

Shipping origin attaches to `products.shipping_origin_address_id`. Checkout snapshots remain on `orders` as immutable order history:

- shipping option ID
- shipping option name
- shipping transport type
- expedition name
- estimated days
- shipping quote snapshot if applicable
- origin address snapshot
- destination address snapshot

That means sale surfaces resolve shipping through the product, while order creation freezes the selected shipping truth at checkout.

---

## 4. Status Model

### 4.1 `product.status`

States:

- `draft`
- `active`
- `sold`
- `archived`

Transitions:

| From | To | Trigger | Invariant |
|---|---|---|---|
| `draft` | `active` | Seller publish / system publish | Product is fully described enough for a sale surface to publish. |
| `active` | `sold` | Canonical sale settlement | A terminal order exists and no further sales may consume the same product. |
| `active` | `archived` | Seller archive / admin archive | Product leaves commerce but is retained. |
| `draft` | `archived` | Seller abandons or archives draft | No terminal sale exists. |
| `sold` | `archived` | Optional admin cleanup only | Sold history remains intact; this is not a re-sell path. |

The `product` table is the physical item authority. The product itself is never the final checkout authority.

### 4.2 `fixed_price_listings.status`

States:

- `draft`
- `active`
- `withdrawn`
- `sold`

Transitions:

| From | To | Trigger | Invariant |
|---|---|---|---|
| `draft` | `active` | Seller publish | Product must not already be in a live auction state. Shipping and product authority must be valid. |
| `draft` | `withdrawn` | Seller abandons draft | No sale authority is granted. |
| `active` | `withdrawn` | Seller withdraw | No terminal order exists yet, or the withdrawal is a governance override. |
| `active` | `sold` | Canonical order settlement | The fixed-price sale produced a terminal order. |
| `withdrawn` | `draft` | Seller re-enters editing | Only if the implementation supports reopening drafts; otherwise this can be omitted. |

The crucial rule is that an active fixed-price listing cannot remain active while the same product is activated into auction.

### 4.3 `auction.status`

States:

- `draft`
- `scheduled`
- `active`
- `waiting_settlement`
- `ended`
- `cancelled`

Transitions:

| From | To | Trigger | Invariant |
|---|---|---|---|
| `draft` | `scheduled` | Seller publish / schedule | Product is not sold and no conflicting live fixed-price listing exists. |
| `scheduled` | `active` | Scheduler / worker at `start_at` | Product remains unsold and exclusivity still holds. |
| `active` | `waiting_settlement` | Buyer wins or Buy Now accepts | Canonical terminal buyer/order path begins. |
| `waiting_settlement` | `ended` | Order created and settled | The canonical order exists and the auction is terminal. |
| `active` | `ended` | Auction closes unsold | Product returns to `active` availability. |
| `waiting_settlement` | `ended` | Claim window expires without terminal order | Product returns to `active` availability. |
| `draft` | `cancelled` | Seller abandons draft | No sale has started. |
| `scheduled` | `cancelled` | Seller withdraw / governance cancel | No bids are accepted and the product returns to available. |
| `active` | `cancelled` | Seller withdraw before bids or governance cancel | If bids already exist, the service may require a stricter resolution path. |

`expired_bnr` from the current system is not part of the target lifecycle. It should be collapsed into the `ended` / `cancelled` handling during the model split.

### 4.4 Order model closure

Target order schema:

- `orders.source_type` uses a sale-surface enum with `fixed_price_listing`, `auction`, and `negotiation` if negotiation checkout remains in scope.
- `orders.source_id` points at the canonical source record for that source type.
- `order_items.order_id` remains the parent FK.
- `order_items.product_id` becomes the line-item product authority.
- `order_items` keeps immutable snapshots for item name and unit price.
- `orders` keeps immutable snapshots for shipping origin, shipping destination, shipping option, and pricing inputs.

Terminal-order safety:

- Fixed-price checkout creates exactly one terminal order for the `fixed_price_listing` source.
- Auction Buy Now creates exactly one terminal order for the `auction` source.
- Auction claim creates exactly one terminal order for the `auction` source.
- `orders.source_type` / `orders.source_id` must be unique for terminal sale sources.
- `auctions.order_id` remains a unique terminal-order link.
- `fixed_price_listings.sold_at` and `auctions.ended_at` are set in the same transactional path as the terminal order.
- One `Product` cannot produce two terminal orders because the product row is locked, the source row is locked, and the terminal source reference is unique.

---

## 5. Sale Exclusivity Design

### 5.1 Rules

- One `Product` cannot have a live fixed-price listing and a live auction at the same time.
- One `Product` cannot have two live auctions at the same time.
- Active fixed-price listing must be withdrawn before auction activation.
- Sold product cannot be reused.
- Drafts may coexist as a product draft plus a sale-surface draft.
- Buy Now / order / settlement cannot double-sell the product.

### 5.2 Enforcement matrix

| Rule | DB constraint | Partial unique index | Transaction lock | Service validation | Worker reconciliation |
|---|---|---|---|---|---|
| One live fixed-price listing per product | Yes | Yes | No | Yes | No |
| One live auction per product | Yes | Yes | No | Yes | No |
| Fixed-price must be withdrawn before auction activation | No | No | Yes | Yes | No |
| Sold product cannot be reused | No | No | Yes | Yes | No |
| Buy Now / order / settlement cannot double-sell | Yes on order source uniqueness | Yes on `(source_type, source_id)` for terminal sale sources | Yes | Yes | Yes for repair |

### 5.3 Implementation notes

- DB constraints should handle local invariants within each table.
- Partial unique indexes should block multiple live rows of the same type for the same product.
- A transaction lock on the `products` row should guard the cross-table transition from fixed-price to auction and from auction settlement to product sold.
- Service validation is required for cross-table checks because the DB cannot express every cross-table rule cleanly.
- Worker reconciliation is only for repair of stale lifecycle state, not for deciding sale truth.

---

## 6. Auction Buy Now Schema And Invariant

### 6.1 Schema rule

- `buy_now_price` is nullable.
- If `buy_now_price` is `NULL`, Buy Now is disabled.
- If `buy_now_price` is present, the database check must enforce:

`buy_now_price IS NULL OR buy_now_price >= start_price + bid_increment`

### 6.2 Behavioral rules

- The first bid does not disable Buy Now.
- Buy Now remains available while the auction is active until the canonical terminal settlement/order path has completed.
- Once a terminal order exists, Buy Now is no longer available.
- After a terminal order / settlement, the product is no longer available for a second sale.
- Buy Now must route through the canonical backend order/payment flow.

### 6.3 DB vs service split

DB-level checks:

- nullable `buy_now_price`
- floor rule relative to `start_price` and `bid_increment`
- status enum validity

Service-level checks:

- Buy Now may only be accepted while the auction is live
- Buy Now must lock the auction and product rows before settlement
- Buy Now must create exactly one canonical order
- Buy Now must not bypass the order/payment authority

---

## 7. Shipping Model

### 7.1 Decision

Shipping attaches to `Product`.

### 7.2 Rule set

- `product_shipping_options` defines which shipping options the product can use.
- `products.shipping_origin_address_id` defines the seller-origin address for the item.
- `products.preparation_time` and `products.preparation_note` define shipping readiness.
- Fixed-price listings and auctions resolve shipping through the linked product.
- The order snapshots shipping at checkout.

### 7.3 Publish versus checkout

- Draft creation does not require shipping completeness.
- Publish of an active sale surface should require the product to have enough shipping data to support checkout.
- Checkout is where shipping truth becomes immutable in the order snapshot.

This keeps the UX sale-intent-first while still protecting order creation from incomplete shipping truth.

### 7.4 Shipping quote closure

`shipping_quotes` should become product-anchored.

- Required authority link: `product_id`
- Required contextual snapshots: `source_type` and `source_id`
- Quote validation must resolve shipping options through the linked product
- `shipping_quotes.listing_id` must be removed from the target model
- `shipping_quotes.auction_id` may be kept only if it is represented as the generic `source_id` context; it must not be the authority link

Orders snapshot:

- shipping option ID
- shipping option name
- shipping transport type
- expedition name
- estimated days
- origin address snapshot
- destination address snapshot
- quote ID / quote price if a quote was used

---

## 8. Migration Sequence Proposal

### 8.1 Recommended order

A. Create product and sale-surface enums if needed.  
B. Create `products` table.  
C. Create `fixed_price_listings` table.  
D. Alter or rebuild `auctions` to use `product_id` and nullable `buy_now_price`.  
E. Create `product_shipping_options`.  
F. Update `orders`, `order_items`, `shipping_quotes`, `pricing_tokens`, `negotiation_sessions`, `promotion_instances`, and any search/notification projection helpers to use the split references.  
G. Drop old listing-as-item columns.  
H. Remove auction `listing_id`.  
I. Update seed and fixture data.

### 8.2 Destructive steps

Destructive or data-reset steps are:

- D if the auction table is rebuilt instead of altered in place.
- G because old listing-as-item columns are being removed.
- H because auction `listing_id` is being removed.
- I if fixtures are reseeded from scratch.

Because the project is pre-production, the cleanest implementation path is to prefer reset/reseed over compatibility backfill.

---

## 9. Existing Data Handling

Recommended path:

- Back up any local owner-test data if it matters.
- Drop or recreate the dev schema if that is the fastest clean split.
- Reseed using the new canonical split.
- Do not invest in compatibility backfill logic unless a future owner explicitly asks for it.

For Labuda in its current state, the cleanest path is a fresh split rather than a translation layer.

---

## 10. Phase 2 Runtime Closure

Phase 2 must update the compile-critical runtime surface in the same implementation batch as the schema split.

### 10.1 Auction runtime files

- `backend/internal/commerce/auction/delivery/http/auction_handler.go`
- `backend/internal/commerce/auction/application/auction_service.go`
- `backend/internal/commerce/auction/entity/auction.go`
- `backend/internal/commerce/auction/infrastructure/repository/auction_repository.go`

Intent:

- remove handler DTO `listing_id`
- accept `product_id`
- accept nullable `buy_now_price`
- apply canonical Buy Now floor validation
- remove `expired_bnr` from the target lifecycle
- ensure auction settlement uses product/source locks and canonical order idempotency

Risk: P0

### 10.2 Pricing token files

- `backend/internal/pricing/token/delivery/http/pricing_token_handler.go`
- `backend/internal/pricing/token/application/pricing_token_service.go`
- `backend/internal/pricing/token/entity/pricing_token.go`
- `backend/internal/pricing/token/infrastructure/repository/pricing_token_repository_impl.go`

Intent:

- replace listing binding with sale-surface binding
- generate tokens for fixed-price listing, negotiation, and auction contexts
- keep pricing authority backend-owned

Risk: P0

### 10.3 Order files

- `backend/internal/commerce/order/application/order_creation_service.go`
- `backend/internal/commerce/order/application/order_query_service.go`
- `backend/internal/commerce/order/entity/order.go`

Intent:

- move order item references to `product_id`
- keep order header source type / source id as canonical sale-surface binding
- ensure terminal orders are unique per source and cannot double-sell a product
- keep shipping and pricing snapshots immutable

Risk: P0

### 10.4 Shipping / quote files

- `backend/internal/commerce/shipping/delivery/http/shipping_handler.go`
- `backend/internal/commerce/shipping/quote/repository/shipping_quote_repository.go`
- `backend/internal/commerce/shipping/quote/entity/*`
- `backend/internal/commerce/shipping/quote/infrastructure/repository/*`
- `backend/internal/commerce/shipping/application/*` if quote validation is present there

Intent:

- replace `listing_id` anchor with `product_id`
- keep sale-surface context as snapshots only
- resolve shipping options from product authority

Risk: P1

### 10.5 Negotiation / promotion / projection label files

- `backend/internal/commerce/negotiation/entity/negotiation_session.go`
- `backend/internal/commerce/negotiation/entity/negotiation_resource_type.go`
- `backend/internal/commerce/negotiation/consumer/*`
- `backend/internal/commerce/listing/*` label emitters that still say listing when they mean fixed-price listing
- promotion / search / notification projection code that still exposes the old monolithic listing noun on active commerce surfaces

Intent:

- negotiation target remains fixed-price listing only
- auction negotiation remains forbidden
- active labels render Product / FixedPriceListing / Auction nouns consistently

Risk: P1

---

## 11. Model / Entity Package Design

### 10.1 Canonical package layout

- `backend/internal/commerce/product`
- `backend/internal/commerce/fixedprice`
- `backend/internal/commerce/auction`
- `backend/internal/commerce/shipping`

### 10.2 Entity names

- `product/entity.Product`
- `fixedprice/entity.FixedPriceListing`
- `auction/entity.Auction`
- `auction/entity.Bid`
- `shipping/entity.ProductShippingOption` or join-link equivalent

### 10.3 Value objects

- `ProductStatus`
- `FixedPriceListingStatus`
- `AuctionStatus`
- `Money`
- `BidIncrement`
- `ShippingReadiness`
- `SettlementDeadline`
- `BuyNowPrice`

### 10.4 Repository boundaries

- `ProductRepository`
- `FixedPriceListingRepository`
- `AuctionRepository`
- `BidRepository`
- `ProductShippingOptionRepository`

Repositories should not cross aggregate boundaries. Cross-aggregate operations belong in services.

### 10.5 Service boundaries

- `ProductService`
- `FixedPriceListingService`
- `AuctionService`
- `AuctionSettlementService`
- `ShippingConfigurationService`

### 10.6 Event names

Suggested outbox/event vocabulary:

- `product.created`
- `product.draft_saved`
- `product.activated`
- `product.archived`
- `product.sold`
- `fixed_price_listing.created`
- `fixed_price_listing.draft_saved`
- `fixed_price_listing.published`
- `fixed_price_listing.withdrawn`
- `fixed_price_listing.sold`
- `auction.created`
- `auction.draft_saved`
- `auction.scheduled`
- `auction.activated`
- `auction.bid_placed`
- `auction.buy_now_selected`
- `auction.waiting_settlement`
- `auction.ended`
- `auction.cancelled`
- `auction.settled`

---

## 12. API Impact Preview

### 11.1 Explicit product endpoints

- `POST /api/v1/products`
- `PATCH /api/v1/products/:id`
- `GET /api/v1/products/:id`

### 11.2 Fixed-price listing endpoints

- `POST /api/v1/fixed-price-listings`
- `PATCH /api/v1/fixed-price-listings/:id`
- `POST /api/v1/fixed-price-listings/:id/publish`
- `POST /api/v1/fixed-price-listings/:id/withdraw`

### 11.3 Auction endpoints

- `POST /api/v1/auctions`
- `PATCH /api/v1/auctions/:id`
- `POST /api/v1/auctions/:id/publish`
- `POST /api/v1/auctions/:id/bids`
- `POST /api/v1/auctions/:id/buy-now`
- `POST /api/v1/auctions/:id/claim`

### 11.4 Mobile one-step endpoints

- `POST /api/v1/mobile/fixed-price-listings`
- `POST /api/v1/mobile/auctions`

The mobile endpoints may create product + sale surface in one transaction, but they must still preserve the canonical backend split.

### 11.5 Explicit prohibition

- No auction create request may accept `listing_id`.

---

## 12. Risk Matrix

| Risk | Severity | Why it matters | Mitigation |
|---|---|---|---|
| Schema split misses a reference edge | P0 | A hidden `listing_id` dependency can break auction creation or settlement. | Grep-proof Phase 2, service validation, and explicit schema review. |
| Double-sell race | P0 | Two live sale surfaces or two settlements on one product would corrupt commerce truth. | Partial unique indexes, row locks, canonical order uniqueness, reconciliation. |
| Buy Now settlement bypasses order authority | P0 | Money flow could diverge from the canonical order/payment pipeline. | Keep Buy Now inside backend order authority only. |
| Auction lifecycle mismatch | P0 | Wrong terminal state can leak product availability or stall settlement. | Narrow auction enum, worker reconciliation, strict state transitions. |
| Shipping snapshot mismatch | P1 | Buyers may see one shipping truth and pay another. | Attach shipping to Product and snapshot at checkout. |
| Search / promotion still points at listing | P1 | Discovery will reintroduce the old monolith. | Update projections and route all new surfaces through product + sale-surface IDs. |
| Mobile contract drifts back to inventory-first wording | P1 | UX regresses to the wrong mental model. | Lock the visible flow labels and API shape. |
| Package boundaries stay ambiguous | P2 | Future code will mix product and sale-surface logic. | Canonical package layout plus repository/service separation. |
| Draft handling becomes ad hoc | P2 | Sellers lose the first-class draft workflow. | Explicit draft columns and publish/withdraw transitions. |
| Docs drift | P3 | Later agents may rediscover the old model. | Keep the doctrine, contract notes, and this design doc aligned. |

---

## 14. Grep Proof Targets For Future Implementation

Future phases should satisfy the following checks:

1. `rg -n "listing_id" backend/internal/commerce/auction backend/internal/commerce/order backend/internal/pricing/token backend/cmd/core_server/routes_core.go`
   - Expected: no auction-create, order, or pricing-token dependency on `listing_id`.

2. `rg -n "inventory-first|create inventory|mandatory inventory" apps/mobile docs`
   - Expected: no positive UX guidance that forces inventory-first creation.

3. `rg -n "negotiation.*auction|auction.*negotiation" backend/internal/commerce docs`
   - Expected: no negotiation target on auction; negotiation remains fixed-price only.

4. `rg -n "compatibility shim|old signature|legacy preservation|listing-as-item" backend docs`
   - Expected: no new compatibility path or old model preservation language.

5. `rg -n "fixed_price_listing|product_id" backend/internal/commerce/auction`
   - Expected: target auction code should clearly use product-based ownership, not listing-based ownership.

6. `rg -n "buy_now_price.*start_price.*bid_increment" backend docs`
   - Expected: the new Buy Now floor rule should appear in code and docs.

7. `rg -n "negotiation_resource_enum|resource_type.*auction" backend`
   - Expected: no auction negotiation resource in the target implementation.

8. `rg -n "listing_shipping_options|product_shipping_options|shipping_quotes\\.listing_id|order_items\\.listing_id" backend`
   - Expected: shipping attachment should move to product-based terminology and old listing-linked references should be gone from active runtime.

Active implementation files should pass these checks; archived docs may still mention the old model only as historical context.

---

## 15. Final Recommendation

Use this prompt for Phase 2:

> Phase 2: implement the commerce DB/model split from `docs/contracts/commerce-db-model-split-design.md`.
> Create the new enums/tables/constraints, move product fields out of `listings`, remove auction `listing_id`, add nullable `buy_now_price` with the canonical floor check, attach shipping to `products`, update the model/entity packages to `product`, `fixedprice`, and `auction`, and preserve the full-split doctrine.
> Do not add compatibility shims, do not support old signatures, do not reintroduce inventory-first UX, and do not treat tests as business truth.
