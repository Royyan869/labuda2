# MOBILE RATING RESIDUE AUDIT

Read-only authority/residue audit of the remaining mobile Rating residue identified in
`RATING_HTTP_CONTRACT_CONVERGENCE_IMPLEMENTATION_REPORT.md`. No production behavior was
modified. No cleanup was performed.

---

## VERDICT

**MOBILE_RATING_RESIDUE = ALL ZOMBIE (inert, non-canonical, non-compiling residue)**

Every audited artifact is dead weight:

- `RatingState` and `order_rating_cta_gate.dart` are production-shaped (`lib/`) but have
  **zero live consumers**: no provider, datasource, or repository produces `RatingState`
  data, and neither gate function has any `lib` call site. They are the mobile analog of the
  rejected `can_submit`/`rating_state` surface and must not be resurrected.
- All 5 rating test files and the `verified_purchase` profile test are **non-compiling**
  (analyzer: 237 errors in 9 files) and assert the rejected `reviewer`/`verified_purchase`/
  `RatingState`/`RatingListResponseDto`/`has_more`/`next_cursor` contract. A non-compiling
  test is not a spec; none of it is canonical evidence.
- The 4 cross-domain test suites (feed/follow/router/profile) reference the residue
  **only inside their own test fakes**; their production code never touches it (grep-proof
  below).

No artifact has a canonical consumer. No artifact's data is produced by any live
provider/datasource/repository. No deletion would change Feed/Follow/Router/Profile
**production** behavior. The ONLY entanglement is that deleting the shared `RatingState`
symbol deepens the (already-existing) compile breakage inside 4 unrelated-domain test
suites — a parallel-owner decision, flagged as a STOP condition, not an authority.

Canonical mobile consumers are confirmed healthy and untouched:
`rating_api_datasource.dart`, `rating_repository_api.dart`, `rating_api_models.dart`,
`rating_api_mapper.dart`, `i_rating_repository.dart`, `rating_provider.dart`,
`profile_reviews_tab.dart`, `rating_list_screen.dart`, `order_detail_screen.dart`.

---

## CANONICAL_CONTRACT

Locked (unchanged by this audit):

- 4 live endpoints only: `POST /orders/:id/ratings`, `GET /users/:id/ratings`,
  `GET /users/:id/ratings/summary`, `GET /users/me/ratings/given`.
- `limit` + `cursor` (`int64` Unix-ns); `0` = first page; bare collection list response.
- snake_case keys; raw `buyer_id` / `seller_id`.
- Rejected and NOT to be resurrected: `reviewer`, `verified_purchase`, `RatingCursor`,
  `RatingState`/`can_submit`/`rating_state`, `GET /ratings/state`,
  `page`/`page_size` (rating), `items`/`has_more`/`next_cursor`.

---

## AUTHORITY_MAP

| Artifact (mobile) | Role in canonical rating flow | Status |
|---|---|---|
| `lib/domains/social/rating/data/datasources/rating_api_datasource.dart` | Bare-list HTTP consumer; `limit`/`cursor int64` | CANONICAL_CONSUMER (live, converged) |
| `lib/domains/social/rating/data/repositories/api/rating_repository_api.dart` | Passes `limit`/`cursor`, `List<Rating>` | CANONICAL_CONSUMER (live, converged) |
| `lib/domains/social/rating/data/dto/rating_api_models.dart` | snake_case `RatingApiResponse`/`RatingSummaryApiResponse`; NO `reviewer` | CANONICAL_CONSUMER (live) |
| `lib/domains/social/rating/data/mappers/rating_api_mapper.dart` | DTO → entity mapping | CANONICAL_CONSUMER (live) |
| `lib/domains/social/rating/domain/repositories/i_rating_repository.dart` | `limit`/`cursor` interface; NO state method | CANONICAL_CONSUMER (live) |
| `lib/domains/social/rating/presentation/providers/rating_provider.dart` | `RatingListState` (`ratings/isLoading/error/summary` only) | CANONICAL_CONSUMER (live) |
| `lib/domains/user/profile/presentation/widgets/profile_reviews_tab.dart` | Renders `Rating`/`RatingSummary`; NO `RatingReviewCard` | CANONICAL_CONSUMER (live, protected) |
| `lib/domains/commerce/transaction/order/presentation/screens/order_detail_screen.dart:253-257` | uses `hasUserRatedOrderProvider(orderId/buyerId/sellerId)` | CANONICAL_CONSUMER (live; state-free) |
| `lib/domains/social/rating/domain/entities/rating_entity.dart` `RatingState` (:247-250) | Mobile analog of rejected state; no producer | **ZOMBIE** |
| `lib/.../order_detail/order_rating_cta_gate.dart` (`shouldShowOrderRatingCta`, `gateOrderDecisionForRating`) | Gate over dead state; zero callers | **ZOMBIE** |
| 5 rating test files, `profile_reviews_tab_verified_purchase_test.dart`, residue in feed/follow/router suites | Assert rejected contract; non-compiling | **ZOMBIE** |

