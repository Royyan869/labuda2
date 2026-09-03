import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:labuda/core/core.dart';
import '../../domain/entities/account_status.dart';
import '../../domain/entities/firebase_principal.dart';
// R4.3: Import data layer providers instead of constructing inline
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data
    show authRepositoryProvider;
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show userSyncServiceProvider;
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';
import '../providers/auth_sign_in_service.dart';
import '../providers/auth_sign_up_service.dart';
import '../providers/auth_profile_service.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart'
    show fcmServiceProvider;


/// PASS 2A / F1 â€” structured classification of a failed backend auth-sync
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
  /// â€” force signOut.
  identityInvalid,

  /// The account no longer exists (backend `ACCOUNT_DELETED`, HTTP 403).
  /// Force signOut â€” there is nothing left to sync against.
  accountDeleted,

  /// The account exists but is suspended/banned/inactive (backend
  /// `ACCOUNT_INACTIVE`, HTTP 403). This is NOT a transient backend outage
  /// and must not be auto-retried; it also does not force a Firebase
  /// signOut (mirrors the mid-session [AuthStateAccountRestricted] gate
  /// elsewhere, which likewise never signs the user out).
  accountInactive,

  /// Transient network/server issue (timeout, 5xx, no connection) â€” safe
  /// to auto-retry with backoff.
  backendUnavailable,

  /// Backend rejected the request for another reason (validation, business
  /// rule, etc). Not retryable automatically, but not an identity or
  /// availability problem either.
  backendFailure,
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

/// ðŸ”’ ERROR CLASSIFICATION (fallback, free-text): Determine if error is
/// identity-related. Used only when no structured `errorCode` is available
/// â€” see [classifyAuthSyncError] for the structured-first classification.
bool isIdentityErrorMessage(String? error) {
  if (error == null) return false;
  final lower = error.toLowerCase();

  // Identity errors - user's Firebase session is invalid
  return lower.contains('invalid token') ||
      lower.contains('token_expired') ||
      lower.contains('no user record') ||
      lower.contains('firebase_auth/unknown') ||
      lower.contains('user-not-found') ||
      lower.contains('user deleted') ||
      lower.contains('auth/invalid-credential');
}

/// ðŸ”’ ERROR CLASSIFICATION (fallback, free-text): Determine if error is
/// backend unavailable. Used only when no structured `errorCode`/
/// `statusCode` is available â€” see [classifyAuthSyncError].
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

