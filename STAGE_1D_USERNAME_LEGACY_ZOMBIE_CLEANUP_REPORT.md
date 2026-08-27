# STAGE 1D — USERNAME LEGACY / ZOMBIE CLEANUP REPORT

## 1. VERDICT

**CLEANUP PERFORMED — BOUNDED, SCOPE-RESPECTING.**

The username legacy/zombie cleanup is complete for the mobile codebase, the
backend was audited and found to contain **zero** dead rename/reservation
mechanisms (the canonical design is already the only design), and stale
rename-advertising documentation was corrected to the canonical immutability
truth.

One pre-existing zombie test was found inside the username scope
(`user_handler_reserved_username_test.go`) plus one profile-scope test
(`user_handler_public_profile_not_found_test.go`) that blocked the whole
`internal/identity/user/delivery/http` package from compiling under
`-tags=integration`; both were repaired to the current constructor signature.

---

## 2. PRE-CLEANUP RESIDUE INVENTORY

### Backend — candidates found: 0 dead mechanisms

| Candidate | Definition | Callers | Routes | Verdict |
|---|---|---|---|---|
| Username rename/change service | none | — | — | did not exist |
| Username reservation endpoint | none | — | — | did not exist |
| `/usernames/reserve` | none | — | — | did not exist |
| `PATCH /users/:id/username` | none | — | — | did not exist |
| Duplicate reserved-name authority | `internal/identity/username/validation.go` is the SINGLE authority | auth handler, user handler, feed/search/commerce projections | — | canonical — KEEP |
| `USERNAME_TAKEN` / `RESERVED` / `IMMUTABLE` / `ALREADY_SET` | `auth_handler.go`, `user_handler.go` | integration tests | — | canonical — KEEP |
| `user_handler.go` immutability comment | "no rename capability anywhere" | — | — | canonical documentation — KEEP |
| `user_handler_reserved_username_test.go` | stale 4-arg `NewUserHandler` call | — | — | ZOMBIE — FIXED (blocked package compile) |
| `user_handler_public_profile_not_found_test.go` | stale 4-arg `NewUserHandler` call | — | — | ZOMBIE — FIXED (blocked package compile) |

### Mobile — candidates

| Candidate | Definition | Callers | Verdict |
|---|---|---|---|
| `UsernameManagementService` | `data/username_management_service.dart` | **0** (lib + test) | DEAD — DELETED |
| `reserveUsername` (service + datasource) | `username_management_service.dart` + `auth_api_datasource.dart` | only the dead service | DEAD — DELETED |
| `changeUsername` (service + datasource) | same | only the dead service | DEAD — DELETED |
| `getUsernameHistory` / `canChangeUsername` | dead service | only the dead service | DEAD — DELETED |
| `AuthApiDatasource.changeUsername` / `reserveUsername` | `auth_api_datasource.dart` | only the dead service; pointed at routes that do not exist on backend | DEAD — DELETED |
| `UsernameValidationService._reservedUsernames` | `username_validation_service.dart` | self-contained; actively FAILED the C1B3 contract | DEAD — REMOVED (list + reserved check) |
| `UsernameValidationService.validateUsernameFormat` divergent regex | same file (regex allowed `.`/`-`/uppercase) | `UsernameService` (CompleteProfile) | DUPLICATE VALIDATOR — now delegates to canonical |
| `shared/widgets/username_field.dart` (shared UsernameField) | exported via `shared.dart` | **0** production callers; sign-up uses the auth-domain field | DEAD — DELETED + export removed |
| `EditProfileValidators.validateUsername` | `edit_profile_validators.dart` | **0** (Edit Profile username is read-only) | DEAD — DELETED |
| Stale `changeUsername` doc comment | `auth_profile_repository.dart` | — | STALE — UPDATED |
| Stale datasource header (listed `/username`, `/usernames/reserve`) | `auth_api_datasource.dart` | — | STALE — UPDATED |

### Docs — candidates

| Doc | Problem | Verdict |
|---|---|---|
| `docs/flows/doctrine/username-lifecycle.md` | "Mutable but Governed" rename doctrine (cooldown, anti-handle-sniping, reservation windows) | STALE — REWRITTEN to canonical immutability |
| `docs/flows/foundation/manage-profile.md` | advertised username rename via Manage Profile | STALE — REWRITTEN |
| `docs/flows/foundation/complete-profile.md` | "cooldown rename berlaku di Manage Profile" | STALE — UPDATED |
| `docs/flows/foundation/sign-up.md` | "rename governance", "backend looser rules" | STALE — UPDATED |
| `docs/flows/cross-domain-relations.md` §17 | "Username Rename (cross-domain)" | STALE — REWRITTEN |
| `docs/flows/doctrine/README.md`, `docs/flows/foundation/README.md` | lifecycle doc blurb advertised rename rules | STALE — UPDATED |

