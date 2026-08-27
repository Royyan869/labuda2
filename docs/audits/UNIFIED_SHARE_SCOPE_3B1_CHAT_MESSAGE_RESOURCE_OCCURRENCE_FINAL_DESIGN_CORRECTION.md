# UNIFIED SHARE SCOPE 3B1 — CHAT MESSAGE RESOURCE OCCURRENCE FINAL DESIGN

**Scope ID:** `UNIFIED_SHARE_SCOPE_3B1`
**Date:** 2026-08-08
**Mode:** DESIGN CORRECTION — NO IMPLEMENTATION
**Supersedes:** Scope 3B design document

---

## 1. VERDICT

**READY_FOR_SCOPE_3_IMPLEMENTATION**

All design corrections applied. One owner decision remains (Content nesting depth).

---

## 2. CORRECTED OCCURRENCE SCHEMA

```sql
CREATE TYPE chat_resource_occurrence_operation_enum AS ENUM (
    'share_to_chat',
    'direct_commerce_insert_chat'
);

CREATE TABLE chat_message_resource_occurrences (
    -- message_id IS the primary key: at most one occurrence per message.
    -- room_id and actor_id are derived from chat_messages (sender_id, room_id).
    message_id uuid PRIMARY KEY REFERENCES chat_messages(id) ON DELETE CASCADE,

    -- Operation discriminator
    operation chat_resource_occurrence_operation_enum NOT NULL,

    -- Exactly one typed source FK. The non-null FK IS the type authority —
    -- no separate source_type column.
    profile_source_id           uuid REFERENCES users(id) ON DELETE RESTRICT,
    content_source_id           uuid REFERENCES contents(id) ON DELETE RESTRICT,
    fixed_price_sale_source_id  uuid REFERENCES fixed_price_sales(id) ON DELETE RESTRICT,
    auction_source_id           uuid REFERENCES auctions(id) ON DELETE RESTRICT,

    -- Server-built immutable display fallback
    fallback_snapshot jsonb NOT NULL,

    created_at timestamp with time zone NOT NULL DEFAULT now(),

    -- Exactly-one-source invariant
    CONSTRAINT exactly_one_source CHECK (
        (CASE WHEN profile_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN content_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN fixed_price_sale_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN auction_source_id IS NOT NULL THEN 1 ELSE 0 END) = 1
    ),

    -- Operation/resource-type compatibility:
    -- direct_commerce_insert_chat permits FPS and Auction only
    CONSTRAINT valid_operation_for_resource CHECK (
        (operation = 'direct_commerce_insert_chat' AND
            (fixed_price_sale_source_id IS NOT NULL OR auction_source_id IS NOT NULL))
        OR
        (operation = 'share_to_chat')
    )
);

-- Index per source type for lifecycle/status lookups
CREATE INDEX idx_chat_message_resource_occurrences_profile_source
    ON chat_message_resource_occurrences (profile_source_id) WHERE profile_source_id IS NOT NULL;
CREATE INDEX idx_chat_message_resource_occurrences_content_source
    ON chat_message_resource_occurrences (content_source_id) WHERE content_source_id IS NOT NULL;
CREATE INDEX idx_chat_message_resource_occurrences_fps_source
    ON chat_message_resource_occurrences (fixed_price_sale_source_id) WHERE fixed_price_sale_source_id IS NOT NULL;
CREATE INDEX idx_chat_message_resource_occurrences_auction_source
    ON chat_message_resource_occurrences (auction_source_id) WHERE auction_source_id IS NOT NULL;
```

### FK Delete Semantics

| FK | ON DELETE | Rationale |
|---|---|---|
| `message_id → chat_messages(id)` | CASCADE | Message deletion naturally removes its occurrence |
| `profile_source_id → users(id)` | RESTRICT | Physical user deletion must explicitly handle occurrences |
| `content_source_id → contents(id)` | RESTRICT | Content deletion is soft (lifecycle state); physical deletion exceptional |
| `fixed_price_sale_source_id → fixed_price_sales(id)` | RESTRICT | FPS deletion is soft; physical deletion exceptional |
| `auction_source_id → auctions(id)` | RESTRICT | Auction deletion is soft; physical deletion exceptional |

