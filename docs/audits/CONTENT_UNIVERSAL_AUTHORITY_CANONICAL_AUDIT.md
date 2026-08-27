# CONTENT UNIVERSAL AUTHORITY — CANONICAL AUDIT

**Date**: 2026-08-05
**Scope**: `CONTENT_UNIFIED_AUTHORITY_FEED_RECOVERY_AND_HARD_PURGE`
**Verdict**: `CONTENT_UNIVERSAL_AUTHORITY_CANONICAL_AUDIT_FINALIZED_READY_FOR_IMPLEMENTATION_PROMPT`

This is the single canonical audit document for the Content Universal Authority scope. All older reports in `docs/audits/` are superseded and retained only as historical evidence.

---

## 1. Canonical Truth

One universal domain object:

```text
Content
```

### Forbidden Designs (must never exist, must be removed if found)

```text
Post type
Request type
contents.type column
content_type_enum
ContentType.post
ContentType.request
ContentKind
Post/Request enum
Post/Request DTO
Post/Request route
Post/Request filter
Post/Request selector
Post/Request repository/service/handler
content_type=post
content_type=request
default post
legacy request
compatibility alias
backward compatibility shim
optional legacy type field
defaultValue on legacy field
deprecated wrapper
parser that accepts Post/Request
mapper that accepts legacy type then always maps to Content
constant mapper with unused backendType
"type": "content" as a replacement for Post/Request subtype
```

### Chat Content Decision (2026-08-05)

Content MUST NOT be shared to Chat. Profile MUST NOT be a Chat message attachment/context capability. All Content/Post/Request/Profile attachment scaffolding in Chat must be hard-purged:

```text
Content Chat destination
Content Chat context
Content Chat wire values (post, request, content)
Profile Chat wire values (as attachment/context)
Content Chat normalization
Content Chat parser compatibility
Content Chat SQL validation (validateContentReferenceExists)
Content Chat error sentinels (ErrAttachmentPostNotFound, ErrAttachmentRequestNotFound)
Content Chat reply preview branches
Content Chat tests and documentation
```

Canonical Chat attachment/context values that remain:
```text
fixed_price_sale
auction
```

Profile → direct-chat entry ONLY (recipient routing, not message attachment/context capability).

### Canonical Authorities (unchanged)

```text
comments always active
visibility uses contents.visibility (public, followers_only, private)
is_hidden only for moderation
status: active → deleted (terminal)
author lifecycle: active, unavailable, removed
search: heterogeneous bucket-level discrimination
feed: explicit feed_item_kind envelope discriminator
  feed_item_kind = content
  feed_item_kind = promoted_fixed_price_sale
  feed_item_kind = promoted_auction
  feed_item_kind = promoted_external
```

---

## 2. Accepted Runtime and Schema Facts

### 2.1 Database

- **PostgreSQL 16.13**, Docker container `labuda-postgres`, database `labuda`
- **`contents` table**: 14 columns — `id, author_id, status, caption, city, province, is_hidden, original_author_id, share_reference, created_at, updated_at, deleted_at, search_vector, visibility`
- **No `type` column** — confirmed via `information_schema.columns` (0 rows for `column_name = 'type'`)
- **No `content_type_enum`** — confirmed via `pg_type`/`pg_enum` introspection
- **30 migrations applied** (001–030), all `dirty = false`
- **0 content rows** in live database
- **Seed**: 25 rows (20 visible, 3 hidden, 2 deleted), no `type` column, no `user_profiles` created (gap)

### 2.2 Feed Root Cause

**`backend/internal/social/feed/infrastructure/repository/feed_repository_impl.go:237`**:
```sql
SELECT id, author_id, type, status, ... FROM ranked_feed
```
`type` does not exist in `feed_base` CTE (which correctly omits it) → PostgreSQL raises `SQLSTATE 42703 column "type" does not exist` → HTTP 500 on every `GET /api/v1/feed`.

**Secondary**: `rows.Scan` (20 targets) vs SELECT (21 columns including phantom `type`) — column-count mismatch.

**Call chain**: `routes_core.go:1243` → `feed_handler.go:51` → `feed_service.go:45` → `feed_repository_impl.go:43` → SQL error → 500.

### 2.3 Mobile Feed DTO

