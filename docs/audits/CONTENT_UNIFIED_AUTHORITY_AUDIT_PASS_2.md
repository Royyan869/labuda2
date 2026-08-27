# CONTENT UNIFIED AUTHORITY — AUDIT PASS 2

**Date**: 2026-08-05
**Scope**: `CONTENT_UNIFIED_AUTHORITY_FEED_RECOVERY_AND_HARD_PURGE`
**Phase**: Audit-Only Pass 2 (no code changes)
**Status**: `AUDIT_PASS_2_COMPLETE_READY_FOR_DESIGN_DECISION`

Canonical Chat decision (2026-08-05): Content MUST NOT be shared to Chat. All Content-reference scaffolding in Chat must be hard-purged. See [CONTENT_UNIVERSAL_AUTHORITY_CANONICAL_AUDIT.md](docs/audits/CONTENT_UNIVERSAL_AUTHORITY_CANONICAL_AUDIT.md).

---

## A. Accepted Facts

Based on Pass 1 findings independently re-verified:

1. **`contents` table**: 14 columns, NO `type` column, NO `content_type_enum`. Confirmed live via `docker exec labuda-postgres psql`.
2. **Feed SQL**: `feed_repository_impl.go:237` outer SELECT on `ranked_feed` still lists `type` — column not in CTE → SQLSTATE 42703.
3. **Feed Scan**: 20 scan targets, 21 SELECT columns after phantom `type` removal. Mismatch.
4. **Chat SQL**: `chat_service.go:2302` queries `contents.type` — same 42703 defect.
5. **Working tree**: Uncommitted mid-refactor state. Entity + CTE already canonical; outer SELECT left behind.
6. **Integration tests broken**: `go vet -tags integration` fails on `comment_list_integration_test.go` (undefined `contententity.Type`) and `content_service_share_validation_test.go` (too many arguments to `CreateContent`).

---

## B. Corrections to Pass 1

| Pass 1 Claim | Correction |
|---|---|
| "Fix is one line: remove `type,` from SELECT" | **INCOMPLETE.** Fixing only the backend SQL still leaves mobile crash: `FeedItemDto.type` is a REQUIRED non-null cast (`json['type'] as String`) — every organic feed item would throw TypeError. Both ends must be fixed. |
| "FeedItemDto.type is legacy Content classification" | **PARTIALLY INCORRECT.** The `type` field IS legacy for the DTO, but the PASS 2 feed wire audit reveals it's also used as the DISCRIMINATOR between organic and promoted items (via `startsWith('promoted_')`). Making it merely optional (`String?`) preserves the discriminator function. |
| "Mobile mapper `_mapFeedItemType` switch on post/request is dead" | **CONFIRMED** but with nuance: the mapper always receives fabricated `'post'` (from `?? 'post'` fallback) or nothing after backend fix. Either way all paths return `FeedItemType.content`. The switch is functionally dead. |
| "Share-to-feed works for listing/auction/profile" | **FALSE.** `createShareReferencePost` sends `'type': 'post'` + `targetType: 'listing'` — both rejected by backend `strictBindJSON` (unknown field) and `oneof=content fixed_price_sale auction profile` (invalid value). ALL listing/auction/profile share-to-feed calls receive HTTP 400. |
| "Only the feed endpoint is broken" | **INCOMPLETE.** The feed endpoint returns 500 (backend SQL), the feed would crash mobile even after SQL fix (required `type` cast), search DTO always fabricates `'post'`, listing/auction/profile share-to-feed is 400, and chat content-reference also hits 42703. FIVE independent breakages exist. |

---

## C. Canonical Contract Maps

### C.1 Feed Wire Contract

```
Backend SQL (CTE feed_base)
  → ranked_feed (CTE adds feed_priority)
    → Outer SELECT (21 cols, includes phantom `type` → BROKEN)
      → rows.Scan → FeedItem entity (no Type field)
        → feedItemToResponseCanonical (map[string]interface{})
          → JSON response (NO `type` key for organic items)
            → FeedResponseDto.fromJson (reads `item['type'] ?? ''` for discriminator)
              → PromotedFeedItemDto (when startsWith('promoted_'))
              → FeedItemDto.fromJson (generated: `json['type'] as String` — REQUIRED → TypeError when null)
                → _mapFeedItemType (switch post/request → FeedItemType.content)
                  → FeedItem (domain entity, type: FeedItemType enum)
                    → renderer (switches on FeedItemType for card dispatch)
                    → share action (ExternalShareType.post for content items)
                    → navigation (/content/:id)
```

**Critical finding**: The discriminator at `FeedResponseDto.fromJson` line 77 uses `item['type']` to separate promoted (`startsWith('promoted_')`) from organic (everything else). This works for promoted detection. However, the downstream `FeedItemDto.fromJson` generated parser then REQUIRES `type` as a non-null String cast on organic items. The backend emits NO `type` key for organic items. **Result: after SQL fix, every organic feed item would throw TypeError → entire feed in error state.**

### C.2 Search Contract

```
Backend: GET /api/v1/search
  → results.contents[] (NO per-item `type` key)
  → results.listings[], results.users[], results.hashtags[]
  → Heterogeneous discrimination: container-level only
    → Mobile: UnifiedSearchResults.fromJson
      → Separate buckets by container
        → ContentSearchResultDto.fromJson
          → Line 274: `json['type'] as String? ?? 'post'` — ALWAYS fabricates 'post'
          → `_mapContentTypeToType` → SearchResultType.content (always)
            → SearchResult type field (SearchResultType.content)
              → Tab: "Post" (label), filtered via `getByType(content)`
              → Navigation via `result.type` switch
```

