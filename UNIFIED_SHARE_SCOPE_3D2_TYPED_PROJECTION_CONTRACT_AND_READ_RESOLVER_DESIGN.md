# UNIFIED SHARE SCOPE 3D2 — TYPED PROJECTION CONTRACT AND READ RESOLVER DESIGN

**STATUS:** `UNIFIED_SHARE_SCOPE_3D2_TYPED_PROJECTION_CONTRACT_AND_READ_RESOLVER_DESIGN_COMPLETE`

**DATE:** 2026-08-08

**MODE:** DESIGN ONLY — NO IMPLEMENTATION

**REPOSITORY:** `d:\Project\labuda`

---

## 1. VERDICT

`UNIFIED_SHARE_SCOPE_3D2_TYPED_PROJECTION_CONTRACT_AND_READ_RESOLVER_DESIGN_COMPLETE`

No blockers. The design reuses existing canonical public-card types (`publiccard.UserCard`,
`publiccard.SellerCard`, `publiccard.FixedPriceSaleCard`, `publiccard.AuctionCard`,
`publiccard.ContentCard`) as LIVE payloads, defines typed fallback parsers matching the
existing Scope 3C fallback shapes, and proposes a single batch-oriented resolver
interface. Migration 000034 remains sufficient.

---

## 2. PROJECTION ENVELOPE

### Go type

```go
// ResourceProjection is the canonical typed viewer-facing resource projection
// for a Chat message occurrence. Exactly one payload field is populated based
// on ResourceType and State.
//
// FILE: backend/internal/interaction/chat/application/chat_resource_projection.go (NEW)
package application

type ProjectionState string

const (
    ProjectionStateLive            ProjectionState = "LIVE"
    ProjectionStateFallbackAllowed ProjectionState = "FALLBACK_ALLOWED"
    ProjectionStateTombstone       ProjectionState = "TOMBSTONE"
)

type ResourceProjection struct {
    State        ProjectionState `json:"state"`
    ResourceType string          `json:"resource_type"` // "profile"|"content"|"fixed_price_sale"|"auction"
    ResourceID   string          `json:"resource_id"`   // UUID string
    CanonicalURL string          `json:"canonical_url"`

    // Capabilities present on LIVE and FALLBACK_ALLOWED; always empty on TOMBSTONE.
    Capabilities ResourceCapabilities `json:"capabilities"`

    // Exactly ONE of the following is non-nil based on State + ResourceType.
    Profile        *ProfileProjectionPayload        `json:"-"`
    Content        *ContentProjectionPayload        `json:"-"`
    FixedPriceSale *FixedPriceSaleProjectionPayload `json:"-"`
    Auction        *AuctionProjectionPayload        `json:"-"`
}
```

### Custom JSON serialization

The four typed payloads use `json:"-"` on the struct and a custom `MarshalJSON` that
emits the correct key:

```json
// LIVE profile:
{
  "state": "LIVE",
  "resource_type": "profile",
  "resource_id": "uuid",
  "canonical_url": "/user/uuid",
  "capabilities": { "can_open": true, "can_chat": true },
  "profile": { /* ProfileProjectionPayload */ }
}

// LIVE content:
{
  "state": "LIVE",
  "resource_type": "content",
  "resource_id": "uuid",
  "canonical_url": "/content/uuid",
  "capabilities": { "can_open": true, "can_interact": true },
  "content": { /* ContentProjectionPayload */ }
}

// TOMBSTONE:
{
  "state": "TOMBSTONE",
  "resource_type": "content",
  "resource_id": "uuid",
  "canonical_url": "/content/uuid",
  "capabilities": {}
}
```

**Why this approach over alternatives:**
- **Typed interface/sealed-style**: Go has no sealed interfaces. A marker interface with
  type-switch in MarshalJSON works but adds indirection.
- **Four nullable fields**: Creates null-soup — explicitly prohibited by Scope 3D §4.
- **Discriminated DTO with custom MarshalJSON**: One non-nil typed field, discriminated
  at serialization time. Clean, testable, no null-soup. **This is the recommended approach.**

### Why not reuse publiccard types directly in the envelope?

The `publiccard` types (`FixedPriceSaleCard`, `AuctionCard`, `ContentCard`) carry
`Lifecycle` as a coarsened string and use `SellerCard`/`UserCard` for identity. These
are exactly the LIVE payload shapes — but the Chat projection needs additional fields
beyond what `publiccard` exposes today:

- **Profile**: needs `store_name` and `is_seller` (not on `UserCard`)
- **Content**: needs `ShareReference` → depth-1 nested indicator (not on `ContentCard`)
- **FPS/Auction**: need richer capability signals than `publiccard` lifecycle

So: **Chat projection types WRAP or EXTEND publiccard types**, not replace them.
The LIVE payload for FPS is `FixedPriceSaleCard` with additional Chat-specific
capability fields at the envelope level.

---

## 3. STATE SEMANTICS

### LIVE

> Current canonical resource authority is available and viewer may see it.

Selection: the resource's current domain truth (visibility, lifecycle, privacy,
moderation) permits the viewer to access it.

Implementation: batch-query current resource state → apply canonical viewer access
rules → if allowed, populate typed payload.

### FALLBACK_ALLOWED

> Live projection cannot legitimately be produced, but historical display is
> still safe and permitted.

Selection: the live resource query fails with a **transient infrastructure error**
(NOT a privacy/block/delete decision), AND the resource type + viewer relationship
permits historical display.

Implementation: catch transient errors from live batch loads → parse
`fallback_snapshot` → apply typed sanitization → emit FALLBACK_ALLOWED payload.

**This state is RARE.** Most "resource not available" cases are permanent
privacy/removal decisions → TOMBSTONE.

