# STAGE 4G — PROFILE UPDATE LEGACY PATH AUDIT

**MODE:** READ-ONLY AUDIT — NO CODE CHANGES MADE
**SCOPE:** `UpdateProfileUseCase.updateFields` + `ProfileNotifier.updateFields`
**SEARCH SURFACE:** `apps/mobile/lib/**`, `apps/mobile/test/**`

---

## VERDICT

**AUDIT COMPLETE — DEAD PATHS PROVEN**

Both target symbols have **zero production callers**, **no DI/provider routing to the method**, **no indirect invocation discovered**, and **no API route depending on them**. The live (canonical) path is `ProfileActions.updateFields` (via `profileActionsProvider`), consumed only by `edit_profile_save_handler.dart`.

---

## 1. `UpdateProfileUseCase.updateFields`

**Definition:**
`apps/mobile/lib/domains/user/profile/domain/use_cases/update_profile_use_case.dart:35`

**Production callers:**
`0`

- `ProfileActions.updateFields` (`profile_core_provider.dart:97`) does **NOT** delegate to `UpdateProfileUseCase.updateFields`. It re-implements the field-merge logic inline and calls `updateProfile(updatedProfile)` directly.
- `ProfileNotifier.updateFields` (`profile_notifier.dart:74`) also re-implements inline; does not delegate.
- `edit_profile_save_handler.dart:134` calls `ref.read(profileActionsProvider).updateFields(...)` — i.e. `ProfileActions.updateFields`, not the use case method.
- Grep for `.updateFields(` across `apps/mobile/lib` returns only: the definition, `ProfileActions.updateFields`, `ProfileNotifier.updateFields`, and the `edit_profile_save_handler` call to `profileActionsProvider`. No reference to `UpdateProfileUseCase.updateFields`.

**Test callers:**
`1 file` — `apps/mobile/test/domains/user/profile/domain/use_cases/update_profile_use_case_test.dart`
- 4 tests directly invoke `useCase.updateFields(...)` (lines 60, 80, 95, 109).
- `_NoopUpdateProfileUseCase.updateFields` (test fake, line 242) returns `Result.error('Not used')` — explicit proof the method is never exercised in production flows.

**Indirect callers discovered:**
`0`

**API/persistence consumers:**
N/A for the method itself. If ever invoked, it would route:
`updateFields → _repository.getProfile → copyWith → call() → _repository.updateProfile → UpdateProfileApiRequest → datasource.updateMyProfile`.
But nothing invokes it.

**DI/provider registration:**
`updateProfileUseCaseProvider` (`profile_core_provider.dart:40`) is registered and consumed by `profileActionsProvider` (line 63). However `profileActionsProvider` uses the `call` operator (`updateUseCase(profile)`), **never** `.updateFields`. So the method is never reached through DI.

**Reachability:**
**UNREACHABLE** (dead).

**API/persistence path:**
Not reachable from any caller. (Hypothetical sink: `profile_repository_api.updateProfile` → `_profileEntityToUpdateRequest` → `UpdateProfileApiRequest.coverPhotoUrl` → `datasource.updateMyProfile`.)

**Cover involvement:**
YES — has a cover branch:
`update_profile_use_case.dart:50` `coverPhotoUrl: fields['coverPhotoUrl'] as String?`
Then `_validateProfile` (`update_profile_use_case.dart:66-73`) calls `_validation.validateUrl(profile.coverPhotoUrl!)`.

**Canonical/legacy:**
**LEGACY.** The canonical Stage 4F path is `ProfileActions.updateFields` (storage-key persistence, no `validateUrl` on storage keys — see `edit_profile_save_handler.prepareCoverPhoto` which persists `result.data!.storageKey`). `UpdateProfileUseCase.updateFields` is a superseded duplicate with its own cover handling and a divergent validation path.

**Evidence:**
```
update_profile_use_case.dart:35   DEFINITION_ONLY
update_profile_use_case_test.dart:60,80,95,109   TEST_ONLY
update_profile_use_case_test.dart:242 (_NoopUpdateProfileUseCase.updateFields)   TEST_ONLY (returns 'Not used')
profile_core_provider.dart:97   ProfileActions.updateFields — RE-IMPLEMENTS, does NOT call use-case.updateFields
profile_notifier.dart:74        ProfileNotifier.updateFields — RE-IMPLEMENTS, does NOT call use-case.updateFields
edit_profile_save_handler.dart:134  calls profileActionsProvider.updateFields — NOT the use-case method
```

---

## 2. `ProfileNotifier.updateFields`

**Definition:**
`apps/mobile/lib/domains/user/profile/presentation/providers/notifiers/profile_notifier.dart:74`

**Production callers:**
`0`

