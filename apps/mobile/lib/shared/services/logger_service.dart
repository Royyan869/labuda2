import 'dart:developer' as developer;

import 'package:flutter/foundation.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/core/common/result.dart';

/// Implementation LoggerService dengan `Result<T>` pattern
///
/// Mengikuti DEVELOPMENT_STANDARDS_V1_ID.md:
/// - Interface-first design ✅
/// - `Result<T>` pattern untuk error handling ✅
/// - No hardcoded secrets ✅
/// - Proper error handling dengan try-catch ✅
/// - Performance optimized ✅
class LoggerService implements ILoggerService {
  static const String _appName = 'Labuda';
  LogLevel _currentLogLevel = LogLevel.debug;
  final List<LogEntryImpl> _logs = [];
  static const int _maxLogEntries = 1000; // Prevent memory issues

  static final LoggerService _instance = LoggerService._internal();
  static LoggerService get instance => _instance;

  LoggerService._internal();

  @override
  Future<Result<void>> debug(
    String message, {
    Map<String, dynamic>? extra,
  }) async {
    try {
      if (_shouldLog(LogLevel.debug)) {
        final logEntry = LogEntryImpl(
          timestamp: DateTime.now(),
          level: LogLevel.debug,
          message: message,
          extra: extra,
          source: _getCallerInfo(),
        );

        _addLogEntry(logEntry);
        developer.log(
          message,
          name: _appName,
          level: 500, // DEBUG level
          error: extra,
        );
      }

      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to log debug message: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> info(
    String message, {
    Map<String, dynamic>? extra,
  }) async {
    try {
      if (_shouldLog(LogLevel.info)) {
        final logEntry = LogEntryImpl(
          timestamp: DateTime.now(),
          level: LogLevel.info,
          message: message,
          extra: extra,
          source: _getCallerInfo(),
        );

        _addLogEntry(logEntry);
        developer.log(
          message,
          name: _appName,
          level: 800, // INFO level
          error: extra,
        );
      }

      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to log info message: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async {
    try {
      if (_shouldLog(LogLevel.warning)) {
        final logEntry = LogEntryImpl(
          timestamp: DateTime.now(),
          level: LogLevel.warning,
          message: message,
          extra: extra,
          source: _getCallerInfo(),
        );

        _addLogEntry(logEntry);
        developer.log(
          message,
          name: _appName,
          level: 900, // WARNING level
          error: extra,
        );
      }

      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to log warning message: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async {
    try {
      if (_shouldLog(LogLevel.error)) {
        final logEntry = LogEntryImpl(
          timestamp: DateTime.now(),
          level: LogLevel.error,
          message: message,
          extra: extra,
          stackTrace: stackTrace,
          source: _getCallerInfo(),
        );

        _addLogEntry(logEntry);
        developer.log(
          message,
          name: _appName,
          level: 1000, // ERROR level
          error: extra,
          stackTrace: stackTrace,
        );
      }

      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to log error message: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async {
    try {
      final logEntry = LogEntryImpl(
        timestamp: DateTime.now(),
        level: LogLevel.fatal,
        message: message,
        extra: extra,
        stackTrace: stackTrace,
        source: _getCallerInfo(),
      );

      _addLogEntry(logEntry);
      developer.log(
        message,
        name: _appName,
        level: 1200, // FATAL level
        error: extra,
        stackTrace: stackTrace,
      );

      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to log fatal message: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) async {
    try {
      final enrichedParameters = {
        'action': action,
        'user_id': userId,
        'timestamp': DateTime.now().toIso8601String(),
        if (parameters != null) ...parameters,
      };

      return await info('User Action: $action', extra: enrichedParameters);
    } catch (e) {
      return Result.error('Failed to log user action: ${e.toString()}');
    }
  }

  Future<void> logError(String message, dynamic error) async {
    await this.error(message, extra: {'error': error.toString()});
  }

  Future<void> logInfo(String message) async {
    await info(message);
  }

  Future<void> logWarning(String message) async {
    await warning(message);
  }

  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) async {
    try {
      final performanceData = {
        'operation': operation,
        'duration_ms': duration.inMilliseconds,
        'timestamp': DateTime.now().toIso8601String(),
        if (metrics != null) ...metrics,
      };

      return await info(
        'Performance: $operation (${duration.inMilliseconds}ms)',
        extra: performanceData,
      );
    } catch (e) {
      return Result.error('Failed to log performance: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) async {
    try {
      final securityData = {
        'event': event,
        'user_id': userId,
        'severity': severity ?? 'medium',
        'timestamp': DateTime.now().toIso8601String(),
        if (details != null) ...details,
      };

      final logLevel = _getLogLevelFromSeverity(severity);

      switch (logLevel) {
        case LogLevel.warning:
          return await warning('Security Event: $event', extra: securityData);
        case LogLevel.error:
          return await error('Security Event: $event', extra: securityData);
        case LogLevel.fatal:
          return await fatal('Security Event: $event', extra: securityData);
        default:
          return await info('Security Event: $event', extra: securityData);
      }
    } catch (e) {
      return Result.error('Failed to log security event: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
  }) async {
    try {
      final apiCallData = {
        'endpoint': endpoint,
        'method': method,
        'status_code': statusCode,
        'duration_ms': duration.inMilliseconds,
        'timestamp': DateTime.now().toIso8601String(),
        if (requestData != null) 'request': _sanitizeData(requestData),
        if (responseData != null) 'response': _sanitizeData(responseData),
      };

      final logLevel = statusCode >= 400 ? LogLevel.error : LogLevel.info;
      final message =
          'API Call: $method $endpoint ($statusCode) - ${duration.inMilliseconds}ms';

      switch (logLevel) {
        case LogLevel.error:
          return await error(message, extra: apiCallData);
        default:
          return await info(message, extra: apiCallData);
      }
    } catch (e) {
      return Result.error('Failed to log API call: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> setLogLevel(LogLevel level) async {
    try {
      _currentLogLevel = level;
      await info('Log level changed to: ${level.name}');
      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to set log level: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> clearLogs() async {
    try {
      _logs.clear();
      await info('Logs cleared');
      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to clear logs: ${e.toString()}');
    }
  }

  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) async {
    try {
      var filteredLogs = _logs.where((log) {
        // Filter by log level
        if (minLevel != null && log.level.index < minLevel.index) {
          return false;
        }

        // Filter by date range
        if (startDate != null && log.timestamp.isBefore(startDate)) {
          return false;
        }
        if (endDate != null && log.timestamp.isAfter(endDate)) {
          return false;
        }

        return true;
      }).toList();

      // Sort by timestamp (newest first)
      filteredLogs.sort((a, b) => b.timestamp.compareTo(a.timestamp));

      // Apply limit
      if (limit != null && filteredLogs.length > limit) {
        filteredLogs = filteredLogs.take(limit).toList();
      }

      return Result.success(filteredLogs.cast<LogEntry>());
    } catch (e) {
      return Result.error('Failed to get logs: ${e.toString()}');
    }
  }

  // Private helper methods

  bool _shouldLog(LogLevel level) {
    return level.index >= _currentLogLevel.index;
  }

  // 🔍 DEBUG: Track sync flow
  @override
  Future<void> debugSync(String userId) async {
    await info('[SYNC] Starting backend sync', extra: {'userId': userId});
  }

  @override
  Future<void> debugSyncSuccess(String userId) async {
    await info('[SYNC] Backend sync succeeded', extra: {'userId': userId});
  }

  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {
    await error(
      '[SYNC] Backend sync failed',
      extra: {'userId': userId, 'error': errorMessage},
    );
  }

  @override
  Future<void> debugCallingGetCurrentUser() async {
    await info('[SYNC] Calling getCurrentUser()...');
  }

  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {
    await info(
      '[SYNC] getCurrentUser succeeded',
      extra: {'userId': userId, 'isEmailVerified': isEmailVerified},
    );
  }

  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {
    await error(
      '[SYNC] getCurrentUser failed',
      extra: {'userId': userId, 'error': errorMessage},
    );
  }

  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {
    await error(
      '[SYNC] Exception during sync',
      extra: {
        'userId': userId,
        'error': errorMessage,
        'stackTrace': stackTrace,
      },
    );
  }

  // Add router debug methods
  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {
    await info(
      '[ROUTER] Redirect check',
      extra: {
        'userId': userId,
        'isEmailVerified': isEmailVerified,
        'location': location,
        'isVerificationRoute': isVerificationRoute,
      },
    );
  }

  void _addLogEntry(LogEntryImpl entry) {
    _logs.add(entry);

    // Prevent memory issues by limiting log entries
    if (_logs.length > _maxLogEntries) {
      _logs.removeRange(0, _logs.length - _maxLogEntries);
    }
  }

  String _getCallerInfo() {
    try {
      final stackTrace = StackTrace.current;
      final stackLines = stackTrace.toString().split('\n');

      // Find the first line that's not from this logger class
      for (final line in stackLines) {
        if (!line.contains('LoggerService') && line.contains('dart:')) {
          // Extract method/class info from stack trace
          final match = RegExp(r'#\d+\s+(.+?)\s+\(').firstMatch(line);
          if (match != null) {
            return match.group(1) ?? 'Unknown';
          }
        }
      }

      return 'Unknown';
    } catch (e) {
      return 'Unknown';
    }
  }

  LogLevel _getLogLevelFromSeverity(String? severity) {
    switch (severity?.toLowerCase()) {
      case 'low':
        return LogLevel.info;
      case 'medium':
        return LogLevel.warning;
      case 'high':
        return LogLevel.error;
      case 'critical':
        return LogLevel.fatal;
      default:
        return LogLevel.info;
    }
  }

  Map<String, dynamic> _sanitizeData(Map<String, dynamic> data) {
    // Remove sensitive information from logs
    final sensitiveKeys = [
      'password',
      'token',
      'secret',
      'key',
      'authorization',
    ];
    final sanitized = Map<String, dynamic>.from(data);

    for (final key in sensitiveKeys) {
      if (sanitized.containsKey(key)) {
        sanitized[key] = '[REDACTED]';
      }
    }

    return sanitized;
  }

  @override
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {
    // Skip debug logs in release mode
    if (kReleaseMode && level == LogLevel.debug) return;

    // Use existing methods based on level
    switch (level) {
      case LogLevel.debug:
        await debug(message);
        break;
      case LogLevel.info:
        await info(message);
        break;
      case LogLevel.warning:
        await warning(message);
        break;
      case LogLevel.error:
        await error(message);
        break;
      case LogLevel.fatal:
        await fatal(message);
        break;
    }
  }
}

/// Implementation dari LogEntry abstract class
class LogEntryImpl implements LogEntry {
  @override
  final DateTime timestamp;

  @override
  final LogLevel level;

  @override
  final String message;

  @override
  final String? userId;

  @override
  final Map<String, dynamic>? extra;

  @override
  final StackTrace? stackTrace;

  @override
  final String source;

  const LogEntryImpl({
    required this.timestamp,
    required this.level,
    required this.message,
    required this.source,
    this.userId,
    this.extra,
    this.stackTrace,
  });

  @override
  String toString() {
    final buffer = StringBuffer();
    buffer.write('[${timestamp.toIso8601String()}] ');
    buffer.write('[${level.name.toUpperCase()}] ');
    buffer.write('[$source] ');
    buffer.write(message);

    if (userId != null) {
      buffer.write(' (User: $userId)');
    }

    if (extra != null && extra!.isNotEmpty) {
      buffer.write(' - Extra: $extra');
    }

    return buffer.toString();
  }
}
