# FOUNDATION-04 — FIREBASE SERVICE FORENSIC AUDIT

**Date:** 2026-09-01
**Scope:** Complete Firebase surface audit of `apps/mobile`
**Method:** Import tracing, runtime consumer verification, dependency analysis
**Verdict:** `FOUNDATION-04 — PASS WITH RESIDUAL RISK`

---

## 1. EXECUTIVE VERDICT

```
FOUNDATION-04 — PASS WITH RESIDUAL RISK
```

The current canonical Firebase surface is:

```
Firebase Auth       ← identity (ID token → Go backend)
Firebase Messaging  ← push notifications (token → Go backend)
Firebase Analytics  ← event/screen tracking (standalone)
Firebase Core       ← infrastructure (required by above)
```

Three Firebase services have **dependency presence with zero runtime consumers**:

```
Firebase Crashlytics  ← dead residue (setUserIdentifier only, no error recording)
Firebase Performance  ← dead residue (provider defined, never consumed)
Firebase Remote Config ← dead residue (provider defined, never consumed)
```

One legacy artifact exists with **zero consumers**:

```
firestore_collections.dart ← dead residue (exported via core.dart, never imported)
```

**No P0 or P1 findings.** No security bypass. No dual storage authority. No active feature depends on an obsolete Firebase service.

---

## 2. FIREBASE SERVICE INVENTORY

| Service | Dependency | Initialization | Runtime Consumer | Purpose | Authority | Status |
|---------|-----------|---------------|-----------------|---------|-----------|--------|
| **Auth** | `firebase_auth: ^6.1.0` | `Firebase.initializeApp()` in `main.dart:159` | `FirebaseAuthenticationService`, `AuthInterceptor`, `AuthController`, repositories | ID token for Go backend auth | Go backend verifies Firebase token | **CANONICAL** |
| **Firestore** | ❌ Commented out | ❌ None | ❌ Zero consumers | — | — | **DEAD / SUNSET** |
| **Storage** | ❌ Commented out | ❌ None | ❌ Zero consumers | — | — | **DEAD / MIGRATED TO S3** |
| **FCM** | `firebase_messaging: ^16.0.3` | `FirebaseMessaging.instance` | `FcmService` → `FCMTokenManager` → `NotificationApiDatasource` → Go backend | Push notification delivery | Backend sends via FCM | **CANONICAL** |
| **Analytics** | `firebase_analytics: ^12.0.3` | `FirebaseAnalytics.instance` in `main.dart:212` | `ScreenViewRouteObserver` (mounted in router), `AuthController`, `FollowActionsProvider` | Event/screen tracking | Standalone Firebase | **CANONICAL** |
| **Crashlytics** | `firebase_crashlytics: ^5.0.3` | `FirebaseCrashlyticsImpl.instance()` in `auth_controller.dart:369` | `AuthController` — only `setUserIdentifier`/`clearUserIdentifier` | User identity tagging (NO error recording) | — | **DEAD RESIDUE** |
| **Performance** | `firebase_performance: ^0.11.0` | `FirebasePerformance.instance` via provider | ❌ Zero consumers | — | — | **DEAD** |
| **Remote Config** | `firebase_remote_config: ^6.0.2` | `FirebaseRemoteConfig.instance` via provider | ❌ Zero consumers | — | — | **DEAD** |
| **Functions** | ❌ Not in pubspec | ❌ None | ❌ None | — | — | **NOT PRESENT** |
| **Dynamic Links** | ❌ Not in pubspec | ❌ None | ❌ None | — | — | **NOT PRESENT** |
| **App Check** | ❌ Not in pubspec | ❌ None | ❌ None | — | — | **NOT PRESENT** |

---

## 3. FIREBASE INITIALIZATION

**Single initialization point:** `main.dart:159`

```dart
await Firebase.initializeApp(
  options: DefaultFirebaseOptions.currentPlatform,
);
```

**Characteristics:**
- One initialization authority (single call in `_initServices()`)
- Unconditional (runs on every app start)
- Error-tolerant: catches `duplicate-app` and logs non-fatal failures
- `firebase_options.dart`: FlutterFire CLI generated from `labuda-79de2` project
- Configuration files: `google-services.json` (Android), `GoogleService-Info.plist` (iOS)

