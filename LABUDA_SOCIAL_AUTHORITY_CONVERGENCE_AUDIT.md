# LABUDA — SOCIAL AUTHORITY CONVERGENCE AUDIT

**Status:** READ-ONLY — NO IMPLEMENTATION, NO CLEANUP, NO MODIFICATION PERFORMED
**Scope (12):** Content, Feed, Comments, Likes, Follow, Block, Mute, Mentions, Share Reference, Social Notifications/Outbox, Social Search/Discovery, Social Realtime/WebSocket
**Evidence basis:** live source (`backend/`, `apps/mobile/`, `backend/migrations/`), route registrations (`routes_core.go`), DI wiring (`serverboot/dependencies.go`), outbox/notification/realtime workers, mobile datasources/repositories/providers. GitHub is treated as backup only (not consulted as an implementation baseline). No restore/rollback/checkout/cherry-pick.

**Authoritative-source doctrine:** the repository/filesystem and actual DB schema are the implementation truth. Every conclusion below follows one business fact → schema authority → writer → service → API → event → consumer → mobile/admin consumer → lifecycle → deletion/expiry/moderation → runtime-proof boundary.

**Evidence discipline:** static proof = code/schema/route/DI; runtime proof = observable production-path wiring (an enabled worker draining a real outbox event into a real consumer; a route→handler→repo chain that exists in the live wiring). "Test passed", "code exists", and "no caller" are never used as authority proof. Commerce/transaction internals are NOT audited; Social↔commerce touch points are marked **PARALLEL / PROVENANCE UNKNOWN**.

---

## 1. EXECUTIVE VERDICT

The Social domain has **one coherent, single-authority core** (CONTENT / FEED / COMMENTS-create-list-delete / FOLLOW / BLOCK / MUTE / SHARE-occurrences / NOTIFICATION-outbox for 8 events) that is statically proven and mostly runtime-unproven only for the lack of a live-DB observation in this read-only session.

Three material authority breaks sit outside that healthy core, and one domain (MENTIONS) is **confirmed broken scaffold rather than implemented**:

1. **COMMENTS — mobile comment count is a duplicate/absent authority.** Backend canonical count = `CountTopLevelCommentsByContent` (comments, excl. deleted + replies). Mobile `getCommentCount` calls the **legacy `GET /social/comments` route that the backend does not register** and reads a `total` of a different (page, includes replies) semantic. The two counts can and do disagree, and the mobile path is currently dangling against the server contract. **P0.**
2. **LIKES — `content_likes` has two independent writer implementations** (`LikeRepositoryImpl` and `TargetLikeRepository`). They converge on the same physical table (no value divergence today) but are administratively independent, neither authoritative over the other. **P1.**
3. **SEARCH — content search skips the `visibility` gate and (in its default Shadow mode) the block filter** (`search_repository_impl.go:316`, no `visibility`/`user_blocks` predicate), while feed/content-detail/comments all enforce both. Same content can be visible in `/search/content` but excluded from `/feed`. **P1.**
4. **MENTIONS — confirmed broken residue.** Orphan schema `content_mentioned_users`, dead `MentionedUserIDs` DTO field, five non-compiling tests referencing non-existent production symbols, and a client-only mention UI/notifier with no server persistence or event. **P1.**
5. **MOBILE dead endpoints — `/content/trending`, `/content/search`, `/content/contents`, `/social/comments` are called by mobile but do not exist on the backend.** **P1** for the ones actively surfaced in navigation/cleanup (trending/search via `ContentRepositoryImpl`), **P2** for dormant ones.
6. **Realtime:** Social has **no production WebSocket/SSE/push** path at all — feed is pull/cursor, comments poll once/5s, likes poll once/10s, notifications are in-app DB rows read by REST poll, and FCM push is gated **off** for Social types (only `chat_message` pushes). The single websocket (`/api/v1/ws`) is chat-only and drops every Social event. The presence websocket is **unwired test-only residue**. This is a capability gap, cleanly classified **NOT PROVEN AS REQUIRED** (no current Social consumer depends on it) — not a defect.

Healthy, single-authority, and **PROVEN**: `contents` (single writer), feed-as-direct-contents-query (no projection table), comment cursor create/list/delete, follow/block/mute (`SocialRepositoryImpl` single writer), share occurrences (DB-immutable), 8 notification events end-to-end.

---

## 2. MASTER AUTHORITY MAP

BUSINESS FACT → CANONICAL TABLE/ENTITY → CANONICAL WRITER → CANONICAL SERVICE → CANONICAL API → CANONICAL EVENT → CONSUMERS → CACHE/DERIVED.