**Critical finding**: The `type` field in search DTO is a completely fabricated value. The backend never emits it. All heterogeneous search discrimination works correctly at the container level. The `type` → `contentType` → `_mapContentTypeToType` chain is entirely dead Post/Request legacy plumbing. No functional breakage — the search works despite this — but the fabricated `'post'` + stale doc comment (`"type": "post|request|repost"`) violates the canonical contract.

### C.3 Share/Repost Contract

**Content Repost** (Content → Content share):
```
Mobile: ExternalShareType.post
  → createRepost(contentID, originalAuthorID, caption, title, imageUrl)
    → POST /api/v1/contents/{id}/repost ✅ WORKS
      → Backend: RepostContent → CreateRepost
        → New Content with ShareReference(content) + original_author_id
```

**Listing/Auction/Profile share-to-feed** (Commerce → Content share):
```
Mobile: ExternalShareType.listing | auction | profile
  → createShareReferencePost ❌ BROKEN
    → POST /api/v1/contents
      → Payload: { "type": "post", ... } ← REJECTED by strictBindJSON (DisallowUnknownFields)
      → Payload: { targetType: "listing" } ← REJECTED by oneof (not in {content, fixed_price_sale, auction, profile})
      → Payload: { media: ["raw_url"] } ← REJECTED (not {url, type} objects)
      → HTTP 400 on every call
```

**Canonical replacement path**:
```
Mobile: ShareReference.fixedPriceSale(...).toJson() | auction(...).toJson() | profile(...).toJson()
  → POST /api/v1/contents
    → Payload: { caption, visibility, share_reference: { targetType: "fixed_price_sale"|"auction"|"profile", targetId, preview }, media: [{url, type}] }
      → ✅ Accepted by strictBindJSON (no unknown fields) + oneof (valid values) + media struct
```

### C.4 Chat Content Reference Contract

```
Mobile (share to Chat): DISABLED — "Coming soon", no UI path
Mobile (chat composer): content filtered out by _buildReferenceFromChatContext → null
Mobile (normalization): post/request only via _readChatContentType → would normalize to wireTargetType

Backend (validator): accepts {post, request} for content, REJECTS {content} ← DIVERGENCE from content domain
Backend (service): validates via SELECT ... WHERE type = $2 ← SQLSTATE 42703 (column doesn't exist)
Backend (errors): ErrAttachmentPostNotFound/RequestNotFound ← UNREACHABLE (SQL never reaches ErrNoRows)
Backend (reply label): "post"/"request"/"content" → "Content" ← harmless display
```

### C.5 Content Create Contract

```
Mobile: CreateContentDto { caption, visibility, media, tags, share_reference, location }
  → POST /api/v1/contents ✅
    → Backend: strictBindJSON(DisallowUnknownFields) + validateShareReferenceInput
      → NO type field accepted
      → share_reference.targetType: oneof=content|fixed_price_sale|auction|profile
      → media: [{url, type: oneof=image|video}]
```

### C.6 Content Detail / Profile Contract

```
Backend: GET /api/v1/contents/:id
  → response includes author.card with lifecycle
  → NO type key in response
Mobile: ContentDto parsing via generated + hand-written author lifecycle extraction
  → Navigation: /content/:contentId (single canonical route)
  → Profile: ListByAuthor with cursor pagination
```

### C.7 Notifications / Deep Links

```
Backend: notification_worker_social.go — SELECT author_id FROM contents WHERE id = $1 ✅
Mobile: notification_navigation_handler.dart — canonical types only (content.liked, comment, mention)
Deep link: /content/:contentId only ✅
```

---

## D. Reachable Legacy Inventory

| # | Producer | Wire Value | Consumer | Runtime Consequence | Removal Direction |
|---|---|---|---|---|---|
| 1 | `feed_repository_impl.go:237` | `type` in SELECT (phantom) | PostgreSQL | **500 on every GET /api/v1/feed** | Delete `type,` from SELECT |
| 2 | `feed_dto.g.dart:34` | `json['type'] as String` (required cast) | FeedItemDto parser | **TypeError on every organic feed item** | Change to `as String?` + regenerate |
| 3 | `feed_mapper.dart:41-49` | switch `'post'`/`'request'` | FeedItem mapper | No-op — all paths return `FeedItemType.content` | Simplify to direct return |
| 4 | `search_dto.dart:274` | `json['type'] as String? ?? 'post'` | ContentSearchResultDto | Always fabricates `'post'` from null wire | Remove field or change to `?? 'content'` |
| 5 | `search_mapper.dart:62-71` | title generator: `'request'`→'Request', default→'Post' | Search result cards | Fallback labels — actual titles from caption/share | Remove switch, use canonical label |
| 6 | `search_api_service.dart:34-56` | `contentType` parameter (never passed) | No consumer | None — dead parameter | Delete parameter |
| 7 | `share_api_datasource.dart:72` | `'type': 'post'` in payload | `POST /api/v1/contents` | **400 on listing/auction/profile share-to-feed** | Delete field + remove @Deprecated method |
| 8 | `share_api_datasource.dart:72` | `targetType: 'listing'` | Backend oneof validator | **400 on listing share** | Map to `'fixed_price_sale'` |
| 9 | `share_target.dart:100` | `ExternalShareType.request` | Switch arms in 5 files | Dead — never constructed | Delete enum value + branches |
| 10 | `chat_service.go:2302` | `WHERE type = $2` (non-existent column) | Chat message send | **500 on content-attached chat messages** | Delete `AND type = $2` predicate |
| 11 | `validator.go:109` | `validTargetTypes` includes `post`/`request`, rejects `content` | Chat validator | Blocks canonical `content` type | Replace with `{fixed_price_sale, auction, content, profile}` |
| 12 | `chat_repository_impl.dart:516` | `content_type=post or content_type=request` gate | Chat send | Hard-error (unreachable due to UI filter) | Remove gate |
| 13 | `chat_repository.go:185-189` | `ErrAttachmentPostNotFound`/`RequestNotFound` | Chat error mapping | Unreachable (SQL 42703 before ErrNoRows) | Delete sentinels |

