// My Bids convergence proof (Owner-final business truth).
//
//   - My Bids = open auctions only (active + waiting_settlement); the backend
//     is the single visibility authority, mobile is pure projection.
//   - "Bid saya" = latest bid by time (your_last_bid, snake_case wire).
//   - Countdown reads end_at, ticks realtime, never mutates business state.
//   - Error is distinct from valid empty; tap navigates to canonical detail.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/observability/screen_view_route_observer.dart';
import 'package:labuda/core/src/router/modules/auction_module.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/bidding_item_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/my_bids_provider.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/my_bids_screen.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';

BiddingItemDto _item({
  required String id,
  required String title,
  required int yourLastBid,
  required int currentBid,
  required String status,
  required DateTime endAt,
}) {
  return BiddingItemDto(
    auctionId: id,
    title: title,
    yourLastBid: yourLastBid,
    currentBid: currentBid,
    status: status,
    endAt: endAt,
  );
}

class _FakeAnalyticsRepository implements IAnalyticsRepository {
  @override
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  }) async => Result.error('unused');

  @override
  Future<Result<void>> logScreenView({
    required String screenName,
    String? screenClass,
  }) async => Result.error('unused');
}

class _FakeScreenViewRouteObserver extends ScreenViewRouteObserver {
  _FakeScreenViewRouteObserver() : super(_FakeAnalyticsRepository());
}

Widget _wrap(List<BiddingItemDto> items) {
  return ProviderScope(
    overrides: [
      myBidsProvider.overrideWith((ref) async => items),
      screenViewRouteObserverProvider.overrideWithValue(
        _FakeScreenViewRouteObserver(),
      ),
    ],
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const MyBidsScreen(),
    ),
  );
}

