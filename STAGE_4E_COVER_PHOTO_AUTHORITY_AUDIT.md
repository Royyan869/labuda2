# STAGE 4E — COVER PHOTO AUTHORITY & ROUND-TRIP READ-ONLY AUDIT

**Date:** 2026-08-25
**Type:** Read-only audit. No production code, schema, migration, or test was modified.
**Scope:** Cover photo only — mobile client round-trip, backend persistence/API, tests, media infrastructure.

---

## 1. VERDICT

**Cover photo is a "schema-present, API-absent, upload-only" feature.** The database column exists (`user_profiles.cover_photo_url` + `cover_photo_updated_at`), the mobile client can upload a real image to S3 via the presigned endpoint and receives a real public URL — but **no Go code reads or writes the column, no API request/response carries the field, and the mobile mapper hardcodes `null` on every hydration**. The user can never see a cover photo anywhere; re-uploads accumulate orphaned S3 objects; removal deletes nothing.

The single root cause is the **missing wire contract** (request + response), not the schema and not the UI. The schema, the media pipeline (presigned upload + CDN read resolution), the mobile entity, the edit UI, the save choreography, and several tests already exist and pin the canonical key convention `images/profile-covers/{userId}.jpg`.

---

## 2. CURRENT FACTUAL FLOW (verified file:line)

### Mobile (write path)
```
unified_edit_profile_screen.dart:439-462   _changeCover/_removeCover → AvatarEditorWidget.showEditModal(16:9)
edit_profile_save_handler.dart:58-94       save() two-phase: savePersonal() [avatar only] + saveProfileFields()
edit_profile_save_handler.dart:151-178     prepareCoverPhoto():
                                             removal → coverPhotoUploadService.deleteCoverPhoto(userId) :155
                                             new     → coverPhotoUploadService.uploadCoverPhoto(userId, path) :162
                                             no-op   → (hasUpdate:false, url: coverPhotoUrl) :177
edit_profile_save_handler.dart:117         fields['coverPhotoUrl'] = coverResult.url
profile_core_provider.dart:97-169          ProfileActions.updateFields → ProfileEntity(coverPhotoUrl: fields[...]) :125
update_profile_use_case.dart:18-30         call() → _validateProfile :66-73 (validates cover URL format)
profile_repository_api.dart:74-89          updateProfile → _profileEntityToUpdateRequest :285-297
                                             **coverPhotoUrl DROPPED — not in request** ← GAP 1
user_api_datasource.dart:92-99             PATCH /users/me/profile
```

### Upload mechanics (mobile)
```
cover_photo_upload_service.dart:44         key = 'images/covers/{userId}.jpg'  ← LEGACY prefix
cover_photo_upload_service.dart:45         s3Service.uploadImageWithKey(file, key)
s3_service.dart:336-338                    uploadImageWithKey IGNORES key → falls through to presigned flow
s3_service.dart:260-328                    presigned POST /media/upload-url → PUT bytes → returns backend-assigned publicUrl
cover_photo_upload_service.dart:52         Result.success(publicUrl)  ← real S3 URL, object now exists in storage
cover_photo_upload_service.dart:67-74      getCoverPhotoUrl(userId) — ZOMBIE helper, 0 callers, points at fixed key object that never gets created
s3_service.dart:368-373                    deleteFile — no-op stub returning success (removal deletes nothing)
```

### Backend (current state)
```
migrations/000020_add_profile_cover.up.sql:2     ALTER TABLE user_profiles ADD cover_photo_url text;         ← EXISTS
migrations/000022_asset_specific_media_authority.up.sql:2  ADD cover_photo_updated_at; backfill          ← EXISTS
entity/user_profile.go:11-34                       UserProfile struct — NO CoverPhotoURL field
entity/user_profile.go:58-67                       UpdateProfileInput — NO CoverPhotoURL field
entity/user.go:53-81                               UserPublicInfo — NO CoverPhotoURL field
repository/user_repository_impl.go:244-261         GetProfileByID SELECT — NO cover column
repository/user_repository_impl.go:318-335         GetPublicInfo SELECT — NO cover column
repository/user_repository_impl.go:551-633         UpdateProfile dynamic UPDATE — NO cover column
delivery/http/user_handler.go:33-39               updateProfileRequest DTO — NO cover field
delivery/http/dto/user_response.go:35-55          ProfileDTO — NO cover field
delivery/http/dto/user_response.go:90-113         PublicUserResponse — NO cover field
application/user_profile_service.go:389-427       entityToProfileDTO — NO cover
cmd/core_server/routes_core.go:219-220            GET /users/me, PATCH /users/me/profile (no cover anywhere)
```

