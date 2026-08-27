# UNIFIED SHARE SCOPE 3D — BACKEND READ PROJECTION AND TOMBSTONE AUTHORITY AUDIT

**STATUS:** `UNIFIED_SHARE_SCOPE_3D_BACKEND_READ_PROJECTION_AUDIT_COMPLETE`

**DATE:** 2026-08-08

**MODE:** AUDIT ONLY — NO IMPLEMENTATION

**REPOSITORY:** `d:\Project\labuda`

---

## 1. VERDICT

`UNIFIED_SHARE_SCOPE_3D_BACKEND_READ_PROJECTION_AUDIT_COMPLETE`

The audit is unblocked. All 18 sections are populated with executable findings backed by
direct filesystem evidence. No schema change is required to begin implementation. The
read projection is essentially a **greenfield build on top of a fully operational write
path** — the occurrence rows are correctly persisted, the fallback snapshots are
correctly built, but **nothing reads them back**.

### Core gap

`chat_message_resource_occurrences` rows are atomically inserted during `SendMessage`
(Scope 3C verified). `GetResourceOccurrencesByMessageIDs` is implemented in the
repository (`chat_repository_impl.go:532-579` — single `ANY($1)` batch query). However,
**it has zero production callers**. The read path (`ListMessages` → `messageToResponse`)
never loads occurrences and never emits resource projection data.

### What exists today

| Capability | Status |
|---|---|
| Occurrence write (atomic message + occurrence + outbox) | ✅ Scope 3C verified |
| Occurrence batch-load repository method | ✅ Implemented, unused |
| Server-built fallback_snapshot (all 4 types) | ✅ Populated at write time |
| ResourceAuthorizer port (share + direct) | ✅ Wired in production |
| Viewer-aware resource projection on read | ❌ NOT IMPLEMENTED |
| LIVE / FALLBACK_ALLOWED / TOMBSTONE states | ❌ NOT IMPLEMENTED |
| Content depth-1 nested indicator | ❌ NOT IMPLEMENTED |
| Capability projection per resource type | ❌ NOT IMPLEMENTED |

---

## 2. CURRENT READ CALL GRAPH

### Primary message read: `GET /api/v1/chat/rooms/:room_id/messages`

```
routes_core.go:429
  └─> chat_handler.go:948  ChatHandler.ListMessages
        ├─> chat_handler.go:995  chatService.GetRoom (participant gate)
        │     └─> chat_service.go:431  Service.GetRoom
        │           └─> chat_repository_impl.go:61  GetRoomByID
        │                 SQL: SELECT ... FROM chat_rooms WHERE id=$1
        │
        ├─> chat_handler.go:999  blockcheck.IsBidirectionallyBlocked (social gate)
        │     SQL: SELECT EXISTS(SELECT 1 FROM user_blocks WHERE ...)
        │
        ├─> chat_handler.go:1011  chatService.ListMessages
        │     └─> chat_service.go:1156  Service.ListMessages
        │           ├─> repo.GetRoomByID (participant check)
        │           └─> chat_repository_impl.go:395  ListMessagesByRoom
        │                 SQL: SELECT id, room_id, sender_id, message_type, body,
        │                           attachment_json, idempotency_key,
        │                           command_fingerprint, created_at,
        │                           deleted_at, deleted_by, deletion_reason
        │                      FROM chat_messages
        │                      WHERE room_id=$1
        │                      ORDER BY created_at DESC, id DESC LIMIT $n
        │
        ├─> chat_handler.go:1032  hydrateMessageSenders
        │     └─> chat_handler.go:1724  buildChatParticipantCardsWithLifecycle
        │           SQL: SELECT u.id, COALESCE(p.username,''), p.avatar_url,
        │                     u.account_status, (u.deleted_at IS NOT NULL)
        │                FROM users u LEFT JOIN user_profiles p ON p.user_id=u.id
        │                WHERE u.id = ANY($1)
        │
        ├─> chat_handler.go:1033  hydrateAttachmentSellerLifecycles
        │     └─> chat_handler.go:1940  UNION batch query
        │           SQL: WITH item_sellers AS (
        │                  SELECT id::text, seller_id FROM fixed_price_sales WHERE id=ANY($1)
        │                  UNION ALL
        │                  SELECT id::text, seller_id FROM auctions WHERE id=ANY($2)
        │                ) SELECT is2.item_id, u.account_status, u.deleted_at, ss.status
        │                  FROM item_sellers LEFT JOIN users u ...
        │                  LEFT JOIN LATERAL seller_subscriptions ...
        │
        └─> chat_handler.go:1563  messageToResponse (per-message)
              └─> map[string]interface{} — NO occurrence data
```

### Secondary message read: `GET /api/v1/chat/rooms/by-order/:order_id`

Same as above, but also calls `ListMessages` inline (last 50 messages, line 385).

### Room-list message preview: `GET /api/v1/chat/rooms`

```
chat_handler.go:148  ListRooms
  └─> chat_handler.go:1434  batchLatestMessages
        SQL: SELECT DISTINCT ON (room_id) id, room_id, sender_id, message_type,
                  body, attachment_json, idempotency_key, created_at,
                  deleted_at, deleted_by, deletion_reason
             FROM chat_messages WHERE room_id = ANY($1)
             ORDER BY room_id, created_at DESC, id DESC
```

### No single-message read endpoint exists

`repo.GetMessageByID` (`chat_repository_impl.go:359`) exists but is only called by
moderation enforcement (`SoftHideForModeration` at `chat_service.go:1351`,
`RestoreFromModeration` at line 1376). There is no `GET /api/v1/chat/messages/:id`.

---

