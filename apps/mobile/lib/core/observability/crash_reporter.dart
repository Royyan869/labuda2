/// Core Crash Reporter Interface
///
/// Pure interface without Firebase dependencies.
/// Implementation will be provided by FirebaseCrashlyticsImpl.
///
/// **Location:** lib/core/observability/crash_reporter.dart
/// **Implementation:** lib/core/observability/firebase_crashlytics_impl.dart
abstract class CrashReporter {
  /// Record a non-fatal error.
  Future<void> recordError(
    Object error,
    StackTrace? stack, {
    bool fatal = false,
    String? reason,
  });

  /// Set a custom key for additional context.
  Future<void> setCustomKey(String key, dynamic value);

  /// Set user identifier for crash reports.
  Future<void> setUserIdentifier(String userId);

  /// Clear user identifier.
  Future<void> clearUserIdentifier();

  /// Enable/disable crash collection.
  Future<void> setCrashCollectionEnabled(bool enabled);

  /// Check if crash collection is enabled.
  Future<bool> isCrashCollectionEnabled();
}