### Mobile (read/hydration path)
```
profile_repository_api.dart:34-51        getProfile → UserApiMapper.toProfileEntity
user_api_mapper.dart:61                  coverPhotoUrl: null  // Not in API yet   ← GAP 2 (single choke point)
user_api_models.dart:404-602             UserApiResponse/UserProfileApiResponse fromJson — NO cover key parsed
profile_repository_api.dart:197-206      watchProfile → polls getProfile every 30s → always null cover
profile_screen.dart:965-968              coverPhotoUrl = profileStreamProvider.value?.coverPhotoUrl → always null
profile_screen.dart:997/1042             'coverPhotoUrl' map entry → ProfileCover → gradient fallback (never an image)
```

**Net effect:** the S3 object exists, but no API call records it; every hydration returns null; the user never sees the cover; removal is a silent no-op; each re-upload leaves another orphaned object.

---

## 3. CANONICAL AUTHORITY (existing)

| Layer | Authority | State |
|---|---|---|
| Wire field name | `cover_photo_url` (snake_case) | **PINNED by mobile test** `profile_repository_api_watch_profile_test.dart:107,131` (JSON `'cover_photo_url'` → entity) and backend test `user_profile_service_media_resolution_test.go:166,174,194` |
| Storage key convention | `images/profile-covers/{userId}.jpg` | **PINNED** by `update_profile_use_case_test.dart` (canonical key), `profile_post_save_audit_test.dart` fakes, `user_profile_service_media_resolution_test.go`, `mediaupload/handler_test.go` (explicitly REJECTS legacy `images/covers/`) |
| Mobile entity field | `ProfileEntity.coverPhotoUrl` | Exists, read by profile_screen/ProfileCover/edit sections |
| Backend schema | `user_profiles.cover_photo_url` + `cover_photo_updated_at` | **EXISTS** (000020/000022) |
| Media read resolution | `mediaresolve.ResolveMediaReadURL` → CDN or presigned GET | Exists, used by commerce/feed projections; NOT wired into user profile service |
| Upload endpoint | `POST /api/v1/media/upload-url` (auth-protected, MIME allowlist) | Exists, returns `{storage_key, upload_url, expires_at, public_url}` |
| Mobile mapper | `UserApiMapper.toProfileEntity` | **Hardcodes null** — canonical site, currently wrong-by-omission |

---

## 4. EXACT GAP / ROOT CAUSE

**Root cause: the wire contract is missing on both directions.**

- **GAP 1 (write):** `UpdateProfileApiRequest` (mobile `user_api_models.dart:235-265`) has **no cover field**; `_profileEntityToUpdateRequest` (`profile_repository_api.dart:285-297`) drops `profile.coverPhotoUrl`; backend `updateProfileRequest` (`user_handler.go:33-39`) has **no cover field**; backend `UpdateProfile` repository SQL never touches `cover_photo_url`. → The uploaded URL never reaches the database.
- **GAP 2 (read):** backend SELECTs never include `cover_photo_url`; response DTOs (`ProfileDTO`, `PublicUserResponse`) never serialize it; mobile `fromJson` never parses it; `UserApiMapper.toProfileEntity:61` hardcodes `null`. → Hydration is always null.
- **GAP 3 (key convention divergence):** mobile `cover_photo_upload_service.dart:44` uses legacy `images/covers/` while the entire downstream contract expects `images/profile-covers/`. Also `uploadImageWithKey` ignores the key (`s3_service.dart:336-338`), so the client cannot produce a fixed key at all today.
- **GAP 4 (delete):** mobile `deleteFile` is a no-op; backend has no delete endpoint (delete-url tests reference nonexistent `RequestDeleteURL`/`PresignDELETE`). Cover removal can only clear a DB field — which today is never set anyway.

