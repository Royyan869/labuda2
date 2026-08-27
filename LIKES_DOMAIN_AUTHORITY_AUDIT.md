# LIKES DOMAIN — DEEP AUTHORITY AUDIT

Read-only audit. No code, schema, test, config, DB, or Git changes made.
Scope: Likes only. Commerce/payment/refund/dispute/finance/ledger/payout untouched.

Type hierarchy: `content_likes` shares no path with commerce. Comment likes included because
they live in the same `like` module and share repositories/handler.

Every claim tagged: **PROVEN / PARTIALLY PROVEN / CONTRADICTED / UNRESOLVED / NOT PROVEN / BLOCKED**.

---

## 1. Executive verdict

- **Canonical Like identity: `(content_id, user_id)`** — composite PK on `content_likes`. **PROVEN**.
- **Canonical count: live `COUNT(*)` against `content_likes`/`comment_likes`.** No stored counter,
  no denormalized column anywhere. **PROVEN**.
- **Single authoritative write chain exists: HTTP `POST /api/v1/likes/toggle` → `LikeHandler.ToggleLike`
  → `LikeService.ToggleContentLike` → `LikeRepositoryImpl` (content) / `CommentLikeService` (comment).** **PROVEN**.
- **There are two competing LIKE repositories. Both reachable. Neither is "canonical" by declaration.**
  `LikeRepositoryImpl` is the content write authority (governed). `TargetLikeRepository` is the
  comment write authority + the *only* stats reader for both target types. Do not pick one; they are
  complementary, not alternatives. **PARTIALLY PROVEN** (reachability proven, "canonical" is a
  judgment call for an owner).
- **Production HTTP toggle gate is live and consistent:** `middleware.RequireActiveAccount`
  (active account + verified email). **PROVEN**.
- **Major defect:** like → unlike → like (within outbox retention) silently drops the second
  `content.liked` notification because the deterministic idempotency key is retained and insert is
  `ON CONFLICT DO NOTHING`. **CONTRADICTED** (idempotency-by-design vs notification re-delivery).
- **Broken wire contract:** mobile profile feed reads `engagement.likeCount` from
  `GET /users/:id/contents`, backend never populates it there → profile feed like counts are always 0.
  The integration test that locks this contract is RED against the current handler. **CONTRADICTED**.
- **No stored/denormalized like counter, no soft-delete on likes, no unlike-event, no comment-like
  notification, no admin like consumer, no recent-liker endpoint.**

---

## 2. Canonical Like state

| Fact | Value | Evidence | Class |
|---|---|---|---|
| Identity for content like | `(content_id, user_id)` | `content_likes` PK `content_likes_pkey (content_id, user_id)` | **PROVEN** |
| Identity for comment like | `(comment_id, user_id)` | `comment_likes` PK `comment_likes_pkey (comment_id, user_id)` | **PROVEN** |
| Like row shape | content_id, user_id, created_at | `000001_canonical_schema.up.sql:663-667` | **PROVEN** |
| No `id` column on like row | yes | schema; backend entity has no `id` | **PROVEN** |
| Entity representation | `Like{ContentID, UserID, CreatedAt}` | `entity/like.go:28-32` | **PROVEN** |
| TargetType vocabulary | `content`, `comment` only | `entity/like.go:18-23`, mobile `LikeTargetType {content, comment}` | **PROVEN** |
| Immutable-after-create | yes (no UPDATE of like rows anywhere) | repo queries only INSERT/DELETE/SELECT/COUNT | **PROVEN** |
| Mobile `Like` entity (with `id`) | unused — only declared | mobile `domain/entities/like.dart:12`; no construction site outside its own `copyWith` | **PROVEN** (zombie) |

---

## 3. Schema authority

Schema authority: **`backend/migrations/000001_canonical_schema.up.sql`**. **PROVEN** (single canonical
migration; no later migration alters either likes table — grep over all 85 migrations returns likes
only in 000001).

- `content_likes (content_id uuid NOT NULL, user_id uuid NOT NULL, created_at timestamptz DEFAULT now() NOT NULL)` — line 663.
- `comment_likes (comment_id uuid NOT NULL, user_id uuid NOT NULL, created_at ...)` — line 629.
- PKs: lines 1869/1873 (composite; these are the ONLY dedup guarantees).
- Indexes: `idx_content_likes_content_id`, `idx_content_likes_user_id`, comment equivalents — lines 2037-2046.
- FKs: content_id → `contents.id`, comment_id → `comments.id`, user_id → `users.id`, all **ON DELETE CASCADE** — lines 2306-2312.

