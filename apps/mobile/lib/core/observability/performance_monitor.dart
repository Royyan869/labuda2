import 'package:firebase_performance/firebase_performance.dart';

/// Canonical performance-monitoring authority.
///
/// Boundary: performance monitoring answers "how FAST/HEALTHY is the app?".
/// It is NOT product analytics, crash reporting, or diagnostic logging.
///
/// Initialization enables Firebase Performance's native automatic collection
/// (app-start trace and native HTTP metrics). API latency for the Dart HTTP
/// client is reported by the single canonical Dio interceptor — never by
/// per-screen timers.
abstract final class PerformanceMonitoring {
  static bool _initialized = false;
  static FirebasePerformance? _performance;

  /// True once [initialize] has run.
  static bool get isInitialized => _initialized;

  /// The underlying instance, or null when not initialized (no-op).
  static FirebasePerformance? get instance => _performance;

  /// Initialize exactly once. Safe to call multiple times.
  static void initialize({FirebasePerformance? performance}) {
    if (_initialized) return;
    _initialized = true;
    try {
      _performance = performance ?? FirebasePerformance.instance;
    } catch (_) {
      // Firebase unavailable — the app continues without performance data.
      _performance = null;
    }
  }

  /// Start a canonical custom trace for a meaningful operation.
  ///
  /// Returns null when performance monitoring is not initialized; callers must
  /// treat the trace as optional.
  static Trace? startTrace(String name) {
    final performance = _performance;
    if (performance == null) return null;
    try {
      final trace = performance.newTrace(name);
      trace.start();
      return trace;
    } catch (_) {
      return null;
    }
  }

  /// Stop a trace previously returned by [startTrace].
  static Future<void> stopTrace(Trace? trace) async {
    if (trace == null) return;
    try {
      await trace.stop();
    } catch (_) {
      // A trace that fails to stop must never break the measured operation.
    }
  }
}
