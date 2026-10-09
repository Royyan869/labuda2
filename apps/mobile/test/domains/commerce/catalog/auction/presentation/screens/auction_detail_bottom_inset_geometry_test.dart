// AUCTION DETAIL — BOTTOM-INSET GEOMETRY CONTRACT (detail body law).
//
// The law this file locks is IDENTICAL to the For Sale detail contract
// (`for_sale_detail_bottom_inset_geometry_test.dart`) — one detail layout
// law, two consumers:
//
//   State A — viewer (buyer): the CTA surface exists →
//       `Scaffold.bottomNavigationBar` is non-null, `BottomActionBar` owns
//       the LIVE system bottom inset exactly once (its own SafeArea), and
//       the body ends EXACTLY at the bar top (no body-side second
//       reservation).
//   State B — author/owner: NO viewer-directed bottom surface exists
//       (Owner truth, applied identically on both detail channels — the
//       author sees their own listing without a viewer CTA) → the slot is
//       NULL — never a zero-height stand-in — and the body `SafeArea` is
//       the sole bottom-inset authority.
//
// The framework MEDIATES between the two states: the Scaffold strips the
// body's bottom MediaQuery padding exactly when `bottomNavigationBar !=
// null` (material/scaffold.dart — `removeBottomPadding:
// widget.bottomNavigationBar != null`), so one unconditional body SafeArea
// consumes the inset when no bar exists and contributes NOTHING when the
// bar exists. There is no `if author` layout branch and no fixed clearance:
// the body widget is identical in both states.
//
// Geometry is measured on the REAL AuctionDetailScreen with injected window
// metrics (0 / 24 / 34 / 48), after scrolling to the real end.
import 'dart:io';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_recommendation_providers.dart'
    show ownerOtherAuctionsProvider, similarAuctionsProvider;
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_recommendations_section.dart';
import 'package:labuda/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository_provider.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/widgets/bottom_action_bar.dart';

const List<double> _insets = <double>[0, 24, 34, 48];

/// Design spacing below the last auction content (the recommendations
/// sliver rides `AppMetrics.p48` bottom padding — content-owned, NOT a
/// body inset).
const double _designGap = 48;

int _scopeGeneration = 0;

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeAuctionNotifier extends AuctionNotifier {
  _FakeAuctionNotifier(this._state);

  final AuctionNotifierState _state;

  @override
  AuctionNotifierState build() => _state;

  @override
  Future<void> loadAuctionDetails(String auctionId) async {}

  @override
  Future<void> loadAuctionBids(String auctionId, {int limit = 50}) async {}
}

class _FakeNavigationHandler extends Fake implements NavigationHandler {}

