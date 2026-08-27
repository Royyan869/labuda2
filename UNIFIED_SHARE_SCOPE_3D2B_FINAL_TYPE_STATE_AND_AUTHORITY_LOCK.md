# UNIFIED SHARE SCOPE 3D2B — FINAL TYPE, STATE, AND AUTHORITY LOCK

**STATUS:** `UNIFIED_SHARE_SCOPE_3D2_TYPED_PROJECTION_CONTRACT_AND_READ_RESOLVER_DESIGN_COMPLETE`

**DATE:** 2026-08-08

**MODE:** FINAL DESIGN LOCK — NO IMPLEMENTATION

**SUPERSEDES:** `UNIFIED_SHARE_SCOPE_3D2A_FINAL_CONTRACT_RECONCILIATION.md`

---

## 1. VERDICT

`UNIFIED_SHARE_SCOPE_3D2_TYPED_PROJECTION_CONTRACT_AND_READ_RESOLVER_DESIGN_COMPLETE`

All remaining contract issues reconciled. Design locked.

---

## 2. LOCKED GENERIC VIEWER CAPABILITY CONTRACT

Per 3B2 FINAL DESIGN LOCK, the Chat projection owns three **generic projection-state
capabilities** that are independent of resource type:

```go
// ProjectionViewerCapabilities are the generic, projection-state-level viewer
// capabilities owned by the Chat projection. They apply to all resource types
// and are derived from the projection state, not from domain action authority.
type ProjectionViewerCapabilities struct {
    CanView            bool `json:"can_view"`
    CanInteract        bool `json:"can_interact"`
    BlockedByTombstone bool `json:"blocked_by_tombstone"`
}
```

### State → capability mapping

| State | `can_view` | `can_interact` | `blocked_by_tombstone` |
|---|---|---|---|
| LIVE | true | true or false (per resource action caps) | false |
| FALLBACK_ALLOWED | true | false | false |
| TOMBSTONE | false | false | true |

### JSON shape (always present, never omitted)

```json
"viewer_capabilities": {
    "can_view": true,
    "can_interact": true,
    "blocked_by_tombstone": false
}
```

---

## 3. RESOURCE ACTION CAPABILITY CONTRACT

Separate from generic projection-state capabilities. Derived from canonical domain
authority (`commerce/shared.ViewerCapabilities`). Applicable only to commerce
resource types (FPS, Auction). For Profile and Content, all action capabilities
are `false`/zero-value.

```go
// ResourceActionCapabilities are the resource-specific action capabilities
// derived from canonical commerce domain authority. They mirror
// commerce/shared.ViewerCapabilities where applicable.
type ResourceActionCapabilities struct {
    Role         string `json:"role,omitempty"`   // "owner" | "buyer" | ""
    CanChat      bool   `json:"can_chat"`
    CanNegotiate bool   `json:"can_negotiate"`    // FPS only
    CanBuy       bool   `json:"can_buy"`          // FPS only
    CanBid       bool   `json:"can_bid"`          // Auction only
    CanManage    bool   `json:"can_manage"`
}
```

### Derivation rule (Chat intersection)

```
Chat action capability = canonical domain capability AND Chat projection access gate
```

Where the Chat projection access gate is:
- Viewer is NOT blocked by resource owner/seller
- Resource owner/seller lifecycle is active
- Viewer lifecycle is active (for actions requiring viewer liveness)

This means: even if `commerce/shared.ViewerCapabilities.CanBuy` would be `true`,
Chat sets `CanBuy=false` if the viewer is blocked by the seller.

### Per-type applicability

| Field | Profile | Content | FPS | Auction |
|---|---|---|---|---|
| `role` | `"owner"` or `""` | `"owner"` or `""` | `"owner"` / `"buyer"` / `""` | same as FPS |
| `can_chat` | `!blocked && viewerActive` | `false` | `!blocked && viewerActive && sellerTrustActive` | same as FPS |
| `can_negotiate` | `false` | `false` | from commerce auth, intersected with Chat gate | `false` |
| `can_buy` | `false` | `false` | from commerce auth, intersected with Chat gate | `false` |
| `can_bid` | `false` | `false` | `false` | from commerce auth, intersected with Chat gate |
| `can_manage` | `viewer==self \|\| admin` | `viewer==author \|\| admin` | `viewer==seller \|\| admin` | `viewer==seller \|\| admin` |

---

## 4. FINAL TYPED RESOURCE PROJECTION UNION

### Design: Option B — typed payload interface with concrete variants