**`apps/mobile/lib/features/home/data/dto/feed_dto.g.dart:34`**: `type: json['type'] as String` — required non-null cast. Backend `feedItemToResponseCanonical` does NOT emit a `type` key for organic items. After SQL fix, every organic feed item would throw TypeError → entire feed in error state.

### 2.4 Search Fabrication

**`apps/mobile/lib/features/search/search/data/dto/search_dto.dart:274`**: `json['type'] as String? ?? 'post'` — backend search response has NO per-item `type` key. Fallback `'post'` always executes. Heterogeneous discrimination works correctly via bucket-level containers (`results.contents[]`, `results.listings[]`, etc.). The `type` → `contentType` → `_mapContentTypeToType` chain is dead legacy plumbing.

### 2.5 Share-to-Feed Breakage

**`apps/mobile/lib/domains/social/share/data/datasources/share_api_datasource.dart:72`**: `createShareReferencePost` sends `'type': 'post'` in payload to `POST /api/v1/contents`. Backend `strictBindJSON` with `DisallowUnknownFields` rejects unknown `type` field → HTTP 400. Also sends `targetType: 'listing'` which is not in backend `oneof=content fixed_price_sale auction profile` → 400. **Listing/Auction/Profile share-to-feed is fully broken.**

Content repost (`createRepost` → `POST /contents/:id/repost`) works correctly.

### 2.6 Chat Content-Reference — Must Be Hard-Purged

- **`chat_service.go:2302`**: `SELECT id FROM contents WHERE id = $1 AND type = $2` — queries non-existent `contents.type` → SQLSTATE 42703
- **`validator.go:109`**: `validTargetTypes` includes `post`/`request`, rejects canonical `content`
- **`chat_service.go:1889-1902`**: Switch on `post`/`request` target types → calls broken SQL
- **`chat_repository.go:185-189`**: `ErrAttachmentPostNotFound`/`ErrAttachmentRequestNotFound` sentinels
- **Decision**: Content-to-Chat is rejected. All above must be removed.

### 2.7 Integration Tests Broken

`go vet -tags integration` fails with 13 errors across 5 test files. Production code compiles.

### 2.8 Working Tree

- **959** git status entries: **702** tracked modified, **0** staged, **348** untracked
- Branch `main`, parent `41aa4ee fix(seller): guard listing creation and renewal flow`
- Mid-refactor state: `content_type.go` deleted, `feed_item.go` Type field removed, feed CTE canonical, outer SELECT stale

---

## 3. Contract Maps

### 3.1 Feed Contract

```
Backend: feed_base CTE (20 canonical columns, no type)
  → ranked_feed CTE (adds feed_priority)
    → Outer SELECT ❌ BROKEN: includes phantom `type` (line 237)
      → rows.Scan → FeedItem entity (no Type field)
        → feedItemToResponseCanonical → JSON
          → Every item MUST have feed_item_kind
          → feed_item_kind = content | promoted_fixed_price_sale | promoted_auction | promoted_external
          → Promotion injector emits feed_item_kind with promoted_* values
          → NO type field anywhere in response
            → Mobile: FeedResponseDto.fromJson discriminator on feed_item_kind
              → PromotedFeedItemDto (when feed_item_kind starts with 'promoted_')
              → FeedItemDto (when feed_item_kind == 'content')
                → FeedItemDto has NO type field
                → Renderer: FeedCardFactory.buildCardForFeedItem (switches on FeedItemType)
                → Share action: ExternalShareType.content → /content/:id navigation
```

**Corrected contract**: Every feed item carries `feed_item_kind`. No item carries `type`. Mobile discriminates on `feed_item_kind`. `FeedItemDto` has no subtype field.

### 3.2 Share/Repost Contract

```
Content Repost ✅:
  ExternalShareType.content → createRepost → POST /contents/:id/repost
  → Backend: RepostContent → CreateRepost → new Content with ShareReference(content)

Listing/Auction/Profile Share-to-Feed ❌ (broken, must be fixed):
  ExternalShareType.listing|auction|profile → createShareReferencePost → POST /contents
  → Payload: { type: "post", share_reference: { targetType: "listing"|"auction"|"profile", ... }, media: ["raw_url"] }
  → REJECTED: unknown "type" field (strictBindJSON) + "listing" not in oneof + media not {url,type} objects

Canonical replacement:
  ShareReference.fixedPriceSale(...).toJson() | auction(...).toJson() | profile(...).toJson()
  → POST /api/v1/contents
  → Payload: { caption, visibility, share_reference: { targetType: "fixed_price_sale"|"auction"|"profile", targetId, preview }, media: [{url, type: "image"|"video"}] }
```

