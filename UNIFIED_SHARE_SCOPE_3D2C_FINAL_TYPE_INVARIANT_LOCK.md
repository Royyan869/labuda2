# UNIFIED SHARE SCOPE 3D2C — FINAL TYPE INVARIANT LOCK

**STATUS:** `UNIFIED_SHARE_SCOPE_3D2_TYPED_PROJECTION_CONTRACT_AND_READ_RESOLVER_DESIGN_COMPLETE`

**DATE:** 2026-08-08

**MODE:** FINAL DESIGN LOCK — NO IMPLEMENTATION

**SUPERSEDES:** `UNIFIED_SHARE_SCOPE_3D2B_FINAL_TYPE_STATE_AND_AUTHORITY_LOCK.md`

---

## 1. VERDICT

`UNIFIED_SHARE_SCOPE_3D2_TYPED_PROJECTION_CONTRACT_AND_READ_RESOLVER_DESIGN_COMPLETE`

All type invariants, serialization contracts, and capability authorities locked.
No contradictions remain.

---

## 2. STRICT RESOURCEPROJECTION INVARIANTS

### Validate() — called before any serialization

```go
func (p ResourceProjection) Validate() error {
    // 1. State must be a known constant.
    switch p.State {
    case ProjectionStateLive, ProjectionStateFallbackAllowed, ProjectionStateTombstone:
    default:
        return fmt.Errorf("chat: invalid projection state %q", p.State)
    }

    // 2. ResourceType must be valid.
    if !p.Identity.ResourceType.IsValid() {
        return fmt.Errorf("chat: invalid resource type %q", p.Identity.ResourceType)
    }

    // 3. State-specific identity rules.
    switch p.State {
    case ProjectionStateLive, ProjectionStateFallbackAllowed:
        if p.Identity.ResourceID == uuid.Nil {
            return fmt.Errorf("chat: %s projection requires non-nil resource ID", p.State)
        }
    case ProjectionStateTombstone:
        // resource_id is intentionally absent from serialization.
        // p.Identity.ResourceID is ignored; may be uuid.Nil.
    }

    // 4. State-specific payload rules.
    switch p.State {
    case ProjectionStateLive:
        if p.Payload == nil {
            return fmt.Errorf("chat: LIVE projection requires non-nil payload")
        }
        if err := validateLivePayloadType(p.Identity.ResourceType, p.Payload); err != nil {
            return err
        }
    case ProjectionStateFallbackAllowed:
        if p.Payload == nil {
            return fmt.Errorf("chat: FALLBACK_ALLOWED projection requires non-nil payload")
        }
        if err := validateFallbackPayloadType(p.Identity.ResourceType, p.Payload); err != nil {
            return err
        }
    case ProjectionStateTombstone:
        if p.Payload != nil {
            return fmt.Errorf("chat: TOMBSTONE projection requires nil payload")
        }
    }

    // 5. Viewer capabilities must be consistent with state.
    if err := validateViewerCapabilities(p.State, p.ViewerCapabilities); err != nil {
        return err
    }

    return nil
}

func validateLivePayloadType(rt ResourceOccurrenceResourceType, p ResourceProjectionPayload) error {
    switch rt {
    case ResourceOccurrenceResourceTypeProfile:
        if _, ok := p.(ProfileLivePayload); !ok {
            return fmt.Errorf("chat: LIVE profile requires ProfileLivePayload, got %T", p)
        }
    case ResourceOccurrenceResourceTypeContent:
        if _, ok := p.(ContentLivePayload); !ok {
            return fmt.Errorf("chat: LIVE content requires ContentLivePayload, got %T", p)
        }
    case ResourceOccurrenceResourceTypeFixedPriceSale:
        if _, ok := p.(FixedPriceSaleLivePayload); !ok {
            return fmt.Errorf("chat: LIVE FPS requires FixedPriceSaleLivePayload, got %T", p)
        }
    case ResourceOccurrenceResourceTypeAuction:
        if _, ok := p.(AuctionLivePayload); !ok {
            return fmt.Errorf("chat: LIVE auction requires AuctionLivePayload, got %T", p)
        }
    }
    return nil
}

func validateFallbackPayloadType(rt ResourceOccurrenceResourceType, p ResourceProjectionPayload) error {
    switch rt {
    case ResourceOccurrenceResourceTypeFixedPriceSale:
        if _, ok := p.(FixedPriceSaleFallbackPayload); !ok {
            return fmt.Errorf("chat: FALLBACK FPS requires FixedPriceSaleFallbackPayload, got %T", p)
        }
    case ResourceOccurrenceResourceTypeAuction:
        if _, ok := p.(AuctionFallbackPayload); !ok {
            return fmt.Errorf("chat: FALLBACK auction requires AuctionFallbackPayload, got %T", p)
        }
    default:
        return fmt.Errorf("chat: FALLBACK_ALLOWED not valid for resource type %s", rt)
    }
    return nil
}

func validateViewerCapabilities(state ProjectionState, caps ProjectionViewerCapabilities) error {
    switch state {
    case ProjectionStateLive:
        if !caps.CanView {
            return fmt.Errorf("chat: LIVE projection requires can_view=true")
        }
    case ProjectionStateFallbackAllowed:
        if !caps.CanView {
            return fmt.Errorf("chat: FALLBACK_ALLOWED projection requires can_view=true")
        }
        if caps.CanInteract {
            return fmt.Errorf("chat: FALLBACK_ALLOWED projection requires can_interact=false")
        }
        if caps.BlockedByTombstone {
            return fmt.Errorf("chat: FALLBACK_ALLOWED projection requires blocked_by_tombstone=false")
        }
    case ProjectionStateTombstone:
        if caps.CanView {
            return fmt.Errorf("chat: TOMBSTONE projection requires can_view=false")
        }
        if caps.CanInteract {
            return fmt.Errorf("chat: TOMBSTONE projection requires can_interact=false")
        }
        if !caps.BlockedByTombstone {
            return fmt.Errorf("chat: TOMBSTONE projection requires blocked_by_tombstone=true")
        }
    }
    return nil
}
```

