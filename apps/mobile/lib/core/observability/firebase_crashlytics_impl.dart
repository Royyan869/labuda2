import 'package:firebase_crashlytics/firebase_crashlytics.dart';
import 'crash_reporter.dart';

/// Firebase Crashlytics Implementation
///
/// **Location:** lib/core/observability/firebase_crashlytics_impl.dart
/// **Interface:** lib/core/observability/crash_reporter.dart
///
/// Firebase SDK is ONLY allowed in this file.
class FirebaseCrashlyticsImpl implements CrashReporter {
  final FirebaseCrashlytics _crashlytics;

  FirebaseCrashlyticsImpl(this._crashlytics);

  /// Factory for default instance
  factory FirebaseCrashlyticsImpl.instance() {
    return FirebaseCrashlyticsImpl(FirebaseCrashlytics.instance);
  }

  @override
  Future<void> recordError(
    Object error,
    StackTrace? stack, {
    bool fatal = false,
    String? reason,
  }) async {
    await _crashlytics.recordError(error, stack, reason: reason, fatal: fatal);
  }

  @override
  Future<void> setCustomKey(String key, dynamic value) async {
    _crashlytics.setCustomKey(key, value);
  }

  @override
  Future<void> setUserIdentifier(String userId) async {
    await _crashlytics.setUserIdentifier(userId);
  }

  @override
  Future<void> clearUserIdentifier() async {
    await _crashlytics.setUserIdentifier('');
  }

  @override
  Future<void> setCrashCollectionEnabled(bool enabled) async {
    await _crashlytics.setCrashlyticsCollectionEnabled(enabled);
  }

  @override
  Future<bool> isCrashCollectionEnabled() async {
    return _crashlytics.isCrashlyticsCollectionEnabled;
  }
}