### 3.3 Search Contract

```
Backend: GET /api/v1/search → { results: { contents:[], listings:[], users:[], hashtags:[] } }
  → Heterogeneous discrimination: container-level only (no per-item type key)
    → Content items: { id, author_id, caption, media_urls, created_at, author, media, card }
      → Enrichment (from share_reference.targetType): fixed_price_sale | auction | profile
    → Mobile: UnifiedSearchResults.fromJson → separate buckets by container
      → ContentSearchResultDto: NO type or contentType field
        → Mapper directly uses SearchResultType.content based on result bucket
          → Tab label: owner decision (Indonesian)
          → Filter: getByType(SearchResultType.content)
          → Navigation: result.type switch → /content/:id
```

ContentSearchResultDto must NOT have a `type` field, `contentType` field, or any Post/Request fallback. Heterogeneous discrimination works via response buckets. The entire `type` → `contentType` → `_mapContentTypeToType` chain must be deleted end-to-end.

### 3.4 Chat Attachment/Context Contract (CANONICAL — Preserved)

```
Mobile: Listing → ShareReference.fixedPriceSale → chat message attachment ✅
Mobile: Auction → ShareReference.auction → chat message attachment ✅
Backend validator: fixed_price_sale → GetByID on fixed_price_sales table ✅
Backend validator: auction → GetByID on auctions table ✅

Profile → direct-chat entry ONLY (recipient routing).
Profile is NOT a message attachment/context capability.
```

### 3.5 Content Create Contract

```
Mobile: CreateContentDto { caption, visibility, media: [{url, type}], tags, share_reference, location }
  → POST /api/v1/contents ✅
    → Backend: strictBindJSON(DisallowUnknownFields)
      → Rejects: type, fulfilled_at, fulfilled_by, status="fulfilled"
      → Accepts: share_reference { targetType: oneof=content|fixed_price_sale|auction|profile, targetId, preview }
      → Accepts: media: [{url, type: oneof=image|video}]
```

### 3.6 Content Detail / Profile Contract

```
Backend: GET /api/v1/contents/:id → { id, caption, author_id, status, visibility, media, author: { id, username, avatar_url, lifecycle }, card }
  → NO type key in response ✅
  → author lifecycle: coarsened from account_status + deleted_at via CoarsenLifecycle
Mobile: ContentDto parsing → Content entity → /content/:contentId navigation
Profile: GET /users/:id/contents → ListByAuthor with cursor pagination
```

### 3.7 Notifications / Deep Links

```
Backend: notification_worker_social.go → SELECT author_id FROM contents WHERE id = $1 ✅
Mobile: notification_navigation_handler.dart → canonical types: content.liked, comment, mention
Deep link: /content/:contentId only ✅
```

---

## 4. Hard-Purge Manifest

### 4.1 Backend Production Code

**Feed (P0 — in scope)**:
- `internal/social/feed/infrastructure/repository/feed_repository_impl.go:237` — Remove `type,` from outer SELECT
- `internal/social/feed/delivery/http/feed_share_projection.go:45-63` — Add `feed_item_kind` key to response map: `"content"` for organic items
- `internal/social/feed/delivery/http/feed_promotion_injector.go` — Change emitted discriminator from `"type"` to `"feed_item_kind"` with values `promoted_fixed_price_sale`, `promoted_auction`, `promoted_external`
- `internal/social/feed/delivery/http/feed_share_projection_test.go:331` — Update guard: assert `resp["feed_item_kind"]` present, `resp["type"]` absent

**Chat Content-Reference Slice (P0 — hard-purge)**:
- `internal/interaction/chat/application/chat_service.go:2292-2313` — Delete `validateContentReferenceExists` function
- `internal/interaction/chat/application/chat_service.go:1884-1905` — Remove `case "post"`, `case "request"`, `case "content"` branches, `case "profile"` branch from `validateAttachmentReferences`. Keep `case "fixed_price_sale"` and `case "auction"` only.
- `internal/interaction/chat/application/chat_service.go:2079` — Remove `case "post", "request", "content"` from reply preview label
- `internal/interaction/chat/attachmentvalidator/validator.go:109` — Change `validTargetTypes` to `{fixed_price_sale, auction}`
- `internal/interaction/chat/repository/chat_repository.go:185-189` — Delete `ErrAttachmentPostNotFound`, `ErrAttachmentRequestNotFound`
- `internal/interaction/chat/delivery/http/chat_handler.go:1437-1444` — Delete post/request error mappings