---

## 5. AFFECTED FILES (factual inventory — nothing modified)

### Mobile — would need change to complete the round-trip
| File | Change |
|---|---|
| `lib/domains/user/profile/data/models/api/user_api_models.dart` | Add `coverPhotoUrl` to `UpdateProfileApiRequest` (+ toJson), parse `cover_photo_url` in `UserProfileApiResponse.fromJson` |
| `lib/domains/user/profile/data/mappers/user_api_mapper.dart` | `toUpdateProfileRequest` add cover; `toProfileEntity:61` map `cover_photo_url` instead of hardcoded null |
| `lib/domains/user/profile/data/repositories/profile_repository_api.dart` | `_profileEntityToUpdateRequest:285` include `profile.coverPhotoUrl` |
| `lib/domains/user/profile/data/services/cover_photo_upload_service.dart` | Fix prefix `images/profile-covers/`; use a real fixed-key upload path (needs S3Service change) |
| `lib/core/services/s3_service.dart` | Either implement fixed-key presign or surface `storageKey` from the presign response and persist the key (canonical pattern: persist KEY, not resolved URL) |
| `lib/domains/user/profile/data/services/avatar_upload_service.dart`, `store_photo_upload_service.dart` | Same prefix/key contract (identical clones) — only if in scope (avatar/store are NOT cover scope; list for completeness) |
| `lib/domains/user/profile/domain/use_cases/update_profile_use_case.dart` | `updateFields` (ZOMBIE, has `fields['farmInfo?']` typo :52) — decide: repair + use, or delete |

### Backend — would need change
| File | Change |
|---|---|
| `internal/identity/user/domain/entity/user_profile.go` | Add `CoverPhotoURL` (+`CoverPhotoUpdatedAt`) to `UserProfile`; add `CoverPhotoURL` to `UpdateProfileInput` |
| `internal/identity/user/domain/entity/user.go` | Add `CoverPhotoURL` to `UserPublicInfo` |
| `internal/identity/user/infrastructure/repository/user_repository_impl.go` | SELECT cover in GetProfileByID/GetPublicInfo; SET cover in UpdateProfile; INSERT in create fallback |
| `internal/identity/user/delivery/http/user_handler.go` | `updateProfileRequest` + mapping + URL validation (mirror avatar :247-259) |
| `internal/identity/user/delivery/http/dto/user_response.go` | `ProfileDTO` + `PublicUserResponse` add cover |
| `internal/identity/user/application/user_profile_service.go` | `entityToProfileDTO` + `GetPublicProfile` projection; optional `mediaresolve` resolution (persist KEY, resolve to CDN on read) |

### Already-present (no change needed)
- `migrations/000020_add_profile_cover.{up,down}.sql`, `000022_asset_specific_media_authority.{up,down}.sql`

---

## 6. EXISTING MEDIA / UPLOAD INFRASTRUCTURE (reusable, factual)

| Primitive | Location | Reusability for cover |
|---|---|---|
| Presigned upload pipeline (client) | `s3_service.dart:124-167` (`_requestMediaPresignURL` + `_putToPresignedUrl`) | Reuse as-is; need fixed-key variant or key persistence |
| Upload endpoint (backend) | `mediaupload/handler.go:89-142`, `s3presign.PresignPUT`, route `routes_core.go:227` | Reuse; currently mints timestamped keys, no fixed-key allowlist |
| Read resolution (backend) | `mediaresolve/mediaresolve.go:61-91` (`ResolveMediaReadURL`, CDN/presigned-GET) | Reuse; already proven for `images/profile-covers/` by test; NOT wired into user profile service |
| Media URL constants (mobile) | `cover_photo_upload_service.dart:67-74` (`getCoverPhotoUrl` — CDN/S3 base) | ZOMBIE; would be correct only after fixed-key upload exists |
| Save choreography | `edit_profile_save_handler.dart:113-178` | Reuse as-is (already orchestrates upload/removal/publication-revision) |
| Edit UI | `edit_profile_cover_section.dart` (+ 7 sibling files) | Reuse as-is |
| Canonical key pattern (reference) | Seller store image: `store_image_url` + `UpdateStoreImageTx` + mediaresolve in seller projections | **Pattern reference** — the closest shipped analog to cover |