### Negative test matrix (for 3D-2a implementation)

| Test | Input | Expected |
|---|---|---|
| Invalid state | `State: "UNKNOWN"` | error: "invalid projection state" |
| Invalid resource type | `ResourceType: "order"` | error: "invalid resource type" |
| LIVE nil payload | State=LIVE, Payload=nil | error: "LIVE requires non-nil payload" |
| LIVE wrong payload type | State=LIVE, rt=Profile, Payload=AuctionLivePayload{} | error: "LIVE profile requires ProfileLivePayload" |
| FALLBACK nil payload | State=FALLBACK_ALLOWED, Payload=nil | error: "FALLBACK_ALLOWED requires non-nil payload" |
| FALLBACK profile | State=FALLBACK_ALLOWED, rt=Profile | error: "FALLBACK_ALLOWED not valid for resource type profile" |
| FALLBACK content | State=FALLBACK_ALLOWED, rt=Content | error: "FALLBACK_ALLOWED not valid for resource type content" |
| FALLBACK wrong payload | State=FALLBACK_ALLOWED, rt=FPS, Payload=AuctionFallbackPayload{} | error: "FALLBACK FPS requires FixedPriceSaleFallbackPayload" |
| TOMBSTONE non-nil payload | State=TOMBSTONE, Payload=ProfileLivePayload{} | error: "TOMBSTONE requires nil payload" |
| LIVE nil UUID | State=LIVE, ResourceID=uuid.Nil | error: "LIVE projection requires non-nil resource ID" |
| FALLBACK nil UUID | State=FALLBACK_ALLOWED, ResourceID=uuid.Nil | error: "FALLBACK_ALLOWED projection requires non-nil resource ID" |
| LIVE can_view=false | State=LIVE, CanView=false | error: "LIVE projection requires can_view=true" |
| TOMBSTONE can_view=true | State=TOMBSTONE, CanView=true | error: "TOMBSTONE projection requires can_view=false" |
| TOMBSTONE blocked_by_tombstone=false | State=TOMBSTONE, BlockedByTombstone=false | error: "TOMBSTONE projection requires blocked_by_tombstone=true" |
| Unknown payload type (defensive) | LIVE, rt=FPS, Payload=someUnknownType{} | error: "LIVE FPS requires FixedPriceSaleLivePayload" |

---

## 3. FINAL TYPED IDENTITY

### Internal (Go)

```go
// ResourceProjectionIdentity is the typed internal identity for a projection.
// CanonicalURL is NOT stored — derived via CanonicalResourceURL().
type ResourceProjectionIdentity struct {
    ResourceType chatEntity.ResourceOccurrenceResourceType
    ResourceID   uuid.UUID
}
```

