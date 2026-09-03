import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/shared/data/dto/commerce_media_request_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/seller_auctions_pager.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/seller_auction_draft_edit_screen.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/seller_auctions_screen.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  AuthState _state;

  @override
  AuthState build() => _state;

  void setAuthState(AuthState state) {
    _state = state;
    this.state = state;
  }
}

class _FakeAuctionWatchRepository implements AuctionWatchRepository {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeLoggerService implements ILoggerService {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeAuctionRepository implements AuctionRepository {
  _FakeAuctionRepository({required this.onGetUserAuctions});

  final Future<RepositoryResult<List<Auction>>> Function(
    String sellerId,
    AuctionStatus? status,
    int limit,
    String? lastAuctionId,
  ) onGetUserAuctions;

  final requestedSellerIds = <String>[];
  final requestedStatuses = <AuctionStatus?>[];
  final requestedLimits = <int>[];
  final requestedCursors = <String?>[];
  final updateCalls = <Map<String, dynamic>>[];
  final cancelCalls = <({String auctionId, String sellerId, String reason})>[];

  @override
  Future<RepositoryResult<Auction>> createAuction({
    required String sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    required String title,
    required String description,
    List<CommerceMediaRequestDto> media = const [],
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
    AuctionLocation? location,
    required List<String> shippingSetupIds,
    String? preparationNote,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<Auction>> getAuctionById(String auctionId) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<List<Auction>>> getAuctionsByIds(
    List<String> auctionIds,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<List<Auction>>> getActiveAuctions({
    String? variety,
    double? minSize,
    double? maxSize,
    double? maxBid,
    int limit = 20,
    String? lastAuctionId,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<List<Auction>>> getUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 20,
    String? lastAuctionId,
  }) {
    requestedSellerIds.add(sellerId);
    requestedStatuses.add(status);
    requestedLimits.add(limit);
    requestedCursors.add(lastAuctionId);
    return onGetUserAuctions(sellerId, status, limit, lastAuctionId);
  }

  @override
  Future<RepositoryResult<Auction>> updateAuction(
    String auctionId,
    Map<String, dynamic> updates,
  ) async {
    updateCalls.add(updates);
    return RepositoryResult.success(
      _auction(
        id: auctionId,
        status: AuctionStatus.draft,
        title: (updates['title'] as String?) ?? 'Updated Auction',
        openingBid: (updates['startPrice'] as int?) ?? 1000000,
        currentBid: (updates['startPrice'] as int?) ?? 1000000,
        bidIncrement: (updates['bidIncrement'] as int?) ?? 100000,
      ),
    );
  }

  @override
  Future<RepositoryResult<Auction>> updateAuctionStatus({
    required String auctionId,
    required AuctionStatus status,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<List<AuctionBid>>> getAuctionBids({
    required String auctionId,
    int limit = 50,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<void>> cancelAuction({
    required String auctionId,
    required String sellerId,
    required String reason,
  }) async {
    cancelCalls.add((auctionId: auctionId, sellerId: sellerId, reason: reason));
    return RepositoryResult.success(null);
  }

  @override
  Future<RepositoryResult<AuctionBid>> placeBid({
    required String auctionId,
    required String bidderId,
    required int amount,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<String>> claimAuction({
    required String auctionId,
    required String addressId,
    required String shippingSetupId,
    String? discountCode,
    bool useCoins = false,
  }) async {
    throw UnimplementedError();
  }

  @override
  Stream<List<Auction>> watchActiveAuctions({int limit = 50}) =>
      const Stream.empty();

  @override
  Stream<List<AuctionBid>> watchAuctionBids(
    String auctionId, {
    int limit = 50,
  }) =>
      const Stream.empty();

  @override
  Stream<Auction?> watchAuction(String auctionId) => const Stream.empty();

  @override
  Stream<List<Auction>> watchUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 50,
  }) =>
      const Stream.empty();
}

Auction _auction({
  required String id,
  required AuctionStatus status,
  String title = 'Kohaku 50cm',
  int openingBid = 1000000,
  int currentBid = 1200000,
  int bidIncrement = 100000,
  int totalBidders = 0,
  DateTime? settlementDeadline,
  String? winnerId,
  String? winnerUsername,
  DateTime? startTime,
  DateTime? endTime,
}) {
  final baseStart = startTime ?? DateTime.utc(2026, 7, 1, 8);
  final baseEnd = endTime ?? DateTime.utc(2026, 7, 2, 8);
  return Auction(
    id: id,
    sellerId: 'seller-1',
    sellerUsername: 'seller',
    sellerFarmName: 'Farm',
    sellerAvatar: null,
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
    title: title,
    description: 'desc',
    koiDetails: const KoiDetails(
      variety: 'Kohaku',
      sizeInCm: 50,
      ageInMonths: 12,
      gender: 'male',
    ),
    openingBid: openingBid,
    currentBid: currentBid,
    bidIncrement: bidIncrement,
    startTime: baseStart,
    endTime: baseEnd,
    settlementDeadline: settlementDeadline,
    status: status,
    winnerId: winnerId,
    winnerUsername: winnerUsername,
    totalBidders: totalBidders,
    createdAt: DateTime.utc(2026, 7, 1, 7),
  );
}

List<Auction> _auctionPage({
  required int start,
  required int count,
  required AuctionStatus Function(int index) statusForIndex,
}) {
  return List.generate(count, (offset) {
    final index = start + offset;
    return _auction(
      id: 'a$index',
      status: statusForIndex(index),
    );
  });
}

AuthUser _seller({
  required String id,
  bool activeMarketAuthority = true,
}) {
  return AuthUser(
    id: id,
    createdAt: DateTime.utc(2026, 7, 1),
    updatedAt: DateTime.utc(2026, 7, 1),
    email: '$id@example.com',
    username: id,
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    hasSellerProfile: true,
    sellerSubscriptionStatus: activeMarketAuthority ? 'active' : 'expired',
    hasMarketAuthority: activeMarketAuthority,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
}

Future<void> _settle() async {
  await Future<void>.delayed(Duration.zero);
  await Future<void>.delayed(Duration.zero);
  await Future<void>.delayed(const Duration(milliseconds: 1));
}

void main() {
  group('SellerAuctionsPagerController', () {
    test('loads first page and dedupes duplicate IDs', () async {
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          expect(sellerId, 'seller-1');
          expect(status, isNull);
          expect(limit, 20);
          expect(cursor, isNull);
          return RepositoryResult.success([
            _auction(id: 'a1', status: AuctionStatus.draft),
            _auction(id: 'a1', status: AuctionStatus.draft),
            _auction(id: 'a2', status: AuctionStatus.scheduled),
          ]);
        },
      );
      final auth = _FakeAuthController(AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true));
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
          auctionWatchRepositoryProvider.overrideWithValue(
            _FakeAuctionWatchRepository(),
          ),
        ],
      );
      addTearDown(container.dispose);

      final subscription = container.listen(
        sellerAuctionsPagerProvider,
        (_, __) {},
        fireImmediately: true,
      );
      addTearDown(subscription.close);

      await _settle();

      final state = container.read(sellerAuctionsPagerProvider);
      expect(state.auctions.map((a) => a.id), ['a1', 'a2']);
      expect(state.initialError, isNull);
      expect(state.hasMore, isFalse);
      expect(repo.requestedCursors, [null]);
    });

    test('load more appends in order without duplicating IDs', () async {
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          if (cursor == null) {
            return RepositoryResult.success(_auctionPage(
              start: 1,
              count: 20,
              statusForIndex: (index) =>
                  index.isEven ? AuctionStatus.active : AuctionStatus.draft,
            ));
          }
          return RepositoryResult.success([
            _auction(id: 'a20', status: AuctionStatus.active),
            _auction(id: 'a21', status: AuctionStatus.waitingSettlement),
            _auction(id: 'a22', status: AuctionStatus.ended),
            _auction(id: 'a23', status: AuctionStatus.cancelled),
          ]);
        },
      );
      final auth = _FakeAuthController(AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true));
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
          auctionWatchRepositoryProvider.overrideWithValue(
            _FakeAuctionWatchRepository(),
          ),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(
        sellerAuctionsPagerProvider,
        (_, __) {},
        fireImmediately: true,
      );
      addTearDown(subscription.close);

      await _settle();
      await container.read(sellerAuctionsPagerProvider.notifier).loadMore();
      await _settle();

      final state = container.read(sellerAuctionsPagerProvider);
      expect(
        state.auctions.map((a) => a.id),
        [for (var i = 1; i <= 23; i++) 'a$i'],
      );
      expect(repo.requestedCursors, [null, 'a20']);
    });

    test('rapid duplicate loadMore is blocked while request is in flight', () async {
      final page2 = Completer<RepositoryResult<List<Auction>>>();
      var page2Calls = 0;
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          if (cursor == null) {
            return RepositoryResult.success(_auctionPage(
              start: 1,
              count: 20,
              statusForIndex: (index) =>
                  index.isEven ? AuctionStatus.active : AuctionStatus.draft,
            ));
          }
          page2Calls += 1;
          return page2.future;
        },
      );
      final auth = _FakeAuthController(AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true));
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
          auctionWatchRepositoryProvider.overrideWithValue(
            _FakeAuctionWatchRepository(),
          ),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(
        sellerAuctionsPagerProvider,
        (_, __) {},
        fireImmediately: true,
      );
      addTearDown(subscription.close);

      await _settle();
      final notifier = container.read(sellerAuctionsPagerProvider.notifier);
      unawaited(notifier.loadMore());
      unawaited(notifier.loadMore());
      await Future<void>.delayed(Duration.zero);

      expect(page2Calls, 1, reason: 'second call blocked while loading');
      page2.complete(
        RepositoryResult.success([
          _auction(id: 'a21', status: AuctionStatus.scheduled),
        ]),
      );
      await _settle();

      final state = container.read(sellerAuctionsPagerProvider);
      expect(state.auctions.length, 21);
    });