---

## E. Dead-Code Manifest

Every candidate proven dead or alive with evidence beyond naming:

### DELETE_WHOLE_FILE

| # | File | Symbol | Proof Summary |
|---|---|---|---|
| 1 | `apps/mobile/lib/shared/services/media_upload_service.dart` | `MediaUploadService`, `UploadResult`, `UploadFailure`, `UploadFileType`, `submitContent`, `_submitPost`, `_submitRequest` | Zero constructor calls, zero DI registrations, zero method callers, zero tests. `_submitPost`/`_submitRequest` both unconditionally throw. Exported via `shared.dart:152` barrel but no consumer imports it. |
| 2 | `apps/mobile/lib/core/src/interfaces/services/i_content_moderation_service.dart` | `IContentModerationService`, `ContentType` enum, `ModerationResult`, `ModerationAction`, `FlaggedContent` | Zero implementations in entire repo. `ContentType.post` zero references. All companion types file-local. Exported via `core.dart:23` barrel. |

### DELETE_METHOD / DELETE_PARAM

| # | File | Symbol | Proof Summary |
|---|---|---|---|
| 3 | `apps/mobile/lib/domains/system/analytics/data/repositories/firebase_analytics_repository_impl.dart:111-130` | `trackEngagement(contentType: ...)` | Zero production callers. Interface `IAnalyticsRepository:74` declares it (must also be removed). 5 test fakes reference it (must be cleaned). |
| 4 | `apps/mobile/lib/features/search/search/data/remote/search_api_service.dart:34-56` | `searchContents({contentType})` param | Never passed by any UI caller. Only logged, never sent to backend. `SearchUseCase.searchContents` (the use-case method) also has zero callers. |
| 5 | `apps/mobile/lib/features/home/data/mappers/feed_mapper.dart:41-49` | `_mapFeedItemType` switch on `'post'`/`'request'` | All branches (including default) return `FeedItemType.content`. Promoted items never reach this method (use `_mapPromotedType`). Replace with inline constant. |

### DELETE_ENUM_VALUE

| # | File | Symbol | Proof Summary |
|---|---|---|---|
| 6 | `apps/mobile/lib/domains/social/share/domain/entities/share_target.dart:100` | `ExternalShareType.request` | Never constructed in entire `lib/`. Dead switch arms in 3 files (share_target.dart, share_preview_card.dart, share_dto.dart). Dart exhaustive switches force cleanup of all 6 branches. |

### RETAIN — Live but Broken

| # | File | Symbol | Status |
|---|---|---|---|
| 7 | `apps/mobile/lib/domains/social/share/data/datasources/share_api_datasource.dart:61-80` | `createShareReferencePost` (`@Deprecated`) | LIVE — called from `share_repository_api.dart:118` for listing/auction/profile share-to-feed. But the flow is **broken at runtime**: sends `'type': 'post'` (rejected by strictBindJSON DisallowUnknownFields → 400) + `targetType: 'listing'` (not in oneof → 400). Method must be retained until flow is fixed, not deleted. |

### REQUIRES_OWNER_DECISION

| # | File | Symbol | Question |
|---|---|---|---|
| 8 | Chat scaffolding (multiple files) | post/request branches, error sentinels, SQL function, normalization logic | Implement or remove? (See Section H.1) |

---

## F. Test Cleanup Manifest

### F.1 Integration Build Proof

**Command**: `cd d:/Project/labuda/backend && go vet -tags integration ./internal/social/content/delivery/http/ 2>&1`
**Exit code**: 1
**Output**:
```
vet.exe: internal\social\content\delivery\http\comment_list_integration_test.go:112:28: undefined: contententity.Type
```

**Command**: `cd d:/Project/labuda/backend && go vet -tags integration ./internal/social/content/application/ 2>&1`
**Exit code**: 1
**Output**:
```
vet.exe: internal\social\content\application\content_service_share_validation_test.go:97:5: too many arguments in call to service.CreateContent
    have (...entity.Visibility, string, ...entity.Visibility, nil, nil, ...)
    want (...string, ...entity.Visibility, *string, *string, ...)
```

**Total**: 13 compile errors across 5 test files. Production code compiles clean (`go build -tags integration` passes; only tests are broken).

### F.2 Test Classification Table

#### BROKEN STALE FIXTURES (won't compile or can't run)