**Not reusable / absent:** backend delete endpoint (none; tests reference a design that was not shipped), mobile `deleteFile` (no-op stub), `uploadImageWithFixedKey` (does not exist), fixed-key validation in `mediaupload/handler.go` (absent; only in tests).

---

## 7. SCHEMA / API STATUS

| Layer | Status | Evidence |
|---|---|---|
| DB column | ✅ **EXISTS** | `000020` adds `cover_photo_url`; `000022` adds `cover_photo_updated_at` + backfill |
| Go entity | ❌ **ABSENT** | No field on `UserProfile`/`UserPublicInfo`/`UpdateProfileInput` |
| Repository SQL | ❌ **ABSENT** | No SELECT/SET/INSERT touches cover |
| Request API | ❌ **ABSENT** | No cover in `updateProfileRequest` or mobile `UpdateProfileApiRequest` |
| Response API | ❌ **ABSENT** | No cover in `ProfileDTO`/`PublicUserResponse` or mobile `fromJson` |
| Mobile mapper | ⚠️ **NULL HARDCODE** | `user_api_mapper.dart:61` `coverPhotoUrl: null // Not in API yet` — factually accurate today |
| Upload endpoint | ✅ **EXISTS (generic)** | `POST /media/upload-url` works; no fixed-key/ownership semantics for cover |
| Delete endpoint | ❌ **ABSENT** | Only referenced in non-compiling tests |

---

## 8. CONSUMERS (all readers/writers of coverPhotoUrl)

### Mobile — writers
| File:line | Role | Status |
|---|---|---|
| `user_api_mapper.dart:61` | Only mapper; hardcodes null | CANONICAL (choke point) |
| `profile_core_provider.dart:120-125` (`ProfileActions.updateFields`) | Local entity write before repo call | ACTIVE — value dies at repo boundary |
| `update_profile_use_case.dart:50` (`updateFields`) | cover in entity rebuild | **ZOMBIE** — never called; has `'farmInfo?'` typo :52 |
| `profile_notifier.dart:91-94` (`ProfileNotifier.updateFields`) | cover copyWith | **ZOMBIE** — notifier not consumed by any screen |
| `unified_edit_profile_screen.dart:217-218, 448-451` | Seed + UI state | ACTIVE (seed always null) |
| `profile_entity.dart:153` (copyWith) | `coverPhotoUrl ?? this.coverPhotoUrl` — cannot express removal | ACTIVE (structural quirk) |

### Mobile — readers
| File:line | Role | Status |
|---|---|---|
| `profile_screen.dart:965-968, 997, 1042` | Primary display — always null | ACTIVE (broken by GAP 2) |
| `profile_header_builder.dart:58` | Pass-through to ProfileCover | ACTIVE |
| `profile_cover.dart:11, 48-50` | `Image.network` + gradient fallback | ACTIVE (never renders image) |
| `edit_profile_cover_section.dart:9, 27, 118-120` | Edit preview | ACTIVE |
| `edit_profile_save_handler.dart:39, 154-177` | Save decision | ACTIVE |
| `update_profile_use_case.dart:66-73` | Validates a value that never arrives | ACTIVE (dead validation) |

**No consumers outside the profile domain.** `AuthUser` has no cover; `authController.updateProfile` has no cover param; profile_share_builder/drawer/seller identity do not use cover.

---

## 9. TESTS (inventory + status)

### Mobile tests
| Test | Targets | Status |
|---|---|---|
| `profile_repository_api_watch_profile_test.dart` | Repository hydration: `'cover_photo_url': 'images/profile-covers/…'` → `ProfileEntity.coverPhotoUrl` | ⚠️ **STALE/FAILING at baseline** — pins the wire contract that the mapper hardcodes to null; cannot pass against current `toProfileEntity` |
| `cover_photo_upload_service_test.dart` | Upload service: asserts `images/profile-covers/{userId}.jpg` + `uploadImageWithFixedKey` | ⚠️ **STALE** — `uploadImageWithFixedKey` doesn't exist; production service uses `images/covers/` + `uploadImageWithKey` |
| `avatar_upload_service_test.dart` | Same mirror for avatar | ⚠️ **STALE** — same nonexistent method |
| `update_profile_use_case_test.dart` | Use case: canonical key accepted, traversal rejected, legacy https accepted | ✅ **LIVE** — pins canonical prefix + validation contract |
| `profile_post_save_audit_test.dart` | Save handler: cover publish/removal/failure/rapid-save; fakes use `images/profile-covers/` | ✅ **LIVE** — richest cover consumer test |
| `edit_profile_cover_section_test.dart` | Widget contract (props/labels) | ✅ **LIVE** — but no upload-wiring assertions |
| `profile_screen_identity_state_test.dart` | ProfileScreen: cover as INPUT fixture only | ✅ LIVE — no cover-rendering assertion |

