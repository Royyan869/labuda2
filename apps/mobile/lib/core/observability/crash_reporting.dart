import 'package:firebase_crashlytics/firebase_crashlytics.dart';
import 'package:flutter/foundation.dart';

/// Canonical crash/error monitoring authority.
///
/// Boundary: crash monitoring answers "what is BROKEN?". It is NOT product
/// analytics, performance monitoring, or diagnostic logging.
///
/// Only genuine crashes and unexpected errors are reported. Expected business
/// errors (validation, network, auth outcomes) are handled by their owning
/// domain and MUST NOT be reported here — turning them into crash noise would
/// hide the real crashes.
abstract final class CrashReporting {
  static bool _initialized = false;
  static FirebaseCrashlytics? _crashlytics;

  /// True once [initialize] has run.
  static bool get isInitialized => _initialized;

  /// Initialize exactly once.
  ///
  /// Wires Flutter framework errors to Crashlytics as fatal. Uncaught
  /// asynchronous errors are wired by the app's guarded zone through
  /// [recordFatal].
  static void initialize({FirebaseCrashlytics? crashlytics}) {
    if (_initialized) return;
    _initialized = true;

    final FirebaseCrashlytics instance;
    try {
      instance = crashlytics ?? FirebaseCrashlytics.instance;
    } catch (_) {
      // Firebase unavailable — the app continues without crash reporting.
      return;
    }
    _crashlytics = instance;

    FlutterError.onError = (FlutterErrorDetails details) {
      // Preserve the default console presentation for developers.
      FlutterError.presentError(details);
      // A Flutter framework error is fatal by definition.
      instance.recordFlutterFatalError(details);
    };
  }

  /// Report an unexpected, uncaught error as fatal.
  static void recordFatal(Object error, StackTrace stack) {
    _crashlytics?.recordError(error, stack, fatal: true);
  }

  /// Report an unexpected but non-fatal error (an operation that failed
  /// outside its expected error contract). Use sparingly.
  static Future<void> recordNonFatal(
    Object error,
    StackTrace? stack, {
    String? reason,
  }) async {
    await _crashlytics?.recordError(
      error,
      stack,
      reason: reason,
      fatal: false,
    );
  }
}