```go
// FILE: backend/internal/interaction/chat/application/chat_resource_projection.go (NEW)
package application

import (
    "encoding/json"
    "github.com/google/uuid"
    chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
)

// =========================================================================
// Projection state
// =========================================================================

type ProjectionState string

const (
    ProjectionStateLive            ProjectionState = "LIVE"
    ProjectionStateFallbackAllowed ProjectionState = "FALLBACK_ALLOWED"
    ProjectionStateTombstone       ProjectionState = "TOMBSTONE"
)

// =========================================================================
// Viewer capabilities (generic, projection-state-level)
// =========================================================================

type ProjectionViewerCapabilities struct {
    CanView            bool `json:"can_view"`
    CanInteract        bool `json:"can_interact"`
    BlockedByTombstone bool `json:"blocked_by_tombstone"`
}

// =========================================================================
// Resource action capabilities (domain-derived)
// =========================================================================

type ResourceActionCapabilities struct {
    Role         string `json:"role,omitempty"`
    CanChat      bool   `json:"can_chat"`
    CanNegotiate bool   `json:"can_negotiate"`
    CanBuy       bool   `json:"can_buy"`
    CanBid       bool   `json:"can_bid"`
    CanManage    bool   `json:"can_manage"`
}

// =========================================================================
// Typed identity
// =========================================================================

// ResourceProjectionIdentity is the typed internal identity for a projection.
// Uses canonical enums + UUID internally; serializes to strings for JSON.
type ResourceProjectionIdentity struct {
    ResourceType chatEntity.ResourceOccurrenceResourceType `json:"-"`
    ResourceID   uuid.UUID                                 `json:"-"`
    CanonicalURL string                                    `json:"-"`
}

func (id ResourceProjectionIdentity) MarshalJSON() ([]byte, error) {
    return json.Marshal(struct {
        ResourceType string `json:"resource_type"`
        ResourceID   string `json:"resource_id"`
        CanonicalURL string `json:"canonical_url"`
    }{
        ResourceType: string(id.ResourceType),
        ResourceID:   id.ResourceID.String(),
        CanonicalURL: id.CanonicalURL,
    })
}

// =========================================================================
// Typed payload interface
// =========================================================================

// ResourceProjectionPayload is the sealed interface for typed projection
// payloads. Each concrete type below implements it.
type ResourceProjectionPayload interface {
    resourceProjectionPayloadMarker()
}

// =========================================================================
// LIVE payloads — reuse publiccard types where canonical
// =========================================================================

// ProfileLivePayload reuses publiccard.UserCard with a flat inline shape
// that adds store fields beyond what UserCard carries.
type ProfileLivePayload struct {
    Username  string  `json:"username"`
    AvatarURL *string `json:"avatar_url,omitempty"`
    StoreName *string `json:"store_name,omitempty"`
    IsSeller  bool    `json:"is_seller"`
    Lifecycle string  `json:"lifecycle"`
}

func (ProfileLivePayload) resourceProjectionPayloadMarker() {}

// ContentLivePayload — Chat-specific Content projection.
type ContentLivePayload struct {
    Caption        *string                  `json:"caption,omitempty"`
    Media          []MediaRef               `json:"media"`
    Lifecycle      string                   `json:"lifecycle"`
    CreatedAt      string                   `json:"created_at"`
    Author         ContentAuthorIdentity    `json:"author"`
    NestedResource *NestedResourceIndicator `json:"nested_resource,omitempty"`
}

func (ContentLivePayload) resourceProjectionPayloadMarker() {}

type ContentAuthorIdentity struct {
    ID        uuid.UUID `json:"id"`
    Username  string    `json:"username"`
    AvatarURL *string   `json:"avatar_url,omitempty"`
    Lifecycle string    `json:"lifecycle"`
}

type MediaRef struct {
    URL string `json:"url"`
}

// FixedPriceSaleLivePayload — Chat-specific FPS projection.
// Wraps the canonical publiccard.FixedPriceSaleCard.
type FixedPriceSaleLivePayload struct {
    ID                uuid.UUID              `json:"id"`
    Title             string                 `json:"title"`
    ThumbnailURL      *string                `json:"thumbnail_url,omitempty"`
    Price             int64                  `json:"price"`
    Lifecycle         string                 `json:"lifecycle"`
    QuantityAvailable *int64                 `json:"quantity_available,omitempty"`
    Seller            SellerLiveIdentity     `json:"seller"`
}

func (FixedPriceSaleLivePayload) resourceProjectionPayloadMarker() {}

// AuctionLivePayload — Chat-specific Auction projection.
type AuctionLivePayload struct {
    ID           uuid.UUID          `json:"id"`
    Title        string             `json:"title"`
    ThumbnailURL *string            `json:"thumbnail_url,omitempty"`
    CurrentBid   *int64             `json:"current_bid,omitempty"`
    BuyNowPrice  *int64             `json:"buy_now_price,omitempty"`
    EndAt        string             `json:"end_at"`
    Lifecycle    string             `json:"lifecycle"`
    Seller       SellerLiveIdentity `json:"seller"`
}

func (AuctionLivePayload) resourceProjectionPayloadMarker() {}

// SellerLiveIdentity — compact seller block.
// Flat inline shape; does NOT fork publiccard.SellerCard.
type SellerLiveIdentity struct {
    ID        uuid.UUID `json:"id"`
    Username  string    `json:"username"`
    FarmName  *string   `json:"farm_name,omitempty"`
    AvatarURL *string   `json:"avatar_url,omitempty"`
    Lifecycle string    `json:"lifecycle"`        // user-identity axis
    TrustAxis *string   `json:"trust_axis,omitempty"` // nil="active", present="unavailable"
}

// =========================================================================
// FALLBACK_ALLOWED payloads — typed historical display only
// =========================================================================

type FixedPriceSaleFallbackPayload struct {
    Title           string  `json:"title"`
    ImageURL        *string `json:"image_url,omitempty"`
    SellerStoreName string  `json:"seller_store_name"`
}

func (FixedPriceSaleFallbackPayload) resourceProjectionPayloadMarker() {}

type AuctionFallbackPayload struct {
    Title           string  `json:"title"`
    ImageURL        *string `json:"image_url,omitempty"`
    SellerStoreName string  `json:"seller_store_name"`
}

func (AuctionFallbackPayload) resourceProjectionPayloadMarker() {}

// Profile and Content have NO fallback payload types — they never use
// FALLBACK_ALLOWED.

// =========================================================================
// Nested resource (depth-1, identity-only)
// =========================================================================

type NestedResourceIndicator struct {
    ResourceType string `json:"resource_type"`
    ResourceID   string `json:"resource_id"`
}

// =========================================================================
// ResourceProjection — the top-level envelope
// =========================================================================

type ResourceProjection struct {
    State              ProjectionState                `json:"state"`
    Identity           ResourceProjectionIdentity     `json:"identity"`
    ViewerCapabilities ProjectionViewerCapabilities   `json:"viewer_capabilities"`
    ActionCapabilities ResourceActionCapabilities     `json:"action_capabilities"`
    Payload            ResourceProjectionPayload       `json:"-"`
}

// MarshalJSON produces the canonical typed-envelope JSON shape.
// Exactly one payload key is emitted based on State + ResourceType.
func (p ResourceProjection) MarshalJSON() ([]byte, error) {
    outer := map[string]interface{}{
        "state":               string(p.State),
        "resource_type":       string(p.Identity.ResourceType),
        "resource_id":         p.Identity.ResourceID.String(),
        "canonical_url":       p.Identity.CanonicalURL,
        "viewer_capabilities": p.ViewerCapabilities,
        "action_capabilities": p.ActionCapabilities,
    }
    switch payload := p.Payload.(type) {
    case ProfileLivePayload:
        outer["profile"] = payload
    case ContentLivePayload:
        outer["content"] = payload
    case FixedPriceSaleLivePayload:
        outer["fixed_price_sale"] = payload
    case AuctionLivePayload:
        outer["auction"] = payload
    case FixedPriceSaleFallbackPayload:
        outer["fixed_price_sale"] = payload
    case AuctionFallbackPayload:
        outer["auction"] = payload
    case nil:
        // TOMBSTONE — no payload key emitted
    default:
        // Should be unreachable; defensive
    }
    return json.Marshal(outer)
}

// TombstoneProjection is a convenience constructor for TOMBSTONE state.
func TombstoneProjection(rt chatEntity.ResourceOccurrenceResourceType) ResourceProjection {
    return ResourceProjection{
        State: ProjectionStateTombstone,
        Identity: ResourceProjectionIdentity{
            ResourceType: rt,
            ResourceID:   uuid.Nil,
            CanonicalURL: "",
        },
        ViewerCapabilities: ProjectionViewerCapabilities{
            CanView: false, CanInteract: false, BlockedByTombstone: true,
        },
        ActionCapabilities: ResourceActionCapabilities{},
        Payload:            nil,
    }
}
```