**Documentation**:
- `internal/pkg/publiccard/content_card.go:34-35` — Fix stale doc claiming "Type: required string"
- `internal/worker/notification_worker_social.go:159` — Remove stale "fallback to 'post'" comment
- `cmd/core_server/routes_core.go:1241,1247,1259,1282` — Update comments mentioning "posts, requests"

### 4.2 Mobile Production Code

**Feed DTO (P1)**:
- `apps/mobile/lib/features/home/data/dto/feed_dto.dart:130` — Remove `type` field from `FeedItemDto`; add `feed_item_kind` field (`String`, required)
- `apps/mobile/lib/features/home/data/dto/feed_dto.dart:77` — Update `FeedResponseDto.fromJson` discriminator to use `feed_item_kind` instead of `type`
- `apps/mobile/lib/features/home/data/dto/feed_dto.dart` — Update `PromotedFeedItemDto` to read `feed_item_kind` instead of `type`
- `apps/mobile/lib/features/home/data/dto/feed_dto.g.dart` — Regenerate after DTO changes
- `apps/mobile/lib/features/home/data/mappers/feed_mapper.dart:41-49` — Delete `_mapFeedItemType`; inline `FeedItemType.content`
- `apps/mobile/lib/features/home/data/remote/feed_api_datasource.dart:25` — Update comment

**Share/Repost (P1)**:
- `apps/mobile/lib/domains/social/share/data/datasources/share_api_datasource.dart:61-80` — Replace `createShareReferencePost` with canonical method using `ShareReference.toJson()`
- `apps/mobile/lib/domains/social/share/domain/entities/share_target.dart:100` — Delete `ExternalShareType.request`; rename `post` → `content`
- `apps/mobile/lib/domains/social/share/presentation/widgets/share_preview_card.dart` — Remove `request` branches
- `apps/mobile/lib/domains/social/share/data/dto/share_dto.dart` — Remove `request` branches, `orElse: ExternalShareType.post`
- `apps/mobile/lib/domains/social/share/data/repositories/share_repository_api.dart:91` — Remove `ExternalShareType.post` gate

**Chat Content-Reference Slice (hard-purge with caller safety)**:

Before deleting any helper, audit all callers. Remove only Content/Post/Request/Profile branches; preserve Listing→fixed_price_sale and Auction→auction:

- `apps/mobile/lib/shared/attachment/entities/share_reference.dart:253-278` — Audit callers of `chatWireTargetType` and `asChatReference`. Remove Content/Post/Request/Profile branches. Preserve `fixed_price_sale` and `auction` mapping. Delete helper only if no caller or function remains after simplification.
- `apps/mobile/lib/domains/chat/chat/data/repositories/chat_repository_impl.dart:512-518` — Delete `content_type=post or content_type=request` gate. Remove Profile branch.
- `apps/mobile/lib/domains/chat/chat/data/repositories/chat_repository_impl.dart:756-790` — Audit callers of `_normalizeReferenceForChat` and `_readChatContentType`. Remove Content/Post/Request/Profile branches. Preserve Listing→fixed_price_sale and Auction→auction. Delete helpers only if no function remains after simplification.
- `apps/mobile/lib/domains/chat/chat/data/dto/attachment_dto.dart:147-153` — Remove `post`/`request` parsing for content. Remove `profile` parsing.

**Search (P2)**:
- `apps/mobile/lib/features/search/search/data/dto/search_dto.dart:274` — Remove `type` field from `ContentSearchResultDto` entirely. Remove `contentType` field. Delete `?? 'post'` fallback. Delete `_mapContentTypeToType`.
- `apps/mobile/lib/features/search/search/data/dto/search_dto.dart:199-208` — Delete stale doc comment claiming backend sends `"type": "post|request|repost"`.
- `apps/mobile/lib/features/search/search/data/mappers/search_mapper.dart:62-71` — Delete `case 'request'` branch. Change default title label (owner decision on Indonesian term).
- `apps/mobile/lib/features/search/search/data/remote/search_api_service.dart:34-56` — Delete dead `contentType` parameter from `searchContents`.
- `apps/mobile/lib/features/search/search/domain/usecases/search_usecase.dart:139-163` — Delete uncalled `searchContents` method.
- `apps/mobile/lib/features/search/search/data/search_repository_impl.dart:563-572` — Delete `_mapContentTypeToType`. Mapper uses `SearchResultType.content` directly from bucket.

