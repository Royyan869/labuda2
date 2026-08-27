# ADR-008 — ListingCard Family

## Status

Accepted

## Related Documents

- [`docs/foundation.md`](../foundation.md) — Canonical Authorities (Public Exposure, Identity, Discovery); FCFS inventory
- [`docs/architecture.md`](../architecture.md) — Discovery / Projection Design, Commerce Authority Model
- [`docs/contracts/public-card-boundary.md`](../contracts/public-card-boundary.md) — boundary contract
- ADR-001 Pricing Token Authority; ADR-002 Ledger as Authority; ADR-003 Governance Evaluator; ADR-004 Discovery / Projection Boundary
- Companion ADRs: 007 SellerCard, 009 AuctionCard, 010 ContentCard

---

## 1. Decision

ListingCard is the canonical exposure shape for a listing entity. Every listing-emitting public surface (discovery, search, storefront, feed, content embed, favorites / shortlist, chat attachment, reference resolution) MUST flow through the ListingCard family builder.

ListingCard is **distinct from** AuctionCard (separate lifecycle, pricing model, and embed semantics — see ADR-009). It is **embedded by** ContentCard for commerce shares (ADR-010). It **embeds** SellerCard for the listing's seller (ADR-007).

## 2. Ownership

**Commerce domain** owns ListingCard.

ListingCard exposure authority covers: public-display fields, coarse availability vocabulary, coarse shipping summary, lifecycle shapes, redacted / tombstone / removed shapes, cross-surface convergence rules.

Authority boundaries:

- **Pricing authority** remains with the PricingSnapshot / pricing token / checkout pipeline (ADR-001). ListingCard exposes only the public display price; it is **not** checkout authority. Stale display price does not constitute a transaction commitment; revalidation at checkout against the canonical pricing token is the canonical commerce path.
- **Inventory authority** remains with the listing / order transaction path (ADR-002). ListingCard exposes only the coarse availability state derived from canonical inventory truth; it is **not** inventory authority. ListingCard never reserves, holds, or commits inventory.
- **Evaluator decision** is produced by the canonical evaluator (ADR-003).
- **Seller embedding** flows through SellerCard (ADR-007). ListingCard does not redefine seller rendering.

## 3. Canonical Card Shape

```
ListingCard {
  id              : opaque listing reference                (every semantic)
  card_state      : enum {full, sold, hidden, expired,
                          seller_unavailable, removed,
                          tombstone, redacted,
                          anonymous_fallback}

  -- Public Identity Reference (listing-level)
  title           : public listing title                    (full / redacted)
  cover_image     : canonical cover-image reference         (full only)
  gallery_refs    : ordered list of public-image references (full only)
  public_slug     : opaque routing slug | null              (full / redacted)

  -- Public Commerce Attributes
  display_price   : public display price                    (full only)
                    -- never the pricing token
                    -- never raw fee / discount / coins internals
  display_currency : currency code                          (full only)
  availability    : enum {available, unavailable, sold,
                          removed, hidden, expired,
                          seller_unavailable}
  shipping_summary : coarse shipping summary | null         (full / redacted)
                    -- never raw courier API payload
                    -- never raw seller shipping config

  -- Public Lifecycle State (coarse only)
  lifecycle_state : enum {active, unavailable, removed} | null

  -- Embedded SellerCard
  seller          : SellerCard reference                    (every non-omission semantic)
                    -- the canonical seller embedding;
                       never raw seller fields
}
```

## 4. Allowed Field Categories

| Category | Slot | Rule |
|---|---|---|
| Public Identity Reference (listing-level) | `title`, `public_slug` | listing's own public name; not the seller's |
| Public Display Attributes | `cover_image`, `gallery_refs` | canonical media references |
| Public Commerce Attributes | `display_price`, `display_currency`, `availability`, `shipping_summary` | coarse public exposure; never authority |
| Public Lifecycle State | `lifecycle_state` | coarse only |
| Public Audit Reference | `id` | opaque |
| Embedded card | `seller: SellerCard` | canonical seller embedding |