**Unique-constraint / duplicate-prevention audit:**

| Layer | Mechanism | Class |
|---|---|---|
| DB content | PK `(content_id, user_id)` | **PROVEN** |
| DB comment | PK `(comment_id, user_id)` | **PROVEN** |
| App content insert | `ON CONFLICT (content_id, user_id) DO NOTHING` (`like_repository_impl.go:35`) | **PROVEN** |
| App comment insert | `ON CONFLICT (comment_id, user_id) DO NOTHING` (`like_repository.go:36`) | **PROVEN** |
| DB hard-delete | FK CASCADE when content/comment/user row hard-deleted | **PROVEN** |
| Soft-delete interaction | `contents` soft-delete (status='deleted') does NOT cascade; like rows survive (no purge on soft delete anywhere) | **PROVEN** |

No `like_count` column, no counter table, no trigger, no materialized projection exist anywhere.
**PROVEN** (repo-wide grep for `content_likes|comment_likes` outside repos/schema/seed/reset: none in
any SQL builder or projection).

---

## 4. Writer matrix

| # | Writer | Target | Governance | Outbox event | Reached from | Class |
|---|---|---|---|---|---|---|
| W1 | `LikeService.Like` → `LikeRepositoryImpl.InsertLike` | content | content-exists, status≠deleted, block check, outbox emit | `content.liked` | `LikeService.Like`; reachable via `ContentService.LikeContent`, plus `cmd/seed` | **PROVEN** |
| W2 | `LikeService.Unlike` → `LikeRepositoryImpl.DeleteLike` | content | none (idempotent delete) | none | `ContentService.UnlikeContent`, `ToggleContentLike` (liked branch) | **PROVEN** |
| W3 | `LikeService.ToggleContentLike` → `ExistsLike` + `InsertLike`/`DeleteLike` | content | W1 validation on like path; skip validation on unlike path | `content.liked` on like path | `LikeHandler.ToggleLike` (target_type=content) — **production HTTP writer** | **PROVEN** |
| W4 | `CommentLikeService.ToggleCommentLike` → `TargetLikeRepository` | comment | comment exists + not soft-deleted, parent content exists + not deleted, block checks vs comment author + content author | **none** | `LikeHandler.ToggleLike` (target_type=comment) — **production HTTP writer** | **PROVEN** |
| W5 | `ContentService.LikeContent` / `UnlikeContent` | content | delegates to `LikeService` | via LikeService | **ZERO callers** in backend | **PROVEN (zombie)** |
| W6 | `TargetLikeRepository.InsertLike/DeleteLike/ExistsLike` (generic path) | content AND comment | none (raw SQL) | none | `InsertLike/DeleteLike` reached only for comments via W4; `ExistsLike/CountLikes` reached for BOTH via `LikeHandler.GetLikeStats` | **PARTIALLY PROVEN** (write paths unused for content; read paths live for both) |
| W7 | `LikeRepositoryImpl` (all 4 methods) | content | read-only surface reused by `ContentService` reads | — | used by content like writes + `ContentService.GetLikeCount`/`IsLiked` | **PROVEN** |
| W8 | `cmd/seed` → `likeService.Like` | content | nil blockChecker/nil invariantLogger | emits events (dev only) | seed binary | **PROVEN (dev)** |
| W9 | `cmd/dev-reset-data` / seed truncate | both | destructive dev reset of `content_likes`,`comment_likes` | — | dev tooling | **PROVEN (dev)** |

**Which writer is authoritative?**
Trace from wire: toggle → handler → services (W3/W4). `ContentService.LikeContent/UnlikeContent` (W5)
and the raw `TargetLikeRepository` content-write path (W6) are unreachable dead weight — do NOT delete.
**Authoritative content writer = W3. Authoritative comment writer = W4.** PROVEN by caller trace.

No admin writer, no worker writer, no projection writer exist. **PROVEN.**

---

## 5. Reader matrix