### 4.3 Dead Code Removal

- `apps/mobile/lib/shared/services/media_upload_service.dart` — Delete entire file (zero callers, zero DI)
- `apps/mobile/lib/shared/shared.dart:152` — Remove barrel export for media_upload_service.dart
- `apps/mobile/lib/core/src/interfaces/services/i_content_moderation_service.dart` — Delete entire file (zero implementations)
- `apps/mobile/lib/core/core.dart:23` — Remove barrel export
- `apps/mobile/lib/domains/system/analytics/data/repositories/firebase_analytics_repository_impl.dart:111-130` — Delete `trackEngagement` method
- `apps/mobile/lib/core/src/interfaces/services/i_analytics_repository.dart:74` — Remove `trackEngagement` from interface
- `apps/mobile/lib/features/search/search/domain/usecases/search_usecase.dart:139-163` — Delete uncalled `searchContents` method

### 4.4 Tests

**Broken — must be repaired**:
- `internal/social/content/delivery/http/comment_list_integration_test.go` — Fix `contententity.Type` → remove contentType param; update `CreateContent` arity
- `internal/social/content/delivery/http/content_visibility_authority_integration_test.go` — Fix `CreateContent` arity; remove `"type":"post"` from test bodies
- `internal/social/content/application/content_service_share_validation_test.go` — Fix `CreateContent` arity
- `internal/social/content/application/content_share_reference_update_integration_test.go` — Fix `CreateContent` arity
- `internal/social/content/application/content_visibility_authority_integration_test.go` — Fix `CreateContent` arity; remove `type` from INSERTs
- `internal/social/feed/infrastructure/repository/feed_repository_test.go` — Remove `type` from ~16 INSERT statements
- `internal/social/feed/infrastructure/repository/feed_follow_first_bootstrap_test.go` — Remove `type` from 2 INSERT statements

**Chat tests — delete Content-reference cases**:
- `internal/interaction/chat/application/chat_service_attachment_reference_test.go:221-245` — Delete `target_type: "post"/"request"` test cases
- `apps/mobile/test/domains/chat/attachment_contract_alignment_test.dart:68-102,456-475` — Delete post/request wire compat tests

**No-op guards — make executable**:
- `cmd/core_server/content_universal_contract_test.go:172-177` — `TestAntiResurrection_NoContentTypeEnumInMigration`: assert migration SQL has no `content_type_enum`
- `cmd/core_server/content_universal_contract_test.go:180-187` — `TestAntiResurrection_NoFeedDummy`: assert feed_repository_impl.go SELECT has no `type`

**Canonical naming — rename**:
- `content_service_repost_gate_test.go` — Rename `TestCreateRepost_AllowsActivePost/Request` → `...ActiveContent`
- `content_service_moderation_restore_test.go` — Rename `_NormalPost`/`_FulfilledRequest` → `_ActiveContent`
- `comment_list_s4_test.go:197` — Rename `_FulfilledRequest` → `_ActiveContent`
- `share_context_test.go:85`, `content_phase2b_response_test.go:35`, `search_projection_adapter_test.go:38` — Rename case labels `"post"`/`"request"` → `"content"`
- `apps/mobile/test/domains/social/content/create_request_submission_contract_test.dart` — Rename file (tests canonical content)

**Negative guards — REMAIN unchanged**:
- `cmd/core_server/content_universal_contract_test.go:34-168` — strictBindJSON rejection tests
- `content_share_reference_detail_test.go:152-181` — Response omits request metadata
- `feed_share_projection_test.go:331` — Response has no `type` key
- `attachment_validator_test.go:137` — Validator rejects legacy types
- `request_fulfill_contract_test.dart` — No fulfill/ContentType/type field
- `create_request_route_contract_test.dart` — No `/create/request` route
- `content_notification_navigation_behavioral_test.dart` — No post/request navigation

