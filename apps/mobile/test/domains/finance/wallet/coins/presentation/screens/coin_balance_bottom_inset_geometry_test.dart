// SAFE-AREA-33 — COIN BALANCE SCREEN: BOTTOM-INSET GEOMETRY.
//
// Locks the system-window authorities on CoinBalanceScreen (a STANDALONE
// pushed route — coins_module registers `/coins` as a top-level GoRoute,
// so no shell bar and no MainScreen bottomNavigationBar owns the inset):
//
//   * body content bottom inset → the ONE body `SafeArea` wrapping the
//     entire `body:` switch (LIVE: every state's body region ends at
//     `surface - inset` for 0 / 24 / 34 / 48; a fixed clearance could not);
//   * top inset                 → the primary AppBar (Scaffold removes the
//     top padding for its body slot when an AppBar exists, so the body
//     SafeArea sees padding.top == 0 → NO phantom top space is possible;
//     proven here anyway at every inset);
//   * FAB / bottom CTA          → NONE exist on this screen (proven by
//     absence in both the widget tree and the source);
//   * design spacing            → the list tail (Lihat Semua p16 padding +
//     32 spacer) — constants measured INSIDE the scroll content, proven
//     invariant across insets and separate from the system inset;
//   * keyboard                  → N/A: the screen owns no text input.
//
// Geometry is measured on the REAL screen (populated, transactions-empty,
// balance-empty, balance-loading and balance-error states) with injected
// window metrics.
import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/auth/app_role.dart';
import 'package:labuda/domains/finance/wallet/coins/coins_di.dart';
import 'package:labuda/domains/finance/wallet/coins/presentation/screens/coin_balance_screen.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart';

const _uid = 'user-1';

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

CoinBalance _balance() => CoinBalance(
  userId: _uid,
  balance: 1234,
  lifetimeEarned: 2000,
  lifetimeSpent: 766,
  createdAt: DateTime.utc(2026, 1, 1),
  updatedAt: DateTime.utc(2026, 1, 1),
  lastTransactionAt: DateTime.utc(2026, 1, 1),
);

CoinTransaction _tx(String id) => CoinTransaction(
  id: id,
  userId: _uid,
  type: CoinTransactionType.earn,
  sourceType: CoinSourceType.orderReward,
  amount: 50,
  balanceAfter: 1234,
  description: 'Reward $id',
  createdAt: DateTime.utc(2026, 1, 1),
);

/// Enough transactions that the list always exceeds one viewport, so the
/// scroll-end contract is exercised for real.
List<CoinTransaction> _txs(int count) => [
  for (var i = 0; i < count; i++) _tx('tx-$i'),
];

/// Fake repository: only the TWO stream reads are on the geometry path;
/// everything else fails loudly via noSuchMethod instead of being masked.
class _FakeCoinRepository implements CoinRepository {
  _FakeCoinRepository({
    required this.onWatchBalance,
    required this.onWatchTransactions,
  });

  final Stream<Result<CoinBalance>> Function(String userId) onWatchBalance;
  final Stream<Result<List<CoinTransaction>>> Function(String userId, int limit)
  onWatchTransactions;

  @override
  Stream<Result<CoinBalance>> watchCoinBalance(String userId) =>
      onWatchBalance(userId);

  @override
  Stream<Result<List<CoinTransaction>>> watchTransactions({
    required String userId,
    int limit = 50,
  }) => onWatchTransactions(userId, limit);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

/// Populated: balance + a full page of transactions.
_FakeCoinRepository _loaded({int transactions = 16}) => _FakeCoinRepository(
  onWatchBalance: (_) => Stream.value(Result.success(_balance())),
  onWatchTransactions: (_, _) =>
      Stream.value(Result.success(_txs(transactions))),
);

/// Transactions empty: the populated tree with the empty transactions tail.
_FakeCoinRepository _txEmpty() => _FakeCoinRepository(
  onWatchBalance: (_) => Stream.value(Result.success(_balance())),
  onWatchTransactions: (_, _) => Stream.value(Result.success(const [])),
);

/// Balance EMPTY state: a failed RESULT is mapped to `null` by the screen's
/// canonical stream mapping, which renders `_buildEmptyState`.
_FakeCoinRepository _balanceEmpty() => _FakeCoinRepository(
  onWatchBalance: (_) => Stream.value(Result.error('no balance')),
  onWatchTransactions: (_, _) => Stream.value(Result.success(const [])),
);

/// Balance ERROR state: the stream itself throws, so `balanceAsync.error`
/// renders `_buildError`.
_FakeCoinRepository _balanceError() => _FakeCoinRepository(
  onWatchBalance: (_) =>
      Stream<Result<CoinBalance>>.error(Exception('balance stream boom')),
  onWatchTransactions: (_, _) => Stream.value(Result.success(const [])),
);

/// Injects window metrics on the TEST VIEW (same idiom as
/// SAFE-AREA-01/10/31/32) so every inset below is the REAL, LIVE one.
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
  required CoinRepository repository,