---

## RESIDUE_CLASSIFICATION

Legend — every row answers: live? / lib producer? / canonical consumer? / independently deletable?

| Artifact | Classification | Live | lib produces data? | canonical consumer? | Why / evidence | Independently deletable? |
|---|---|---|---|---|---|---|
| `RatingState` (rating_entity.dart:247-250) | **ZOMBIE** | No | No | No | Only consumer is the dead gate; zero producers (`getRatingStateForOrder` absent from `IRatingRepository`, datasource, providers). Rejected `can_submit` analog. Analyzer: lib file clean — it compiles, it is just dead. | No — deleting it adds undefined-symbol errors to 4 cross-domain test suites (see BLOCKERS). Production-wise safe. |
| `order_rating_cta_gate.dart` | **ZOMBIE** | No | No | No | `shouldShowOrderRatingCta`/`gateOrderDecisionForRating` have zero `lib` callers. Only importer is its own broken test. Requires `RatingState`. | Yes (with its own test), but coupled with `RatingState` for full removal. |
| `rating_runtime_proof_test.dart` | **ZOMBIE** | No | N/A | No | 99 analyzer errors. References undefined `RatingListResponseDto`, `RatingStateApiResponse`, `RatingStateReasonApiResponse`, `RatingReviewerDto`, `.reviewer`, `getRatingStateForOrder` (invalid override), `getOrderRatingStateProvider`, positional `hasUserRatedOrderProvider('o1')`. | Yes — rating-owned; no cross-domain reference. |
| `rating_dto_mapper_test.dart` | **ZOMBIE** | No | N/A | No | 36 errors. `RatingReviewerDto`, `RatingListResponseDto`, `reviewer`, `has_more`, `next_cursor` — all rejected/undefined. | Yes — rating-owned. |
| `rating_provider_disposal_proof_test.dart` | **ZOMBIE** | No | N/A | No | 35 errors. `RatingListPage` (undefined), `RatingState(orderId…)`, `_DeferredRatingRepository` missing `getRatingForOrder`, `RatingListState.nextCursor/hasMore/cursorError` and `.reset()` don't exist on the live provider. | Yes — rating-owned. |
| `order_detail_rating_submit_proof_test.dart` | **ZOMBIE** | No | N/A | No | 6 errors. `RatingListPage`, `Rating(...reviewer*)`, `RatingState`, `ratingState:'active'`, positional `hasUserRatedOrderProvider`. | Yes — rating-owned. |
| `order_rating_cta_gate_test.dart` | **ZOMBIE** | No | N/A | No | 8 errors. `RatingState(orderId/hasRating)` invalid against live ctor. Tests a gate with no production consumer. | Yes — its own test. |
| `feed_root_wiring_test.dart` residue (`_FakeRatingRepository` :578-613) | **ZOMBIE** (inside live Feed suite) | No | N/A | No | 16 total errors; rating-caused: `RatingListPage` :582, invalid override :582:34, `getRatingStateForOrder` :606 (override_on_non_overriding), `RatingState(orderId…)` :608-611. | No — removal edits a Feed-owned file (parallel-owner STOP). |
| `follow_status_provider_lifecycle_test.dart` residue (`_FakeRatingRepository` :218-246) | **ZOMBIE** (inside live Follow suite) | No | N/A | No | 9 total errors; `RatingListPage`, `RatingState(orderId…)` :230, invalid overrides :234/:241. | No — removal edits a Follow-owned file (parallel-owner STOP). |
| `router_lifetime_preservation_test.dart` residue (fake repo ~:699-745) | **ZOMBIE** (inside live Router suite) | No | N/A | No | 19 total errors; `RatingListPage` :720-721, `RatingState` :743, `getRatingStateForOrder` :740. Unrelated breakage (sellerIdentity, `PrincipalOperationCheck`, `AnalyticsRepository.trackEngagement`) also present. | No — removal edits a Router-owned file (parallel-owner STOP). |
| `profile_reviews_tab_verified_purchase_test.dart` | **ZOMBIE** | No | N/A | No | 9 errors. `Rating(...verifiedPurchase/reviewer*)` :16-20 (rejected fields), `RatingReviewCard` :33-50 undefined (no such widget in lib). Asserts the rejected `verified_purchase` badge. | Yes as a file, but it is Profile-owned territory (parallel-owner STOP). |

