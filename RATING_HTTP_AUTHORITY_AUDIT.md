# RATING HTTP AUTHORITY AUDIT

Read-only audit. No production code, DTO, handler, route, command, interface, or test was modified. The financial contract is untouched. All conclusions are drawn from the current filesystem (implementation truth); git is used only as provenance evidence.

---

## 1. VERDICT

**CONTRACT_REMAINS_UNRESOLVED / PROOF_INCOMPLETE**

The current source does NOT prove a single canonical Rating HTTP contract. Three mutually incompatible wire-response surfaces coexist and no one surface is backed by a complete, self-consistent authority chain (route → application command → domain model → canonical consumer).

- What IS agreed by almost all authoritative sources (routes, application interfaces, domain entity, mobile repository interface): the **4 existing endpoints**, request params `limit`/`cursor` (**Unix-ns int64**), the **raw `OrderRating`/`RatingSummary` field set**, and the **absence of any `GetRatingState` endpoint**.
- What is NOT agreed: the **response field-key casing** (current raw handler → PascalCase; mobile consumer → snake_case), the **list pagination envelope** (current handler → bare array; rich DTO → `items`/`has_more`/`next_cursor`; mobile → `page`/`page_size` + `meta`), and the **identity shape** (current/raw → `buyer_id`/`seller_id` scalars; rich DTO → single `reviewer` card + `verified_purchase`; mobile → optional `buyer`/`seller` `UserBrief` pair).

Because no single contract is proven across the full chain, and the candidate surfaces are mutually incompatible at the field/pagination/envelope level, the mission STOP rule applies. Neither surface A nor surface B is selected as canonical. An owner/business decision is required (Section 15).

---

## 2. CANONICAL_AUTHORITY

There is NO single provable canonical Rating HTTP contract. The partial authorities are:

| Authority | Scope | Status |
|---|---|---|
| `entity/order_rating.go` `OrderRating` | Domain model: raw fields `{ID, OrderID, BuyerID, SellerID, RatingValue, Comment, CreatedAt, InvalidatedAt}`, NO json tags, NO reviewer/verified_purchase | **CANONICAL_AUTHORITY** (domain truth, appendix only about business behavior) |
| `rating_interfaces.go` `RatingReader`/`RatingMutator` | Canonical application API: `CreateRating`, `GetRatingByOrder`, `GetRatingSummary`, `GetAverageRatingForPeriod`, `ListRatingsGivenByBuyer`, `ListRatingsReceivedBySeller`, `InvalidateForOrder`. **NO `GetRatingState`.** | **CANONICAL_AUTHORITY** (application contract) |
| `rating_service.go` | Implementations; list input `Cursor int64` (Unix ns); returns bare `[]*OrderRating`, `*RatingSummary` | **CANONICAL_AUTHORITY** (application behavior) |
| `order_rating_repository.go` | `ListByBuyer`/`ListBySeller` take `cursor int64` (Unix ns), filter `created_at < $n`; `RatingSummary` no json tags | **CANONICAL_AUTHORITY** (persistence) |
| `routes_core.go:1382-1385` | The only 4 live rating routes → `deps.RatingHandler.*` | **CANONICAL_AUTHORITY** (route set; no `/ratings/state`) |
| `mobile i_rating_repository.dart` | Mobile canonical interface: 4 endpoints, `limit`/`cursor` (Unix ns), §3 fields; **no `getRatingStateForOrder`** | **CANONICAL_CONSUMER** (mobile contract intent) |

These agree on endpoints + request params + domain field set + absence of state, but do NOT agree on the exact wire envelope/key-casing, so the HTTP contract itself is UNRESOLVED (Section 15).

---

## 3. DOMAIN_AND_APPLICATION_MAP

- **Domain model:** `ratingEntity.OrderRating` (raw, immutable, buyer→seller, completed-order-only, 1–5). Constructor `NewOrderRating` validates. No reviewer, no verified_purchase, no state.
- **Application commands/queries (interface + service):**
  - `CreateRating` (mutator — completed order, buyer-only, no dispute, idempotent `ErrAlreadyRated`).
  - `InvalidateForOrder` (mutator — refund path).
  - `GetRatingByOrder`, `GetRatingSummary`, `GetAverageRatingForPeriod`, `ListRatingsGivenByBuyer`, `ListRatingsReceivedBySeller` (reader).
