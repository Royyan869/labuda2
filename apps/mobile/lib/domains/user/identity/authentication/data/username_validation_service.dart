import 'package:labuda/shared/helpers/canonical_username_validator.dart';

/// Username check status enum
enum UsernameCheckStatus {
  idle,
  checking,
  validFormat,
  available,
  unavailable,
  invalid,
  error,
}

/// Username check result class
class UsernameCheckResult {
  final UsernameCheckStatus status;
  final String? message;

  const UsernameCheckResult({required this.status, this.message});

  factory UsernameCheckResult.validFormat() {
    return const UsernameCheckResult(
      status: UsernameCheckStatus.validFormat,
      message: 'Username format is valid',
    );
  }

  factory UsernameCheckResult.available([String? message]) {
    return UsernameCheckResult(
      status: UsernameCheckStatus.available,
      message: message ?? 'Username is available',
    );
  }

  factory UsernameCheckResult.unavailable([String? message]) {
    return UsernameCheckResult(
      status: UsernameCheckStatus.unavailable,
      message: message ?? 'Username is already taken',
    );
  }

  factory UsernameCheckResult.invalid(String message) {
    return UsernameCheckResult(
      status: UsernameCheckStatus.invalid,
      message: message,
    );
  }

  factory UsernameCheckResult.error([String? message]) {
    return UsernameCheckResult(
      status: UsernameCheckStatus.error,
      message: message ?? 'Error checking username',
    );
  }

  factory UsernameCheckResult.checking() {
    return const UsernameCheckResult(
      status: UsernameCheckStatus.checking,
      message: 'Checking username...',
    );
  }

  bool get isAvailable => status == UsernameCheckStatus.available;
  bool get isValid =>
      status == UsernameCheckStatus.validFormat ||
      status == UsernameCheckStatus.available;
}

/// Username Validation Service - Handles username format validation
class UsernameValidationService {
  /// Validate username format against the canonical mobile authority
  /// ([CanonicalUsernameValidator], which mirrors the backend
  /// identityusername rules exactly).
  ///
  /// NOTE: There is NO local reserved-name list and NO divergent local regex.
  /// Reserved names are backend authority (identityusername.IsReserved); the
  /// local check only establishes format validity so reserved/taken names
  /// still reach the backend availability check.
  static UsernameCheckResult validateUsernameFormat(String username) {
    if (username.isEmpty) {
      return UsernameCheckResult.invalid('Username cannot be empty');
    }

    final canonical = CanonicalUsernameValidator.normalizeAndValidate(username);
    if (canonical == null) {
      return UsernameCheckResult.invalid(
        'Username must be 3-30 chars: lowercase letters, numbers, '
        'and underscores only',
      );
    }

    return UsernameCheckResult.validFormat();
  }
}
