import 'package:labuda/core/common/result.dart';

/// Interface untuk logger service yang menggunakan `Result<T>` pattern
///
/// Mengikuti DEVELOPMENT_STANDARDS_V1_ID.md:
/// - Interface-first design (WAJIB)
/// - `Result<T>` pattern untuk error handling (WAJIB)
/// - Centralized logging system (WAJIB)
abstract class ILoggerService {
  /// Log debug message
  Future<Result<void>> debug(String message, {Map<String, dynamic>? extra});

  /// Log info message
  Future<Result<void>> info(String message, {Map<String, dynamic>? extra});

  /// Log warning message
  Future<Result<void>> warning(String message, {Map<String, dynamic>? extra});

  /// Log error message
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  });

  /// Log critical/fatal error
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  });

  /// Log user action untuk analytics
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  });

  /// Log performance metrics
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  });

  /// Log security events (anti-circumvention, authentication, etc)
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  });

  /// Log API calls untuk monitoring
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
  });

  /// Set log level
  Future<Result<void>> setLogLevel(LogLevel level);

  /// Clear logs (untuk development/testing)
  Future<Result<void>> clearLogs();

  /// Get logs untuk debugging (development only)
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  });

  // 🔍 DEBUG: Track sync flow - Helper methods for auth controller
  Future<void> debugSync(String userId);
  Future<void> debugSyncSuccess(String userId);
  Future<void> debugSyncFailed(String userId, String? errorMessage);
  Future<void> debugCallingGetCurrentUser();
  Future<void> debugGetCurrentUserSuccess(String userId, bool isEmailVerified);
  Future<void> debugGetCurrentUserFailed(String userId, String? errorMessage);
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  );
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  );

  /// Simple logging method with level
  Future<void> log(String message, {LogLevel level = LogLevel.debug});
}

/// Log levels sesuai standar
enum LogLevel { debug, info, warning, error, fatal }

/// Log entry structure
abstract class LogEntry {
  DateTime get timestamp;
  LogLevel get level;
  String get message;
  String? get userId;
  Map<String, dynamic>? get extra;
  StackTrace? get stackTrace;
  String get source; // Class/method yang log
}