**No old Firebase bootstrap remains.** No multiple initialization mechanisms.

---

## 4. FIREBASE AUTH

**Status:** CANONICAL — PROVEN ACTIVE

**Verified flow:**

```
Firebase Auth (mobile)
    ↓ signInWithEmail / signInWithGoogle / signInWithApple
    ↓ FirebaseAuth.instance.currentUser.getIdToken(forceRefresh)
    ↓
AuthInterceptor (Dio interceptor)
    ↓ Attaches Bearer token to every API request
    ↓
Go Backend API
    ↓ Firebase Admin SDK verifies token
    ↓ Business authorization
```

**Runtime consumers (production code only):**

| File | Usage |
|------|-------|
| `firebase_authentication_service.dart` | Email, Google, Apple sign-in; sign-out |
| `firebase_auth_core_service.dart` | Core Firebase Auth operations |
| `firebase_auth_social_service.dart` | Google/Apple OAuth |
| `firebase_auth_password_service.dart` | Password reset, email verification |
| `firebase_auth_token_service.dart` | Token refresh, account deletion |
| `auth_interceptor.dart` | Bearer token attachment, 401 retry, session expiry |
| `auth_controller.dart` | Auth state management, session hydration |
| `auth_repository_impl.dart` | Backend sync on auth events |
| `auth_signup_repository.dart` | Registration flow |
| `auth_core_repository.dart` | Core auth operations |
| `auth_google_repository.dart` | Google sign-in backend sync |
| `auth_profile_repository.dart` | Profile sync after auth |
| `auth_persistence_service.dart` | Local auth persistence |
| `firebase_principal.dart` | Domain entity for auth principal |
| `user_sync_service.dart` | Backend user sync |
| `phone_verification_service.dart` | Phone OTP verification |
| `sign_in_screen.dart` | Sign-in UI |
| `email_verification_controller.dart` | Email verification flow |
| `seller_upgrade_wizard_screen.dart` | Uses `FirebaseAuth` for current user check |

**No other Firebase service is used as an identity authority.**

---

## 5. FIRESTORE

**Status:** DEAD / SUNSET — PROVEN DEAD

**Evidence:**

| Check | Result |
|-------|--------|
| `cloud_firestore` in `pubspec.yaml`? | ❌ Commented out: `# cloud_firestore: ^6.0.2  # ❌ REMOVED` |
| `cloud_firestore` in `pubspec.lock`? | ❌ Not resolved |
| `import.*cloud_firestore` in Dart? | ❌ Zero matches |
| `FirebaseFirestore` in Dart? | ❌ Zero matches |
| Firestore initialization? | ❌ None |

**`firestore_collections.dart` analysis:**

- 200+ lines defining collection name constants (`AuthCollections`, `ProfileCollections`, `ListingCollections`, etc.)
- `FirestoreCollectionMigration` helper class with `migrationMap` and `isDeprecated()`
- Exported via `core.dart` barrel: `export 'constants/firestore_collections.dart';`
- **Zero consumers outside the file itself** — all references to `AuthCollections.`, `ChatCollections.`, etc. are self-referential within `firestore_collections.dart`
- No Dart file in `lib/` imports or uses these collection constants for any business purpose

**Classification:**
```
Firestore = DEAD / SUNSET
firestore_collections.dart = DEAD RESIDUE (zero consumers, barrel-exported orphan)
```

---

## 6. FIREBASE STORAGE

**Status:** DEAD / MIGRATED TO S3 — PROVEN DEAD

**Evidence:**

| Check | Result |
|-------|--------|
| `firebase_storage` in `pubspec.yaml`? | ❌ Commented out: `# firebase_storage: ^13.0.2  # ❌ Removed - Migrated to S3` |
| `firebase_storage` in `pubspec.lock`? | ❌ Not resolved |
| `import.*firebase_storage` in Dart? | ❌ Zero matches |
| `FirebaseStorage` in Dart? | ❌ Zero matches |
| `putFile` / `putData` / `getDownloadURL` for Firebase? | ❌ Zero matches |
| `firebasestorage.app` in Dart code? | ❌ Only in `firebase_options.dart` config (required by Firebase SDK) |