## 3. CURRENT RESPONSE CONTRACT

### Normal message (`messageToResponse`, chat_handler.go:1563-1614)

```json
{
  "id": "uuid",
  "room_id": "uuid",
  "sender_id": "uuid",
  "message_type": "text",
  "created_at": "ISO8601",
  "body": "message text",
  "attachment_json": { "type": "...", "data": {...} },
  "attachment_metadata": {
    "seller_user_lifecycle": "active",
    "seller_trust_lifecycle": "active"
  },
  "sender": { /* ChatParticipantCard */ }
}
```

### Tombstone message (deleted_at != nil)

```json
{
  "id": "uuid",
  "room_id": "uuid",
  "sender_id": "uuid",
  "message_type": "text",
  "created_at": "ISO8601",
  "is_hidden": true
}
```

Body, attachment_json, attachment_metadata, and sender card are ALL suppressed.
Timeline structure preserved.

### Old resource reference (attachment_json type=reference)

Currently emitted through `attachment_json` passthrough — client receives the full
reference blob including client-trusted `preview` data:

```json
{
  "attachment_json": {
    "type": "reference",
    "data": {
      "target_type": "fixed_price_sale",
      "target_id": "uuid",
      "preview": {
        "title": "Product Title",
        "imageUrl": "https://...",
        "isAvailable": true,
        "isSold": false,
        "isClosed": false,
        "isDeleted": false
      }
    }
  }
}
```

### New resource occurrence

**Not projected at all.** The occurrence row exists in the database but is never
read back through the message response. There is no `resource_occurrence`,
`resource_projection`, or equivalent field in the current response contract.

---

## 4. OCCURRENCE READ AUTHORITY

### Verdict: ROWS EXIST, NEVER READ

`GetResourceOccurrencesByMessageIDs` is defined in the repository interface
(`chat_repository.go:107-108`) and implemented as a single `ANY($1)` batch query
(`chat_repository_impl.go:532-579`). It returns `map[uuid.UUID]*ChatMessageResourceOccurrence`
keyed by `message_id`.

**Production callers: ZERO**

The only references are test stubs:
- `chat_link_order_authorization_test.go:92`
- `negotiation_event_handler_chatroom_test.go:82`
- `chat_service_room_updated_producer_test.go:270`

**No JOIN exists** between `chat_messages` and `chat_message_resource_occurrences`
anywhere in production SQL.

The write path (`SendMessage` → `authorizeOccurrence` → `CreateResourceOccurrence`)
is fully operational and verified by Scope 3C. The read path has not been built yet.

---

## 5. PROFILE READ FINDINGS

### Current state

Profile references can arrive through two channels:
1. **Legacy**: `attachment_json.type=reference` with `target_type=profile`
2. **New**: `resource_occurrence` with `resource_type=profile` (write-only)

### Live authority available

The `ResourceAuthorizer` adapter (`chat_resource_authorizer_adapter.go:121-139`) already
implements `authorizeProfileShare`:
- Checks `users.deleted_at IS NULL` → `ErrResourceNotFound`
- Checks bidirectional block (viewer ≠ profile) → `ErrResourceNotAccessible`
- Builds fallback on success

The handler already has `buildChatParticipantCardsWithLifecycle` which queries
`users.account_status` + `users.deleted_at` and coarsens to `active/unavailable/removed`.

### Privacy/block handling

Block checks exist in the handler (`chat_handler.go:995-1008` for `ListMessages`,
`chat_handler.go:199-226` for `ListRooms`) using the shared `blockcheck` package.

### Gap analysis

| Concern | Status |
|---|---|
| Profile LIVE projection | ❌ Not built — no occurrence read |
| Suspended profile → tombstone | ❌ Not built |
| Deleted profile → tombstone | ❌ Not built |
| Blocked profile → viewer-specific tombstone | ❌ Not built |
| Fallback exposure (username/avatar/store) | ⚠️ Stored in DB, never read |
| Query pattern | ❌ No occurrence load means no profile queries |

### Required implementation

On read, for each Profile occurrence:
1. Query the user's current `account_status`, `deleted_at` from `users`
2. Check viewer ↔ profile block status
3. Derive state: LIVE (active, not blocked) / TOMBSTONE (suspended, deleted, blocked)
4. For LIVE: project current username, avatar_url, store_name from `user_profiles` + `seller_profiles`
5. For FALLBACK_ALLOWED: use `fallback_snapshot` (historical display, no privacy leak)
6. For TOMBSTONE: suppress all identity data

---

## 6. CONTENT READ FINDINGS

### Current state

Content references arrive through:
1. **Legacy**: `attachment_json.type=reference` with `target_type=post` or `target_type=request`
2. **New**: `resource_occurrence` with `resource_type=content` (write-only)

### Live authority available

The `authorizeContentShare` method (`chat_resource_authorizer_adapter.go:142-171`) already
implements the canonical visibility check:
- deleted/hidden/moderated → `ErrResourceNotAccessible`
- `public` → OK
- `private` → author only
- `followers_only` → author, or follower not blocked

### Content nested reference (ShareReference)

The Content entity (`content.go:26`) carries `ShareReference *ShareReference` — when a
Content is a repost, this field contains the canonical reference to the original resource
(`share_reference.go:55-58`):
- `TargetType`: `content | fixed_price_sale | auction | profile`
- `TargetID`: raw string UUID
- `Preview`: cached display data (title, imageUrl, availability flags)

This is the Depth-1 nested indicator source. It is resolved from the Content's CURRENT
canonical state at read time — no cache staleness problem.

### Gap analysis

