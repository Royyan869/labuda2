# UNIFIED SHARE SCOPE 3B — CHAT MESSAGE RESOURCE OCCURRENCE DESIGN RECONCILIATION

**Scope ID:** `UNIFIED_SHARE_SCOPE_3B_CHAT_MESSAGE_RESOURCE_OCCURRENCE_DESIGN_RECONCILIATION`
**Date:** 2026-08-08
**Mode:** DESIGN RECONCILIATION ONLY — NO IMPLEMENTATION
**Prerequisite:** Scope 3A Chat idempotency security CLOSED

---

## 1. VERDICT

**READY_FOR_SCOPE_3_IMPLEMENTATION**

All architectural decisions are resolved. One owner decision remains (Content nesting depth — §10). The design is fully specified, every FK, CHECK, validation rule, authorization gate, and switch step is explicit.

---

## 2. PROFILE CANONICAL FK AUTHORITY

**Finding:** The canonical identity authority is `users.id`.

Evidence:
- `users` table: `id uuid PK`, contains `account_status`, `deleted_at`, `email_verified_at` — the lifecycle authority
- `user_profiles` table: `user_id uuid FK → users(id)`, contains `username`, `avatar_url`, `bio` — the display authority
- `seller_profiles` table: `user_id uuid FK → users(id)`, contains `store_name`, `tier` — the seller authority
- Canonical public URL: `GET /profile/{id}` where `id` = `users.id` (`routes_core.go:73`)
- `publiccard.UserCard.ID` is `uuid.UUID` — stores `users.id`

**Design:** `chat_message_resource_occurrences.profile_source_id` → `users(id)`.

This is the same FK target used by every other domain (content author, comment author, chat participant, listing seller, auction seller, order buyer/seller, etc.). No separate `profiles` table with its own PK exists — `user_profiles` is a 1:1 display extension of `users`. The canonical public URL is `GET /users/{id}` (`routes_core.go:160`).

---

## 3. FINAL OCCURRENCE TABLE DESIGN

