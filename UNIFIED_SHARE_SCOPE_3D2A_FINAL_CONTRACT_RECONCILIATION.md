# UNIFIED SHARE SCOPE 3D2A — FINAL CONTRACT RECONCILIATION

**STATUS:** `UNIFIED_SHARE_SCOPE_3D2_TYPED_PROJECTION_CONTRACT_AND_READ_RESOLVER_DESIGN_COMPLETE`

**DATE:** 2026-08-08

**MODE:** DESIGN CORRECTION — NO IMPLEMENTATION

**SUPERSEDES:** `UNIFIED_SHARE_SCOPE_3D2_TYPED_PROJECTION_CONTRACT_AND_READ_RESOLVER_DESIGN.md`

---

## 1. VERDICT

`UNIFIED_SHARE_SCOPE_3D2_TYPED_PROJECTION_CONTRACT_AND_READ_RESOLVER_DESIGN_COMPLETE`

All inconsistencies reconciled against actual filesystem evidence.

---

## 2. FINAL PROJECTION GO TYPES

```go
// FILE: backend/internal/interaction/chat/application/chat_resource_projection.go (NEW)
package application

type ProjectionState string

const (
    ProjectionStateLive            ProjectionState = "LIVE"
    ProjectionStateFallbackAllowed ProjectionState = "FALLBACK_ALLOWED"
    ProjectionStateTombstone       ProjectionState = "TOMBSTONE"
)

// ResourceProjection is the canonical typed viewer-facing resource projection
// for a Chat message occurrence. Exactly one typed payload is populated based
// on ResourceType and State.
type ResourceProjection struct {
    State        ProjectionState      `json:"state"`
    ResourceType string               `json:"resource_type"`
    ResourceID   string               `json:"resource_id"`
    CanonicalURL string               `json:"canonical_url"`
    Capabilities ResourceCapabilities `json:"capabilities"`

    // Exactly ONE non-nil, discriminated at MarshalJSON time.
    Profile        *ProfileProjectionPayload        `json:"-"`
    Content        *ContentProjectionPayload        `json:"-"`
    FixedPriceSale *FixedPriceSaleProjectionPayload `json:"-"`
    Auction        *AuctionProjectionPayload        `json:"-"`
}

// ResourceCapabilities is the viewer-scoped capability projection.
// Mirrors commerce/shared.ViewerCapabilities where applicable; omits fields
// irrelevant in Chat context (CanEdit, CanPromote, CanBuyNow).
// All fields are deterministic Boolean — no "unknown" state.
type ResourceCapabilities struct {
    Role         string `json:"role,omitempty"`   // "owner" | "buyer" | "" (guest/other)
    CanChat      bool   `json:"can_chat"`
    CanNegotiate bool   `json:"can_negotiate"`    // FPS only
    CanBuy       bool   `json:"can_buy"`          // FPS only
    CanBid       bool   `json:"can_bid"`          // Auction only
    CanManage    bool   `json:"can_manage"`       // owner || admin
}
```

### Typed payloads

```go
// ProfileProjectionPayload — LIVE payload for profile occurrences.
type ProfileProjectionPayload struct {
    Username  string  `json:"username"`
    AvatarURL *string `json:"avatar_url,omitempty"`
    StoreName *string `json:"store_name,omitempty"`
    IsSeller  bool    `json:"is_seller"`
    Lifecycle string  `json:"lifecycle"` // "active" | "unavailable" | "removed"
}

// ContentProjectionPayload — LIVE payload for content occurrences.
type ContentProjectionPayload struct {
    Caption        *string                  `json:"caption,omitempty"`
    Media          []MediaRef               `json:"media"`
    Lifecycle      string                   `json:"lifecycle"`
    CreatedAt      string                   `json:"created_at"`
    Author         ContentAuthorIdentity    `json:"author"`
    NestedResource *NestedResourceIndicator `json:"nested_resource,omitempty"`
}

type ContentAuthorIdentity struct {
    ID        uuid.UUID `json:"id"`
    Username  string    `json:"username"`
    AvatarURL *string   `json:"avatar_url,omitempty"`
    Lifecycle string    `json:"lifecycle"`
}

type MediaRef struct {
    URL string `json:"url"`
}

// NestedResourceIndicator — depth-1 only. Identity only, no preview/title.
type NestedResourceIndicator struct {
    ResourceType string `json:"resource_type"`
    ResourceID   string `json:"resource_id"`
}

// FixedPriceSaleProjectionPayload — LIVE payload for FPS occurrences.
type FixedPriceSaleProjectionPayload struct {
    ID                uuid.UUID `json:"id"`
    Title             string    `json:"title"`
    ThumbnailURL      *string   `json:"thumbnail_url,omitempty"`
    Price             int64     `json:"price"`
    Lifecycle         string    `json:"lifecycle"`  // "active" | "unavailable"
    QuantityAvailable *int64    `json:"quantity_available,omitempty"`
    Seller            SellerIdentity `json:"seller"`
}

// AuctionProjectionPayload — LIVE payload for auction occurrences.
type AuctionProjectionPayload struct {
    ID           uuid.UUID      `json:"id"`
    Title        string         `json:"title"`
    ThumbnailURL *string        `json:"thumbnail_url,omitempty"`
    CurrentBid   *int64         `json:"current_bid,omitempty"`
    BuyNowPrice  *int64         `json:"buy_now_price,omitempty"`
    EndAt        string         `json:"end_at"`
    Lifecycle    string         `json:"lifecycle"`
    Seller       SellerIdentity `json:"seller"`
}

// SellerIdentity — compact seller block for Chat commerce projections.
type SellerIdentity struct {
    ID        uuid.UUID `json:"id"`
    Username  string    `json:"username"`
    FarmName  *string   `json:"farm_name,omitempty"`
    AvatarURL *string   `json:"avatar_url,omitempty"`
    Lifecycle string    `json:"lifecycle"`       // user-identity axis
    TrustAxis *string   `json:"trust_axis,omitempty"` // seller-trust axis; nil when active
}
```