Legitimate FALLBACK_ALLOWED cases:
- DB connection error during live batch query
- Resource row missing (deleted between occurrence write and read) but not for
  privacy-sensitive types

### TOMBSTONE

> Neither live data nor historical fallback may be exposed.

Selection: the resource is deleted, hidden, moderated, blocked, private (viewer ≠
author), suspended, banned, or any other permanent/permanent-seeming privacy decision.

Implementation: return projection with State=TOMBSTONE, empty capabilities, no payload.

### State selection precedence

```
1. Is viewer blocked by resource owner?
   YES → TOMBSTONE (for Profile/Content; not applicable to FPS/Auction)

2. Is resource deleted/hidden/moderated?
   YES → TOMBSTONE

3. Is resource private with viewer ≠ author?
   YES → TOMBSTONE

4. Is resource owner suspended/banned/deleted?
   Profile: TOMBSTONE
   Content: TOMBSTONE (author lifecycle failure = content inaccessible)
   FPS/Auction: TOMBSTONE (seller removed = commerce inaccessible)

5. Can live state be loaded?
   YES → LIVE

6. Is fallback display safe?
   Profile: NO (privacy always wins) → TOMBSTONE
   Content: NO (deleted/hidden/moderated content must not leak) → TOMBSTONE
   FPS/Auction: YES for transient DB errors where seller is still active → FALLBACK_ALLOWED

7. Else → TOMBSTONE (fail-closed)
```

---

## 4. PROFILE CONTRACT

### LIVE payload fields

```go
type ProfileProjectionPayload struct {
    Username  string  `json:"username"`
    AvatarURL *string `json:"avatar_url,omitempty"`
    StoreName *string `json:"store_name,omitempty"`
    IsSeller  bool    `json:"is_seller"`
    Lifecycle string  `json:"lifecycle"` // "active" | "unavailable" | "removed"
}
```

### Canonical sources

| Field | Source |
|---|---|
| Username | `user_profiles.username` (via `userdisplay.FetchMany`) |
| AvatarURL | `user_profiles.avatar_url` |
| StoreName | `seller_profiles.store_name` WHERE `status='active'` |
| IsSeller | `seller_profiles.user_id IS NOT NULL` |
| Lifecycle | `viewercontext.CoarsenLifecycle(users.account_status, users.deleted_at)` |

### TOMBSTONE conditions

| Condition | State |
|---|---|
| Profile deleted (`users.deleted_at IS NOT NULL`) | TOMBSTONE |
| Profile banned (`users.account_status = 'banned'`) | TOMBSTONE |
| Profile suspended (`users.account_status = 'suspended'`) | TOMBSTONE (temporary) |
| Viewer blocked by profile | TOMBSTONE (viewer-specific) |

### FALLBACK_ALLOWED

**Profile never uses FALLBACK_ALLOWED.** Privacy always wins over historical display.
If a profile is deleted/suspended/banned, the tombstone is permanent. If the DB query
fails transiently, the error propagates (fail-closed per 3D-1 pattern).

### Viewer capabilities

| Capability | Source |
|---|---|
| `can_open` | Always true (profile identity is the resource) |
| `can_chat` | `!blocked && viewer.account_status == 'active'` |
| `can_report` | `viewer.account_status == 'active' && viewer != profile` |

---

## 5. CONTENT CONTRACT

### LIVE payload fields

```go
type ContentProjectionPayload struct {
    Caption     *string            `json:"caption,omitempty"`
    Media       []MediaRef         `json:"media"`
    Lifecycle   string             `json:"lifecycle"` // "active" | "removed"
    CreatedAt   string             `json:"created_at"` // RFC3339
    Author      AuthorIdentity     `json:"author"`
    NestedResource *NestedResourceIndicator `json:"nested_resource,omitempty"`
}

type AuthorIdentity struct {
    ID        uuid.UUID `json:"id"`
    Username  string    `json:"username"`
    AvatarURL *string   `json:"avatar_url,omitempty"`
    Lifecycle string    `json:"lifecycle"`
}

type MediaRef struct {
    URL string `json:"url"`
}
```

### Canonical sources

| Field | Source |
|---|---|
| Caption | `contents.caption` |
| Media | `content_media.media_url` ORDER BY `sort_order` / `position` |
| Lifecycle | `contents.Status.PublicLifecycle()` → "active" / "removed" |
| CreatedAt | `contents.created_at` |
| Author.ID | `contents.author_id` |
| Author.Username | `user_profiles.username` |
| Author.AvatarURL | `user_profiles.avatar_url` |
| Author.Lifecycle | `viewercontext.CoarsenLifecycle(author.account_status, author.deleted_at)` |

### TOMBSTONE conditions

| Condition | State |
|---|---|
| Content deleted (`contents.deleted_at IS NOT NULL`) | TOMBSTONE |
| Content hidden (`contents.is_hidden = true`) | TOMBSTONE |
| Content moderated (`contents.status = 'deleted'`) | TOMBSTONE |
| Private content, viewer ≠ author | TOMBSTONE |
| Followers-only, viewer doesn't follow | TOMBSTONE |
| Viewer blocked by author | TOMBSTONE |
| Author suspended/banned/deleted | TOMBSTONE |

### FALLBACK_ALLOWED

**Content never uses FALLBACK_ALLOWED for privacy-sensitive states.** The only
conceivable case is a transient DB error during batch load of a public,
non-hidden, non-deleted content from an active author — but even then, returning
stale fallback data for content that might have been deleted/hidden since would
risk privacy leakage. Safer to fail-closed (propagate DB error per 3D-1).

### Viewer capabilities

