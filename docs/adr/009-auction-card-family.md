# ADR-009 — AuctionCard Family

## Status

Accepted

## Related Documents

- [`docs/foundation.md`](../foundation.md) — Canonical Authorities (Public Exposure, Identity, Discovery); Commerce Authority Model; auction / bid semantics
- [`docs/architecture.md`](../architecture.md) — Discovery / Projection Design, Commerce Authority Model
- [`docs/contracts/public-card-boundary.md`](../contracts/public-card-boundary.md) — boundary contract
- ADR-001 Pricing Token Authority; ADR-002 Ledger as Authority; ADR-003 Governance Evaluator; ADR-004 Discovery / Projection Boundary
- Companion ADRs: 007 SellerCard, 008 ListingCard, 010 ContentCard

---

## 1. Decision

AuctionCard is the canonical exposure shape for an auction entity. Every auction-emitting public surface (discovery, search, storefront, feed, content embed, watchlist, chat attachment, reference resolution) MUST flow through the AuctionCard family builder.

AuctionCard is **distinct from** ListingCard (separate lifecycle, price model, winner phase — see Section 4) and is NOT a ListingCard variant. It **embeds** SellerCard for the auction's seller (ADR-007) and is **embedded by** ContentCard for commerce shares (ADR-010).

## 2. Ownership

**Commerce domain** owns AuctionCard.

Authority boundaries:

- **Pricing authority** for auction claim (post-win) is the pricing token / claim pipeline (ADR-001). Public current-bid display is a coarse projection; checkout authority is the canonical pipeline.
- **Bid authority** is the canonical auction write model. AuctionCard exposes only the public coarse current bid; bid history internals, bidder identities, and proxy-bid internals never appear.
- **Settlement authority** is the canonical claim / order / payment / escrow pipeline. AuctionCard renders coarse states (`waiting_settlement`, `settled`, `expired_bnr`); it never carries claim tokens, order IDs, payment IDs, or winner identities.
- **Evaluator decision** is produced by the canonical evaluator (ADR-003).
- **Seller embedding** flows through SellerCard (ADR-007).

## 3. Why AuctionCard ≠ ListingCard

| Concern | ListingCard | AuctionCard |
|---|---|---|
| Lifecycle vocabulary | `available` / `unavailable` / `sold` / `removed` / `hidden` / `expired` / `seller_unavailable` | `pending` / `live` / `ended_waiting_settlement` / `settled` / `expired_bnr` / `cancelled` / `removed` / `hidden` / `seller_unavailable` |
| Price model | seller-set public display price | bid-driven public current bid + coarse bid count + optional Buy Now indicator |
| Winner concept | not applicable (FCFS — buyer creates an order) | applicable (winner-determination is a canonical phase between auction end and order creation) |
| BNR terminal | not applicable | canonical (`expired_bnr` — winning bidder did not settle within window) |
| Buy Now | not applicable | optional surface — distinct from bidding flow but constrained by the same checkout / pricing-token authority |

A surface that emits both listings and auctions emits two distinct families. There is no "ProductCard" superset.

## 4. Canonical Card Shape

```
AuctionCard {
  id              : opaque auction reference                (every semantic)
  card_state      : enum {full, ended_waiting_settlement,
                          settled, expired_bnr, cancelled,
                          removed, hidden,
                          seller_unavailable,
                          tombstone, redacted,
                          anonymous_fallback}

  -- Public Identity Reference (auction-level)
  title           : public auction title                    (full / redacted)
  cover_image     : canonical cover-image reference         (full only)
  gallery_refs    : ordered list of public-image references (full only)
  public_slug     : opaque routing slug | null              (full / redacted)

  -- Public Lifecycle State
  lifecycle_state : enum {pending, live, ended_waiting_settlement,
                          settled, expired_bnr, cancelled,
                          removed, hidden, seller_unavailable}

  start_time      : ISO timestamp | null                    (full / lifecycle-relevant)
  end_time        : ISO timestamp | null                    (full / lifecycle-relevant)

  -- Public Bid State
  current_bid     : public coarse current bid | null        (live / ended)
                    -- never raw bid IDs, never bidder identity,
                       never proxy-bid internals
  bid_count       : coarse bid count | null                 (live / ended)
                    -- coarse only; never raw bid history rows
  starting_bid    : public starting bid                     (full)
  buy_now_price   : public Buy Now price | null             (full; if Buy Now offered)

  -- Public Currency
  display_currency : currency code                          (full)

  -- Embedded SellerCard
  seller          : SellerCard reference                    (every non-omission semantic)
                    -- canonical seller embedding; never raw seller fields
}
```

