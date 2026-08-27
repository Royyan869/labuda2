/// Appeal Repository Interface
///
/// Defines contract for appeal data operations.
/// Pure domain interface - no Firebase, no HTTP dependencies.
library;

import '../entities/entities.dart';

/// Appeal Repository Interface
abstract class AppealRepository {
  // =====================
  // User Operations
  // =====================

  /// Submit a new appeal
  Future<Appeal> submitAppeal(CreateAppealRequest request);

  /// Get appeals by user
  Future<List<Appeal>> getUserAppeals(
    String userId, {
    AppealStatus? status,
    int limit = 20,
  });

  /// Get appeal by ID
  Future<Appeal?> getAppealById(String appealId);

  /// Cancel appeal (user)
  Future<void> cancelAppeal(String appealId);

  /// Check if user has pending appeal for source
  Future<bool> hasPendingAppeal({
    required String userId,
    required AppealType type,
    String? sourceId,
  });

  // =====================
  // REMOVED: All Admin Operations
  // =====================
  // - getAppeals() - Admin-only endpoint
  // - reviewAppeal() - Admin-only endpoint
  // - watchPendingAppealsCount() - Admin-only endpoint
}

/// Appeal Failure types
class AppealFailure {
  final String message;
  final AppealFailureType type;

  const AppealFailure({
    required this.message,
    this.type = AppealFailureType.unknown,
  });

  factory AppealFailure.network(String message) =>
      AppealFailure(message: message, type: AppealFailureType.network);

  factory AppealFailure.notFound(String message) =>
      AppealFailure(message: message, type: AppealFailureType.notFound);

  factory AppealFailure.unauthorized(String message) =>
      AppealFailure(message: message, type: AppealFailureType.unauthorized);

  factory AppealFailure.validation(String message) =>
      AppealFailure(message: message, type: AppealFailureType.validation);

  factory AppealFailure.alreadyAppealed(String message) =>
      AppealFailure(message: message, type: AppealFailureType.alreadyAppealed);

  factory AppealFailure.cannotCancel(String message) =>
      AppealFailure(message: message, type: AppealFailureType.cannotCancel);

  @override
  String toString() => 'AppealFailure($type: $message)';
}

enum AppealFailureType {
  network,
  notFound,
  unauthorized,
  validation,
  alreadyAppealed,
  cannotCancel,
  unknown,
}
