import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('goRouterProvider mounts the Stack A screen view observer', () {
    final source = File(
      'lib/core/src/router/app_router.dart',
    ).readAsStringSync();

    expect(source, contains('screenViewRouteObserverProvider'));
    expect(source, isNot(contains('analyticsRouteObserverProvider')));
    expect(source, isNot(contains('AnalyticsRouteObserver')));
  });
}