| Business fact | Canonical table/entity | Canonical writer | Canonical service | Canonical API (backend) | Canonical event | Canonical consumers | Cache / derived |
|---|---|---|---|---|---|---|---|
| Content post | `contents` | `ContentRepositoryImpl` (only via `ContentService`) | `ContentService` | `POST/PUT/DELETE /contents`; `GET /contents/:id`, `GET /users/:id/contents` | **none (create/delete)** | feed/search/detail/list readers (SQL) | `search_vector` (generated col), `content_hashtags`, `content_resource_occurrences` (share provenance) |
| Repost / share | `content_resource_occurrences` (immutable) | `ContentRepositoryImpl.CreateResourceOccurrence` | `ContentService` (+ internal share authority) | part of `POST /contents`, `POST /contents/:id/repost` | none | projection resolver → feed/search/detail/mobile | `content_resource_projection` (live derived envelope) |
| Feed | (none; reads `contents`) | n/a | `FeedService` | `GET /feed` (cursor) | none | mobile home feed | none (direct query) |
| Comment | `comments` (+ `comment_commerce_references`) | `CommentRepositoryImpl` | `CommentService` | `POST/GET /contents/:id/comments`, `DELETE /comments/:id`, `POST /comments/reference` | `comment.created`, `comment.reply`, `seller.response` | `NotificationEventHandler.handleCommentCreated/Reply/SellerResponse` | count = live `COUNT(*)` |
| Content like | `content_likes` | **DUPLICATE**: `LikeRepositoryImpl` + `TargetLikeRepository` | `LikeService` | `POST /likes/toggle`, `GET /likes/stats` | `content.liked` (like only) | `NotificationEventHandler.handleContentLiked` | count = live `COUNT(*)` |
| Comment like | `comment_likes` | `TargetLikeRepository` | `CommentLikeService` | `POST /likes/toggle` (comment), `GET /likes/stats` (comment) | **none** | none | `TargetLikeRepository.CountLikes` |
| Follow | `user_follows` | `SocialRepositoryImpl` | `SocialService` | `POST/DELETE /users/:id/follow`; `GET /users/:id/followers|following`; `GET /follows/status/:id` | `user.followed`, `user.unfollowed` | `handleUserFollowed`, cleanup | live `COUNT(*)` follower/following |
| Block | `user_blocks` | `SocialRepositoryImpl` | `SocialService` | `POST/DELETE /users/:id/block`; `GET /blocks` | `user.blocked`, `user.unblocked` | `handleUserBlocked`, cleanup | `blockcheck` API (reader facade) |
| Mute | `user_mutes` | `SocialRepositoryImpl` | `SocialService` | `POST/DELETE /users/:id/mute`; `GET /mutes` | **none** | none | feed/chat mute filter (reader) |
| Mention | `content_mentioned_users` (**orphan**) | **none (residue)** | **none** | **none** | **none** | **none** | n/a |
| Social notification | `notifications` | `NotificationServiceInserter` (outbox worker) | `NotificationEventHandler` | `GET /notifications`; `GET /notifications/unread-count`; read/delete | social producers (8) | mobile notification list/unread | unread = live `COUNT(*)` |
| Search content | `contents` + `search_vector` + `content_hashtags` | `ContentService` (via `InsertTags`) | `SearchService` | `GET /search/content`, `GET /search/users` | none | mobile `SearchApiService` → `/search/*` | none (no cache); `search_history` user log |
| Search history | `search_history` | `SearchService.AddSearchHistory` | `SearchService` | `POST/GET/DELETE /search/history` | none | mobile search history | capped-20 user log |
| Realtime | n/a | n/a | n/a | `GET /ws` (**chat only**) | chat only | chat WS | none for Social |

Anything outside each row's chain is classified in the matrices below.

---

## 3. WRITER AUTHORITY MATRIX

For every durable Social table, every writer. Class: CANONICAL WRITER / DERIVED WRITER / DEV ONLY / TEST ONLY / LEGACY / ZOMBIE / DUPLICATE WRITER / UNKNOWN.

| Table | Writer (file:line) | Class |
|---|---|---|
| `contents` | `ContentRepositoryImpl.Create/Update` (`content_repository_impl.go:24,230`) via `ContentService` only (create, occurrence-create, delete, hide, unhide, moderation, restore, caption/visibility, repost/share) | CANONICAL WRITER |
| `contents` | `cmd/seed/main.go:470` direct INSERT | DEV ONLY |
| `content_media` | `ContentRepositoryImpl.CreateMedia` (`content_repository_impl.go:67`) | CANONICAL WRITER |
| `content_hashtags` | `ContentRepositoryImpl.InsertTags` (`content_repository_impl.go:585`) | DERIVED WRITER (tag index, idempotent) |
| `content_resource_occurrences` | `ContentRepositoryImpl.CreateResourceOccurrence` (`content_repository_impl.go:467`) via `ContentService` + internal share authority | CANONICAL WRITER (DB-immutable; UPDATE blocked by trigger) |
| `comments` | `CommentRepositoryImpl.Create`, reference-comment path (via `CommentService`) | CANONICAL WRITER |
| `comments` (update/soft-delete) | `CommentRepositoryImpl.Update/SoftDelete/Restore` via `CommentService` (delete, moderation) | CANONICAL WRITER |
| `comment_commerce_references` | `CommentService.AddCommerceReferenceComment`; `CommentRepositoryImpl` insert | CANONICAL WRITER |
| `content_likes` | `LikeRepositoryImpl.InsertLike/DeleteLike` (`like_repository_impl.go:33,58`) | CANONICAL WRITER |
| `content_likes` | `TargetLikeRepository.InsertLike/DeleteLike` (`like_repository.go:24-69`, content branch) | **DUPLICATE WRITER** (converges on same table) |
| `comment_likes` | `TargetLikeRepository` (comment branch) | CANONICAL WRITER (single) |
| `user_follows` | `SocialRepositoryImpl.InsertFollow/DeleteFollow/DeleteFollowBothDirections` (`social_repository_impl.go:79,103,127`) | CANONICAL WRITER |
| `user_blocks` | `SocialRepositoryImpl.InsertBlock/DeleteBlock` (`:290,314`) | CANONICAL WRITER |
| `user_mutes` | `SocialRepositoryImpl.InsertMute/DeleteMute` (`:403,427`) | CANONICAL WRITER |
| `content_mentioned_users` | **none in production** (only test files reference intended writers) | ZOMBIE (no writer) |
| `notifications` | `NotificationEventHandler` → `NotificationServiceInserter.InsertNotification` (`notification_worker.go:556`) | CANONICAL WRITER |
| `outbox` | `OutboxRepository.InsertEvent/InsertTx` (`outbox_repository.go:89,150`) from social producers | CANONICAL WRITER (transactional with domain write) |
| `search_history` | `SearchService.RecordSearch` → `AddSearchHistory` (`search_repository_impl.go:28`) | CANONICAL WRITER (user activity log) |
| `search_results` | **none** | ZOMBIE (dead table) |
| `user_profiles.followers_count/following_count` | `auth_handler.go:363` writes hardcoded `0` at account create only; never updated | **LEGACY / DEAD (schema residue)** — every real read is a live `COUNT(*)` (see §5/§6) |

---

## 4. READER AUTHORITY MATRIX

Every production reader of each durable table, and whether it reads canonical, derived, or a non-canonical source.