**`storageBucket: 'labuda-79de2.firebasestorage.app'`** appears in:
- `firebase_options.dart` (4 platform configs)
- `google-services.json`
- `GoogleService-Info.plist`

This is **standard Firebase boilerplate** required by the Firebase Core SDK initialization. It does NOT prove Firebase Storage is used for media.

**All media uploads verified going through S3:**

```
S3Service → _requestMediaPresignURL() → POST /media/upload-url → Backend
         → _putToPresignedUrl() → PUT to presigned S3 URL
         → Returns public_url / storage_key
```

Consumers: `avatar_upload_service`, `cover_photo_upload_service`, `store_photo_upload_service`, `avatar_image_processor`, `content_submission_handler`, `commerce_media_upload_coordinator`, `for_sale_media_handler`, `external_product_detail_screen`, `order_refund_handler`, `direct_dispute_dialog`.

**No Firebase Storage fallback exists. No dual media authority.**

**Classification:**
```
Firebase Storage = DEAD / MIGRATED TO S3
```

---

## 7. FIREBASE CLOUD MESSAGING (FCM)

**Status:** CANONICAL — PROVEN ACTIVE

**Verified flow:**

```
FirebaseMessaging.instance
    ↓ getToken()
    ↓
FCMTokenManager.initializeToken(userId)
    ↓
NotificationRemoteDatasource.saveUserToken(userId, token)
    ↓
NotificationApiDatasource.registerFCMToken(RegisterFCMTokenRequest)
    ↓ POST /notifications/fcm/token (Go backend)
    ↓
Go Backend stores token → sends push via FCM
```

**Runtime consumers:**

| File | Usage |
|------|-------|
| `fcm_service.dart` | Orchestrator: init, cleanup, topic subscription |
| `fcm_token_manager.dart` | Token fetch, refresh, backend registration, deletion |
| `fcm_message_handler.dart` | Foreground message stream handling |
| `notification_api_datasource.dart` | `registerFCMToken()` / `removeFCMToken()` via Go backend |
| `notification_remote_datasource.dart` | Delegates to `NotificationApiDatasource` |
| `notification_initializer.dart` | Widget that initializes FCM on login |
| `auth_controller.dart:1420,1460` | FCM init on login, cleanup on logout |
| `fcm_service_impl.dart` | `NotificationService` implementation (lazily initialized streams) |
| `notification_trigger_impl.dart` | Notification tap handling |
| `local_notification_service.dart` | Local notification display |
| `notification_di.dart` | DI registration |

**FCM token lifecycle:**
- `initializeToken` → bounded retry (2 attempts) → backend registration
- `onTokenRefresh` → re-registration with old-token cleanup
- `deleteToken` → device-side + backend-side deletion (logout-tolerant)
- `cleanup` on logout → prevents cross-account push leakage

**Classification:**
```
FCM = CANONICAL (active, well-integrated, backend-backed)
```

---

## 8. FIREBASE ANALYTICS

**Status:** CANONICAL — PROVEN ACTIVE (lightweight)

**Runtime consumers:**

| File | Usage |
|------|-------|
| `firebase_analytics_service.dart` | SDK wrapper: `logEvent`, `setUserId`, `setUserProperty`, `logScreenView` |
| `firebase_analytics_repository_impl.dart` | Implements `IAnalyticsRepository` |
| `screen_view_route_observer.dart` | Mounted in `app_router.dart:87` — tracks all route pushes/pops/replaces |
| `auth_controller.dart` | `_analytics` for auth events |
| `follow_actions_provider.dart` | `_analytics` for follow events |

**Mounted in router (confirmed):**
```dart
// app_router.dart:87
observers: [ref.watch(screenViewRouteObserverProvider)],
```

**Note on iOS:** `GoogleService-Info.plist` has `IS_ANALYTICS_ENABLED = false`. This is a legacy plist flag that only affects Firebase SDK auto-collection. The FlutterFire plugin (`firebase_analytics`) operates independently and IS active on iOS.

**Classification:**
```
Analytics = CANONICAL (active, screen_view tracking + custom events)
```

---

## 9. CRASHLYTICS

**Status:** DEAD RESIDUE — dependency present, minimal usage, zero error recording

