import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/common/types/preparation_time.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/create_auction_route_contract.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/seller_auctions_pager.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/seller_auction_edit_screen.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/seller_auctions_screen.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/loading_indicator.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';

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

class _FakeLoggerService implements ILoggerService {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

/// Cancel-only notifier: the real `cancelAuction` reloads the auction through
/// `getAuctionById`, which the list fake does not implement. This keeps the
/// list repository fake and the cancel funnel independent.
class _FakeCancelNotifier extends AuctionNotifier {
  final cancelled = <String>[];

  @override
  AuctionNotifierState build() => const AuctionNotifierState();

  @override
  Future<bool> cancelAuction({
    required String auctionId,
    required String sellerId,
    required String reason,
  }) async {
    cancelled.add(auctionId);
    return true;
  }
}

class _FakeAuctionRepository implements AuctionRepository {
  _FakeAuctionRepository({required this.onGetUserAuctions});

  final Future<Result<List<Auction>>> Function(
    String sellerId,
    AuctionStatus? status,
    int limit,
    String? lastAuctionId,
  )
  onGetUserAuctions;

  final requestedSellerIds = <String>[];
  final requestedStatuses = <AuctionStatus?>[];
  final requestedLimits = <int>[];
  final requestedCursors = <String?>[];
  final updateCalls = <Map<String, dynamic>>[];
  final cancelCalls = <({String auctionId, String sellerId, String reason})>[];
  final relistCalls = <String>[];