### Why Option B over Option A

Option A (separate `LivePayload` + `FallbackPayload` fields) requires TWO nullable
fields and runtime validation that exactly one is set. Option B (single `Payload`
interface) makes the invariant structural: the concrete type IS the discriminator.
TOMBSTONE has nil payload. This is cleaner and eliminates the "four-nullable-fields"
anti-pattern the original scope explicitly prohibits.

---

## 5. TYPED RESOURCE IDENTITY CONTRACT

Internal (Go): `chatEntity.ResourceOccurrenceResourceType` + `uuid.UUID`.
External (JSON): string + string.

`CanonicalURL` is server-built from identity at projection time:
- Profile: `/user/{id}`
- Content: `/content/{id}`
- FPS: `/listing/{id}`
- Auction: `/auction/{id}`

Path-only, not absolute URL. The client prefixes the base URL.

---

## 6. FINAL LIVE JSON

### Profile LIVE

```json
{
  "state": "LIVE",
  "resource_type": "profile",
  "resource_id": "11111111-1111-1111-1111-111111111111",
  "canonical_url": "/user/11111111-1111-1111-1111-111111111111",
  "viewer_capabilities": {
    "can_view": true,
    "can_interact": false,
    "blocked_by_tombstone": false
  },
  "action_capabilities": {
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
  "viewer_capabilities": {
    "can_view": true,
    "can_interact": false,
    "blocked_by_tombstone": false
  },
  "action_capabilities": {
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
      "lifecycle": "active"
    },
    "nested_resource": {
      "resource_type": "fixed_price_sale",
      "resource_id": "44444444-4444-4444-4444-444444444444"
    }
  }
}
```

### LIVE FPS (active, non-owner viewer)

```json
{
  "state": "LIVE",
  "resource_type": "fixed_price_sale",
  "resource_id": "44444444-4444-4444-4444-444444444444",
  "canonical_url": "/listing/44444444-4444-4444-4444-444444444444",
  "viewer_capabilities": {
    "can_view": true,
    "can_interact": true,
    "blocked_by_tombstone": false
  },
  "action_capabilities": {
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
      "lifecycle": "active"
    }
  }
}
```

### LIVE Auction (active, non-owner viewer)

```json
{
  "state": "LIVE",
  "resource_type": "auction",
  "resource_id": "55555555-5555-5555-5555-555555555555",
  "canonical_url": "/auction/55555555-5555-5555-5555-555555555555",
  "viewer_capabilities": {
    "can_view": true,
    "can_interact": true,
    "blocked_by_tombstone": false
  },
  "action_capabilities": {
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
      "lifecycle": "active"
    }
  }
}
```