**Evidence:**

| Check | Result |
|-------|--------|
| `firebase_crashlytics` in `pubspec.yaml`? | ✅ `^5.0.3` |
| Implementation exists? | ✅ `firebase_crashlytics_impl.dart` |
| `GlobalErrorHandler` instantiated? | ❌ NEVER — defined but never constructed |
| `FlutterError.onError` → Crashlytics? | ❌ No — `main.dart` only does `debugPrint` |
| `PlatformDispatcher.onError` → Crashlytics? | ❌ No |
| `crashReporterProvider` consumed? | ❌ Never read outside `providers.dart` |
| `FirebaseCrashlyticsImpl` used directly? | ✅ `auth_controller.dart:369` — `setUserIdentifier()` / `clearUserIdentifier()` only |

**What actually happens:** `AuthController` creates `FirebaseCrashlyticsImpl.instance()` and calls `setUserIdentifier(backendUser.id)` on login and `clearUserIdentifier()` on logout. No errors are ever recorded. No `recordError()` call exists in production code outside the dead `GlobalErrorHandler`.

**Classification:**
```
Crashlytics = DEAD RESIDUE
- Dependency present: YES
- SDK initialized: YES (lazily, via AuthController)
- Error recording: NONE
- Runtime value: user identity tagging only (harmless but pointless without error recording)
```

---

## 10. CLOUD FUNCTIONS

**Status:** NOT PRESENT

| Check | Result |
|-------|--------|
| `cloud_functions` in `pubspec.yaml`? | ❌ Not present |
| `FirebaseFunctions` in Dart? | ❌ Zero matches |
| `httpsCallable` in Dart? | ❌ Zero matches |

**Classification: NOT PRESENT**

---

## 11. REMOTE CONFIG

**Status:** DEAD — dependency present, zero runtime consumers

**Evidence:**

| Check | Result |
|-------|--------|
| `firebase_remote_config` in `pubspec.yaml`? | ✅ `^6.0.2` |
| Implementation exists? | ✅ `firebase_remote_config_impl.dart` |
| `experimentServiceProvider` consumed? | ❌ Never read outside `providers.dart` |
| `FeatureFlags` uses Remote Config? | ❌ Hardcoded: `useGoBackend => true` |
| `FeatureFlag` enum values used in business logic? | ❌ Only example values (`newCheckout`, `enhancedSearch`, etc.) — none referenced |

**Classification:**
```
Remote Config = DEAD
- Dependency present: YES
- SDK initialized: NO (no consumer triggers initialization)
- A/B testing: NOT ACTIVE
```

---

## 12. DYNAMIC LINKS

**Status:** NOT PRESENT

| Check | Result |
|-------|--------|
| `firebase_dynamic_links` in `pubspec.yaml`? | ❌ Not present |
| `DynamicLinks` in Dart? | ❌ Zero matches |

**Note:** `GoogleService-Info.plist` has `IS_APPINVITE_ENABLED = true` — this is legacy boilerplate and does NOT mean Dynamic Links is active.

**Classification: NOT PRESENT**

---

## 13. APP CHECK

**Status:** NOT PRESENT

| Check | Result |
|-------|--------|
| `firebase_app_check` in `pubspec.yaml`? | ❌ Not present |
| `FirebaseAppCheck` in Dart? | ❌ Zero matches |

**Classification: NOT PRESENT**

---

## 14. OTHER FIREBASE SERVICES

### firebase_wilayah_service.dart

Despite the `firebase_` prefix, this is a **local JSON-only** service:

- `USE_FIREBASE = false` — Firebase queries explicitly removed
- Zero Firebase imports
- Reads from local asset JSON files (`assets/data/full/provinces.json`, etc.)
- Classification: **MISNOMER** — active local service with dead Firebase name

---

## 15. CONFIGURATION FILES

| File | Purpose | Services Supported |
|------|---------|-------------------|
| `android/app/google-services.json` | Android Firebase config | Auth, FCM, Analytics |
| `ios/Runner/GoogleService-Info.plist` | iOS Firebase config | Auth, FCM, Analytics |
| `lib/firebase_options.dart` | FlutterFire CLI generated | All platforms |
| `pubspec.yaml` (Firebase section) | Dependency declarations | All active + dead |