  @override
  Future<Result<Auction>> createAuction({
    required String sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    required String title,
    required String description,
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
    String? farmAddressId,
    required PreparationTime preparationTime,
    required List<String> shippingSetupIds,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<Auction>> getAuctionById(String auctionId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<List<Auction>>> getAuctionsByIds(
    List<String> auctionIds,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<List<Auction>>> getActiveAuctions({
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
  Future<Result<List<Auction>>> getUserAuctions({
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
  Future<Result<Auction>> updateAuction(
    String auctionId,
    Map<String, dynamic> updates,
  ) async {
    updateCalls.add(updates);
    return Result.success(
      _auction(
        id: auctionId,
        status: AuctionStatus.scheduled,
        title: (updates['title'] as String?) ?? 'Updated Auction',
        openingBid: (updates['startPrice'] as int?) ?? 1000000,
        currentBid: (updates['startPrice'] as int?) ?? 1000000,
        bidIncrement: (updates['bidIncrement'] as int?) ?? 100000,
      ),
    );
  }

  @override
  Future<Result<List<AuctionBid>>> getAuctionBids({
    required String auctionId,
    int limit = 50,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<void>> cancelAuction({
    required String auctionId,
    required String sellerId,
    required String reason,
  }) async {
    cancelCalls.add((auctionId: auctionId, sellerId: sellerId, reason: reason));
    return Result.success(null);
  }

  @override
  Future<Result<void>> relistAuction({
    required String auctionId,
    required String title,
    required String description,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
  }) async {
    relistCalls.add(auctionId);
    return Result.success(null);
  }

  @override
  Future<Result<AuctionBid>> placeBid({
    required String auctionId,
    required String bidderId,
    required int amount,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<String>> claimAuction({
    required String auctionId,
    required String addressId,
    String? shippingSetupId,
    String? shippingQuoteId,
    String? chatId,
    String? discountCode,
    bool useCoins = false,
  }) async {
    throw UnimplementedError();
  }

  @override
  Stream<List<AuctionBid>> watchAuctionBids(
    String auctionId, {
    int limit = 50,
  }) => const Stream.empty();

  @override
  Stream<Auction?> watchAuction(String auctionId) => const Stream.empty();
}

Auction _auction({
  required String id,
  required AuctionStatus status,
  String title = 'Kohaku 50cm',
  int openingBid = 1000000,
  int currentBid = 1200000,
  int bidIncrement = 100000,
  String? winnerId,
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
    status: status,
    winnerId: winnerId,
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
    return _auction(id: 'a$index', status: statusForIndex(index));
  });
}

AuthUser _seller({required String id, bool activeMarketAuthority = true}) {
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
          return Result.success([
            _auction(id: 'a1', status: AuctionStatus.scheduled),
            _auction(id: 'a1', status: AuctionStatus.scheduled),
            _auction(id: 'a2', status: AuctionStatus.scheduled),
          ]);
        },
      );
      final auth = _FakeAuthController(
        AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
      );
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
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
            return Result.success(
              _auctionPage(
                start: 1,
                count: 20,
                statusForIndex: (index) => index.isEven
                    ? AuctionStatus.active
                    : AuctionStatus.scheduled,
              ),
            );
          }
          return Result.success([
            _auction(id: 'a20', status: AuctionStatus.active),
            _auction(id: 'a21', status: AuctionStatus.waitingSettlement),
            _auction(id: 'a22', status: AuctionStatus.ended),
            _auction(id: 'a23', status: AuctionStatus.cancelled),
          ]);
        },
      );
      final auth = _FakeAuthController(
        AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
      );
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
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
      expect(state.auctions.map((a) => a.id), [
        for (var i = 1; i <= 23; i++) 'a$i',
      ]);
      expect(repo.requestedCursors, hasLength(2));
      expect(repo.requestedCursors.first, isNull);
      final cursor = repo.requestedCursors[1];
      expect(cursor, isNotNull);
      // LOCK: the backend parses cursor as RFC3339 and 400s anything else,
      // so an auction id here would break load-more on the real API.
      expect(
        DateTime.tryParse(cursor!),
        isNotNull,
        reason: 'cursor must be an RFC3339 timestamp',
      );
      expect(cursor, isNot('a20'), reason: 'never send an auction id');
    });

    test(
      'rapid duplicate loadMore is blocked while request is in flight',
      () async {
        final page2 = Completer<Result<List<Auction>>>();
        var page2Calls = 0;
        final repo = _FakeAuctionRepository(
          onGetUserAuctions: (sellerId, status, limit, cursor) async {
            if (cursor == null) {
              return Result.success(
                _auctionPage(
                  start: 1,
                  count: 20,
                  statusForIndex: (index) => index.isEven
                      ? AuctionStatus.active
                      : AuctionStatus.scheduled,
                ),
              );
            }
            page2Calls += 1;
            return page2.future;
          },
        );
        final auth = _FakeAuthController(
          AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
        );
        final container = ProviderContainer(
          overrides: [
            authControllerProvider.overrideWith(() => auth),
            auctionRepositoryProvider.overrideWithValue(repo),
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
          Result.success([
            _auction(id: 'a21', status: AuctionStatus.scheduled),
          ]),
        );
        await _settle();

        final state = container.read(sellerAuctionsPagerProvider);
        expect(state.auctions.length, 21);
      },
    );

    test('refresh resets to the latest first page', () async {
      var firstPageCalls = 0;
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          if (cursor != null) {
            return Result.success(const []);
          }
          firstPageCalls += 1;
          if (firstPageCalls == 1) {
            return Result.success([
              _auction(id: 'old-1', status: AuctionStatus.scheduled),
              _auction(id: 'old-2', status: AuctionStatus.active),
            ]);
          }
          return Result.success([
            _auction(id: 'new-1', status: AuctionStatus.scheduled),
            _auction(id: 'new-2', status: AuctionStatus.waitingSettlement),
          ]);
        },
      );
      final auth = _FakeAuthController(
        AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
      );
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
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
          return Result.success([
            _auction(id: 'a1', status: AuctionStatus.scheduled),
            _auction(id: 'a2', status: AuctionStatus.scheduled),
            _auction(id: 'a3', status: AuctionStatus.active),
            _auction(id: 'a4', status: AuctionStatus.waitingSettlement),
            _auction(id: 'a5', status: AuctionStatus.ended),
          ]);
        },
      );
      final auth = _FakeAuthController(
        AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
      );
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
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
      notifier.setFilter(AuctionStatus.active);
      expect(
        container
            .read(sellerAuctionsPagerProvider)
            .visibleAuctions
            .map((a) => a.id),
        ['a3'],
      );
      notifier.setFilter(null);
      expect(
        container
            .read(sellerAuctionsPagerProvider)
            .visibleAuctions
            .map((a) => a.id),
        ['a1', 'a2', 'a3', 'a4', 'a5'],
      );
    });

    test(
      'load more failure preserves current data and retry recovers',
      () async {
        var attempts = 0;
        final repo = _FakeAuctionRepository(
          onGetUserAuctions: (sellerId, status, limit, cursor) async {
            if (cursor == null) {
              return Result.success(
                _auctionPage(
                  start: 1,
                  count: 20,
                  statusForIndex: (index) => index.isEven
                      ? AuctionStatus.active
                      : AuctionStatus.scheduled,
                ),
              );
            }
            attempts += 1;
            if (attempts == 1) {
              return Result.error('load more failed');
            }
            return Result.success([
              _auction(id: 'a21', status: AuctionStatus.ended),
            ]);
          },
        );
        final auth = _FakeAuthController(
          AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
        );
        final container = ProviderContainer(
          overrides: [
            authControllerProvider.overrideWith(() => auth),
            auctionRepositoryProvider.overrideWithValue(repo),
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
      },
    );

    test(
      'auth change discards stale publication and reloads from new seller',
      () async {
        final seller1Pending = Completer<Result<List<Auction>>>();
        final repo = _FakeAuctionRepository(
          onGetUserAuctions: (sellerId, status, limit, cursor) async {
            if (sellerId == 'seller-1') {
              return seller1Pending.future;
            }
            return Result.success([
              _auction(id: 'b1', status: AuctionStatus.scheduled),
            ]);
          },
        );
        final auth = _FakeAuthController(
          AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
        );
        final container = ProviderContainer(
          overrides: [
            authControllerProvider.overrideWith(() => auth),
            auctionRepositoryProvider.overrideWithValue(repo),
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
          Result.success([_auction(id: 'a1', status: AuctionStatus.active)]),
        );
        await _settle();

        final state = container.read(sellerAuctionsPagerProvider);
        expect(state.ownerId, 'seller-2');
        expect(state.auctions.map((a) => a.id), ['b1']);
        expect(repo.requestedSellerIds, ['seller-1', 'seller-2']);
      },
    );
  });

  group('SellerAuctionsScreen', () {
    testWidgets(
      'waitingSettlement is labeled clearly and detail backstack works',
      (tester) async {
        final repo = _FakeAuctionRepository(
          onGetUserAuctions: (sellerId, status, limit, cursor) async {
            return Result.success([
              _auction(
                id: 'a1',
                status: AuctionStatus.waitingSettlement,
                currentBid: 1500000,
                winnerId: 'buyer-1',
              ),
            ]);
          },
        );
        final auth = _FakeAuthController(
          AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
        );
        final router = GoRouter(
          initialLocation: '/seller/auctions',
          routes: [
            GoRoute(
              path: '/seller/auctions',
              builder: (context, state) => ProviderScope(
                overrides: [
                  authControllerProvider.overrideWith(() => auth),
                  auctionRepositoryProvider.overrideWithValue(repo),
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

        expect(
          find.descendant(
            of: find.byKey(const ValueKey('seller-auction-card-a1')),
            matching: find.text('Menunggu Penyelesaian'),
          ),
          findsOneWidget,
        );
        expect(find.text('detail a1'), findsNothing);

        await tester.tap(find.byKey(const ValueKey('seller-auction-card-a1')));
        await tester.pumpAndSettle();
        expect(find.text('detail a1'), findsOneWidget);

        router.pop();
        await tester.pumpAndSettle();
        expect(
          find.descendant(
            of: find.byKey(const ValueKey('seller-auction-card-a1')),
            matching: find.text('Menunggu Penyelesaian'),
          ),
          findsOneWidget,
        );
      },
    );

    testWidgets('edit is exposed only for scheduled auctions', (tester) async {
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          return Result.success([
            _auction(id: 'draft-1', status: AuctionStatus.scheduled),
            _auction(id: 'active-1', status: AuctionStatus.active),
          ]);
        },
      );
      final auth = _FakeAuthController(
        AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
      );
      final router = GoRouter(
        initialLocation: '/seller/auctions',
        routes: [
          GoRoute(
            path: '/seller/auctions',
            builder: (context, state) => ProviderScope(
              overrides: [
                authControllerProvider.overrideWith(() => auth),
                auctionRepositoryProvider.overrideWithValue(repo),
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
      expect(find.text('Edit'), findsOneWidget);
      expect(find.text('Batalkan'), findsOneWidget);

      await tester.tapAt(const Offset(1, 1));
      await tester.pumpAndSettle();

      await tester.tap(popupButtons.last);
      await tester.pumpAndSettle();
      expect(find.text('Edit'), findsNothing);
    });

    testWidgets(
      'relist is offered only for auctions that ended without a bid',
      (tester) async {
        final repo = _FakeAuctionRepository(
          onGetUserAuctions: (sellerId, status, limit, cursor) async {
            return Result.success([
              _auction(id: 'expired-1', status: AuctionStatus.ended),
              _auction(
                id: 'sold-1',
                status: AuctionStatus.ended,
                winnerId: 'buyer-1',
              ),
            ]);
          },
        );
        final auth = _FakeAuthController(
          AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
        );
        final router = GoRouter(
          initialLocation: '/seller/auctions',
          routes: [
            GoRoute(
              path: '/seller/auctions',
              builder: (context, state) => ProviderScope(
                overrides: [
                  authControllerProvider.overrideWith(() => auth),
                  auctionRepositoryProvider.overrideWithValue(repo),
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

        // POSITIVE: only the no-bid ended auction advertises relist.
        expect(find.text('Bisa direlist'), findsOneWidget);

        // POSITIVE: the no-bid ended auction offers Relist.
        await tester.tap(popupButtons.first);
        await tester.pumpAndSettle();
        expect(find.text('Relist'), findsOneWidget);
        expect(repo.relistCalls, isEmpty, reason: 'menu must not fire on open');
        await tester.tapAt(const Offset(1, 1));
        await tester.pumpAndSettle();

        // NEGATIVE: an auction that produced a winner never offers Relist.
        await tester.tap(popupButtons.last);
        await tester.pumpAndSettle();
        expect(find.text('Relist'), findsNothing);
        expect(find.text('Bisa direlist'), findsOneWidget);
      },
    );

    testWidgets('scheduled editor calls update endpoint and returns success', (
      tester,
    ) async {
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          return Result.success([
            _auction(id: 'scheduled-1', status: AuctionStatus.scheduled),
          ]);
        },
      );
      final auth = _FakeAuthController(
        AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(() => auth),
            auctionRepositoryProvider.overrideWithValue(repo),
            loggerServiceProvider.overrideWithValue(_FakeLoggerService()),
          ],
          child: MaterialApp(
            home: SellerAuctionEditScreen(
              auction: _auction(
                id: 'scheduled-1',
                status: AuctionStatus.scheduled,
              ),
            ),
          ),
        ),
      );

      await tester.enterText(find.byType(TextFormField).at(0), 'Judul Baru');
      await tester.enterText(
        find.byType(TextFormField).at(1),
        'Deskripsi Baru',
      );

      await tester.tap(find.text('Simpan Perubahan'));
      await tester.pumpAndSettle();

      expect(repo.updateCalls, hasLength(1));
      expect(repo.updateCalls.single['title'], 'Judul Baru');
      expect(repo.updateCalls.single['description'], 'Deskripsi Baru');
      expect(find.text('Edit Lelang'), findsNothing);
    });

    testWidgets(
      'cancel confirmation uses AppDialog and calls the cancel endpoint once',
      (tester) async {
        final repo = _FakeAuctionRepository(
          onGetUserAuctions: (sellerId, status, limit, cursor) async {
            return Result.success([
              _auction(id: 'scheduled-1', status: AuctionStatus.scheduled),
            ]);
          },
        );
        final cancelNotifier = _FakeCancelNotifier();
        final auth = _FakeAuthController(
          AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
        );
        final router = GoRouter(
          initialLocation: '/seller/auctions',
          routes: [
            GoRoute(
              path: '/seller/auctions',
              builder: (context, state) => ProviderScope(
                overrides: [
                  authControllerProvider.overrideWith(() => auth),
                  auctionRepositoryProvider.overrideWithValue(repo),
                  auctionNotifierProvider.overrideWith(() => cancelNotifier),
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

        // Open the card menu → Batalkan → canonical confirmation.
        await tester.tap(find.byType(PopupMenuButton<String>).first);
        await tester.pumpAndSettle();
        await tester.tap(find.text('Batalkan'));
        await tester.pumpAndSettle();

        expect(find.text('Batalkan lelang'), findsOneWidget);
        final scheme = Theme.of(
          tester.element(find.byType(AlertDialog)),
        ).colorScheme;
        final confirm = tester.widget<ElevatedButton>(
          find.widgetWithText(ElevatedButton, 'Batalkan'),
        );
        expect(
          confirm.style?.backgroundColor?.resolve(const <WidgetState>{}),
          scheme.error,
          reason: 'auction cancellation is a destructive confirmation',
        );

        // Cancel first: no endpoint call.
        await tester.tap(find.text('Batal'));
        await tester.pumpAndSettle();
        expect(cancelNotifier.cancelled, isEmpty);

        // Confirm: exactly one endpoint call.
        await tester.tap(find.byType(PopupMenuButton<String>).first);
        await tester.pumpAndSettle();
        await tester.tap(find.text('Batalkan'));
        await tester.pumpAndSettle();
        await tester.tap(find.widgetWithText(ElevatedButton, 'Batalkan'));
        await tester.pumpAndSettle();

        expect(cancelNotifier.cancelled, <String>['scheduled-1']);
      },
    );
  });

  group('My Auctions canonical filter contract', () {
    test('options are the six canonical statuses plus Semua, ordered', () {
      expect(kSellerAuctionFilters, const <AuctionStatus?>[
        null,
        AuctionStatus.scheduled,
        AuctionStatus.active,
        AuctionStatus.waitingSettlement,
        AuctionStatus.ended,
        AuctionStatus.cancelled,
        AuctionStatus.lapsed,
      ]);
      expect(sellerAuctionFilterLabel(null), 'Semua');
      for (final status in AuctionStatus.values) {
        expect(sellerAuctionFilterLabel(status), status.displayName);
      }
      expect(
        AuctionStatus.waitingSettlement.displayName,
        'Menunggu Penyelesaian',
      );
    });

    test('each filter selects exactly one canonical status', () async {
      final data = [
        _auction(id: 's', status: AuctionStatus.scheduled),
        _auction(id: 'a', status: AuctionStatus.active),
        _auction(id: 'w', status: AuctionStatus.waitingSettlement),
        _auction(id: 'e', status: AuctionStatus.ended),
        _auction(id: 'c', status: AuctionStatus.cancelled),
        _auction(id: 'l', status: AuctionStatus.lapsed),
      ];
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async =>
            Result.success(data),
      );
      final auth = _FakeAuthController(
        AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
      );
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(
        sellerAuctionsPagerProvider,
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(subscription.close);
      await _settle();

      List<String> visible() => container
          .read(sellerAuctionsPagerProvider)
          .visibleAuctions
          .map((a) => a.id)
          .toList();

      // Default = Semua (all six).
      expect(visible(), ['s', 'a', 'w', 'e', 'c', 'l']);

      final notifier = container.read(sellerAuctionsPagerProvider.notifier);
      final expected = <AuctionStatus, List<String>>{
        AuctionStatus.scheduled: ['s'],
        AuctionStatus.active: ['a'],
        AuctionStatus.waitingSettlement: ['w'],
        AuctionStatus.ended: ['e'],
        AuctionStatus.cancelled: ['c'],
        AuctionStatus.lapsed: ['l'],
      };
      for (final entry in expected.entries) {
        notifier.setFilter(entry.key);
        expect(visible(), entry.value, reason: '${entry.key}');
      }

      notifier.setFilter(null);
      expect(visible(), ['s', 'a', 'w', 'e', 'c', 'l']);
    });

    test('production code has no finished aggregate or local label map', () {
      final pager = File(
        'lib/domains/commerce/catalog/auction/presentation/providers/seller_auctions_pager.dart',
      ).readAsStringSync();
      final screen = File(
        'lib/domains/commerce/catalog/auction/presentation/screens/seller_auctions_screen.dart',
      ).readAsStringSync();

      expect(pager.contains('finished'), isFalse);
      expect(pager.contains('enum SellerAuctionFilter'), isFalse);
      expect(pager.contains('SellerAuctionFilter.'), isFalse);
      expect(pager.contains("'Selesai'"), isFalse);
      expect(pager.contains('AuctionStatus.waitingSettlement ||'), isFalse);
      expect(screen.contains('_statusLabel'), isFalse);
      expect(screen.contains('AuctionStatus.waitingSettlement ||'), isFalse);
      // The ChoiceChip filter control is fully replaced by a real TabBar.
      expect(screen.contains('ChoiceChip'), isFalse);
      expect(screen.contains('TabBar'), isTrue);
      // Tabs derive from the single canonical source.
      expect(screen.contains('kSellerAuctionFilters'), isTrue);
    });

    testWidgets('renders exactly the seven canonical filter tabs', (
      tester,
    ) async {
      _useTallViewport(tester);
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async =>
            Result.success([_auction(id: 'a1', status: AuctionStatus.active)]),
      );
      await _pumpSellerAuctions(tester, repo);

      expect(find.byType(TabBar), findsOneWidget);
      expect(find.byType(Tab), findsNWidgets(7));
      expect(find.byType(ChoiceChip), findsNothing);

      const labels = [
        'Semua',
        'Terjadwal',
        'Aktif',
        'Menunggu Penyelesaian',
        'Berakhir',
        'Dibatalkan',
        'Kadaluarsa',
      ];
      for (final label in labels) {
        expect(_tab(label), findsOneWidget, reason: label);
      }
    });

    testWidgets('defaults to the Semua tab (activeFilter == null)', (
      tester,
    ) async {
      _useTallViewport(tester);
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async =>
            Result.success([
              _auction(
                id: 's',
                status: AuctionStatus.scheduled,
                title: 'Koi s',
              ),
              _auction(id: 'a', status: AuctionStatus.active, title: 'Koi a'),
            ]),
      );
      await _pumpSellerAuctions(tester, repo);

      // Semua = every canonical status.
      expect(find.text('Koi s'), findsOneWidget);
      expect(find.text('Koi a'), findsOneWidget);
      expect(tester.widget<TabBar>(find.byType(TabBar)).controller!.index, 0);
    });

    testWidgets(
      'tab selection maps to the canonical filter and filters the list',
      (tester) async {
        _useTallViewport(tester);
        var fetchCount = 0;
        final data = [
          _auction(id: 's', status: AuctionStatus.scheduled, title: 'Koi s'),
          _auction(id: 'a', status: AuctionStatus.active, title: 'Koi a'),
          _auction(
            id: 'w',
            status: AuctionStatus.waitingSettlement,
            title: 'Koi w',
          ),
          _auction(id: 'e', status: AuctionStatus.ended, title: 'Koi e'),
          _auction(id: 'c', status: AuctionStatus.cancelled, title: 'Koi c'),
          _auction(id: 'l', status: AuctionStatus.lapsed, title: 'Koi l'),
        ];
        final repo = _FakeAuctionRepository(
          onGetUserAuctions: (sellerId, status, limit, cursor) async {
            fetchCount++;
            return Result.success(data);
          },
        );

        await _pumpSellerAuctions(tester, repo);
        expect(fetchCount, 1);

        // Semua.
        for (final id in ['s', 'a', 'w', 'e', 'c', 'l']) {
          expect(find.text('Koi $id'), findsOneWidget, reason: 'Semua/$id');
        }

        final cases = <String, String>{
          'Terjadwal': 's',
          'Aktif': 'a',
          'Menunggu Penyelesaian': 'w',
          'Berakhir': 'e',
          'Dibatalkan': 'c',
          'Kadaluarsa': 'l',
        };
        for (final entry in cases.entries) {
          await tester.tap(_tab(entry.key));
          await tester.pumpAndSettle();
          expect(
            find.text('Koi ${entry.value}'),
            findsOneWidget,
            reason: entry.key,
          );
          for (final other in ['s', 'a', 'w', 'e', 'c', 'l']) {
            if (other != entry.value) {
              expect(find.text('Koi $other'), findsNothing, reason: entry.key);
            }
          }
        }

        await tester.tap(_tab('Semua'));
        await tester.pumpAndSettle();
        for (final id in ['s', 'a', 'w', 'e', 'c', 'l']) {
          expect(find.text('Koi $id'), findsOneWidget);
        }

        // Changing tabs must not recreate/refetch the owner inventory.
        expect(fetchCount, 1);
      },
    );

    testWidgets('empty filtered result keeps the canonical empty state', (
      tester,
    ) async {
      _useTallViewport(tester);
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async =>
            Result.success([
              _auction(
                id: 's',
                status: AuctionStatus.scheduled,
                title: 'Koi s',
              ),
            ]),
      );
      await _pumpSellerAuctions(tester, repo);

      await tester.tap(_tab('Aktif'));
      await tester.pumpAndSettle();

      // Auctions exist, the active status tab matched none → FILTER empty
      // with a reset back to "Semua", never the collection-empty copy.
      expect(find.text('Tidak Ada Hasil'), findsOneWidget);
      expect(find.text('Belum ada lelang'), findsNothing);
      expect(find.widgetWithText(FilledButton, 'Atur Ulang'), findsOneWidget);

      await tester.tap(find.widgetWithText(FilledButton, 'Atur Ulang'));
      await tester.pumpAndSettle();

      // Reset returns to the unfiltered collection, where the data lives.
      expect(find.text('Tidak Ada Hasil'), findsNothing);
      expect(find.text('Koi s'), findsOneWidget);
    });

    testWidgets('loading state is preserved', (tester) async {
      final completer = Completer<Result<List<Auction>>>();
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) =>
            completer.future,
      );
      await _pumpSellerAuctions(tester, repo, settle: false);
      await tester.pump();

      // First load without data renders the canonical LoadingIndicator —
      // never EmptyState, never a raw spinner branch.
      expect(find.byType(LoadingIndicator), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      completer.complete(Result.success(const <Auction>[]));
      await tester.pumpAndSettle();
    });

    testWidgets('error state is preserved', (tester) async {
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async =>
            Result.error('boom'),
      );
      await _pumpSellerAuctions(tester, repo);

      // First-load failure renders the canonical PageErrorState with safe
      // localized copy only — the raw backend error never reaches the UI.
      expect(find.byType(PageErrorState), findsOneWidget);
      expect(find.text('Terjadi Kesalahan'), findsOneWidget);
      expect(find.text('Coba Lagi'), findsOneWidget);
      expect(find.text('boom'), findsNothing);
      expect(find.text('Gagal memuat lelang'), findsNothing);
      expect(find.byType(EmptyState), findsNothing);
    });
  });

  group('SellerAuctions loading foundation convergence', () {
    ProviderContainer pagerContainer(_FakeAuctionRepository repo) {
      final auth = _FakeAuthController(
        AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
      );
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => auth),
          auctionRepositoryProvider.overrideWithValue(repo),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(
        sellerAuctionsPagerProvider,
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(subscription.close);
      return container;
    }

    test('refresh failure preserves auctions with refreshError only', () async {
      var failNext = false;
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          if (failNext) {
            failNext = false;
            return Result.error('HTTP 500');
          }
          return Result.success([
            _auction(id: 'a1', status: AuctionStatus.active, title: 'Koi a1'),
          ]);
        },
      );
      final container = pagerContainer(repo);
      await _settle();
      expect(
        container.read(sellerAuctionsPagerProvider).auctions.map((a) => a.id),
        ['a1'],
      );

      failNext = true;
      await container.read(sellerAuctionsPagerProvider.notifier).refresh();
      await _settle();

      final state = container.read(sellerAuctionsPagerProvider);
      expect(state.auctions.map((a) => a.id), ['a1']);
      expect(state.initialError, isNull);
      expect(state.refreshError, isNotNull);
      expect(state.isRefreshing, isFalse);
      expect(state.isInitialLoading, isFalse);
    });

    test('refresh success replaces auctions and clears refreshError', () async {
      var data = [
        _auction(id: 'a1', status: AuctionStatus.active, title: 'Koi a1'),
      ];
      var failNext = true;
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          if (failNext) {
            failNext = false;
            return Result.error('HTTP 500');
          }
          return Result.success(data);
        },
      );
      final container = pagerContainer(repo);
      await _settle();

      // Initial load failed (failNext consumed); refresh recovers to data.
      await container.read(sellerAuctionsPagerProvider.notifier).refresh();
      await _settle();
      expect(
        container.read(sellerAuctionsPagerProvider).auctions.map((a) => a.id),
        ['a1'],
      );
      expect(container.read(sellerAuctionsPagerProvider).refreshError, isNull);

      data = [
        _auction(id: 'a2', status: AuctionStatus.active, title: 'Koi a2'),
      ];
      await container.read(sellerAuctionsPagerProvider.notifier).refresh();
      await _settle();

      final state = container.read(sellerAuctionsPagerProvider);
      expect(state.auctions.map((a) => a.id), ['a2']);
      expect(state.refreshError, isNull);
      expect(state.initialError, isNull);
    });

    testWidgets('successful collection empty renders EmptyState', (
      tester,
    ) async {
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async =>
            Result.success(const <Auction>[]),
      );
      await _pumpSellerAuctions(tester, repo);

      expect(find.byType(EmptyState), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.byType(LoadingIndicator), findsNothing);
    });

    testWidgets('refresh failure keeps rows with inline banner and retry', (
      tester,
    ) async {
      _useTallViewport(tester);
      var calls = 0;
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          calls++;
          if (calls == 1) {
            return Result.success([
              _auction(
                id: 'a1',
                status: AuctionStatus.active,
                title: 'Koi lama',
              ),
            ]);
          }
          if (calls == 2) return Result.error('HTTP 500');
          return Result.success([
            _auction(id: 'a2', status: AuctionStatus.active, title: 'Koi baru'),
          ]);
        },
      );
      await _pumpSellerAuctions(tester, repo);
      expect(find.text('Koi lama'), findsOneWidget);

      // App-bar refresh → pager.refresh() (same operation as pull-to-refresh).
      await tester.tap(find.byIcon(Icons.refresh_outlined));
      await tester.pumpAndSettle();

      // Old data stays; no full-page error; inline banner with retry.
      expect(find.text('Koi lama'), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      // The raw backend failure never reaches the UI.
      expect(find.textContaining('HTTP 500'), findsNothing);

      // Banner retry re-executes refresh and swaps in the new data.
      await tester.tap(find.text('Coba Lagi'));
      await tester.pumpAndSettle();
      expect(find.text('Koi baru'), findsOneWidget);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsNothing,
      );
      expect(calls, 3);
    });

    testWidgets('in-flight refresh keeps rows with update indicator', (
      tester,
    ) async {
      _useTallViewport(tester);
      final gate = Completer<Result<List<Auction>>>();
      var calls = 0;
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) {
          calls++;
          if (calls == 1) {
            return Future.value(
              Result.success([
                _auction(
                  id: 'a1',
                  status: AuctionStatus.active,
                  title: 'Koi lama',
                ),
              ]),
            );
          }
          return gate.future;
        },
      );
      await _pumpSellerAuctions(tester, repo);
      expect(find.text('Koi lama'), findsOneWidget);

      await tester.tap(find.byIcon(Icons.refresh_outlined));
      await tester.pump();

      // Rows stay visible with a thin update indicator — never a full-page
      // loading swap.
      expect(find.text('Koi lama'), findsOneWidget);
      expect(find.byType(LinearProgressIndicator), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete(
        Result.success([
          _auction(id: 'a2', status: AuctionStatus.active, title: 'Koi baru'),
        ]),
      );
      await tester.pumpAndSettle();
      expect(find.text('Koi baru'), findsOneWidget);
    });

    testWidgets('pagination loading uses LoadingIndicator and keeps rows', (
      tester,
    ) async {
      _useTallViewport(tester);
      final gate = Completer<Result<List<Auction>>>();
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          if (cursor == null) {
            return Result.success(
              _auctionPage(
                start: 1,
                count: 20,
                statusForIndex: (_) => AuctionStatus.active,
              ),
            );
          }
          return gate.future;
        },
      );
      await _pumpSellerAuctions(tester, repo);
      // All 20 generated rows share the default title.
      expect(find.text('Kohaku 50cm'), findsWidgets);

      await tester.tap(find.text('Muat lebih banyak'));
      await tester.pump();

      expect(find.byType(LoadingIndicator), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete(
        Result.success([_auction(id: 'b1', status: AuctionStatus.active)]),
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey('seller-auction-card-b1')),
        findsOneWidget,
      );
    });

    testWidgets('pagination failure stays controlled with retry', (
      tester,
    ) async {
      _useTallViewport(tester);
      var failNext = false;
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async {
          if (cursor != null) {
            if (failNext) {
              failNext = false;
              return Result.error('pg-boom-500');
            }
            return Result.success([
              _auction(id: 'b1', status: AuctionStatus.active),
            ]);
          }
          return Result.success(
            _auctionPage(
              start: 1,
              count: 20,
              statusForIndex: (_) => AuctionStatus.active,
            ),
          );
        },
      );
      await _pumpSellerAuctions(tester, repo);

      failNext = true;
      await tester.tap(find.text('Muat lebih banyak'));
      await tester.pumpAndSettle();

      // Loaded rows stay; controlled inline row with safe copy + retry —
      // the raw backend error never reaches the UI.
      expect(
        find.byKey(const ValueKey('seller-auction-card-a1')),
        findsOneWidget,
      );
      expect(find.byType(PageErrorState), findsNothing);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.textContaining('pg-boom-500'), findsNothing);

      await tester.tap(find.text('Coba Lagi'));
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey('seller-auction-card-b1')),
        findsOneWidget,
      );
    });

