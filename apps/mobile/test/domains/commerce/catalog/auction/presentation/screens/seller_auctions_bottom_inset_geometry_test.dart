// SAFE-AREA-31 — SELLER AUCTIONS SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the two SEPARATE system-window authorities on SellerAuctionsScreen
// (a STANDALONE pushed route — auction_module builds it directly via
// MaterialPage, so no shell bar can own the inset):
//
//   * body content bottom inset → the body `SafeArea` wrapping the
//     `RefreshIndicator`/`CustomScrollView` (LIVE: the viewport bottom
//     tracks 0 / 24 / 34 / 48; a fixed clearance could not);
//   * FAB positioning inset     → Flutter Scaffold `endFloat`
//     (`minViewPadding.bottom` + 16 px margin) — measured SEPARATELY from
//     the body authority and never conflated with it: the body SafeArea
//     must not double-lift the FAB, and the FAB lift must not stand in
//     for body content inset;
//   * design spacing            → the list `SliverPadding(p16 bottom)` —
//     a constant measured BELOW the live inset;
//   * keyboard                  → N/A: the screen owns no text input
//     (create/edit/relist forms are separate route surfaces).
//
// Geometry is measured on the REAL screen (populated and empty states)
// with injected window metrics.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/seller_auctions_screen.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/seller_management_row.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/widgets/empty_state.dart';

const _sellerId = 'seller-1';

/// Static repository: only the pager's `getUserAuctions` is on the geometry
/// path. Everything else fails loudly via noSuchMethod instead of being
/// masked.
class _StaticAuctionRepository implements AuctionRepository {
  _StaticAuctionRepository(this._auctions);

  final List<Auction> _auctions;

  @override
  Future<Result<List<Auction>>> getUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 20,
    String? lastAuctionId,
  }) async => Result.success(_auctions);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeLoggerService implements ILoggerService {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

AuthUser _seller({required String id}) => AuthUser(
  id: id,
  createdAt: DateTime.utc(2026, 7, 1),
  updatedAt: DateTime.utc(2026, 7, 1),
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

Auction _auction({required String id, required AuctionStatus status}) =>
    Auction(
      id: id,
      sellerId: _sellerId,
      sellerUsername: 'seller',
      sellerFarmName: 'Farm',
      sellerAvatar: null,
      sellerUserLifecycle: ContentLifecycle.active,
      sellerTrustLifecycle: ContentLifecycle.active,
      title: 'Kohaku 50cm',
      description: 'desc',
      koiDetails: const KoiDetails(
        variety: 'Kohaku',
        sizeInCm: 50,
        ageInMonths: 12,
        gender: 'male',
      ),
      openingBid: 1000000,
      currentBid: 1200000,
      bidIncrement: 100000,
      startTime: DateTime.utc(2026, 7, 1, 8),
      endTime: DateTime.utc(2026, 7, 2, 8),
      status: status,
      createdAt: DateTime.utc(2026, 7, 1, 7),
    );

/// Enough auctions that the list always exceeds one viewport, but fewer
/// than the page size (20) so `hasMore` is false and the footer collapses
/// to `SizedBox.shrink()` — the last card is then the last meaningful
/// content, and the design gap below it is exactly the list SliverPadding
/// (AppMetrics.p16).
List<Auction> _auctions(int count) => [
  for (var i = 1; i <= count; i++)
    _auction(id: 'a$i', status: AuctionStatus.active),
];

/// Injects window metrics on the TEST VIEW (same idiom as
/// SAFE-AREA-10/17/27/28/29/30) so every inset below is the REAL, LIVE one.
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
  List<Auction>? auctions,
}) async {
  addTearDown(tester.view.reset);
  _setInsets(tester, bottom: inset);

  final _FakeLoggerService logger = _FakeLoggerService();
  await tester.pumpWidget(
    ProviderScope(
      // No retry: a static load must never schedule timers.
      retry: (retryCount, error) => null,
      overrides: [
        authControllerProvider.overrideWith(
          () => _FakeAuthController(
            AuthState.authenticated(
              _seller(id: _sellerId),
              emailVerified: true,
            ),
          ),
        ),
        auctionRepositoryProvider.overrideWithValue(
          _StaticAuctionRepository(auctions ?? _auctions(16)),
        ),
        loggerServiceProvider.overrideWithValue(logger),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: const SellerAuctionsScreen(),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
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

  // The last auction card. Slivers build lazily, so the anchor is only
  // reliable AFTER the scroll-to-end above. SellerManagementRow's render
  // bounds include its own internal margin/rhythm (bottom p12), so the gap
  // below it at scroll-end is exactly the list SliverPadding (p16).
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
        'the last auction card (${m['lastRowBottom']}) must stay outside '
        'the system region (start ${m['regionStart']}) at inset $inset',
  );

  expect(
    m['reachable'],
    1,
    reason:
        'the last auction card must be fully visible inside the viewport '
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

void main() {
  group('SAFE-AREA-31 — populated list: live inset geometry (body + FAB)', () {
    testWidgets('inset 0 — design spacing only, no phantom reservation', (
      tester,
    ) async {
      await _pump(tester, inset: 0);
      expect(find.byType(SellerManagementRow), findsWidgets);
      await _expectLoadedGeometry(tester, inset: 0);
    });

    testWidgets('inset 24 — last card and FAB clear the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 24);
      await _expectLoadedGeometry(tester, inset: 24);
    });

    testWidgets('inset 34 — last card and FAB clear the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 34);
      await _expectLoadedGeometry(tester, inset: 34);
    });

    testWidgets('inset 48 — last card and FAB clear the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 48);
      await _expectLoadedGeometry(tester, inset: 48);
    });
  });

  group('SAFE-AREA-31 — authority proof', () {
    testWidgets(
      'the viewport follows the system inset 1:1 while the content extent '
      'stays constant (no double inset, no dead space) and the FAB lift '
      'stays a live Scaffold authority',
      (tester) async {
        await _pump(tester, inset: 0);
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

        // Static source check: the screen owns exactly one SafeArea.
        final String src = File(
          'lib/domains/commerce/catalog/auction/presentation/screens/seller_auctions_screen.dart',
        ).readAsStringSync();
        expect(
          'SafeArea('.allMatches(src).length,
          1,
          reason:
              'the screen must own exactly one SafeArea authority — no '
              'duplicate ownership',
        );
      },
    );

    testWidgets('empty state shares the same viewport authority', (
      tester,
    ) async {
      await _pump(tester, inset: 34, auctions: const <Auction>[]);
      expect(find.byType(EmptyState), findsOneWidget);

      final double surface = _surfaceBottomOf(tester);
      expect(
        _scrollRectOf(tester).bottom,
        closeTo(surface - 34, 0.01),
        reason:
            'the empty state (SliverFillRemaining) must end at the same '
            'SafeArea-owned boundary at inset 34',
      );

      // The FAB keeps its own authority in the empty state too.
      final Rect fab = tester.getRect(find.byType(FloatingActionButton));
      expect(
        fab.bottom,
        lessThanOrEqualTo(surface - 34 + 0.01),
        reason: 'the FAB must stay outside the system region when empty',
      );
    });
  });
}