No `CanonicalURL` field. URL is derived, not stored.

### Constructor (for LIVE and FALLBACK_ALLOWED)

```go
func NewResourceProjectionIdentity(
    rt chatEntity.ResourceOccurrenceResourceType,
    id uuid.UUID,
) (ResourceProjectionIdentity, error) {
    if id == uuid.Nil {
        return ResourceProjectionIdentity{}, fmt.Errorf("chat: identity requires non-nil resource ID")
    }
    if !rt.IsValid() {
        return ResourceProjectionIdentity{}, fmt.Errorf("chat: invalid resource type %q", rt)
    }
    return ResourceProjectionIdentity{ResourceType: rt, ResourceID: id}, nil
}
```

### TOMBSTONE identity (resource_type only)

```go
func TombstoneIdentity(rt chatEntity.ResourceOccurrenceResourceType) ResourceProjectionIdentity {
    return ResourceProjectionIdentity{ResourceType: rt} // ResourceID is uuid.Nil
}
```

---

## 4. CANONICAL URL DERIVATION

### Pure function

```go
// CanonicalResourceURL derives the canonical URL path from typed identity.
// Returns error for unknown resource types or nil UUID.
// TOMBSTONE callers must NOT call this — the URL is omitted from serialization.
func CanonicalResourceURL(rt chatEntity.ResourceOccurrenceResourceType, id uuid.UUID) (string, error) {
    if id == uuid.Nil {
        return "", fmt.Errorf("chat: cannot derive URL for nil resource ID")
    }
    switch rt {
    case chatEntity.ResourceOccurrenceResourceTypeProfile:
        return "/user/" + id.String(), nil
    case chatEntity.ResourceOccurrenceResourceTypeContent:
        return "/content/" + id.String(), nil
    case chatEntity.ResourceOccurrenceResourceTypeFixedPriceSale:
        return "/listing/" + id.String(), nil
    case chatEntity.ResourceOccurrenceResourceTypeAuction:
        return "/auction/" + id.String(), nil
    default:
        return "", fmt.Errorf("chat: unknown resource type %q", rt)
    }
}
```

Path-only, not absolute URL. Client prefixes base.

---

## 5. FINAL TYPED PAYLOAD UNION

### Interface

```go
// ResourceProjectionPayload is the sealed interface for typed projection
// payloads. Only the concrete types defined in this package implement it.
type ResourceProjectionPayload interface {
    resourceProjectionPayloadMarker()
}
```

### Concrete variants (7 total)

```go
// LIVE payloads (4)
type ProfileLivePayload struct { ... }
func (ProfileLivePayload) resourceProjectionPayloadMarker() {}

type ContentLivePayload struct { ... }
func (ContentLivePayload) resourceProjectionPayloadMarker() {}

type FixedPriceSaleLivePayload struct { ... }
func (FixedPriceSaleLivePayload) resourceProjectionPayloadMarker() {}

type AuctionLivePayload struct { ... }
func (AuctionLivePayload) resourceProjectionPayloadMarker() {}

// FALLBACK_ALLOWED payloads (2)
type FixedPriceSaleFallbackPayload struct { ... }
func (FixedPriceSaleFallbackPayload) resourceProjectionPayloadMarker() {}

type AuctionFallbackPayload struct { ... }
func (AuctionFallbackPayload) resourceProjectionPayloadMarker() {}

// TOMBSTONE: nil payload (no concrete type needed)
```

### Valid state × type × payload matrix

| State | ResourceType | Payload Type |
|---|---|---|
| LIVE | profile | `ProfileLivePayload` |
| LIVE | content | `ContentLivePayload` |
| LIVE | fixed_price_sale | `FixedPriceSaleLivePayload` |
| LIVE | auction | `AuctionLivePayload` |
| FALLBACK_ALLOWED | fixed_price_sale | `FixedPriceSaleFallbackPayload` |
| FALLBACK_ALLOWED | auction | `AuctionFallbackPayload` |
| TOMBSTONE | any | nil |

All other combinations → `Validate()` error.

---

## 6. FINAL ACTION CAPABILITY REPRESENTATION

### Commerce action capabilities — FPS/Auction only