### Fallback typed payloads (FALLBACK_ALLOWED only)

```go
// ProfileFallbackDisplay — never emitted (Profile never uses FALLBACK_ALLOWED).
// Defined for completeness; always results in TOMBSTONE.

// ContentFallbackDisplay — never emitted (Content never uses FALLBACK_ALLOWED).

type FixedPriceSaleFallbackDisplay struct {
    Title           string  `json:"title"`
    ImageURL        *string `json:"image_url,omitempty"`
    SellerStoreName string  `json:"seller_store_name"`
}

type AuctionFallbackDisplay struct {
    Title           string  `json:"title"`
    ImageURL        *string `json:"image_url,omitempty"`
    SellerStoreName string  `json:"seller_store_name"`
}
```

---

## 3. FINAL JSON ENVELOPE

### LIVE Profile

```json
{
  "state": "LIVE",
  "resource_type": "profile",
  "resource_id": "11111111-1111-1111-1111-111111111111",
  "canonical_url": "/user/11111111-1111-1111-1111-111111111111",
  "capabilities": {
    "role": "",
    "can_chat": true,
    "can_negotiate": false,
    "can_buy": false,
    "can_bid": false,
    "can_manage": false
  },
  "profile": {
    "username": "seller_name",
    "avatar_url": "https://cdn.example.com/avatar.jpg",
    "store_name": "Toko Maju Jaya",
    "is_seller": true,
    "lifecycle": "active"
  }
}
```

### LIVE Content (with nested FPS)

```json
{
  "state": "LIVE",
  "resource_type": "content",
  "resource_id": "22222222-2222-2222-2222-222222222222",
  "canonical_url": "/content/22222222-2222-2222-2222-222222222222",
  "capabilities": {
    "role": "",
    "can_chat": false,
    "can_negotiate": false,
    "can_buy": false,
    "can_bid": false,
    "can_manage": false
  },
  "content": {
    "caption": "Check this out",
    "media": [{"url": "https://cdn.example.com/img1.jpg"}],
    "lifecycle": "active",
    "created_at": "2026-08-08T10:30:00Z",
    "author": {
      "id": "33333333-3333-3333-3333-333333333333",
      "username": "author_name",
      "avatar_url": null,
      "lifecycle": "active"
    },
    "nested_resource": {
      "resource_type": "fixed_price_sale",
      "resource_id": "44444444-4444-4444-4444-444444444444"
    }
  }
}
```

### LIVE FPS

```json
{
  "state": "LIVE",
  "resource_type": "fixed_price_sale",
  "resource_id": "44444444-4444-4444-4444-444444444444",
  "canonical_url": "/listing/44444444-4444-4444-4444-444444444444",
  "capabilities": {
    "role": "buyer",
    "can_chat": true,
    "can_negotiate": true,
    "can_buy": true,
    "can_bid": false,
    "can_manage": false
  },
  "fixed_price_sale": {
    "id": "44444444-4444-4444-4444-444444444444",
    "title": "Premium Product",
    "thumbnail_url": "https://cdn.example.com/product.jpg",
    "price": 150000,
    "lifecycle": "active",
    "quantity_available": 5,
    "seller": {
      "id": "11111111-1111-1111-1111-111111111111",
      "username": "seller_name",
      "farm_name": "Toko Maju Jaya",
      "avatar_url": "https://cdn.example.com/avatar.jpg",
      "lifecycle": "active",
      "trust_axis": null
    }
  }
}
```

### LIVE Auction

```json
{
  "state": "LIVE",
  "resource_type": "auction",
  "resource_id": "55555555-5555-5555-5555-555555555555",
  "canonical_url": "/auction/55555555-5555-5555-5555-555555555555",
  "capabilities": {
    "role": "buyer",
    "can_chat": true,
    "can_negotiate": false,
    "can_buy": false,
    "can_bid": true,
    "can_manage": false
  },
  "auction": {
    "id": "55555555-5555-5555-5555-555555555555",
    "title": "Rare Collectible",
    "thumbnail_url": "https://cdn.example.com/auction.jpg",
    "current_bid": 75000,
    "buy_now_price": 120000,
    "end_at": "2026-08-15T18:00:00Z",
    "lifecycle": "active",
    "seller": {
      "id": "11111111-1111-1111-1111-111111111111",
      "username": "seller_name",
      "farm_name": "Toko Maju Jaya",
      "avatar_url": "https://cdn.example.com/avatar.jpg",
      "lifecycle": "active",
      "trust_axis": null
    }
  }
}
```