---

## 3. EACH DELETED MECHANISM + PROOF IT WAS DEAD

### 3.1 `UsernameManagementService` (file deleted)
- **Proof:** `grep UsernameManagementService` across `apps/mobile` → 0 hits in
  lib and test. No provider registered it, no screen constructed it. Its
  `reserveUsername`/`changeUsername` called backend routes that **do not exist**
  in `routes_core.go` (verified: no `/usernames`, no `/users/:id/username`).

### 3.2 `AuthApiDatasource.changeUsername` / `reserveUsername` (methods deleted)
- **Proof:** only caller was the deleted service. Pointed at
  `PATCH /users/:id/username` and `POST /usernames/reserve` — neither route
  exists in the backend. They were dangling wire contracts.

### 3.3 `UsernameValidationService._reservedUsernames` (list + check removed)
- **Proof:** the C1B3 contract test asserted `moderator` (a former mobile-only
  reserved name, NOT backend-reserved) must pass local format and reach the
  remote check — and was **failing** because the local list still contained
  `moderator`. The list contradicted the canonical reserved authority
  (`identityusername.IsReserved`). Removing it makes the contract pass (10/10).

### 3.4 `UsernameValidationService.validateUsernameFormat` divergent regex (replaced)
- **Proof:** its regex `^[a-zA-Z0-9._-]+$` allowed `.`/`-`/uppercase, directly
  contradicting the canonical format `^[a-z0-9_]+$` (backend
  `identityusername.ValidateFormat`, mirrored by `CanonicalUsernameValidator`).
  The C1B3 test asserted `john-doe` must be invalid locally (zero remote calls)
  — only true when delegating to the canonical validator. Now delegates.

### 3.5 `shared/widgets/username_field.dart` (file deleted + export removed)
- **Proof:** zero production consumers. `sign_up_screen.dart` imports the
  auth-domain `UsernameField` and used `hide UsernameField` only to avoid the
  collision with this dead shared duplicate. `CompleteProfileScreen` uses its
  own `TextField` + `UsernameService`. No test constructed this widget either.

### 3.6 `EditProfileValidators.validateUsername` (method deleted)
- **Proof:** zero callers in lib or test. Edit Profile renders username
  read-only (`AbsorbPointer` + `enabled: false`) and the save handler omits
  username from the update payload; no edit path ever validated a username.

