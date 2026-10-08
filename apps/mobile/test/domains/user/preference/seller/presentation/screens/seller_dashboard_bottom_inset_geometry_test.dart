// SAFE-AREA-34 — SELLER DASHBOARD SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the system-window authorities on SellerDashboardScreen — a FLAT
// top-level GoRoute (SellerModule → `/seller/dashboard`): no shell bar, no
// nested route, no ancestor SafeArea can own any inset for it.
//
//   * top inset         → the pinned SliverAppBar INSIDE the
//     CustomScrollView. Its embedded AppBar runs `primary: true`, so the
//     framework's own `SafeArea(bottom: false)` (app_bar.dart:1194) plus
//     the header's reserved top padding (app_bar.dart:2094) own the
//     status-bar region. Scaffold.appBar is null here, so the Scaffold
//     RETAINS top padding for the SliverAppBar to consume: the body
//     starts at y=0, the bar paints 0..(toolbar + top inset), and the
//     toolbar content sits below the status bar. No phantom top.
//   * body bottom inset  → the body `SafeArea` wrapping the
//     CustomScrollView (LIVE: the viewport bottom tracks 0/24/34/48;
//     a fixed clearance could not move).
//   * design spacing     → the content `Padding(p16)` tail — a constant
//     measured BELOW the live inset at scroll end.
//   * FAB / CTA inset    → ABSENT: no FloatingActionButton, no
//     bottomNavigationBar, no BottomActionBar on this screen (proved
//     absent below, each inset).
//   * keyboard           → N/A: the screen owns no text input.
//
// The three FILL branches (auth-required, seller-status loading,
// profile-required) are separate Scaffolds; each must keep its centered
// content outside the system region at every inset.
//
// Geometry is measured on the REAL screen with injected window metrics
// (top inset fixed at 24, bottom inset varied — the series idiom).
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart';
import 'package:labuda/shared/widgets/bottom_action_bar.dart';

const _sellerId = 'seller-geo-001';

/// Design tail below the last meaningful content of the populated
/// dashboard body: `Padding(all: AppMetrics.p16)` in `_buildContent`.
const double _designTail = 16;

/// Fixed TOP inset of the injected window (series idiom): the status-bar
/// region the SliverAppBar must clear.
const double _topInset = 24;

enum _Branch { main, authRequired, loading, profileRequired }

