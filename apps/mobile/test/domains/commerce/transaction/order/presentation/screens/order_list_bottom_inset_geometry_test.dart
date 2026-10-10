// SAFE-AREA-28 — ORDER LIST SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the single canonical bottom system-window authority on
// OrderListScreen (a STANDALONE pushed route — order_module/seller_module
// build it directly, so no MainScreen shell bar can own the inset):
//
//   * system bottom inset → the body `SafeArea` wrapping the `TabBarView`
//     (LIVE: the active tab's scroll viewport bottom tracks 0 / 24 / 34 /
//     48 as the window metrics change; a fixed clearance could not);
//   * design spacing      → the list `SliverPadding(AppMetrics.p16)` — a
//     constant measured BELOW the live inset, never a stand-in for it;
//   * top inset           → the primary AppBar + TabBar (the Scaffold
//     strips body top padding when an AppBar exists), asserted here as
//     "AppBar bottom == viewport top" so the body SafeArea adds no
//     phantom top space;
//   * keyboard            → N/A: the screen owns no text input.
//
// Geometry is measured on the REAL screen (active "All" tab, populated
// and empty states) with injected window metrics.
import 'package:flutter/material.dart' hide Action;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/src/auth/app_role.dart';
import 'package:hishumi/domains/commerce/transaction/order/order.dart'
    hide Action;
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/domains/user/identity/authentication/authentication.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/widgets/empty_state.dart';

const _currentUserId = 'user-list-1';

