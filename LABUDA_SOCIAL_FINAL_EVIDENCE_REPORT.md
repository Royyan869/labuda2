# LABUDA — SOCIAL AUDIT FINAL EVIDENCE REPORT

**Status:** READ-ONLY VERIFICATION — NO IMPLEMENTATION PERFORMED
**Scope:** CONTENT / FEED / COMMENTS / LIKES / FOLLOW-BLOCK-MUTE / MENTIONS / SHARE REFERENCE / SOCIAL→NOTIFICATION-OUTBOX
**Evidence basis:** live source inspection (`backend/`, `apps/mobile/`, `backend/migrations/`), route registrations, wiring in `serverboot/dependencies.go`, outbox drain in `internal/worker`. No code, test, schema, migration, DB, or Git change was made.

**Evidence discipline:** every conclusion follows FACT → AUTHORITY → DURABLE STATE → CURRENT READER/WRITER → STATIC PROOF → RUNTIME PROOF → VERDICT. Static proof is sourced from code/schema/route; runtime proof is sourced only from observable production-path wiring (a drained outbox event into a real consumer, a real route→handler→repo chain), never from the mere existence of a test. "Test passes", "code exists", and "no caller" are not used as authority proof.

---

## 1. CONTENT

#### A. Canonical durable authority
- **Canonical table/entity:** `contents` (schema `000001_canonical_schema.up.sql:678`); entity `content/entity/content.go:13-28` (fields: id, author_id, status, visibility, caption, city, province, is_hidden, original_author_id, created_at, updated_at, deleted_at).
- **Canonical writer:** `ContentRepositoryImpl` (`content/infrastructure/repository/content_repository_impl.go:24 Create`, `:230 Update`), invoked **only** by `ContentService` (`content_service.go:146,243,397,434,471,769,809,919` and the internal-share path `internal_share_authority.go:118,152`). No admin, commerce, or worker writes `contents` — moderation reuses the same repo Update.
- **Canonical reader:** feed (`feed_repository_impl.go:150`), search (`search_repository_impl.go:311,442`), profile/user list (`content_repository_impl.go:289`), content detail (`content_handler.go` `GetContent`), moderation preview (`moderation_handler.go:789`), notification workers (read `author_id` only).
- **Derived state:** `content_resource_occurrences` (share/repost provenance), `content_hashtags` (tags), `content_media` (media), search `search_vector` (generated column).
- **Duplicate authorities:** none for the `contents` write path. Non-production seed writer `cmd/seed/main.go:470` exists only as data bootstrap.

#### B. Lifecycle
- **create:** `CreateContent` / `CreateContentWithResourceOccurrence`; visibility→`is_hidden` mirrored only on the occurrence-create path (`content_service.go:239`) and `UpdateCaptionAndVisibility` (`:936-942`); the plain `CreateContent` normalizes visibility without syncing `is_hidden`.
- **persist:** `content_repository_impl.go:24` INSERT in the caller's tx; occurrence + tags written in same tx on the canonical path.
- **read:** feed/search/detail/user-list readers filter `status='active'`, `is_hidden=false`, `deleted_at IS NULL`, and authorize visibility in SQL + `validatePublicContentVisibility` (`content_service.go:544-584`).
- **update:** `UpdateCaptionAndVisibility` (`:919-946`).
- **delete/soft-delete:** `DeleteContent` (`:397`) transitions via `Content.Delete()` (`content.go:76-95`) → `StatusDeleted` + `DeletedAt`; `contents` keeps the row (hard purge not used for content).
- **moderation:** `SoftDeleteForModeration` (`content_service.go:769`) / `RestoreFromModeration` (`:809`), driven by `worker/moderation_event_handler.go` (`handleContentRemoved :304`, `handleContentRestored :390`). **There is no `suspended`/`banned` content status** — content moderation reuses soft-delete; author-level restriction lives on `users.account_status`, not content.
- **downstream effects:** no outbox event on content create or delete (see §8). Repost/share writes `content_resource_occurrences` (see §7).

#### C. State proof
- **Is durable state authoritative? YES** — single writer implementation, DB-sourced.
- **Another competing state? NO** — no shadow content table.
- **Can two writers disagree? NO** — all writes funnel through `ContentRepositoryImpl` reached only by `ContentService`.
- **Can state disappear without terminal record? NO** — soft-delete keeps the row with `deleted_at`.
- **Is current schema consistent with the code? YES** — `contents` columns match the entity; `share_reference` correctly removed (migration 000041) and strict-bind rejects it (`content_handler.go:505,713`); visibility/status enums match.

#### D. Runtime proof
**PARTIALLY PROVEN** — static path is complete and single-writer; runtime wiring (route→service→repo→DB) is present, but the content object graph was not exercised against a live DB in this read-only session.

#### E. Residue / duplicate / zombie
| Candidate | Evidence | Current caller/writer | Classification |
|---|---|---|---|
| `contents.share_reference` column | Dropped by 000041; Go code removed it (`content.go:156`); DB integration test asserts column absent | none | ZOMBIE (schema: gone; any legacy mention in mocks is stray) |
| `AllowComments` field (`content_handler.go:208`) | DTO-only; **no `allow_comments` column** in schema; mobile always sends `true` (`content_submission_handler.dart:112`) | mobile DTO/entity propagation return only | ACTIVE NON-CANONICAL (decoration, not enforced) |
| content moderation "suspended/banned status" | `content_status.go` has only active/deleted; no content status value exists | moderation worker uses `StatusDeleted` | DEPRECATED BUT REACHABLE (functionally, "removed" == soft-delete) |

#### F. False closure
- "Moderation bans content via a status." → CONTRADICTED (only soft-delete; author-level ban is on `users`).
- "Content create/delete is notified downstream." → CONTRADICTED (no such outbox event).
- "Visibility is enforced at write AND read." → PROVEN (static), see §D runtime caveat.
- "`is_hidden` is always kept in sync with visibility." → PARTIALLY PROVEN (synced on occurrence-create and update, not on plain `CreateContent`).