| # | File | Errors | Fix Direction |
|---|---|---|---|
| T1 | `internal/social/content/delivery/http/comment_list_integration_test.go` | 3 errors: NewCommentService arity, undefined contententity.Type, CreateContent arity | Fix to new signatures, drop contentType param |
| T2 | `internal/social/content/delivery/http/content_visibility_authority_integration_test.go` | 3 errors: CreateContent 11-arg calls (:207, :300, :318) | Fix call sites to 10-arg signature |
| T3 | `internal/social/content/application/content_service_share_validation_test.go` | 2 errors: CreateContent 11-arg calls (:97, :147) | Fix call sites |
| T4 | `internal/social/content/application/content_share_reference_update_integration_test.go` | 3 errors: CreateContent 11-arg calls (:62, :159, :233) | Fix call sites |
| T5 | `internal/social/content/application/content_visibility_authority_integration_test.go` | 2 errors: CreateContent 11-arg calls (:151, :213) | Fix call sites |
| T6 | `internal/social/feed/infrastructure/repository/feed_repository_test.go` | ~16 INSERTs with phantom `type` column + `'post'` value | Drop `type` from INSERT column list and VALUES |
| T7 | `internal/social/feed/infrastructure/repository/feed_follow_first_bootstrap_test.go` | 2 INSERTs with phantom `type` column (:103, :156) | Drop `type` from INSERT |

#### LEGACY-POSITIVE (expects old behavior — must fix test bodies)

| # | File | Issue | Fix Direction |
|---|---|---|---|
| T8 | `internal/social/content/delivery/http/content_visibility_authority_integration_test.go` | :529 and others send `{"type":"post"}` expecting 201 — now gets 400 from strictBindJSON | Remove `"type":"post"` from test bodies so real scenarios are tested |
| T9 | Same file | :660 invalid-media test expects 400 `"Invalid media"` — gets 400 `"Invalid request"` (wrong rejection reason) | Remove `"type":"post"` from body |

#### NEGATIVE ANTI-RESURRECTION GUARDS (must REMAIN)

| # | File | What It Guards |
|---|---|---|
| T10 | `cmd/core_server/content_universal_contract_test.go:34-168` | strictBindJSON rejects `{"type":"post"}`, `{"type":"request"}`, `fulfilled_at`, `fulfilled_by`, `"status":"fulfilled"`; response has no `type` field; share_reference targetType preserved |
| T11 | `internal/social/content/delivery/http/content_share_reference_detail_test.go:152-181` | Response omits `request` metadata block |
| T12 | `internal/social/feed/delivery/http/feed_share_projection_test.go:331` | Response has no `type` key |
| T13 | `internal/interaction/chat/delivery/http/attachment_validator_test.go:137` | Validator rejects legacy wire types |
| T14 | `apps/mobile/test/domains/social/content/request_fulfill_contract_test.dart` | No `fulfillRequest`, `ContentType`, `ContentStatus.fulfilled`, `type` field |
| T15 | `apps/mobile/test/core/router/create_request_route_contract_test.dart` | No `/create/request` route |
| T16 | `apps/mobile/test/domains/system/notification/content_notification_navigation_behavioral_test.dart:171-180,216` | No `case 'post':`/`case 'request':`, no `/post/`/`/request/` navigation |

#### NO-OP GUARDS (must become executable)

| # | File | What's Missing |
|---|---|---|
| T17 | `cmd/core_server/content_universal_contract_test.go:172-177` | `TestAntiResurrection_NoContentTypeEnumInMigration` — comment body, no assertion. Should read migration file and assert no `content_type_enum` |
| T18 | `cmd/core_server/content_universal_contract_test.go:180-187` | `TestAntiResurrection_NoFeedDummy` — comment body, no assertion. Should read feed_repository_impl.go and assert SELECT has no `type` or `_unused` |

#### CANONICAL-STALE NAMING (must RENAME)

| # | File | Stale Names |
|---|---|---|
| T19 | `internal/social/content/application/content_service_repost_gate_test.go` | `TestCreateRepost_AllowsActivePost/AllowsActiveRequest/RejectsDeletedRequest/RejectsHiddenPost/RejectsHiddenRequest` — Post/Request pairs are behavioral duplicates (no type distinction exists) |
| T20 | `internal/social/content/application/content_service_moderation_restore_test.go` | `TestRestoreFromModeration_NormalPost...`, `_FulfilledRequest...` (fulfilled state no longer exists) |
| T21 | `internal/social/content/delivery/http/comment_list_s4_test.go:197` | `TestIsParentContentPubliclyListable_FulfilledRequest` — identical to ActiveContent test |
| T22 | `internal/social/content/application/share_context_test.go:85`, `.../content_phase2b_response_test.go:35`, `internal/discovery/search/.../search_projection_adapter_test.go:38` | Table-case names `"post"`/`"request"` — rename to `"content"` |
| T23 | `apps/mobile/test/domains/social/content/create_request_submission_contract_test.dart` | File name is legacy (tests canonical universal content) |

#### REQUIRES_OWNER_DECISION

| # | File | Question |
|---|---|---|
| T24 | `internal/interaction/chat/application/chat_service_attachment_reference_test.go:221-245` | Tests `target_type: "post"/"request"` with mock that never runs real SQL — pins the broken validation. Fix or delete depending on H.1 decision. |
| T25 | `apps/mobile/test/domains/chat/attachment_contract_alignment_test.dart:68-102,456-475` | Tests accept post/request wire target types — pins mobile parse behavior. Fix or keep depending on H.1 decision. |

---

## G. Proposed Scope Boundary — Chat Content-Reference Cleanup

**Files within Content-reference slice (should be cleaned up):**