- **Application read models returned:** `*OrderRating`, `[]*OrderRating`, `*repository.RatingSummary`, `float64`.
- **NOT in the canonical application contract:** reviewer identity projection, `verified_purchase`, opaque cursor, `has_more`/`next_cursor`, and `GetRatingState` → **all ABSENT** from `RatingReader`/`RatingMutator`/`RatingService`/repository.
- **Application-layer tests** (`rating_service_test.go`) validate only the raw `OrderRating` constructor & invariants — corroborating the raw-entity/aggregate domain truth.
- **Production app-layer consumers** (all use the canonical interface/aggregates, none use reviewer/opaque-cursor/state):
  - `seller/delivery/http/seller_handler.go:974` `GetPerformance` — reads `ratingSummary.AverageRating` (scalar).
  - `worker/seller_monthly_metrics_worker.go:74` — factory reader (period average).
  - `worker/rating_invalidation_worker.go` — factory (invalidate).
  - `commerce/order/application/order_completion_service.go:135` — factory (create/invalidate).

Conclusion: The application layer definitively does NOT require the rich DTO behavior. Reviewer projection, `verified_purchase`, opaque cursor, `has_more`/`next_cursor`, and rating-state are **not part of the canonical application/domain contract**.

---

## 4. HTTP_ROUTE_MATRIX

Exact current routes (`cmd/core_server/routes_core.go`):

| Route | Handler method (rating_handler.go) | App command | Consumers | DTO | Auth |
|---|---|---|---|---|---|
| `POST /orders/:id/ratings` | `CreateRating` | `RatingService.CreateRating` | mobile `createRatingForOrder` | raw `*OrderRating` (PascalCase) | `middleware.RequireActiveAccount` |
| `GET /users/:id/ratings` | `ListRatingsReceived` (reads `limit`,`cursor`) | `ListRatingsReceivedBySeller(int64 cursor)` | mobile `getRatingsReceived` | raw `[]*OrderRating` bare array (PascalCase) | none |
| `GET /users/:id/ratings/summary` | `GetRatingSummary` | `GetRatingSummary` | mobile `getRatingSummary` | raw `*RatingSummary` (PascalCase) | none |
| `GET /users/me/ratings/given` | `ListRatingsGiven` (reads `limit`,`cursor`) | `ListRatingsGivenByBuyer(int64 cursor)` | mobile `getRatingsGiven` | raw `[]*OrderRating` bare array (PascalCase) | authenticated via `userID` context |

**Not registered:** `GET /orders/:id/ratings/state` (no route, no handler, no app command). `rating_state_route_guard_test.go` asserts it and FAILS (exit 1) because the route string is absent — confirming it does not exist.
**No alternative route is registered or proposed in production.** Mobile `getRatingForOrder` (`GET /orders/:id/ratings`) is explicitly PARKED ("Backend has no GET /orders/:id/ratings route").

---

## 5. CONSUMER_MAP