/// Maps a backend registration-username error code to a user-facing message
/// for the registration screen. Returns `null` when the code is not one of the
/// canonical username rejections (USERNAME_TAKEN / USERNAME_RESERVED /
/// USERNAME_INVALID_FORMAT / USERNAME_IMMUTABLE), so the caller falls through
/// to generic sync-error classification.
///
/// Backend remains the final authority for these outcomes (Stage 1A contract);
/// this only converts the machine-readable code into presentational text.
String? _mapRegistrationUsernameError(String? errorCode) {
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
/// 1. AUTH: Firebase login/signup/signout, OAuth
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
/// **âœ… NEW SIMPLIFIED EMAIL VERIFICATION FLOW:**
/// - Signup â†’ Verify email (outside app) â†’ Login â†’ DONE
/// - NO blocking for unverified emails - backend enforces verification
/// - Users can sign in with unverified emails but functionality is limited
///
/// **STATE MACHINE FLOW (Prevents Premature Routing):**
/// 1. AuthStateInitial â†’ Initial state
/// 2. AuthStateFirebaseAuthenticated â†’ Firebase login succeeded (internal transition)
/// 3. AuthStateSyncingWithBackend â†’ Backend sync in progress (NO redirect)
/// 4. AuthStateAuthenticated â†’ Backend data loaded, router can NOW evaluate redirects
/// 5. AuthStateUnauthenticated â†’ User logged out
/// 6. AuthStateError â†’ Error occurred
/// 7. AuthStateRequiresProfileCompletion â†’ Profile completion needed (router â†’ /auth/complete-profile)
/// 8. AuthStateBackendFailure/BackendUnavailable â†’ Degraded mode (router: no redirect)
///
/// **APP ENTRY ROUTING OWNER: goRouterProvider**
/// - Router watches authControllerProvider state
/// - Router's _handleAuthenticationRedirect() decides route based on state
/// - AuthController ONLY emits state, NEVER decides routes
///
/// Mengikuti DEVELOPMENT_STANDARDS_V1_ID.md:
/// - Interface-first design dengan dependency injection âœ…
/// - Result pattern untuk error handling âœ…
/// - Proper state management âœ…
class AuthController extends Notifier<AuthState> {
  late final IAuthRepository _authRepository;
  late final ILoggerService _logger;
  late final IAnalyticsRepository _analytics;
  late final AuthSignInService _signInService;
  late final AuthSignUpService _signUpService;
  late final AuthProfileService _profileService;
  late final ILocalStorageService _localStorage;
  UserSyncService? _userSyncService; // Backend sync service for roles

  // ðŸ”’ SECURITY FIX: Periodic session validation timer
  Timer? _sessionValidationTimer;

  // ðŸ” AUTH PERSISTENCE FIX: Stream subscription for Firebase Auth state changes
  StreamSubscription<User?>? _authStateSubscription;

  // ðŸ”§ SYNC FIX: Track if backend sync has been completed for current user
  // This prevents race condition where /users/me is called before /users/sync completes
  String? _syncedUserId;
  bool _syncInProgress = false;

  // ðŸ”’ MUTEX: Prevent concurrent sync operations
  // Use Completable as a simple mutex lock
  Future<void>? _ongoingSync;

  // ðŸ›¡ï¸ RE-ENTRANCY GUARD: Prevent multiple rapid taps on Google sign-in button
  bool _isGoogleSigningIn = false;

  // ðŸ” USERNAME DECOUPLING: Pending signup username (NOT from provider metadata)
  // Set during email signup and used for backend sync
  String? _pendingSignupUsername;

  // ðŸ”’ FLOW AWARENESS: Flag to distinguish email signup from login
  // Set in signUpWithEmail(), cleared after sync completes
  // This makes _syncWithBackend() explicitly aware of the authentication flow
  bool _isInitiatingEmailSignup = false;

  // ðŸ”„ DEGRADED MODE RETRY: Auto-retry with exponential backoff
  int _retryCount = 0;
  static const int _maxRetries = 3;
  Timer? _retryTimer;
  int _authHydrationGeneration = 0;

  /// True while an automatic backend-sync retry is still scheduled (or in
  /// flight) after a transient backend-unavailable failure.
  ///
  /// STAGE 3B: this is the retry-budget signal consumed by the startup UI.
  /// While it is true the degraded splash (e.g. "Server Tidak Bisa
  /// Dijangkau") must NOT be shown — the app is still inside its canonical
  /// automatic recovery cycle, so the startup presentation stays in a
  /// pending/loading state. Once the budget is exhausted (no timer
  /// scheduled) the degraded screen becomes the truthful terminal state
  /// for that attempt.
  ///
  /// Pure read of existing retry state — no new auth semantics.
  bool get isBackendRetryPending => _retryTimer != null;

  /// Set new state with logging for all transitions (debug-mode verbosity).
  void _setState(AuthState newState) {
    final oldState = state;
    state = newState;

    _logger.log(
      '[AUTH] State: ${oldState.runtimeType} â†’ ${newState.runtimeType}',
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
  /// hanya melihat 3 status final ini.
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

    // Initialize services
    // UserSyncService is only used by AuthController, not by sign-in/sign-up services
    _signInService = AuthSignInService(
      authRepository: _authRepository,
      logger: _logger,
    );
    _signUpService = AuthSignUpService(
      authRepository: _authRepository,
      logger: _logger,
    );
    _profileService = AuthProfileService(
      authRepository: _authRepository,
      logger: _logger,
    );

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
  /// unauthenticated state. The router then redirects to /welcome â€”
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
      'handleSessionExpired: forcing signOut â€” '
      'token refresh failed on a 401 (session no longer trustworthy)',
    );

    // Reuse the canonical sign-out path so FCM cleanup, analytics,
    // sync-lock reset, and the AuthStateUnauthenticated transition all
    // run exactly as on a user-initiated logout.
    await signOut();
  }

  /// ðŸ” AUTH PERSISTENCE FIX: Listen to Firebase Auth state changes
  /// This is the most reliable way to check if user is logged in on app start
  /// Firebase Auth will emit the current state once it loads from platform storage
  ///
  /// âœ… NEW SIMPLIFIED FLOW: Email verification happens outside app
  /// - Signup â†’ Verify email (outside app) â†’ Login â†’ DONE
  /// - No blocking for unverified emails - backend enforces verification
  void _initializeAuthState() {
    _logger.log('[AUTH] _initializeAuthState() called', level: LogLevel.info);

    // Cancel any existing subscription
    _authStateSubscription?.cancel();

    // ðŸ›¡ï¸ STARTUP FAILSAFE: If Firebase listener never fires (network hang,
    // Firebase SDK issue), force unauthenticated after 15s to escape splash.
    Future.delayed(const Duration(seconds: 15), () {
      if (state is AuthStateInitial) {
        _logger.log(
          '[AUTH] STARTUP TIMEOUT: still AuthStateInitial after 15s â€” forcing unauthenticated',
          level: LogLevel.error,
        );
        _setState(const AuthState.unauthenticated());
      }
    });

    // Listen to Firebase Auth state changes
    _authStateSubscription = FirebaseAuth.instance.authStateChanges().listen(
      (User? user) async {
        _logger.log(
          '[AUTH] Firebase Auth listener fired â†’ ${user != null ? "User(${user.uid})" : "None"}',
          level: LogLevel.info,
        );
        if (user != null) {
          final principal = FirebasePrincipal.fromFirebaseUser(user);
          // ðŸ” CRITICAL: RELOAD Firebase user to get fresh data
          // ðŸ›¡ï¸ TIMEOUT: user.reload() can hang indefinitely on bad networks.
          // The catch block below handles TimeoutException and continues
          // with stale cached data rather than blocking forever.
          try {
            _logger.log('[AUTH] user.reload() start', level: LogLevel.debug);
            await user.reload().timeout(const Duration(seconds: 5));
            _logger.log('[AUTH] user.reload() done', level: LogLevel.debug);
            final refreshedUser = activeFirebaseUser;
            if (refreshedUser == null) {
              // User was deleted during reload
              _setState(const AuthState.unauthenticated());
              return;
            }

            // Email verification is NOT a separate auth state.
            // Surface unverified status via persistent banner + inline gate
            // (see EmailVerificationBanner / blocked_action_gate).
            // Backend (route middleware) is the enforcement boundary â€” mobile
            // does NOT halt sync on `!emailVerified`.
            _setState(
              AuthState.firebaseAuthenticated(
                refreshedUser.uid,
                principal: FirebasePrincipal.fromFirebaseUser(refreshedUser),
              ),
            );

            // ðŸ”’ FLOW AWARENESS: Pass explicit isEmailSignup flag from signup flow
            final isEmailSignupFlow = _isInitiatingEmailSignup;
            _syncWithBackend(
              refreshedUser.uid,
              refreshedUser,
              isEmailSignup: isEmailSignupFlow,
            );
          } catch (e) {
            _logger.error(
              'Failed to reload Firebase user',
              extra: {'error': e.toString()},
            );
            // Continue with stale user data on reload failure
            _setState(
              AuthState.firebaseAuthenticated(user.uid, principal: principal),
            );

            final isEmailSignupFlow = _isInitiatingEmailSignup;
            _syncWithBackend(user.uid, user, isEmailSignup: isEmailSignupFlow);
          }
        } else {
          _setState(const AuthState.unauthenticated());
          _stopSessionValidation();
        }
      },
      onError: (e) {
        _logger.error(
          'Firebase Auth state error',
          extra: {'error': e.toString()},
        );
        _setState(const AuthState.unauthenticated());
      },
    );
  }

  /// ðŸ”§ BACKEND SYNC: Sync user with backend and get complete user data
  /// This is the PRIMARY method that handles the full sync flow:
  /// 1. Call /users/sync to ensure user exists in PostgreSQL
  /// 2. Call /users/me to get complete user data including roles
  /// 3. Only then set AuthStateAuthenticated (router can NOW evaluate redirects)
  ///
  /// âœ… NEW SIMPLIFIED FLOW:
  /// - NO email verification enforcement - backend handles it
  /// - Users can sync regardless of emailVerified status
  /// - Backend will enforce verification restrictions where needed
  ///
  /// ðŸ”’ MUTEX GUARD: Only one sync operation can run at a time per user session.
  /// Multiple calls for the same userId will be deduplicated.
  ///
  /// ðŸ” FLOW AWARENESS: username handling differs by authentication flow:
  /// - Email signup (isEmailSignup=true): Requires _pendingSignupUsername; only username is collected at signup
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

    // ðŸ”’ GUARD 1: Skip if already in a valid post-sync state
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

    // ðŸ”’ GUARD 2: Wait for ongoing sync if any, then skip
    if (_syncInProgress || _ongoingSync != null) {
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

    // ðŸ”’ GUARD 3: Mark sync as in progress
    _syncInProgress = true;
    final syncCompleter = Completer<void>();
    _ongoingSync = syncCompleter.future;
    final requestGeneration = _beginHydrationRequest();

    try {
      _publishIfCurrent(
        requestGeneration,
        AuthState.syncingWithBackend(userId, principal: principal),
      );

      if (_userSyncService == null) {
        _logger.log(
          '[SYNC] Backend service not configured â€” userSyncService is null',
          level: LogLevel.error,
        );
        _setState(const AuthState.unauthenticated());
        return;
      }
      _logger.log('[SYNC] Calling /users/sync...', level: LogLevel.info);

      // ðŸ” FLOW AWARENESS: Determine username based on explicit flow type
      String syncUsername;

      if (isEmailSignup) {
        // ðŸ“§ EMAIL SIGNUP: Require pending username
        if (_pendingSignupUsername == null) {
          _logger.error('[SYNC] Email signup missing username');
          _setState(
            const AuthState.error('Signup data missing. Please try again.'),
          );
          return;
        }

        syncUsername = _pendingSignupUsername!.trim();

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
        // ðŸ” EMAIL LOGIN or GOOGLE: Send empty username â€” backend decides profileComplete
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
          _retryTimer?.cancel();
          _retryTimer = null;
          _retryCount = 0;
          _setState(AuthState.error(error ?? 'Backend session user mismatch'));
          return;
        }

        // ðŸ›¡ï¸ SIGN-OUT GUARD: Do not overwrite state if user already signed out
        if (activeFirebaseUser == null || state is AuthStateUnauthenticated) {
          _logger.log(
            '[SYNC] User signed out during sync, preserving Unauthenticated state',
            level: LogLevel.debug,
          );
          return;
        }

        // ðŸ” REGISTRATION USERNAME REJECTION (Stage 1B contract):
        // When the authenticated exchange rejects the registration username,
        // surface a user-facing message and keep the user on the registration
        // form (backendFailure → degraded → no router redirect) so they can
        // correct the username and retry. These are terminal business
        // rejections — NOT retryable with backoff and NOT identity errors, so
        // they must not trigger signOut, auto-retry, or a generic message.
        final usernameError = _mapRegistrationUsernameError(syncResult.errorCode);
        if (usernameError != null) {
          _retryTimer?.cancel();
          _retryTimer = null;
          _retryCount = 0;
          _logger.warning(
            'Registration username rejected by backend',
            extra: {'error': error, 'errorCode': syncResult.errorCode},
          );
          _publishIfCurrent(
            requestGeneration,
            AuthState.backendFailure(usernameError),
          );
          return;
        }

        // PASS 2A / F1: structured-first classification using the backend's
        // errorCode/statusCode (INVALID_TOKEN, ACCOUNT_DELETED,
        // ACCOUNT_INACTIVE), falling back to free-text matching only when
        // no structured code is present.
        final errorKind = classifyAuthSyncError(
          error,
          errorCode: syncResult.errorCode,
          statusCode: syncResult.statusCode,
        );

        if (!_isCurrentHydrationRequest(requestGeneration)) {
          return;
        }

        // ðŸ”’ IDENTITY INVALID / ACCOUNT DELETED: Firebase token rejected or
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
        // Account inactive sessions return a terminal auth error.
        // This path returns a terminal auth error when the backend exchange
        // cannot complete. We do not publish an authenticated state until
        // the backend session is fully established.
        if (errorKind == AuthSyncErrorKind.accountInactive) {
          _logger.warning(
            'Account inactive detected during sync, not retrying',
            extra: {'error': error, 'errorCode': syncResult.errorCode},
          );
          _retryTimer?.cancel();
          _retryTimer = null;
          _retryCount = 0;
          _setState(AuthState.error(error ?? 'Your account is not active.'));
          return;
        }
        // Backend unavailable: timeout, network, and 5xx errors do not sign out.
        if (errorKind == AuthSyncErrorKind.backendUnavailable) {
          _logger.warning(
            'Backend unavailable - keeping Firebase session',
            extra: {'error': error, 'retryCount': _retryCount},
          );
          _publishIfCurrent(
            requestGeneration,
            AuthState.backendUnavailable(error ?? 'Backend unavailable'),
          );

          // ðŸ”„ AUTO-RETRY with exponential backoff
          if (_retryCount < _maxRetries) {
            _retryCount++;
            final delay = Duration(seconds: 2 * _retryCount); // 2s, 4s, 6s
            _logger.info(
              'Scheduling auto-retry $_retryCount/$_maxRetries in ${delay.inSeconds}s',
            );
            _retryTimer?.cancel();
            _retryTimer = Timer(delay, () {
              if (activeFirebaseUser != null) {
                _logger.info('Executing auto-retry $_retryCount/$_maxRetries');
                _syncWithBackend(
                  userId,
                  firebaseUser,
                  isEmailSignup: isEmailSignup,
                );
              }
            });
          } else {
            // Max retries reached - user must manually retry or logout
            _logger.error(
              'Max retries ($_maxRetries) reached - manual retry required',
            );
          }
          return;
        }

        // ðŸ”’ BACKEND FAILURE: 4xx validation errors - do NOT signOut
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

      // ðŸ”„ DEGRADED MODE: Reset retry counter on successful sync
      _retryCount = 0;
      _retryTimer?.cancel();
      _retryTimer = null;

      // ðŸ” BACKEND-AUTHORITATIVE PROFILE COMPLETION: Check backend profile_complete flag
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
        _pendingSignupUsername = null;
        _isInitiatingEmailSignup = false;
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

      _pendingSignupUsername = null;
      _isInitiatingEmailSignup = false;

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

      // ðŸ›¡ï¸ SIGN-OUT GUARD: Do not overwrite state if user already signed out
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

      // ðŸ”’ IDENTITY ERROR in catch - MUST signOut
      if (errorKind == AuthSyncErrorKind.identityInvalid ||
          errorKind == AuthSyncErrorKind.accountDeleted) {
        await performFirebaseSignOut();
        _setState(const AuthState.unauthenticated());
        return;
      }

      // ðŸ”’ ACCOUNT INACTIVE in catch - unreachable in practice (no
      // structured code is ever available here), kept for consistency.
      if (errorKind == AuthSyncErrorKind.accountInactive) {
        _setState(AuthState.error('Account inactive: $errorStr'));
        return;
      }

      // ðŸ”’ BACKEND UNAVAILABLE in catch - do NOT signOut
      if (errorKind == AuthSyncErrorKind.backendUnavailable) {
        _publishIfCurrent(
          requestGeneration,
          AuthState.backendUnavailable('Connection error: $errorStr'),
        );
        return;
      }

      // ðŸ”’ BACKEND FAILURE in catch - do NOT signOut
      _publishIfCurrent(
        requestGeneration,
        AuthState.backendFailure('Sync error: $errorStr'),
      );
    } finally {
      _syncInProgress = false;
      _ongoingSync = null;
      syncCompleter.complete();
      _logger.log('[SYNC] Backend sync complete', level: LogLevel.debug);
    }
  }

  /// ðŸ”’ SECURITY FIX: Start periodic session validation
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

  /// ðŸ”’ SECURITY FIX: Stop periodic session validation
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

        // ID1F: Account restriction gate â€” mid-session suspension/ban on resume
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
        // BaseEntity) over every backend-authoritative field â€” role,
        // accountStatus, hasMarketAuthority, hasSellerProfile,
        // sellerSubscriptionStatus, sellerTier, penalty points,
        // verification flags â€” so this single `!=` check both (a) catches
        // authority-relevant changes that don't touch role at all (e.g. a
        // seller subscription expiring mid-session, which flips
        // hasMarketAuthority but never touches roles) and (b) is a no-op
        // when the user is genuinely unchanged, avoiding unnecessary
        // rebuilds.
        if (freshUser != currentState.user) {
          _logger.info(
            'Authority-relevant user data changed on resume: '
            'role ${currentState.user.role} â†’ ${freshUser.role}, '
            'hasMarketAuthority ${currentState.user.hasMarketAuthority} â†’ '
            '${freshUser.hasMarketAuthority}',
          );
          // Update state with new user data so the router / SellerGuard /
          // permission gates observe the change immediately instead of
          // holding a stale cached AuthUser until the next full login sync.
          // Preserve current emailVerified flag â€” this is the resume hook,
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

  /// ðŸ”’ SECURITY FIX: Validate current session and refresh roles
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

      // ðŸ” CRITICAL: RELOAD Firebase user to get fresh data
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

        // ID1F: Account restriction gate â€” mid-session suspension/ban
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
        // backend-authoritative field), not just `.role` â€” otherwise a
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
            'validation: role ${currentState.user.role} â†’ ${freshUser.role}, '
            'hasMarketAuthority ${currentState.user.hasMarketAuthority} â†’ '
            '${freshUser.hasMarketAuthority}',
          );
          // Refresh emailVerified from Firebase â€” the periodic validation is
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
      // âœ… FIXED: Don't force sign out on validation error
      // Could be network issue or temporary problem
      _logger.error('Session validation error', extra: {'error': e.toString()});
      // Just log the error and continue - Firebase will handle auth state
    }
  }

  /// Sign in dengan email dan password
  ///
  /// ðŸ”’ DETERMINISTIC FLOW: Firebase auth listener handles backend sync
  /// This method only initiates Firebase login, listener will trigger
  /// and call _syncWithBackend() with mutex protection.
  ///
  /// âš ï¸ CRITICAL: Do NOT overwrite state after successful Firebase login.
  /// The Firebase listener may have already triggered and set Authenticated state.
  /// Overwriting would cause UI to get stuck in non-authenticated state.
  Future<void> signInWithEmail({
    required String email,
    required String password,
  }) async {
    _setState(const AuthState.loading());

    final result = await _signInService.signInWithEmail(
      email: email,
      password: password,
    );

    if (result.isError) {
      // Only set error state if login failed
      _setState(AuthState.error(result.error!));
    }
    // If success: DO NOT set state here
    // Firebase listener will handle state transition:
    // loading â†’ firebaseAuthenticated â†’ syncingWithBackend â†’ authenticated
    // This prevents state overwrite race condition
  }

  /// Sign in dengan Google
  ///
  /// ðŸ”’ DETERMINISTIC FLOW: Firebase auth listener handles backend sync
  /// This method only initiates Firebase login, listener will trigger
  /// and call _syncWithBackend() with mutex protection.
  ///
  /// âš ï¸ CRITICAL: Do NOT overwrite state after successful Firebase login.
  /// The Firebase listener may have already triggered and set Authenticated state.
  /// Overwriting would cause UI to get stuck in non-authenticated state.
  ///
  /// ðŸ”’ PREMATURE STATE FIX: NO state change before Firebase sign-in completes.
  /// Setting loading state triggers router redirect to splash before Google picker.
  /// Firebase auth state listener handles ALL state transitions on success.
  ///
  /// ðŸ›¡ï¸ RE-ENTRANCY GUARD: Prevent multiple rapid taps from triggering multiple flows.
  Future<void> signInWithGoogle() async {
    // ðŸ›¡ï¸ GUARD: Ignore if already signing in (user double-tapped button)
    if (_isGoogleSigningIn) {
      await _logger.debug('Google sign-in already in progress, ignoring tap');
      return;
    }

    // âŒ REMOVED: _setState(const AuthState.loading());
    // This was causing premature redirect to splash screen
    // before Google account picker completed.

    _isGoogleSigningIn = true;

    try {
      final result = await _signInService.signInWithGoogle();

      if (result.isError) {
        // Only set error state if login failed
        _setState(AuthState.error(result.error!));
      }
      // If success: DO NOT set state here
      // Firebase listener will handle state transition:
      // firebaseAuthenticated â†’ syncingWithBackend â†’ authenticated
      // This prevents premature routing and state overwrite race condition
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
  /// ðŸ”’ DETERMINISTIC FLOW: Firebase auth listener handles backend sync
  /// This method only initiates Firebase signup, listener will trigger
  /// and call _syncWithBackend() with mutex protection.
  ///
  /// âš ï¸ CRITICAL: Do NOT overwrite state after successful Firebase signup.
  /// The Firebase listener may have already triggered and set Authenticated state.
  /// Overwriting would cause UI to get stuck in non-authenticated state.
  ///
  /// ðŸ” USERNAME DECOUPLING: Store username for backend sync
  /// instead of relying on provider metadata as authoritative source.
  ///
  /// ðŸ”’ DATA INTEGRITY: Validate username is not empty before proceeding.
  ///
  /// ðŸ”’ FLOW AWARENESS: Set flag so _syncWithBackend() knows this is email signup
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

    // ðŸ”’ RACE CONDITION FIX: Set pending username BEFORE Firebase signup
    // Firebase authStateChanges listener can fire during/after signUpWithEmail()
    _isInitiatingEmailSignup = true;
    _pendingSignupUsername = trimmedUsername;

    _setState(const AuthState.loading());

    final result = await _signUpService.signUpWithEmail(
      email: email,
      password: password,
      username: username,
    );

    if (result.isError) {
      // Only set error state if signup failed
      _setState(AuthState.error(result.error!));
      return;
    }

    // If success: DO NOT set state here
    // Firebase listener will handle state transition:
    // loading â†’ firebaseAuthenticated â†’ syncingWithBackend â†’ authenticated
    // This prevents state overwrite race condition

    // Track signup event for analytics (non-blocking)
    final firebaseUser = activeFirebaseUser;
    if (firebaseUser != null) {
      await _analytics.logEvent(
        'sign_up',
        parameters: {'method': 'email', 'user_id': firebaseUser.uid},
        userId: firebaseUser.uid,
      );
    }
  }

  /// True while an email signup is mid-flight: the Firebase account has been
  /// created but the backend registration username has not yet been committed.
  ///
  /// After a backend USERNAME_TAKEN / USERNAME_RESERVED / USERNAME_INVALID_FORMAT
  /// rejection, `_isInitiatingEmailSignup` remains true and `_pendingSignupUsername`
  /// still holds the rejected choice, so this is the signal the registration UI
  /// uses to switch from "create a Firebase account" to "retry the exchange".
  bool get hasPendingRegistration =>
      _isInitiatingEmailSignup && _pendingSignupUsername != null;

  /// Retry the authenticated exchange with a corrected registration username.
  ///
  /// Stage 1C — Part B recovery. When the backend rejects the first username
  /// choice (USERNAME_TAKEN / USERNAME_RESERVED), the Firebase account already
  /// exists, so re-running `signUpWithEmail` would hit EMAIL_ALREADY_IN_USE.
  /// Instead this re-runs ONLY the authenticated exchange with the corrected
  /// username — the Firebase account is never recreated and no session is
  /// broken. The canonical backend assigns the corrected username exactly once
  /// (Stage 1A), then a full session / authenticated state is emitted.
  Future<void> retryRegistrationUsername(String normalizedUsername) async {
    final trimmed = normalizedUsername.trim();
    if (trimmed.isEmpty) {
      _setState(const AuthState.error('Username is required'));
      return;
    }

    final firebaseUser = activeFirebaseUser;
    if (firebaseUser == null) {
      _setState(const AuthState.unauthenticated());
      return;
    }

    _pendingSignupUsername = trimmed;
    _isInitiatingEmailSignup = true;

    // Reset degraded-mode retry state before re-running the exchange.
    _retryTimer?.cancel();
    _retryTimer = null;
    _retryCount = 0;
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
  /// ðŸ”’ DETERMINISTIC FIX: Reset sync locks on logout to ensure
  /// next login session starts fresh without stale sync state.
  Future<void> signOut() async {
    // ðŸ”’ SECURITY FIX: Stop session validation timer
    _stopSessionValidation();

    // ðŸ”„ DEGRADED MODE: Clear retry timer
    _retryTimer?.cancel();
    _retryTimer = null;
    _retryCount = 0;
    final currentState = state;

    // 1. Attempt backend logout BEFORE any local cleanup removes tokens.
    if (currentState is AuthStateAuthenticated) {
      try {
        final refreshResult = await _localStorage.getRefreshToken();
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

    // Presence: stop tracking BEFORE socket closes (sends offline status
    // while connection is still live). Non-fatal: don't block logout.
    try {
      ref.read(presenceManagerProvider.notifier).clearUser();
    } catch (_) {}

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
        'WebSocket disconnect failed during sign out â€” '
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
    _syncInProgress = false;
    _ongoingSync = null;

    // 5. Proceed with Firebase Auth sign out
    final result = await _signInService.signOut();

    if (result.isSuccess) {
      _setState(const AuthState.unauthenticated());
    } else {
      _setState(AuthState.error(result.error!));
    }
  }

  /// Activate WebSocket connection and presence tracking after successful
  /// backend sync. Called once per login; idempotent on re-entry (WS guards
  /// duplicate connect, presence handles same-user no-op).
  ///
  /// Fire-and-forget: failures are logged but NEVER block the auth flow.
  void _activateRealtimeServices(String userId, User firebaseUser) {
    // WebSocket: connect with fresh Firebase ID token
    unawaited(
      Future<void>(() async {
        try {
          final token = await firebaseUser.getIdToken();
          if (token == null || token.isEmpty) return;
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

    // Presence: start tracking user online status
    try {
      ref.read(presenceManagerProvider.notifier).setUser(userId);
    } catch (e) {
      _logger.log(
        '[AUTH] Presence setUser failed (non-fatal): $e',
        level: LogLevel.warning,
      );
    }
  }

  /// ðŸ”„ DEGRADED MODE: Manually retry backend sync after failure
  /// Can be called by UI when user taps "Retry" button
  Future<void> retryBackendSync() async {
    final firebaseUser = activeFirebaseUser;
    if (firebaseUser == null) {
      _setState(const AuthState.unauthenticated());
      return;
    }

    // Clear any pending retry timer
    _retryTimer?.cancel();
    _retryTimer = null;

    // Reset retry count for manual retry
    _retryCount = 0;

    _logger.info('Manual retry triggered for backend sync');

    // Clear sync flag to force fresh sync
    _syncedUserId = null;

    // Trigger backend sync
    _syncWithBackend(firebaseUser.uid, firebaseUser, isEmailSignup: false);
  }

  /// Change email for current user
  Future<bool> changeEmail({
    required String newEmail,
    required String currentPassword,
  }) async {
    final result = await _profileService.changeEmail(
      newEmail: newEmail,
      currentPassword: currentPassword,
    );

    if (result.isError) {
      _setState(AuthState.error(result.error!));
    }

    return result.isSuccess;
  }

  /// Change password for current user
  Future<bool> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async {
    final result = await _profileService.changePassword(
      currentPassword: currentPassword,
      newPassword: newPassword,
    );

    if (result.isError) {
      _setState(AuthState.error(result.error!));
    }

    return result.isSuccess;
  }

  /// Send email verification
  Future<bool> sendEmailVerification() async {
    final result = await _profileService.sendEmailVerification();

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
    final result = await _profileService.updateProfile(
      photoUrl: photoUrl,
      username: username,
      bio: bio,
      phoneNumber: phoneNumber,
      location: location,
      phoneVerifiedAt: phoneVerifiedAt,
      dateOfBirth: dateOfBirth,
    );

    if (result.isSuccess) {
      // Preserve current emailVerified flag â€” profile update does not affect
      // email-verification status.
      await forceRefreshAuthState();
      return true;
    } else {
      _setState(AuthState.error(result.error!));
      return false;
    }
  }

  /// Complete the profile after restricted Firebase exchange.
  Future<bool> completeProfile({required String username}) async {
    final currentState = state;
    if (currentState is! AuthStateRequiresProfileCompletion) {
      _setState(AuthState.error('Invalid authentication state'));
      return false;
    }

    final result = await _profileService.completeProfile(username: username);

    if (result.isSuccess && result.data != null) {
      final firebaseUser = activeFirebaseUser;
      if (firebaseUser == null) {
        _setState(const AuthState.unauthenticated());
        return false;
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
        return true;
      }

      _pendingSignupUsername = null;
      _isInitiatingEmailSignup = false;
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

      return true;
    }

    final errorCode = result.errorCode;
    final error = result.error ?? 'Failed to complete profile';

    if (errorCode == 'SESSION_USER_MISMATCH') {
      _setState(AuthState.error(error));
      return false;
    }

    if (errorCode == 'PROFILE_ALREADY_COMPLETED' ||
        errorCode == 'INVALID_TOKEN' ||
        errorCode == 'INVALID_SCOPE' ||
        errorCode == 'TOKEN_EXPIRED' ||
        error.contains('already completed') ||
        error.contains('invalid token') ||
        error.contains('expired')) {
      await refreshAuthState();
      return true;
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
      return false;
    }

    _setState(AuthState.error(error));
    return false;
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
  /// ðŸ” CRITICAL: RELOAD Firebase user to get fresh data
  /// This ensures external verification (e.g., email link) is detected
  Future<void> refreshAuthState() async {
    try {
      // ðŸ” CRITICAL: RELOAD Firebase user before checking status
      await activeFirebaseUser?.reload();
      final firebaseUser = activeFirebaseUser;
      if (firebaseUser == null) {
        _setState(const AuthState.unauthenticated());
        return;
      }

      // Clear sync flag to force fresh sync
      _syncedUserId = null;

      // Trigger full backend sync with refreshed user
      // This handles the flow: firebaseAuthenticated â†’ syncingWithBackend â†’ authenticated
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
      _syncWithBackend(firebaseUser.uid, firebaseUser, isEmailSignup: false);
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

  /// Force refresh auth state from backend API
  /// SOURCE OF TRUTH: PostgreSQL (Backend API /users/me)
  ///
  /// ðŸ”§ FIX: Allow refresh from RequiresProfileCompletion state
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
        // Pull emailVerified directly from Firebase user â€” forceRefresh is the
        // path used after Complete Profile, where the flag is canonical.
        _publishAuthenticatedIfCurrent(
          requestGeneration,
          completeUser,
          emailVerified: firebaseUser.emailVerified,
        );
        _syncedUserId = firebaseUser.uid;

        // Activate WS + Presence for the complete-profileâ†’authenticated path.
        // Idempotent: safe if already connected from primary login path.
        _activateRealtimeServices(completeUser.id, firebaseUser);

        await _logger.log(
          '[AUTH] State â†’ Authenticated (after profile completion)',
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
  /// ðŸ”’ DETERMINISTIC FIX: Reset all sync locks when resetting controller.
  /// This ensures clean state for testing or edge cases.
  void reset() {
    // ðŸ” AUTH PERSISTENCE FIX: Cancel stream subscription when resetting
    _authStateSubscription?.cancel();
    _authStateSubscription = null;

    // ðŸ”’ SYNC LOCK RESET: Clear all sync state
    _beginHydrationRequest();
    _syncedUserId = null;
    _syncInProgress = false;
    _ongoingSync = null;
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
  /// Calls backend soft-delete â†’ Firebase credential delete â†’ local signOut.
  /// Returns the error string on failure (null on success).
  Future<String?> deleteAccount() async {
    final result = await _authRepository.deleteAccount();
    if (result.isSuccess) {
      await signOut();
      return null;
    }
    return result.error ?? 'Failed to delete account';
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
