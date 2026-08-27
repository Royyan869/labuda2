**SUPERSEDED FOR IMPLEMENTATION AUTHORITY**

Use:
[docs/audits/CONTENT_UNIVERSAL_AUTHORITY_CANONICAL_AUDIT.md](docs/audits/CONTENT_UNIVERSAL_AUTHORITY_CANONICAL_AUDIT.md)

This file remains only as historical audit evidence.

---

# CONTENT UNIFIED AUTHORITY, FEED RECOVERY, AND HARD-PURGE — AUDIT REPORT

**Date**: 2026-08-05
**Scope**: `CONTENT_UNIFIED_AUTHORITY_FEED_RECOVERY_AND_HARD_PURGE`
**Phase**: Audit-Only First Pass (no code changes, no migrations, no deletions)

---

## 1. Verdict

**ROOT CAUSE CONFIRMED.** `GET /api/v1/feed` returns HTTP 500 `SQLSTATE 42703 column "type" does not exist` because `feed_repository_impl.go:237` selects a phantom `type` column from a CTE that never produces one.

The `contents` table schema is already canonical — it has **no `type` column**. The canonical migration (000001) never created one. No Post/Request enum exists. The schema is correct; the query is stale.

A **second production path** with the same root cause exists in `chat_service.go:2302` (protected domain — noted, not in scope).

The working tree is in a **mid-refactor state** with `content_type.go` deleted, `feed_item.go` stripped of its `Type` field, and the `feed_base` CTE correctly omitting `type` — but the outer SELECT on `ranked_feed` was left behind.

---

## 2. Current Database Truth

### Connection
- PostgreSQL 16.13, Docker container `labuda-postgres`, port 5432
- Database: `labuda`, user: `labuda`
- Commands executed via `docker exec labuda-postgres psql -U labuda -d labuda`

### `contents` table — 14 columns, NO `type`

```sql
SELECT column_name, data_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_name = 'contents'
ORDER BY ordinal_position;
```

| column | data_type | nullable | default |
|---|---|---|---|
| id | uuid | NO | gen_random_uuid() |
| author_id | uuid | NO | — |
| status | content_status_enum | NO | 'active' |
| caption | text | YES | — |
| city | text | YES | — |
| province | text | YES | — |
| is_hidden | boolean | NO | false |
| original_author_id | uuid | YES | — |
| share_reference | jsonb | YES | — |
| created_at | timestamptz | NO | now() |
| updated_at | timestamptz | NO | now() |
| deleted_at | timestamptz | YES | — |
| search_vector | tsvector | YES | GENERATED STORED |
| visibility | content_visibility_enum | NO | 'public' |

```sql
-- Proof: type column does not exist
SELECT column_name FROM information_schema.columns
WHERE table_name = 'contents' AND column_name = 'type';
-- Returns: 0 rows
```

### Enum types
- `content_status_enum`: `active`, `deleted`
- `content_visibility_enum`: `public`, `followers_only`, `private`
- **No `content_type_enum` exists. No Post/Request values in any enum.**

### Row counts
```sql
SELECT COUNT(*) FROM contents;              -- 0 total
SELECT COUNT(*) WHERE is_hidden = true;     -- 0
SELECT COUNT(*) WHERE deleted_at IS NOT NULL; -- 0
```

### Indexes on contents (no index on type)
- `contents_pkey` (id)
- `idx_contents_author_id` (author_id, created_at DESC)
- `idx_contents_search_vector` GIN
- `idx_contents_status` (status)
- `idx_contents_visibility` (visibility)

### Migration sync
- Exactly 30 migrations applied (001–030), all `dirty = false`
- `000001_canonical_schema.up.sql:671-686` creates `contents` with 14 columns — no `type`
- No migration ever adds/drops `type` on `contents`

---

## 3. Feed Root Cause

### Exact failure location

**File**: `d:/Project/labuda/backend/internal/social/feed/infrastructure/repository/feed_repository_impl.go`
**Line**: 237

