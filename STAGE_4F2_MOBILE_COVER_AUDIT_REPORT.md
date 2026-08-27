# STAGE 4F-2 — MOBILE COVER AUDIT (STEP A — READ-ONLY)

**Date:** 2026-08-25
**Backend contract (locked, Stage 4F-1):** persistence = storage key `images/profile-covers/{userId}.jpg`; PATCH accepts `cover_photo_url`; read DTO returns resolved URL; legacy `images/covers/` rejected; empty string = clear → NULL; no delete endpoint; media upload response has `read_url`.

---

## 1. CURRENT MOBILE FLOW (factual, file:line)

### Upload path
```
EditProfileCoverSection (onChangeCover)
→ unified_edit_profile_screen.dart:439-462 (_changeCover/_removeCover)
→ edit_profile_save_handler.dart:151-178 (prepareCoverPhoto)
    removal   → coverPhotoUploadService.deleteCoverPhoto(userId) :155  [no-op]
    new image → coverPhotoUploadService.uploadCoverPhoto(userId, path) :162
→ cover_photo_upload_service.dart:44-45
    key = 'images/covers/{userId}.jpg'          ← LEGACY prefix
    s3Service.uploadImageWithKey(file, key)     ← key IGNORED (s3_service.dart:336-338)
    → presigned POST /media/upload-url (no storage_key) → backend mints timestamped key
    → returns public_url (NOT the canonical key)
→ edit_profile_save_handler.dart:117  fields['coverPhotoUrl'] = result.data  (public URL!)
→ profile_core_provider.dart:120-134  ProfileActions.updateFields → entity coverPhotoUrl
→ update_profile_use_case.dart:26     repository.updateProfile(profile)
→ profile_repository_api.dart:285-297 _profileEntityToUpdateRequest
    **coverPhotoUrl DROPPED — not in UpdateProfileApiRequest**  ← GAP (wire)
```

### Read/hydration path
```
profile_repository_api.dart:34-51  getProfile → UserApiMapper.toProfileEntity
user_api_mapper.dart:61            coverPhotoUrl: null  // Not in API yet   ← GAP
user_api_models.dart:566-598       UserProfileApiResponse.fromJson — NO cover_photo_url parse  ← GAP
profile_repository_api.dart:197-206 watchProfile → polls getProfile → always null cover
profile_screen.dart:965-968        coverPhotoUrl = profileStreamProvider.value?.coverPhotoUrl → null
profile_cover.dart:47-71           Image.network only when non-null → always gradient fallback
```

### Clear path
```
edit_profile_save_handler.dart:153-158  removal → deleteCoverPhoto (S3 no-op) → url: null
profile_core_provider.dart:125           entity coverPhotoUrl = null (local only)
profile_repository_api.dart:285-297      dropped at wire → backend never receives clear
```

---

## 2. CANONICAL TARGET FLOW

```
Upload:
  uploadCoverPhoto → fixed key images/profile-covers/{userId}.jpg
  → POST /media/upload-url {content_type: image/jpeg, folder: images,
                            storage_key: images/profile-covers/{userId}.jpg}
  → PUT bytes → backend returns storage_key + read_url
  → return (storage_key, read_url) pair
  → PATCH /users/me/profile {cover_photo_url: images/profile-covers/{userId}.jpg}
  → backend persists storage key

Hydration:
  GET /users/me / GET /users/:id → profile.cover_photo_url (resolved URL)
  → UserProfileApiResponse.fromJson parses cover_photo_url
  → UserApiMapper.toProfileEntity maps it (replaces hardcoded null)
  → ProfileCover renders

Clear:
  PATCH {cover_photo_url: ""} → backend NULL → hydration yields no cover
  (no S3 delete call needed — removal = clear DB reference)
```

---

## 3. EXACT GAPS