| Consumer | Endpoint(s) | Request | Response fields expected | Pagination | Reviewer | State expectation |
|---|---|---|---|---|---|---|
| mobile `rating_repository_api.dart` (live) | all 4 | passes `limit`/`cursor`→datasource `page:1,pageSize:limit` | `RatingApiResponse` = `id,order_id,buyer_id,seller_id,rating_value,comment,created_at` + optional `buyer`/`seller` `UserBrief`; `RatingSummaryApiResponse` = `total_ratings,average_rating,*_star_count` | datasource sends `page`/`page_size`; parses `PaginatedApiResponse` `{data, meta:{page,per_page,total,total_pages}}` | `buyer`/`seller` (UserBrief), NOT a single `reviewer` | none; `GET /orders/:id/ratings` parked |
| mobile `rating_api_datasource.dart` | all 4 | `page`/`page_size` QP | same `RatingApiResponse`/`RatingSummaryApiResponse` | offset pages + `PaginatedApiResponse.meta` | `buyer`/`seller` | none |
| mobile `i_rating_repository.dart` (canonical interface) | all 4 | `limit`/`cursor` (Unix ns) | same field set | **`limit`/`cursor` (Unix ns)** | `buyer`/`seller` | **no state method** |
| mobile `order_detail_handlers.dart` (live create flow) | `POST /orders/:id/ratings` | `{rating_value,comment}` | only success/error (+`EMAIL_VERIFICATION_REQUIRED`) | — | not consumed | not consumed |
| mobile `order_rating_cta_gate.dart` | n/a | n/a | `RatingState.canSubmit` | — | — | consumed BUT fed from mock/test only; no `lib` provider/datasource wiring exists |
| seller `GetPerformance` (backend) | `GetRatingSummary` app | — | `AverageRating` scalar | — | — | — |
| backend `rating_handler_integration_test.go` (`integration` tag) | all 4 + `GetRatingState` (direct handler call, un-routed) | `limit`/`cursor`(translated) | `RatingResponse`/`RatingListResponse`/`RatingSummaryResponse`/`RatingStateResponse` (reviewer, verified_purchase, has_more/next_cursor, can_submit) | rich opaque-like | single `reviewer` | YES |
| backend `rating_http_test.go` (unit) | DTO builders only | — | `toRatingResponse`/`toSummaryResponse`/`reviewerCard` | rich | single `reviewer` | YES (`RatingStateResponse`) |
| backend `rating_state_route_guard_test.go` | asserts (missing) state route | — | — | — | — | YES |
| web/admin client, API-docs spec | **NONE found** | — | — | — | — | — |

---

## 6. DTO_AND_MAPPER_MAP

| Artifact | Producer | Consumers | Route-wired | App-backed | Test-only | Classification |
|---|---|---|---|---|---|---|
| `rating_handler.go` handlers | — | routes | YES | YES | — | CANONICAL_AUTHORITY (route/app level), but wire serialization UNRESOLVED |
| `rating_response.go` `RatingResponse`, `RatingListResponse`, `RatingSummaryResponse`, `RatingStateResponse`, `RatingStateReasonResponse` | `rating_response.go` | integration + unit tests ONLY | NO (never emitted by a handler) | NO (no builder producer; `toRatingResponse`/`toSummaryResponse` undefined) | YES | LEGACY/ZOMBIE/INCORRECT* (unwired; `reviewer`+`verified_purchase`+`has_more` model conflicts with mobile canonical) |
| `rating_reviewer.go` `buildReviewerCards`, `projectSingleReviewer`, `projectReviewerCard`, `ReviewerInfo` | `rating_reviewer.go` | integration/unit tests ONLY | NO (no ratio handler calls them) | NO | YES | LEGACY/ZOMBIE/INCORRECT* (unwired; `publiccard` used only here in rating) |
| `toRatingResponse` | — (never defined in any committed production file) | `rating_http_test.go` | N/A | N/A | N/A | CONFLICTING (build break; producer absent) |
| `toSummaryResponse` | — (never defined) | `rating_http_test.go` | N/A | N/A | N/A | CONFLICTING (build break; producer absent) |
| `reviewerCard` | — (never defined) | `rating_http_test.go` | N/A | N/A | N/A | CONFLICTING (build break; producer absent) |
| `RatingCursor`/`DecodeRatingCursor` (`rating_cursor.go`) | `rating_cursor.go` | unit test only | NO | NO (service uses `int64`) | YES | DUPLICATE_AUTHORITY + LEGACY/ZOMBIE (documented "sole cursor authority" but unused; duplicates `int64` cursor) |
| `repository.RatingSummary` | repo | handler/app | YES | YES | — | CANONICAL_AUTHORITY (aggregation, but no json tags → PascalCase wire) |
| mobile `RatingApiResponse`/`RatingSummaryApiResponse`/`PaginatedApiResponse` | mobile DTO | mobile repo/datasource/mapper | N/A | N/A | — | CANONICAL_CONSUMER (but pagination envelope + keys do not match current raw handler) |
| mobile `RatingState` | mobile entity | CTA gate + tests | N/A | N/A (no live data path) | — | UNKNOWN → residue (no backend endpoint/route/app command; no live mobile datasource) |