| Capability | Source |
|---|---|
| `can_open` | Always true when LIVE |
| `can_interact` | `!content.is_hidden && content.status == 'active' && viewer.account_status == 'active'` |
| `can_report` | `viewer.account_status == 'active' && viewer != author` |
| `can_manage` | `viewer == author \|\| viewer.isAdmin` |

---

## 6. CONTENT DEPTH-1 CONTRACT

### Design

When a Content occurrence is LIVE and the Content carries `ShareReference != nil`:

1. Resolve the `ShareReference.TargetType` → `ShareReference.TargetID` to a compact
   identity-only nested indicator.
2. Depth is exactly 1 — do NOT recurse into the nested resource's own references.
3. The primary occurrence identity remains the Content, NOT the nested resource.
4. If the nested resource is inaccessible (deleted/private/blocked): suppress
   `nested_resource` entirely. Do NOT leak historical title/image.

### Nested indicator type

```go
type NestedResourceIndicator struct {
    ResourceType string `json:"resource_type"` // "content"|"fixed_price_sale"|"auction"|"profile"
    ResourceID   string `json:"resource_id"`   // UUID string
}
```

No title, no image, no preview. Identity only. The mobile client resolves display
from its own cache or a follow-up request.

### Resolution

`ShareReference.TargetType` maps directly to chat resource types:

| ShareTargetType | Chat ResourceType |
|---|---|
| `content` | `content` |
| `fixed_price_sale` | `fixed_price_sale` |
| `auction` | `auction` |
| `profile` | `profile` |

### Access check for nested indicator

For each nested resource: apply the SAME canonical access rules as if it were a
primary occurrence:
- Profile: check deleted/banned/suspended/blocked
- Content: check visibility/privacy/moderation/author lifecycle
- FPS/Auction: check seller lifecycle/deletion

If access denied → suppress `nested_resource`. No tombstone leakage.

### Batch implications

Nested resources require one additional batch query per distinct nested resource
type present across all Content occurrences on the page. See §16 (Query Budget).

---

## 7. FPS CONTRACT

### LIVE payload fields

Reuse `publiccard.FixedPriceSaleCard` as the base, with additional Chat-specific
fields at the envelope capability level:

```go
type FixedPriceSaleProjectionPayload struct {
    publiccard.FixedPriceSaleCard          // embeds ID, Title, Thumbnail, Price, Currency, Lifecycle, Seller
    QuantityAvailable *int64  `json:"quantity_available,omitempty"`
}
```

`QuantityAvailable` is nullable — only populated when the canonical FPS entity
provides it (the `quantity_available` column from migration 000009).

### Canonical sources

| Field | Source |
|---|---|
| ID | `fixed_price_sales.id` |
| Title | `products.title` |
| Thumbnail | `products.media_urls->>0` |
| Price | `fixed_price_sales.price_per_unit` |
| Lifecycle | `fps.Status.PublicLifecycle()` → "active" / "unavailable" |
| Seller | `publiccard.NewSellerCardWithBothLifecycles(...)` from seller identity |
| QuantityAvailable | `fixed_price_sales.quantity_available` (may be NULL in older rows) |

### Lifecycle ↔ state matrix

| FPS Status | PublicLifecycle | Projection State | can_interact |
|---|---|---|---|
| `active` | `active` | LIVE | true |
| `draft` | `unavailable` | LIVE | false |
| `sold` | `unavailable` | LIVE | false |
| `withdrawn` | `unavailable` | LIVE | false |
| Seller deleted | N/A | TOMBSTONE | — |

**Terminal lifecycle does NOT automatically tombstone.** Draft/sold/withdrawn
listings remain publicly viewable as LIVE with `can_interact=false`.

### Viewer capabilities

| Capability | Source |
|---|---|
| `can_open` | Always true when LIVE |
| `can_interact` | `fps.Status == 'active' && seller_lifecycle == 'active'` |
| `can_chat` | `viewer.account_status == 'active' && !blocked` |
| `can_negotiate` | `fps.negotiation_enabled && viewer != seller && can_interact` |
| `can_buy` | `can_interact && viewer != seller && viewer.account_status == 'active'` |
| `can_report` | `viewer.account_status == 'active' && viewer != seller` |
| `can_manage` | `viewer == seller \|\| viewer.isAdmin` |

### FALLBACK_ALLOWED

Permitted when:
- Live FPS query fails with transient DB error
- Seller is NOT deleted/banned (verified by a lightweight seller lifecycle check
  that is cheaper than the full FPS query)

Fallback payload: typed historical display only — title, image, store name.
Price/quantity/status/capabilities are NOT derived from fallback.

### TOMBSTONE

- FPS row deleted from DB (`ON DELETE RESTRICT` should prevent this, but
  defensive check)
- Seller banned/deleted
- Legal/privacy removal of the listing

---

## 8. AUCTION CONTRACT

### LIVE payload fields

Reuse `publiccard.AuctionCard` as the base:

```go
type AuctionProjectionPayload struct {
    publiccard.AuctionCard  // embeds ID, Title, Thumbnail, CurrentBid, BuyNowPrice, EndAt, Lifecycle, Seller
}
```

### Canonical sources

| Field | Source |
|---|---|
| ID | `auctions.id` |
| Title | `auctions.title` (or `products.title` for legacy rows) |
| Thumbnail | `products.media_urls->>0` |
| CurrentBid | `auctions.current_bid` (NULL when no bids) |
| BuyNowPrice | `auctions.buy_now_price` (NULL when no buy-now) |
| EndAt | `auctions.end_at` (RFC3339) |
| Lifecycle | `auc.Status.PublicLifecycle()` → "active" / "unavailable" |
| Seller | `publiccard.NewSellerCardWithBothLifecycles(...)` |

### Lifecycle ↔ state matrix

