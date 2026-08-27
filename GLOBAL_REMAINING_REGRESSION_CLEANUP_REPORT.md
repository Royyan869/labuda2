# GLOBAL REMAINING REGRESSION CLEANUP REPORT

Executor mission. Ran sequentially: PHASE 1 baseline → PHASE 2 investigation → PHASE 3 safe
convergence → PHASE 4 residue sweep → PHASE 5 regression proof → PHASE 6 final global sweep.
No rollback, no checkout, no cherry-pick. Current filesystem was the only authority.

---

## VERDICT

**MOBILE_CFE_UNBLOCKED_BACKEND_LOCKED_DOMAINS_GREEN_WIP_REMAINS**

Precisely:
- The two known **mobile** regressions that blocked the Flutter test runner (stale
  generated `feed_dto.g.dart` + missing localization keys) are **converged and proven**:
  the Rating test suite went from `10 pass / 3 failed-to-load` to **30/30 pass**.
- All **locked-domain** backend contract suites (rating, order, finance incl. withdrawal
  idempotency + refund-history, pricing token, integration/payment, wallet, dispute) are
  **vet-clean and test-green**.
- The remaining failures are **pre-existing, unrelated-domain drift** (10 backend
  test-drift packages incl. two in-flight WIP domains — worker, chat — plus ~662 broken
  mobile test files). These are NOT actionable without their domain owners; they are
  classified, protected, and reported below. No new business/accounting contradiction was
  discovered that requires stopping (the one candidate — a P+S+C release fallback — was
  confirmed **PROTECTED** by the prior financial convergence report, see below).

---

## CANONICAL_CONTRACT

Protected and verified unchanged:

**Financial (locked, NOT reopened):**
- Pricing token = canonical financial snapshot authority; `PD = Subtotal − Discount`;
  `BuyerBase/EscrowAmount = PD + Shipping`.
- Commission is seller/platform-side allocation; buyer cash excludes commission.
- Seller entitlement = `BuyerBase − Commission`; platform revenue = `PaymentFee + Commission`.
- Coin K funded `PLATFORM_BANK → GATEWAY_CLEARING`; consumption `RESERVE → CONSUME`;
  `GATEWAY_CLEARING` never over-debited.
- Rejected DEAD: `P+S+C` / `P+S+C−D` escrow, `CalculateGrossEscrowFromSnapshot`,
  `LegacyGross`, `orders.discount_amount/escrow_amount/coins_used` as financial authority.
- `order_payment_service.go:326-331` — the P+S+C **legacy-row release fallback** fires only
  when `total_before_coins_amount <= 0`, matches the escrow row funded under the old model,
  and is **explicitly PROTECTED** by `GLOBAL_FINANCIAL_AUTHORITY_FINAL_CLEANUP_REPORT.md`
  (§7, §9; STOP RULE 7 — do not remove without a pre-convergence-row migration strategy).
  Unreachable for new orders. NOT residue; per fleet decision it must not be touched.

**Rating (locked):** 4 routes; snake_case; bare list; `limit`/`cursor int64`; raw
`buyer_id/seller_id`; no `GetRatingState`; no reviewer/verified_purchase/opaque cursor/
`has_more`/`next_cursor`; no `page/page_size`.

**Mobile Rating (locked):** datasource/repository use `limit`/`cursor`; bare collection
parsing; `RatingState` + CTA gate + stale reviewer/verified_purchase/has_more models are
DEAD; cross-domain fakes converged to `IRatingRepository`.

---

## CHANGES

Smallest correct convergence, filesystem (committed source) as authority. No new behavior.

1. **`apps/mobile/lib/features/home/data/dto/feed_dto.g.dart`** (gitignored generated
   artifact) — **regenerated** via
   `dart run build_runner build --build-filter='lib/features/home/data/dto/feed_dto.g.dart'`.
   The on-disk `.g.dart` was stale: it referenced `visibility` (`_$FeedItemDtoFromJson` /
   `_$FeedItemDtoToJson`) that the committed source `FeedItemDto` no longer declares —
   a hard CFE compile error whenever any test linked the provider/core closure. Regeneration
   produced exactly `-2 lines` (the two `visibility` references), matching the committed
   source contract. Unblocked the entire Flutter test runner.

