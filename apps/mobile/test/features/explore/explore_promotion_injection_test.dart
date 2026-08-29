import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/auction_providers.dart'
    show auctionRepositoryProvider;
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/repositories/auction_repository.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/repositories/for_sale_repository.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart'
    show forSaleRepositoryProvider;
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/promotion_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/promotion_discovery_service.dart';
import 'package:labuda/features/explore/explore.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart'
    show PromotedExternalCard;
import 'package:labuda/shared/services/logger_service.dart';

class _FakeForSaleRepository implements ForSaleRepository {
  final List<ForSale> listings;

  _FakeForSaleRepository(this.listings);

  @override
  Future<Result<List<ForSale>>> getForSales(GetForSalesParams params) async {
    return Result.success(listings);
  }

  @override
  Future<Result<ForSale?>> getForSaleById(
    String forSaleId,
  ) async {
    final listing =
        listings
            .where((item) => item.forSaleId == forSaleId)
            .isEmpty
        ? null
        : listings.firstWhere(
            (item) => item.forSaleId == forSaleId,
          );
    return Result.success(listing);
  }

  @override
  Future<Result<List<ForSale>>> getForSalesByIds(
    List<String> forSaleIds,
  ) async {
    return Result.success(
      listings
          .where((listing) => forSaleIds.contains(listing.forSaleId))
          .toList(),
    );
  }

  @override
  Future<Result<List<ForSale>>> getSellerForSales(
    String sellerId, {
    int page = 1,
    int pageSize = 20,
  }) async {
    return Result.success(const []);
  }