| Auction Status | PublicLifecycle | Projection State | can_interact |
|---|---|---|---|
| `active` | `active` | LIVE | true |
| `waiting_settlement` | `active` | LIVE | false (no new bids) |
| `scheduled` | `unavailable` | LIVE | false |
| `ended` | `unavailable` | LIVE | false |
| `cancelled` | `unavailable` | LIVE | false |
| `expired_bnr` | `unavailable` | LIVE | false |
| `draft` | `unavailable` | LIVE | false |
| Seller deleted | N/A | TOMBSTONE | — |

### Viewer capabilities

| Capability | Source |
|---|---|
| `can_open` | Always true when LIVE |
| `can_interact` | `auc.Status IN ('active') && seller_lifecycle == 'active'` |
| `can_chat` | `viewer.account_status == 'active' && !blocked` |
| `can_bid` | `can_interact && viewer != seller && viewer.account_status == 'active'` |
| `can_report` | `viewer.account_status == 'active' && viewer != seller` |
| `can_manage` | `viewer == seller \|\| viewer.isAdmin` |

### FALLBACK_ALLOWED / TOMBSTONE

Same rules as FPS (§7).

---

## 9. CAPABILITY AUTHORITY TABLE

### Canonical source confirmed by agents

The existing `commerce/shared.ViewerCapabilities` struct
([viewer_capabilities.go](backend/internal/commerce/shared/viewer_capabilities.go))
is the canonical commerce capability authority:

```go
type ViewerCapabilities struct {
    Role         string `json:"role"`          // "guest" | "owner" | "buyer"
    CanManage    bool   `json:"can_manage"`
    CanEdit      bool   `json:"can_edit"`
    CanPromote   bool   `json:"can_promote"`
    CanChat      bool   `json:"can_chat"`
    CanNegotiate bool   `json:"can_negotiate"`
    CanBuy       bool   `json:"can_buy"`
    CanBid       bool   `json:"can_bid"`
    CanBuyNow    bool   `json:"can_buy_now"`
}
```

Built by `buildListingViewerCapabilities` (FPS) and `buildAuctionViewerCapabilities`
(Auction) in their respective response projection files. These are **display-only
computed projections** — not enforced at mutation gates. The actor/capability store
(`internal/platform/capability/`) is admin-focused (finance, governance, moderation
clusters) and NOT used for viewer-facing commerce capabilities.

### `can_report` does not exist

Confirmed by full-codebase grep across all agents: there is no `can_report` or
`CanReport` anywhere in the backend. It must NOT be added to the Chat projection.
The report action is handled by a separate report-create endpoint, not gated by
a pre-computed capability flag.

### Chat projection capability model

The Chat projection carries a **subset** of the commerce `ViewerCapabilities`,
omitting fields irrelevant to Chat context:

```go
// ResourceCapabilities is the viewer-scoped capability projection for a Chat
// resource occurrence. It mirrors the canonical commerce ViewerCapabilities
// but omits Chat-irrelevant fields (CanEdit, CanPromote) and fields never
// applicable in Chat (CanBuyNow — same as CanBid for compact display).
type ResourceCapabilities struct {
    Role         string `json:"role,omitempty"`   // "owner" | "buyer" | "" (guest/other)
    CanChat      bool   `json:"can_chat"`
    CanNegotiate bool   `json:"can_negotiate"`    // FPS only
    CanBuy       bool   `json:"can_buy"`          // FPS only
    CanBid       bool   `json:"can_bid"`          // Auction only
    CanManage    bool   `json:"can_manage"`       // owner | admin
}
```

### Capability derivation (per resource type, per viewer)

| Capability | Profile | Content | FPS | Auction |
|---|---|---|---|---|
| `role` | `"owner"` if viewer==profile, else `""` | `"owner"` if viewer==author, else `""` | `"owner"` if viewer==seller, else `"buyer"` if viewer≠nil, else `"guest"` | Same as FPS |
| `can_chat` | `!blocked && viewer active` | N/A | `!blocked && viewer active && sellerTrustActive` | Same as FPS |
| `can_negotiate` | N/A | N/A | `can_chat && fps.NegotiationEnabled && fps.IsAvailable()` | N/A |
| `can_buy` | N/A | N/A | `can_chat && fps.IsAvailable()` | N/A |
| `can_bid` | N/A | N/A | N/A | `can_chat && auction.Status == active` |
| `can_manage` | `viewer==self \|\| admin` | `viewer==author \|\| admin` | `viewer==seller \|\| admin` | `viewer==seller \|\| admin` |

### Canonical sources for each derivation

| Input | Source |
|---|---|
| `blocked` | `blockcheck.IsBidirectionallyBlocked` (bidirectional; fail-open on error per existing commerce convention) |
| `viewer active` | `viewercontext.CoarsenLifecycle(account_status, deleted_at) == "active"` |
| `sellerTrustActive` | `viewercontext.CoarsenSellerTrust(subscription_status) == "active"` |
| `fps.IsAvailable()` | `FixedPriceSale.Status == Active && QuantityAvailable > 0` |
| `fps.NegotiationEnabled` | `fixed_price_sales.negotiation_enabled` column |
| `auction.Status` | `auctions.status` column |
| `isAdmin` | `auth.RoleChecker.IsAdmin` or gin context `is_admin` |

**No new capability names invented. No Chat-owned duplicated business rules.**
Every capability derivation reuses an existing canonical check already present
in the codebase.

---

## 10. FALLBACK TYPED CONTRACTS

### Parsing contract

Each fallback type has a typed Go struct that matches the exact JSON shape produced
by Scope 3C fallback builders (`chat_occurrence_fallback_builder.go` and
`chat_resource_authorizer_adapter.go`).