**Platform config:**
- Android: `com.labuda.app.labuda`
- iOS: `com.labuda.app.labuda`
- Project: `labuda-79de2`
- No service account files in mobile app

**`storageBucket` in config** — present in all platform configs as `labuda-79de2.firebasestorage.app`. This is standard Firebase Core boilerplate and does NOT prove Firebase Storage usage.

**No configuration exists solely for dead Firebase services** (beyond the `storageBucket` boilerplate).

---

## 16. FIREBASE / S3 BOUNDARY

```
BAD (verified NOT present):
Media → Firebase Storage AND S3

GOOD (verified present):
Media → S3Service → Backend presign → S3 PUT
```

**Evidence of clean boundary:**
- Zero `FirebaseStorage` imports
- Zero `firebase_storage` imports
- All 15+ upload call sites use `S3Service` methods
- `S3Service` uses backend-presigned-URL pattern (no direct AWS credentials)
- `firebase_storage_service.dart` explicitly removed (comment in `shared.dart`)
- No Firebase Storage fallback anywhere

**Boundary: CLEAN — single media storage authority (S3)**

---

## 17. FIREBASE / GO BACKEND BOUNDARY

```
Firebase (mobile responsibilities):
├── Authentication     → ID token generation
├── FCM               → Device token, message streams
├── Analytics          → Event tracking (standalone)
└── Core               → Infrastructure

Go Backend (server responsibilities):
├── API                → Business logic
├── Firebase Admin     → Token verification
├── FCM delivery       → Push notification sending
├── PostgreSQL         → Business persistence
├── Redis              → Cache / realtime
└── S3                 → Media storage
```

**Firebase responsibilities after migration:**
- Firebase Auth: mobile-side identity (token generation, session management)
- FCM: device-side token management, foreground message handling
- Analytics: client-side event tracking (no backend involvement)

**Everything else: Go backend.**

---

## 18. LEGACY FIREBASE ARCHITECTURE

| Pattern | Location | Status | Classification |
|---------|----------|--------|---------------|
| Firestore collection constants | `firestore_collections.dart` | Exported via `core.dart`, zero consumers | **DEAD RESIDUE** |
| `FirestoreCollectionMigration` | `firestore_collections.dart` | Zero consumers | **DEAD RESIDUE** |
| `FirebaseWilayahService` class name | `firebase_wilayah_service.dart` | Active local service, zero Firebase usage | **MISNOMER** |
| `notification_service.dart` Firestore comments | `notification_service.dart` | Comments reference Firestore, implementation throws `UnimplementedError` | **DEAD COMMENTS** |
| `shared.dart` removal comments | `shared.dart` | Documents removed services | **HISTORICAL** |
| `pubspec.yaml` commented-out deps | `pubspec.yaml` | Documents removed deps | **HISTORICAL** |

---

## 19. DEPENDENCY RESIDUE

### pubspec.yaml vs. Actual Usage

| Package | In pubspec | Active Consumer | Cleanup Candidate? |
|---------|-----------|----------------|-------------------|
| `firebase_core: ^4.1.1` | ✅ | ✅ Init + Auth + FCM + Analytics | **NO** |
| `firebase_auth: ^6.1.0` | ✅ | ✅ Full auth flow | **NO** |
| `firebase_messaging: ^16.0.3` | ✅ | ✅ FCM token + push | **NO** |
| `firebase_analytics: ^12.0.3` | ✅ | ✅ Screen view + events | **NO** |
| `firebase_crashlytics: ^5.0.3` | ✅ | ⚠️ `setUserIdentifier` only | **YES** |
| `firebase_performance: ^0.11.0` | ✅ | ❌ Zero consumers | **YES** |
| `firebase_remote_config: ^6.0.2` | ✅ | ❌ Zero consumers | **YES** |
| `cloud_firestore: ^6.0.2` | ❌ (commented) | ❌ | **YES** (file residue) |
| `firebase_storage: ^13.0.2` | ❌ (commented) | ❌ | **YES** (file residue) |

### Barrel Export Residue

