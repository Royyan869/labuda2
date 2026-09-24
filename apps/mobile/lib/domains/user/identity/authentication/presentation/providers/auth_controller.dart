import 'dart:async';
import 'dart:convert';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:labuda/core/core.dart';
import '../../domain/entities/account_status.dart';
import '../../domain/entities/firebase_principal.dart';
// R4.3: Import data layer providers instead of constructing inline
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data
    show authRepositoryProvider, authApiDatasourceProvider;
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show userSyncServiceProvider;
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart'
    show fcmServiceProvider;

/// PASS 2A / F1 — structured classification of a failed backend auth-sync
/// call (POST /api/v1/auth/firebase/exchange, GET /users/me).
///
/// Free-text message matching drifts silently whenever the backend's
/// wording changes and was already wrong on day one: the backend's actual
/// messages ("Invalid or expired Firebase token", "Account has been
/// deleted") never matched the Firebase-SDK-shaped substrings the old
/// matcher looked for ('invalid token', 'user deleted'), so both cases fell
/// through to a generic, indefinitely-retryable [AuthSyncErrorKind.backendFailure].
///
/// The backend's machine-readable `errorCode`/`statusCode` (see
/// backend/internal/identity/auth/delivery/http/auth_handler.go) are the
/// PRIMARY signal here and take priority whenever present.
enum AuthSyncErrorKind {
  /// Firebase ID token itself is invalid/expired (backend `INVALID_TOKEN`,
  /// HTTP 401). The client-side Firebase session can no longer be trusted
  /// — force signOut.
  identityInvalid,

  /// The account no longer exists (backend `ACCOUNT_DELETED`, HTTP 403).
  /// Force signOut — there is nothing left to sync against.
  accountDeleted,

  /// The account exists but is suspended/banned/inactive (backend
  /// `ACCOUNT_INACTIVE`, HTTP 403). This is NOT a transient backend outage
  /// and must not be auto-retried; it also does not force a Firebase
  /// signOut (mirrors the mid-session [AuthStateAccountRestricted] gate
  /// elsewhere, which likewise never signs the user out).
  accountInactive,

  /// D2 HARD GATE: backend rejected the exchange because the presented
  /// Firebase identity's email is not verified (`EMAIL_NOT_VERIFIED`,
  /// HTTP 403). This is a business-flow decision with its own UI (the
  /// verify-email screen) — NOT a degraded backend state (INV-7: splash
  /// degraded is reserved for infrastructure failures only).
  pendingEmailVerification,

  /// D4: backend rejected the exchange because the email's account row is
  /// already bound to a DIFFERENT Firebase UID (`IDENTITY_CONFLICT`,
  /// HTTP 409). Two Firebase identities claiming one account is a canonical
  /// anomaly — terminal for this session; forces a clean sign-out. Never
  /// auto-retried and never silently re-bound (no fallback exists).
  identityConflict,

  /// Transient network/server issue (timeout, 5xx, no connection) — terminal
  /// degraded state. Recovery is explicit via manual retryBackendSync().
  backendUnavailable,

  /// Backend rejected the request for another reason (validation, business
  /// rule, etc). Terminal degraded state, not an identity or availability
  /// problem.
  backendFailure,
}

/// D2-A — Minimal explicit operation intent for listener coordination.
/// _isGoogleSigningIn remains separate (UI re-entrancy).
sealed class AuthIntent {
  const AuthIntent();
}

class EmailLoginIntent extends AuthIntent {
  const EmailLoginIntent();
}

class EmailSignupIntent extends AuthIntent {
  const EmailSignupIntent(this.username);
  final String username;
}

class GoogleLoginIntent extends AuthIntent {
  const GoogleLoginIntent();
}

/// D1/D2 — Intent stored while the session is parked in the
/// pending-email-verification state. It carries everything the SINGLE
/// post-verification exchange needs:
/// - signup: the pending registration username (USERNAME_TAKEN retry path)
/// - mixed-provider (D1): the Google credential that hit
///   `account-exists-with-different-credential`, to be linked into the
///   verified identity after verification (one Firebase UID per human).
/// It is scoped to the Firebase UID + email it was created for, so a stale
/// intent can never attach to a different identity.
class PendingEmailVerificationIntent {
  const PendingEmailVerificationIntent({
    required this.email,
    required this.firebaseUid,
    this.username,
    this.googleCredential,
  });

  final String email;
  final String firebaseUid;
  final String? username;
  final AuthCredential? googleCredential;

  /// Whether this intent provably belongs to [user] (same UID AND same email).
  bool matches(User user) =>
      user.uid == firebaseUid &&
      (user.email ?? '').trim().toLowerCase() == email.trim().toLowerCase();
}

/// Outcome of [AuthController.completeProfile] — carries the backend's
/// canonical rejection back to the completion SURFACE instead of mutating the
/// global auth state, so the user corrects the username on the same screen
/// (backend = the single username authority, inline feedback, one language
/// shared with the registration form via [registrationUsernameErrorMessage]).
class ProfileCompletionOutcome {
  final bool success;

  /// Backend rejected the chosen username (USERNAME_TAKEN / RESERVED /
  /// INVALID_FORMAT / 409 race). Presentational — show inline, stay on the
  /// completion screen.
  final String? usernameError;

  /// Transient/generic failure (unreachable backend, unexpected 4xx).
  /// State is preserved; the surface shows the message and the user may
  /// retry or use the Sign Out escape hatch.
  final String? failureError;

  const ProfileCompletionOutcome.success()
    : success = true,
      usernameError = null,
      failureError = null;

  const ProfileCompletionOutcome.usernameRejected(this.usernameError)
    : success = false,
      failureError = null;

  const ProfileCompletionOutcome.failure(this.failureError)
    : success = false,
      usernameError = null;
}

/// Structured-first classification. See [AuthSyncErrorKind] for the
/// semantics of each outcome.
///
/// [errorCode]/[statusCode] should be threaded straight from the failing
/// [Result] (`result.errorCode`, `result.statusCode`) wherever one is
/// available. They will be `null` for raw Dart/Firebase-SDK exceptions
/// caught outside of an HTTP response, in which case this falls back to
/// free-text matching on [error].
AuthSyncErrorKind classifyAuthSyncError(
  String? error, {
  String? errorCode,
  int? statusCode,
}) {
  switch (errorCode) {
    case 'INVALID_TOKEN':
      return AuthSyncErrorKind.identityInvalid;
    case 'ACCOUNT_DELETED':
      return AuthSyncErrorKind.accountDeleted;
    case 'ACCOUNT_INACTIVE':
      return AuthSyncErrorKind.accountInactive;
    // D2 hard gate: the presented identity's email is not verified. Business
    // flow with its own screen — never a degraded backend state (INV-7).
    case 'EMAIL_NOT_VERIFIED':
      return AuthSyncErrorKind.pendingEmailVerification;
    // D4: the account row for this email is bound to a different Firebase
    // UID. Canonical anomaly — terminal, clean sign-out, no re-bind.
    case 'IDENTITY_CONFLICT':
      return AuthSyncErrorKind.identityConflict;
  }

  // A 5xx with no matching structured code above is always a backend
  // outage, regardless of message wording.
  if (statusCode != null && statusCode >= 500) {
    return AuthSyncErrorKind.backendUnavailable;
  }

  // FALLBACK (legacy free-text matching): raw Firebase SDK exceptions and
  // transport-level failures never carry errorCode/statusCode since they
  // aren't HTTP responses.
  if (isIdentityErrorMessage(error)) return AuthSyncErrorKind.identityInvalid;
  if (isBackendUnavailableErrorMessage(error)) {
    return AuthSyncErrorKind.backendUnavailable;
  }
  return AuthSyncErrorKind.backendFailure;
}

/// 🔒 ERROR CLASSIFICATION (fallback, free-text): Determine if error is
/// identity-related. Used only when no structured `errorCode` is available
/// — see [classifyAuthSyncError] for the structured-first classification.
bool isIdentityErrorMessage(String? error) {
  if (error == null) return false;
  final lower = error.toLowerCase();

  // Identity errors - user's Firebase session is invalid
  // `no-current-user` / `no user currently signed in`: the User object's
  // underlying session is gone (account deleted out-of-band, token revoked
  // mid-flow). Recovery via "Coba Lagi" is impossible — the honest state is
  // a clean sign-out back to the welcome flow, NOT a degraded retry screen.
  return lower.contains('invalid token') ||
      lower.contains('token_expired') ||
      lower.contains('no user record') ||
      lower.contains('firebase_auth/unknown') ||
      lower.contains('user-not-found') ||
      lower.contains('user deleted') ||
      lower.contains('no-current-user') ||
      lower.contains('no user currently signed in') ||
      lower.contains('auth/invalid-credential');
}

/// 🔒 ERROR CLASSIFICATION (fallback, free-text): Determine if error is
/// backend unavailable. Used only when no structured `errorCode`/
/// `statusCode` is available — see [classifyAuthSyncError].
bool isBackendUnavailableErrorMessage(String? error) {
  if (error == null) return false;
  final lower = error.toLowerCase();

  // Backend unavailable errors - temporary issues
  return lower.contains('timeout') ||
      lower.contains('connection') ||
      lower.contains('network') ||
      lower.contains('500') ||
      lower.contains('502') ||
      lower.contains('503') ||
      lower.contains('504') ||
      lower.contains('socket') ||
      lower.contains('host lookup') ||
      lower.contains('connection refused');
}

/// THE single message mapping for backend username rejections
/// (USERNAME_TAKEN / USERNAME_RESERVED / USERNAME_INVALID_FORMAT /
/// USERNAME_IMMUTABLE) — shared by EVERY username surface (registration form
/// AND complete-profile) so all screens speak the same language about the same
/// backend authority. Returns `null` when the code is not a canonical username
/// rejection.
///
/// Backend remains the final authority for these outcomes (Stage 1A contract);
/// this only converts the machine-readable code into presentational text.
String? registrationUsernameErrorMessage(String? errorCode) {
  switch (errorCode) {
    case 'USERNAME_TAKEN':
      return 'Username ini sudah digunakan. Silakan pilih username lain.';
    case 'USERNAME_RESERVED':
      return 'Username ini tidak dapat digunakan. Silakan pilih username lain.';
    case 'USERNAME_INVALID_FORMAT':
      return 'Format username tidak valid. Gunakan 3-30 karakter huruf kecil, '
          'angka, dan underscore.';
    case 'USERNAME_IMMUTABLE':
      return 'Username tidak dapat diubah setelah pendaftaran.';
  }
  return null;
}