### FALLBACK_ALLOWED (FPS — transient DB error, seller confirmed active)

```json
{
  "state": "FALLBACK_ALLOWED",
  "resource_type": "fixed_price_sale",
  "resource_id": "44444444-4444-4444-4444-444444444444",
  "canonical_url": "/listing/44444444-4444-4444-4444-444444444444",
  "capabilities": {
    "role": "",
    "can_chat": false,
    "can_negotiate": false,
    "can_buy": false,
    "can_bid": false,
    "can_manage": false
  },
  "fixed_price_sale": {
    "title": "Premium Product",
    "image_url": "https://cdn.example.com/product.jpg",
    "seller_store_name": "Toko Maju Jaya"
  }
}
```

### TOMBSTONE

```json
{
  "state": "TOMBSTONE",
  "resource_type": "content",
  "resource_id": "66666666-6666-6666-6666-666666666666",
  "canonical_url": "/content/66666666-6666-6666-6666-666666666666",
  "capabilities": {
    "role": "",
    "can_chat": false,
    "can_negotiate": false,
    "can_buy": false,
    "can_bid": false,
    "can_manage": false
  }
}
```

---

## 4. EXACT CAPABILITY CONTRACT

### Go type (final)

```go
type ResourceCapabilities struct {
    Role         string `json:"role,omitempty"`
    CanChat      bool   `json:"can_chat"`
    CanNegotiate bool   `json:"can_negotiate"`
    CanBuy       bool   `json:"can_buy"`
    CanBid       bool   `json:"can_bid"`
    CanManage    bool   `json:"can_manage"`
}
```

### JSON (always present, never omitted)

All six fields always serialized. Boolean fields are `false` when not applicable.

### Canonical source for each field

| Field | Canonical authority | Proof |
|---|---|---|
| `Role` | Viewer identity compared to resource owner/author/seller | `commerce/shared.ViewerCapabilities.Role` — identical logic in `buildListingViewerCapabilities` and `buildAuctionViewerCapabilities` |
| `CanChat` | `!blocked && viewerLifecycle=="active" && (Profile: true; Content: N/A; FPS/Auction: sellerTrustActive)` | `commerce/shared.ViewerCapabilities.CanChat` — identical logic |
| `CanNegotiate` | `CanChat && fps.NegotiationEnabled && fps.IsAvailable()` | `commerce/shared.ViewerCapabilities.CanNegotiate` |
| `CanBuy` | `CanChat && fps.IsAvailable()` | `commerce/shared.ViewerCapabilities.CanBuy` |
| `CanBid` | `CanChat && auction.Status == active` | `commerce/shared.ViewerCapabilities.CanBid` |
| `CanManage` | `viewer == owner \|\| isAdmin` | `commerce/shared.ViewerCapabilities.CanManage` |

### Fields deliberately excluded

| Field | Reason |
|---|---|
| `CanEdit` | Irrelevant in Chat — edit is a detail-surface action, not a chat-card action |
| `CanPromote` | Irrelevant in Chat — promotion is a seller-dashboard action |
| `CanBuyNow` | Redundant with `CanBid` for compact Chat display; Auction's buy-now is indicated by `buy_now_price != null` in payload |
| `can_open` | Does not exist in any canonical authority. Resource is "openable" iff projection is LIVE — no separate Boolean needed |
| `can_interact` | Does not exist in `commerce/shared.ViewerCapabilities`. The comment-system `CommerceViewerCapabilities.CanInteract` is dead placeholder code (never constructed) |
| `can_report` | Does not exist anywhere in the backend. Report is gated by a separate endpoint, not a pre-computed capability flag |

---

## 5. STATE SEMANTICS

### LIVE

> Current canonical resource authority is available and the viewer is permitted to
> see it.

**Eligibility**: ALL of:
1. Resource row exists in canonical domain table
2. Resource is not deleted/hidden/moderated
3. Viewer passes canonical access rules (visibility, privacy, block)
4. Resource owner/author/seller passes lifecycle check (not deleted/banned)

### FALLBACK_ALLOWED

> Live projection genuinely cannot be produced, the failure is NOT a
> privacy/moderation/block decision, AND historical display remains safe.

**Eligibility**: ALL of:
1. Live source query succeeded but returned no row (resource deleted between
   occurrence write and read) OR live source batch query failed with a
   **localized** error for this specific resource (not a whole-batch failure)
2. The resource type permits historical display: FPS and Auction only.
   Profile and Content NEVER use FALLBACK_ALLOWED
3. A lightweight seller-liveness check **positively confirms** the seller is
   still active (not deleted/banned). If seller liveness cannot be determined,
   fail to TOMBSTONE
4. `fallback_snapshot` parses successfully into the typed fallback struct

**Not eligible** (→ TOMBSTONE or error):
- Viewer is blocked by the resource owner → TOMBSTONE
- Resource is deleted/hidden/moderated by policy → TOMBSTONE
- Privacy/visibility rule denies access → TOMBSTONE
- DB pool failure / batch infrastructure error → error propagates
- Seller liveness cannot be determined → TOMBSTONE
- Fallback JSON is malformed → error (data integrity failure)

