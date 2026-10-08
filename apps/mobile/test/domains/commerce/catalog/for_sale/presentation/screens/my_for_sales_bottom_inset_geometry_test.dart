// SAFE-AREA-32 — MY FOR SALES SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the two SEPARATE system-window authorities on MyForSalesScreen
// (a STANDALONE pushed route — for_sale_module registers `/seller/for-sale`
// as a top-level GoRoute with no shell, so no shell bar owns the inset):
//
//   * body content bottom inset → the body `SafeArea` wrapping the
//     `RefreshIndicator`/`CustomScrollView` (LIVE: the viewport bottom
//     tracks 0 / 24 / 34 / 48; a fixed clearance could not);
//   * FAB positioning inset     → Flutter Scaffold `endFloat`
//     (`minViewPadding.bottom` + 16 px margin) — measured SEPARATELY from
//     the body authority and never conflated with it: the body SafeArea
//     must not double-lift the FAB, and the FAB lift must not stand in
//     for body content inset;
//   * design spacing            → the list `SliverPadding(p16 all)` —
//     a constant measured BELOW the live inset;
//   * keyboard                  → N/A: the screen owns no text input
//     (create/detail forms are separate route surfaces).
//
// Geometry is measured on the REAL screen (populated, empty, loading and
// error states) with injected window metrics.
import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/core/src/auth/app_role.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/screens/my_for_sales_screen.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/seller_management_row.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/services/logger_service.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/loading_indicator.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';

const _uid = 'seller-1';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() =>
      AuthState.authenticated(_user(_uid), emailVerified: true);
}

AuthUser _user(String id) => AuthUser(
  id: id,
  createdAt: DateTime.utc(2026, 1, 1),
  updatedAt: DateTime.utc(2026, 1, 1),
  email: '$id@example.com',
  username: id,
  isEmailVerified: true,
  accountStatus: AccountStatus.active,
  hasSellerProfile: true,
  sellerSubscriptionStatus: 'active',
  hasMarketAuthority: true,
  roles: const [UserRole.user],
  provider: AuthProvider.email,
  lifecycle: ContentLifecycle.active,
);

ForSale _forSale(String id, ForSaleStatus status) => ForSale(
  forSaleId: id,
  title: 'Koi $id',
  description: 'desc',
  price: 100000,
  stock: 1,
  sellerId: _uid,
  status: status,
  createdAt: DateTime.utc(2026, 1, 1),
  updatedAt: DateTime.utc(2026, 1, 1),
);

/// Enough active listings that the list always exceeds one viewport. The
/// screen defaults to the `active` filter, so every item is visible without
/// touching the tabs.
List<ForSale> _active(int count) => [
  for (var i = 0; i < count; i++) _forSale('active-$i', ForSaleStatus.active),
];

/// Static repository: the geometry path only ever needs the ONE canonical
/// fetch (`getSellerForSales`); everything else fails loudly via noSuchMethod
/// instead of being masked.
class _StaticRepo implements ForSaleRepository {
  _StaticRepo._(this._handler);

  factory _StaticRepo.loaded(List<ForSale> items) =>
      _StaticRepo._((_) async => Result.success(items));

  factory _StaticRepo.error(String message) =>
      _StaticRepo._((_) async => Result.error(message));

  factory _StaticRepo.gated(Completer<Result<List<ForSale>>> gate) =>
      _StaticRepo._((_) => gate.future);

  final Future<Result<List<ForSale>>> Function(int call) _handler;

