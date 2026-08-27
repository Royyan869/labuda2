# RATING HTTP CONTRACT CONVERGENCE IMPLEMENTATION REPORT

## VERDICT

**RATING_HTTP_CONTRACT_CONVERGED**

The Rating HTTP layer is converged to the owner-locked contract end-to-end:
- Exactly the 4 live endpoints; no `/ratings/state`, no `GetRatingState`.
- `limit` + `cursor` (`int64` Unix-ns) request semantics; bare collection for list endpoints.
- snake_case JSON keys over the existing `OrderRating` / `RatingSummary` domain models.
- Raw identity `buyer_id` / `seller_id`; NO `reviewer`, NO `verified_purchase`, NO `RatingCursor`, NO `items`/`has_more`/`next_cursor`.
- All rejected backend richness killed; zero live rejected authority remains.
- Mobile HTTP datasource/repository converged to the canonical backend contract.

The only open items are **documented, pre-existing, mobile-side broken/aspirational** residue that touches unrelated-domain owner work (feed/follow/router/profile test fakes referencing the rejected state/reviewer surface). These are NOT part of the HTTP contract authority and were intentionally left for owner decision (see RESIDUE_STATUS). The backend — the contract authority — is fully converged and proven by tests.

---

## CANONICAL_CONTRACT

Locked and implemented as-is (no invention, no redesign):

- Endpoints (unchanged): `POST /orders/:id/ratings`, `GET /users/:id/ratings`, `GET /users/:id/ratings/summary`, `GET /users/me/ratings/given`.
- Pagination authority: `limit`, `cursor int64` (Unix nanoseconds). `0` = first page.
- Domain/application response authority: `OrderRating`, `RatingSummary`.
- HTTP JSON keys: **snake_case** (`id`, `order_id`, `buyer_id`, `seller_id`, `rating_value`, `comment`, `created_at`; `total_ratings`, `average_rating`, `one_star_count`..`five_star_count`).
- Identity: raw `buyer_id` / `seller_id`.
- List endpoints return a **bare rating collection** (no `page`/`meta`, no `items`/`has_more`/`next_cursor`, no opaque cursor).
- Rejected (do NOT resurrect): `reviewer`, `verified_purchase`, `RatingCursor`, `RatingState`/`can_submit`/`rating_state`, `GET /ratings/state`, `page_size` for rating.

---

## AUTHORITY_MAP

| Artifact | Role | Status |
|---|---|---|
| `rating/entity/order_rating.go` `OrderRating` | Canonical domain model + canonical HTTP resource (snake_case json tags added) | CANONICAL_AUTHORITY |
| `rating/infrastructure/repository/order_rating_repository.go` `RatingSummary` | Canonical aggregate (snake_case json tags added) | CANONICAL_AUTHORITY |
| `rating/interfaces` + `rating_service.go` | Canonical application contract (`int64` cursor, no state) | CANONICAL_AUTHORITY (unchanged) |
| `rating/delivery/http/rating_handler.go` | Live handler emitting the canonical representation | CANONICAL_AUTHORITY (unchanged) |
| `routes_core.go:1382-1385` | The 4 live endpoints | CANONICAL_AUTHORITY (unchanged) |
| mobile `rating_api_datasource.dart` | Converged to `limit`/`cursor` + `executeListRequest` bare list | CANONICAL_CONSUMER (converged) |
| mobile `rating_repository_api.dart` | Passes `limit`/`cursor`, returns `List<Rating>` | CANONICAL_CONSUMER (converged) |
| mobile `rating_api_models.dart` / `rating_api_mapper.dart` | snake_case `RatingApiResponse`/`RatingSummaryApiResponse` parsing | CANONICAL_CONSUMER (unchanged, verified clean) |
| mobile `i_rating_repository.dart` | `limit`/`cursor` interface, no state method | CANONICAL_CONSUMER (unchanged) |

Killed (rejected contract, zero canonical consumers): `rating_response.go`, `rating_reviewer.go`, `rating_http_test.go`, `rating_handler_integration_test.go`, `rating_cursor.go`, `cmd/core_server/rating_state_route_guard_test.go`.

---

## CHANGES

