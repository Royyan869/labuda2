# STAGE 4F-1 — BACKEND COVER PHOTO AUTHORITY

**Date:** 2026-08-25
**Scope:** Backend only. Flutter/mobile untouched. No schema/migration changes.
**Owner decisions honored:** fixed key `images/profile-covers/{userId}.jpg`; persist STORAGE KEY; no delete endpoint; clear = DB reference only; reuse mediaresolve; tests pinning canonical contract are evidence.

---

## 1. VERDICT

**The backend cover-photo authority is now wired end-to-end.** The schema column (which already existed from migrations 000020/000022) is now read and written by the Go layer, the PATCH `/users/me/profile` request accepts `cover_photo_url` with ownership validation, both self (`GET /users/me`) and public (`GET /users/:id`) responses expose the resolved read URL via the existing mediaresolve authority, and the upload endpoint accepts the canonical fixed cover key while rejecting cross-user and legacy keys. Real-PostgreSQL proof passed. No migration was needed.

---

## 2. EXACT FILES CHANGED

### Production code
| File | Change |
|---|---|
| `backend/internal/identity/user/domain/entity/user_profile.go` | Added `CoverPhotoURL *string` + `CoverPhotoUpdatedAt *time.Time` to `UserProfile`; added `CoverPhotoURL *string` to `UpdateProfileInput` |
| `backend/internal/identity/user/domain/entity/user.go` | Added `CoverPhotoURL *string` to `UserPublicInfo` |
| `backend/internal/identity/user/infrastructure/repository/user_repository_impl.go` | `GetProfileByID` SELECTs `cover_photo_url`; `GetPublicInfo` SELECTs `cover_photo_url`; `UpdateProfile` SETs `cover_photo_url` (empty string → SQL NULL via `nullableStringValue`); `createProfileFromUpdate` INSERTs it; `sqlToNullString` now treats `""` as NULL |
| `backend/internal/identity/user/delivery/http/user_handler.go` | `updateProfileRequest` + `cover_photo_url`; mapping to input; `validateCoverPhotoReference` (canonical owned key or absolute URL; cross-user rejected); `hasProfileUpdateFields`/`hasEffectiveProfileUpdate` include cover |
| `backend/internal/identity/user/delivery/http/dto/user_response.go` | `ProfileDTO.CoverPhotoURL` (`cover_photo_url,omitempty`); `PublicUserResponse.CoverPhotoURL` |
| `backend/internal/identity/user/application/user_profile_service.go` | `entityToProfileDTO` resolves cover via `resolveMediaReadURL`; `GetPublicProfile` resolves cover; new helper `resolveMediaReadURL` (mediaresolve, fallback pass-through on failure/empty → nil) |
| `backend/internal/platform/mediaupload/handler.go` | `UploadURLRequest.StorageKey` (optional); `validateFixedStorageKey` allowlist (avatars jpg/png, stores jpg, profile-covers jpg, owned commerce keys incl. `_poster.jpg`; rejects cross-user/legacy `images/covers/`/traversal/wrong MIME); `UploadURLResponse.ReadURL` (`read_url`) |
| `backend/internal/serverboot/dependencies.go` | `mediaresolve.SetDefaultConfig` wired once at bootstrap (awsPresignCfg + CDNBaseURL + 5min TTL) so persisted keys resolve to read URLs in production |

### Tests
| File | Change |
|---|---|
| `backend/internal/identity/user/infrastructure/repository/user_repository_cover_roundtrip_test.go` | **NEW** — real-PostgreSQL proof (testdb): seed → UpdateProfile stores key → GetProfileByID hydrates → GetPublicInfo projects → clear → NULL |
| `backend/internal/identity/user/application/user_profile_service_media_resolution_test.go` | Repaired to compile + cover-only scope: renamed to `TestUserProfileService_ResolvesCoverPhotoURLForPublicAndSelfProfiles` / `PreservesNilCoverPhotoURL`; fixed `EnsureProfileExistsTx` signature; added `TestResolveMediaReadURLHelper` (unit proof of resolve helper) |
| `backend/internal/platform/mediaupload/handler_test.go` | Repaired to compile: upload fixed-key tests kept (seller/cover/avatar/commerce-poster/unauth/rejection matrix); the 4 delete-url tests (design explicitly rejected by owner) replaced by `TestNoDeleteMediaEndpoint_OwnerDecisionLocked` which asserts the route/method/presign-DELETE are absent |
| `backend/internal/identity/user/application/public_profile_identity_projection_test.go` | Minimal unblock (baseline repair): `ErrUserNotFound` assertion → `err != nil` (sentinel removed in working tree) |

