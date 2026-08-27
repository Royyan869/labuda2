# UNIFIED SHARE SCOPE 3B2 — CHAT MESSAGE RESOURCE OCCURRENCE FINAL DESIGN LOCK

**Scope ID:** `UNIFIED_SHARE_SCOPE_3B2`
**Date:** 2026-08-08
**Mode:** FINAL DESIGN LOCK — NO IMPLEMENTATION
**Supersedes:** Scope 3B1 design correction

---

## 1. VERDICT

**READY_FOR_SCOPE_3_IMPLEMENTATION**

All design elements are locked. One owner decision is resolved as LOCKED_DEPTH_1 (see §4).

---

## 2. FINAL RESPONSE-STATE SEMANTICS

Server determines ONE canonical projection state per occurrence. Client renders the state supplied by server. No dual fallback+live as competing client authorities.

### States

```
LIVE
  Resource is accessible and server can resolve current live projection.
  → live typed payload present
  → fallback is SERVER-INTERNAL (stored in DB, not exposed to client)
  → viewer capabilities derived from live authority:
      * can_view = true
      * can_interact = true/false (resource may be viewable but not actionable)

FALLBACK_ALLOWED
  Live projection genuinely cannot be produced (resource physically gone,
  author deleted, etc.) AND exposing historical display is legally/product-wise
  permitted AND condition is NOT privacy/moderation/block removal.
  → live payload absent
  → fallback present (server-built historical display)
  → viewer capabilities:
      * can_view = true
      * can_interact = false

TOMBSTONE
  Privacy, moderation, or block rule requires suppression.
  → live payload absent
  → fallback suppressed (not exposed to client — DB may retain for audit)
  → redacted/tombstone metadata only:
      * resource_type
      * tombstone_reason (where safe to expose)
      * viewer_capabilities.blocked_by_tombstone = true
```

### Single Typed Envelope

```json
{
  "id": "uuid",
  "room_id": "uuid",
  "sender_id": "uuid",
  "message_type": "text",
  "body": "optional",
  "created_at": "ISO8601",

  "resource_occurrence": {
    "operation": "share_to_chat",
    "resource_type": "profile",
    "resource_id": "uuid",
    "canonical_url": "/user/{id}",

    "state": "LIVE | FALLBACK_ALLOWED | TOMBSTONE",

    "live": { /* typed payload — present only when state == LIVE */ },
    "fallback": { /* historical — present only when state == FALLBACK_ALLOWED */ },

    "viewer_capabilities": {
      "can_view": true,
      "can_interact": true | false,
      "blocked_by_tombstone": true | false
    }
  }
}
```

### Critical: LIVE does NOT expose fallback

When state is LIVE, `fallback` is absent from the response. The live payload is the single client rendering authority. The fallback remains stored in the database for potential future FALLBACK_ALLOWED or TOMBSTONE transitions, but it is never a second UI authority alongside live data.

---

## 3. FINAL LIFECYCLE / TOMBSTONE MATRIX

### Profile

| Resource Condition | Viewer Condition | State |
|---|---|---|
| Active | Not blocked | LIVE |
| Suspended | Not blocked | TOMBSTONE (temporary redacted — canonical suspended treatment) |
| Banned | Any | TOMBSTONE |
| Deleted | Any | TOMBSTONE — fallback suppressed (privacy) |
| Any | Viewer blocked by profile owner | TOMBSTONE |
| Any | Viewer blocked profile owner | TOMBSTONE |

### Content

| Resource Condition | Viewer Condition | State |
|---|---|---|
| Public + active | Not blocked | LIVE |
| Followers-only + viewer follows | Not blocked | LIVE |
| Followers-only + viewer does not follow | Any | TOMBSTONE |
| Private + viewer is not author | Any | TOMBSTONE |
| Hidden/moderated | Any | TOMBSTONE |
| Deleted | Any | TOMBSTONE — fallback suppressed |
| Author banned | Any | TOMBSTONE |
| Any | Viewer blocked by author | TOMBSTONE |
| Any | Viewer blocked author | TOMBSTONE |

### FixedPriceSale

