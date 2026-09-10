// GUEST WELCOME → HOME navigation contract.
//
// Owner canonical truth (navigation scope):
//
//   Guest
//   → Welcome Screen
//   → tap Home icon
//   → /home (canonical Home — MainScreen/Home tab)
//
//   Home intent = /home = canonical Home
//   Home intent ≠ For Sale (/for-sale) destination
//   Home intent ≠ any legacy listing destination
//
// This file proves the Welcome Screen producer:
//   1. Positive: tapping the Home icon invokes the canonical Home navigation
//      (NavigationHandler.navigateToHome → /home).
//   2. Negative: the Home action no longer contains any For Sale / listing
//      navigation mapping (source-level lock).

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/navigation/navigation_handler.dart';
import 'package:labuda/core/navigation/navigation_provider.dart'
    show navigationHandlerProvider;
import 'package:labuda/domains/user/preference/onboarding/presentation/screens/welcome_screen.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Records every navigation call so tests can assert exactly which navigation
/// action the Welcome Home icon produced — and that no For Sale/listing
/// navigation was invoked.
class _RecordingNavigationHandler implements NavigationHandler {
  final List<String> invoked = <String>[];
  int homeCalls = 0;

  @override
  void navigateToHome() {
    homeCalls += 1;
    invoked.add('navigateToHome');
  }

  @override
  dynamic noSuchMethod(Invocation invocation) {
    invoked.add(invocation.memberName.toString());
    return super.noSuchMethod(invocation);
  }
}

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues(<String, Object>{});
  });

  group('Welcome Screen guest Home icon', () {
    testWidgets(
      'Guest taps Home icon → canonical Home navigation is invoked',
      (tester) async {
        final handler = _RecordingNavigationHandler();

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              navigationHandlerProvider.overrideWithValue(handler),
            ],
            child: const MaterialApp(home: WelcomeScreen()),
          ),
        );
        // Let the entrance animations finish before tapping.
        await tester.pumpAndSettle();

        // The Home affordance on the Welcome screen (top-left home icon).
        final homeIcon = find.byIcon(Icons.home_outlined);
        expect(homeIcon, findsOneWidget);
        expect(find.byTooltip('Home'), findsOneWidget);

        await tester.tap(homeIcon);
        await tester.pumpAndSettle();

        // POSITIVE: the Home action went through the canonical Home
        // navigation (navigateToHome → /home).
        expect(handler.homeCalls, 1, reason: 'Home icon must invoke Home nav');
        expect(
          handler.invoked,
          contains('navigateToHome'),
          reason: 'Home icon must produce the canonical Home action',
        );
      },
    );

    testWidgets(
      'Guest Home action must NOT invoke any For Sale / listing navigation',
      (tester) async {
        final handler = _RecordingNavigationHandler();

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              navigationHandlerProvider.overrideWithValue(handler),
            ],
            child: const MaterialApp(home: WelcomeScreen()),
          ),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.byIcon(Icons.home_outlined));
        await tester.pumpAndSettle();

        // The ONLY navigation produced by tapping Home is navigateToHome.
        expect(handler.invoked, ['navigateToHome']);

        // NEGATIVE: no commerce/For Sale navigation helper was called
        // (navigateToForSaleDetail / navigateToCreateForSale / ...).
        final forbiddenCommerceNav = handler.invoked.where(
          (name) =>
              name.toLowerCase().contains('forSale') ||
              name.toLowerCase().contains('listing'),
        );
        expect(
          forbiddenCommerceNav,
          isEmpty,
          reason: 'Home action must never resolve to a For Sale/listing '
              'destination',
        );
      },
    );
  });

  group('Welcome Screen source-level negative lock', () {
    test('welcome_screen.dart no longer maps Home to a For Sale route', () {
      final source = File(
        'lib/domains/user/preference/onboarding/presentation/screens/'
        'welcome_screen.dart',
      )
          .readAsStringSync()
          .replaceAll('\r\n', '\n');

      // Canonical producer: Home icon delegates to the canonical Home action.
      expect(source, contains('ref.navigation.navigateToHome()'));

      // Legacy Home → For Sale/listing mapping is gone (no hardcoded route,
      // no stale guidance comment, no "Explore as Guest" caption).
      expect(source, isNot(contains('RoutePaths.forSales')));
      expect(source, isNot(contains('context.go(RoutePaths.forSales)')));
      expect(source, isNot(contains('for-sale route for guest browsing')));
      expect(source, isNot(contains('Explore as Guest')));
    });
  });
}