```sql
-- feed_base CTE (lines 102-219): selects 19 columns. No `type`.
-- ranked_feed CTE (lines 221-235): feed_base.* + feed_priority = 20 columns. No `type`.
-- Final SELECT (line 237):
SELECT
    id, author_id, type, status,   -- ❌ `type` does not exist in ranked_feed
    body, caption,
    city, province,
    is_hidden, created_at, updated_at,
    original_author_id, share_reference,
    author_username, author_avatar,
    author_city, author_province,
    author_account_status, author_deleted,
    media, feed_priority
FROM ranked_feed
```

`type` resolves against `ranked_feed` → column not found → PostgreSQL raises `SQLSTATE 42703` at plan time. The query never reaches execution, regardless of data.

### Secondary bug (same query)

The `rows.Scan` call (lines 276-297) has **20 scan targets** but the broken SELECT has **21 columns** (including phantom `type`). Even if `type` existed in the CTE, the scan would fail with a column-count mismatch.

### Call chain

```
GET /api/v1/feed
→ routes_core.go:1243  feedRoutes.GET("", deps.FeedHandler.GetFeed)
→ feed_handler.go:51   GetFeed() → h.feedService.GetFeed(ctx, tx, callerID, cursor, limit)
→ feed_service.go:45   GetFeed() → s.feedRepo.GetFeed(ctx, tx, callerID, cursor, limit)
→ feed_repository_impl.go:43  GetFeed() → q.Query(ctx, query, ...)
→ PostgreSQL: ERROR column "type" does not exist → 500
```

### Why the error surfaces as HTTP 500

`feed_handler.go:101-107`: Any error from `feedService.GetFeed` is logged and mapped to `response.InternalServerError(c, "Failed to retrieve feed")`.

---

## 4. Legacy Authority Inventory

### 4.1 Production Backend — Feed (P0, in-scope)

| # | File | Reference | Classification | Runtime |
|---|---|---|---|---|
| 1 | `internal/social/feed/infrastructure/repository/feed_repository_impl.go:237` | `SELECT ... type, ... FROM ranked_feed` | **legacy Content classification** — residual column in SELECT, CTE already canonical | **P0 — HTTP 500 on every feed request** |
| 2 | `internal/social/feed/infrastructure/repository/feed_repository_impl.go:276-297` | `rows.Scan` with 20 targets, SELECT has 21 columns | **duplicate authority** — scan/select column mismatch | Would 500 even if `type` existed |

### 4.2 Production Backend — Chat Attachment Validation (P0, protected)

| # | File | Reference | Classification | Runtime |
|---|---|---|---|---|
| 3 | `internal/interaction/chat/application/chat_service.go:2302` | `SELECT id FROM contents WHERE id = $1 AND type = $2 AND deleted_at IS NULL` | **legacy Content classification** — queries `contents.type` | **P0 — HTTP 500 on chat messages with content attachment** |
| 4 | `internal/interaction/chat/application/chat_service.go:1889-1902` | Switch on `"post"` / `"request"` target types | **legacy Content classification** — distinguishes Post/Request | Active caller path; also handles `case "content":` with legacy `data["content_type"]` sub-switch |
| 5 | `internal/interaction/chat/repository/chat_repository.go:185-189` | `ErrAttachmentPostNotFound` / `ErrAttachmentRequestNotFound` | **legacy Content classification** — Post/Request error types | Unreachable today (SQL errors before ErrNoRows), but active definitions |
| 5a | `internal/interaction/chat/attachmentvalidator/validator.go:109` | `validTargetTypes := {"fixed_price_sale", "auction", "post", "request", "profile"}` — **rejects `"content"`** which is the canonical target type in `dto/share_reference_request.go:6` (`oneof=content fixed_price_sale auction profile`) | **legacy Content classification** — validator diverges from canonical DTO; rejects valid `"content"` type, accepts legacy `"post"/"request"` | Active HTTP-boundary validation |

### 4.3 Test Residue