| Resource Condition | Viewer Condition | State |
|---|---|---|
| Active | Not blocked | LIVE |
| Sold | Not blocked | LIVE (publicly viewable — can_interact = false) |
| Withdrawn | Not blocked | LIVE (publicly viewable — can_interact = false) |
| Draft | Owner only | LIVE for owner; TOMBSTONE for others |
| Seller suspended | Any | TOMBSTONE |
| Seller banned | Any | TOMBSTONE |
| Any | Viewer blocked by seller | TOMBSTONE |
| Any | Viewer blocked seller | TOMBSTONE |

Commerce sold/ended/withdrawn/cancelled resources that remain legitimately publicly viewable stay LIVE. Their `viewer_capabilities.can_interact = false` signals they are not actionable. FALLBACK_ALLOWED is NOT used for normal terminal commerce states — it is reserved for when live projection genuinely cannot be produced.

### Auction

| Resource Condition | Viewer Condition | State |
|---|---|---|
| Scheduled | Not blocked | LIVE |
| Active | Not blocked | LIVE |
| Ended | Not blocked | LIVE (publicly viewable — can_interact = false) |
| Cancelled | Not blocked | LIVE (publicly viewable — can_interact = false) |
| ExpiredBNR | Not blocked | LIVE (publicly viewable — can_interact = false) |
| Seller suspended | Any | TOMBSTONE |
| Seller banned | Any | TOMBSTONE |
| Any | Viewer blocked by seller | TOMBSTONE |
| Any | Viewer blocked seller | TOMBSTONE |

### FALLBACK_ALLOWED Triggers (Rare)

FALLBACK_ALLOWED is used ONLY when ALL of:
1. Live projection genuinely cannot be produced (resource row physically gone, author identity row missing, etc.)
2. Exposing historical display is legally/product-wise permitted
3. Condition is NOT privacy, moderation, or block removal

Examples:
- Content author physically deleted from `users` table but Content remains → FALLBACK_ALLOWED may show historical author display
- FPS product image storage key expired/deleted but listing row remains → LIVE (live projection still works, just without image)

---

## 4. FINAL CONTENT NESTING RULE

**LOCKED_DEPTH_1**

The owner has explicitly approved:

1. Chat Message → exact shared Content (primary)
2. If that Content carries a canonical `ShareReference`, a compact immediate nested resource indicator is shown
3. No expansion of the nested resource's own nested resource
4. Outer Content navigation always: `/content/{selected-content-id}`
5. Nested indicator navigation only when live viewer access permits the nested resource
6. If nested resource is inaccessible/private/deleted/blocked: redacted indicator or omitted entirely — do NOT expose historical title from nested resource

**Backend:** The Content's `ShareReference` is resolved from the live Content entity at read time. The immutable Chat occurrence fallback stores NO nested resource metadata — no `has_nested_resource`, no `nested_resource_type`, no `nested_resource_id`. Nested identity is authority of the Content's own canonical reference, not copied into Chat occurrence.

---

## 5. FINAL OCCURRENCE SCHEMA

```sql
CREATE TYPE chat_resource_occurrence_operation_enum AS ENUM (
    'share_to_chat',
    'direct_commerce_insert_chat'
);

CREATE TABLE chat_message_resource_occurrences (
    message_id uuid PRIMARY KEY REFERENCES chat_messages(id) ON DELETE CASCADE,

    operation chat_resource_occurrence_operation_enum NOT NULL,

    -- Exactly one typed source FK. Non-null FK IS the type authority.
    profile_source_id           uuid REFERENCES users(id) ON DELETE RESTRICT,
    content_source_id           uuid REFERENCES contents(id) ON DELETE RESTRICT,
    fixed_price_sale_source_id  uuid REFERENCES fixed_price_sales(id) ON DELETE RESTRICT,
    auction_source_id           uuid REFERENCES auctions(id) ON DELETE RESTRICT,

    -- Server-built immutable display fallback (DB-resident; exposed only in
    -- FALLBACK_ALLOWED state; server-internal in LIVE state; suppressed in
    -- TOMBSTONE state)
    fallback_snapshot jsonb NOT NULL,

    created_at timestamp with time zone NOT NULL DEFAULT now(),

    CONSTRAINT exactly_one_source CHECK (
        (CASE WHEN profile_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN content_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN fixed_price_sale_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN auction_source_id IS NOT NULL THEN 1 ELSE 0 END) = 1
    ),

    CONSTRAINT valid_operation_for_resource CHECK (
        (operation = 'direct_commerce_insert_chat' AND
            (fixed_price_sale_source_id IS NOT NULL OR auction_source_id IS NOT NULL))
        OR
        (operation = 'share_to_chat')
    )
);

-- Partial indexes per source type
CREATE INDEX idx_chat_occurrence_profile ON chat_message_resource_occurrences
    (profile_source_id, created_at DESC) WHERE profile_source_id IS NOT NULL;
CREATE INDEX idx_chat_occurrence_content ON chat_message_resource_occurrences
    (content_source_id, created_at DESC) WHERE content_source_id IS NOT NULL;
CREATE INDEX idx_chat_occurrence_fps ON chat_message_resource_occurrences
    (fixed_price_sale_source_id, created_at DESC) WHERE fixed_price_sale_source_id IS NOT NULL;
CREATE INDEX idx_chat_occurrence_auction ON chat_message_resource_occurrences
    (auction_source_id, created_at DESC) WHERE auction_source_id IS NOT NULL;
```

