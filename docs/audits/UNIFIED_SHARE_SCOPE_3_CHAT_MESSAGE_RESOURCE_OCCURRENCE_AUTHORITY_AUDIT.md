# UNIFIED SHARE SCOPE 3 — CHAT MESSAGE RESOURCE OCCURRENCE AUTHORITY AUDIT

**Scope ID:** `UNIFIED_SHARE_SCOPE_3_CHAT_MESSAGE_RESOURCE_OCCURRENCE_AUTHORITY_AUDIT`
**Date:** 2026-08-08
**Mode:** FRESH AUDIT OF CURRENT FILESYSTEM — AUDIT ONLY — NO IMPLEMENTATION
**Repository:** `d:\Project\labuda`
**Audit method:** 7 parallel exploration agents across backend schema, backend services/handlers, backend idempotency/outbox, backend room commerce context, mobile API/DTOs, mobile producers/consumers, tests/legacy residue

---

## 1. VERDICT

**Current chat message resource authority is `attachment_json` on `chat_messages`, with type `reference` supporting only `fixed_price_sale` and `auction`. No `chat_message_resource_occurrences` table exists. Profile and Content cannot be shared to Chat. Room commerce context (`chat_commerce_references`) is correctly separated from message attachment authority. The architecture has clean separation between room-scoped commerce context and message-scoped attachment, but the canonical Unified Share `chat_message_resource_occurrences` model is entirely absent from the codebase.**

---

## 2. CURRENT BACKEND AUTHORITY

### 2.1 Message Resource Attachment Authority

**Canonical authority:** `chat_messages.attachment_json` (jsonb column, migration `000001_canonical_schema.up.sql:584`)

**Validated attachment types** (`attachmentvalidator/validator.go:34-87`):
- `reference` — resource share (fixed_price_sale, auction only)
- `negotiation_offer`
- `negotiation_proposal`
- `negotiation_result`
- `shipping_quote`
- `location`

**Reference attachment schema** (`validator.go:89-140`):
```json
{
  "type": "reference",
  "data": {
    "target_type": "fixed_price_sale" | "auction",
    "target_id": "<uuid>",
    "preview": {
      "title": "...",
      "imageUrl": "...",
      "isAvailable": true/false,
      "isSold": true/false,
      "isClosed": true/false,
      "isDeleted": true/false
    }
  }
}
```

**Critical:** `target_type` accepts ONLY `fixed_price_sale` and `auction`. Content and Profile are rejected at the schema validation layer (`attachment_validator_test.go:124-135` proves legacy wire types including `content`/`profile` are rejected).

### 2.2 SendMessage Flow (Backend)

**Handler:** `chat_handler.go:1300-1457`
- `SendMessageRequest` binding: `message_type ∈ {text, negotiation_proposal, system}`, `idempotency_key` required, `attachment_json` optional
- Handler calls `ValidateAttachmentJSON(req.AttachmentJSON)` (line 1333) — strict schema validation
- All inside single `WithTx`

**Service:** `chat_service.go:1056-1281`
- Rate limits: 5 msgs/5s, 60 msgs/min per sender
- Room participant check → block check (sender→recipient, with order-context and support-room exemptions)
- `validateAttachmentReferences` — existence check of referenced listing/auction IDs against live tables; skipped entirely if checkers are nil ("backward compatibility")
- Idempotency lookup → `CreateMessage` → `UpdateRoomLastMessageAt` → read state upsert → outbox events

**Message types in enum:** `text`, `negotiation_proposal`, `shipping_quote`, `system` — NO `share` or `commerce_insert` type exists

### 2.3 Endpoints That Do NOT Exist

| Endpoint | Status |
|---|---|
| `POST /chat/rooms/{id}/share` | Does not exist |
| `POST /chat/rooms/{id}/insert-commerce` | Does not exist |
| `POST /chat/rooms/{id}/resource-occurrence` | Does not exist |
| `POST /chat/share` | Does not exist |
| `ShareToChat` handler | Does not exist |
| `InsertCommerceToChat` handler | Does not exist |

The only commerce-in-chat endpoints are:
- `POST /chat/rooms/{room_id}/commerce-references` — room-scoped commerce context (NOT message attachment)
- `PUT /chat/rooms/{room_id}/link-order` — order↔room linkage

### 2.4 Projection / Message Rendering

**`messageToResponse`** (`chat_handler.go:2020-2119`):
- Emits `msg.AttachmentJSON` verbatim as `attachment_json` in response
- Adds `sender_lifecycle` from identity projection
- Adds `attachment_seller_trust_lifecycle` from attachment's referenced seller
- Never reads `chat_commerce_references` for rendering

**Tombstoned messages** (soft-deleted): render as `{id, room_id, sender_id, message_type, created_at, is_hidden: true}` — body and attachment suppressed

---

## 3. CURRENT DATABASE MODEL

### 3.1 Tables (6 chat tables)

| Table | Migration | Purpose |
|---|---|---|
| `chat_rooms` | 000001 | Room with participants, linked_order_id; `context_json`/`context_set_by` DROPPED by 000030 |
| `chat_messages` | 000001 + 000027 | Messages with `attachment_json`, `idempotency_key` UNIQUE, `reply_to_message_id`, `reply_preview_json` |
| `chat_read_states` | 000001 | Per-user last-read tracking |
| `chat_media_assets` | 000027 | Media upload lifecycle |
| `chat_message_media_assets` | 000027 | Message↔media join |
| `chat_commerce_references` | 000029 | Room-scoped immutable commerce context |

### 3.2 chat_messages Columns (Complete)

```sql
id uuid PK DEFAULT gen_random_uuid()
room_id uuid FK → chat_rooms CASCADE
sender_id uuid FK → users CASCADE
message_type chat_message_type_enum NOT NULL  -- text|negotiation_proposal|system|shipping_quote
body text
attachment_json jsonb                        -- THE CURRENT RESOURCE AUTHORITY
reply_to_message_id uuid FK → chat_messages SET NULL
reply_preview_json jsonb
idempotency_key text NOT NULL UNIQUE          -- GLOBAL uniqueness, not scoped
created_at timestamp with time zone
deleted_at timestamp with time zone           -- moderation soft-hide
deleted_by uuid FK → users SET NULL
deletion_reason text
```