### 4.5 Seed

- Add `user_profiles` INSERTs for seed users (buyer/seller/admin have null usernames)
- Add idempotency guard for contents/comments INSERTs (or document `--clean` requirement)
- No `type` column to add or remove (already canonical)

### 4.6 Purge Tool

- **File**: `apps/mobile/tool/check_universal_content_purge.dart`
- **Current state**: 60 violations, 100% false positives. Cannot gate CI.
- **Blind spots** (must add): single-quoted `'type': 'post'`, `as String? ?? 'post'` cast form, `case 'post':`/`case 'request':` switch literals, `PopupMoreOptionsContentType.post`, `Buat Content`/`Lihat Post` copy, `'Post'`/`'Request'` label literals
- **Allowlist rules** (must follow):
  - Exact-path + exact-semantic only. No file-level allowlist for production paths.
  - No allowlist entry may hide Content/Post/Request residue.
  - Canonical allowlist entries: HTTP request terminology (`createRequest` in payment files), follow-request notification (`case 'request':` in `fcm_action_mapper.dart` — social-graph alias), MIME `content_type` headers (S3, verification, media upload), anti-resurrection guard tests (must contain forbidden strings to assert absence), commerce `sellerResponse` field.
  - Dead `IContentModerationService.ContentType` enum remains in deletion manifest — not allowlisted.

### 4.7 User-Visible Labels (Owner Decision Pending)

30+ occurrences of "Post" / "Buat Content" / "Lihat Post" in user-visible UI. Owner must choose canonical Indonesian term (see Section 7).

---

## 5. Protected Paths

These domains MUST NOT be touched during implementation:

```text
Auth state machine
Email verification portal
Global router/redirect
Stable GoRouter lifetime
Session lifecycle
Listing business authority (pricing, status, negotiation)
Auction business authority (bidding, settlement, anti-sniping)
Checkout
Order
Shipping authority
Seller subscription
Shared media implementation (S3 presign, upload, resolve)
Report/Case
Comments business logic (canonical comment types preserved)
Likes business logic

Chat EXCEPT Content-reference slice:
  Room lifecycle
  Realtime/presence
  Message storage/retrieval
  Media-only sending
  Listing (fixed_price_sale) references → PRESERVED
  Auction references → PRESERVED
  Profile direct-chat entry (recipient routing) → PRESERVED
  Profile as attachment/context → REMOVED (not canonical)
```

---

## 6. Required Executable Proof

### Backend

```bash
# Unit tests (no DB)
go test ./internal/social/feed/... -count=1
go test ./internal/social/content/... -count=1
go test ./cmd/core_server/ -run TestAntiResurrection -count=1 -v

# Integration tests (require test DB)
go test -tags=integration ./internal/social/feed/infrastructure/repository/... -count=1
go test -tags=integration ./internal/social/content/... -count=1

# Chat tests (post-Content-reference removal)
go test ./internal/interaction/chat/... -count=1
```

### Mobile

```bash
flutter analyze lib/features/home/
flutter analyze lib/domains/social/content/
flutter analyze lib/domains/social/share/
flutter analyze lib/domains/chat/chat/
flutter analyze lib/features/search/search/
flutter test test/features/home/
flutter test test/domains/social/
flutter test test/domains/chat/
```

### Purge Tool

```bash
cd apps/mobile && dart run tool/check_universal_content_purge.dart
# Must exit 0
```

### Runtime Verification

```bash
# Feed endpoint returns 200 with items (non-empty after seed)
curl -s http://localhost:8080/api/v1/feed -H "Authorization: Bearer <token>" | jq '.data[0] | keys'
# Every item MUST have feed_item_kind
# Organic item MUST have feed_item_kind = "content"
# Promoted item MUST have feed_item_kind = "promoted_fixed_price_sale" | "promoted_auction" | "promoted_external"
# NO item may have a "type" key
# NO response may use "type" as item discriminator
# NO response may contain Post/Request Content classification

# Search endpoint returns results
curl -s "http://localhost:8080/api/v1/search?q=test" -H "Authorization: Bearer <token>"
# Content items in results.contents[] must NOT have type or contentType fields

# Share-to-feed for listing returns 201
curl -s -X POST http://localhost:8080/api/v1/contents \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"caption":"test","visibility":"public","share_reference":{"targetType":"fixed_price_sale","targetId":"<uuid>","preview":{"title":"test"}}}'

# Strict JSON rejects legacy payloads
curl -s -X POST http://localhost:8080/api/v1/contents \
  -H "Content-Type: application/json" \
  -d '{"caption":"test","type":"post"}' 
# Must return 400 "Invalid request"

# Chat: Listing context dapat dikirim dan dibuka
# Chat: Auction context dapat dikirim dan dibuka
# Chat: Content tidak tersedia sebagai Chat context
# Chat: Profile tidak tersedia sebagai message attachment/context
# Chat: Direct chat dari Profile tetap bekerja
# Chat: Legacy post/request/content/profile attachment payload ditolak
```

