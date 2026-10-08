import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// Locks the Analytics & Performance foundation convergence.
///
/// Proves: one authority, canonical taxonomy, truthful login semantics, no
/// dead analytics code, and a single screen-tracking authority.
void main() {
  final libFiles = Directory('lib')
      .listSync(recursive: true)
      .whereType<File>()
      .where((f) => f.path.endsWith('.dart'))
      .toList();

  String read(String path) => File(path).readAsStringSync();

  final authController = read(
    'lib/domains/user/identity/authentication/presentation/providers/'
    'auth_controller.dart',
  );
  final followActions = read(
    'lib/domains/social/follow/presentation/providers/'
    'follow_actions_provider.dart',
  );
  final observer = read(
    'lib/core/observability/screen_view_route_observer.dart',
  );

  test('no producer emits a raw event-name literal', () {
    final rawEventLiteral = RegExp(r"logEvent\(\s*'");
    for (final producer in <String>[authController, followActions]) {
      expect(
        rawEventLiteral.hasMatch(producer),
        isFalse,
        reason: 'producers must use AnalyticsEvents constants',
      );
    }
  });

  test('login is emitted only for an explicit user login', () {
    // The lifecycle switch exists and distinguishes login from restore/refresh.
    expect(authController, contains('AnalyticsEvents.login'));
    expect(authController, contains('AuthSyncOrigin.userLogin'));
    expect(authController, contains('AuthSyncOrigin.sessionRestore'));
    expect(authController, contains('AuthSyncOrigin.sessionRefresh'));
    expect(authController, contains('AnalyticsEvents.sessionRestored'));
    expect(authController, contains('AnalyticsEvents.sessionRefreshed'));

    // The listener path is a session restore, never a login.
    expect(authController, contains('origin: AuthSyncOrigin.sessionRestore'));
    // The refresh authority is a refresh, never a login.
    expect(authController, contains('origin: AuthSyncOrigin.sessionRefresh'));
  });

  test('the single analytics authority has no competing SDK entry point', () {
    final directSdk = libFiles
        .where((f) => f.readAsStringSync().contains('FirebaseAnalytics.instance'))
        .map((f) => f.path.replaceAll(r'\', '/'))
        .toList();

    // Only bootstrap wiring may touch the SDK instance directly.
    expect(directSdk, <String>['lib/main.dart']);
  });

  test('screen tracking has one authority', () {
    final router = read('lib/core/src/router/app_router.dart');
    expect(router, contains('screenViewRouteObserverProvider'));
    expect(router, isNot(contains('AnalyticsRouteObserver')));
    // The observer uses the canonical screen resolver and the screen API.
    expect(observer, contains('AnalyticsScreen.resolve'));
    expect(observer, contains('logScreenView'));
    expect(observer, isNot(contains("'screen_view'")));
  });

  test('no dead analytics instrumentation remains', () {
    const forbidden = <String>[
      'AnalyticsCircumventionStats',
      'logCircumventionAttempt',
      'getCircumventionStats',
      'trackEngagement',
      'setUserProperties',
      'analytics_event.dart',
    ];
    final offenders = <String>[];
    for (final file in libFiles) {
      final source = file.readAsStringSync();
      for (final name in forbidden) {
        if (source.contains(name)) {
          offenders.add('${file.path} names $name');
        }
      }
    }
    expect(offenders, isEmpty, reason: offenders.join('\n'));
    expect(
      File('lib/domains/system/analytics/domain/entities/analytics_event.dart')
          .existsSync(),
      isFalse,
    );
  });
}