```go
// CommerceActionCapabilities mirrors commerce/shared.ViewerCapabilities.
// Populated ONLY for FPS and Auction resource types. Nil for Profile/Content.
type CommerceActionCapabilities struct {
    Role         string `json:"role,omitempty"`
    CanChat      bool   `json:"can_chat"`
    CanNegotiate bool   `json:"can_negotiate"`
    CanBuy       bool   `json:"can_buy"`
    CanBid       bool   `json:"can_bid"`
    CanManage    bool   `json:"can_manage"`
}
```

### ResourceProjection action field

```go
type ResourceProjection struct {
    // ...
    // CommerceActions is the resource-specific action capability block.
    // Populated for FPS and Auction (from canonical commerce authority,
    // intersected with Chat access gates).
    // Omitted (nil) for Profile and Content — no canonical action
    // capability authority exists for those types.
    CommerceActions *CommerceActionCapabilities `json:"commerce_actions,omitempty"`
    // ...
}
```

### Why nil for Profile/Content

Profile and Content have NO existing canonical action capability projections in
production code:
- `GetPublicUser` emits no capability flags
- `GetContent` emits `IsLiked`/`IsSaved` but no `can_manage`/`can_chat` capability
- The commerce `ViewerCapabilities` struct is specific to FPS/Auction detail surfaces

Manufacturing `can_manage` from `viewer==author` or `can_chat` from block check
would invent new authority that doesn't exist in the current codebase. Chat
projection must not create new business rules.

### FPS commerce actions derivation

| Field | Value |
|---|---|
| `role` | `"owner"` if viewer==seller; `"buyer"` if viewer≠nil and not blocked and seller active; else `""` |
| `can_chat` | viewer active AND not blocked AND sellerTrustActive |
| `can_negotiate` | can_chat AND `fps.negotiation_enabled` AND `fps.IsAvailable()` |
| `can_buy` | can_chat AND `fps.IsAvailable()` |
| `can_bid` | `false` (not applicable to FPS) |
| `can_manage` | viewer==seller OR isAdmin |

### Auction commerce actions derivation

| Field | Value |
|---|---|
| `role` | same as FPS |
| `can_chat` | same as FPS |
| `can_negotiate` | `false` (not applicable to Auction) |
| `can_buy` | `false` |
| `can_bid` | can_chat AND `auction.Status == active` |
| `can_manage` | viewer==seller OR isAdmin |

---

## 7. EXACT CAN_INTERACT INVARIANT

`can_interact` lives in `ProjectionViewerCapabilities` (generic, projection-state-level).
It has one precise meaning:

> The viewer may perform at least one resource-appropriate action beyond passive viewing.

Specifically:

| State | `can_interact` | Meaning |
|---|---|---|
| LIVE, FPS active + seller active + not blocked | true | Viewer can buy/negotiate |
| LIVE, Auction active + seller active + not blocked | true | Viewer can bid |
| LIVE, FPS active + seller inactive | false | Viewer can view but not act |
| LIVE, Profile | false | No resource-appropriate interactive action exists for profiles |
| LIVE, Content | false | No resource-appropriate interactive action exists for content |
| FALLBACK_ALLOWED | false | Per 3B2 lock: "can_interact = false" |
| TOMBSTONE | false | No action possible |

**Invariant**: `can_interact=true` implies `CommerceActions` is non-nil and at least
one action flag (`can_buy`, `can_bid`, `can_negotiate`) is true. The reverse is not
required — `can_interact=false` may coexist with `CommerceActions` being present
(e.g., FPS sold → `can_interact=false`, `can_manage=true` for owner).

This invariant is enforced in `Validate()`:

```go
if caps.CanInteract && p.CommerceActions == nil {
    return fmt.Errorf("chat: can_interact=true requires commerce_actions")
}
if caps.CanInteract && !(p.CommerceActions.CanBuy || p.CommerceActions.CanBid || p.CommerceActions.CanNegotiate) {
    return fmt.Errorf("chat: can_interact=true requires at least one actionable commerce flag")
}
```

---

## 8. FINAL TOMBSTONE SERIALIZATION

### JSON

```json
{
  "state": "TOMBSTONE",
  "resource_type": "content",
  "viewer_capabilities": {
    "can_view": false,
    "can_interact": false,
    "blocked_by_tombstone": true
  }
}
```

### Fields ABSENT (not uuid.Nil, not empty string)

- `resource_id`
- `canonical_url`
- `commerce_actions`
- `profile` / `content` / `fixed_price_sale` / `auction` (no payload key)

### MarshalJSON path for TOMBSTONE