| Table / fact | Reader(s) (file:line) | Reads canonical? |
|---|---|---|
| `contents` | feed (`feed_repository_impl.go:150`), content detail (`content_handler.go`), user list (`content_repository_impl.go:289`), search (`search_repository_impl.go:311`), moderation preview (`moderation_handler.go:789`), notification workers (author_id only) | YES (canonical) |
| `contents` visibility/block | feed SQL (authoritative), detail viewercontext + evaluator (authoritative), comments parent gate (`comment_handler.go:318`), **search content: NO visibility predicate (`search_repository_impl.go:316`)**, **search content: NO block predicate in default Shadow mode** | **search = reads canonical row but DOES NOT enforce visibility/block (authority defect)** |
| `content_resource_occurrences` | projection resolver (`content_resource_projection_resolver.go:298`), feed join, search join, chat projection | YES (canonical, derived-to-LIVE) |
| `comments` | cursor `ListByTarget` (`comment_repository_impl.go:91`), count `CountTopLevelCommentsByContent` (`comment_repository_addition.go:180`), moderation preview (`moderation_handler.go:830`), appeals `GetByID` | YES |
| `content_likes` | `LikeRepositoryImpl.CountLikes/ExistsLike`, `TargetLikeRepository` (content), `/likes/stats` | YES (both read same table) |
| `comment_likes` | `TargetLikeRepository.CountLikes`, `CommentLikeService` | YES |
| `user_follows` | `SocialRepositoryImpl` list/exists; feed SQL (`feed_repository_impl.go:151`); profile count (`user_repository_impl.go:712,722`); follow cards (`follow_response.go:68`); admin (`admin_repository_impl.go:164`); `chat_resource_authorizer_adapter.go:163` (uses `followed_id` — **column is `following_id`, never matches**) | YES except the chat adapter (mismatch) |
| `user_blocks` | `SocialRepositoryImpl`; `blockcheck` (`pkg/blockcheck/blockcheck.go:31,65`); content/comment/feed/search viewercontexts | YES |
| `user_mutes` | `SocialRepositoryImpl`; feed/chat mute filters | YES |
| `content_mentioned_users` | **none in production** | n/a (ZOMBIE) |
| `notifications` | `NotificationRepository` (list/unread/read/delete); `CountUnread` (`notification_repository.go:159`) | YES |
| `search_history` | `SearchService.GetSearchHistory` (`search_repository_impl.go:37`) | YES |
| `search_results` | **none** | n/a (ZOMBIE) |
| `user_profiles.followers_count/following_count` | **never read** (production computes live; a test asserts the columns are NOT selected) | LEGACY / DEAD |

---

## 5. CALCULATION AUTHORITY MATRIX

| Fact | Canonical calculation | Other calculations | Difference | Same fact / two calcs? | Class |
|---|---|---|---|---|---|
| Comment count | `CountTopLevelCommentsByContent` (`comment_repository_addition.go:180`, excl. deleted + replies) | mobile `getCommentCount` → `GET /social/comments` `total` (page incl. replies) on an **unregistered route** | semantic mismatch (top-level vs page-total) + dangling endpoint | **YES — can disagree** | **DUPLICATE AUTHORITY** (mobile path) |
| Content like count | `LikeRepositoryImpl.CountLikes` (`like_repository_impl.go:109`) | `TargetLikeRepository.CountLikes` via `/likes/stats` (`like_handler.go:214`) | both `COUNT(*) content_likes` — identical | NO divergence (same table) | DUPLICATE ENTRY POINT, consistent |
| Comment like count | `CommentLikeService`/`TargetLikeRepository` on `comment_likes` | none | — | single | DERIVED |
| Follower/following count | live `COUNT(*) user_follows` (`user_repository_impl.go:710-728`) | follow cards, admin, mobile `/users/:id` — all live `COUNT(*) user_follows` | identical | 4 sites, all same table | DUPLICATE ENTRY POINT, consistent; dead `user_profiles.*_count` columns contradict the schema (LEGACY) |
| Unread notification count | `CountUnread` (`notification_repository.go:159`) on `notifications` `is_read=false` | none | single | single | DERIVED |
| Visibility | feed SQL (authoritative), detail preview gate + evaluator (authoritative), comments parent gate, search (**missing**) | search does not gate `visibility` | a private/followers-only content can appear in search but not feed | **YES — inconsistent** | **DUPLICATE AUTHORITY / INCONSISTENT** |
| Block filter | feed SQL, detail evaluator, comments `filterBlockedComments`, user search handler `blockcheck` | content search: **none in default Shadow mode** | blocked author's content can appear in content search | **YES — inconsistent** | **DUPLICATE AUTHORITY / INCONSISTENT** |
| Feed eligibility | `feed_repository_impl.go:156-214` (status/hidden/deleted/visibility/block/repost-governance) | search `search_repository_impl.go:316-362` (status/hidden/deleted/repost-governance, **no visibility/block**) | divergent WHERE | **YES — same content, different eligibility** | DUPLICATE AUTHORITY |
| Latest content preview | `ContentResourceProjection` live envelope (`content_resource_projection.go:75`) resolved at request time | — | derived, not stored | single | DERIVED |
| Search ranking | repository ts_rank on `contents.search_vector` (relevance/recency) | — | — | single | DERIVED |

---

## 6. API / MOBILE CONTRACT MATRIX

Trace route→handler→service→repo→response→mobile→provider for every Social API. Class each: LIVE / DUPLICATE / DEPRECATED-REACHABLE / **DANGLING (mobile calls route backend does not serve)** / ORPHANED (backend route, no consumer).

