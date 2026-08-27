import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/observability/screen_view_route_observer.dart';

class _RecordingAnalyticsRepository implements IAnalyticsRepository {
  String? lastEventName;
  Map<String, dynamic>? lastParameters;
  String? lastUserId;

  @override
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  }) async {
    lastEventName = eventName;
    lastParameters = parameters;
    lastUserId = userId;
    return Result.success(null);
  }

  @override
  Future<Result<void>> flush() async => Result.success(null);

  @override
  Future<Result<AnalyticsCircumventionStats>> getCircumventionStats({
    required DateTime startDate,
    required DateTime endDate,
    String? userId,
    String? violationType,
  }) async {
    return Result.success(
      const AnalyticsCircumventionStats(
        totalAttempts: 0,
        uniqueUsers: 0,
        violationTypes: <String, int>{},
        dailyAttempts: <String, int>{},
        averageConfidence: 0,
        blockedAttempts: 0,
        filteredAttempts: 0,
      ),
    );
  }

  @override
  Future<Result<void>> logCircumventionAttempt(
    String content,
    String userId, {
    Map<String, dynamic>? extra,
  }) async {
    return Result.success(null);
  }

  @override
  Future<Result<void>> logUserAction(
    String action,
    String userId, {
    Map<String, dynamic>? extra,
  }) async {
    return Result.success(null);
  }

  @override
  Future<Result<void>> setUserProperties(
    Map<String, dynamic> properties,
  ) async {
    return Result.success(null);
  }

  @override
  Future<Result<void>> trackEngagement({
    required String userId,
    required String contentId,
    required String contentType,
    required String engagementType,
    int? duration,
  }) async {
    return Result.success(null);
  }
}

void main() {
  test(
    'ScreenViewRouteObserver emits screen_view through Stack A analytics',
    () {
      final repo = _RecordingAnalyticsRepository();
      final observer = ScreenViewRouteObserver(repo);
      final route = MaterialPageRoute<void>(
        settings: const RouteSettings(name: '/seller/promotions'),
        builder: (_) => const SizedBox.shrink(),
      );

      observer.didPush(route, null);

      expect(repo.lastEventName, 'screen_view');
      expect(repo.lastParameters?['screen_name'], 'promotions');
      expect(repo.lastParameters?['screen_path'], '/seller/promotions');
      expect(repo.lastParameters?['screen_class'], 'MaterialPageRoute<void>');
    },
  );
}