## 5. Allowed Field Categories

| Category | Slot | Rule |
|---|---|---|
| Public Identity Reference (auction-level) | `title`, `public_slug` | auction's own public name |
| Public Display Attributes | `cover_image`, `gallery_refs` | canonical media references |
| Public Commerce Attributes | `current_bid`, `bid_count`, `starting_bid`, `buy_now_price`, `display_currency` | coarse public exposure |
| Public Lifecycle State | `lifecycle_state`, `start_time`, `end_time` | nine-state coarse vocabulary |
| Public Audit Reference | `id` | opaque |
| Embedded card | `seller: SellerCard` | canonical seller embedding |

## 6. Forbidden Field Categories

- **Auth Identity** in any slot, including transitively via the embedded SellerCard.
- **Bid Internals** — raw bid IDs, bidder identities, proxy-bid internals, raw bid history rows. The card carries only the coarse current bid + coarse bid count.
- **Winner Identity** — under any semantic, including `ended_waiting_settlement` and `settled`. Winner identity is a private commerce truth between the parties; it is not part of public exposure. Surfaces that need to render "you won" do so via authenticated personal-surface flows, not via the public AuctionCard.
- **Claim / Settlement Internals** — claim tokens, order IDs, payment IDs, escrow state, settlement state.
- **Pricing Authority Fields** — raw pricing tokens, pricing snapshot internals, fee / discount internals.
- **Financial Authority Fields** — escrow state, seller payable, payout state, settlement state.
- **Internal Moderation Metadata**.
- **Inventory Internals** — auctions are single-item by canonical rule; no stock counters apply.
- **Capability Internals**.
- **Realtime Transport Internals**.

## 7. Auction Lifecycle Rendering

The nine-state coarse vocabulary maps to canonical backend lifecycle:

| Backend state | Card lifecycle_state | card_state | Slot rendering |
|---|---|---|---|
| `draft` | hidden (seller-only surface) | `hidden` | not exposed publicly |
| `scheduled` | `pending` | `full` | `start_time` future; `current_bid`/`bid_count` absent |
| `active` | `live` | `full` | `current_bid`, `bid_count` present |
| `ended` (winner determination phase) | `ended_waiting_settlement` | `ended_waiting_settlement` | `current_bid` final; winner identity NOT exposed |
| `waiting_settlement` (post-end, pre-order) | `ended_waiting_settlement` | `ended_waiting_settlement` | as above |
| `settled` (winner created order) | `settled` | `settled` | terminal; `current_bid` final |
| `expired_bnr` (winner failed BNR) | `expired_bnr` | `expired_bnr` | terminal; coarse public state — NOT "sold" and NOT "cancelled" |
| `cancelled` | `cancelled` | `cancelled` | terminal; auction cancelled by seller / admin |
| seller suspended / removed | `seller_unavailable` | `seller_unavailable` | per SellerCard lifecycle propagation |
| `deleted_at IS NOT NULL` | `removed` | `removed` | default = slot omission |

`ended_waiting_settlement` is the canonical "auction won by someone, settlement pending" state — distinct from `settled` and `expired_bnr`.

## 8. Bid / Current Price Rendering

`current_bid` is the **public coarse current bid** — the highest bid amount visible publicly. It is sourced from the canonical bid write model.

`current_bid` is NOT checkout authority. Per ADR-001, claim authority is the pricing token issued at the claim pipeline. Card-level current bid is allowed to be momentarily stale relative to the canonical bid stream; checkout revalidation closes the loop.

The card MUST NOT carry: raw bid IDs, bidder identities (anonymized or not), bid timestamps with fine granularity, proxy-bid internals (max-bid amounts), bid history rows.