| Endpoint | Backend route | Backend handler | Mobile caller | Contract status |
|---|---|---|---|---|
| Feed | `GET /feed` (`routes_core.go:1219`) | `FeedHandler.GetFeed` | `feed_api_datasource.dart:49` → home feed | LIVE |
| Content create/update/delete | `POST/PUT/DELETE /contents`, `POST /contents/:id/repost` (`:1237-1243`) | `ContentHandler` | `content_api_datasource.dart` | LIVE |
| Content detail / user contents | `GET /contents/:id`, `GET /users/:id/contents` (`:161,164`) | `ContentHandler` | content detail / profile | LIVE |
| Comment list/create/delete/reference | `GET/POST /contents/:id/comments`, `DELETE /comments/:id`, `POST /:id/comments/reference` (`:1247-1256`) | `CommentHandler` | `comment_api_datasource.dart` | LIVE |
| Comment count | (backend COUNT on content-detail engagement) | estimator | `comment_repository_impl.dart:129` → **`GET /social/comments`** | **DANGLING (route absent)** |
| Legacy comment list | **none registered** | — | `comment_api_datasource.dart:97` `GET /social/comments` | **DANGLING** |
| Like toggle | `POST /likes/toggle` (`:1230`) | `LikeHandler.ToggleLike` | `like_api_datasource.dart:29` | LIVE |
| Like stats | `GET /likes/stats` (`:167` v1Browse) | `LikeHandler.GetLikeStats` | `like_api_datasource.dart:50` (10s poll) | LIVE |
| Follow/unfollow | `POST/DELETE /users/:id/follow` (`:1343`) | `FollowHandler` | `follow_api_datasource.dart:30,41` | LIVE |
| Followers/following/status | `GET /users/:id/followers|following`, `GET /follows/status/:id` (`:1359,1363`) | `FollowHandler` | `follow_api_datasource.dart:59,83,122` | LIVE |
| Block/unblock | `POST/DELETE /users/:id/block` (`:1349`) | `FollowHandler` | `follow_api_datasource.dart:139,150` | LIVE |
| Blocked list | `GET /blocks` (`:1366`) | `FollowHandler` | `follow_api_datasource.dart:165` | LIVE |
| Mute/unmute | `POST/DELETE /users/:id/mute` (`:1354`) | `FollowHandler` | `follow_api_datasource.dart:186,197` | LIVE |
| Muted list | `GET /mutes` (`:1367`) | `FollowHandler` | `follow_api_datasource.dart:239` | LIVE |
| Notifications list/unread/read/delete | `GET /notifications`, `GET /unread-count`, `POST /read...`, `DELETE /:id` (`:1263-1279`) | `NotificationHandler` | `notification_api_datasource.dart` | LIVE |
| FCM token | `POST/DELETE /notifications/fcm-token` (`:1282,1285`) | `FCMTokenHandler` | `notification_api_datasource.dart:109,119` | LIVE |
| Search content | `GET /search/content` (`:156` v1Browse) | `SearchHandler.SearchContent` | `SearchApiService` → content search | LIVE |
| Search users | `GET /search/users` (`:157` v1Browse) | `SearchHandler.SearchUsers` | `SearchApiService` → user search | LIVE |
| Search history | `POST/GET/DELETE /search/history` (`:392-397`) | `SearchHandler` | `SearchApiService` | LIVE |
| Content list (mobile) | **none registered** | — | `content_api_datasource.dart:57` `GET /content/contents` | **DANGLING** |
| Content search (mobile legacy) | **none registered** | — | `content_api_datasource.dart:146` `GET /content/search` | **DANGLING** |
| Trending (mobile) | **none registered** | — | `content_api_datasource.dart:119` `GET /content/trending` | **DANGLING** |
| Listings/auctions search | `GET /search/listings`, `GET /search/auctions` (`:147,153`) | commerce handlers | `SearchApiService` | **PARALLEL / PROVENANCE UNKNOWN** |

**DTO fields with no producer / fields silently ignored / never consumed:**
- **`MentionedUserIDs`** (`content_handler.go:211`) — decoded from request, **never passed to any service** (silently ignored).
- **`AllowComments`** (`content_handler.go:208`) — backend DTO only; no schema column; mobile always sends `true`; never enforced (silently ignored producer).
- **`user_profiles.followers_count/following_count`** — written as 0 at create, never read (dead columns; the live wire value is computed).
- **`search_results`** — full table with FK/index, zero readers/writers.
- Content search on mobile via `ContentApiDatasource` (`/content/search`, `/content/contents`, `/content/trending`) uses **legacy/nonexistent routes**; the *legitimate* search path is `SearchApiService` → `/search/content`.

---

## 7. EVENT / OUTBOX MATRIX

Producer → outbox → worker → consumer → durable effect for every Social event.

| Event | Producer (file:line) | Outbox write | Consumer (handler) | Durable effect | Required? |
|---|---|---|---|---|---|
| `comment.created` | `comment_service.go:272,288` | tx `InsertTx` | `handleCommentCreated` (`notification_worker_social.go:99`) | `notifications` row (in-app) | PROVEN REQUIRED |
| `comment.reply` | `comment_service.go:256` | tx | `handleCommentReply` (`:158`) | `notifications` row | PROVEN REQUIRED |
| `seller.response` | `comment_service.go:474` | tx | `handleSellerResponse` (`:219`) | `notifications` row | PROVEN REQUIRED |
| `content.liked` | `like_service.go:120,207` | tx | `handleContentLiked` (`:50`) | `notifications` row | PROVEN REQUIRED |
| `content.unliked` | **none** | — | — | — | NOT PROVEN AS REQUIRED (private removal; no consumer workflow) |
| `user.followed` | `social_service.go:108` | tx | `handleUserFollowed` (`:14`) | `notifications` row | PROVEN REQUIRED |
| `user.unfollowed` | `social_service.go:146` | tx | `handleUserUnfollowed` (`:414`) | cleanup | PROVEN REQUIRED (cleanup) |
| `user.blocked` | `social_service.go:198` | tx | `handleUserBlocked` (`:367`) | cleanup | PROVEN REQUIRED (cleanup) |
| `user.unblocked` | `social_service.go:237` | tx | cleanup | cleanup | PROVEN REQUIRED (cleanup) |
| `mute`/`unmute` | **none** | — | — | — | NOT PROVEN AS REQUIRED (mute = silent suppression) |
| `content.mentioned` / `mention.created` | **none (constant absent)** | — | — | — | see §MENTIONS |
| `content.created` / `content.deleted` | **none (constant absent)** | — | — | — | NOT PROVEN AS REQUIRED (no Social consumer reads a content.created event today) |