| Export | File | Consumers | Cleanup Candidate? |
|--------|------|-----------|-------------------|
| `firestore_collections.dart` | `core.dart:38` | Zero | **YES** |
| `firebase_crashlytics_impl.dart` | `providers.dart` (indirect) | `auth_controller.dart` (setUserIdentifier only) | **YES** (after removing Crashlytics) |
| `firebase_performance_impl.dart` | `providers.dart` (indirect) | Zero | **YES** |
| `firebase_remote_config_impl.dart` | `providers.dart` (indirect) | Zero | **YES** |

---

## 20. FIREBASE AUTHORITY MATRIX

| Firebase Service | Active? | Runtime Proof | Business Purpose | Canonical? | Cleanup Candidate |
|-----------------|--------:|---------------|-----------------|------------|-------------------|
| **Authentication** | ✅ YES | `firebase_auth` imports in 20+ files, `AuthInterceptor` attaches token to all API calls | Identity — ID token for Go backend | ✅ YES | NO |
| **Firestore** | ❌ NO | Zero imports, zero consumers, dependency removed | — | — | YES (file residue) |
| **Storage** | ❌ NO | Zero imports, zero consumers, dependency removed | — | — | YES (comment residue) |
| **FCM** | ✅ YES | `FirebaseMessaging` in 6 files, token flow → Go backend, `auth_controller.dart` init/cleanup | Push notifications | ✅ YES | NO |
| **Analytics** | ✅ YES | `FirebaseAnalytics` in 3 files, `ScreenViewRouteObserver` mounted in router | Event/screen tracking | ✅ YES | NO |
| **Crashlytics** | ⚠️ MINIMAL | `setUserIdentifier` in `auth_controller.dart` only; no error recording | User identity tagging (useless without error recording) | ❌ NO | YES |
| **Performance** | ❌ NO | Provider defined but never consumed; never mounted in router | — | — | YES |
| **Functions** | ❌ N/A | Not in pubspec | — | — | N/A |
| **Remote Config** | ❌ NO | Provider defined but never consumed; `FeatureFlags` hardcoded | — | — | YES |
| **Dynamic Links** | ❌ N/A | Not in pubspec | — | — | N/A |
| **App Check** | ❌ N/A | Not in pubspec | — | — | N/A |

---

## 21. P0 / P1 FINDINGS

### P0: NONE

No Firebase service conflict exists. No security bypass is possible. No dual storage authority. No production identity can be compromised.

### P1: NONE

No active production feature depends on an obsolete Firebase service. The dead services (Crashlytics, Performance, Remote Config) are harmless residue — they add no functional behavior and create no runtime conflicts.

- Dead Crashlytics: `setUserIdentifier` is a no-op without error recording. Removing it has zero functional impact.
- Dead Performance: Never consumed. Removing it has zero functional impact.
- Dead Remote Config: Never consumed. `FeatureFlags` is hardcoded. Removing it has zero functional impact.

---

## 22. OWNER DECISIONS

These require owner/business input before cleanup:

| Decision | Context | Options |
|----------|---------|---------|
| **Analytics** | Currently active — tracks screen views + some events. Is this desired? | KEEP (current) / REMOVE (if not needed) |
| **Crashlytics** | Currently dead residue — zero error recording. Is crash reporting desired? | REMOVE (current) / RESTORE (if needed) |
| **FCM** | Confirmed canonical — no decision needed. | — |
| **Remote Config** | Currently dead — no A/B testing. Is this desired as a product feature? | REMOVE (current) / RESTORE (if needed) |

---

## 23. CLEANUP CANDIDATES

### Tier 1: Zero-Risk Cleanup (dead code, no consumers)

| Item | Type | Location | Evidence |
|------|------|----------|----------|
| `firestore_collections.dart` | Dead file | `core/constants/firestore_collections.dart` | Zero consumers outside itself |
| `FirebaseWilayahService` class name | Misnomer | `shared/services/firebase_wilayah_service.dart` | Active service, zero Firebase usage |
| `notification_service.dart` Firestore comments | Dead comments | `core/messaging/notification_service.dart` | Comments reference Firestore |
| `shared.dart` removal comments | Historical | `shared/shared.dart` | Documents removed services |

### Tier 2: Safe Dependency Removal (dependency present, zero runtime consumers)