**Not stored:**
- `actor_id` — derived from `chat_messages.sender_id`
- `room_id` — derived from `chat_messages.room_id`
- `source_type` enum — FK presence is the type authority
- `tombstone` / `is_tombstoned` flag — state is live-computed per viewer
- `backfilled` flag — no backfill exists
- `client_preview` — server builds fallback only

---

## 6. FINAL QUERY-PROOF MATRIX

### Conceptual Query Classes

| Class | Description | When |
|---|---|---|
| C1 | Message page (cursor pagination, reply/media joins, sender identity) | Always |
| C2 | Occurrence batch (`WHERE message_id = ANY($1)`) | Always |
| C3 | Profile live hydration (users + user_profiles + seller_profiles) | Profile occurrences present |
| C4 | Content live hydration (contents + ordered content_media + author identity + depth-1 nested resource resolution from Content.ShareReference) | Content occurrences present |
| C5 | FPS live hydration (fixed_price_sales + products + product_media + seller identity) | FPS occurrences present |
| C6 | Auction live hydration (auctions + products + product_media + seller identity) | Auction occurrences present |
| C7 | Viewer block/access state (viewer ↔ all resource owners) | Any occurrence present |

### Scenarios

| # | Scenario |
|---|---|
| S0 | Normal page — no occurrences |
| S1 | 1 Profile occurrence |
| S2 | 20 Profile occurrences (page saturation) |
| S3 | 1 Content occurrence |
| S4 | 20 Content occurrences |
| S5 | 1 FPS occurrence |
| S6 | 20 FPS occurrences |
| S7 | 1 Auction occurrence |
| S8 | 20 Auction occurrences |
| S9 | 1 each mixed (4 occurrences, all types) |
| S10 | 20 each mixed (80 occurrences, all types) |

### Invariants (to be proven at implementation)

1. **Type-constant:** Within the same resource-type mixture (S1 vs S2, S3 vs S4, etc.), query count does not grow with occurrence row count.
2. **No N+1:** Zero per-occurrence or per-message resource/seller/media queries.
3. **Page bound:** Every scenario fits within the existing message page-size limit (default 50, max 100).
4. **Batch resolution:** All resource hydration uses `WHERE id = ANY($1)` with uuid arrays.

Actual integer query counts are implementation proof artifacts, not design assumptions.

---

## 7. FINAL SWITCH / PURGE SEMANTICS

### No Deployed Compatibility Window

During implementation, old and new code may coexist temporarily in the **developer filesystem** so the coherent slice can be built incrementally.

There is NO acceptable deployed/closed state where:
- new `resource_occurrence` writes are live
- AND `attachment_json.type=reference` remains a supported resource write authority

### Release Sequence

```
1. Migration 000034 (additive, inert)
2. Implement canonical backend (write + read + outbox)
3. Implement matching canonical mobile (producer + consumer)
4. Execute focused switch proof (all occurrences pass, old reference path still works)
5. IMMEDIATELY reject attachment_json.type=reference (400 typed error)
6. Remove attachment_json reference validator branch
7. Remove Chat resource-reference DTO/parser/helper code
8. Delete dead code inventory
9. Convert legacy reference tests to negative contracts
10. Final regression
```

### Post-Purge attachment_json

`attachment_json` column remains for non-resource message semantics:
- `negotiation_offer` / `negotiation_proposal` / `negotiation_result`
- `shipping_quote`
- `location`