---

## 3. ACTOR AUTHORITY

`actor_id` is NOT stored in `chat_message_resource_occurrences`.

Canonical actor authority chain:

```
chat_message_resource_occurrences.message_id
  → chat_messages.sender_id    (actor who sent the message)
  → chat_messages.room_id      (room context)
```

At read time, the occurrence joins through `chat_messages` for both `sender_id` and `room_id`. No duplicated authority. No risk of occurrence.actor_id disagreeing with chat_messages.sender_id.

---

## 4. CORRECTED CONTENT MODEL

Content uses the **canonical Content entity** (`internal/social/content/entity/content.go`):

| Canonical Field | Type | Usage in Chat |
|---|---|---|
| `Caption` | `*string` | Display text (NOT "body") |
| `Status` | `Status` enum | Lifecycle: active, moderated (hidden), deleted |
| `Visibility` | `Visibility` enum | public, followers_only, private |
| `AuthorID` | `uuid.UUID` | Canonical author identity |
| `ShareReference` | `*ShareReference` | If this Content reposts another resource |
| `IsHidden` | `bool` | Moderation flag |
| Media | Via `content_media` join | Ordered typed media projection |

**Rejected from Chat Content model:**
- ❌ `content_type` enum (image|video|text) — no such discriminator in canonical Content
- ❌ `body` as Content field — canonical field is `Caption`
- ❌ `media_urls` string list — canonical media is typed ordered projection via `content_media`

**Chat Content fallback stores:**
- Caption excerpt (truncated)
- First canonical media preview URL
- Author historical display identity (username, avatar at occurrence time)
- Whether this Content carries a `ShareReference` (nested resource indicator)

**Chat Content live projection resolves:**
- Full caption
- Complete ordered typed media projection
- Live author identity with lifecycle
- Live visibility/status
- Whether nested resource is accessible to viewer

---

## 5. CONTENT NESTING DESIGN

**DEPTH 1 ONLY** — OWNER_DECISION_REQUIRED

If a shared Content itself carries a `ShareReference` (canonical repost), the Chat message shows:

1. The shared Content card (primary — exact Content sender selected)
2. A compact single-line nested resource indicator (e.g., type icon + title)

**Backend:** The Content's `ShareReference` is resolved at read time via the canonical Content entity. The occurrence fallback stores only `has_nested_resource: true` — no duplicated `nested_resource_type`/`nested_resource_id` in the fallback. The live projection resolves the nested resource identity from the Content's current `ShareReference`.

**Recursion protection:** Depth 1 means the nested resource is shown as compact metadata only — no further expansion. If Content A → Content B → Content A (cycle), depth-1 truncation prevents infinite recursion.

**Navigation:** Tapping the Content card opens `/content/{id}` (exact Content selected by sender). Tapping the nested indicator opens the nested resource's canonical URL — only if live viewer access permits it.

---

## 6. CORRECTED FALLBACK MODEL

All fallbacks are **server-built at write time, immutable, occurrence-specific.**

### Profile Fallback

```json
{
  "username": "string (historical)",
  "avatar_url": "string | null (historical)",
  "store_name": "string | null (historical, if seller at occurrence time)",
  "is_seller": "boolean (historical)"
}
```

Historical display identity only. No current lifecycle, no current verification status.

### Content Fallback

```json
{
  "caption_excerpt": "string (truncated)",
  "first_media_preview_url": "string | null",
  "author_username": "string (historical)",
  "author_avatar_url": "string | null (historical)",
  "has_nested_resource": "boolean"
}
```

No Content type discriminator. No nested resource identity duplication. No current visibility/status.

### FixedPriceSale Fallback

```json
{
  "title": "string (historical)",
  "image_url": "string | null (historical first product image)",
  "seller_store_name": "string (historical)",
  "seller_store_image": "string | null (historical)"
}
```

### Auction Fallback

```json
{
  "title": "string (historical)",
  "image_url": "string | null (historical first product image)",
  "seller_store_name": "string (historical)",
  "seller_store_image": "string | null (historical)"
}
```

### Explicitly NOT in fallback