---

## 7. FINAL FALLBACK_ALLOWED JSON

### FPS FALLBACK_ALLOWED

```json
{
  "state": "FALLBACK_ALLOWED",
  "resource_type": "fixed_price_sale",
  "resource_id": "44444444-4444-4444-4444-444444444444",
  "canonical_url": "/listing/44444444-4444-4444-4444-444444444444",
  "viewer_capabilities": {
    "can_view": true,
    "can_interact": false,
    "blocked_by_tombstone": false
  },
  "action_capabilities": {
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

Note: `can_interact` is ALWAYS false in FALLBACK_ALLOWED. No price/bid/status/quantity
from fallback. The payload key is `fixed_price_sale` (same key as LIVE) — the client
discriminates by `state`.

---

## 8. FINAL TOMBSTONE JSON

```json
{
  "state": "TOMBSTONE",
  "resource_type": "content",
  "viewer_capabilities": {
    "can_view": false,
    "can_interact": false,
    "blocked_by_tombstone": true
  },
  "action_capabilities": {
    "can_chat": false,
    "can_negotiate": false,
    "can_buy": false,
    "can_bid": false,
    "can_manage": false
  }
}
```

### TOMBSTONE identity contract

| Field | Present? | Rationale |
|---|---|---|
| `state` | Yes — `"TOMBSTONE"` | Required discriminator |
| `resource_type` | Yes | Enables correct placeholder icon without leaking which specific resource |
| `resource_id` | **No** | Prevents navigation to inaccessible resource |
| `canonical_url` | **No** | Same — no valid URL for this viewer |
| `viewer_capabilities` | Yes — all false/blocked | Canonical 3B2 tombstone semantics |
| `action_capabilities` | Yes — all false | No actionable authority |
| typed payload key | **No** | No `profile`/`content`/`fixed_price_sale`/`auction` key |

### MarshalJSON behavior for TOMBSTONE

When `State == TOMBSTONE` (or `Payload == nil`), `MarshalJSON` omits:
- `resource_id`
- `canonical_url`
- payload key

When `State == LIVE || State == FALLBACK_ALLOWED`, all identity fields + one payload
key are emitted.

---

## 9. FALLBACK_ALLOWED REACHABLE-CONDITION ANALYSIS

### ON DELETE behavior of occurrence source FKs

From `000034_chat_message_resource_occurrences.up.sql`:
```sql
profile_source_id           uuid REFERENCES users(id) ON DELETE RESTRICT,
content_source_id           uuid REFERENCES contents(id) ON DELETE RESTRICT,
fixed_price_sale_source_id  uuid REFERENCES fixed_price_sales(id) ON DELETE RESTRICT,
auction_source_id           uuid REFERENCES auctions(id) ON DELETE RESTRICT,
```

All four are `ON DELETE RESTRICT`. This means:
- A user/profile CANNOT be hard-deleted while any occurrence references it
- A content CANNOT be hard-deleted while any occurrence references it
- An FPS CANNOT be hard-deleted while any occurrence references it
- An auction CANNOT be hard-deleted while any occurrence references it

### Soft-delete analysis

| Resource | Soft-delete mechanism | Effect on occurrence FK |
|---|---|---|
| Profile | `users.deleted_at` | FK still valid (row exists) |
| Content | `contents.deleted_at` | FK still valid (row exists) |
| FPS | No `deleted_at` column; status changes only | FK still valid; row always exists |
| Auction | No `deleted_at` column; status changes only | FK still valid; row always exists |

### FALLBACK_ALLOWED eligibility verdict

| Condition | Reachable? | Notes |
|---|---|---|
| Source row hard-deleted | **No** | Blocked by ON DELETE RESTRICT |
| Source row soft-deleted (Profile/Content) | Yes | → TOMBSTONE (not FALLBACK_ALLOWED per privacy rules) |
| Source row status change (FPS: active→sold→withdrawn) | Yes | → LIVE non-actionable (not FALLBACK_ALLOWED — row still exists) |
| Source row status change (Auction: active→ended→cancelled) | Yes | → LIVE non-actionable |
| Seller deleted/banned | Yes | → TOMBSTONE |
| Localized DB error on single-row source query | **No** | Current repository architecture has no per-resource error granularity; batch queries fail whole-batch |

### Final rule

**FALLBACK_ALLOWED has NO currently reachable production trigger.** It is retained
in the contract (wire enum, Go types, JSON shape) for future use when a reachable
live-unavailable-but-historically-safe condition is identified. Until then:

- The fallback typed parsers (`FixedPriceSaleFallbackPayload`, `AuctionFallbackPayload`)
  are defined in 3D-2b as pure parsing code
- The resolver implementation (3D-3 through 3D-6) never selects FALLBACK_ALLOWED
- Slice 3D-7 ("FALLBACK_ALLOWED Gate") is deferred until a concrete trigger is proven

This is NOT a design gap. It is explicit contract preservation for a known state
machine slot whose runtime path is gated on a future concrete condition.

---

## 10. INFRASTRUCTURE / ERROR SEMANTICS

| Condition | Outcome |
|---|---|
| DB pool failure during any batch query | **Error propagates** — page fails (500) |
| BlockedSet query error | **Error propagates** |
| Follow query error (when followers_only content present) | **Error propagates** |
| Seller lifecycle batch query error | **Error propagates** |
| `fallback_snapshot` unparseable (when FALLBACK_ALLOWED is selected) | **Error propagates** |
| Occurrence row with no non-null source FK (CHECK violation — defensive) | **Error propagates** |
| Resource soft-deleted (deleted_at, status=deleted) | **TOMBSTONE** |
| Resource hidden/moderated | **TOMBSTONE** |
| Viewer blocked by resource owner | **TOMBSTONE** |
| Privacy/visibility denial | **TOMBSTONE** |
| Owner/seller lifecycle failed (suspended/banned/deleted) | **TOMBSTONE** |
| Seller subscription expired (FPS/Auction) | **LIVE** + `can_chat`/`can_buy`/`can_bid`=false |
| FPS/Auction terminal lifecycle (sold/ended/cancelled) | **LIVE** + `can_buy`/`can_bid`=false |
| Resource row found + viewer permitted | **LIVE** |

### Seller-liveness query error

If the seller lifecycle batch query (step 5 in batch plan) fails, this is an
infrastructure error → page fails. TOMBSTONE requires a POSITIVELY-KNOWN lifecycle
state. Seller lifecycle uncertainty must not be mislabeled as TOMBSTONE.

### Every occurrence produces a map entry

The resolver returns `map[messageID]*ResourceProjection` with exactly one entry
per input messageID. Every entry has a valid `ResourceProjection` (LIVE,
FALLBACK_ALLOWED, or TOMBSTONE). Nil entries are not permitted.

---

## 11. EXACT CURRENT SCHEMA VERIFICATION

All sources verified against `backend/migrations/000001_canonical_schema.up.sql`
and supplementary migrations.

### `fixed_price_sales` columns (verified)

| Column | Type | Source |
|---|---|---|
| `id` | uuid | 000001:863 |
| `product_id` | uuid NOT NULL | 000001:864 |
| `seller_id` | uuid NOT NULL | 000001:865 |
| `price_per_unit` | bigint NOT NULL | 000001:866 |
| `negotiation_enabled` | boolean DEFAULT false | 000001:867 |
| `status` | `fixed_price_sale_status_enum` | 000001:868 |
| `published_at` | timestamptz | 000001:869 |
| `sold_at` | timestamptz | 000001:870 |
| `withdrawn_at` | timestamptz | 000001:871 |
| `created_at` | timestamptz | 000001:872 |
| `updated_at` | timestamptz | 000001:873 |
| `quantity_available` | integer DEFAULT 1 NOT NULL | 000009:16 |

**NO `visibility` column exists on `fixed_price_sales`.** The Go entity has
`Visibility FixedPriceSaleVisibility` as a computed/memory field derived from
status (draft→private, active→public). For Chat projection: draft → owner-only
(LIVE for owner, TOMBSTONE for others). Active/sold/withdrawn → publicly viewable.

**No `deleted_at` column.** FPS rows are never soft-deleted — status transitions
handle lifecycle.

### `auctions` columns (verified)

| Column | Type | Source |
|---|---|---|
| `id` | uuid | 000001:471 |
| `seller_id` | uuid NOT NULL | 000001:472 |
| `listing_id` | uuid | 000001:473 |
| `order_id` | uuid | 000001:474 |
| `settlement_deadline` | timestamptz | 000001:475 |
| **`title`** | **text NOT NULL** | **000001:476** |
| `description` | text NOT NULL | 000001:477 |
| `preparation_time` | `preparation_time_enum` NOT NULL | 000001:478 |
| `preparation_note` | text | 000001:479 |
| `start_price` | bigint NOT NULL | 000001:480 |
| `bid_increment` | bigint NOT NULL | 000001:481 |
| `buy_now_price` | bigint | 000001:482 |
| `start_at` | timestamptz NOT NULL | 000001:483 |
| `end_at` | timestamptz NOT NULL | 000001:484 |
| `current_bid` | bigint | 000001:485 |
| `current_winner_id` | uuid | 000001:486 |
| `status` | `auction_status_enum` | 000001:487 |
| `created_at` | timestamptz | 000001:488 |
| `updated_at` | timestamptz | 000001:489 |
| `product_id` | uuid NOT NULL | 000001:490 |

**`auctions.title` exists (line 476).** This is the canonical title authority for
LIVE Auction projection. The fallback builder uses `p.title` from `products` via
JOIN — that is the historical fallback data, used as-is for FALLBACK_ALLOWED.

### `content_media` columns (verified)

| Column | Type | Source |
|---|---|---|
| `id` | uuid | 000001:663 |
| `content_id` | uuid NOT NULL | 000001:664 |
| `media_url` | text NOT NULL | 000001:665 |
| `media_type` | `media_type_enum` NOT NULL | 000001:666 |
| **`position`** | **integer DEFAULT 0 NOT NULL** | **000001:667** |
| `created_at` | timestamptz | 000001:668 |

**`position` is the canonical ordering column.** The `sort_order` reference in
`chat_resource_authorizer_adapter.go:274` is a latent bug — it does not match
the actual schema. Chat projection uses `ORDER BY position`.

### Occurrence FK constraints (verified)

All four source FKs use `ON DELETE RESTRICT` (000034:26-29). Source rows cannot
be hard-deleted while occurrences exist. See §9 for implications.

---

## 12. PUBLICCARD REUSE MATRIX

| publiccard Type | Reused in Chat? | How |
|---|---|---|
| `UserCard` | **No** — Chat uses inline `ProfileLivePayload` and `ContentAuthorIdentity` and `SellerLiveIdentity` | `UserCard` carries `followers_count`/`following_count` irrelevant to Chat; `Lifecycle` is a `*string` with nil-means-unknown semantics. Chat needs explicit lifecycle strings with no nil ambiguity |
| `SellerCard` | **No** — Chat uses inline `SellerLiveIdentity` | `SellerCard` wraps `UserCard` (nested JSON) + adds `Tier`. Chat needs a flat seller block (no nested `user` object) without tier complexity. The fields are the same canonical sources but the JSON shape is Chat-specific |
| `FixedPriceSaleCard` | **No** — Chat uses inline `FixedPriceSaleLivePayload` | `FixedPriceSaleCard` nests `SellerCard` (which nests `UserCard`). Chat needs a flat structure. `FixedPriceSaleCard` also carries `currency` (unused in Chat) and lacks `quantity_available` |
| `AuctionCard` | **No** — Chat uses inline `AuctionLivePayload` | Same nesting issue. Chat projection is compact — flat seller block, no nested `user` object |
| `ContentCard` | **No** — Chat uses inline `ContentLivePayload` | `ContentCard` carries `SharedFixedPriceSale` / `SharedAuction` (reserved for future), lacks `NestedResource`. Different contract |

### Why not reuse directly

The `publiccard` types are designed for detail-page surfaces (listing detail, auction
detail, content detail). Chat is a compact inline card — it needs:
- Flat identity blocks (no nested `seller.user.username`)
- Chat-specific fields (`quantity_available`, `nested_resource`)
- Explicit lifecycle strings (no `*string` nil-means-unknown)

The canonical DB sources are identical. The JSON shapes differ for Chat context.

---

## 13. PROFILE CONTRACT

### State matrix

| Condition | State | `can_view` | `can_interact` | `blocked_by_tombstone` |
|---|---|---|---|---|
| Active, not blocked | LIVE | true | false | false |
| Self (viewer==profile) | LIVE | true | false | false |
| Blocked by profile | TOMBSTONE | false | false | true |
| Suspended | TOMBSTONE | false | false | true |
| Banned | TOMBSTONE | false | false | true |
| Deleted | TOMBSTONE | false | false | true |

### Action capabilities (Profile)

| Condition | `role` | `can_chat` | `can_manage` |
|---|---|---|---|
| Viewer != profile, not blocked, viewer active | `""` | true | false |
| Viewer == profile | `"owner"` | false | true |
| Admin | `""` | false | true |
| Blocked/suspended/banned/deleted | — | false | false |

### Canonical sources

| Field | Source |
|---|---|
| `username` | `user_profiles.username` |
| `avatar_url` | `user_profiles.avatar_url` |
| `store_name` | `seller_profiles.store_name` WHERE `status='active'` |
| `is_seller` | `seller_profiles.user_id IS NOT NULL` |
| `lifecycle` | `CoarsenLifecycle(users.account_status, users.deleted_at)` |
| block | `user_blocks` bidirectional EXISTS |

---

## 14. CONTENT CONTRACT

### State matrix

| Condition | State | `can_view` | `can_interact` | `blocked_by_tombstone` |
|---|---|---|---|---|
| Public, active, not hidden/deleted, author active, not blocked | LIVE | true | false | false |
| Followers-only, viewer follows, same conditions | LIVE | true | false | false |
| Followers-only, viewer doesn't follow | TOMBSTONE | false | false | true |
| Private, viewer==author | LIVE | true | false | false |
| Private, viewer≠author | TOMBSTONE | false | false | true |
| Deleted (`deleted_at IS NOT NULL`) | TOMBSTONE | false | false | true |
| Hidden (`is_hidden = true`) | TOMBSTONE | false | false | true |
| Status = `deleted` | TOMBSTONE | false | false | true |
| Author suspended/banned/deleted | TOMBSTONE | false | false | true |
| Viewer blocked by author | TOMBSTONE | false | false | true |

### Action capabilities (Content)

All `false`. Content has no commerce actions.

| Field | Value |
|---|---|
| `role` | `"owner"` if viewer==author, else `""` |
| `can_chat` | `false` |
| `can_manage` | `true` if viewer==author or admin |

### Canonical sources

| Field | Source |
|---|---|
| `caption` | `contents.caption` |
| `media[].url` | `content_media.media_url` ORDER BY `position` |
| `lifecycle` | `contents.status` → `Status.PublicLifecycle()` |
| `created_at` | `contents.created_at` |
| `author.*` | `contents.author_id` → `user_profiles` + `users.account_status` |
| `visibility` | `contents.visibility` (`content_visibility_enum`) |
| `is_hidden` | `contents.is_hidden` |
| `deleted_at` | `contents.deleted_at` |
| `share_reference` | `contents.share_reference` (JSONB) |

---

## 15. FPS CONTRACT

### State matrix

| FPS Status | Seller | Viewer Block | Visibility (derived) | State | `can_view` | `can_interact` |
|---|---|---|---|---|---|---|
| `active` | active | no | public | LIVE | true | true |
| `active` | active | **yes** | public | TOMBSTONE | false | false |
| `active` | inactive (sub expired) | no | public | LIVE | true | false |
| `active` | suspended/banned/deleted | — | — | TOMBSTONE | false | false |
| `draft` | — | no (owner) | owner-only | LIVE | true | false |
| `draft` | — | no (non-owner) | owner-only | TOMBSTONE | false | false |
| `sold` | active | no | public | LIVE | true | false |
| `withdrawn` | active | no | public | LIVE | true | false |

**Visibility derivation**: FPS has no `visibility` DB column. Derived from status:
- `draft` → owner-only (LIVE for seller, TOMBSTONE for others)
- `active` / `sold` / `withdrawn` → public

### Action capabilities (FPS)

| Condition | `role` | `can_chat` | `can_negotiate` | `can_buy` | `can_manage` |
|---|---|---|---|---|---|
| Seller (owner) | `"owner"` | false | false | false | true |
| Non-owner, `can_interact`=true, not blocked, seller active | `"buyer"` | true | `negotiation_enabled` | true | false |
| Non-owner, `can_interact`=false | `""` | false | false | false | false |
| Admin | `""` | false | false | false | true |

### Canonical sources

| Field | Source |
|---|---|
| `title` | `products.title` via `fixed_price_sales.product_id` |
| `thumbnail_url` | `products.media_urls->>0` |
| `price` | `fixed_price_sales.price_per_unit` |
| `lifecycle` | `FixedPriceSaleStatus.PublicLifecycle()` → `"active"` / `"unavailable"` |
| `quantity_available` | `fixed_price_sales.quantity_available` (000009) |
| `negotiation_enabled` | `fixed_price_sales.negotiation_enabled` |
| `seller.*` | `fixed_price_sales.seller_id` → `users` + `user_profiles` + `seller_profiles` + `seller_subscriptions` |

---

## 16. AUCTION CONTRACT

### State matrix

| Auction Status | Seller | Viewer Block | State | `can_view` | `can_interact` |
|---|---|---|---|---|---|
| `active` | active | no | LIVE | true | true |
| `active` | active | **yes** | TOMBSTONE | false | false |
| `active` | inactive (sub expired) | no | LIVE | true | false |
| `active` | suspended/banned/deleted | — | TOMBSTONE | false | false |
| `scheduled` | active | no | LIVE | true | false |
| `waiting_settlement` | active | no | LIVE | true | false |
| `ended` | active | no | LIVE | true | false |
| `cancelled` | active | no | LIVE | true | false |
| `expired_bnr` | active | no | LIVE | true | false |
| `draft` | — | no (owner) | LIVE | true | false |
| `draft` | — | no (non-owner) | TOMBSTONE | false | false |

**Note**: Auctions have no separate visibility concept. Draft is owner-only (by
convention — the handler 404s for non-owner draft access).

### Action capabilities (Auction)

| Condition | `role` | `can_chat` | `can_bid` | `can_manage` |
|---|---|---|---|---|
| Seller (owner) | `"owner"` | false | false | true |
| Non-owner, `can_interact`=true, not blocked, seller active | `"buyer"` | true | true | false |
| Non-owner, `can_interact`=false | `""` | false | false | false |
| Admin | `""` | false | false | true |

### Canonical sources

| Field | Source |
|---|---|
| `title` | `auctions.title` (000001:476) |
| `thumbnail_url` | `products.media_urls->>0` via `auctions.product_id` |
| `current_bid` | `auctions.current_bid` |
| `buy_now_price` | `auctions.buy_now_price` |
| `end_at` | `auctions.end_at` |
| `lifecycle` | `Status.PublicLifecycle()` → `"active"` (active, waiting_settlement) / `"unavailable"` (others) |
| `seller.*` | `auctions.seller_id` → same as FPS seller identity |

### `auctions.title` vs historical Scope 3C evidence

Prior Scope 3C evidence may have reported `a.title` as an invalid column reference.
**Current schema (000001 line 476) confirms `auctions.title text NOT NULL` exists.**
If Scope 3C used `a.title` and it failed, that was a transient environment issue
or incorrect column reference syntax. The column is present and is the canonical
title authority for LIVE Auction projection.

---

## 17. CONTENT DEPTH-1 CONTRACT

Unchanged from 3D-2A. LOCKED.

- Primary: selected Content occurrence
- Nested: resolved from `contents.share_reference` JSONB at read time
- Depth: exactly 1 — no recursion
- Indicator: `NestedResourceIndicator{ResourceType, ResourceID}` — identity only
- Access check: apply canonical rules for nested type (block, lifecycle, visibility)
- Inaccessible → suppress `nested_resource` field entirely
- No historical title/image leakage

---

## 18. RESOLVER INTERFACE

```go
// FILE: backend/internal/interaction/chat/application/chat_resource_projection.go (NEW)