| # | Reader | Surface | Reads | Data source | Reachable | Class |
|---|---|---|---|---|---|---|
| R1 | `ContentHandler.GetContent` (`GET /contents/:id`) | content detail + `engagement.likeCount` + `is_liked` | live count via `ContentService.GetLikeCount`→`LikeRepositoryImpl.CountLikes`; liked via `IsLiked`→`ExistsLike` | `content_likes` | YES, public browse | **PROVEN** |
| R2 | `LikeHandler.GetLikeStats` (`GET /likes/stats`) | `{target_id,target_type,count,is_liked}` for content or comment | `TargetLikeRepository.CountLikes`/`ExistsLike` | `content_likes`/`comment_likes` | YES, public browse (viewer-optional is_liked) | **PROVEN** |
| R3 | Feed (`GET /feed`) | **no like fields at all** | — | — | reader exists, like-absent | **PROVEN** |
| R4 | Profile content list (`GetUserContent`) | DTO has `engagement` + `is_liked` fields but **never populated** | — | — | reader live, fields dead | **PROVEN** |
| R5 | Search content (`GET /search/content`) | **no like fields** | — | — | reader exists, like-absent | **PROVEN** |
| R6 | Comments list (`GET /contents/:id/comments`) | **no like fields** | — | — | reader exists, like-absent | **PROVEN** |
| R7 | Notifications consumer | consumes `content.liked`, inserts in-app/push notification | reads event payload (actor/recipient/content), not `content_likes` | outbox → notifications | YES, worker live (serverboot `SetupNotificationHandlers`) | **PROVEN** |
| R8 | Counters / analytics / admin | **none** | — | — | absent | **PROVEN** |
| R9 | Mobile poller `watchLikeStats` (10s) | `totalLikes`, `isLikedByCurrentUser` from R2 | — | — | live | **PROVEN** |

Feed and search give zero like visibility; like counts only exist on content detail + `/likes/stats`.

---

## 6. Count authority matrix

| Fact | Authority | Class |
|---|---|---|
| Content count source | live `SELECT COUNT(*) FROM content_likes WHERE content_id=$1` — two independent impls (`LikeRepositoryImpl.CountLikes`, `TargetLikeRepository.CountLikes`), same table, same result | **PARTIALLY PROVEN** (single truth source, duplicated code path) |
| Comment count source | live `COUNT(*)` via `TargetLikeRepository.CountLikes` on `comment_likes` | **PROVEN** |
| Stored counter | does not exist | **PROVEN** |
| Denormalized field | does not exist in any table (contents has no like_count) | **PROVEN** |
| Competing facts | none (both impls read same table; no cache, no read-model) | **PROVEN** |
| Count on soft-deleted content | no surface reads it (detail hides deleted; stats endpoint does NOT filter — a deleted content still returns its count via `/likes/stats` and `is_liked` if authed) | **UNRESOLVED** (semantic: is exposing counts for deleted/hidden content intended?) |
| Count on hidden/private content | same leak surface | **UNRESOLVED** |

`GET /likes/stats` performs **no** visibility/status/block checks: counts and `is_liked` for any known
target id. Verdict: leak-adjacent read surface. NOT fixed here.

---

## 7. Like/unlike lifecycle (soft-delete vs hard-delete)

- Like row lifecycle = **INSERT (hard) → DELETE (hard)**. No soft-delete, no `deleted_at`, no like
  "virtual deletion". **PROVEN** (schema columns + repo SQL).
- Content soft-delete (status='deleted', `content.go:91`) is checked by `LikeService` on every like
  path (`like_service.go:87,182`) — rejected with `ErrContentDeleted`. **PROVEN**.
- `IsHidden` (private content, `content.go:108`; moderation restore keeps hidden) is **NOT** checked by
  the like service → hidden/private content is likeable by anyone holding the id. Detail hides it for
  non-admin, but toggle does not. **CONTRADICTED** (visibility doctrine vs like guard).
- Unlike skips validation **intentionally** (documented `like_service.go:155-157`, `comment_like_service.go:66-67`)
  so a user can remove a like after content is deleted/blocked. **PROVEN**.
- No purge of likes on content soft-delete; FKs only cascade on hard delete. **PROVEN**.
- Block check on like path: treated as `ErrContentNotFound` (no block-info leak). **PROVEN**.
- Self-like allowed; no self-like guard (block check skipped for own content). **PROVEN**; self-notification
  suppressed at consumer level (`notification_worker_social.go:72`). **PROVEN**.

---

## 8. Idempotency matrix

