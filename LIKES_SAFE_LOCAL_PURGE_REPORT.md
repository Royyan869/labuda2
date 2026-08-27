# LIKES DOMAIN — SAFE LOCAL PURGE REPORT

Forward-only convergence. Current filesystem = only implementation truth. No Git history used as authority.
No rollback/restore/checkout/cherry-pick. No commerce/transaction domain touched. No global cleanup.

This pass = SAFE DEAD-CODE PURGE of the LIKES scope + regression check. Forward potential.
Standard applied: PROVEN DEAD → PURGE; NOT PROVEN DEAD → KEEP.

---

## A. PURGE VERDICT

7 candidates from `LIKES_DOMAIN_AUTHORITY_AUDIT.md` were independently re-verified from the current filesystem.

| # | Candidate | Verdict |
|---|---|---|
| 1 | `ContentService.LikeContent` + `ContentService.UnlikeContent` (+ now-dead `likeService` injection) | **PROVEN DEAD → PURGED** |
| 2 | `DuplicateLikeViolation` (like/application) | **PROVEN DEAD → PURGED** |
| 3 | `DuplicateLikeViolation` (content/application) | **PROVEN DEAD → PURGED** |
| 4 | `TargetLikeRepository` generic content-write path | **NOT DEAD (inseparable) → KEPT** |
| 5 | Mobile `Like` entity | **PROVEN DEAD → PURGED** |
| 6 | Recent-liker UI/path (`LikeRecentAvatars`, `recentLikerUserIds`, mapper hardcode) | **PROVEN DEAD → PURGED** |
| 7 | Hardcoded `likeCount: 0` residue (`ShareDto.toPostJson`) | **PROVEN DEAD → PURGED** |

Canonical Likes authority (identity, content writer, comment writer, stats reader, routes,
notification/event path, repository interfaces, unresolved visibility/idempotency semantics) — **all intact**.

---

## B. Per-candidate evidence + action

### Candidate 1 — `ContentService.LikeContent` / `ContentService.UnlikeContent` — PROVEN DEAD → PURGED

- Definition: `backend/internal/social/content/application/content_service.go` (both methods, plus field `likeService *likeapp.Service`, constructor param, assignment).
- References/callers: grep `LikeContent|UnlikeContent` across `backend/` → only the two definitions. **Zero callers, zero test callers.**
- DI registration: none. Route exposure: none (only `/likes/toggle` via `LikeHandler`). Wire: `LikeHandler` gets `likeService` directly; `ContentService` did not need it after method removal.
- Interface conformance: no interface requires these methods (grep found no receiver-interface use).
- Serialization/generated/migration/reflection dependency: none.
- Active replacement: the canonical content writer is `LikeService.ToggleContentLike` (unchanged); seed already called `likeService.Like` directly, never these wrappers.
- Action: removed the two methods, the now-unused `likeService` field/constructor-param/assignment, and the now-unused `likeapp` import. Updated all 12 `NewContentService(...)` call sites (prod seed + serverboot deps + 10 integration/unit test fixtures) by dropping the 3rd arg.
- Verification: `go build ./...` + `go vet ./cmd/seed ./internal/serverboot ./internal/social/...` clean. Like/content tests pass.

### Candidate 2 — `DuplicateLikeViolation` (like/application) — PROVEN DEAD → PURGED