/// Authentication Controller - Owner of Auth State & Session
///
/// **PRIMARY RESPONSIBILITIES (BOUNDARIES):**
/// 1. AUTH: Firebase login/signup/signout, OAuth, provider linking (D1)
/// 2. SESSION: Backend sync, periodic validation, FCM cleanup
/// 3. STATE MACHINE: Emits states that router uses for decisions
///
/// **NOT THIS CONTROLLER'S RESPONSIBILITY:**
/// - ROUTE DECISIONS: Owned by goRouterProvider redirect logic
/// - PROFILE UI: Owned by profile feature
/// - APP ENTRY UI: Owned by onboarding feature (splash/welcome screens)
///
/// **DOMAIN SEPARATION:**
/// - Auth feature: Authentication state, session management
/// - Onboarding feature: Splash screen, welcome screen (app entry UI only)
/// - Profile feature: User profile data, settings UI
///
/// **SOURCE OF TRUTH (Single Source):**
/// - Identity (email, password, emailVerified): Firebase Auth (PRIMARY)
/// - Profile Data (username, bio, avatarUrl): PostgreSQL via Backend API /users/me
/// - Roles & Permissions: PostgreSQL via Backend API /users/me
///
/// **EMAIL VERIFICATION (D2 HARD GATE, design scope v2):**
/// - Signup → createUser + sendEmailVerification → AuthStatePendingEmailVerification
/// - Login with unverified email → AuthStatePendingEmailVerification
/// - The backend exchange happens ONLY after Firebase reports emailVerified
///   (INV-8: verify → exchange; there is no second path)
/// - EMAIL_NOT_VERIFIED from the backend routes to the verify-email screen —
///   it is a business decision, not a degraded server state (INV-7)
///
/// **STATE MACHINE FLOW (Prevents Premature Routing):**
/// 1. AuthStateInitial → Initial state
/// 2. AuthStateFirebaseAuthenticated → Firebase login succeeded (internal transition)
/// 3. AuthStateSyncingWithBackend → Backend sync in progress (NO redirect)
/// 4. AuthStateAuthenticated → Backend data loaded, router can NOW evaluate redirects
/// 5. AuthStateUnauthenticated → User logged out
/// 6. AuthStateError → Error occurred
/// 7. AuthStateRequiresProfileCompletion → Profile completion needed (router → /auth/complete-profile)
/// 8. AuthStatePendingEmailVerification → Email unverified (router → /auth/verify-email)
/// 9. AuthStateBackendFailure/BackendUnavailable → Degraded mode (router: no redirect)
///
/// **APP ENTRY ROUTING OWNER: goRouterProvider**
/// - Router watches authControllerProvider state
/// - Router's _handleAuthenticationRedirect() decides route based on state
/// - AuthController ONLY emits state, NEVER decides routes
///
/// Mengikuti DEVELOPMENT_STANDARDS_V1_ID.md:
/// - Interface-first design dengan dependency injection ✅
/// - Result pattern untuk error handling ✅
/// - Proper state management ✅
class AuthController extends Notifier<AuthState> {
  late final IAuthRepository _authRepository;
  late final ILoggerService _logger;
  late final IAnalyticsRepository _analytics;
  late final ILocalStorageService _localStorage;
  UserSyncService? _userSyncService; // Backend sync service for roles

  // 🔒 SECURITY FIX: Periodic session validation timer
  Timer? _sessionValidationTimer;

  // 🔐 AUTH PERSISTENCE FIX: Stream subscription for Firebase Auth state changes
  StreamSubscription<User?>? _authStateSubscription;

  // 🔧 SYNC FIX: Track if backend sync has been completed for current user
  // This prevents race condition where /users/me is called before /users/sync completes
  String? _syncedUserId;
  // 🔒 MUTEX: Prevent concurrent sync operations
  // Use Completable as a simple mutex lock
  Future<void>? _ongoingSync;

  // 🛡️ RE-ENTRANCY GUARD: Prevent multiple rapid taps on Google sign-in button
  bool _isGoogleSigningIn = false;

  // D2-A — Single explicit intent carrying signup username.
  // _isGoogleSigningIn remains separate (UI re-entrancy).
  AuthIntent? _authIntent;

  // D1/D2 — Intent parked while the session waits for email verification.
  // Flushed into the single post-verification exchange by
  // [checkPendingEmailVerification].
  PendingEmailVerificationIntent? _pendingVerificationIntent;

  // Canonical username-rejection message from the LATEST backend rejection of
  // the registration exchange. Consumed by the sign-up screen to render the
  // SAME message the complete-profile surface shows inline — one language,
  // one authority (backend). Cleared whenever a new signup/retry begins.
  String? _lastRegistrationUsernameError;
  String? get lastRegistrationUsernameError => _lastRegistrationUsernameError;

  int _authHydrationGeneration = 0;

  // Phase 3D: Labuda startup single-flight
  Future<bool>? _labudaRestoreFuture;
  bool _initialFirebaseEventPending = true;

  /// Set new state with logging for all transitions (debug-mode verbosity).
  void _setState(AuthState newState) {
    final oldState = state;
    state = newState;

    _logger.log(
      '[AUTH] State: ${oldState.runtimeType} → ${newState.runtimeType}',
      level: LogLevel.info,
    );
  }

  User? get activeFirebaseUser => FirebaseAuth.instance.currentUser;

  Future<void> performFirebaseSignOut() => FirebaseAuth.instance.signOut();

  bool get shouldInitializeAuthListener => true;

  int _beginHydrationRequest() => ++_authHydrationGeneration;

  bool _isCurrentHydrationRequest(int requestGeneration) =>
      requestGeneration == _authHydrationGeneration;

  AuthUser _canonicalizeBackendUser(AuthUser user) {
    return user;
  }

  void _publishIfCurrent(int requestGeneration, AuthState newState) {
    if (!_isCurrentHydrationRequest(requestGeneration)) return;
    _setState(newState);
  }

  void _publishAuthenticatedIfCurrent(
    int requestGeneration,
    AuthUser user, {
    required bool emailVerified,
  }) {
    _publishIfCurrent(
      requestGeneration,
      AuthState.authenticated(user, emailVerified: emailVerified),
    );
  }

  /// Router-level authentication status - simplified for redirect logic
  ///
  /// Router hanya membaca status ini, bukan AuthState secara langsung.
  /// State machine internal AuthState tetap kompleks, tapi router
  /// hanya melihat status final ini.
  AppAuthStatus get appAuthStatus {
    final currentState = state;

    // Initializing states - show splash
    if (currentState is AuthStateInitial ||
        currentState is AuthStateLoading ||
        currentState is AuthStateFirebaseAuthenticated ||
        currentState is AuthStateSyncingWithBackend) {
      return AppAuthStatus.initializing;
    }

    // Profile completion required - show complete profile screen
    if (currentState is AuthStateRequiresProfileCompletion) {
      return AppAuthStatus.initializing;
    }

    // D2 hard gate: pending email verification is an explicit business-flow
    // state. The router maps the AuthState directly to /auth/verify-email
    // (same pattern as RequiresProfileCompletion); status-wise it parks on
    // splash like the other pre-authenticated states.
    if (currentState is AuthStatePendingEmailVerification) {
      return AppAuthStatus.initializing;
    }

    // Degraded states - backend issues but Firebase session valid
    // Router will NOT redirect automatically - stay on current route
    if (currentState is AuthStateBackendFailure ||
        currentState is AuthStateBackendUnavailable) {
      return AppAuthStatus.degraded;
    }

    // Unauthenticated states - show welcome/login
    if (currentState is AuthStateUnauthenticated ||
        currentState is AuthStateError) {
      return AppAuthStatus.unauthenticated;
    }

    // Account restricted - show restriction screen
    if (currentState is AuthStateAccountRestricted) {
      return AppAuthStatus.accountRestricted;
    }

    // Authenticated state - show home
    if (currentState is AuthStateAuthenticated) {
      return AppAuthStatus.authenticated;
    }

    // Fallback - treat as initializing
    return AppAuthStatus.initializing;
  }

  @override
  AuthState build() {
    // Get auth repository from Riverpod provider
    _authRepository = ref.read(authRepositoryProvider);

    // R4.3: Use provider instead of singleton instance
    _logger = ref.read(loggerServiceProvider);

    // R4.3: Use core provider instead of GetIt
    _analytics = ref.read(coreAnalyticsRepositoryProvider);

    // R4.3: Use provider instead of GetIt for UserSyncService
    try {
      _userSyncService = ref.read(userSyncServiceProvider);
    } catch (e) {
      // UserSyncService not initialized yet, skip
      // Backend sync will be skipped until API layer is configured
    }

    _localStorage = ref.read(localStorageServiceProvider);

    // SESSION HONESTY (Tier 2): Wire the AuthInterceptor's session-
    // expired signal to this controller so that a 401 followed by a
    // failed Firebase token refresh forces a clean logout rather than
    // leaving the user in an authenticated shell where every API call
    // 401-loops silently. The callback is idempotent (no-op when
    // already AuthStateUnauthenticated) so duplicate signals from a
    // 401 burst are harmless.
    AuthInterceptor.onSessionExpired = handleSessionExpired;

    // Initialize auth state asynchronously (don't wait)
    if (shouldInitializeAuthListener) {
      _initializeAuthState();
    }

    return const AuthState.initial();
  }

  /// SESSION HONESTY (Tier 2): Called by [AuthInterceptor] when a 401
  /// is received AND the subsequent Firebase token refresh fails (or
  /// returns a null/empty token, or there is no current Firebase user).
  ///
  /// In all of those cases the client-side session can no longer be
  /// trusted, so the only honest behavior is to drop into the
  /// unauthenticated state. The router then redirects to /welcome —
  /// no separate "session expired" route is needed because
  /// AuthStateUnauthenticated already drives the canonical sign-out
  /// flow.
  ///
  /// Idempotent: if state is already [AuthStateUnauthenticated] this
  /// returns immediately. That prevents a burst of in-flight 401s from
  /// invoking [signOut] more than once.
  Future<void> handleSessionExpired() async {
    if (state is AuthStateUnauthenticated) {
      // Either we were already signed out (e.g. logout in flight) or a
      // previous 401 in the same burst already drove the transition.
      _logger.log(
        '[AUTH] handleSessionExpired: already AuthStateUnauthenticated, '
        'no-op',
        level: LogLevel.debug,
      );
      return;
    }

    _logger.warning(
      'handleSessionExpired: forcing signOut — '
      'token refresh failed on a 401 (session no longer trustworthy)',
    );

    // Reuse the canonical sign-out path so FCM cleanup, analytics,
    // sync-lock reset, and the AuthStateUnauthenticated transition all
    // run exactly as on a user-initiated logout.
    await signOut();
  }

  // Phase 3D: Labuda-first startup — Firebase must NOT resurrect Labuda session.
  bool _isLabudaAccessTokenExpired(String token) {
    try {
      final parts = token.split('.');
      if (parts.length != 3) return true;
      final payload = parts[1];
      final normalized = base64Url.normalize(payload);
      final decoded = utf8.decode(base64Url.decode(normalized));
      final map = jsonDecode(decoded) as Map<String, dynamic>;
      final exp = map['exp'];
      int? expSec;
      if (exp is int) expSec = exp;
      if (exp is double) expSec = exp.toInt();
      if (expSec == null) return true;
      final expDate = DateTime.fromMillisecondsSinceEpoch(expSec * 1000);
      return DateTime.now().isAfter(expDate.subtract(const Duration(seconds: 30)));
    } catch (_) {
      return true;
    }
  }

  Future<bool> _restoreLabudaSession() async {
    if (_labudaRestoreFuture != null) return _labudaRestoreFuture!;
    final completer = Completer<bool>();
    _labudaRestoreFuture = completer.future;
    bool result = false;
    try {
      result = await _doRestoreLabudaSession();
      completer.complete(result);
    } catch (_) {
      if (!completer.isCompleted) completer.complete(false);
      result = false;
    } finally {
      _labudaRestoreFuture = null;
    }
    return result;
  }