Backend:
- `internal/interaction/chat/attachmentvalidator/validator.go:109` — validTargetTypes list
- `internal/interaction/chat/application/chat_service.go:1884-1905` — validateAttachmentReferences switch
- `internal/interaction/chat/application/chat_service.go:2292-2313` — validateContentReferenceExists (SQL fix)
- `internal/interaction/chat/application/chat_service.go:2079` — reply preview label
- `internal/interaction/chat/repository/chat_repository.go:185-189` — error sentinel deletion
- `internal/interaction/chat/delivery/http/chat_handler.go:1437-1444` — error mapping update

Mobile:
- `lib/shared/attachment/entities/share_reference.dart:253-278` — chatWireTargetType, asChatReference
- `lib/domains/chat/chat/data/repositories/chat_repository_impl.dart:512-518,756-790` — content_type gate + normalization
- `lib/domains/chat/chat/data/dto/attachment_dto.dart:147-153` — post/request parsing
- `lib/domains/chat/chat/data/dto/message_dto.dart:289` — contentType naming (MIME type, not content subtype)
- `lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart:1486-1497` — _buildReferenceFromChatContext filter

**Protected (NOT in scope):**
- Room lifecycle
- Presence / realtime
- Commerce-chat authority
- Media-only sending
- Unrelated Chat presentation
- Chat message storage/retrieval
- Chat list/pagination

---

## H. Purge-Tool Audit

**File**: `d:/Project/labuda/apps/mobile/tool/check_universal_content_purge.dart`

### H.1 Current State

- **44 patterns**, case-insensitive, scanned over `apps/mobile/lib` + `apps/mobile/test` (`.dart` only)
- **Currently FAILS with 60 violations — 100% false positives**
- Cannot gate CI in current state
- False positives from: HTTP request terminology (`createRequest` in payment), anti-resurrection guard tests (which must contain forbidden strings to assert absence), docstring examples (`postId` in code samples)

### H.2 Three Critical Blind Spots (with file:line evidence)

| # | Blind Spot | Why Tool Misses | Live/Dead | File:line |
|---|---|---|---|---|
| 1 | `json['type'] as String? ?? 'post'` | Pattern 42 expects `json['type']\s*\?\?\s*'post'` but the ` as String? ` cast sits between `]` and `??` — adjacency broken | **LIVE** — always executes (backend never emits `type`) | `search_dto.dart:274` |
| 2 | `'type': 'post'` (single-quoted) | Patterns 16/17 match only `"type": "post"` (double-quoted JSON) | **LIVE in @Deprecated method** — still called from `share_repository_api.dart:118` | `share_api_datasource.dart:72` |
| 3 | `case 'post': case 'request':` | No case-literal pattern exists at all | Dead (all branches → `FeedItemType.content`) | `feed_mapper.dart:43-44` |

### H.3 Additional Evasions Found (9 more live patterns with zero tool coverage)

| File:line | Code | Live/Dead |
|---|---|---|
| `search_mapper.dart:63-71` | `case 'request': return 'Request'; default: return 'Post';` | **LIVE** — user-visible fallback titles |
| `share_reference.dart:259-260,370-371` | `case 'post'/'request'` in chatWireTargetType | Live-compat |
| `attachment_dto.dart:148-149` | `case 'post'/'request'` → content | Live-compat |
| `chat_repository_impl.dart:784-785` | `case 'post'/'request'` in normalization | Live-compat (hidden behind MIME allowlist) |
| `media_upload_service.dart:138-145` | `case 'post'/'request'` + exception strings | Dead (always throws) |
| `fcm_action_mapper.dart:50` | `label: 'Lihat Post'` | **LIVE user-visible** (like-notification button) |
| `popup_more_options_button.dart:7` | `enum PopupMoreOptionsContentType { post, ... }` | **LIVE** — member `post` used as default |
| `share_target.dart:100` | `enum ExternalShareType { post, ..., request, ... }` | **LIVE** — `post` active, `request` dead |
| `share_reference.dart:31` | `content('...','Post')` — displayName | **LIVE** |

### H.4 Proposed New Patterns (priority order)

```dart
_ForbiddenPattern(r"case\s+'post':",            'switch case literal "post"'),       // catches 6 files
_ForbiddenPattern(r"case\s+'request':",         'switch case literal "request"'),
_ForbiddenPattern(r"'type'\s*:\s*'post'",       "single-quoted 'type': 'post'"),    // catches share_api
_ForbiddenPattern(r"json\['type'\]\s*as\s*String\?\s*\?\?\s*'post'", 'cast-fallback to post'), // catches search_dto
_ForbiddenPattern(r"\?\?\s*'post'",             'any ?? post fallback'),
_ForbiddenPattern(r"\b_submitPost\b|\b_submitRequest\b", 'legacy submit methods'),
_ForbiddenPattern(r"\bPopupMoreOptionsContentType\.post\b", 'popup .post member'),
_ForbiddenPattern(r"\bExternalShareType\.(post|request)\b", 'share type legacy members'),
_ForbiddenPattern(r"\bBuat Content\b",          '"Buat Content" copy (rename to Buat Konten)'),
_ForbiddenPattern(r"\bLihat Post\b",            '"Lihat Post" copy'),
```

### H.5 Tool Cannot Gate CI Until