| # | File | Reference | Classification |
|---|---|---|---|
| 6 | `internal/social/feed/infrastructure/repository/feed_repository_test.go:56,365,636,716,725,766,775,814,823,862,925,962,999,1036,1086,1136` | `INSERT INTO contents (id, author_id, type, status, ...)` | **test residue** — inserts into phantom `type` column; tests cannot pass against canonical schema |
| 7 | `internal/social/feed/infrastructure/repository/feed_follow_first_bootstrap_test.go:103,156` | `INSERT INTO contents (id, author_id, type, status, ...)` | **test residue** — same phantom column |
| 8 | `internal/worker/notification_worker_social_test.go:1479` | `// First call: SELECT author_id, type FROM contents.` | **test residue** — comment references old query; mock returns 2 values but production code selects 1 column |
| 9 | `internal/social/content/delivery/http/comment_list_integration_test.go:112,122` | `contentType contententity.Type` | **test residue** — references `contententity.Type` which no longer exists; **breaks `-tags integration` build entirely** |
| 9a | `internal/social/content/delivery/http/content_visibility_authority_integration_test.go:457,487-495,529,660,802,897,988-992` | Request bodies `{"type":"post",...}` expecting HTTP 201 | **test residue** — strictBindJSON rejects unknown `type` field; all will 400 instead of 201 |
| 9b | `internal/social/content/application/content_visibility_authority_integration_test.go:100-106` | `INSERT INTO contents (..., type, ...)` with `'post'` | **test residue** — phantom `type` column; runtime DB error |

### 4.4 Backend Contract Tests (anti-resurrection stubs)

| # | File | Reference | Classification |
|---|---|---|---|
| 10 | `cmd/core_server/content_universal_contract_test.go:172-177` | `TestAntiResurrection_NoContentTypeEnumInMigration` — body is a comment, no validation | **test residue** — no-op test |
| 11 | `cmd/core_server/content_universal_contract_test.go:180-187` | `TestAntiResurrection_NoFeedDummy` — body is a comment, no validation | **test residue** — no-op test |

### 4.5 Backend Stale Documentation

| # | File | Reference | Classification |
|---|---|---|---|
| 11a | `internal/pkg/publiccard/content_card.go:34-35` | Doc block claims "Type: required string ('post','request',…)" but struct (line 46-56) has **no Type field** and line 48 correctly says "Universal Content: no type field" | **documentation residue** — self-contradictory doc |
| 11b | `internal/worker/notification_worker_social.go:159` | Comment: "targetType fallback to 'post' on DB error is intentional — do not remove" — describes code that no longer exists | **documentation residue** — stale comment |
| 11c | `cmd/core_server/routes_core.go:1241,1247,1259,1282` | Comments mention "posts, requests, and reposts" / "seller responses to requests" | **documentation residue** |

### 4.5 Mobile — FeedItemDto (P1)

| # | File | Reference | Classification |
|---|---|---|---|
| 12 | `apps/mobile/lib/features/home/data/dto/feed_dto.dart:130` | `final String type;` — required field | **legacy Content classification** — expects `type` from wire but backend doesn't emit it |
| 13 | `apps/mobile/lib/features/home/data/dto/feed_dto.g.dart:34` | `type: json['type'] as String` — non-null cast | **duplicate authority** — will throw TypeError when backend returns items without `type` key |
| 14 | `apps/mobile/lib/features/home/data/mappers/feed_mapper.dart:41-49` | `_mapFeedItemType` switch on `'post'`/`'request'` → all paths return `FeedItemType.content` | **legacy Content classification** — dead switch, always returns same value |
| 15 | `apps/mobile/lib/features/home/data/remote/feed_api_datasource.dart:25` | Comment: "Social content only (posts, requests, reposts)" | **documentation residue** — mentions Post/Request |

### 4.6 Mobile — Share Domain (P1, in-scope)