| Concern | Status |
|---|---|
| Content LIVE projection | ❌ Not built |
| Visibility gates (public/private/followers) | ⚠️ Authorizer has logic; not invoked on read |
| Hidden/moderated content | ⚠️ `c.IsHidden` / `c.Status` checked in authorizer only |
| Deleted content | ⚠️ `c.DeletedAt != nil` checked in authorizer only |
| Block-based tombstone | ❌ Not built |
| Fallback exposure (caption, media preview) | ⚠️ Stored, never read |
| Nested reference indicator | ❌ Not built |
| Query pattern | ❌ No occurrence load means no Content queries |

### Required implementation

On read, for each Content occurrence:
1. Query the Content's current `status`, `visibility`, `is_hidden`, `deleted_at`, `author_id`
2. Check viewer ↔ author block status
3. Apply canonical visibility evaluator (or inline equivalent):
   - `EvaluateContentDetail` in `internal/governance/evaluator/content_detail_shadow.go`
4. Derive state: LIVE / FALLBACK_ALLOWED / TOMBSTONE
5. For LIVE: project current Content detail (caption, media, author)
6. For FALLBACK_ALLOWED: use `fallback_snapshot`
7. For TOMBSTONE: suppress
8. If LIVE and `ShareReference != nil`: resolve depth-1 nested indicator

---

## 7. FPS READ FINDINGS

### Current state

FPS references arrive through:
1. **Legacy**: `attachment_json.type=reference` with `target_type=fixed_price_sale`, or flat `type: "fixed_price_sale"`
2. **New**: `resource_occurrence` with `resource_type=fixed_price_sale` (write-only)

### Live authority available

FPS share authorization (`chat_resource_authorizer_adapter.go:174-179`) is existence-only.
FPS direct authorization (lines 192-213) checks:
- Ownership (`SellerID == actorID`)
- `HasActiveSellerCapability`
- `Status.IsRepostable()` (only `FixedPriceSaleStatusActive`)

The `hydrateAttachmentSellerLifecycles` handler method already batch-queries FPS seller
lifecycle via UNION query.

### Terminal lifecycle

`FixedPriceSaleStatus` values: `active`, `sold`, `inactive`, `deleted` etc.
`IsRepostable()` returns true only for `active`.

Terminal lifecycle (sold, inactive) does NOT mean tombstone — the listing detail may
still be publicly viewable. The FPS should remain LIVE with `can_interact=false` when
the listing is publicly viewable but not purchasable.

### Gap analysis

| Concern | Status |
|---|---|
| FPS LIVE projection | ❌ Not built |
| Current price, quantity, status | ❌ Not projected |
| Seller lifecycle enrichment | ⚠️ Already done for attachments; reusable |
| Terminal lifecycle (sold/inactive) | ❌ Not projected |
| can_interact capability | ❌ Not built |
| Fallback exposure | ⚠️ Stored, never read |
| Query pattern | ❌ No occurrence load means no FPS queries |

### Required implementation

On read, for each FPS occurrence:
1. Batch-query `fixed_price_sales` + `products` for current state
2. Batch-query seller lifecycle (already implemented pattern)
3. Derive state based on visibility + lifecycle
4. LIVE: project title, price, quantity, image, status, seller, capabilities
5. FALLBACK_ALLOWED: use `fallback_snapshot` (historical title/image/store only)
6. TOMBSTONE: only if privacy/legal removal requires it
7. Capabilities: `can_view`, `can_interact` (= status is active AND not sold), `can_chat`, `can_buy`

---

## 8. AUCTION READ FINDINGS

### Current state

Auction references arrive through:
1. **Legacy**: `attachment_json.type=reference` with `target_type=auction`, or flat `type: "auction"`
2. **New**: `resource_occurrence` with `resource_type=auction` (write-only)

### Live authority available

Auction share authorization (`chat_resource_authorizer_adapter.go:182-187`) is existence-only.
Auction direct authorization (lines 215-236) checks:
- Ownership (`SellerID == actorID`)
- `HasActiveSellerCapability`
- `Status.IsRepostable()` (Scheduled or Active)

### Terminal lifecycle

Auction `Status` values: `scheduled`, `active`, `ended`, `cancelled`, `expired_bnr`, etc.
`IsRepostable()` returns true for `StatusScheduled || StatusActive`.

Like FPS, terminal lifecycle does NOT automatically mean tombstone. Completed auctions
with visible results (winner, final price) should remain LIVE.

### Gap analysis

| Concern | Status |
|---|---|
| Auction LIVE projection | ❌ Not built |
| Current bid, bid count, time remaining | ❌ Not projected |
| Seller lifecycle enrichment | ⚠️ Already done for attachments; reusable |
| Terminal lifecycle (ended/cancelled) | ❌ Not projected |
| can_interact / can_bid capability | ❌ Not built |
| Fallback exposure | ⚠️ Stored, never read |
| Query pattern | ❌ No occurrence load means no Auction queries |

### Required implementation

Same pattern as FPS but against `auctions` + `products` tables, with auction-specific
status and capability fields (can_bid vs can_buy).

---

## 9. TOMBSTONE FINDINGS

### Current tombstone behavior

**Single tombstone type exists**: message-level soft-deletion (`deleted_at IS NOT NULL`).

Implementation: `messageToResponse` in `chat_handler.go:1563-1614`:
```go
if msg.DeletedAt != nil {
    resp["is_hidden"] = true
    return resp  // early return — body, attachment, sender ALL suppressed
}
```

Test coverage: `chat_message_tombstone_test.go` — 3 test cases proving body, attachment,
attachment_metadata, and sender card are all suppressed; `is_hidden=true` emitted;
timeline fields preserved.