## 5. Forbidden Field Categories

- **Auth Identity** in any slot, including transitively via the embedded SellerCard (`COALESCE(storefront_name, email)` is forbidden recursively).
- **Pricing Authority Fields** — raw pricing token, pricing snapshot internals, fee / discount / coins internals. ListingCard exposes only the coarse display price.
- **Inventory Internals** — reservation flags (which do not exist by canonical FCFS rule), hidden hold counters, internal FCFS sequencing data, raw stock counters with fine granularity (the coarse availability vocabulary is the public exposure; raw counts are forbidden unless explicitly added as a coarse capacity indicator by future amendment).
- **Financial Authority Fields** — escrow state, settlement state, seller payable, payout state.
- **Internal Moderation Metadata**.
- **Shipping Internals** — raw courier API payloads, internal shipping rate calculation internals, seller-side shipping config beyond the coarse public summary.
- **Capability Internals** — raw subscription state, raw entitlement bitmaps.
- **Realtime Transport Internals**.

## 6. FCFS Inventory / Availability

Per Foundation, inventory is FCFS — no reservation, no shortlist hold, no chat hold, no negotiation hold. The coarse availability vocabulary reflects this:

| Availability state | Meaning |
|---|---|
| `available` | listing exists, not sold, seller capability active, not hidden / expired / removed |
| `unavailable` | catch-all coarse state when listing exists but is not currently buyable (operational degradation, transient unavailability) |
| `sold` | terminal sold state |
| `removed` | listing soft-deleted by seller or admin |
| `hidden` | listing exists but is hidden by seller (visibility scope = private / hidden) |
| `expired` | listing past its expiry window |
| `seller_unavailable` | seller capability is suspended / removed (canonical SellerCard lifecycle propagation) |

Stale availability on a card does NOT commit the platform to a transaction. Order creation revalidates against canonical inventory state (FCFS); the buyer who happens to checkout first wins. Card-level "available" is best-effort projection.

ListingCard MUST NOT carry any field whose presence would imply a reservation has occurred (e.g., `hold_token`, `reserved_until`).

## 7. Price / Snapshot Rendering

`display_price` is the **public display price** — the price a buyer sees on a card before initiating checkout. It is sourced from the canonical listing public-price field.

`display_price` is **never** the checkout authority. Per ADR-001, checkout authority is the pricing token / PricingSnapshot, computed at the checkout pipeline. Card-level price is allowed to be stale relative to the canonical pricing path; checkout revalidation closes the loop.

ListingCard MUST NOT carry: raw pricing token, pricing snapshot internals, fee / discount / coins / shipping internals from the pricing pipeline.

If `display_price` is null for any reason (price hidden, listing in `redacted` state), the card emits null or a redacted-shape variant — never falls back to any other field.

## 8. Seller Embedding

ListingCard ALWAYS embeds a canonical SellerCard reference. The embedded SellerCard:

- is constructed by the SellerCard family builder (ADR-007), never inlined,
- carries its own evaluator decision — the seller's evaluator decision may differ from the listing's (a listing may be ALLOW while its seller is SUSPENDED → the listing emits `seller_unavailable` shape and the SellerCard slot emits its `suspended_*` shape),
- is never substituted with raw seller fields, never with a UserCard, never with a per-surface inline seller variant.

If the seller-capability hydration fails partial, the embedded SellerCard emits `card_state = anonymous_fallback`. Silent fallback to raw seller-row exposure is forbidden.

## 9. Lifecycle Rendering