| # | File | Reference | Classification |
|---|---|---|---|
| 16 | `apps/mobile/lib/domains/social/share/domain/entities/share_target.dart:100` | `enum ExternalShareType { post, listing, request, auction, profile }` — has both `post` AND `request`; `request` never constructed, always falls into same `/content/$id` URL | **legacy Content classification** — dead `request` value, live `post` naming |
| 17 | `apps/mobile/lib/domains/social/share/data/datasources/share_api_datasource.dart:56-80` | `@Deprecated createShareReferencePost` sends `'type': 'post'` in API payload at line 72; still called from `share_repository_api.dart:118` | **legacy Content classification** — active emission, single-quoted form dodges purge tool |
| 18 | `apps/mobile/lib/domains/social/share/presentation/widgets/share_preview_card.dart:115-118,375-403` | Separate request/post icon + label branches ('Request'/'Post') | **legacy Content classification** — dead `request` branch |
| 19 | `apps/mobile/lib/domains/social/share/data/dto/share_dto.dart:102-107` | `_parseTargetType` with `orElse: () => ExternalShareType.post`; `generateDefaultCaption` keeps request caption | **legacy Content classification** — `post` default |

### 4.7 Mobile — Search DTO (P2, in-scope, purge-tool blind spot)

| # | File | Reference | Classification |
|---|---|---|---|
| 20 | `apps/mobile/lib/features/search/search/data/dto/search_dto.dart:274` | `type: json['type'] as String? ?? 'post'` — fallback to `'post'`, exactly the pattern the purge tool should catch but misses due to `as String?` cast | **legacy Content classification** — PURGE TOOL BLIND SPOT |

### 4.8 Mobile — Dead Code (in-scope removal)

| # | File | Reference | Classification |
|---|---|---|---|
| 21 | `apps/mobile/lib/shared/services/media_upload_service.dart:134-163` | `submitContent()` switches on `'post'/'request'`; entire `MediaUploadService` class has **zero call sites** | **dead code** |
| 22 | `apps/mobile/lib/domains/system/analytics/data/repositories/firebase_analytics_repository_impl.dart:111-130` | `trackEngagement(contentType: ...)` — **zero callers** | **dead code** |
| 23 | `apps/mobile/lib/core/src/interfaces/services/i_content_moderation_service.dart:62` | `enum ContentType { text, image, video, audio, post, comment, listing, profile }` — `post` value; interface has **no implementation** | **dead code** |

### 4.9 Mobile — Cosmetic/Legacy Naming (P3, in-scope)

| # | File | Reference | Classification |
|---|---|---|---|
| 24 | Multiple search files | UI labels content as "Post": `search_results_screen.dart:63` Tab, `global_search_bar.dart:139` FilterChip, `search_result_type_helper.dart:48`, `search_mapper.dart:70` title → 'Post', `search_repository_impl.dart:452` subtitle → 'Post' | **legacy naming residue** |
| 25 | `apps/mobile/lib/shared/widgets/popup_more_options_button.dart:7` | `enum PopupMoreOptionsContentType { post, profile, listing, auction }` — `post` used as default content type label | **legacy naming residue** |
| 26 | `apps/mobile/lib/shared/attachment/entities/share_reference.dart:31` | `ShareTargetType.content` displayName = `'Post'` | **legacy naming residue** |
| 27 | `apps/mobile/lib/domains/social/content/presentation/screens/content_detail_screen.dart:249,282-283` | `'Posted on ...'` copy, doc "Request type / buyer location for requests" | **legacy naming residue** |
| 28 | `apps/mobile/lib/domains/social/content/data/dto/content_dto.dart:7,85,238` | Comments: "Request fulfillment", "// Request DTOs", "// Request-specific fields" | **documentation residue** |

### 4.10 Mobile — Chat (protected)

| # | File | Reference | Classification |
|---|---|---|---|
| 29 | `apps/mobile/lib/domains/chat/chat/data/repositories/chat_repository_impl.dart:516` | `'Content shares require content_type=post or content_type=request for chat'` | **legacy Content classification** — chat attachment |
| 30 | `apps/mobile/lib/domains/chat/chat/data/repositories/chat_repository_impl.dart:766-779` | `_readChatContentType` reads `content_type` from attachment | **legacy Content classification** — chat attachment |
| 31 | `apps/mobile/lib/domains/chat/chat/data/dto/message_dto.dart:289` | `final String contentType;` | **legacy Content classification** — chat message attachment |