**Backend (snake_case canonical serialization):**
- `rating/entity/order_rating.go` — added canonical snake_case `json` tags to `OrderRating` (`id`, `order_id`, `buyer_id`, `seller_id`, `rating_value`, `comment,omitempty`, `created_at`, `invalidated_at,omitempty`). Raw identity preserved; `buyer_id`/`seller_id`; no `reviewer`/`verified_purchase`. Domain semantics unchanged.
- `rating/infrastructure/repository/order_rating_repository.go` — added snake_case `json` tags to `RatingSummary`.
- `rating/infrastructure/repository/order_rating_repository_integration_test.go` — converged the opaque-cursor calls (`time.Time{}, uuid.Nil, bool`) to the canonical `int64` cursor; removed the rejected UUID-tie-break test (`TestListBySeller_EqualTimestamps_UUIDTieBreak`); removed now-unused `sort` import.
- `rating/delivery/http/rating_http_contract_test.go` — NEW locked-contract proof (see TESTS).

**Mobile (canonical consumer convergence):**
- `rating_api_datasource.dart` — `getRatingsReceived`/`getRatingsGiven` now send `limit`/`cursor` (int64 Unix-ns) and parse a **bare collection** via `executeListRequest` → `Result<List<RatingApiResponse>>`; removed `page`/`page_size` + `PaginatedApiResponse`; removed the now-unused `common_api_models.dart` import.
- `rating_repository_api.dart` — `getRatingsReceived`/`getRatingsGiven` pass `limit`/`cursor` through and map `List<RatingApiResponse>` → `List<Rating>`.

No routes, handlers, commands, application interfaces, repository methods, or business behavior were created.

---

## DELETED

Proven zero canonical production consumers before deletion (verified by full-repo grep):
- `rating/delivery/http/rating_response.go` — rich DTOs (`RatingResponse`, `RatingListResponse`, `RatingSummaryResponse`, `RatingStateResponse`, `RatingStateReasonResponse`).
- `rating/delivery/http/rating_reviewer.go` — reviewer hydration helpers (`buildReviewerCards`, `projectSingleReviewer`, `projectReviewerCard`, `ReviewerInfo`).
- `rating/delivery/http/rating_http_test.go` — build-breaking test referencing never-defined `toRatingResponse`/`toSummaryResponse`/`reviewerCard`.
- `rating/delivery/http/rating_handler_integration_test.go` — asserted the rejected rich/state/has_more contract.
- `rating/application/rating_cursor.go` — opaque `RatingCursor`/`DecodeRatingCursor` (unused by application/repository which use `int64`).
- `cmd/core_server/rating_state_route_guard_test.go` — asserted the nonexistent `/ratings/state` route.

---

## PROTECTED

- `OrderRating` / `RatingSummary` domain + json tags (canonical now).
- `rating_interfaces.go` `RatingReader`/`RatingMutator`, `rating_service.go`, `rating_factory.go` — canonical application contract including `int64` cursor and absence of state.
- `order_rating_repository.go` read/write methods, `RatingDomains` aggregation.
- `rating_handler.go` and the 4 route registrations.
- `rating_service_test.go`, `order_rating_test.go`, `order_rating_repository_integration_test.go` (canonical).
- Immobile consumers `profile_reviews_tab.dart` / `rating_list_screen.dart` / `rating_provider.dart` (use plain `Rating`/`RatingSummary`).
- The locked financial contract — untouched.

---

## TESTS (exact commands + exit codes, cwd = backend unless noted)

| Command | Exit | Result |
|---|---|---|
| `go build ./...` | 0 | Full backend builds |
| `go vet ./internal/commerce/order/rating/...` | 0 | Clean |
| `go vet ./cmd/core_server/` | 0 | Clean (route-guard test gone) |
| `go vet -tags integration ./internal/commerce/order/rating/...` | 0 | Integration-rating tests compile (converged `int64` cursor) |
| `go vet ./internal/commerce/order/... ./internal/commerce/seller/...` | 0 | Clean |
| `go test ./internal/commerce/order/rating/...` | 0 | application / http / entity pass |
| `go test ./internal/commerce/order/rating/delivery/http/ -v` | 0 | New locked-contract tests PASS (4 test funcs) |
| `go build ./...` + `go test ./internal/commerce/order/rating/...` | 0 | Combined regression |
| `dart analyze` (mobile 5 changed rating files) | 0 | "No issues found!" |

