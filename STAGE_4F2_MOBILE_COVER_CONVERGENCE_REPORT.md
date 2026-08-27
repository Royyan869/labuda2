# STAGE 4F-2 — MOBILE COVER PHOTO CONVERGENCE

**Date:** 2026-08-25
**Scope:** Mobile profile cover-photo only. Backend untouched (Stage 4F-1 contract honored). No schema/migration changes. No new endpoints.

---

## 1. VERDICT

**Mobile now converges end-to-end onto the Stage 4F-1 backend cover authority.** The upload path produces the canonical fixed storage key (`images/profile-covers/{userId}.jpg`), persists the STORAGE KEY (never the resolved URL), serializes `cover_photo_url` on PATCH, hydrates the resolved URL from the profile response, and clears via empty-string PATCH (no S3 delete, no legacy prefix). Focused tests (20) pass; `flutter analyze` on the touched scope has no errors.

---

## 2. AUDIT (summary — full detail in STAGE_4F2_MOBILE_COVER_AUDIT_REPORT.md)

**Gaps found (all closed):**
| Gap | Root cause | Fix |
|---|---|---|
| Legacy prefix `images/covers/` + ignored key | `cover_photo_upload_service.dart` used `uploadImageWithKey` (key ignored by S3Service) | Rewrote service: canonical key + `uploadImageWithFixedKey` |
| Request mapper dropped cover | `_profileEntityToUpdateRequest` omitted `coverPhotoUrl` | Added to request |
| No `cover_photo_url` in `UpdateProfileApiRequest` | DTO lacked field | Added field + toJson (empty string = clear) |
| Hydration always null | `UserProfileApiResponse.fromJson` didn't parse; mapper hardcoded null | Parse `cover_photo_url`; mapper maps it |
| Clear semantics broken | removal called S3 no-op delete; never reached wire | Removal → `coverPhotoUrl = ''` → PATCH clear |
| No fixed-key presign path | `uploadImageWithKey` ignored key | Added `uploadImageWithFixedKey` (sends `storage_key` to backend) |

---

## 3. ACTUAL CHANGES / FILES CHANGED

### Production (mobile)
| File | Change |
|---|---|
| `lib/core/services/s3_service.dart` | `_requestMediaPresignURL` accepts optional `storageKey` (sent as `storage_key`); parses `read_url`; `_MediaPresignResult.readUrl`; new `uploadImageWithFixedKey(File, key, {mediaLabel})` → fixed-key presign → PUT → returns `S3UploadResult(key, readUrl)` |
| `lib/domains/user/profile/data/services/cover_photo_upload_service.dart` | Canonical key `images/profile-covers/{userId}.jpg`; `uploadCoverPhoto` → `uploadImageWithFixedKey` → returns `CoverUploadResult(storageKey, readUrl)`; removed `getCoverPhotoUrl` + `deleteCoverPhoto` (no delete by design); added `storageKeyFor` |
| `lib/domains/user/profile/data/models/api/user_api_models.dart` | `UpdateProfileApiRequest.coverPhotoUrl` + toJson `cover_photo_url` (empty string serialized = clear); `UserProfileApiResponse.coverPhotoUrl` parsed from `cover_photo_url` |
| `lib/domains/user/profile/data/mappers/user_api_mapper.dart` | `toProfileEntity` maps `profile?.coverPhotoUrl` (replaces hardcoded null); `toUpdateProfileRequest` accepts `coverPhotoUrl` |
| `lib/domains/user/profile/data/repositories/profile_repository_api.dart` | `_profileEntityToUpdateRequest` includes `coverPhotoUrl` |
| `lib/domains/user/profile/presentation/screens/edit_profile/edit_profile_save_handler.dart` | `prepareCoverPhoto`: removal → `''` (PATCH clear, no S3 delete); upload → persists `result.data!.storageKey` (not read URL) |

### Tests
| File | Change |
|---|---|
| `test/domains/user/profile/cover_photo_contract_test.dart` | **NEW** — serialization (canonical key, empty-string clear, omitted), mapper hydration (resolved URL, null), `toUpdateProfileRequest` carries cover, legacy-prefix absence |
| `test/domains/user/profile/cover_photo_upload_service_test.dart` | Converged to new API: asserts canonical key passed, storage key returned, read URL available, no legacy prefix |
| `test/domains/user/profile/profile_post_save_audit_test.dart` | Updated cover fakes (`_NoopCoverPhotoUploadService`/`_FailingCoverPhotoUploadService`) to `Result<CoverUploadResult>`; removed `deleteCoverPhoto` overrides |