No artifact classifies as CANONICAL_AUTHORITY, CANONICAL_CONSUMER, DERIVED,
DUPLICATE_AUTHORITY, CONFLICTING_AUTHORITY, or LEGACY:

- Not `LEGACY` — no evidence any of these ever compiled/ran against a canonical system;
  they assert a contract that was rejected before landing.
- Not `CONFLICTING_AUTHORITY` — a conflict needs a live authority; none of these is wired
  to any data source or consumer.

---

## LIVE_CALLER_PROOF

Full-repo grep (`apps/mobile`) evidence, symbol by symbol:

- `RatingState` — lib matches ONLY `rating_entity.dart` (definition) and
  `order_rating_cta_gate.dart` (parameter types). Zero providers/datasources/repos feed it.
- `shouldShowOrderRatingCta` / `gateOrderDecisionForRating` — lib matches ONLY the
  definition file. Zero `lib` call sites.
- `getRatingStateForOrder` — zero matches in `lib/` (interface, datasource, providers).
  All 6 hits are test fakes.
- `getOrderRatingStateProvider` — zero matches anywhere in `lib/`. Single test hit.
- `RatingListPage`, `RatingReviewerDto`, `RatingListResponseDto`, `RatingStateApiResponse`,
  `RatingStateReasonApiResponse`, `RatingStateReason`, `RatingReviewCard` — **zero matches
  in `lib/`**; not defined anywhere (no `class X` found repo-wide in mobile).
- `reviewerId` / `reviewerUsername` / `reviewerAvatarUrl` / `reviewerLifecycle` /
  `verifiedPurchase` — zero matches on the rating `Rating`/`RatingApiResponse` types.
  (Reviewer fields found in `domains/system/report` are report-moderation `Appeal`/
  `ReviewHistory`, unrelated domain — protected.)
- `Rating.fromJson`/`RatingApiResponse.fromJson` — canonical, parse only
  `id/order_id/buyer_id/seller_id/rating_value/comment/created_at`. No `reviewer`, no
  `verified_purchase`, no state.
- Canonical CTA in production is state-`free`: `order_detail_screen.dart:253-257` uses
  `hasUserRatedOrderProvider(orderId:…, buyerId:…, sellerId:…)` (family). It does not use
  `RatingState` or the gate.

---

## CROSS_DOMAIN_IMPACT

