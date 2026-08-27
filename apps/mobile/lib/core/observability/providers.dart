import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:firebase_crashlytics/firebase_crashlytics.dart';
import 'package:firebase_performance/firebase_performance.dart';
import 'package:firebase_remote_config/firebase_remote_config.dart';
import 'package:firebase_messaging/firebase_messaging.dart';

import '../providers/core_providers.dart';
// Core interfaces
import 'crash_reporter.dart';
import 'performance_monitor.dart';
import '../experiment/experiment_service.dart';
import '../messaging/notification_service.dart';

// Firebase implementations
import 'firebase_crashlytics_impl.dart';
import 'firebase_performance_impl.dart';
import '../experiment/firebase_remote_config_impl.dart';
import '../messaging/fcm_service_impl.dart';

// Observers
import 'screen_view_route_observer.dart';
import 'performance_route_observer.dart';

// =============================================================================
// CRASHLYTICS
// =============================================================================

/// Provider for Firebase Crashlytics instance
final firebaseCrashlyticsProvider = Provider<FirebaseCrashlytics>((ref) {
  return FirebaseCrashlytics.instance;
});

/// Provider for CrashReporter (core interface)
final crashReporterProvider = Provider<CrashReporter>((ref) {
  final crashlytics = ref.watch(firebaseCrashlyticsProvider);
  return FirebaseCrashlyticsImpl(crashlytics);
});

// =============================================================================
// PERFORMANCE
// =============================================================================

/// Provider for Firebase Performance instance
final firebasePerformanceProvider = Provider<FirebasePerformance>((ref) {
  return FirebasePerformance.instance;
});

/// Provider for PerformanceMonitor (core interface)
final performanceMonitorProvider = Provider<PerformanceMonitor>((ref) {
  final performance = ref.watch(firebasePerformanceProvider);
  return FirebasePerformanceImpl(performance);
});

// =============================================================================
// EXPERIMENT / A-B TESTING
// =============================================================================

/// Provider for Firebase Remote Config instance
final firebaseRemoteConfigProvider = Provider<FirebaseRemoteConfig>((ref) {
  return FirebaseRemoteConfig.instance;
});

/// Provider for ExperimentService (core interface)
final experimentServiceProvider = Provider<ExperimentService>((ref) {
  final remoteConfig = ref.watch(firebaseRemoteConfigProvider);
  return FirebaseRemoteConfigImpl(remoteConfig);
});

// =============================================================================
// MESSAGING / NOTIFICATION
// =============================================================================

/// Provider for Firebase Messaging instance
final firebaseMessagingProvider = Provider<FirebaseMessaging>((ref) {
  return FirebaseMessaging.instance;
});

/// Provider for NotificationService (core interface)
///
/// FIRESTORE SUNSET (2025-02-20): Firestore removed from FCM service.
/// FCM operations only - no data storage.
final notificationServiceProvider = Provider<NotificationService>((ref) {
  final messaging = ref.watch(firebaseMessagingProvider);
  return FcmServiceImpl(messaging);
});

// =============================================================================
// OBSERVERS
// =============================================================================

/// Provider for Performance Route Observer
/// Automatically tracks screen rendering performance
final performanceRouteObserverProvider = Provider<CorePerformanceRouteObserver>(
  (ref) {
    final performance = ref.watch(performanceMonitorProvider);
    return CorePerformanceRouteObserver(performance);
  },
);

/// Provider for screen view tracking through the canonical Stack A analytics sink.
final screenViewRouteObserverProvider = Provider<ScreenViewRouteObserver>((
  ref,
) {
  final analytics = ref.watch(coreAnalyticsRepositoryProvider);
  return ScreenViewRouteObserver(analytics);
});

// =============================================================================
// CONVENIENCE EXPORTS
// =============================================================================

/// All core infra providers export
/// Use this to import everything at once:
/// `import 'package:labuda/core/infra_providers.dart';`