  @override
  Future<Result<List<ForSale>>> getSellerForSales(
    String sellerId, {
    int page = 1,
    int pageSize = 20,
    bool includeWithdrawn = false,
  }) => _handler(1);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

/// Injects window metrics on the TEST VIEW (same idiom as
/// SAFE-AREA-01/10/31) so every inset below is the REAL, LIVE one.
void _setInsets(WidgetTester tester, {required double bottom}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(top: 24 * dpr, bottom: bottom * dpr);
  tester.view.viewPadding = FakeViewPadding(
    top: 24 * dpr,
    bottom: bottom * dpr,
  );
  tester.view.viewInsets = const FakeViewPadding();
}

Future<void> _pump(
  WidgetTester tester, {
  required double inset,
  required ForSaleRepository repository,

  /// Gated (never-completing) loads run without settle so no animation is
  /// awaited: the loading state is measured from the static frames instead.
  bool settle = true,
}) async {
  addTearDown(tester.view.reset);
  _setInsets(tester, bottom: inset);

  await tester.pumpWidget(
    ProviderScope(
      // No retry: a static load must never schedule timers.
      retry: (retryCount, error) => null,
      overrides: [
        authControllerProvider.overrideWith(_FakeAuthController.new),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        forSaleRepositoryProvider.overrideWithValue(repository),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: const MyForSalesScreen(),
      ),
    ),
  );
  if (settle) await tester.pumpAndSettle();
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
/// Body numbers and FAB numbers are printed SEPARATELY.
Future<Map<String, double>> _measure(
  WidgetTester tester, {
  required double inset,
}) async {
  final Rect surfaceBox = tester.getRect(find.byType(Scaffold));
  final Rect scrollBox = tester.getRect(find.byType(CustomScrollView));
  final Rect appBarRect = tester.getRect(find.byType(AppBar));

  final ScrollableState scrollable = _scrollableOf(tester);
  await _scrollToEnd(tester);

  // The last For Sale card. Slivers build lazily, so the anchor is only
  // reliable AFTER the scroll-to-end above. SellerManagementRow's render
  // bounds include its own internal rhythm, so the gap below it at
  // scroll-end is exactly the list SliverPadding (p16).
  final Finder lastRowFinder = find.byType(SellerManagementRow);
  expect(lastRowFinder, findsWidgets, reason: 'the list must be populated');
  final Rect lastRow = tester.getRect(lastRowFinder.last);
  final Rect fab = tester.getRect(find.byType(FloatingActionButton));

  final double viewport = scrollBox.height;
  final double maxScroll = scrollable.position.maxScrollExtent;
  final double contentExtent = viewport + maxScroll;
  final double regionStart = surfaceBox.bottom - inset;
  final double gap = regionStart - lastRow.bottom;
  final double reachable =
      (lastRow.top >= scrollBox.top - 0.01 &&
          lastRow.bottom <= scrollBox.bottom + 0.01)
      ? 1
      : 0;
  final double fabClearance = regionStart - fab.bottom;

  // ignore: avoid_print
  print(
    'GEOM inset=$inset '
    'pixels=${scrollable.position.pixels.toStringAsFixed(2)} '
    'viewport=${viewport.toStringAsFixed(2)} '
    'maxScroll=${maxScroll.toStringAsFixed(2)} '
    'contentExtent=${contentExtent.toStringAsFixed(2)} '
    'lastRowBottom=${lastRow.bottom.toStringAsFixed(2)} '
    'scrollBottom=${scrollBox.bottom.toStringAsFixed(2)} '
    'surfaceBottom=${surfaceBox.bottom.toStringAsFixed(2)} '
    'systemRegionStart=${regionStart.toStringAsFixed(2)} '
    'gap=$gap reachable=$reachable '
    'appBarBottom=${appBarRect.bottom.toStringAsFixed(2)} '
    'scrollTop=${scrollBox.top.toStringAsFixed(2)} '
    'fabBottom=${fab.bottom.toStringAsFixed(2)} '
    'fabClearance=$fabClearance',
  );

  return <String, double>{
    'viewport': viewport,
    'maxScroll': maxScroll,
    'contentExtent': contentExtent,
    'lastRowBottom': lastRow.bottom,
    'scrollBottom': scrollBox.bottom,
    'surfaceBottom': surfaceBox.bottom,
    'regionStart': regionStart,
    'gap': gap,
    'reachable': reachable,
    'appBarBottom': appBarRect.bottom,
    'scrollTop': scrollBox.top,
    'fabBottom': fab.bottom,
    'fabClearance': fabClearance,
  };
}

Rect _scrollRectOf(WidgetTester tester) =>
    tester.getRect(find.byType(CustomScrollView));

double _surfaceBottomOf(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

/// Populated-state BODY contract at [inset], plus the SEPARATE FAB
/// contract (positioning authority stays the Scaffold's).
Future<void> _expectLoadedGeometry(
  WidgetTester tester, {
  required double inset,
}) async {
  final Map<String, double> m = await _measure(tester, inset: inset);

  // — body authority —
  expect(
    m['scrollBottom'],
    closeTo(m['surfaceBottom']! - inset, 0.01),
    reason:
        'the scroll viewport bottom must follow the live system inset '
        '($inset) — the body SafeArea, not a fixed constant, owns the '
        'bottom inset',
  );

  expect(
    m['scrollTop'],
    closeTo(m['appBarBottom']!, 0.01),
    reason:
        'the body must start exactly at the AppBar bottom — the body '
        'SafeArea must not add phantom top space (top ${m['scrollTop']} vs '
        'appBar bottom ${m['appBarBottom']})',
  );

  expect(
    m['maxScroll'],
    greaterThan(0),
    reason: 'the list must actually scroll for the end contract to hold',
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
    m['lastRowBottom']!,
    lessThanOrEqualTo(m['regionStart']! + 0.01),
    reason:
        'the last For Sale card (${m['lastRowBottom']}) must stay outside '
        'the system region (start ${m['regionStart']}) at inset $inset',
  );

  expect(
    m['reachable'],
    1,
    reason:
        'the last For Sale card must be fully visible inside the viewport '
        'at scroll-end at inset $inset',
  );

  // — FAB authority (SEPARATE from the body) —
  expect(
    m['fabBottom']!,
    lessThanOrEqualTo(m['regionStart']! + 0.01),
    reason:
        'the FAB (${m['fabBottom']}) must stay outside the system region '
        '(start ${m['regionStart']}) at inset $inset — its authority is '
        'the Scaffold endFloat lift, not the body SafeArea',
  );
}

/// Shared viewport contract for a `SliverFillRemaining` state: the scroll
/// viewport itself must end at the SafeArea boundary, so the fill content
/// can never spill into the system region.
void _expectFillViewportGeometry(
  WidgetTester tester, {
  required double inset,
  required String state,
}) {
  final double surface = _surfaceBottomOf(tester);
  expect(
    _scrollRectOf(tester).bottom,
    closeTo(surface - inset, 0.01),
    reason:
        'the $state state viewport must end exactly at the live system '
        'inset boundary ($inset) — the body SafeArea owns it in every state',
  );

  final Rect fab = tester.getRect(find.byType(FloatingActionButton));
  expect(
    fab.bottom,
    lessThanOrEqualTo(surface - inset + 0.01),
    reason: 'the FAB must stay outside the system region in the $state state',
  );
}

void main() {
  group('SAFE-AREA-32 — populated list: live inset geometry (body + FAB)', () {
    testWidgets('inset 0 — design spacing only, no phantom reservation', (
      tester,
    ) async {
      await _pump(
        tester,
        inset: 0,
        repository: _StaticRepo.loaded(_active(16)),
      );
      expect(find.byType(SellerManagementRow), findsWidgets);
      await _expectLoadedGeometry(tester, inset: 0);
    });

    testWidgets('inset 24 — last card and FAB clear the system region', (
      tester,
    ) async {
      await _pump(
        tester,
        inset: 24,
        repository: _StaticRepo.loaded(_active(16)),
      );
      await _expectLoadedGeometry(tester, inset: 24);
    });

    testWidgets('inset 34 — last card and FAB clear the system region', (
      tester,
    ) async {
      await _pump(
        tester,
        inset: 34,
        repository: _StaticRepo.loaded(_active(16)),
      );
      await _expectLoadedGeometry(tester, inset: 34);
    });

    testWidgets('inset 48 — last card and FAB clear the system region', (
      tester,
    ) async {
      await _pump(
        tester,
        inset: 48,
        repository: _StaticRepo.loaded(_active(16)),
      );
      await _expectLoadedGeometry(tester, inset: 48);
    });
  });

  group('SAFE-AREA-32 — states share the viewport authority', () {
    testWidgets('empty state at inset 0', (tester) async {
      await _pump(tester, inset: 0, repository: _StaticRepo.loaded(const []));
      expect(find.byType(EmptyState), findsOneWidget);
      _expectFillViewportGeometry(tester, inset: 0, state: 'empty');
    });

    testWidgets('empty state at inset 34', (tester) async {
      await _pump(tester, inset: 34, repository: _StaticRepo.loaded(const []));
      expect(find.byType(EmptyState), findsOneWidget);
      _expectFillViewportGeometry(tester, inset: 34, state: 'empty');

      final Rect empty = tester.getRect(find.byType(EmptyState));
      expect(
        empty.bottom,
        lessThanOrEqualTo(_scrollRectOf(tester).bottom + 0.01),
        reason: 'the empty state content must stay inside the viewport',
      );
    });

    testWidgets('loading state at inset 0', (tester) async {
      await _pump(
        tester,
        inset: 0,
        repository: _StaticRepo.gated(Completer<Result<List<ForSale>>>()),
        settle: false,
      );
      await tester.pump();
      await tester.pump();
      expect(find.byType(LoadingIndicator), findsOneWidget);
      _expectFillViewportGeometry(tester, inset: 0, state: 'loading');
    });

    testWidgets('loading state at inset 34', (tester) async {
      await _pump(
        tester,
        inset: 34,
        repository: _StaticRepo.gated(Completer<Result<List<ForSale>>>()),
        settle: false,
      );
      await tester.pump();
      await tester.pump();
      expect(find.byType(LoadingIndicator), findsOneWidget);
      _expectFillViewportGeometry(tester, inset: 34, state: 'loading');
    });

    testWidgets('error state at inset 0', (tester) async {
      await _pump(tester, inset: 0, repository: _StaticRepo.error('boom'));
      expect(find.byType(PageErrorState), findsOneWidget);
      _expectFillViewportGeometry(tester, inset: 0, state: 'error');
    });

    testWidgets('error state at inset 34', (tester) async {
      await _pump(tester, inset: 34, repository: _StaticRepo.error('boom'));
      expect(find.byType(PageErrorState), findsOneWidget);
      _expectFillViewportGeometry(tester, inset: 34, state: 'error');

      final Finder retry = find.widgetWithText(ElevatedButton, 'Coba Lagi');
      expect(retry, findsOneWidget);
      expect(
        tester.getRect(retry).bottom,
        lessThanOrEqualTo(_scrollRectOf(tester).bottom + 0.01),
        reason: 'the retry action must stay inside the viewport',
      );
    });
  });

  group('SAFE-AREA-32 — authority proof', () {
    testWidgets(
      'the viewport follows the system inset 1:1 while the content extent '
      'stays constant (no double inset, no dead space) and the FAB lift '
      'stays a live Scaffold authority',
      (tester) async {
        await _pump(
          tester,
          inset: 0,
          repository: _StaticRepo.loaded(_active(16)),
        );
        final Map<String, double> at0 = await _measure(tester, inset: 0);

        // System bar appears (48 px): a live body authority moves the
        // viewport, and the Scaffold's own FAB lift moves the FAB — each
        // exactly once.
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

        // FAB: live Scaffold endFloat lift — moves 1:1 with the inset and
        // is NOT double-lifted by the body SafeArea (its slot sits outside
        // the body).
        expect(
          at0['fabBottom']! - at48['fabBottom']!,
          closeTo(48, 0.01),
          reason:
              'the FAB must follow the live inset via the Scaffold lift — '
              'a fixed bottom offset would not move',
        );
        expect(
          at48['fabClearance']!,
          closeTo(at0['fabClearance']!, 0.01),
          reason:
              'the FAB clearance must be invariant across insets — growth '
              'would mean the body SafeArea double-lifts the FAB',
        );

        // Exactly ONE canonical SafeArea authority owns the BODY scroll
        // geometry: count SafeArea ANCESTORS of the scrollable. (The
        // MaterialAppBar's internal SafeArea is `bottom: false` in a
        // sibling slot; the FAB slot is outside the body entirely.)
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
              'exactly one SafeArea must sit between the scrollable and the '
              'root — no duplicate ownership, none missing',
        );
      },
    );

    testWidgets('the screen source owns one SafeArea and no residue', (
      tester,
    ) async {
      final String src = File(
        'lib/domains/commerce/catalog/for_sale/presentation/screens/'
        'my_for_sales_screen.dart',
      ).readAsStringSync();

      expect(
        'SafeArea('.allMatches(src).length,
        1,
        reason:
            'the screen must own exactly one SafeArea authority — no '
            'duplicate ownership, none missing',
      );

      const List<String> banned = <String>[
        'MediaQuery',
        'viewPadding',
        'viewInsets',
        'bottomNavigationBar',
        'BottomActionBar',
        'bottomBarClearance',
        'fabClearance',
      ];
      for (final String token in banned) {
        expect(
          src.contains(token),
          isFalse,
          reason:
              '`$token` must not appear in the screen: the body SafeArea is '
              'the ONE body inset authority and the Scaffold endFloat lift '
              'is the ONE FAB inset authority',
        );
      }
    });
  });
}