    test('negative proof — obsolete seller-auctions paths cannot return', () {
      final screen = File(
        'lib/domains/commerce/catalog/auction/presentation/screens/seller_auctions_screen.dart',
      ).readAsStringSync().replaceAll('\r\n', '\n');

      // No local obsolete error authority.
      expect(screen.contains('class _ErrorState'), isFalse);
      expect(screen.contains('_ErrorState('), isFalse);
      // No raw spinners for the touched state semantics.
      expect(screen.contains('CircularProgressIndicator'), isFalse);
      // No raw backend error force-unwrapped into the widget tree.
      expect(screen.contains('initialError!'), isFalse);
      expect(screen.contains('loadMoreError!'), isFalse);
      expect(screen.contains('refreshError!'), isFalse);
      // Canonical renderers own each semantic exactly once.
      expect(screen.contains('LoadingIndicator('), isTrue);
      expect(screen.contains('PageErrorState('), isTrue);
      expect(screen.contains('EmptyState('), isTrue);
      // Refresh separation is consumed, not just produced.
      expect(screen.contains('isRefreshing'), isTrue);
      expect(screen.contains('refreshError'), isTrue);
      // The single initial-retry authority is used by the screen; the raw
      // initial-load call is not a second retry path.
      expect(screen.contains('pager.retryInitial'), isTrue);
      expect(screen.contains('onRetry: pager.loadInitial'), isFalse);
    });
  });

  group('SellerAuctionsScreen — canonical create entry', () {
    testWidgets('exposes the page-level create affordance', (tester) async {
      _useTallViewport(tester);
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async =>
            Result.success([_auction(id: 'a1', status: AuctionStatus.active)]),
      );
      await _pumpSellerAuctions(tester, repo);

      expect(
        find.widgetWithText(FloatingActionButton, 'Buat Lelang'),
        findsOneWidget,
      );
    });

    testWidgets('opens the canonical route carrying the stay intent', (
      tester,
    ) async {
      _useTallViewport(tester);
      final repo = _FakeAuctionRepository(
        onGetUserAuctions: (sellerId, status, limit, cursor) async =>
            Result.success([_auction(id: 'a1', status: AuctionStatus.active)]),
      );
      final auth = _FakeAuthController(
        AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
      );
      CreateAuctionRouteArgs? capturedExtra;
      final router = GoRouter(
        initialLocation: '/seller/auctions',
        routes: [
          GoRoute(
            path: '/seller/auctions',
            builder: (context, state) => ProviderScope(
              overrides: [
                authControllerProvider.overrideWith(() => auth),
                auctionRepositoryProvider.overrideWithValue(repo),
                loggerServiceProvider.overrideWithValue(_FakeLoggerService()),
              ],
              child: const SellerAuctionsScreen(),
            ),
          ),
          GoRoute(
            path: RoutePaths.createAuction,
            builder: (context, state) {
              capturedExtra = state.extra is CreateAuctionRouteArgs
                  ? state.extra as CreateAuctionRouteArgs
                  : null;
              return const Scaffold(body: Text('create-route'));
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

      await tester.tap(
        find.widgetWithText(FloatingActionButton, 'Buat Lelang'),
      );
      await tester.pumpAndSettle();

      expect(find.text('create-route'), findsOneWidget);
      expect(capturedExtra, isNotNull);
      expect(capturedExtra!.landing, CreateAuctionLanding.stay);
      expect(capturedExtra!.landsOnMarketplace, isFalse);
    });

    test('entry uses the canonical route/intent and adds no duplicate flow', () {
      final screen = File(
        'lib/domains/commerce/catalog/auction/presentation/screens/seller_auctions_screen.dart',
      ).readAsStringSync();

      // Canonical route constant, never a hardcoded path literal.
      expect(screen.contains('RoutePaths.createAuction'), isTrue);
      expect(screen.contains("'/create/auction'"), isFalse);
      // Explicit caller intent (not the default Marketplace landing).
      expect(screen.contains('CreateAuctionRouteArgs.stay()'), isTrue);
      // No duplicate create screen / no local create controller.
      expect(screen.contains('CreateAuctionScreen('), isFalse);
      expect(screen.contains('createAuctionController'), isFalse);
    });
  });
}

Finder _tab(String label) =>
    find.descendant(of: find.byType(TabBar), matching: find.text(label));

void _useTallViewport(WidgetTester tester) {
  tester.view.physicalSize = const Size(2000, 5000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}

Future<void> _pumpSellerAuctions(
  WidgetTester tester,
  _FakeAuctionRepository repo, {
  bool settle = true,
}) async {
  final auth = _FakeAuthController(
    AuthState.authenticated(_seller(id: 'seller-1'), emailVerified: true),
  );
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(() => auth),
        auctionRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(_FakeLoggerService()),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: const SellerAuctionsScreen(),
      ),
    ),
  );
  if (settle) await tester.pumpAndSettle();
}