### Backend tests
| Test | Targets | Status |
|---|---|---|
| `user_profile_service_media_resolution_test.go` | Service read projection resolves `CoverPhotoURL` → CDN | ⚠️ **NON-COMPILING** — references `UserPublicInfo.CoverPhotoURL` / `UserProfile.CoverPhotoURL`; **fields do not exist in current entities** |
| `mediaupload/handler_test.go` | Fixed-key cover upload, rejection of legacy `images/covers/`, delete-url endpoint | ⚠️ **PARTIALLY NON-COMPILING** — references `StorageKey`, `RequestDeleteURL`, `validateOwnedCommerceMediaKey`, `PresignDELETE`; none exist in production handler |
| `seller_profile_update_integration_test.go` | Seller update preserves cover via `UpdateProfileInput.CoverPhotoURL` | ⚠️ **NON-COMPILING** — `UpdateProfileInput` has no such field |
| `user_handler_test.go`, `public_profile_identity_projection_test.go` | — | ❌ **GAP** — zero cover coverage |

**Pattern:** mobile tests pin the *future* wire contract (`cover_photo_url`, `images/profile-covers/`) while backend tests reference *future* entity fields — i.e. the tests describe the intended design; the implementation is absent. Several backend tests do not compile against the current tree (consistent with pre-existing `backend/vet_errors.txt` baseline failures).

---

## 10. RECOMMENDED BOUNDED IMPLEMENTATION PLAN (NOT executed — for decision)

### Decision gate FIRST (see §11) — owner must confirm:
1. **Persist KEY vs resolved URL** — canonical pattern (mediaresolve + `user_profile_service_media_resolution_test.go`) says **persist the storage key** (`images/profile-covers/{userId}.jpg`), resolve to CDN on read. Mobile must therefore store the returned `storage_key` from the presign response, not the `public_url`.
2. **Fixed-key vs timestamped-key** — the tests pin `images/profile-covers/{userId}.jpg` (fixed, owned, jpeg-only). The current handler mints timestamped keys. Decision: implement the fixed-key allowlist (tests already specify it) or adopt timestamped keys + persist whatever key is returned.

### Implementation slices (smallest coherent units)
**Slice A — Backend wire (after decision):**
- `user_profile.go`/`user.go`: add `CoverPhotoURL` (+ `CoverPhotoUpdatedAt`) fields
- `user_repository_impl.go`: SELECT/SET/INSERT cover
- `user_handler.go`: accept + validate cover URL (mirror avatar); `hasEffectiveProfileUpdate` includes it
- `user_response.go`: serialize cover in `ProfileDTO` (+ optionally `PublicUserResponse`)
- `user_profile_service.go`: map + resolve via `mediaresolve` (persist key, emit CDN URL)
- `mediaupload/handler.go`: fixed-key allowlist for `images/profile-covers/{userId}.jpg` (jpeg only, ownership check) OR accept timestamped keys — per decision
- Repair the 3 non-compiling backend tests to match shipped design

**Slice B — Mobile wire:**
- `user_api_models.dart`: add cover to `UpdateProfileApiRequest` + parse `cover_photo_url` in `UserProfileApiResponse.fromJson`
- `user_api_mapper.dart`: map cover in `toUpdateProfileRequest` and `toProfileEntity` (replace `:61` hardcode)
- `profile_repository_api.dart`: include cover in `_profileEntityToUpdateRequest`
- `cover_photo_upload_service.dart`: use canonical prefix; persist `storage_key` (needs `s3_service.dart` to surface key from presign response or implement fixed-key presign)
- Repair stale mobile tests (`watch_profile`, `cover_photo_upload_service`) to the decided contract