```sql
CREATE TYPE chat_resource_occurrence_operation_enum AS ENUM (
    'share_to_chat',
    'direct_commerce_insert_chat'
);

CREATE TABLE chat_message_resource_occurrences (
    -- message_id IS the primary key: at most one occurrence per message
    message_id uuid PRIMARY KEY REFERENCES chat_messages(id) ON DELETE CASCADE,

    -- operation discriminator
    operation chat_resource_occurrence_operation_enum NOT NULL,

    -- actor who performed the operation
    actor_id uuid NOT NULL REFERENCES users(id),

    -- Exactly one typed source FK (no separate source_type column —
    -- the non-null FK IS the type authority)
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

    -- Operation/resource-type compatibility
    CONSTRAINT valid_operation_for_resource CHECK (
        (operation = 'direct_commerce_insert_chat' AND
            (fixed_price_sale_source_id IS NOT NULL OR auction_source_id IS NOT NULL))
        OR
        (operation = 'share_to_chat')
    )
);

-- Index for actor-scoped queries
CREATE INDEX idx_chat_message_resource_occurrences_actor
    ON chat_message_resource_occurrences (actor_id, created_at DESC);

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

---

## 4. SOURCE-TYPE AUTHORITY DECISION

**Typed FK presence IS the physical source-type authority. No separate `source_type` enum column.**

Reasoning:
- The `exactly_one_source` CHECK already guarantees exactly one FK is non-null
- The non-null FK column identity unambiguously determines the source type
- A separate `source_type` column would be redundant dual authority requiring an additional CHECK to prevent disagreement
- At read time, the type is derived from which FK is populated — trivial and zero-cost

If a discriminator is ever needed in a query without joining, a generated column or VIEW can provide it. The storage authority is the FK set.

---

## 5. MESSAGE_ID / ROOM_ID DECISION

**`message_id` is the PRIMARY KEY.** No separate `id` column.

**`room_id` is NOT stored.** Rationale:
- `chat_messages.room_id` is the canonical room authority
- Occurrence always joins through `message_id` → `chat_messages.room_id`
- Storing `room_id` on the occurrence table would be duplicated authority
- No query pattern requires `room_id` without the message (message list queries join messages anyway)
- Adding `room_id` would require a DB constraint cross-referencing `chat_messages.room_id` — complex, fragile, and unnecessary

Read queries join: `chat_message_resource_occurrences` ← `chat_messages` (for room_id, sender_id, created_at).

---

## 6. FK DELETE SEMANTICS

| FK | ON DELETE | Rationale |
|---|---|---|
| `message_id → chat_messages(id)` | CASCADE | Message deletion naturally removes its occurrence |
| `profile_source_id → users(id)` | RESTRICT | Physical user deletion must explicitly handle occurrences |
| `content_source_id → contents(id)` | RESTRICT | Content deletion is soft (lifecycle state); physical deletion is exceptional |
| `fixed_price_sale_source_id → fixed_price_sales(id)` | RESTRICT | FPS deletion is soft; physical deletion is exceptional |
| `auction_source_id → auctions(id)` | RESTRICT | Auction deletion is soft; physical deletion is exceptional |

RESTRICT means: attempting to physically DELETE a resource row that has occurrences will fail with a FK violation. This is intentional — it forces explicit handling rather than silent cascade deletion of chat history.

Soft lifecycle changes (deleted_at, status changes, bans) do NOT trigger FK cascades. They are handled at read time via live authority checks (§11).

---

## 7. FALLBACK SNAPSHOT DESIGN

**All fallbacks are server-built at write time. Client submits only resource identity.**

### Profile Fallback

```json
{
  "username": "string",
  "avatar_url": "string | null",
  "store_name": "string | null (present only for seller profile)",
  "is_seller": "boolean"
}
```

Built from `users` + `user_profiles` + `seller_profiles` at occurrence creation time. Immutable snapshot — current username/avatar changes do NOT update existing occurrences.

### Content Fallback

```json
{
  "content_type": "string (image|video|text)",
  "preview_text": "string (truncated body/caption)",
  "preview_url": "string | null (thumbnail/first media)",
  "author_username": "string",
  "is_nested_resource": "boolean (whether this Content itself references another resource)"
}
```

If the Content references another resource (nested), the fallback includes `nested_resource_type` and `nested_resource_id` as compact metadata only — no recursive expansion (§10).

### FixedPriceSale Fallback

```json
{
  "title": "string",
  "image_url": "string | null",
  "display_value": "int64",
  "seller_id": "string (uuid)",
  "seller_name": "string (store_name)",
  "seller_store_image": "string | null"
}
```

Note: `is_available`, `is_sold`, `is_closed`, `is_deleted` are NOT stored in fallback — they are live authorities queried at read time.

### Auction Fallback

```json
{
  "title": "string",
  "image_url": "string | null",
  "display_value": "int64 (current bid or starting price at snapshot time)",
  "seller_id": "string (uuid)",
  "seller_name": "string (store_name)",
  "seller_store_image": "string | null"
}
```

Same principle: status/phase/current-bid are live authorities.

---

## 8. LIVE RESPONSE PROJECTION

### Single typed envelope per message

```json
{
  "id": "uuid",
  "room_id": "uuid",
  "sender_id": "uuid",
  "message_type": "text",
  "body": "optional text",
  "created_at": "ISO8601",

  "resource_occurrence": {
    "operation": "share_to_chat | direct_commerce_insert_chat",
    "resource_type": "profile | content | fixed_price_sale | auction",
    "resource_id": "uuid",
    "canonical_url": "/profile/{id} | /content/{id} | /listing/{id} | /auction/{id}",

    "fallback": { /* §7 fallback — present only when live authority allows */ },

    "live": {
      /* §8.1-8.4 below — present when resource is accessible to viewer */
    },

    "viewer_capabilities": {
      "can_view": true,
      "can_interact": true | false,
      "blocked_by_tombstone": true | false
    }
  }
}
```

### 8.1 Profile Live Payload

```json
{
  "username": "string",
  "avatar_url": "string | null",
  "store_name": "string | null (present if seller)",
  "store_image": "string | null",
  "is_seller": false,
  "lifecycle": "active | suspended | removed | unavailable"
}
```

### 8.2 Content Live Payload

```json
{
  "content_type": "image | video | text",
  "body": "string (full)",
  "media_urls": ["string"],
  "author": { "id": "uuid", "username": "string", "lifecycle": "active" },
  "nested_resource": { /* compact only — §10 */ },
  "lifecycle": "public | followers_only | moderated | deleted"
}
```

### 8.3 FixedPriceSale Live Payload

```json
{
  "title": "string",
  "image_url": "string | null",
  "price": { "amount": "int64", "currency": "IDR" },
  "status": "active | inactive | sold | deleted",
  "seller": {
    "id": "uuid",
    "store_name": "string",
    "store_image": "string | null",
    "username": "string",
    "lifecycle": "active | suspended | removed"
  },
  "quantity_available": "int",
  "navigation_url": "/listing/{id}"
}
```

### 8.4 Auction Live Payload

```json
{
  "title": "string",
  "image_url": "string | null",
  "current_bid": { "amount": "int64", "currency": "IDR" },
  "phase": "scheduled | active | ended | cancelled",
  "seller": {
    "id": "uuid",
    "store_name": "string",
    "store_image": "string | null",
    "username": "string",
    "lifecycle": "active | suspended | removed"
  },
  "navigation_url": "/auction/{id}"
}
```

### Suppression Rules

| Condition | `fallback` | `live` |
|---|---|---|
| Resource active + viewer allowed | Present | Present |
| Resource soft-deleted | Suppressed (privacy) | Absent |
| Resource moderated/hidden | Suppressed (privacy) | Absent |
| Author/seller banned | Suppressed (privacy) | Absent |
| Viewer blocked by author/seller | Viewer-specific tombstone | Absent |
| Viewer blocked author/seller | Viewer-specific tombstone | Absent |

---

## 9. PROFILE RENDERING RULE

**Self-share is allowed.** Sharing your own profile to a chat renders identically to someone else sharing your profile — the card represents the PROFILE, not the sharer.

**Ordinary user:** `username` + `avatar_url` from `user_profiles`.

**Seller:** `store_name` primary, `username` secondary, `store_image` primary, `personal_avatar` secondary. The canonical seller identity design in `user_profiles`/`seller_profiles` governs.

**Suspended:** Fallback suppressed, live shows `lifecycle: "suspended"` with redacted display fields.

**Banned/Deleted:** Fallback suppressed, live shows `lifecycle: "removed"` with redacted display fields, no username/avatar.

**Blocked viewer:** Tombstone — resource_occurrence shows `viewer_capabilities.blocked_by_tombstone: true`, no fallback, no live payload.

---

## 10. CONTENT NESTING PROPOSAL

**OWNER_DECISION_REQUIRED**

### Proposed: DEPTH 1 ONLY

A shared Content may itself reference another resource (e.g., a listing). The chat message shows:

1. The shared Content card (primary)
2. A compact single-line nested resource indicator below it (e.g., "📦 Listing: Judul Produk")

The nested resource renders as:
```json
"nested_resource": {
  "resource_type": "fixed_price_sale | auction | content | profile",
  "resource_id": "uuid",
  "title": "string (compact label only)",
  "canonical_url": "/listing/{id}"
}
```

No further expansion. Tapping the Content card opens the Content detail. Tapping the nested indicator opens the nested resource.

### Recursion / Cycle Protection

If Content A → Content B → Content A (cycle), depth-1 truncation prevents infinite recursion. Only the immediate nested resource identity is included, never expanded beyond compact metadata.

### Backend Projection

The occurrence fallback for Content includes `is_nested_resource: true` and `nested_resource_type`/`nested_resource_id` (compact metadata). The live projection resolves the nested resource's title/canonical_url at read time via batch lookup. No recursive projection.

---

## 11. FINAL WRITE API SHAPE

### Decision: ONE canonical SendMessage endpoint with optional `resource_occurrence` block.

**Rationale:** Both SHARE_TO_CHAT and DIRECT_COMMERCE_INSERT_CHAT create identical message + occurrence artifacts. Separate endpoints would duplicate validation, transaction, and outbox logic. The operation discriminator and authorization rules handle the difference.

### Request

```json
POST /api/v1/chat/rooms/{room_id}/messages

