import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:hishumi/core/common/result.dart';

/// Wrapper for the Firebase Analytics SDK.
///
/// **Tanggung jawab:**
/// - Wrap the Firebase Analytics SDK behind `Result<T>`.
/// - Sanitize event names to Firebase's naming restrictions.
/// - Stateless, no business logic.
class FirebaseAnalyticsService {
  final FirebaseAnalytics _analytics;

  FirebaseAnalyticsService(this._analytics);

  /// Log a custom event to Firebase Analytics.
  ///
  /// **Parameters:**
  /// - [eventName]: event name (max 40 chars, alphanumeric + underscore).
  /// - [parameters]: event parameters (max 25 params).
  Future<Result<void>> logEvent({
    required String eventName,
    Map<String, dynamic>? parameters,
  }) async {
    try {
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

  /// Set the SDK user id for Firebase Analytics.
  Future<Result<void>> setUserId(String? userId) async {
    try {
      await _analytics.setUserId(id: userId);
      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to set user ID: ${e.toString()}');
    }
  }

  /// Explicitly enable or disable analytics collection.
  ///
  /// Product analytics is a decided capability: collection is enabled
  /// explicitly at startup so the app never ships silently disabled.
  Future<Result<void>> setAnalyticsCollectionEnabled(bool enabled) async {
    try {
      await _analytics.setAnalyticsCollectionEnabled(enabled);
      return Result.success(null);
    } catch (e) {
      return Result.error(
        'Failed to set analytics collection enabled: ${e.toString()}',
      );
    }
  }

  /// Log a canonical screen view through the Firebase screen API.
  ///
  /// This is the canonical screen-tracking mechanism (reserved `screen_view`
  /// event with `firebase_screen`). Do NOT emit a custom event named
  /// `screen_view`.
  Future<Result<void>> logScreenView({
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
        'Failed to log screen view $screenName: ${e.toString()}',
      );
    }
  }

  /// Sanitize an event name for Firebase restrictions.
  ///
  /// Rules:
  /// - Max 40 characters.
  /// - Only alphanumeric and underscore.
  /// - Must not start with a number.
  String _sanitizeEventName(String name) {
    var sanitized = name
        .toLowerCase()
        .replaceAll(RegExp(r'[^a-z0-9_]'), '_')
        .replaceAll(RegExp(r'_+'), '_');

    sanitized = sanitized.replaceAll(RegExp(r'^_+|_+$'), '');

    if (sanitized.isNotEmpty && RegExp(r'^\d').hasMatch(sanitized)) {
      sanitized = 'event_$sanitized';
    }

    if (sanitized.length > 40) {
      sanitized = sanitized.substring(0, 40);
    }

    return sanitized.isNotEmpty ? sanitized : 'unknown_event';
  }
}