#### G. Final verdict
**PROVEN** (static, single-writer, schema-consistent). Marked `PARTIALLY PROVEN` only for live-DB runtime evidence.

---

## 2. FEED

#### A. Canonical durable authority
- **Canonical table/entity:** there is **no feed table**. The feed reads the canonical `contents` table directly (`feed_repository_impl.go:150` `FROM contents c`), left-joined to `content_resource_occurrences`, `user_profiles`, `users`, `content_media`, `user_follows` (follower gating / feed priority).
- **Canonical writer:** n/a (feed is a read view over `contents`; writes land via ContentService, §1).
- **Canonical reader:** `FeedService.GetFeed` (`feed/application/feed_service.go:44`) → `FeedRepository.GetFeed` → cursor-based SQL.
- **API:** `GET /api/v1/feed` (`routes_core.go:1219`), handler `feed_handler.go:93 GetFeed`; cursor-based (`next_cursor` + `has_more` from LIMIT+1 probe). Rejects anonymous.
- **Mobile path:** `feed_api_datasource.dart:49` `GET /feed` → `home_repository_impl.dart:25` → `feed_notifier.dart:30` (auto-loads at `build()` via `Future.microtask`) → `home_screen.dart` (pull-to-refresh `:151`, cursor `loadMore` `:111-170` with dedupe + cursor-stall guard).
- **Derived state:** `feed_item.go` read projection; **feed is NOT a projection table** (confirmed: no `feed` table in schema; `schemaguard` lists feed_repository_impl as canonical content reader).

#### B. Lifecycle
- **create→persist→read:** content create (§1) → feed query reads live `contents` → `GET /feed` → mobile provider.
- **update/delete:** soft-deletes and visibility changes are reflected because the feed reads canonical state (no projection lag).
- **moderation:** removed content drops out of feed via `status/active` + `deleted_at` filters.
- **downstream effects:** feed page returns resource projection (`feed_share_projection.go`) resolved from `content_resource_occurrences` (§7).

#### C. State proof
- **Is durable state authoritative? YES** — feed IS the canonical state by direct query, no shadow.
- **Another competing state? NO.**
- **Can two writers disagree? NO.**
- **Can state disappear without terminal record? NO** (same as contents).
- **Schema consistent with code? YES.**

#### D. Runtime proof
**PARTIALLY PROVEN** — one authoritative query → one route → mobile chain; empty-feed behavior not resolved at runtime (below).

#### E. Residue / duplicate / zombie
| Candidate | Evidence | Current caller/writer | Classification |
|---|---|---|---|
| historic "empty Feed although data exists" | no dedicated feed table; feed is a direct contents query; no provider lifecycle flag at controller layer; the feed mobile provider auto-loads and is guarded against cursor stalls | live CORE feed path | UNKNOWN (root cause not determinable from static evidence alone) |

#### F. False closure
- "Feed works." → PARTIALLY PROVEN (static chain complete).
- "Feed requires a projection table." → CONTRADICTED (feed is a direct contents query; no projection).
- "The empty-feed issue is caused by mobile provider lifecycle." → NOT PROVEN (no runtime evidence isolates the cause; see below).

#### G. Final verdict
**PROVEN** (capability), with the empty-feed semantics separated, not proven at runtime.

**Empty-feed separation (as required):**
- **STATIC CANDIDATE:** feed is a direct `contents` query (not a projection); mobile auto-loads synchronously at provider build; no feed table to fall out of sync with.
- **STATE PROOF:** because feed reads live `contents`, an "empty feed despite data" state is not explained by a missing projection. Filters that could legitimately empty a feed: `status='active'`, `is_hidden=false`, `deleted_at IS NULL`, visibility authorization, and follow-based preference gating — all present in `feed_repository_impl.go:150-235`.
- **RUNTIME PROOF:** NOT PROVEN. No production evidence (logs, reproduction) distinguishes between (a) a data-side filter excluding rows, (b) a client render/timing issue, or (c) an authorization edge case. Per the brief, provider lifecycle is NOT claimed as root cause.

---

## 3. COMMENTS

#### A–C. Canonical durable authority
- **Canonical table/entity:** `comments` (schema `000001_canonical_schema.up.sql:635`) with canonical columns `id, author_id, body, target_id, target_type, parent_id, created_at, updated_at, deleted_at`. Legacy `type`, `fixed_price_sale_id`, `share_reference` were **purged** in migration `000031_comment_commerce_reference_canonical.up.sql:103-105`, which introduced `comment_commerce_references` for commerce-target association.
- **Canonical create:** `CommentService.AddComment` (`comment_service.go:235`) + `CreateCommerceReferenceComment` (`:456`), writing `comments` via `CommentRepositoryImpl`.
- **Canonical list (cursor):** `ListComments` → `ListByTarget` (`comment_repository_impl.go:91-173`), cursor-based (`created_at` opaque cursor, LIMIT+1). **There is NO page-based backend comment list.** Route `GET /contents/:id/comments` (`routes_core.go:1248`).
- **Canonical count:** backend `CountTopLevelCommentsByContent` excludes deleted + replies (`comment_repository_addition.go:180-197`); the handler-computed count path (`CommentCount`/`CountTopLevelCommentsByContent`) is the canonical reader. **Note:** the mobile comment-count path does NOT use this (see legacy count). CommentResponse carries **no like-count field** (verified: no comment like-count in DTO; only the like toggle returns a count).
- **Canonical delete/soft-delete:** `CommentService.DeleteComment` (`:552`) → `SoftDelete` sets `deleted_at`; `SoftDeleteForModeration`/`RestoreFromModeration` (`:568,:604`).
- **Legacy count (mobile):** `CommentRepositoryImpl.getCommentCount` (`comment_repository_impl.dart:129-148`) calls **`listComments`** → `GET /social/comments` (page-based). **This backend route DOES NOT exist** (`routes_core.go` has only `/contents/:id/comments`, `/comments/:id`, `/contents/:id/comments/reference`; grep for `social/comments` in `backend` = no match). **The mobile legacy count path is DANGLING — it calls a never-registered endpoint.** The `listComments` datasource method (`comment_api_datasource.dart:77-100`) is legacy-but-reachable in mobile code.
- **allowComments residue:** see §1.E — DTO/entity decoration only; no schema column, always `true`.
- **comment DTO residue:** `comment_response.go` still carries `Type`/`Reference *entity.ShareReference`/`Listing` — `Type` corresponds to the purged `comments.type` column (now always rederived), `Reference`/`Listing` are the commerce-reference preview path backed by `comment_commerce_references` (canonical). No stale object is read from a dropped column (verified handler/listing hydration), so these are additive wire fields, not live residue reads.