| # | Gap | File | Severity |
|---|---|---|---|
| G1 | Upload uses legacy prefix `images/covers/` + ignored key; never produces canonical key; returns public URL instead of storage key | `cover_photo_upload_service.dart:18,44-45` | P1 |
| G2 | Request mapper drops `coverPhotoUrl` entirely — never on the wire | `profile_repository_api.dart:285-297` | P1 |
| G3 | `UpdateProfileApiRequest` has no `cover_photo_url` field | `user_api_models.dart:220-334` | P1 |
| G4 | `UserProfileApiResponse.fromJson` doesn't parse `cover_photo_url`; mapper hardcodes null | `user_api_models.dart:566-598`, `user_api_mapper.dart:61` | P1 |
| G5 | Clear path calls S3 no-op delete + never reaches wire (removal semantics broken) | `edit_profile_save_handler.dart:153-158` | P2 |
| G6 | `S3Service` has no fixed-key presign method (uploadImageWithKey ignores key) | `s3_service.dart:336-338` | P1 (enabler) |
| G7 | `CoverPhotoUploadService.getCoverPhotoUrl` zombie helper (fixed-key URL for legacy prefix) | `cover_photo_upload_service.dart:67-74` | P3 |

---

## 4. CALLERS / CONSUMERS

- `CoverPhotoUploadService` — consumers: `unified_edit_profile_screen.dart:142-143` (via `coverPhotoUploadServiceProvider`, `profile_providers.dart:89-94`), `edit_profile_save_handler.dart:51,155,162`.
- `ProfileActions.updateFields` — consumer: `edit_profile_save_handler.dart:134` (only cover writer; `profile_notifier.dart:74-115` `ProfileNotifier.updateFields` is a ZOMBIE — no screen consumes `profileNotifierProvider`).
- `UpdateProfileUseCase.updateFields` (`update_profile_use_case.dart:35-56`) — ZOMBIE writer with `fields['farmInfo?']` literal typo at :52; the live path calls `ProfileActions.updateFields` (profile_core_provider) which calls `updateProfile` → `call()` → repository.
- Cover readers: `profile_screen.dart:965-968,997,1042`, `profile_header_builder.dart:58`, `profile_cover.dart:11,48-50`, `edit_profile_cover_section.dart:9,27,118-120`.
- No other domain reads `coverPhotoUrl` (verified Stage 4E grep: zero outside profile domain).

---

## 5. LEGACY / ZOMBIE PATHS

