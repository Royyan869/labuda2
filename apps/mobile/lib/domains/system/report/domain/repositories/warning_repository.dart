/// Warning Repository Interface
///
/// Defines contract for warning data operations.
/// Pure domain interface - no Firebase, no HTTP dependencies.
///
/// V1: Passive warning records only. No acknowledge, no escalation.
library;

import '../entities/user_warning.dart';

/// Warning Repository Interface
abstract class WarningRepository {
  // =====================
  // User Operations
  // =====================

  /// Get warnings for a user
  Future<List<UserWarning>> getUserWarnings(
    String userId, {
    WarningStatus? status,
    int limit = 20,
  });

  /// Get active warnings for a user
  Future<List<UserWarning>> getActiveWarnings(String userId);

  /// Get warning by ID
  Future<UserWarning?> getWarningById(String warningId);

  /// Check if user has active warnings
  Future<bool> hasActiveWarnings(String userId);

  /// Stream user's active warnings count
  Stream<int> watchActiveWarningsCount(String userId);

  // =====================
  // REMOVED: All Admin Operations (Pure Admin)
  // =====================
  // - issueWarning() - Admin-only endpoint
  // - revokeWarning() - Admin-only endpoint
  //
  // Users can only READ warnings, not create or revoke them
}

/// Warning Failure types
class WarningFailure {
  final String message;
  final WarningFailureType type;

  const WarningFailure({
    required this.message,
    this.type = WarningFailureType.unknown,
  });

  factory WarningFailure.network(String message) =>
      WarningFailure(message: message, type: WarningFailureType.network);

  factory WarningFailure.notFound(String message) =>
      WarningFailure(message: message, type: WarningFailureType.notFound);

  factory WarningFailure.unauthorized(String message) =>
      WarningFailure(message: message, type: WarningFailureType.unauthorized);

  factory WarningFailure.validation(String message) =>
      WarningFailure(message: message, type: WarningFailureType.validation);

  factory WarningFailure.cannotRevoke(String message) =>
      WarningFailure(message: message, type: WarningFailureType.cannotRevoke);

  @override
  String toString() => 'WarningFailure($type: $message)';
}

enum WarningFailureType {
  network,
  notFound,
  unauthorized,
  validation,
  cannotRevoke,
  unknown,
}