### TOMBSTONE

> Neither live data nor historical fallback may be exposed.

**Triggers**:
- Resource deleted/hidden/moderated
- Resource owner/author/seller suspended/banned/deleted
- Viewer blocked by resource owner
- Private/restricted visibility with viewer ≠ owner
- FALLBACK_ALLOWED eligibility check fails at any positive gate

---

## 6. FALLBACK_ALLOWED ELIGIBILITY RULES

### Per-type eligibility

| Type | FALLBACK_ALLOWED? | Rationale |
|---|---|---|
| Profile | **Never** | Privacy always wins. Historical username/avatar leakage from a deleted/suspended account is prohibited |
| Content | **Never** | Deleted/hidden/moderated content must not leak historical caption/media. A transient DB error on a public content from an active author could theoretically be safe, but the risk of serving stale data for content that has since been moderated/hidden outweighs the UX benefit |
| FPS | **Yes**, when: live FPS query fails for this specific resource AND seller liveness is positively confirmed AND fallback parses | `fallback_snapshot` only contains title + image + store_name — no price/status leakage. Acceptable degradation |
| Auction | **Yes**, same conditions as FPS | Same rationale — fallback shape is safe |

### NOT eligible (common errors that must NOT trigger FALLBACK_ALLOWED)

| Condition | Result |
|---|---|
| Whole-batch DB pool error | Error propagates (fail request) |
| Viewer relation query error (block/follow) | Error propagates |
| Seller liveness check error | TOMBSTONE (cannot determine safety) |
| Fallback JSON unparseable | Error propagates (data integrity) |
| Viewer blocked by seller | TOMBSTONE |

---

## 7. MALFORMED FALLBACK BEHAVIOR

### When LIVE succeeds
Do not parse fallback. Fallback is irrelevant.

### When FALLBACK_ALLOWED is selected and fallback parsing fails
**Error propagates.** A persisted occurrence must not silently lose its projection
because of corrupt JSON. The `fallback_snapshot` column is `NOT NULL` and built by
server-side code — corruption indicates a storage/integrity problem that should
surface, not be silently swallowed.

### When fallback parses but contains unexpected/missing fields
Typed parsers use `json.Unmarshal` into structs with zero-value defaults. Missing
optional fields (e.g., `image_url`) → nil/zero. Unexpected extra fields →
ignored by Go's JSON decoder. This is safe.

---

## 8. PROFILE STATE / ACCESS MATRIX

| Condition | State | Capabilities |
|---|---|---|
| Profile active, viewer not blocked | LIVE | `can_chat`=true (if viewer active) |
| Profile active, viewer IS blocked | TOMBSTONE | all false |
| Profile suspended | TOMBSTONE | all false |
| Profile banned | TOMBSTONE | all false |
| Profile deleted | TOMBSTONE | all false |
| Viewer == profile (self) | LIVE | `can_chat`=false, `can_manage`=true |
| Block relation query fails | **Error** | — |

**LIVE payload**: `username`, `avatar_url`, `store_name` (if seller), `is_seller`, `lifecycle`.

**Canonical sources**:
- `username`/`avatar_url`: `user_profiles` via `userdisplay.FetchMany` (only reads username, avatar_url — never email/phone/firebase_uid)
- `store_name`/`is_seller`: `seller_profiles` WHERE `status='active'`
- `lifecycle`: `viewercontext.CoarsenLifecycle(users.account_status, users.deleted_at)`
- Block: `blockcheck.BlockedSet` (bidirectional, batched, fail-closed)

---

## 9. CONTENT STATE / ACCESS MATRIX

| Condition | State | Capabilities |
|---|---|---|
| Public, active author, not hidden/deleted, viewer not blocked | LIVE | `can_chat`=false, `can_manage`=false |
| Public, viewer == author | LIVE | `can_manage`=true |
| Followers-only, viewer follows, not blocked | LIVE | same as public |
| Followers-only, viewer doesn't follow | TOMBSTONE | all false |
| Private, viewer == author | LIVE | `can_manage`=true |
| Private, viewer ≠ author | TOMBSTONE | all false |
| Content deleted (`deleted_at IS NOT NULL`) | TOMBSTONE | all false |
| Content hidden (`is_hidden = true`) | TOMBSTONE | all false |
| Content status = `deleted` | TOMBSTONE | all false |
| Author suspended/banned/deleted | TOMBSTONE | all false |
| Viewer blocked by author | TOMBSTONE | all false |
| Follow or block relation query fails | **Error** | — |

**LIVE payload**: `caption`, `media` (from `content_media` ORDER BY `position`), `lifecycle` (via `Status.PublicLifecycle()`), `created_at`, `author` block.

**Canonical sources**:
- `caption`: `contents.caption`
- `media`: `content_media.media_url` WHERE `content_id = $1` ORDER BY `position`
- `lifecycle`: `contents.Status.PublicLifecycle()` → `"active"` / `"removed"`
- `created_at`: `contents.created_at`
- `author.id`: `contents.author_id`
- `author.username`/`avatar_url`: `user_profiles`
- `author.lifecycle`: `CoarsenLifecycle(author.account_status, author.deleted_at)`
- `share_reference`: `contents.share_reference` JSONB column (for depth-1)