`attachmentvalidator.ValidateAttachmentJSON` removes the `reference` type from its allowlist.

### Cross-field Rejection

After purge, `SendMessage` with BOTH `resource_occurrence` AND `attachment_json` → 400. No combined resource+non-resource attachment in initial canonical scope unless a proven business flow requires it.

---

## 8. FINAL CLEANUP BOUNDARY

### DELETE_DURING_SCOPE_3 — Chat resource-reference authority ONLY

| Item | Location |
|---|---|
| `attachment_json.type=reference` validator branch | `attachmentvalidator/validator.go` |
| `validateAttachmentReferences` (reference existence check) | `chat_service.go` |
| `validateFixedPriceSaleReferenceExists` | `chat_service.go` |
| `validateAuctionReferenceExists` | `chat_service.go` |
| `ErrAttachmentFixedPriceSaleNotFound` | `chat_repository.go` |
| `ErrAttachmentAuctionNotFound` | `chat_repository.go` |
| `ShareReference.chatWireTargetType` | `share_reference.dart` |
| `ShareReference.asChatReference()` | `share_reference.dart` |
| `ChatMapper` resource reference conversion | `chat_mapper.dart` |
| `attachment_dto` reference docs/DTOs | `attachment_dto.dart` |
| `ChatGateway` (both copies, 0 consumers) | `chat_gateway.dart` ×2 |
| `LinkOrderToChatUseCase` (0 importers) | `link_order_to_chat_usecase.dart` |
| `GetFixedPriceSaleShareReferenceUseCase` (0 importers, both copies) | `get_listing_share_reference_usecase.dart` ×2 |
| `CreateChatDto` (orphaned) | `chat_dto.dart` |
| `ChatListDto` (bypassed) | `chat_dto.dart` |
| `ObjectReference` wire alias in chat mapper | `chat_mapper.dart` |
| Legacy `offer_reference` fallback | `chat_mapper.dart` |
| `chat_entities.dart` legacy payload defaults | `chat_entities.dart` |
| `object_reference_bridge.dart` stale comment | `object_reference_bridge.dart` |
| Backend nil-checker skip ("backward compatibility") | `chat_service.go` |

### NOT DELETED — independently canonical

| Item | Rationale |
|---|---|
| `negotiation_*` attachment types | Live non-resource chat functionality |
| `shipping_quote` attachment type | Live non-resource chat functionality |
| `location` attachment type | Live non-resource chat functionality |
| `attachment_json` column | Still serves non-resource attachments |
| `attachmentvalidator.ValidateAttachmentJSON` | Still validates non-reference types |
| `ShareReference` entity | Used by Content repost, Feed, Comment |
| `ObjectReference` / `ObjectPreviewCard` | Used by Feed, Content detail |
| `widget_lib.AttachmentWidget` | Used by negotiation/shipping/location |
| Room-list response legacy format compat | Independent backward-compat concern |
| Realtime envelope legacy parsing | Independent backward-compat concern |

---

## 9. REMAINING AMBIGUITIES

**None.** All design elements are locked. Content nesting is LOCKED_DEPTH_1.

The query-count matrix defines conceptual classes and invariants. Actual integer counts are implementation proof artifacts.

---

## 10. RECOMMENDATION

**READY_FOR_SCOPE_3_IMPLEMENTATION**

All design elements are finalized:
- ✅ Response state: LIVE (no fallback exposed), FALLBACK_ALLOWED, TOMBSTONE
- ✅ Commerce sold/ended/withdrawn/cancelled → LIVE with can_interact=false
- ✅ FALLBACK_ALLOWED only for genuine live-projection failure (not normal terminal states)
- ✅ Privacy/banned/deleted/blocked → TOMBSTONE (fallback suppressed)
- ✅ Content nesting: LOCKED_DEPTH_1
- ✅ Occurrence schema: message_id PK, no actor_id/room_id/source_type/tombstone flag
- ✅ Query: conceptual classes defined, invariants specified, counts are proof artifacts
- ✅ Switch: no deployed compatibility window, immediate purge after proof
- ✅ Cleanup: Chat resource-reference only, non-resource attachments preserved
- ✅ Next migration: 000034

**STOP.** Design fully locked. No implementation performed.