\* Classified LEGACY/ZOMBIE/INCORRECT with respect to the LIVE wire path (they are not emitted and contradict both the route handler and the mobile canonical interface). They are NOT provably "rejected" as a product decision — see OWNER_DECISION (Section 15).

---

## 7. PAGINATION_CONTRACT

- **Canonical application/repository/model:** keyset on `created_at DESC` with **structured `cursor int64` (Unix ns)**, `limit` bounds 20/1–50. No `has_more`, no `next_cursor`, no opaque compound cursor. Return = bare `[]*OrderRating`.
- **Canonical mobile interface intent:** `limit`/`cursor` (Unix ns) — matches the application layer's `int64` cursor.
- **Live mobile datasource (actual bytes sent):** `page`/`page_size`, parsing a `{data, meta:{page,per_page,total,total_pages}}` envelope (offset-style). This contradicts the interface comment and the backend request/response.
- **Rich DTO surface:** opaque `RatingCursor{CreatedAt,ID}`, `has_more`/`next_cursor` envelope.

THREE mutually inconsistent pagination models exist (structured-int64 bare-array; opaque-cursor envelope; page-offset envelope). The source does not prove which is canonical — the live backend implements the first, the live mobile datasource expects the third, the rich tests assert the second, and the mobile interface documents the first while its datasource sends the third.

---

## 8. RATING_STATE_CONTRACT

- **No `GetRatingState` exists** in: `rating_interfaces.go`, `rating_service.go`, `order_rating_repository.go`, `routes_core.go` (no route), `rating_handler.go` (no method), or the mobile `lib` data layer (no datasource/repository method; `getOrderRatingStateProvider` does not exist in `lib`).
- The only residents of the state concept are: `rating_response.go` (`RatingStateResponse` type), `rating_http_test.go`, `rating_handler_integration_test.go`, `cmd/core_server/rating_state_route_guard_test.go`, and mobile test mocks + the `RatingState` entity. The mobile `order_rating_cta_gate.dart` consumes a `RatingState` **only from test/mock-injected inputs** — there is no production provider/datasource feeding it a real backend response.
- **No authoritative consumer/use-case exists** for a rating-state endpoint on either side. Per the mission rule, it is therefore classified as **residue (invented-but-unwired)**, not a canonical requirement. Wiring it would require building a new route + handler + application command + mobile data path (out of scope / owner decision).

---

## 9. BUILD_FAILURE_ANALYSIS

- Failure: `rating_http_test.go` (unit, no build tag) references undefined `toRatingResponse`, `toSummaryResponse`, `reviewerCard`.
- Exact undefined symbols (compile evidence):
  - `toRatingResponse` — `rating_http_test.go:34,171,243,260,330,344`
  - `toSummaryResponse` — `rating_http_test.go:94,364`
  - `reviewerCard` — `rating_http_test.go:268,278`
- **Producer:** none. These functions are **never defined in any committed production file in the entire backend history** (verified by `git log -S` and `git grep` at the snapshot commit); they are expected to live alongside `rating_response.go`/`rating_reviewer.go` but were never written.
- **Consumer:**

  `rating_http_test.go` (undefined refs) ← (would read) DTO/helper contract that has no producer.

- **Command + exit codes:**
  - `go build ./...` → **exit 0** (production builds; the failure is test-package-only).
  - `go vet ./internal/commerce/order/rating/delivery/http/` → **exit 1**: `undefined: toRatingResponse`.
  - `go test ./internal/commerce/order/rating/delivery/http/` → **exit 1** (build failed).
  - `go test ./cmd/core_server/ -run TestRatingStateRoute_RequiresActiveAccount` → **exit 1**: `MISSING: GET /orders/:id/ratings/state ...` (separate but related route-gap).