---

## 10. CONTENT DEPTH-1 SHAPE

### NestedResourceIndicator

```go
type NestedResourceIndicator struct {
    ResourceType string `json:"resource_type"`
    ResourceID   string `json:"resource_id"`
}
```

### Resolution

1. Parse `contents.share_reference` JSONB → `ShareReference{TargetType, TargetID, Preview}`
2. Map `TargetType`: `content` → `content`, `fixed_price_sale` → `fixed_price_sale`, `auction` → `auction`, `profile` → `profile`
3. Apply canonical access rules for the nested type (block, lifecycle, visibility) using the SAME batched viewer relations already loaded for primary projections
4. If accessible → emit `NestedResourceIndicator` with identity only
5. If inaccessible → suppress `nested_resource` field entirely

### What the indicator is sufficient to render

The `resource_type` + `resource_id` pair is sufficient for mobile to:
- Navigate to the nested resource detail (via `canonical_url` derivation)
- Display a resource-type icon
- Look up cached display data if available

No title, image, or preview crosses the Chat boundary for nested resources.

### Access check reuse

If the primary page already contains occurrences of the same type as the nested
resource, the batched source data is reused. No additional query.

If the nested type is NOT present among primary occurrences, one additional
lightweight batch query checks existence + deleted_at for the nested IDs.

---

## 11. FPS STATE / ACCESS / CAPABILITY MATRIX

| FPS Status | Seller State | Viewer Block | State | `can_chat` | `can_negotiate` | `can_buy` | `can_manage` |
|---|---|---|---|---|---|---|---|
| `active` | active | no | LIVE | true | if negotiable | true | viewer==seller |
| `active` | active | **yes** | TOMBSTONE | false | false | false | false |
| `active` | inactive (sub expired) | no | LIVE | false | false | false | viewer==seller |
| `active` | suspended/banned/deleted | no | TOMBSTONE | false | false | false | false |
| `sold` | active | no | LIVE | true | false | false | viewer==seller |
| `withdrawn` | active | no | LIVE | true | false | false | viewer==seller |
| `draft` | — | no (owner only) | LIVE (owner) | false | false | false | true |
| `draft` | — | no (non-owner) | TOMBSTONE | false | false | false | false |
| Visibility=`private`, non-owner | — | — | TOMBSTONE | false | false | false | false |
| Block query fails | — | — | **Error** | — | — | — | — |

**Canonical sources**:
- `title`: `products.title` via `fixed_price_sales.product_id` JOIN
- `thumbnail_url`: `products.media_urls->>0`
- `price`: `fixed_price_sales.price_per_unit`
- `lifecycle`: `FixedPriceSaleStatus.PublicLifecycle()` → `"active"` / `"unavailable"`
- `quantity_available`: `fixed_price_sales.quantity_available` (may be NULL)
- `seller.id`: `fixed_price_sales.seller_id`
- `seller.username`/`farm_name`/`avatar_url`: `user_profiles` + `seller_profiles`
- `seller.lifecycle`: `CoarsenLifecycle(account_status, deleted_at)`
- `seller.trust_axis`: `CoarsenSellerTrust(subscription_status)` — nil when `"active"`, `"unavailable"` when expired/suspended

---

## 12. AUCTION STATE / ACCESS / CAPABILITY MATRIX

| Auction Status | Seller State | Viewer Block | State | `can_chat` | `can_bid` | `can_manage` |
|---|---|---|---|---|---|---|
| `active` | active | no | LIVE | true | true | viewer==seller |
| `active` | active | **yes** | TOMBSTONE | false | false | false |
| `active` | inactive (sub expired) | no | LIVE | false | false | viewer==seller |
| `active` | suspended/banned/deleted | no | TOMBSTONE | false | false | false |
| `scheduled` | active | no | LIVE | true | false | viewer==seller |
| `waiting_settlement` | active | no | LIVE | true | false | viewer==seller |
| `ended` | active | no | LIVE | true | false | viewer==seller |
| `cancelled` | active | no | LIVE | true | false | viewer==seller |
| `expired_bnr` | active | no | LIVE | true | false | viewer==seller |
| `draft` | — | no (owner only) | LIVE (owner) | false | false | true |
| `draft` | — | no (non-owner) | TOMBSTONE | false | false | false |
| Block query fails | — | — | **Error** | — | — | — |

**Canonical sources**:
- `title`: `auctions.title` (auctions table has its own `title` column — migration 000001 line 476)
- `thumbnail_url`: `products.media_urls->>0` via `auctions.product_id` JOIN
- `current_bid`: `auctions.current_bid` (NULL when no bids)
- `buy_now_price`: `auctions.buy_now_price` (NULL when no buy-now option)
- `end_at`: `auctions.end_at` (RFC3339)
- `lifecycle`: `Status.PublicLifecycle()` → `"active"` (active, waiting_settlement) / `"unavailable"` (all others)
- `seller.*`: same as FPS

**Note on `auctions.title` vs `products.title`**: The `auctions` table has its own
`title text NOT NULL` column (000001 line 476). The Scope 3C fallback builder
uses `p.title` from products, but the LIVE projection should use `auctions.title`
(the canonical auction title). For FALLBACK_ALLOWED, the fallback contains
`p.title` from the products join — that is the historical data, used as-is.