| Domain | Production impact of deleting all residue | Test impact |
|---|---|---|
| Feed (`lib/features/home`) | None — no lib reference to `RatingState`, gate, or rejected DTOs. | `feed_root_wiring_test.dart` references `RatingListPage`/`RatingState`/`getRatingStateForOrder` (already non-compiling, also broken for non-rating reasons: `PrincipalOperationCheck`, `ApiClient.testing`, required `datasource` arg). |
| Follow (`lib/domains/social/follow`) | None. | `follow_status_provider_lifecycle_test.dart` `_FakeRatingRepository` (:218-246) references residue (already non-compiling). |
| Router (`lib/core/router`) | None. | `router_lifetime_preservation_test.dart` fake repo (:699-745) references residue (already non-compiling; also unrelated breakage). |
| Profile (`lib/domains/user/profile`) | None — `profile_reviews_tab.dart` uses only canonical `Rating`/`RatingSummary`; no `RatingReviewCard`, no `verifiedPurchase`. | `profile_reviews_tab_verified_purchase_test.dart` is 100% residue and non-compiling. |

Deleting residue does **not** require changing any Feed/Follow/Router/Profile **production**
behavior. It only touches those domains' *test fakes*, each already non-compiling.

---

## DELETE_CANDIDATES

Rating-domain owned, no cross-domain entanglement (safe for rating-owner decision):

1. `apps/mobile/test/domains/social/rating/rating_runtime_proof_test.dart`
2. `apps/mobile/test/domains/social/rating/rating_dto_mapper_test.dart`
3. `apps/mobile/test/domains/social/rating/rating_provider_disposal_proof_test.dart`
4. `apps/mobile/test/domains/social/rating/order_detail_rating_submit_proof_test.dart`
5. `apps/mobile/test/domains/commerce/transaction/order/presentation/screens/order_detail/order_rating_cta_gate_test.dart`

Coupled pair (must delete together; see BLOCKERS before touching `RatingState`):

6. `lib/domains/commerce/transaction/order/presentation/screens/order_detail/order_rating_cta_gate.dart`
7. `lib/domains/social/rating/domain/entities/rating_entity.dart` — the `RatingState` class (:247-250) only. `Rating` / `RatingSummary` / `RatingSummary` JSON are canonical and MUST stay.

Profile-owned file (parallel-owner decision required):

8. `apps/mobile/test/domains/user/profile/profile_reviews_tab_verified_purchase_test.dart`

---

## PROTECTED_LIST

Do not touch:

- `Rating`, `RatingSummary`, `Rating.fromJson`/`RatingSummary.fromJson` (rating_entity.dart) — canonical entities.
- `i_rating_repository.dart` — canonical `limit`/`cursor` interface (including parked `getRatingForOrder`).
- `rating_api_datasource.dart` / `rating_api_models.dart` / `rating_api_mapper.dart` / `rating_repository_api.dart` — converged canonical consumers.
- `rating_provider.dart` + `.g.dart` — `RatingListState`, `RatingNotifier`, `hasUserRatedOrderProvider` family — canonical, live, consumed by `order_detail_screen.dart`.
- `profile_reviews_tab.dart` / `rating_list_screen.dart` / `rating_card.dart` — canonical UI consumers.
- `order_detail_screen.dart` `hasUserRatedOrderProvider` usage (:253-257).
- Shared pagination helpers used by non-rating domains (`PaginatedApiResponse`, `executePaginatedRequest`, `paginationParams`) — used by `follow`, not rating residue.
- All backend, all migrations, all schemas, all routes.

---

## KILL_LIST

Covered by DELETE_CANDIDATES 1–8. All are ZOMBIE. All are non-compiling except the two
`lib/` files (`RatingState` + gate), which compile but are provably dead.

---

## UNKNOWN / BLOCKERS

**STOP conditions (defer to owners; no cleanup authorized by this audit):**