### Resource-level tombstone: NOT IMPLEMENTED

There is no concept of resource-level tombstone anywhere in the read path. The following
cases are handled in the write-side authorizer but never projected on read:

| Tombstone Case | Write Authorizer | Read Projection |
|---|---|---|
| Profile suspended | N/A (share gate only) | ❌ Not projected |
| Profile deleted | `ErrResourceNotFound` | ❌ Not projected |
| Profile blocked | `ErrResourceNotAccessible` | ❌ Not projected |
| Content deleted | `ErrResourceNotAccessible` | ❌ Not projected |
| Content hidden | `ErrResourceNotAccessible` | ❌ Not projected |
| Content moderated | `ErrResourceNotAccessible` | ❌ Not projected |
| Content private (non-author viewer) | `ErrResourceNotAccessible` | ❌ Not projected |
| FPS/Auction deleted/removed | Existence check → `ErrResourceNotFound` | ❌ Not projected |
| FPS/Auction terminal lifecycle | N/A (publicly viewable) | ❌ Should be LIVE not tombstone |

### Required tombstone design

| Case | State | Behavior |
|---|---|---|
| Message soft-deleted | TOMBSTONE | Timeline preserved; no body/attachment/sender |
| Profile suspended | TOMBSTONE (temporary) | Redacted identity; no username/avatar leakage |
| Profile banned/deleted | TOMBSTONE (permanent) | No historical identity leakage |
| Profile blocked by viewer | TOMBSTONE (viewer-specific) | No identity visible to blocking viewer |
| Content deleted | TOMBSTONE | No Content data leakage |
| Content hidden/moderated | TOMBSTONE | No Content data leakage |
| Content private (non-author) | TOMBSTONE | No Content data leakage |
| Content blocked author | TOMBSTONE | No Content data leakage |
| FPS/Auction publicly viewable, terminal lifecycle | LIVE | `can_interact=false`; historical data visible |
| FPS/Auction deleted/privacy-removed | TOMBSTONE | No commerce data leakage |

---

## 10. CONTENT DEPTH-1 FINDINGS

### Current support: ZERO

No nested resource indicator exists in the chat read path.

### Available infrastructure

The Content entity (`content.go:26`) carries:
```go
ShareReference *ShareReference // Set when this is a repost
```

`ShareReference` (`share_reference.go:55-58`) contains:
- `TargetType ShareTargetType` — `content | fixed_price_sale | auction | profile`
- `TargetID string` — raw UUID string
- `Preview SharePreview` — cached display (title, imageUrl, availability flags)

This is the EXACT mechanism needed for depth-1 nesting. When a Content occurrence is
shared to Chat and that Content is itself a repost of another resource, the
`ShareReference` provides the nested identity.

### Design confirmation

The Content's `ShareReference.TargetType` maps 1:1 to the chat occurrence resource types:
- `content` → `ResourceOccurrenceResourceTypeContent`
- `fixed_price_sale` → `ResourceOccurrenceResourceTypeFixedPriceSale`
- `auction` → `ResourceOccurrenceResourceTypeAuction`
- `profile` → `ResourceOccurrenceResourceTypeProfile`

### Gap analysis