  @override
  Future<Result<ForSale>> createForSale(CreateForSaleRequest request) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<ForSale>> updateForSale(
    String forSaleId,
    UpdateForSaleRequest request,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<void>> deleteForSale(String forSaleId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<ForSale>> updateForSaleStatus(
    String fixedPriceSaleId,
    ForSaleStatus status,
  ) async {
    throw UnimplementedError();
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeAuctionRepository implements AuctionRepository {
  final List<Auction> auctions;

  _FakeAuctionRepository(this.auctions);

  @override
  Future<RepositoryResult<List<Auction>>> getActiveAuctions({
    String? variety,
    double? minSize,
    double? maxSize,
    double? maxBid,
    int limit = 20,
    String? lastAuctionId,
  }) async {
    return RepositoryResult.success(auctions);
  }

  @override
  Future<RepositoryResult<List<Auction>>> getUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 20,
    String? lastAuctionId,
  }) async {
    return RepositoryResult.success(const []);
  }

  @override
  Future<RepositoryResult<Auction>> getAuctionById(String auctionId) async {
    return RepositoryResult.success(
      auctions.firstWhere((auction) => auction.id == auctionId),
    );
  }

  @override
  Future<RepositoryResult<List<Auction>>> getAuctionsByIds(
    List<String> auctionIds,
  ) async {
    return RepositoryResult.success(
      auctions.where((auction) => auctionIds.contains(auction.id)).toList(),
    );
  }

  @override
  Future<RepositoryResult<Auction>> createAuction({
    required String sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    required String title,
    required String description,
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required double openingBid,
    required double bidIncrement,
    double? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
    String? farmAddressId,
    AuctionLocation? location,
    required List<String> shippingOptionIds,
    String? preparationNote,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<Auction>> updateAuction(
    String auctionId,
    Map<String, dynamic> updates,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<Auction>> updateAuctionStatus({
    required String auctionId,
    required AuctionStatus status,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<void>> cancelAuction({
    required String auctionId,
    required String sellerId,
    required String reason,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<AuctionBid>> placeBid({
    required String auctionId,
    required String bidderId,
    required double amount,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<List<AuctionBid>>> getAuctionBids({
    required String auctionId,
    int limit = 50,
  }) async {
    return RepositoryResult.success(const []);
  }

  @override
  Future<RepositoryResult<String>> claimAuction({
    required String auctionId,
    required String addressId,
    required String shippingOptionId,
    String? discountCode,
    bool useCoins = false,
  }) async {
    throw UnimplementedError();
  }

  @override
  Stream<List<Auction>> watchUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 100,
  }) {
    return const Stream<List<Auction>>.empty();
  }

  @override
  Stream<List<Auction>> watchActiveAuctions({int limit = 50}) {
    return Stream<List<Auction>>.value(auctions);
  }

  @override
  Stream<Auction?> watchAuction(String auctionId) {
    return const Stream<Auction?>.empty();
  }

  @override
  Stream<List<AuctionBid>> watchAuctionBids(
    String auctionId, {
    int limit = 50,
  }) {
    return const Stream<List<AuctionBid>>.empty();
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakePromotionDiscoveryService extends PromotionDiscoveryService {
  final List<String> promotedFixedPriceSaleIds;
  final List<String> promotedAuctionIds;

  _FakePromotionDiscoveryService({
    required this.promotedFixedPriceSaleIds,
    required this.promotedAuctionIds,
  }) : super(const _StubApiClient());

  @override
  Future<PromotedItemsResponse> getPromotedForSales({
    int limit = 10,
  }) async {
    return PromotedItemsResponse(
      promotedItems: promotedFixedPriceSaleIds
          .take(limit)
          .map(
            (id) => PromotedItemDto(
              instanceId: 'promo-$id',
              targetType: 'for_sale',
              targetId: id,
            ),
          )
          .toList(),
      count: promotedFixedPriceSaleIds.length,
    );
  }

  @override
  Future<PromotedItemsResponse> getPromotedAuctions({int limit = 10}) async {
    return PromotedItemsResponse(
      promotedItems: promotedAuctionIds
          .take(limit)
          .map(
            (id) => PromotedItemDto(
              instanceId: 'promo-$id',
              targetType: 'auction',
              targetId: id,
            ),
          )
          .toList(),
      count: promotedAuctionIds.length,
    );
  }
}

class _StubApiClient implements ApiClient {
  const _StubApiClient();

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError();

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _ThrowingApiClient implements ApiClient {
  const _ThrowingApiClient();

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    throw StateError('boom');
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

ForSale _forSale({required String id, required String title}) {
  final now = DateTime.utc(2026, 6, 9);
  return ForSale(
    forSaleId: id,
    title: title,
    description: '$title description',
    price: 150000,
    stock: 10,
    sellerId: 'seller-1',
    sellerUsername: 'seller_user',
    sellerFarmName: 'Farm Name',
    status: ForSaleStatus.active,
    createdAt: now,
    updatedAt: now,
  );
}

Auction _auction({required String id, required String title}) {
  final now = DateTime.now();
  return Auction(
    id: id,
    sellerId: 'seller-1',
    sellerUsername: 'seller_user',
    sellerFarmName: 'Farm Name',
    title: title,
    description: '$title description',
    koiDetails: const KoiDetails(
      variety: 'Kohaku',
      sizeInCm: 30,
      ageInMonths: 12,
      gender: 'male',
    ),
    openingBid: 150000,
    currentBid: 200000,
    bidIncrement: 5000,
    startTime: now,
    endTime: now.add(const Duration(days: 1)),
    status: AuctionStatus.active,
    totalBidders: 2,
    createdAt: now,
  );
}

Widget _wrapExplore({
  required Widget child,
  required List<ForSale> listings,
  required List<Auction> auctions,
  required List<String> promotedFixedPriceSaleIds,
  required List<String> promotedAuctionIds,
}) {
  final router = GoRouter(
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) => Scaffold(body: child),
      ),
      GoRoute(
        path: '/for-sale/:forSaleId',
        builder: (context, state) => Scaffold(
          body: Text(
            'for-sale detail ${state.pathParameters['forSaleId']}',
          ),
        ),
      ),
      GoRoute(
        path: '/auction/:auctionId',
        builder: (context, state) => Scaffold(
          body: Text('auction detail ${state.pathParameters['auctionId']}'),
        ),
      ),
    ],
    initialLocation: '/',
  );

  return ProviderScope(
    overrides: [
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
      forSaleRepositoryProvider.overrideWithValue(
        _FakeForSaleRepository(listings),
      ),
      auctionRepositoryProvider.overrideWithValue(
        _FakeAuctionRepository(auctions),
      ),
      explorePromotionDiscoveryServiceProvider.overrideWithValue(
        _FakePromotionDiscoveryService(
          promotedFixedPriceSaleIds: promotedFixedPriceSaleIds,
          promotedAuctionIds: promotedAuctionIds,
        ),
      ),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

void main() {
  testWidgets(
    'listing tab renders promoted listing section, dedups organic items, and excludes external cards',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1080, 2400));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      await tester.pumpWidget(
        _wrapExplore(
          child: const ExploreScreen(initialTab: 0),
          listings: [
            _forSale(id: 'for-sale-1', title: 'Promo ForSale'),
            _forSale(id: 'for-sale-2', title: 'Organic ForSale'),
          ],
          auctions: const [],
          promotedFixedPriceSaleIds: const ['for-sale-1'],
          promotedAuctionIds: const [],
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Listing Dipromosikan'), findsOneWidget);
      expect(find.text('Promo ForSale'), findsOneWidget);
      expect(find.text('Organic ForSale'), findsOneWidget);
      expect(find.byType(PromotedExternalCard), findsNothing);
    },
  );

  testWidgets('listing tab promoted card navigates to for-sale detail', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1080, 2400));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      _wrapExplore(
        child: const ExploreScreen(initialTab: 0),
        listings: [_forSale(id: 'for-sale-1', title: 'Promo ForSale')],
        auctions: const [],
        promotedFixedPriceSaleIds: const ['for-sale-1'],
        promotedAuctionIds: const [],
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byType(ForSaleCard).first);
    await tester.pumpAndSettle();

    expect(find.text('for-sale detail for-sale-1'), findsOneWidget);
  });

  testWidgets(
    'auction tab renders promoted auction section and dedups organic items',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1080, 2400));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      await tester.pumpWidget(
        _wrapExplore(
          child: const ExploreScreen(initialTab: 1),
          listings: const [],
          auctions: [
            _auction(id: 'auction-1', title: 'Promo Auction'),
            _auction(id: 'auction-2', title: 'Organic Auction'),
          ],
          promotedFixedPriceSaleIds: const [],
          promotedAuctionIds: const ['auction-1'],
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Lelang Dipromosikan'), findsOneWidget);
      expect(find.text('Promo Auction'), findsOneWidget);
      expect(find.text('Organic Auction'), findsOneWidget);
      expect(find.byType(PromotedExternalCard), findsNothing);
    },
  );

  testWidgets('auction tab promoted card navigates to auction detail', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1080, 2400));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      _wrapExplore(
        child: const ExploreScreen(initialTab: 1),
        listings: const [],
        auctions: [_auction(id: 'auction-1', title: 'Promo Auction')],
        promotedFixedPriceSaleIds: const [],
        promotedAuctionIds: const ['auction-1'],
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byType(AuctionCard).first);
    await tester.pumpAndSettle();

    expect(find.text('auction detail auction-1'), findsOneWidget);
  });

  testWidgets('empty promotions hide the promoted section', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1080, 2400));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      _wrapExplore(
        child: const ExploreScreen(initialTab: 0),
        listings: [_forSale(id: 'for-sale-1', title: 'Organic ForSale')],
        auctions: const [],
        promotedFixedPriceSaleIds: const [],
        promotedAuctionIds: const [],
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Listing Dipromosikan'), findsNothing);
      expect(find.text('Organic ForSale'), findsOneWidget);
  });

  test('promotion discovery service fails open on transport errors', () async {
    final service = PromotionDiscoveryService(_ThrowingApiClient());

    final listings = await service.getPromotedForSales(limit: 2);
    final auctions = await service.getPromotedAuctions(limit: 2);

    expect(listings, PromotedItemsResponse.empty);
    expect(auctions, PromotedItemsResponse.empty);
  });
}
