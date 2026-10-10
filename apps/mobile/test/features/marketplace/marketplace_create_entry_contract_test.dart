import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/features/home/home.dart';
import 'package:hishumi/features/marketplace/marketplace.dart';

// ============================================================================
// MARKETPLACE CREATE ENTRY CONTRACT (owner canonical, 2026-09-29)
//
// The marketplace surface carries ZERO create buttons. The single create
// entry is the bottom bar (main screen): the sheet pushes the create form
// DIRECTLY, and on success the create screens set a pending marketplace
// switch (for-sale → sub-tab 0, auction → sub-tab 1) so the user lands on
// the surface where the listing now lives.
//
// Three things this locks:
//
// 1. No create buttons on either marketplace tab (the regression the owner
//    rejected: "Buat Listing" / "Buat Lelang" inside the marketplace).
// 2. The bottom-bar sheet pushes the form; it does not merely move a tab and
//    leave the form one tap away.
// 3. Successful creates request the marketplace landing themselves — main
//    screen must not pre-set a switch before the form exists.
//
// The pending-switch plumbing (listener instead of a one-shot initState read)
// stays locked: this screen lives in an IndexedStack, so initState runs
// exactly once and later requests would be dropped.
//
// Comment and chat are NOT part of this contract: they are pickers that push
// the create screen directly and consume its result (no landing switch).
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

  testWidgets('the For Sale tab carries NO create button', (tester) async {
    await pumpScreen(tester, initialTab: 0);

    expect(find.text('Buat Listing'), findsNothing);
    expect(find.text('Buat Lelang'), findsNothing);
    expect(find.byType(FilledButton), findsNothing);
  });

  testWidgets('the Auction tab carries NO create button', (tester) async {
    await pumpScreen(tester, initialTab: 1);

    expect(find.text('Buat Lelang'), findsNothing);
    expect(find.text('Buat Listing'), findsNothing);
    expect(find.byType(FilledButton), findsNothing);
  });

  testWidgets('the pending switch still moves the marketplace sub-tab', (
    tester,
  ) async {
    final container = await pumpScreen(tester, initialTab: 0);

    // The exact request a successful auction create issues on pop.
    container
        .read(pendingTabSwitchProvider.notifier)
        .setSwitch('marketplace', subTab: 1);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    // Consumed by its one owner, so the same request cannot fire twice.
    expect(container.read(pendingTabSwitchProvider).hasSwitch, isFalse);
  });

  test('the bottom-bar sheet pushes the create forms directly', () {
    final main = File(
      'lib/features/home/presentation/screens/main_screen.dart',
    ).readAsStringSync();

    // The sheet entry points push the forms — no "land on the tab first" hop.
    expect(main.contains('context.push(RoutePaths.createForSale)'), isTrue);
    expect(main.contains('context.push(RoutePaths.createAuction)'), isTrue);

    // No pre-set landing switch before the form succeeds: the create screens
    // own the landing, not the sheet.
    expect(main.contains("setSwitch('marketplace'"), isFalse);

    // Negative proof: the helper seams stay dead so nothing can route around
    // the sheet (seller surfaces manage listings via their own push).
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

  test('successful creates land on the matching marketplace tab', () {
    final forSale = File(
      'lib/domains/commerce/catalog/for_sale/presentation/screens/create_for_sale_screen.dart',
    ).readAsStringSync();
    expect(
      forSale.contains("setSwitch('marketplace', subTab: 0)"),
      isTrue,
      reason: 'successful for-sale create must land on the For Sale tab',
    );

    final auction = File(
      'lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart',
    ).readAsStringSync();
    expect(
      auction.contains("setSwitch('marketplace', subTab: 1)"),
      isTrue,
      reason: 'successful auction create must land on the Auction tab',
    );
  });

  test('the marketplace listens for the switch instead of reading it once', () {
    final screen = File(
      'lib/features/marketplace/presentation/screens/marketplace_screen.dart',
    ).readAsStringSync();

    expect(
      screen.contains('ref.listen(pendingTabSwitchProvider, (previous, next)'),
      isTrue,
    );
    // Negative proof: no create buttons, no create routes on this surface.
    expect(screen.contains('RoutePaths.createForSale'), isFalse);
    expect(screen.contains('RoutePaths.createAuction'), isFalse);
    expect(screen.contains('Buat Listing'), isFalse);
    expect(screen.contains('Buat Lelang'), isFalse);
  });
}