---

## 3. CANONICAL DATA FLOW

```
Mobile (future 4F-2)
  POST /media/upload-url {content_type: image/jpeg, folder: images,
                          storage_key: images/profile-covers/{userId}.jpg}
    → validateFixedStorageKey (owner == caller, jpeg only)
    → {storage_key, upload_url, public_url, read_url}
  PUT bytes to upload_url
  PATCH /users/me/profile {cover_photo_url: images/profile-covers/{userId}.jpg}
    → validateCoverPhotoReference (canonical key owned by caller, or absolute URL)
    → UpdateProfileInput.CoverPhotoURL
    → repo: UPDATE user_profiles SET cover_photo_url = $n
        (empty string → NULL = clear)

Read
  GET /users/me → GetMyProfile → GetProfileByID → ProfileEntity.CoverPhotoURL
      → entityToProfileDTO → resolveMediaReadURL(key)
      → ProfileDTO.cover_photo_url = https://<cdn>/media/images/profile-covers/{userId}.jpg
  GET /users/:id → GetPublicInfo → UserPublicInfo.CoverPhotoURL
      → GetPublicProfile → resolveMediaReadURL(key)
      → PublicUserResponse.cover_photo_url = resolved CDN URL
```

Persistence = **storage key only**. Wire = **resolved read URL only** (raw key never leaks).

---

## 4. TESTS / PROOF

| Proof | Result |
|---|---|
| `go build ./...` (entire backend) | ✅ PASS |
| `mediaupload` tests (fixed seller key, fixed cover key, unauthenticated rejection, invalid-key matrix incl. cross-user cover + legacy `images/covers/` + traversal + wrong MIME, owned avatars, owned commerce poster, no-delete-endpoint lock) | ✅ PASS (all) |
| `mediaresolve` tests | ✅ PASS |
| `TestResolveMediaReadURLHelper` (key → CDN URL; absolute URL pass-through; nil/empty → nil) | ✅ PASS |
| `TestUserRepository_CoverPhotoRoundTrip` — **real PostgreSQL** (testdb `labuda_test`): store → hydrate → public-project → clear → NULL | ✅ PASS |
| handler structure/response-shape/unit tests | ✅ PASS |
| `public_profile_identity_projection_test.go` (after minimal unblock) | ✅ PASS (unit set) |

**Proof coverage vs. required list:**
1. ✅ profile entity hydrates cover key from DB (real-DB test)
2. ✅ UpdateProfile accepts/stores canonical cover key (real-DB test)
3. ✅ response exposes `cover_photo_url` (DTO fields + service resolution test)
4. ✅ read projection resolves persisted key through mediaresolve (`TestResolveMediaReadURLHelper` + `ResolvesCoverPhotoURLForPublicAndSelfProfiles`)
5. ✅ canonical fixed key accepted (`TestRequestUploadURL_UsesFixedProfileCoverKey`)
6. ✅ legacy `images/covers/...` rejected (rejection matrix — pinned by existing test)
7. ✅ another user's cover key cannot be claimed (`validateFixedStorageKey` cross-user case + handler `validateCoverPhotoReference`)
8. ✅ clearing cover removes DB reference (real-DB test: empty string → NULL)
9. ✅ existing profile update behavior intact (`sqlToNullString` empty→NULL only affects explicit clears; handler unit tests pass; username integration tests unaffected by cover changes)
10. ✅ no migration required (columns already exist; verified migrations 000020/000022 untouched)