**Event vs durable-state authority:** For the 8 implemented events, payload is an **invalidation/notification** — the durable truth stays in `notifications` after the worker inserts it; consumers treat the event as a trigger, not as a second source of the business fact. There is **no** event whose payload re-derives content/like/follow state. No event can become a second authority for any core fact; the worker dedupes via `ON CONFLICT` idempotency keys.

**Can an event be missed?** Yes, by design of the outbox worker (post-commit drain + `ON CONFLICT DO NOTHING`). But because events only trigger notifications (never state mutation), a missed event costs a notification, not state corruption. **No later REST/state reconciliation overrides a missed event for state** — reconciliation is unnecessary because the event never writes state.

---

## 8. CACHE AUTHORITY MATRIX

Audit every Social cache — source, owner, key, invalidation, isolation, lifecycle, staleness, rebuild. Class: CACHE or ACCIDENTAL SECOND AUTHORITY.

| Cache | Source | Owner | Key | Invalidation | Lifecycle | Account isolation | Stale? | Class |
|---|---|---|---|---|---|---|---|---|
| Mobile like count (10s poll) | `GET /likes/stats` → `content_likes` | `likeStatsProvider` (StreamProvider.family) | content id | re-poll every 10s | per-provider | per-viewer stream | up to 10s | **CACHE** (cannot diverge from server truth beyond 10s; never local-persisted) |
| Mobile comment list watch (5s poll) | `GET /contents/:id/comments` | `watchComments` stream | target id | re-poll every 5s | per-provider | viewer-authed | up to 5s | **CACHE** |
| Mobile follow stats optimistic ±1 | local `followersCount` in-memory | `follow_stats_provider.dart:80-96` | user id | invalidates server streams after action | session-only | per-user | transient until re-fetch | **CACHE** (in-memory heuristic; not disk; server re-fetch reconciles) |
| Mobile unread count | `GET /notifications/unread-count` poll | `unreadCountProvider` | recipient | re-fetch | session | per-recipient | instance | **CACHE** |
| Feed | none (pull/cursor REST) | n/a | n/a | n/a | n/a | per-viewer SQL | n/a | n/a — no feed cache |
| Search | none (no Redis/mem/DB-cache; shadow runner is telemetry-only, never affects response) | — | — | — | — | — | n/a | **no cache — no second-authority risk** |
| `user_presence` via Redis pub/sub | presence subscriber | — | — | — | — | — | — | **not wired into Social** (test-only; see §11) |

**No cache can become an independent business truth:** every cache above is in-memory/mobile-only, server-authored, and (except the 10s like poll and 5s comment poll, which are staleness windows, and the transient follow ±1, which reconciles on invalidate) does not store business state on the client. No disk persistence of any Social count or feed exists (`user_profiles.*_count` dead columns are schema residue, not a cache). **No ACCIDENTAL SECOND AUTHORITY found.**

---

## 9. ZOMBIE / LEGACY / RESIDUE MATRIX

Cleanup-readiness view. "No active caller" alone is NOT sufficient — each is classified only after proving the canonical replacement exists, business behavior is represented, and no writer/reader/event/schema/API depends on it.

| Path | Why it exists | Current writer | Current reader | Reachable? | Canonical? | Classification |
|---|---|---|---|---|---|---|
| `content_mentioned_users` table | migration 000043; feature never built into production | **none** | **none** | no | no (orphan schema) | **RESIDUE / ZOMBIE** (orphan authority) |
| `MentionedUserIDs` DTO field | decoded, never used | content handler (dead value) | none | no | no | RESIDUE |
| 5 mention test files (`content_mention_*`, `content_mentioned_notification*`) | encode unimplemented feature; reference non-existent symbols (`AddMentionedUserIDs`, `RemoveMentionedUserIDs`, `EventContentMentioned`, `handleContentMentioned`, `SetContentVisibilityChecker`, `ContentMentionedPayload`, 7-arg `NewContentService`) | none | none | cannot compile | no | ZOMBIE |
| mobile `listComments` → `GET /social/comments` (page-based) + `getCommentCount` | legacy route removed; datasource/repository retained | none (server) | mobile count provider | mobile-only, dangling | no | DEPRECATED BUT REACHABLE (mobile) → DANGLING |
| mobile `getTrendingContents` → `/content/trending` | feature never has a backend route | none | `trendingContentsFutureProvider` | mobile-only, dangling | no | **DANGLING / ZOMBIE call** |
| mobile `getContents` → `/content/contents` | legacy list path, no route | none | mobile content list/search | mobile-only, dangling | no | DANGLING |
| mobile `searchContents` → `/content/search` | legacy alias of `/search/content` | none | mobile content search | mobile-only, dangling | no | DANGLING |
| `allowComments` (backend + mobile DTO/entity) | decoration; no schema authority | none (ignored) | mobile settings passthrough | always `true` | no | RESIDUE |
| `comment_response.go` `Type` (re-derived) | column `comments.type` dropped (000031) | n/a (handler derives) | wire | yes | no | ACTIVE NON-CANONICAL (harmless additive) |
| `comment_response.go` `Reference`/`Listing` | commerce-reference preview | handler | commerce-reference comments | yes | yes (backed by `comment_commerce_references`) | CANONICAL ACTIVE |
| `user_profiles.followers_count/following_count` columns | denormalization attempt; never synced | auth create (0) | **never** (production live-counts; test asserts not selected) | no | no | LEGACY / DEAD (schema residue) |
| `search_results` table | older search design | **none** | **none** | no | no | ZOMBIE (dead table, full FK/index) |
| `contents.share_reference` column | legacy share blob; dropped (000041), rejected in bind | none | none | no | no | ZOMBIE (already removed) |
| mobile like 10s poll / comment 5s poll | "until WebSocket support" placeholder | n/a | n/a | yes | no | ACTIVE NON-CANONICAL (staleness windows; not a defect) |
| presence websocket (`presence_subscriber.go` + test) | aspirational realtime presence; never wired into `serverboot` | none | none (only tests) | no | no | RESIDUE (unwired code) |
| Backend content search `content_api_datasource.dart:57,119,146` | legacy mobile content datasource paths, no backend routes | none | mobile | mobile-only, dangling | no | DANGLING |