| Rejected Field | Why |
|---|---|
| `price` / `current_bid` | Live authority — changes over time |
| `display_value` | Live authority |
| `status` / `phase` | Live authority |
| `quantity_available` | Live authority |
| `is_available` / `is_sold` / `is_closed` / `is_deleted` | Live authority |
| `seller_id` (uuid) | Resolved via live lookup through occurrence FK |
| `content_type` | Does not exist in canonical Content |
| `body` / `media_urls` | Not canonical Content fields |
| `nested_resource_type` / `nested_resource_id` | Duplicated authority — resolved from Content.ShareReference |

---

## 7. CORRECTED RESPONSE STATE MODEL

Server determines ONE canonical projection state per occurrence. Client renders the state supplied by server — no dual fallback+live as competing client authorities.

### States

```
LIVE
  Resource is accessible to viewer.
  → full live payload present
  → fallback present for historical context

FALLBACK_ALLOWED
  Resource is soft-deleted/ended/sold but still has legitimate
  public detail that may be shown.
  → fallback present
  → live payload absent
  → viewer_capabilities.can_interact = false

TOMBSTONE
  Resource is banned/deleted (privacy) OR viewer is blocked.
  → fallback suppressed
  → live payload absent
  → viewer_capabilities.blocked_by_tombstone = true
```

### Single Typed Envelope

```json
{
  "id": "uuid",
  "room_id": "uuid",
  "sender_id": "uuid",
  "message_type": "text",
  "body": "optional text",
  "created_at": "ISO8601",

  "resource_occurrence": {
    "operation": "share_to_chat",
    "resource_type": "profile",
    "resource_id": "uuid",
    "canonical_url": "/user/{id}",

    "state": "LIVE | FALLBACK_ALLOWED | TOMBSTONE",

    "fallback": { /* §6 — present when state != TOMBSTONE */ },
    "live": { /* resource-specific — present when state == LIVE */ },

    "viewer_capabilities": {
      "can_view": true,
      "can_interact": true | false,
      "blocked_by_tombstone": true | false
    }
  }
}
```

### Live Payloads (state == LIVE)

**Profile:**
```json
{
  "username": "string",
  "avatar_url": "string | null",
  "store_name": "string | null",
  "store_image": "string | null",
  "is_seller": false,
  "lifecycle": "active | suspended | removed | unavailable"
}
```

**Content:**
```json
{
  "caption": "string",
  "media": [{ "type": "image|video", "url": "string", "sort_order": 0 }],
  "author": { "id": "uuid", "username": "string", "avatar_url": "string | null", "lifecycle": "active | ..." },
  "visibility": "public | followers_only | private",
  "lifecycle": "active | moderated | deleted",
  "has_nested_resource": true,
  "nested_resource": { /* compact only — resolved from Content.ShareReference at read time */ }
}
```

**FixedPriceSale:**
```json
{
  "title": "string",
  "image_url": "string | null",
  "price": { "amount": "int64", "currency": "IDR" },
  "status": "active | sold | withdrawn",
  "seller": {
    "id": "uuid",
    "store_name": "string",
    "store_image": "string | null",
    "username": "string",
    "lifecycle": "active | suspended | removed"
  },
  "quantity_available": 0,
  "canonical_url": "/listing/{id}"
}
```

**Auction:**
```json
{
  "title": "string",
  "image_url": "string | null",
  "current_bid": { "amount": "int64", "currency": "IDR" },
  "phase": "scheduled | active | ended | cancelled | expired_bnr",
  "seller": {
    "id": "uuid",
    "store_name": "string",
    "store_image": "string | null",
    "username": "string",
    "lifecycle": "active | suspended | removed"
  },
  "canonical_url": "/auction/{id}"
}
```

### State Determination Rules

| Resource Condition | Viewer Condition | State |
|---|---|---|
| Active/accessible | Not blocked | LIVE |
| Sold/ended/withdrawn (commerce) | Not blocked | FALLBACK_ALLOWED |
| Soft-deleted (content/profile) | Not blocked | FALLBACK_ALLOWED (if public detail permitted) |
| Banned author/seller | Any | TOMBSTONE |
| Hard-deleted (privacy) | Any | TOMBSTONE |
| Any | Viewer blocked by resource owner | TOMBSTONE |
| Any | Viewer blocked resource owner | TOMBSTONE |