---

## 13. TOMBSTONE IDENTITY / REDACTION CONTRACT

### Fields preserved in TOMBSTONE

| Field | Preserved? | Justification |
|---|---|---|
| `state` | Yes — `"TOMBSTONE"` | Required discriminator |
| `resource_type` | Yes | Enables mobile to render correct placeholder icon ("Content unavailable" vs "Listing unavailable") without leaking which specific content/listing |
| `resource_id` | **No** — removed | The resource ID enables navigation to the resource detail. If the resource is tombstoned (deleted/blocked/private), the viewer must not navigate to it. Removing the ID prevents accidental deeplink construction |
| `canonical_url` | **No** — removed | Same rationale as `resource_id`. A tombstoned resource has no valid URL for this viewer |
| `capabilities` | Yes — all false | Deterministic zero-value; no "unknown" state |

### Revised TOMBSTONE JSON

```json
{
  "state": "TOMBSTONE",
  "resource_type": "content",
  "capabilities": {
    "role": "",
    "can_chat": false,
    "can_negotiate": false,
    "can_buy": false,
    "can_bid": false,
    "can_manage": false
  }
}
```

`resource_id` and `canonical_url` are **absent** for TOMBSTONE. This is the
privacy-safe minimum: the viewer knows a resource of type X was shared but cannot
identify or navigate to it.

### Message-level vs resource-level tombstone independence

| Message State | Resource State | HTTP Response |
|---|---|---|
| Normal | LIVE | `message` + `resource_projection` {state: LIVE, ...} |
| Normal | TOMBSTONE | `message` + `resource_projection` {state: TOMBSTONE, resource_type only} |
| Normal | FALLBACK_ALLOWED | `message` + `resource_projection` {state: FALLBACK_ALLOWED, ...} |
| Soft-deleted | Any | `message` {is_hidden: true}, **no `resource_projection` field** |

The message tombstone (soft-delete) suppresses the entire `resource_projection`
field, just as it suppresses `body`, `attachment_json`, and `sender` today.

---

## 14. RESOLVER INTERFACE

```go
// FILE: backend/internal/interaction/chat/application/chat_resource_projection.go (NEW)

// ResourceProjectionResolver resolves occurrence rows into viewer-aware
// ResourceProjections. Batch-oriented: all occurrences for a message page
// are resolved together in one call.
//
// Owned by Chat application layer. Implemented in serverboot to wire
// domain services without package cycles.
type ResourceProjectionResolver interface {
    // ResolveResourceProjections resolves all occurrences into viewer-specific
    // projections. Returns a map keyed by message_id.
    //
    // Every occurrence in the input MUST produce an entry in the output map
    // (LIVE, FALLBACK_ALLOWED, or TOMBSTONE). A nil entry is not permitted —
    // a persisted occurrence must always produce a deterministic projection.
    //
    // Errors:
    //   - Whole-batch infrastructure failure (DB pool, batch query) → error
    //   - Individual resource not found + FALLBACK_ALLOWED ineligible → TOMBSTONE
    //   - Malformed fallback for a FALLBACK_ALLOWED-eligible resource → error
    //   - Viewer relation query failure → error
    ResolveResourceProjections(
        ctx context.Context,
        viewerID uuid.UUID,
        occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
    ) (map[uuid.UUID]*ResourceProjection, error)
}
```

### Package placement

```
chat/application/
  chat_resource_projection.go   ← ResourceProjection, ResourceProjectionResolver, typed payloads

serverboot/
  chat_projection_resolver.go   ← struct implementing ResourceProjectionResolver
  dependencies.go               ← wires resolver into chatApp.Service
```

No cycle. `serverboot` already imports `chat/application` + all domain packages.

### No production no-op

There is no "no-op resolver." The resolver interface is defined in 3D-2a (types
only). The first implementation is 3D-3 (Profile LIVE + TOMBSTONE). Until the
resolver is wired, `MessagePageResult.ResourceOccurrencesByMessageID` is available
but unused in the HTTP response (3D-1 behavior).

---

## 15. BATCH / QUERY PLAN

### Algorithm

```
Input:  occurrences map[messageID]*Occurrence, viewerID

1. Partition by type → deduped []uuid.UUID per type
2. Batch-load source rows (one query per present type):
   a. Profile:  users + user_profiles + seller_profiles WHERE id = ANY($1)
   b. Content:  contents + content_media (first by position) WHERE id = ANY($1)
   c. FPS:      fixed_price_sales + products WHERE fps.id = ANY($1)
   d. Auction:  auctions + products WHERE a.id = ANY($1)

3. Collect all distinct author/seller IDs across all types.
   One BlockedSet query: blockcheck.BlockedSet(viewerID, allAuthorSellerIDs)
   → fail-closed: if query errors, return error

4. If Content present:
   One follow query: SELECT following_id FROM user_follows
     WHERE follower_id = $1 AND following_id = ANY($2)
   → only needed if any Content has visibility = followers_only
   → fail-closed

5. One seller lifecycle batch:
   SELECT id, account_status, deleted_at FROM users WHERE id = ANY($1)
   + seller_profiles + latest seller_subscriptions
   → for all seller IDs across FPS/Auction/Content(ShareReference→FPS/Auction)

6. Per-occurrence resolution (pure in-memory, no DB):
   a. Apply block → TOMBSTONE
   b. Apply lifecycle (deleted/banned/suspended) → TOMBSTONE
   c. Apply visibility/privacy → TOMBSTONE
   d. Apply status (terminal-but-viewable → LIVE non-actionable)
   e. Populate capabilities
   f. Build typed payload

7. For each LIVE Content with ShareReference:
   a. Map target type → resource type
   b. Check accessibility via already-loaded data if same type present
      OR one batch query for missing types
   c. If accessible → emit NestedResourceIndicator
   d. If not → suppress

8. Return map[messageID]*ResourceProjection (every input messageID has an entry)
```