### 3.3 chat_commerce_references

```sql
id uuid PK
room_id uuid FK → chat_rooms CASCADE
target_type chat_commerce_reference_target_type_enum  -- fixed_price_sale|auction
target_id uuid NOT NULL
creator_id uuid FK → users CASCADE
display_snapshot jsonb NOT NULL              -- immutable display fallback
created_at timestamp with time zone
UNIQUE (room_id, target_type, target_id)
```

**Behavior:** `INSERT ... ON CONFLICT DO NOTHING` — immutable; no UPDATE/DELETE paths exist; no `updated_at` column

### 3.4 What Does NOT Exist

- ❌ `chat_message_resource_occurrences` — zero matches in entire repo
- ❌ `chat_attachments` — zero matches
- ❌ `chat_references` — zero matches
- ❌ Any `share_reference_id` or `commerce_reference_id` column on `chat_messages`
- ❌ Any `resource_occurrence` table or type

---

## 4. CURRENT MOBILE PRODUCERS

### 4.1 Active Entry Points INTO Chat

| Producer | Entry Point | Resource Carried | Lines |
|---|---|---|---|
| Listing detail → Chat | `openCommerceChat(reference: ShareReference.fixedPriceSale(...))` | FPS ShareReference | `listing_detail_screen.dart:592-621` |
| Listing detail → Nego | `openCommerceChat(..., autoOpenNegotiation: true)` | FPS ShareReference | `listing_detail_screen.dart:623-653` |
| Auction detail → Chat | `openCommerceChat(reference: ShareReference.auction(...))` | Auction ShareReference | `auction_detail_screen.dart:642-661` |
| Chat attachment button → "Bagikan Listing" | `ListingPickerBottomSheet` → `_sendReferenceMessage` | FPS ShareReference | `chat_detail_screen.dart:1621-1677` |
| Create Listing return → Chat | `_navigateToCreateListing()` → pop result → `_sendFixedPriceSaleAttachment` | FPS ShareReference | `chat_detail_screen.dart:1680-1700` |
| Profile → Chat | `getOrCreateChat(userId, targetUserId)` | None (plain room creation) | `profile_screen.dart:1192-1205` |
| Checkout → Chat seller | `getOrCreateChat` + `context.push('/chat/{id}')` | None | `checkout_screen_impl.dart:969-1047` |
| Order → Chat | `context.push('/chat/{id}')` | None | `order_widgets_impl.dart:125` |
| Notification deep-link → Chat | `navigateToChatConversation(chatRoomId)` | None | `notification_navigation_service.dart:211-217` |

### 4.2 Central Commerce Chat Funnel

