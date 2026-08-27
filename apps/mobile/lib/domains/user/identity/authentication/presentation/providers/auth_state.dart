import '../../domain/entities/account_status.dart';
import '../../domain/entities/firebase_principal.dart';
import '../../domain/entities/auth_user.dart' as domain;

/// Router-level authentication status - simplified for redirect logic
///
/// Router hanya membaca status ini, bukan AuthState secara langsung.
/// State machine internal AuthState tetap kompleks, tapi router
/// hanya melihat 5 status final ini.
enum AppAuthStatus {
  /// Show splash screen - app is initializing
  initializing,

  /// Show welcome/login screen - user is not authenticated
  unauthenticated,

  /// Show home screen - user is fully authenticated
  authenticated,

  /// Show account restricted screen - user is suspended or banned
  accountRestricted,

  /// Backend is degraded but Firebase auth is valid
  /// User stays on current route - NO automatic redirect
  degraded,
}

/// Authentication state dengan sealed classes untuk type safety
sealed class AuthState {
  const AuthState();

  const factory AuthState.initial() = AuthStateInitial;
  const factory AuthState.loading({
    FirebasePrincipal? principal,
  }) = AuthStateLoading;
  const factory AuthState.firebaseAuthenticated(
    String userId, {
    FirebasePrincipal? principal,
  }) = AuthStateFirebaseAuthenticated;
  const factory AuthState.syncingWithBackend(
    String userId, {
    FirebasePrincipal? principal,
  }) = AuthStateSyncingWithBackend;
  factory AuthState.authenticated(
    domain.AuthUser user, {
    required bool emailVerified,
  }) = AuthStateAuthenticated;
  const factory AuthState.unauthenticated() = AuthStateUnauthenticated;
  const factory AuthState.error(String message) = AuthStateError;
  const factory AuthState.backendFailure(String message) =
      AuthStateBackendFailure;
  const factory AuthState.backendUnavailable(String message) =
      AuthStateBackendUnavailable;
  const factory AuthState.requiresProfileCompletion({
    required String userId,
    required String email,
  }) = AuthStateRequiresProfileCompletion;
  const factory AuthState.accountRestricted(
    domain.AuthUser user, {
    required AccountStatus restrictionType,
  }) = AuthStateAccountRestricted;
}

final class AuthStateInitial extends AuthState {
  const AuthStateInitial();
}

final class AuthStateLoading extends AuthState {
  final FirebasePrincipal? principal;
  const AuthStateLoading({this.principal});
}

/// Firebase Authenticated state - Firebase sign-in succeeded, token is available
/// But backend sync has NOT started yet
/// This prevents router from making redirect decisions before backend data is ready
final class AuthStateFirebaseAuthenticated extends AuthState {
  final String userId;
  final FirebasePrincipal? principal;
  const AuthStateFirebaseAuthenticated(
    this.userId, {
    this.principal,
  });
}

/// Syncing with Backend state - Backend sync is in progress
/// Router should NOT redirect while in this state - show loading/splash screen
/// SOURCE OF TRUTH: PostgreSQL (Backend API /users/me)
final class AuthStateSyncingWithBackend extends AuthState {
  final String userId;
  final FirebasePrincipal? principal;
  const AuthStateSyncingWithBackend(
    this.userId, {
    this.principal,
  });
}

/// Authenticated state - User is fully authenticated with backend data loaded
/// SOURCE OF TRUTH: PostgreSQL (Backend API /users/me)
/// Router ONLY evaluates redirects when in this state
///
/// `emailVerified` reflects Firebase Auth's `emailVerified` flag at the
/// last sync/refresh. Unverified email is NOT a separate auth state — it is a
/// property of an Authenticated user. Surface unverified status via banner / inline
/// gate, not via redirect.
final class AuthStateAuthenticated extends AuthState {
  final domain.AuthUser user;
  final bool emailVerified;
  const AuthStateAuthenticated(this.user, {required this.emailVerified});
}

final class AuthStateUnauthenticated extends AuthState {
  const AuthStateUnauthenticated();
}

final class AuthStateError extends AuthState {
  final String message;
  const AuthStateError(this.message);
}

/// Backend Failure state - Backend returned validation/business error (4xx)
/// User is authenticated with Firebase but backend rejected the request
/// Router should treat as initializing - NO redirect to /welcome
/// Examples: 400 Bad Request, 409 Conflict, 422 Unprocessable Entity
final class AuthStateBackendFailure extends AuthState {
  final String message;
  const AuthStateBackendFailure(this.message);
}

/// Backend Unavailable state - Backend is unreachable or returned server error
/// User is authenticated with Firebase but backend is down
/// Router should treat as initializing - NO redirect to /welcome
/// Examples: timeout, 500 Internal Server Error, network error
final class AuthStateBackendUnavailable extends AuthState {
  final String message;
  const AuthStateBackendUnavailable(this.message);
}

/// Requires Profile Completion state - Google sign-in for new user
/// Router should redirect to /auth/complete-profile
/// User must complete profile before accessing app
final class AuthStateRequiresProfileCompletion extends AuthState {
  final String userId;
  final String email;
  const AuthStateRequiresProfileCompletion({
    required this.userId,
    required this.email,
  });
}

/// Account Restricted state - User's account is suspended or banned
/// Router should redirect to /account-restricted screen
/// User can only view restriction info and logout
final class AuthStateAccountRestricted extends AuthState {
  final domain.AuthUser user;
  final AccountStatus restrictionType;
  const AuthStateAccountRestricted(this.user, {required this.restrictionType});
}