class _FakeLoggerService implements ILoggerService {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _StaticAuthController extends AuthController {
  _StaticAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

AuthUser _sellerUser({required bool hasSellerProfile}) => AuthUser(
  id: _sellerId,
  createdAt: DateTime.utc(2026, 7, 1),
  updatedAt: DateTime.utc(2026, 7, 1),
  email: 'seller@example.com',
  username: 'seller-geo',
  isEmailVerified: true,
  roles: const [UserRole.user],
  provider: AuthProvider.email,
  hasSellerProfile: hasSellerProfile,
  sellerSubscriptionStatus: 'active',
  hasMarketAuthority: true,
);

/// Two recent orders so the scroll tail renders the POPULATED state: the
/// last tile is the last meaningful content, and the design tail below it
/// is exactly the content `Padding(p16)`. (Leaving the recent provider to
/// the widget-test backend yields an error surface whose message text is
/// unbounded — it would dominate the content extent with environment
/// noise instead of dashboard content.)
Order _order({required String id, required OrderStatus status}) => Order(
  id: id,
  buyerId: 'buyer-001',
  sellerId: _sellerId,
  items: [
    OrderItem(
      id: 'item-$id',
      productId: 'product-$id',
      forSaleName: 'Kohaku 50cm',
      forSaleImage: 'https://example.com/$id.jpg',
      price: 100000,
    ),
  ],
  status: status,
  paymentMethodCode: 'bank_transfer',
  paymentStatus: PaymentStatus.pending,
  shippingInfo: const ShippingInfo(
    recipientName: 'Buyer',
    phone: '08123456789',
    address: 'Jl. Contoh 1',
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
  createdAt: DateTime.utc(2026, 7, 1, 9),
  source: OrderSource.forSale,
);

/// Injects window metrics on the TEST VIEW (same idiom as
/// SAFE-AREA-10/17/27/28/29/30/31/32/33) so every inset below is the
/// REAL, LIVE one.
void _setInsets(WidgetTester tester, {required double bottom}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(
    top: _topInset * dpr,
    bottom: bottom * dpr,
  );
  tester.view.viewPadding = FakeViewPadding(
    top: _topInset * dpr,
    bottom: bottom * dpr,
  );
  tester.view.viewInsets = const FakeViewPadding();
}

/// The default 600-logical test window is shorter than any supported
/// portrait device. On it, the profile-required fill column overflows at
/// EVERY inset — including inset 0 with nothing injected — so its bottom
/// boundary there is a pre-existing content-fit defect (RenderFlex
/// overflow), not an inset-ownership question: a bottom SafeArea cannot
/// move content whose children are laid from a region top the bottom
/// inset does not touch. Its inset contract is therefore measured on a
/// device-like surface (logical 800x900 at the default dpr 3), where the
/// branch lays out as designed.
void _useDeviceLikeHeight(WidgetTester tester) {
  tester.view.physicalSize = const Size(2400, 2700);
}

GoRouter _router() => GoRouter(
  initialLocation: RoutePaths.sellerDashboard,
  routes: [
    GoRoute(
      path: RoutePaths.sellerDashboard,
      builder: (context, state) => const SellerDashboardScreen(),
    ),
  ],
);

Future<void> _pump(
  WidgetTester tester, {
  required double inset,
  required _Branch branch,
}) async {
  addTearDown(tester.view.reset);
  _setInsets(tester, bottom: inset);
  if (branch == _Branch.profileRequired) {
    _useDeviceLikeHeight(tester);
  }

  final AuthUser snapshot = _sellerUser(
    hasSellerProfile: branch != _Branch.profileRequired,
  );
  final AuthState auth = switch (branch) {
    _Branch.authRequired => const AuthState.unauthenticated(),
    _Branch.main || _Branch.loading || _Branch.profileRequired =>
      AuthState.authenticated(snapshot, emailVerified: true),
  };
  // The loading branch keeps the session but has no backend snapshot yet,
  // so both seller axes report `unknown`.
  final AuthUser? exposed = switch (branch) {
    _Branch.main || _Branch.profileRequired => snapshot,
    _Branch.loading || _Branch.authRequired => null,
  };

  await tester.pumpWidget(
    ProviderScope(
      // No retry: a failing static load must never schedule timers.
      retry: (retryCount, error) => null,
      overrides: [
        authControllerProvider.overrideWith(() => _StaticAuthController(auth)),
        authenticatedUserProvider.overrideWith((ref) => exposed),
        loggerServiceProvider.overrideWithValue(_FakeLoggerService()),
        recentSellerOrdersProvider(_sellerId).overrideWith(
          (ref) async => [
            _order(id: 'order-geo-1', status: OrderStatus.paid),
            _order(id: 'order-geo-2', status: OrderStatus.delivered),
          ],
        ),
        for (final status in OrderStatus.values)
          if (status == OrderStatus.pending ||
              status == OrderStatus.paid ||
              status == OrderStatus.shipped ||
              status == OrderStatus.completed)
            watchSellerOrdersProvider(
              sellerId: _sellerId,
              status: status,
            ).overrideWith((ref) => Stream.value(const <Order>[])),
      ],
      child: MaterialApp.router(
        routerConfig: _router(),
        theme: ThemeData(useMaterial3: true),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
      ),
    ),
  );
  if (branch == _Branch.loading) {
    // The loading fill renders an indeterminate spinner, which
    // pumpAndSettle would wait on forever; bounded frames settle layout.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump(const Duration(milliseconds: 100));
  } else {
    await tester.pumpAndSettle();
  }
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

/// Measured geometry of the POPULATED scroll branch at [inset], after
/// scrolling to the very end. The last meaningful content is the bottom
/// of the body content Column (SliverToBoxAdapter child); the design tail
/// below it is the content `Padding(p16)`.
Future<Map<String, double>> _measureMain(
  WidgetTester tester, {
  required double inset,
}) async {
  expect(find.byType(Scaffold), findsOneWidget);
  final Rect surfaceBox = tester.getRect(find.byType(Scaffold));
  final Rect scrollBox = tester.getRect(find.byType(CustomScrollView));

  await _scrollToEnd(tester);

  // Pre-order traversal: the first Column under SliverToBoxAdapter is the
  // content column of `_buildContent` (everything else nests inside it).
  final Finder contentColumn = find.descendant(
    of: find.byType(SliverToBoxAdapter),
    matching: find.byType(Column),
  );
  expect(contentColumn, findsWidgets);
  final Rect column = tester.getRect(contentColumn.first);

  expect(find.byType(AppBar), findsOneWidget);
  final Rect appBar = tester.getRect(find.byType(AppBar));
  final Rect title = tester.getRect(find.text('Dashboard Penjual'));

  final ScrollableState scrollable = _scrollableOf(tester);
  final double viewport = scrollBox.height;
  final double maxScroll = scrollable.position.maxScrollExtent;
  final double contentExtent = viewport + maxScroll;
  final double regionStart = surfaceBox.bottom - inset;
  final double gap = regionStart - column.bottom;
  final double reachable =
      (column.bottom <= scrollBox.bottom + 0.01 &&
          column.bottom >= scrollBox.top)
      ? 1
      : 0;

  // ignore: avoid_print
  print(
    'GEOM branch=main inset=$inset '
    'pixels=${scrollable.position.pixels.toStringAsFixed(2)} '
    'viewport=${viewport.toStringAsFixed(2)} '
    'maxScroll=${maxScroll.toStringAsFixed(2)} '
    'contentExtent=${contentExtent.toStringAsFixed(2)} '
    'contentEnd=${column.bottom.toStringAsFixed(2)} '
    'scrollTop=${scrollBox.top.toStringAsFixed(2)} '
    'scrollBottom=${scrollBox.bottom.toStringAsFixed(2)} '
    'surfaceBottom=${surfaceBox.bottom.toStringAsFixed(2)} '
    'systemRegionStart=${regionStart.toStringAsFixed(2)} '
    'gap=$gap reachable=$reachable '
    'appBarTop=${appBar.top.toStringAsFixed(2)} '
    'appBarBottom=${appBar.bottom.toStringAsFixed(2)} '
    'titleTop=${title.top.toStringAsFixed(2)}',
  );

  return <String, double>{
    'viewport': viewport,
    'maxScroll': maxScroll,
    'contentExtent': contentExtent,
    'contentEnd': column.bottom,
    'scrollTop': scrollBox.top,
    'scrollBottom': scrollBox.bottom,
    'surfaceBottom': surfaceBox.bottom,
    'regionStart': regionStart,
    'gap': gap,
    'reachable': reachable,
    'appBarTop': appBar.top,
    'appBarBottom': appBar.bottom,
    'titleTop': title.top,
  };
}

/// Populated-state BODY contract at [inset]: the live body SafeArea owns
/// the bottom, the in-scroll SliverAppBar owns the top, and the design
/// tail sits below the live system region.
Future<void> _expectMainGeometry(
  WidgetTester tester, {
  required double inset,
}) async {
  final Map<String, double> m = await _measureMain(tester, inset: inset);

  // — body bottom authority —
  expect(
    m['scrollBottom'],
    closeTo(m['surfaceBottom']! - inset, 0.01),
    reason:
        'the scroll viewport bottom must follow the live system inset '
        '($inset) — the body SafeArea, not a fixed constant, owns the '
        'bottom inset (measured ${m['scrollBottom']} vs surface '
        '${m['surfaceBottom']})',
  );

  expect(
    m['gap'],
    closeTo(_designTail, 0.01),
    reason:
        'at scroll end the gap to the system region must be exactly the '
        'design content tail ($_designTail) at inset $inset: negative '
        'means overlap (${m['gap']}), larger means a fixed clearance '
        'standing in for the live inset',
  );

  expect(
    m['maxScroll'],
    greaterThan(0),
    reason: 'the dashboard must actually scroll for the end contract to hold',
  );

  expect(
    m['reachable'],
    1,
    reason:
        'the last dashboard content (${m['contentEnd']}) must be fully '
        'reachable inside the viewport at scroll end at inset $inset',
  );

  // — top authority: the in-scroll SliverAppBar (Scaffold.appBar is null
  // here, so the body keeps the top padding for the bar to consume) —
  expect(
    m['scrollTop'],
    closeTo(0, 0.01),
    reason:
        'the body must start at y=0 — the pinned SliverAppBar consumes '
        'the retained top padding itself; a phantom top reservation would '
        'push the scroll view down (measured ${m['scrollTop']})',
  );

  expect(
    m['appBarTop'],
    closeTo(m['scrollTop']!, 0.01),
    reason:
        'the app bar must start at the body top — the bar paints through '
        'the status-bar region itself',
  );

  expect(
    m['appBarBottom']! - m['appBarTop']!,
    closeTo(kToolbarHeight + _topInset, 0.01),
    reason:
        'the SliverAppBar header must reserve the top inset: collapsed '
        'height = toolbar (${kToolbarHeight.toStringAsFixed(0)}) + status '
        'bar ($_topInset) (measured ${m['appBarBottom']! - m['appBarTop']!})',
  );

  expect(
    m['titleTop'],
    greaterThanOrEqualTo(_topInset - 0.01),
    reason:
        'the bar title must sit below the status-bar region '
        '(title top ${m['titleTop']})',
  );
}

/// Measured geometry of a FILL branch at [inset]: the centered content
/// must stay clear of the system region and of the status bar.
Future<Map<String, double>> _measureFill(
  WidgetTester tester, {
  required double inset,
  required _Branch branch,
}) async {
  expect(find.byType(Scaffold), findsOneWidget);
  final Rect surfaceBox = tester.getRect(find.byType(Scaffold));

  late final Finder firstFinder;
  late final Finder lastFinder;
  switch (branch) {
    case _Branch.authRequired:
      firstFinder = find.byIcon(Icons.lock_outline);
      lastFinder = find.text('Silakan login untuk mengakses dashboard penjual');
    case _Branch.loading:
      firstFinder = find.byType(CircularProgressIndicator);
      lastFinder = find.text(
        'Menunggu identitas dan kapabilitas dari backend.',
      );
    case _Branch.profileRequired:
      firstFinder = find.byIcon(Icons.store_outlined);
      lastFinder = find.widgetWithText(ElevatedButton, 'Mulai Jualan');
    case _Branch.main:
      throw StateError('the main branch has its own measurement');
  }

  final Rect first = tester.getRect(firstFinder);
  final Rect last = tester.getRect(lastFinder);

  // Only the profile-required branch has an app bar; its body starts at
  // the bar bottom. The other fills own the full body.
  double topBoundary = 0;
  if (branch == _Branch.profileRequired) {
    expect(find.byType(AppBar), findsOneWidget);
    topBoundary = tester.getRect(find.byType(AppBar)).bottom;
  }

  final double regionStart = surfaceBox.bottom - inset;
  final double gap = regionStart - last.bottom;
  final double topGap = first.top - topBoundary;

  // ignore: avoid_print
  print(
    'GEOM branch=${branch.name} inset=$inset '
    'firstTop=${first.top.toStringAsFixed(2)} '
    'lastBottom=${last.bottom.toStringAsFixed(2)} '
    'surfaceBottom=${surfaceBox.bottom.toStringAsFixed(2)} '
    'systemRegionStart=${regionStart.toStringAsFixed(2)} '
    'gap=$gap topGap=${topGap.toStringAsFixed(2)} '
    'topBoundary=${topBoundary.toStringAsFixed(2)}',
  );

  return <String, double>{
    'firstTop': first.top,
    'lastBottom': last.bottom,
    'surfaceBottom': surfaceBox.bottom,
    'regionStart': regionStart,
    'gap': gap,
    'topGap': topGap,
  };
}

Future<void> _expectFillGeometry(
  WidgetTester tester, {
  required double inset,
  required _Branch branch,
}) async {
  final Map<String, double> m = await _measureFill(
    tester,
    inset: inset,
    branch: branch,
  );

  expect(
    m['lastBottom'],
    lessThanOrEqualTo(m['regionStart']! - 0.01),
    reason:
        'the ${branch.name} fill content (${m['lastBottom']}) must stay '
        'outside the system region (start ${m['regionStart']}) at inset '
        '$inset',
  );

  expect(
    m['topGap'],
    greaterThanOrEqualTo(-0.01),
    reason:
        'the ${branch.name} fill content must not intrude into the '
        'top boundary (gap ${m['topGap']})',
  );
}

void main() {
  group('SAFE-AREA-34 — populated dashboard: live inset geometry', () {
    testWidgets('inset 0 — design tail only, no phantom reservation', (
      tester,
    ) async {
      await _pump(tester, inset: 0, branch: _Branch.main);
      await _expectMainGeometry(tester, inset: 0);
    });

    testWidgets('inset 24 — content end clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 24, branch: _Branch.main);
      await _expectMainGeometry(tester, inset: 24);
    });

    testWidgets('inset 34 — content end clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 34, branch: _Branch.main);
      await _expectMainGeometry(tester, inset: 34);
    });

    testWidgets('inset 48 — content end clears the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 48, branch: _Branch.main);
      await _expectMainGeometry(tester, inset: 48);
    });
  });

