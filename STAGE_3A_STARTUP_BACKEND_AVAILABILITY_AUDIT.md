# STAGE 3A — STARTUP / BACKEND AVAILABILITY AUDIT

## STAGE 3A — FINAL AUDIT REPORT

### VERDICT

**ROOT CAUSE PARTIALLY IDENTIFIED**

The behavior is fully mapped to a concrete, deterministic code path, and the
probable root cause (a hard-coded dev base URL `192.168.1.8:8080` that does not
match the device's network, combined with the startup sync request failing fast
and being classified as backend-unavailable) is strongly supported by static
evidence. It cannot be elevated to "ROOT CAUSE IDENTIFIED" because no runtime
reproduction was possible in this environment (no backend is listening; the
mobile app cannot be launched headlessly here), and therefore the "automatic
reload" step that transitions the degraded screen back to Home is not proven by
runtime evidence — it is inferred from the auto-retry path in code.

---

### 1. USER-OBSERVED SYMPTOM

Every cold Flutter launch:

1. App starts and the splash screen is shown.
2. The splash screen is replaced by a **degraded scaffold** titled
   **"Server Tidak Bisa Dijangkau"** ("Server Cannot Be Reached") with the
   message *"Tidak bisa terhubung ke server Labuda. Pastikan backend sedang
   berjalan dan HP berada di jaringan yang sama."* and buttons **Coba Lagi**
   (Retry) and **Keluar** (Sign out).
3. After some seconds (reported: "some time"), the app **automatically**
   recovers and lands on Home without user interaction.

The owner's suspicion is that this is a startup race / over-eager error
surface, not a genuine backend outage. The audit goal is to prove what actually
happens.

---

### 2. STARTUP PATH MAP

Concrete code path (all in `apps/mobile/lib`):

| Step | File | Class / function | What happens |
|---|---|---|---|
| 1 | `main.dart:67` | `main()` | `runZonedGuarded` → `WidgetsFlutterBinding.ensureInitialized()` |
| 2 | `main.dart:127` | `_initServices()` | `EnvConfig.init()`; `Firebase.initializeApp()` (failure tolerated, no rethrow); `LocalStorageService.initialize()`; constructs `ApiClient` (`main.dart:157`); `FirebaseAuthenticationService`; `AppRouter()` + `initializeRouterModules()` (`main.dart:200-203`); notification/analytics/WS/presence stack |
| 3 | `main.dart:79` | `runApp(ProviderScope(...))` | All services injected via overrides; `LabudaApp()` built |
| 4 | `app.dart:26` | `LabudaApp.build` | `ref.watch(goRouterProvider)` — router created, watches auth state |
| 5 | `core/src/router/app_router.dart:62-120` | `goRouterProvider` | `GoRouter(initialLocation: '/splash', redirect: ...)` |
| 6 | `auth_controller.dart:342` | `AuthController.build()` | First read of `authControllerProvider` (from router watch) triggers `_initializeAuthState()` (fire-and-forget, line 396), returns `AuthState.initial()` |
| 7 | `auth_controller.dart:446-533` | `_initializeAuthState()` | Subscribes to `FirebaseAuth.instance.authStateChanges()`; schedules a 15s startup failsafe (line 454) that forces `AuthStateUnauthenticated` if still `Initial` |
| 8 | `auth_controller.dart:465-524` | Firebase listener | Emits cached user (Firebase persists session locally) → `user.reload().timeout(5s)` → `AuthStateFirebaseAuthenticated` → `_syncWithBackend(...)` |
| 9 | `auth_controller.dart:553-945` | `_syncWithBackend()` | Publishes `AuthStateSyncingWithBackend` → calls `UserSyncService.syncUser(...)` with `.timeout(15s)` (line 662) |
| 10 | `user_sync_service.dart:76` | `syncUser()` | `firebaseUser.getIdToken()` → `UserApiDatasource.exchangeFirebaseSession()` → `POST /auth/firebase/exchange` |
| 11 | `api_client.dart:32-43` | `ApiClient._createDio()` | Base URL from `ApiConfig` (dev: `http://192.168.1.8:8080/api/v1`), connect/send/receive timeouts 10s each, `validateStatus: status < 500` |
| 12 | `error_interceptor.dart:20-91` | `ErrorInterceptor.onError` | Transport failure → `NetworkException('Cannot reach Labuda server...', code: 'BACKEND_UNREACHABLE')`; 5xx → `ApiExceptionFactory.fromStatusCode(...)` (message-only statusCode 500/502/503) |
| 13 | `base_api_repository.dart:176-201` | `executeRequest` DioException branch | `Result.error(exception.message, code: exception.code)` — **note: `statusCode` is dropped on this branch** |
| 14 | `auth_controller.dart:669-791` | sync error branch | `classifyAuthSyncError(error, errorCode, statusCode)` → `AuthSyncErrorKind.backendUnavailable` → `_publishIfCurrent(..., AuthState.backendUnavailable(error))` (line 763) |
| 15 | `auth_controller.dart:766-783` | auto-retry | `if (_retryCount < _maxRetries)` → `_retryCount++`, `Timer(2s, 4s, 6s)` → re-runs `_syncWithBackend(...)` |
| 16 | `app_router.dart:227-236` | degraded redirect | `AppAuthStatus.degraded` → **no redirect** — user stays parked on `/splash` |
| 17 | `splash_screen.dart:170-173, 249-347` | `SplashScreen.build` | `AuthStateBackendUnavailable`/`BackendFailure` → `_buildDegradedScaffold` → **"Server Tidak Bisa Dijangkau"** UI with Coba Lagi / Keluar |
| 18 | `splash_screen.dart:311-315` | Coba Lagi | `authControllerProvider.notifier.retryBackendSync()` (manual retry) |
| 19 | `auth_controller.dart:1544-1565` | `retryBackendSync()` | Cancels timer, resets `_retryCount = 0`, `_syncedUserId = null`, re-runs `_syncWithBackend` |
| 20 | `auth_controller.dart:805-879` | successful sync | `_syncedUserId = userId`, `_retryCount = 0`, `_publishAuthenticatedIfCurrent(...)` → `AuthStateAuthenticated` |
| 21 | `app_router.dart:219-225` | authenticated redirect | `AppAuthStatus.authenticated` → if on `/splash`/`/welcome`/`/auth` → **`/home`** |