// ResourceProjectionResolver resolves occurrence rows into viewer-aware
// ResourceProjections. Batch-oriented: all occurrences for a message page
// are resolved together in one call.
//
// Owned by Chat application layer. Implemented in serverboot.
type ResourceProjectionResolver interface {
    // ResolveResourceProjections resolves all occurrences into viewer-specific
    // projections. Returns a map keyed by message_id with exactly one entry
    // per input messageID. Every entry is a valid projection (LIVE,
    // FALLBACK_ALLOWED, or TOMBSTONE). Nil entries are not permitted.
    //
    // Error conditions (fail the page):
    //   - DB pool / batch query infrastructure failure
    //   - Viewer relation query failure (block, follow)
    //   - Seller lifecycle batch query failure
    //   - Malformed fallback_snapshot (when FALLBACK_ALLOWED is selected)
    //   - Occurrence row with invalid source FK (defensive)
    //
    // Non-error outcomes:
    //   - Resource soft-deleted/hidden/moderated → TOMBSTONE
    //   - Viewer blocked → TOMBSTONE
    //   - Privacy/visibility denial → TOMBSTONE
    //   - Owner lifecycle failed → TOMBSTONE
    //   - Resource found + viewer permitted → LIVE
    ResolveResourceProjections(
        ctx context.Context,
        viewerID uuid.UUID,
        occurrences map[uuid.UUID]*chatEntity.ChatMessageResourceOccurrence,
    ) (map[uuid.UUID]*ResourceProjection, error)
}
```

---

## 19. BATCH / QUERY PLAN

### Algorithm (unchanged from 3D-2A)

```
1. Partition occurrences by resource type → deduped ID arrays
2. Batch-load source rows (one query per present type)
3. One BlockedSet query for all distinct author/seller IDs (fail-closed)
4. One follow query if followers-only Content present (fail-closed)
5. One seller lifecycle batch query (fail-closed)
6. Per-occurrence in-memory resolution (no DB)
7. Depth-1: batch-check nested IDs if needed (fail → suppress indicator)
```

### Query budget (additional on 3D-1 baseline of 5)

| Scenario | Added queries | Total |
|---|---|---|
| Normal only | 0 | 5 |
| Profile only | +2 (profile batch + BlockedSet) | 7 |
| Content only | +3 (content batch + BlockedSet + follow) | 8 |
| FPS only | +3 (FPS batch + BlockedSet + seller lifecycle) | 8 |
| Auction only | +3 (Auction batch + BlockedSet + seller lifecycle) | 8 |
| All 4 types | +6 (4 batches + BlockedSet + seller lifecycle; follow merged) | 11 |
| Content + depth-1 (new type) | +0-1 (only if nested type not in primary set) | 11-12 |

---

## 20. IMPLEMENTATION SLICES

| Slice | Name | Scope |
|---|---|---|
| **3D-2a** | Projection Types + Envelope | All Go types, `MarshalJSON`, typed payloads, capabilities. Pure Go, zero DB. ~250 LOC. |
| **3D-2b** | Fallback Typed Parsers | Typed fallback structs + `ParseFallbackSnapshot`. Pure parsing, zero DB. ~80 LOC. |
| **3D-3** | Resolver Interface + Profile | Define `ResourceProjectionResolver` interface. Profile batch-load + block + lifecycle + tombstone in serverboot. Real PostgreSQL tests. ~400 LOC. |
| **3D-4** | Content + Depth-1 | Content batch-load + visibility + `ShareReference` nested indicator. ~350 LOC. |
| **3D-5** | FPS + Capabilities | FPS batch-load + seller identity + action capabilities. ~300 LOC. |
| **3D-6** | Auction + Capabilities | Auction batch-load + seller identity + action capabilities. ~300 LOC. |
| **3D-7** | FALLBACK_ALLOWED Gate | Wire fallback path. **Deferred** until a reachable trigger is proven. |
| **3D-8** | HTTP Response Wiring | Add `resource_projection` to `messageToResponse`. ~100 LOC. |
| **3D-9** | Legacy Hard Purge | Remove `type=reference` from attachment validation. Requires mobile switch. |

---

## 21. BLOCKERS / OWNER DECISIONS

### No blockers

All design decisions are resolved. No owner questions required.

### Design decisions documented for review

1. **FALLBACK_ALLOWED has no current production trigger** (§9). The contract types
   and wire enum are defined; the runtime path is deferred until a concrete
   reachable condition is identified. This is intentional contract preservation.

2. **TOMBSTONE strips `resource_id` and `canonical_url`** (§8). This errs on the
   side of privacy — a tombstoned resource must not expose navigation targets.
   If the owner wants tombstone identity preservation for timeline stability,
   `resource_id` can be restored with `can_view=false` as the enforcement gate.

3. **`publiccard` types are not directly reused** (§12). The Chat projection uses
   flat inline shapes rather than nested `publiccard` types. The canonical DB
   sources are identical; only the JSON shape differs.

4. **Chat action capabilities intersect domain authority with Chat access gates**
   (§3). `can_buy` from commerce authority is `true` only when Chat additionally
   confirms the viewer is not blocked by the seller and the seller is active.
   This is stricter than the commerce detail surface (which is fail-open on
   block errors).

---

## 22. RECOMMENDATION

**Start with Slice 3D-2a (Projection Types + Envelope).**

~250 lines of pure Go:
- `ResourceProjection` + `MarshalJSON`
- `ProjectionState` constants
- `ProjectionViewerCapabilities` + `ResourceActionCapabilities`
- `ResourceProjectionIdentity` + custom `MarshalJSON`
- All typed payloads (7 concrete types implementing `ResourceProjectionPayload`)
- `NestedResourceIndicator`
- `TombstoneProjection` constructor
- Unit tests for `MarshalJSON` of every state × type combination

Zero DB dependencies. Establishes the immutable typed contract for all subsequent
slices.

---

**STOP.** Design locked. No implementation.