| Operation | Requirement | Enforcement | Class |
|---|---|---|---|
| Content like (W1/W3) | duplicate-safe | PK + `ON CONFLICT (content_id,user_id) DO NOTHING` at repo; toggle reads `ExistsLike` first | **PROVEN** |
| Content unlike | idempotent delete | `DELETE ... WHERE content_id=$1 AND user_id=$2` returns nil regardless of row | **PROVEN** |
| Comment like/unlike (W4) | duplicate-safe | PK + `ON CONFLICT DO NOTHING` + `ExistsLike` toggle | **PROVEN** |
| Outbox dedup | one `content.liked` per (content,actor) until event lifecycle frees the key | UNIQUE `outbox.idempotency_key` (migration :1979) + `ON CONFLICT DO NOTHING` (`outbox_repository.go:187`) | **PROVEN** |
| Outbox key format | `content.liked.{contentID}.{userID}` (service) → stored as `content.liked.` + that = **double prefix** `content.liked.content.liked.{contentID}.{userID}` | deterministic; test-locked (`like_service_test.go:585-618`) | **PROVEN** |
| Re-like after unlike | **should re-notify** | **NOT satisfied**: unlike emits no event and the succeeded outbox row keeps its UNIQUE key until archival; re-like insert hits `ON CONFLICT DO NOTHING` → silent no-op → author never re-notified | **CONTRADICTED** |
| Concurrent toggle (two devices) | sane net state | no row lock on `content_likes`; race resolved by PK conflict + outbox key dedup; count stays consistent, exactly one event | **PARTIALLY PROVEN** |
| Comment like event | none exists; no key needed | n/a | **PROVEN** |

---

## 9. Event / outbox matrix

| Event | Emitter | Payload | Consumer | Idempotency key | Class |
|---|---|---|---|---|---|
| `content.liked` | `LikeService.Like` + like-branch of `ToggleContentLike` | `{actor_id, recipient_id, content_id}` | `NotificationEventHandler.handleContentLiked` (`notification_worker_social.go:50`), registered (`notification_worker.go:621`), routed (`notification_worker.go:190`); self-notification suppressed; policy category Social | `content.liked.{contentID}.{userID}` (+type prefix) | **PROVEN** |
| `content.unliked` | **does not exist** (no constant, no emitter) | — | none | — | **PROVEN** |
| `comment.liked` | **does not exist** — comment likes produce NO notification | — | none | — | **PROVEN** |
| `comment.unliked` | does not exist | — | — | — | **PROVEN** |
| Stale notification on unlike | unlike does not remove prior in-app like notification | — | — | — | **UNRESOLVED** (likely intended: no scrub path) |

Event registry includes `content.liked` (`outbox_event_registry_test.go:173`). Worker is started
(`serverboot/dependencies.go:1274`) and handlers registered (:1347). **PROVEN.**

---

## 10. Mobile/API contract matrix

| Contract | Mobile expects | Backend sends | Verdict | Class |
|---|---|---|---|---|
| Toggle `POST /api/v1/likes/toggle` | body `{target_id, target_type∈[content,comment]}`; reads `data.liked` | body matches (`like_handler.go:46-49`); response `SuccessWithMessage` → `data.liked` + `data.count` | **MATCH** | **PROVEN** |
| Stats `GET /api/v1/likes/stats` | query `{target_id,target_type}`; DTO `{target_id,target_type,count,is_liked}` | response `LikeStatsResponse` with exactly those 4 keys | **MATCH** | **PROVEN** |
| Content detail `GET /contents/:id` | `engagement.likeCount`, `engagement.commentCount`, optional `is_liked` | populated live (`content_handler.go:1012-1015`); camelCase locked by `content_engagement_c7c_test.go` | **MATCH** | **PROVEN** |
| Profile `GET /users/:id/contents` | per-item `engagement.likeCount` + `comments` + `is_liked` (mobile `profile_feed_tab.dart:104-105`, `content_dto`) | **never populated** (`GetUserContent` `content_handler.go:1139-1173`); engagement omitted (omitempty) → mobile renders 0 | **MISMATCH — always 0 likes on profile feed** | **CONTRADICTED** |
| Create/Update/Repost content | `engagement.likeCount` | emitted as **zero-value** Engagement (`content_handler.go:642,798,1297`) | **MATCH** (new content has 0 likes) | **PROVEN** |
| Recent likers | `LikeStats.recentLikerUserIds` | **no endpoint/field**; mapper hardcodes `const []` (`like_mapper.dart:19`); `like_recent_avatars` widget renders nothing | **MISMATCH — dead UI feeding empty data** | **PROVEN** |
| Mobile "list users who liked" / "my like history" | documented in datasource docstring | **not implemented** (only toggle+stats in datasource) | dead aspiration | **PROVEN** |