**New locked-contract tests (`rating_http_contract_test.go`) prove:**
1. `TestRatingOrderRatingSerializesSnakeCaseCanonical` — snake_case keys; no `reviewer`/`verified_purchase`/PascalCase.
2. `TestRatingSummarySerializesSnakeCaseCanonical` — snake_case `total_ratings`..`five_star_count`; no PascalCase.
3. `TestRatingRoutesExactLiveSetAndNoState` — all 4 routes present; no `GetRatingState`; no `/ratings/state` route.
4. `TestRatingHandlerUsesLimitCursorInt64` — `Cursor int64` + `Limit int`; `ShouldBindQuery`; bare `response.Success`; no rejected `next_cursor`/`has_more`/`RatingCursor`/`reviewer`/`verified_purchase`.

**Pre-existing unrelated failures (NOT introduced by this mission, NOT touched):**
- `internal/worker/*` — untracked-WIP content-mention/alert test build breaks (`SetContentVisibilityChecker`, `ContentMentionedPayload`, `EventContentMentioned`) and `alert_detection_rules_multi_test.go` signature drift. These are `??`-untracked, documented in the prior cleanup report §15, unrelated to rating.

---

## MOBILE_CONVERGENCE

- Datasource + repository converged to `limit`/`cursor` + bare collection + snake_case fields (`buyer_id`/`seller_id`). Verified via `dart analyze` on the changed files (no issues).
- The stale `page`/`page_size` + `data`/`meta` assumption was removed from the rating datasource. On purpose, the SHARED mobile helpers `PaginatedApiResponse`/`executePaginatedRequest`/`paginationParams` were **kept** — they are legitimately used by the `follow` domain and are not rating residue.
- The mobile `RatingApiResponse`/`RatingSummaryApiResponse` models already parse the canonical snake_case fields; unchanged and compatible.
- Backend was NOT changed to satisfy any mobile stale shape; mobile was converged to the canonical backend.

---

## RESIDUE_STATUS

**Backend — CLEAN (zero live rejected authority).** Full-repo grep for `GetRatingState|RatingStateResponse|rating_state|RatingCursor|toRatingResponse|toSummaryResponse|reviewerCard|verified_purchase|RatingListResponse|RatingSummaryResponse` returns ONLY:
- Documentation comment in `order_rating.go:26` ("no reviewer ... no verified_purchase") — anti-proof.
- Negative assertions/comment in `rating_http_contract_test.go` (deliberately asserts these are ABSENT).

No live producer, route, handler, application command, or repository method references the rejected contract. Object-only matches in unrelated domains are protected: `external_product_handler.go:76 CanSubmit` (promotion domain, unrelated).

**Mobile — documented owner-decision (pre-existing, NOT part of HTTP authority, NOT touched):**
- `RatingState` entity (`canSubmit`) + `order_rating_cta_gate.dart` — mobile analog of the rejected `can_submit`/state concept; **dead** (zero live `lib` callers; no provider/datasource feeds it; the gated functions have no `lib` call sites). Left in place because removing it would ripple into unrelated-domain test fakes (feed/follow/router/profile) — a STOP-condition ("unrelated owner work would be touched").
- Pre-existing non-compiling mobile TEST files asserting the rejected reviewer/state/`RatingListResponseDto`/`has_more` contract: `rating_runtime_proof_test.dart`, `rating_dto_mapper_test.dart`, `rating_provider_disposal_proof_test.dart`, `order_detail_rating_submit_proof_test.dart`, `order_rating_cta_gate_test.dart`, and cross-domain fakes in `feed_root_wiring_test.dart` / `follow_status_provider_lifecycle_test.dart` / `router_lifetime_preservation_test.dart` / `profile_reviews_tab_verified_purchase_test.dart`. These were already non-compiling before this mission (they reference `RatingReviewerDto`/`RatingListResponseDto`/`RatingState` constructor shapes that do not exist in `lib`) and are aspirational tests for the rejected contract. Not touched because they span unrelated-domain owner test suites.