- **Does "fixing" require creating a new business/API contract?** Yes. Resurrecting `toRatingResponse`/`toSummaryResponse`/`reviewerCard` to make the unit test compile would (a) create builder functions with **zero production consumers**, and (b) leave the handler still not emitting `reviewer`/`verified_purchase`/`has_more`/`next_cursor`, so the integration spec and the rich DTO would remain unwired — a green-by-shim, not a contract. Completing the rich contract properly would require rewriting the handler + adding a `GetRatingState` route + command, i.e., inventing a new API. Per the mission, the build is **not fixed** here.

---

## 10. CANONICAL_CONSUMERS

- Backend live route-wired handlers `rating_handler.go` (raw entity + `int64` cursor) — the only server consumers of the canonical application layer.
- Backend `seller_handler.go` `GetPerformance` (reads `AverageRating` scalar via `GetRatingSummary`).
- Backend workers `seller_monthly_metrics_worker.go`, `rating_invalidation_worker.go`, and `order_completion_service.go` (via `RatingDomainFactory` getters).
- Mobile live `rating_repository_api.dart` / `order_detail_handlers.dart` (`createRatingForOrder`; success/error only).
- Mobile canonical interface `i_rating_repository.dart` (4 endpoints, `limit`/`cursor`, snake_case entity fields, no state).

These consumers pin: the **4 endpoints**, **request `limit`/`cursor` int64**, the **`OrderRating`/`RatingSummary` field sets**, and the **absence of a state endpoint**. They do NOT collectively pin the **response envelope / key-casing** (see Section 15).

---

## 11. DUPLICATE/CONFLICTING_AUTHORITIES

- **`RatingCursor` (opaque)** vs **`cursor int64` (Unix ns)**: `rating_cursor.go` calls itself "the sole cursor authority" but the application service, repository, mobile interface, and live handler all use the `int64` structured cursor. → DUPLICATE_AUTHORITY (rich) / the `int64` is the effective one. CONFLICTING.
- **`RatingResponse` (single `reviewer`, `verified_purchase`)** vs **`buyer_id`/`seller_id` raw entity + mobile `buyer`/`seller` `UserBrief`**: the rich DTO `reviewer` model has no route producer and contradicts both the raw field model and the mobile `buyer`/`seller` pair. CONFLICTING.
- **`RatingListResponse` (`items`/`has_more`/`next_cursor`)** vs **bare `[]*OrderRating`** vs **mobile `PaginatedApiResponse` (`data`/`meta` with page numbers)**: three incompatible list envelopes. CONFLICTING.
- **`RatingSummaryResponse` (snake_case)** vs **raw `RatingSummary` (PascalCase)**: the rich summary DTO's snake_case keys match the mobile consumer, the raw handler's PascalCase keys do not. Yet the rich DTO is unwired. CONFLICTING at the wire level.

---

## 12. LEGACY/ZOMBIE/INCORRECT_RESIDUE

- `rating_response.go` rich DTO types — unwired (never emitted by a route handler); `RatingStateResponse`/`RatingStateReasonResponse` in particular have no producer, route, or consumer.
- `rating_reviewer.go` helpers (`buildReviewerCards`, `projectSingleReviewer`, `projectReviewerCard`, `ReviewerInfo`) — unused by any route-wired handler; `publiccard` used only here in the rating package.
- `RatingCursor` opaque cursor — unused by the application/repository; duplicates the `int64` cursor.
- `GetRatingState` concept — no route, no handler, no app command, no live backend or mobile data path; residents are DTO type + tests + mocks only.
- `rating_http_test.go` — references never-defined producers (build break).
- `rating_handler_integration_test.go` (`integration` tag) — asserts a contract the route-wired handlers do not implement; would fail against current handlers (requires a live DB and does not run in the default build).
- `cmd/core_server/rating_state_route_guard_test.go` — asserts a non-existent route; fails (exit 1).
- mobile `PaginatedApiResponse`-based datasource + `page`/`page_size` — internally inconsistent with the mobile `IRatingRepository` interface (`limit`/`cursor`) and with the backend handler.