Auth expectations: toggle gated by active-account + verified-email (403 `EMAIL_VERIFICATION_REQUIRED`),
which mobile treats as blocked-action. **PROVEN.**

---

## 11. Admin consumer matrix

| Surface | Like read/write | Class |
|---|---|---|
| Admin API/panel (`apps/admin`) | **zero** like references | **PROVEN** |
| Moderation worker | may soft-delete content/comment; likes survive (no cascade, no purge, no recount) | **PROVEN** |
| governance-constitution D10 | lists "likes" for future SQL-only visibility projection | **NOT PROVEN / BLOCKED** (approved in docs, not implemented) |

---

## 12. Runtime proof status

| Layer | Proof level | Evidence | Class |
|---|---|---|---|
| Like service logic | unit-tested w/ mocks (Like, Unlike, Toggle, idempotency, outbox key, block, deleted, no-outbox-on-unlike) | `like_service_test.go` (17 tests), `comment_like_service_test.go` | **PARTIALLY PROVEN** (no DB-backed repo test) |
| Repository SQL against real schema | **no direct DB test** for either repository; only service-level mocks + integration-gated handler tests | no `like_repository_impl_test.go` | **NOT PROVEN** |
| Detail engagement contract | non-gated shape test only; live-count hydration only in gated integration tests | `content_engagement_c7c_test.go` (shape), `content_visibility_authority_integration_test.go` (`//go:build integration`) | **PARTIALLY PROVEN** |
| Profile engagement contract | asserted by **integration-gated** test but **not implemented in handler** | `content_visibility_authority_integration_test.go:270` vs `content_handler.go:1042+` | **CONTRADICTED** (test RED against current handler) |
| Worker consumer | unit/integration-tested; worker wired + started | `notification_worker*_test.go`; `serverboot/dependencies.go` | **PROVEN** (static); live-delivery **NOT PROVEN** |
| Count correctness at runtime scale | live COUNT(*) only; no index on (content_id) additionally? — yes `idx_content_likes_content_id` exists | schema :2045 | **PROVEN** for correctness; performance envelope **NOT PROVEN** |

---

## 13. Contradiction matrix

| # | Contradiction | Evidence | Class |
|---|---|---|---|
| C1 | Like toggle allows liking **hidden/private** (`IsHidden`) content; detail hides it. Like service checks only `StatusDeleted`. | `like_service.go:87,182` vs `content.go:104-108` + trickle `UpdateContent` private→hidden | **CONTRADICTED** |
| C2 | Re-like after unlike emits no `content.liked` (idempotency key retained). Doc claims "outbox event for notification" per like. | `outbox_repository.go:187` + `like_service.go:119` + no unlike event | **CONTRADICTED** |
| C3 | Profile feed wire promises `engagement.likeCount`/`is_liked` (mobile consumes; integration test asserts) but handler never sets them. | mobile `profile_feed_tab.dart:104` vs `content_handler.go:1042-1174` | **CONTRADICTED** |
| C4 | Two like repositories both claim content-likes authority; content write path in `TargetLikeRepository` is unreachable for content while `LikeRepositoryImpl` carries all content writes/stats reads; stats reads duplicated. | grep of all callers | **CONTRADICTED**-as-design (duplicate implementations; resolved only by caller trace, not by code intent) |
| C5 | Entity comment "Uses ON CONFLICT ... for idempotent behavior" while `ToggleContentLike` outbox re-emit is non-idempotent across unlike cycles. | docstring vs C2 | **CONTRADICTED** |
| C6 | `GetLikeStats` leaks counts/is_liked for deleted/hidden content (no governance guards), contradicting visibility doctrine on all other social read surfaces. | `like_handler.go:212-228` (no status/visibility/block check) | **UNRESOLVED** (behaviorally contradicted unless intended) |

Never silently resolved — owner decision required.

---

## 14. Legacy / zombie candidates — DO NOT DELETE

Proven dead by caller/consumer absence:

| Candidate | Why zombie | Proof |
|---|---|---|
| `ContentService.LikeContent` / `UnlikeContent` | zero callers across backend | grep (only definitions) |
| `like.DuplicateLikeViolation` (like pkg) | zero callers | grep |
| `content_app.DuplicateLikeViolation` | zero callers | grep |
| `TargetLikeRepository.InsertLike/DeleteLike/ExistsLike/CountLikes` content-type write path | insert/delete reached only for comments; content branch reaches stats only | caller trace (W6) |
| Mobile `Like` entity + `share_dto` `likeCount: 0` hardcode | entity unused; share DTO fakes zero | mobile grep |
| Mobile "recent liker avatars" widget | feeds on `recentLikerUserIds` that is always `[]` | `like_mapper.dart:19`, widget files |
| Mobile datasource docstrings: "list users who liked", "user's like history" | methods not implemented | `like_api_datasource.dart` |
| `scripts/query_db_state.go` likes introspection (`ILIKE '%likes%'` + `tablename ILIKE '%likes%'`) | dev tooling | file |

None deleted. None cleaned.

---

## 15. Protected boundaries

- Content `201`/`200` creation surfaces: engagement zero is wired and tested — do not treat as bug.
- `ON CONFLICT DO NOTHING` at row level for content/comment likes: safe, keep.
- `ON DELETE CASCADE` FKs: keep.
- Commerce/payment/refund/dispute/finance/ledger/payout modules: **not audited, not touched**.
- Worker/outbox internals (archive, stuck recovery, retry): not modified.

---

## 16. Owner decisions required

1. **C3** — should `GET /users/:id/contents` populate `engagement` (like/comment counts) + `is_liked` per item, or should mobile drop that read? (Integration test already asserts populate.)
2. **C2** — desired semantics for re-like notifications: re-emit, suppress permanently, or add an unlike-event to scrub then allow re-emit? Decide key lifecycle (e.g., scope key to an epoch or delete/rotate on unlike).
3. **C1** — should like path validate `IsHidden`/visibility (like `ErrContentNotFound`), or is id-only like acceptable?
4. **C6** — should `GET /likes/stats` hide counts for deleted/hidden content (and gate `is_liked` on block)?
5. Leave one canonical content-like repo interface or keep dual-repo? (Content counts are currently duplicated across two impls.)
6. Comment-like notifications: intentionally absent or a gap?

---

## 17. What is PROVEN

- Canonical Like identity `(content_id, user_id)` (+ `(comment_id, user_id)`); PK-enforced, dual `ON CONFLICT` defense.
- Canonical count = live `COUNT(*)`; **single storage fact**, zero stored/denormalized counters.
- Authoritative production writers: `LikeService.ToggleContentLike` (content), `CommentLikeService` (comment), both behind `POST /likes/toggle` with active+verified gate.
- No admin, worker, or projection like writer/reader exists.
- `content.liked` event: emitter + registered consumer + policy category + tests.
- Feed, search, comments-list carry **no** like fields.
- Mobile toggle + stats + detail contracts match backend byte-for-byte (keys, types, envelope `data.*`).
- Dead-code set in §14 is caller-proven absent.

## 18. What is BROKEN

- **C3** — profile content feed like counts always `0` on mobile; integration contract test RED.
- **C2** — like→unlike→like silently loses the second notification.
- **C1/C6** — like/read surfaces reachable for hidden/private/deleted content despite visibility doctrine elsewhere.

## 19. What is NOT PROVEN

- Repository SQL against a real DB (no direct repo tests).
- Live end-to-end delivery of `content.liked` → in-app/push in a running stack (only static wiring + unit tests).
- Performance of `COUNT(*)` at scale (index exists; behavior under high cardinality unmeasured).
- Concurrent double-toggle outcome in production (no row lock; only PK/outbox-key dedup).

## 20. Recommended next implementation pass (NOT executed)

1. Implement C3 decision (populate engagement in `GetUserContent`, or update mobile + delete RED test).
2. Fix C2 idempotency-key lifecycle for re-like notifications.
3. Add `IsHidden`/visibility + content-status guard parity between like path and read surfaces (C1/C6).
4. Add direct DB-backed repository tests for both like repositories against current schema.
5. Collapse dual count implementations to one authority per target type (decision 5) or explicitly document the two as content-write vs stats-read layering.
6. Purge §14 zombies only after owners sign off (this audit is read-only).
7. Decide comment-like notification semantics (decision 6).