---

## 10. CONTRADICTION MATRIX

| Contradiction | Evidence A | Evidence B | Which is authoritative? | Resolution |
|---|---|---|---|---|
| Comment count path | backend canonical `CountTopLevelCommentsByContent` (excl. deleted+replies) on content-detail | mobile reads `GET /social/comments` `total` (page, incl. replies) on an unregistered route | backend canonical; mobile path invalid | **UNRESOLVED** — client must be re-pointed to a real endpoint; currently DANGLING |
| Search content visibility | feed/detail enforce `visibility` gate; comments enforce parent visibility | `/search/content` has NO `visibility` predicate (`search_repository_impl.go:316`) | feed/detail enforcement is the doctrine; search is non-conformant | **UNRESOLVED** — CONFLICT (private/followers-only leak potential) |
| Search content block | feed SQL + user-search handler enforce `user_blocks` | `/search/content` has NO block predicate in default Shadow evaluator mode | feed/handler enforcement is doctrine | **UNRESOLVED** — CONFLICT (blocked-author leak potential) |
| `user_profiles` counts | schema has `followers_count/following_count` (LEGACY) | every production read is a live `COUNT(*) user_follows`; the columns are never updated | live `COUNT(*)` | Internally consistent (columns dead); schema residue must not be treated as truth |
| Mentions authority | schema has `content_mentioned_users`; DTO advertises `MentionedUserIDs` | no production writer/reader/event/consumer; tests reference non-existent symbols | **nothing is authoritative** (feature absent) | **UNRESOLVED** — business decision required (implement or drop) |
| Realtime presence | mobile `app_presence_service_api.dart` claims "WebSocket for real-time status" | backend presence subscriber is test-only/unwired; no presence endpoints | backend (no presence WS) | mobile claim is aspirational residue; no runtime conflict |

---

## 11. STATE-PROOF MATRIX

| Claim | Static Proof | Schema Proof | State Proof | Runtime Proof | Verdict |
|---|---|---|---|---|---|
| `contents` single-writer, no competing state | PROVEN | PROVEN | PROVEN (soft-delete, no disappearance) | PARTIALLY PROVEN | **PROVEN** |
| Feed is a direct canonical-`contents` query, no projection table | PROVEN | PROVEN (no feed table in schema) | PROVEN | PARTIALLY PROVEN | **PROVEN** |
| Comment cursor create/list/delete is single-authority | PROVEN | PROVEN | PROVEN | PARTIALLY PROVEN | **PROVEN** |
| Mobile comment count uses a removed route | PROVEN | PROVEN (route absent) | NOT PROVEN (no live call observed) | NOT PROVEN | **CONTRADICTED (contract break)** |
| `content_likes` has one writer | CONTRADICTED (two: LikeRepositoryImpl + TargetLikeRepository) | PROVEN (each writes same table) | PROVEN (no value divergence, but two paths) | PARTIALLY PROVEN | **DUPLICATE AUTHORITY** |
| `comment_likes` single writer | PROVEN | PROVEN | PROVEN | PARTIALLY PROVEN | **PROVEN** |
| Visibility enforced on every Social read surface | CONTRADICTED (search skips it) | PROVEN (column exists) | NOT PROVEN (leak potential) | NOT PROVEN | **CONFLICT** |
| Block enforced on every Social read surface | CONTRADICTED (content search shadow-mode) | PROVEN | NOT PROVEN | NOT PROVEN | **CONFLICT** |
| Mentions schema is authoritative | CONTRADICTED | CONTRADICTED (orphan) | CONTRADICTED (no writer/reader) | NOT PROVEN | **CONTRADICTED (broken residue)** |
| Share authority is `content_resource_occurrences` (immutable) | PROVEN | PROVEN (immutability trigger) | PROVEN | PARTIALLY PROVEN | **PROVEN** |
| 8 social events producer→consumer wired | PROVEN | PROVEN (outbox) | PROVEN (notifications dedup) | PARTIALLY PROVEN | **PROVEN** |
| Social has a production realtime path | CONTRADICTED (no WS/SSE/push for Social) | PROVEN (only chat WS) | PROVEN (no Social WS) | PROVEN (chat-only dispatcher + FCM gating) | **NOT PROVEN AS REQUIRED** (capability gap, not defect) |
| follower/like/unread counts are externally consistent | PROVEN (all live-CS on one table each) | PROVEN | PROVEN | PARTIALLY PROVEN | **PROVEN** |
| empty-feed "why data exists but feed empty" | NOT PROVEN | NOT PROVEN | NOT PROVEN | NOT PROVEN | **NOT PROVEN** (no cause isolated; provider lifecycle NOT claimed) |

---

## 12. CLEANUP-READINESS MATRIX

Do NOT clean up — classify only. READY only when canonical authority, lifecycle, all callers, replacement, and non-dependency are all proven (the six READY criteria in the brief).