1. **`RatingState` deletion touches 4 unrelated-owner test suites.** Deleting the class
   adds undefined-type errors to `feed_root_wiring_test.dart`,
   `follow_status_provider_lifecycle_test.dart`, `router_lifetime_preservation_test.dart`,
   and `profile_reviews_tab_verified_purchase_test.dart` (all already non-compiling). Full
   cleanup therefore requires editing Feed/Follow/Router/Profile-owned test files in
   parallel — the "unrelated owner work would be touched" STOP from the convergence report
   still applies. Recommendation: rating owner removes items 6+7 and the coordinate of
   removing the residue in the four cross-domain suites.
2. **`profile_reviews_tab_verified_purchase_test.dart` is implicitly Profile-owned** — file
   sits under `test/domains/user/profile/`. Confirm Profile owner before deleting.
3. **Non-rating residual breakage in the same cross-domain files** (e.g.
   `PrincipalOperationCheck`, `ApiClient.testing`, `datasource` required arg,
   `AnalyticsRepository.trackEngagement`, `sellerIdentity`/`updatedAt`) belongs to those
   domain owners and is out of scope.
4. **No `UNKNOWN` artifacts** — every artifact was classified with named symbols, files,
   and analyzer counts. No parallel-owner *production* work is entangled.

---

## EXACT_PATHS_AND_SYMBOLS

Residue definitions:
- `apps/mobile/lib/domains/social/rating/domain/entities/rating_entity.dart:247-250` — `class RatingState { final bool canSubmit; }` (constructor accepts only `canSubmit`).
- `apps/mobile/lib/domains/commerce/transaction/order/presentation/screens/order_detail/order_rating_cta_gate.dart` — `shouldShowOrderRatingCta(RatingState?)`, `gateOrderDecisionForRating(DecisionContract, {required RatingState?})`, `_ratingActionType = 'rate'`.

Test files referencing residue (exact): as listed in DELETE_CANDIDATES 1–8.

Key reject-asserted symbols, all undefined in `lib/`:
`RatingListResponseDto`, `RatingListPage`, `RatingStateApiResponse`,
`RatingStateReasonApiResponse`, `RatingStateReason`, `RatingReviewerDto`,
`RatingReviewCard`, `getRatingStateForOrder`, `getOrderRatingStateProvider`,
`ReviewerInfo`, plus entity fields `reviewerId`, `reviewerUsername`, `reviewerAvatarUrl`,
`reviewerLifecycle`, `verifiedPurchase`, and JSON keys `reviewer`, `verified_purchase`,
`has_more`, `next_cursor`, `rating_state`, `can_submit`/`canSubmit` (rating).

Baseline canonical proof of absence:
- `RatingApiResponse` (rating_api_models.dart:51-115) has no `reviewer`.
- `Rating` (rating_entity.dart:15-122) has no reviewer/verifiedPurchase/state fields.
- `RatingListState` (rating_provider.dart:64-90) has only `ratings/isLoading/error/summary`
  — no `nextCursor/hasMore/cursorError`; `RatingNotifier` has no `reset()`.
- `RatingApiDatasource` (rating_api_datasource.dart) has no state method.
- `IRatingRepository` (i_rating_repository.dart:14-80) has no `getRatingStateForOrder`.

---

## TEST/ANALYZE EVIDENCE

`dart analyze <file>` (Dart SDK 3.11.5, cwd = `apps/mobile`), per-file error counts:

| File | Errors | Anchor errors |
|---|---|---|
| `rating_runtime_proof_test.dart` | 99 | `undefined_class RatingListResponseDto` (:41), `invalid_override` (:50:41), `RatingStateApiResponse` (:44), `.reviewer` (:143/:195), `getOrderRatingStateProvider` (:784) |
| `rating_dto_mapper_test.dart` | 36 | `undefined_getter reviewer` (:88-89), `undefined_name RatingListResponseDto` (:137-197) |
| `rating_provider_disposal_proof_test.dart` | 35 | `RatingListPage` non-type (:21-58), `invalid_override` (:39/:51), `undefined_named_parameter orderId` (:98), missing `getRatingForOrder` (:19) |
| `order_detail_rating_submit_proof_test.dart` | 6 | `RatingListPage` (:29-77), `override_on_non_overriding` (:111) |
| `order_rating_cta_gate_test.dart` | 8 | `undefined_named_parameter orderId/hasRating` (:53-89) |
| `feed_root_wiring_test.dart` | 16 | `RatingListPage` (:582), `override_on_non_overriding` (:606) |
| `follow_status_provider_lifecycle_test.dart` | 9 | `RatingListPage` (:234-246), `undefined_named_parameter orderId/hasRating` (:230) |
| `router_lifetime_preservation_test.dart` | 19 | `RatingListPage` (:720-721), `override_on_non_overriding` (:740) |
| `profile_reviews_tab_verified_purchase_test.dart` | 9 | `undefined_named_parameter verifiedPurchase/reviewer*` (:16-20), `undefined_function RatingReviewCard` (:33-50) |

