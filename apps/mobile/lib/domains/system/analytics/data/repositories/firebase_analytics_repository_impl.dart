import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_analytics_repository.dart';
import '../services/firebase_analytics_service.dart';

/// Implementation dari IAnalyticsRepository menggunakan Firebase Analytics.
///
/// FIRESTORE SUNSET (2025-02-20): Firestore logging removed.
/// Now only uses Firebase Analytics for event tracking.
/// Circumvention stats are no longer tracked in Firestore.
///
/// **Tanggung jawab:**
/// - Implement IAnalyticsRepository contract
/// - Orchestrate calls ke FirebaseAnalyticsService
/// - All data is sent to Firebase Analytics only
///
/// **GUIDELINES compliance:**
/// - Repository returns `Result<T>` ✅
/// - Uses Firebase service wrapper ✅
/// - Max 250 lines ✅
class FirebaseAnalyticsRepositoryImpl implements IAnalyticsRepository {
  final FirebaseAnalyticsService _analyticsService;

  FirebaseAnalyticsRepositoryImpl(this._analyticsService);

  @override
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  }) async {
    try {
      // Set user ID if provided
      if (userId != null) {
        final userIdResult = await _analyticsService.setUserId(userId);
        if (userIdResult.isFailure) {
          return userIdResult;
        }
      }

      // Log the event
      return await _analyticsService.logEvent(
        eventName: eventName,
        parameters: parameters,
      );
    } catch (e) {
      return Result.error('Failed to log event $eventName: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> logUserAction(
    String action,
    String userId, {
    Map<String, dynamic>? extra,
  }) async {
    return logEvent(
      'user_action',
      parameters: {'action': action, 'user_id': userId, ...?extra},
      userId: userId,
    );
  }

  @override
  Future<Result<void>> logCircumventionAttempt(
    String content,
    String userId, {
    Map<String, dynamic>? extra,
  }) async {
    try {
      // Log to Firebase Analytics for reporting
      // Firestore detailed tracking removed - use Backend API for detailed logging
      return await logEvent(
        'circumvention_attempt',
        parameters: {
          'user_id': userId,
          'content_length': content.length,
          ...?extra,
        },
        userId: userId,
      );
    } catch (e) {
      return Result.error(
        'Failed to log circumvention attempt: ${e.toString()}',
      );
    }
  }

  @override
  Future<Result<void>> setUserProperties(
    Map<String, dynamic> properties,
  ) async {
    try {
      // Set each property individually
      for (final entry in properties.entries) {
        final result = await _analyticsService.setUserProperty(
          name: entry.key,
          value: entry.value?.toString(),
        );

        if (result.isFailure) {
          return result;
        }
      }

      return Result.success(null);
    } catch (e) {
      return Result.error('Failed to set user properties: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> trackEngagement({
    required String userId,
    required String contentId,
    required String contentType,
    required String engagementType,
    int? duration,
  }) async {
    return logEvent(
      'content_engagement',
      parameters: {
        'user_id': userId,
        'content_id': contentId,
        'content_type': contentType,
        'engagement_type': engagementType,
        'duration': ?duration,
      },
      userId: userId,
    );
  }

  @override
  Future<Result<AnalyticsCircumventionStats>> getCircumventionStats({
    required DateTime startDate,
    required DateTime endDate,
    String? userId,
    String? violationType,
  }) async {
    // FIRESTORE SUNSET (2025-02-20): Firestore stats query removed.
    // Use Backend API for detailed statistics.
    // Returning empty stats for now.
    return Result.success(
      const AnalyticsCircumventionStats(
        totalAttempts: 0,
        uniqueUsers: 0,
        violationTypes: {},
        dailyAttempts: {},
        averageConfidence: 0,
        blockedAttempts: 0,
        filteredAttempts: 0,
      ),
    );
  }

  @override
  Future<Result<void>> flush() async {
    // Firebase Analytics automatically handles batching and flushing
    // No explicit flush needed in current SDK
    return Result.success(null);
  }
}