Viewer-specific block state is computed once per request (viewer vs. each resource owner), not per occurrence.

---

## 8. AUTHORIZATION / LIFECYCLE MATRIX

### Middleware (CONFIRMED)

`RequireActiveAccount` is a **two-gate** middleware:
1. Account status check (DB: `account_status` + `deleted_at`)
2. Email verification check (`email_verified_at IS NOT NULL`)

Both must pass. No separate `RequireEmailVerified` exists. "Active Account" in the matrix below implies both.

### SHARE_TO_CHAT

| Resource | Login | Active Account | Room Membership | Block Check | Resource Accessibility | Ownership | Market Authority |
|---|---|---|---|---|---|---|---|
| Profile | ✅ | ✅ | ✅ | ✅ sender→recipient | Profile accessible to actor | ❌ | ❌ |
| Content | ✅ | ✅ | ✅ | ✅ sender→recipient | Content visible to actor | ❌ | ❌ |
| FPS | ✅ | ✅ | ✅ | ✅ sender→recipient | Listing publicly accessible | ❌ | ❌ |
| Auction | ✅ | ✅ | ✅ | ✅ sender→recipient | Auction publicly accessible | ❌ | ❌ |

**Resource accessibility for SHARE_TO_CHAT:** The resource must be currently accessible to the sharing actor. A sold/ended listing that still has a legitimate public detail page may remain shareable. The authority is: "can the actor view this resource's public detail right now?" — NOT "is the resource in an active/promotable state?"

### DIRECT_COMMERCE_INSERT_CHAT

| Resource | Login | Active Account | Room Membership | Block Check | Ownership | Market Authority | Lifecycle Gate |
|---|---|---|---|---|---|---|---|
| FPS | ✅ | ✅ | ✅ | ✅ sender→recipient | ✅ actor = seller | ✅ `HasActiveSellerCapability` | `FixedPriceSaleStatus.IsRepostable()` → only `active` |
| Auction | ✅ | ✅ | ✅ | ✅ sender→recipient | ✅ actor = seller | ✅ `HasActiveSellerCapability` | `Status.IsRepostable()` → `scheduled` or `active` |

**Lifecycle gates use canonical entity methods:**
- FPS: `FixedPriceSaleStatus.IsRepostable()` — `active` only
- Auction: `Status.IsRepostable()` — `scheduled` or `active`

These are the SAME gates used by Content repost (`content_service.validateFixedPriceSaleTarget` / `validateAuctionTarget`). No new lifecycle rules.

### HTTP Error Mapping

| Condition | HTTP | Code |
|---|---|---|
| Not authenticated | 401 | UNAUTHORIZED |
| Account not active / email not verified | 403 | ACCOUNT_SUSPENDED / ACCOUNT_BANNED / ACCOUNT_REMOVED / EMAIL_VERIFICATION_REQUIRED |
| Not room participant | 403 | NOT_PARTICIPANT |
| Blocked by recipient | 403 | USER_BLOCKED |
| Resource not found | 404 | RESOURCE_NOT_FOUND |
| Resource not accessible (share) | 403 | RESOURCE_NOT_ACCESSIBLE |
| Not resource owner (insert) | 403 | NOT_RESOURCE_OWNER |
| No market authority (insert) | 403 | MARKET_AUTHORITY_REQUIRED |
| Resource lifecycle prevents insert | 400 | RESOURCE_NOT_PROMOTABLE |
| Idempotency conflict | 409 | IDEMPOTENCY_CONFLICT |

---

## 9. IDEMPOTENCY / REPLAY ORDER

Scope 3A authority is frozen. Corrected high-level order:

### SendMessage Transaction