class _FakeSavedItemRepository implements SavedItemRepository {
  @override
  Future<bool> isSaved({
    required String targetType,
    required String targetId,
  }) async => false;

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

AuthUser _user(String id) => AuthUser(
  id: id,
  createdAt: DateTime.utc(2026, 1, 1),
  updatedAt: DateTime.utc(2026, 1, 1),
  email: '$id@example.com',
  username: id,
  isEmailVerified: true,
  roles: const [UserRole.user],
  provider: AuthProvider.email,
  lifecycle: ContentLifecycle.active,
);

const CommerceViewerCapabilities _buyerCaps = CommerceViewerCapabilities(
  role: 'buyer',
  canManage: false,
  canEdit: false,
  canPromote: false,
  canChat: true,
  canNegotiate: false,
  canBuy: false,
  canBid: true,
  canBuyNow: true,
);

const CommerceViewerCapabilities _ownerCaps = CommerceViewerCapabilities(
  role: 'owner',
  canManage: true,
  canEdit: false,
  canPromote: false,
  canChat: false,
  canNegotiate: false,
  canBuy: false,
  canBid: false,
  canBuyNow: false,
);

Auction _auction({
  required String sellerId,
  required CommerceViewerCapabilities capabilities,
}) {
  final DateTime now = DateTime.utc(2026, 1, 1);
  return Auction(
    id: 'geom-auction',
    sellerId: sellerId,
    sellerUsername: 'seller_user',
    sellerFarmName: 'Acme Farm',
    sellerAvatar: null,
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
    sellerTier: 'pro',
    viewerCapabilities: capabilities,
    title: 'Sanke Auction',
    description: 'Live auction',
    koiDetails: const KoiDetails(
      variety: 'Kohaku',
      sizeInCm: 30,
      ageInMonths: 12,
      gender: 'male',
      breeder: 'Hiro',
      bloodline: 'Miyabi',
      certificates: ['import', 'health'],
    ),
    preparationTime: null,
    openingBid: 1000000,
    currentBid: 1500000,
    bidIncrement: 50000,
    buyNowPrice: 2500000,
    media: const <MediaEntity>[],
    startTime: now,
    endTime: now.add(const Duration(days: 1)),
    status: AuctionStatus.active,
    createdAt: now,
    updatedAt: now,
    productId: 'product-1',
  );
}

/// Window metrics on the TEST VIEW (same idiom as SAFE-AREA-01/09/32), so
/// every inset below is the REAL, LIVE one. `MediaQuery.padding` is derived
/// from viewPadding − viewInsets on a real device, so the keyboard case
/// collapses the covered system inset exactly like the engine does.
void _setWindowInsets(
  WidgetTester tester, {
  double systemBottom = 0,
  double keyboard = 0,
}) {
  final double dpr = tester.view.devicePixelRatio;
  final double derivedBottom = math.max(0, systemBottom - keyboard);
  tester.view.padding = FakeViewPadding(
    top: 24 * dpr,
    bottom: derivedBottom * dpr,
  );
  tester.view.viewPadding = FakeViewPadding(
    top: 24 * dpr,
    bottom: systemBottom * dpr,
  );
  tester.view.viewInsets = FakeViewPadding(bottom: keyboard * dpr);
}

Future<void> _pump(
  WidgetTester tester, {
  required double inset,
  required AuthState auth,
  required Auction auction,
  double keyboard = 0,
}) async {
  addTearDown(tester.view.reset);
  _setWindowInsets(tester, systemBottom: inset, keyboard: keyboard);

  await tester.pumpWidget(
    ProviderScope(
      // A fresh key per pump guarantees a fresh container, so re-pumping
      // with different overrides cannot observe the previous pump's state.
      key: ValueKey('auction-detail-geom-${_scopeGeneration++}'),
      overrides: [
        authControllerProvider.overrideWith(() => _FakeAuthController(auth)),
        auctionNotifierProvider.overrideWith(
          () => _FakeAuctionNotifier(
            AuctionNotifierState(selectedAuction: auction),
          ),
        ),
        auctionStreamProvider(
          auction.id,
        ).overrideWith((ref) => Stream<Auction?>.value(auction)),
        auctionBidsStreamProvider(auction.id).overrideWith(
          (ref) => Stream<List<AuctionBid>>.value(const <AuctionBid>[]),
        ),
        ownerOtherAuctionsProvider(
          auction.id,
        ).overrideWith((ref) async => const <Auction>[]),
        similarAuctionsProvider(
          auction.id,
        ).overrideWith((ref) async => const <Auction>[]),
        navigationHandlerProvider.overrideWithValue(_FakeNavigationHandler()),
        savedItemRepositoryProvider.overrideWithValue(
          _FakeSavedItemRepository(),
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.lightTheme,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: AuctionDetailScreen(auctionId: auction.id),
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

Scaffold _scaffold(WidgetTester tester) =>
    tester.widget<Scaffold>(find.byType(Scaffold));

int _safeAreaAncestorsOfScrollable(WidgetTester tester) {
  int count = 0;
  tester.element(find.byType(CustomScrollView)).visitAncestorElements((
    element,
  ) {
    if (element.widget is SafeArea) count++;
    return true;
  });
  return count;
}

/// Measured geometry row at [inset], after scrolling to the very end.
Future<Map<String, double>> _measure(
  WidgetTester tester, {
  required double inset,
  required Finder lastContent,
}) async {
  await _scrollToEnd(tester);

  final Rect surface = tester.getRect(find.byType(Scaffold));
  final Rect appBar = tester.getRect(find.byType(AppBar));
  final Rect scrollBox = tester.getRect(find.byType(CustomScrollView));
  final Rect last = tester.getRect(lastContent);
  final ScrollableState scrollable = _scrollableOf(tester);

  final Finder barFinder = find.byType(BottomActionBar);
  final bool slotNonNull = _scaffold(tester).bottomNavigationBar != null;
  final bool barPresent = barFinder.evaluate().isNotEmpty;

  double? barTop;
  double? barBottom;
  double? ctaBottom;
  if (barPresent) {
    final Rect bar = tester.getRect(barFinder);
    barTop = bar.top;
    barBottom = bar.bottom;
    final Finder cta = find.descendant(
      of: barFinder,
      matching: find.byType(ElevatedButton),
    );
    if (cta.evaluate().isNotEmpty) {
      ctaBottom = tester.getRect(cta.first).bottom;
    }
  }

  final double viewport = scrollBox.height;
  final double maxScroll = scrollable.position.maxScrollExtent;
  final double contentExtent = viewport + maxScroll;
  final double regionStart = surface.bottom - inset;
  final double scrollBottom = scrollBox.bottom;
  final double gap = regionStart - last.bottom;
  final double gapToScroll = scrollBottom - last.bottom;
  final double reachable =
      (last.top >= scrollBox.top - 0.01 &&
          last.bottom <= scrollBox.bottom + 0.01)
      ? 1
      : 0;

  // ignore: avoid_print
  print(
    'AUCTION-GEOM inset=$inset '
    'slotNonNull=${slotNonNull ? 1 : 0} bar=${barPresent ? 1 : 0} '
    'viewport=${viewport.toStringAsFixed(2)} '
    'pixels=${scrollable.position.pixels.toStringAsFixed(2)} '
    'maxScroll=${maxScroll.toStringAsFixed(2)} '
    'contentExtent=${contentExtent.toStringAsFixed(2)} '
    'scrollBottom=${scrollBottom.toStringAsFixed(2)} '
    'surfaceBottom=${surface.bottom.toStringAsFixed(2)} '
    'systemRegionStart=${regionStart.toStringAsFixed(2)} '
    'lastBottom=${last.bottom.toStringAsFixed(2)} '
    'gap=$gap gapToScroll=$gapToScroll reachable=$reachable '
    'scrollTop=${scrollBox.top.toStringAsFixed(2)} '
    'appBarBottom=${appBar.bottom.toStringAsFixed(2)} '
    'barTop=${barTop?.toStringAsFixed(2)} '
    'barBottom=${barBottom?.toStringAsFixed(2)} '
    'ctaBottom=${ctaBottom?.toStringAsFixed(2)}',
  );

  return <String, double>{
    'inset': inset,
    'slotNonNull': slotNonNull ? 1 : 0,
    'bar': barPresent ? 1 : 0,
    'viewport': viewport,
    'maxScroll': maxScroll,
    'contentExtent': contentExtent,
    'scrollBottom': scrollBottom,
    'surfaceBottom': surface.bottom,
    'regionStart': regionStart,
    'lastBottom': last.bottom,
    'gap': gap,
    'gapToScroll': gapToScroll,
    'reachable': reachable,
    'scrollTop': scrollBox.top,
    'appBarBottom': appBar.bottom,
    'barTop': barTop ?? -1,
    'barBottom': barBottom ?? -1,
    'ctaBottom': ctaBottom ?? -1,
  };
}

/// The ONE body law (identical on both detail channels): with a bar the
/// body ends at the bar top (the bar owns the region); without a bar the
/// body SafeArea ends at the live system inset. Nothing else may decide the
/// body's bottom edge.
void _expectBodyLaw(Map<String, double> m) {
  final bool barPresent = m['bar'] == 1;
  final double expected = barPresent
      ? m['barTop']!
      : m['surfaceBottom']! - m['inset']!;
  expect(
    m['scrollBottom']!,
    closeTo(expected, 0.01),
    reason:
        'BODY LAW violated at inset ${m['inset']}: scrollBottom '
        '${m['scrollBottom']} expected $expected (barPresent=$barPresent) — '
        'either the bar did not reserve its region or the body SafeArea did '
        'not consume the LIVE system inset',
  );
}

/// Shared scroll/content contract: the top edge, reachability and the
/// design spacing below the last content are invariant; the system region
/// never overlaps content.
void _expectScrollContract(Map<String, double> m, {required double designGap}) {
  expect(
    m['scrollTop']!,
    closeTo(m['appBarBottom']!, 0.01),
    reason:
        'the scroll viewport must start exactly at the AppBar bottom — the '
        'body SafeArea must not add phantom top space',
  );
  expect(
    m['maxScroll']!,
    greaterThan(0),
    reason: 'the detail must actually scroll for the end contract to hold',
  );
  expect(m['reachable'], 1, reason: 'the last content must be fully visible');
  expect(
    m['gapToScroll']!,
    closeTo(designGap, 0.01),
    reason:
        'the design spacing below the last content (carried by the content '
        'itself) must stay invariant at ${m['inset']} — it must not absorb '
        'the system inset',
  );
  expect(
    m['gap']!,
    greaterThanOrEqualTo(-0.01),
    reason:
        'NEGATIVE gap=${m['gap']} at inset ${m['inset']}: last content '
        '(bottom ${m['lastBottom']}) entered the system region (start '
        '${m['regionStart']}) — the body lost its bottom-inset authority',
  );
  expect(
    m['lastBottom']!,
    lessThanOrEqualTo(m['regionStart']! + 0.01),
    reason: 'the last content must sit outside the system region',
  );
}

void main() {
  group('State A — viewer, CTA visible: the bar owns the live inset', () {
    for (final double inset in _insets) {
      testWidgets('inset $inset — bar present, body ends at the bar', (
        tester,
      ) async {
        final Auction auction = _auction(
          sellerId: 'seller-1',
          capabilities: _buyerCaps,
        );
        await _pump(
          tester,
          inset: inset,
          auction: auction,
          auth: AuthState.authenticated(_user('buyer-1'), emailVerified: true),
        );

        final Map<String, double> m = await _measure(
          tester,
          inset: inset,
          lastContent: find.byType(AuctionRecommendationsSection),
        );

        expect(find.byType(BottomActionBar), findsOneWidget);
        expect(find.text('Pasang Bid'), findsOneWidget);
        expect(m['slotNonNull'], 1);
        _expectBodyLaw(m);
        _expectScrollContract(m, designGap: _designGap);

        // The BAR is the bottom surface: it reaches the screen bottom …
        expect(
          m['barBottom']!,
          closeTo(m['surfaceBottom']!, 0.01),
          reason: 'the bar must be flush with the screen bottom',
        );
        // … and its CTA clears the live inset exactly once (p12 chrome).
        expect(
          m['ctaBottom']!,
          closeTo(m['surfaceBottom']! - inset - AppMetrics.p12, 0.01),
          reason:
              'the CTA did not consume the ${inset}px inset exactly once in '
              'the CTA-visible state',
        );
        // No body-side second reservation beside the bar.
        expect(
          m['scrollBottom']!,
          closeTo(m['barTop']!, 0.01),
          reason: 'the body carried a SECOND bottom reservation beside the bar',
        );
      });
    }

    testWidgets(
      'live inset: viewport follows 0 → 48 while content extent stays '
      'constant (no fixed clearance, no double inset)',
      (tester) async {
        final Auction auction = _auction(
          sellerId: 'seller-1',
          capabilities: _buyerCaps,
        );
        await _pump(
          tester,
          inset: 0,
          auction: auction,
          auth: AuthState.authenticated(_user('buyer-1'), emailVerified: true),
        );
        final Map<String, double> at0 = await _measure(
          tester,
          inset: 0,
          lastContent: find.byType(AuctionRecommendationsSection),
        );

        _setWindowInsets(tester, systemBottom: 48);
        await tester.pump();
        await tester.pump();
        final Map<String, double> at48 = await _measure(
          tester,
          inset: 48,
          lastContent: find.byType(AuctionRecommendationsSection),
        );

        expect(
          at0['scrollBottom']! - at48['scrollBottom']!,
          closeTo(48, 0.01),
          reason: 'the body bottom must follow the live system inset',
        );
        expect(
          at48['contentExtent']!,
          closeTo(at0['contentExtent']!, 0.01),
          reason: 'the content extent must stay constant across insets',
        );
        expect(
          at48['gapToScroll']!,
          closeTo(at0['gapToScroll']!, 0.01),
          reason: 'the design spacing must stay invariant across insets',
        );
      },
    );
  });

  group('State B — author/owner, CTA hidden: the body SafeArea is the law', () {
    for (final double inset in _insets) {
      testWidgets('inset $inset — slot null, body clears the system region', (
        tester,
      ) async {
        final Auction auction = _auction(
          sellerId: 'seller-1',
          capabilities: _ownerCaps,
        );
        await _pump(
          tester,
          inset: inset,
          auction: auction,
          auth: AuthState.authenticated(_user('seller-1'), emailVerified: true),
        );

        // Measure FIRST so the GEOM row is captured even while the contract
        // below is failing on the pre-fix code.
        final Map<String, double> m = await _measure(
          tester,
          inset: inset,
          lastContent: find.byType(AuctionRecommendationsSection),
        );

        // BUSINESS TRUTH (Owner, identical on both detail channels): the
        // author gets NO viewer-directed bottom surface — the slot mirrors
        // the surface exactly: NULL, never a zero-height stand-in the
        // Scaffold would treat as "a bar exists".
        expect(
          _scaffold(tester).bottomNavigationBar,
          isNull,
          reason:
              'the owner has no bottom surface — the slot must be null so '
              'the Scaffold keeps the body\'s bottom MediaQuery padding for '
              'the body SafeArea',
        );
        expect(find.byType(BottomActionBar), findsNothing);
        expect(find.text('Pasang Bid'), findsNothing);
        expect(find.text('Chat'), findsNothing);

        expect(m['slotNonNull'], 0);
        expect(m['bar'], 0);
        _expectBodyLaw(m);
        _expectScrollContract(m, designGap: _designGap);
      });
    }

    testWidgets(
      'live inset: the no-CTA body follows 0 → 48 1:1 with constant extent',
      (tester) async {
        final Auction auction = _auction(
          sellerId: 'seller-1',
          capabilities: _ownerCaps,
        );
        await _pump(
          tester,
          inset: 0,
          auction: auction,
          auth: AuthState.authenticated(_user('seller-1'), emailVerified: true),
        );
        final Map<String, double> at0 = await _measure(
          tester,
          inset: 0,
          lastContent: find.byType(AuctionRecommendationsSection),
        );

        _setWindowInsets(tester, systemBottom: 48);
        await tester.pump();
        await tester.pump();
        final Map<String, double> at48 = await _measure(
          tester,
          inset: 48,
          lastContent: find.byType(AuctionRecommendationsSection),
        );

        expect(
          at48['scrollBottom']!,
          closeTo(at48['surfaceBottom']! - 48, 0.01),
          reason:
              'the no-CTA body must lift by the LIVE inset — a fixed '
              'clearance or a missing authority would not move',
        );
        expect(
          at48['contentExtent']!,
          closeTo(at0['contentExtent']!, 0.01),
          reason: 'the content extent must stay constant across insets',
        );
      },
    );

    testWidgets(
      'the CTA does not change the content: extent identical across states',
      (tester) async {
        final Auction auction = _auction(
          sellerId: 'seller-1',
          capabilities: _buyerCaps,
        );
        await _pump(
          tester,
          inset: 34,
          auction: auction,
          auth: AuthState.authenticated(_user('buyer-1'), emailVerified: true),
        );
        final Map<String, double> stateA = await _measure(
          tester,
          inset: 34,
          lastContent: find.byType(AuctionRecommendationsSection),
        );

        await _pump(
          tester,
          inset: 34,
          auction: _auction(sellerId: 'seller-1', capabilities: _ownerCaps),
          auth: AuthState.authenticated(_user('seller-1'), emailVerified: true),
        );
        final Map<String, double> stateB = await _measure(
          tester,
          inset: 34,
          lastContent: find.byType(AuctionRecommendationsSection),
        );

        expect(
          stateB['contentExtent']!,
          closeTo(stateA['contentExtent']!, 0.01),
          reason:
              'the business state must not change the scroll content — only '
              'the bottom authority differs',
        );
      },
    );
  });

  group('Authority proofs', () {
    testWidgets('exactly ONE SafeArea owns the scrollable in both states', (
      tester,
    ) async {
      final Auction auction = _auction(
        sellerId: 'seller-1',
        capabilities: _buyerCaps,
      );
      await _pump(
        tester,
        inset: 34,
        auction: auction,
        auth: AuthState.authenticated(_user('buyer-1'), emailVerified: true),
      );
      expect(
        _safeAreaAncestorsOfScrollable(tester),
        1,
        reason: 'State A: exactly one body SafeArea, none missing',
      );

      await _pump(
        tester,
        inset: 34,
        auction: _auction(sellerId: 'seller-1', capabilities: _ownerCaps),
        auth: AuthState.authenticated(_user('seller-1'), emailVerified: true),
      );
      expect(
        _safeAreaAncestorsOfScrollable(tester),
        1,
        reason: 'State B: exactly one body SafeArea, none missing',
      );
    });

    testWidgets('keyboard: CTA state — bar and body stay above the keyboard', (
      tester,
    ) async {
      final Auction auction = _auction(
        sellerId: 'seller-1',
        capabilities: _buyerCaps,
      );
      await _pump(
        tester,
        inset: 48,
        keyboard: 300,
        auction: auction,
        auth: AuthState.authenticated(_user('buyer-1'), emailVerified: true),
      );

      final Rect surface = tester.getRect(find.byType(Scaffold));
      final Rect bar = tester.getRect(find.byType(BottomActionBar));
      final Rect cta = tester.getRect(
        find.descendant(
          of: find.byType(BottomActionBar),
          matching: find.byType(ElevatedButton),
        ),
      );
      final Rect scrollBox = tester.getRect(find.byType(CustomScrollView));

      expect(
        cta.bottom,
        lessThanOrEqualTo(surface.bottom - 300 + 0.01),
        reason: 'the CTA must stay above the keyboard (bar self-lift)',
      );
      expect(
        scrollBox.bottom,
        lessThanOrEqualTo(surface.bottom - 300 + 0.01),
        reason: 'the body must stay above the keyboard',
      );
      expect(scrollBox.bottom, lessThanOrEqualTo(bar.top + 0.01));
    });

    testWidgets(
      'keyboard: no-CTA state — body resized by the keyboard only, no '
      'phantom inset double-stack',
      (tester) async {
        final Auction auction = _auction(
          sellerId: 'seller-1',
          capabilities: _ownerCaps,
        );
        await _pump(
          tester,
          inset: 48,
          keyboard: 300,
          auction: auction,
          auth: AuthState.authenticated(_user('seller-1'), emailVerified: true),
        );

        final Rect surface = tester.getRect(find.byType(Scaffold));
        final Rect scrollBox = tester.getRect(find.byType(CustomScrollView));

        // The keyboard covers the system inset (padding derives to 0), so
        // the body must end EXACTLY at the keyboard — not keyboard + inset
        // (double) and not at the surface bottom (under the keyboard).
        expect(
          scrollBox.bottom,
          closeTo(surface.bottom - 300, 0.01),
          reason:
              'the no-CTA body must be lifted by exactly the keyboard height '
              '(${scrollBox.bottom} vs ${surface.bottom - 300})',
        );
      },
    );
  });

  group('Source residue sweep — one detail law, two consumers', () {
    test('both detail screens spell the SAME body law', () {
      const String forSalePath =
          'lib/domains/commerce/catalog/for_sale/presentation/screens/'
          'for_sale_detail_screen.dart';
      const String auctionPath =
          'lib/domains/commerce/catalog/auction/presentation/screens/'
          'auction_detail_screen.dart';

      for (final String path in <String>[forSalePath, auctionPath]) {
        final String src = File(path).readAsStringSync();
        expect(
          src,
          contains('body: SafeArea('),
          reason: '$path: the body SafeArea is the canonical no-bar authority',
        );
        expect(
          src.contains('bottom: false'),
          isFalse,
          reason:
              '$path: `SafeArea(bottom: false)` makes the body depend on the '
              'CTA — the old law where the bar\'s presence was the body\'s '
              'safety',
        );
        expect(src.contains('MediaQuery'), isFalse, reason: path);
        expect(src.contains('bottomBarClearance'), isFalse, reason: path);
        expect(src.contains('fabClearance'), isFalse, reason: path);
        expect(
          RegExp(r'SizedBox\(\s*height:\s*(?:80|96|100)\b').hasMatch(src),
          isFalse,
          reason: '$path: no fixed bottom clearance beside the optional CTA',
        );
      }
    });
  });
}
