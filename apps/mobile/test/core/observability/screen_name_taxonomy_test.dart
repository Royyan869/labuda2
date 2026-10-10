import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/observability/screen_names.dart';

/// Locks the canonical screen taxonomy.
///
/// Owner-locked rule: a screen name describes the KIND of screen, never a
/// resource identity. No user/product/auction/order id may appear in a screen
/// name.
void main() {
  final routePathsSource = File(
    'lib/core/src/router/route_paths.dart',
  ).readAsStringSync();

  final routeNamesBlock = routePathsSource.substring(
    routePathsSource.indexOf('class RouteNames {'),
  );
  final routeNameValues = RegExp(
    r"static const String \w+ = '([^']+)';",
  ).allMatches(routeNamesBlock).map((m) => m.group(1)!).toList();

  test('every canonical RouteNames value has a screen-name mapping', () {
    expect(routeNameValues, isNotEmpty);
    final unmapped = routeNameValues
        .where((name) => !AnalyticsScreen.byRouteName.containsKey(name))
        .toList();
    expect(
      unmapped,
      isEmpty,
      reason: 'RouteNames without a canonical screen mapping: $unmapped',
    );
  });

  test('no screen name contains a resource identifier', () {
    final identifiers = RegExp(r'[0-9]|/');
    for (final entry in AnalyticsScreen.byRouteName.entries) {
      expect(
        identifiers.hasMatch(entry.value),
        isFalse,
        reason:
            'screen name "${entry.value}" for route "${entry.key}" looks like '
            'a resource identifier',
      );
    }
  });

  test('resolver maps a parameterized path to the product concept', () {
    expect(AnalyticsScreen.resolve('/user/12345'), AnalyticsScreen.userProfile);
    expect(
      AnalyticsScreen.resolve('/for-sale/abc-123'),
      AnalyticsScreen.productDetail,
    );
    expect(
      AnalyticsScreen.resolve('/auction/987'),
      AnalyticsScreen.auctionDetail,
    );
  });

  test('resolver never returns a value containing a path segment id', () {
    for (final path in <String>[
      '/user/42',
      '/for-sale/xyz',
      '/auction/1',
      '/orders/order-9',
      '/chat/room-7',
      '/seller/auctions/5/edit',
    ]) {
      final resolved = AnalyticsScreen.resolve(path);
      expect(
        resolved,
        isNot(contains('/')),
        reason: 'resolved "$resolved" for "$path" still contains a path',
      );
      expect(
        RegExp(r'\d').hasMatch(resolved),
        isFalse,
        reason: 'resolved "$resolved" for "$path" contains a digit/id',
      );
    }
  });

  test('unknown routes resolve to the unknown sentinel', () {
    expect(AnalyticsScreen.resolve('totallyUnknownRoute'), AnalyticsScreen.unknown);
    expect(AnalyticsScreen.resolve(null), AnalyticsScreen.unknown);
    expect(AnalyticsScreen.resolve(''), AnalyticsScreen.unknown);
  });
}