- No occurrence of `profileNotifierProvider.notifier.updateFields(` or `ref.read(profileNotifierProvider)...updateFields` anywhere in `apps/mobile/lib`.
- `profileNotifierProvider` (alias of generated `profileProvider`, `profile_notifier.dart:182`) is only **watched** by convenience providers (`currentUserProfileProvider`, `isProfileLoadingProvider`, `profileErrorProvider` in `profile_providers.dart:36,42,48`) for **state** — none invoke `updateFields`.
- The live edit-profile save flow uses `profileActionsProvider`, not `profileNotifierProvider`.

**Test callers:**
`0` — no test references `ProfileNotifier.updateFields` or `profileNotifierProvider.updateFields`. (`profile_post_save_audit_test.dart` exercises `ProfileActions`, not `ProfileNotifier`.)

**Indirect callers discovered:**
`0`

**API/persistence consumers:**
If invoked, routes:
`updateFields → repository.updateProfile(updatedProfile) → profile_repository_api.updateProfile → UpdateProfileApiRequest → datasource.updateMyProfile`.
Nothing invokes it.

**DI/provider registration:**
`profileNotifierProvider` is registered (generated `profileProvider`) and exported (`profile_providers.dart:26-27`). It is watched for state only. The `updateFields` **method** is never called from any production or test code.

**Reachability:**
**UNREACHABLE** (dead).

**API/persistence path:**
Not reachable from any caller. (Hypothetical sink identical to above: `profile_repository_api.updateProfile` → `UpdateProfileApiRequest.coverPhotoUrl` → `datasource.updateMyProfile`.)

**Cover involvement:**
YES — has a cover branch:
`profile_notifier.dart:91-95` `if (fields.containsKey('coverPhotoUrl')) updatedProfile = updatedProfile.copyWith(coverPhotoUrl: fields['coverPhotoUrl'] as String?);`
No URL validation is performed here. It simply forwards whatever string is passed into `repository.updateProfile`.

**Canonical/legacy:**
**LEGACY / ORPHANED.** Pure Riverpod-notifier re-implementation that never got wired into any UI flow. The live path is `ProfileActions.updateFields`. It is a duplicate cover-handling path that could (if ever called) write an arbitrary string — including a resolved URL — into `cover_photo_url`, violating the canonical storage-key contract. It is currently unreachable, so no live risk, but it is a latent footgun.

**Evidence:**
```
profile_notifier.dart:74        DEFINITION_ONLY
profile_notifier.dart:182       profileNotifierProvider alias — watched for STATE only, method never called
profile_providers.dart:36,42,48 convenience providers — ref.watch(state) only
(no .updateFields invocation anywhere in apps/mobile/lib)
(no test references ProfileNotifier.updateFields)
```

---

## 3. Caller Trace

### Live (canonical) path — NOT an audit target, shown for contrast
```
edit_profile_save_handler.saveProfileFields()
  ↓  ref.read(profileActionsProvider).updateFields(profile, fields)   [edit_profile_save_handler.dart:134]
  ↓  ProfileActions.updateFields()   [profile_core_provider.dart:97]
  ↓    build ProfileEntity (location / coverPhotoUrl / contactInfo / farmInfo)
  ↓    updateProfile(updatedProfile) → updateUseCase(profile)   [profile_core_provider.dart:82-83]
  ↓      UpdateProfileUseCase.call() → repository.updateProfile()
  ↓        profile_repository_api.updateProfile → _profileEntityToUpdateRequest
  ↓          UpdateProfileApiRequest.coverPhotoUrl  (persisted as STORAGE KEY)
  ↓          datasource.updateMyProfile  (HTTP PATCH /users/me/profile)
```
Cover source: `prepareCoverPhoto()` persists `coverPhotoUploadService.uploadCoverPhoto(...).storageKey` (`edit_profile_save_handler.dart:178`) — **canonical storage-key contract**. ✅

### Audit target A — `UpdateProfileUseCase.updateFields` (DEAD)
```
[NO CALLER]
  ↓  (only reachable if something called useCase.updateFields — nothing does)
UpdateProfileUseCase.updateFields(userId, fields)   [update_profile_use_case.dart:35]
  ↓  _repository.getProfile → copyWith(coverPhotoUrl: fields['coverPhotoUrl'])
  ↓  call(updatedProfile) → _repository.updateProfile
  ↓  profile_repository_api.updateProfile → UpdateProfileApiRequest → datasource.updateMyProfile
```
Cover validation divergence: `_validateProfile` calls `_validation.validateUrl(coverPhotoUrl)` unconditionally for non-null cover (`update_profile_use_case.dart:66-73`). This is NOT how the canonical path behaves.

### Audit target B — `ProfileNotifier.updateFields` (DEAD)
```
[NO CALLER]
  ↓  (only reachable if something called profileNotifierProvider.notifier.updateFields — nothing does)
ProfileNotifier.updateFields(userId, fields)   [profile_notifier.dart:74]
  ↓  updatedProfile.copyWith(coverPhotoUrl: fields['coverPhotoUrl'])
  ↓  repository.updateProfile(updatedProfile)
  ↓  profile_repository_api.updateProfile → UpdateProfileApiRequest → datasource.updateMyProfile
```
No validation; forwards raw string. Latent canonical-contract violation if ever wired.