```
1. authenticate + account gate (RequireActiveAccount middleware)
2. parse request — strict binding
3. Begin WithTx:
   a. load room, verify sender is participant
   b. block check (sender→recipient)
   c. if resource_occurrence present: validate operation/resource_type compatibility
   d. compute normalized command fingerprint (including occurrence)
   e. actor-scoped idempotency lookup (Scope 3A authority)
      ─ IF found AND fingerprint matches → return existing message
        (stable replay — does NOT re-run mutable resource/market authority)
      ─ IF found AND fingerprint mismatch → 409 ErrIdempotencyConflict
      ─ IF not found → continue
   f. validate reply target (if present)
   g. validate media assets (if present)
   h. if resource_occurrence present (new command):
      - resolve resource from live table
      - validate operation-specific authority:
        * SHARE_TO_CHAT: resource accessibility check
        * DIRECT_COMMERCE_INSERT: ownership + market authority + lifecycle gate
      - build server fallback snapshot
   i. create chat_messages row
   j. create chat_message_resource_occurrences row (if applicable)
   k. create media asset relations (if applicable)
   l. update room last_message_at
   m. upsert sender read state
   n. emit outbox events
4. Commit
```

### Key: Replay authorization

Steps 3a-3b (room membership + block check) run BEFORE idempotency lookup (3e). A replay must pass actor/room authorization — a blocked user replaying an old key from before the block must be rejected at the block check, not silently returned.

Steps 3f-3h (resource validation) run AFTER idempotency lookup and only for new commands. Replay does not re-validate mutable resource state — the original message+occurrence is returned as-is.

### Fingerprint Extension

`computeSendMessageCommandFingerprint` extended to include (when `resource_occurrence` present):
```go
h.Write([]byte(occurrence.Operation))
h.Write([]byte(occurrence.ResourceType))
h.Write([]byte(occurrence.ResourceID.String()))
```

---

## 10. CANONICAL SWITCH / PURGE SEQUENCE

**No deployed dual-write compatibility window.**

### Phase A: Additive (Migration 000034)

1. Create `chat_resource_occurrence_operation_enum`
2. Create `chat_message_resource_occurrences` table with all constraints
3. Backend: add `resource_occurrence` block to `SendMessageRequest`
4. Backend: add occurrence creation in `SendMessage` transaction
5. Backend: add occurrence projection to `messageToResponse` (state: LIVE/FALLBACK_ALLOWED/TOMBSTONE)
6. Backend: extend outbox/realtime envelopes
7. **No mobile changes yet**
8. Old `attachment_json.type=reference` path continues to work for existing clients

### Phase B: Mobile Canonical Switch

1. Mobile: new `resource_occurrence` producer path (replaces `attachment_json.type=reference`)
2. Mobile: read path consumes `resource_occurrence` envelope
3. Mobile: Profile/Content share activated (via new `resource_occurrence`, NOT `ShareReference.chatWireTargetType`)
4. Mobile: `ShareReference.chatWireTargetType` NOT extended to Profile/Content
5. Mobile: OBJECT_PREVIEW_CARD rendering switched to occurrence-based projection

### Phase C: Immediate Hard Purge

1. Backend: `attachment_json.type=reference` REJECTED in `ValidateAttachmentJSON`
2. Backend: `SendMessageRequest.AttachmentJSON` with type=reference → 400 with typed error
3. Backend: old reference validation/rendering code removed
4. Mobile: `ChatMapper.domainAttachmentToDto` no longer converts resource references
5. Mobile: `ObjectReference` / `ShareReference` chat transport helpers REMOVED
6. Mobile: `ShareReference.chatWireTargetType` REMOVED (not extended)
7. `attachment_dto` reference documentation REMOVED (not updated to list Profile/Content)
8. Dead code inventory deleted
9. Legacy tests converted to negative contracts

### NOT Purged

- `attachment_json` column — still used by negotiation/shipping/location
- `attachmentvalidator.ValidateAttachmentJSON` — still validates non-reference types
- `chat_commerce_references` — room-scoped, untouched

---

## 11. QUERY-COUNT PROOF MATRIX

Define full end-to-end query count per message page read.

### Query Classes

