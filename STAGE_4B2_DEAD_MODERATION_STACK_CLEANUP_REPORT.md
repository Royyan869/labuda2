# STAGE 4B-2 — DEAD CLIENT-SIDE MODERATION / ANTI-CIRCUMVENTION CLEANUP

## STAGE 4B-2 FINAL REPORT

### Verdict

**IMPLEMENTED + PROVEN**

The client-side moderation / anti-circumvention stack identified in Stage 4A
was audited component-by-component, proven dead, and removed in a bounded
fashion. `ValidationService` was kept and slimmed (its dead moderation hook
removed; canonical email/phone/URL delegation preserved). The backend
moderation domain (the canonical authority) was not touched. All focused and
regression tests pass; `flutter analyze` on the touched scope is clean; the
final residue audit finds zero references to any deleted symbol.

---

### Audit evidence (read-only, before any change)

Full symbol sweep of `apps/mobile` (lib + test + tool) for every
moderation-stack symbol produced exactly this dependency map:

- **`AntiCircumventionService`** (`lib/shared/services/anti_circumvention_service.dart`):
  `@Deprecated('Move to features/moderation/ ...')`, `_trackCircumventionAttempt`
  is an empty no-op body, **not exported from any barrel** (`shared.dart:155-158`
  says "REMOVED"), **zero references outside its own file**.
- **`BasicAntiCircumventionService`** (`lib/core/src/services/basic_anti_circumvention_service.dart`):
  exported only by `core.dart:76` ("Security Services exports"); **zero
  production consumers** — no instantiation, no provider, no `ref.read`, no
  constructor reference anywhere.
- **`IAntiCircumventionService`** (`lib/core/src/interfaces/services/i_anti_circumvention_service.dart`):
  consumed only by the two dead implementations and by
  `ValidationService._antiCircumventionService`; **no DI registration,
  no provider, no other consumer**.
- **`IContentModerationService`** (`lib/core/src/interfaces/services/i_content_moderation_service.dart`):
  **zero implementations** (`implements` search = none), exported only by
  `core.dart:24`.
- **`ValidationService` moderation hook**: `_antiCircumventionService` field +
  `validateContent` branch. Constructor is always invoked with **no argument**
  (`main.dart:150`: `ValidationService()`), so the field is always null.
  `validateContent`/`validateForm` have **zero production callers** (the only
  `validateContent` call is `comment_repository_impl.dart:134`, a local
  length-check — a different path).
- **Backend moderation** (`backend/internal/governance/moderation/`): ACTIVE
  canonical authority — real `ModerationService`, handlers, workers,
  repository. Out of scope, untouched.
- The only remaining "Circumvention" identifiers in the repo belong to the
  **analytics domain** (`AnalyticsCircumventionStats` /
  `getCircumventionStats` in `IAnalyticsRepository` +
  `firebase_analytics_repository_impl.dart`) — a separate concept, not part of
  this stack, untouched.

---

### Component-by-component status

| Component | Status | Decision |
|---|---|---|
| `AntiCircumventionService` (deprecated impl) | **DEAD** | Deleted |
| `BasicAntiCircumventionService` (in-memory impl) | **DEAD** | Deleted |
| `IAntiCircumventionService` (interface) | **DEAD** | Deleted |
| `IContentModerationService` (zombie interface) | **DEAD** | Deleted |
| `core.dart` exports (lines 24, 25, 76) | DEAD wiring | Removed |
| `shared.dart` stale comment block | stale doc | Removed |
| `ValidationService` circumvention field + `validateContent` hook | DEAD hook | Removed (service kept) |
| `ValidationService` canonical email/phone/URL delegation | **ACTIVE** | Preserved |
| Backend moderation domain | **ACTIVE (canonical)** | Untouched |

No component was classified UNCERTAIN — every one had direct grep proof.

---

### Actual files changed / deleted

Deleted (4):
1. `lib/shared/services/anti_circumvention_service.dart`
2. `lib/core/src/services/basic_anti_circumvention_service.dart`
3. `lib/core/src/interfaces/services/i_anti_circumvention_service.dart`
4. `lib/core/src/interfaces/services/i_content_moderation_service.dart`