Transition to Home is therefore a **provider-driven router redirect** driven by
the auth state, not a navigation call in splash.

---

### 3. SERVER AVAILABILITY AUTHORITY

There is **exactly one canonical startup availability authority**:

- **`AuthController._syncWithBackend()`** (`auth_controller.dart:553`) — the
  only place that decides `AuthStateBackendUnavailable` vs `AuthStateAuthenticated`
  at startup, and the only place that schedules degraded-mode auto-retries.

Supporting pieces (single chain, no competitors):

- **Classification:** `classifyAuthSyncError()` / `isBackendUnavailableErrorMessage()`
  (`auth_controller.dart:71-136`) — sole classifier, used by every auth-sync call site.
- **UI authority:** `SplashScreen._buildDegradedScaffold()` (`splash_screen.dart:249`) —
  the only producer of "Server Tidak Bisa Dijangkau".
- **Router authority:** `_authRedirectForLocation()` (`app_router.dart:171`) — sole
  redirect decision, `degraded` → no redirect.

Negative findings (no competing authorities found):

- No `ConnectivityService`, no `connectivity_plus` usage, no `checkConnectivity`
  call anywhere in `apps/mobile/lib`.
- No health-check call at startup: `AuthInterceptor._isPublicEndpoint` lists
  `/health` as a public endpoint (`auth_interceptor.dart:382`), but no startup
  code calls it. The only backend requests at startup are the auth exchange
  (`POST /auth/firebase/exchange`) and `GET /users/me`.
- `ApiClient.isNetworkError()` and `ApiConfig.maxRetryAttempts`/`retryDelay`/`retryableStatusCodes`
  (`api_config.dart:108-115`) are **dead configuration** — no `RetryInterceptor`
  is registered in `_createDio()` (`api_client.dart:46-61`), so no HTTP-layer retry exists.
- Onboarding navigation repository (`onboarding_navigation_repository_impl.dart`)
  only forwards to `AppRouter.navigateTo*`; it is not used by splash for startup navigation.

**Conclusion: one canonical startup availability decision — no competing checks.**

---

### 4. ERROR STATE AUTHORITY

- **Where:** `splash_screen.dart:170-173` (build) → `_buildDegradedScaffold`
  (`splash_screen.dart:249`).