/// Header text of the LAST card: `order.id.substring(0, 8).toUpperCase()`
/// for id 'zz-lastcard-...' — the anchor for measuring the last meaningful
/// content bottom at scroll-end.
const _lastCardHeader = 'ZZ-LASTC';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.parse('2026-07-01T00:00:00.000Z');
    final user = AuthUser(
      id: _currentUserId,
      createdAt: now,
      updatedAt: now,
      email: 'buyer@example.com',
      username: 'buyer1',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: false,
      sellerSubscriptionStatus: 'none',
      hasMarketAuthority: false,
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

OrderItem _item() => OrderItem(
  id: 'item-1',
  productId: 'product-1',
  forSaleName: 'Koi Kohaku',
  forSaleImage: 'https://example.com/koi.jpg',
  price: 100000,
  quantity: 1,
);

Order _order({required String id, required OrderStatus status}) => Order(
  id: id,
  buyerId: _currentUserId,
  sellerId: 'seller-1',
  items: [_item()],
  status: status,
  paymentMethodCode: 'bank_transfer',
  paymentStatus: PaymentStatus.pending,
  shippingInfo: const ShippingInfo(
    recipientName: 'Buyer',
    phone: '08123456789',
    address: 'Some address',
    method: ShippingMethod.bus,
    shippingCost: 10000,
  ),
  pricing: const OrderPricing(
    subtotal: 100000,
    shippingCost: 10000,
    commissionAmount: 0,
    totalBeforeCoinsAmount: 110000,
    totalPayableAmount: 110000,
  ),
  createdAt: DateTime.utc(2026, 6, 1),
  source: OrderSource.forSale,
);

/// Enough cards that the active tab always exceeds one viewport, so the
/// scroll-end contract is exercised for real. Every id is at least 8 chars
/// (the screen header reads `order.id.substring(0, 8)`), and the LAST id
/// renders the [_lastCardHeader] anchor text.
List<Order> _orders(int count) => [
  for (var i = 1; i <= count; i++)
    i == count
        ? _order(id: 'zz-lastcard-$i', status: OrderStatus.completed)
        : _order(
            id: 'order-${i.toString().padLeft(2, '0')}',
            status: OrderStatus.values[(i - 1) % OrderStatus.values.length],
          ),
];

const _allStatuses = <OrderStatus?>[
  null,
  OrderStatus.pending,
  OrderStatus.paid,
  OrderStatus.shipped,
  OrderStatus.completed,
];

/// Injects window metrics on the TEST VIEW (same idiom as
/// SAFE-AREA-10/17/27) so every inset below is the REAL, LIVE one.
void _setInsets(WidgetTester tester, {required double bottom}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(top: 24 * dpr, bottom: bottom * dpr);
  tester.view.viewPadding = FakeViewPadding(
    top: 24 * dpr,
    bottom: bottom * dpr,
  );
  tester.view.viewInsets = const FakeViewPadding();
}

Future<void> _pump(WidgetTester tester, {required double inset}) async {
  addTearDown(tester.view.reset);
  _setInsets(tester, bottom: inset);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(_FakeAuthController.new),
        for (final status in _allStatuses)
          watchBuyerOrdersProvider(
            buyerId: _currentUserId,
            status: status,
          ).overrideWith((ref) => Stream.value(_orders(10))),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: const OrderListScreen(),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

ScrollableState _scrollableOf(WidgetTester tester) =>
    tester.state<ScrollableState>(
      find
          .descendant(
            of: find.byType(CustomScrollView),
            matching: find.byType(Scrollable),
          )
          .first,
    );

/// Scrolls to the REAL end: `maxScrollExtent` is an estimate until the
/// children at the new offset are laid out, so re-jump until stable.
Future<void> _scrollToEnd(WidgetTester tester) async {
  final ScrollableState scrollable = _scrollableOf(tester);
  for (var i = 0; i < 4; i++) {
    scrollable.position.jumpTo(scrollable.position.maxScrollExtent);
    await tester.pump();
  }
}

/// Measured geometry row at [inset], after scrolling to the very end.
/// Printed so every run documents the raw numbers behind the contract.
Future<Map<String, double>> _measure(
  WidgetTester tester, {
  required double inset,
}) async {
  final Rect surface = tester.getRect(find.byType(Scaffold));
  final Finder scrollFinder = find.byType(CustomScrollView);
  final Rect scrollRect = tester.getRect(scrollFinder);
  final Rect appBarRect = tester.getRect(find.byType(AppBar));

  final ScrollableState scrollable = _scrollableOf(tester);
  await _scrollToEnd(tester);

  // The last card's GestureDetector (its bounds include the card's p12
  // bottom margin — the gap below it to the scroll end is then exactly
  // the list's design SliverPadding p16).
  final Finder lastCardFinder = find.ancestor(
    of: find.text(_lastCardHeader),
    matching: find.byType(GestureDetector),
  );
  final Rect lastCard = tester.getRect(lastCardFinder);

  final double viewport = scrollRect.height;
  final double maxScroll = scrollable.position.maxScrollExtent;
  final double contentExtent = viewport + maxScroll;
  final double regionStart = surface.bottom - inset;
  final double gap = regionStart - lastCard.bottom;
  final double reachable =
      (lastCard.top >= scrollRect.top - 0.01 &&
          lastCard.bottom <= scrollRect.bottom + 0.01)
      ? 1
      : 0;

  // ignore: avoid_print
  print(
    'GEOM inset=$inset '
    'pixels=${scrollable.position.pixels.toStringAsFixed(2)} '
    'viewport=${viewport.toStringAsFixed(2)} '
    'maxScroll=${maxScroll.toStringAsFixed(2)} '
    'contentExtent=${contentExtent.toStringAsFixed(2)} '
    'lastCardBottom=${lastCard.bottom.toStringAsFixed(2)} '
    'scrollBottom=${scrollRect.bottom.toStringAsFixed(2)} '
    'surfaceBottom=${surface.bottom.toStringAsFixed(2)} '
    'systemRegionStart=${regionStart.toStringAsFixed(2)} '
    'gap=$gap reachable=$reachable '
    'appBarBottom=${appBarRect.bottom.toStringAsFixed(2)} '
    'scrollTop=${scrollRect.top.toStringAsFixed(2)}',
  );

  return <String, double>{
    'viewport': viewport,
    'maxScroll': maxScroll,
    'contentExtent': contentExtent,
    'lastCardBottom': lastCard.bottom,
    'scrollBottom': scrollRect.bottom,
    'surfaceBottom': surface.bottom,
    'regionStart': regionStart,
    'gap': gap,
    'reachable': reachable,
    'appBarBottom': appBarRect.bottom,
    'scrollTop': scrollRect.top,
  };
}

/// Populated-state contract at [inset]:
/// 1. the active tab's viewport follows the LIVE inset (body SafeArea);
/// 2. the AppBar/TabBar hands over the body with no phantom top space;
/// 3. the scroll path is real (maxScrollExtent > 0);
/// 4. at scroll-end the last card sits exactly the design `p16` ABOVE the
///    system region — never inside it, and the 16 px never grows into a
///    fixed inset stand-in;
/// 5. the last card is fully reachable inside the viewport.
Future<void> _expectLoadedGeometry(
  WidgetTester tester, {
  required double inset,
}) async {
  final Map<String, double> m = await _measure(tester, inset: inset);

  expect(
    m['scrollBottom'],
    closeTo(m['surfaceBottom']! - inset, 0.01),
    reason:
        'the active tab scroll viewport bottom must follow the live system '
        'inset ($inset) — the body SafeArea, not a fixed constant, owns the '
        'bottom inset',
  );

  expect(
    m['scrollTop'],
    closeTo(m['appBarBottom']!, 0.01),
    reason:
        'the body must start exactly at the AppBar/TabBar bottom — the body '
        'SafeArea must not add phantom top space (top ${m['scrollTop']} vs '
        'appBar bottom ${m['appBarBottom']})',
  );

  expect(
    m['maxScroll'],
    greaterThan(0),
    reason: 'the collection must actually scroll for the end contract to hold',
  );

  expect(
    m['gap'],
    closeTo(16, 0.01),
    reason:
        'at scroll-end the gap to the system region must be exactly the '
        'design SliverPadding (AppMetrics.p16 = 16) at inset $inset: '
        'negative would mean overlap (${m['gap']}), larger would mean a '
        'fixed clearance standing in for the live inset',
  );

  expect(
    m['lastCardBottom']!,
    lessThanOrEqualTo(m['regionStart']! + 0.01),
    reason:
        'the last card (${m['lastCardBottom']}) must stay outside the system '
        'region (start ${m['regionStart']}) at inset $inset',
  );

  expect(
    m['reachable'],
    1,
    reason:
        'the last card must be fully visible inside the viewport at '
        'scroll-end at inset $inset',
  );
}

void main() {
  group('SAFE-AREA-28 — populated tab: live inset geometry', () {
    testWidgets('inset 0 — design spacing only, no phantom reservation', (
      tester,
    ) async {
      await _pump(tester, inset: 0);
      await _expectLoadedGeometry(tester, inset: 0);
    });

    testWidgets('inset 24 — last card clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 24);
      await _expectLoadedGeometry(tester, inset: 24);
    });

    testWidgets('inset 34 — last card clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 34);
      await _expectLoadedGeometry(tester, inset: 34);
    });

    testWidgets('inset 48 — last card clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 48);
      await _expectLoadedGeometry(tester, inset: 48);
    });
  });

  group('SAFE-AREA-28 — authority proof', () {
    testWidgets(
      'the viewport follows the system inset 1:1 while the content extent '
      'stays constant (no double inset, no dead space)',
      (tester) async {
        await _pump(tester, inset: 0);
        final Map<String, double> at0 = await _measure(tester, inset: 0);

        // System bar appears (48 px): only a live authority moves the
        // active tab's viewport bottom boundary.
        _setInsets(tester, bottom: 48);
        await tester.pump();
        await tester.pump();
        final Map<String, double> at48 = await _measure(tester, inset: 48);

        expect(
          at0['scrollBottom']! - at48['scrollBottom']!,
          closeTo(48, 0.01),
          reason:
              'the scroll viewport bottom must follow the system inset — a '
              'fixed clearance would not move',
        );

        // The viewport shrinks exactly by the inset; the content extent is
        // invariant: no child scrollable (TabBarView page, nested CV)
        // absorbs the inset a second time (that would grow the content
        // extent by the same 48 px).
        expect(
          at48['viewport']!,
          closeTo(at0['viewport']! - 48, 0.01),
          reason: 'the viewport must shrink exactly by the live inset',
        );
        expect(
          at48['contentExtent']!,
          closeTo(at0['contentExtent']!, 0.01),
          reason:
              'the content extent must stay constant across insets — growth '
              'would prove a second, duplicate inset absorption',
        );

        // Exactly ONE canonical SafeArea authority owns the body scroll
        // geometry: count SafeArea ANCESTORS of the scrollable. (The
        // MaterialAppBar's internal SafeArea is `bottom: false` in a
        // sibling slot — it can never own the body's bottom inset.)
        int safeAreaAncestors = 0;
        tester.element(find.byType(CustomScrollView)).visitAncestorElements((
          element,
        ) {
          if (element.widget is SafeArea) safeAreaAncestors++;
          return true;
        });
        expect(
          safeAreaAncestors,
          1,
          reason:
              'exactly one SafeArea must sit between the active tab '
              'scrollable and the root — no duplicate ownership, none '
              'missing',
        );
      },
    );

    testWidgets('empty state shares the same viewport authority', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      _setInsets(tester, bottom: 34);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(_FakeAuthController.new),
            for (final status in _allStatuses)
              watchBuyerOrdersProvider(
                buyerId: _currentUserId,
                status: status,
              ).overrideWith((ref) => Stream.value(const <Order>[])),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('id'),
            home: const OrderListScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(EmptyState), findsOneWidget);

      final Rect surface = tester.getRect(find.byType(Scaffold));
      final Rect scrollRect = tester.getRect(find.byType(CustomScrollView));
      expect(
        scrollRect.bottom,
        closeTo(surface.bottom - 34, 0.01),
        reason:
            'the empty state (SliverFillRemaining) must end at the same '
            'SafeArea-owned boundary at inset 34',
      );
    });
  });
}