### 4.11 Mobile — Feed UX Bug (P2)

| # | File | Reference | Classification |
|---|---|---|---|
| 32 | `apps/mobile/lib/features/home/presentation/providers/feed/feed_notifier.dart:80-87` + `home_screen.dart:145` | Refresh failure sets `errorMessage` but preserves `items` — however, `home_screen.dart` checks `errorMessage != null` FIRST and renders full-screen error, **hiding the last-good list** | **UX defect** — pull-to-refresh nukes visible feed on failure |

### 4.12 Legacy — But Canonical (NO ACTION NEEDED)

| # | File | Reference | Why Canonical |
|---|---|---|---|
| FeedMedia.Type (backend) | `feed_item.go:37` | `Type string` — "image"/"video" | Media type, not content type |
| MediaType enum (mobile) | `content.dart:58` | `enum MediaType { image, video }` | Media type, not content type |
| MediaDto.type (mobile) | `content_dto.dart:414` | `final String type;` — "image"/"video" | Media type, not content type |
| FeedMediaDto.type (mobile) | `feed_dto.dart:300` | `final String type;` — "image"/"video" | Media type, not content type |
| FeedItemType enum (mobile) | `feed_item.dart:11-16` | `enum FeedItemType { content, promotedListing, ... }` | Feed card type, not Post/Request |
| FeedResponseDto.fromJson type check | `feed_dto.dart:77` | `item['type'] as String?` — checks for "promoted_" prefix | Promoted item detection |
| ContentTypeVisibilityHeader | `content_type_visibility_header.dart` | Readability-only wrapping of visibility dropdown | No Post/Request, just visibility |
| comment.Type / CommentType | `comment.go:11` | `CommentType` enum for comments | Comments table has its own type (normal/list_reference), canonical |
| notification worker | `notification_worker_social.go:124` | `SELECT author_id FROM contents WHERE id = $1` | Correct — only selects author_id |

---

## 5. Canonical Capability Inventory

### Capabilities formerly split between Post and Request

| Capability | Current Status |
|---|---|
| Social publishing (create caption+media) | ✅ Unified — `CreateContent` with no type |
| Feed display (home timeline) | ❌ Broken — 500 due to phantom `type` in SELECT |
| Content detail | ✅ Unified — `GetContentVisibleToViewer` |
| Profile content listing | ✅ Unified — `ListByAuthor` |
| Repost/Share | ✅ Unified — via `ShareReference` with `targetType: "content"` |
| Moderation (hide/delete) | ✅ Unified — `SoftDeleteForModeration` / `RestoreFromModeration` |
| Comments | ✅ Canonical — `targetType: "content"` |
| Likes | ✅ Canonical — via `content_likes` table |
| Chat attachment to content | ❌ Broken — queries `contents.type = $2` with "post"/"request" |
| Notifications for content events | ✅ Canonical — uses `SELECT author_id FROM contents` |

---

## 6. Proposed Removal Manifest

### 6.1 Backend

**Immediate (Feed fix — in scope):**
- `internal/social/feed/infrastructure/repository/feed_repository_impl.go:237` — Remove `type,` from SELECT list (line becomes `id, author_id, status,`)
- `internal/social/feed/infrastructure/repository/feed_repository_impl.go:276-297` — Re-verify rows.Scan aligns with corrected SELECT (should be 20-20 after fix)

**Test cleanup (in scope):**
- `internal/social/feed/infrastructure/repository/feed_repository_test.go` — Remove `type` from all INSERT statements and values
- `internal/social/feed/infrastructure/repository/feed_follow_first_bootstrap_test.go` — Remove `type` from INSERT statements
- `internal/worker/notification_worker_social_test.go:1479-1483` — Fix mock to return 1 value (author_id only), matching production query
- `internal/social/content/delivery/http/comment_list_integration_test.go:112,122` — Remove `contententity.Type` parameter, update call to `CreateContent`

