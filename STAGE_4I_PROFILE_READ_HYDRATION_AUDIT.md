# STAGE 4I — PROFILE READ / HYDRATION AUDIT

## 1. VERDICT

**AUDIT COMPLETE**

No duplicate read authority proven inside bounded `user/profile` path. Canonical read/hydration chain still intact for profile entity and cover photo.

---

## 2. CANONICAL READ PATH

### Profile read
```
UserApiDatasource.getUserById('/users/{id}')
  → UserApiResponse.fromJson
  → UserApiMapper.toProfileEntity
  → ProfileRepositoryApi.getProfile
  → profileStreamProvider / profile_notifier.fetchProfile
  → profile_screen._getProfileData / UI consumers
```

### Cover read
```
backend UserApiResponse.profile.cover_photo_url (resolved URL)
  → UserProfileApiResponse.fromJson
  → UserApiMapper.toProfileEntity(profile.coverPhotoUrl)
  → ProfileRepositoryApi.getProfile
  → profileStreamProvider
  → profile_screen._getProfileData.coverPhotoUrl
  → UI cover display
```

### Write/persist contract still separate
- persist path uses `UpdateProfileApiRequest.coverPhotoUrl` as storage key
- read path uses backend-resolved URL from `UserProfileApiResponse.coverPhotoUrl`

---

## 3. FIELD AUTHORITY TABLE

| Field | Source on read | Mapper target | UI/state consumer | Authority | Status |
|---|---|---|---|---|---|
| username | `UserApiResponse.username` / `UserProfileApiResponse.username` | `AuthUser.username` | `profile_screen.dart` | backend profile/user response | CANONICAL |
| avatar_url | `UserProfileApiResponse.avatarUrl` / `AuthUser.avatarUrl` | `AuthUser.avatarUrl`, some `ProfileEntity` consumers | profile UI, auth state, chat/social consumers | backend identity/profile response | CANONICAL |
| cover_photo_url | `UserProfileApiResponse.coverPhotoUrl` | `ProfileEntity.coverPhotoUrl` | `profile_screen.dart:966-997`, `:1042` | backend resolved read URL | CANONICAL |
| store_name | not in `ProfileEntity`; from seller identity models | `SellerIdentityData.storeName` | seller-facing UI only | seller domain authority | VALID DOMAIN BOUNDARY |
| store_image_url | not in `ProfileEntity`; from seller identity models | `SellerIdentityData.storeImageUrl` | seller-facing UI only | seller domain authority | VALID DOMAIN BOUNDARY |

---

## 4. PRODUCTION READ CALLER MAP

| Caller | Method | Authority | Status |
|---|---|---|---|
| `profile_notifier.dart:33-52` | `fetchProfile` → `repository.getProfile` | repository read authority | CANONICAL |
| `profile_core_provider.dart:47-59` | `profileProvider` / `getProfileUseCaseProvider` | use case read authority | CANONICAL |
| `profile_stream_provider.dart` | repository-backed profile stream | repository read authority | CANONICAL |
| `profile_screen.dart:965-1042` | `profileStreamProvider(userId)` and `_getProfileData` | UI hydration from profile stream + auth state | CANONICAL |
| `edit_profile_save_handler.dart:98-129` | reads `cachedProfile` before save | canonical pre-save read | CANONICAL |
| `user_api_datasource.dart:73-77` | `getUserById` | direct datasource boundary inside repository flow | VALID DOMAIN BOUNDARY |
| `profile_repository_api.dart:34-50` | `getProfile` | canonical repository read | CANONICAL |

No production caller found that bypasses repository/state for profile hydration.

---

## 5. DUPLICATE / BYPASS FINDINGS

### 5.1 No duplicate read authority proven
- `ProfileRepositoryApi.getProfile` is sole profile hydration repository.
- `GetProfileUseCase` wraps repository only; no alternate profile-read authority found.
- `ProfileNotifier.fetchProfile` delegates to repository.

### 5.2 No bypass proven for profile read
- `profile_screen.dart` uses `profileStreamProvider(userId)` for profile entity data and `authState` for auth identity data.
- Direct API call exists only inside repository boundary (`UserApiDatasource.getUserById`) — valid boundary, not bypass.

### 5.3 Cover field authority is split but consistent
- Write/persist: `UpdateProfileApiRequest.coverPhotoUrl` stores storage key.
- Read/hydration: `UserProfileApiResponse.coverPhotoUrl` is resolved URL.
- `UserApiMapper.toProfileEntity` passes `profile?.coverPhotoUrl` through unchanged.

No path found that converts read URL into persisted value on hydration.

---

## 6. DEAD / ZOMBIE CANDIDATES

- No new dead profile-read symbols proven.
- `UpdateProfileUseCase.updateFields` and `ProfileNotifier.updateFields` remain absent; not part of read/hydration audit.
- `profile_core_provider.profileActionsProvider` and `profile_notifier.fetchProfile` remain live.

---

## 7. STALE TEST CONTRACTS

Potential stale/read-contract locks found:

1. `apps/mobile/test/domains/user/profile/cover_photo_contract_test.dart`
   - locks `UpdateProfileApiRequest.coverPhotoUrl` serialization to `cover_photo_url` with empty string clear semantics.
   - still valid for persist contract.

2. `apps/mobile/test/domains/user/profile/profile_screen_identity_state_test.dart`
   - likely locks hydration from profile/user state; no stale read contract proven from current inspection.

3. `apps/mobile/test/domains/user/profile/profile_post_save_audit_test.dart`
   - not a read test; uses profile save path and canonical `ProfileActions`.

No test was proven to still assert `coverPhotoUrl == null` on read when backend returns resolved URL.

---

## 8. COVER READ-BACK PROOF

Evidence from current read path:
- `UserProfileApiResponse.fromJson` reads `cover_photo_url` into `coverPhotoUrl` (`user_api_models.dart:589-595`).
- `UserApiMapper.toProfileEntity` assigns `coverPhotoUrl: profile?.coverPhotoUrl` unchanged (`user_api_mapper.dart:55-71`).
- `profile_screen.dart:965-997` uses `profileAsync.value?.coverPhotoUrl` directly for UI cover display.

Therefore:
- backend read value is treated as resolved URL on hydration,
- mapper does not null it out,
- UI consumer receives same value,
- persistence contract remains separate and unchanged.

No evidence of avatar/cover/store field swapping inside profile read path.

---

## 9. BASELINE BLOCKERS

Full suite remains blocked by pre-existing unrelated errors in Commerce/Seller/other domains. Not re-litigated here.

No profile-read-specific blocker found in bounded inspection.

---

## 10. RECOMMENDED NEXT SINGLE BOUNDED STEP

**Stage 4I-1:** add one focused read contract test for `UserApiMapper.toProfileEntity` proving `profile.coverPhotoUrl` preserves backend resolved URL from `UserProfileApiResponse.coverPhotoUrl` and does not touch persistence behavior.

---

## 11. FILES CHANGED

- production: `0`
- tests: `0`
- schema: `0`
- docs: `1` (`STAGE_4I_PROFILE_READ_HYDRATION_AUDIT.md`)