  Future<bool> _doRestoreLabudaSession() async {
    final accessRes = await _localStorage.readLabudaAccessToken();
    final access = accessRes.data?.trim();
    if (access != null && access.isNotEmpty && !_isLabudaAccessTokenExpired(access)) {
      try {
        final svc = _userSyncService;
        if (svc != null) {
          final userRes = await svc.getCurrentUser().timeout(const Duration(seconds: 10));
          if (userRes.isSuccess && userRes.data != null) {
            final user = _canonicalizeBackendUser(userRes.data!);
            final gen = _beginHydrationRequest();
            _publishAuthenticatedIfCurrent(gen, user, emailVerified: user.isEmailVerified);
            _startSessionValidation();
            _logger.info('[AUTH] Labuda startup restore via valid access token');
            return true;
          }
        }
      } catch (e) {
        _logger.warning('[AUTH] Labuda access validation failed, trying refresh', extra: {'error': e.toString()});
      }
    }
    // Try refresh if access missing/expired or validation failed
    final refreshRes = await _localStorage.readLabudaRefreshToken();
    final refresh = refreshRes.data?.trim();
    if (refresh == null || refresh.isEmpty) {
      _logger.info('[AUTH] No Labuda refresh token for startup');
      return false;
    }
    try {
      final apiDs = ref.read(auth_data.authApiDatasourceProvider);
      final refreshResult = await apiDs.refreshPlatformToken(refresh).timeout(const Duration(seconds: 10));
      if (refreshResult.isError || refreshResult.data == null) {
        _logger.warning('[AUTH] Labuda refresh failed at startup', extra: {'error': refreshResult.error});
        return false;
      }
      final pair = refreshResult.data!;
      final saveRes = await _localStorage.saveLabudaCredential(pair.accessToken, pair.refreshToken);
      if (saveRes.isError) {
        _logger.warning('[AUTH] saveLabudaCredential failed after startup refresh');
        return false;
      }
      final svc = _userSyncService;
      if (svc != null) {
        final userRes = await svc.getCurrentUser().timeout(const Duration(seconds: 10));
        if (userRes.isSuccess && userRes.data != null) {
          final user = _canonicalizeBackendUser(userRes.data!);
          final gen = _beginHydrationRequest();
          _publishAuthenticatedIfCurrent(gen, user, emailVerified: user.isEmailVerified);
          _startSessionValidation();
          _logger.info('[AUTH] Labuda startup restore via refresh succeeded');
          return true;
        }
      }
      return false;
    } catch (e) {
      _logger.warning('[AUTH] Labuda refresh exception at startup', extra: {'error': e.toString()});
      return false;
    }
  }

  /// Listener-side routing after a Firebase identity event for a NON-explicit
  /// flow (external session changes, token refresh). Explicit flows own their
  /// own completion (AUTH-2). Applies the D2 hard gate so an unverified
  /// session never reaches the exchange (INV-8), then delegates to the
  /// canonical backend sync.
  void _routeAfterFirebaseEvent(User firebaseUser) {
    if (!firebaseUser.emailVerified) {
      _logger.info(
        '[AUTH] Firebase event with unverified email — entering pending '
        'verification (hard gate, no exchange)',
        extra: {'uid': firebaseUser.uid},
      );
      _publishIfCurrent(
        _beginHydrationRequest(),
        AuthState.pendingEmailVerification(email: firebaseUser.email ?? ''),
      );
      return;
    }
    // Restore a matching parked signup intent as the exchange intent so the
    // post-verification exchange carries the registration username.
    final intent = _pendingVerificationIntent;
    if (intent != null && intent.matches(firebaseUser)) {
      final username = intent.username?.trim() ?? '';
      if (username.isNotEmpty) _authIntent = EmailSignupIntent(username);
    }
    _syncWithBackend(firebaseUser.uid, firebaseUser, isEmailSignup: false);
  }

  void _setupFirebaseAuthListener() {
    _initialFirebaseEventPending = true;
    _authStateSubscription = FirebaseAuth.instance.authStateChanges().listen(
      (User? user) async {
        _logger.log('[AUTH] Firebase Auth listener fired → ${user != null ? "User(${user.uid})" : "None"}', level: LogLevel.info);
        final isInitial = _initialFirebaseEventPending;
        if (isInitial) _initialFirebaseEventPending = false;
        // Phase 3D: Firebase must NOT resurrect Labuda when Labuda missing at startup.
        // The initial emission reflects pre-existing Firebase session, not explicit login.
        if (isInitial &&
            user != null &&
            _authIntent == null &&
            _pendingVerificationIntent == null) {
          bool hasLabuda = false;
          try {
            final r = await _localStorage.hasLabudaCredential();
            hasLabuda = r.data == true;
          } catch (_) {
            hasLabuda = false;
          }
          if (!hasLabuda && state is AuthStateUnauthenticated) {
            _logger.info('[AUTH] Initial Firebase user present but no Labuda session — staying unauthenticated (no exchange fallback)');
            return;
          }
        }
        if (user != null) {
          if (_authIntent == null &&
              _pendingVerificationIntent == null &&
              !isInitial) {
            bool hasLabuda2 = false;
            try {
              final r2 = await _localStorage.hasLabudaCredential();
              hasLabuda2 = r2.data == true;
            } catch (_) {
              hasLabuda2 = false;
            }
            final hasLabuda = hasLabuda2;
            if (!hasLabuda && state is AuthStateUnauthenticated) {
              _logger.info('[AUTH] Firebase user present but no Labuda session — staying unauthenticated (no exchange fallback)');
              return;
            }
            if (state is AuthStateAuthenticated) {
              _logger.debug('[AUTH] Firebase event while Labuda authenticated — ignoring');
              return;
            }
          }
          final principal = FirebasePrincipal.fromFirebaseUser(user);
          try {
            _logger.log('[AUTH] user.reload() start', level: LogLevel.debug);
            await user.reload().timeout(const Duration(seconds: 5));
            _logger.log('[AUTH] user.reload() done', level: LogLevel.debug);
            final refreshedUser = activeFirebaseUser;
            if (refreshedUser == null) {
              _setState(const AuthState.unauthenticated());
              return;
            }
            _setState(AuthState.firebaseAuthenticated(refreshedUser.uid, principal: FirebasePrincipal.fromFirebaseUser(refreshedUser)));
            // AUTH-2 (CANONICAL COMPLETION): During an EXPLICIT login the
            // initiating method owns the backend sync and its deterministic
            // completion (login must not depend on listener timing). The listener
            // is used for session lifecycle / external auth changes only. When an
            // explicit flow (login or pending verification) is in progress,
            // defer to it so we never produce a duplicate exchange or a
            // conflicting terminal state.
            if (_authIntent != null || _pendingVerificationIntent != null) {
              _logger.debug('[AUTH] Explicit flow in progress — deferring sync to initiator');
              return;
            }
            _routeAfterFirebaseEvent(refreshedUser);
          } catch (e) {
            _logger.error('Failed to reload Firebase user', extra: {'error': e.toString()});
            _setState(AuthState.firebaseAuthenticated(user.uid, principal: principal));
            if (_authIntent != null || _pendingVerificationIntent != null) {
              _logger.debug('[AUTH] Explicit flow in progress — deferring sync to initiator (after reload failure)');
              return;
            }
            _routeAfterFirebaseEvent(user);
          }
        } else {
          // Firebase signed out — but if Labuda still authenticated, keep Labuda (step 7)
          bool hasLabuda3 = false;
          try {
            final r3 = await _localStorage.hasLabudaCredential();
            hasLabuda3 = r3.data == true;
          } catch (_) {
            hasLabuda3 = false;
          }
          final hasLabuda = hasLabuda3;
          if (hasLabuda && state is AuthStateAuthenticated) {
            _logger.info('[AUTH] Firebase null but Labuda authenticated — keeping Labuda session');
            return;
          }
          _setState(const AuthState.unauthenticated());
          _stopSessionValidation();
        }
      },
      onError: (e) {
        _logger.error('Firebase Auth state error', extra: {'error': e.toString()});
        // Do not clear Labuda authenticated state on Firebase stream error
        if (state is AuthStateAuthenticated) return;
        _setState(const AuthState.unauthenticated());
      },
    );
  }

  void _initializeAuthState() {
    _logger.log('[AUTH] _initializeAuthState() called', level: LogLevel.info);
    _authStateSubscription?.cancel();
    Future.delayed(const Duration(seconds: 15), () {
      if (state is AuthStateInitial) {
        _logger.log('[AUTH] STARTUP TIMEOUT: still AuthStateInitial after 15s — forcing unauthenticated', level: LogLevel.error);
        _setState(const AuthState.unauthenticated());
      }
    });
    // Labuda-first startup authority
    _restoreLabudaSession().then((restored) {
      if (restored) {
        _logger.info('[AUTH] Startup Labuda restore succeeded');
      } else {
        _logger.info('[AUTH] Startup Labuda restore not available — unauthenticated');
        if (state is AuthStateInitial) _setState(const AuthState.unauthenticated());
      }
      _setupFirebaseAuthListener();
    });
  }