{
  "message_type": "text",
  "body": "Check this out!",
  "idempotency_key": "550e8400-e29b-41d4-a716-446655440000",

  "resource_occurrence": {
    "operation": "share_to_chat",
    "resource_type": "fixed_price_sale",
    "resource_id": "550e8400-e29b-41d4-a716-446655440001"
  },

  "reply_to_message_id": null,
  "media_asset_ids": [],
  "mentioned_user_ids": []
}
```

### Validation

| Field | Rule |
|---|---|
| `resource_occurrence` | Optional; if absent → normal message (no occurrence) |
| `resource_occurrence.operation` | Required if block present; `share_to_chat` or `direct_commerce_insert_chat` |
| `resource_occurrence.resource_type` | Required if block present; `profile` / `content` / `fixed_price_sale` / `auction` |
| `resource_occurrence.resource_id` | Required if block present; canonical non-nil UUID |
| Unknown fields in `resource_occurrence` | Rejected (strict schema) |
| `operation` = `direct_commerce_insert_chat` | `resource_type` must be `fixed_price_sale` or `auction` |
| `operation` = `share_to_chat` | `resource_type` may be any of the four |

### Rejected Fields (not in request)

- ❌ `preview` / `snapshot` — client does NOT supply fallback data
- ❌ `seller_id` / `seller_name` — server resolves from resource
- ❌ `title` / `image_url` — server resolves from resource
- ❌ `commerce_reference_id` — not a message concern
- ❌ `room_context_id` — not a message concern

---

## 12. CROSS-FIELD VALIDATION

### Resource occurrence with other message fields

| Combination | Allowed? | Rule |
|---|---|---|
| `resource_occurrence` + empty `body` | ✅ | Body is optional when a resource is present |
| `resource_occurrence` + `body` | ✅ | Body provides accompanying text |
| `resource_occurrence` + `media_asset_ids` | ✅ | Media is independent attachment functionality |
| `resource_occurrence` + `reply_to_message_id` | ✅ | Reply + resource share is valid |
| `resource_occurrence` + `attachment_json` (reference) | ❌ | **Hard rejection** — cannot use both old and new resource authority simultaneously; 400 |
| `resource_occurrence` + `attachment_json` (non-reference: negotiation/shipping/location) | ❌ | **Not in initial scope** — may be relaxed later if a legitimate use case exists; 400 for now |
| `resource_occurrence` with unknown fields | ❌ | 400 (strict binding) |

### Backward compatibility

The `attachment_json.type=reference` path continues to work for legacy clients until the switch+purge phase (§19). During the additive phase (migration deployed, backend supports both), `resource_occurrence` and `attachment_json.type=reference` are mutually exclusive within a single request — using both is 400.

---

## 13. AUTHORIZATION MATRIX

### Middleware Audit (CONFIRMED)

`RequireActiveAccount` (`middleware/active_account_middleware.go:23-53`) is a **two-gate middleware**:

**Gate 1 — Account status** (DB-backed): Queries `SELECT account_status, deleted_at FROM users WHERE id = $1`. Rejects suspended/banned/removed/inactive accounts with typed 403 codes (`ACCOUNT_SUSPENDED`, `ACCOUNT_BANNED`, `ACCOUNT_REMOVED`, `ACCOUNT_INACTIVE`). System caller UUID bypasses.

**Gate 2 — Email verification**: Reads `actor.EmailVerified` (derived from `email_verified_at IS NOT NULL` in the DB) falling back to Firebase token `email_verified` claim. Unverified → 403 `EMAIL_VERIFICATION_REQUIRED`.

There is NO separate `RequireEmailVerified` middleware. `RequireActiveAccount` IS the email verification gate for chat — both conditions must pass.

The chat routes (`routes_core.go:410-455`) apply `middleware.RequireActiveAccount(db.Pgx())` to all POST/PUT mutations. GET routes have only auth middleware (no account/email gate).

**Design decision:** `RequireActiveAccount` is sufficient. The authorization matrix below reflects this — "Active Account" implies both account status AND email verification.

### Final Authorization Matrix

| Operation | Resource | Login | Active Account | Room Membership | Block Check | Resource Visibility | Ownership | Market Authority | HTTP Rejection |
|---|---|---|---|---|---|---|---|---|---|
| SHARE_TO_CHAT | Profile | ✅ | ✅ | ✅ | ✅ sender→recipient | ✅ profile accessible to viewer | ❌ | ❌ | 403/404 |
| SHARE_TO_CHAT | Content | ✅ | ✅ | ✅ | ✅ sender→recipient | ✅ content visible to viewer | ❌ | ❌ | 403/404 |
| SHARE_TO_CHAT | FPS | ✅ | ✅ | ✅ | ✅ sender→recipient | ✅ listing active + visible | ❌ | ❌ | 403/404 |
| SHARE_TO_CHAT | Auction | ✅ | ✅ | ✅ | ✅ sender→recipient | ✅ auction active + visible | ❌ | ❌ | 403/404 |
| DIRECT_COMMERCE_INSERT | FPS | ✅ | ✅ | ✅ | ✅ sender→recipient | ✅ listing active | ✅ sender = seller | ✅ HasActiveSellerCapability | 403/404 |
| DIRECT_COMMERCE_INSERT | Auction | ✅ | ✅ | ✅ | ✅ sender→recipient | ✅ auction active | ✅ sender = seller | ✅ HasActiveSellerCapability | 403/404 |

### Error Mapping

| Condition | HTTP | Code |
|---|---|---|
| Not authenticated | 401 | UNAUTHORIZED |
| Account not active | 403 | ACCOUNT_NOT_ACTIVE |
| Not room participant | 403 | NOT_PARTICIPANT |
| Blocked by recipient | 403 | USER_BLOCKED |
| Resource not found | 404 | RESOURCE_NOT_FOUND |
| Resource not visible to viewer | 403 | RESOURCE_NOT_ACCESSIBLE |
| DIRECT_COMMERCE_INSERT: not owner | 403 | NOT_RESOURCE_OWNER |
| DIRECT_COMMERCE_INSERT: no market authority | 403 | MARKET_AUTHORITY_REQUIRED |
| Resource lifecycle prevents operation | 400 | RESOURCE_UNAVAILABLE |
| Idempotency conflict | 409 | IDEMPOTENCY_CONFLICT |

---

## 14. TRANSACTION + IDEMPOTENCY INTEGRATION

### Transaction Order

1. Authenticate + account gate (`RequireActiveAccount`)
2. Parse request — strict binding validation including occurrence block
3. Begin `WithTx`:
   a. Load room, verify sender is participant
   b. Block check (sender→recipient, with order-context exemption)
   c. If `resource_occurrence` present:
      - Validate `operation` + `resource_type` compatibility
      - Resolve resource from live table (with existence check)
      - Verify resource visibility/access for viewer
      - If `direct_commerce_insert_chat`: verify ownership + market authority
      - Build server-side fallback snapshot
   d. Validate reply target (if present)
   e. Validate media assets (if present)
   f. Compute idempotency command fingerprint (including occurrence)
   g. Actor-scoped idempotency resolution (Scope 3A authority)
   h. Create `chat_messages` row
   i. Create `chat_message_resource_occurrences` row (if applicable)
   j. Create media asset relations (if applicable)
   k. Update room `last_message_at`
   l. Upsert sender read state
   m. Emit outbox events (including occurrence data)
4. Commit

### Idempotency Fingerprint Update

`computeSendMessageCommandFingerprint` extended to include:

```go
// After existing fields:
if resourceOccurrence != nil {
    h.Write([]byte(resourceOccurrence.Operation))
    h.Write([]byte(resourceOccurrence.ResourceType))
    h.Write([]byte(resourceOccurrence.ResourceID.String()))
}
```

Same Scope 3A semantics: same actor + same key + same fingerprint → stable replay; different fingerprint → 409.

---

## 15. CURRENT LEGACY DATA COUNTS

**Labuda has zero production data.** All `chat_messages` rows exist only in ephemeral test databases.

| Metric | Count |
|---|---|
| Total `chat_messages` rows in production | 0 |
| `attachment_json` reference rows | 0 |
| FPS reference rows | 0 |
| Auction reference rows | 0 |

The `dev-reset-data` tool truncates all chat tables. The `seed` tool does not create chat messages. Integration tests create ephemeral rows in `labuda_test` databases.

---

## 16. BACKFILL / ZERO-DATA DECISION

**Option A: No backfill needed because zero production rows.**

No migration step is required to convert legacy `attachment_json.type=reference` rows into `chat_message_resource_occurrences` rows. There are no such rows.

The additive migration (000034) creates the occurrence table. New messages use the canonical path. Old messages (none exist) have no occurrence row — the read path handles absent occurrence gracefully (no `resource_occurrence` field in response).

**No backfill SQL. No operation inference. No `backfilled` flag. No compatibility wrapper.**

---

## 17. REALTIME / OUTBOX / ROOM-LIST DESIGN

### Outbox Event Extension

`chat.message.sent` outbox payload extended with optional `resource_occurrence`:

```json
{
  "room_id": "uuid",
  "message_id": "uuid",
  "sender_id": "uuid",
  "recipient_id": "uuid",
  "message_type": "text",
  "created_at": "ISO8601",
  "resource_occurrence": {
    "operation": "share_to_chat",
    "resource_type": "fixed_price_sale",
    "resource_id": "uuid"
  }
}
```

Compact identity only — no fallback, no live data. Consumers fetch full projection via REST.

### Realtime (WebSocket)

Message event envelope extended with `resource_occurrence` compact identity (same shape as outbox). Client fetches full projection via `GET /chat/rooms/{id}/messages`.

### Room Last-Message Preview

`buildChatRoomLastMessagePayload` extended to include `resource_occurrence` compact identity when the last message carries one. Room list cards can show a resource-type indicator (e.g., "📦 shared a listing").

### Notification Preview

Push notification title/body remain generic ("New message"). In-app notification `data` already carries `{chatId, messageId}` — unchanged. Resource occurrence rendering happens when the user opens the chat, not in the notification payload.

### DTO Unification

One canonical `ResourceOccurrenceProjection` struct used across:
- HTTP message response
- Outbox payload
- Realtime envelope
- Room last-message summary

No separate DTO shapes unless transport framing requires it (e.g., WebSocket uses a minimal version with only identity, no fallback).

---

## 18. BOUNDED QUERY DESIGN

### Occurrence Hydration (per message page)

Single batch query loads all occurrences for visible messages:

```sql
SELECT message_id, operation, actor_id,
       profile_source_id, content_source_id, fixed_price_sale_source_id, auction_source_id,
       fallback_snapshot, created_at