`bid_count` is a coarse integer — total number of public bids. It MUST NOT leak fine-grained bidder demographics or bidding pattern internals.

## 9. Winner / Claim / Settlement Rendering

Winner identity is **private commerce truth**. The public AuctionCard NEVER carries winner identity under any semantic.

- `ended_waiting_settlement` — public state. Card shows `current_bid` (final), `bid_count` (final), no winner identity, no claim token.
- `settled` — public state. Card shows the auction is settled. The order created from settlement is a separate Order entity; AuctionCard does not embed or reference order identity.
- `expired_bnr` — public state. Card shows the auction expired without settlement. No winner identity.

A surface that needs to render "you won this auction" or "claim your win" does so via an authenticated personal-surface flow (`/auctions/:id/claim` or analogous), not via the public AuctionCard. The personal surface uses its own card or DTO governed by personal-surface rules.

## 10. Buy Now / Checkout Separation

If the auction supports Buy Now, `buy_now_price` is a public display indicator. It is NOT checkout authority — checkout for Buy Now follows the same pricing-token pipeline as listing checkout (ADR-001). `buy_now_price` is allowed to be stale relative to the canonical price; revalidation at checkout closes the loop.

The card MUST NOT carry: Buy Now pricing token, Buy Now order draft, Buy Now reservation flags (which do not exist; Buy Now follows FCFS).

## 11. Seller Embedding

AuctionCard ALWAYS embeds a canonical SellerCard reference, with the same rules as ListingCard (Section 8 of ADR-008): canonical builder, own evaluator decision, never substituted with raw seller fields or UserCard.

## 12. Tombstone, Redaction, Anonymous Fallback

- **TOMBSTONE** preserves slot existence (e.g., auction referenced from chat / order history) while suppressing content. Only `id` + tombstone marker.
- **REDACT** preserves entity existence with explicit redaction markers — not silent omission.
- **Anonymous fallback** for partial title / public-slug hydration. Structural reference only. Never falls back to seller-storefront name, never to email.

## 13. Shipping Summary

If the auction carries a public coarse shipping summary, the same rules as ListingCard apply: coarse only, no raw courier payloads, no seller shipping config internals. Detailed shipping is computed at claim / checkout, not on the card.

## 14. Embedded-Card Rules

AuctionCard embeds:
- exactly one SellerCard (always required, never substituted).

AuctionCard is embedded by:
- ContentCard for auction commerce_share (ADR-010),
- chat attachments / reference resolution endpoints.

## 15. Cross-Surface Convergence

AuctionCard is a single family. The same family applies across:

- Auction detail (`/auctions/:id`)
- Search (`/search/auctions`)
- Storefront auctions, feed auctions, watchlist hydration
- Chat attachment preview, content auction commerce_share embed
- Reference resolution

No per-surface variants.

## 16. Forbidden Patterns

- **Email fallback** in any slot, including transitively via the embedded SellerCard.
- **Auth Identity in any slot**.
- **Endpoint-specific AuctionCard variants** (FeedAuctionCard, SearchAuctionCard, etc.).
- **Repository-built auction cards**.
- **Handler-built auction cards**.
- **Direct DTO exposure** of raw projection rows.
- **Frontend-rendered auction exposure** — clients render what the boundary emits.
- **Silent fallback to raw auction-row exposure**.
- **Raw seller embedding** — substituting SellerCard with raw seller fields, UserCard, or per-surface inline variants.
- **Winner identity leakage** — under any semantic, including `ended_waiting_settlement` and `settled`. Winner identity is private commerce truth.
- **Bid internals leakage** — raw bid IDs, bidder identities, proxy-bid internals, raw bid history rows.
- **Claim / settlement internals leakage** — claim tokens, order IDs, payment IDs, escrow state.
- **Pricing-token leakage** — raw pricing token for claim or Buy Now.
- **AuctionCard rendered as ListingCard variant** — the two are distinct families; no "ProductCard" superset, no per-surface conflation.
- **State conflation** — emitting `settled` for `expired_bnr`, or `sold` for `settled`. The nine-state vocabulary is canonical.