**Route→handler→service→repository→mobile→provider reachability (as required):**
- **Cursor list:** `GET /contents/:id/comments` (`routes_core.go:1248`) → `CommentHandler.ListComments` → `CommentService.ListComments` → `ListByTarget` → mobile `listContentComments` (`comment_api_datasource.dart:57`) → provider/UI. Reachable end-to-end.
- **Page list:** `GET /social/comments` referenced in mobile `listComments` (`comment_api_datasource.dart:97`) → **no backend route/handler**. Not reachable from the server. Zero backend import of `social/comments` is consistent with the route genuinely not existing (confirmed by full-backend grep), so this is not a "search-blindness" false negative.
- **Create:** `POST /contents/:id/comments` (`routes_core.go:1247`) → `CommentHandler.CreateComment` → `AddComment`. Reachable.
- **Delete:** `DELETE /comments/:id` (`routes_core.go:1253`) → `CommentHandler.DeleteComment`. Reachable.

#### D. Runtime proof
**PARTIALLY PROVEN** — the cursor create/list/delete chain is wired end-to-end; the **mobile legacy count is NOT PROVEN** because its endpoint is absent at runtime (would return 404), and no production log evidence was inspected.

#### E. Residue / duplicate / zombie
| Candidate | Evidence | Current caller/writer | Classification |
|---|---|---|---|
| mobile `listComments` datasource (`GET /social/comments`) | page-based; **no backend route** | `getCommentCount` (`comment_repository_impl.dart:136`) | DEPRECATED BUT REACHABLE (mobile only; server endpoint gone → effectively dangling) |
| mobile comment count via legacy endpoint | reads `total` from a non-existent route | `CommentRepositoryImpl.getCommentCount` | CONFIRMED dangling call (mobile-side defect candidate, NOT authority residue) |
| `allowComments` (mobile entity/DTO + backend DTO) | no schema authority; always `true` | mobile content mapper/settings, content create | RESIDUE (decoration only) |
| `CommentResponse.Type` | column `comments.type` dropped (000031) | re-derived in handler | ACTIVE NON-CANONICAL (harmless additive) |
| `CommentResponse.Reference`/`Listing` | backed by `comment_commerce_references` (canonical) | commerce-reference comments | CANONICAL ACTIVE |

#### F. False closure
- "Comment list is page-based." → CONTRADICTED (cursor-based on backend; the page-based form is mobile-only and its endpoint no longer exists).
- "Comment count reads from the canonical list." → PARTIALLY PROVEN on backend; CONTRADICTED on mobile (legacy route absent).
- "Comment delete is permanent." → CONTRADICTED (soft-delete with `deleted_at`).
- "There is a `comment.liked` event." → CONTRADICTED (no such event; see §4).

#### G. Final verdict
**CONFLICT** — canonical cursor path is sound and single-authority, but the mobile count path targets a removed route, and the comment count is therefore not reliably served to clients. This is a client↔server contract break, not a duplicate schema authority.

---

## 4. LIKES

#### A. Canonical durable authority
- **Canonical tables/entities:** `content_likes` (`000001:663`) and `comment_likes` (`:629`), both `(target_id, user_id)` + `created_at`.
- **Canonical writers:** `LikeRepositoryImpl` writes/reads **only `content_likes`** (`like_repository_impl.go:33,58,83,109`). `TargetLikeRepository` writes/reads **`content_likes` (targetType=content) AND `comment_likes` (targetType=comment)** via table mapping (`like_repository.go:123-133`).
- **Canonical readers:** same two repos; **content like count** surfaces via `ContentService.GetLikeCount→CountLikes` (`content_service.go:718-724`) on `content_likes`, attached to `ContentResponse.EngagementResponse.LikeCount` (`content_handler.go:277-286,1012`) and to feed cards.
- **Comment like count:** CommentLikeService reads `comment_likes` (`comment_like_service.go:72-93`); counts surface only in the like-toggle HTTP response, NOT in comment list DTOs.

#### B. Lifecycle
- **content like/unlike:** `LikeService.Like/Unlike/ToggleContentLike` (`like_service.go:105,132,161`); content toggle → `content_likes`; emits `content.liked` on **like only**, idempotency key `content.liked.{contentID}.{actorID}`; **unlike emits no event**.
- **comment like/unlike:** `CommentLikeService.ToggleCommentLike` (`:87`) → `comment_likes`; **no event on like or unlike**.
- **count/stat path:** content `CountLikes` on `content_likes`; `GET /likes/stats` (`routes_core.go:167` public browse) → `LikeHandler.GetLikeStats` → `TargetLikeRepository` (bypasses service governance); mobile polls `GET /likes/stats` every 10s (`like_repository_impl.dart:111-119`) — not optimistic.

#### C. State proof
- **Durable state authoritative? YES** (both tables are the single physical store).
- **Competing state? None.**
- **Can two writers disagree?** For `content_likes`, **YES — two independent writer implementations converge on the same table** (`LikeRepositoryImpl` and `TargetLikeRepository` content branch). Both funnel to the same INSERT `ON CONFLICT`-style upsert semantics, so a single row per (content,user); but the two are administratively independent and neither wraps the other. For `comment_likes`, exactly one writer (`TargetLikeRepository`).
- **Disappear without terminal record?** `DeleteLike` is an unconditional `DELETE` (hard delete) — a like disappearing leaves no terminal record by design (like/unlike is a pure toggle, not an audited lifecycle). This is acceptable for a like, but there is no tombstone/audit.
- **Schema consistent with code? YES.**