---

## 7. Remaining Owner Decision

### Indonesian Content Label

**Status**: `OWNER_DECISION_REQUIRED`

30+ user-visible labels use "Post" / "Buat Content" / "Lihat Post". One canonical Indonesian term must be chosen:

- **A**: "Konten" (consistent with existing usage in "Konten dihapus", "Konten tidak ditemukan")
- **B**: "Kiriman" (colloquial Indonesian social-media term)
- **C**: Keep "Post" (already familiar to users, but English in Indonesian app)

Full inventory available in Pass 2 audit report Section J.

---

## 8. Implementation Slices (Proposed Order)

### Slice 1 — Backend Feed SQL + Envelope Fix
3 files backend. Removes phantom `type` from SELECT. Adds `feed_item_kind` to response projection. Updates promotion injector discriminator from `type` to `feed_item_kind`. Unblocks feed endpoint with canonical envelope.

### Slice 2 — Mobile Feed Consumer Fix
4 files mobile. Removes `type` field from `FeedItemDto`; adds `feed_item_kind`. Updates `FeedResponseDto.fromJson` discriminator. Deletes dead `_mapFeedItemType` mapper. Regenerates `feed_dto.g.dart`. Restores feed rendering.

### Slice 3 — Share/Repost Canonicalization
5 files mobile. Replaces `createShareReferencePost` with canonical `ShareReference.toJson()` payload. Restores listing/auction/profile share-to-feed. Deletes `ExternalShareType.request`; renames `post` → `content`.

### Slice 4 — Chat Content-Reference Hard Purge
6 files backend, 5 files mobile. Removes Content/Post/Request/Profile attachment branches. Preserves Listing→fixed_price_sale and Auction→auction mappings. Narrows validator to `{fixed_price_sale, auction}`. Deletes `validateContentReferenceExists`, error sentinels.

### Slice 5 — Search Cleanup
5 files mobile. Removes `type` and `contentType` fields from `ContentSearchResultDto`. Deletes `_mapContentTypeToType`, dead `contentType` parameter chain, `?? 'post'` fallback, `case 'request'` branch. Mapper uses `SearchResultType.content` directly from bucket.

### Slice 6 — Dead Code, Naming, Docs, Purge Tool
15+ files. Deletes dead files/methods, renames stale tests, fixes docs, hardens purge tool.

### Slice 7 — Integration Tests and Executable Proof
25+ test fixes. Repairs compile errors, removes phantom columns, makes no-op guards executable. Runs full proof suite.

---

## 9. Final Audit Verdict

`CONTENT_UNIVERSAL_AUTHORITY_CANONICAL_AUDIT_FINALIZED_READY_FOR_IMPLEMENTATION_PROMPT`

### What is proven

- Schema is canonical: `contents` has 14 columns, no `type`, no Post/Request enum
- Root cause identified: `feed_repository_impl.go:237` phantom `type` in SELECT
- Mobile also broken: `FeedItemDto.type` required cast on absent wire key
- Share-to-feed broken: `'type':'post'` + `targetType:'listing'` rejected
- Chat Content-reference decided: hard-purge
- 5 dead files/functions identified
- 13 integration compile errors catalogued
- 3 purge-tool blind spots documented
- 30+ UI labels inventoried

### What is NOT needed

- No database migration — schema is canonical
- No business truth change — Post/Request already removed from canonical paths
- No new enum or column
- No backward compatibility

### What IS needed before implementation

1. Owner decision on Indonesian label (Section 7)
2. Design sign-off on slice ordering (Section 8)
3. Implementation authorization

---

*This is the canonical audit document. All implementation must reference this document. Older reports in docs/audits/ are superseded.*