  group('SAFE-AREA-34 — fill branches: live inset geometry', () {
    for (final inset in const <double>[0, 24, 34, 48]) {
      testWidgets('auth-required at inset $inset', (tester) async {
        await _pump(tester, inset: inset, branch: _Branch.authRequired);
        await _expectFillGeometry(
          tester,
          inset: inset,
          branch: _Branch.authRequired,
        );
      });

      testWidgets('seller-status loading at inset $inset', (tester) async {
        await _pump(tester, inset: inset, branch: _Branch.loading);
        await _expectFillGeometry(
          tester,
          inset: inset,
          branch: _Branch.loading,
        );
      });

      testWidgets('profile-required at inset $inset', (tester) async {
        await _pump(tester, inset: inset, branch: _Branch.profileRequired);
        await _expectFillGeometry(
          tester,
          inset: inset,
          branch: _Branch.profileRequired,
        );
      });
    }
  });

  group('SAFE-AREA-34 — authority proof', () {
    testWidgets(
      'the viewport follows the system inset 1:1 while the content extent '
      'stays constant (no double inset, no dead space)',
      (tester) async {
        await _pump(tester, inset: 0, branch: _Branch.main);
        final Map<String, double> at0 = await _measureMain(tester, inset: 0);

        // System bar appears (48 px): a live body authority moves the
        // viewport exactly once.
        _setInsets(tester, bottom: 48);
        await tester.pump();
        await tester.pump();
        final Map<String, double> at48 = await _measureMain(tester, inset: 48);

        expect(
          at0['scrollBottom']! - at48['scrollBottom']!,
          closeTo(48, 0.01),
          reason:
              'the scroll viewport bottom must follow the system inset — '
              'a fixed clearance would not move',
        );

        expect(
          at48['viewport']!,
          closeTo(at0['viewport']! - 48, 0.01),
          reason: 'the viewport must shrink exactly by the live inset',
        );

        expect(
          at48['contentExtent']!,
          closeTo(at0['contentExtent']!, 0.01),
          reason:
              'the content extent must stay constant across insets — '
              'growth would prove a second, duplicate inset absorption',
        );

        // Exactly ONE canonical SafeArea authority owns the BODY scroll
        // geometry: count SafeArea ANCESTORS of the scrollable. (The
        // SliverAppBar's internal SafeArea sits INSIDE the scroll view in
        // a descendant slot; it is not an ancestor of the scrollable.)
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
              'exactly one SafeArea must sit between the scrollable and '
              'the root — no duplicate ownership, none missing',
        );
      },
    );

    testWidgets('the screen owns no FAB, no bottom bar, no text input '
        '(their absence is the proof)', (tester) async {
      await _pump(tester, inset: 34, branch: _Branch.main);

      expect(
        find.byType(FloatingActionButton),
        findsNothing,
        reason: 'the dashboard owns no FAB — no FAB inset authority',
      );
      expect(
        find.byType(NavigationBar),
        findsNothing,
        reason: 'the dashboard route carries no bottom navigation bar',
      );
      expect(
        find.byType(BottomActionBar),
        findsNothing,
        reason: 'the dashboard owns no BottomActionBar CTA',
      );
      expect(
        find.byType(TextField),
        findsNothing,
        reason:
            'the dashboard owns no text input — keyboard inset is '
            'not in scope for this screen',
      );
      expect(
        find.byType(TextFormField),
        findsNothing,
        reason: 'the dashboard owns no form input',
      );
    });

    testWidgets('the production screen holds no second inset authority', (
      tester,
    ) async {
      await _pump(tester, inset: 34, branch: _Branch.main);

      final String src = File(
        'lib/domains/user/preference/seller/presentation/screens/'
        'seller_dashboard_screen.dart',
      ).readAsStringSync();

      expect(
        'SafeArea('.allMatches(src).length,
        1,
        reason:
            'the screen must own exactly one SafeArea authority — no '
            'duplicate ownership, none missing',
      );
      for (final token in const <String>[
        'MediaQuery',
        'viewPadding',
        'viewInsets',
        'bottomNavigationBar',
        'bottomBarClearance',
        'fabClearance',
        'FloatingActionButton',
      ]) {
        expect(
          token.allMatches(src).length,
          0,
          reason:
              'the screen must not carry a second inset authority or '
              'a fixed clearance ($token)',
        );
      }
    });
  });
}
