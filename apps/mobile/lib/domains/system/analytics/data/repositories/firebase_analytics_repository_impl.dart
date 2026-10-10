import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/core/src/interfaces/services/i_analytics_repository.dart';
import '../services/firebase_analytics_service.dart';

/// Canonical implementation of [IAnalyticsRepository] backed by Firebase
/// Analytics.
///
/// **Tanggung jawab:**
/// - Implement the canonical analytics contract.
/// - Orchestrate calls to [FirebaseAnalyticsService].
/// - All data is sent to Firebase Analytics only.
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
      // Set the SDK user id once per event when provided. This is the single
      // identity authority for product analytics — callers must not also
      // duplicate the id into event parameters.
      if (userId != null) {
        final userIdResult = await _analyticsService.setUserId(userId);
        if (userIdResult.isError) {
          return userIdResult;
        }
      }

      return await _analyticsService.logEvent(
        eventName: eventName,
        parameters: parameters,
      );
    } catch (e) {
      return Result.error('Failed to log event $eventName: ${e.toString()}');
    }
  }

  @override
  Future<Result<void>> logScreenView({
    required String screenName,
    String? screenClass,
  }) {
    return _analyticsService.logScreenView(
      screenName: screenName,
      screenClassOverride: screenClass,
    );
  }
}
