# STAGE 4G — BOUNDED CLEANUP REPORT (DEAD LEGACY PATH REMOVAL)

**MODE:** BOUNDED CLEANUP ONLY — no audit of other domains.
**PREVIOUS EVIDENCE:** `STAGE_4G_PROFILE_UPDATE_LEGACY_PATH_AUDIT.md` (both targets proven DEAD).
**CANONICAL PATH PRESERVED:**
`edit_profile_save_handler` → `profileActionsProvider` → `ProfileActions.updateFields` → `UpdateProfileUseCase.call()` → `repository.updateProfile()` → PATCH `/users/me/profile`.

---

## 1. VERDICT

**CLEANUP COMPLETE — TWO DEAD LEGACY PATHS REMOVED — CANONICAL PATH INTACT**

---

## 2. Exact files changed

| File | Change |
|------|--------|
| `apps/mobile/lib/domains/user/profile/domain/use_cases/update_profile_use_case.dart` | Removed `updateFields` method + its doc comment (lines ~32–56). |
| `apps/mobile/lib/domains/user/profile/presentation/providers/notifiers/profile_notifier.dart` | Removed `updateFields` method + doc comment (lines ~73–115). |
| `apps/mobile/test/domains/user/profile/domain/use_cases/update_profile_use_case_test.dart` | **DELETED** (entire file only tested the dead `UpdateProfileUseCase.updateFields`). |
| `apps/mobile/test/domains/user/profile/profile_post_save_audit_test.dart` | Removed the `updateFields` override from `_NoopUpdateProfileUseCase` (residue fix so it keeps compiling against the interface). Canonical `ProfileActions.updateFields` coverage untouched. |

No other files changed.

---

## 3. Exact symbols deleted

- `UpdateProfileUseCase.updateFields(String userId, Map<String, dynamic> fields)` — `update_profile_use_case.dart`.
- `ProfileNotifier.updateFields(String userId, Map<String, dynamic> fields)` — `profile_notifier.dart`.
- Test-only: `update_profile_use_case_test.dart::main` group `UpdateProfileUseCase cover reference validation` (entire file).
- Test-only residue: `_NoopUpdateProfileUseCase.updateFields` override.

NOT deleted (kept canonical/required):
- `UpdateProfileUseCase.call()` → still used by `ProfileActions.updateProfile`.
- `UpdateProfileUseCase._validateProfile` / `_validateFarmInfo` → still used by `call()`.
- `ProfileNotifier.updateProfile` / `updateFarmInfo` / `fetchProfile` / `searchProfiles`.
- `ProfileActions.updateFields` (canonical).

---

## 4. Caller / consumer audit after deletion

**Production callers of removed symbols:** ZERO (confirmed by residue grep — see §10).
- `UpdateProfileUseCase.updateFields` had no production caller before/after.
- `ProfileNotifier.updateFields` had no production caller before/after.
- `ProfileActions.updateFields` remains the sole live entry; still invoked by `edit_profile_save_handler.dart:134`.

**Provider / DI references:**
- `updateProfileUseCaseProvider` (`profile_core_provider.dart:40`) still exists and is still consumed by `profileActionsProvider`; only `call` is used. Removal of the dead method does not affect DI.
- `profileNotifierProvider` (alias of generated `profileProvider`) still exists; only watched for state. Removal of the dead method does not affect DI.

**Tests:**
- `update_profile_use_case_test.dart` removed entirely (only referenced the dead method).
- `profile_post_save_audit_test.dart` still covers the canonical `ProfileActions.updateFields` via `_FakeProfileActions.updateFieldsCalls`. `_NoopUpdateProfileUseCase` no longer carries a dead override but still satisfies `implements UpdateProfileUseCase` via `call`.

**Imports:**
- `update_profile_use_case.dart`: no new unused imports; `ProfileEntity`, `ContactInfo`, `FarmInfo` still used by `call`/`_validate*`.
- `profile_notifier.dart`: `ProfileState`, `profileRepositoryProvider`, `FarmInfo` still used.
- No stale import of removed symbols anywhere.

**Comments/docs:**
- `canonical_url_validator_test.dart:4` references `UpdateProfileUseCase cover/farm URLs` as a doc comment. This still holds for `call()`/`_validateProfile` (which still calls `validateUrl`), so left intact (not describing the deleted method specifically).

---

## 5. Tests removed and reason for deadness

**Removed:** `apps/mobile/test/domains/user/profile/domain/use_cases/update_profile_use_case_test.dart`
- Contained only `group('UpdateProfileUseCase cover reference validation')` with 4 tests calling `useCase.updateFields(...)`.
- The method under test was proven DEAD in Stage 4G (zero production callers, no DI routing, no indirect invocation).
- The test existed only to keep the legacy method "alive" — exactly the stale-test category flagged in Stage 4G (§5).
- Per instruction §4: removed without resurrecting legacy behavior into the canonical path.

**Not removed:** `profile_post_save_audit_test.dart` (canonical `ProfileActions.updateFields` coverage) — kept per instruction §5.

---

## 6. Canonical path proof

