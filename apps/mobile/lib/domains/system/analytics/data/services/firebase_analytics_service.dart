import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:labuda/core/common/result.dart';

/// Service untuk wrap Firebase Analytics SDK.
///
/// **Tanggung jawab:**
/// - Wrapper untuk Firebase Analytics SDK
/// - Convert Firebase exceptions ke `Result<T>`
/// - Stateless, no business logic
///
/// **GUIDELINES compliance:**
/// - Firebase SDK must be wrapped in service ✅
/// - Service is stateless ✅
/// - No business rules in Firebase layer ✅
class FirebaseAnalyticsService {
  final FirebaseAnalytics _analytics;

  FirebaseAnalyticsService(this._analytics);

  /// Log custom event ke Firebase Analytics.
  ///
  /// **Parameters:**
  /// - [eventName]: Nama event (max 40 characters, alphanumeric + underscore)
  /// - [parameters]: Event parameters (max 25 parameters, max 100 chars per value)
  Future<Result<void>> logEvent({
    required String eventName,
    Map<String, dynamic>? parameters,
  }) async {
    try {
      // Firebase Analytics event name restrictions:
      // - Max 40 characters
      // - Alphanumeric and underscore only
      final sanitizedName = _sanitizeEventName(eventName);

      await _analytics.logEvent(
        name: sanitizedName,
        parameters: parameters?.cast<String, Object>(),
      );

      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to log event $eventName: ${e.toString()}');
    }
  }

  /// Set user ID untuk Firebase Analytics.
  Future<Result<void>> setUserId(String? userId) async {
    try {
      await _analytics.setUserId(id: userId);
      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to set user ID: ${e.toString()}');
    }
  }

  /// Set user property untuk segmentation.
  ///
  /// **Parameters:**
  /// - [name]: Property name (max 24 characters)
  /// - [value]: Property value (max 36 characters)
  Future<Result<void>> setUserProperty({
    required String name,
    required String? value,
  }) async {
    try {
      final sanitizedName = _sanitizePropertyName(name);
      final sanitizedValue = value != null
          ? _sanitizePropertyValue(value)
          : null;

      await _analytics.setUserProperty(
        name: sanitizedName,
        value: sanitizedValue,
      );

      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to set user property $name: ${e.toString()}');
    }
  }

  /// Set current screen name untuk screen tracking.
  Future<Result<void>> setCurrentScreen({
    required String screenName,
    String? screenClassOverride,
  }) async {
    try {
      await _analytics.logScreenView(
        screenName: screenName,
        screenClass: screenClassOverride,
      );

      return Result.success(null);
    } catch (e) {
      return Result.error(
        'Failed to set current screen $screenName: ${e.toString()}',
      );
    }
  }

  /// Reset analytics data (untuk testing atau logout).
  Future<Result<void>> resetAnalyticsData() async {
    try {
      await _analytics.resetAnalyticsData();
      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to reset analytics data: ${e.toString()}');
    }
  }

  // Helper methods untuk sanitization

  /// Sanitize event name untuk Firebase restrictions.
  ///
  /// Rules:
  /// - Max 40 characters
  /// - Only alphanumeric and underscore
  /// - Must not start with number
  String _sanitizeEventName(String name) {
    // Replace spaces and special chars with underscore
    var sanitized = name
        .toLowerCase()
        .replaceAll(RegExp(r'[^a-z0-9_]'), '_')
        .replaceAll(RegExp(r'_+'), '_');

    // Remove leading/trailing underscores
    sanitized = sanitized.replaceAll(RegExp(r'^_+|_+$'), '');

    // Ensure doesn't start with number
    if (sanitized.isNotEmpty && RegExp(r'^\d').hasMatch(sanitized)) {
      sanitized = 'event_$sanitized';
    }

    // Truncate to 40 characters
    if (sanitized.length > 40) {
      sanitized = sanitized.substring(0, 40);
    }

    return sanitized.isNotEmpty ? sanitized : 'unknown_event';
  }

  /// Sanitize property name untuk Firebase restrictions.
  ///
  /// Rules:
  /// - Max 24 characters
  /// - Only alphanumeric and underscore
  String _sanitizePropertyName(String name) {
    var sanitized = name
        .toLowerCase()
        .replaceAll(RegExp(r'[^a-z0-9_]'), '_')
        .replaceAll(RegExp(r'_+'), '_')
        .replaceAll(RegExp(r'^_+|_+$'), '');

    if (sanitized.length > 24) {
      sanitized = sanitized.substring(0, 24);
    }

    return sanitized.isNotEmpty ? sanitized : 'unknown';
  }

  /// Sanitize property value untuk Firebase restrictions.
  ///
  /// Rules:
  /// - Max 36 characters
  String _sanitizePropertyValue(String value) {
    if (value.length > 36) {
      return value.substring(0, 36);
    }
    return value;
  }
}
