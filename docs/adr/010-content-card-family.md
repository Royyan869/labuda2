# ADR-010 — ContentCard Family

## Status

Accepted

## Related Documents

- [`docs/foundation.md`](../foundation.md) — Canonical Authorities (Public Exposure, Identity, Discovery)
- [`docs/architecture.md`](../architecture.md) — Discovery / Projection Design, Identity / Trust Model
- [`docs/contracts/public-card-boundary.md`](../contracts/public-card-boundary.md) — boundary contract
- [`docs/contracts/viewer-context.md`](../contracts/viewer-context.md) — viewer input contract
- ADR-003 Governance Evaluator; ADR-004 Discovery / Projection Boundary
- Companion ADRs: 006 UserCard, 007 SellerCard, 008 ListingCard, 009 AuctionCard

---

## 1. Decision

ContentCard is the canonical exposure shape for social content (posts, articles). Every content-emitting public surface (`/search/content`, feed, profile content, notification preview, websocket preview) MUST flow through the ContentCard family builder.

ContentCard **embeds** UserCard or SellerCard for the author and **embeds** ListingCard or AuctionCard for commerce shares.

## 2. Ownership

**Social domain** owns ContentCard.

The Social domain defines: allowed field categories, lifecycle / redaction / tombstone shapes, embedded-card selection rules.

Embedded-card families (UserCard, SellerCard, ListingCard, AuctionCard) are owned by their respective domains; ContentCard does not redefine their rendering.

## 3. Canonical Card Shape

```
ContentCard {
  id              : opaque content reference                (every semantic)
  card_state      : enum {full, suspended_author, removed_author,
                          tombstone, redacted, anonymous_fallback}

  -- Public Content Attributes
  type            : enum {post, article, ...}              (full / redacted)
  caption         : string                                  (full only;
                                                            redacted-shape on REDACT)
  media           : []MediaReference                        (full only;
                                                            redacted-shape on REDACT)
  created_at      : timestamp                               (full / redacted)

  -- Embedded author
  author          : UserCard | SellerCard                   (every non-tombstone semantic)

  -- Embedded commerce share
  commerce_share  : ListingCard | AuctionCard | null        (when content embeds
                                                            a listing or auction)

  -- Optional public audit metadata
  governance_decision_id : opaque audit reference | null

  -- Optional public lifecycle marker
  lifecycle_marker : enum {active, unavailable, removed} | null
}
```

`author` is present on every non-tombstone semantic. `commerce_share` is present only when content embeds a listing/auction.

## 4. Allowed Field Categories

| Category | Slot | Rule |
|---|---|---|
| Public Content Attributes | `type`, `caption`, `media`, `created_at` | full on `full`; redacted-shape on `redacted`; absent on `tombstone` / `removed` |
| Public Identity Reference | recursive via `author` | always present on non-tombstone semantics; never directly on ContentCard |
| Public Display Attributes | `caption`, `media`; recursive via `author` | as above |
| Public Lifecycle State | `lifecycle_marker`; recursive via `author` | optional surface-level marker; recursive author lifecycle from UserCard / SellerCard |
| Public Capability State | recursive via `author` | only via embedded UserCard / SellerCard |
| Public Verification Indicator | recursive via `author` | only via embedded UserCard / SellerCard; coarse boolean only |
| Public Commerce Attributes | recursive via `commerce_share` | only via embedded ListingCard / AuctionCard |
| Public Audit Reference | `id`, optional `governance_decision_id` | opaque only |

ContentCard does NOT carry Public Relationship Indicators directly. Viewer-relative indicators, if surface needs them, are computed at the discovery / feed envelope level.

## 5. Forbidden Field Categories

ContentCard inherits the boundary contract's forbidden set:

- **Auth Identity** in any slot, including transitively via embedded author or commerce_share.
- **Internal Moderation Metadata** — `governance_decision_id` is an opaque reference only; raw moderation metadata is never emitted.
- **Financial Authority Fields** — content surfaces are not commerce-decision surfaces.
- **Pricing Authority Fields** — embedded ListingCard / AuctionCard handle commerce attributes per their own ADRs; ContentCard does not promote pricing internals to its own slots.
- **Inventory Internals** — handled by embedded commerce families.
- **Capability Internals** — handled by embedded UserCard / SellerCard.
- **Relationship Graph Internals**.
- **Verification Internals** — handled by embedded UserCard / SellerCard as coarse booleans.
- **Realtime Transport Internals**.