  /// 🔧 BACKEND SYNC: Sync user with backend and get complete user data
  /// This is the PRIMARY method that handles the full sync flow:
  /// 1. Call /users/sync to ensure user exists in PostgreSQL
  /// 2. Call /users/me to get complete user data including roles
  /// 3. Only then set AuthStateAuthenticated (router can NOW evaluate redirects)
  ///
  /// 🔐 D2 HARD GATE (defense-in-depth): signup exchanges are only reached
  /// after verification. The explicit login/Google paths check
  /// [_mayExchangeVerifiedEmail] BEFORE calling this method; an unverified
  /// signup identity here is routed to the pending-verification state.
  ///
  /// 🔒 MUTEX GUARD: Only one sync operation can run at a time per user session.
  /// Multiple calls for the same userId will be deduplicated.
  ///
  /// 🔐 FLOW AWARENESS: username handling differs by authentication flow:
  /// - Email signup (isEmailSignup=true): Requires the pending username; only username is collected at signup
  /// - Email login (isEmailSignup=false): Does NOT use pending values, does NOT generate new username
  /// - Google login (isEmailSignup=false): Generates username from email for new users
  Future<void> _syncWithBackend(
    String userId,
    User firebaseUser, {
    required bool isEmailSignup,
  }) async {
    final principal = FirebasePrincipal.fromFirebaseUser(firebaseUser);
    _logger.log(
      '[SYNC] Starting backend sync for userId: $userId',
      level: LogLevel.debug,
    );

    // 🔒 GUARD 1: Skip if already in a valid post-sync state
    // RequiresProfileCompletion is a valid completed state (sync succeeded, profile needs username)
    // Re-syncing from this state on token refresh causes splash loop
    if (_syncedUserId == userId) {
      if (state is AuthStateAuthenticated ||
          state is AuthStateRequiresProfileCompletion ||
          state is AuthStateAccountRestricted) {
        _logger.log(
          '[SYNC] Skip sync - already in valid post-sync state',
          level: LogLevel.debug,
        );
        return;
      }
      _logger.log(
        '[SYNC] State mismatch - forcing re-sync',
        level: LogLevel.warning,
      );
      _syncedUserId = null;
    }

    // 🔒 GUARD 2: Wait for ongoing sync if any, then skip
    if (_ongoingSync != null) {
      await _logger.info(
        'Sync already in progress, waiting',
        extra: {'userId': userId},
      );
      await _ongoingSync;
      if (_syncedUserId == userId) {
        await _logger.info(
          'User synced during concurrent call, skipping',
          extra: {'userId': userId},
        );
        return;
      }
    }

    // 🔒 GUARD 3: Mark sync as in progress
    final syncCompleter = Completer<void>();
    _ongoingSync = syncCompleter.future;
    final requestGeneration = _beginHydrationRequest();

    try {
      // 🔐 D2 HARD GATE (INV-4/INV-8, defense-in-depth): a signup exchange is
      // only reachable after verification. An unverified signup identity here
      // means a caller bypassed the pending-verification state — park it
      // there instead of exchanging. Login/Google unverified sessions are
      // gated before this call and, if the backend still rejects, the
      // EMAIL_NOT_VERIFIED classification below routes them to the same
      // verify screen.
      if (isEmailSignup && !firebaseUser.emailVerified) {
        _logger.warning(
          '[SYNC] Signup exchange blocked: Firebase email not verified (hard gate)',
          extra: {'uid': firebaseUser.uid},
        );
        _pendingVerificationIntent = PendingEmailVerificationIntent(
          email: firebaseUser.email ?? '',
          firebaseUid: firebaseUser.uid,
          username: _authIntent is EmailSignupIntent
              ? (_authIntent as EmailSignupIntent).username
              : null,
        );
        _publishIfCurrent(
          requestGeneration,
          AuthState.pendingEmailVerification(
            email: firebaseUser.email ?? '',
            username: _pendingVerificationIntent!.username,
          ),
        );
        return;
      }

      _publishIfCurrent(
        requestGeneration,
        AuthState.syncingWithBackend(userId, principal: principal),
      );

      if (_userSyncService == null) {
        _logger.log(
          '[SYNC] Backend service not configured — userSyncService is null',
          level: LogLevel.error,
        );
        _setState(const AuthState.unauthenticated());
        return;
      }
      _logger.log('[SYNC] Calling /users/sync...', level: LogLevel.info);

      // 🔐 FLOW AWARENESS: Determine username based on explicit flow type
      String syncUsername;

      if (isEmailSignup) {
        // 📧 EMAIL SIGNUP: Require pending username
        if (_authIntent is! EmailSignupIntent) {
          _logger.error('[SYNC] Email signup missing username');
          _setState(
            const AuthState.error('Signup data missing. Please try again.'),
          );
          return;
        }

        syncUsername = (_authIntent as EmailSignupIntent).username.trim();

        if (syncUsername.isEmpty) {
          _logger.error('[SYNC] Email signup has empty username');
          _setState(const AuthState.error('Username is required.'));
          return;
        }

        _logger.log(
          '[SYNC] Email signup using pending username: $syncUsername',
          level: LogLevel.debug,
        );
      } else {
        // 🔐 EMAIL LOGIN or GOOGLE: Send empty username — backend decides profileComplete
        syncUsername = '';
        _logger.log(
          '[SYNC] Login/Google - empty username, backend decides profileComplete',
          level: LogLevel.debug,
        );
      }

      // Step 1: Sync user to backend (call /users/sync)
      final syncResult = await _userSyncService!
          .syncUser(
            username: syncUsername,
            phoneNumber: firebaseUser.phoneNumber,
          )
          .timeout(
            const Duration(seconds: 15),
            onTimeout: () {
              throw Exception('SYNC TIMEOUT');
            },
          );

      if (syncResult.isError) {
        await _logger.debugSyncFailed(userId, syncResult.error);
        final error = syncResult.error;
        if (syncResult.errorCode == 'SESSION_USER_MISMATCH') {
          _setState(AuthState.error(error ?? 'Backend session user mismatch'));
          return;
        }

        // 🛡️ SIGN-OUT GUARD: Do not overwrite state if user already signed out
        if (activeFirebaseUser == null || state is AuthStateUnauthenticated) {
          _logger.log(
            '[SYNC] User signed out during sync, preserving Unauthenticated state',
            level: LogLevel.debug,
          );
          return;
        }

        // REGISTRATION USERNAME REJECTION (Stage 1B contract):
        // When the authenticated exchange rejects the registration username,
        // the canonical surface for the correction is the registration form:
        // the session returns to the unauthenticated state (the Firebase
        // identity and the pending signup intent are KEPT, so the sign-up
        // screen retries via retryRegistrationUsername without recreating
        // the account), and the router's exclusive-surface rule moves the
        // user off the verify screen to /welcome. These are terminal
        // business rejections — NOT identity errors: no signOut, no
        // identityInvalid classification.
        final usernameError = registrationUsernameErrorMessage(
          syncResult.errorCode,
        );
        if (usernameError != null) {
          _logger.warning(
            'Registration username rejected by backend — returning to the '
            'registration flow',
            extra: {'error': error, 'errorCode': syncResult.errorCode},
          );
          _pendingVerificationIntent = null;
          _lastRegistrationUsernameError = usernameError;
          _setState(const AuthState.unauthenticated());
          return;
        }

        // PASS 2A / F1: structured-first classification using the backend's
        // errorCode/statusCode (INVALID_TOKEN, ACCOUNT_DELETED,
        // ACCOUNT_INACTIVE, EMAIL_NOT_VERIFIED, IDENTITY_CONFLICT), falling
        // back to free-text matching only when no structured code is present.
        final errorKind = classifyAuthSyncError(
          error,
          errorCode: syncResult.errorCode,
          statusCode: syncResult.statusCode,
        );

        if (!_isCurrentHydrationRequest(requestGeneration)) {
          return;
        }

        // 🔒 IDENTITY INVALID / ACCOUNT DELETED: Firebase token rejected or
        // account no longer exists - MUST signOut. Leaving the Firebase
        // session alive here would keep the user in an indefinite retry
        // loop against a token/account the backend has already rejected.
        if (errorKind == AuthSyncErrorKind.identityInvalid ||
            errorKind == AuthSyncErrorKind.accountDeleted) {
          _logger.warning(
            'Identity error detected ($errorKind), signing out',
            extra: {'error': error, 'errorCode': syncResult.errorCode},
          );
          await performFirebaseSignOut();
          _setState(const AuthState.unauthenticated());
          return;
        }

        // 🔐 D2: business-flow state — route to the verify-email screen.
        // The backend refused the exchange because the email is unverified.
        // This is NOT degraded (INV-7); no username is known here, so the
        // pending intent is dropped (the user re-authenticates after
        // verifying).
        if (errorKind == AuthSyncErrorKind.pendingEmailVerification) {
          _logger.warning(
            '[SYNC] Backend rejected exchange: email not verified — routing to verify screen',
            extra: {'errorCode': syncResult.errorCode},
          );
          _pendingVerificationIntent = null;
          _publishIfCurrent(
            requestGeneration,
            AuthState.pendingEmailVerification(email: firebaseUser.email ?? ''),
          );
          return;
        }

        // 🔒 D4: canonical identity anomaly — the account row for this email
        // is bound to a different Firebase UID. Terminal: sign out cleanly.
        // Never auto-retried, never silently re-bound.
        if (errorKind == AuthSyncErrorKind.identityConflict) {
          _logger.warning(
            '[SYNC] Backend rejected exchange: IDENTITY_CONFLICT — signing out cleanly',
            extra: {'errorCode': syncResult.errorCode},
          );
          _pendingVerificationIntent = null;
          _authIntent = null;
          await performFirebaseSignOut();
          _setState(const AuthState.unauthenticated());
          return;
        }

        // Account inactive sessions return a terminal auth error.
        // This path returns a terminal auth error when the backend exchange
        // cannot complete. We do not publish an authenticated state until
        // the backend session is fully established.
        if (errorKind == AuthSyncErrorKind.accountInactive) {
          _logger.warning(
            'Account inactive detected during sync',
            extra: {'error': error, 'errorCode': syncResult.errorCode},
          );
          _setState(AuthState.error(error ?? 'Your account is not active.'));
          return;
        }
        // Backend unavailable: timeout, network, and 5xx errors — terminal
        // degraded state. Do not sign out; recovery is explicit via
        // retryBackendSync() (current Firebase identity → _syncWithBackend).
        if (errorKind == AuthSyncErrorKind.backendUnavailable) {
          _logger.warning(
            'Backend unavailable - keeping Firebase session',
            extra: {'error': error},
          );
          _publishIfCurrent(
            requestGeneration,
            AuthState.backendUnavailable(error ?? 'Backend unavailable'),
          );
          return;
        }

        // 🔒 BACKEND FAILURE: 4xx validation errors - do NOT signOut
        _logger.warning(
          'Backend sync failed - validation/business error',
          extra: {'error': error},
        );
        _publishIfCurrent(
          requestGeneration,
          AuthState.backendFailure(error ?? 'Backend sync failed'),
        );
        return;
      }

      await _logger.debugSyncSuccess(userId);
      _syncedUserId = userId;

      // 🔐 BACKEND-AUTHORITATIVE PROFILE COMPLETION: Check backend profile_complete flag
      // Profile completion is determined by backend, not by created flag or provider type
      // Backend returns profile_complete = true ONLY if username is set and not empty
      final syncData = syncResult.data!;
      final profileComplete = syncData.profileComplete;

      if (activeFirebaseUser == null || state is AuthStateUnauthenticated) {
        _logger.log(
          '[SYNC] User signed out during sync, preserving Unauthenticated state',
          level: LogLevel.debug,
        );
        return;
      }

      if (!profileComplete) {
        _authIntent = null;
        _publishIfCurrent(
          requestGeneration,
          AuthState.requiresProfileCompletion(
            userId: syncData.userId,
            email: syncData.email ?? firebaseUser.email ?? '',
          ),
        );
        return;
      }

      final backendUserData = syncData.user;
      if (backendUserData == null) {
        _logger.error('[SYNC] Complete exchange response missing backend user');
        _publishIfCurrent(
          requestGeneration,
          AuthState.backendFailure('Failed to load backend user'),
        );
        return;
      }

      final backendUser = _canonicalizeBackendUser(backendUserData);

      final accountStatus = backendUser.accountStatus ?? AccountStatus.active;
      if (!_isCurrentHydrationRequest(requestGeneration)) {
        return;
      }
      if (accountStatus.isRestricted) {
        _logger.warning('[SYNC] Account restricted: ${accountStatus.apiValue}');
        _publishIfCurrent(
          requestGeneration,
          AuthState.accountRestricted(
            backendUser,
            restrictionType: accountStatus,
          ),
        );
        return;
      }
      _authIntent = null;

      _publishAuthenticatedIfCurrent(
        requestGeneration,
        backendUser,
        emailVerified: firebaseUser.emailVerified,
      );

      _startSessionValidation();

      _activateRealtimeServices(backendUser.id, firebaseUser);

      await _analytics.logEvent(
        'login',
        parameters: {'method': 'firebase_auth', 'user_id': backendUser.id},
        userId: backendUser.id,
      );
    } catch (e, stackTrace) {
      final errorStr = e.toString();
      _logger.error(
        '[SYNC ERROR] $e',
        extra: {'stackTrace': stackTrace.toString()},
      );
      await _logger.debugSyncException(userId, errorStr, stackTrace.toString());

      // 🛡️ SIGN-OUT GUARD: Do not overwrite state if user already signed out
      if (activeFirebaseUser == null || state is AuthStateUnauthenticated) {
        _logger.log(
          '[SYNC] User signed out during exception, preserving Unauthenticated state',
          level: LogLevel.debug,
        );
        return;
      }

      // PASS 2A / F1: no Result is available in a caught exception (it's a
      // raw Dart/Firebase-SDK throw, not an HTTP response), so this always
      // falls back to free-text matching inside classifyAuthSyncError.
      final errorKind = classifyAuthSyncError(errorStr);

      // 🔒 IDENTITY ERROR in catch - MUST signOut
      if (errorKind == AuthSyncErrorKind.identityInvalid ||
          errorKind == AuthSyncErrorKind.accountDeleted) {
        await performFirebaseSignOut();
        _setState(const AuthState.unauthenticated());
        return;
      }

      // 🔒 ACCOUNT INACTIVE in catch - unreachable in practice (no
      // structured code is ever available here), kept for consistency.
      if (errorKind == AuthSyncErrorKind.accountInactive) {
        _setState(AuthState.error('Account inactive: $errorStr'));
        return;
      }

      // 🔒 BACKEND UNAVAILABLE in catch - do NOT signOut
      if (errorKind == AuthSyncErrorKind.backendUnavailable) {
        _publishIfCurrent(
          requestGeneration,
          AuthState.backendUnavailable('Connection error: $errorStr'),
        );
        return;
      }

      // 🔒 BACKEND FAILURE in catch - do NOT signOut
      _publishIfCurrent(
        requestGeneration,
        AuthState.backendFailure('Sync error: $errorStr'),
      );
    } finally {
      _ongoingSync = null;
      // D2-A: clear login intents, keep EmailSignupIntent for retry (USERNAME_TAKEN)
      if (_authIntent is! EmailSignupIntent) {
        _authIntent = null;
      }
      syncCompleter.complete();
      _logger.log('[SYNC] Backend sync complete', level: LogLevel.debug);
    }
  }

  /// D2 HARD GATE (INV-8): the backend exchange is single-path and
  /// verified-only. Returns true when the CURRENT Firebase identity's email
  /// is verified and the caller may proceed; otherwise parks the session in
  /// [AuthStatePendingEmailVerification] (router → /auth/verify-email) and
  /// returns false. There is no "exchange dulu, gating belakangan" path.
  bool _mayExchangeVerifiedEmail(User firebaseUser) {
    if (firebaseUser.emailVerified) return true;
    _logger.warning(
      '[AUTH] Exchange blocked: Firebase email not verified (hard gate)',
      extra: {'uid': firebaseUser.uid},
    );
    _publishIfCurrent(
      _beginHydrationRequest(),
      AuthState.pendingEmailVerification(email: firebaseUser.email ?? ''),
    );
    return false;
  }