### Query budget (additional on top of 3D-1 baseline of 5)

| Scenario | Source batches | Relation batches | Total added | Notes |
|---|---|---|---|---|
| Normal only | 0 | 0 | 0 | |
| Profile only | 1 | 1 (BlockedSet) | 2 | |
| Content only | 1 | 2 (BlockedSet + follow) | 3 | Follow only if followers_only present |
| FPS only | 1 | 2 (BlockedSet + seller lifecycle) | 3 | |
| Auction only | 1 | 2 (BlockedSet + seller lifecycle) | 3 | |
| All 4 types | 4 | 2 (BlockedSet + seller lifecycle; follow merged if needed) | 6-7 | |
| Content + depth-1 (new type) | +1 per new nested type | reuse existing | +0-1 | Only if nested type not in primary set |

---

## 16. ERROR MODEL

### Category A: Expected resource state → projection state

| Condition | Outcome |
|---|---|
| Resource deleted/hidden/moderated | TOMBSTONE |
| Viewer blocked | TOMBSTONE |
| Private/restricted visibility, viewer != owner | TOMBSTONE |
| Owner lifecycle failed (suspended/banned/deleted) | TOMBSTONE |
| FPS/Auction terminal lifecycle (sold/ended/cancelled) | LIVE, `can_buy`/`can_bid`=false |
| Seller subscription expired | LIVE, `can_chat`=false |

### Category B: Infrastructure uncertainty → error

| Condition | Outcome |
|---|---|
| Whole-batch DB pool error | **Error** — fail the page |
| BlockedSet query error | **Error** — cannot determine viewer relations safely |
| Follow query error (when followers_only content present) | **Error** — cannot determine access |
| Seller lifecycle batch query error | **Error** — cannot determine seller state |

### Category C: Corrupt required persisted data → error

| Condition | Outcome |
|---|---|
| `fallback_snapshot` unparseable for FALLBACK_ALLOWED-eligible resource | **Error** — data integrity failure |
| Occurrence row has no non-null source FK (violates CHECK constraint — should be impossible) | **Error** — data integrity failure |

### Category D: Optional indicator failure → suppress

| Condition | Outcome |
|---|---|
| Nested resource lookup fails (transient) | Suppress `nested_resource`; Content projection still LIVE |
| Nested resource access check fails (resource deleted/blocked) | Suppress `nested_resource`; Content projection still LIVE |

### Every occurrence produces a map entry

The resolver must return an entry for every input `messageID`. A nil entry is not
permitted. If resolution fails in a way that doesn't fall into Categories B or C
(which error the whole page), the result for that message is TOMBSTONE.

---

## 17. EXACT CURRENT SCHEMA-FIELD SOURCES

Verified against `backend/migrations/000001_canonical_schema.up.sql` and current
production code.

### Profile

| Projection Field | DB Source | Migration Proof |
|---|---|---|
| `username` | `user_profiles.username` | 000001 line 1731 |
| `avatar_url` | `user_profiles.avatar_url` | 000001 line 1731 |
| `store_name` | `seller_profiles.store_name` WHERE `status='active'` | 000001 line 1498 |
| `is_seller` | `seller_profiles.user_id IS NOT NULL` | 000001 line 1499 |
| `lifecycle` | `CoarsenLifecycle(users.account_status, users.deleted_at)` | 000001 lines 372, 1781 |
| block check | `user_blocks` bidirectional EXISTS | 000001 line 1687 |

### Content

| Projection Field | DB Source | Migration Proof |
|---|---|---|
| `caption` | `contents.caption` | 000001 line 675 |
| `media[].url` | `content_media.media_url` ORDER BY `position` | 000001 lines 662-669 (`position` is the canonical column; `sort_order` in authorizer adapter is a latent bug, not canonical) |
| `lifecycle` | `contents.status` → `Status.PublicLifecycle()` | 000001 line 674, `content_status_enum` lines 83-86 |
| `created_at` | `contents.created_at` | 000001 line 681 |
| `author_id` | `contents.author_id` | 000001 line 673 |
| `is_hidden` | `contents.is_hidden` | 000001 line 678 |
| `visibility` | `contents.visibility` (`content_visibility_enum`) | 000001 lines 88-92, line 685 |
| `deleted_at` | `contents.deleted_at` | 000001 line 683 |
| `share_reference` | `contents.share_reference` (JSONB) | 000001 line 680 |