---

## 4. Deadness Evidence

### `UpdateProfileUseCase.updateFields` — DEAD
Proof (all conditions met):
- **zero production callers** — confirmed via `.updateFields(` grep + manual trace of `ProfileActions`/`ProfileNotifier`/`edit_profile_save_handler`.
- **no provider/DI registration routes to the method** — `updateProfileUseCaseProvider` exists but only `call` is used by `profileActionsProvider`.
- **no indirect invocation discovered** — no string-based/reflection dispatch in Dart; no other symbol delegates to it.
- **no API route depends on it** — it is a method, not a route; the underlying `repository.updateProfile` is reached via the live `ProfileActions` path instead.

Conclusion: **DEAD** (orphaned legacy method, only kept alive by its own unit test).

### `ProfileNotifier.updateFields` — DEAD
Proof (all conditions met):
- **zero production callers** — no `.updateFields` invocation on `profileNotifierProvider` anywhere in `apps/mobile/lib`.
- **no provider/DI registration routes to the method** — `profileNotifierProvider` is registered/exported and watched for **state only**; the `updateFields` method is never called.
- **no indirect invocation discovered** — none.
- **no API route depends on it** — same reasoning; `repository.updateProfile` reached via live path.

Conclusion: **DEAD** (orphaned notifier method, no test, no caller).

---

## 5. Test Audit

| Test | Target | Status |
|------|--------|--------|
| `update_profile_use_case_test.dart` (group `UpdateProfileUseCase cover reference validation`, lines 54-118) | `UpdateProfileUseCase.updateFields` | **STALE TEST — DO NOT MODIFY** (see note) |
| `profile_post_save_audit_test.dart` (lines 210-246, 1061, 1176, 1266, 1354, 1438, 1478, 1525, 1581, 1635, 1698, 1754, 1799, 1846, 1896, 1946, 2008) | `ProfileActions.updateFields` (via `_FakeProfileActions`) | Not a target; validates canonical `ProfileActions` path. Compiles, relevant. |

**Note on `update_profile_use_case_test.dart`:**
- It is the *only* thing keeping `UpdateProfileUseCase.updateFields` "alive" reference-wise.
- Discrepancy found (read-only observation, NOT a fix): test at line 55 expects `validateUrlCallCount == 0` for a storage-key cover (`images/profile-covers/user-1.jpg`), but the current implementation at `update_profile_use_case.dart:66-73` calls `_validation.validateUrl(profile.coverPhotoUrl!)` for **any** non-null cover. This means the storage-key test would currently **fail** (expects 0, code calls once) unless the implementation was changed after the test was written.
- The https-URL test (line 104) expects `validateUrlCallCount == 1`, consistent with the code.
- Classification: **STALE TEST — DO NOT MODIFY**. Flagged for next stage owner; not in scope to fix here.

---

## 6. External / Baseline Problems

- **Test/implementation divergence** in `update_profile_use_case_test.dart:70` vs `update_profile_use_case.dart:66-73` (storage-key cover expects 0 `validateUrl` calls; code calls once). Real mismatch, but it concerns a DEAD method, so impact is limited to a failing legacy test. Not modified (read-only stage).
- No other external/baseline blockers (e.g. missing SDK, network) were required to establish caller/reachability proof.

---

## 7. Recommendation (for next stage — NOT implemented here)

1. **Do NOT delete during Stage 4G.** Both `UpdateProfileUseCase.updateFields` and `ProfileNotifier.updateFields` are proven dead by caller/DI/reachability evidence.
2. Next stage (owner/ChatGPT approval) may:
   - Remove `UpdateProfileUseCase.updateFields` (+ the now-stale `update_profile_use_case_test.dart` cover group, or rewrite it against the canonical path).
   - Remove `ProfileNotifier.updateFields` (orphaned notifier method, no test, no caller).
   - Confirm `ProfileActions.updateFields` remains the single canonical entry point and that `prepareCoverPhoto()` storage-key persistence is preserved.
3. Before any deletion, re-run the full `apps/mobile` test suite to confirm no hidden indirect caller surfaces (none found in static audit).
4. No changes to avatar / social / settings / Commerce / C3 / Feed / Comment — out of scope per HARD RULES.

---

## STOP CONDITION — MET
- Both symbols traced ✅
- Production callers proven (zero) ✅
- API/persistence boundary traced ✅
- Cover involvement traced (both have legacy cover branches; live path uses storage key) ✅
- Test references inspected ✅
- Report written ✅

No cleanup, no avatar, no social, no settings, no Commerce, no C3 performed.