The forbidden-category enforcement is **recursive**: a slot sourced from a forbidden category via an embedded card is itself a forbidden emission, regardless of which card carried it.

## 6. Lifecycle Rendering

ContentCard rendering is driven by:
- the content entity's own lifecycle (active / removed),
- the content's moderation state (`is_hidden`),
- the embedded author's lifecycle (active / suspended / removed).

| Content state | Author state | Card state | Slot composition |
|---|---|---|---|
| active | active | `full` | full content + full author + optional commerce_share |
| active | suspended | `suspended_author` | full content + author suspended-shape + optional commerce_share |
| active | removed | `removed_author` | per surface: omit OR emit removed_author shape with author removed-shape + minimal content |
| moderated (`is_hidden`) | n/a | `tombstone` | `id` + opaque audit reference; no content fields, no author |
| `deleted_at` set | n/a | per surface: omission default OR `removed` for reference-integrity surfaces | `id` only + optional opaque audit reference |
| evaluator REDACT | n/a | `redacted` | full structural fields + redaction markers; embedded author redacted-shape |

Per-state surface policy (omission vs persistence) is decided at the surface-context evaluator-input level. Discovery default is omission for `removed` / `tombstone`; chat / comment surfaces may require persistence for reference integrity in conversation threads.

## 7. Tombstone, Redaction, Anonymous Fallback

- **REDACT** — explicit redaction markers; never silent omission. `caption` redacted-shape, `media` redacted-shape, `author` recursively redacted, `commerce_share` recursively redacted (or preserved if its own evaluator decision allows).
- **TOMBSTONE** — slot persists, content suppressed. Only `id` + optional `governance_decision_id`. Author and commerce_share absent.
- **Anonymous fallback** — rare on ContentCard itself; embedded `author` carries the anonymous-safe fallback per UserCard / SellerCard rules. Email fallback forbidden under all semantics.

## 8. Embedded-Card Rules

### 8.1 Author embedding

- Typical user author → embed UserCard.
- Seller-acting author (promotional content, author embedding own commerce shares, content tagged with seller capability) → embed SellerCard.

The capability-context decision is an evaluator-input concern; the family-builder consumes the evaluator's emitted decision and selects the embedded card accordingly. The family-builder does NOT decide capability context itself.

### 8.2 Commerce-share embedding

- Listing share → embed ListingCard (ADR-008).
- Auction share → embed AuctionCard (ADR-009).

### 8.3 Recursive embedding

The embedded author has its own evaluator decision (the author may be SUSPENDED while the content is ALLOW). The embedded commerce_share has its own evaluator decision recursively. Recursion bottoms out at leaf families (UserCard, SellerCard).

### 8.4 No callback to evaluator from card-builder

The ContentCard builder consumes per-entity decisions the caller already obtained. It dispatches to embedded family builders, but neither it nor the embedded builders re-enter the evaluator. The hydration topology is one-way (repository → evaluator → card-builder → serializer).

## 9. Cross-Surface Convergence

ContentCard is a single family. The same family applies across `/search/content`, feed, profile content, notification preview, websocket preview. No per-surface families.

Surface-specific reduced variants (e.g., notification-preview with `caption` truncated and `media` absent) are permitted at the family-domain ADR amendment level — they remain members of the canonical ContentCard family, parameterized renderings, not separate families.

## 10. Forbidden Patterns

- **Endpoint-specific ContentCard variants** (SearchContentCard, FeedContentCard, ProfileContentCard, NotificationContentCard).
- **Handler-built cards** — no inline `gin.H` content-card maps.
- **Repository-built cards** — repositories return raw entities + metadata.
- **Frontend-rendered governance** — clients render what the boundary emits; never re-decide redaction / tombstone / anonymous fallback.
- **Inline ContentCard mutation** — handler / middleware / serializer modifying the card based on evaluator decision tuple. Shape decisions belong in the family-builder.
- **Per-surface lifecycle reinterpretation** — surfaces that interpret canonical lifecycle states differently from this ADR.
- **Silent fallback to raw exposure** — fail-loud only.
- **Mixed legacy / canonical emission** — concurrent emission of legacy gin.H content shape and canonical ContentCard on the same surface.
- **Direct field promotion from forbidden categories** — including via embedded card emergence. Recursive enforcement.