  /// Consumes the stored pending Google credential when it provably belongs
  /// to the CURRENT Firebase session (same UID + email). A stale/mismatched
  /// intent (and its credential) is discarded — it must never be linked to
  /// the wrong account.
  AuthCredential? _takePendingGoogleCredentialFor(User firebaseUser) {
    final intent = _pendingVerificationIntent;
    if (intent == null) return null;
    _pendingVerificationIntent = null;
    if (intent.googleCredential != null && intent.matches(firebaseUser)) {
      return intent.googleCredential;
    }
    return null;
  }

  /// Verify-screen "continue" authority (D2): runs once Firebase reports the
  /// email as verified. This is the ONLY bridge from the
  /// pending-verification state back into the canonical exchange
  /// (INV-8: verify → exchange, one path, no "coba exchange dulu").
  ///
  /// - Signup intent → exchange carries the pending username
  /// - Mixed-provider intent (D1) → the stored Google credential is linked
  ///   into this identity first (one Firebase UID per human); on link
  ///   failure the intent is KEPT so the user can retry from the screen
  /// - Plain login intent → normal login exchange
  Future<void> checkPendingEmailVerification() async {
    final firebaseUser = activeFirebaseUser;
    if (firebaseUser == null) {
      _pendingVerificationIntent = null;
      _setState(const AuthState.unauthenticated());
      return;
    }

    try {
      await firebaseUser.reload();
    } catch (e) {
      _logger.warning(
        '[AUTH] reload() failed while checking verification',
        extra: {'error': e.toString()},
      );
    }
    final current = activeFirebaseUser;
    if (current == null) {
      _pendingVerificationIntent = null;
      _setState(const AuthState.unauthenticated());
      return;
    }

    if (!current.emailVerified) {
      // Still unverified — stay parked on the verify screen.
      _publishIfCurrent(
        _beginHydrationRequest(),
        AuthState.pendingEmailVerification(email: current.email ?? ''),
      );
      return;
    }

    // Stale intent for a different identity is discarded; a matching intent
    // hands its Google credential to the D1 linking step below.
    final intent = _pendingVerificationIntent;
    if (intent != null && !intent.matches(current)) {
      _logger.warning(
        '[AUTH] Pending verification intent belongs to a different Firebase '
        'identity — discarding',
        extra: {'intentUid': intent.firebaseUid, 'sessionUid': current.uid},
      );
      _pendingVerificationIntent = null;
    }

    final matchingIntent = _pendingVerificationIntent;
    final credential = matchingIntent?.googleCredential;
    if (credential != null) {
      // D1: unify the Google identity into THIS Firebase user before the
      // exchange. Failure keeps the intent so the user can retry from the
      // verify screen — no silent retry, no lost credential.
      final linkResult = await _authRepository.signInWithGoogle(
        pendingGoogleCredential: credential,
      );
      if (linkResult.isError) {
        _logger.warning(
          '[AUTH] Linking pending Google credential failed',
          extra: {'error': linkResult.error},
        );
        _setState(
          AuthState.error(linkResult.error ?? 'Gagal menautkan akun Google.'),
        );
        return;
      }
    }

    // Consume the intent: carry the signup username into the exchange.
    _pendingVerificationIntent = null;
    final username = matchingIntent?.username?.trim() ?? '';
    if (username.isNotEmpty) {
      _authIntent = EmailSignupIntent(username);
    }
    _syncedUserId = null;
    await _syncWithBackend(
      current.uid,
      current,
      isEmailSignup: _authIntent is EmailSignupIntent,
    );
  }

  /// Sign in dengan email dan password
  ///
  /// 🔒 DETERMINISTIC FLOW (AUTH-2): Explicit login CANONICAL COMPLETION.
  ///
  /// This is the SINGLE canonical login-completion authority for email.
  /// The explicit success path runs the D2 hard gate, completes the D1
  /// mixed-provider link when a pending Google credential exists, and then
  /// calls the canonical backend sync directly. The Firebase listener is
  /// still used for session lifecycle / external auth changes, but it
  /// DEDUPES against the same `_syncedUserId` / `in-flight` guards, so:
  ///   - explicit completion + listener event  → single backend exchange
  ///   - no double credential write
  ///   - no stale state overwrite
  ///   - no stranded login
  ///
  /// Terminal failure (backendFailure / backendUnavailable) is surfaced as an
  /// explicit AuthState so the UI can render a visible error + retry instead
  /// of silently stopping on the Login screen.
  Future<void> signInWithEmail({
    required String email,
    required String password,
  }) async {
    _authIntent = const EmailLoginIntent();
    _setState(const AuthState.loading());

    final result = await _authRepository.signInWithEmail(
      email: email,
      password: password,
    );

    if (result.isError) {
      _authIntent = null;
      _setState(AuthState.error(result.error!));
      return;
    }

    // Success: complete login through the canonical backend sync authority.
    // Do NOT navigate manually — the router reacts to the resulting AuthState.
    final firebaseUser = activeFirebaseUser;
    if (firebaseUser == null) {
      _authIntent = null;
      _setState(const AuthState.unauthenticated());
      return;
    }

    // D1 mixed-provider: a previously stored Google credential for THIS
    // identity is unified into it — after verification when the email is
    // still unverified (credential travels with the parked intent), or
    // immediately before the exchange when already verified.
    final pendingCredential = _takePendingGoogleCredentialFor(firebaseUser);

    // D2 HARD GATE: unverified email NEVER reaches the exchange — the
    // session parks on the verify-email screen, carrying the pending Google
    // credential so the D1 link happens once verification lands.
    if (!firebaseUser.emailVerified) {
      _pendingVerificationIntent = PendingEmailVerificationIntent(
        email: firebaseUser.email ?? '',
        firebaseUid: firebaseUser.uid,
        googleCredential: pendingCredential,
      );
      _publishIfCurrent(
        _beginHydrationRequest(),
        AuthState.pendingEmailVerification(email: firebaseUser.email ?? ''),
      );
      _authIntent = null;
      return;
    }

    if (pendingCredential != null) {
      final linkResult = await _authRepository.signInWithGoogle(
        pendingGoogleCredential: pendingCredential,
      );
      if (linkResult.isError) {
        _authIntent = null;
        _setState(
          AuthState.error(linkResult.error ?? 'Gagal menautkan akun Google.'),
        );
        return;
      }
    }

    await _syncWithBackend(
      firebaseUser.uid,
      firebaseUser,
      isEmailSignup: false,
    );
    // _authIntent cleared in _syncWithBackend finally.
  }

  /// Sign in dengan Google
  ///
  /// 🔒 DETERMINISTIC FLOW: explicit canonical completion (same as email).
  ///
  /// 🛡️ RE-ENTRANCY GUARD: Prevent multiple rapid taps from triggering
  /// multiple flows.
  Future<void> signInWithGoogle() async {
    // 🛡️ GUARD: Ignore if already signing in (user double-tapped button)
    if (_isGoogleSigningIn) {
      await _logger.debug('Google sign-in already in progress, ignoring tap');
      return;
    }

    _isGoogleSigningIn = true;
    _authIntent = const GoogleLoginIntent();
    try {
      final userBefore = activeFirebaseUser;
      final pendingCredential = userBefore == null
          ? null
          : _takePendingGoogleCredentialFor(userBefore);

      final result = await _authRepository.signInWithGoogle(
        pendingGoogleCredential: pendingCredential,
      );

      if (result.isError) {
        _authIntent = null;
        _setState(AuthState.error(result.error!));
        return;
      }

      // Success: complete login through the canonical backend sync authority.
      // Do NOT navigate manually - the router reacts to the resulting AuthState.
      final firebaseUser = activeFirebaseUser;
      if (firebaseUser == null) {
        _authIntent = null;
        _setState(const AuthState.unauthenticated());
        return;
      }

      // D2 HARD GATE: unverified email NEVER reaches the exchange — the
      // session parks on the verify-email screen. A consumed pending Google
      // credential travels with the parked intent so the D1 link runs after
      // verification (the repository re-parked its own copy on conflict).
      if (!firebaseUser.emailVerified) {
        _pendingVerificationIntent = PendingEmailVerificationIntent(
          email: firebaseUser.email ?? '',
          firebaseUid: firebaseUser.uid,
          googleCredential: pendingCredential,
        );
        _publishIfCurrent(
          _beginHydrationRequest(),
          AuthState.pendingEmailVerification(email: firebaseUser.email ?? ''),
        );
        _authIntent = null;
        return;
      }

      // Verified: unify the pending Google credential (if any) into this
      // identity right before the exchange.
      if (pendingCredential != null) {
        final linkResult = await _authRepository.signInWithGoogle(
          pendingGoogleCredential: pendingCredential,
        );
        if (linkResult.isError) {
          _authIntent = null;
          _setState(
            AuthState.error(linkResult.error ?? 'Gagal menautkan akun Google.'),
          );
          return;
        }
      }

      await _syncWithBackend(
        firebaseUser.uid,
        firebaseUser,
        isEmailSignup: false,
      );
      // _authIntent cleared in _syncWithBackend finally.
    } finally {
      _isGoogleSigningIn = false;
    }
  }

  /// Sign up dengan Google (same as sign in for Google)
  Future<void> signUpWithGoogle() async {
    // Google sign up is the same as sign in - it auto-creates user if not exists
    await signInWithGoogle();
  }

  /// Sign up dengan email dan password
  ///
  /// 🔐 D2 HARD GATE: signup NEVER exchanges before verification.
  /// Flow: create Firebase account → sendEmailVerification (repository) →
  /// park in [AuthStatePendingEmailVerification] (router → verify screen)
  /// → user verifies → [checkPendingEmailVerification] runs the single
  /// exchange with the pending username (INV-8).
  Future<void> signUpWithEmail({
    required String email,
    required String password,
    required String username,
  }) async {
    final trimmedUsername = username.trim();

    if (trimmedUsername.isEmpty) {
      _setState(const AuthState.error('Username is required'));
      return;
    }

    _lastRegistrationUsernameError = null;
    _setState(const AuthState.loading());

    final result = await _authRepository.signUpWithEmail(
      email: email,
      password: password,
    );

    if (result.isError) {
      _setState(AuthState.error(result.error!));
      return;
    }

    final firebaseUser = activeFirebaseUser;
    if (firebaseUser == null) {
      _setState(const AuthState.unauthenticated());
      return;
    }

    await _analytics.logEvent(
      'sign_up',
      parameters: {'method': 'email', 'user_id': firebaseUser.uid},
      userId: firebaseUser.uid,
    );

    if (!firebaseUser.emailVerified) {
      // D2 HARD GATE: park in the explicit verification state. The pending
      // username travels with the intent so the post-verification exchange
      // completes registration without asking again.
      _pendingVerificationIntent = PendingEmailVerificationIntent(
        email: firebaseUser.email ?? '',
        firebaseUid: firebaseUser.uid,
        username: trimmedUsername,
      );
      _logger.info(
        '[AUTH] Signup verification email sent — waiting for verification '
        'before exchange',
        extra: {'uid': firebaseUser.uid},
      );
      _publishIfCurrent(
        _beginHydrationRequest(),
        AuthState.pendingEmailVerification(
          email: firebaseUser.email ?? '',
          username: trimmedUsername,
        ),
      );
      return;
    }

    // Already verified (e.g. re-provisioned identity): go straight to the
    // canonical exchange.
    _authIntent = EmailSignupIntent(trimmedUsername);
    await _syncWithBackend(
      firebaseUser.uid,
      firebaseUser,
      isEmailSignup: true,
    );
    // _authIntent cleared in _syncWithBackend (keep for retry on USERNAME_TAKEN)
  }