**Slice C — Residue (bounded, cover-only):**
- Resolve the two ZOMBIE writers (`update_profile_use_case.updateFields` with `'farmInfo?'` typo, `ProfileNotifier.updateFields`) — repair + wire or delete
- Cover removal semantics: with no backend delete endpoint, removal = clear DB field + optionally leave object for lifecycle policy (document; do NOT build a delete endpoint in this stage unless owner decides)
- `getCoverPhotoUrl` zombie helper — fix or delete

**Proof:** backend `go test` for user handler/service/repo + mediaupload; mobile `flutter test` for mapper/repository/save-handler/watch-profile; manual round-trip proof (upload → PATCH → GET → render).

---

## 11. STOP CONDITIONS / BUSINESS DECISIONS

| # | Decision | Options | STOP if… |
|---|---|---|---|
| D-1 | **Key convention**: fixed `images/profile-covers/{userId}.jpg` (single object, overwrite) vs timestamped keys (append-only, multiple objects) | (a) Fixed key per tests (Recommended — matches all pinned tests, natural overwrite) (b) Timestamped (needs no handler change, but storage grows + tests must be rewritten) | Owner picks (b) → STOP and rewrite contract expectations; do not implement fixed-key |
| D-2 | **What to persist**: storage key (canonical, mediaresolve resolves on read) vs resolved public URL | (a) Key (Recommended — matches mediaresolve pattern + backend test) (b) URL (simpler client, but breaks CDN resolution + backend test) | Owner picks (b) → STOP; document divergence |
| D-3 | **Delete semantics**: no backend delete endpoint exists; removal clears DB field only | (a) Clear field, leave object to S3 lifecycle (no new endpoint) (b) Build delete endpoint (scope expansion) | Owner requires (b) → STOP, separate stage |
| D-4 | **Scope of the 3 non-compiling backend tests + stale mobile tests** | (a) Repair to shipped design (b) Delete if design changes | Tests contradict chosen design → STOP and reconcile |
| D-5 | **Zombie writers** (`UpdateProfileUseCase.updateFields` with `'farmInfo?'` typo, `ProfileNotifier.updateFields`) | (a) Repair + wire (b) Delete | Any consumer of the zombie path exists → STOP and audit |

**Hard STOP conditions (no implementation):**
- Any new schema/migration — the column already exists; none needed.
- Any change to Commerce/username/password/validation/settings/address/global cleanup.
- Any invention of business rules, endpoints, or media architecture beyond what tests already pin.

---

## 12. FACT vs INFERENCE

| Claim | Status |
|---|---|
| `cover_photo_url` column exists in DB (000020/000022) | FACT (read migrations) |
| No Go field/SQL/API touches cover | FACT (read entity, repo, handler, DTO, service) |
| Mobile upload produces a real S3 public URL via presigned flow | FACT (read s3_service + cover service) |
| Uploaded URL dropped at `_profileEntityToUpdateRequest` | FACT (read) |
| `user_api_mapper.dart:61` hardcodes null | FACT (read) |
| Profile screen can never render cover | FACT (trace: all readers receive null) |
| `uploadImageWithKey` ignores key; `deleteFile` is no-op; `uploadImageWithFixedKey` absent | FACT (read s3_service; grep 0 hits) |
| Mobile upload service uses legacy `images/covers/` prefix | FACT (read) |
| Canonical prefix `images/profile-covers/` pinned by multiple tests | FACT (read 5 test files) |
| `user_profile_service_media_resolution_test.go`, `seller_profile_update_integration_test.go` reference nonexistent entity fields | FACT (read test + entity; fields absent) |
| `mediaupload/handler_test.go` references nonexistent `RequestDeleteURL`/`PresignDELETE` | FACT (grep: only in tests) |
| `profile_repository_api_watch_profile_test.dart` cannot pass against current mapper | FACT (read test + mapper) |
| Recommended plan is the minimal viable path | INFERENCE (based on existing tests + mediaresolve pattern) |
| Persisting KEY is canonical | INFERENCE (from mediaresolve design + backend test) — flagged as D-2 |