| Listing state | Card state | Slot rendering |
|---|---|---|
| active, hydrated, seller active | `full` | full card |
| sold | `sold` | terminal — `availability=sold`, optionally redact pricing |
| `is_hidden=true` | `hidden` | seller-controlled hide; default = slot omission on discovery |
| past expiry window | `expired` | `availability=expired`, no checkout |
| seller suspended / removed | `seller_unavailable` | listing visible per evaluator; SellerCard emits suspended/removed shape; checkout blocked |
| `deleted_at IS NOT NULL` | `removed` | default = slot omission; reference-integrity surfaces emit redacted-shape with `id` only |
| evaluator REDACT | `redacted` | per evaluator; explicit redaction markers |
| evaluator TOMBSTONE | `tombstone` | slot persists; no display attributes; only `id` + tombstone marker |
| canonical title / cover hydration partial | `anonymous_fallback` | structural reference only |

SUSPENDED / hidden states are reversible. REMOVED / sold are terminal (per their domain rules).

## 10. Tombstone, Redaction, Anonymous Fallback

- **TOMBSTONE** preserves slot existence (e.g., a listing referenced from chat / order history) while suppressing content. No display attributes, no pricing, no seller details — only `id` + tombstone marker.
- **REDACT** preserves entity existence while marking specific categories redacted. Per evaluator decision; explicit redaction markers — not silent omission.
- **Anonymous fallback** applies when canonical title / public-slug hydration is unavailable. Emits structural reference only. Never substitutes title with seller fields, never falls back to seller-storefront name, never falls back to email.

## 11. Shipping Summary

`shipping_summary` is a **coarse** public summary — typically derived from seller-configured shipping options + public coverage indicators (e.g., "ships from Bandung; shipping calculated at checkout"). The card MUST NOT carry:

- raw courier API payloads,
- raw seller shipping rate config,
- live courier rate calculations,
- internal shipping cost overrun signals.

Detailed shipping cost is computed at checkout, not on the card.

## 12. Embedded-Card Rules

ListingCard embeds:
- exactly one SellerCard (always required, never substituted).

ListingCard is embedded by:
- ContentCard for `commerce_share` content (ADR-010),
- chat attachments / order references / reference resolution endpoints.

Per the boundary contract, recursive embedding follows the same hydration topology — embedded SellerCard has its own evaluator decision and its own builder pass.

## 13. Cross-Surface Convergence

ListingCard is a single family. The same family applies across:

- Listing detail (`/listings/:id`)
- Search (`/search/listings`) — predicate filters on canonical listing-public attributes; never aliasing email or other Auth Identity columns
- Storefront listings, feed listings, favorites / shortlist hydration
- Chat attachment preview, content commerce_share embed
- Reference resolution

No per-surface variants (FeedListingCard, SearchListingCard, StorefrontListingCard) — all forbidden.

## 14. Forbidden Patterns

- **Email fallback** in any slot, including transitively through the embedded SellerCard.
- **Auth Identity in any slot**, including SQL aliasing.
- **Endpoint-specific ListingCard variants** (FeedListingCard, SearchListingCard, StorefrontListingCard, FavoritesListingCard).
- **Repository-built listing cards** — repositories return raw entities + metadata.
- **Handler-built listing cards** — no inline `gin.H` listing-card maps.
- **Direct DTO exposure** of raw projection rows to the wire on a public surface.
- **Frontend-rendered listing exposure** — clients render what the boundary emits.
- **Silent fallback to raw listing-row exposure** — fail-loud.
- **Raw seller embedding** — substituting SellerCard with raw seller fields, with a UserCard, or with per-surface inline seller variants.
- **Pricing-token leakage** — emitting raw pricing token, raw snapshot internals, raw fee / discount / coins fields.
- **Inventory-internal leakage** — emitting reservation flags (which do not exist), hidden hold counters, raw stock with fine granularity.
- **Financial-field promotion** — slots sourced from `escrow_balance`, `seller_payable`, `gateway_payload`, `payout_state`, etc.
- **Reservation implication** — fields that imply a hold has occurred (`hold_token`, `reserved_until`). Inventory is FCFS; no reservation exists.