```go
func marshalTombstoneJSON(p ResourceProjection) ([]byte, error) {
    aux := struct {
        State              string                        `json:"state"`
        ResourceType       string                        `json:"resource_type"`
        ViewerCapabilities ProjectionViewerCapabilities  `json:"viewer_capabilities"`
    }{
        State:              string(p.State),
        ResourceType:       string(p.Identity.ResourceType),
        ViewerCapabilities: p.ViewerCapabilities,
    }
    return json.Marshal(aux)
}
```

No `resource_id`, no `canonical_url`, no payload, no `commerce_actions`. The
auxiliary struct enforces this structurally — those fields cannot appear because
the struct does not have them.

---

## 9. FINAL LIVE SERIALIZATION

### MarshalJSON path for LIVE

```go
func marshalLiveJSON(p ResourceProjection) ([]byte, error) {
    url, err := CanonicalResourceURL(p.Identity.ResourceType, p.Identity.ResourceID)
    if err != nil {
        return nil, fmt.Errorf("chat: LIVE marshal: %w", err)
    }

    aux := struct {
        State              string                        `json:"state"`
        ResourceType       string                        `json:"resource_type"`
        ResourceID         string                        `json:"resource_id"`
        CanonicalURL       string                        `json:"canonical_url"`
        ViewerCapabilities ProjectionViewerCapabilities  `json:"viewer_capabilities"`
        CommerceActions    *CommerceActionCapabilities   `json:"commerce_actions,omitempty"`
        Profile            *ProfileLivePayload           `json:"profile,omitempty"`
        Content            *ContentLivePayload           `json:"content,omitempty"`
        FixedPriceSale     *FixedPriceSaleLivePayload    `json:"fixed_price_sale,omitempty"`
        Auction            *AuctionLivePayload           `json:"auction,omitempty"`
    }{
        State:              string(p.State),
        ResourceType:       string(p.Identity.ResourceType),
        ResourceID:         p.Identity.ResourceID.String(),
        CanonicalURL:       url,
        ViewerCapabilities: p.ViewerCapabilities,
        CommerceActions:    p.CommerceActions,
    }
    // Exactly one payload key populated (guaranteed by Validate).
    switch payload := p.Payload.(type) {
    case ProfileLivePayload:
        aux.Profile = &payload
    case ContentLivePayload:
        aux.Content = &payload
    case FixedPriceSaleLivePayload:
        aux.FixedPriceSale = &payload
    case AuctionLivePayload:
        aux.Auction = &payload
    }
    return json.Marshal(aux)
}
```

Four nullable payload pointers on the auxiliary struct — but `Validate()` guarantees
exactly one is non-nil for the correct resource type. The struct uses `omitempty`
so only the populated key appears in JSON.

---

## 10. FINAL FALLBACK_ALLOWED SERIALIZATION

### MarshalJSON path for FALLBACK_ALLOWED

```go
func marshalFallbackJSON(p ResourceProjection) ([]byte, error) {
    url, err := CanonicalResourceURL(p.Identity.ResourceType, p.Identity.ResourceID)
    if err != nil {
        return nil, fmt.Errorf("chat: FALLBACK_ALLOWED marshal: %w", err)
    }

    aux := struct {
        State              string                        `json:"state"`
        ResourceType       string                        `json:"resource_type"`
        ResourceID         string                        `json:"resource_id"`
        CanonicalURL       string                        `json:"canonical_url"`
        ViewerCapabilities ProjectionViewerCapabilities  `json:"viewer_capabilities"`
        FixedPriceSale     *FixedPriceSaleFallbackPayload `json:"fixed_price_sale,omitempty"`
        Auction            *AuctionFallbackPayload        `json:"auction,omitempty"`
    }{
        State:              string(p.State),
        ResourceType:       string(p.Identity.ResourceType),
        ResourceID:         p.Identity.ResourceID.String(),
        CanonicalURL:       url,
        ViewerCapabilities: p.ViewerCapabilities,
    }
    switch payload := p.Payload.(type) {
    case FixedPriceSaleFallbackPayload:
        aux.FixedPriceSale = &payload
    case AuctionFallbackPayload:
        aux.Auction = &payload
    }
    return json.Marshal(aux)
}
```

No `commerce_actions` for FALLBACK_ALLOWED — all actions are false per 3B2 lock
(`can_interact=false`).