```go
// FILE: backend/internal/interaction/chat/application/chat_fallback_parsers.go (NEW)

type ProfileFallbackPayload struct {
    Username  string  `json:"username"`
    AvatarURL *string `json:"avatar_url"`
    StoreName *string `json:"store_name"`
    IsSeller  bool    `json:"is_seller"`
}

type ContentFallbackPayload struct {
    CaptionExcerpt  *string `json:"caption_excerpt"`
    FirstMediaURL   *string `json:"first_media_url"`
    AuthorUsername  string  `json:"author_username"`
    AuthorAvatarURL *string `json:"author_avatar_url"`
}

type FixedPriceSaleFallbackPayload struct {
    Title            string  `json:"title"`
    ImageURL         *string `json:"image_url"`
    SellerStoreName  string  `json:"seller_store_name"`
    SellerStoreImage *string `json:"seller_store_image"`
}

type AuctionFallbackPayload struct {
    Title            string  `json:"title"`
    ImageURL         *string `json:"image_url"`
    SellerStoreName  string  `json:"seller_store_name"`
    SellerStoreImage *string `json:"seller_store_image"`
}
```

### Parsing function

```go
func ParseFallbackSnapshot(raw json.RawMessage, resourceType ResourceOccurrenceResourceType) (interface{}, error)
```

Dispatches on resource type, unmarshals into the typed struct. Returns error on
malformed JSON.

### Malformed fallback handling

If `json.Unmarshal` fails on a `fallback_snapshot` that was produced by Scope 3C
builders (which always produce valid JSON), this indicates data corruption.

