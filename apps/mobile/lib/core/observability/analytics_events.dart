/// Canonical product-analytics event taxonomy for Labuda mobile.
///
/// ONE authority for event names and parameter keys. Producers must reference
/// these constants — never raw string literals — so the taxonomy stays stable,
/// auditable, and free of accidental duplicates.
///
/// Boundary: these events describe what the USER does. They are NOT crash
/// reporting, performance monitoring, or diagnostic logging.
library;

/// Canonical Firebase Analytics event names.
abstract final class AnalyticsEvents {
  // ---------------------------------------------------------------------------
  // Authentication
  // ---------------------------------------------------------------------------

  /// A successful authentication explicitly performed by the user
  /// (email/password or Google sign-in, or the first authentication that
  /// completes a new account's profile).
  ///
  /// MUST NOT be emitted for session restore, token refresh, or a Firebase
  /// auth-state listener event.
  static const String login = 'login';

  /// A successful account registration.
  static const String signUp = 'sign_up';

  /// The user explicitly signed out.
  static const String logout = 'logout';

  // ---------------------------------------------------------------------------
  // Authentication lifecycle (observability, NOT user-performed login)
  // ---------------------------------------------------------------------------

  /// An existing session was restored without an explicit user login
  /// (startup restore or an external auth change).
  static const String sessionRestored = 'session_restored';

  /// The live session was re-read/refreshed from backend truth without a new
  /// user login.
  static const String sessionRefreshed = 'session_refreshed';

  // ---------------------------------------------------------------------------
  // Account lifecycle
  // ---------------------------------------------------------------------------

  /// The user deactivated their own account.
  static const String accountDeactivated = 'account_deactivated';

  // ---------------------------------------------------------------------------
  // Social
  // ---------------------------------------------------------------------------

  /// The user followed another account.
  static const String follow = 'follow';

  /// The user unfollowed another account.
  static const String unfollow = 'unfollow';
}

/// Canonical Firebase Analytics parameter keys.
abstract final class AnalyticsParams {
  /// Authentication method (see [AnalyticsAuthMethods]).
  static const String method = 'method';

  /// Whether a sign-out covered every device.
  static const String allDevices = 'all_devices';

  /// Reason supplied for an account-lifecycle action.
  static const String reason = 'reason';

  /// The acting user's identifier on a social action.
  static const String followerId = 'follower_id';

  /// The target user's identifier on a social action.
  static const String followingId = 'following_id';
}

/// Canonical authentication method values for [AnalyticsParams.method].
abstract final class AnalyticsAuthMethods {
  static const String email = 'email';
  static const String google = 'google';

  /// First authentication that completes a new account's profile.
  static const String profileCompletion = 'profile_completion';

  /// Session restored without an explicit user login.
  static const String sessionRestore = 'session_restore';

  /// Session refreshed without an explicit user login.
  static const String sessionRefresh = 'session_refresh';
}