1. Allowlist the 60 current false positives (payment `createRequest`, guard tests, doc comments)
2. Add the blind-spot patterns above
3. Replace file-level allowlist (`chat_repository_impl.dart` MIME skip) with per-pattern `allowedIn` so the file's non-MIME post/request code is still caught
4. Fix the real violations found (do NOT allowlist them)

---

## I. Seed and Runtime Data

### I.1 Seed Content Matrix

| Batch | Count | status | is_hidden | visibility | deleted_at | Author | Media |
|---|---|---|---|---|---|---|---|
| Normal | 20 | `'active'` | `false` | DB default `'public'` | NULL | sellerID `...002` | none |
| Hidden | 3 | `'active'` | `true` | `'public'` | NULL | sellerID | none |
| Deleted | 2 | `'active'` → soft-deleted via ContentService | `false` | `'public'` | set | sellerID | none |

- **No `type` column**: Verified against migration 000001:671-686 and live DB. Correct.
- **Idempotency**: Users ON CONFLICT upsert (idempotent). **Contents are plain INSERT with fresh UUIDs — NOT idempotent.** Each rerun adds 25 more rows. `--clean` mode truncates first.
- **Prerequisites**: 3 UUID-fixed users (`...000001/2/3`) with `email_verified_at` and `account_status='active'` — all exist in live DB. Seed runnable as-is.
- **Gap: `user_profiles` empty for all 3 seed users** (no username/avatar) — feed author cards will render nulls. Fix: add profile INSERTs to seed.
- **Gap: No `content_media` rows** — all content is text-only. Acceptable but document.

### I.2 Expected Post-Seed State

| Metric | Count |
|---|---|
| Total contents | 25 |
| Feed-visible | **20** (active, not hidden, not deleted, public, author active) |
| Hidden | 3 |
| Deleted | 2 |
| Run command | `cd backend && go run ./cmd/seed` (additive) or `--clean` (truncate-reseed) |

---

## J. UI Terminology Inventory

30+ user-visible occurrences catalogued. **Owner must choose canonical Indonesian term.**

| Count | Current | Context |
|---|---|---|
| 6 | `'Post'` | Search tab, filter chip, result label, share sheet, share badge, mapper fallback |
| 5 | `'Buat Content'` | Create button, bottom sheet, quick action, app bar |
| 1 | `'Lihat Post'` | Notification banner (like action) |
| 1 | `'Konten'` | Already neutral — content detail deleted state |
| 1 | `'Feed'` | Profile content tab |
| 1 | `'Request'` | Dead share label (mapper fallback) |
| 1 | `'Content'` | Report target label |

**Options for owner**: (A) "Konten" everywhere, (B) "Kiriman" for colloquial Indonesian, (C) Keep "Post" as-is. Full table in the agent report.

---

## K. Working-Tree Integrity

- **958 modified files** in working tree (git status --short)
- **Branch**: `main`, parent `41aa4ee fix(seller): guard listing creation and renewal flow`
- Key deleted files in working tree: `i_authentication_service.dart`, `i_presence_service.dart`, `app_lifecycle_observer.dart`, `app_presence_service_api.dart`, `auction_list_screen.dart`
- Backend `content_type.go` deleted (entity Type definition removed) — root of integration test compile failures
- Feed repository + entity modified (mid-refactor — CTE canonical, outer SELECT stale)
- All changes are uncommitted; current filesystem IS authority per scope rules
- No inconsistency: Pass 1's baseline commit claim matches

---

## L. Owner Decisions Required

### L.1 Chat: Content Reference — Hard Purge (DECIDED)

**Decision (2026-08-05)**: Content MUST NOT be shared to Chat. All Content/Post/Request attachment scaffolding must be hard-purged from Chat. Listing (`fixed_price_sale`) and Auction references remain canonical.

**Scope of Chat hard-purge (Content-reference slice only)**:
- Backend: Delete `validateContentReferenceExists` function, `post`/`request` branches in `validateAttachmentReferences`, `ErrAttachmentPostNotFound`/`ErrAttachmentRequestNotFound` sentinels, `case "post"/"request"/"content"` in reply preview
- Backend: Update `validator.go:109` `validTargetTypes` to `{fixed_price_sale, auction, profile}`
- Mobile: Delete `chatWireTargetType`, `asChatReference`, `_normalizeReferenceForChat`, `_readChatContentType`, `content_type=post/request` gate
- Tests: Delete post/request attachment validation tests and mocks
- Protected: room lifecycle, realtime, presence, message storage, media sending, Listing/Auction references

### L.2 Indonesian Content Label
**Status**: `OWNER_BUSINESS_DECISION_REQUIRED`

30+ user-visible labels reference "Post" / "Buat Content" / "Lihat Post".

**Options**:
- **A**: "Konten" (already used in some areas — "Konten dihapus", "Konten tidak ditemukan")
- **B**: "Kiriman" (colloquial Indonesian social-media term)
- **C**: Keep "Post" (already familiar, but English in Indonesian app)

### L.3 Feed Discriminator Contract
**Status**: `DESIGN_DECISION_REQUIRED`

After backend SQL fix, feed organic items have NO `type` key. Two paths:
- **Option A (absence-as-organic)**: No `type` for organic, mobile treats null/empty as organic. Asymmetric but canonical.
- **Option B (explicit `type`)**: Add `"type": "content"` to backend response. Symmetric, self-documenting.

---

## M. Implementation Slices (Proposed — Do Not Implement)

Ordered by blast radius and dependency chain:

### Slice 1 — Backend Feed SQL Fix
**Blast radius**: 1 line. **Unblocks**: Feed endpoint.
- `feed_repository_impl.go:237`: remove `type,` from outer SELECT
- Verify 20-column SELECT ↔ 20-column CTE ↔ 20-target Scan

### Slice 2 — Mobile Feed Consumer Fix
**Blast radius**: 3 files. **Unblocks**: Feed rendering on mobile.
- `feed_dto.dart:130`: Remove `type` field from `FeedItemDto`
- Regenerate `feed_dto.g.dart` after removal
- Update `FeedResponseDto.fromJson` discriminator to route organic by absence of `type` key (already handles null via `?? ''`)
- `feed_mapper.dart:41-49`: Delete `_mapFeedItemType`; inline `FeedItemType.content` at call site
- Verify discriminator still works for promoted items via `startsWith('promoted_')`

### Slice 3 — Share/Repost Contract Fix
**Blast radius**: 4 files mobile. **Fixes**: Listing/auction/profile share-to-feed (currently 400).
- `share_api_datasource.dart`: Replace `createShareReferencePost` with canonical payload (drop `'type':'post'`, map `listing`→`fixed_price_sale`, send media as `[{url,type}]`)
- `share_target.dart`: Delete `ExternalShareType.request`, rename `post`→`content`
- `share_preview_card.dart`, `share_dto.dart`: Remove `request` branches
- `feed_renderers.dart`: Update `ExternalShareType.post`→`.content`

### Slice 4 — Chat Content-Reference Fix
**Blast radius**: 6 backend, 5 mobile. **Depends on**: Owner decision L.1.
- Backend: Fix SQL (drop `AND type = $2`), align validator `validTargetTypes`, collapse error sentinels, update handler mappings
- Mobile: Remove `content_type=post/request` gate, simplify normalization to `content`, remove dead branches

### Slice 5 — Search Contract Cleanup
**Blast radius**: 4 files mobile. **No functional break** — cosmetic.
- `search_dto.dart:274`: Change `?? 'post'` to `?? ''` or remove field
- `search_mapper.dart:62-71`: Remove `'request'` case, change default to `'Konten'`
- `search_api_service.dart`: Delete dead `contentType` parameter chain
- Tab/filter labels: Change 'Post' → 'Konten' (per owner decision L.2)

### Slice 6 — Dead Code, Naming, Docs, Purge Tool
**Blast radius**: 15+ files. **Post-cleanup**.
- Delete: `MediaUploadService`, `IContentModerationService` (2 whole files)
- Delete methods: `trackEngagement`, `searchContents` use-case, `_mapFeedItemType` switch
- Delete enum values: `ExternalShareType.request`
- Rename: ~8 test files/functions with stale Post/Request naming
- Fix: `content_card.go:34-35` stale doc block
- Harden: Purge tool — add 10+ blind-spot patterns, fix 60 false positives, add allowlist

### Slice 7 — Integration Tests and Executable Proof
**Blast radius**: ~25 test fixes. **Verification gate**.
- Fix 13 compile errors in 5 integration test files
- Fix ~18 phantom `type` column INSERTs in 2 test files
- Replace 2 no-op anti-resurrection tests with executable assertions
- Run full suite: `go test -tags integration ./internal/social/...`
- Run purge tool: must exit 0
- Verify feed endpoint returns 200 with correct shape

### Slice Dependency Graph
```
Slice 1 (backend SQL) ──┐
                         ├──> Slice 2 (mobile parser) ──> Slice 5 (search) ──> Slice 6 (cleanup)
Slice 3 (share/repost) ──┤                                    │
                         │                                    └──> Slice 7 (proof)
Slice 4 (chat - gated) ──┘
```
Slices 1+3 are independent. Slice 4 requires owner decision. Slice 2 depends on Slice 1 (need to see what backend actually emits). Slices 5-7 are post-stabilization.

---

## N. Verdict

`AUDIT_PASS_2_COMPLETE_READY_FOR_DESIGN_DECISION`

### Evidence base

| Contract Area | Status | Key Finding |
|---|---|---|
| Database schema | **Canonical** | 14 columns, no `type`, no Post/Request enum, 0 rows |
| Feed wire | **Broken both ends** | Backend SQL 500 (phantom `type` column) + Mobile TypeError (required `type` cast) |
| Search contract | **Fabricated values** | `?? 'post'` always fires; dead `contentType` parameter chain; tabs work via bucket-level discrimination |
| Share/Repost | **Broken (400)** | Listing/auction/profile share-to-feed blocked by `'type':'post'` + `targetType:'listing'` |
| Chat content-reference | **Broken + Owner Decision Required** | SQL 42703, validator rejects canonical `content`, accepts legacy `post`/`request` |
| Dead code | **5 files/functions confirmed** | MediaUploadService, IContentModerationService, trackEngagement, searchContents use-case, _mapFeedItemType switch |
| Test authority | **13 compile errors, ~25 fixture errors** | Integration build broken; production code compiles |
| Purge tool | **100% false-positive, 3 blind spots** | 60/60 FPs; cannot gate CI |
| Seed data | **Gap: no user_profiles** | Content rows correct (no `type`), 20 visible, not idempotent |
| UI terminology | **30+ user-visible "Post" labels** | Owner must choose Indonesian label |
| Working tree | **958 modified files** | Mid-refactor; no inconsistency with Pass 1 claims |