  /// True while an email signup exchange is mid-flight with a pending
  /// registration username.
  bool get hasPendingRegistration => _authIntent is EmailSignupIntent;

  /// Retry the authenticated exchange with a corrected registration username.
  ///
  /// Stage 1C — Part B recovery. When the backend rejects the first username
  /// choice (USERNAME_TAKEN / USERNAME_RESERVED), the Firebase account already
  /// exists, so re-running `signUpWithEmail` would hit EMAIL_ALREADY_IN_USE.
  /// Instead this re-runs ONLY the authenticated exchange with the corrected
  /// username — the Firebase account is never recreated and no session is
  /// broken. The canonical backend assigns the corrected username exactly once
  /// (Stage 1A), then a full session / authenticated state is emitted.
  ///
  /// D2: if the email is somehow still unverified, the corrected username is
  /// parked into the pending-verification intent instead of being exchanged.
  Future<void> retryRegistrationUsername(String normalizedUsername) async {
    final trimmed = normalizedUsername.trim();
    if (trimmed.isEmpty) {
      _setState(const AuthState.error('Username is required'));
      return;
    }

    _lastRegistrationUsernameError = null;

    final firebaseUser = activeFirebaseUser;
    if (firebaseUser == null) {
      _setState(const AuthState.unauthenticated());
      return;
    }

    if (!firebaseUser.emailVerified) {
      _pendingVerificationIntent = PendingEmailVerificationIntent(
        email: firebaseUser.email ?? '',
        firebaseUid: firebaseUser.uid,
        username: trimmed,
      );
      _publishIfCurrent(
        _beginHydrationRequest(),
        AuthState.pendingEmailVerification(
          email: firebaseUser.email ?? '',
          username: trimmed,
        ),
      );
      return;
    }

    _authIntent = EmailSignupIntent(trimmed);
    _syncedUserId = null;

    _logger.log(
      '[AUTH] Retrying registration username: $trimmed',
      level: LogLevel.debug,
    );

    await _syncWithBackend(
      firebaseUser.uid,
      firebaseUser,
      isEmailSignup: true,
    );
  }

  /// Sign out user
  ///
  /// IMPORTANT: FCM cleanup is done BEFORE Firebase Auth sign out
  /// to ensure we can delete the token from Firestore while user is authenticated.
  /// This prevents notifications from going to wrong user after account switch.
  ///
  /// 🔒 DETERMINISTIC FIX: Reset sync locks on logout to ensure
  /// next login session starts fresh without stale sync state.
  Future<void> signOut() async {
    // SECURITY FIX: Stop session validation timer
    _stopSessionValidation();

    final currentState = state;

    // 1. Attempt backend logout BEFORE any local cleanup removes tokens.
    if (currentState is AuthStateAuthenticated) {
      try {
        // AUTH-2 (CREDENTIAL AUTHORITY): read via the canonical credential
        // boundary (readLabudaRefreshToken), not the legacy getRefreshToken().
        final refreshResult = await _localStorage.readLabudaRefreshToken();
        final refreshToken = refreshResult.data?.trim();
        final fcmService = ref.read(fcmServiceProvider);
        final fcmToken = fcmService.fcmToken?.trim();
        String? deviceId;
        if (fcmToken == null || fcmToken.isEmpty) {
          final deviceIdResult = await _localStorage.getString(
            StorageKeys.deviceId,
          );
          deviceId = deviceIdResult.data?.trim();
        }

        if (refreshToken != null && refreshToken.isNotEmpty) {
          final result = await _authRepository.logoutCurrentSession(
            refreshToken: refreshToken,
            fcmToken: fcmToken != null && fcmToken.isNotEmpty ? fcmToken : null,
            deviceId: deviceId != null && deviceId.isNotEmpty ? deviceId : null,
          );

          if (result.isError) {
            await _logger.warning(
              'Backend logout failed; local logout will continue',
              extra: {'error': result.error},
            );
          }
        } else {
          await _logger.warning(
            'Backend logout skipped because refresh token was unavailable',
            extra: {'userId': currentState.user.id},
          );
        }
      } catch (e) {
        await _logger.warning(
          'Backend logout failed; local logout will continue',
          extra: {'error': e.toString()},
        );
      }
    }

    // Phase 3E: Clear local Labuda credential via canonical abstraction (fail-closed even if server logout failed)
    try {
      await _localStorage.clearLabudaCredential();
    } catch (_) {}

    // 2. Cleanup FCM BEFORE sign out (while user is still authenticated)
    if (currentState is AuthStateAuthenticated) {
      try {
        final fcmService = ref.read(fcmServiceProvider);
        await fcmService.cleanup(userId: currentState.user.id);
      } catch (e) {
        // Log but don't block sign out
        await _logger.warning(
          'FCM cleanup failed during sign out',
          extra: {'error': e.toString()},
        );
      }
    }

    // Presence is server-derived via WS lease; no mobile writer needed.

    // Tier 4 (Runtime Honesty): close the WebSocket on logout so the
    // session no longer holds a connection authenticated with the
    // about-to-be-invalid Firebase token. Without this, the WS stays
    // open with a stale token and any subsequent send would either
    // succeed under the wrong identity or be silently rejected by the
    // server. Best-effort and bounded: a failure here must not block
    // logout. Treated symmetrically to the FCM cleanup above.
    try {
      final ws = ref.read(webSocketServiceProvider);
      await ws.disconnect().timeout(const Duration(seconds: 3));
    } catch (e) {
      await _logger.warning(
        'WebSocket disconnect failed during sign out — '
        'connection may linger until OS reaps it',
        extra: {'error': e.toString()},
      );
    }

    // 3. Track logout event (before auth state changes)
    if (currentState is AuthStateAuthenticated) {
      await _analytics.logEvent(
        'logout',
        parameters: {'user_id': currentState.user.id},
        userId: currentState.user.id,
      );
    }

    // 4. Reset sync locks - critical for deterministic flow on next login
    _syncedUserId = null;
    _ongoingSync = null;
    _pendingVerificationIntent = null;

    // 5. Proceed with Firebase Auth sign out
    final result = await _authRepository.signOut();

    if (result.isSuccess) {
      _setState(const AuthState.unauthenticated());
    } else {
      _setState(AuthState.error(result.error!));
    }
  }

  /// Sign out from all devices (logout-all).
  Future<void> signOutAll() async {
    _stopSessionValidation();
    final currentState = state;
    if (currentState is AuthStateAuthenticated) {
      try {
        final result = await _authRepository.logoutAllSessions();
        if (result.isError) {
          await _logger.warning('Backend logout-all failed; local logout will continue', extra: {'error': result.error});
        }
      } catch (e) {
        await _logger.warning('Backend logout-all failed; local logout will continue', extra: {'error': e.toString()});
      }
    }
    try {
      await _localStorage.clearLabudaCredential();
    } catch (_) {}
    try {
      final ws = ref.read(webSocketServiceProvider);
      await ws.disconnect().timeout(const Duration(seconds: 3));
    } catch (_) {}
    if (currentState is AuthStateAuthenticated) {
      await _analytics.logEvent('logout', parameters: {'user_id': currentState.user.id, 'all_devices': true}, userId: currentState.user.id);
    }
    _syncedUserId = null;
    _ongoingSync = null;
    _pendingVerificationIntent = null;
    final result = await _authRepository.signOut();
    if (result.isSuccess) {
      _setState(const AuthState.unauthenticated());
    } else {
      _setState(AuthState.error(result.error!));
    }
  }

  /// Activate WebSocket connection after successful
  /// backend sync. Called once per login; idempotent on re-entry (WS guards
  /// duplicate connect, presence handles same-user no-op).
  ///
  /// Fire-and-forget: failures are logged but NEVER block the auth flow.
  void _activateRealtimeServices(String userId, User firebaseUser) {
    // Phase 5: WebSocket uses Labuda access JWT (canonical). No Firebase fallback.
    // Install Labuda token provider for future reconnects, then connect with current Labuda token.
    try {
      final ws = ref.read(webSocketServiceProvider);
      ws.setLabudaTokenProvider(() async {
        final res = await _localStorage.readLabudaAccessToken();
        return res.data?.trim();
      });
    } catch (_) {}
    unawaited(
      Future<void>(() async {
        try {
          final res = await _localStorage.readLabudaAccessToken();
          final token = res.data?.trim();
          if (token == null || token.isEmpty) {
            _logger.log('[AUTH] WebSocket connect skipped — Labuda access missing (no Firebase fallback)', level: LogLevel.debug);
            return;
          }
          final ws = ref.read(webSocketServiceProvider);
          await ws.connect(token);
        } catch (e) {
          _logger.log(
            '[AUTH] WebSocket connect failed (non-fatal): $e',
            level: LogLevel.warning,
          );
        }
      }),
    );

  }

  /// Degraded manual recovery: retry backend sync using the current
  /// Firebase identity. This is the sole recovery producer for
  /// AuthStateBackendUnavailable / AuthStateBackendFailure (no automatic
  /// timer). Called by the Splash degraded UI "Coba Lagi".
  ///
  /// D2: an unverified session is routed to the verify-email screen by the
  /// hard gate — never to the exchange.
  Future<void> retryBackendSync() async {
    final firebaseUser = activeFirebaseUser;
    if (firebaseUser == null) {
      _setState(const AuthState.unauthenticated());
      return;
    }

    _logger.info('Manual retry triggered for backend sync');

    if (!_mayExchangeVerifiedEmail(firebaseUser)) return;

    // Clear sync flag to force fresh sync
    _syncedUserId = null;

    // Trigger backend sync
    _syncWithBackend(firebaseUser.uid, firebaseUser, isEmailSignup: false);
  }

  /// Verify-screen resend authority (D2): sends the verification email for
  /// the CURRENT pending-verification session. Returns true when Firebase
  /// accepted the send; false otherwise (error state is set for display).
  /// This is the ONLY resend path in the app — the profile resend path is
  /// dead under the hard gate (an authenticated user is always verified).
  Future<bool> resendVerificationEmail() async {
    final result = await _authRepository.sendEmailVerification();
    if (result.isError) {
      _logger.warning(
        '[AUTH] Resend verification email failed',
        extra: {'error': result.error},
      );
      return false;
    }
    return true;
  }

  /// Change password for current user
  Future<bool> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async {
    final result = await _authRepository.changePassword(
      currentPassword: currentPassword,
      newPassword: newPassword,
    );

    if (result.isError) {
      _setState(AuthState.error(result.error!));
    }

    return result.isSuccess;
  }

  /// Update user profile
  Future<bool> updateProfile({
    String? photoUrl,
    String? username,
    String? bio,
    String? phoneNumber,
    String? location,
    DateTime? phoneVerifiedAt,
    DateTime? dateOfBirth,
  }) async {
    final result = await _authRepository.updateProfile(
      photoUrl: photoUrl,
      username: username,
      bio: bio,
      phoneNumber: phoneNumber,
      location: location,
      phoneVerifiedAt: phoneVerifiedAt,
      dateOfBirth: dateOfBirth,
    );

    if (result.isSuccess) {
      // Preserve current emailVerified flag — profile update does not affect
      // email-verification status.
      await forceRefreshAuthState();
      return true;
    } else {
      _setState(AuthState.error(result.error!));
      return false;
    }
  }