    test('refresh resets to the latest first page', () async {
      var firstPageCalls = 0;
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          if (cursor != null) {
            return RepositoryResult.success(const []);
          }
          firstPageCalls += 1;
          if (firstPageCalls == 1) {
            return RepositoryResult.success([
              _auction(id: 'old-1', status: AuctionStatus.draft),
              _auction(id: 'old-2', status: AuctionStatus.active),
            ]);
          }
          return RepositoryResult.success([
            _auction(id: 'new-1', status: AuctionStatus.draft),
            _auction(id: 'new-2', status: AuctionStatus.waitingSettlement),
          ]);
        },
      );
      final auth = _FakeAuthController(AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true));
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
          auctionWatchRepositoryProvider.overrideWithValue(
            _FakeAuctionWatchRepository(),
          ),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(
        sellerAuctionsPagerProvider,
        (_, __) {},
        fireImmediately: true,
      );
      addTearDown(subscription.close);

      await _settle();
      await container.read(sellerAuctionsPagerProvider.notifier).refresh();
      await _settle();

      final state = container.read(sellerAuctionsPagerProvider);
      expect(state.auctions.map((a) => a.id), ['new-1', 'new-2']);
      expect(firstPageCalls, 2);
    });

    test('filter changes are local and reset back to all', () async {
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          return RepositoryResult.success([
            _auction(id: 'a1', status: AuctionStatus.draft),
            _auction(id: 'a2', status: AuctionStatus.scheduled),
            _auction(id: 'a3', status: AuctionStatus.active),
            _auction(id: 'a4', status: AuctionStatus.waitingSettlement),
            _auction(id: 'a5', status: AuctionStatus.ended),
          ]);
        },
      );
      final auth = _FakeAuthController(AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true));
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
          auctionWatchRepositoryProvider.overrideWithValue(
            _FakeAuctionWatchRepository(),
          ),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(
        sellerAuctionsPagerProvider,
        (_, __) {},
        fireImmediately: true,
      );
      addTearDown(subscription.close);

      await _settle();
      final notifier = container.read(sellerAuctionsPagerProvider.notifier);
      notifier.setFilter(SellerAuctionFilter.active);
      expect(
        container.read(sellerAuctionsPagerProvider).visibleAuctions.map((a) => a.id),
        ['a3'],
      );
      notifier.setFilter(SellerAuctionFilter.all);
      expect(
        container.read(sellerAuctionsPagerProvider).visibleAuctions.map((a) => a.id),
        ['a1', 'a2', 'a3', 'a4', 'a5'],
      );
    });

    test('load more failure preserves current data and retry recovers', () async {
      var attempts = 0;
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          if (cursor == null) {
            return RepositoryResult.success(_auctionPage(
              start: 1,
              count: 20,
              statusForIndex: (index) =>
                  index.isEven ? AuctionStatus.active : AuctionStatus.draft,
            ));
          }
          attempts += 1;
          if (attempts == 1) {
            return RepositoryResult.error('load more failed');
          }
          return RepositoryResult.success([
            _auction(id: 'a21', status: AuctionStatus.ended),
          ]);
        },
      );
      final auth = _FakeAuthController(AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true));
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
          auctionWatchRepositoryProvider.overrideWithValue(
            _FakeAuctionWatchRepository(),
          ),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(
        sellerAuctionsPagerProvider,
        (_, __) {},
        fireImmediately: true,
      );
      addTearDown(subscription.close);

      await _settle();
      final notifier = container.read(sellerAuctionsPagerProvider.notifier);
      await notifier.loadMore();
      await _settle();

      var state = container.read(sellerAuctionsPagerProvider);
      expect(state.auctions.length, 20);
      expect(state.loadMoreError, 'load more failed');

      await notifier.retryLoadMore();
      await _settle();

      state = container.read(sellerAuctionsPagerProvider);
      expect(state.auctions.length, 21);
      expect(state.loadMoreError, isNull);
    });

    test('auth change discards stale publication and reloads from new seller', () async {
      final seller1Pending = Completer<RepositoryResult<List<Auction>>>();
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          if (sellerId == 'seller-1') {
            return seller1Pending.future;
          }
          return RepositoryResult.success([
            _auction(id: 'b1', status: AuctionStatus.draft),
          ]);
        },
      );
      final auth = _FakeAuthController(AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true));
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
          auctionWatchRepositoryProvider.overrideWithValue(
            _FakeAuctionWatchRepository(),
          ),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(
        sellerAuctionsPagerProvider,
        (_, __) {},
        fireImmediately: true,
      );
      addTearDown(subscription.close);

      await Future<void>.delayed(Duration.zero);
      auth.setAuthState(
        AuthState.authenticated(_seller(id: 'seller-2'), emailVerified: true),
      );
      await _settle();
      seller1Pending.complete(
        RepositoryResult.success([
          _auction(id: 'a1', status: AuctionStatus.active),
        ]),
      );
      await _settle();

      final state = container.read(sellerAuctionsPagerProvider);
      expect(state.ownerId, 'seller-2');
      expect(state.auctions.map((a) => a.id), ['b1']);
      expect(repo.requestedSellerIds, ['seller-1', 'seller-2']);
    });
  });

  group('SellerAuctionsScreen', () {
    testWidgets('waitingSettlement is labeled clearly and detail backstack works',
        (tester) async {
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          return RepositoryResult.success([
            _auction(
              id: 'a1',
              status: AuctionStatus.waitingSettlement,
              currentBid: 1500000,
              winnerId: 'buyer-1',
              winnerUsername: 'buyer_one',
              settlementDeadline: DateTime.utc(2026, 7, 6),
            ),
          ]);
        },
      );
      final auth = _FakeAuthController(AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true));
      final router = GoRouter(
        initialLocation: RoutePaths.sellerAuctions,
        routes: [
          GoRoute(
            path: RoutePaths.sellerAuctions,
            builder: (context, state) => ProviderScope(
              overrides: [
                authControllerProvider.overrideWith(() => auth),
                auctionRepositoryProvider.overrideWithValue(repo),
                auctionWatchRepositoryProvider.overrideWithValue(
                  _FakeAuctionWatchRepository(),
                ),
                loggerServiceProvider.overrideWithValue(_FakeLoggerService()),
              ],
              child: const SellerAuctionsScreen(),
            ),
          ),
          GoRoute(
            path: RoutePaths.auctionDetails,
            builder: (context, state) {
              final auctionId = state.pathParameters['auctionId']!;
              return Scaffold(
                appBar: AppBar(),
                body: Center(child: Text('detail $auctionId')),
              );
            },
          ),
        ],
      );

      await tester.pumpWidget(
        MaterialApp.router(
          routerConfig: router,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: const Locale('id'),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Menunggu Penyelesaian'), findsOneWidget);
      expect(find.text('detail a1'), findsNothing);

      await tester.tap(find.byKey(const ValueKey('seller-auction-card-a1')));
      await tester.pumpAndSettle();
      expect(find.text('detail a1'), findsOneWidget);

      router.pop();
      await tester.pumpAndSettle();
      expect(find.text('Menunggu Penyelesaian'), findsOneWidget);
    });

    testWidgets('draft edit is exposed only for draft auctions', (tester) async {
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          return RepositoryResult.success([
            _auction(id: 'draft-1', status: AuctionStatus.draft),
            _auction(id: 'active-1', status: AuctionStatus.active),
          ]);
        },
      );
      final auth = _FakeAuthController(AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true));
      final router = GoRouter(
        initialLocation: RoutePaths.sellerAuctions,
        routes: [
          GoRoute(
            path: RoutePaths.sellerAuctions,
            builder: (context, state) => ProviderScope(
              overrides: [
                authControllerProvider.overrideWith(() => auth),
                auctionRepositoryProvider.overrideWithValue(repo),
                auctionWatchRepositoryProvider.overrideWithValue(
                  _FakeAuctionWatchRepository(),
                ),
                loggerServiceProvider.overrideWithValue(_FakeLoggerService()),
              ],
              child: const SellerAuctionsScreen(),
            ),
          ),
        ],
      );

      await tester.pumpWidget(
        MaterialApp.router(
          routerConfig: router,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: const Locale('id'),
        ),
      );
      await tester.pumpAndSettle();

      final popupButtons = find.byType(PopupMenuButton<String>);
      expect(popupButtons, findsNWidgets(2));

      await tester.tap(popupButtons.first);
      await tester.pumpAndSettle();
      expect(find.text('Edit draft'), findsOneWidget);
      expect(find.text('Batalkan'), findsOneWidget);

      await tester.tapAt(const Offset(1, 1));
      await tester.pumpAndSettle();

      await tester.tap(popupButtons.last);
      await tester.pumpAndSettle();
      expect(find.text('Edit draft'), findsNothing);
    });

    testWidgets('draft editor calls update endpoint and returns success', (
      tester,
    ) async {
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          return RepositoryResult.success([
            _auction(id: 'draft-1', status: AuctionStatus.draft),
          ]);
        },
      );
      final auth = _FakeAuthController(AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true));

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(() => auth),
            auctionRepositoryProvider.overrideWithValue(repo),
            auctionWatchRepositoryProvider.overrideWithValue(
              _FakeAuctionWatchRepository(),
            ),
            loggerServiceProvider.overrideWithValue(_FakeLoggerService()),
          ],
          child: MaterialApp(
            home: SellerAuctionDraftEditScreen(
              auction: _auction(id: 'draft-1', status: AuctionStatus.draft),
            ),
          ),
        ),
      );

      await tester.enterText(find.byType(TextFormField).at(0), 'Draft Baru');
      await tester.enterText(find.byType(TextFormField).at(1), 'Deskripsi Baru');
      await tester.enterText(find.byType(TextFormField).at(2), '2000000');
      await tester.enterText(find.byType(TextFormField).at(3), '150000');
      await tester.enterText(find.byType(TextFormField).at(4), '2500000');

      await tester.tap(find.text('Simpan Draft'));
      await tester.pumpAndSettle();

      expect(repo.updateCalls, hasLength(1));
      expect(repo.updateCalls.single['title'], 'Draft Baru');
      expect(repo.updateCalls.single['startPrice'], 2000000);
      expect(repo.updateCalls.single['bidIncrement'], 150000);
      expect(find.text('Edit Draft Lelang'), findsNothing);
    });
  });
}