| Package | In pubspec | Consumers | Risk |
|---------|-----------|-----------|------|
| `firebase_performance: ^0.11.0` | ✅ | Zero | NONE |
| `firebase_remote_config: ^6.0.2` | ✅ | Zero | NONE |

### Tier 3: Low-Risk Dependency Removal (minimal usage, value questionable)

| Package | In pubspec | Consumers | Risk |
|---------|-----------|-----------|------|
| `firebase_crashlytics: ^5.0.3` | ✅ | `setUserIdentifier` only | LOW — removing requires removing 3 lines from `auth_controller.dart` |

### Tier 4: File Cleanup After Tier 2-3

| File | Depends On | Notes |
|------|-----------|-------|
| `firebase_crashlytics_impl.dart` | `firebase_crashlytics` dep | After Crashlytics removal |
| `firebase_performance_impl.dart` | `firebase_performance` dep | After Performance removal |
| `firebase_remote_config_impl.dart` | `firebase_remote_config` dep | After Remote Config removal |
| `providers.dart` (observability section) | All three above | Remove dead providers |
| `core.dart` (observability exports) | All three above | Remove dead exports |
| `global_error_handler.dart` | `CrashReporter` interface | Never instantiated |

---

## 24. RECOMMENDED CLEANUP ORDER

```
Phase 1: Zero-risk dead code (no dependencies affected)
  ├── Delete firestore_collections.dart
  ├── Remove barrel export from core.dart
  ├── Clean dead Firestore comments from notification_service.dart
  └── Rename FirebaseWilayahService → WilayahService (misnomer fix)

Phase 2: Remove zero-consumer Firebase dependencies
  ├── Remove firebase_performance from pubspec.yaml
  ├── Remove firebase_remote_config from pubspec.yaml
  ├── Delete firebase_performance_impl.dart
  ├── Delete firebase_remote_config_impl.dart
  ├── Remove dead providers from providers.dart
  └── Remove dead exports from core.dart

Phase 3: Remove Crashlytics (questionable value)
  ├── [OWNER DECISION REQUIRED]
  ├── If approved: remove firebase_crashlytics from pubspec.yaml
  ├── Delete firebase_crashlytics_impl.dart
  ├── Remove crashlytics code from auth_controller.dart (3 lines)
  ├── Delete global_error_handler.dart (never instantiated)
  └── Remove dead providers from providers.dart

Phase 4: Optional — Analytics decision
  ├── [OWNER DECISION REQUIRED]
  └── If removed: remove firebase_analytics + related files
```

---

## 25. CRITICAL DISTINCTIONS

### PROVEN ACTIVE

- **Firebase Auth** — 20+ Dart files import `firebase_auth`, `AuthInterceptor` attaches token to every API request, Go backend verifies
- **Firebase Messaging** — 6 Dart files import `firebase_messaging`, token flow reaches Go backend via `POST /notifications/fcm/token`
- **Firebase Analytics** — 3 Dart files import `firebase_analytics`, `ScreenViewRouteObserver` mounted in `app_router.dart`
- **Firebase Core** — Required infrastructure for all above

### PROVEN DEAD

- **Firestore** — dependency removed, zero imports, `firestore_collections.dart` has zero consumers
- **Firebase Storage** — dependency removed, zero imports, all media goes through S3Service
- **Firebase Performance** — dependency present, zero consumers, observer never mounted
- **Firebase Remote Config** — dependency present, zero consumers, FeatureFlags hardcoded

### INFERRED

- **Crashlytics runtime initialization** — `FirebaseCrashlyticsImpl.instance()` creates `FirebaseCrashlytics.instance` lazily in `auth_controller.dart`, but this happens only on login. Whether the SDK auto-initializes without explicit `setCrashCollectionEnabled(true)` call is SDK-implementation-dependent.

### UNVERIFIED

- **iOS `IS_ANALYTICS_ENABLED = false`** in `GoogleService-Info.plist` — this legacy flag may or may not affect the FlutterFire analytics plugin. Empirical testing on iOS would be needed to confirm Analytics is truly active on iOS.

---

*End of Foundation-04 Audit Report*
*Generated: 2026-09-01*
*Method: Import tracing, runtime consumer verification, dependency analysis*