```
edit_profile_save_handler.saveProfileFields()          [edit_profile_save_handler.dart:97]
  ↓  ref.read(profileActionsProvider).updateFields(profile, fields)   [:134]
ProfileActions.updateFields(...)                        [profile_core_provider.dart:97]
  ↓  build ProfileEntity (location / coverPhotoUrl / contactInfo / farmInfo)
  ↓  updateProfile(updatedProfile) → updateUseCase(profile)           [:82-83]
UpdateProfileUseCase.call(profile)                      [update_profile_use_case.dart:18]
  ↓  _validateProfile → _validation.validateUrl(cover)  (unchanged)
  ↓  _repository.updateProfile(profile)
profile_repository_api.updateProfile → _profileEntityToUpdateRequest
  ↓  UpdateProfileApiRequest.coverPhotoUrl              [profile_repository_api.dart:290]
  ↓  datasource.updateMyProfile (PATCH /users/me/profile)
```
All symbols in this chain remain present and unchanged by this cleanup.

---

## 7. Cover storage-key invariant proof

- Source of `coverPhotoUrl` field in the canonical path: `edit_profile_save_handler.prepareCoverPhoto()` persists `coverPhotoUploadService.uploadCoverPhoto(...).storageKey` (`edit_profile_save_handler.dart:178`).
- Contract: **persistence = storage key**, **read/display = resolved URL** — unchanged.
- No `validateUrl` was added against storage keys in this stage. `_validateProfile` still validates `coverPhotoUrl` exactly as before (pre-existing behavior, not modified).
- Removed `UpdateProfileUseCase.updateFields` had a duplicate cover branch (`fields['coverPhotoUrl']`) and `ProfileNotifier.updateFields` had a duplicate cover branch (`fields['coverPhotoUrl']`) — both DEAD, now removed, eliminating two latent paths that could have written a resolved URL into `cover_photo_url`.

---

## 8. Tests + exact PASS/FAIL

**Bounded regression attempted:**
- `flutter test test/domains/user/profile/profile_post_save_audit_test.dart` → **FAILED TO COMPILE**, but the failures are **pre-existing BASELINE blockers unrelated to this cleanup**:
  - Commerce/catalog `Listing` types and `listing_*` providers missing (`lib/domains/commerce/catalog/listing/...` not found).
  - `AuthUser.storeName` / `storeImageUrl` getters missing (seller domain drift).
  - `profilePublicationProvider` / `profileCoverPublicationRevisionProvider` / `storeImagePublicationRevisionProvider` not exported.
  - `_TestEditProfileHostState` missing `EditProfileSaveHandler` mixin implementations (coverPhotoUrl, farmNameController, etc.).
  - These errors are identical in nature to the Stage 4G "BASELINE / EXTERNAL BLOCKER" category and are NOT caused by deleting the two dead methods.
- `dart analyze` on the test file shows **no errors referencing** `updateFields`, `_NoopUpdateProfileUseCase`, or `UpdateProfileUseCase` introduced by this change (see residue grep, §10).
- `update_profile_use_case_test.dart` (the only file that tested the dead method) was deleted as required.

**Exact PASS/FAIL:**
- `profile_post_save_audit_test.dart`: **BASELINE COMPILE FAIL** (external Commerce/seller missing symbols) → cannot run. Not caused by this cleanup.
- `update_profile_use_case_test.dart`: **REMOVED** (dead-method test).
- No canonical-profile test could be executed due to the same baseline Commerce/seller breakage.

---

## 9. `flutter analyze` result (touched scope)

```
dart analyze
  lib/domains/user/profile/domain/use_cases/update_profile_use_case.dart
  lib/domains/user/profile/presentation/providers/notifiers/profile_notifier.dart
  lib/domains/user/profile/presentation/providers/profile_core_provider.dart
  lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_save_handler.dart
  lib/domains/user/profile/data/repositories/profile_repository_api.dart

→ Analyzing ... No issues found!
```

(Warnings on `profile_post_save_audit_test.dart` are pre-existing baseline issues; none reference the removed symbols.)

---

## 10. Final residue grep result

Grep across `apps/mobile/**/*.dart` for `updateFields|UpdateProfileUseCase\.|ProfileNotifier\.update`:

Remaining matches are **canonical only**:
- `profile_core_provider.dart:97` — `ProfileActions.updateFields` (kept, canonical).
- `edit_profile_save_handler.dart:134` — `profileActionsProvider.updateFields` (kept, canonical).
- `profile_post_save_audit_test.dart` — `fakeActions.updateFieldsCalls` (canonical `ProfileActions` coverage).

**Zero** references to:
- `UpdateProfileUseCase.updateFields` → **0**
- `ProfileNotifier.updateFields` → **0**
- Any stale import/reference caused by the deletion → **0**

---

## 11. Baseline failures (pre-existing, NOT caused by this cleanup)

- Commerce/catalog `Listing` domain + `listing_*` providers missing (entire `lib/domains/commerce/catalog/listing/**` absent) → blocks compilation of many files incl. `commerce_preview_section.dart`, `checkout_*`, `comment_input_with_commerce_reference.dart`, `object_preview_*`.
- `AuthUser.storeName` / `storeImageUrl` getters removed/renamed in seller domain → breaks `profile_post_save_audit_test.dart` and others.
- `profilePublicationProvider`, `profileCoverPublicationRevisionProvider`, `storeImagePublicationRevisionProvider` not exported from `profile_core_provider.dart`.
- `EditProfileSaveHandler` mixin members not implemented by `_TestEditProfileHostState`.

These are out of scope (Commerce / seller / C3 / Feed / Comment) and must not be fixed here.

---

## 12. STOP

Stage 4G bounded cleanup complete. No further scope (avatar / social / settings / Commerce / C3 / Feed / Comment / seller) touched. STOP.