Modified (3):
5. `lib/core/core.dart` — removed the 2 interface exports + the security-services export of the dead impl.
6. `lib/shared/shared.dart` — removed the stale "REMOVED: anti_circumvention_service.dart" comment block.
7. `lib/shared/services/validation_service.dart` — removed the `IAntiCircumventionService` import, the `_antiCircumventionService` field, the constructor param (now `const ValidationService()`), the circumvention branch inside `validateContent`, and updated the doc comment. Canonical email/phone/URL delegation and all other methods untouched.

No tests were deleted (no test referenced the stack — the only
`Circumvention` test identifiers are the analytics-domain mocks, which are
unrelated and pre-existing).

---

### Tests / proof

| Run | Result |
|---|---|
| `flutter test test/shared/helpers test/shared/services/validation_service_delegation_test.dart test/domains/user/preference/onboarding` | **+143 All tests passed** |
| `flutter test test/domains/social/comment` | **+28 All tests passed** |
| `flutter analyze lib/shared/services/validation_service.dart lib/core/core.dart lib/shared/shared.dart lib/main.dart` | **No issues found** |
| `flutter analyze lib` (full) | 88 issues — **identical pre-existing baseline set** (Chat/Commerce/Share/verify-email from other sessions), **none reference the deleted symbols** |

The validation delegation test still passes, proving `ValidationService`
remains a working thin delegation seam for email/phone/URL after the hook
removal.

---

### Residue audit (post-deletion)

- Symbol search for every deleted identifier
  (`IAntiCircumventionService`, `AntiCircumventionService`,
  `BasicAntiCircumventionService`, `IContentModerationService`,
  `CircumventionCheckResult`, `CircumventionContext`, `CircumventionType`,
  `CircumventionAction`, `CircumventionStats`, `AntiCircumventionMessages`,
  `CircumventionPatterns`, `anti_circumvention`, `i_content_moderation_service`,
  `i_anti_circumvention_service`, `basic_anti_circumvention`) across the whole
  mobile app: **zero matches** — the only "Circumvention" hits are the
  analytics-domain `AnalyticsCircumventionStats`/`getCircumventionStats`,
  which are unrelated.
- Imports/exports: no dangling import of the deleted files (full-lib analyze
  would surface undefined_class errors otherwise — none appeared).
- DI/providers: none ever registered these services; nothing to remove.
- Tests/mocks: no test referenced the stack.
- Docs: the stale `shared.dart` comment was removed; no other doc referenced it.
- Backend moderation: unchanged (see boundary confirmation).

---

### Baseline failures (genuinely pre-existing)

The 88 full-lib analyze issues and the ~16 test files that fail to load are
the same pre-existing baseline breakage documented in Stages 3B and 4B-1
(removed APIs like `AuthState.requiresEmailVerification`,
`PrincipalOperationCheck`, `ApiClient.testing`, `MediaViewerVideoEngineBuilder`,
`createCommerceReference`, etc. from other sessions' in-flight work). None
reference Stage 4B-2 files or symbols. Recorded, not repaired (doctrine).

---

### Out of scope — confirmed untouched

- **Backend moderation** (`backend/internal/governance/moderation/`): the
  canonical authority — not modified (the `M` entries for those files in the
  working tree are the pre-existing dirty baseline from other sessions, not
  this stage).
- Username, password, email/phone/URL canonical validators (Stage 4B-1
  deliverables intact — `validation_service_delegation_test` proves it).
- Commerce / Product / FPS / Auction / quantity / product discovery.
- Auth architecture.
- No global cleanup; no other files touched.

---

### Open issues

- None. No contradiction with business truth was found; no new client-side
  moderation architecture was introduced; the backend remains the sole
  moderation authority.
- The analytics-domain `AnalyticsCircumventionStats` naming is a separate,
  pre-existing concern and out of scope (noted for awareness only).

---

### Exact stopping point

Stage 4B-2 complete and proven.
No Stage 4C or other domain work was started.
Awaiting instruction before any next stage.