  /// Complete the profile after restricted Firebase exchange.
  ///
  /// CANONICAL USERNAME AUTHORITY: backend rejections of the chosen username
  /// (USERNAME_TAKEN / USERNAME_RESERVED / USERNAME_INVALID_FORMAT / 409 race)
  /// are returned as [ProfileCompletionOutcome.usernameRejected] so the
  /// completion surface renders them INLINE without mutating the global auth
  /// state — the user stays on the correction screen instead of being routed
  /// away to the login flow.
  Future<ProfileCompletionOutcome> completeProfile({
    required String username,
  }) async {
    final currentState = state;
    if (currentState is! AuthStateRequiresProfileCompletion) {
      _setState(AuthState.error('Invalid authentication state'));
      return const ProfileCompletionOutcome.failure(
        'Invalid authentication state',
      );
    }

    final result = await _authRepository.completeProfile(username: username);

    if (result.isSuccess && result.data != null) {
      final firebaseUser = activeFirebaseUser;
      if (firebaseUser == null) {
        _setState(const AuthState.unauthenticated());
        return const ProfileCompletionOutcome.failure(
          'Your session has ended. Please sign in again.',
        );
      }

      final completedUser = _canonicalizeBackendUser(result.data!);

      final completedStatus =
          completedUser.accountStatus ?? AccountStatus.active;
      if (completedStatus.isRestricted) {
        _setState(
          AuthState.accountRestricted(
            completedUser,
            restrictionType: completedStatus,
          ),
        );
        return const ProfileCompletionOutcome.success();
      }
      _authIntent = null;
      _pendingVerificationIntent = null;
      _syncedUserId = firebaseUser.uid;

      _publishAuthenticatedIfCurrent(
        _beginHydrationRequest(),
        completedUser,
        emailVerified: firebaseUser.emailVerified,
      );

      _startSessionValidation();
      _activateRealtimeServices(completedUser.id, firebaseUser);
      await _analytics.logEvent(
        'login',
        parameters: {
          'method': 'profile_completion',
          'user_id': completedUser.id,
        },
        userId: completedUser.id,
      );

      return const ProfileCompletionOutcome.success();
    }

    final errorCode = result.errorCode;
    final error = result.error ?? 'Failed to complete profile';

    if (errorCode == 'SESSION_USER_MISMATCH') {
      _setState(AuthState.error(error));
      return ProfileCompletionOutcome.failure(error);
    }

    if (errorCode == 'PROFILE_ALREADY_COMPLETED' ||
        errorCode == 'INVALID_TOKEN' ||
        errorCode == 'INVALID_SCOPE' ||
        errorCode == 'TOKEN_EXPIRED' ||
        error.contains('already completed') ||
        error.contains('invalid token') ||
        error.contains('expired')) {
      await refreshAuthState();
      return const ProfileCompletionOutcome.success();
    }

    // CANONICAL USERNAME AUTHORITY — backend rejected the chosen username.
    // Presentational only: same message mapping as the registration form,
    // no global state mutation — the user stays here to correct it.
    final usernameError = registrationUsernameErrorMessage(errorCode);
    if (usernameError != null) {
      _logger.warning(
        'Complete-profile username rejected by backend',
        extra: {'errorCode': errorCode},
      );
      return ProfileCompletionOutcome.usernameRejected(usernameError);
    }

    // 409 race fallback: the username passed pre-checks but lost the unique
    // race between check and commit. Same inline surface as above.
    if (result.statusCode == 409) {
      return const ProfileCompletionOutcome.usernameRejected(
        'Username ini baru saja diambil orang lain. Silakan pilih yang lain.',
      );
    }

    final errorKind = classifyAuthSyncError(
      error,
      errorCode: errorCode,
      statusCode: result.statusCode,
    );
    if (errorKind == AuthSyncErrorKind.backendUnavailable) {
      _logger.log(
        '[AUTH] Preserving RequiresProfileCompletion state after transient failure',
        level: LogLevel.warning,
      );
      return ProfileCompletionOutcome.failure(
        'Tidak dapat terhubung ke server. Periksa koneksi lalu coba lagi.',
      );
    }

    // Generic unexpected failure (restricted token invalid/expired, etc.):
    // state preserved — the surface shows the message and keeps the Sign Out
    // escape hatch. Never a global error kick to the login flow.
    _logger.warning(
      'Complete-profile failed without state change',
      extra: {'error': error, 'errorCode': errorCode},
    );
    return ProfileCompletionOutcome.failure(error);
  }

  /// Reset password via email
  Future<bool> resetPassword({required String email}) async {
    final result = await _authRepository.resetPassword(email: email);

    if (result.isError) {
      _setState(AuthState.error(result.error!));
    }

    return result.isSuccess;
  }

  /// Clear error state
  void clearError() {
    if (state is AuthStateError) {
      _setState(const AuthState.unauthenticated());
    }
  }

  /// Refresh auth state after a Firebase user change that requires a full
  /// backend resync.
  ///
  /// 🔐 CRITICAL: RELOAD Firebase user to get fresh data
  /// This ensures external verification (e.g., email link) is detected.
  ///
  /// D2: an unverified session is routed to the verify-email screen by the
  /// hard gate — never to the exchange.
  Future<void> refreshAuthState() async {
    try {
      // 🔐 CRITICAL: RELOAD Firebase user before checking status
      await activeFirebaseUser?.reload();
      final firebaseUser = activeFirebaseUser;
      if (firebaseUser == null) {
        _setState(const AuthState.unauthenticated());
        return;
      }

      // Clear sync flag to force fresh sync
      _syncedUserId = null;

      if (!_mayExchangeVerifiedEmail(firebaseUser)) return;

      // Trigger full backend sync with refreshed user
      // This handles the flow: firebaseAuthenticated → syncingWithBackend → authenticated
      _syncWithBackend(firebaseUser.uid, firebaseUser, isEmailSignup: false);
    } catch (e) {
      _logger.error(
        'Failed to refresh auth state',
        extra: {'error': e.toString()},
      );
      // Continue with stale user data on reload failure
      final firebaseUser = activeFirebaseUser;
      if (firebaseUser == null) {
        _setState(const AuthState.unauthenticated());
        return;
      }
      _syncedUserId = null;
      if (!_mayExchangeVerifiedEmail(firebaseUser)) return;
      _syncWithBackend(firebaseUser.uid, firebaseUser, isEmailSignup: false);
    }
  }

  /// Force refresh auth state from backend API
  /// SOURCE OF TRUTH: PostgreSQL (Backend API /users/me)
  ///
  /// 🔧 FIX: Allow refresh from RequiresProfileCompletion state
  /// This enables navigation after profile completion
  Future<void> forceRefreshAuthState() async {
    final currentState = state;
    final requestGeneration = _beginHydrationRequest();

    // Allow refresh from Authenticated or RequiresProfileCompletion states
    final canRefresh =
        currentState is AuthStateAuthenticated ||
        currentState is AuthStateRequiresProfileCompletion;

    if (!canRefresh) {
      _logger.log(
        '[AUTH] forceRefreshAuthState skipped - invalid state: ${currentState.runtimeType}',
        level: LogLevel.warning,
      );
      return;
    }

    // Clear sync flag to force fresh data fetch
    _syncedUserId = null;

    try {
      // Get current Firebase user
      final firebaseUser = activeFirebaseUser;
      if (firebaseUser == null) {
        _setState(const AuthState.unauthenticated());
        return;
      }

      // Don't show loading for RequiresProfileCompletion -> keep UI stable
      final shouldShowLoading = currentState is AuthStateAuthenticated;
      if (shouldShowLoading) {
        _setState(
          AuthState.loading(
            principal: FirebasePrincipal.fromFirebaseUser(firebaseUser),
          ),
        );
      }

      // SOURCE OF TRUTH: Get fresh user data from backend API (PostgreSQL)
      final result = await _userSyncService!.getCurrentUser().timeout(
        const Duration(seconds: 15),
        onTimeout: () {
          throw Exception('GET USER TIMEOUT');
        },
      );

      if (result.isSuccess && result.data != null) {
        final completeUser = _canonicalizeBackendUser(result.data!);
        await _logger.debugGetCurrentUserSuccess(
          completeUser.id,
          completeUser.isEmailVerified,
        );

        if (!_isCurrentHydrationRequest(requestGeneration)) {
          return;
        }

        // Emit Authenticated state - router will navigate to Home.
        // Pull emailVerified directly from Firebase user — forceRefresh is the
        // path used after Complete Profile, where the flag is canonical.
        _publishAuthenticatedIfCurrent(
          requestGeneration,
          completeUser,
          emailVerified: firebaseUser.emailVerified,
        );
        _syncedUserId = firebaseUser.uid;

        // Activate WS + Presence for the complete-profile→authenticated path.
        // Idempotent: safe if already connected from primary login path.
        _activateRealtimeServices(completeUser.id, firebaseUser);

        await _logger.log(
          '[AUTH] State → Authenticated (after profile completion)',
          level: LogLevel.info,
        );
      } else {
        // Backend API error - preserve current state for profile completion flow
        final error = result.error ?? 'Failed to refresh user data';
        await _logger.error('[AUTH] Refresh failed: $error');
        final errorKind = classifyAuthSyncError(
          error,
          errorCode: result.errorCode,
          statusCode: result.statusCode,
        );

        // For RequiresProfileCompletion, stay in that state (don't show error)
        // For Authenticated, classify the refresh failure so degraded
        // backend issues do not blow away the cached seller/user state.
        if (errorKind == AuthSyncErrorKind.identityInvalid ||
            errorKind == AuthSyncErrorKind.accountDeleted) {
          if (!_isCurrentHydrationRequest(requestGeneration)) {
            return;
          }
          await performFirebaseSignOut();
          _setState(const AuthState.unauthenticated());
        } else if (currentState is AuthStateRequiresProfileCompletion &&
            (errorKind == AuthSyncErrorKind.backendUnavailable ||
                errorKind == AuthSyncErrorKind.backendFailure)) {
          _logger.log(
            '[AUTH] Preserving RequiresProfileCompletion state after refresh failure',
            level: LogLevel.warning,
          );
          // State already RequiresProfileCompletion, no change needed
        } else if (errorKind == AuthSyncErrorKind.backendUnavailable) {
          _publishIfCurrent(
            requestGeneration,
            AuthState.backendUnavailable(error),
          );
        } else if (errorKind == AuthSyncErrorKind.backendFailure) {
          _publishIfCurrent(requestGeneration, AuthState.backendFailure(error));
        } else {
          _publishIfCurrent(requestGeneration, AuthState.error(error));
        }
      }
    } catch (e, stackTrace) {
      await _logger.error(
        '[AUTH] forceRefreshAuthState error: $e',
        extra: {'stackTrace': stackTrace.toString()},
      );

      // Fallback to previous state on error
      if (!_isCurrentHydrationRequest(requestGeneration)) {
        return;
      }
      _setState(currentState);
    }
  }

  /// Reset state to initial
  ///
  /// 🔒 DETERMINISTIC FIX: Reset all sync locks when resetting controller.
  /// This ensures clean state for testing or edge cases.
  void reset() {
    // 🔐 AUTH PERSISTENCE FIX: Cancel stream subscription when resetting
    _authStateSubscription?.cancel();
    _authStateSubscription = null;

    // 🔒 SYNC LOCK RESET: Clear all sync state
    _beginHydrationRequest();
    _syncedUserId = null;
    _ongoingSync = null;
    _pendingVerificationIntent = null;
    _setState(const AuthState.initial());
  }

  /// Deactivate user account with reason
  Future<bool> deactivateAccount({
    required String userId,
    required String reason,
  }) async {
    final result = await _authRepository.deactivateAccount(
      userId: userId,
      reason: reason,
    );

    if (result.isSuccess) {
      // Track account deactivation
      await _analytics.logEvent(
        'account_deactivated',
        parameters: {'user_id': userId, 'reason': reason},
        userId: userId,
      );

      // Sign out after deactivation
      await signOut();
      return true;
    } else {
      _setState(AuthState.error(result.error!));
      return false;
    }
  }

  /// Permanently delete the authenticated user's account.
  ///
  /// Calls backend soft-delete → Firebase credential delete → local signOut.
  /// Returns the error string on failure (null on success).
  Future<String?> deleteAccount() async {
    final result = await _authRepository.deleteAccount();
    if (result.isSuccess) {
      await signOut();
      return null;
    }
    return result.error ?? 'Failed to delete account';
  }

