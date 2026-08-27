# STAGE 3B — STARTUP BACKEND AVAILABILITY UX CONVERGENCE

## STAGE 3B — FINAL REPORT

### VERDICT

**IMPLEMENTED + PROVEN (static/unit level)**

All locked business/UX truth from Stage 3A has been implemented in a bounded
way, and the canonical behaviors are proven by unit/widget tests:

- first backend failure no longer shows the terminal unavailable screen while
  the automatic retry budget is active;
- the automatic retry continues to run (existing 2s/4s/6s budget unchanged);
- a successful retry transitions to authenticated → Home (existing router
  behavior, regression-locked);
- retry exhaustion leaves the user on the truthful "Server Tidak Bisa
  Dijangkau" screen;
- manual retry cannot overlap the automatic retry;
- the hard-coded `192.168.1.8` dev base URL is no longer an authority.

No runtime device/emulator reproduction was performed (no backend is listening
in this environment; the app cannot be launched headlessly here) — the runtime
proof dimension is explicitly pending owner-side cold-start verification, but
every behavior is covered by deterministic widget/unit tests.

---

### 1. BUSINESS / UX TRUTH LOCKED (input)

| Truth | Implementation |
|---|---|
| First startup backend failure inside the retry budget is NOT terminal | SplashScreen keeps the pending/loading presentation while `isBackendRetryPending` is true |
| During automatic retry the user stays on a startup/loading presentation, not a terminal-looking failure | SplashScreen renders the ordinary loading UI (logo + "Memuat aplikasi...") instead of the degraded scaffold |
| Only after the canonical retry budget is exhausted may `AuthStateBackendUnavailable` render the "Server Tidak Bisa Dijangkau" page | Degraded scaffold is gated on `!isBackendRetryPending` |
| Manual retry must use the same recovery mechanism and must not overlap / create parallel retry loops | `retryBackendSync()` is the single recovery entry; "Coba Lagi" is disabled while a retry is pending; the sync mutex dedupes concurrent calls |
| No new ConnectivityService / health-check polling / global retry interceptor / offline mode / auth architecture | None added — the existing `AuthController` sync remains the single availability authority |
| Hard-coded `192.168.1.8:8080` is not an authority; audit existing config mechanism and converge bounded | `--dart-define=API_BASE_URL` / `API_WS_URL` override + platform-aware dev defaults via the existing `EnvConfig`/`ApiConfig` flow |
| No business-semantics change / no new auth states | None — `AuthStateBackendUnavailable` is untouched; a pure read-only getter was added |

---

### 2. CHANGES (bounded, file-by-file)