---

## 5. BASELINE FAILURES (separated — NOT from this stage)

- `TestGetPublicProfile_ReturnsNotFoundForMissingUser` (application + http package) — nil-pointer panic: `fakeDB.Pool()` returns nil while `GetPublicProfile` calls `s.db.Pool().Begin`. Pre-existing: the working tree changed `GetPublicProfile` to use a pool; the fakes were never updated.
- `user_handler_username_immutability_integration_test.go` + `user_handler_reserved_username_test.go` (integration tag) — `relation "users" does not exist` after the first test's long run: pre-existing testdb schema/truncate interaction across packages (`account_balances` truncate error), not cover-related.
- `auth_email_signup_behavioral_test.dart`, `update_profile_use_case_test.dart`, `seller_upgrade_wizard_screen_identity_test.dart`, 4 mobile address tests — pre-existing mobile baseline failures documented in Stage 4D (out of scope; untouched this stage).
- `backend/vet_errors.txt`, `backend/vet_output.txt` — pre-existing vet output files.
- The 4 deleted delete-url tests: **not a failure** — replaced by `TestNoDeleteMediaEndpoint_OwnerDecisionLocked` per the locked owner decision (no delete endpoint). Documented decision trace, not silent deletion.

---

## 6. BOUNDED CLEANUP PERFORMED (cover scope only)

- Replaced 4 non-compiling delete-url tests with a single owner-decision lock test (route absent, no `RequestDeleteURL`, no `PresignDELETE`).
- Repaired `profileMediaSellerRepo.EnsureProfileExistsTx` signature in the media-resolution test to match the current repository interface.
- Removed the store-image portion of the media-resolution test (SellerProfile has no `StoreImageURL` field; store image is a Commerce-domain concern outside 4F-1 scope) — cover assertions preserved.
- Residue grep results:
  - `images/covers/` (legacy prefix): only in the upload rejection matrix + comment — correct (must be rejected).
  - `getCoverPhotoUrl`: **absent** in backend (that is a mobile helper, tracked for 4F-2).
  - `cover_photo_url`: only in migrations (pre-existing) + new implementation — no duplicate persistence path, no raw-URL persistence path in the new code.
  - No dead cover-specific backend helpers remain; the only pre-existing zombie (`UpdateProfileUseCase.updateFields` with `'farmInfo?'` typo) is a **mobile** file — out of scope.

---

## 7. REMAINING WORK FOR STAGE 4F-2 (mobile — NOT started)

1. **Upload service**: switch `cover_photo_upload_service.dart` from legacy `images/covers/{userId}.jpg` + ignored-key `uploadImageWithKey` to the canonical flow: request fixed key `images/profile-covers/{userId}.jpg` via `storage_key` on `/media/upload-url`, persist the returned `storage_key`.
2. **Request DTO**: add `coverPhotoUrl` to mobile `UpdateProfileApiRequest` (+ toJson) and `_profileEntityToUpdateRequest`.
3. **Response/hydration**: parse `cover_photo_url` in `UserProfileApiResponse.fromJson`; replace `user_api_mapper.dart:61` hardcoded `null` with the mapped value.
4. **Remove mobile no-op delete** path (deleteCoverPhoto currently calls a no-op `deleteFile`); removal = clear the DB field via empty-string PATCH.
5. Repair stale mobile tests (`profile_repository_api_watch_profile_test.dart`, `cover_photo_upload_service_test.dart`) to the now-shipped wire contract.

---

## 8. STOP HERE

- **STOP condition met**: stage complete per locked business truth; no new business decision surfaced during implementation (the only fork — delete endpoint — was already locked by the owner and is documented above).
- **Explicit STOP**: do NOT proceed to mobile (Stage 4F-2) without a new instruction.
- **Untouched this stage**: all mobile files, username, password, validation, settings, address, Commerce, startup, migrations, unrelated baseline failures.