**`commerce_chat_navigation.dart:10-84`** (`openCommerceChat`):
1. Auth gate
2. Self-seller guard (can't chat with yourself about your own listing)
3. `reference.asChatReference()` — normalizes; **only FPS and Auction pass** (content/profile return null)
4. `getOrCreateChat(userId, sellerId)`
5. `api.createCommerceReference(chat.id, {target_type, target_id, preview})` — separate API call
6. `router.push('/chat/{id}?referenceId=...')`

### 4.3 NOT Implemented

| Producer | Status |
|---|---|
| Share sheet "Send to Chat" | **DEAD CODE** — `ShareDestination.sendToChat` declared but excluded from `_getShareDestinations()`, marked "PHASE 2 HARDENING: not implemented" |
| Content share → Chat | **NOT IMPLEMENTED** — Content shares only through social share (feed/external). `ShareReference.chatWireTargetType` returns null for `content` |
| Profile share → Chat | **NOT IMPLEMENTED** — Profile→Chat is direct room creation, not a message resource occurrence. No "share profile to chat" flow exists |

---

## 5. CURRENT MOBILE CONSUMERS

### 5.1 Message Rendering Chain

```
MessageDto.fromJson (tolerant: attachment || attachment_json)
  → ChatMapper.toDomain
    → Message.objectReference (ShareReference) ← canonical domain field
      → MessageBubble._buildAttachment
        → ObjectPreviewCard (for objectReference: ShareReference)
        → widget_lib.AttachmentWidget (for negotiation/shipping/location — legacy)
```

### 5.2 ObjectPreviewCard Behavior

**File:** `shared/object/presentation/widgets/object_preview_card.dart`

**HARD LIVE merge logic:**
- **Status/price:** live-only (fetched from current resource state)
- **Title/image:** snapshot-fallback (uses attachment preview if live fetch fails)
- **Tap:** disabled when resource is unavailable (line 139)
- **Batch mode:** `MessagesBatchWidget` collects all `objectReference`s → `objectPreviewBatchProvider` → passes `preResolved` into each `MessageBubble`

### 5.3 Room Commerce Context Banner

**`chat_detail_screen.dart:601-716`** — `_buildListingContextBanner`:
- Reads from `commerceReferencesByRoomProvider`
- Shows "Diskusi Pembelian"/"Diskusi Lelang" + thumbnail + "Lihat Detail Listing/Lelang"
- Separate from message rendering; does not infect message bubbles

### 5.4 Optimistic State & Retry

- **No optimistic message insert** — strictly wait-for-server-then-append
- **No in-thread retry** — failures surface as SnackBars
- **Text drafts NOT preserved** across screen dispose
- **Media drafts preserved** with upload progress tracking, retry, cancel
- **Reply targets preserved** across renders

### 5.5 Duplicate/Legacy Consumer Paths

- **`ChatWebSocketHandler`** (`core/src/websocket/chat_websocket_handler.dart`) — separate legacy WS handler with own wire types, not wired into `ChatRepositoryImpl`. Still registered in `service_locator.dart:114-116` but its events are ignored by the canonical repository path
- **`widget_lib.AttachmentWidget`** — legacy widget still used for negotiation/shipping/location attachments (`message_bubble.dart:522`)

---

## 6. ROOM CONTEXT VS MESSAGE OCCURRENCE FINDINGS

### 6.1 Current Separation

| Concern | Room Commerce Context | Message Resource Occurrence |
|---|---|---|
| **Storage** | `chat_commerce_references` table | `chat_messages.attachment_json` column |
| **Scope** | Room-level (one per room+target_type+target_id) | Message-level (per-message attachment) |
| **Created by** | `POST /chat/rooms/{id}/commerce-references` (client-initiated, separate from room creation) | `POST /chat/rooms/{id}/messages` with `attachment_json` |
| **Mutability** | Immutable (INSERT ON CONFLICT DO NOTHING) | Immutable after creation (moderation can tombstone) |
| **Read by message renderer** | NO — `messageToResponse` never reads commerce_references | YES — `attachment_json` is the render authority |
| **Resource types** | `fixed_price_sale`, `auction` | `fixed_price_sale`, `auction` (through reference attachment) |
| **Authorization** | Seller-of-target must be other participant (`ErrOrderRoomParticipantMismatch`) | No ownership required for ordinary send (only participant membership) |

### 6.2 Architectural Assessment

**Good:** Room commerce context and message attachment are separate concerns with separate storage, separate endpoints, and separate rendering paths. Sending a message does NOT mutate room commerce context. Message rendering does NOT read room commerce context.

**Gap:** Both currently support only `fixed_price_sale` and `auction`. Neither supports Profile or Content. The locked Unified Share architecture requires Profile and Content support on the message occurrence side.

**Gap:** Room creation and commerce-reference creation are two non-atomic API calls. The mobile `openCommerceChat` function calls `getOrCreateChat` then `createCommerceReference` sequentially — a failure between them leaves a room without commerce context.

---

## 7. SHARE_TO_CHAT CURRENT MATRIX

| Resource Type | Backend Endpoint | Backend Validator | Mobile Producer | Status |
|---|---|---|---|---|
| Profile | ❌ None | ❌ Rejected by attachment validator (`target_type` must be `fixed_price_sale\|auction`) | ❌ `ShareReference.chatWireTargetType` returns null for `profile` | **NOT SUPPORTED** |
| Content | ❌ None | ❌ Rejected by attachment validator | ❌ `ShareReference.chatWireTargetType` returns null for `content`; `sendToChat` share destination is dead code | **NOT SUPPORTED** |
| FixedPriceSale | ✅ `POST /chat/rooms/{id}/messages` with `attachment_json.type=reference` | ✅ Accepted | ✅ `openCommerceChat` + `ListingPickerBottomSheet` + chat attachment button | **SUPPORTED** |
| Auction | ✅ `POST /chat/rooms/{id}/messages` with `attachment_json.type=reference` | ✅ Accepted | ✅ `openCommerceChat` + auction detail Chat button | **SUPPORTED** |

---

## 8. DIRECT_COMMERCE_INSERT_CHAT CURRENT MATRIX

| Resource Type | Backend Endpoint | Authorization | Mobile Producer | Status |
|---|---|---|---|---|
| FixedPriceSale | ✅ `POST /chat/rooms/{id}/messages` (same as SHARE_TO_CHAT — no distinct "insert" endpoint) | No ownership check for attachment; only participant membership | ✅ Chat attachment button → ListingPicker → `_sendReferenceMessage` | **SAME PATH AS SHARE** |
| Auction | ✅ Same — no distinct path | Same | ❌ No "insert auction from chat" UI found | **SAME PATH AS SHARE** |

**Critical finding:** There is NO distinct `DIRECT_COMMERCE_INSERT_CHAT` path. The current "insert commerce into chat" flow (chat attachment button → listing picker) uses the EXACT same `SendMessage` endpoint with `attachment_json.type=reference` as any ordinary message with an attachment. The locked Unified Share requirement for distinct authorization (actor must own resource, market-increasing authority) does NOT exist in current code — there is no ownership check on attachment references, only existence validation.

**Gap:** Seller-created FPS from Chat (open canonical Create Listing flow → return FPS identity → attach → explicit Send) is implemented on mobile (`_navigateToCreateListing` → pop result → `_sendFixedPriceSaleAttachment`) but the backend has no awareness that this is a seller-owned insertion — it treats it identically to any user attaching any listing.

---

## 9. AUTHORIZATION FINDINGS

### 9.1 Current Authorization Matrix (Actual)

| Operation | Login Required | Email Verified | Active Account | Ownership | Market Authority | Room Membership | Block Check | Resource Visibility |
|---|---|---|---|---|---|---|---|---|
| SHARE_TO_CHAT FPS (via SendMessage) | ✅ (auth middleware) | ❌ Not checked separately | ✅ `RequireActiveAccount` | ❌ None | ❌ None | ✅ `room.HasParticipant` | ✅ sender→recipient (with order-context exemption) | ❌ None (only existence check, skip if checker nil) |
| SHARE_TO_CHAT Auction | ✅ | ❌ | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ (same skip-if-nil) |
| SHARE_TO_CHAT Profile | N/A — not implemented | | | | | | | |
| SHARE_TO_CHAT Content | N/A — not implemented | | | | | | | |
| DIRECT_COMMERCE_INSERT FPS | No distinct path — same as SHARE | | | | | | | |
| DIRECT_COMMERCE_INSERT Auction | No distinct path | | | | | | | |
| COMMERCE_ROOM_CONTEXT FPS | ✅ | ❌ | ✅ | ❌ (seller-of-target must be other participant) | ❌ | ✅ `room.HasParticipant` | ❌ No explicit block check on commerce reference creation | ❌ None |
| COMMERCE_ROOM_CONTEXT Auction | ✅ | ❌ | ✅ | ❌ (same seller constraint) | ❌ | ✅ | ❌ | ❌ |

### 9.2 Gaps vs Locked Canonical Requirements

| Requirement | Current State | Gap |
|---|---|---|
| DIRECT_COMMERCE_INSERT: actor must own resource | No ownership check on attachment references | **Missing** — any room participant can attach any listing ID |
| DIRECT_COMMERCE_INSERT: market-increasing authority | No market authority concept | **Missing** |
| SHARE_TO_CHAT: no ownership requirement | Correctly absent | ✅ Aligned |
| Email verification check | Not enforced separately from `RequireActiveAccount` | **Ambiguity** — does `RequireActiveAccount` imply email-verified? Needs verification |
| Destination-room membership | Enforced via `room.HasParticipant` | ✅ Aligned |
| Block check on commerce reference creation | Not enforced | **Gap** — blocked users could create commerce references |
| Resource visibility/access check | Existence check is skip-if-nil ("backward compatibility") | **Gap** — deleted/unavailable resources pass validation silently |

---

## 10. IDEMPOTENCY FINDINGS

### 10.1 Current Mechanism

- **Scope:** Message-level only. No generic idempotency middleware for chat.
- **Key source:** Client-submitted `idempotency_key` in `SendMessageRequest` body (required, `binding:"required"`)
- **Storage:** `chat_messages.idempotency_key UNIQUE` — GLOBAL uniqueness across all rooms and all senders
- **Replay behavior:** Service pre-checks `GetMessageByIdempotencyKey` before insert; found → returns existing message (200 OK), no new outbox events, no rate-limit metrics. DB unique constraint as safety net (maps to `ErrDuplicateMessage`)

### 10.2 Idempotency Semantics Audit

| Question | Answer |
|---|---|
| Where does key enter HTTP/mobile flow? | `SendMessageRequest.idempotency_key` body field; mobile generates `Uuid().v4()` in `ChatRepositoryImpl.sendMessage` (line 530) |
| Same actor + same key + same command? | Returns first message silently — no payload comparison |
| Same actor + same key + changed command? | **Returns first message silently** — no request-hash validation. Different body/attachment with same key returns the original message |
| Different actors using same key? | Global uniqueness — second actor gets... **first actor's message returned to second actor** (key not scoped to sender). This is a **P1 security gap**: idempotency key collision across users leaks message data |
| Destination room included in command semantics? | **No** — key is globally unique, not scoped to room. Same key from same actor in different rooms returns the first room's message |
| Resource identity included in command semantics? | **No** — only the opaque key matters |
| Body included in command semantics? | **No** |
| Replay response stability? | Stable — returns the exact same message (same ID, same created_at, same body/attachment) |
| Duplicate message prevention? | ✅ Yes — unique constraint + pre-check |
| Duplicate occurrence prevention? | N/A — no `chat_message_resource_occurrences` table exists |
| Duplicate outbox prevention? | ✅ Yes — outbox `ON CONFLICT (idempotency_key) DO NOTHING` |

### 10.3 Key Gap

**P1: Cross-user idempotency key collision leaks message data.** If User A sends message with key X, and User B (in a different room) sends message with the same key X, User B receives User A's message in the response (same message ID, body, attachment). The key is globally unique, not scoped to `(sender_id, room_id)`.

**P2: No command-semantics validation.** Same key with different body/attachment/room returns the first message silently. A client retry with modified payload is not detected.

**P2: No key expiry.** Keys are permanent — a replay 6 months later returns the original message.

---

## 11. SNAPSHOT / FALLBACK FINDINGS

### 11.1 Current Snapshot Mechanisms

| Mechanism | Who Builds | Where Stored | What It Contains | Message-Specific? | Affected by Resource Lifecycle? |
|---|---|---|---|---|---|
| **Attachment preview** (message) | **Client-submitted** in `attachment_json.data.preview` | `chat_messages.attachment_json` | `{title, imageUrl, isAvailable, isSold, isClosed, isDeleted}` | ✅ Yes (per-message) | ❌ No — stored verbatim, never refreshed |
| **Commerce reference display_snapshot** | **Hybrid** — client submits `preview`, server overwrites from live data via `resolveCommerceTarget` | `chat_commerce_references.display_snapshot` | `{title, image_url, display_value, seller_id, seller_name, is_available, is_sold, is_closed, is_deleted}` | ❌ No (room-scoped) | ❌ No — immutable after creation |
| **Reply preview** | **Server-built** from target message via `buildReplyPreviewForMessage` | `chat_messages.reply_preview_json` | `{message_id, content, sender_name, type, is_hidden}` | ✅ Yes | ✅ Refreshed on moderation hide/restore |
| **Room last_message** (outbox) | **Server-built** in `buildChatRoomLastMessagePayload` | Outbox event payload | `{body, attachment_json, reply_preview, media_assets}` | ❌ No (room-scoped summary) | ❌ Snapshot at event time |

### 11.2 Gap Analysis vs Canonical Requirements

| Canonical Rule | Current State | Verdict |
|---|---|---|
| Client submits resource identity, not preview authority | **VIOLATED** — client submits `attachment_json.data.preview` with title, imageUrl, availability flags; server stores it verbatim (no server-side overwrite for message attachments, unlike commerce references) | **Gap** — message attachment preview is client-authoritative |
| Server builds occurrence fallback | **NOT IMPLEMENTED** — no server-side occurrence fallback builder exists for message attachments. Commerce reference snapshots have server overwrite, but message attachments do not | **Gap** |
| Live resource authority governs current actions/status | **Partially implemented** — mobile `ObjectPreviewCard` does live merge (status/price live, title/image snapshot-fallback). Backend response emits attachment_json verbatim — client must reconcile | **Architecture split** — server gives snapshot, client live-merges |
| Fallback suppressed for privacy tombstones | **Partially implemented** — tombstoned (soft-deleted) messages suppress body AND attachment. But resource deletion/block/ban does NOT update existing message attachments | **Gap** — deleted listing still renders with its original preview in old messages |
| Commerce reference `SellerName` | **Gap** — `commerceReferenceSnapshotFromListing` and `commerceReferenceSnapshotFromAuction` never set `SellerName` server-side. Only survives if client supplied it in preview | **Bug** |

---

## 12. LIFECYCLE / TOMBSTONE FINDINGS

### 12.1 Current Behavior Matrix

| Resource | Lifecycle State | Current Chat Behavior | Canonical Requirement |
|---|---|---|---|
| **Profile** | Active | N/A — Profile not shareable to chat | Should render with personal/store representation rules |
| **Profile** | Suspended | N/A | Should render tombstone |
| **Profile** | Banned | N/A | Should render tombstone |
| **Profile** | Deleted | N/A | Privacy tombstone — suppress fallback |
| **Profile** | Blocked viewer | N/A | Should hide or tombstone |
| **Content** | Active/public | N/A — Content not shareable to chat | Should render with canonical Content identity |
| **Content** | Followers-only/private | N/A | Should enforce visibility |
| **Content** | Hidden/moderated | N/A | Should render tombstone |
| **Content** | Deleted | N/A | Privacy tombstone — suppress fallback |
| **FPS** | Active | Renders attachment preview verbatim; mobile live-merges status | Should render with live status |
| **FPS** | Inactive/ended/sold/deleted | Preview carries stale `isAvailable`/`isSold`/`isClosed`/`isDeleted` from send time; mobile live-merges | Live authority should override |
| **FPS** | Seller suspended/expired/banned | `attachment_seller_trust_lifecycle` added to response; mobile can react | ✅ Partially implemented |
| **FPS** | Blocked seller | Block check prevents message SEND to blocked user, but does not hide existing messages | Existing messages still visible — **policy ambiguity** |
| **Auction** | Scheduled | Same as FPS — preview verbatim | Live authority should govern |
| **Auction** | Active | Same | ✅ |
| **Auction** | Ended | Same — stale preview | Live authority should show ended state |
| **Auction** | Cancelled | Same — stale preview | Live authority should show cancelled |
| **Auction** | Blocked seller | Same as FPS block behavior | **Same ambiguity** |

### 12.2 Tombstone Mechanism

Current tombstone is moderation-only: `chat_messages.deleted_at/deleted_by/deletion_reason` set by moderation `SoftHideForModeration`. Renders as `{id, room_id, sender_id, message_type, created_at, is_hidden: true}` — body and attachment suppressed.

**No resource-lifecycle-driven tombstoning exists.** A listing deleted after being shared to chat remains visible with its original preview in the message. No backfill/refresh mechanism updates message attachments when the underlying resource changes state.

---

## 13. QUERY / N+1 FINDINGS

### 13.1 Current Hydration Pattern

**Backend `ListMessages`:**
- Single query with `jsonb_agg` for media assets join
- `attachment_json` returned inline — no per-message resource lookup
- `sender_lifecycle` and `attachment_seller_trust_lifecycle` resolved during `messageToResponse` mapping
- **Query classes:** 1 primary message query + identity lookups for sender/attachment-seller (potentially per-message if not batched)

**Mobile `ObjectPreviewCard` batch mode:**
- `MessagesBatchWidget` collects all `objectReference`s from visible messages
- Single batch `objectPreviewBatchProvider` call resolves all previews
- **Good:** Avoids N+1 for object preview resolution
- **Gap:** Each `ObjectPreviewCard` still independently fetches live status on mount (though batch preResolved data mitigates this)

### 13.2 Query Classes Identified

| Query Class | Current Behavior | N+1 Risk |
|---|---|---|
| Per-message resource lookup | Not performed server-side (attachment_json is inline) | None server-side; mobile batch-resolves |
| Per-message seller lookup | `attachment_seller_trust_lifecycle` resolved per-message in `messageToResponse` | **Potential N+1** if not batched in handler |
| Per-message media lookup | Joined via `jsonb_agg` in message query | ✅ None |
| Client-side preview fetch | Mobile live-merges via `ObjectPreviewCard` individual fetches (mitigated by batch `preResolved`) | **Low** (batch mode helps) |
| Room-context fetch per message | NOT performed — room context is separate endpoint, not per-message | ✅ None |
| Identity projection | Sender identity fetched during `messageToResponse` | **Potential N+1** — needs handler-level batching verification |

---

## 14. LEGACY / RESIDUE INVENTORY

### 14.1 Dead Code (Production)

| Item | Location | Lines | Impact |
|---|---|---|---|
| `ShareDestination.sendToChat` | `share_destination.dart:65-70` | Declared but excluded from `internalDestinations` | Dead constant |
| `ChatGateway` (copy 1) | `lib/shared/chat/chat_gateway.dart` | 67 lines | 0 implementers, 0 consumers |
| `ChatGateway` (copy 2) | `lib/domains/chat/chat_gateway.dart` | 67 lines | 0 implementers, 0 consumers |
| `LinkOrderToChatUseCase` | `lib/domains/commerce/transaction/usecases/link_order_to_chat_usecase.dart` | ~30 lines | 0 importers |
| `GetFixedPriceSaleShareReferenceUseCase` (chat) | `lib/domains/chat/chat/domain/usecases/get_listing_share_reference_usecase.dart` | ~30 lines | 0 importers, absent from barrel |
| `GetFixedPriceSaleShareReferenceUseCase` (catalog) | `lib/domains/commerce/catalog/usecases/get_listing_share_reference_usecase.dart` | ~30 lines | 0 importers |
| `CreateChatDto` | `chat_dto.dart:357-366` | 10 lines | Orphaned — room creation uses `POST /chat/direct/{userId}` with no body |
| `ChatListDto` | `chat_dto.dart:368-391` | 24 lines | Bypassed — `listRooms` parses `data['data']` directly |

### 14.2 Legacy / Compatibility Code (Live)

| Item | Location | Nature |
|---|---|---|
| `ChatWebSocketHandler` | `core/src/websocket/chat_websocket_handler.dart` | Separate legacy WS handler with own wire types; still registered in service_locator but events ignored by canonical repository path |
| `widget_lib.AttachmentWidget` | `shared/widgets/attachment_widget.dart` | Legacy widget for negotiation/shipping/location; still used in `message_bubble.dart:522` |
| `object_reference_bridge.dart` | `shared/object/object_reference_bridge.dart` | Stale doc comment claiming "migration from ShareReference to ObjectReference" — direction is opposite of Phase-1 cleanup |
| `chat_mapper.dart` wire compat | `data/mappers/chat_mapper.dart:170,537,623-625` | Reads old wire key `objectReference`, converts to `ShareReference` |
| `chat_dto.dart` legacy format | `data/dto/chat_dto.dart:130-237` | Handles legacy list response format with `participant_names`, `unread_counts` |
| `attachment_dto.dart:54` stale comment | `data/dto/attachment_dto.dart:54` | Still lists `content, profile` as valid chat target types — backend only accepts `fixed_price_sale\|auction` |
| `chat_mapper.dart` legacy fallbacks | `data/mappers/chat_mapper.dart:188-193,364,432-434` | Null senderLifecycle, group/channel types, offer_reference fallback |
| `chat_entities.dart` legacy defaults | `domain/entities/chat_entities.dart:352,570-579` | "polluted compatibility maps", legacy payload defaults |
| Backend "backward compatibility" skip | `chat_service.go:1849` | Attachment existence validation skipped entirely if checkers are nil |
| Realtime legacy envelope parsing | `realtime/connection.go:112,281,345-387` | "legacy top-level fallback" pinned by contract test |

### 14.3 Already Purged (Confirmed)

| Item | Evidence |
|---|---|
| `chat_rooms.context_json` + `context_set_by` | Dropped by migration 000030 after backfill to `chat_commerce_references` |
| Legacy attachment types (listing, auction, post, request) | Removed; `attachment_validator_test.go:124-135` proves rejection |
| Group/channel room types | Removed from mobile enum; backend enum has `direct\|negotiation\|support` |
| `/chat?userId=` bypass | Confirmed absent; tests assert it is gone (`order_chat_canonical_contract_test.dart:34`) |
| `MessageAttachment` wrapper | Removed in favor of explicit nullable fields on `Message` |
| `ChatApiDI` (GetIt) | Replaced by Riverpod (`chat_providers.dart:6-7`) |

### 14.4 Cleanup Candidates (Do NOT Delete Yet)

**Production code safe to delete after verification:**
1. Both `ChatGateway` files (2 × 67 lines) — dead, 0 implementers
2. Three dead usecase files — 0 importers
3. `CreateChatDto`, `ChatListDto` — orphaned DTOs
4. `ShareDestination.sendToChat` constant — dead code path

**Compatibility code to remove after canonical switch:**
5. `ChatWebSocketHandler` — legacy WS handler (but verify nothing depends on it at runtime first)
6. `widget_lib.AttachmentWidget` for chat attachments — migrate negotiation/shipping/location to canonical renderer
7. `object_reference_bridge.dart` stale comment
8. `attachment_dto.dart:54` stale target_type comment
9. Chat mapper legacy wire compat (objectReference key, legacy format parsing)

---

## 15. TEST INVENTORY

### 15.1 Backend Tests

**109 `func Test`** in `backend/internal/interaction/chat/`:

| Area | Files | Key Coverage |
|---|---|---|
| Application | 9 files | Attachment reference authority, media verification/contract, link-order ownership, mute/unread, room event outbox producers |
| Delivery/HTTP | 12 files | Attachment JSON schema validation (incl. rejection of legacy wire types), tombstoning, negotiation endpoints, unread/read closure, E10 room list lifecycle, reply media projection |
| Consumer | 2 files | Negotiation event handler |

**~60 `func Test`** in chat-adjacent packages:
- Negotiation chatroom linkage (5)
- Moderation chat_message intake/preview (18)
- Support chat authority (4)
- Chat account gate middleware (8)
- Realtime dispatcher chat events (5)
- Realtime room authorizer (5)
- Realtime envelope contract (10)

### 15.2 Mobile Tests

**339 `test`/`testWidgets`** in `apps/mobile/test/domains/chat/`:

| Area | Files | Key Coverage |
|---|---|---|
| Participant identity | c1b1, c1b2, e12_1 | Chatcard identity/display, new chat runtime, current-user display |
| Room list lifecycle | e10, e11_1 | E10/E11 lifecycle rendering |
| Attachment contract | 1 file (21 tests) | Attachment DTO alignment |
| Composer authority | 1 file | Message compose permissions |
| WS envelope contract | 1 file (10 tests) | WebSocket envelope parsing |
| Hidden message contract | 1 file | Tombstone rendering |
| Pagination cursors | 1 file | Cursor encode/decode |
| Presence behavior | 1 file (12 tests) | Typing indicator, online status |
| Unread surface | 1 file | Unread count alignment |

### 15.3 Reusable Test Categories for Canonical Switch

| Category | Reusable? | Notes |
|---|---|---|
| Attachment validator schema tests | ✅ Directly reusable — extend with Profile/Content types | `attachment_validator_test.go` |
| Tombstone tests | ✅ Reusable — extend with resource-lifecycle-driven tombstoning | `chat_message_tombstone_test.go` |
| Outbox producer tests | ✅ Reusable — verify new occurrence fields in outbox payload | `chat_service_room_updated_producer_test.go` |
| Room event projection tests | ✅ Reusable — extend with occurrence data in last_message | `chat_room_list_response_test.go` |
| Identity projection tests | ✅ Reusable | `chat_identity_projection_test.go` |
| Lifecycle rendering tests (E10) | ✅ Reusable | `e10_chat_room_list_lifecycle_test.dart` |
| Attachment contract alignment tests | ✅ Reusable | `attachment_contract_alignment_test.dart` |
| WS envelope contract tests | ✅ Reusable | `ws_envelope_contract_test.dart` |
| Pagination cursor tests | ✅ Reusable | Cursor tests |
| Legacy wire type rejection tests | ⚠️ Will need updating — currently asserts content/profile REJECTED; canonical switch will ACCEPT them | `attachment_validator_test.go:124-135` |

---

## 16. ROOT ARCHITECTURAL GAPS

### Gap 1: No Message Resource Occurrence Table
**Severity: BLOCKER**
`chat_message_resource_occurrences` does not exist. Current authority is `attachment_json` — a jsonb column with no typed schema, no CHECK constraints for exactly-one-source, no UNIQUE(message_id), no separation from other attachment types (negotiation, shipping, location).

### Gap 2: No Profile or Content Share to Chat
**Severity: BLOCKER**
The locked Unified Share architecture requires Profile and Content as shareable resource types. The backend attachment validator rejects them. The mobile `ShareReference.chatWireTargetType` returns null for them. The share sheet's `sendToChat` destination is declared but dead code.

### Gap 3: No Distinct DIRECT_COMMERCE_INSERT Authorization
**Severity: BLOCKER**
The locked architecture requires distinct authorization for DIRECT_COMMERCE_INSERT (actor must own resource, market-increasing authority). Current code has zero ownership checks on attachment references. The same `SendMessage` endpoint serves both SHARE_TO_CHAT and the de-facto "insert" path with identical authorization.

### Gap 4: Client-Submitted Preview Is Message Attachment Authority
**Severity: HIGH**
The `attachment_json.data.preview` object is client-submitted and stored verbatim. Unlike commerce references (where server overwrites from live data), message attachment previews have zero server-side authority. The mobile `ObjectPreviewCard` partially mitigates with live status merge, but the snapshot stored in the database is client-authoritative.

### Gap 5: Idempotency Key Is Globally Unique, Not Scoped
**Severity: HIGH**
Idempotency key UNIQUE is global across all rooms and all senders. Cross-user key collision returns another user's message. No payload/command hashing. No room scoping. No expiry.

### Gap 6: No Resource-Lifecycle-Driven Tombstoning
**Severity: MEDIUM**
Only moderation can tombstone messages. Resource deletion, seller ban, content moderation, or user block does not update existing message attachments or apply privacy tombstones.

### Gap 7: commerce_reference SellerName Never Set Server-Side
**Severity: LOW**
`commerceReferenceSnapshotFromListing` and `commerceReferenceSnapshotFromAuction` construct the snapshot but never populate `SellerName` from the seller entity — only survives if client supplied it.

### Gap 8: Room Creation + Commerce Reference Are Non-Atomic
**Severity: LOW**
Mobile `openCommerceChat` makes two sequential API calls. A failure between them leaves a room without commerce context. No transactional get-or-create-room-with-commerce endpoint exists.

### Gap 9: Attachment Existence Validation Skippable
**Severity: LOW**
`validateAttachmentReferences` silently skips validation when checkers are nil ("backward compatibility"). In production, this means a misconfigured server could accept references to non-existent resources.

---

## 17. PROPOSED CANONICAL REPAIR

### Phase A: Additive — Create Canonical Schema (Zero Breaking Changes)

**A1. Create `chat_message_resource_occurrences` table:**
```sql
CREATE TYPE chat_resource_occurrence_operation_enum AS ENUM (
  'share_to_chat',
  'direct_commerce_insert_chat'
);

CREATE TYPE chat_resource_occurrence_source_type_enum AS ENUM (
  'profile',
  'content',
  'fixed_price_sale',
  'auction'
);

CREATE TABLE chat_message_resource_occurrences (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  message_id uuid NOT NULL UNIQUE REFERENCES chat_messages(id) ON DELETE CASCADE,
  room_id uuid NOT NULL REFERENCES chat_rooms(id) ON DELETE CASCADE,
  operation chat_resource_occurrence_operation_enum NOT NULL,
  actor_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_type chat_resource_occurrence_source_type_enum NOT NULL,
  -- Exactly one typed source (CHECK constraint):
  profile_source_id uuid REFERENCES profiles(id),
  content_source_id uuid REFERENCES contents(id),
  fixed_price_sale_source_id uuid REFERENCES fixed_price_sales(id),
  auction_source_id uuid REFERENCES auctions(id),
  CONSTRAINT exactly_one_source CHECK (
    (CASE WHEN profile_source_id IS NOT NULL THEN 1 ELSE 0 END +
     CASE WHEN content_source_id IS NOT NULL THEN 1 ELSE 0 END +
     CASE WHEN fixed_price_sale_source_id IS NOT NULL THEN 1 ELSE 0 END +
     CASE WHEN auction_source_id IS NOT NULL THEN 1 ELSE 0 END) = 1
  ),
  -- Server-built fallback/snapshot:
  fallback_snapshot jsonb,
  -- Privacy tombstone:
  is_tombstoned boolean NOT NULL DEFAULT false,
  tombstoned_at timestamp with time zone,
  tombstone_reason text,
  created_at timestamp with time zone NOT NULL DEFAULT now()
);
```

**A2. Add `occurrence_id` to outbox payloads** (non-breaking — additive field)

**A3. Extend attachment validator to accept `profile` and `content` target types for `SHARE_TO_CHAT` operation only**

**A4. Wire new `POST /chat/rooms/{id}/share` endpoint** — distinct from `SendMessage`:
- Request: `{resource_type, resource_id, idempotency_key}`
- Validates resource existence, visibility, viewer capabilities
- Creates message + occurrence + outbox in single transaction
- No client preview — server builds fallback from live resource

**A5. Wire `POST /chat/rooms/{id}/insert-commerce` endpoint** for DIRECT_COMMERCE_INSERT:
- Request: `{resource_type, resource_id, idempotency_key}`
- Validates ownership + market authority
- Same persistence path as SHARE_TO_CHAT

### Phase B: Switch — Dual-Write Window

**B1.** New endpoints write to BOTH `chat_message_resource_occurrences` (canonical) AND `attachment_json` (legacy compatibility) for N weeks

**B2.** Message rendering reads `chat_message_resource_occurrences` first, falls back to `attachment_json` for legacy messages

**B3.** Mobile updated to call new share/insert endpoints; old `attachment_json` path in `SendMessage` deprecated but still accepted

**B4.** Outbox events include occurrence data alongside existing payload

### Phase C: Purge — Remove Legacy Authority

**C1.** Remove `attachment_json` from `SendMessageRequest` — attachment_json column retained for legacy message read but no longer written for new messages

**C2.** Drop `attachment_json.data.preview` client authority — server always builds fallback

**C3.** Remove legacy wire compat in chat mapper, DTOs, parsers

**C4.** Delete dead code inventory (ChatGateway ×2, dead usecases ×3, orphaned DTOs, `sendToChat` constant)

**C5.** Consider dropping `attachment_json` column after sufficient migration window (risky — breaks old message rendering)

---

## 18. MIGRATION / SWITCH / PURGE PLAN

| Step | What | Risk | Rollback |
|---|---|---|---|
| 1 | Create `chat_message_resource_occurrences` table + enums (migration N) | Low — new table, no existing data dependency | Drop table + types |
| 2 | Add occurrence endpoints (`/share`, `/insert-commerce`) | Low — new endpoints, old path still works | Remove routes |
| 3 | Extend attachment validator for Profile/Content | Medium — must ensure only new endpoints use new types, not old SendMessage | Remove new type cases from validator |
| 4 | Mobile: add share-to-chat for Profile/Content via new endpoints | Medium — new UI; old UI unchanged | Revert mobile changes |
| 5 | Mobile: switch commerce insert to new endpoint | Medium — changes existing flow | Revert to old SendMessage path |
| 6 | Dual-read: message renderer reads occurrence first, falls back to attachment_json | Low — additive read path | Remove occurrence read, keep fallback only |
| 7 | Backfill: populate `chat_message_resource_occurrences` from existing `attachment_json` reference messages | Medium — one-time migration; must handle bad data | Re-run with fixes |
| 8 | Deprecate `attachment_json` in SendMessage (server rejects if occurrence endpoint should be used) | High — breaks old mobile clients | Revert rejection |
| 9 | Purge dead code | Low — all verified 0-consumer | Revert deletions |
| 10 | Drop `attachment_json` column (optional, long-term) | High — breaks legacy message rendering if occurrence backfill incomplete | Restore column from backup |

**Recommended sequence:** Steps 1-6 are additive and can ship together. Steps 7-10 require the dual-write window to have populated enough data.

---

## 19. REQUIRED EXECUTABLE PROOFS

Before closure, the following must pass as executable tests:

### Schema & Persistence
1. `chat_message_resource_occurrences` table exists with exactly-one-source CHECK constraint enforced
2. UNIQUE(message_id) prevents duplicate occurrences
3. Migration replay clean (up + down + up)
4. `INSERT` with zero sources → constraint violation
5. `INSERT` with two sources → constraint violation
6. `INSERT` with exactly one source → success

### Share to Chat (Profile, Content, FPS, Auction)
7. Share Profile to Chat → message created + occurrence row + outbox events in single transaction
8. Share Content to Chat → same
9. Share FPS to Chat → same
10. Share Auction to Chat → same
11. Non-participant share → 403
12. Blocked user share → 403
13. Deleted resource share → 400/404 with appropriate error
14. Private/invisible resource share → 403

### Direct Commerce Insert
15. Insert own FPS → success, occurrence.operation = 'direct_commerce_insert_chat'
16. Insert own Auction → success
17. Insert OTHER seller's FPS → 403 (ownership required)
18. Insert with insufficient market authority → 403
19. Insert Profile via commerce-insert endpoint → 400 (wrong operation for resource type)
20. Insert Content via commerce-insert endpoint → 400

### Idempotency
21. Same actor + same key + same command → replay returns existing occurrence
22. Same actor + same key + different resource → should NOT silently return first (command semantics in key)
23. Different actor + same key → NOT collide (key scoped to actor or room)

### Fallback / Snapshot
24. Server fallback built from live resource data, not client preview
25. Resource deletion toggles `is_tombstoned` on occurrence
26. Tombstoned occurrence renders privacy tombstone, not original fallback
27. Seller ban/suspension reflects in occurrence rendering

### Room Context Independence
28. Sending message occurrence does NOT mutate `chat_commerce_references`
29. Message renderer does NOT read `chat_commerce_references` for occurrence data

### Legacy Purge
30. Old `attachment_json` reference path still readable for legacy messages
31. New messages do NOT write `attachment_json` (after switch cutoff)
32. Dead code files confirmed deleted, tests still pass

### Query Bounds
33. Message list with occurrences does not trigger N+1 resource lookups
34. Batch occurrence resolution works (analogous to current `ObjectPreviewCard` batch mode)

---

## 20. BUSINESS AMBIGUITIES / OWNER DECISIONS NEEDED

### Q1: Email Verification Gate for Share to Chat?
Current `RequireActiveAccount` middleware gates all POST chat mutations. Does this imply email-verified? Or should Share-to-Chat have an explicit email-verification gate? **The locked spec says "email verification required?" for all operations — is `RequireActiveAccount` sufficient or does it need an explicit `RequireEmailVerified` middleware?**

### Q2: Content Share Recursive Rendering
The locked spec states: "bounded rendering if shared Content itself references another resource." What is the depth limit? Is it 1 level (Content → no further expansion) or should nested references render as text links?

### Q3: Profile Personal vs Store Representation
The locked spec says Profile share should follow "personal/store representation rules." What are these rules? When User A shares their own Profile vs User B's Profile, does the rendering differ? What fields are included?

### Q4: Privacy Tombstone Scope
When a resource is deleted, should ALL occurrences across ALL chat rooms be tombstoned (backfill update), or only new renderings suppress the fallback? The locked spec says "Privacy tombstone suppresses fallback where required" — does this mean proactive backfill or render-time check?

### Q5: Idempotency Key Scoping
The current global key scope is a P1 concern. Should the canonical key be scoped to `(sender_id, room_id)` or `(sender_id, operation, resource_id)`? The locked spec says "same actor + same key + changed command" should be detectable — does this imply a request-hash approach or simply tighter key scoping?

### Q6: Room Commerce Context for Profile/Content?
The locked spec says room commerce context is FPS/Auction only. Is this correct? Should sharing a Profile to a chat create any room-level context? **Presumed: NO — room commerce context remains FPS/Auction only.**

### Q7: Dual-Write Window Duration
How long should the additive phase run before switching to canonical-only writes? Suggestion: 2 release cycles minimum.

### Q8: Backfill Strategy
Should existing `attachment_json.type=reference` messages be backfilled into `chat_message_resource_occurrences`? Or only new messages use the occurrence table? **Recommendation: backfill for read-path unification; mark backfilled rows with a flag distinguishing original from backfill.**

---

## 21. RECOMMENDATION

**REMAINS_BLOCKED_BY_AUDIT_AMBIGUITY**

The audit has identified the complete current state and all architectural gaps. No implementation should begin until the 8 owner decisions in Section 20 are resolved. Once resolved, the canonical repair direction (Section 17) is well-defined and can proceed through additive → switch → purge phases.

The codebase is in good structural shape for this change:
- Room commerce context is already cleanly separated from message attachment authority
- Migration 000030 already proved the backfill-then-purge pattern works for chat
- The attachment validator already has a strict schema that can be extended
- Mobile already has the `ShareReference` abstraction and `ObjectPreviewCard` live-merge pattern
- Test coverage is strong (109 backend + 339 mobile chat tests) and largely reusable

The primary work is:
1. Create the `chat_message_resource_occurrences` table (new)
2. Add Profile/Content as valid share targets (extension of existing validator)
3. Add distinct SHARE_TO_CHAT and DIRECT_COMMERCE_INSERT endpoints (new, additive)
4. Add ownership/market-authority checks for DIRECT_COMMERCE_INSERT (new authorization)
5. Server-built fallback instead of client-submitted preview (authority transfer)
6. Resource-lifecycle-driven tombstoning (new capability)
7. Scope idempotency keys to prevent cross-user collision (P1 fix, can be scoped separately)

---

*End of audit. No implementation has been performed. All findings are from filesystem inspection of current code at `d:\Project\labuda` on 2026-08-08.*