**Contract tests (in scope):**
- `cmd/core_server/content_universal_contract_test.go:172-187` — Replace no-op stubs with actual assertions:
  - Verify migration 000001 SQL doesn't contain `content_type_enum` or `contents.type`
  - Verify feed_repository_impl.go SELECT doesn't contain `type` or `_unused`

**Protected but noted:**
- `internal/interaction/chat/application/chat_service.go:2292-2313` — Remove `AND type = $2` from query, remove `contentType string` parameter from `validateContentReferenceExists`
- `internal/interaction/chat/application/chat_service.go:1889-1902` — Replace "post"/"request" switch with direct content validation (no type distinction)
- `internal/interaction/chat/repository/chat_repository.go:185-189` — Remove `ErrAttachmentPostNotFound` / `ErrAttachmentRequestNotFound`, unify to `ErrAttachmentContentNotFound`

### 6.2 Mobile

**DTO (in scope):**
- `apps/mobile/lib/features/home/data/dto/feed_dto.dart:130` — Remove `type` field from `FeedItemDto`. The discriminator at `FeedResponseDto.fromJson:77` already handles null `type` correctly (`?? ''`) for the organic/promoted split.
- Regenerate `feed_dto.g.dart` after change.

**Mapper (in scope):**
- `apps/mobile/lib/features/home/data/mappers/feed_mapper.dart:41-49` — Delete `_mapFeedItemType` method. At the call site in `FeedItemMapper.toFeedItem()`, use `FeedItemType.content` directly.

**Documentation (in scope):**
- `apps/mobile/lib/features/home/data/remote/feed_api_datasource.dart:25` — Update comment to remove "posts, requests"

**Protected but noted:**
- `apps/mobile/lib/domains/chat/chat/data/repositories/chat_repository_impl.dart` — Remove `content_type=post/request` validation
- `apps/mobile/lib/domains/chat/chat/data/dto/message_dto.dart:289` — Remove `contentType` field or repurpose

### 6.3 Tests

- All feed repository integration tests need `type` column removed from INSERTs
- Feed bootstrap tests need `type` column removed from INSERTs
- Notification worker social test mock needs to match production query (1 value, not 2)
- Comment list integration test needs `contententity.Type` parameter removed
- Anti-resurrection contract tests need actual assertions instead of comments

### 6.4 No Action Needed

- **No database migration** — schema is already canonical
- **No enum changes** — no Post/Request enum exists
- **No seed changes** — seed queries don't use `type`
- **No generated code changes** (except `feed_dto.g.dart` after DTO change)

---

## 7. Proposed Executable Proof

### Existing tests to verify post-fix

```bash
# Backend unit tests (no DB needed)
cd backend && go test ./internal/social/feed/... -count=1

# Integration tests (require test DB)
cd backend && go test -tags=integration ./internal/social/feed/infrastructure/repository/... -count=1

# Anti-resurrection contract tests
cd backend && go test ./cmd/core_server/ -run TestAntiResurrection -count=1 -v

# Full content domain tests
cd backend && go test ./internal/social/content/... -count=1
```

### Tests to add

1. **Feed SQL contract test** — Parse `feed_repository_impl.go` SELECT statement, verify it doesn't contain `type` as a bare column name in the column list
2. **Feed response shape test** — Hit the actual feed endpoint, verify response JSON doesn't contain `type` key in items
3. **Strict anti-resurrection** — Grep entire codebase for `contents.type`, `content_type_enum`, `"post"`/`"request"` as content type strings in SQL queries (excluding comments table, media type)

### Manual verification

```bash
# After fix: curl feed endpoint
curl -s http://localhost:8080/api/v1/feed \
  -H "Authorization: Bearer <valid-token>" \
  | jq '.data[0] | keys'  # Should NOT contain "type"

# Verify migration doesn't contain forbidden types
grep -r "contents\.type\|content_type_enum\|listing_origin_enum" backend/migrations/000001_canonical_schema.up.sql
# Expected: no output
```