These are classified as residue relative to the live wire path, but their removal/retention is an owner call (Section 15), NOT something this audit deletes.

---

## 13. KILL_LIST

Artifacts provably stale/duplicate relative to the live wire path — candidates for later removal (owner-authorized):
1. `toRatingResponse`, `toSummaryResponse`, `reviewerCard` — never-defined; only the build-break unit test references them. Fix the test, do not add these.
2. `rating_response.go` rich DTO types — unwired; no route producer; `RatingStateResponse`/`RatingStateReasonResponse` in particular reference a non-existent endpoint.
3. `rating_reviewer.go` helpers — no route-wired producer.
4. `rating_cursor.go` opaque `RatingCursor` — unused duplicate of the canonical `int64` cursor (the `has_more`/`next_cursor` concept in `RatingListResponse` is likewise unwired).
5. `GetRatingState` route/handler/concept — no production consumer on backend or mobile; test/mock-only.
6. `rating_http_test.go` — build-breaking unit test asserting the unwired rich contract (replace or remove).
7. `rating_handler_integration_test.go` — asserts an unwired contract (reconcile or remove).
8. `cmd/core_server/rating_state_route_guard_test.go` — asserts a non-existent route.

(NOT deleted in this read-only audit.)

---

## 14. PROTECTED_LIST

Artifacts proven canonical and must not be touched (financial contract also never touched):
1. `entity/order_rating.go` `OrderRating` + `NewOrderRating` — immutable buyer→seller domain truth; raw field set.
2. `rating_interfaces.go` `RatingReader`/`RatingMutator` + `rating_service.go` + `rating_domain_factory.go` — the application contract (incl. `int64` cursor, absence of state), and the factory boundary.
3. `order_rating_repository.go` — persistence incl. the aggregate read models.
4. `rating_handler.go` route-wired handlers (`CreateRating`, `ListRatingsReceived`, `ListRatingsGiven`, `GetRatingSummary`) — the only live server surface; their route registrations.
5. `routes_core.go:1382-1385` — the 4 canonical endpoints.
6. Inner-domain/worker consumers (`seller GetPerformance`, `seller_monthly_metrics_worker`, `rating_invalidation_worker`, `order_completion_service`) — all use the canonical interface; none require rich DTO.
7. Mobile `Rating`/`RatingSummary` entities + `RatingSummaryApiResponse` field set (the snake_case aggregation truth) and the 4-endpoint `IRatingRepository` shape.
8. The locked financial contract — untouched.

---

## 15. OWNER_DECISION_REQUIRED

The source proves ONE OF TWO incompatible wire contracts, but not which, because each is contradicted by a different authoritative artifact. Evidence required from product/architecture:

1. **Response key casing + per-rating identity model** — choose ONE:
   - (Current raw handler, canonical domain/route level): PascalCase keys via bare `OrderRating`; identity as `buyer_id`/`seller_id` scalars. **Problem:** does NOT match mobile (`order_id` etc., `buyer`/`seller` `UserBrief`).
   - (Rich DTO / mobile summary-style): snake_case keys; single `reviewer` `UserCard` + `verified_purchase` (rich) OR `buyer`/`seller` `UserBrief` pair (mobile). **Problem:** rich `reviewer`+`verified_purchase` isn't the mobile shape either; mobile uses a `buyer`/`seller` pair.

2. **List pagination envelope** — choose ONE:
   - `limit`/`cursor` bare array (current backend + mobile interface): returned as a bare `[]*OrderRating`.
   - `items`/`has_more`/`next_cursor` (rich DTO, opaque cursor).
   - `page`/`page_size` + `meta {page,per_page,total,total_pages}` (mobile datasource). The current handler reads `limit`/`cursor` and returns a bare array; the mobile datasource sends `page`/`page_size` and expects `meta`. These are not interoperable.

3. **`GetRatingState` endpoint** — decide whether a buyer-eligibility state endpoint is a required feature. Today it is test/mock-only with no route, handler, app command, or mobile data path. If required, it is new work (route + handler + application command + client provisioning) — not a pre-existing canonical contract. If not required, remove the DTO/tests referencing it.

