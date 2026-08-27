/// Typed outcomes for the backend sync operation.
///
/// Replaces nullable/boolean returns with explicit sealed types
/// that drive distinct auth state transitions.
library;

/// Sealed class describing the outcome of a backend sync operation.
sealed class SyncOutcome {
  const SyncOutcome();
}

/// Sync succeeded but email verification is required.
///
/// This outcome occurs when:
/// - Backend exchange succeeds
/// - Firebase user is a password provider
/// - emailVerified is false
///
/// The auth controller should publish AuthStateRequiresEmailVerification
/// instead of AuthStateAuthenticated.
class SyncRequiresEmailVerification extends SyncOutcome {
  final String firebaseUid;
  final int principalEpoch;
  final String backendUserId;
  final String email;

  const SyncRequiresEmailVerification({
    required this.firebaseUid,
    required this.principalEpoch,
    required this.backendUserId,
    required this.email,
  });
}

/// Sync failed with an error.
///
/// This outcome occurs when:
/// - Backend exchange fails
/// - Network timeout
/// - Server error
class SyncFailed extends SyncOutcome {
  final String error;

  const SyncFailed({required this.error});
}

/// Sync succeeded but profile completion is required.
///
/// This outcome occurs when:
/// - Backend exchange succeeds
/// - profileComplete is false (Google sign-in for new user)
///
/// The auth controller should publish AuthStateRequiresProfileCompletion.
class SyncRequiresProfileCompletion extends SyncOutcome {
  final String userId;
  final String email;

  const SyncRequiresProfileCompletion({
    required this.userId,
    required this.email,
  });
}

/// Sync succeeded and user is fully authenticated.
///
/// This outcome occurs when:
/// - Backend exchange succeeds
/// - profileComplete is true
/// - emailVerified is true (or user is not a password provider)
class SyncAuthenticated extends SyncOutcome {
  final String userId;
  final String email;
  final bool emailVerified;

  const SyncAuthenticated({
    required this.userId,
    required this.email,
    required this.emailVerified,
  });
}