| # | Query | When |
|---|---|---|
| Q1 | Messages (cursor pagination, includes reply/media joins) | Always |
| Q2 | Occurrences batch (by message_id IN) | Always |
| Q3 | Profile live data (users + user_profiles + seller_profiles) | When profile occurrences present |
| Q4 | Content live data (contents + content_media + author) | When content occurrences present |
| Q5 | FPS live data (fixed_price_sales + products + product_media + seller) | When FPS occurrences present |
| Q6 | Auction live data (auctions + products + product_media + seller) | When auction occurrences present |
| Q7 | Viewer block state (viewer ↔ all resource owners) | When any occurrence present |

### Query-Count Matrix (per 50-message page)

| Scenario | Q1 | Q2 | Q3 | Q4 | Q5 | Q6 | Q7 | Total |
|---|---|---|---|---|---|---|---|---|
| No occurrences | 1 | 1 | — | — | — | — | — | **2** |
| 1 Profile | 1 | 1 | 1 | — | — | — | 1 | **4** |
| 20 Profile | 1 | 1 | 1 | — | — | — | 1 | **4** |
| 1 Content | 1 | 1 | — | 1 | — | — | 1 | **4** |
| 20 Content | 1 | 1 | — | 1 | — | — | 1 | **4** |
| 1 FPS | 1 | 1 | — | — | 1 | — | 1 | **4** |
| 20 FPS | 1 | 1 | — | — | 1 | — | 1 | **4** |
| 1 Auction | 1 | 1 | — | — | — | 1 | 1 | **4** |
| 20 Auction | 1 | 1 | — | — | — | 1 | 1 | **4** |
| 1 each mixed (4 types) | 1 | 1 | 1 | 1 | 1 | 1 | 1 | **7** |
| 20 each mixed (80 total) | 1 | 1 | 1 | 1 | 1 | 1 | 1 | **7** |

### Invariants

1. Query count is **constant within the same resource-type mixture** regardless of count (1 vs 20).
2. Maximum query count = **7** (all four resource types present).
3. Minimum query count = **2** (no occurrences).
4. **Zero per-message queries.**
5. **Zero client-side preview fetch.**

### Batch Resolution

All batch queries use `WHERE id = ANY($1)` with a uuid array. Occurrences are loaded once per page. Live resource data is loaded once per resource type per page. Seller identities are resolved within the commerce live-data queries (JOIN seller_profiles). Viewer block state is resolved once for all resource owners.

---

## 12. MOBILE DIRECT-INSERT FLOW

Locked business rule for DIRECT_COMMERCE_INSERT_CHAT FPS from Chat:

```
1. User taps "Bagikan Listing" in Chat composer
2. Listing picker opens (existing ListingPickerBottomSheet)
3. User may:
   a. Select existing listing → attached immediately
   b. Tap "Buat Listing Baru" → navigates to CreateListingScreen
      - Current Chat draft preserved (text, media drafts, reply target)
      - Create Listing flow opens (canonical, unchanged)
      - On success, returns canonical Listing/FPS ID
      - Listing attached to occurrence selector
4. Resource occurrence is SELECTED but NOT auto-sent
5. User reviews the composed message with attached occurrence
6. User taps Send (explicit)
7. SendMessage API call with resource_occurrence block
8. On failure: draft + occurrence selection preserved
   On success: message sent, occurrence created
```

### Required executable proofs:
- Draft preserved across Create Listing navigation and return
- Occurrence selection survives composer state changes
- No auto-send after listing creation
- Explicit Send required
- Failure preserves draft + selection
- Success clears draft + selection

---

## 13. CORRECTED DEAD-CODE DISPOSITION

Revalidated against current callers. Only Chat-specific dead/compat authority is deleted. Shared abstractions serving Feed/Content/Comment are NOT deleted.

### DELETE_DURING_SCOPE_3

