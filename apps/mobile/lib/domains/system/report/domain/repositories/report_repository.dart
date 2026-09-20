/// Report Repository Interface
///
/// Defines contract for report data operations.
/// Pure domain interface - no Firebase, no HTTP dependencies.
library;

import '../entities/entities.dart';

/// Report Repository Interface
abstract class ReportRepository {
  // =====================
  // User Operations
  // =====================

  /// Create a new report
  Future<Report> createReport({
    required String reporterId,
    required CreateReportRequest request,
  });

  /// Get report by ID
  Future<Report?> getReportById(String reportId);

  /// Get reports by user (as reporter)
  Future<List<Report>> getReportsByUser({
    required String userId,
    int page = 1,
    int limit = 20,
  });

  /// Check if user already reported this target
  Future<bool> hasUserReported({
    required String userId,
    required String targetId,
    required ReportTargetType targetType,
  });
}

/// Report Failure types
class ReportFailure {
  final String message;
  final ReportFailureType type;

  const ReportFailure({
    required this.message,
    this.type = ReportFailureType.unknown,
  });

  factory ReportFailure.network(String message) =>
      ReportFailure(message: message, type: ReportFailureType.network);

  factory ReportFailure.notFound(String message) =>
      ReportFailure(message: message, type: ReportFailureType.notFound);

  factory ReportFailure.unauthorized(String message) =>
      ReportFailure(message: message, type: ReportFailureType.unauthorized);

  factory ReportFailure.validation(String message) =>
      ReportFailure(message: message, type: ReportFailureType.validation);

  factory ReportFailure.alreadyReported(String message) =>
      ReportFailure(message: message, type: ReportFailureType.alreadyReported);

  @override
  String toString() => 'ReportFailure($type: $message)';
}

enum ReportFailureType {
  network,
  notFound,
  unauthorized,
  validation,
  alreadyReported,
  unknown,
}