void main() {
  test('BiddingItemDto parses canonical snake_case JSON', () {
    final dto = BiddingItemDto.fromJson({
      'auction_id': 'a1',
      'title': 'Kohaku 30cm',
      'your_last_bid': 120000,
      'current_bid': 150000,
      'status': 'leading',
      'end_at': '2026-10-01T10:00:00.000Z',
    });
    expect(dto.auctionId, 'a1');
    expect(dto.title, 'Kohaku 30cm');
    expect(dto.yourLastBid, 120000);
    expect(dto.currentBid, 150000);
    expect(dto.status, 'leading');
    expect(dto.endAt, DateTime.parse('2026-10-01T10:00:00.000Z'));
  });

  test('BiddingItemDto tolerates missing optional presentation fields', () {
    final dto = BiddingItemDto.fromJson({
      'auction_id': 'a1',
      'end_at': '2026-10-01T10:00:00.000Z',
    });
    expect(dto.title, '');
    expect(dto.yourLastBid, 0);
    expect(dto.currentBid, 0);
    expect(dto.status, '');
  });

  testWidgets('My Bids renders latest bid and canonical status labels', (
    tester,
  ) async {
    final now = DateTime.now();
    await tester.pumpWidget(
      _wrap([
        _item(
          id: 'a1',
          title: 'Kohaku Lead',
          yourLastBid: 120000,
          currentBid: 120000,
          status: 'leading',
          endAt: now.add(const Duration(hours: 2)),
        ),
        _item(
          id: 'a2',
          title: 'Sanke Outbid',
          yourLastBid: 180000,
          currentBid: 200000,
          status: 'outbid',
          endAt: now.add(const Duration(hours: 1)),
        ),
        _item(
          id: 'a3',
          title: 'Showa Claim',
          yourLastBid: 300000,
          currentBid: 300000,
          status: 'waiting_claim',
          endAt: now.subtract(const Duration(hours: 1)),
        ),
      ]),
    );
    await tester.pump();

    expect(find.text('My Bids'), findsOneWidget);
    expect(find.text('Kohaku Lead'), findsOneWidget);
    // Latest-by-time bid is displayed, not the highest ever.
    expect(find.textContaining('Bid saya'), findsNWidgets(3));
    expect(find.textContaining('120'), findsWidgets);
    expect(find.textContaining('Terkini'), findsNWidgets(3));
    // Existing Labuda vocabulary, no invented backend statuses.
    expect(find.text('Anda Memimpin'), findsOneWidget);
    expect(find.text('Ter-Lelang'), findsOneWidget);
    expect(find.text('Menunggu Penyelesaian'), findsOneWidget);
  });

  testWidgets('My Bids countdown ticks realtime from end_at', (tester) async {
    var fakeNow = DateTime(2026, 10, 1, 12, 0, 0);
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: MyBidCountdown(
            endAt: fakeNow.add(const Duration(hours: 1, seconds: 5)),
            now: () => fakeNow,
          ),
        ),
      ),
    );
    await tester.pump();

    final countdown = find.textContaining(
      RegExp(r'\d{2}:\d{2}:\d{2}'),
    );
    expect(countdown, findsOneWidget);
    expect(tester.widget<Text>(countdown).data, '01:00:05');
    // Advance both the ticker (fake clock) and the injected wall clock:
    // the presentation countdown must move without any backend round-trip.
    fakeNow = fakeNow.add(const Duration(seconds: 3));
    await tester.pump(const Duration(seconds: 3));
    expect(tester.widget<Text>(countdown).data, '01:00:02');
  });

  testWidgets('waiting_claim shows settlement state, not a bid countdown', (
    tester,
  ) async {
    final now = DateTime.now();
    await tester.pumpWidget(
      _wrap([
        _item(
          id: 'a1',
          title: 'Active One',
          yourLastBid: 100000,
          currentBid: 100000,
          status: 'leading',
          endAt: now.add(const Duration(hours: 1)),
        ),
        _item(
          id: 'a2',
          title: 'Claim One',
          yourLastBid: 300000,
          currentBid: 300000,
          status: 'waiting_claim',
          endAt: now.subtract(const Duration(hours: 1)),
        ),
      ]),
    );
    await tester.pump();

    // Exactly one realtime countdown: the active auction.
    expect(find.textContaining(RegExp(r'\d{2}:\d{2}:\d{2}')), findsOneWidget);
    expect(find.text('Menunggu Penyelesaian'), findsOneWidget);
  });

  testWidgets('unknown status never fabricates a valid position', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap([
        _item(
          id: 'a9',
          title: 'Stale',
          yourLastBid: 50000,
          currentBid: 50000,
          status: 'lost',
          endAt: DateTime.now().add(const Duration(hours: 1)),
        ),
      ]),
    );
    await tester.pump();

    expect(find.text('Anda Memimpin'), findsNothing);
    expect(find.text('Ter-Lelang'), findsNothing);
  });

  testWidgets('empty result is distinct from error', (tester) async {
    await tester.pumpWidget(_wrap(const []));
    await tester.pump();
    expect(
      find.text('Belum ada lelang aktif yang kamu bid'),
      findsOneWidget,
    );
    expect(find.byType(PageErrorState), findsNothing);
  });

  testWidgets('API failure renders error, never silent empty', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          // Deterministic AsyncError: proves the failure surface without
          // depending on future timing.
          myBidsProvider.overrideWithValue(
            AsyncValue<List<BiddingItemDto>>.error(
              Exception('boom'),
              StackTrace.empty,
            ),
          ),
          screenViewRouteObserverProvider.overrideWithValue(
            _FakeScreenViewRouteObserver(),
          ),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const MyBidsScreen(),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.byType(PageErrorState), findsOneWidget);
    expect(
      find.text('Belum ada lelang aktif yang kamu bid'),
      findsNothing,
    );
  });

  testWidgets('tap navigates to canonical auction detail', (tester) async {
    final now = DateTime.now();
    String? openedAuctionId;
    final router = GoRouter(
      initialLocation: RoutePaths.myBids,
      routes: [
        GoRoute(
          path: RoutePaths.myBids,
          builder: (_, _) => const MyBidsScreen(),
        ),
        GoRoute(
          path: RoutePaths.auctionDetails,
          builder: (_, state) {
            openedAuctionId = state.pathParameters['auctionId'];
            return Scaffold(body: Text('DETAIL:$openedAuctionId'));
          },
        ),
      ],
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          myBidsProvider.overrideWith(
            (ref) async => [
              _item(
                id: 'auction-42',
                title: 'Tap Me',
                yourLastBid: 120000,
                currentBid: 130000,
                status: 'outbid',
                endAt: now.add(const Duration(hours: 1)),
              ),
            ],
          ),
          screenViewRouteObserverProvider.overrideWithValue(
            _FakeScreenViewRouteObserver(),
          ),
        ],
        child: MaterialApp.router(
          routerConfig: router,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
        ),
      ),
    );
    await tester.pump();

    await tester.tap(find.text('Tap Me'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(openedAuctionId, 'auction-42');
    expect(find.text('DETAIL:auction-42'), findsOneWidget);
  });

  test('canonical route table exposes exactly one /my-bids route', () {
    expect(RoutePaths.myBids, '/my-bids');
    final hits = AuctionModule().routes.where(
      (r) => r.path == RoutePaths.myBids,
    );
    expect(hits.length, 1);
  });
}
