import 'package:firebase_performance/firebase_performance.dart';
import 'performance_monitor.dart';

/// Firebase Performance Implementation
///
/// **Location:** lib/core/observability/firebase_performance_impl.dart
/// **Interface:** lib/core/observability/performance_monitor.dart
///
/// Firebase SDK is ONLY allowed in this file.
class FirebasePerformanceImpl implements PerformanceMonitor {
  final FirebasePerformance _performance;
  final Map<String, Trace> _activeTraces = {};

  FirebasePerformanceImpl(this._performance);

  /// Factory for default instance
  factory FirebasePerformanceImpl.instance() {
    return FirebasePerformanceImpl(FirebasePerformance.instance);
  }

  @override
  Future<T> trace<T>(String name, Future<T> Function() action) async {
    final trace = _performance.newTrace(name);
    await trace.start();
    try {
      return await action();
    } finally {
      await trace.stop();
    }
  }

  @override
  Future<void> startTrace(String name) async {
    if (_activeTraces.containsKey(name)) {
      return; // Already started
    }
    final trace = _performance.newTrace(name);
    await trace.start();
    _activeTraces[name] = trace;
  }

  @override
  Future<void> stopTrace(String name) async {
    final trace = _activeTraces.remove(name);
    if (trace != null) {
      await trace.stop();
    }
  }

  @override
  void setTraceAttribute(String traceName, String key, String value) {
    final trace = _activeTraces[traceName];
    trace?.putAttribute(key, value);
  }

  @override
  void setTraceMetric(String traceName, String metricName, int value) {
    final trace = _activeTraces[traceName];
    trace?.setMetric(metricName, value);
  }

  @override
  Future<void> setPerformanceCollectionEnabled(bool enabled) async {
    await _performance.setPerformanceCollectionEnabled(enabled);
  }

  @override
  Future<bool> isPerformanceCollectionEnabled() async {
    return _performance.isPerformanceCollectionEnabled();
  }
}