### FixedPriceSale

| Projection Field | DB Source | Migration Proof |
|---|---|---|
| `title` | `products.title` via `fixed_price_sales.product_id` | 000001 line 1306 |
| `thumbnail_url` | `products.media_urls->>0` | 000001 line 1308 |
| `price` | `fixed_price_sales.price_per_unit` | 000001 line 866 |
| `lifecycle` | `fixed_price_sales.status` → `PublicLifecycle()` | 000001 lines 132-137, 868 |
| `quantity_available` | `fixed_price_sales.quantity_available` | Migration 000009 |
| `negotiation_enabled` | `fixed_price_sales.negotiation_enabled` | 000001 line 867 |
| `seller_id` | `fixed_price_sales.seller_id` | 000001 line 865 |
| `visibility` | `fixed_price_sales.visibility` (separate column from migration — verify) | Needs migration check |

### Auction

| Projection Field | DB Source | Migration Proof |
|---|---|---|
| `title` | `auctions.title` | 000001 line 476 |
| `thumbnail_url` | `products.media_urls->>0` via `auctions.product_id` | 000001 lines 490, 1308 |
| `current_bid` | `auctions.current_bid` | 000001 line 485 |
| `buy_now_price` | `auctions.buy_now_price` | 000001 line 482 |
| `end_at` | `auctions.end_at` | 000001 line 484 |
| `lifecycle` | `auctions.status` → `PublicLifecycle()` | 000001 lines 36-44, 487 |
| `seller_id` | `auctions.seller_id` | 000001 line 472 |

### Seller identity (shared across FPS/Auction)

| Projection Field | DB Source |
|---|---|
| `id` | `users.id` |
| `username` | `user_profiles.username` |
| `farm_name` | `seller_profiles.store_name` |
| `avatar_url` | `user_profiles.avatar_url` (UserCard fallback; SellerCard mirrors it) |
| `lifecycle` | `CoarsenLifecycle(users.account_status, users.deleted_at)` |
| `trust_axis` | `CoarsenSellerTrust(seller_subscriptions.status)` — nil when `"active"`, `"unavailable"` when expired/suspended |

---

## 18. IMPLEMENTATION SLICE PLAN

| Slice | Name | Scope | New Code |
|---|---|---|---|
| **3D-2a** | Projection Types + Envelope | `ResourceProjection`, `ProjectionState`, `ResourceCapabilities`, typed payloads, custom `MarshalJSON`. Pure Go, zero DB. Unit-testable. | ~200 LOC |
| **3D-2b** | Fallback Typed Parsers | Typed fallback structs + `ParseFallbackSnapshot`. Pure parsing, zero DB. | ~80 LOC |
| **3D-3** | Resolver Interface + Profile Implementation | Define `ResourceProjectionResolver` interface. Implement Profile batch-load + block + lifecycle + tombstone in serverboot. Real PostgreSQL tests. Wire into `ListMessages` (still no HTTP exposure). | ~400 LOC |
| **3D-4** | Content + Depth-1 | Content batch-load + visibility + ShareReference nested indicator. Reuse block/follow batches from 3D-3. | ~350 LOC |
| **3D-5** | FPS + Capabilities | FPS batch-load + lifecycle + seller identity + capabilities. | ~300 LOC |
| **3D-6** | Auction + Capabilities | Auction batch-load + lifecycle + seller identity + capabilities. | ~300 LOC |
| **3D-7** | FALLBACK_ALLOWED Gate | Wire fallback path for eligible resource types (FPS/Auction only). Malformed fallback → error. | ~200 LOC |
| **3D-8** | HTTP Response Wiring | Add `resource_projection` to `messageToResponse`. Final public contract. | ~100 LOC |
| **3D-9** | Legacy Hard Purge | Remove `type=reference` from attachment validation. Requires mobile switch first. | ~50 LOC deletion |

---

## 19. BLOCKERS / OWNER DECISIONS

### No blockers

All design decisions are resolved against actual code. No owner questions required.

### Design choices documented for review

1. **TOMBSTONE strips `resource_id` and `canonical_url`**: Prevents navigation to
   inaccessible resources. If the owner wants tombstone-preserved identity for
   timeline stability, this can be changed to include `resource_id` with
   `capabilities` all false (mobile would need to honor the capability gate and
   not construct navigation). Current design errs on the side of privacy.

2. **Content never uses FALLBACK_ALLOWED**: The risk of serving stale data for
   content that has since been moderated/hidden outweighs the UX benefit of
   historical display. This can be revisited if production metrics show significant
   Content projection failures from transient DB errors on public content.

3. **Seller trust_axis nil when active**: Matches the existing E8 doctrine where
   `SellerCard.Lifecycle` is nil when there is no degradation. Mobile shows no
   badge when `trust_axis` is absent; shows "Seller inactive" badge when present.

---

## 20. RECOMMENDATION

**Start with Slice 3D-2a (Projection Types + Envelope).**

This is ~200 lines of pure Go:
- All types, constants, and `MarshalJSON`
- Unit tests for every state × resource_type combination
- Zero DB dependencies

This establishes the typed contract that all subsequent slices build against.

---

**STOP.** Design reconciled. No implementation.