2. **`apps/mobile/lib/l10n/app_en.arb` + `apps/mobile/lib/l10n/app_id.arb`** — restored
   12 localization keys that the working-tree l10n migration had dropped from the generated
   output while the committed UI still references them. Keys added (en / id values taken
   verbatim from the committed HEAD generated files):
   `noActiveSessions`, `unknownDevice`, `revokeSession`, `revokeSessionTitle`,
   `revokeSessionMessage`, `signOutAllDevices`, `signOutAllDevicesTitle`,
   `signOutAllDevicesMessage`, `sessionRevokedSuccess`, `allSessionsRevokedSuccess`,
   `failedToLoadSessions`, `lastActive`. These are the 12 live getters used by
   `apps/mobile/lib/domains/user/identity/authentication/presentation/screens/login_sessions_screen.dart`
   (`title: l10n.revokeSessionTitle`, `l10n.lastActive`, etc.). The 4 `commission*` getters
   that HEAD had are confirmed **unused anywhere** in `lib/` — correctly dropped by the
   regen; not restored.

3. **`apps/mobile/lib/generated/app_localizations.dart` + `_en.dart` + `_id.dart`** —
   regenerated via `flutter gen-l10n` (exit 0) against `l10n.yaml`
   (`arb-dir: lib/l10n`, `output-dir: lib/generated`, `output-class: AppLocalizations`).
   Getters went 382 → 394 (the 12 restored); commission getters remain absent (0).
   `flutter analyze` clean on the regenerated files.

No backend code changed. No migration/schema/DB change.

---

## DELETED

Nothing deleted this mission. (Prior missions deleted the Rating/DTO/zombie surface; those
deletions remain valid and were re-verified in the sweep: `RatingState`, CTA gate,
`RatingListResponseDto`/`RatingReviewerDto`/`RatingCursor` surface, rich rating DTOs, stale
rating tests — zero references.)

---

## PROTECTED

- `backend/internal/commerce/order/application/order_payment_service.go:318-337`
  `ReleaseGatewayEscrowToSeller` — canonical gross `TotalBeforeCoinsAmount`,
  `sellerNet = gross − commission`, `commission > gross` guard, and the **legacy P+S+C
  fallback (:326-331)** (see VERDICT/protection).