Also verified:
- `dart analyze lib/.../rating_entity.dart lib/.../order_rating_cta_gate.dart` → **"No issues found!"** — the residue `lib/` code compiles; it is merely dead, NOT canonical-duplicative.
- `git status --porcelain` → clean; zero files modified by this audit.
- Full-repo grep for `class RatingListPage|class RatingReviewCard|class RatingStateApiResponse|class RatingReviewerDto|class RatingListResponseDto|class RatingStateReason` → **no files found** (mobile).

---

## RISK_SCORE

**5 / 30.**

Rationale:
- All residue is inert: no live authority, no canonical consumer, no producer, no data
  route. Production builds/analyzes clean; canonical consumers verified.
- Residual risk kept low-but-nonzero because (a) 9 test files are non-compiling and carry
  "proof"/"canonical" language asserting the *rejected* reviewer/state/verified_purchase
  surface — a future dev could mistake them for a spec and resurrect
  `reviewer`/`verified_purchase`/`RatingState`/`has_more`/`next_cursor`; mitigation is
  documentation + owner-approval cleanup, not yet deletion; (b) `RatingState` deletion is
  blocked by 4 cross-domain suites, so the surface remains present until owners act;
  (c) two production-shaped files (`RatingState`, the gate) are dead but still in `lib/`.
- Not scored higher: no financial/domain behavior, no routes, no schema, and the core
  authority (backend + converged mobile HTTP consumers) is already proven by
  `rating_http_contract_test.go` and `dart analyze` clean.

---

## FILES_CHANGED

**NONE**

## DATABASE_CHANGED

**NONE**

## MIGRATIONS_CHANGED

**NONE**

---

## FINAL_STOP

STOP — audit complete, zero modifications:

- Mobile Rating residue is exhaustively classified as **ZOMBIE**: `RatingState`
  (rating_entity.dart:247-250), `order_rating_cta_gate.dart`, 5 rating test files,
  `profile_reviews_tab_verified_purchase_test.dart`, and the residue in the feed/follow/
  router suites.
- **No artifact has a genuine canonical production consumer** — the only consumer of
  `RatingState` is the dead gate; the gate has zero callers; no provider/datasource/
  repository produces state data. (STOP rule 2: not triggered.)
- `Rating`/`RatingSummary`, the canonical consumers, `hasUserRatedOrderProvider`, and
  `profile_reviews_tab.dart` are protected and untouched. (STOP rule 1: not triggered —
  deleting residue changes no unrelated production behavior.)
- **STOP rule 3 IS triggered for cross-domain cleanup only:** removing the residue in
  Feed/Follow/Router/Profile test suites, or deleting the shared `RatingState` symbol
  without their coordination, edits parallel-owner work. Defer to owners. Rating-owner can
  proceed safely with DELETE_CANDIDATES 1–5 plus—with coordination—6–8.
- Do NOT make any of these broken tests compile by resurrecting `RatingState`, `reviewer`,
  `verified_purchase`, `RatingCursor`, `has_more`, `next_cursor`, or rating `page/page_size`.
- The authoritative Rating HTTP contract remains **CONVERGED**; this audit adds no change,
  no migration, no DB access.