- Definition: `internal/social/like/application/invariant_logger.go`.
- References: grep `DuplicateLikeViolation` → definition only. **Zero production + zero test callers.** Its lone consumer would be a `DuplicateLikeAttempt` guard that the toggle path deliberately does NOT implement (toggle reads `ExistsLike` and unlikes instead).
- Imports: removing the function orphans nothing (file's `LikeOnDeletedContentViolation`, `LogInvariantViolation`, richer helpers remain live).
- Constant `DuplicateLikeAttempt` left in place (exported invariant vocabulary; shared name with content package).
- Action: removed the function.

### Candidate 3 — `DuplicateLikeViolation` (content/application) — PROVEN DEAD → PURGED

- Definition: `internal/social/content/application/invariant_logger.go`.
- References: definition only; zero callers (grep). `DuplicateLikeAttempt` const kept — it is still referenced by that file's `InvariantType.String()` switch, so it is NOT dead.
- Action: removed the function. No import changes needed.

### Candidate 4 — `TargetLikeRepository` generic content-write path — NOT DEAD (BLOCKED) → KEPT

- Independent re-verification:
  - `InsertLike`/`DeleteLike` are reached ONLY through `CommentLikeService.ToggleCommentLike` (via `CommentLikeRepository` interface) with `TargetTypeComment` hardcoded (`comment_like_service.go`). **LIVE, canonical comment writer.**
  - `CountLikes`/`ExistsLike` are reached through `LikeHandler.GetLikeStats` for BOTH `content` and `comment` target types (`like_handler.go:214,223`). **LIVE, canonical stats reader.**
  - `getTableMapping` `TargetTypeContent` case is not separable from the shared method; removing the content branch would be a behavior-neutral refactor of a LIVE file, and the method cannot be removed without breaking comment writes.
- Verdict: the "generic content-write path" is not a removable symbol — it is the same method serving the canonical comment writer. **NOT DEAD → KEEP.**
- No change made.

### Candidate 5 — Mobile `Like` entity — PROVEN DEAD → PURGED

- Definition: `apps/mobile/lib/domains/social/like/domain/entities/like.dart` `class Like {...}` (id/userId/targetId/targetType/createdAt/metadata + copyWith/==/hashCode/toString).
- References: across `apps/mobile` (lib + test), `Like(` constructor and the `Like` type are referenced ONLY inside its own file (`copyWith` self-call, `toString`). No lib consumer, no test consumer. `LikeStats` and `LikeTargetType` (both live) declared in the same file.
- Wire/serialization/generated dependency: none. The mobile app's active like surface consumes only `LikeStats` + `LikeTargetType` (verified in `content_like_action.dart`, `content_detail_screen.dart`, `comment_card.dart`, handlers, datasource).
- Action: removed `class Like` only. Kept `LikeTargetType` enum + `LikeStats`.

### Candidate 6 — Recent-liker UI/path — PROVEN DEAD → PURGED

- Definitions:
  - `presentation/widgets/like_recent_avatars.dart` (`LikeRecentAvatars` widget).
  - `LikeStats.recentLikerUserIds` field + `copyWith` param (entity).
  - `like_mapper.dart` hardcode `recentLikerUserIds: const [], // Not provided by API`.
  - `like_count_widget.dart` recent-avatars branch.
  - `like.dart` barrel export of `like_recent_avatars.dart`.
- References: `recentLikerUserIds` read only by `like_count_widget.dart`/`LikeRecentAvatars` (both in the dead ring). `LikeStats.==`/`hashCode`/`toString` already IGNORE `recentLikerUserIds` → removing the field cannot change equality/identity semantics. No backend field/endpoint supplies it (no endpoint, never emitted).
- Construction sites: 5 home test fixtures passed `recentLikerUserIds: const []` (10 lines) — updated to compile against the removed field.
- Action: deleted `like_recent_avatars.dart`; removed the field from `LikeStats` + `copyWith`; removed mapper hardcode line; removed the widget branch + import in `like_count_widget.dart`; removed the barrel export; updated the 5 test files (10 fixture lines).
- Kept: `LikeCountWidget`, `LikeCountStates`, `LikeCountStyleUtils`, and the `LikeCountStyle.withRecent` enum variant (not listed as candidates; style knob only, harmless).

### Candidate 7 — Hardcoded `likeCount: 0` residue — PROVEN DEAD → PURGED

- Location: `apps/mobile/lib/domains/social/share/data/dto/share_dto.dart` `ShareDto.toPostJson(...)` — Firestore share-post builder containing `'engagement': { 'likeCount': 0, ... }`.
- References: `toPostJson` has ZERO callers in lib or test (grep). The active share flow posts through the Go backend (`/contents`, `share_api_datasource.dart`); this Firestore legacy writer is dead.
- Action: removed the entire `toPostJson` method. No imports orphaned (`ShareDto`, `ShareMapper`, `ShareTarget`, `ExternalShareType` remain in use).

---

## C. Exact files changed

Backend (11):
1. `backend/internal/social/content/application/content_service.go`
2. `backend/internal/social/content/application/invariant_logger.go`
3. `backend/internal/social/like/application/invariant_logger.go`
4. `backend/cmd/seed/main.go`
5. `backend/internal/serverboot/dependencies.go`
6. `backend/internal/social/content/delivery/http/content_visibility_authority_integration_test.go`
7. `backend/internal/social/content/delivery/http/comment_wire_contract_integration_test.go`
8. `backend/internal/social/content/delivery/http/comment_query_count_test.go`
9. `backend/internal/social/content/delivery/http/comment_list_integration_test.go`
10. `backend/internal/social/content/application/content_visibility_authority_integration_test.go`
11. `backend/internal/social/content/application/content_resource_occurrence_integration_test.go`

Mobile (11):
12. `apps/mobile/lib/domains/social/like/domain/entities/like.dart`
13. `apps/mobile/lib/domains/social/like/data/mappers/like_mapper.dart`
14. `apps/mobile/lib/domains/social/like/presentation/widgets/like_count_widget.dart`
15. `apps/mobile/lib/domains/social/like/presentation/widgets/like_recent_avatars.dart` — **DELETED**
16. `apps/mobile/lib/domains/social/like/like.dart`
17. `apps/mobile/lib/domains/social/share/data/dto/share_dto.dart`
18. `apps/mobile/test/features/home/presentation/screens/home_screen_promoted_card_rendering_test.dart`
19. `apps/mobile/test/features/home/presentation/screens/home_screen_cross_boundary_pipeline_test.dart`
20. `apps/mobile/test/features/home/presentation/root_wiring/feed_root_wiring_test.dart`
21. `apps/mobile/test/features/home/presentation/providers/feed_promoted_impression_authority_test.dart`
22. `apps/mobile/test/features/home/presentation/providers/feed_promoted_click_destination_authority_test.dart`

(File `LIKES_DOMAIN_AUTHORITY_AUDIT.md` is the source audit deliverable, unchanged in this pass.)

---

## D. Tests executed and results

### Backend
| Command | Result |
|---|---|
| `go build ./...` (full backend) | **PASS** |
| `go vet ./cmd/seed ./internal/serverboot ./internal/social/...` | **PASS** |
| `go test ./internal/social/like/...` | **PASS** (like/application ok; others no test files) |
| `go test ./internal/social/content/application/...` | **PASS** |
| `go test ./internal/social/content/delivery/http/... -run "TestEngagementResponse|TestContentResponse_Engagement"` | **PASS** (C7C engagement contract intact) |
| `go test ./internal/social/graph/... ./internal/social/feed/...` | **PASS** |
| `go vet -tags integration ./internal/social/content/delivery/http/...` | **PASS** (my edited integration-gated files compile) |
| `go test ./internal/social/content/delivery/http/...` (full pkg) | 2 PRE-EXISTING failures, **independent of this purge** (see E) |
| `go test ./internal/serverboot/...` | 2 PRE-EXISTING failures, **independent of this purge** (see E) |
| `go test ./internal/worker/...` | PRE-EXISTING [build failed] on `content_mentioned_notification_matrix_test.go:76 undefined: events.EventContentMentioned` (see E) |

### Mobile (Flutter)
| Command | Result |
|---|---|
| `flutter analyze lib/domains/social/like lib/domains/social/share/data/dto/share_dto.dart` | **PASS — No issues found** |
| `flutter analyze .../content_detail_screen.dart .../comment_card.dart .../content_like_handlers.dart .../comment_like_handlers.dart` | **PASS — No issues found** (all live `LikeStats`/`LikeTargetType` consumers) |
| `flutter analyze` on edited home test files | only PRE-EXISTING errors (`ApiClient.testing`, `FeedItem(author:)`, `CommerceMarketplaceCardShell`…) — **no like-domain errors introduced** |
| `flutter test` of affected home/like tests | **BLOCKED** by PRE-EXISTING compile breaks in these test files (home feed refactor drift) and in `content/presentation` (`content_like_action.dart` ↔ `content_like_handlers.dart` signature mismatch). None reference symbols purged by this pass. |

---

## E. Remaining Likes contradictions (unchanged, out of scope per instruction #8)

1. Profile contents missing `engagement`/`is_liked` (`GetUserContent` vs mobile `profile_feed_tab.dart`; integration test RED). NOT TOUCHED.
2. like → unlike → like notification dedup/idempotency (outbox key retention). NOT TOUCHED.
3. Hidden/deleted content visibility enforcement on like path + `/likes/stats` leak. NOT TOUCHED.

All three intentionally deferred per instructions.

---

## F. Candidates intentionally NOT removed and why

| Item | Reason |
|---|---|
| `TargetLikeRepository` content-write path | NOT DEAD — same methods serve the canonical comment writer + both-target stats reader; content branch is inseparable. KEEP. |
| `DuplicateLikeAttempt` const (both packages) | Exported invariant vocabulary; content copy still referenced by `InvariantType.String()`. Removing an exported symbol for "unused-looking" reason violates the KEEP-over-DELETE rule. |
| `LikeCountWidget`, `LikeCountStates`, `LikeCountStyleUtils`, `LikeCountStyle.withRecent` | Not purge candidates; kept as style/UI scaffolding. Recent-liker ring (the actual candidate) removed from within. |
| Two stale mention integration tests (`content_mention_proof_*`, `content_mention_outbox_atomic_*`) | Pre-existing arity/符号 breakage (`NewContentService` 7 args, `AddMentionedUserIDs`/`EventContentMentioned` undefined). OUTSIDE Likes scope; not part of any candidate. Not my purge's concern. |

---

## G. Commerce/transaction boundaries — CONFIRMED CLEAN

Files touched in this purge are confined to: `internal/social/content/application`, `internal/social/like/*`,
`cmd/seed`, `internal/serverboot/dependencies.go` (only the `NewContentService` arg list), and mobile
`domains/social/like/*`, `domains/social/share/data/dto/share_dto.dart` (Firestore share DTO only), plus 5 home
test fixtures.

No change to: refund, dispute, payment, order, coins, finance, ledger, payout, wallet, subscription, escrow,
midtrans, or any commerce/transaction module's logic, schema, routes, or tests. The only commerce-adjacent
file in the git diff is `share_dto.dart`, which is the mobile share (social) Firestore DTO — not a commerce writer.

## H. Global cleanup — CONFIRMED NOT PERFORMED

No migrations touched. No schema changed. No routes added/removed/changed. No repository interfaces changed.
No outbox/worker behavior changed. No formatting/line-ending normalization performed (working tree remains
CRLF as found; `gofmt -l` flagging is line-ending pre-existing, deliberately not "fixed").
The pre-existing, large uncommitted working-tree drift in other scopes (order/rating/refund/comment/home-feed)
was left exactly as found. This pass touched ONLY the 22 Likes-scope files above.

---

## Final status

- Backend production build + vet: clean.
- Backend likes + content application tests: pass.
- Backend failures observed are all PRE-EXISTING and reference symbols/files untouched by this purge
  (`content_handler.go` source-grep tests, `CommentHandler.contentService`/`sellerCapabilityChecker` wiring
  assertions, `events.EventContentMentioned` worker test-build break). No purge-exposed active dependency
  found → no STOP condition triggered.
- Mobile like-domain + share DTO + all live like consumers: analyzer clean.
- Mobile affected tests: blocked by pre-existing workspace compile breaks unrelated to the purged symbols;
  re-verify within the home-feed/rating convergence pass that owns that drift.
- Canonical Likes authority fully intact: identity `(content_id,user_id)`, `LikeService.ToggleContentLike`
  (content), `CommentLikeService` (comment), `LikeHandler.GetLikeStats` (stats), routes, `content.liked`
  event + consumer, repository interfaces, unresolved visibility/idempotency semantics.

LIKES domain is NOT declared fully PROVEN. This is a safe purge + regression check only.