---

## 4. CANONICAL FLOW (as implemented)

```
Upload:
  UI onChangeCover → unified_edit_profile_screen → EditProfileSaveHandler.prepareCoverPhoto
  → CoverPhotoUploadService.uploadCoverPhoto
      key = images/profile-covers/{userId}.jpg
      S3Service.uploadImageWithFixedKey → POST /media/upload-url {storage_key: key}
      → PUT bytes → backend returns storage_key + read_url
      → CoverUploadResult(storageKey, readUrl)
  → fields['coverPhotoUrl'] = storageKey            ← STORAGE KEY persisted
  → ProfileActions.updateFields → repository.updateProfile
  → UpdateProfileApiRequest{cover_photo_url: storageKey}
  → PATCH /users/me/profile
  → backend persists storage key

Hydration:
  GET /users/me (or /:id) → profile.cover_photo_url (resolved URL)
  → UserProfileApiResponse.fromJson parses cover_photo_url
  → UserApiMapper.toProfileEntity → ProfileEntity.coverPhotoUrl
  → ProfileCover / EditProfileCoverSection render the image

Clear:
  UI remove → isCoverMarkedForRemoval → prepareCoverPhoto → url: ''
  → fields['coverPhotoUrl'] = '' → PATCH cover_photo_url: "" → backend NULL
  → subsequent hydration yields no cover
```

**Invariant:** the value persisted on the wire is the STORAGE KEY — never the resolved CDN/public URL. The read URL is only for display (`CoverUploadResult.readUrl` / hydrated `coverPhotoUrl`).

---

## 5. TESTS / PROOF

| Proof | Result |
|---|---|
| `cover_photo_contract_test.dart` (7 tests: serialization ×3, hydration ×3, legacy-absence ×1) | ✅ PASS |
| `cover_photo_upload_service_test.dart` (canonical key + storage key + read URL; failure text) | ✅ PASS |
| `edit_profile_cover_section_test.dart` (widget contract) | ✅ PASS |
| `canonical_url_validator_test.dart` (regression guard for URL validation path) | ✅ PASS |
| `profile_repository_api_watch_profile_test.dart` — cover hydration assertion | ✅ assertion passes (test still times out at the end on the infinite `Stream.periodic` — baseline behavior, unrelated to cover; see §7) |
| `flutter analyze` on all 10 touched files | ✅ no errors (6 pre-existing `curly_braces` infos in untouched s3_service code remain) |

**Required proofs from the stage spec:**
1. ✅ canonical storage key produced (`uploadCoverPhoto` passes `images/profile-covers/{userId}.jpg`)
2. ✅ upload → read_url handled (`CoverUploadResult.readUrl` from backend `read_url`)
3. ✅ request serialization `cover_photo_url` (toJson test)
4. ✅ hydration (mapper test: resolved URL from profile)
5. ✅ clear (`cover_photo_url: ""` serialization test + `prepareCoverPhoto` returns `''`)
6. ✅ no legacy prefix (residue grep + test)
7. ✅ persistence value remains storage key (`prepareCoverPhoto` persists `storageKey`, not `readUrl`)
8. ✅ cover UI round-trip feasible (upload service + hydration + widget tests)

---

## 6. ANALYZE

`flutter analyze` on the touched scope: **0 errors**. Remaining 6 infos are pre-existing `curly_braces_in_flow_control_structures` in `s3_service.dart` lines 191–415 (code untouched by this stage).

---

## 7. BASELINE FAILURES (separated — NOT regressions from 4F-2)