| Concern | Status |
|---|---|
| Nested indicator in response | ❌ Not built |
| Resolve nested identity from Content | ⚠️ `ShareReference` available, not queried |
| Depth enforcement (max 1 level) | ❌ Not built (but trivial — just don't recurse) |
| Nested resource tombstone | ❌ Not built |
| Navigation preserves Content identity | ⚠️ Already true (Content is primary occurrence) |

### Required implementation

When a Content occurrence is LIVE:
1. Load Content's `ShareReference` (if any)
2. If `ShareReference != nil` AND `ShareReference.TargetType` is a known type:
   - Resolve nested resource's current identity (NOT full projection, just identity)
   - Apply nested resource's canonical access rules
   - If accessible: emit `nested_resource: { type, id, compact_label }`
   - If inaccessible: suppress `nested_resource` (no tombstone leakage)
3. Do NOT recurse beyond this one level
4. Navigation target remains the originally selected Content

---

## 11. ROOM-COMMERCE-CONTEXT SEPARATION RESULT

### Verdict: CLEAN — NO ACCIDENTAL COUPLING

`chat_commerce_references` is a separate table, separate entity, separate repository
methods, and separate HTTP endpoints. Message reads do NOT join, reference, or depend
on commerce references in any way.

**Evidence:**

1. **Separate table**: `chat_commerce_references` (migration 000029) — columns: `id`,
   `room_id`, `target_type` (FPS/Auction only), `target_id`, `creator_id`,
   `display_snapshot`, `created_at`

2. **Separate endpoints**:
   - `POST /api/v1/chat/rooms/:room_id/commerce-references` — create/get idempotent
   - `GET /api/v1/chat/rooms/:room_id/commerce-references` — list by room
   - `GET /api/v1/chat/commerce-references/:reference_id` — get by ID

3. **Message read independence**: `ListMessagesByRoom` SQL (`chat_repository_impl.go:403-422`)
   queries only `chat_messages` — no JOIN to `chat_commerce_references`.

4. **Occurrence independence**: `GetResourceOccurrencesByMessageIDs` SQL
   (`chat_repository_impl.go:542-549`) queries only `chat_message_resource_occurrences` —
   no JOIN to `chat_commerce_references`.

5. **Entity separation**: `ChatMessage` has no commerce reference field.
   `ChatMessageResourceOccurrence` has no commerce reference field.

6. **Immutable commerce references**: `chat_commerce_references` rows are created once
   and never updated (ON CONFLICT DO UPDATE SET id = chat_commerce_references.id is a
   no-op returning the existing row).

7. **Room entity cleaned**: `ChatRoom` no longer carries `ContextJSON`/`ContextSetBy`
   fields (purged by migration 000030).

8. **Handler comment**: `chat_handler.go:114-118` explicitly states room context was
   migrated to `chat_commerce_references`.

**No coupling found.** Message reads, occurrence reads, and commerce reference reads
are three independent subsystems sharing only `room_id`.

---

## 12. QUERY COUNT TABLE

### Method

Queries were counted by tracing the actual Go call chain from HTTP handler entry to SQL
execution, not by counting helper methods. Each `db.Pool().Query()` / `tx.Query()` /
`tx.QueryRow()` call counts as one query.

### Current state (NO occurrence loading)

| Scenario | Queries | Breakdown |
|---|---|---|
| A. Normal messages (1 msg) | 5 | GetRoom + ListMessages + hydrateSenders + hydrateSellerLifecycles + block check |
| A. Normal messages (20 msgs) | 5 | Same — all batched |
| B. Profile occurrences (1) | 5 | Same (occurrences not loaded) |
| B. Profile occurrences (20) | 5 | Same |
| C. Content occurrences (1) | 5 | Same |
| C. Content occurrences (20) | 5 | Same |
| D. FPS occurrences (1) | 5 | Same |
| D. FPS occurrences (20) | 5 | Same |
| E. Auction occurrences (1) | 5 | Same |
| E. Auction occurrences (20) | 5 | Same |
| F. Small mixed page (5 msgs, 3 types) | 5 | Same |
| G. Larger mixed page (20 msgs, 4 types) | 5 | Same |

**Current query count: CONSTANT 4-5 regardless of message count or resource type mixture.**

This is because occurrences are never loaded. The 5 queries are:
1. `GetRoomByID` — participant gate
2. `blockcheck.IsBidirectionallyBlocked` — social gate (skipped for order/support rooms)
3. `ListMessagesByRoom` — messages only
4. `buildChatParticipantCardsWithLifecycle` — sender cards (batch `ANY($1)`)
5. `hydrateAttachmentSellerLifecycles` — seller lifecycle from attachment_json (batch UNION)

### Projected state (WITH occurrence loading, target architecture)

| Scenario | Queries | Breakdown |
|---|---|---|
| A. Normal messages (1-20) | 5 | No change (no occurrences) |
| B. Profile occurrences (1) | 7 | +GetOccurrences (1 batch) + Profile batch (1) |
| B. Profile occurrences (20) | 7 | Same — all batched |
| C. Content occurrences (1) | 7-8 | +GetOccurrences + Content batch + (optional) nested ref batch |
| C. Content occurrences (20) | 7-8 | Same |
| D. FPS occurrences (1) | 7 | +GetOccurrences + FPS batch |
| D. FPS occurrences (20) | 7 | Same |
| E. Auction occurrences (1) | 7 | +GetOccurrences + Auction batch |
| E. Auction occurrences (20) | 7 | Same |
| F. Small mixed page (3 types) | 6-8 | +GetOccurrences + 1 batch per present type |
| G. Larger mixed page (4 types) | 6-9 | +GetOccurrences + up to 4 source batches |

**Target query count: BOUNDED by resource type mixture (max 4 additional queries for 4 types), NOT message count.**

Additional viewer-context queries (block checks, follow checks for private/followers_only
content) would add per-resource queries if not batched. These MUST be batched.

---

## 13. N+1 VERDICT

### Current state: NO N+1

The current read path is N+1 free:
- Messages: 1 query for any page size
- Sender cards: 1 batch query for all distinct senders
- Seller lifecycles: 1 batch query (UNION) for all attachment-referenced items
- Block check: 1 query (bidirectional pair check, not per-message)

### Implementation risk

The implementation MUST avoid introducing N+1 in these areas:

| Risk | Mitigation |
|---|---|
| Per-message occurrence lookup | Use `GetResourceOccurrencesByMessageIDs` — already batch `ANY($1)` |
| Per-occurrence source query | Batch by resource type: one `ANY($1)` per type present |
| Per-source block check | Batch: collect all author/seller IDs, check blocks in one query |
| Per-source follow check | Batch: collect all (viewer, author) pairs, check in one query |
| Per-source seller capability | Batch: collect all seller IDs, check `HasActiveSellerCapability` per ID (or batch SQL) |
| Per-Content nested reference | If Content count >0, one batch query for nested reference identities |

**Target: query count grows only with resource type diversity (max ~4 extra), never with message count.**

---

## 14. LEGACY READ RESIDUE INVENTORY

### Classification key

| Tag | Meaning |
|---|---|
| `LEGACY_RESOURCE_REFERENCE_AUTHORITY` | Active production code handling `attachment_json.type=reference` |
| `ACTIVE_UNRELATED_ATTACHMENT` | Active production attachment handling NOT about resource references |
| `TEST_ONLY` | Only referenced in test files |
| `DEAD` | Defined but never called in production |
| `DOC/COMMENT` | Documentation or comments only |

### Production hits

| # | File | Lines | Content | Classification |
|---|---|---|---|---|
| 1 | `attachmentvalidator/validator.go` | 8, 72, 89-140 | `"reference": true` valid type; `validateReferenceAttachmentV2` — validates `target_type`, `target_id`, `preview` with availability flags | `LEGACY_RESOURCE_REFERENCE_AUTHORITY` |
| 2 | `chat_service.go` | 1464-1485 | `case "reference"` in `validateAttachmentReferences` — switches on `target_type` (FPS/Auction/post/request/profile) for existence validation | `LEGACY_RESOURCE_REFERENCE_AUTHORITY` |
| 3 | `chat_service.go` | 1487-1517 | Flat `case "fixed_price_sale"` / `case "auction"` — non-reference attachment types carrying commerce IDs directly | `LEGACY_RESOURCE_REFERENCE_AUTHORITY` |
| 4 | `chat_handler.go` | 91, 1087-1095, 1100-1104 | `AttachmentJSON` field on `SendMessageRequest`; validation call; mutual exclusion with `resource_occurrence` | `LEGACY_RESOURCE_REFERENCE_AUTHORITY` |
| 5 | `chat_handler.go` | 1587-1603 | `messageToResponse` emits `attachment_json` raw to clients — client receives preview data directly | `LEGACY_RESOURCE_REFERENCE_AUTHORITY` |
| 6 | `chat_handler.go` | 1590-1600 | `attachment_metadata` with seller lifecycle — enriches legacy attachment responses | `ACTIVE_UNRELATED_ATTACHMENT` |
| 7 | `chat_handler.go` | 1828-1859 | `extractReferencedItemIDFromAttachment` — parses `type=reference` + flat types for seller lifecycle hydration | `LEGACY_RESOURCE_REFERENCE_AUTHORITY` |
| 8 | `chat_handler.go` | 1861-1993 | `hydrateAttachmentSellerLifecycles` — UNION batch query for seller lifecycle from attachment item IDs | `ACTIVE_UNRELATED_ATTACHMENT` |
| 9 | `chat_handler.go` | 1434-1490 | `batchLatestMessages` — SELECTs `attachment_json` for room-list preview | `ACTIVE_UNRELATED_ATTACHMENT` |
| 10 | `chat_room_event_projection.go` | 82-84 | Outbox payload includes `attachment_json` passthrough | `ACTIVE_UNRELATED_ATTACHMENT` |
| 11 | `negotiation_event_handler.go` | 308-320 | `buildNegotiationProposalFromStarted` — builds `attachment_json` with `resource_type`/`resource_id` inside data | `LEGACY_RESOURCE_REFERENCE_AUTHORITY` |
| 12 | `chat_repository_impl.go` | 329-355, 359-391, 395-458 | INSERT/SELECT of `attachment_json` column — persistence plumbing | `ACTIVE_UNRELATED_ATTACHMENT` |
| 13 | `ATTACHMENT_SCHEMA_V2.md` | 25-39 | Documents `type=reference` as valid attachment type | `DOC/COMMENT` |

### Dead/unused

| # | File | Content | Classification |
|---|---|---|---|
| 14 | `chat_media_asset.go` | 115-126 | `ChatReplyPreview` struct — defined, zero production usages | `DEAD` |
| 15 | `chat_repository_impl.go` | 532-579 | `GetResourceOccurrencesByMessageIDs` — implemented, zero production callers | `DEAD` (in read path) |

### Test-only

| # | Files | Classification |
|---|---|---|
| 16 | `chat_message_tombstone_test.go:29` | `TEST_ONLY` |
| 17 | `chat_attachment_metadata_test.go:56,63,70` | `TEST_ONLY` |
| 18 | `attachment_validator_test.go:11-171` | `TEST_ONLY` |
| 19 | `chat_handler_param_binding_test.go:110` | `TEST_ONLY` |
| 20 | `chat_service_attachment_reference_test.go:197-420` | `TEST_ONLY` |
| 21 | `chat_idempotency_security_proof_test.go:212,228` | `TEST_ONLY` |

### Already purged

| Item | Status |
|---|---|
| `ContextJSON` / `ContextSetBy` | ✅ Purged by migration 000030 |
| `chat_rooms.context_json` column | ✅ Dropped by migration 000030 |
| `chat_rooms.context_set_by` column | ✅ Dropped by migration 000030 |
| Room-level context on ChatRoom entity | ✅ Removed |

### Residue summary

- **14 production hits** — 9 are `LEGACY_RESOURCE_REFERENCE_AUTHORITY`, 5 are `ACTIVE_UNRELATED_ATTACHMENT`
- **2 dead items** — `ChatReplyPreview` struct and unused `GetResourceOccurrencesByMessageIDs`
- **6 test files** — can be updated when the read projection is built
- **1 doc file** — `ATTACHMENT_SCHEMA_V2.md` needs update

The `ACTIVE_UNRELATED_ATTACHMENT` items (negotiation, shipping_quote, location
attachments) must remain untouched — they are NOT resource references and are not
in scope for the resource occurrence projection.

---

## 15. CANONICAL IMPLEMENTATION PLAN

### Recommended sub-scopes (minimum)

| Slice | Name | Scope | Dependencies |
|---|---|---|---|
| **3D-1** | Occurrence Batch Load Wiring | Wire `GetResourceOccurrencesByMessageIDs` into `ListMessages` service method. Add occurrence data to `messageToResponse` as raw identity (type + id + operation + fallback_snapshot passthrough). Zero viewer awareness yet. | None |
| **3D-2** | Typed Resource Projection Envelope | Define typed `ResourceProjection` struct with `state`, `resource_type`, `resource_id`, `canonical_url`, `capabilities`, typed payload. Wire per-type empty payloads. | 3D-1 |
| **3D-3** | Profile LIVE + Tombstone | Batch-query profile sources. Apply account_status + block logic. Emit LIVE / TOMBSTONE. | 3D-2 |
| **3D-4** | Content LIVE + Tombstone + Depth-1 | Batch-query Content sources. Apply visibility + moderation + block logic. Resolve `ShareReference` for depth-1 indicator. | 3D-2, 3D-3 (block batching) |
| **3D-5** | FPS LIVE + Capabilities | Batch-query FPS sources. Apply lifecycle logic. Emit LIVE with `can_interact`, `can_buy`, `can_chat`. | 3D-2 |
| **3D-6** | Auction LIVE + Capabilities | Batch-query Auction sources. Apply lifecycle logic. Emit LIVE with `can_interact`, `can_bid`, `can_chat`. | 3D-2 |
| **3D-7** | FALLBACK_ALLOWED Gate | Wire fallback_snapshot exposure only when LIVE cannot be produced AND privacy rules permit. | 3D-3, 3D-4, 3D-5, 3D-6 |
| **3D-8** | Legacy Residue Hard Purge | Remove `type=reference` from validator. Remove `validateReferenceAttachmentV2`. Remove `case "reference"` from service/handler switches. Clean up flat `fixed_price_sale`/`auction` attachment types. Update docs. | 3D-7 (all read projection complete) |

### Why this order

1. **3D-1** is the smallest possible change — just wire what already exists.
2. **3D-2** defines the contract before any resource-specific logic.
3. **3D-3 through 3D-6** are independent of each other and could be parallelized, but sequential is safer.
4. **3D-7** depends on all resource types being projected.
5. **3D-8** is the final cleanup — safe only after the new projection is fully operational.

### What does NOT change

- Migration 000034 schema (already sufficient)
- Write path (`SendMessage`, `authorizeOccurrence`, `CreateResourceOccurrence`)
- `fallback_snapshot` population logic
- `ResourceAuthorizer` port (write-side only; read-side uses domain services directly)
- `chat_commerce_references` (separate subsystem)
- Negotiation, shipping_quote, location attachments (separate attachment types)

---

## 16. TEST PLAN

### PostgreSQL matrix required for implementation closure

Each slice requires the following test categories:

#### 3D-1 (Occurrence Batch Load Wiring)

| Test ID | Scenario | Verification |
|---|---|---|
| 3D1-01 | Message with no occurrence → response has no resource_projection | Field absent |
| 3D1-02 | Message with Profile occurrence → response has resource_projection with type+id | Identity present |
| 3D1-03 | Message with Content occurrence → same | Identity present |
| 3D1-04 | Message with FPS occurrence → same | Identity present |
| 3D1-05 | Message with Auction occurrence → same | Identity present |
| 3D1-06 | 20 messages, 5 with occurrences (mixed types) → all identities correct | Batch correctness |
| 3D1-07 | occurrence row missing (FK violation impossible, but defensive) → graceful degradation | No panic |

#### 3D-2 (Typed Resource Projection Envelope)

| Test ID | Scenario | Verification |
|---|---|---|
| 3D2-01 | Projection envelope has state field | state present |
| 3D2-02 | Projection envelope has resource_type + resource_id | Identity present |
| 3D2-03 | Projection envelope has canonical_url | URL present |
| 3D2-04 | Projection envelope has capabilities (empty initially) | capabilities present |
| 3D2-05 | Projection envelope has exactly one typed payload | No null-soup |

#### 3D-3 (Profile LIVE + Tombstone)

| Test ID | Scenario | Verification |
|---|---|---|
| 3D3-01 | Active, non-blocked profile → LIVE with username/avatar/store | LIVE state |
| 3D3-02 | Suspended profile → TOMBSTONE | state=TOMBSTONE, no identity leak |
| 3D3-03 | Deleted profile → TOMBSTONE | state=TOMBSTONE, no identity leak |
| 3D3-04 | Blocked profile → TOMBSTONE (viewer-specific) | state=TOMBSTONE |
| 3D3-05 | Self profile share → LIVE even if viewer=profile | Identity present |
| 3D3-06 | 20 messages, 5 Profile occurrences (mixed active/suspended/blocked) → correct per-occurrence state | Batch mixed states |

#### 3D-4 (Content LIVE + Tombstone + Depth-1)

| Test ID | Scenario | Verification |
|---|---|---|
| 3D4-01 | Public, non-hidden, non-deleted Content → LIVE | LIVE state |
| 3D4-02 | Private Content, viewer=author → LIVE | LIVE state |
| 3D4-03 | Private Content, viewer≠author → TOMBSTONE | state=TOMBSTONE |
| 3D4-04 | Followers-only Content, viewer follows → LIVE | LIVE state |
| 3D4-05 | Followers-only Content, viewer doesn't follow → TOMBSTONE | state=TOMBSTONE |
| 3D4-06 | Deleted Content → TOMBSTONE | state=TOMBSTONE |
| 3D4-07 | Hidden Content → TOMBSTONE | state=TOMBSTONE |
| 3D4-08 | Moderated Content → TOMBSTONE | state=TOMBSTONE |
| 3D4-09 | Blocked author Content → TOMBSTONE | state=TOMBSTONE |
| 3D4-10 | Content with ShareReference to FPS → nested_resource present | Depth-1 indicator |
| 3D4-11 | Content with ShareReference to deleted Content → nested_resource suppressed | No tombstone leak |
| 3D4-12 | Content with ShareReference to blocked profile → nested_resource suppressed | No tombstone leak |
| 3D4-13 | Content with no ShareReference → no nested_resource | Field absent |
| 3D4-14 | 20 messages, 5 Content occurrences (mixed visibility) → correct per-occurrence state | Batch mixed states |

#### 3D-5 (FPS LIVE + Capabilities)

| Test ID | Scenario | Verification |
|---|---|---|
| 3D5-01 | Active FPS → LIVE with title, price, quantity, image, status | LIVE state |
| 3D5-02 | Active FPS → can_interact=true, can_buy=true | Capabilities correct |
| 3D5-03 | Sold FPS → LIVE with can_interact=false, can_buy=false | Terminal lifecycle LIVE |
| 3D5-04 | Inactive FPS → LIVE with can_interact=false | Terminal lifecycle LIVE |
| 3D5-05 | Deleted FPS → TOMBSTONE | Privacy removal |
| 3D5-06 | FPS from suspended seller → seller lifecycle in projection | Seller lifecycle |
| 3D5-07 | 20 messages, 5 FPS occurrences (mixed statuses) → correct per-occurrence | Batch correctness |

#### 3D-6 (Auction LIVE + Capabilities)

| Test ID | Scenario | Verification |
|---|---|---|
| 3D6-01 | Active Auction → LIVE with title, current_bid, image, status, end_time | LIVE state |
| 3D6-02 | Active Auction → can_interact=true, can_bid=true | Capabilities correct |
| 3D6-03 | Ended Auction → LIVE with can_interact=false, can_bid=false | Terminal lifecycle LIVE |
| 3D6-04 | Cancelled Auction → LIVE with can_interact=false | Terminal lifecycle LIVE |
| 3D6-05 | Deleted Auction → TOMBSTONE | Privacy removal |
| 3D6-06 | 20 messages, 5 Auction occurrences (mixed statuses) → correct per-occurrence | Batch correctness |

#### 3D-7 (FALLBACK_ALLOWED Gate)

| Test ID | Scenario | Verification |
|---|---|---|
| 3D7-01 | Profile LIVE fails (DB error) but privacy permits → FALLBACK_ALLOWED with historical data | Fallback used |
| 3D7-02 | Content LIVE fails (deleted) → TOMBSTONE not FALLBACK_ALLOWED (privacy) | Fallback NOT used |
| 3D7-03 | Content LIVE fails (DB error) but public → FALLBACK_ALLOWED with caption excerpt | Fallback used |
| 3D7-04 | FPS LIVE fails (DB error) → FALLBACK_ALLOWED with title/image/store | Fallback used |
| 3D7-05 | FALLBACK_ALLOWED does NOT expose price/bid/quantity/status | No live data leak |

#### 3D-8 (Legacy Residue Hard Purge)

| Test ID | Scenario | Verification |
|---|---|---|
| 3D8-01 | Send message with type=reference → 400 rejected | Validator rejects |
| 3D8-02 | Old messages with type=reference → still readable (attachment_json preserved) | Backward compat |
| 3D8-03 | All existing test suites pass | No regressions |

### Integration test requirements

- All tests must run against a real PostgreSQL database (`-tags=integration`)
- Each test must seed its own data and clean up
- Query count assertions: use `EXPLAIN ANALYZE` or pgx tracing to verify bounded queries
- Concurrent read tests: verify no read-during-write anomalies

---

## 17. BLOCKERS / OWNER DECISIONS

### No genuine blockers

The following are design decisions already locked by the scope document. No owner
questions needed:

| Decision | Locked by |
|---|---|
| Resource types: profile, content, FPS, auction only | Scope §3 |
| Typed projection envelope (not null-soup) | Scope §4 |
| LIVE must use current canonical authority | Scope §5 |
| FALLBACK_ALLOWED: historical display only, no live authority | Scope §6 |
| TOMBSTONE cases | Scope §7 |
| Content depth-1 only | Scope §8 |
| Room commerce context is separate | Scope §9 |
| No schema migration for projection convenience | Scope §14 |

### The one owner decision

**Should `attachment_json` keep its legacy `type=reference` support after the new
resource occurrence read projection is live?**

This is a rollout strategy question, not an architecture question:
- **Option A (hard cut)**: Once 3D-7 is verified, purge reference type from attachment
  validation and remove all `case "reference"` branches. Old messages keep their
  `attachment_json` data but mobile stops sending it.
- **Option B (grace period)**: Keep reference type valid but deprecated for N weeks
  while mobile migrates to the new projection envelope.

This decision belongs in 3D-8 and can be deferred until the projection is verified.
It does not block 3D-1 through 3D-7.

---

## 18. RECOMMENDATION

### First implementation slice: 3D-1 (Occurrence Batch Load Wiring)

**Rationale:**
1. Smallest possible change (~50 lines of Go across 3 files)
2. No viewer awareness needed — just wire what already exists
3. Immediately proves the read path works end-to-end
4. Zero risk of regression (no existing behavior changes)
5. Unblocks all subsequent slices

**Files to modify:**
1. `chat_service.go` — `ListMessages`: add `GetResourceOccurrencesByMessageIDs` call
   after `ListMessagesByRoom`
2. `chat_handler.go` — `ListMessages`: pass occurrences map through to
   `messageToResponse`
3. `chat_handler.go` — `messageToResponse`: add `resource_occurrence` field with
   raw identity (type, id, operation) and fallback_snapshot passthrough when present

**Concrete change:**
```go
// In chat_service.go ListMessages:
occurrences, err := s.repo.GetResourceOccurrencesByMessageIDs(ctx, tx, messageIDs)
// degrade gracefully on error, pass to handler

// In messageToResponse:
if occ, ok := occurrences[msg.ID]; ok {
    resp["resource_occurrence"] = map[string]interface{}{
        "operation":     string(occ.Operation),
        "resource_type": string(occ.ResourceType()),
        "resource_id":   occ.SourceID().String(),
        "fallback_snapshot": occ.FallbackSnapshot,
    }
}
```

**Test requirement:** 7 test cases (see §16, 3D-1 matrix).

**Estimated effort:** 1 implementation session.

---

**STOP.** Audit complete. No implementation performed.