### Mobile build verification

```bash
cd apps/mobile && flutter analyze lib/features/home/
cd apps/mobile && flutter test test/features/home/
```

---

## 8. Protected Paths

### Chat domain (protected — NOT in scope)

The chat attachment validation path (`chat_service.go:2302`) also queries `contents.type` and will 500 on any chat message that attaches to content. This is a **P0** defect but is in the **Chat protected path**.

**Evidence of impact**: `chat_service.go:1889-1902` — when a chat message references `targetType: "post"` or `"request"`, `validateContentReferenceExists` executes `SELECT id FROM contents WHERE id = $1 AND type = $2 AND deleted_at IS NULL` with `$2 = "post"` or `$2 = "request"`. This fails with the same SQLSTATE 42703.

**Mobile counterpart**: `chat_repository_impl.dart:516` still requires `content_type=post or content_type=request` — the mobile chat attachment code also enforces the old classification.

**Recommendation**: Fix together with Content scope changes since it's the same root cause, but flag for explicit approval before touching chat code.

### Other protected paths — verified clean

- **Auth state machine**: No impact ✅
- **Email verification portal**: No impact ✅
- **Global router/redirect**: No impact ✅
- **GoRouter lifetime**: No impact ✅
- **Session lifecycle**: No impact ✅
- **Listing**: No impact ✅
- **Auction**: No impact ✅
- **Checkout**: No impact ✅
- **Order**: No impact ✅
- **Shipping authority**: No impact ✅
- **Seller subscription**: No impact ✅
- **Shared media implementation**: No impact ✅
- **Report/Case**: No impact ✅
- **Comments/Like**: No impact — `comments.type` is canonical (comment type, not content type) ✅

---

## 9. Out-of-Scope Findings

| # | Severity | Finding | Domain |
|---|---|---|---|
| 1 | **P0** | `chat_service.go:2302` queries `contents.type` — 500 on content-attached chat messages | Chat (protected) |
| 2 | **P0** | Mobile `chat_repository_impl.dart` requires `content_type=post/request` | Chat (protected) |
| 3 | **P3** | `content_type_visibility_header.dart` filename misleading — contains only visibility, not content type | Content (cosmetic) |
| 4 | **P3** | Refresh failure in `feed_notifier.dart` doesn't set error message — user may see stale data without indication | Feed (UX) |

---

## 10. Git Working Tree Status

The working tree has extensive uncommitted changes (300+ files modified). Key content-relevant changes:

**Deleted files (in working tree):**
- `apps/mobile/lib/core/src/interfaces/services/i_authentication_service.dart`
- `apps/mobile/lib/core/src/interfaces/services/i_presence_service.dart`
- `apps/mobile/lib/core/src/services/app_lifecycle_observer.dart`
- `apps/mobile/lib/core/src/services/app_presence_service_api.dart`
- `apps/mobile/lib/domains/commerce/catalog/auction/presentation/screens/auction_list_screen.dart`
- Various other mobile files

**Modified files relevant to content:**
- `backend/internal/social/feed/infrastructure/repository/feed_repository_impl.go` — Uncommitted mid-refactor: CTE + GROUP BY already canonical, outer SELECT still has `type`
- `backend/internal/social/feed/entity/feed_item.go` — `Type` field removed from `FeedItem`
- Various mobile files with unrelated changes

**Branch**: `main`, clean baseline commit `41aa4ee`

---

## Parent Commit

`41aa4ee fix(seller): guard listing creation and renewal flow`

---

### 6.5 Mobile — Share Domain

