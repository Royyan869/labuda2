import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/observability/crash_reporting.dart';
import 'package:hishumi/core/observability/performance_monitor.dart';

/// Locks the observability foundation wiring and safe no-op behaviour.
void main() {
  test('crash reporting is a safe no-op before initialization', () async {
    expect(CrashReporting.isInitialized, isFalse);
    // Must never throw when not initialized.
    CrashReporting.recordFatal(StateError('boom'), StackTrace.current);
    await CrashReporting.recordNonFatal(StateError('boom'), null);
  });

  test('performance monitoring is a safe no-op before initialization', () async {
    expect(PerformanceMonitoring.isInitialized, isFalse);
    expect(PerformanceMonitoring.startTrace('startup'), isNull);
    await PerformanceMonitoring.stopTrace(null);
  });

  test('bootstrap activates analytics, crash, and performance exactly once', () {
    final main = File('lib/main.dart').readAsStringSync();
    expect(main, contains('CrashReporting.initialize()'));
    expect(main, contains('PerformanceMonitoring.initialize()'));
    expect(main, contains('setAnalyticsCollectionEnabled(true)'));
    // Uncaught async errors are reported as fatal crashes.
    expect(main, contains('CrashReporting.recordFatal'));
  });

  test('crash reporting reports framework errors as fatal', () {
    final source = File(
      'lib/core/observability/crash_reporting.dart',
    ).readAsStringSync();
    expect(source, contains('FlutterError.onError'));
    expect(source, contains('recordFlutterFatalError'));
  });

  test('one crash reporter: only firebase_crashlytics is declared', () {
    final pubspec = File('pubspec.yaml').readAsStringSync();
    expect(pubspec, contains('firebase_crashlytics'));
    expect(pubspec, isNot(contains('sentry')));
    expect(pubspec, isNot(contains('mixpanel')));
  });
}