### 3.7 Stale docs (updated)
- **Proof:** all described a rename design (cooldown, anti-handle-sniping,
  reservation windows, seller-stricter rename) that has no implementation
  anywhere and contradicts the Stage 1A–1C proven canonical truth
  ("username is immutable after establishment; there is no rename capability
  anywhere in the codebase" — `user_handler.go` line 194-202).

---

## 4. ACTUAL FILES CHANGED

**Mobile (8 files):**
- DELETED `apps/mobile/lib/domains/user/identity/authentication/data/username_management_service.dart`
- DELETED `apps/mobile/lib/shared/widgets/username_field.dart`
- MODIFIED `apps/mobile/lib/domains/user/identity/authentication/data/datasources/auth_api_datasource.dart`
- MODIFIED `apps/mobile/lib/domains/user/identity/authentication/data/repositories/auth_profile_repository.dart`
- MODIFIED `apps/mobile/lib/domains/user/identity/authentication/data/username_validation_service.dart`
- MODIFIED `apps/mobile/lib/domains/user/identity/authentication/presentation/screens/sign_up_screen.dart` (removed `hide UsernameField`)
- MODIFIED `apps/mobile/lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_validators.dart`
- MODIFIED `apps/mobile/lib/shared/shared.dart` (removed dead export)

**Backend (2 files):**
- MODIFIED `backend/internal/identity/user/delivery/http/user_handler_reserved_username_test.go`
- MODIFIED `backend/internal/identity/user/delivery/http/user_handler_public_profile_not_found_test.go`

**Docs (7 files):**
- MODIFIED `docs/flows/doctrine/username-lifecycle.md`
- MODIFIED `docs/flows/foundation/manage-profile.md`
- MODIFIED `docs/flows/foundation/complete-profile.md`
- MODIFIED `docs/flows/foundation/sign-up.md`
- MODIFIED `docs/flows/cross-domain-relations.md`
- MODIFIED `docs/flows/doctrine/README.md`
- MODIFIED `docs/flows/foundation/README.md`

---

## 5. ROUTES / DTOs / SERVICES REMOVED

**Backend routes:** none existed; none removed. Verified `routes_core.go` has
only `GET /users/check-username`, `GET /users/me`,
`PATCH /users/me/profile`, `POST /users/me/verification/refresh`,
`DELETE /users/me`.

**Mobile services removed:**
- `UsernameManagementService` (whole class)
- `AuthApiDatasource.changeUsername`
- `AuthApiDatasource.reserveUsername`

**Mobile DTOs removed:** none — the dead datasource methods returned generic
`Map<String, dynamic>`; no dedicated rename DTO existed.

---

## 6. VALIDATORS / RESERVED LISTS REMOVED

- `UsernameValidationService._reservedUsernames` — removed (mobile-only list).
- `UsernameValidationService.validateUsernameFormat` divergent regex — removed;
  method now delegates to `CanonicalUsernameValidator` (the single mobile
  authority mirroring backend `identityusername`).
- `EditProfileValidators.validateUsername` — removed.

**KEPT (canonical):**
- `CanonicalUsernameValidator` (mobile authority).
- Backend `identityusername.Normalize / IsReserved / ValidateFormat`.
- Backend `USERNAME_TAKEN / USERNAME_RESERVED / USERNAME_IMMUTABLE /
  USERNAME_ALREADY_SET / USERNAME_INVALID_FORMAT / USERNAME_UNAVAILABLE`.
- `UsernameCheckStatus` / `UsernameCheckResult` (canonical UI state types).
- `UsernameService` + `usernameServiceProvider` (canonical CompleteProfile path).
- `UserApiDatasource.checkUsernameAvailability` + `UserSyncService` forwarder
  (canonical authenticated availability check).
- `ValidationService.validateUsername` (generic platform `IValidationService`
  primitive — not a rename mechanism).

---

## 7. TESTS / DOCS / COMMENTS REMOVED OR UPDATED

- **C1B3 test** (`c1b3_reserved_name_availability_contract_test.dart`): was
  failing 3/10 before cleanup (moderator local rejection, john-doe
  invalid-format, zero-remote-calls). Now passes **10/10** — the test was
  correctly asserting the intended post-cleanup contract, and the dead local
  list was the only thing breaking it. No test code change needed.
- **Backend zombie tests repaired** (stale 4-arg `NewUserHandler` → 3-arg):
  `user_handler_reserved_username_test.go`, `user_handler_public_profile_not_found_test.go`.
- **Docs updated** to canonical immutability (7 files, see §4).
- **Comments updated:** `auth_api_datasource.dart` header (removed
  `/username` + `/usernames/reserve` lines), `auth_profile_repository.dart`
  (removed `changeUsername` from migration list), `username_validation_service.dart`
  (documented no-local-reserved-list authority).

---

## 8. CANONICAL BEHAVIOR REGRESSION PROOF

**Register → canonical validation → Firebase → authenticated exchange(username)
→ backend assignment → authenticated session:**
- `registration_username_format_gate_test.dart` — 6/6 PASS
- `exchange_username_threading_test.dart` — 4/4 PASS
- `user_sync_username_threading_test.dart` — 2/2 PASS
- `auth_email_signup_listener_ordering_test.dart` — 12/12 PASS
- Backend `TestRegistrationUsername_ValidUsernameAssignedOnNewUser` — PASS (exit 0)

**Edit Profile → username read-only → no mutation:**
- `edit_profile_username_immutability_test.dart` — 6/6 PASS
- `username_only_identity_authority_test.dart` — 4/4 PASS
- Backend `TestUpdateMyProfile_RenameAttempt_RejectedWithUsernameAlreadySet`,
  `TestUpdateMyProfile_FirstTimeUsernameEstablishment_Succeeds`,
  `TestUpdateMyProfile_SameUsernameResubmitted_NotTreatedAsRename`,
  `TestCheckUsername_DoesNotMutateAnyState` — PASS (exit 0)

**USERNAME_TAKEN → corrected username → authenticated exchange retry → success
→ no Firebase recreation:**
- `registration_username_recovery_test.dart` — 2/2 PASS

---

## 9. TESTS RUN + EXACT RESULTS

### Flutter (focused username/auth/profile)
| Test | Result |
|---|---|
| `c1b3_reserved_name_availability_contract_test.dart` | **10/10 PASS** |
| `registration_username_format_gate_test.dart` | **6/6 PASS** |
| `registration_username_recovery_test.dart` | **2/2 PASS** |
| `exchange_username_threading_test.dart` | **4/4 PASS** |
| `auth_email_signup_listener_ordering_test.dart` | **12/12 PASS** |
| `user_sync_username_threading_test.dart` | **2/2 PASS** |
| `edit_profile_username_immutability_test.dart` | **6/6 PASS** |
| `username_only_identity_authority_test.dart` | **4/4 PASS** |
| `auth_api_datasource_exchange_test.dart` | **2/2 PASS** |

### Backend (Go, integration tag, real Postgres)
| Test | Result |
|---|---|
| `TestIsReserved_Labuda_IsReserved` | PASS (0.00s) |
| `TestCheckUsername_ReservedUsername_ReturnsUnavailable` | PASS (130.24s) |
| `TestCheckUsername_ValidNonReserved_ProceedsToUniquenessCheck` | PASS (120.80s) |
| `TestUpdateMyProfile_*` immutability suite (4 tests) | PASS (exit 0, 4m17s) |
| `TestRegistrationUsername_*` (2 tests) | PASS (exit 0) |

---

## 10. FLUTTER ANALYZE

`flutter analyze` on `apps/mobile` reports **1989 issues** — all pre-existing
baseline in unrelated domains (commerce, feed, chat, presence, search, avatar
test fixtures). **Zero issues in any file touched by this cleanup.** Verified by
grepping the full analyze log for every touched file path.

---

## 11. GO TEST / GO VET

- `go build ./internal/identity/... ./cmd/core_server/...` — **PASS** (clean).
- `go vet ./internal/identity/username/... ./internal/identity/auth/...` — **PASS** (clean).
- `go vet -tags=integration ./internal/identity/user/delivery/http/...` — **PASS** after
  the two zombie test constructor fixes.
- Pre-existing `go vet` failures in `internal/identity/address/application`
  and `internal/identity/user/application` (test files referencing removed
  symbols) are baseline working-tree issues unrelated to this cleanup — reported
  under §13.

---

## 12. FINAL SYMBOL / CALLER AUDIT

Searched across the entire repo:

| Symbol | Mobile lib | Mobile test | Backend | Docs |
|---|---|---|---|---|
| `UsernameManagementService` | 0 | 0 | — | 0 |
| `username_management_service` | 0 | 0 | — | 0 |
| `reserveUsername` | 0 | 0 | 0 | 0 |
| `changeUsername` | 0 | 0 | 0 | 0 |
| `_reservedUsernames` | 0 | 1 comment (C1B3, "after removing…") | — | 0 |
| `getUsernameHistory` / `canChangeUsername` | 0 | 0 | 0 | 0 |
| `/usernames/reserve`, `/users/:id/username`, `new_username` | — | — | 0 | 0 |
| rename governance / cooldown / anti-handle-sniping (docs) | — | — | — | 0 (only seller-verification retry cooldown remains, unrelated) |

**Confirmed:** no canonical caller remains for any deleted mechanism; no
route/DTO/schema/test/comment advertises the deleted username-rename design.

---

## 13. BASELINE FAILURES (pre-existing, NOT caused by this cleanup)

These failed before and after this cleanup, all in unrelated domains:

- Flutter: `auth_controller_principal_runtime_test.dart`,
  `auth_email_signup_behavioral_test.dart`, `auth_portal_protected_provider_blocking_test.dart`,
  `require_email_verification_gate_test.dart`, `signup_outcome_binding_test.dart`,
  `auth_signup_production_path_test.dart` (reference removed
  `AuthStateRequiresEmailVerification` / `BackendSyncOutcome` — a different
  concurrent change); `profile_post_save_audit_test.dart`,
  `edit_profile_canonical_identity_contract_test.dart` (seller store/farm
  domain); 1989 analyze issues (commerce/feed/chat/presence/search).
- Go: `internal/identity/address/application/address_service_primary_test.go`,
  `internal/identity/user/application/public_profile_identity_projection_test.go`
  (vet errors referencing unrelated removed symbols).

None of these reference the deleted username-rename mechanisms; they are
"unrelated divergent tests" per the NO GLOBAL CLEANUP rule.

---

## 14. REMAINING OPEN ISSUE: `seller_upgrade_wizard_screen` username consumer

`seller_upgrade_wizard_screen.dart` is a pre-existing consumer of
`updateProfile(username:)`. Per the stage instructions, this file was **NOT
touched**. Although established usernames cannot be renamed by the backend
(immutability enforced), this consumer requires a separate seller-domain
audit/decision. **Reported as an open issue — not addressed in Stage 1D.**

---

## 15. OUT OF SCOPE (untouched)

- Commerce, Product/FPS/Auction, quantity, social, feed, chat, search,
  startup/server-unavailable UX, password UX.
- Seller upgrade wizard (`seller_upgrade_wizard_screen.dart`).
- Unrelated divergent tests and analyze failures (see §13).
- No schema changes made.
- No other domain involved.

---

## 16. CLEANUP VERDICT

**BOUNDED CLEANUP COMPLETE.** The username legacy/zombie design
(rename/reserve services, dangling datasource methods, the mobile reserved
list, the divergent local validator, the dead shared UsernameField, the dead
Edit-Profile username validator, and the rename-advertising documentation) no
longer exists in the active codebase. The canonical registration, Edit-Profile
read-only, and USERNAME_TAKEN recovery behaviors are proven intact by the
focused test suites and backend integration tests. No STOP condition was
triggered; the only cross-domain consumer encountered is the documented open
seller-domain issue.