**Immediate (in scope):**
- `apps/mobile/lib/domains/social/share/domain/entities/share_target.dart:100` — Remove `request` from `ExternalShareType` enum (dead value); rename `post` → `content`
- `apps/mobile/lib/domains/social/share/presentation/widgets/share_preview_card.dart` — Remove `request` branches (115-118, 375-403), simplify to single content type
- `apps/mobile/lib/domains/social/share/data/datasources/share_api_datasource.dart:72` — Remove `'type': 'post'` from API payload; remove/update `@Deprecated createShareReferencePost`
- `apps/mobile/lib/domains/social/share/data/dto/share_dto.dart:102-107,110-123` — Remove `request` branches, remove `orElse: ExternalShareType.post` default
- `apps/mobile/lib/domains/social/share/data/repositories/share_repository_api.dart:91` — Remove `ExternalShareType.post` gate — repost for all content

### 6.6 Mobile — Search

- `apps/mobile/lib/features/search/search/data/dto/search_dto.dart:274` — Change `?? 'post'` to empty string or remove type fallback entirely

### 6.7 Mobile — Dead Code Removal

- `apps/mobile/lib/shared/services/media_upload_service.dart` — Delete entire file (zero call sites)
- `apps/mobile/lib/domains/system/analytics/data/repositories/firebase_analytics_repository_impl.dart:111-130` — Delete `trackEngagement` method
- `apps/mobile/lib/core/src/interfaces/services/i_content_moderation_service.dart:62` — Delete `ContentType` enum or entire interface (no implementation)

### 6.8 Mobile — Cosmetic

- Rename UI labels from 'Post' → 'Content' in search: `search_results_screen.dart`, `global_search_bar.dart`, `search_result_type_helper.dart`, `search_mapper.dart`, `search_repository_impl.dart`
- `apps/mobile/lib/shared/widgets/popup_more_options_button.dart:7` — Rename `PopupMoreOptionsContentType.post` → `.content`
- `apps/mobile/lib/shared/attachment/entities/share_reference.dart:31` — Change `ShareTargetType.content` displayName from 'Post' → 'Content'
- `apps/mobile/lib/domains/social/content/data/dto/content_dto.dart:7,85,238` — Update stale comments mentioning "Request"
- `apps/mobile/lib/domains/social/content/presentation/screens/content_detail_screen.dart:249,282-283` — Update "Posted on" and "Request type" comments

### 6.9 Purge Tool Hardening

The existing `tool/check_universal_content_purge.dart` has blind spots:
1. `json['type'] as String? ?? 'post'` — the `as String?` cast and `??` operator evade the regex
2. Single-quoted `'type': 'post'` — only matches double-quoted `"type": "post"`
3. Bare `case 'post':` / `case 'request':` switches with no string context

These should be hardened to catch the patterns found in this audit.

---

## 7. Purge Tool Blind Spots (Existing Tool Gap)

The existing purge tool at `tool/check_universal_content_purge.dart` misses these live violations:

| Pattern | Location | Why Missed |
|---|---|---|
| `json['type'] as String? ?? 'post'` | `search_dto.dart:274` | `as String?` cast + `??` operator evades type coercion regex |
| `'type': 'post'` (single-quoted) | `share_api_datasource.dart:72` | Tool regex only matches `"type": "post"` (double-quoted) |
| `case 'post': case 'request': return FeedItemType.content` | `feed_mapper.dart:41-49` | Bare switch cases with no string context |

All three are active code, not dead branches. Recommendation: harden purge tool regexes to catch these patterns before implementation phase.

---

## Summary Counts

| Category | Count |
|---|---|
| P0 production breakages found | 2 (feed + chat) |
| P1 mobile DTO crash risk | 1 (FeedItemDto.type → TypeError) |
| P1 mobile active legacy emission | 2 (`'type': 'post'` + search `?? 'post'`) |
| P2 feed UX bug | 1 (refresh hides last-good items) |
| P2 purge tool blind spots | 3 patterns undetected |
| Test files with phantom `type` column | 4 |
| Test fixture INSERTs with `type` | ~18 |
| Mobile files with legacy references (in-scope) | 15 |
| Dead code files/functions | 5 |
| Cosmetic/naming residue | 5 |
| No-op anti-resurrection tests | 2 |
| Protected paths affected | 1 (chat, P0) |
| Database changes needed | **0** (schema is canonical) |