- `profile_repository_api_watch_profile_test.dart` — cover assertion now passes; the test still times out after 30s because `watchProfile` returns `Stream.periodic(30s)` (never completes) while the test asserts `emitsDone`. Pre-existing stream-contract mismatch; cover assertion inside the predicate passes.
- `update_profile_use_case_test.dart` (3 failures: `validateUrlCallCount`, null-removal, traversal) — pre-existing (documented Stage 4D): use-case `_validateProfile` validates cover URL but test expects storage-key pass-through; not touched by 4F-2.
- `profile_post_save_audit_test.dart` — numerous pre-existing compile errors (`profilePublicationProvider`, `AuthUser.storeName/storeImageUrl`, `publishStoreIdentity`, `updatedAt`) from working-tree changes outside this stage; only the cover fakes were updated here (the file does not compile at baseline).
- Mobile baseline failures from Stage 4D (`auth_email_signup_behavioral_test`, `seller_upgrade_wizard_screen_identity_test`, address tests) — untouched.

---

## 8. CLEANUP (scoped, cover-only)

- Removed `CoverPhotoUploadService.getCoverPhotoUrl` (zombie helper, 0 callers, legacy fixed-URL).
- Removed `CoverPhotoUploadService.deleteCoverPhoto` (S3 no-op delete; removal = DB clear by design).
- Removed legacy `images/covers/` prefix from production code (remains only in a doc comment explaining it is rejected).
- Not removed (out of safe scope, documented as open issues): `UpdateProfileUseCase.updateFields` (zombie writer with `'farmInfo?'` typo — has a baseline test pinning it); `ProfileNotifier.updateFields` cover branch (provider chain still references `profileNotifierProvider`).

---

## 9. RESIDUE AUDIT

- `images/covers/` in `lib/`: **0** production references (1 doc comment explaining rejection).
- `getCoverPhotoUrl`: **0** references.
- `deleteCoverPhoto`: **0** references.
- `uploadImageWithKey` cover calls: **0** (cover now uses `uploadImageWithFixedKey`).
- `coverPhotoUrl: null // Not in API yet`: **0** (hardcode removed).
- Hardcoded-URL persistence: **0** (save handler persists storage key).

---

## 10. RUNTIME / INTEGRATION PROOF

The mobile write+read+clear path is consistent with the locked backend contract by construction and focused tests:

- **Write**: fixed-key presign (backend validates ownership; legacy rejected) → PUT → storage key persisted via `cover_photo_url` PATCH. Backend Stage 4F-1 real-DB proof (`TestUserRepository_CoverPhotoRoundTrip`) already proved the backend accepts/stores/hydrates/clears exactly this contract.
- **Read**: response `profile.cover_photo_url` (resolved) parsed by `UserProfileApiResponse.fromJson` → mapper → UI. Watch-profile test's cover assertion passes against the new mapper.
- **Clear**: empty-string PATCH → backend NULL (proven in Stage 4F-1 real-DB test step 4).

A full end-to-end widget/integration run against a live backend was not executed in this stage (requires running server + AWS); the contract is locked on both sides by deterministic tests (mobile serialization/hydration + backend real-PostgreSQL round-trip).

---

## 11. OPEN ISSUES

1. `UpdateProfileUseCase.updateFields` — zombie writer with `fields['farmInfo?']` literal typo; has a baseline-failing test. Needs an owner decision (repair+wire vs delete) in a future stage.
2. `ProfileNotifier.updateFields` cover branch — writer not consumed by any active screen (provider chain still references it). Candidate for a later bounded cleanup.
3. Avatar upload still uses the ignored-key `uploadImageWithKey` (`avatar_image_processor.dart`, `avatar_upload_service.dart`) — same fixed-key gap the cover path just closed; **out of 4F-2 scope** (avatar), flagged for a future stage.
4. `profile_post_save_audit_test.dart` and `update_profile_use_case_test.dart` are baseline-broken by unrelated working-tree changes; repairing them is out of scope.
5. `watchProfile` stream never completes (30s `Stream.periodic`) — baseline stream-contract mismatch.

---

## 12. STOP HERE

- Stage 4F-2 complete: canonical mobile write + read + clear path is implemented and proven against the Stage 4F-1 backend contract.
- **Do NOT proceed to Stage 4F-3 or any other domain.**
- **Do NOT touch** unrelated baseline failures, Commerce, seller domain, username, password, auth, or migrations.
- Backend was not modified in this stage.

**Next authorized step (owner decision):** address the open issues above (zombie writers, avatar fixed-key, baseline test repairs) in a future bounded stage.
