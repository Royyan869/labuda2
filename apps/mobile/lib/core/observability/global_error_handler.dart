import 'dart:async';

import 'package:flutter/foundation.dart';
import 'crash_reporter.dart';

/// Global error handler that captures all uncaught errors
/// Integrates with CrashReporter (core interface) to report crashes
///
/// **Location:** lib/core/observability/global_error_handler.dart
/// **Replaces:** features/crashlytics/lib/presentation/handlers/global_error_handler.dart
class GlobalErrorHandler {
  final CrashReporter _crashReporter;

  GlobalErrorHandler(this._crashReporter);

  /// Initializes global error handling
  /// Call this in main() before runApp()
  void initialize() {
    // Catch Flutter framework errors
    FlutterError.onError = (FlutterErrorDetails details) {
      _handleFlutterError(details);
    };

    // Catch errors outside of Flutter framework
    PlatformDispatcher.instance.onError = (error, stack) {
      _handlePlatformError(error, stack);
      return true; // Prevent default error handling
    };
  }

  /// Handles Flutter framework errors
  void _handleFlutterError(FlutterErrorDetails details) {
    // Log to console in debug mode
    if (kDebugMode) {
      FlutterError.dumpErrorToConsole(details);
    }

    // Report to Crashlytics
    _crashReporter.recordError(
      Exception(details.exceptionAsString()),
      details.stack,
      reason: details.context?.toString(),
      fatal: !details.silent,
    );
  }

  /// Handles errors outside Flutter framework
  void _handlePlatformError(Object error, StackTrace stack) {
    // Log to console in debug mode
    if (kDebugMode) {
      debugPrint('Platform error: $error');
      debugPrint('Stack trace: $stack');
    }

    // Report to Crashlytics
    _crashReporter.recordError(
      error is Exception ? error : Exception(error.toString()),
      stack,
      reason: 'Platform error',
      fatal: true,
    );
  }

  /// Manually log an error
  Future<void> logError({
    required Exception exception,
    StackTrace? stackTrace,
    String? reason,
    Map<String, dynamic>? customKeys,
    bool fatal = false,
  }) async {
    // Set custom keys if provided
    if (customKeys != null) {
      for (final entry in customKeys.entries) {
        await _crashReporter.setCustomKey(entry.key, entry.value);
      }
    }

    await _crashReporter.recordError(
      exception,
      stackTrace,
      reason: reason,
      fatal: fatal,
    );
  }

  /// Wraps a function to catch and report errors
  Future<T?> catchErrors<T>(
    Future<T> Function() function, {
    String? context,
  }) async {
    try {
      return await function();
    } catch (e, stack) {
      await logError(
        exception: e is Exception ? e : Exception(e.toString()),
        stackTrace: stack,
        reason: context,
        fatal: false,
      );
      return null;
    }
  }
}
