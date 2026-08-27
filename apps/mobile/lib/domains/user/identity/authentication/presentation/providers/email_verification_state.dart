/// Email Verification State
///
/// Sealed class for type-safe email verification state management.
///
/// States:
/// - initial: Not checked yet
/// - checking: Currently verifying/sending email
/// - verified: Email is verified
/// - unverified: Email not verified
/// - error: Error during verification process
library;

/// Sealed class for email verification state
sealed class EmailVerificationState {
  const EmailVerificationState();

  const factory EmailVerificationState.initial() = EmailVerificationInitial;
  const factory EmailVerificationState.checking() = EmailVerificationChecking;
  const factory EmailVerificationState.verified() = EmailVerificationVerified;
  const factory EmailVerificationState.unverified() =
      EmailVerificationUnverified;
  const factory EmailVerificationState.error(String message) =
      EmailVerificationError;
}

/// Initial state - verification status not yet checked
class EmailVerificationInitial extends EmailVerificationState {
  const EmailVerificationInitial();
}

/// Checking state - currently verifying or sending email
class EmailVerificationChecking extends EmailVerificationState {
  const EmailVerificationChecking();
}

/// Verified state - email has been verified
class EmailVerificationVerified extends EmailVerificationState {
  const EmailVerificationVerified();
}

/// Unverified state - email has not been verified
class EmailVerificationUnverified extends EmailVerificationState {
  const EmailVerificationUnverified();
}

/// Error state - an error occurred during verification
class EmailVerificationError extends EmailVerificationState {
  final String message;

  const EmailVerificationError(this.message);
}
