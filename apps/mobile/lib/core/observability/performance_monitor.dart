/// Core Performance Monitor Interface
///
/// Pure interface without Firebase dependencies.
/// Implementation will be provided by FirebasePerformanceImpl.
///
/// **Location:** lib/core/observability/performance_monitor.dart
/// **Implementation:** lib/core/observability/firebase_performance_impl.dart
abstract class PerformanceMonitor {
  /// Trace a function execution.
  Future<T> trace<T>(String name, Future<T> Function() action);

  /// Start a custom trace manually.
  Future<void> startTrace(String name);

  /// Stop a custom trace manually.
  Future<void> stopTrace(String name);

  /// Set attribute on active trace.
  void setTraceAttribute(String traceName, String key, String value);

  /// Set metric on active trace.
  void setTraceMetric(String traceName, String metricName, int value);

  /// Enable/disable performance monitoring.
  Future<void> setPerformanceCollectionEnabled(bool enabled);

  /// Check if performance collection is enabled.
  Future<bool> isPerformanceCollectionEnabled();
}