Behavior: **projection omits the occurrence** (nil entry in the map for that
message_id). The message itself is still returned normally. This is safer than
returning TOMBSTONE (which would assert the resource is inaccessible — it might
not be) and safer than returning LIVE (which would assert current authority —
we don't have it).

### Safe historical fields for FALLBACK_ALLOWED projection

| Type | Safe fields (display-only) | Unsafe fields (must NOT project) |
|---|---|---|
| Profile | N/A — FALLBACK_ALLOWED not used | — |
| Content | N/A — FALLBACK_ALLOWED not used | — |
| FPS | `title`, `image_url`, `seller_store_name` | (none in fallback — price/status not stored) |
| Auction | `title`, `image_url`, `seller_store_name` | (none in fallback — bid/status not stored) |

The fallback builders intentionally exclude price, bid, quantity, availability,
and status. So the fallback is already safe for display. The typed parsers
simply preserve this contract.

---

## 11. TOMBSTONE CONTRACT

### Message tombstone vs. resource tombstone

These are independent:

| Message State | Resource State | Behavior |
|---|---|---|
| Normal | LIVE | Full message + resource projection |
| Normal | TOMBSTONE | Full message + `resource_projection: {state: "TOMBSTONE", ...}` |
| Normal | FALLBACK_ALLOWED | Full message + historical resource projection |
| Soft-deleted | Any | `is_hidden: true`, no body/attachment/sender, no resource_projection |

A message can exist normally (`deleted_at IS NULL`) while its resource
projection is TOMBSTONE. The message timeline is preserved; only the
resource identity is redacted.

### Tombstone response shape

```json
{
  "state": "TOMBSTONE",
  "resource_type": "profile",
  "resource_id": "uuid",
  "canonical_url": "/user/uuid",
  "capabilities": {}
}
```

No `profile`/`content`/`fixed_price_sale`/`auction` payload key. No reason string.
No internal moderation/account detail. Identity + timeline preserved for
stable rendering.

### When to include vs. omit

**Always include**: Even for TOMBSTONE, the projection preserves `resource_type`,
`resource_id`, and `canonical_url`. This enables mobile to render a stable
placeholder card ("Content unavailable") without leaking what was there.

**Exception**: When the message itself is soft-deleted (`deleted_at != NULL`),
the entire `resource_projection` field is suppressed (same as body/attachment
suppression today).

---

## 12. RESOLVER INTERFACE

### Go boundary

```go
// FILE: backend/internal/interaction/chat/application/chat_resource_projection.go (NEW)

// ResourceProjectionResolver resolves occurrence rows into viewer-aware
// ResourceProjections. It is batch-oriented: all occurrences for a message
// page are resolved together.
//
// The resolver is a READ-ONLY port owned by the Chat application layer.
// Implementation lives in serverboot to wire domain services without
// package cycles.
type ResourceProjectionResolver interface {
    // ResolveResourceProjections resolves all occurrences for a message page
    // into viewer-specific projections. Returns a map keyed by message_id.
    // Occurrences that fail resolution (malformed fallback, missing source row
    // for FALLBACK_ALLOWED path) are omitted from the map — the message is
    // still returned normally.
    ResolveResourceProjections(
        ctx context.Context,
        viewerID uuid.UUID,
        occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
    ) (map[uuid.UUID]*ResourceProjection, error)
}
```

### Package placement — no cycles

```
chat/application/
  chat_resource_projection.go     ← ResourceProjection type + ResourceProjectionResolver interface
  chat_message_page_result.go     ← MessagePageResult (existing)
  chat_service.go                 ← Service (existing)

serverboot/
  chat_projection_resolver.go     ← chatProjectionResolver implements ResourceProjectionResolver
  dependencies.go                 ← wires resolver into Service
```

**Dependency chain**: `serverboot` → `chat/application` (existing) → `chat/entity` (existing).
`serverboot` imports domain packages (`commerce/fixedprice`, `commerce/auction`,
`social/content`, `identity/user`) to implement the resolver.
`chat/application` imports NONE of those — it only imports `chat/entity`.

No cycle. The resolver interface lives in `chat/application` (owned by Chat).
The implementation lives in `serverboot` (wires domain services).

### Why not reuse ResourceAuthorizer?

`ResourceAuthorizer` (Scope 3C) is a **write-side** authorization port:
- `AuthorizeShare` checks whether viewerID may share a resource → returns fallback
- `AuthorizeDirect` checks ownership + market capability for direct insert
- Both mutate nothing but are tightly coupled to write-path semantics

The read resolver needs:
- Batch loading (authorizer is one-at-a-time)
- Viewer-aware state determination (authorizer checks access for sharing, not viewing)
- Capability projection (authorizer doesn't compute can_buy/can_bid/can_chat)
- Fallback parsing (authorizer builds fallbacks, doesn't read them)

**Different port, different contract.** Read resolver is a new interface.

---

## 13. BATCH LOAD PLAN

### Algorithm

```
Input:  occurrences map[messageID]*ChatMessageResourceOccurrence
        viewerID uuid.UUID

1. Partition occurrences by resource type:
   profileIDs, contentIDs, fpsIDs, auctionIDs []uuid.UUID

2. Dedupe within each type (same resource shared multiple times in a page)

3. Batch-load current resource state (one query per present type):
   a. Profile batch:   SELECT id, account_status, deleted_at FROM users WHERE id = ANY($1)
                       + user_profiles + seller_profiles JOIN
   b. Content batch:   SELECT id, author_id, status, visibility, is_hidden, deleted_at,
                              caption, share_reference FROM contents WHERE id = ANY($1)
                       + first media URL per content
   c. FPS batch:       SELECT fps.id, fps.status, fps.price_per_unit, fps.seller_id,
                              fps.negotiation_enabled, fps.quantity_available,
                              p.title, p.media_urls
                       FROM fixed_price_sales fps JOIN products p ON p.id=fps.product_id
                       WHERE fps.id = ANY($1)
   d. Auction batch:   SELECT a.id, a.status, a.current_bid, a.buy_now_price,
                              a.end_at, a.seller_id, a.title, p.media_urls
                       FROM auctions a LEFT JOIN products p ON p.id=a.product_id
                       WHERE a.id = ANY($1)

4. Collect all distinct author/seller IDs across present types.
   Batch-load viewer relations:
   a. Block check:   blockcheck.BlockedSet(viewerID, allAuthorAndSellerIDs)
   b. Follow check:  SELECT following_id FROM user_follows
                     WHERE follower_id=$1 AND following_id = ANY($2)
                     (only if Content with visibility=followers_only is present)
   c. Account status: SELECT id, account_status, deleted_at FROM users
                      WHERE id = ANY($1)  (for author/seller lifecycle)

5. For each occurrence, resolve independently:
   a. Look up source row from batch results
   b. Apply viewer access rules (block → tombstone, deleted → tombstone, etc.)
   c. If LIVE: populate typed payload
   d. If FALLBACK_ALLOWED: parse + sanitize fallback
   e. If TOMBSTONE: identity-only

6. For each LIVE Content occurrence with ShareReference:
   a. Collect nested resource IDs by type
   b. Batch-check existence + accessibility (lightweight: just ID lookup + deleted_at)
   c. If accessible: emit NestedResourceIndicator
   d. If not: suppress

7. Return map[messageID]*ResourceProjection
```

### Batch query formulas

| Resource types present | Source queries | Viewer relation queries | Nested ref queries | Total (approx) |
|---|---|---|---|---|
| None (plain messages) | 0 | 0 | 0 | 0 |
| Profile only | 1 | 1 (blocks) | 0 | 2 |
| Content only | 1 | 2 (blocks + follows) | 0-1 | 3-4 |
| FPS only | 1 | 2 (blocks + seller lifecycle) | 0 | 3 |
| Auction only | 1 | 2 (blocks + seller lifecycle) | 0 | 3 |
| All 4 types | 4 | 2 (blocks + seller lifecycle, follows merged) | 0-1 | 6-7 |
| Content with depth-1 | +1 per nested type present | reuse existing | +1 per nested type | +1-2 |

**Bounded by resource type diversity (max ~7 additional queries), never by message count.**

---

## 14. PACKAGE / DEPENDENCY PLAN

```
chat/application/                          ← owns projection contract
  chat_resource_projection.go              ← ResourceProjection, ProjectionState,
                                              ResourceProjectionResolver interface,
                                              capability types, typed payloads
  chat_fallback_parsers.go                 ← typed fallback structs + ParseFallbackSnapshot
  chat_message_page_result.go             ← existing (3D-1)

serverboot/
  chat_projection_resolver.go             ← implements ResourceProjectionResolver
                                              wires domain repos/services for batch loading
  dependencies.go                          ← wires resolver into chatApp.Service

chat/delivery/http/
  chat_handler.go                          ← future: messageToResponse emits resource_projection
```

No new package needed. No circular dependencies.

---

## 15. ERROR MODEL

### A. Expected resource state (→ projection state)

| Condition | Result |
|---|---|
| Profile deleted/suspended/banned | TOMBSTONE |
| Profile blocked | TOMBSTONE |
| Content deleted/hidden/moderated | TOMBSTONE |
| Content private (viewer ≠ author) | TOMBSTONE |
| Content followers-only (viewer not following) | TOMBSTONE |
| Author/seller deleted/suspended | TOMBSTONE |
| FPS/Auction terminal lifecycle (sold/ended/cancelled) | LIVE with `can_interact=false` |
| FPS/Auction seller inactive (subscription expired) | LIVE with `can_interact=false`, seller lifecycle on card |

### B. Infrastructure failure (→ error or FALLBACK_ALLOWED)

| Failure | Behavior |
|---|---|
| DB pool error during batch query | Error propagates (fail-closed, per 3D-1 pattern) |
| Single resource query returns pgx.ErrNoRows | FPS/Auction: FALLBACK_ALLOWED (if seller still active). Profile/Content: TOMBSTONE |
| Viewer relation query fails | Error propagates (cannot safely determine block/follow state) |
| Nested resource lookup fails | Suppress nested_resource (graceful degradation) |

Default direction: **fail request rather than silently misproject.** Occurrence
batch-load errors already propagate per 3D-1. Source batch-load errors should
also propagate — returning 5 messages with 3 projections and 2 silently missing
is worse than returning a 500.

### C. Malformed historical fallback

If `fallback_snapshot` JSON is unparseable:
- Omit the projection for that message_id (nil entry in map)
- Log a warning
- Do NOT fail the entire page
- Do NOT return TOMBSTONE (asserts privacy decision that may not be true)

---

## 16. QUERY BUDGET

### Baseline (from 3D-1 measurements)

Current read path: **5 queries** for any message page.

Breakdown:
1. GetRoomByID (participant gate)
2. IsBidirectionallyBlocked (social gate, skipped for order/support rooms)
3. ListMessagesByRoom (messages)
4. GetResourceOccurrencesByMessageIDs (occurrence batch — 3D-1)
5. buildChatParticipantCardsWithLifecycle (sender cards)
6. hydrateAttachmentSellerLifecycles (seller lifecycle from attachment_json)

### Projected additional queries for 3D-2+ (per resource type mixture)

| Scenario | Added queries | Total (from baseline 5) |
|---|---|---|
| Normal messages only | 0 | 5 |
| Profile occurrences | +2 (profile batch + block set) | 7 |
| Content occurrences | +3 (content batch + block + follow) | 8 |
| FPS occurrences | +3 (FPS batch + block + seller lifecycle) | 8 |
| Auction occurrences | +3 (Auction batch + block + seller lifecycle) | 8 |
| All 4 types | +5-7 (4 source batches + merged viewer relations) | 10-12 |
| Content with depth-1 | +1-2 (nested resource existence check per type) | +1-2 on top of Content baseline |

**Invariant**: query count depends on `distinct_resource_types_present +
small_fixed_viewer_relation_batches`, never on `number_of_messages` or
`number_of_occurrences`.

### Viewer relation query merging

The same viewer is used for ALL occurrences on a page. So:
- Block check: ONE query collecting ALL distinct author/seller IDs → `BlockedSet`
- Follow check: ONE query for ALL (viewer, author) pairs on followers-only content
- Seller lifecycle: ONE query for ALL distinct seller IDs

These are NOT per-occurrence or per-resource-type.

---

## 17. HTTP CONTRACT EXAMPLES

### Future field name

`resource_projection` (singular, one per message).

**Why not `resource_occurrence`**: "Occurrence" is storage/internal terminology.
"Projection" is viewer-facing — the server has resolved the occurrence into a
viewer-specific projection with state, capabilities, and payload.

### LIVE Profile

```json
{
  "resource_projection": {
    "state": "LIVE",
    "resource_type": "profile",
    "resource_id": "11111111-1111-1111-1111-111111111111",
    "canonical_url": "/user/11111111-1111-1111-1111-111111111111",
    "capabilities": {
      "can_open": true,
      "can_chat": true,
      "can_report": true
    },
    "profile": {
      "username": "seller_name",
      "avatar_url": "https://cdn.example.com/avatar.jpg",
      "store_name": "Toko Maju Jaya",
      "is_seller": true,
      "lifecycle": "active"
    }
  }
}
```

### LIVE Content

```json
{
  "resource_projection": {
    "state": "LIVE",
    "resource_type": "content",
    "resource_id": "22222222-2222-2222-2222-222222222222",
    "canonical_url": "/content/22222222-2222-2222-2222-222222222222",
    "capabilities": {
      "can_open": true,
      "can_interact": true,
      "can_report": true
    },
    "content": {
      "caption": "Check out this amazing product!",
      "media": [{"url": "https://cdn.example.com/img1.jpg"}],
      "lifecycle": "active",
      "created_at": "2026-08-08T10:30:00Z",
      "author": {
        "id": "33333333-3333-3333-3333-333333333333",
        "username": "content_creator",
        "avatar_url": "https://cdn.example.com/author.jpg",
        "lifecycle": "active"
      },
      "nested_resource": {
        "resource_type": "fixed_price_sale",
        "resource_id": "44444444-4444-4444-4444-444444444444"
      }
    }
  }
}
```

### LIVE FPS

```json
{
  "resource_projection": {
    "state": "LIVE",
    "resource_type": "fixed_price_sale",
    "resource_id": "44444444-4444-4444-4444-444444444444",
    "canonical_url": "/listing/44444444-4444-4444-4444-444444444444",
    "capabilities": {
      "can_open": true,
      "can_interact": true,
      "can_chat": true,
      "can_negotiate": true,
      "can_buy": true,
      "can_report": true
    },
    "fixed_price_sale": {
      "id": "44444444-4444-4444-4444-444444444444",
      "title": "Premium Product",
      "thumbnail_url": "https://cdn.example.com/product.jpg",
      "price": 150000,
      "currency": "IDR",
      "lifecycle": "active",
      "quantity_available": 5,
      "seller": {
        "user": {"id": "...", "username": "seller_name", "lifecycle": "active"},
        "farm_name": "Toko Maju Jaya",
        "avatar_url": "https://cdn.example.com/avatar.jpg",
        "lifecycle": "active"
      }
    }
  }
}
```

### TOMBSTONE

```json
{
  "resource_projection": {
    "state": "TOMBSTONE",
    "resource_type": "content",
    "resource_id": "55555555-5555-5555-5555-555555555555",
    "canonical_url": "/content/55555555-5555-5555-5555-555555555555",
    "capabilities": {}
  }
}
```

### FALLBACK_ALLOWED (FPS transient DB error)

```json
{
  "resource_projection": {
    "state": "FALLBACK_ALLOWED",
    "resource_type": "fixed_price_sale",
    "resource_id": "44444444-4444-4444-4444-444444444444",
    "canonical_url": "/listing/44444444-4444-4444-4444-444444444444",
    "capabilities": {
      "can_open": true
    },
    "fixed_price_sale": {
      "title": "Premium Product",
      "image_url": "https://cdn.example.com/product.jpg",
      "seller_store_name": "Toko Maju Jaya"
    }
  }
}
```

### Message without occurrence

No `resource_projection` field at all. (Field absent, not null.)

---

## 18. SCHEMA RESULT

**Migration 000034 remains sufficient.** No new columns, tables, or indexes needed.

The existing columns provide everything the read projection requires:
- `message_id` — join key to chat_messages
- `operation` — share_to_chat vs direct_commerce_insert_chat
- `profile_source_id` / `content_source_id` / `fixed_price_sale_source_id` / `auction_source_id` — typed source FKs for batch lookups
- `fallback_snapshot` — server-built historical display data
- `created_at` — occurrence timestamp

The read projection derives LIVE state dynamically from canonical domain tables.
No tombstone/status/cache columns needed.

---

## 19. IMPLEMENTATION SLICES

Recommended sequential slices after 3D-2 design approval:

| Slice | Name | Scope | Depends On |
|---|---|---|---|
| **3D-2a** | Projection Types + Envelope | Define `ResourceProjection`, `ProjectionState`, typed payloads, `ResourceCapabilities`, custom `MarshalJSON`. Pure data types — no DB. | 3D-1 |
| **3D-2b** | Fallback Parsers | Define typed fallback structs + `ParseFallbackSnapshot`. Pure parsing — no DB. | 3D-2a |
| **3D-2c** | Resolver Interface + No-op Wiring | Define `ResourceProjectionResolver` interface. Wire a no-op implementation in serverboot. Pass through MessagePageResult. | 3D-2a |
| **3D-3** | Profile LIVE + Tombstone | Implement Profile batch-load + viewer access in resolver. Real PostgreSQL tests. | 3D-2c |
| **3D-4** | Content LIVE + Tombstone + Depth-1 | Implement Content batch-load + visibility + ShareReference nested indicator. | 3D-2c, 3D-3 (block batching) |
| **3D-5** | FPS LIVE + Capabilities | Implement FPS batch-load + lifecycle + capabilities. | 3D-2c |
| **3D-6** | Auction LIVE + Capabilities | Implement Auction batch-load + lifecycle + capabilities. | 3D-2c |
| **3D-7** | FALLBACK_ALLOWED Gate | Wire fallback path for transient errors. | 3D-3, 3D-4, 3D-5, 3D-6 |
| **3D-8** | HTTP Response Wiring | Add `resource_projection` field to `messageToResponse`. | 3D-7 |
| **3D-9** | Legacy Hard Purge | Remove `type=reference` from attachment validation. | 3D-8 + mobile switch |

### Why 3D-2a through 3D-2c are separate from 3D-3+

The type definitions, fallback parsers, and resolver interface are pure code with
zero DB dependencies. They can be built, tested, and reviewed independently. The
resource-specific slices (3D-3 through 3D-6) each add one domain's worth of batch
queries and viewer logic.

---

## 20. BLOCKERS / OWNER DECISIONS

### No genuine blockers

All design decisions are locked by the scope document or resolved by existing
codebase patterns:

| Decision | Resolution |
|---|---|
| Payload types | Extend `publiccard` types with Chat-specific fields |
| Capability names | Use existing canonical checks; no new names invented |
| Fallback exposure | Typed parsers; malformed → omit projection |
| Tombstone shape | Identity-only, no reason string |
| Resolver package | Interface in `chat/application`, impl in `serverboot` |
| Schema change | None required; 000034 sufficient |
| URL scheme | Path-only (`/user/{id}`, `/content/{id}`, `/listing/{id}`, `/auction/{id}`) matching existing backend URL patterns |
| Depth-1 indicator | Identity-only; no preview/title/image |

### The one owner decision

**Should `FALLBACK_ALLOWED` be implemented at all, or should the entire read path
be LIVE-or-TOMBSTONE only?**

Arguments for omitting FALLBACK_ALLOWED:
- Simpler state machine (2 states instead of 3)
- Fewer code paths to test
- Privacy-safe by construction (no historical data leakage risk)
- The fallback data is already stale by definition (written at share time)

Arguments for keeping FALLBACK_ALLOWED:
- Graceful degradation when a listing is temporarily unavailable (DB replica lag)
- Better UX than showing "Content unavailable" for a resource that still exists
- The fallback data is already paid for (stored in DB, built at write time)

**Recommendation**: Start with LIVE-or-TOMBSTONE only (omit FALLBACK_ALLOWED).
If production metrics show significant projection failure rates from transient
errors, add FALLBACK_ALLOWED in a later slice. This keeps 3D-2 through 3D-8
simpler and safer.

---

## 21. RECOMMENDATION

**Start with Slice 3D-2a (Projection Types + Envelope).**

This is ~150 lines of pure Go with zero DB dependencies:
- `ResourceProjection` struct + custom `MarshalJSON`
- `ProjectionState` constants
- `ResourceCapabilities` struct
- Four typed payload structs
- Unit tests for JSON marshaling of all state/resource type combinations

Rationale: The typed envelope is the contract that all subsequent slices build
against. It must be right before any DB work begins.

---

**STOP.** Design complete. No implementation performed.