#### D. Runtime proof
**PARTIALLY PROVEN** — route→handler→service→repo chain wired for both likes; live `content.liked`→consumer drain is evidenced only statically, not observed at runtime in this read-only session.

#### E. Residue / duplicate / zombie
| Candidate | Evidence | Current caller/writer | Classification |
|---|---|---|---|
| `LikeRepositoryImpl` (content_likes) | hard-codes `content_likes` | LikeService (`dependencies.go:2213`) + ContentService (`:2230`) | CANONICAL ACTIVE |
| `TargetLikeRepository` (content + comment) | dispatches by targetType onto both tables | CommentLikeService (`:2311`) + LikeHandler stats (`:2314`) | ACTIVE NON-CANONICAL (duplicate writer for `content_likes`) |
| `GET /likes/stats` repo bypass | `GetLikeStats` reads repo directly (`like_handler.go:214,223`), skipping service governance/block-check | mobile 10s poll | ACTIVE NON-CANONICAL (governance bypass) |
| `comment.liked` event | absent from `events.go` | none | ZOMBIE (never existed) |
| unlike/unfollow "one-way event" | content unlike emits no event; follow unfollow does | n/a | ACTIVE NON-CANONICAL (inconsistent eventing) |
| mobile like-count polling | non-optimistic 10s poll of `/likes/stats` | mobile provider | ACTIVE NON-CANONICAL (uncoupled from toggle result) |

**LikeRepositoryImpl vs TargetLikeRepository determination (by evidence, NOT naming):**
1. Physically both write the same `content_likes` table → **duplicate writers for content likes** (verdict 2/3, below).
2. They are **not** wrapping each other and not one-legacy-one-new with a deprecation marker.
3. They differ only in table dispatch: `LikeRepositoryImpl` is content-specialized; `TargetLikeRepository` generalizes to comments.
→ **CONCLUSION: duplicate/content-specialized responsibility is NOT cleanly separated; `content_likes` has two independent writer implementations.** This is a **confirmed duplicate writer (one table, two paths)**, not a legitimate orthogonal separation, because both may serve content likes and neither is authoritative over the other.

#### F. False closure
- "LikeRepositoryImpl and TargetLikeRepository are separate responsibilities." → CONTRADICTED for `content_likes` (both write it).
- "There is a comment-like event for downstream notifications." → CONTRADICTED.
- "Unlike is notified." → CONTRADICTED.
- "Top-level post like total is the only like stat." → CONTRADICTED (stats endpoint bypasses service-layer block-check).

#### G. Final verdict
**CONFLICT** — single physical authority per table, but `content_likes` has two independent writer implementations (authority-convergence risk) and a governance-bypassing stats reader.

---

## 5. FOLLOW / BLOCK / MUTE

#### A. Canonical durable authority
- **Canonical tables/entities:** `user_follows` (`000001:1719`), `user_blocks` (`:1700`), `user_mutes` (`:1725`); entities `user_follow.go` / `user_block.go` / `user_mute.go`.
- **Canonical writer:** `SocialRepositoryImpl` (`graph/infrastructure/repository/social_repository_impl.go:68-537`), reached only through `SocialService` (`graph/application/social_service.go`). Single implementation.
- **Canonical readers:** same repo + feed SQL reads `user_follows` (`feed_repository_impl.go:151`); `blockcheck` (`pkg/blockcheck/blockcheck.go:31,65`); content/comment/feed/search/chat viewer-context block checks; user-profile follow counts read `user_follows` (`user_repository_impl.go:712,722`).
- **Routes:** follow/unfollow `POST/DELETE /users/:id/follow` (`routes_core.go:1343-1344`), block/unblock `:1349-1350`, mute/unmute `:1354-1355`, followers/following `:1359-1360`, `GET /follows/status/:userId` `:1363`, `GET /blocks` `:1366`, `GET /mutes` `:1367`. Mobile datasource paths match (`follow_api_datasource.dart`).

#### B. Lifecycle
- **follow/unfollow:** `SocialService.Follow`/`Unfollow` → `InsertFollow`/`DeleteFollow`; emits `user.followed` / `user.unfollowed` transactionally with the write.
- **block/unblock:** `InsertBlock`/`DeleteBlock`; block removes both follow directions (`DeleteFollowBothDirections`); emits `user.blocked` / `user.unblocked`.
- **mute/unmute:** `InsertMute`/`DeleteMute`; **emits NO event** (mute has no outbox event constant).
- **read:** feed gating uses `user_follows` (follows-first preference); block gate checked across surfaces via `blockcheck`/viewer-context.
- **delete:** `user_blocks`/`user_mutes` rows are hard-deleted on unblock/unmute (no tombstone).

#### C. State proof
- **Durable state authoritative? YES** — single writer per table.
- **Competing state? No.**
- **Two writers disagree? No** (single repo).
- **Disappear without terminal record?** follows/blocks/mutes are hard-DELETE on removal; acceptably a toggle, but no audit tombstone.
- **Schema consistent with code?** Mostly YES; **latent column mismatch** `chat_resource_authorizer_adapter.go:163` queries `user_follows.followed_id` while the real column is `following_id` (`000001:1719-1725`) — the follow check in that adapter can never match.

#### D. Runtime proof
**PARTIALLY PROVEN** — routes, repo, events wired; feed/consumer behavior not runtime-observed in this read-only session.

#### E. Residue / duplicate / zombie
| Candidate | Evidence | Current caller/writer | Classification |
|---|---|---|---|
| `chat_resource_authorizer_adapter.go:163` `followed_id` vs `following_id` | column mismatch against schema | chat projection authorizer | UNKNOWN severity (Chat/parallel domain — NOT audited internally; reported as provenance-unknown cross-reference) |
| mute event | no `user.muted` / `user.unmuted` event | none | RESIDUE (gap, not defect per §8) |
| entity-layer "no outbox events" doc comments (`user_follow.go:18` etc.) | imply no events, yet service layer DOES emit follow/block events | doc only | RESIDUE (stale comment) |

