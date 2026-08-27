import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/repositories/auction_watch_repository_impl.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_bottom_bar.dart';
import 'package:labuda/domains/user/preference/saved_item/data/repositories/saved_item_repository.dart';
import 'package:labuda/domains/user/preference/saved_item/models/saved_item_model.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

Widget _wrap(Widget child) {
  return ProviderScope(
    child: MaterialApp(home: Scaffold(body: child)),
  );
}

Auction _auction() {
  return Auction(
    id: 'auction-1',
    sellerId: 'seller-1',
    sellerUsername: 'yayan',
    sellerFarmName: 'Farm Koi Nusantara',
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
    title: 'Sanke Auction',
    description: 'Live auction',
    koiDetails: const KoiDetails(
      variety: 'Kohaku',
      sizeInCm: 0,
      ageInMonths: 0,
      gender: 'unknown',
      certificates: [],
    ),
    openingBid: 1000000,
    currentBid: 1500000,
    bidIncrement: 50000,
    buyNowPrice: 2500000,
    startTime: DateTime.parse('2026-01-01T00:00:00.000Z'),
    endTime: DateTime.parse('2026-01-02T00:00:00.000Z'),
    status: AuctionStatus.active,
    totalBidders: 0,
    totalWatchers: 0,
    totalViews: 0,
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
  );
}

class _NoopLogger implements ILoggerService {
  @override
  dynamic noSuchMethod(Invocation invocation) =>
      Future.value(Result.success(null));
}

class _FakeSavedItemRepository extends SavedItemRepository {
  _FakeSavedItemRepository()
    : super(dio: Dio(BaseOptions(baseUrl: 'http://localhost')));

  bool watched = false;
  int addCalls = 0;
  int removeCalls = 0;
  int isSavedCalls = 0;
  String? lastTargetType;
  String? lastTargetId;

  @override
  Future<SavedItemModel> addSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    addCalls += 1;
    watched = true;
    lastTargetType = targetType;
    lastTargetId = targetId;
    return SavedItemModel(
      id: 'saved-1',
      userId: 'user-1',
      targetType: TargetType.auction,
      targetId: targetId,
      intentType: IntentType.watch,
      createdAt: DateTime.utc(2026, 1, 1),
    );
  }

  @override
  Future<void> removeSavedItem({
    required String targetType,
    required String targetId,
  }) async {
    removeCalls += 1;
    watched = false;
    lastTargetType = targetType;
    lastTargetId = targetId;
  }

  @override
  Future<bool> isSaved({
    required String targetType,
    required String targetId,
  }) async {
    isSavedCalls += 1;
    lastTargetType = targetType;
    lastTargetId = targetId;
    return watched;
  }
}

void main() {
  group('Auction watch/favorite parity', () {
    testWidgets('bottom bar shows saved-state copy instead of numeric count', (
      tester,
    ) async {
      await tester.pumpWidget(
        _wrap(
          AuctionDetailBottomBar(
            auction: _auction(),
            watchStatsAsync: AsyncValue.data(
              const AuctionWatchStats(
                auctionId: 'auction-1',
                totalWatchers: null,
                isWatchedByCurrentUser: false,
              ),
            ),
            currentUserId: 'user-1',
            currentUserName: 'alice',
            onWatch: () {},
            onChat: () {},
            onAction: () {},
          ),
        ),
      );

      expect(find.text('Simpan'), findsOneWidget);
      expect(find.text('Tersimpan'), findsNothing);
      expect(find.text('0'), findsNothing);
    });

    testWidgets(
      'bottom bar shows tersimpan when current user already watches',
      (tester) async {
        await tester.pumpWidget(
          _wrap(
            AuctionDetailBottomBar(
              auction: _auction(),
              watchStatsAsync: AsyncValue.data(
                const AuctionWatchStats(
                  auctionId: 'auction-1',
                  totalWatchers: null,
                  isWatchedByCurrentUser: true,
                ),
              ),
              currentUserId: 'user-1',
              currentUserName: 'alice',
              onWatch: () {},
              onChat: () {},
              onAction: () {},
            ),
          ),
        );

        expect(find.text('Tersimpan'), findsOneWidget);
        expect(find.text('Simpan'), findsNothing);
        expect(find.text('0'), findsNothing);
      },
    );

    test(
      'watch repository stays on saved-items and count/list are unsupported',
      () async {
        final savedRepo = _FakeSavedItemRepository();
        final repo = AuctionWatchRepositoryImpl(
          savedItemRepo: savedRepo,
          logger: _NoopLogger(),
        );

        final watchResult = await repo.watchAuction(
          auctionId: 'auction-1',
          userId: 'user-1',
        );
        expect(watchResult.isSuccess, isTrue);
        expect(savedRepo.addCalls, 1);
        expect(savedRepo.lastTargetType, 'auction');
        expect(savedRepo.lastTargetId, 'auction-1');

        final watchingResult = await repo.isWatching(
          auctionId: 'auction-1',
          userId: 'user-1',
        );
        expect(watchingResult.isSuccess, isTrue);
        expect(watchingResult.data, isTrue);
        expect(savedRepo.isSavedCalls, 1);

        final toggleOffResult = await repo.toggleWatch(
          auctionId: 'auction-1',
          userId: 'user-1',
        );
        expect(toggleOffResult.isSuccess, isTrue);
        expect(toggleOffResult.data, isFalse);
        expect(savedRepo.removeCalls, 1);

        final countResult = await repo.getWatchCount('auction-1');
        expect(countResult.isError, isTrue);
        expect(countResult.error, contains('real aggregate'));

        final watchersResult = await repo.getAuctionWatchers(
          auctionId: 'auction-1',
        );
        expect(watchersResult.isError, isTrue);
        expect(watchersResult.error, contains('real source'));
      },
    );

    test('source contract keeps guest login gate and avoids /watch endpoint', () {
      final root = Directory.current;
      final screenSource = File(
        '${root.path}\\lib\\domains\\commerce\\catalog\\auction\\presentation\\screens\\auction_detail_screen.dart',
      ).readAsStringSync();
      final datasourceSource = File(
        '${root.path}\\lib\\domains\\commerce\\catalog\\auction\\data\\remote\\auction_remote_datasource.dart',
      ).readAsStringSync();

      expect(
        screenSource.contains('You must be logged in to watch auction'),
        isTrue,
      );
      expect(screenSource.contains('auctionWatchRepositoryProvider'), isTrue);
      expect(datasourceSource.contains('/auctions/:id/watch'), isFalse);
    });
  });
}