### What is NOT needed
- **No database migration** — schema is canonical
- **No business truth change** — Post/Request already designed out
- **No new enum or column** — forbidden by scope rules
- **No Git history restoration** — forbidden by scope rules

### What IS needed before implementation
1. **Owner decision L.1** (Chat content-reference: implement or remove?)
2. **Owner decision L.2** (Indonesian label: "Konten", "Kiriman", or keep "Post"?)
3. **Owner decision L.3** (Feed discriminator: absence-as-organic or explicit `"type":"content"`?)
4. **Design decision** (Slice ordering — confirm or reorder)

### Complexity estimate
- **Slice 1**: 1 line change (trivial)
- **Slice 2**: ~5 line changes (trivial after Slice 1)
- **Slice 3**: ~50 line changes (moderate — payload contract rewrite)
- **Slice 4**: ~80 line changes (moderate — depends on L.1 decision)
- **Slice 5**: ~30 line changes (trivial — cosmetic)
- **Slice 6-7**: ~200 line changes (moderate — bulk mechanical)

**Total**: ~350 lines changed across 7 slices, with most complexity concentrated in Slices 3+4.

---

*End of PASS 2 audit. Awaiting owner decisions before implementation.*

The capability is fully scaffolded end-to-end (mobile normalization, backend validator, service branch, SQL, error types) but:
- UI excludes it ("Coming soon" destination, composer filter)
- Backend SQL is broken (42703)
- Validator accepts legacy `post`/`request` but rejects canonical `content`

**Options:**
- **A: Implement the feature** → Fix backend SQL + validator + normalize wire to `content`; implement share destination UI
- **B: Remove the scaffolding** → Delete `post`/`request` branches, error sentinels, normalization logic, SQL function
- **C: Defer but fix latent bugs** → Fix SQL to not crash, align validator with canonical types, keep UI disabled

### H.2 Indonesian Content Label
**Status**: `OWNER_BUSINESS_DECISION_REQUIRED`

Current user-visible labels use "Post" (English) for Content. Search tab, filter chips, content detail header, feed empty state all reference "Post". Options:
- **A**: Indonesian "Konten" for consistency
- **B**: Indonesian "Kiriman" (more colloquial Indonesian social-media term)
- **C**: Keep "Post" (already familiar to users, but English in Indonesian app)

Awaiting full UI terminology inventory from agent.

### H.3 Feed Discriminator Contract
**Status**: `DESIGN_DECISION_REQUIRED`

After SQL fix, the backend feed wire has NO `type` key for organic items. Two options:
- **Option A (absence-as-organic)**: Keep no `type` for organic, mobile treats null/empty as organic. Asymmetric but canonical.
- **Option B (explicit `type`)**: Add `"type": "content"` to backend response, mobile maps it explicitly. Symmetric, self-documenting.

---

## I. Proposed Implementation Slices

Based on audit evidence — order reflects dependency chains and blast radius:

### Slice 1 — Backend Feed SQL Fix (smallest blast radius, unblocks everything)
- `feed_repository_impl.go:237`: remove `type,` from outer SELECT
- Verify CTE/SELECT/Scan column counts match (20-20-20)

### Slice 2 — Mobile Feed Consumer Fix (restores feed rendering)
- `feed_dto.dart`: make `FeedItemDto.type` optional (`String?` with `@JsonKey`)
- Regenerate `feed_dto.g.dart`
- Simplify `_mapFeedItemType` to direct return
- Verify discriminator still works for promoted items

### Slice 3 — Share/Repost Contract Fix (restores listing/auction/profile share-to-feed)
- Delete `createShareReferencePost` deprecated method
- Create single canonical share creation method using `ShareReference.toJson()`
- Map `ExternalShareType.listing` → `fixed_price_sale` wire value
- Delete `ExternalShareType.request`
- Update all callers to use canonical method

### Slice 4 — Chat Content-Reference Fix (depends on H.1 decision)
- If H.1-A (implement): Fix SQL, align validator, implement UI
- If H.1-B (remove): Delete `post`/`request` branches, SQL function, error sentinels
- If H.1-C (defer): Fix SQL only, align validator, keep UI disabled

### Slice 5 — Search Contract Cleanup (no functional break — cosmetic)
- Remove fabricated `type`/`contentType` from search DTO and mapper
- Delete dead `searchContents({String? contentType})` parameter chain
- Update tab label (owner decision H.2)

### Slice 6 — Tests, Dead Code, Naming, Docs, Purge Tool
- Fix/delete broken integration test fixtures
- Rename canonical tests with stale Post/Request naming
- Delete proven dead code (MediaUploadService, etc.)
- Update stale documentation
- Harden purge tool with Pass 2 blind-spot patterns

### Slice 7 — Executable Proof and Residue Closure
- Run full canonical contract test suite
- Run purge tool — must pass
- Run feed integration tests — must pass
- Verify mobile build compiles and feed renders

---

*End of PASS 2 audit. See [CONTENT_UNIVERSAL_AUTHORITY_CANONICAL_AUDIT.md](docs/audits/CONTENT_UNIVERSAL_AUTHORITY_CANONICAL_AUDIT.md) for the consolidated implementation authority.*

---

**SUPERSEDED FOR IMPLEMENTATION AUTHORITY**

Use:
[docs/audits/CONTENT_UNIVERSAL_AUTHORITY_CANONICAL_AUDIT.md](docs/audits/CONTENT_UNIVERSAL_AUTHORITY_CANONICAL_AUDIT.md)

This file remains only as historical audit evidence.