| Item | Path | Verified |
|---|---|---|
| `ChatGateway` (copy 1) | `lib/shared/chat/chat_gateway.dart` | 0 implementers, 0 consumers |
| `ChatGateway` (copy 2) | `lib/domains/chat/chat_gateway.dart` | 0 implementers, 0 consumers |
| `LinkOrderToChatUseCase` | `lib/domains/commerce/transaction/usecases/link_order_to_chat_usecase.dart` | 0 importers |
| `GetFixedPriceSaleShareReferenceUseCase` (chat) | `lib/domains/chat/chat/domain/usecases/get_listing_share_reference_usecase.dart` | 0 importers, absent from barrel |
| `GetFixedPriceSaleShareReferenceUseCase` (catalog) | `lib/domains/commerce/catalog/usecases/get_listing_share_reference_usecase.dart` | 0 importers |
| `CreateChatDto` | `chat_dto.dart` | Orphaned |
| `ChatListDto` | `chat_dto.dart` | Bypassed by datasource |
| `ShareReference.chatWireTargetType` | `share_reference.dart` | REMOVED (not extended) |
| `ShareReference.asChatReference()` | `share_reference.dart` | REMOVED (not extended) |
| `attachment_dto` reference documentation | `attachment_dto.dart:54` | REMOVED (not updated) |
| Chat mapper legacy wire aliases | `chat_mapper.dart` | `objectReference` key, legacy format parsing, `offer_reference` fallback |
| `object_reference_bridge.dart` stale comment | `shared/object/` | Stale migration direction comment |
| `chat_entities.dart` legacy defaults | `chat_entities.dart` | Polluted compatibility maps, legacy payload defaults |
| Backend attachment validation nil-checker skip | `chat_service.go` | `validateAttachmentReferences` skip-if-nil |
| `ShareDestination.sendToChat` dead constant | `share_destination.dart` | **ACTIVATED** (becomes live with new occurrence path) |

### KEEP_CANONICAL (shared abstractions — NOT deleted)

| Item | Rationale |
|---|---|
| `ShareReference` (entity) | Used by Content repost, Feed, Comment — NOT chat-specific |
| `ObjectReference` / `ObjectPreviewCard` | Used by Feed, Content detail — NOT chat-specific; chat switches to occurrence-based rendering |
| `widget_lib.AttachmentWidget` | Still used by negotiation/shipping/location rendering |
| `ChatWebSocketHandler` | Verify liveness; may still serve legacy WS clients |

### SEPARATE_NON_RESOURCE_CHAT_SCOPE

| Item | Rationale |
|---|---|
| `ChatWebSocketHandler` removal | Separate scope if confirmed dead |
| `widget_lib.AttachmentWidget` chat migration | Separate scope for negotiation/shipping/location canonicalization |

---

## 14. REMAINING OWNER DECISIONS

### Q1: Content Nesting Depth

**OWNER_DECISION_REQUIRED**

Proposed: DEPTH 1 ONLY. Shared Content shows compact nested resource indicator (from `Content.ShareReference`), no recursive expansion. Nested indicator is tappable only if live viewer access permits.

This is the only remaining owner decision. All other design elements are resolved.

---

## 15. RECOMMENDATION

**READY_FOR_SCOPE_3_IMPLEMENTATION**

All design corrections applied:
- ✅ `actor_id` removed — derived from `chat_messages.sender_id`
- ✅ Content model uses canonical `Caption`/media projection/author — no `content_type`/`body`/`media_urls`
- ✅ Canonical URLs: `/user/{id}`, `/content/{id}`, `/listing/{id}`, `/auction/{id}`
- ✅ Fallback: historical display only — no current price/status/quantity/phase/sold/closed/available flags
- ✅ Response: server-determined LIVE/FALLBACK_ALLOWED/TOMBSTONE state — no dual client authority
- ✅ Share lifecycle: sold/ended may be shareable; Direct Insert uses `IsRepostable()` gates
- ✅ Idempotency: replay passes actor/room auth before lookup; mutable resource authority skipped on replay
- ✅ No legacy transport extension (`ShareReference.chatWireTargetType` REMOVED, not extended)
- ✅ Query-count matrix defined with invariants (2–7 queries, constant within same mixture)
- ✅ Mobile direct-insert flow locked with draft preservation
- ✅ Dead code revalidated against current callers
- ✅ `attachment_dto` reference documentation REMOVED, not updated
- ✅ Switch: additive → mobile → immediate purge (no dual-write window)

**Next migration:** 000034 (after 000032 + 000033)

**One owner decision pending:** Content nesting depth (§5)

---

*End of design correction. No implementation performed.*
