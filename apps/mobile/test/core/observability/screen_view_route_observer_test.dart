import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/core/observability/screen_names.dart';
import 'package:hishumi/core/observability/screen_view_route_observer.dart';

class _RecordingAnalyticsRepository implements IAnalyticsRepository {
  final List<String> screens = <String>[];

  @override
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  }) async {
    return Result.success(null);
  }

  @override
  Future<Result<void>> logScreenView({
    required String screenName,
    String? screenClass,
  }) async {
    screens.add(screenName);
    return Result.success(null);
  }
}

void main() {
  test('ScreenViewRouteObserver emits the canonical screen name', () {
    final repo = _RecordingAnalyticsRepository();
    final observer = ScreenViewRouteObserver(repo);
    final route = MaterialPageRoute<void>(
      settings: const RouteSettings(name: RouteNames.auctionDetails),
      builder: (_) => const SizedBox.shrink(),
    );

    observer.didPush(route, null);

    expect(repo.screens, <String>[AnalyticsScreen.auctionDetail]);
  });

  test('ScreenViewRouteObserver never emits a resource identifier', () {
    final repo = _RecordingAnalyticsRepository();
    final observer = ScreenViewRouteObserver(repo);
    final route = MaterialPageRoute<void>(
      settings: const RouteSettings(name: '/user/12345'),
      builder: (_) => const SizedBox.shrink(),
    );

    observer.didPush(route, null);

    expect(repo.screens, <String>[AnalyticsScreen.userProfile]);
    expect(repo.screens.single, isNot(contains('12345')));
  });

  test('ScreenViewRouteObserver ignores an unknown route', () {
    final repo = _RecordingAnalyticsRepository();
    final observer = ScreenViewRouteObserver(repo);
    final route = MaterialPageRoute<void>(
      settings: const RouteSettings(name: 'someUnknownRoute'),
      builder: (_) => const SizedBox.shrink(),
    );

    observer.didPush(route, null);

    expect(repo.screens, isEmpty);
  });
}