| Item | Status |
|---|---|
| `images/covers/` prefix | LEGACY — must be removed; backend rejects it |
| `CoverPhotoUploadService.getCoverPhotoUrl` | ZOMBIE — 0 callers; points at non-existent objects |
| `S3Service.deleteFile` / `deleteFiles` | no-op stubs — cover removal must NOT depend on them (clear = DB field) |
| `ProfileNotifier.updateFields` (cover branch) | ZOMBIE — notifier not consumed |
| `UpdateProfileUseCase.updateFields` (`'farmInfo?'` typo) | ZOMBIE — never called by live path |
| `ProfileEntity.copyWith` cover param (can't express null) | structural quirk — cover removal goes through full-constructor rebuild (already the live path) |

---

## 6. TEST CONTRADICTIONS

| Test | Assertion | Status vs canonical |
|---|---|---|
| `cover_photo_upload_service_test.dart` | Asserts `uploadImageWithFixedKey` (nonexistent) + `result.data == 'images/profile-covers/...'` | **STALE / non-compiling** — must converge to canonical: fixed-key presign + returns storage key (+ read_url) |
| `profile_post_save_audit_test.dart` | Fakes return `images/profile-covers/{userId}.jpg` (canonical key) | **ALIGNED** — proves save-handler choreography expects canonical key |
| `update_profile_use_case_test.dart` | Pins canonical key accepted, traversal rejected, legacy https accepted | **ALIGNED** (validates the use-case contract; note: use-case `updateFields` is zombie, but `call()` → `_validateProfile` cover URL check is live) |
| `profile_repository_api_watch_profile_test.dart` | Asserts `'cover_photo_url': 'images/profile-covers/...'` → `ProfileEntity.coverPhotoUrl` | **AHEAD of implementation** — fails today because mapper hardcodes null; converges once G4 is fixed |
| `edit_profile_cover_section_test.dart` | Widget contract (props/labels) | ALIGNED — no change needed |

---

## 7. PROPOSED BOUNDED IMPLEMENTATION

1. **`s3_service.dart`**: add `uploadImageWithFixedKey(File, key, {mediaLabel})` → `_requestMediaPresignURL(contentType, folder, storageKey: key)` (extend helper with optional `storage_key` body field) → PUT → return `S3UploadResult(key: presign.storageKey, url: presign.readUrl ?? presign.publicUrl)`. Keep existing methods untouched.
2. **`cover_photo_upload_service.dart`**: use `images/profile-covers/{userId}.jpg`; call fixed-key method; return `(storageKey, readUrl)` — i.e. return a result carrying both; `uploadCoverPhoto` returns the **storage key** (what must be persisted) while exposing read URL for preview. Delete `getCoverPhotoUrl`. Rewrite `deleteCoverPhoto` → clear semantics handled at save handler (PATCH ""), remove S3 delete call.
3. **`user_api_models.dart`**: add `coverPhotoUrl` to `UpdateProfileApiRequest` (+ `toJson` `cover_photo_url`, empty string allowed = clear); parse `cover_photo_url` in `UserProfileApiResponse.fromJson`.
4. **`user_api_mapper.dart`**: `toUpdateProfileRequest` add `coverPhotoUrl`; `toProfileEntity` map `profile?.coverPhotoUrl` (replace `:61` hardcode).
5. **`profile_repository_api.dart`**: `_profileEntityToUpdateRequest` include `coverPhotoUrl: profile.coverPhotoUrl`.
6. **`edit_profile_save_handler.dart`**: `prepareCoverPhoto` removal → `fields['coverPhotoUrl'] = ''` (PATCH clear, no S3 delete); new upload → `fields['coverPhotoUrl'] = storageKey`.
7. **Tests**: converge `cover_photo_upload_service_test.dart` to new API; add serialization test (toJson includes `cover_photo_url`), mapper hydration test, clear-serialization test; run watch-profile test (should pass after G4).
8. **Cleanup**: remove zombie `ProfileNotifier.updateFields` cover branch? — NO, out of 4F-2 minimal scope (it is a separate zombie writer; deleting requires consumer audit — flagged for Step D decision). Remove `getCoverPhotoUrl` (proven 0 callers). Legacy `images/covers/` grep must be zero in lib after changes.

---

## 8. RISKS

- **Persisting the wrong value**: must persist the **storage key** (`presign.storageKey`), never `read_url`/`public_url` — backend contract. The fixed-key presign guarantees `storageKey == images/profile-covers/{userId}.jpg`.
- **Avatar collision**: avatar upload still uses ignored-key `uploadImageWithKey` (out of 4F-2 scope) — cover change must not touch avatar path.
- **Clear semantics**: `toJson` must serialize `cover_photo_url: ""` even though it's "empty" — the existing `if (coverPhotoUrl != null)` pattern already allows empty-string serialization.
- **Watch-profile test timing**: `profile_repository_api_watch_profile_test.dart` should start passing once G4 lands; if it was failing at baseline (it was — mapper hardcoded null), that's a convergence win, not a new failure.

---

## 9. BUSINESS DECISIONS REQUIRED

**None.** The backend contract is fully locked from Stage 4F-1; mobile convergence is purely mechanical (use canonical key, persist storage key, serialize/parse `cover_photo_url`, clear via empty string). No new business rule is invented.

---

## 10. VERDICT

Proceed to Step B (implementation). All gaps are mechanical convergence onto the locked Stage 4F-1 contract; tests pinning the canonical contract (watch-profile, post-save-audit fakes, update-use-case) already exist and will serve as proof targets.