These do NOT create any live rejected authority — nothing supplies `RatingState` or a rating `reviewer` from a backend, and the mobile HTTP calls now hit the canonical endpoints.

---

## REGRESSION

- `go build ./...` → **0**.
- `go vet` on rating, order, seller, core_server → **0**.
- `go vet -tags integration` on rating → **0**.
- `go test ./internal/commerce/order/... ./internal/commerce/seller/...` → **all ok** (order app/http/entity/repo, seller http/entity; rating app/http/entity).
- `go test ./internal/commerce/order/rating/...` → **0**.
- New locked-contract rating http tests → **PASS**.
- Mobile `dart analyze` (changed rating files) → **0 / no issues**.
- The `internal/worker` package build-break is the documented pre-existing untracked-WIP content-mention/alert issue, unrelated to this convergence and not caused by my changes.

No DB was queried or modified. No migration was created or applied.

---

## FILES_CHANGED

**Backend — modified:**
- `backend/internal/commerce/order/rating/entity/order_rating.go`
- `backend/internal/commerce/order/rating/infrastructure/repository/order_rating_repository.go`
- `backend/internal/commerce/order/rating/infrastructure/repository/order_rating_repository_integration_test.go`

**Backend — deleted (rejected surface):**
- `backend/internal/commerce/order/rating/application/rating_cursor.go`
- `backend/internal/commerce/order/rating/delivery/http/rating_response.go`
- `backend/internal/commerce/order/rating/delivery/http/rating_reviewer.go`
- `backend/internal/commerce/order/rating/delivery/http/rating_http_test.go`
- `backend/internal/commerce/order/rating/delivery/http/rating_handler_integration_test.go`
- `backend/cmd/core_server/rating_state_route_guard_test.go`

**Backend — new:**
- `backend/internal/commerce/order/rating/delivery/http/rating_http_contract_test.go`

**Mobile — modified:**
- `apps/mobile/lib/domains/social/rating/data/datasources/rating_api_datasource.dart`
- `apps/mobile/lib/domains/social/rating/data/repositories/api/rating_repository_api.dart`

**Mobile tests / `RatingState` / CTA gate:** intentionally NOT touched (owner-decision residue in unrelated cross-domain test fakes).

---

## DATABASE_CHANGED

None.

## MIGRATIONS_CHANGED

None.

---

## RISK_SCORE

**8 / 30.** Rationale (heuristic): the backend contract authority is fully converged and proven by tests with zero live rejected authority; the mobile HTTP datasource/repository is converged and analyzer-clean. Residual risk (why not lower):
- The dead mobile `RatingState`/CTA-gate and several pre-existing non-compiling mobile rating/cross-domain test files remain. They are inert (no live authority) but a careless future dev could mistake them for a spec or try to resurrect the state/reviewer surface — mitigated only by documentation, not deletion.
- A later owner decision is still required to remove the mobile-side `RatingState`/CTA-gate and reconcile the pre-existing broken test fakes across unrelated domains (feed/follow/router/profile).
Not scored higher because these do not affect the authoritative backend contract and none of them are wired to any live data source.

---

## FINAL_STOP

STOP — convergence complete:
- The 4 live Routes, the live handler, application contract, domain model, and mobile canonical interface all emit/consume the same locked contract (snake_case, buyer_id/seller_id, limit/cursor int64, bare collection, no state).
- Rejected backend surface (`GetRatingState`, `rating_state`, `can_submit`, `RatingCursor`, rich `RatingResponse`/`RatingListResponse`/`RatingSummaryResponse`, `reviewerCard`, `toRatingResponse`, `toSummaryResponse`, `/ratings/state` route guard) is deleted.
- Zero live rejected authority remains.
- New tests lock the contract and the residual is documented.
- No financial behavior, no unrelated tracked work, no DB, no migration changed.

Mobile-side dead `RatingState`/CTA-gate and the pre-existing non-compiling rating/cross-domain test fakes remain as documented owner-decision residue (they predate this mission, span unrelated domains, and carry no live authority). The authoritative Rating HTTP contract is **CONVERGED**.