- Backend locked estates: `internal/finance/**` (`finance_service.go`, `refund_*`,
  `verifier.go`, `withdraw_request*`), `internal/pricing/token/**`
  (`pricing_token_service.go` `escrowAmount = (P−D)+S`), `internal/commerce/order/rating/**`
  (`OrderRating`/`RatingSummary` + `rating_http_contract_test.go`), `internal/integration/payment/**`,
  `canonical_finalization_service.go`, `coins_service.go`, recon/*, `internal/core/wallet/**`,
  `internal/governance/dispute/**`.
- Mobile canonical rating consumers (`rating_api_datasource.dart`, `rating_repository_api.dart`,
  `rating_api_models.dart`, `rating_api_mapper.dart`, `i_rating_repository.dart`,
  `rating_provider.dart`), `profile_reviews_tab.dart`, `order_detail_screen.dart`,
  `order_detail_handlers.dart`.
- Feed/Follow/Router/Profile production code.
- `lib/generated/app_localizations*.dart`, `lib/l10n/app_en.arb`, `lib/l10n/app_id.arb`,
  `l10n.yaml` — canonical l10n contract (regenerated).

---

## CLASSIFICATION

Every investigated failure, with A/B/C/D/E per mission taxonomy:

| # | Issue | Evidence | Class |
|---|---|---|---|
| 1 | `feed_dto.g.dart` stale (`visibility`) | gitignored generated; source `feed_dto.dart` has no `visibility`; CFE broke every provider/core test | **B** — pre-existing but actionable → **CONVERGED** |
| 2 | missing l10n session keys | working-tree regen dropped 12 keys; committed `login_sessions_screen.dart` uses them; HEAD generated had them | **B** — pre-existing but actionable → **CONVERGED** |
| 3 | mobile CFE blocked tests (rating 3×, follow, etc.) | same two root causes (#1,#2) | **B** — root-cause fixed → **RESOLVED** (rating suite 30/30) |
| 4 | withdrawal idempotency (`finance/application/withdraw_request*`) | `go vet ./internal/finance/...` exit 0; `go test ./internal/finance/application/ -run Withdraw` ok | **A-none** — clean, no action |
| 5 | refund-history contracts (`finance/refund/**`, `order_refund_history_contract_test.go` deleted pre-existing) | `go vet ./internal/finance/refund/...` clean; `go test ./internal/finance/refund/...` all ok | **A-none** — clean |
| 6 | worker content-mention/alert WIP | untracked tests reference NON-existent `events.EventContentMentioned` (not in `internal/platform/events/events.go`), `handleContentMentioned`, `ContentMentionedPayload`, `SetContentVisibilityChecker`; no production producer; untracked migrations 000042/000043 | **D** — unrelated parallel WIP; authority not proven → STOP, untouched |
| 7 | chat projection (`chat_room_event_resource_projection_test.go`; `-tags integration` proof tests) | committed `Service` (chat_service.go:78-90) has NO `resourceAuthorizer`, NO `SetResourceProjectionResolver`, `SendMessage` is 7-arg (:772); committed test references all three + 10-arg SendMessage; integration proof tests expect 12-arg `NewService`/`fallbackBuilders` field. Deep conflicting committed artifacts | **D/UNKNOWN** — in-flight chat convergence, no proven authority → STOP, untouched |
| 8 | rating (live errors) | `go vet ./internal/commerce/order/rating/...` exit 0; `go test .../rating/...` ok; flutter rating 30/30 | **A-none** — clean |
| 9 | comment domain (mobile + backend) | working-tree modified comment source/tests (mobile `comment_api_datasource.dart`, `comment_repository_impl.dart`, `comment_notifier.dart`; backend `comment_service.go`, new `comment_response.go`, `comment_repository_impl.go`, etc.); mobile errors `listComments`/`getCommentCount` undefined; files written mid-session (external actor) | **D** — active parallel WIP → STOP, untouched |
| 10 | other `go vet ./...` breakage | auction(2 pkgs), fixedprice(2), shipping, governance/evaluator, moderation(2), identity/address, identity/user(2), platform/mediaupload — all TEST files referencing removed/changed struct fields/methods of their committed sources | **D** — unrelated-domain stale tests; owner convergence needed → untouched |
| 11 | mobile ~662 test files / 1666 errors | `flutter analyze`: 1666 errors / 226 warnings / 99 infos across chat, commerce, auth, content, home, profile, shared, etc.; NONE in files touched by this or prior rating cleanup | **D** — pre-existing repo-wide test drift → untouched, reported |
| 12 | `order_payment_service.go` P+S+C fallback | fires only when `total_before_coins_amount <= 0`; protected by financial convergence report (STOP RULE 7) | **PROTECTED** — deliberately kept, NOT residue, NOT a new contradiction |
| 13 | dead financial columns (`orders.escrow_amount/discount_amount/coins_used`) | never written by production; `coins_refund_handler.go` explicitly ignores `order.coins_used`; projection SELECT reads `o.total_before_coins_amount` (:785); `order_summaries.escrow_amount` is a read-model column name only | **clean** — no authority use; converged |
| 14 | `gs.txt`, `.commandcode/*`, root scratch files | stray wrapper output / tool config, untracked, non-code | **E→D** — UNKNOWN untracked artifacts; ignored |

---

## TESTS

Exact commands + results (exit codes):

| Command (cwd) | Exit | Result |
|---|---|---|
| `go build ./...` (backend) | 0 | full backend builds |
| `go test ./internal/commerce/order/rating/... ./internal/finance/application/ -run 'Rating\|Withdraw'` | 0 | rating app/http/entity ok; finance app ok |
| `go test ./internal/commerce/order/... ./internal/finance/... ./internal/pricing/token/... ./internal/integration/payment/...` | 0 | all locked-domain suites ok |
| `go vet ./internal/finance/...` | 0 | finance clean |
| `go vet ./internal/commerce/order/rating/...` | 0 | rating clean |
| `go vet ./internal/commerce/order/... ./internal/integration/payment/... ./internal/pricing/token/... ./internal/core/wallet/... ./internal/governance/dispute/...` | 0 | order+payment+token+wallet+dispute clean |
| `go vet ./...` | 1 | fails ONLY in the 10 unrelated/WIP package test-drift list (#6,#7,#10) |
| `dart run build_runner build --build-filter='lib/features/home/data/dto/feed_dto.g.dart'` | 0 | regenerated `feed_dto.g.dart` (−2 `visibility` refs) |
| `flutter gen-l10n` | 0 | `lib/generated/app_localizations*` regenerated; 1 (pre-existing?= commission-adjacent) untranslated message note only |
| `flutter test test/domains/social/rating/ --reporter compact` | 0 | **00:18 +30: All tests passed!** |
| `dart analyze lib/domains/user/identity/authentication/presentation/screens/login_sessions_screen.dart lib/features/home/data/dto/feed_dto.dart` | 0 | No issues |
| `flutter analyze` (whole project) | 1 | 1666 errors / 226 warnings / 99 infos — ALL in pre-existing unrelated test files; **zero in rating/l10n/feed-dto files touched** |

Prior-blocked Rating tests that now execute and pass: `rating_runtime_proof_test.dart`,
`rating_provider_disposal_proof_test.dart`, `order_detail_rating_submit_proof_test.dart`
(previously `Failed to load … feed_dto.g.dart / app_localizations`), plus the already-green
`rating_dto_mapper_test.dart`.

---

## REGRESSION

- **Proven by tests:** locked financial contract (pricing token, PD+S escrow, gross release,
  refund math, withdrawal idempotency, coin settlement), locked Rating HTTP contract, and
  mobile Rating datasource/repository/provider — all green.
- **Proven by changes:** mobile CFE unblocked — every provider-touching test compiles;
  rating suite 30/30; login_sessions screen and feed DTO analyze clean; no file related to
  this cleanup has an analyzer error.
- **Remains pre-existing / owner work (NOT caused by, and NOT fixable by, this mission):**
  - Backend `go vet ./...` red in worker (untracked content-mention WIP), chat (conflicting
    committed projection artifacts), and 8 unrelated-domain packages whose committed tests
    drifted from their committed sources.
  - Mobile: ~662 test files across every domain carry pre-existing source/spec drift; the
    comment domain is mid-WIP (modified today by an external actor).
- Earlier "analyzer clean" claims in prior reports were based on a truncated/mangled capture
  of `flutter analyze`; the true repo-wide baseline is the 1666-error drift above. This
  mission's re-run establishes the honest baseline and proves the cleanup-added delta is zero.

---

## RESIDUE_STATUS

Repo-wide semantic sweep (final):

- **Rating (mobile + backend, live code):** zero. Only intentional negative assertions:
  `rating_runtime_proof_test.dart` ("NO GetRatingState…", "NO RatingCursor"),
  `rating_dto_mapper_test.dart` (`containsKey('verified_purchase')` false), and backend
  `order_rating.go:26` + `rating_http_contract_test.go` anti-proofs.
- **Financial:** rejected formulas `CalculateGrossEscrowFromSnapshot` / `LegacyGross` /
  `P+S+C−D` are gone; remaining `P+S+C` strings are anti-proof comments plus the one
  PROTECTED legacy fallback. `orders.escrow_amount/discount_amount/coins_used` are never
  written and never read as financial authority; projection reads `total_before_coins_amount`.
  `order_dto.dart` (mobile) already dropped `sellerCommission`/`sellerPayout`.
- **l10n:** 394 getters; 12 session keys live; commission getters absent (unused) — correct.
- **feed DTO:** on-disk `.g.dart` matches committed source.
- Stray non-code files (`gs.txt`, scratch) noted as untracked; `.commandcode/*` is tool
  configuration — untouched.

---

## FILES_CHANGED

Tracked (exact):
1. `apps/mobile/lib/l10n/app_en.arb`
2. `apps/mobile/lib/l10n/app_id.arb`
3. `apps/mobile/lib/generated/app_localizations.dart`
4. `apps/mobile/lib/generated/app_localizations_en.dart`
5. `apps/mobile/lib/generated/app_localizations_id.dart`

Gitignored / generated artifact regenerated (not a tracked change, but the effective fix):
6. `apps/mobile/lib/features/home/data/dto/feed_dto.g.dart`

Files observed modified by an external/parallel actor mid-session (comment-domain WIP) —
**NOT touched by this mission**: `apps/mobile/lib/domains/social/comment/presentation/providers/comment_providers.dart`,
`backend/internal/social/content/application/comment_response.go`,
`backend/internal/social/content/infrastructure/repository/{comment_author_lifecycle_test,comment_repository_addition,comment_repository_impl}.go`,
`backend/internal/social/content/repository/comment_repository.go`.

## DATABASE_CHANGED

**NONE**

## MIGRATIONS_CHANGED

**NONE** (untracked worker migrations 000042/000043 belong to the parallel WIP; not applied,
not modified).

---

## RISK_SCORE

**5 / 30**

Rationale: the two fixes are minimal and fully reversible (2 `.arb` key-block restorations
taken verbatim from committed HEAD generated output; regeneration of already-generator-owned
files; the gitignored `.g.dart` realignment). They unblock the mobile test runner and are
proven by 30 passing rating tests plus clean analysis of every touched file — net risk of
the change itself is low (~1/30). The score is held up because the repository-wide baseline
is heavily dirty (backend `go vet ./...` red in 10 packages, two in-flight WIP domains,
~662 broken mobile test files) which can mislead future tooling into believing convergence
is complete; that residue is pre-existing, classified, owner-owned, and — because the
authority is not proven for those domains — deliberately untouched rather than guessed.

---

## FINAL_STOP

The repository is **partially clean**: every locked financial/rating contract domain is
build/vet/test green and the mobile Flutter test runner is unblocked with the targeted
Rating suite passing 30/30. The mission's named actionable regressions (stale generated feed
DTO, missing localization keys) are converged; withdrawal-idempotency and refund-history are
verified green; live rating surface is proven residue-free.

**Concrete remaining blockers (parallel owner work, not actionable here):**
1. `internal/worker` — untracked content-mention/alert WIP tests reference undefined
   `events.EventContentMentioned` / `handleContentMentioned` / `ContentMentionedPayload` /
   `SetContentVisibilityChecker`; no production producer exists. Owner must either finish
   the feature (new event + handler + visibility checker + migrations) or drop the WIP.
2. `internal/interaction/chat` — committed proof/integration tests expect a newer
   occurrence-aware architecture (10-arg `SendMessage`, `fallbackBuilders` field, 12-arg
   `NewService`) than the committed 7-arg `Service`; plus a stale unit test referencing
   removed wiring. Chat owner must reconcile committed source vs committed proof tests.
3. Eight unrelated backend packages + ~662 mobile test files carry pre-existing
   source-test drift; require their domain owners, not this mission.
4. Comment domain is actively mid-WIP (external actor modified sources mid-session).

STOP — no further in-scope work remains; no new business/accounting contradiction was
discovered; the protected P+S+C legacy fallback and all locked contracts were left intact.