### JSON

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
  "fixed_price_sale": {
    "title": "Premium Product",
    "image_url": "https://cdn.example.com/product.jpg",
    "seller_store_name": "Toko Maju Jaya"
  }
}
```

---

## 11. INVALID-COMBINATION BEHAVIOR

### MarshalJSON — top-level dispatch with validation

```go
func (p ResourceProjection) MarshalJSON() ([]byte, error) {
    if err := p.Validate(); err != nil {
        return nil, err
    }
    switch p.State {
    case ProjectionStateLive:
        return marshalLiveJSON(p)
    case ProjectionStateFallbackAllowed:
        return marshalFallbackJSON(p)
    case ProjectionStateTombstone:
        return marshalTombstoneJSON(p)
    default:
        return nil, fmt.Errorf("chat: unreachable: Validate should have caught invalid state %q", p.State)
    }
}
```

### Constructor conventions

```go
// NewLiveProjection builds a LIVE projection. Returns error on invalid inputs.
func NewLiveProjection(
    rt chatEntity.ResourceOccurrenceResourceType,
    id uuid.UUID,
    payload ResourceProjectionPayload,
    viewerCaps ProjectionViewerCapabilities,
    commerceActions *CommerceActionCapabilities,
) (ResourceProjection, error) {
    identity, err := NewResourceProjectionIdentity(rt, id)
    if err != nil {
        return ResourceProjection{}, err
    }
    p := ResourceProjection{
        State:              ProjectionStateLive,
        Identity:           identity,
        ViewerCapabilities: viewerCaps,
        CommerceActions:    commerceActions,
        Payload:            payload,
    }
    if err := p.Validate(); err != nil {
        return ResourceProjection{}, err
    }
    return p, nil
}