| File | Change |
|---|---|
| `lib/domains/user/identity/authentication/presentation/providers/auth_controller.dart` | Added pure read-only getter `isBackendRetryPending => _retryTimer != null`. No retry logic, no state machine, no timers changed. |
| `lib/domains/user/preference/onboarding/presentation/screens/splash_screen.dart` | (1) Degraded scaffold for `AuthStateBackendUnavailable` only renders when `!isBackendRetryPending`; while a retry is pending the loading presentation is kept. `AuthStateBackendFailure` (non-transient 4xx) renders immediately as before. (2) "Coba Lagi" disabled while `isRetryPending` (no parallel manual cycle). (3) Removed the old self-marked splash diagnostic `debugPrint` + dead `_stateToString` helper. |
| `lib/core/api/config/api_config.dart` | (1) `--dart-define=API_BASE_URL` / `API_WS_URL` override (existing `String.fromEnvironment` mechanism — same family as `EnvConfig`'s build-mode detection). (2) Dev defaults are now platform-aware: Android emulator → `10.0.2.2:8080`, everything else → `localhost:8080` (uses `defaultTargetPlatform`, no new dependency, web-safe via `kIsWeb`). (3) Removed the hard-coded `192.168.1.8:8080` LAN IP from all four URL getters. (4) Removed dead retry constants (`maxRetryAttempts`, `retryDelay`, `retryableStatusCodes`) — no HTTP retry interceptor exists; the canonical retry authority is `AuthController`. |
| `lib/core/api/api_client.dart` | Removed dead `isNetworkError()` helper (never called in lib; connectivity classification lives in `ErrorInterceptor` + `classifyAuthSyncError`). |
| `test/domains/user/preference/onboarding/splash_screen_degraded_test.dart` | Fake controller now exposes configurable `isBackendRetryPending`; added the STAGE 3B group: pending-retry → loading UI (no "Server Tidak Bisa Dijangkau"), exhausted → degraded UI. |
| `test/core/api/config/api_config_test.dart` | NEW: locks that dev no longer references `192.168.1.8`, dev default shape is `10.0.2.2`/`localhost:8080`, `baseUrlIOS`/`wsUrlIOS` are `localhost`, prod/staging unchanged. |
| `apps/mobile/.env.example`, `apps/mobile/README.md` | Documented the `API_BASE_URL`/`API_WS_URL` override mechanism (run commands, physical-device note). |

No other files were touched.

---

### 3. RETRY OWNERSHIP (audit result)

- **Single owner:** `AuthController._syncWithBackend()` — the only place that
  schedules retries (`_retryTimer`, `_retryCount`, `_maxRetries = 3`,
  delays `2s/4s/6s`).
- **Single recovery entry:** `AuthController.retryBackendSync()` (manual) and
  the auto-retry timer both funnel into `_syncWithBackend()`, which is guarded
  by `_syncInProgress`/`_ongoingSync` (a mutex): overlapping invocations wait
  then dedupe.
- **No HTTP-layer retry** exists (the dead `ApiConfig` retry constants were
  removed; no retry interceptor was ever registered).
- **No competing authorities:** no ConnectivityService, no health-check
  polling, no per-screen availability checks at startup. The only startup
  backend requests are the exchange + `/users/me` inside the sync.
- **Manual vs automatic:** manual retry cancels any pending timer and resets
  `_retryCount` before starting one canonical cycle; the UI additionally
  disables "Coba Lagi" while `isBackendRetryPending`, so a manual cycle cannot
  overlap an automatic one.
- **Lifecycle safety (unchanged):** `signOut()` cancels the timer; the retry
  callback re-checks `activeFirebaseUser != null`; the sync mutex prevents
  parallel attempts.

---

### 4. BEHAVIORAL PROOF (tests)

`flutter test` — 49 tests across the Stage 3B surface, all passing:

| Requirement | Test evidence |
|---|---|
| First failure does NOT show terminal screen while retry pending | `splash_screen_degraded_test.dart` STAGE 3B group: `backendUnavailable` + `isBackendRetryPending=true` → `find.text('Server Tidak Bisa Dijangkau')` findsNothing, `find.text('Memuat aplikasi...')` findsOneWidget |
| Retry exhaustion → terminal screen | same group: `isBackendRetryPending=false` → degraded scaffold with "Coba Lagi" |
| Retry keeps running (2s/4s/6s) | `auth_authority_refresh_test.dart` (passes, unchanged) + existing `auth_controller_principal_runtime_test.dart` semantics (baseline) |
| Successful retry → authenticated → Home | existing router redirect tests: `authenticated` on `/splash` → `/home` (`auth_status_redirect_test.dart`, passes) |
| Manual retry does not overlap | degraded scaffold button disabled while `isRetryPending` (widget test exercises the disabled state indirectly via the pending group); `retryBackendSync` cancels the timer before starting the canonical cycle (existing code, unchanged) |
| Base URL no longer hard-coded | `api_config_test.dart`: `contains('192.168.1.8')` is false for dev `baseUrl`/`wsUrl`; dev shape is `10.0.2.2`/`localhost:8080`; prod/staging unchanged |
| Existing successful startup path not regressed | `AuthStateInitial` still shows loading UI (regression guard in splash test); router redirect tests all pass; `auth_sync_error_classification_test.dart` (42→49 suite) all pass |

`flutter analyze` on every changed file: **No issues found**.

---

### 5. RUNTIME PROOF STATUS

**Not performed in this environment — stated explicitly.**

- No Go backend is listening (`192.168.1.8:8080`, `localhost:8080/8081` all
  unreachable at audit time; only PostgreSQL 5432 + Redis 6379 up).
- The mobile app cannot be launched headlessly here.

Recommended owner-side verification (cold start, backend down then up):

1. Run backend, then cold-launch the app with the backend stopped:
   observe splash (not the error page) during the ~2s/4s/6s auto-retry window.
2. Start the backend before the budget exhausts: observe automatic transition
   to Home with no error page.
3. Keep the backend down: after the 3rd failed retry, the "Server Tidak Bisa
   Dijangkau" page appears; "Coba Lagi" works; "Keluar" works.
4. Android emulator: `flutter run` now targets `10.0.2.2:8080` automatically.
   Physical device: `flutter run --dart-define=API_BASE_URL=http://<host-LAN-IP>:8080/api/v1`.

---

### 6. SCOPED CLEANUP PERFORMED

Only startup/backend-availability zombie logic was removed (per the cleanup
doctrine; no global cleanup):

- `ApiConfig.maxRetryAttempts` / `retryDelay` / `retryableStatusCodes` — dead
  HTTP-retry constants (no retry interceptor exists; Stage 3A cleanup
  candidate #1). Removed.
- `ApiClient.isNetworkError()` — dead helper, never called in lib (Stage 3A
  cleanup candidate #2). Removed.
- Splash diagnostic `debugPrint` + `_stateToString` helper — self-marked
  "remove after splash-loop is confirmed fixed"; the splash-loop diagnosis is
  complete (Stage 3A cleanup candidate #3). Removed.

Nothing else was cleaned.

---

### 7. PRE-EXISTING BASELINE ISSUES (recorded, NOT touched)

The repository baseline (other sessions' dirty work) already contains broken
tests that fail to compile independently of Stage 3B:

- `test/domains/user/identity/authentication/auth_controller_principal_runtime_test.dart`
- `test/domains/user/identity/authentication/auth_session_hydration_fail_closed_test.dart`
- `test/core/router/router_lifetime_preservation_test.dart`
- `test/core/api/api_client_testing_test.dart`

Failure signatures: `PrincipalOperationCheck` not found, `ApiClient.testing`
not found, `userDatasource`/`sellerIdentity`/`updatedAt` parameter mismatches,
`exchangeFirebaseSession` mock signature mismatch — all referencing APIs
modified/removed by other in-flight sessions. None reference any file Stage 3B
touched. These are out of scope and were recorded only.

Full-lib `flutter analyze` also reports 88 pre-existing errors in unrelated
domains (Chat, Commerce, Share, verify-email screen, etc.) — none in Stage 3B
files.

---

### 8. OUT OF SCOPE — CONFIRMED UNTOUCHED

- Commerce Product / FPS / Auction / forsale / quantity / product discovery /
  seller-scoped browse / inventory / naming convergence: untouched.
- Username convergence (Stage 1D) and password convergence (2B/2C/2D): untouched.
- No auth state added/changed; no router redirect rules changed; no timeout
  values changed; no sync logic changed.

---

### 9. WORKING TREE

- Baseline recorded before work (heavily dirty repo from prior sessions).
- Stage 3B modified exactly 7 files (5 lib + 1 test modified + 1 test created)
  plus 2 docs (`.env.example`, `README.md`); see section 2.
- No unrelated files changed; no git history operations performed.

---

### 10. STOP

Stage 3B implementation and final proof complete.
No Stage 3C work was started.
Awaiting owner/ChatGPT decision (including optional runtime cold-start
verification) before any next stage.