#### F. False closure
- "Following is notified." → PROVEN (event exists).
- "Muting is notified." → CONTRADICTED.
- "Block fully severs follow relationships." → PROVEN.

#### G. Final verdict
**PROVEN**

---

## 6. MENTIONS

#### A. Durable authority
- **Canonical table:** `content_mentioned_users` (`migrations/000043_content_mentioned_users.up.sql:1-8`), PK `(content_id,user_id)`, FK to `contents`/`users` CASCADE, indexed by `user_id`.

#### B. Explicit requirements
- **Does the feature exist in current business implementation?** **NO** as a persisted/notified feature. The mentions UI capability exists **client-side only** (shared mention widgets + `MentionNotificationService`), and it is disconnected from any backend mention write.
- **Is the schema authoritative or residue?** **Residue** — the table is created and referenced by FKs, but **no production writer or reader touches it**. It is a persisted-but-unused schema (orphan authority), not a live authority.
- **Does a production writer exist?** **NO.** Content create decodes `MentionedUserIDs` in the HTTP DTO (`content_handler.go:211`) but **never passes it to any service** (`content_handler.go:481,569,571` call sites omit it). `ContentService` has no mention parameter or write; `ContentRepositoryImpl` has no mention-insert method. `AddMentionedUserIDs`/`RemoveMentionedUserIDs` exist **only in test files**.
- **Does a production reader exist?** **NO.** No repo/service/handler/worker queries `content_mentioned_users`; the only SELECTs are in test files.
- **Does a mention event exist?** **NO.** `events.go` has no `EventContentMentioned`/`content.mentioned` constant → no outbox event can be produced.
- **Does a notification consumer exist?** **NO.** `NotificationEventHandler` has no mention handler; `notification_worker_social.go` has none; no mention event is registered in the dispatcher.
- **Is mobile data actually persisted?** **NO.** Mentions are ephemeral compose-time UI state (resolved text → user IDs); there is no device store wiring. `mention_parser.dart` has a `MentionData` "for caching" model but it is not wired to any store. `sync_outcome.dart` (new untracked) is unrelated — it is an auth sync sealed union, not mentions.
- **Why do the old mention tests compile/fail?** They **cannot compile**. `content_mention_proof_integration_test.go` and `content_mention_outbox_atomic_integration_test.go` call `NewContentService(…, mentionOutbox)` with a **7-arg signature** (production takes no 7th arg), invoke `AddMentionedUserIDs`/`RemoveMentionedUserIDs` (nonexistent), and reference `events.EventContentMentioned` (nonexistent). The three `content_mentioned_notification*.go` tests reference `h.handleContentMentioned`, `h.SetContentVisibilityChecker`, `ContentMentionedPayload`, `events.EventContentMentioned` — none of which exist; only the 6-arg `NewNotificationEventHandler` and `NewNotificationServiceInserter` they use are real. **Verdict: the mention test suite is obsolete/broken residue — it encodes a feature that was never built into production.**
- **Mobile mention notifications:** `MentionNotificationService.sendMentionNotifications` sends `NotificationType.mention` via a **local `INotificationTrigger`** (device-side, not the server outbox), so mentions can produce client-local notifications but the mention relation itself is not persisted server-side.

#### C. State proof
- **Durable state authoritative? NO** — the table is durable but unmanaged (no writer/reader).
- **Competing state? n/a** (no live feature).
- **Two writers disagree?** Irrelevant (none).
- **Disappear without terminal record?** Table is append-only empty; nothing is ever written.
- **Schema consistent with code?** **NO** — schema has the table; code has no production path to it. This is a confirmed **broken/obsolete residue**, not merely "not implemented": the migration exists and the HTTP DTO advertises a capability that the service layer does not implement, and five test files reference production symbols that do not exist.

#### D. Runtime proof
**NOT PROVEN** (there is no runtime path to prove).

#### E. Residue / duplicate / zombie
| Candidate | Evidence | Current caller/writer | Classification |
|---|---|---|---|
| `content_mentioned_users` table | migration 000043; no production writer/reader | none | RESIDUE (orphan schema authority) |
| `MentionedUserIDs` DTO field (`content_handler.go:211`) | decoded, never passed to service | content-create handler (dead value) | RESIDUE |
| mobile mention widgets + `MentionNotificationService` | client feature; local trigger; no backend persistence | shared mention widgets / content+comment create | ACTIVE NON-CANONICAL (client-only, unbacked) |
| 5 mention test files | reference nonexistent production symbols | tests only | ZOMBIE (cannot compile; encode unimplemented feature) |
| `AddMentionedUserIDs`/`RemoveMentionedUserIDs` | exist only in tests | tests only | ZOMBIE |

#### F. False closure
- "Mentions are implemented." → CONTRADICTED.
- "Mentions are only not-yet-notified." → CONTRADICTED (no writer, reader, or event at all).
- "Mention tests pass." → CONTRADICTED (cannot compile).
- "Mentions just need a notification wire-up." → PARTIALLY PROVEN (multiple layers — write, read, event, consumer — are all absent, not one).

#### G. Final verdict
**CONFLICT** — schema/HTTP-DTO advertise a mentions contract the service layer never implements. **Category: CONFIRMED BROKEN/OBSELEETE RESIDUE.** (Not "architecturally broken in production" because nothing runs; not merely "runtime-unproven".)

---

## 7. SHARE REFERENCE