// NewTombstoneProjection builds a TOMBSTONE projection.
func NewTombstoneProjection(rt chatEntity.ResourceOccurrenceResourceType) ResourceProjection {
    return ResourceProjection{
        State:    ProjectionStateTombstone,
        Identity: TombstoneIdentity(rt),
        ViewerCapabilities: ProjectionViewerCapabilities{
            CanView: false, CanInteract: false, BlockedByTombstone: true,
        },
        CommerceActions: nil,
        Payload:         nil,
    }
}
```

### Every invalid path is a Go error

No silent default. No `fmt.Printf` warning. No "should be unreachable" comment.
Every invalid combination returns a non-nil `error` from `Validate()` or
`MarshalJSON()`.

---

## 12. TYPED NESTED IDENTITY

### Internal (Go)

```go
type NestedResourceIndicator struct {
    ResourceType chatEntity.ResourceOccurrenceResourceType
    ResourceID   uuid.UUID
}
```

### JSON serialization

```go
func (n NestedResourceIndicator) MarshalJSON() ([]byte, error) {
    if !n.ResourceType.IsValid() {
        return nil, fmt.Errorf("chat: nested indicator has invalid resource type %q", n.ResourceType)
    }
    if n.ResourceID == uuid.Nil {
        return nil, fmt.Errorf("chat: nested indicator has nil resource ID")
    }
    return json.Marshal(struct {
        ResourceType string `json:"resource_type"`
        ResourceID   string `json:"resource_id"`
    }{
        ResourceType: string(n.ResourceType),
        ResourceID:   n.ResourceID.String(),
    })
}
```

### Depth enforcement

The `NestedResourceIndicator` has NO `NestedResource` field. Depth-1 is
structurally enforced — there is no place to put a recursive reference.

---

## 13. FALLBACK PARSER DISPOSITION

### Types: YES

`FixedPriceSaleFallbackPayload` and `AuctionFallbackPayload` remain defined as
Go types. They complete the payload union so the `ResourceProjectionPayload`
interface covers all valid state × type combinations.

### Parser: NO

No standalone `ParseFallbackSnapshot` function is implemented in 3D-2a or any
immediate slice. The fallback payload types exist but are never constructed
from `fallback_snapshot` JSON until a concrete reachable fallback trigger is
proven.

### Slice 3D-2b: REMOVED

The previously planned "Fallback Typed Parsers" slice is removed from the
implementation plan. Strict parsing and runtime fallback wiring belong to the
future deferred FALLBACK_ALLOWED slice.

---

## 14. CORRECTED SCHEMA-AUTHORITY STATEMENT

### For 3D-2 type design

Only schema facts needed by payload types are retained. The following are the
verified columns used by Chat projection types:

| Table | Columns used |
|---|---|
| `users` | `id`, `account_status`, `deleted_at` |
| `user_profiles` | `user_id`, `username`, `avatar_url` |
| `seller_profiles` | `user_id`, `store_name`, `status` |
| `seller_subscriptions` | `user_id`, `status`, `started_at`, `expires_at` |
| `contents` | `id`, `author_id`, `status`, `visibility`, `is_hidden`, `caption`, `created_at`, `deleted_at`, `share_reference` |
| `content_media` | `content_id`, `media_url`, `position` |
| `fixed_price_sales` | `id`, `product_id`, `seller_id`, `price_per_unit`, `negotiation_enabled`, `status`, `quantity_available` |
| `products` | `id`, `title`, `media_urls` |
| `auctions` | `id`, `seller_id`, `product_id`, `title`, `status`, `current_bid`, `buy_now_price`, `end_at` |
| `user_blocks` | `blocker_id`, `blocked_id` |
| `user_follows` | `follower_id`, `following_id` |

### Verification rule for implementation slices

Before each resolver slice (3D-3 through 3D-6), verify the relevant columns
against the fully-migrated `labuda_test` database. Do not rely solely on
migration files — columns may have been added, dropped, or renamed by later
migrations.

### Dropped columns NOT listed

`auctions.listing_id` was removed by a later migration and is NOT listed above.
Only columns verified present in the current fully-migrated schema are used.

---

## 15. FINAL IMPLEMENTATION ORDER

| Slice | Name | Scope |
|---|---|---|
| **3D-2a** | Projection Types + Strict Envelope | All Go types, `Validate()`, state-specific `marshal*JSON`, constructors, typed payloads, capabilities. Pure Go, zero DB. Fallback payload TYPES included; NO fallback parser. ~300 LOC + tests. |
| **3D-3** | Resolver Interface + Profile | `ResourceProjectionResolver` interface. Profile batch-load + block + lifecycle + tombstone in serverboot. Real PostgreSQL tests. ~400 LOC. |
| **3D-4** | Content + Depth-1 | Content batch-load + visibility + `ShareReference` nested indicator. ~350 LOC. |
| **3D-5** | FPS + Commerce Actions | FPS batch-load + seller identity + `CommerceActionCapabilities`. ~300 LOC. |
| **3D-6** | Auction + Commerce Actions | Auction batch-load + seller identity + `CommerceActionCapabilities`. ~300 LOC. |
| **3D-7** | HTTP Response Wiring | Add `resource_projection` to `messageToResponse`. ~100 LOC. |
| **3D-8** | Legacy Hard Purge | Remove `type=reference` from attachment validation. Requires mobile switch. |
| **(deferred)** | FALLBACK_ALLOWED Runtime | Strict parsing + fallback wiring. Only after a concrete reachable trigger is proven. |

---

## 16. BLOCKERS

### No blockers

All design decisions are resolved. No owner questions required.

### Summary of deferred items

| Item | Status |
|---|---|
| FALLBACK_ALLOWED runtime path | Contract types defined; runtime deferred until reachable trigger proven |
| Fallback parser (`ParseFallbackSnapshot`) | Deferred with FALLBACK_ALLOWED slice |
| `publiccard` type reuse | Explicitly not reused — Chat uses flat inline shapes; same canonical DB sources |

---

## 17. RECOMMENDATION

**Start with Slice 3D-2a (Projection Types + Strict Envelope).**

~300 lines of pure Go:
- `ResourceProjection` struct + `Validate()` + `MarshalJSON`
- Three state-specific `marshal*JSON` functions (LIVE, FALLBACK, TOMBSTONE)
- `ResourceProjectionIdentity` + `NewResourceProjectionIdentity` + `TombstoneIdentity`
- `CanonicalResourceURL` pure function
- `ProjectionViewerCapabilities` + `validateViewerCapabilities`
- `CommerceActionCapabilities`
- `ResourceProjectionPayload` interface + 7 concrete types
- `NestedResourceIndicator` + `MarshalJSON`
- Constructors: `NewLiveProjection`, `NewTombstoneProjection`
- 15 negative tests for `Validate()` (all invalid combinations → error)
- Positive tests for correct JSON output of each state × type combination
- Round-trip test: `NewLiveProjection` → `MarshalJSON` → verify JSON shape

Zero DB dependencies. Establishes the immutable typed contract with strict
invariants for all subsequent slices.

---

**STOP.** Design locked. No implementation.