| Candidate | Cleanup class |
|---|---|
| `search_results` table | **NOT READY — AUTHORITY UNCLEAR → then CALLER UNCLEAR** (dead today, but confirm no migration/contract depends before dropping) |
| `content_mentioned_users` table + mention test files | **NOT READY — BUSINESS DECISION REQUIRED** (implement or drop; cannot delete until the MENTIONS decision is made) |
| `MentionedUserIDs` DTO field, `allowComments` | **NOT READY — BUSINESS DECISION REQUIRED** (dead value; confirm no planned feature) |
| `user_profiles.followers_count/following_count` dead columns | **NOT READY — RUNTIME PROOF REQUIRED** (verify no admin/legacy reader outside the grep set before a migration drops them) |
| mobile `GET /social/comments` count path + `/content/trending`/`/content/search`/`/content/contents` calls | **NOT READY — CALLER UNCLEAR** (map every mobile caller before removing; note trending/search are actively surfaced) |
| `CommentResponse.Type` re-derived field | **READY FOR CLEANUP** (additive only; no writer/reader depends; wire consumers confirmed to ignore) |
| presence websocket (`presence_subscriber.go` + test) | **NOT READY — AUTHORITY UNCLEAR** (confirm it is not planned to be wired before removal) |
| `content_likes` dual writer | **NOT READY — RUNTIME PROOF REQUIRED** (collapse requires proving both current writers' callers and no divergence) |
| mobile 5s comment poll / 10s like poll | **NOT READY — BUSINESS DECISION REQUIRED** (evaluate if realtime is wanted before removing the polling shims) |
| chat `followed_id` vs `following_id` mismatch | **NOT READY — PARALLEL DOMAIN** (chat domain, not Social) |

---

## 13. PRIORITY MAP

**P0 — canonical business truth contradicted/broken:**
- Mobile comment count → `GET /social/comments` (unregistered route). Comment counts are not reliably served to clients and the count semantic diverges from the canonical top-level count. (COMMENTS)

**P1 — authority/lifecycle conflict that can corrupt or diverge durable state:**
- `content_likes` has two independent writers (`LikeRepositoryImpl` + `TargetLikeRepository`) — duplicate-writer risk, no single authority. (LIKES)
- `/search/content` skips the `visibility` gate and (in default Shadow mode) block filter — a private/followers-only or blocked author's content can appear in search while excluded from feed/detail. (SEARCH)
- MENTIONS is confirmed broken scaffold (orphan schema + dead DTO + non-compiling tests + client-only notifier) — a business decision is required. (MENTIONS)
- Mobile content search/trending/content-list call non-existent routes (`/content/search`, `/content/trending`, `/content/contents`) — actively surfaced in mobile nav → broken UX paths. (API/MOBILE)

**P2 — confirmed legacy/zombie/duplicate structural debt:**
- `search_results` dead table; `user_profiles.followers_count/following_count` dead columns; `allowComments`/`MentionedUserIDs` decoration; re-derived `CommentResponse.Type`; legacy mobile `listComments`; 5 zombie mention tests; presence websocket unwired residue; content `content_likes` structural debt; `GET /likes/stats` governance bypass (repo read skips service block-check).

**P3 — hygiene/low-risk cleanup:**
- stale "no outbox events" entity doc comments (`user_follow.go:18` etc. contradict service behavior); mobile poll cadence constants; comment-count staleness windows (5s/10s).

**Runtime-proof gaps are NOT auto-promoted** — they stay out of P0/P1/P2 unless accompanied by the concrete state/authority/residue facts above. The "empty-feed" cause, live-DB observation of content/feed/comment/like/follow, and drain observation of the 8 events remain NOT PROVEN but are not defects.

---

## 14. OWNER DECISIONS REQUIRED

1. **COMMENTS:** Decide the canonical comment-count contract. Backend top-level count (`CountTopLevelCommentsByContent`, excl. replies) vs a page `total`. Must re-point mobile `getCommentCount` to a real endpoint.
2. **LIKES:** Decide a single writer for `content_likes`. Choose `LikeRepositoryImpl` (content-specialized, already used by `ContentService` + `LikeService`) or make `TargetLikeRepository` the sole writer; remove the second.
3. **SEARCH:** Decide whether `/search/content` must enforce `visibility` and block filtering like feed/detail. If yes, add the predicate + handler-level block set. If visibility is intentionally excluded, document it (currently CONFLICT).
4. **MENTIONS:** Decide the business requirement. Implement writer→read→event→consumer, or drop the table + DTO + tests as obsolete scaffold. Do not leave it in the ambiguous half-state.
5. **SOCIAL REALTIME:** Decide whether Social (feed/comments/likes/notifications) needs realtime at all. If not, keep REST polling and mark it deliberate. If yes, a realtime path must be designed (the current WS is chat-only and drops Social events).
6. **MOBILE DEAD ENDPOINTS:** Decide whether `/content/trending`, `/content/search`, `/content/contents` had a real product intent. Trending has no backend route at all; either build it or remove the mobile nav.

---

## 15. PROTECTED PARALLEL BOUNDARIES

Marked **PARALLEL / PROVENANCE UNKNOWN**, NOT audited internally, and NOT touched:
- **All Order / Payment / Coins / Discount / Refund / Dispute / Finance / Ledger / Payout internals.**
- `content_resource_occurrences` fixed_price_sale/auction sources (`_fps_source`, `_auction_source`), `comment_commerce_references` fixed_price_sale/auction FKs, `seller.response` handler — recognized as parallel commerce touch, reported only as provenance-boundary.
- **Chat realtime** (`backend/internal/realtime/*`, `/api/v1/ws`, mobile chat WS + FCM for `chat_message`) — the only production WebSocket and push path; commerce/chat domain.
- `/search/listings`, `/search/auctions`, commerce repost-governance filters (FIX-3/4) — commerce search/provenance.
- **`chat_resource_authorizer_adapter.go:163`** `followed_id` vs `following_id` column mismatch — a **chat-domain cross-reference** flagged UNKNOWN severity; requires a chat-domain audit, not a Social change.
- `moderation`/`appeals` that read `contents`/`comments` through raw entities — governance is a boundary consumer (reads Social state for moderation; does not mutate Social state except via the canonical `SoftDelete`/`Restore`).

---

## 16. WHAT IS ACTUALLY HEALTHY

- **`contents` single-writer authority** behind `ContentService`; soft-delete; visibility enforced at both write and read on feed/detail/comments.
- **Feed is a correct direct-query over canonical `contents`** (not a projection), cursor-based, guarded, reaching mobile home feed correctly.
- **Comment cursor create/list/delete** is sound and single-authority; soft-delete + moderation reuse the canonical repo.
- **Follow/block/mute** each have a single repository; routes match mobile; follow/block emit transactional outbox events; block severs both follow directions at write time; `blockcheck` gates the read surfaces that use it.
- **Share authority (`content_resource_occurrences`) is DB-immutable, exactly-one-source, anti-self-reference**, and correctly replaces the dropped `contents.share_reference`.
- **8 Social events** flow producer→outbox→worker→notifications→(in-app) end-to-end with idempotency dedup and a push-retry queue; payloads never become a second state authority.
- **Like/follower/unread counts** are all live `COUNT(*)` on a single table each — externally consistent (no denormalized count drift except the dead columns).
- **No Social cache becomes an independent truth;** search has no cache at all; content likes on `content_likes` are value-consistent across both readers today.

---

## 17. WHAT IS ACTUALLY BROKEN

- **Mobile comment count is a client↔server contract break**: it calls `GET /social/comments`, which no backend route serves, and reads a `total` whose semantics (page, includes replies) differ from the backend canonical top-level count. Comment counts are not reliably served to clients. (P0)
- **Search content leaks by omission**: `/search/content` does not apply the `visibility` gate and (in default Shadow mode) does not filter blocked authors, so private/followers-only content and blocked authors' content can surface in search while correctly excluded from feed/detail. This is a live authority defect (a reader using a less-restrictive source than the canonical read path). (P1)
- **Mobile content search/trending/content-list call non-existent routes** (`/content/search`, `/content/trending`, `/content/contents`): active mobile navigation (trending, content search) hits 404. (P1)

---

## 18. WHAT IS CONFIRMED DUPLICATE AUTHORITY

- **`content_likes`: two independent writer implementations** (`LikeRepositoryImpl` — content-specialized, used by `ContentService`/`LikeService`; `TargetLikeRepository` — content+comment dispatch, used by `CommentLikeService`/stats). Both write the same physical table; neither is authoritative over the other. Value-consistent today, duplicate-writer by design.
- **Comment count: two authorities** (backend top-level canonical vs mobile page-total on a dangling route).
- **Follower/following counts: four live calculation sites** (profile repo, follow cards, admin, mobile) — all consistent because all read `user_follows`, but structurally duplicated; plus dead denormalized `user_profiles.*_count` columns that contradict the schema.
- **Visibility/block eligibility: feed/detail authors one result; search authors a divergent (unfiltered) result.**

---

## 19. WHAT IS CONFIRMED ZOMBIE / RESIDUE

- **ZOMBIE:** `content_mentioned_users` (no production writer/reader), `search_results` (dead table with FK/index), `contents.share_reference` (already removed), the 5 mention test files (reference non-existent symbols; cannot compile), the presence websocket (`presence_subscriber.go` + test; unwired in `serverboot`).
- **RESIDUE:** `MentionedUserIDs` DTO field (decoded, never used), `allowComments` (no schema authority; always `true`), `user_profiles.followers_count/following_count` dead columns, `CommentResponse.Type` (re-derived after `comments.type` dropped), stale "no outbox events" entity doc comments, dangling mobile endpoints (`/content/trending`, `/content/search`, `/content/contents`, `/social/comments`).

---

## 20. WHAT IS STILL NOT PROVEN

- **Runtime observation of the healthy chains** (content R/W, feed, comment cursor, follow/block/mute, share occurrences, the 8 notification events) — statically PROVEN, runtime PARTIALLY PROVEN (no live-DB inspection in this read-only session). These are NOT defects; they are proof gaps.
- **Empty-feed cause** — NOT PROVEN; provider lifecycle is NOT claimed as root cause.
- **Mentions business requirement** — NOT PROVEN (feature may be unimplemented-by-intent); that is why it needs an owner decision, not a fire-and-forget fix.
- **Social realtime requirement** — NOT PROVEN AS REQUIRED (no current consumer workflow needs a feed/comment/like/notification WS; presence WS is aspirational residue).
- **`chat_resource_authorizer_adapter` follow-column mismatch severity** — NOT PROVEN (chat domain).
- **Whether `/search/content` visibility/block omission has been producer-visible in real data** — concrete leak instances are NOT PROVEN (the omission is static fact; occurrence in real data is unobserved).

---

## 21. NEXT AUDIT TARGET

Prioritized and scoped, in dependency order:

1. **COMMENTS count contract (P0)** — a focused re-audit of `CommentApiDatasource`'s `listComments/getCommentCount`, the actual mobile `commentCountProvider` consumers, and the backend canonical count, to pin the precise replacement endpoint and semantics for the owner decision.
2. **Social Search visibility/block (P1)** — verify the `search_content` evaluator's effective mode default (Shadow vs Enforce), list every content-search consumer, and confirm the exact visibility/block gap before any change.
3. **`content_likes` dual-writer (P1)** — enumerate all callers of `LikeRepositoryImpl` vs `TargetLikeRepository` on content targets, plus `/likes/stats` governance bypass, to drive the single-writer decision.
4. **MENTIONS (P1)** — a pure requirements/search re-audit: is there ANY product trace (PRD, config, UI) that requires server-side mentions? Feed that back into the implement-or-drop decision. Do not implement.
5. **Mobile dead endpoints (`/content/trending|search|contents`, `/social/comments`) (P1/P2)** — map every mobile nav/provider consumer to decide build-vs-remove.
6. **Deferred/parallel:** chat `followed_id` mismatch and presence websocket intend need a **chat/real-time domain audit** (PARALLEL / PROVENANCE UNKNOWN). Do not touch in the Social thread.
7. **Finally**, once the above decisions are cast, a cleanup-readiness pass over `search_results`, `user_profiles` dead columns, and the mention scaffold can be gated for a future execution thread.

This document is read-only evidence. It makes no code, schema, test, config, or database changes, and contains no implementation instructions. A correct NOT PROVEN is preferred over a false closure, and every unresolved contradiction above is marked UNRESOLVED rather than silently resolved.