4. **Mobile side cleanup** — the mobile `IRatingRepository` (`limit`/`cursor`) contradicts its own `RatingApiDatasource` (`page`/`page_size`); one is authoritative.

The financial contract is NOT part of this decision and remains locked.

---

## 16. RISK_SCORE

**Risk score: 21 / 30.**

Rationale (heuristics, not objective truth):
- +6 High surface presence: both a route-wired raw handler and a substantive but fully-unwired rich DTO/test scaffold exist, each with confident "canonical" documentation, so an implementer could reasonably pick wrong.
- +5 Build broken at the test layer with undefined production symbols (`toRatingResponse`/`toSummaryResponse`/`reviewerCard`), and a separate route-guard test fails — two independent red flags.
- +4 Pagination is triply divergent (structured-int64 bare array / opaque `has_more` envelope / page-page_size `meta`), a change here is a breaking wire change for mobile.
- +4 Consumer splits: the mobile client (snake_case + `buyer`/`seller` + page envelope) matches NEITHER the raw handler (PascalCase bare array) NOR the rich DTO (`reviewer`+`verified_purchase`+`has_more`). No revert-safe middle ground.
- +2 A test-and-DTO-only concept (`GetRatingState`) could be mistaken for canonical and built as a new endpoint, adding speculative surface.
- Not +9/+10 because: the 4 endpoints, domain entity, application commands, and request params are well-established; there is a clear, defensible route to resolution once an owner picks the wire shape; and no financial/accounting risk is involved. Kept below 30 because the domain/application truth is coherent — only the wire adaptation layer is contested.

---

## 17. TESTS_AND_COMMANDS

All commands read-only; exact exit codes:

| Command (cwd=backend) | Exit | Result |
|---|---|---|
| `go build ./...` | 0 | Full production backend builds |
| `go vet ./internal/commerce/order/rating/delivery/http/` | 1 | `undefined: toRatingResponse` (test-only build break) |
| `go test ./internal/commerce/order/rating/delivery/http/` | 1 | Build failed (undefined refs) |
| `go test ./cmd/core_server/ -run TestRatingStateRoute_RequiresActiveAccount -v` | 1 | `MISSING: GET /orders/:id/ratings/state ...` |
| `grep GetRatingState/rating_state/can_submit (backend)` | 0 | Only `rating_response.go` + tests + route-guard reference it; zero production route/handler/app |
| `grep getOrderRatingStateProvider (mobile lib)` | — | Not found in `lib` (test-only) |
| `grep toRatingResponse\|toSummaryResponse\|reviewerCard (all .go)` | 0 | Only `rating_http_test.go`; no producer anywhere |

No integration (DB) tests were run; their contract is documented in source and assessed statically.

---

## 18. FILES_CHANGED

- **None modified.**
- Created: `RATING_HTTP_AUTHORITY_AUDIT.md` (this report).
- (Note: prior mission already converged `refund_history_contract_test.go` and deleted `order_refund_history_contract_test.go`; those are not rating changes and are outside this audit.)

---

## 19. DATABASE_CHANGED

None.

---

## 20. MIGRATIONS_CHANGED

None.

---

## 21. FINAL_STOP

The current source does NOT prove a single canonical Rating HTTP contract. The route set, application layer, domain entity, and mobile repository interface all agree on the 4 endpoints, the raw `OrderRating`/`RatingSummary` field sets, `limit`/`cursor` (Unix-ns int64) pagination, and the absence of `GetRatingState` — but the **response field-key casing**, **list pagination envelope**, and **reviewer identity shape** are each represented by three mutually incompatible surfaces (current raw handler / rich DTO / mobile consumer), and neither surface A nor surface B is backed by a complete, self-consistent authority chain.

STOP. Do NOT implement A or B, do NOT fix the rating build, do NOT choose a contract on my behalf. Owner/architecture decision required on the four items in Section 15.

VERDICT: **CONTRACT_REMAINS_UNRESOLVED / PROOF_INCOMPLETE**