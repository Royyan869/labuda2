import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/features/marketplace/marketplace.dart';

// ============================================================================
// MARKETPLACE CREATE ENTRY CONTRACT
//
// The Create FAB lands on the MARKETPLACE, on the tab that matches the choice,
// instead of pushing the form directly — the form sits one tap away behind the
// entry rendered here. Two things this locks, both of which were broken before:
//
// 1. The pending switch was read ONCE in initState. This screen lives in an
//    IndexedStack, so initState runs exactly once and every later request (the
//    FAB, a deep link) was dropped.
// 2. Nobody claimed the sub-tab: main screen cleared the pending switch in a
//    microtask while this screen read it in a post-frame callback.
//
// Comment and chat are NOT part of this contract: they are pickers that push
// the create screen directly and consume its result.
// ============================================================================

void main() {
  Future<ProviderContainer> pumpScreen(
    WidgetTester tester, {
    required int initialTab,
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          home: MarketplaceScreen(initialTab: initialTab),
        ),
      ),
    );
    // Never pumpAndSettle here: the grid shows an indeterminate spinner while
    // the list loads, so settling would spin forever.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    return ProviderScope.containerOf(
      tester.element(find.byType(MarketplaceScreen)),
    );
  }

  testWidgets('the create entry follows the active tab', (tester) async {
    final container = await pumpScreen(tester, initialTab: 0);

    expect(find.text('Buat Listing'), findsOneWidget);
    expect(find.text('Buat Lelang'), findsNothing);

    // The exact path the Create FAB takes for "Buat Lelang".
    container
        .read(pendingTabSwitchProvider.notifier)
        .setSwitch('marketplace', subTab: 1);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.text('Buat Lelang'), findsOneWidget);
    expect(find.text('Buat Listing'), findsNothing);

    // Consumed by its one owner, so the same request cannot fire twice.
    expect(container.read(pendingTabSwitchProvider).hasSwitch, isFalse);
  });

  testWidgets('the entry exists on the Auction tab from the start', (
    tester,
  ) async {
    await pumpScreen(tester, initialTab: 1);

    expect(find.text('Buat Lelang'), findsOneWidget);
    expect(find.text('Buat Listing'), findsNothing);
  });

  test('the create FAB lands on the marketplace, not on the form', () {
    final main = File(
      'lib/features/home/presentation/screens/main_screen.dart',
    ).readAsStringSync();

    expect(main.contains("setSwitch('marketplace', subTab: 0)"), isTrue);
    expect(main.contains("setSwitch('marketplace', subTab: 1)"), isTrue);

    // Negative proof: the FAB no longer pushes either create screen. The push
    // stays only where a caller NEEDS the result (comment/chat pickers) or on
    // seller surfaces that manage listings.
    expect(main.contains('navigation.navigateToCreateForSale'), isFalse);
    expect(main.contains('navigation.navigateToCreateAuction'), isFalse);

    // Stronger negative proof: the interface and its one implementation no
    // longer expose those helpers at all, so nothing can route through them.
    for (final path in const [
      'lib/core/navigation/navigation_handler.dart',
      'lib/core/src/router/app_router.dart',
    ]) {
      final source = File(path).readAsStringSync();
      expect(
        source.contains('navigateToCreateForSale'),
        isFalse,
        reason: '$path must not expose navigateToCreateForSale',
      );
      expect(
        source.contains('navigateToCreateAuction'),
        isFalse,
        reason: '$path must not expose navigateToCreateAuction',
      );
    }
  });

  test('the marketplace listens for the switch instead of reading it once', () {
    final screen = File(
      'lib/features/marketplace/presentation/screens/marketplace_screen.dart',
    ).readAsStringSync();

    expect(
      screen.contains('ref.listen(pendingTabSwitchProvider, (previous, next)'),
      isTrue,
    );
    expect(screen.contains('RoutePaths.createForSale'), isTrue);
    expect(screen.contains('RoutePaths.createAuction'), isTrue);
  });
}