- **Triggering state:** `AuthStateBackendUnavailable` (title "Server Tidak Bisa
  Dijangkau") or `AuthStateBackendFailure` (title "Gagal Memuat Data").
- **Producing event:** a failed `POST /auth/firebase/exchange` (or `GET /users/me`)
  inside `_syncWithBackend()` classified as `backendUnavailable` by
  `classifyAuthSyncError()` (5xx via `statusCode`, or free-text match on
  timeout/connection/network/socket/etc.) → `_publishIfCurrent(..., AuthState.backendUnavailable(...))`
  (`auth_controller.dart:763` — Result-error path) or `auth_controller.dart:929`
  (exception path).
- **Why the screen stays:** `appAuthStatus` maps degraded states to
  `AppAuthStatus.degraded` (`auth_controller.dart:317-320`), and the router
  performs **no redirect** for degraded (`app_router.dart:227-236`), so the user
  remains on `/splash`, where SplashScreen renders the degraded scaffold.

---

### 5. RUNTIME EVIDENCE

**Not reproducible in this environment — stated explicitly.**

Probes run at audit time (2026-08-25):

- `Test-NetConnection 192.168.1.8:8080` → **TcpTestSucceeded: False**
- `Test-NetConnection localhost:8080` → False
- `Test-NetConnection localhost:8081` → False
- Listening ports include PostgreSQL `5432` and Redis `6379`, but **no Go
  backend on 8080/8081**.

So the exact environment that produces the owner's cold-start flash cannot be
reproduced headlessly here (mobile app cannot be launched in this environment
either). **No runtime evidence was captured. All conclusions below are static,
code-derived evidence.**

Supporting static evidence for the sync-failure chain:

- `base_api_repository.dart:176-201` (DioException branch): builds
  `Result.error(exception.message, code: exception.code)` **without
  `statusCode`** — so for a 5xx or transport failure the classifier falls back
  to free-text matching; the `NetworkException` message ("Cannot reach Labuda
  server...") and the 10s timeout message ("Connection timed out...") both
  contain substrings that `isBackendUnavailableErrorMessage` matches
  (`timeout`/`connection`/`network`) → reliably classified `backendUnavailable`.
- `user_sync_service.dart:118-133` forwards `errorCode`/`statusCode`/`details`
  from the datasource Result, but those are null on the DioException branch.

---

### 6. RETRY / RELOAD ANALYSIS

The ambiguous "automatic reload" decomposes into three distinct mechanisms —
**only one of them (A) actually exists; (B) is the observed "reload"; (C) is
explicitly excluded**:

**(A) Auto-retry of the backend sync (exists)**
- Initiator: `AuthController._syncWithBackend()` — degraded auto-retry branch
  (`auth_controller.dart:766-783`).
- Trigger: a sync failure classified `backendUnavailable` with
  `_retryCount < _maxRetries` (3).
- Schedule: `Timer(delay = 2s * _retryCount)` → 2s, 4s, 6s after each failure.
- Max retries: 3 (`_maxRetries = 3`, line 248).
- Guard: fires only if `activeFirebaseUser != null` (line 775).
- On retry: re-runs `_syncWithBackend` (full exchange again — a brand-new HTTP
  request), guarded by `_syncInProgress`/`_ongoingSync` mutex.
- State during retries: stays `AuthStateBackendUnavailable` (screen keeps showing
  degraded UI).
- On success: `_publishAuthenticatedIfCurrent` → router redirect to `/home`.
- On 3rd failure: `_retryCount == _maxRetries` → logs "manual retry required",
  stays degraded; **only the user tapping Coba Lagi** (→ `retryBackendSync()`)
  or Keluar can exit.

**This is the technically correct explanation of "the app automatically reloads
and navigates to Home":** an HTTP-level retry inside the auth controller that
eventually succeeds, then triggers the router redirect to `/home`. There is no
app restart, no widget-tree rebuild of the whole app, and no Flutter hot reload
involved.

**(B) Router redirect / navigation (exists, but is not "reload")**
- `_authRedirectForLocation` re-evaluated on every auth-state change because
  `goRouterProvider` `ref.watch(authControllerProvider)` (`app_router.dart:64`);
  `authenticated` → `/home` (`app_router.dart:219-225`).

**(C) HTTP request retry (does not exist)**
- `ApiConfig.maxRetryAttempts`/`retryDelay`/`retryableStatusCodes`
  (`api_config.dart:108-115`) are unused; no retry interceptor is registered
  (`api_client.dart:46-61`). The only HTTP-layer retry is the 401-refresh-retry
  in `AuthInterceptor.onError` (`auth_interceptor.dart:236-344`), which is not a
  connectivity retry and does not apply to the exchange (it carries
  `skipAuth: true`; a 401 there returns a typed `UnauthorizedException`).

Potential retry hazards found (worth noting, none currently active):

- `retryBackendSync()` (manual) always re-runs `_syncWithBackend` even if an
  auto-retry timer is pending (timer is cancelled first, line 1552) — safe.
- The degraded auto-retry Timer is cancelled in `signOut()` (line 1396) and in
  `retryBackendSync()` — but not in `reset()` (line 2040) — minor; `reset()` is
  test-only.
- `retryBackendSync()` calls `_syncWithBackend(...)` without awaiting it
  (line 1564, no `await`), and `_syncWithBackend` re-entrancy is guarded by the
  mutex — overlapping retries resolve by waiting then skipping.

---

### 7. TIMEOUT ANALYSIS

| Timeout | Value | Location |
|---|---|---|
| Dio connect timeout | 10s | `api_config.dart:98` → `api_client.dart:36` |
| Dio receive timeout | 10s | `api_config.dart:101` → `api_client.dart:37` |
| Dio send timeout | 10s | `api_config.dart:104` → `api_client.dart:38` |
| Firebase `user.reload()` | 5s | `auth_controller.dart:479` (`.timeout(Duration(seconds: 5))`) |
| **Sync wrapper (dominant)** | **15s** | `auth_controller.dart:662-667` (`.timeout(const Duration(seconds: 15), onTimeout: () => throw Exception('SYNC TIMEOUT'))`) |
| Startup failsafe | 15s | `auth_controller.dart:454` (force unauthenticated if still `Initial`) |
| `getCurrentUser()` (post-sync) | 15s | `auth_controller.dart:1826`, `auth_controller.dart:1947` |
| Session validation periodic | 5 min | `auth_controller.dart:953` |
| Degraded auto-retry delay | 2s → 4s → 6s | `auth_controller.dart:769` (`2 * _retryCount` seconds) |
| WS disconnect on logout | 3s | `auth_controller.dart:1472` |

Relevant mechanics:

- `user.reload()` has a **5s** timeout; the sync has a **15s** wrapper. A backend
  cold start exceeding either would surface as `backendUnavailable`.
- The Dio timeouts (10s each) only bite on a truly unresponsive server; a
  connection-refused / unreachable host (the common dev case) fails
  immediately with `DioExceptionType.connectionError` → `NetworkException`
  (`error_interceptor.dart:57-69`), and the exchange request completes in
  milliseconds.
- `validateStatus: status < 500` (`api_client.dart:41`): a 5xx response is
  treated as an error (good), but a 4xx is treated as a valid response and
  parsed in the success path of `executeRequest`, where the envelope
  `success:false` → `Result.error(message, code, statusCode)` **with**
  statusCode — so 4xx classification (identity/backendFailure) is structured.

**Time budget for the observed degraded flash:** with the backend reachable and
slow, the sync can take up to 15s (wrapper) before `backendUnavailable`; with
the backend unreachable (dev LAN IP mismatch), the failure is near-instant and
the degraded screen appears immediately after the splash animation (~1.2s), then
auto-retries at 2s/4s/6s until success or the 3-retry cap.

---

### 8. INITIALIZATION ORDER

No backend call happens before required initialization completes:

- All service construction (`Firebase.initializeApp`, LocalStorage, ApiClient,
  router modules, FCM, analytics, WS/presence) happens in `_initServices()`
  **before** `runApp()` (`main.dart:127-224`).
- The first backend requests (`POST /auth/firebase/exchange`, `GET /users/me`)
  are only issued by `_syncWithBackend()`, which is only reachable after the
  Firebase auth listener fires (i.e., after Firebase is initialized and a
  session is restored from Firebase's platform storage), and after
  `AuthController.build()` has read `authRepositoryProvider` /
  `userSyncServiceProvider` / `localStorageServiceProvider` from the overriding
  ProviderScope.
- `ApiClient` is constructed with the base URL resolved synchronously from
  `ApiConfig` (`api_client.dart:35, 67-69`) — no async config loading exists to
  race.

One environment-resolution concern (not an ordering bug): the dev base URL is
**hard-coded** to `http://192.168.1.8:8080/api/v1` (`api_config.dart:41`). It is
not derived from the device, does not match the Android emulator convention
(`10.0.2.2`), and the `isIOS` variant is identical. If the backend host
currently listening is on another LAN IP, every startup exchange fails
immediately with connectionError → `AuthStateBackendUnavailable` → degraded
screen → auto-retries → (in the owner's observed case) eventual success when the
backend comes up or the device re-resolves the network.

---

### 9. ROUTER ANALYSIS

- **Startup route:** `GoRouter(initialLocation: RoutePaths.splash)` —
  `app_router.dart:85`.
- **While initializing** (`AppAuthStatus.initializing`): redirect to `/splash`
  (`app_router.dart:194-195`).
- **While syncing** (`isSyncingWithBackendProvider` true): all non-`/splash`
  routes blocked to `/splash` (`app_router.dart:99-101`) — prevents deep-link
  bypass during sync.
- **Degraded** (`AppAuthStatus.degraded`): **no redirect** (`app_router.dart:227-236`);
  SplashScreen renders the degraded scaffold when parked on `/splash`.
- **Authenticated:** redirect `/splash`/`/welcome`/`/auth/*` → `/home`
  (`app_router.dart:219-225`).
- **Unauthenticated:** → `/welcome` (with public-browse passthrough
  `app_router.dart:197-212`).
- **Profile completion / account restricted:** dedicated routes
  (`app_router.dart:184-191, 214-217`).
- The server-unreachable page is **not a route** — it is an inline degraded
  branch of `SplashScreen.build` (`splash_screen.dart:170-173`).
- Race analysis: router redirects are re-evaluated synchronously on every
  `authControllerProvider` emission (watch → rebuild → redirect). The
  `syncing`-block guard and the `degraded`-no-redirect rule prevent redirects
  while bootstrap is incomplete, so no router decision can race ahead of the
  auth state machine.

---

### 10. ROOT CAUSE

**Probable root cause (static evidence, not runtime-proven):**

The "Server Tidak Bisa Dijangkau" flash is a **truthful rendering of a genuine
failure of the first backend request** at cold start, not a render-before-init
race and not an over-eager intermediate state:

1. The backend sync (`POST /auth/firebase/exchange` → `GET /users/me`) is the
   single startup availability probe, and it fails at cold start because the
   configured base URL (`192.168.1.8:8080`, hard-coded for dev) does not match
   the backend host the device can actually reach at that moment (backend
   starting up, or LAN IP mismatch / emulator host mapping). The failure is
   near-instant (`connectionError`) or slow (10s/15s timeouts).
2. The failure is correctly classified as `backendUnavailable` and rendered as
   the degraded screen — by design ("PASS 2B"), a deliberate improvement over
   the old silent bounce to `/welcome`.
3. The automatic recovery is **not** an app reload: it is the auth controller's
   deterministic degraded-mode auto-retry (2s/4s/6s, max 3). When a retry
   succeeds, `AuthStateAuthenticated` is published and the router redirects to
   `/home`.

So the app is behaving **as designed** for the observed input. The UX problem is
that a **transient, expected startup condition** (backend still booting / dev
host mismatch) is surfaced as a terminal-looking full-screen error with a
manual-retry button, while the state machine is simultaneously auto-retrying in
the background — the auto-retry and the manual-retry UI are two overlapping
recovery mechanisms, and the degraded UI hides the fact that recovery is already
in flight.

**Uncertainty:** (a) no runtime capture of the retry timing/transition in the
owner's environment; (b) whether the owner's "wait" window matches 2s/4s/6s
retries plus one successful sync (≈ 6–21s) or something longer; (c) whether the
eventual success is always the auto-retry (in which case the Coba Lagi button is
redundant during the first 3 failures) — all follow from code but were not
observed.

---

### 11. REQUIRED BUSINESS / UX DECISIONS

The following are genuinely business/UX decisions (cannot be derived from code):

1. **Startup wait policy:** how long may the app keep the user on splash (with
   auto-retries) before showing a terminal failure? Current behavior: ~3 retries
   ≈ up to ~51s worst-case (15s × 4 attempts + 2+4+6s delays) if the backend is
   reachable-but-slow; near-instant if unreachable. Decide a bounded total
   (e.g., 30s/45s) and whether the degraded screen may appear before retries are
   exhausted.
2. **Auto-retry acceptability:** is silent background auto-retry during splash
   acceptable, and should the degraded UI appear at all while retries are still
   scheduled (i.e., show "menghubungkan kembali…" instead of the error during
   the 2s/4s/6s window)?
3. **Manual retry vs auto-retry overlap:** should Coba Lagi be hidden/disabled
   while an auto-retry timer is pending, or always active (current)?
4. **Max retry duration / offline mode:** is there an acceptable offline/dev
   mode (e.g., cached last session, or a "dev backend not running" hint), or is
   the truthful failure screen the required terminal state?
5. **Dev environment UX:** the dev base URL is hard-coded to a LAN IP
   (`192.168.1.8:8080`); decide whether dev should auto-resolve the emulator
   host (`10.0.2.2` for Android) or make the base URL configurable at build
   time.

---

### 12. RECOMMENDED IMPLEMENTATION PLAN

Smallest safe sequence (NOT implemented in this stage):

- **Stage 3B-A — Prove runtime behavior:** instrument only logging (no behavior
  change): log every `_syncWithBackend` attempt with timestamps, retry count,
  and error classification; log the router redirect transitions. Capture one
  cold start in the owner's environment to confirm the observed "reload" is the
  2s/4s/6s auto-retry succeeding.
- **Stage 3B-B — Visibility fix (if decision #1/#2 approve):** while an
  auto-retry is scheduled, SplashScreen shows a non-terminal "menghubungkan
  kembali…" progress state instead of the full degraded error (i.e., only render
  the degraded scaffold after `_retryCount >= _maxRetries` or when the user is
  parked on a non-splash route). Keep the truthful degraded scaffold for the
  exhausted-retries case.
- **Stage 3B-C — Retry-state hygiene:** expose `retryPending`/`retryCount` from
  AuthController; disable/hide Coba Lagi while a retry timer is pending; ensure
  `reset()` cancels `_retryTimer`; keep `signOut()` cancellation as-is.
- **Stage 3B-D — Timeout calibration (only if evidence supports):** align the
  sync wrapper timeout (15s) with Dio timeouts (10s) deliberately, or keep 15s
  and document the budget; consider making the startup failsafe (15s) drive a
  single degraded state rather than `unauthenticated` when a Firebase session
  exists but the backend never answers.
- **Stage 3B-E — Dev base URL (dev-only):** make dev base URL configurable
  (build-time/dotenv) or auto-detect Android emulator host; explicit decision
  #5.

Each stage ends with runtime proof. No cleanup is bundled into any stage.

---

### 13. CLEANUP CANDIDATES

Discovered, NOT removed (startup/backend-availability scope only):

- `ApiConfig.maxRetryAttempts` / `retryDelay` / `retryableStatusCodes`
  (`api_config.dart:108-115`): dead configuration — no HTTP retry interceptor
  exists. Either wire it or delete it.
- `ApiClient.isNetworkError()` (`api_client.dart:217-220`): appears unused by
  the startup path.
- `base_api_repository.dart:176-201`: `Result.error(...)` on the DioException
  branch drops `statusCode` (and `details` shape differs from other branches),
  weakening structured classification for 5xx/transport failures; align with
  the success-path branches.
- Splash `debugPrint('[SPLASH] build: ...')` diagnostic log
  (`splash_screen.dart:158-161`) — self-marked "remove after splash-loop is
  confirmed fixed".
- `RouterErrorPage` (`router_error_page.dart:81, 107`) offers `context.go('/splash')`
  — unrelated to the degraded state but worth noting as a second "reload" path
  users could encounter on generic router errors.

Nothing was removed or modified.

---

### 14. OUT OF SCOPE

Confirmed no changes were made to:

- Commerce Product / FPS / Auction / quantity_available / product discovery /
  seller-scoped listing browse / inventory / naming convergence (no Commerce
  files touched; `backend/internal/commerce/forsale/` etc. left as-is).
- Username convergence (Stage 1D) and password convergence (Stage 2B/2C/2D)
  systems — read-only references only.
- No production code, no tests, no configuration were modified. The only file
  created is this report (`STAGE_3A_STARTUP_BACKEND_AVAILABILITY_AUDIT.md`).

---

### 15. WORKING TREE

Baseline recorded via `git status --short` + `git diff --stat` before the audit:
the repository was already heavily dirty from previous sessions (hundreds of
modified files across `apps/mobile`, `backend`, `docs`, migrations, plus many
untracked report files). Per the stage doctrine, this is the pre-existing
baseline and was not touched, restored, or cleaned.

No unrelated files were changed during this stage. Only the deliverable report
was written.

---

### 16. STOP

Stage 3A audit complete.
No implementation performed.
Awaiting owner/ChatGPT decision before Stage 3B.