#### A. Canonical durable authority
- **Canonical table/entity:** `content_resource_occurrences` (`000039_content_resource_occurrences.up.sql:19`), entity `content_resource_occurrence.go`. **Immutable** (DB trigger `000039:68-80`), exactly-one-source CHECK, operation enum `share_to_feed|direct_commerce_insert_content`, anti-self-reference CHECK.
- **Canonical writer:** `ContentRepositoryImpl.CreateResourceOccurrence` (`content_repository_impl.go:467-483`), reached through `createContentResourceOccurrence` (`content_service.go:953-967`) from: (1) `CreateContentWithResourceOccurrence` (`content_service.go:249`) — primary content create; (2) internal content-share/repost path (`internal_share_authority.go:127`); (3) internal reference-share path for fixed_price_sale/auction/profile (`internal_share_authority.go:161`). Commerce integration is provenance-recognized via the same table (see protection note).
- **Canonical reader:** `ContentResourceProjectionResolver.loadOccurrences` (`content_resource_projection_resolver.go:298-327`), exposed on content detail/list, feed (`feed_repository_impl.go:152`), search (`search_repository_impl.go:313,443`), profile, chat projection.
- **Mobile path:** `ContentResponse.ResourceProjection` (`content_handler.go:260`) → mobile content DTO/provider; the home-feed iteration renders share/repost cards from this projection.
- **Deprecated path:** `contents.share_reference` — **dropped** (000041), rejected by strict bind, and DB integration-tested as absent. Ghosted.
- **Durable state:** occurrence rows are the authoritative provenance; DELETE of content cascades its occurrences.

#### B. Lifecycle
- **create:** writing a content or a repost/reference-share inserts an immutable occurrence in the same tx as the content row.
- **read:** projection resolver materializes LIVE or TOMBSTONE single-resource payload (`content_resource_projection.go:75-141`).
- **update:** blocked by immutable trigger (a repost cannot be re-targeted).
- **delete:** cascading content delete removes occurrences (immutable rows removed via cascade, not UPDATE).
- **downstream effects:** feed/search/value surfaces render the projection; commerce sources are touchable only via provenance fields (not audited internally per protection rule).

#### C. State proof
- **Durable state authoritative? YES** — DB-enforced immutability + exactly-one-source make the schema the authority (integration tests `CW16-CW21` prove constraint enforcement against Postgres).
- **Competing state? NO** — legacy `share_reference` is gone.
- **Two writers disagree? No** — occurrences are append-immutable; concurrent duplicate inserts rejected via PK + same-content create authority.
- **Disappear without terminal record?** cascade on content delete removes provenance; acceptable (content terminal event is the soft-delete, occurrences cascade only on hard purge).
- **Schema consistent with code? YES.**

#### D. Runtime proof
**PARTIALLY PROVEN** — immutable schema + resolver + feed/search read path all present; occurrence writes were not exercised against a live DB in this read-only session (DB integration tests exist but are tests, not runtime proof).

#### E. Residue / duplicate / zombie
| Candidate | Evidence | Current caller/writer | Classification |
|---|---|---|---|
| `contents.share_reference` | dropped (000041), rejected in bind | none | ZOMBIE (gone) |
| `entity/share_reference.go` — content-share sense | repost state no longer stored there; used for comment commerce reference (canonical) | comment commerce-reference | ACTIVE NON-CANONICAL (re-purposed, not content share) |
| `share_reference` comment handling / nested refs | comment JSONB reference + chat projection nested-ref | comment/reference + chat | ACTIVE NON-CANONICAL (comment domain, canonical there) |

#### F. False closure
- "Share authority is the `contents.share_reference` blob." → CONTRADICTED (dropped).
- "Occurrences are mutable." → CONTRADICTED (DB trigger rejects UPDATE).
- "Commerce list share is a separate Social authority." → PARALLEL / PROVENANCE UNKNOWN (explicitly not internally audited).

#### G. Final verdict
**PROVEN**

---

## 8. SOCIAL → NOTIFICATION / OUTBOX

#### A. Mechanism (verified)
- **Producer→event→outbox:** social producers write events **transactionally** via `OutboxRepository.InsertEvent/InsertTx` (`outbox_repository.go:89,150`, `ON CONFLICT (idempotency_key) DO NOTHING`) inside the same `db.WithTx` as the domain write.
- **Worker:** `OutboxWorker` (enabled in `serverboot/dependencies.go:1267-1270`) drains `outbox` → dispatches to `NotificationEventHandler` (`notification_worker.go:65`), which inserts a `notifications` row (dedup `ON CONFLICT … DO NOTHING`) and pushes via FCM (`sendPushAsync`), with failure enqueued to `push_retry_queue` and drained by `PushRetryWorker`.
- **Consumer:** `NotificationEventHandler` (`notification_worker_social.go` handlers), registered via `SetupNotificationHandlers` (`dependencies.go:1341`).

#### B. Social event inventory (production producers → consumers)
| Event | Producer (file) | Consumer (handler) | Exists |
|---|---|---|---|
| `comment.created` | `comment_service.go:272,288` | `handleCommentCreated` `:99` | YES |
| `comment.reply` | `comment_service.go:256` | `handleCommentReply` `:158` | YES |
| `seller.response` | `comment_service.go:474` | `handleSellerResponse` `:219` | YES |
| `content.liked` | `like_service.go:120,207` | `handleContentLiked` `:50` | YES |
| `user.followed` | `social_service.go:108` | `handleUserFollowed` `:14` | YES |
| `user.unfollowed` | `social_service.go:146` | `handleUserUnfollowed` `:414` | YES |
| `user.blocked` | `social_service.go:198` | `handleUserBlocked` `:367` | YES |
| `user.unblocked` | `social_service.go:237` | cleanup | YES |
| **`content.mentioned`** | **- (absent)** | **- (absent)** | **NO** |
| **`content.created`** | **- (absent)** | **- (absent)** | **NO** |
| **`mute/unmute`** | **- (absent)** | **- (absent)** | **NO** |
| **comment/comment-reply "unlike", comment unlike** | **- (absent)** | **- (absent)** | **NO** |

Notification types used are string constants (`user.followed`, `content.liked`, `comment`, `comment_reply`, `chat_message`, `seller.response`); there is a **code-level list** `socialNotificationTypes` (`notification_worker_social.go:348-355`); schema `notifications.type` is free-text (no DB enum).

#### C. Missing events — asked per-event: is there evidence a current Social consumer/Business workflow requires it?
- **mention event:** NO consumer exists; the mention feature is unimplemented end-to-end → **NOT PROVEN AS DEFECT** beyond the already-flagged scaffold residue (§6).
- **content.created:** no consumer exists; nothing in the Social business workflow reads it → **NOT PROVEN AS DEFECT** (absence is a gap, not a defect).
- **mute/unmute event:** mute does not need a notification by design (mute is silent suppression); no consumer requires it → **NOT PROVEN AS DEFECT**.
- **comment.liked / unlike events:** unlike is a private removal (no consumer workflow requires notifying on removal); comment-like event absence is a gap, not evidence of a broken chain → **NOT PROVEN AS DEFECT**.