FROM chat_message_resource_occurrences
WHERE message_id = ANY($1)
```

### Resource Live Data (per occurrence batch)

After loading occurrences, batch-load live resource data by type:

| Resource Type | Query Target |
|---|---|
| Profile | `users` + `user_profiles` LEFT JOIN `seller_profiles` WHERE `u.id = ANY($1)` |
| Content | `contents` WHERE `id = ANY($1)` |
| FPS | `fixed_price_sales` + `products` (media) + `seller_profiles` WHERE `id = ANY($1)` |
| Auction | `auctions` + `products` (media) + `seller_profiles` WHERE `id = ANY($1)` |

### Seller Identity Batching

All seller IDs from commerce occurrences are collected and resolved in one batch (users + user_profiles + seller_profiles).

### Viewer Block/Privacy State

Viewer-specific block state is computed once per request (viewer vs. each resource owner), not per occurrence.

### Query Count Target

For a page of 50 messages where 5 carry occurrences:
- **1** occurrence batch query
- **4** resource-specific batch queries (or fewer if only some types are present)
- **1** seller identity batch query
- **1** viewer block state query
- **Total: ≤7 queries regardless of message count**

Zero N+1. Zero per-message queries. Zero client-side preview fetch.

---

## 19. SWITCH + IMMEDIATE PURGE PLAN

### Phase 1: Additive (Migration 000034)

1. Create `chat_resource_occurrence_operation_enum`
2. Create `chat_message_resource_occurrences` table with all constraints
3. Backend: add `resource_occurrence` block support to `SendMessageRequest`
4. Backend: add occurrence creation in `SendMessage` transaction
5. Backend: add occurrence projection to `messageToResponse`
6. Backend: extend outbox/realtime envelopes
7. Mobile: NO changes yet (still uses old `attachment_json.type=reference` path)
8. Both paths coexist: `resource_occurrence` (canonical) and `attachment_json.type=reference` (legacy)
9. Deploy, verify zero errors

### Phase 2: Mobile Switch

1. Mobile: `ShareReference` → `ResourceOccurrence` for new sends
2. Mobile: read path consumes `resource_occurrence` envelope (falls back to `attachment_json` for legacy messages)
3. Mobile: `ShareReference.chatWireTargetType` extended to allow `profile`/`content`
4. Mobile: sendToChat share destination activated
5. Mobile: OBJECT_PREVIEW_CARD rendering switched to occurrence-based live data
6. Deploy, verify

### Phase 3: Immediate Purge (same deployment as Phase 2 or next)

1. Backend: `attachment_json.type=reference` REJECTED in `ValidateAttachmentJSON`
2. Backend: `SendMessageRequest.AttachmentJSON` with type=reference → 400
3. Mobile: `ChatMapper.domainAttachmentToDto` no longer converts resource references
4. Mobile: `ObjectReference` / `ShareReference` chat transport helpers removed
5. Tests: old attachment reference tests converted to negative contracts
6. Dead code: ChatGateway ×2, dead usecases, CreateChatDto, ChatListDto, sendToChat constant — DELETED

### NOT Purged

- `attachment_json` column — still used by negotiation/shipping/location
- `attachmentvalidator.ValidateAttachmentJSON` — still validates non-reference types
- `chat_commerce_references` — room-scoped, untouched
- `ChatWebSocketHandler` — only if confirmed dead; otherwise KEEP_CANONICAL

---

## 20. DEAD/LEGACY CODE DISPOSITION

| Item | Disposition | Rationale |
|---|---|---|
| `ChatGateway` (lib/shared/chat/) | DELETE_DURING_SCOPE_3 | 0 implementers, 0 consumers |
| `ChatGateway` (lib/domains/chat/) | DELETE_DURING_SCOPE_3 | 0 implementers, 0 consumers |
| `LinkOrderToChatUseCase` | DELETE_DURING_SCOPE_3 | 0 importers; live path uses `GetOrCreateCommerceChatUseCase` |
| `GetFixedPriceSaleShareReferenceUseCase` (chat) | DELETE_DURING_SCOPE_3 | 0 importers, absent from barrel |
| `GetFixedPriceSaleShareReferenceUseCase` (catalog) | DELETE_DURING_SCOPE_3 | 0 importers |
| `CreateChatDto` | DELETE_DURING_SCOPE_3 | Orphaned — room creation uses POST with no body |
| `ChatListDto` | DELETE_DURING_SCOPE_3 | Bypassed by datasource |
| `ShareDestination.sendToChat` | ACTIVATED (no longer dead) | Becomes live with Profile/Content share |
| `ChatWebSocketHandler` | SEPARATE_NON_RESOURCE_CHAT_SCOPE | Verify liveness before deletion; may still serve legacy WS clients |
| `widget_lib.AttachmentWidget` (for chat) | SEPARATE_NON_RESOURCE_CHAT_SCOPE | Still used by negotiation/shipping/location rendering |
| `object_reference_bridge.dart` | DELETE_DURING_SCOPE_3 | Stale comment direction |
| `attachment_dto.dart:54` stale comment | DELETE_DURING_SCOPE_3 | Lists content/profile as valid — update to reflect canonical |
| Chat mapper legacy wire aliases | DELETE_DURING_SCOPE_3 | `objectReference` key, legacy format parsing, `offer_reference` fallback |
| `chat_entities.dart` legacy defaults | DELETE_DURING_SCOPE_3 | Polluted compatibility maps, legacy payload defaults |
| Backend "backward compatibility" nil-checker skip | DELETE_DURING_SCOPE_3 | `validateAttachmentReferences` skip-if-nil |

---

## 21. REQUIRED EXECUTABLE PROOF MATRIX

### Schema Proofs (000034)
1. Table exists with all constraints
2. `exactly_one_source` CHECK rejects 0 sources
3. `exactly_one_source` CHECK rejects 2+ sources
4. `valid_operation_for_resource` CHECK rejects content/profile via direct_commerce_insert
5. `message_id` PK prevents duplicate occurrences per message
6. Migration replay clean (up + down + up)
7. RESTRICT FK prevents deletion of resource with occurrences

### Share-to-Chat Proofs
8. Share Profile → message + occurrence created, operation=share_to_chat
9. Share Content → same
10. Share FPS → same
11. Share Auction → same
12. Non-participant → 403
13. Blocked → 403
14. Deleted/banned resource → 400/403
15. Invalid resource ID → 400/404

### Direct Commerce Insert Proofs
16. Insert own FPS → operation=direct_commerce_insert_chat
17. Insert own Auction → same
18. Insert other seller's FPS → 403 NOT_RESOURCE_OWNER
19. Insert without seller capability → 403 MARKET_AUTHORITY_REQUIRED
20. Insert Profile via commerce_insert → 400 (CHECK or validation)

### Idempotency Integration Proofs
21. Same actor + same key + same occurrence → stable replay
22. Same actor + same key + different resource → 409
23. Different actor + same key → independent (Scope 3A already proven)
24. Concurrent same-actor same-occurrence → one artifact (Scope 3A pattern)

### Fallback Proofs
25. Profile fallback built from users + user_profiles at write time
26. Content fallback includes nested resource metadata when applicable
27. FPS fallback excludes live status fields
28. Auction fallback excludes live phase/bid fields
29. Client-submitted preview data rejected (400 if present in request)
30. Fallback suppressed when resource is deleted/banned (read-time check)

### Live Projection Proofs
31. Active resource → live payload present
32. Deleted resource → live payload absent, fallback suppressed
33. Banned seller → live payload absent, fallback suppressed
34. Blocked viewer → viewer-specific tombstone

### Query Bound Proofs
35. Message page with 5 occurrences → ≤7 total queries
36. Zero per-message queries
37. Batch occurrence resolution works with mixed resource types

### Switch + Purge Proofs
38. After purge: attachment_json.type=reference rejected with 400
39. After purge: old mobile reference path rejected with 400
40. After purge: negotiation/shipping/location attachments still work
41. Dead code files confirmed deleted, compilation passes

### Regression
42. Full chat unit test suite passes
43. Full chat integration suite passes
44. Non-reference attachment tests (negotiation/shipping/location) unaffected

---

## 22. ONLY REMAINING OWNER DECISIONS

### Q1: Content Nesting Depth (from §10)

**OWNER_DECISION_REQUIRED**

Proposed: DEPTH 1 ONLY — shared Content shows compact nested resource indicator, no recursive expansion.

Is depth 1 acceptable? Should nested resources be clickable/tappable to navigate to the nested resource?

### All other decisions resolved in this design:

- Profile FK authority: `users.id` ✅
- Source-type authority: FK presence (no enum) ✅
- message_id as PK, no room_id duplication ✅
- FK RESTRICT semantics ✅
- Server-built fallback, no client preview ✅
- Single SendMessage endpoint with resource_occurrence block ✅
- Authorization matrix including market authority for DIRECT_COMMERCE_INSERT ✅
- Zero-data: no backfill, no operation inference ✅
- Immediate purge (no dual-write compatibility window) ✅
- Dead code disposition ✅
- Bounded query design (≤7 queries per page) ✅

---

## 23. RECOMMENDATION

**READY_FOR_SCOPE_3_IMPLEMENTATION**

The design is complete. One owner decision remains (Content nesting depth, §10), but it does not block schema, API, authorization, or transaction design — it affects only the Content fallback and live projection payload shape, which can be finalized before mobile implementation.

**Next migration:** 000034 (after 000032 + 000033).

**Implementation order:**
1. Migration 000034 (additive schema)
2. Backend: occurrence write path + read projection
3. Backend: extend outbox/realtime envelopes
4. Backend: UUID contract + HTTP tests
5. Mobile: canonical occurrence producer/consumer (replaces attachment_json reference)
6. Mobile: activate Profile/Content share-to-chat
7. Purge: remove attachment_json reference authority
8. Purge: delete dead code inventory
9. Executable proof suite (44 proofs)

---

*End of design reconciliation. No implementation performed.*