  /// Gated (never-emitting) streams run without settle so the animating
  /// spinner is never awaited: the loading state is measured from the
  /// static frames instead.
  bool settle = true,
}) async {
  addTearDown(tester.view.reset);
  _setInsets(tester, bottom: inset);

  await tester.pumpWidget(
    ProviderScope(
      // No retry: a static load must never schedule timers.
      retry: (retryCount, error) => null,
      overrides: [
        authenticatedUserProvider.overrideWithValue(_user(_uid)),
        coinRepositoryProvider.overrideWithValue(repository),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: const CoinBalanceScreen(),
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

/// Measured populated-state geometry at [inset], after scrolling to the
/// very end. Body numbers and design-spacing numbers are printed
/// SEPARATELY; this screen has no FAB/CTA to measure.
Future<Map<String, double>> _measure(
  WidgetTester tester, {
  required double inset,
}) async {
  final Rect surfaceBox = tester.getRect(find.byType(Scaffold));
  final Rect scrollBox = tester.getRect(find.byType(CustomScrollView));
  final Rect appBarRect = tester.getRect(find.byType(AppBar));

  final ScrollableState scrollable = _scrollableOf(tester);
  await _scrollToEnd(tester);

  // The last meaningful content: the "Lihat Semua Transaksi" action at the
  // tail of the list (below it sit the design p16 padding and the 32 px
  // spacer — design spacing, NOT inset work).
  final Finder lastContentFinder = find.widgetWithText(
    TextButton,
    'Lihat Semua Transaksi',
  );
  expect(
    lastContentFinder,
    findsOneWidget,
    reason: 'the populated list must render its tail action',
  );
  final Rect lastContent = tester.getRect(lastContentFinder);

  final double viewport = scrollBox.height;
  final double maxScroll = scrollable.position.maxScrollExtent;
  final double contentExtent = viewport + maxScroll;
  final double regionStart = surfaceBox.bottom - inset;
  final double overlap = scrollBox.bottom - regionStart;
  final double designGap = scrollBox.bottom - lastContent.bottom;
  final double gapToRegion = regionStart - lastContent.bottom;
  final double reachable =
      (lastContent.top >= scrollBox.top - 0.01 &&
          lastContent.bottom <= scrollBox.bottom + 0.01)
      ? 1
      : 0;

  // ignore: avoid_print
  print(
    'GEOM inset=$inset '
    'pixels=${scrollable.position.pixels.toStringAsFixed(2)} '
    'viewport=${viewport.toStringAsFixed(2)} '
    'maxScroll=${maxScroll.toStringAsFixed(2)} '
    'contentExtent=${contentExtent.toStringAsFixed(2)} '
    'lastContentBottom=${lastContent.bottom.toStringAsFixed(2)} '
    'scrollBottom=${scrollBox.bottom.toStringAsFixed(2)} '
    'surfaceBottom=${surfaceBox.bottom.toStringAsFixed(2)} '
    'systemRegionStart=${regionStart.toStringAsFixed(2)} '
    'overlap=$overlap designGap=$designGap gapToRegion=$gapToRegion '
    'reachable=$reachable '
    'appBarBottom=${appBarRect.bottom.toStringAsFixed(2)} '
    'scrollTop=${scrollBox.top.toStringAsFixed(2)}',
  );

  return <String, double>{
    'viewport': viewport,
    'maxScroll': maxScroll,
    'contentExtent': contentExtent,
    'lastContentBottom': lastContent.bottom,
    'scrollBottom': scrollBox.bottom,
    'surfaceBottom': surfaceBox.bottom,
    'regionStart': regionStart,
    'overlap': overlap,
    'designGap': designGap,
    'gapToRegion': gapToRegion,
    'reachable': reachable,
    'appBarBottom': appBarRect.bottom,
    'scrollTop': scrollBox.top,
  };
}

Rect _scrollRectOf(WidgetTester tester) =>
    tester.getRect(find.byType(CustomScrollView));

double _surfaceBottomOf(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

double _appBarBottomOf(WidgetTester tester) =>
    tester.getRect(find.byType(AppBar)).bottom;

/// Populated-state BODY contract at [inset]. This screen has no FAB/CTA —
/// the only inset authority under test is the body SafeArea.
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
    m['overlap'],
    closeTo(0, 0.01),
    reason:
        'no part of the scroll viewport may sit inside the system region at '
        'inset $inset (overlap was ${m['overlap']})',
  );

  // Design spacing: p16 tail padding + the 32 px spacer below the last
  // meaningful content — a constant INSIDE the scroll content, measured
  // separately from the system inset.
  expect(
    m['designGap'],
    closeTo(48, 0.01),
    reason:
        'the design tail spacing (p16 padding + 32 spacer = 48) must be '
        'preserved unchanged at inset $inset (got ${m['designGap']})',
  );

  expect(
    m['gapToRegion'],
    closeTo(48, 0.01),
    reason:
        'the last meaningful content must sit exactly its design gap (48) '
        'above the system-region boundary at inset $inset — constant across '
        'insets proves the spacing is design, not inset arithmetic',
  );

  expect(
    m['lastContentBottom']!,
    lessThanOrEqualTo(m['regionStart']! + 0.01),
    reason:
        'the last meaningful content (${m['lastContentBottom']}) must stay '
        'outside the system region (start ${m['regionStart']}) at '
        'inset $inset',
  );

  expect(
    m['reachable'],
    1,
    reason:
        'the last meaningful content must be fully visible inside the '
        'viewport at scroll-end at inset $inset',
  );
}

/// Shared contract for a full-bleed (non-scroll) body branch: the body
/// child itself must end at the SafeArea boundary, so the state can never
/// spill into the system region — and must start at the AppBar bottom.
void _expectFillGeometry(
  WidgetTester tester, {
  required double inset,
  required String state,

  /// The state's content anchor: the body Center is resolved as its
  /// ancestor (Flutter's `Icon` embeds an internal 48×48 `Center` glyph
  /// box, so `find.byType(Center)` alone would be ambiguous).
  required Finder marker,
}) {
  final Finder centerFinder = find.ancestor(
    of: marker,
    matching: find.byType(Center),
  );
  expect(
    centerFinder,
    findsOneWidget,
    reason: 'the $state state must have exactly ONE Center above its content',
  );

  final Rect center = tester.getRect(centerFinder);
  final double surface = _surfaceBottomOf(tester);

  expect(
    center.bottom,
    closeTo(surface - inset, 0.01),
    reason:
        'the $state body region must end exactly at the live system inset '
        'boundary ($inset) — the body SafeArea owns it in every state',
  );

  expect(
    center.top,
    closeTo(_appBarBottomOf(tester), 0.01),
    reason:
        'the $state body region must start exactly at the AppBar bottom — '
        'no phantom top padding from the body SafeArea',
  );
}

void main() {
  group('SAFE-AREA-33 — populated list: live inset geometry (body)', () {
    testWidgets('inset 0 — design spacing only, no phantom reservation', (
      tester,
    ) async {
      await _pump(tester, inset: 0, repository: _loaded());
      expect(find.text('Transaksi Terbaru'), findsOneWidget);
      await _expectLoadedGeometry(tester, inset: 0);
    });

    testWidgets('inset 24 — content and viewport clear the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 24, repository: _loaded());
      await _expectLoadedGeometry(tester, inset: 24);
    });

    testWidgets('inset 34 — content and viewport clear the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 34, repository: _loaded());
      await _expectLoadedGeometry(tester, inset: 34);
    });

    testWidgets('inset 48 — content and viewport clear the system region', (
      tester,
    ) async {
      await _pump(tester, inset: 48, repository: _loaded());
      await _expectLoadedGeometry(tester, inset: 48);
    });
  });

  group('SAFE-AREA-33 — states share the viewport authority', () {
    testWidgets('transactions-empty state at inset 34', (tester) async {
      await _pump(tester, inset: 34, repository: _txEmpty());
      expect(find.text('Belum ada transaksi'), findsOneWidget);

      expect(
        _scrollRectOf(tester).bottom,
        closeTo(_surfaceBottomOf(tester) - 34, 0.01),
        reason:
            'the transactions-empty viewport must end at the live inset '
            'boundary — same authority as populated',
      );
      expect(
        tester.getRect(find.text('Belum ada transaksi')).bottom,
        lessThanOrEqualTo(_surfaceBottomOf(tester) - 34 + 0.01),
        reason:
            'the empty-transactions message must stay outside the system region',
      );
    });

    testWidgets('balance-empty state at inset 0', (tester) async {
      await _pump(tester, inset: 0, repository: _balanceEmpty());
      expect(find.text('Belum ada Coins'), findsOneWidget);
      _expectFillGeometry(
        tester,
        inset: 0,
        state: 'balance-empty',
        marker: find.text('Belum ada Coins'),
      );
    });

    testWidgets('balance-empty state at inset 34', (tester) async {
      await _pump(tester, inset: 34, repository: _balanceEmpty());
      expect(find.text('Belum ada Coins'), findsOneWidget);
      _expectFillGeometry(
        tester,
        inset: 34,
        state: 'balance-empty',
        marker: find.text('Belum ada Coins'),
      );
    });

    testWidgets('balance-loading state at inset 0', (tester) async {
      final controller = StreamController<Result<CoinBalance>>();
      addTearDown(controller.close);
      await _pump(
        tester,
        inset: 0,
        repository: _FakeCoinRepository(
          onWatchBalance: (_) => controller.stream,
          onWatchTransactions: (_, _) => Stream.value(Result.success(const [])),
        ),
        settle: false,
      );
      await tester.pump();
      await tester.pump();
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      _expectFillGeometry(
        tester,
        inset: 0,
        state: 'balance-loading',
        marker: find.byType(CircularProgressIndicator),
      );
    });

    testWidgets('balance-loading state at inset 34', (tester) async {
      final controller = StreamController<Result<CoinBalance>>();
      addTearDown(controller.close);
      await _pump(
        tester,
        inset: 34,
        repository: _FakeCoinRepository(
          onWatchBalance: (_) => controller.stream,
          onWatchTransactions: (_, _) => Stream.value(Result.success(const [])),
        ),
        settle: false,
      );
      await tester.pump();
      await tester.pump();
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      _expectFillGeometry(
        tester,
        inset: 34,
        state: 'balance-loading',
        marker: find.byType(CircularProgressIndicator),
      );
    });

    testWidgets('balance-error state at inset 0', (tester) async {
      await _pump(tester, inset: 0, repository: _balanceError());
      expect(find.text('Terjadi Kesalahan'), findsOneWidget);
      expect(find.widgetWithText(ElevatedButton, 'Coba Lagi'), findsOneWidget);
      _expectFillGeometry(
        tester,
        inset: 0,
        state: 'balance-error',
        marker: find.text('Terjadi Kesalahan'),
      );
    });

    testWidgets('balance-error state at inset 34', (tester) async {
      await _pump(tester, inset: 34, repository: _balanceError());
      expect(find.text('Terjadi Kesalahan'), findsOneWidget);
      expect(find.widgetWithText(ElevatedButton, 'Coba Lagi'), findsOneWidget);
      _expectFillGeometry(
        tester,
        inset: 34,
        state: 'balance-error',
        marker: find.text('Terjadi Kesalahan'),
      );
    });
  });

  group('SAFE-AREA-33 — authority proof', () {
    testWidgets(
      'the viewport follows the system inset 1:1 while the content extent '
      'stays constant (no double inset, no dead space) and no phantom top '
      'padding appears',
      (tester) async {
        await _pump(tester, inset: 0, repository: _loaded());
        final Map<String, double> at0 = await _measure(tester, inset: 0);

        // System bar appears (48 px): a live body authority moves the
        // viewport — exactly once.
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
        expect(
          at48['designGap']!,
          closeTo(at0['designGap']!, 0.01),
          reason:
              'the design tail spacing must be invariant — it is content '
              'spacing, not inset arithmetic',
        );
        expect(
          at48['scrollTop']!,
          closeTo(at48['appBarBottom']!, 0.01),
          reason: 'the AppBar top authority must be untouched by the fix',
        );

        // Exactly ONE canonical SafeArea authority owns the BODY geometry:
        // count SafeArea ANCESTORS of the scrollable. (The MaterialAppBar's
        // internal SafeArea is `bottom: false` in a sibling slot.)
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

    testWidgets('no FAB / bottom action exists — the body SafeArea is the ONLY '
        'bottom-inset authority', (tester) async {
      await _pump(tester, inset: 34, repository: _loaded());

      expect(find.byType(FloatingActionButton), findsNothing);
      expect(
        find.byWidgetPredicate(
          (w) => w is Scaffold && w.bottomNavigationBar != null,
        ),
        findsNothing,
      );
      expect(find.byType(SafeArea), findsWidgets);
    });

    testWidgets('the screen source owns one SafeArea and no residue', (
      tester,
    ) async {
      final String src = File(
        'lib/domains/finance/wallet/coins/presentation/screens/'
        'coin_balance_screen.dart',
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
        'FloatingActionButton',
      ];
      for (final String token in banned) {
        expect(
          src.contains(token),
          isFalse,
          reason:
              '`$token` must not appear in the screen: the body SafeArea is '
              'the ONE bottom inset authority and this screen owns no '
              'FAB/CTA slot at all',
        );
      }

      // No text input → the keyboard claim stays falsifiable.
      expect(
        src.contains('TextField'),
        isFalse,
        reason:
            'the screen owns no text input, so keyboard inset remains N/A — '
            'adding one requires re-auditing the keyboard authority',
      );
    });
  });
}