#### D. Runtime proof
**PARTIALLY PROVEN** — full producer→outbox→worker→notification→push chain is statically wired for the 8 events that exist, but drain behavior was not observed against a live DB in this read-only session.

#### E. Verdict
**PROVEN** for the 8 implemented Social events; the mention/content-created/mute-like gaps are classified `NOT PROVEN AS DEFECT` (no current consumer workflow requires them), except mentions which are broken scaffold (§6).

---

# MASTER AUTHORITY TABLE

| Area | Durable Authority | Writer | Reader | Duplicate Authority | Runtime Proof | Verdict |
|---|---|---|---|---|---|---|
| CONTENT | `contents` | ContentRepositoryImpl (via ContentService only) | feed/search/detail/list/moderation | none | PARTIALLY PROVEN | PROVEN |
| FEED | canonical `contents` (direct query; no table) | (n/a read view) | FeedRepository.GetFeed → GET /feed → mobile | none | PARTIALLY PROVEN | PROVEN |
| COMMENTS | `comments` (+`comment_commerce_references`) | CommentRepositoryImpl (CommentService) | ListByTarget cursor list; count | mobile legacy `/social/comments` count targets removed route | PARTIALLY PROVEN | CONFLICT (client count contract break) |
| LIKES | `content_likes`, `comment_likes` | LikeRepositoryImpl + TargetLikeRepository (content_likes = 2 writers); TargetLikeRepository (comment_likes = 1) | CountLikes (content); CommentLikeService (comment); stats endpoint | duplicate content_likes writer; governance-bypassing stats | PARTIALLY PROVEN | CONFLICT |
| FOLLOW/BLOCK/MUTE | `user_follows`,`user_blocks`,`user_mutes` | SocialRepositoryImpl (via SocialService) | repo + feed + blockcheck + viewer-contexts | none | PARTIALLY PROVEN | PROVEN |
| MENTIONS | `content_mentioned_users` (orphan) | **none (residue)** | **none (residue)** | n/a | NOT PROVEN | CONFLICT (broken scaffold residue) |
| SHARE REFERENCE | `content_resource_occurrences` (immutable) | ContentRepositoryImpl.CreateResourceOccurrence | projection resolver / feed / search / mobile | legacy `contents.share_reference` dropped | PARTIALLY PROVEN | PROVEN |
| SOCIAL→NOTIFICATION | outbox→worker→notifications→push | social producers (8 events) | NotificationEventHandler | none (mention/content.created gaps) | PARTIALLY PROVEN | PROVEN |

# MASTER RESIDUE TABLE

| Path | Evidence | Reachability | Classification | Future Action |
|---|---|---|---|---|
| `content_mentioned_users` table | 000043; no production writer/reader | unreachable in production | RESIDUE | RE-AUDIT (prove whether feature is required; else DELETE migration) |
| `MentionedUserIDs` DTO field | decoded never used | dead value | RESIDUE | CLEANUP (or IMPLEMENT if feature required) |
| mobile `listComments` → `GET /social/comments` | page-based; no backend route | mobile-only, dangling | RESIDUE (client) | RE-AUDIT → PROVE/CLEANUP |
| mobile `getCommentCount` via legacy endpoint | reads `total` from absent route | dangling | RESIDUE (client defect candidate) | PROVE (confirm 404) → IMPLEMENT fix or re-point count |
| `allowComments` (mobile + backend DTO) | no schema column; always `true` | mobile create/entity | RESIDUE | CLEANUP (or implement if a real gate is wanted) |
| `CommentResponse.Type` (re-derived) | column dropped 000031 | additive wire field | ACTIVE NON-CANONICAL | CLEANUP (cosmetic) |
| 5 mention test files | reference nonexistent symbols | cannot compile | ZOMBIE | CLEANUP |
| `LikeRepositoryImpl` vs `TargetLikeRepository` (content_likes) | two writers, one table | both live | DUPLICATE AUTHORITY | RE-AUDIT → IMPLEMENT (collapse to one writer) |
| `GET /likes/stats` repo-bypass | reads repo directly, skips block-check | live | ACTIVE NON-CANONICAL | RE-AUDIT (governance) |
| mobile like-count 10s poll (`/likes/stats`) | non-optimistic; uncoupled | live | ACTIVE NON-CANONICAL | RE-AUDIT (UX/authority) |
| mute has no event; unlike no event | constants absent | n/a | RESIDUE (gap) | NOT PROVEN AS DEFECT |
| `contents.share_reference` | dropped 000041; rejected | none | ZOMBIE | DONE (cleanup already applied) |
| stale entity doc "no outbox events" | `user_follow.go:18` etc. contradict service | doc only | RESIDUE | CLEANUP |
| `chat_resource_authorizer_adapter.go:163 followed_id` | vs schema `following_id` | chat path | UNKNOWN severity | RE-AUDIT (parallel chat domain) |

# MASTER STATE-PROOF TABLE

| Claim | Static | State | Runtime | Verdict |
|---|---|---|---|---|
| `contents` single-writer, no competing state | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN |
| Feed reads canonical `contents`, no projection table | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN |
| Feed visibility/block filters can legitimately exclude rows | PROVEN | PARTIALLY PROVEN | NOT PROVEN | PARTIALLY PROVEN |
| empty-feed root cause == provider lifecycle | NOT PROVEN | NOT PROVEN | NOT PROVEN | NOT PROVEN |
| Comments cursor list end-to-end | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN |
| Mobile comment count path uses a removed route | PROVEN | PROVEN (route absent) | NOT PROVEN (no live call) | CONTRADICTED (contract break) |
| `content_likes` has exactly one writer | CONTRADICTED (two) | PROVEN | PARTIALLY PROVEN | CONFLICT |
| `comment_likes` single writer | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN |
| `comment.liked`/`content.mentioned` events exist | CONTRADICTED | CONTRADICTED | CONTRADICTED | CONTRADICTED |
| mentions schema is authoritative | CONTRADICTED (orphan) | CONTRADICTED | CONTRADICTED | CONTRADICTED |
| share authority is `contents.share_reference` | CONTRADICTED | CONTRADICTED | CONTRADICTED | CONTRADICTED |
| `content_resource_occurrences` is authoritative + immutable | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN |
| 8 social events producer→consumer wired | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN |

# PRIORITY

- **P0 (canonical correctness contradicted/broken):**
  - Mobile comment count → `GET /social/comments` route that does not exist. Comment counts are not served to clients reliably. (COMMENTS)
- **P1 (major state/lifecycle/runtime risk):**
  - `content_likes` two independent writers (`LikeRepositoryImpl` + `TargetLikeRepository`) — duplicate-writer risk with no single authority. (LIKES)
  - `content_mentioned_users` schema advertised but completely unreachable by production code — a broken/spurious authority, with five non-compiling tests encoding an unimplemented feature. (MENTIONS)
- **P2 (confirmed residue/duplicate/structural debt):**
  - `MentionedUserIDs` dead DTO field; `allowComments` decoration; re-derived `CommentResponse.Type`; legacy mobile `listComments` count path; 5 zombie mention tests; `content_likes` duplicate-repo structural debt; `GET /likes/stats` governance bypass; stale "no outbox events" entity comments.
- **P3 (hygiene/cosmetic):**
  - Non-optimistic mobile like polling cadence; movable/re-derivable wire fields. (Listed for completeness; not execution blockers.)

Note: "runtime not proven" is deliberately **NOT** placed into P1/P2 — the above P1/P2 items are concrete state/authority/residue facts, not runtime-evidence gaps.

# CRITICAL DISTINCTION — SEPARATION

- **(A) Implemented but runtime unproven:** CONTENT write path, FEED chain, comment cursor list, follow/block/mute, share occurrences, 8 notification events. These are wired; only live-DB observation is missing → PARTIALLY PROVEN, not broken.
- **(B) Architecturally broken:** none of the live Social chains is architecturally broken. The closest is the mobile comment-count → removed-route contract break, which is a client contract break, not backend architecture.
- **(C) Confirmed residue:** orphan `content_mentioned_users` schema, dead `MentionedUserIDs`, `allowComments`, legacy mobile `listComments` count, stale entity docs.
- **(D) Confirmed duplicate authority:** `content_likes` dual writers (`LikeRepositoryImpl` + `TargetLikeRepository`).
- **(E) Business requirement itself unproven:** the entire MENTIONS feature is unproven as a business requirement — no implementation, no consumer, no requirement trace from the HTTP DTO to any service. Kept separate from "implemented but unproven."

# FINAL EXECUTIVE SUMMARY

**1. WHAT IS ACTUALLY HEALTHY**
- `contents` single-writer authority via `ContentRepositoryImpl` behind `ContentService`; soft-delete; visibility enforced read/write; schema consistent with code.
- Feed is a correct direct-query over canonical `contents` (not a projection), cursor-based, properly reachable to mobile.
- Comment cursor create/list/delete path is sound and single-authority.
- Follow/block/mute have a single repository and matching routes; follow/block emit transactional outbox events.
- Share authority (`content_resource_occurrences`) is DB-immutable, exactly-one-source, and correctly replaces the dropped `contents.share_reference`.
- 8 Social events flow producer→outbox→worker→notifications→push with a retry queue.

**2. WHAT IS ACTUALLY BROKEN**
- Mobile comment count calls `GET /social/comments`, which no backend route serves — the count path is a client↔server contract break (not backend architecture).
- `content_likes` is written by two independent repository implementations — a confirmed duplicate-writer authority risk.

**3. WHAT IS CONFIRMED RESIDUE**
- Orphan `content_mentioned_users` schema, dead `MentionedUserIDs` DTO field, dead mobile legacy `listComments`/count path, `allowComments` decoration, re-derived `CommentResponse.Type`, stale "no outbox events" entity comments.
- Five mention test files reference production symbols that do not exist — they cannot compile and encode an unimplemented feature.

**4. WHAT HAS AUTHORITY CONFLICT**
- `content_likes`: two writers converged on one table.
- MENTIONS: the schema/HTTP-DTO contract vs the (absent) service implementation — the table is dead but advertised.

**5. WHAT IS ONLY A RUNTIME-PROOF GAP**
- CONTENT write/read, FEED, comment cursor list, follow/block/mute, share occurrences, and the 8 notification events are statically complete and single-authority; only live-DB observation is missing. These should NOT be treated as P1/P2.
- The historical empty-Feed cause is **NOT PROVEN** — do not attribute it to provider lifecycle without runtime evidence.

**6. WHAT WE SHOULD AUDIT NEXT**
- Re-audit the mobile comment-count path against the backend route set and confirm the 404 at runtime, then re-point it to the cursor endpoint's canonical count.
- Decide the business requirement for MENTIONS: if required, implement write→read→event→consumer; if not, drop `content_mentioned_users` and the test scaffolding.
- Collapse `content_likes` writes to one authoritative repository.
- Clarify whether `GET /likes/stats` may bypass service governance.

**7. WHAT MUST NOT BE TOUCHED BECAUSE OF PARALLEL WORK**
- Any Order / Payment / Coins / Discount / Refund / Dispute / Finance / Ledger / Payout internals. Where Social touches those domains (e.g. `content_resource_occurrences` fixed_price_sale/auction sources, `comment_commerce_references`, `OrderChatLinked` projection readers), they are reported only as **PARALLEL / PROVENANCE UNKNOWN** and are not part of this report's conclusions.
- The `chat_resource_authorizer_adapter.go:163` follow-column mismatch is a **parallel (chat) domain cross-reference**, flagged as UNKNOWN severity and requiring a chat-domain audit, not a Social change.
- `serverboot/dependencies.go` social wiring must change only under a coordinated, non-parallel change plan.