  /// 🔒 SECURITY FIX: Start periodic session validation
  void _startSessionValidation() {
    // Cancel existing timer if any
    _sessionValidationTimer?.cancel();

    // Start new timer - validate every 5 minutes
    _sessionValidationTimer = Timer.periodic(const Duration(minutes: 5), (
      _,
    ) async {
      await _validateSession();
    });
  }

  /// 🔒 SECURITY FIX: Stop periodic session validation
  void _stopSessionValidation() {
    _sessionValidationTimer?.cancel();
    _sessionValidationTimer = null;
  }

  /// W14-B2: Public method to refresh user data (including roles) on app resume
  /// Can be called from app lifecycle handlers to ensure role changes are reflected
  Future<void> refreshUserData() async {
    final currentState = state;
    if (currentState is! AuthStateAuthenticated) {
      // Not authenticated, nothing to refresh
      return;
    }

    final requestGeneration = _beginHydrationRequest();

    try {
      // Get fresh user data from backend
      final result = await _userSyncService!.getCurrentUser();

      if (result.isSuccess && result.data != null) {
        final freshUser = _canonicalizeBackendUser(result.data!);

        // ID1F: Account restriction gate — mid-session suspension/ban on resume
        final freshStatus = freshUser.accountStatus ?? AccountStatus.active;
        if (freshStatus.isRestricted) {
          _logger.warning(
            '[RESUME] Account restricted: ${freshStatus.apiValue}',
          );
          _stopSessionValidation();
          _publishIfCurrent(
            requestGeneration,
            AuthState.accountRestricted(
              freshUser,
              restrictionType: freshStatus,
            ),
          );
          return;
        }

        // PASS 2A / F2: compare the WHOLE fresh user against the cached
        // one, not just `.role`. AuthUser extends Equatable (via
        // BaseEntity) over every backend-authoritative field — role,
        // accountStatus, hasMarketAuthority, hasSellerProfile,
        // sellerSubscriptionStatus, sellerTier, penalty points,
        // verification flags — so this single `!=` check both (a) catches
        // authority-relevant changes that don't touch role at all (e.g. a
        // seller subscription expiring mid-session, which flips
        // hasMarketAuthority but never touches roles) and (b) is a no-op
        // when the user is genuinely unchanged, avoiding unnecessary
        // rebuilds.
        if (freshUser != currentState.user) {
          _logger.info(
            'Authority-relevant user data changed on resume: '
            'role ${currentState.user.role} → ${freshUser.role}, '
            'hasMarketAuthority ${currentState.user.hasMarketAuthority} → '
            '${freshUser.hasMarketAuthority}',
          );
          // Update state with new user data so the router / SellerGuard /
          // permission gates observe the change immediately instead of
          // holding a stale cached AuthUser until the next full login sync.
          // Preserve current emailVerified flag — this is the resume hook,
          // not the email-verification refresh flow.
          _publishAuthenticatedIfCurrent(
            requestGeneration,
            freshUser,
            emailVerified: currentState.emailVerified,
          );
        }
      } else {
        final error = result.error ?? 'Failed to refresh user data';
        final errorKind = classifyAuthSyncError(
          error,
          errorCode: result.errorCode,
          statusCode: result.statusCode,
        );

        if (!_isCurrentHydrationRequest(requestGeneration)) {
          return;
        }

        if (errorKind == AuthSyncErrorKind.identityInvalid ||
            errorKind == AuthSyncErrorKind.accountDeleted) {
          await performFirebaseSignOut();
          _setState(const AuthState.unauthenticated());
        } else if (errorKind == AuthSyncErrorKind.backendUnavailable) {
          _publishIfCurrent(
            requestGeneration,
            AuthState.backendUnavailable(error),
          );
        } else if (errorKind == AuthSyncErrorKind.backendFailure) {
          _publishIfCurrent(requestGeneration, AuthState.backendFailure(error));
        }
      }
    } catch (e) {
      _logger.error(
        'User data refresh error on resume',
        extra: {'error': e.toString()},
      );
      // Silently ignore - network issues or temporary problems
    }
  }

  /// 🔒 SECURITY FIX: Validate current session and refresh roles
  /// W14-B2: Enhanced to also refresh user data which includes role changes
  Future<void> _validateSession() async {
    try {
      final currentState = state;
      if (currentState is! AuthStateAuthenticated) {
        // Not authenticated, stop validation
        _stopSessionValidation();
        return;
      }

      final requestGeneration = _beginHydrationRequest();

      // 🔐 CRITICAL: RELOAD Firebase user to get fresh data
      final firebaseUser = activeFirebaseUser;
      if (firebaseUser == null) {
        // Firebase session lost, sign out
        _logger.warning('Firebase session lost during validation');
        if (_isCurrentHydrationRequest(requestGeneration)) {
          await signOut();
        }
        return;
      }

      try {
        await firebaseUser.reload();
      } catch (e) {
        _logger.error(
          'Failed to reload Firebase user during session validation',
          extra: {'error': e.toString()},
        );
        // Continue validation even if reload fails
      }

      // Check if user still exists and refresh user data (including roles)
      final result = await _userSyncService!.getCurrentUser();

      if (result.isError || result.data == null) {
        final error = result.error ?? 'Session validation failed';
        final errorKind = classifyAuthSyncError(
          error,
          errorCode: result.errorCode,
          statusCode: result.statusCode,
        );

        if (!_isCurrentHydrationRequest(requestGeneration)) {
          return;
        }

        if (errorKind == AuthSyncErrorKind.identityInvalid ||
            errorKind == AuthSyncErrorKind.accountDeleted) {
          _logger.warning('Session validation failed, signing out user');
          if (_isCurrentHydrationRequest(requestGeneration)) {
            await signOut();
          }
        } else if (errorKind == AuthSyncErrorKind.backendUnavailable) {
          _publishIfCurrent(
            requestGeneration,
            AuthState.backendUnavailable(error),
          );
        } else if (errorKind == AuthSyncErrorKind.backendFailure) {
          _publishIfCurrent(requestGeneration, AuthState.backendFailure(error));
        }
      } else {
        final freshUser = _canonicalizeBackendUser(result.data!);

        // ID1F: Account restriction gate — mid-session suspension/ban
        // Priority over role change: restricted user must be redirected
        // regardless of any other state changes.
        final freshStatus = freshUser.accountStatus ?? AccountStatus.active;
        if (freshStatus.isRestricted) {
          _logger.warning(
            '[VALIDATE] Account restricted mid-session: ${freshStatus.apiValue}',
          );
          _stopSessionValidation();
          _publishIfCurrent(
            requestGeneration,
            AuthState.accountRestricted(
              freshUser,
              restrictionType: freshStatus,
            ),
          );
          return;
        }

        // W14-B2: Session valid - update state with fresh user data
        // This ensures role changes are reflected without requiring re-login
        //
        // PASS 2A / F2: compare the WHOLE fresh user (Equatable over every
        // backend-authoritative field), not just `.role` — otherwise a
        // seller subscription expiring mid-session flips
        // hasMarketAuthority/sellerSubscriptionStatus without ever
        // changing role, and the stale cached AuthUser (still
        // hasMarketAuthority=true) keeps being read by SellerGuard/the
        // router's seller guard for up to the full 5-minute period between
        // validations. See refreshUserData() above for the identical fix
        // on the resume path.
        if (freshUser != currentState.user) {
          _logger.info(
            'Authority-relevant user data changed during periodic '
            'validation: role ${currentState.user.role} → ${freshUser.role}, '
            'hasMarketAuthority ${currentState.user.hasMarketAuthority} → '
            '${freshUser.hasMarketAuthority}',
          );
          // Refresh emailVerified from Firebase — the periodic validation is
          // also a natural place to pick up an out-of-band verification.
          final freshEmailVerified =
              firebaseUser.emailVerified || currentState.emailVerified;
          _publishAuthenticatedIfCurrent(
            requestGeneration,
            freshUser,
            emailVerified: freshEmailVerified,
          );
        }
      }
    } catch (e) {
      // ✅ FIXED: Don't force sign out on validation error
      // Could be network issue or temporary problem
      _logger.error('Session validation error', extra: {'error': e.toString()});
      // Just log the error and continue - Firebase will handle auth state
    }
  }

  /// Refresh the hydrated account after Firebase has already proven that the
  /// email address is verified.
  ///
  /// This is the verification-only path: it keeps the current authenticated
  /// shell intact on transient backend failures and only updates the hydrated
  /// account when the same principal can be refreshed successfully.
  Future<bool> refreshVerifiedEmailAccount() async {
    final currentState = state;
    if (currentState is! AuthStateAuthenticated) {
      _logger.log(
        '[AUTH] refreshVerifiedEmailAccount skipped - invalid state: '
        '${currentState.runtimeType}',
        level: LogLevel.warning,
      );
      return false;
    }

    final firebaseUser = activeFirebaseUser;
    if (firebaseUser == null) {
      _logger.warning(
        '[AUTH] refreshVerifiedEmailAccount skipped - no Firebase user',
      );
      return false;
    }

    if (!firebaseUser.emailVerified) {
      _logger.log(
        '[AUTH] refreshVerifiedEmailAccount skipped - Firebase still '
        'unverified',
        level: LogLevel.debug,
      );
      return false;
    }

    final requestGeneration = _beginHydrationRequest();

    try {
      final result = await _userSyncService!.getCurrentUser().timeout(
        const Duration(seconds: 15),
        onTimeout: () {
          throw Exception('GET USER TIMEOUT');
        },
      );

      if (!_isCurrentHydrationRequest(requestGeneration)) {
        return false;
      }

      if (result.isSuccess && result.data != null) {
        final freshUser = _canonicalizeBackendUser(result.data!);
        final freshStatus = freshUser.accountStatus ?? AccountStatus.active;

        if (freshUser.id != currentState.user.id) {
          _logger.warning(
            '[AUTH] Verified email refresh hit backend user mismatch: '
            'current=${currentState.user.id}, fresh=${freshUser.id}',
          );
          await performFirebaseSignOut();
          _setState(const AuthState.unauthenticated());
          return false;
        }

        if (freshStatus.isRestricted) {
          _logger.warning(
            '[AUTH] Verified email refresh hit restricted account: '
            '${freshStatus.apiValue}',
          );
          _stopSessionValidation();
          _publishIfCurrent(
            requestGeneration,
            AuthState.accountRestricted(
              freshUser.copyWith(isEmailVerified: true),
              restrictionType: freshStatus,
            ),
          );
          return true;
        }

        final mergedUser = freshUser.copyWith(isEmailVerified: true);
        _publishAuthenticatedIfCurrent(
          requestGeneration,
          mergedUser,
          emailVerified: true,
        );
        return true;
      }

      final error = result.error ?? 'Failed to refresh verified email';
      final errorKind = classifyAuthSyncError(
        error,
        errorCode: result.errorCode,
        statusCode: result.statusCode,
      );

      if (errorKind == AuthSyncErrorKind.identityInvalid ||
          errorKind == AuthSyncErrorKind.accountDeleted) {
        await performFirebaseSignOut();
        _setState(const AuthState.unauthenticated());
      } else {
        _logger.warning(
          '[AUTH] Verified email refresh failed without state change: $error',
        );
      }

      return false;
    } catch (e, stackTrace) {
      await _logger.error(
        '[AUTH] refreshVerifiedEmailAccount error: $e',
        extra: {'stackTrace': stackTrace.toString()},
      );
      return false;
    }
  }
}

/// Provider untuk AuthController dengan dependency injection
final authControllerProvider = NotifierProvider<AuthController, AuthState>(
  AuthController.new,
);

/// Provider untuk IAuthRepository
/// Digunakan untuk operasi user lookup seperti searchUsers, getUserById
///
/// R4.3: Now delegates to data layer provider instead of constructing datasource inline.
/// Data layer placement is the canonical source for repository construction.
final authRepositoryProvider = Provider<IAuthRepository>((ref) {
  return ref.watch(auth_data.authRepositoryProvider);
});
